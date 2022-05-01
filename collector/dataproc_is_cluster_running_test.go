package collector

import (
	"reflect"
	"testing"
)

func TestDataprocIsClusterRunningCollectorListMetrics(t *testing.T) {
	cases := []struct {
		desc     string
		expected []string
	}{
		{"should list available metrics for dataproc_is_cluster_running collector", []string{"dataproc_is_cluster_running"}},
	}

	for _, tc := range cases {
		collector := DataprocIsClusterRunningCollector{}
		if !reflect.DeepEqual(collector.ListMetrics(), tc.expected) {
			t.Errorf("expected %s got %+v", tc.expected, collector.ListMetrics())
		}
	}
}
