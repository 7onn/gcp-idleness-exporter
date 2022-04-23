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
	isMachineRunning = prometheus.NewDesc("gce_is_machine_running", "tells whether the VM is running", []string{"project", "zone", "name"}, nil)
)

type GCEIsMachineRunningCollector struct {
	logger           log.Logger
	service          *compute.Service
	project          string
	monitoredRegions []string
	mutex            sync.RWMutex
}

func init() {
	registerCollector("gce_is_machine_running", defaultEnabled, NewGCEIsMachineRunningCollector)
}

// NewGCEIsMachineRunningCollector returns a new Collector exposing gce_is_machine_running metrics
func NewGCEIsMachineRunningCollector(logger log.Logger, project string, monitoredRegions []string) (Collector, error) {
	ctx := context.Background()
	gcpClient, err := NewGCPClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create GCP Client", "err", err)
	}

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create service", "err", err)
	}

	return &GCEIsMachineRunningCollector{
		logger:           logger,
		service:          computeService,
		project:          project,
		monitoredRegions: monitoredRegions,
	}, nil
}

// Update will run each time the metrics endpoint is requested
func (e *GCEIsMachineRunningCollector) Update(ch chan<- prometheus.Metric) error {
	// Protects metrics from concurrent collects.
	e.mutex.Lock()
	defer e.mutex.Unlock()

	regionList, err := e.service.Regions.List(e.project).Do()
	if err != nil {
		level.Error(e.logger).Log("msg", fmt.Sprintf("Failure when querying %s regions", e.project), "err", err)
		regionList = nil
	}

	vms := []*compute.Instance{}

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
				regionalInstances, err := e.service.Instances.List(e.project, zone).Do()
				if err != nil {
					level.Error(e.logger).Log("msg", fmt.Sprintf("error requesting machines for project %s in zone %s", e.project, zone), "err", err)
				}
				vms = append(vms, regionalInstances.Items...)
				wgZones.Done()
			}(ch)
		}
		wgZones.Wait()
	}

	// VM usage metrics
	for _, vm := range vms {
		var isRunning float64
		if vm.Status == "RUNNING" {
			isRunning = 1.0
		} else {
			isRunning = 0
		}

		ch <- prometheus.MustNewConstMetric(
			isMachineRunning,
			prometheus.GaugeValue,
			isRunning,
			e.project,
			GetGCPZoneFromURL(e.logger, vm.Zone),
			vm.Name)
	}

	return nil
}
