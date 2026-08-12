package runtime_test

import (
	"strings"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

func TestRMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "R Plumber project with renv.lock",
			path:     "../testdata/r-plumber",
			expected: true,
		},
		{
			name:     "R Shiny project with app.R",
			path:     "../testdata/r-shiny",
			expected: true,
		},
		{
			name:     "Not an R project",
			path:     "../testdata/deno",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &runtime.R{Log: logger}
			if r.Match(test.path) != test.expected {
				t.Errorf("expected %v, got %v", test.expected, r.Match(test.path))
			}
		})
	}
}

func TestRGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name: "R Plumber API",
			path: "../testdata/r-plumber",
			expected: []string{
				`ARG R_VERSION=4.4.1`,
				`rocker/r-ver`,
				`plumber`,
				`PORT=8080`,
			},
		},
		{
			name: "R Shiny app",
			path: "../testdata/r-shiny",
			expected: []string{
				`ARG R_VERSION=4.4.1`,
				`rocker/shiny`,
				`shiny`,
				`PORT=3838`,
				`shiny-server`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &runtime.R{Log: logger}
			dockerfile, err := r.GenerateDockerfile(test.path)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			for _, expected := range test.expected {
				if !strings.Contains(string(dockerfile), expected) {
					t.Errorf("expected %q not found in:\n%s", expected, string(dockerfile))
				}
			}
		})
	}
}
