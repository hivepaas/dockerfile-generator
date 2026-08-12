package runtime_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

func TestZigMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Zig project with build.zig",
			path:     "../testdata/zig",
			expected: true,
		},
		{
			name:     "Zig project with build.zig.zon",
			path:     "../testdata/zig-zon",
			expected: true,
		},
		{
			name:     "Not a Zig project",
			path:     "../testdata/deno",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			zig := &runtime.Zig{Log: logger}
			if zig.Match(test.path) != test.expected {
				t.Errorf("expected %v, got %v", test.expected, zig.Match(test.path))
			}
		})
	}
}

func TestZigGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     map[string]string
		expected []any
	}{
		{
			name:     "Zig project default 0.13.0",
			path:     "../testdata/zig",
			expected: []any{`ARG ZIG_VERSION=0.13.0`, `CMD ["/app/app"]`},
		},
		{
			name:     "Zig project with build.zig.zon 0.12.0",
			path:     "../testdata/zig-zon",
			expected: []any{`ARG ZIG_VERSION=0.12.0`, `CMD ["/app/app"]`},
		},
		{
			name: "Zig project with build mounts",
			path: "../testdata/zig",
			data: map[string]string{"BuildMounts": `--mount=type=secret,id=_env,target=/app/.env \
    `},
			expected: []any{regexp.MustCompile(`--mount=type=secret,id=_env,target=/app/.env \\$`)},
		},
		{
			name:     "Not a Zig project",
			path:     "../testdata/deno",
			expected: []any{`ARG ZIG_VERSION=0.13.0`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			zig := &runtime.Zig{Log: logger}
			var dockerfile []byte
			var err error
			if test.data != nil {
				dockerfile, err = zig.GenerateDockerfile(test.path, test.data)
			} else {
				dockerfile, err = zig.GenerateDockerfile(test.path)
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
