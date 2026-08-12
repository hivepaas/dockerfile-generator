package runtime_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

func TestDotNetMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     ".NET project",
			path:     "../testdata/dotnet",
			expected: true,
		},
		{
			name:     ".NET project with global.json",
			path:     "../testdata/dotnet-global-json",
			expected: true,
		},
		{
			name:     "Not a .NET project",
			path:     "../testdata/deno",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dotnet := &runtime.DotNet{Log: logger}
			if dotnet.Match(test.path) != test.expected {
				t.Errorf("expected %v, got %v", test.expected, dotnet.Match(test.path))
			}
		})
	}
}

func TestDotNetGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     map[string]string
		expected []any
	}{
		{
			name:     ".NET project net8.0",
			path:     "../testdata/dotnet",
			expected: []any{`ARG DOTNET_VERSION=8.0`, `ARG START_CMD="dotnet MyApi.dll"`},
		},
		{
			name:     ".NET project with global.json net9.0",
			path:     "../testdata/dotnet-global-json",
			expected: []any{`ARG DOTNET_VERSION=9.0`, `ARG START_CMD="dotnet App.dll"`},
		},
		{
			name: "DotNet project with install mounts",
			path: "../testdata/dotnet",
			data: map[string]string{"InstallMounts": `--mount=type=secret,id=_env,target=/app/.env \
    `},
			expected: []any{regexp.MustCompile(`--mount=type=secret,id=_env,target=/app/.env \\$`)},
		},
		{
			name:     "Not a .NET project",
			path:     "../testdata/deno",
			expected: []any{`ARG DOTNET_VERSION=8.0`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dotnet := &runtime.DotNet{Log: logger}
			var dockerfile []byte
			var err error
			if test.data != nil {
				dockerfile, err = dotnet.GenerateDockerfile(test.path, test.data)
			} else {
				dockerfile, err = dotnet.GenerateDockerfile(test.path)
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
