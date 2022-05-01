package collector

import (
	"reflect"
	"testing"
)

func TestGCEIsDiskAttachedCollectorListMetrics(t *testing.T) {
	cases := []struct {
		desc     string
		expected []string
	}{
		{"should list available metrics for gce_is_disk_attached collector", []string{"gce_is_disk_attached"}},
	}

	for _, tc := range cases {
		collector := GCEIsDiskAttachedCollector{}
		if !reflect.DeepEqual(collector.ListMetrics(), tc.expected) {
			t.Errorf("expected %s got %+v", tc.expected, collector.ListMetrics())
		}
	}
}
