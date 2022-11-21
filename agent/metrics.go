package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/knadh/koanf"
	log "github.com/sirupsen/logrus"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Heartbeat struct {
	id        string
	timestamp int64
}

func (h Heartbeat) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        string
		Timestamp int64
	}{
		ID:        h.id,
		Timestamp: h.timestamp,
	})
}

func SendMetrics(config *koanf.Koanf, cluster *Redpanda) {
	ctx := context.Background()
	var wg sync.WaitGroup

	id := config.String("id")
	topic := config.String("metrics.topic")
	interval := config.Int("metrics.interval_sec")
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Create the metrics topic on the destination cluster
	// if it doesn't already exist
	CheckTopics(cluster, []string{topic})
	log.Infof("Sending metrics to topic '%s' every %d second(s)",
		topic, interval)

	for ts := range ticker.C {
		hb := &Heartbeat{
			id:        id,
			timestamp: ts.UnixMilli(),
			// TODO: add metrics to heartbeat message
		}
		value, err := json.Marshal(hb)
		if err != nil {
			log.Errorf("Unable to encode heartbeat: %v", err)
		}
		log.Debugf("Heartbeat: %s", value)

		wg.Add(1)
		record := &kgo.Record{
			Topic:     topic,
			Key:       []byte(id),
			Value:     value,
			Timestamp: ts}
		cluster.client.Produce(ctx, record, func(_ *kgo.Record, err error) {
			defer wg.Done()
			if err != nil {
				log.Errorf("Unable to send heartbeat: %v", err)
			}
		})
		wg.Wait()
	}
}
