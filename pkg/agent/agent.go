package main

import (
	"context"
	"encoding/json"
	"flag"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type SourceCluster struct {
	BootstrapServers   string `yaml:"bootstrap_servers"`
	Topics             []string
	DefaultPartitions  int32 `yaml:"default_partitions"`
	DefaultReplication int16 `yaml:"default_replication"`
}

type DestinationCluster struct {
	BootstrapServers   string `yaml:"bootstrap_servers"`
	DefaultPartitions  int32  `yaml:"default_partitions"`
	DefaultReplication int16  `yaml:"default_replication"`
}

type Config struct {
	Id           string
	CreateTopics bool `yaml:"create_topics"`
	Source       SourceCluster
	Destination  DestinationCluster
}

type Redpanda struct {
	client   *kgo.Client
	adm      *kadm.Client
	isSource bool
}

var (
	config        Config
	src           Redpanda
	srcOnce       sync.Once
	dst           Redpanda
	dstOnce       sync.Once
	maxBackoffSec int = 600 // ten minutes
)

// Returns the hostname reported by the kernel
// to use as the default ID for the agent.
func defaultID() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Unable to get hostname from kernel. Set Id in config")
	}
	log.Debugf("Hostname: %s", hostname)
	return hostname
}

func initClients(path *string) {
	initConfig(path)
	initClient(&src, &srcOnce)
	initClient(&dst, &dstOnce)
	initTopics()
}

func shutdown() {
	log.Infoln("Closing client connections")
	src.adm.Close()
	src.client.Close()
	dst.adm.Close()
	dst.client.Close()
}

func initConfig(path *string) {
	log.Infof("Init config from file: %s", *path)
	buf, err := ioutil.ReadFile(*path)
	if err != nil {
		log.Errorf("Unable to read config file: %s", *path)
		return
	}
	config = Config{
		Id:           defaultID(),
		CreateTopics: false,
		Source: SourceCluster{
			BootstrapServers:   "127.0.0.1:9092",
			DefaultPartitions:  1,
			DefaultReplication: 1,
		},
		Destination: DestinationCluster{
			DefaultPartitions:  3,
			DefaultReplication: 3,
		},
	}
	err = yaml.Unmarshal(buf, &config)
	if err != nil {
		log.Errorf("Unable to decode .yaml file: %v", path)
		return
	}
	cJson, _ := json.Marshal(config)
	log.Infof("Config: %s", string(cJson))

	src.isSource = true
}

func initClient(c *Redpanda, m *sync.Once) {
	m.Do(func() {
		var err error
		opt := []kgo.Opt{}
		if c.isSource {
			opt = append(opt,
				kgo.SeedBrokers(strings.Split(config.Source.BootstrapServers, ",")...),
				kgo.ConsumeTopics(config.Source.Topics...),
				kgo.ConsumerGroup(config.Id),
				kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
				kgo.DisableAutoCommit(),
				kgo.SessionTimeout(60000*time.Millisecond),
			)
		} else {
			opt = append(opt,
				kgo.SeedBrokers(strings.Split(config.Destination.BootstrapServers, ",")...),
			)
		}
		c.client, err = kgo.NewClient(opt...)
		if err != nil {
			log.Fatalf("Unable to load client: %v", err)
		}
		c.adm = kadm.NewClient(c.client)
		brokers, err := c.adm.ListBrokers(context.Background())
		if err != nil {
			log.Fatalf("Unable to list brokers: %v", err)
		}
		msg := "Destination"
		if c.isSource {
			msg = "Source"
		}
		for _, broker := range brokers {
			brokerJson, _ := json.Marshal(broker)
			log.Infof("%s broker: %s\n", msg, string(brokerJson))
		}
	})
}

func initTopics() {
	checkTopics(&src, &config.Source.Topics)
	checkTopics(&dst, &config.Source.Topics)
}

// Check the source topics exist in both the source and destination
// clusters. If the topics to not exist then this function will
// attempt to create them if configured to do so.
func checkTopics(cluster *Redpanda, topics *[]string) {
	ctx := context.Background()
	clusterStr := "source"
	part := config.Source.DefaultPartitions
	repl := config.Source.DefaultReplication
	if !cluster.isSource {
		clusterStr = "destination"
		part = config.Destination.DefaultPartitions
		repl = config.Destination.DefaultReplication
	}
	topicDetails, err := cluster.adm.ListTopics(ctx, *topics...)
	if err != nil {
		log.Errorf("Unable to list %s topics: %s", clusterStr, err.Error())
		return
	}
	for _, topic := range *topics {
		exists := topicDetails.Has(topic)
		if !exists {
			if config.CreateTopics {
				resp, _ := cluster.adm.CreateTopics(ctx, part, repl, nil, topic)
				for _, ctr := range resp {
					if ctr.Err != nil {
						log.Warnf("Unable to create %s topic '%s': %s", clusterStr, ctr.Topic, ctr.Err)
					} else {
						log.Infof("Created %s topic '%s'", clusterStr, ctr.Topic)
					}
				}
			} else {
				log.Fatalf("Topic '%s' does not exist in %s cluster", topic, clusterStr)
			}
		} else {
			log.Infof("Topic '%s' already exists in %s cluster", topic, clusterStr)
		}
	}
}

func backoff(exp int) {
	backoff := math.Pow(float64(exp), 2)
	if backoff >= float64(maxBackoffSec) {
		backoff = float64(maxBackoffSec)
	}
	log.Warnf("Backing off for %d seconds...", int(backoff))
	time.Sleep(time.Duration(backoff) * time.Second)
}

func forwardRecords(ctx context.Context) {
	errCount := 0
	log.Infoln("Starting to forward records...")
	for {
		fetches := src.client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				if e.Err == context.Canceled {
					log.Infof("Received interrupt: %s", e.Err)
					return
				}
				errCount += 1
				log.Errorf("Fetch error: %s", e.Err)
			}
			backoff(errCount)
		}
		iter := fetches.RecordIter()
		log.Debugf("Consumed %d records", len(fetches.Records()))
		for !iter.Done() {
			// Set key to the agent id to route records
			// to the same topic partition.
			record := iter.Next()
			record.Key = []byte(config.Id)
		}
		err := dst.client.ProduceSync(ctx, fetches.Records()...).FirstErr()
		if err != nil {
			errCount += 1
			log.Errorf("Unable to forward %d record(s)", len(fetches.Records()))
			backoff(errCount)
		} else {
			log.Debugf("Forwarded %d records", len(fetches.Records()))
			if log.GetLevel() == log.DebugLevel {
				offsets := src.client.UncommittedOffsets()
				offsetsJson, _ := json.Marshal(offsets)
				log.Debugf("Committing offsets: %s", string(offsetsJson))
			}
			err := src.client.CommitUncommittedOffsets(ctx)
			if err != nil {
				log.Errorf("Unable to commit offsets: %s", err.Error())
			} else {
				log.Debugf("Offsets committed")
			}
		}
	}
}

func main() {
	configFile := flag.String("config", "configs/config.yaml", "path to config file")
	logLevelStr := flag.String("loglevel", "info", "logging level")
	flag.Parse()

	logLevel, _ := log.ParseLevel(*logLevelStr)
	log.SetLevel(logLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	initClients(configFile)
	forwardRecords(ctx)
	stop()
	shutdown()
	log.Infoln("Agent stopped")
}
