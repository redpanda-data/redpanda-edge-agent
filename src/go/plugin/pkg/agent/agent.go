package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"io/ioutil"

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
	Id          string
	Source      SourceCluster
	Destination DestinationCluster
}

type Redpanda struct {
	config   *Config
	client   *kgo.Client
	adm      *kadm.Client
	isSource bool
}

var (
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
	log.Printf("Hostname: %s", hostname)
	return hostname
}

func initConfig(path *string) {
	log.Printf("Init config from file: %s\n", *path)
	buf, err := ioutil.ReadFile(*path)
	if err != nil {
		log.Printf("Error reading config file: %s\n", *path)
		return
	}
	c := Config{
		Id: defaultID(),
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
	err = yaml.Unmarshal(buf, &c)
	if err != nil {
		log.Printf("Error decoding .yaml file: %v\n", path)
		return
	}
	cJson, _ := json.Marshal(c)
	log.Printf("Config: %s\n", string(cJson))

	src.config = &c
	src.isSource = true
	dst.config = &c
}

func initClient(c *Redpanda, m *sync.Once) {
	m.Do(func() {
		var err error
		opt := []kgo.Opt{}
		if c.isSource {
			opt = append(opt,
				kgo.SeedBrokers(strings.Split(c.config.Source.BootstrapServers, ",")...),
				kgo.ConsumeTopics(c.config.Source.Topics...),
				kgo.ConsumerGroup(c.config.Id),
				kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
				kgo.DisableAutoCommit(),
				kgo.SessionTimeout(60000*time.Millisecond),
			)
		} else {
			opt = append(opt,
				kgo.SeedBrokers(strings.Split(c.config.Destination.BootstrapServers, ",")...),
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
		for _, b := range brokers {
			bJson, _ := json.Marshal(b)
			log.Printf("%s broker: %s\n", msg, string(bJson))
		}
	})
}

// Check the source topics exist in both the source and destination
// clusters. If the topics to not exist then this function will
// attempt to create them.
func checkTopics(topics *[]string) {
	for _, topic := range *topics {
		createTopic(
			src.adm,
			src.config.Source.DefaultPartitions,
			src.config.Source.DefaultReplication,
			topic,
		)
		createTopic(
			dst.adm,
			dst.config.Source.DefaultPartitions,
			dst.config.Source.DefaultReplication,
			topic,
		)
	}
}

func createTopic(adm *kadm.Client, partitions int32, replication int16, topic string) {
	resp, _ := adm.CreateTopics(
		context.Background(),
		partitions,
		replication,
		nil,
		topic,
	)
	for _, ctr := range resp {
		if ctr.Err != nil {
			log.Printf("Unable to create topic '%s': %s", ctr.Topic, ctr.Err)
		} else {
			log.Printf("Created topic '%s'", ctr.Topic)
		}
	}
}

func backoff(exp int) {
	backoff := math.Pow(float64(exp), 2)
	if backoff >= float64(maxBackoffSec) {
		backoff = float64(maxBackoffSec)
	}
	log.Printf("Backing off for %d seconds...", int(backoff))
	time.Sleep(time.Duration(backoff) * time.Second)
}

func forwardRecords() {
	ctx := context.Background()
	errCount := 0
	for {
		fetches := src.client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				errCount += 1
				log.Printf("Fetch error: %s", e.Err)
			}
			backoff(errCount)
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			// Set key to the agent id to route records
			// to the same topic partition.
			record := iter.Next()
			record.Key = []byte(dst.config.Id)
		}
		err := dst.client.ProduceSync(ctx, fetches.Records()...).FirstErr()
		if err != nil {
			errCount += 1
			log.Printf("Error forwarding %d records", len(fetches.Records()))
			backoff(errCount)
		} else {
			err := src.client.CommitUncommittedOffsets(ctx)
			if err != nil {
				offsets := src.client.UncommittedOffsets()
				offsetsJson, _ := json.Marshal(offsets)
				log.Printf("Error comitting offsets: %v", string(offsetsJson))
			}
		}
	}
}

func main() {
	config := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	initConfig(config)
	initClient(&src, &srcOnce)
	initClient(&dst, &dstOnce)
	checkTopics(&src.config.Source.Topics)
	forwardRecords()
}
