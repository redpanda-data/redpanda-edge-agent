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
	name   string
	topics []string
	client *kgo.Client
	adm    *kadm.Client
}

var (
	config          = koanf.New(".")
	source          Redpanda
	sourceOnce      sync.Once
	destination     Redpanda
	destinationOnce sync.Once
	wg              sync.WaitGroup
)

// Closes the source and destination client connections
func shutdown() {
	log.Infoln("Closing client connections")
	source.adm.Close()
	source.client.Close()
	destination.adm.Close()
	destination.client.Close()
}

// Creates new Kafka and Admin clients to communicate with a cluster.
//
// The `pathPrefix` must be set to either `source` or `destination` as it
// determines what settings are read from the configuration.
//
// The topics listed in `source.topics` are the topics that will be pushed by
// the agent from the source cluster to the destination cluster.
//
// The topics listed in `destination.topics` are the topics that will be pulled
// by the agent from the destination cluster to the source cluster.
func initClient(rp *Redpanda, mutex *sync.Once, pathPrefix string) {
	mutex.Do(func() {
		var err error
		name := config.String(
			fmt.Sprintf("%s.name", pathPrefix))
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

		rp.name = name
		rp.topics = topics
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
		log.Infof("Created %s client", name)
		for _, broker := range brokers {
			brokerJson, _ := json.Marshal(broker)
			log.Infof("\t %s broker: %s", pathPrefix, string(brokerJson))
		}
	})
}

// Check the topics exist on the given cluster. If the topics to not exist then
// this function will attempt to create them if configured to do so.
func CheckTopics(cluster *Redpanda, topics []string) {
	ctx := context.Background()
	topicDetails, err := cluster.adm.ListTopics(ctx, topics...)
	if err != nil {
		log.Errorf("Unable to list topics on %s: %v", cluster.name, err)
		return
	}
	for _, topic := range topics {
		if !topicDetails.Has(topic) {
			if config.Exists("create_topics") {
				// Use the clusters default partition and replication settings
				resp, _ := cluster.adm.CreateTopics(
					ctx, -1, -1, nil, topic)
				for _, ctr := range resp {
					if ctr.Err != nil {
						log.Warnf("Unable to create topic '%s' on %s: %s",
							ctr.Topic, cluster.name, ctr.Err)
					} else {
						log.Infof("Created topic '%s' on %s",
							ctr.Topic, cluster.name)
					}
				}
			} else {
				log.Fatalf("Topic '%s' does not exist on %s",
					topic, cluster.name)
			}
		} else {
			log.Infof("Topic '%s' already exists on %s",
				topic, cluster.name)
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

// Continuously fetch batches of records from the `src` cluster and forward
// them to the `dst` cluster. Consumer offsets are only committed when the
// `dst` cluster acknowledges the records.
func forwardRecords(src *Redpanda, dst *Redpanda, ctx context.Context) {
	defer wg.Done()
	var errCount int = 0
	var fetches kgo.Fetches
	var sent bool
	var committed bool
	log.Infof("Forwarding records from '%s' to '%s'", src.name, dst.name)
	for {
		if (sent && committed) || len(fetches.Records()) == 0 {
			// Only poll when the previous fetches were successfully
			// forwarded and committed
			log.Debug("Polling for records...")
			fetches = src.client.PollRecords(
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
			// Forward records
			err := dst.client.ProduceSync(
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
			// Records have been forwarded successfully, so commit offsets
			if log.GetLevel() == log.DebugLevel {
				offsets := src.client.UncommittedOffsets()
				offsetsJson, _ := json.Marshal(offsets)
				log.Debugf("Committing offsets: %s", string(offsetsJson))
			}
			err := src.client.CommitUncommittedOffsets(ctx)
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

		src.client.AllowRebalance()
	}
}

func main() {
	configFile := flag.String(
		"config", "agent.yaml", "path to agent config file")
	logLevelStr := flag.String(
		"loglevel", "info", "logging level")
	flag.Parse()

	logLevel, _ := log.ParseLevel(*logLevelStr)
	log.SetLevel(logLevel)

	InitConfig(configFile, config)
	initClient(&source, &sourceOnce, "source")
	initClient(&destination, &destinationOnce, "destination")

	allTopics := append([]string(nil), source.topics...)
	allTopics = append(allTopics, destination.topics...)
	CheckTopics(&source, allTopics)
	CheckTopics(&destination, allTopics)

	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, os.Kill)
	if len(source.topics) > 0 {
		wg.Add(1)
		go forwardRecords(&source, &destination, ctx) // Push to destination
	}
	if len(destination.topics) > 0 {
		wg.Add(1)
		go forwardRecords(&destination, &source, ctx) // Pull from destination
	}
	if config.Bool("metrics.enabled") {
		wg.Add(1)
		go SendMetrics(config, &destination)
	}
	wg.Wait()

	ctx.Done()
	stop()
	shutdown()
	log.Infoln("Agent stopped")
}
