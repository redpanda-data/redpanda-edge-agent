package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kversion"
	"github.com/twmb/franz-go/pkg/sasl/aws"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/tlscfg"

	log "github.com/sirupsen/logrus"
)

const schemaTopic = "_schemas"

var (
	lock   = &sync.Mutex{}
	config = koanf.New(".")
)

// Configuration prefix
type Prefix string

const (
	Source      Prefix = "source"
	Destination Prefix = "destination"
)

type Direction int8

const (
	Push Direction = iota // Push from source topic to destination topic
	Pull                  // Pull from destination topic to source topic
)

func (d Direction) String() string {
	switch d {
	case Push:
		return "push"
	case Pull:
		return "pull"
	default:
		return fmt.Sprintf("%d", int(d))
	}
}

type Topic struct {
	sourceName      string
	destinationName string
	direction       Direction
}

func (t Topic) String() string {
	if t.direction == Push {
		return fmt.Sprintf("%s > %s",
			t.sourceName, t.destinationName)
	} else {
		return fmt.Sprintf("%s < %s",
			t.sourceName, t.destinationName)
	}
}

// Returns the name of the topic to consume from.
// If the topic direction is `Push` then consume from the source topic.
// If the topic direction is `Pull` then consume from the destination topic.
func (t Topic) consumeFrom() string {
	if t.direction == Push {
		return t.sourceName
	} else {
		return t.destinationName
	}
}

// Returns the name of the topic to produce to.
// If the topic direction is `Push` then produce to the destination topic.
// If the topic direction is `Pull` then produce to the source topic.
func (t Topic) produceTo() string {
	if t.direction == Push {
		return t.destinationName
	} else {
		return t.sourceName
	}
}

type SASLConfig struct {
	SaslMethod   string `koanf:"sasl_method"`
	SaslUsername string `koanf:"sasl_username"`
	SaslPassword string `koanf:"sasl_password"`
}

type TLSConfig struct {
	Enabled        bool   `koanf:"enabled"`
	ClientKeyFile  string `koanf:"client_key"`
	ClientCertFile string `koanf:"client_cert"`
	CaFile         string `koanf:"ca_cert"`
}

var defaultConfig = confmap.Provider(map[string]interface{}{
	"id":                            defaultID,
	"create_topics":                 false,
	"max_poll_records":              1000,
	"max_backoff_secs":              600, // ten minutes
	"source.name":                   "source",
	"source.bootstrap_servers":      "127.0.0.1:19092",
	"source.consumer_group_id":      defaultID,
	"destination.name":              "destination",
	"destination.bootstrap_servers": "127.0.0.1:29092",
	"destination.consumer_group_id": defaultID,
}, ".")

// Returns the hostname reported by the kernel to use as the default ID for the
// agent and consumer group IDs
var defaultID = func() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Unable to get hostname from kernel. Set Id in config")
	}
	log.Debugf("Hostname: %s", hostname)
	return hostname
}()

// Initialize the agent configuration from the provided .yaml file
func InitConfig(path *string) {
	lock.Lock()
	defer lock.Unlock()

	config.Load(defaultConfig, nil)
	log.Infof("Init config from file: %s", *path)
	if err := config.Load(file.Provider(*path), yaml.Parser()); err != nil {
		log.Errorf("Error loading config: %v", err)
	}
	validate()
	log.Debugf(config.Sprint())
}

// Parse topic configuration
func parseTopics(topics []string, direction Direction) []Topic {
	var all []Topic
	for _, t := range topics {
		s := strings.Split(t, ":")
		if len(s) == 1 {
			all = append(all, Topic{
				sourceName:      strings.TrimSpace(s[0]),
				destinationName: strings.TrimSpace(s[0]),
				direction:       direction,
			})
		} else if len(s) == 2 {
			// Push from source topic to destination topic
			var src = strings.TrimSpace(s[0])
			var dst = strings.TrimSpace(s[1])
			if direction == Pull {
				// Pull from destination topic to source topic
				src = strings.TrimSpace(s[1])
				dst = strings.TrimSpace(s[0])
			}
			all = append(all, Topic{
				sourceName:      src,
				destinationName: dst,
				direction:       direction,
			})
		} else {
			log.Fatalf("Incorrect topic configuration: %s", t)
		}
	}
	return all
}

func GetTopics(p Prefix) []Topic {
	if p == Source {
		return parseTopics(config.Strings("source.topics"), Push)
	} else {
		return parseTopics(config.Strings("destination.topics"), Pull)
	}
}

func AllTopics() []Topic {
	return append(GetTopics(Source), GetTopics(Destination)...)
}

// Check for circular dependency
// t1.src > t1.dst
// t2.src < t2.dst
func circular(t1, t2 *Topic) bool {
	if t1.direction != t2.direction {
		if t1.sourceName == t2.sourceName {
			if t1.destinationName == t2.destinationName {
				return true
			}
		}
	}
	return false
}

// Validate the config
func validate() {
	config.MustString("id")
	config.MustString("source.bootstrap_servers")
	config.MustString("destination.bootstrap_servers")

	topics := AllTopics()
	if len(topics) == 0 {
		log.Fatal("No push or pull topics configured")
	}
	for i, t1 := range topics {
		for k, t2 := range topics {
			if (i != k) && (t1 == t2) {
				log.Fatalf("Duplicate topic configured: %s", t1.String())
			}
			if circular(&t1, &t2) {
				log.Fatalf("Topic circular dependency configured: (%s) (%s)",
					t1.String(), t2.String())
			}
		}
	}
}

// Initializes the necessary TLS configuration options
func TLSOpt(tlsConfig *TLSConfig, opts []kgo.Opt) []kgo.Opt {
	if tlsConfig.Enabled {
		if tlsConfig.CaFile != "" ||
			tlsConfig.ClientCertFile != "" ||
			tlsConfig.ClientKeyFile != "" {
			tc, err := tlscfg.New(
				tlscfg.MaybeWithDiskCA(
					tlsConfig.CaFile, tlscfg.ForClient),
				tlscfg.MaybeWithDiskKeyPair(
					tlsConfig.ClientCertFile, tlsConfig.ClientKeyFile),
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
func SASLOpt(config *SASLConfig, opts []kgo.Opt) []kgo.Opt {
	if config.SaslMethod != "" ||
		config.SaslUsername != "" ||
		config.SaslPassword != "" {

		if config.SaslMethod == "" ||
			config.SaslUsername == "" ||
			config.SaslPassword == "" {
			log.Fatalln("All of SaslMethod, SaslUsername, SaslPassword " +
				"must be specified if any are")
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

// Set the maximum Kafka protocol version to try
func MaxVersionOpt(version string, opts []kgo.Opt) []kgo.Opt {
	ver := strings.ToLower(version)
	ver = strings.ReplaceAll(ver, "v", "")
	ver = strings.ReplaceAll(ver, ".", "")
	ver = strings.ReplaceAll(ver, "_", "")
	verNum, _ := strconv.Atoi(ver)
	switch verNum {
	case 330:
		opts = append(opts, kgo.MaxVersions(kversion.V3_3_0()))
	case 320:
		opts = append(opts, kgo.MaxVersions(kversion.V3_2_0()))
	case 310:
		opts = append(opts, kgo.MaxVersions(kversion.V3_1_0()))
	case 300:
		opts = append(opts, kgo.MaxVersions(kversion.V3_0_0()))
	case 280:
		opts = append(opts, kgo.MaxVersions(kversion.V2_8_0()))
	case 270:
		opts = append(opts, kgo.MaxVersions(kversion.V2_7_0()))
	case 260:
		opts = append(opts, kgo.MaxVersions(kversion.V2_6_0()))
	default:
		opts = append(opts, kgo.MaxVersions(kversion.Stable()))
	}
	return opts
}
