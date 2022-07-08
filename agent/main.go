package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/aws"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/tlscfg"

	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type SASLConfig struct {
	SaslMethod   string `yaml:"sasl_method"`
	SaslUsername string `yaml:"sasl_username"`
	SaslPassword string `yaml:"sasl_password"`
}

type TLSConfig struct {
	Enabled        bool
	ClientKeyFile  string `yaml:"client_key"`
	ClientCertFile string `yaml:"client_cert"`
	CaFile         string `yaml:"ca_cert"`
}

type SourceCluster struct {
	BootstrapServers   string `yaml:"bootstrap_servers"`
	Topics             []string
	ConsumerGroup      string     `yaml:"consumer_group_id"`
	MaxPollRecords     int        `yaml:"max_poll_records"`
	DefaultPartitions  int32      `yaml:"default_partitions"`
	DefaultReplication int16      `yaml:"default_replication"`
	SASL               SASLConfig `yaml:"sasl"`
	TLS                TLSConfig  `yaml:"tls"`
}

type DestinationCluster struct {
	BootstrapServers   string     `yaml:"bootstrap_servers"`
	DefaultPartitions  int32      `yaml:"default_partitions"`
	DefaultReplication int16      `yaml:"default_replication"`
	SASL               SASLConfig `yaml:"sasl"`
	TLS                TLSConfig  `yaml:"tls"`
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
var defaultID = func() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Unable to get hostname from kernel. Set Id in config")
	}
	log.Debugf("Hostname: %s", hostname)
	return hostname
}()

var defaultLogFile = func() string {
	logName := "agent.log"
	exPath, err := os.Executable()
	if err != nil {
		return filepath.Join("/var/log/redpanda", logName)
	}
	exDir := filepath.Dir(exPath)
	return filepath.Join(exDir, logName)
}()

func initClients(path *string) {
	initConfig(path)
	initClient(&src, &srcOnce)
	initClient(&dst, &dstOnce)
	initTopics()
}

// Closes the source and destination client connections
func shutdown() {
	log.Infoln("Closing client connections")
	src.adm.Close()
	src.client.Close()
	dst.adm.Close()
	dst.client.Close()
}

// Initializes the agent configuration from the provided .yaml file
func initConfig(path *string) {
	log.Infof("Init config from file: %s", *path)
	buf, err := ioutil.ReadFile(*path)
	if err != nil {
		log.Errorf("Unable to read config file: %s", *path)
		return
	}
	config = Config{
		Id:           defaultID,
		CreateTopics: false,
		Source: SourceCluster{
			BootstrapServers:   "127.0.0.1:9092",
			ConsumerGroup:      defaultID,
			MaxPollRecords:     1000,
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

// Creates new Kafka and Admin clients to communicate with a cluster
func initClient(c *Redpanda, m *sync.Once) {
	m.Do(func() {
		var err error
		opts := []kgo.Opt{}
		clusterStr := "Destination"
		if c.isSource {
			clusterStr = "Source"
			opts = append(opts,
				kgo.SeedBrokers(strings.Split(config.Source.BootstrapServers, ",")...),
				kgo.ConsumeTopics(config.Source.Topics...),
				kgo.ConsumerGroup(config.Source.ConsumerGroup),
				kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
				kgo.SessionTimeout(60000*time.Millisecond),
				kgo.DisableAutoCommit(),
				kgo.BlockRebalanceOnPoll(),
			)
			opts = tlsOpt(&config.Source.TLS, opts)
			opts = saslOpt(&config.Source.SASL, opts)
		} else {
			opts = append(opts,
				kgo.SeedBrokers(strings.Split(config.Destination.BootstrapServers, ",")...),
			)
			opts = tlsOpt(&config.Destination.TLS, opts)
			opts = saslOpt(&config.Destination.SASL, opts)
		}

		c.client, err = kgo.NewClient(opts...)
		if err != nil {
			log.Fatalf("Unable to load client: %v", err)
		}
		if err = c.client.Ping(context.Background()); err != nil { // check connectivity to cluster
			log.Errorf("Unable to ping %s cluster: %s", clusterStr, err.Error())
		}

		c.adm = kadm.NewClient(c.client)
		brokers, err := c.adm.ListBrokers(context.Background())
		if err != nil {
			log.Errorf("Unable to list brokers: %v", err)
		}
		log.Infof("Created %s client", clusterStr)
		for _, broker := range brokers {
			brokerJson, _ := json.Marshal(broker)
			log.Infof("%s broker: %s", clusterStr, string(brokerJson))
		}
	})
}

// Initializes the necessary TLS configuration options
func tlsOpt(config *TLSConfig, opts []kgo.Opt) []kgo.Opt {
	if config.Enabled {
		if config.CaFile != "" || config.ClientCertFile != "" || config.ClientKeyFile != "" {
			tc, err := tlscfg.New(
				tlscfg.MaybeWithDiskCA(config.CaFile, tlscfg.ForClient),
				tlscfg.MaybeWithDiskKeyPair(config.ClientCertFile, config.ClientKeyFile),
			)
			if err != nil {
				log.Fatalf("Unable to create TLS config: %v", err)
			}
			opts = append(opts, kgo.DialTLSConfig(tc))
		} else {
			opts = append(opts, kgo.DialTLSConfig(new(tls.Config)))
		}
	}
	return opts
}

// Initializes the necessary SASL configuration options
func saslOpt(config *SASLConfig, opts []kgo.Opt) []kgo.Opt {
	if config.SaslMethod != "" || config.SaslUsername != "" || config.SaslPassword != "" {
		if config.SaslMethod == "" || config.SaslUsername == "" || config.SaslPassword == "" {
			log.Fatalln("All of SaslMechanism, SaslUsername, SaslPassword must be specified if any are")
		}
		method := strings.ToLower(config.SaslMethod)
		method = strings.ReplaceAll(method, "-", "")
		method = strings.ReplaceAll(method, "_", "")
		switch method {
		case "plain":
			opts = append(opts, kgo.SASL(plain.Auth{
				User: config.SaslUsername,
				Pass: config.SaslPassword,
			}.AsMechanism()))
		case "scramsha256":
			opts = append(opts, kgo.SASL(scram.Auth{
				User: config.SaslUsername,
				Pass: config.SaslPassword,
			}.AsSha256Mechanism()))
		case "scramsha512":
			opts = append(opts, kgo.SASL(scram.Auth{
				User: config.SaslUsername,
				Pass: config.SaslPassword,
			}.AsSha512Mechanism()))
		case "awsmskiam":
			opts = append(opts, kgo.SASL(aws.Auth{
				AccessKey: config.SaslUsername,
				SecretKey: config.SaslPassword,
			}.AsManagedStreamingIAMMechanism()))
		default:
			log.Fatalf("Unrecognized sasl method: %s", method)
		}
	}
	return opts
}

func initTopics() {
	checkTopics(&src, &config.Source.Topics)
	checkTopics(&dst, &config.Source.Topics)
}

// Check the source topics exist in both the source and destination
// clusters. If the topics to not exist then this function will
// attempt to create them if configured to do so.
func checkTopics(cluster *Redpanda, topics *[]string) {
	clusterStr := "source"
	part := config.Source.DefaultPartitions
	repl := config.Source.DefaultReplication
	if !cluster.isSource {
		clusterStr = "destination"
		part = config.Destination.DefaultPartitions
		repl = config.Destination.DefaultReplication
	}
	topicDetails, err := cluster.adm.ListTopics(context.Background(), *topics...)
	if err != nil {
		log.Errorf("Unable to list %s topics: %s", clusterStr, err.Error())
		return
	}
	for _, topic := range *topics {
		exists := topicDetails.Has(topic)
		if !exists {
			if config.CreateTopics {
				resp, _ := cluster.adm.CreateTopics(context.Background(), part, repl, nil, topic)
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
	if backoff >= float64(maxBackoffSec) {
		backoff = float64(maxBackoffSec)
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
			// Only poll when the previous fetches were successfully forwarded and committed
			log.Debug("Polling for records...")
			fetches = src.client.PollRecords(ctx, config.Source.MaxPollRecords)
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
				// If the record key is empty, then set it
				// to the agent id to route the records to
				// the same topic partition.
				record := iter.Next()
				if record.Key == nil {
					record.Key = []byte(config.Id)
				}
			}
			sent = false
			committed = false
		}

		if !sent {
			err := dst.client.ProduceSync(ctx, fetches.Records()...).FirstErr()
			if err != nil {
				if err == context.Canceled {
					log.Infof("Received interrupt: %s", err.Error())
					return
				}
				log.Errorf("Unable to forward %d record(s): %s", len(fetches.Records()), err.Error())
				backoff(&errCount)
			} else {
				sent = true
				log.Debugf("Forwarded %d records", len(fetches.Records()))
			}
		}

		if sent && !committed {
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
				errCount = 0 // reset error counter
				committed = true
				log.Debugf("Offsets committed")
			}
		}

		src.client.AllowRebalance()
	}
}

func main() {
	configFile := flag.String("config", "agent.yaml", "path to agent config file")
	logLevelStr := flag.String("loglevel", "info", "logging level")
	enableLogFile := flag.Bool("enablelog", false, "log to a file instead of stderr")
	logFile := flag.String("logfile", defaultLogFile, "if 'enablelog' is true, then log to this file")
	flag.Parse()

	logLevel, _ := log.ParseLevel(*logLevelStr)
	log.SetLevel(logLevel)
	if *enableLogFile {
		logOutput, err := os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			log.Fatalf("Unable to open log file: %v", err)
		}
		defer logOutput.Close()
		log.SetOutput(logOutput)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	initClients(configFile)
	forwardRecords(ctx)
	stop()
	shutdown()
	log.Infoln("Agent stopped")
}
