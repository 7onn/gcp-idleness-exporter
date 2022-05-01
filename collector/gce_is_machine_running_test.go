package collector

import (
	"reflect"
	"testing"
)

func TestGCEIsMachineRunningCollectorListMetrics(t *testing.T) {
	cases := []struct {
		desc     string
		expected []string
	}{
		{"should list available metrics for gce_is_machine_running collector", []string{"gce_is_machine_running"}},
	}

	for _, tc := range cases {
		collector := GCEIsMachineRunningCollector{}
		if !reflect.DeepEqual(collector.ListMetrics(), tc.expected) {
			t.Errorf("expected %s got %+v", tc.expected, collector.ListMetrics())
		}
	}
}
