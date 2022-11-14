package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/knadh/koanf"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	log "github.com/sirupsen/logrus"
)

type Redpanda struct {
	client   *kgo.Client
	adm      *kadm.Client
	isSource bool
}

var (
	config          = koanf.New(".")
	source          Redpanda
	sourceOnce      sync.Once
	destination     Redpanda
	destinationOnce sync.Once
)

// Closes the source and destination client connections
func shutdown() {
	log.Infoln("Closing client connections")
	source.adm.Close()
	source.client.Close()
	destination.adm.Close()
	destination.client.Close()
}

// Creates new Kafka and Admin clients to communicate with a cluster
func initClient(rp *Redpanda, mutex *sync.Once, pathPrefix string) {
	mutex.Do(func() {
		var err error
		servers := config.String(
			fmt.Sprintf("%s.bootstrap_servers", pathPrefix))
		topics := config.Strings(
			fmt.Sprintf("%s.topics", pathPrefix))
		group := config.String(
			fmt.Sprintf("%s.consumer_group_id", pathPrefix))

		opts := []kgo.Opt{}
		opts = append(opts, kgo.SeedBrokers(strings.Split(servers, ",")...))
		if len(topics) > 0 {
			opts = append(opts,
				kgo.ConsumeTopics(topics...),
				kgo.ConsumerGroup(group),
				kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
				kgo.SessionTimeout(60000*time.Millisecond),
				kgo.DisableAutoCommit(),
				kgo.BlockRebalanceOnPoll())
		}
		tlsPath := fmt.Sprintf("%s.tls", pathPrefix)
		if config.Exists(tlsPath) {
			tlsConfig := TLSConfig{}
			config.Unmarshal(tlsPath, &tlsConfig)
			opts = TLSOpt(&tlsConfig, opts)
		}
		saslPath := fmt.Sprintf("%s.sasl", pathPrefix)
		if config.Exists(saslPath) {
			saslConfig := SASLConfig{}
			config.Unmarshal(saslPath, &saslConfig)
			opts = SASLOpt(&saslConfig, opts)
		}

		rp.client, err = kgo.NewClient(opts...)
		if err != nil {
			log.Fatalf("Unable to load client: %v", err)
		}
		// Check connectivity to cluster
		if err = rp.client.Ping(context.Background()); err != nil {
			log.Errorf("Unable to ping %s cluster: %s",
				pathPrefix, err.Error())
		}

		rp.adm = kadm.NewClient(rp.client)
		brokers, err := rp.adm.ListBrokers(context.Background())
		if err != nil {
			log.Errorf("Unable to list brokers: %v", err)
		}
		log.Infof("Created %s client", pathPrefix)
		for _, broker := range brokers {
			brokerJson, _ := json.Marshal(broker)
			log.Infof("\t %s broker: %s", pathPrefix, string(brokerJson))
		}

		if pathPrefix == "source" {
			rp.isSource = true
		}
	})
}

// Check the topics exist in both clusters. If the topics to not exist then
// this function will attempt to create them if configured to do so.
func checkTopics(cluster *Redpanda, topics []string) {
	ctx := context.Background()
	clusterName := "source"
	if !cluster.isSource {
		clusterName = "destination"
	}
	topicDetails, err := cluster.adm.ListTopics(ctx, topics...)
	if err != nil {
		log.Errorf("Unable to list topics on %v: %v", clusterName, err)
		return
	}
	for _, topic := range topics {
		if !topicDetails.Has(topic) {
			log.Debugf("Topic '%s' does not exist on %s", topic, clusterName)
			if config.Exists("create_topics") {
				resp, _ := cluster.adm.CreateTopics(
					ctx, -1, -1, nil, topic)
				for _, ctr := range resp {
					if ctr.Err != nil {
						log.Warnf("Unable to create topic '%s' on %s: %s",
							ctr.Topic, clusterName, ctr.Err)
					} else {
						log.Infof("Created topic '%s' on %s",
							ctr.Topic, clusterName)
					}
				}
			} else {
				log.Fatalf("Topic '%s' does not exist on %s",
					topic, clusterName)
			}
		} else {
			log.Infof("Topic '%s' already exists on %s",
				topic, clusterName)
		}
	}
}

// Pauses fetching new records when a fetch error is received.
// The backoff period is determined by the number of sequential
// fetch errors received, and it increases exponentially up to
// a maximum number of seconds set by 'maxBackoffSec'.
//
// For example:
//   - 2 fetch errors = 2 ^ 2 = 4 second backoff
//   - 3 fetch errors = 3 ^ 2 = 9 second backoff
//   - 4 fetch errors = 4 ^ 2 = 16 second backoff
func backoff(exp *int) {
	*exp += 1
	backoff := math.Pow(float64(*exp), 2)
	if backoff >= config.Float64("max_backoff_secs") {
		backoff = config.Float64("max_backoff_secs")
	}
	log.Warnf("Backing off for %d seconds", int(backoff))
	time.Sleep(time.Duration(backoff) * time.Second)
}

// Continuously fetches batches of records from the source cluster and
// forwards them to the destination cluster. Consumer offsets are only
// committed when the destination cluster acknowledges the records.
func forwardRecords(ctx context.Context) {
	var errCount int = 0
	var fetches kgo.Fetches
	var sent bool
	var committed bool
	log.Infoln("Starting to forward records...")
	for {
		if (sent && committed) || len(fetches.Records()) == 0 {
			// Only poll when the previous fetches were successfully
			// forwarded and committed
			log.Debug("Polling for records...")
			fetches = source.client.PollRecords(
				ctx, config.Int("max_poll_records"))
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, e := range errs {
					if e.Err == context.Canceled {
						log.Infof("Received interrupt: %s", e.Err)
						return
					}
					log.Errorf("Fetch error: %s", e.Err)
				}
				backoff(&errCount)
			}
			log.Debugf("Consumed %d records", len(fetches.Records()))
			iter := fetches.RecordIter()
			for !iter.Done() {
				// If the record key is empty, then set it to the agent id to
				// route the records to the same topic partition.
				record := iter.Next()
				if record.Key == nil {
					record.Key = []byte(config.String("id"))
				}
			}
			sent = false
			committed = false
		}

		if !sent {
			err := destination.client.ProduceSync(
				ctx, fetches.Records()...).FirstErr()
			if err != nil {
				if err == context.Canceled {
					log.Infof("Received interrupt: %s", err.Error())
					return
				}
				log.Errorf("Unable to forward %d record(s): %s",
					len(fetches.Records()), err.Error())
				backoff(&errCount)
			} else {
				sent = true
				log.Debugf("Forwarded %d records", len(fetches.Records()))
			}
		}

		if sent && !committed {
			if log.GetLevel() == log.DebugLevel {
				offsets := source.client.UncommittedOffsets()
				offsetsJson, _ := json.Marshal(offsets)
				log.Debugf("Committing offsets: %s", string(offsetsJson))
			}
			err := source.client.CommitUncommittedOffsets(ctx)
			if err != nil {
				if err == context.Canceled {
					log.Infof("Received interrupt: %s", err.Error())
					return
				}
				log.Errorf("Unable to commit offsets: %s", err.Error())
				backoff(&errCount)
			} else {
				errCount = 0 // Reset error counter
				committed = true
				log.Debugf("Offsets committed")
			}
		}

		source.client.AllowRebalance()
	}
}

func main() {
	configFile := flag.String("config", "agent.yaml", "path to agent config file")
	logLevelStr := flag.String("loglevel", "info", "logging level")
	flag.Parse()

	logLevel, _ := log.ParseLevel(*logLevelStr)
	log.SetLevel(logLevel)

	InitConfig(configFile, config)
	initClient(&source, &sourceOnce, "source")
	initClient(&destination, &destinationOnce, "destination")

	topics := config.Strings("source.topics")
	checkTopics(&source, topics)
	checkTopics(&destination, topics)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	forwardRecords(ctx)
	ctx.Done()
	stop()
	shutdown()
	log.Infoln("Agent stopped")
}
