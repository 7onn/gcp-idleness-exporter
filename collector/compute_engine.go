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
	isMachineRunning = prometheus.NewDesc("gce_machine_running", "tells whether the VM is running", []string{"project", "zone", "name"}, nil)
	isDiskAttached   = prometheus.NewDesc("gce_disk_attached", "tells whether the Disk is attached to some machine", []string{"project", "zone", "name"}, nil)
)

type ComputeEngineCollector struct {
	logger           log.Logger
	service          *compute.Service
	project          string
	monitoredRegions []string
	mutex            sync.RWMutex
}

func init() {
	registerCollector("compute_engine", defaultEnabled, NewComputeEngineCollector)
}

// NewComputeEngineCollector returns a new Collector exposing os-release information.
func NewComputeEngineCollector(logger log.Logger, project string, monitoredRegions []string) (Collector, error) {
	ctx := context.Background()
	gcpClient, err := NewGCPClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create GCP Client", "err", err)
	}

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		level.Error(logger).Log("msg", "Unable to create service", "err", err)
	}

	return &ComputeEngineCollector{
		logger:           logger,
		service:          computeService,
		project:          project,
		monitoredRegions: monitoredRegions,
	}, nil
}

// GetGCEResources connects to the Google API to retreive the list of Compute Engine Instances and their disks.
func (e *ComputeEngineCollector) GetGCEResources() (machines []*compute.Instance, disks []*compute.Disk) {

	regionList, err := e.service.Regions.List(e.project).Do()
	if err != nil {
		level.Error(e.logger).Log("msg", fmt.Sprintf("Failure when querying %s regions", e.project), "err", err)
		regionList = nil
	}

	instances := []*compute.Instance{}
	diskDevices := []*compute.Disk{}

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
				instances = append(instances, regionalInstances.Items...)

				regionalDisks, err := e.service.Disks.List(e.project, zone).Do()
				if err != nil {
					level.Error(e.logger).Log("msg", fmt.Sprintf("error requesting machine disks for project %s in zone %s", e.project, zone), "err", err)
				}
				diskDevices = append(diskDevices, regionalDisks.Items...)

				wgZones.Done()
			}(ch)
		}
		wgZones.Wait()
	}

	return instances, diskDevices
}

// Update will run each time the metrics endpoint is requested
func (e *ComputeEngineCollector) Update(ch chan<- prometheus.Metric) error {
	// To protect metrics from concurrent collects.
	e.mutex.Lock()
	defer e.mutex.Unlock()

	vms, disks := e.GetGCEResources()

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
