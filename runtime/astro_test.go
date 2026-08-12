package runtime_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

func TestAstroMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Astro SSR project",
			path:     "../testdata/astro",
			expected: true,
		},
		{
			name:     "Astro static project",
			path:     "../testdata/astro-static",
			expected: true,
		},
		{
			name:     "Not an Astro project",
			path:     "../testdata/deno",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			astro := &runtime.Astro{Log: logger}
			if astro.Match(test.path) != test.expected {
				t.Errorf("expected %v, got %v", test.expected, astro.Match(test.path))
			}
		})
	}
}

func TestAstroGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     map[string]string
		expected []any
	}{
		{
			name: "Astro SSR project with node adapter",
			path: "../testdata/astro",
			expected: []any{
				`ARG NODE_VERSION=lts`,
				`dist/server/entry.mjs`,
			},
		},
		{
			name: "Astro static project",
			path: "../testdata/astro-static",
			expected: []any{
				`ARG NODE_VERSION=lts`,
				`joseluisq/static-web-server`,
				`/public`,
			},
		},
		{
			name: "Astro project with install mounts",
			path: "../testdata/astro",
			data: map[string]string{"InstallMounts": `--mount=type=secret,id=_env,target=/app/.env \
    `},
			expected: []any{regexp.MustCompile(`--mount=type=secret,id=_env,target=/app/.env \\$`)},
		},
		{
			name:     "Not an Astro project",
			path:     "../testdata/deno",
			expected: []any{`ARG NODE_VERSION=lts`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			astro := &runtime.Astro{Log: logger}
			var dockerfile []byte
			var err error
			if test.data != nil {
				dockerfile, err = astro.GenerateDockerfile(test.path, test.data)
			} else {
				dockerfile, err = astro.GenerateDockerfile(test.path)
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
					t.Errorf("expected %v, not found in %v", line, string(dockerfile))
				}
			}
		})
	}
}
