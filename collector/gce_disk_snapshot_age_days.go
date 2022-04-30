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
	diskSnapshotAge = prometheus.NewDesc("gce_disk_snapshot_age_days", "tells how many days the snapshot has", []string{"project", "disk", "snapshot"}, nil)
)

type GCEDiskSnapshotAgeCollector struct {
	logger  log.Logger
	service *compute.Service
	project string
	mutex   sync.RWMutex
}

func init() {
	registerCollector("gce_disk_snapshot_age_days", defaultEnabled, NewGCEDiskSnapshotAgeCollector)
}

func NewGCEDiskSnapshotAgeCollector(logger log.Logger, project string, monitoredRegions []string) (Collector, error) {
	ctx := context.Background()
	gcpClient, err := NewGCPClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create GCP Client", "err", err)
	}

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create service", "err", err)
	}

	return &GCEDiskSnapshotAgeCollector{
		logger:  logger,
		service: computeService,
		project: project,
	}, nil
}

func (e *GCEDiskSnapshotAgeCollector) Update(ch chan<- prometheus.Metric) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	snapshots, err := e.service.Snapshots.List(e.project).Do()
	if err != nil {
		level.Error(e.logger).Log("msg", fmt.Sprintf("error requesting disk snapshots for project %s", e.project), "err", err)
		return err
	}

	reportedSnapshots := []string{}
	for _, snapshot := range snapshots.Items {
		if lo.Contains(reportedSnapshots, snapshot.Name) {
			continue
		}
		reportedSnapshots = append(reportedSnapshots, snapshot.Name)

		snapshotCreationTimestamp, err := time.Parse(time.RFC3339, snapshot.CreationTimestamp)
		if err != nil {
			level.Error(e.logger).Log("msg", fmt.Sprintf("error parsing %s snapshot's CreationTimestamp for project %s", snapshot.Name, e.project), "err", err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			diskSnapshotAge,
			prometheus.GaugeValue,
			math.Floor(time.Since(snapshotCreationTimestamp).Hours()/24),
			e.project,
			GetDiskNameFromURL(e.logger, snapshot.SourceDisk),
			snapshot.Name)
	}

	return nil
}
