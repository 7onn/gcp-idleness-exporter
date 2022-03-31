package main

import (
	"testing"
)

func TestGetGCPZoneFromURL(t *testing.T) {
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
			"Should return us-east1-a",
			"https://www.googleapis.com/compute/v1/projects/project-id/zones/asia-east1-a",
			"asia-east1-a",
		},
	}

	for _, tc := range cases {
		r := GetGCPZoneFromURL(tc.input)
		if r != tc.expected {
			t.Errorf("%s want %s got %s instead", tc.desc, tc.expected, r)
		}
	}
}
