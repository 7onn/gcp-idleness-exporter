package collector

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

var (
	diskSnapshotAmount = prometheus.NewDesc("gce_disk_snapshot_amount", "tells how many snapshots the Disk has", []string{"project", "disk"}, nil)
)

type GCEDiskSnapshotAmountCollector struct {
	logger           log.Logger
	service          *compute.Service
	project          string
	monitoredRegions []string
	mutex            sync.RWMutex
}

func init() {
	registerCollector("gce_disk_snapshot_amount", defaultEnabled, NewGCEDiskSnapshotAmountCollector)
}

func NewGCEDiskSnapshotAmountCollector(logger log.Logger, project string, monitoredRegions []string) (Collector, error) {
	ctx := context.Background()
	gcpClient, err := NewGCPClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create GCP Client", "err", err)
	}

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create service", "err", err)
	}

	return &GCEDiskSnapshotAmountCollector{
		logger:           logger,
		service:          computeService,
		project:          project,
		monitoredRegions: monitoredRegions,
	}, nil
}

func (e *GCEDiskSnapshotAmountCollector) Update(ch chan<- prometheus.Metric) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	snapshots, err := e.service.Snapshots.List(e.project).Do()
	if err != nil {
		level.Error(e.logger).Log("msg", fmt.Sprintf("error requesting disk snapshots for project %s", e.project), "err", err)
		return err
	}

	sort.Slice(snapshots.Items, func(i, j int) bool {
		return snapshots.Items[i].SourceDiskId < snapshots.Items[j].SourceDiskId
	})

	disks := map[string]int{}

	reportedSnapshots := []string{}
	for _, snapshot := range snapshots.Items {
		if lo.Contains(reportedSnapshots, snapshot.Name) {
			continue
		}
		reportedSnapshots = append(reportedSnapshots, snapshot.Name)
		disks[GetDiskNameFromURL(e.logger, snapshot.SourceDisk)]++
	}

	for disk, amount := range disks {
		ch <- prometheus.MustNewConstMetric(
			diskSnapshotAmount,
			prometheus.GaugeValue,
			float64(amount),
			e.project,
			disk)
	}

	return nil
}
