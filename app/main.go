package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
	"gopkg.in/alecthomas/kingpin.v2"
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
		"http-timeout", "How long should gcp_exporter wait for a result from the Google API ($GCP_EXPORTER_HTTP_TIMEOUT)",
	).Envar("GCP_EXPORTER_HTTP_TIMEOUT").Default("10s").Duration()

	gcpMaxBackoffDuration = kingpin.Flag(
		"max-backoff", "Max time between each request in an exp backoff scenario ($GCP_EXPORTER_MAX_BACKOFF_DURATION)",
	).Envar("GCP_EXPORTER_MAX_BACKOFF_DURATION").Default("5s").Duration()

	gcpBackoffJitterBase = kingpin.Flag(
		"backoff-jitter", "The amount of jitter to introduce in a exp backoff scenario ($GCP_EXPORTER_BACKODFF_JITTER_BASE)",
	).Envar("GCP_EXPORTER_BACKOFF_JITTER_BASE").Default("1s").Duration()

	gcpRetryStatuses = kingpin.Flag(
		"retry-statuses", "The HTTP statuses that should trigger a retry ($GCP_EXPORTER_RETRY_STATUSES)",
	).Envar("GCP_EXPORTER_RETRY_STATUSES").Default("503").Ints()
)

func main() {
	var (
		listenAddress = kingpin.Flag("listen-address", "Address to listen on for web interface and telemetry.").Envar("LISTEN_ADDRESS").Default(":5000").String()
		metricsPath   = kingpin.Flag("metrics-path", "Path under which to expose metrics.").Default("/metrics").Envar("METRICS_PATH").String()
		probePath     = kingpin.Flag("probe-path", "Path under which to respond readiness probe").Envar("HEALTHCHECK_PATH").Default("/health").String()
	)

	kingpin.Version(version.Print("gcp-idle-resources-metrics"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Info().Msgf("Starting gcp-idle-resources-metrics %s", version.Info())
	log.Info().Msgf("Build context %s", version.BuildContext())

	// Detect Project ID
	if *gcpProjectID == "" {
		credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

		if credentialsFile != "" {
			c, err := ioutil.ReadFile(credentialsFile)
			if err != nil {
				log.Fatal().Msgf("Unable to read %s: %v", credentialsFile, err)
			}

			projectId := gjson.GetBytes(c, "project_id")
			if projectId.String() == "" {
				log.Fatal().Msgf("Could not retrieve Project ID from %s", credentialsFile)
			}

			*gcpProjectID = projectId.String()
		} else {
			// Get project id from metadata
			client := metadata.NewClient(&http.Client{})
			project_id, err := client.ProjectID()
			if err != nil {
				log.Fatal().Msgf("error getting GCP project ID from metadata: %+v", err)
			}

			*gcpProjectID = project_id
		}
	}

	if *gcpProjectID == "" {
		log.Fatal().Msg("GCP Project ID cannot be empty")
	}

	monitoredRegions = strings.Split(*gcpRegions, ",")
	log.Info().Msgf("Starting exporter for project %s at %v", *gcpProjectID, monitoredRegions)
	computeEngineExporter, err := NewComputeEngineExporter(*gcpProjectID)
	if err != nil {
		log.Fatal().Err(err)
	}

	prometheus.MustRegister(version.NewCollector("gcp_idle_resources_metrics"))
	prometheus.MustRegister(computeEngineExporter)

	log.Info().Msgf("Google Project: %s", *gcpProjectID)
	log.Info().Msgf("Listening on %s", *listenAddress)

	http.Handle(*metricsPath, promhttp.Handler())

	http.HandleFunc(*probePath, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("aaa aaa aaa aaa staying alive staying alive"))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<head>
					<title>GCP idle resources metrics exporter</title>
				</head>
				<body>
					<h1>GCP idle resources metrics exporter Exporter</h1>
					<p>
						<a href='` + *metricsPath + `'>Metrics</a>
					</p>
				</body>
			</html>`))
	})

	log.Fatal().Err(http.ListenAndServe(*listenAddress, nil))
}
