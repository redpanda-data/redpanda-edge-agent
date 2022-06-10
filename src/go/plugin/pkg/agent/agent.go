package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"strings"
	"sync"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent struct {
		Id string
	}
	Source struct {
		BootstrapServers string `yaml:"bootstrap_servers"`
		Topics           []string
	}
	Destination struct {
		BootstrapServers string `yaml:"bootstrap_servers"`
	}
}

type Redpanda struct {
	config   *Config
	client   *kgo.Client
	adm      *kadm.Client
	isSource bool
}

var (
	local      Redpanda
	localOnce  sync.Once
	remote     Redpanda
	remoteOnce sync.Once
)

func initConfig(path *string) {
	log.Printf("Init config from file: %s\n", *path)
	buf, err := ioutil.ReadFile(*path)
	if err != nil {
		log.Printf("Error reading config file: %s\n", *path)
		return
	}
	c := &Config{}
	err = yaml.Unmarshal(buf, c)
	if err != nil {
		log.Printf("Error decoding .yaml file: %v\n", path)
		return
	}
	cJson, _ := json.Marshal(c)
	log.Printf("Config: %s\n", string(cJson))

	local.config = c
	local.isSource = true
	remote.config = c
}

func initClient(c *Redpanda, m *sync.Once) {
	m.Do(func() {
		var err error
		opt := []kgo.Opt{}
		if c.isSource {
			opt = append(opt,
				kgo.SeedBrokers(strings.Split(c.config.Source.BootstrapServers, ",")...),
				kgo.ConsumeTopics(c.config.Source.Topics...),
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

func checkTopicsExist() {
	topicDetails, err := local.adm.ListTopics(context.Background(), local.config.Source.Topics...)
	if err != nil {
		log.Printf("Unable to list topics: %v", err)
		return
	}
	for _, topic := range local.config.Source.Topics {
		exists := topicDetails.Has(topic)
		if !exists {
			log.Fatalf("Error: source topic '%s' does not exist", topic)
		}
	}
}

func main() {
	config := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	initConfig(config)
	initClient(&local, &localOnce)
	initClient(&remote, &remoteOnce)
	checkTopicsExist()
}
