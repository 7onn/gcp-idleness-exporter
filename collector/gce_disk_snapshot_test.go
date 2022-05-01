package collector

import (
	"reflect"
	"testing"
)

func TestGCEDiskSnapshotCollectorListMetrics(t *testing.T) {
	cases := []struct {
		desc     string
		expected []string
	}{
		{"should list available metrics for gce_disk_snapshot collector", []string{"gce_disk_snapshot_age_days", "gce_disk_snapshot_amount"}},
	}

	for _, tc := range cases {
		collector := GCEDiskSnapshotCollector{}
		if !reflect.DeepEqual(collector.ListMetrics(), tc.expected) {
			t.Errorf("expected %s got %+v", tc.expected, collector.ListMetrics())
		}
	}
}
