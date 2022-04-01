package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/PuerkitoBio/rehttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

var (
	isMachineRunning = prometheus.NewDesc("gce_machine_running", "tells whether the VM is running", []string{"project", "zone", "name"}, nil)
	isDiskAttached   = prometheus.NewDesc("gce_disk_attached", "tells whether the Disk is attached to some machine", []string{"project", "zone", "name"}, nil)
)

type ComputeEngineExporter struct {
	service *compute.Service
	project string
	mutex   sync.RWMutex
}

// GetGCEResources connects to the Google API to retreive the list of Compute Engine Instances and their disks.
func (e *ComputeEngineExporter) GetGCEResources() (machines []*compute.Instance, disks []*compute.Disk) {

	regionList, err := e.service.Regions.List(e.project).Do()
	if err != nil {
		log.Error().Msgf("Failure when querying %s regions: %v", e.project, err)
		regionList = nil
	}

	instances := []*compute.Instance{}
	diskDevices := []*compute.Disk{}

	for _, r := range regionList.Items {

		if !lo.Contains(monitoredRegions, r.Name) {
			continue
		}

		var wgZones sync.WaitGroup
		wgZones.Add(len(r.Zones))

		for _, z := range r.Zones {
			zone := GetGCPZoneFromURL(z)

			ch := make(chan struct{})
			go func(ch chan struct{}) {
				regionalInstances, err := e.service.Instances.List(e.project, zone).Do()
				if err != nil {
					log.Error().Msgf("error requesting machines for project %s in zone %s\n %+v", e.project, zone, err)
				}
				instances = append(instances, regionalInstances.Items...)

				regionalDisks, err := e.service.Disks.List(e.project, zone).Do()
				if err != nil {
					log.Error().Msgf("error requesting machines for project %s in zone %s\n %+v", e.project, zone, err)
				}
				diskDevices = append(diskDevices, regionalDisks.Items...)

				wgZones.Done()
			}(ch)
		}
		wgZones.Wait()
	}

	return instances, diskDevices
}

// Describe is implemented with DescribeByCollect. That's possible because the
// Collect method will always return the same metrics with the same descriptors.
func (e *ComputeEngineExporter) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(e, ch)
}

// Collect will run each time the metrics endpoint is requested
func (e *ComputeEngineExporter) Collect(ch chan<- prometheus.Metric) {
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
			GetGCPZoneFromURL(vm.Zone),
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
			GetGCPZoneFromURL(disk.Zone),
			disk.Name)
	}
}

// NewComputeEngineExporter returns an initialised ComputeEngineExporter.
func NewComputeEngineExporter(project string) (*ComputeEngineExporter, error) {
	ctx := context.Background()

	googleClient, err := google.DefaultClient(ctx, compute.ComputeReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("error creating Google client: %+v", err)
	}

	googleClient.Timeout = *gcpHttpTimeout
	googleClient.Transport = rehttp.NewTransport(
		googleClient.Transport,
		rehttp.RetryAll(
			rehttp.RetryMaxRetries(*gcpMaxRetries),
			rehttp.RetryStatuses(*gcpRetryStatuses...)), // Cloud support suggests retrying on 503 errors
		rehttp.ExpJitterDelay(*gcpBackoffJitterBase, *gcpMaxBackoffDuration), // Set timeout to <10s as that is Prometheus' default timeout
	)

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(googleClient))
	if err != nil {
		log.Fatal().Msgf("Unable to create service: %v", err)
	}

	return &ComputeEngineExporter{
		service: computeService,
		project: project,
	}, nil
}
