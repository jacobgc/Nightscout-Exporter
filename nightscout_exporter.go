package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace                 = "nightscout" // For Prometheus metrics.
	defaultAddress            = ":9552"
	defaultTelemetryEndpoint  = "/metrics"
	defaultNightscoutEndpoint = ""
)

// Exporter collects nightscout stats from machine of a specified user and exports them using
// the prometheus metrics package.
type Exporter struct {
	mutex                sync.RWMutex
	sgvStatusGauge       *prometheus.GaugeVec
	trendStatusGauge     *prometheus.GaugeVec
	directionStatusGauge *prometheus.GaugeVec
	bgdeltaStatusGauge   *prometheus.GaugeVec
	nightscoutAddress    string
}

type NightscoutPebble struct {
	Status []struct {
		Now int64 `json:"now"`
	} `json:"status"`
	Bgs []struct {
		Sgv       string `json:"sgv"`
		Trend     int    `json:"trend"`
		Direction string `json:"direction"`
		Datetime  int64  `json:"datetime"`
		Bgdelta   string `json:"bgdelta"`
	} `json:"bgs"`
	Cals []interface{} `json:"cals"`
}

func getJson(url string) NightscoutPebble {
	r, err := http.Get(url + "/pebble?count=1&units=mmol")
	if err != nil {
		fmt.Println("got error1", err.Error())
	}
	defer r.Body.Close()

	bar := NightscoutPebble{}
	err2 := json.NewDecoder(r.Body).Decode(&bar)
	if err2 != nil {
		fmt.Println("error:", err2.Error())
	}

	return bar

}

// NewNightscoutCheckerExporter returns an initialized Exporter.
func NewNightscoutCheckerExporter() *Exporter {

	return &Exporter{
		mutex: sync.RWMutex{},
		sgvStatusGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "sgv",
				Help:      "The current sgv",
			}, []string{"glucosetype", "url"}),
		trendStatusGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "trend",
				Help:      "The current trend enum",
			}, []string{"glucosetype", "url"}),
		bgdeltaStatusGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "background_delta",
				Help:      "The current background delta in mmol",
			}, []string{"glucosetype", "url"}),
	}

}

// Describe describes all the metrics ever exported by the nightscout exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.sgvStatusGauge.Describe(ch)
	e.trendStatusGauge.Describe(ch)
	e.bgdeltaStatusGauge.Describe(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) error {
	e.sgvStatusGauge.Reset()

	data := getJson(e.nightscoutAddress)
	glucose, _ := strconv.ParseFloat(data.Bgs[0].Sgv, 64)
	bgdelta, _ := strconv.ParseFloat(data.Bgs[0].Bgdelta, 64)

	e.sgvStatusGauge.With(prometheus.Labels{"glucosetype": "mmol", "url": e.nightscoutAddress}).Set(glucose)
	e.trendStatusGauge.With(prometheus.Labels{"glucosetype": "mmol", "url": e.nightscoutAddress}).Set(float64(data.Bgs[0].Trend))
	e.bgdeltaStatusGauge.With(prometheus.Labels{"glucosetype": "mmol", "url": e.nightscoutAddress}).Set(bgdelta)

	return nil
}

// Collect fetches the stats of a user and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
	if err := e.scrape(ch); err != nil {
		log.Printf("Error scraping nightscout url: %s", err)
	}

	e.sgvStatusGauge.Collect(ch)
	e.trendStatusGauge.Collect(ch)
	e.bgdeltaStatusGauge.Collect(ch)

	return
}

func main() {
	metricsPath := ""
	listenAddress := ""
	nightscoutAddress := ""

	val, ok := os.LookupEnv("TELEMETRY_ADDRESS")
	if ok {
		listenAddress = val
	} else {
		listenAddress = defaultAddress
	}

	val, ok = os.LookupEnv("TELEMETRY_ENDPOINT")
	if ok {
		metricsPath = val
	} else {
		metricsPath = defaultTelemetryEndpoint
	}

	val, ok = os.LookupEnv("NIGHTSCOUT_ENDPOINT")
	if ok {
		nightscoutAddress = val
	} else {
		nightscoutAddress = defaultNightscoutEndpoint
	}

	if nightscoutAddress == "" {
		log.Fatal("NIGHTSCOUT_ENDPOINT NOT SET")
	}

	exporter := NewNightscoutCheckerExporter()
	exporter.nightscoutAddress = nightscoutAddress
	prometheus.MustRegister(exporter)
	http.Handle(metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
                <head><title>Nightscout exporter</title></head>
                <body>
                   <h1>nightscout exporter</h1>
                   <p><a href='` + metricsPath + `'>Metrics</a></p>
                   </body>
                </html>
              `))
	})
	println("Starting Server: ", listenAddress)
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
