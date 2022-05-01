package collector

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

var (
	metricDiskSnapshotAge    = prometheus.NewDesc("gce_disk_snapshot_age_days", "tells how many days the snapshot has", []string{"project", "disk", "snapshot"}, nil)
	metricDiskSnapshotAmount = prometheus.NewDesc("gce_disk_snapshot_amount", "tells how many snapshots the Disk has", []string{"project", "disk"}, nil)
)

func init() {
	registerCollector("gce_disk_snapshot", defaultEnabled, NewGCEDiskSnapshotCollector)
}

func (e *GCEDiskSnapshotCollector) ListMetrics() []string {
	return []string{"gce_disk_snapshot_age_days", "gce_disk_snapshot_amount"}
}

type GCEDiskSnapshotCollector struct {
	logger           log.Logger
	service          *compute.Service
	project          string
	monitoredRegions []string
	metrics          []string
	mutex            sync.RWMutex
}

func NewGCEDiskSnapshotCollector(logger log.Logger, project string, monitoredRegions []string) (Collector, error) {
	ctx := context.Background()
	gcpClient, err := NewGCPClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create GCP Client", "err", err)
	}

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create service", "err", err)
	}

	return &GCEDiskSnapshotCollector{
		logger:           logger,
		service:          computeService,
		project:          project,
		monitoredRegions: monitoredRegions,
		metrics:          []string{"gce_disk_snapshot_amount", "gce_disk_snapshot_age_days"},
	}, nil
}

func (e *GCEDiskSnapshotCollector) Update(ch chan<- prometheus.Metric) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	snapshots, err := e.service.Snapshots.List(e.project).Do()
	if err != nil {
		level.Error(e.logger).Log("msg", fmt.Sprintf("error requesting disk snapshots for project %s", e.project), "err", err)
		return err
	}

	diskSnapshotAmount := map[string]int{}
	reportedSnapshots := []string{}
	for _, snapshot := range snapshots.Items {
		if lo.Contains(reportedSnapshots, snapshot.Name) {
			continue
		}
		reportedSnapshots = append(reportedSnapshots, snapshot.Name)
		diskSnapshotAmount[GetDiskNameFromURL(e.logger, snapshot.SourceDisk)]++

		snapshotCreationTimestamp, err := time.Parse(time.RFC3339, snapshot.CreationTimestamp)
		if err != nil {
			level.Error(e.logger).Log("msg", fmt.Sprintf("error parsing %s snapshot's CreationTimestamp for project %s", snapshot.Name, e.project), "err", err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			metricDiskSnapshotAge,
			prometheus.GaugeValue,
			math.Floor(time.Since(snapshotCreationTimestamp).Hours()/24),
			e.project,
			GetDiskNameFromURL(e.logger, snapshot.SourceDisk),
			snapshot.Name)
	}

	for disk, amount := range diskSnapshotAmount {
		ch <- prometheus.MustNewConstMetric(
			metricDiskSnapshotAmount,
			prometheus.GaugeValue,
			float64(amount),
			e.project,
			disk)
	}

	return nil
}
