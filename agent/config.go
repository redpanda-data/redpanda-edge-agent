package main

import (
	"crypto/tls"
	"os"
	"strconv"
	"strings"

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
func InitConfig(path *string, config *koanf.Koanf) {
	config.Load(defaultConfig, nil)
	log.Infof("Init config from file: %s", *path)
	if err := config.Load(file.Provider(*path), yaml.Parser()); err != nil {
		log.Errorf("Error loading config: %v", err)
	}
	validate(config)
	log.Debugf(config.Sprint())
}

// Validate the config
func validate(config *koanf.Koanf) {
	config.MustString("id")
	config.MustString("source.bootstrap_servers")
	config.MustString("destination.bootstrap_servers")

	sourceTopics := config.Strings("source.topics")
	destinationTopics := config.Strings("destination.topics")
	if len(sourceTopics) == 0 && len(destinationTopics) == 0 {
		log.Fatal("No outbound or inbound topics configured")
	}
	for _, s := range sourceTopics {
		for _, d := range destinationTopics {
			if s == d {
				log.Fatal("Topic circular dependency configured")
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
