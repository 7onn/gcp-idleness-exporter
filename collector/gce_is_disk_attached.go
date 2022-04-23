package collector

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

var (
	isDiskAttached = prometheus.NewDesc("gce_is_disk_attached", "tells whether the Disk is attached to some machine", []string{"project", "zone", "name"}, nil)
)

type GCEIsDiskAttachedCollector struct {
	logger           log.Logger
	service          *compute.Service
	project          string
	monitoredRegions []string
	mutex            sync.RWMutex
}

func init() {
	registerCollector("gce_is_disk_attached", defaultEnabled, NewIsDiskAttachedCollector)
}

// NewIsDiskAttachedCollector returns a new Collector exposing gce_is_disk_attached metrics
func NewIsDiskAttachedCollector(logger log.Logger, project string, monitoredRegions []string) (Collector, error) {
	ctx := context.Background()
	gcpClient, err := NewGCPClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create GCP Client", "err", err)
	}

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create service", "err", err)
	}

	return &GCEIsDiskAttachedCollector{
		logger:           logger,
		service:          computeService,
		project:          project,
		monitoredRegions: monitoredRegions,
	}, nil
}

// Update will run each time the metrics endpoint is requested
func (e *GCEIsDiskAttachedCollector) Update(ch chan<- prometheus.Metric) error {
	// To protect metrics from concurrent collects.
	e.mutex.Lock()
	defer e.mutex.Unlock()

	regionList, err := e.service.Regions.List(e.project).Do()
	if err != nil {
		level.Error(e.logger).Log("msg", fmt.Sprintf("Failure when querying %s regions", e.project), "err", err)
		regionList = nil
	}

	disks := []*compute.Disk{}

	for _, r := range regionList.Items {
		if !lo.Contains(e.monitoredRegions, r.Name) {
			continue
		}

		var wgZones sync.WaitGroup
		wgZones.Add(len(r.Zones))

		for _, z := range r.Zones {
			zone := GetGCPZoneFromURL(e.logger, z)
			ch := make(chan struct{})
			go func(ch chan struct{}) {
				regionalDisks, err := e.service.Disks.List(e.project, zone).Do()
				if err != nil {
					level.Error(e.logger).Log("msg", fmt.Sprintf("error requesting machine disks for project %s in zone %s", e.project, zone), "err", err)
				}
				disks = append(disks, regionalDisks.Items...)
				wgZones.Done()
			}(ch)
		}
		wgZones.Wait()
	}

	// Disk usage metrics
	for _, disk := range disks {
		isAttached := float64(len(disk.Users))
		ch <- prometheus.MustNewConstMetric(
			isDiskAttached,
			prometheus.GaugeValue,
			isAttached,
			e.project,
			GetGCPZoneFromURL(e.logger, disk.Zone),
			disk.Name)
	}

	return nil
}
