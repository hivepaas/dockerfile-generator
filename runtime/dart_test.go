package runtime_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

func TestDartMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Dart project",
			path:     "../testdata/dart",
			expected: true,
		},
		{
			name:     "Dart project with bin/main.dart",
			path:     "../testdata/dart-main",
			expected: true,
		},
		{
			name:     "Not a Dart project",
			path:     "../testdata/deno",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dart := &runtime.Dart{Log: logger}
			if dart.Match(test.path) != test.expected {
				t.Errorf("expected %v, got %v", test.expected, dart.Match(test.path))
			}
		})
	}
}

func TestDartGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     map[string]string
		expected []any
	}{
		{
			name:     "Dart project SDK 3.4",
			path:     "../testdata/dart",
			expected: []any{`ARG DART_VERSION=3.4`, `dart compile exe bin/server.dart -o /app/bin/server`},
		},
		{
			name:     "Dart project with bin/main.dart SDK 3.0",
			path:     "../testdata/dart-main",
			expected: []any{`ARG DART_VERSION=3.0`, `dart compile exe bin/main.dart -o /app/bin/server`},
		},
		{
			name: "Dart project with install mounts",
			path: "../testdata/dart",
			data: map[string]string{"InstallMounts": `--mount=type=secret,id=_env,target=/app/.env \
    `},
			expected: []any{regexp.MustCompile(`--mount=type=secret,id=_env,target=/app/.env \\$`)},
		},
		{
			name:     "Not a Dart project",
			path:     "../testdata/deno",
			expected: []any{`ARG DART_VERSION=stable`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dart := &runtime.Dart{Log: logger}
			var dockerfile []byte
			var err error
			if test.data != nil {
				dockerfile, err = dart.GenerateDockerfile(test.path, test.data)
			} else {
				dockerfile, err = dart.GenerateDockerfile(test.path)
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
