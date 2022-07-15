package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	defaultDelayMs  int    = 100
	defaultTopic    string = "telemetry"
	defaultInputDir string = "./data"
	defaultBrokers  string = "localhost:9092"
)

type XmlTrackPoint struct {
	Latitude  string `xml:"lat,attr"`
	Longitude string `xml:"lon,attr"`
	Elevation string `xml:"ele"`
}

type XmlTrack struct {
	Athlete      string          `xml:"athlete"`
	Team         string          `xml:"team"`
	Name         string          `xml:"name"`
	Type         string          `xml:"type"`
	TrackSegment []XmlTrackPoint `xml:"trkseg>trkpt"`
}

type XmlGpx struct {
	XMLName xml.Name `xml:"gpx"`
	Track   XmlTrack `xml:"trk"`
}

type TrackPoint struct {
	Time      int
	Athlete   string
	Team      string
	Name      string
	Latitude  string
	Longitude string
	Elevation string
}

func send(client *kgo.Client, topic *string, gpx *XmlGpx, wg *sync.WaitGroup) {
	defer wg.Done()

	for i, trkpt := range gpx.Track.TrackSegment {
		tp := &TrackPoint{
			Time:      i,
			Athlete:   gpx.Track.Athlete,
			Team:      gpx.Track.Team,
			Name:      gpx.Track.Name,
			Latitude:  trkpt.Latitude,
			Longitude: trkpt.Longitude,
			Elevation: trkpt.Elevation,
		}
		tpJson, _ := json.Marshal(tp)
		println(string(tpJson))

		r := &kgo.Record{
			Topic: *topic,
			Key:   []byte(tp.Athlete),
			Value: []byte(string(tpJson)),
		}
		if err := client.ProduceSync(context.Background(), r).FirstErr(); err != nil {
			log.Printf("Synchronous produce error: %s", err.Error())
		}
		time.Sleep(time.Duration(defaultDelayMs) * time.Millisecond)
	}
}

func main() {
	dir := flag.String("dir", defaultInputDir, "directory containing .gpx files")
	brokers := flag.String("brokers", defaultBrokers, "Kafka API bootstrap servers")
	topic := flag.String("topic", defaultTopic, "Produce events to this topic")
	flag.Parse()

	opts := []kgo.Opt{}
	opts = append(opts,
		kgo.SeedBrokers(*brokers),
	)
	client, err := kgo.NewClient(opts...)
	if err != nil {
		log.Fatalf("Unable to create client: %v", err)
	}
	if err = client.Ping(context.Background()); err != nil { // check connectivity
		log.Fatalf("Unable to connect: %s", err.Error())
	}

	adm := kadm.NewClient(client)
	_, err = adm.CreateTopics(context.Background(), 1, 1, nil, []string{*topic}...)
	if err != nil {
		log.Fatalf("Unable to create topic '%s': %s", *topic, err.Error())
	}

	files, err := ioutil.ReadDir(*dir)
	if err != nil {
		log.Fatalf("Unable to read directory: %s", *dir)
	}
	gpxFiles := []string{}
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".gpx") {
			gpxFiles = append(gpxFiles, filepath.Join(*dir, f.Name()))
		}
	}

	var wg sync.WaitGroup
	for _, gpxFile := range gpxFiles {
		log.Printf("Reading .gpx file: %s", gpxFile)
		data, _ := ioutil.ReadFile(gpxFile)
		gpx := &XmlGpx{}
		if err := xml.Unmarshal([]byte(data), &gpx); err != nil {
			log.Fatal(err.Error())
		}
		wg.Add(1)
		go send(client, topic, gpx, &wg)
	}
	wg.Wait()
	log.Printf("Done!")
}
