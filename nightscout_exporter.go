package main

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace                   = "nightscout" // For Prometheus metrics.
	defaultAddress              = ":9552"
	defaultTelemetryEndpoint    = "/metrics"
	defaultNightscoutEndpoint   = ""
	defaultNightscoutToken      = ""
	defaultBloodGlucoseStandard = "UK"
)

// Taken from: https://github.com/nightscout/cgm-remote-monitor/blob/46418c7ff275ae80de457209c1686811e033b5dd/lib/server/pebble.js#L8-L19
const (
	DoubleUp       = 1
	SingleUp       = 2
	FortyFiveUp    = 3
	Flat           = 4
	FortyFiveDown  = 5
	SingleDown     = 6
	DoubleDown     = 7
	NotComputable  = 8
	RateOutOfRange = 9
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
	token                string
	bloodGlucoseStandard string
	logger               *zap.Logger
}

type APIResponse []struct {
	ID         string    `json:"_id"`
	Device     string    `json:"device"`
	Sgv        int       `json:"sgv"`
	Direction  string    `json:"direction"`
	Noise      int       `json:"noise"`
	SysTime    time.Time `json:"sysTime"`
	Date       int64     `json:"date"`
	DateString time.Time `json:"dateString"`
	Type       string    `json:"type"`
	Unfiltered int       `json:"unfiltered"`
	Filtered   int       `json:"filtered"`
	UtcOffset  int       `json:"utcOffset"`
	Mills      int64     `json:"mills"`
}

func getJson(url string, token string) APIResponse {
	logger := zap.L()
	appendUrl := "/api/v1/entries?count=2"
	if token != "" {
		appendUrl = appendUrl + "&token=" + token
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url+appendUrl, nil)
	req.Header.Set("Accept", "application/json")
	r, err := client.Do(req)

	if err != nil {
		logger.Fatal("Failed to get data from NightScout", zap.Error(err))
	}
	defer r.Body.Close()

	if r.StatusCode == 401 {
		logger.Fatal("Failed to get data from NightScout (Authorization Invalid/Missing)", zap.Error(err))
	}

	bar := APIResponse{}
	err = json.NewDecoder(r.Body).Decode(&bar)
	if err != nil {
		logger.Fatal("Failed to decode data from NightScout", zap.Error(err))
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
		logger: zap.L(),
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

	data := getJson(e.nightscoutAddress, e.token)

	sgv := 0.0
	delta := 0.0

	if e.bloodGlucoseStandard == "US" {
		sgv = float64(data[0].Sgv)
		delta = float64(data[0].Sgv - data[1].Sgv)
	} else {
		// Convert mg/dl to mmol/l
		sgv1 := float64(data[0].Sgv) / 18
		sgv2 := float64(data[1].Sgv) / 18

		sgv = math.Round(float64(sgv1)*100) / 100
		delta = math.Round(float64(sgv1-sgv2)*100) / 100
	}

	trend := 10
	if data[0].Direction == "DoubleUp" {
		trend = DoubleUp
	}
	if data[0].Direction == "SingleUp" {
		trend = SingleUp
	}
	if data[0].Direction == "FortyFiveUp" {
		trend = FortyFiveUp
	}
	if data[0].Direction == "Flat" {
		trend = Flat
	}
	if data[0].Direction == "FortyFiveDown" {
		trend = FortyFiveDown
	}
	if data[0].Direction == "SingleDown" {
		trend = SingleDown
	}
	if data[0].Direction == "DoubleDown" {
		trend = DoubleDown
	}
	if data[0].Direction == "NotComputable" {
		trend = NotComputable
	}
	if data[0].Direction == "RateOutOfRange" {
		trend = RateOutOfRange
	}

	standard := "mmol/L"

	if e.bloodGlucoseStandard == "US" {
		standard = "mg/dL"
	}

	e.sgvStatusGauge.With(prometheus.Labels{"glucosetype": standard, "url": e.nightscoutAddress}).Set(sgv)
	e.trendStatusGauge.With(prometheus.Labels{"glucosetype": standard, "url": e.nightscoutAddress}).Set(float64(trend))
	e.bgdeltaStatusGauge.With(prometheus.Labels{"glucosetype": standard, "url": e.nightscoutAddress}).Set(delta)

	return nil
}

// Collect fetches the stats of a user and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
	if err := e.scrape(ch); err != nil {
		e.logger.Error("Failed to scrape nightscout data", zap.Error(err))
	}

	e.sgvStatusGauge.Collect(ch)
	e.trendStatusGauge.Collect(ch)
	e.bgdeltaStatusGauge.Collect(ch)

	return
}

func main() {

	logger, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	metricsPath := defaultTelemetryEndpoint
	listenAddress := defaultAddress
	nightscoutAddress := defaultNightscoutEndpoint
	nightscoutToken := defaultNightscoutToken
	bloodGlucoseStandard := defaultBloodGlucoseStandard

	val, ok := os.LookupEnv("TELEMETRY_ADDRESS")
	if ok {
		listenAddress = val
	} else {
		listenAddress = defaultAddress
	}

	val, ok = os.LookupEnv("TELEMETRY_ENDPOINT")
	if ok {
		metricsPath = val
	}

	val, ok = os.LookupEnv("NIGHTSCOUT_ENDPOINT")
	if ok && len(val) > 0 {
		nightscoutAddress = val
	} else {
		logger.Fatal("NIGHTSCOUT_ENDPOINT NOT SET")
	}

	val, ok = os.LookupEnv("NIGHTSCOUT_TOKEN")
	if ok {
		nightscoutToken = val
	}

	val, ok = os.LookupEnv("BLOOD_GLUCOSE_STANDARD")
	if ok {
		bloodGlucoseStandard = val
	}

	exporter := NewNightscoutCheckerExporter()
	exporter.nightscoutAddress = nightscoutAddress
	exporter.token = nightscoutToken
	exporter.bloodGlucoseStandard = bloodGlucoseStandard
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
	logger.Info("Starting Server", zap.String("Listening Address", listenAddress))
	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		logger.Fatal("Failed to ListenAndServe", zap.Error(err))
	}
}
