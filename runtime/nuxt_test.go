package runtime_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

func TestNuxtMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Nuxt SSR project",
			path:     "../testdata/nuxt",
			expected: true,
		},
		{
			name:     "Nuxt static project",
			path:     "../testdata/nuxt-static",
			expected: true,
		},
		{
			name:     "Not a Nuxt project",
			path:     "../testdata/deno",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nuxt := &runtime.Nuxt{Log: logger}
			if nuxt.Match(test.path) != test.expected {
				t.Errorf("expected %v, got %v", test.expected, nuxt.Match(test.path))
			}
		})
	}
}

func TestNuxtGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     map[string]string
		expected []any
	}{
		{
			name: "Nuxt SSR project (Nitro server)",
			path: "../testdata/nuxt",
			expected: []any{
				`ARG NODE_VERSION=lts`,
				`server/index.mjs`,
				`NUXT_TELEMETRY_DISABLED=1`,
				`.output`,
			},
		},
		{
			name: "Nuxt static/SSG project",
			path: "../testdata/nuxt-static",
			expected: []any{
				`ARG NODE_VERSION=lts`,
				`joseluisq/static-web-server`,
				`.output/public`,
				`npm run generate`,
			},
		},
		{
			name: "Nuxt project with install mounts",
			path: "../testdata/nuxt",
			data: map[string]string{"InstallMounts": `--mount=type=secret,id=_env,target=/app/.env \
    `},
			expected: []any{regexp.MustCompile(`--mount=type=secret,id=_env,target=/app/.env \\$`)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nuxt := &runtime.Nuxt{Log: logger}
			var dockerfile []byte
			var err error
			if test.data != nil {
				dockerfile, err = nuxt.GenerateDockerfile(test.path, test.data)
			} else {
				dockerfile, err = nuxt.GenerateDockerfile(test.path)
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			for _, line := range test.expected {
				found := false
				lines := strings.Split(string(dockerfile), "\n")

				for _, l := range lines {
					switch v := line.(type) {
					case string:
						if strings.Contains(l, v) {
							found = true
							break
						}
					case *regexp.Regexp:
						if v.MatchString(l) {
							found = true
							break
						}
					}
				}

				if !found {
					t.Errorf("expected %v, not found in:\n%v", line, string(dockerfile))
				}
			}
		})
	}
}
