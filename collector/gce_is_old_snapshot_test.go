package collector

import (
	"os"
	"testing"

	"github.com/go-kit/log"
)

func TestGetDiskNameFromURL(t *testing.T) {
	cases := []struct {
		desc     string
		input    string
		expected string
	}{
		{
			"Should return empty",
			"https://someurl/",
			"",
		},
		{
			"Should return xxx",
			"https://www.googleapis.com/compute/v1/projects/project/zones/us-central1-c/disks/disk",
			"disk",
		},
	}

	for _, tc := range cases {
		r := GetDiskNameFromURL(log.NewJSONLogger(os.Stdout), tc.input)
		if r != tc.expected {
			t.Errorf("%s want %s got %s instead", tc.desc, tc.expected, r)
		}
	}
}
