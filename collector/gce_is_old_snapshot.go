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
	isOldSnapshot = prometheus.NewDesc("gce_is_old_snapshot", "tells whether the Disk has unnecessary old snapshots", []string{"project", "disk", "snapshot"}, nil)
)

type GCEIsOldSnapshotCollector struct {
	logger  log.Logger
	service *compute.Service
	project string
	mutex   sync.RWMutex
}

func init() {
	registerCollector("gce_is_old_snapshot", defaultEnabled, NewGCEIsOldSnapshotCollector)
}

func NewGCEIsOldSnapshotCollector(logger log.Logger, project string, monitoredRegions []string) (Collector, error) {
	ctx := context.Background()
	gcpClient, err := NewGCPClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create GCP Client", "err", err)
	}

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create service", "err", err)
	}

	return &GCEIsOldSnapshotCollector{
		logger:  logger,
		service: computeService,
		project: project,
	}, nil
}

func (e *GCEIsOldSnapshotCollector) Update(ch chan<- prometheus.Metric) error {
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

	reportedSnapshots := []string{}
	for k, snapshot := range snapshots.Items {
		if lo.Contains(reportedSnapshots, snapshot.Name) {
			continue
		}

		if k < len(snapshots.Items)-1 {
			next := snapshots.Items[k+1]
			if snapshot.SourceDiskId == next.SourceDiskId {
				if snapshot.CreationTimestamp < next.CreationTimestamp {
					ch <- prometheus.MustNewConstMetric(
						isOldSnapshot,
						prometheus.GaugeValue,
						1,
						e.project,
						GetDiskNameFromURL(e.logger, snapshot.SourceDisk),
						snapshot.Name)
					reportedSnapshots = append(reportedSnapshots, snapshot.Name)

				} else {
					ch <- prometheus.MustNewConstMetric(
						isOldSnapshot,
						prometheus.GaugeValue,
						1,
						e.project,
						GetDiskNameFromURL(e.logger, next.SourceDisk),
						next.Name)
					reportedSnapshots = append(reportedSnapshots, next.Name)
				}
			}
		}
	}

	return nil
}
