package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"strings"

	stdlog "log"

	"github.com/go-kit/log"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"cloud.google.com/go/compute/metadata"
	"github.com/7onn/gcp-idleness-exporter/collector"
	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/tidwall/gjson"
)

var (
	gcpProjectID = kingpin.Flag(
		"project-id", "GCP Project ID to monitor. ($GCP_PROJECT_ID)",
	).Envar("GCP_PROJECT_ID").String()

	gcpRegions = kingpin.Flag(
		"regions", "Comma-separated GCP regions to monitor. e.g: asia-east1,southamerica-east1,us-east1 ($GCP_REGIONS)",
	).Envar("GCP_REGIONS").String()

	monitoredRegions []string

	gcpMaxRetries = kingpin.Flag(
		"max-retries", "Max number of retries that should be attempted on 503 errors from gcp. ($GCP_EXPORTER_MAX_RETRIES)\n",
	).Envar("GCP_EXPORTER_MAX_RETRIES").Default("0").Int()

	gcpHttpTimeout = kingpin.Flag(
		"http-timeout", "How long in seconds should gcp_exporter wait for a result from the Google API ($GCP_EXPORTER_HTTP_TIMEOUT)",
	).Envar("GCP_EXPORTER_HTTP_TIMEOUT").Default("10s").Duration()

	gcpMaxBackoffDuration = kingpin.Flag(
		"max-backoff", "Max time in seconds between each request in an exp backoff scenario ($GCP_EXPORTER_MAX_BACKOFF_DURATION)",
	).Envar("GCP_EXPORTER_MAX_BACKOFF_DURATION").Default("5s").Duration()

	gcpBackoffJitterBase = kingpin.Flag(
		"backoff-jitter", "The amount in seconds of jitter to introduce in a exp backoff scenario ($GCP_EXPORTER_BACKODFF_JITTER_BASE)",
	).Envar("GCP_EXPORTER_BACKOFF_JITTER_BASE").Default("1s").Duration()

	gcpRetryStatuses = kingpin.Flag(
		"retry-statuses", "The HTTP statuses that should trigger a retry ($GCP_EXPORTER_RETRY_STATUSES)",
	).Envar("GCP_EXPORTER_RETRY_STATUSES").Default("503").Ints()

	disableDefaultCollectors = kingpin.Flag(
		"collector.disable-defaults",
		"Set all collectors to disabled by default.",
	).Default("false").Bool()
)

type MetricsHandler struct {
	exporterMetricsRegistry *prometheus.Registry
	logger                  log.Logger
}

func newMetricsHandler(logger log.Logger) *MetricsHandler {
	h := &MetricsHandler{
		exporterMetricsRegistry: prometheus.NewRegistry(),
		logger:                  logger,
	}
	h.exporterMetricsRegistry.MustRegister(
		promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{}),
		promcollectors.NewGoCollector(),
	)

	return h
}

// ServeHTTP implements http.Handler.
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	gcpCollector, err := collector.NewGCPCollector(ctx, h.logger, *gcpProjectID, monitoredRegions)
	if err != nil {
		level.Error(h.logger).Log("msg", "couldn't create collector", "err", err)
	}

	for n, c := range gcpCollector.Collectors {
		level.Info(h.logger).Log("collector", n, "metrics", fmt.Sprintf("%+v", c.ListMetrics()))
	}

	pr := prometheus.NewRegistry()
	pr.MustRegister(version.NewCollector("gcp_idleness_exporter"))
	if err = pr.Register(gcpCollector); err != nil {
		level.Error(h.logger).Log("msg", "couldn't register gcp_idleness_exporter collector", "err", err)
	}
	handler := promhttp.HandlerFor(
		prometheus.Gatherers{h.exporterMetricsRegistry, pr},
		promhttp.HandlerOpts{
			ErrorLog:      stdlog.New(log.NewStdlibAdapter(level.Error(h.logger)), "", 0),
			ErrorHandling: promhttp.ContinueOnError,
			Registry:      h.exporterMetricsRegistry,
		},
	)

	handler = promhttp.InstrumentMetricHandler(h.exporterMetricsRegistry, handler)
	handler.ServeHTTP(w, r)
}

func main() {
	var (
		listenAddress = kingpin.Flag("listen-address", "Address to listen on for web interface and telemetry.").Envar("LISTEN_ADDRESS").Default(":5000").String()
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("gcp-idleness-exporter"))
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	collector.GCPHttpTimeout = *gcpHttpTimeout
	collector.GCPMaxRetries = *gcpMaxRetries
	collector.GCPRetryStatuses = *gcpRetryStatuses
	collector.GCPBackoffJitterBase = *gcpBackoffJitterBase
	collector.GCPMaxBackoffDuration = *gcpMaxBackoffDuration

	logger := promlog.New(promlogConfig)

	if *disableDefaultCollectors {
		collector.DisableDefaultCollectors()
	}
	level.Info(logger).Log("msg", "Starting gcp-idleness-exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())
	if user, err := user.Current(); err == nil && user.Uid == "0" {
		level.Warn(logger).Log("msg", "gcp-idleness-exporter is running as root user. This exporter is designed to run as unpriviledged user, root is not required.")
	}

	// Detect Project ID
	if *gcpProjectID == "" {
		credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if credentialsFile != "" {
			c, err := ioutil.ReadFile(credentialsFile)
			if err != nil {
				level.Error(logger).Log("msg", fmt.Sprintf("Unable to read %s", credentialsFile), "err", err)
			}

			projectId := gjson.GetBytes(c, "project_id")
			if projectId.String() == "" {
				level.Error(logger).Log("msg", fmt.Sprintf("Could not retrieve Project ID from %s", credentialsFile))
			}

			*gcpProjectID = projectId.String()
		} else {
			// Get project id from metadata
			client := metadata.NewClient(&http.Client{})
			project_id, err := client.ProjectID()
			if err != nil {
				level.Error(logger).Log("msg", fmt.Sprintf("error getting GCP project ID from metadata: %+v", err))
			}

			*gcpProjectID = project_id
		}
	}

	if *gcpProjectID == "" {
		level.Error(logger).Log("msg", "GCP Project ID cannot be empty")
	}

	monitoredRegions = strings.Split(*gcpRegions, ",")
	level.Info(logger).Log("msg", fmt.Sprintf("Starting exporter for project %s at %v", *gcpProjectID, monitoredRegions))

	level.Info(logger).Log("msg", fmt.Sprintf("Listening on %s", *listenAddress))

	http.Handle("/metrics", newMetricsHandler(logger))

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("aaa aaa aaa aaa staying alive staying alive"))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<head>
					<title>gcp-idleness-exporter</title>
				</head>
				<body>
					<h1>GCP idleness exporter</h1>
					<p>
						<a href='/metrics'>Metrics</a>
					</p>
				</body>
			</html>`))
	})

	level.Error(logger).Log("err", http.ListenAndServe(*listenAddress, nil))
}
