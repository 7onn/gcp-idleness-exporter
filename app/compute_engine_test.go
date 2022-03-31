package main

import (
	"testing"
)

func TestNewComputeEngineExporter(t *testing.T) {
	cases := []struct {
		desc  string
		input string
	}{
		{
			"Should return a new ComputeEngineExporter",
			"project-id",
		},
	}

	for _, tc := range cases {
		_, err := NewComputeEngineExporter(tc.input)
		if err != nil {
			t.Errorf("%s", tc.desc)
		}
	}
}
