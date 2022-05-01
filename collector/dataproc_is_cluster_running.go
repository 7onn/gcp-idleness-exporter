package collector

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/dataproc/v1"
	"google.golang.org/api/option"
)

var (
	isDataprocClusterRunning = prometheus.NewDesc("dataproc_is_cluster_running", "tells whether the Dataproc cluster is running", []string{"project", "region", "zone", "name"}, nil)
)

type DataprocIsClusterRunningCollector struct {
	logger           log.Logger
	service          *dataproc.Service
	project          string
	monitoredRegions []string
	mutex            sync.RWMutex
}

func init() {
	registerCollector("dataproc_is_cluster_running", defaultEnabled, NewDataprocIsClusterRunningCollector)
}

func (e *DataprocIsClusterRunningCollector) ListMetrics() []string {
	return []string{"dataproc_is_cluster_running"}
}

func NewDataprocIsClusterRunningCollector(logger log.Logger, project string, monitoredRegions []string) (Collector, error) {
	ctx := context.Background()
	gcpClient, err := NewGCPClient(ctx, dataproc.CloudPlatformScope)
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create GCP Client", "err", err)
	}

	dataprocService, err := dataproc.NewService(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create service", "err", err)
	}

	return &DataprocIsClusterRunningCollector{
		logger:           logger,
		service:          dataprocService,
		project:          project,
		monitoredRegions: monitoredRegions,
	}, nil
}

func (e *DataprocIsClusterRunningCollector) Update(ch chan<- prometheus.Metric) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	var wgRegions sync.WaitGroup
	wgRegions.Add(len(e.monitoredRegions))

	for _, region := range e.monitoredRegions {
		go func(ch chan<- prometheus.Metric, region string) {
			regionalDataprocClusters, err := e.service.Projects.Regions.Clusters.List(e.project, region).Do()
			if err != nil {
				level.Error(e.logger).Log("msg", fmt.Sprintf("Failure when querying Dataproc Clusters in %s at %s", e.project, region), "err", err)
				wgRegions.Done()
				return
			}

			for _, cluster := range regionalDataprocClusters.Clusters {
				zone := GetGCPZoneFromURL(e.logger, cluster.Config.GceClusterConfig.ZoneUri)
				if zone == "" {
					// In case of GKE Dataproc clusters which have no Zone info
					// available through its current API google.golang.org/api/dataproc/v1
					zone = region
				}

				if cluster.Status.State == "RUNNING" {
					ch <- prometheus.MustNewConstMetric(
						isDataprocClusterRunning,
						prometheus.GaugeValue,
						1.,
						e.project,
						region,
						zone,
						cluster.ClusterName)
				} else {
					ch <- prometheus.MustNewConstMetric(
						isDataprocClusterRunning,
						prometheus.GaugeValue,
						0.,
						e.project,
						region,
						zone,
						cluster.ClusterName)
				}
			}

			wgRegions.Done()
		}(ch, region)
	}

	wgRegions.Wait()
	return nil
}
