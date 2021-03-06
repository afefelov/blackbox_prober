package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"

	"./pingers"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// set at build time
	Version = "0.0.0.dev"

	listenAddress = flag.String("web.listen-address", ":9110", "Address to listen on for web interface and telemetry.")
	metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")

	errURLNotAbsolute = errors.New("URL not absolute")
	errNoPinger       = errors.New("No pinger for schema")
)

type pingCollector struct {
	targets targets
	metrics pingers.Metrics
}

// NewPingCollector returns a new pingCollector
func NewPingCollector(targets targets) *pingCollector {
	return &pingCollector{
		targets: targets,
		metrics: pingers.Metrics{
			Up: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: pingers.Namespace,
				Name:      "up",
				Help:      "1 if url is reachable, 0 if not",
			}, []string{"url"}),
			Latency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: pingers.Namespace,
				Name:      "latency_seconds",
				Help:      "Latency of request for url",
			}, []string{"url"}),
			Size: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: pingers.Namespace,
				Name:      "size_bytes",
				Help:      "Size of request for url",
			}, []string{"url"}),
			Code: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: pingers.Namespace,
				Name:      "response_code",
				Help:      "Response code for url",
			}, []string{"url"}),
		},
	}
}

// Collect implements prometheus.Collector.
func (c pingCollector) Collect(ch chan<- prometheus.Metric) {
	for _, target := range c.targets {
		log.Printf("collect %s", target)
		if err := pingers.Ping(target, c.metrics); err != nil {
			panic(err)
		}
	}
	c.metrics.Up.Collect(ch)
	c.metrics.Latency.Collect(ch)
	c.metrics.Size.Collect(ch)
	c.metrics.Code.Collect(ch)
}

// Describe implements prometheus.Collector.
func (c pingCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Up.Describe(ch)
	c.metrics.Latency.Describe(ch)
	c.metrics.Size.Describe(ch)
	c.metrics.Code.Describe(ch)
}

type targets []*url.URL

func (t *targets) String() string { return "" }
func (t *targets) Set(str string) error {
	url, err := url.Parse(str)
	if err != nil {
		return err
	}
	if !url.IsAbs() {
		return errURLNotAbsolute
	}
	if !pingers.CanHandle(url) {
		return errNoPinger
	}
	*t = append(*t, url)
	return nil
}
func main() {
	targets := targets{}
	flag.Var(&targets, "u", "URL to provide metrics for, can be repeated")
	flag.Parse()
	if len(targets) == 0 {
		log.Printf("Please provide urls to ping (-u)")
		flag.PrintDefaults()
		os.Exit(1)
	}
	log.Printf("blackbox_prober v%s providing metrics for %v on %s%s", Version, targets, *listenAddress, *metricsPath)
	prometheus.MustRegister(NewPingCollector(targets))
	http.Handle(*metricsPath, prometheus.Handler())
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
