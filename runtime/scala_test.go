package runtime_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

func TestScalaMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Scala project with build.sbt",
			path:     "../testdata/scala",
			expected: true,
		},
		{
			name:     "Scala project with project/build.properties",
			path:     "../testdata/scala",
			expected: true,
		},
		{
			name:     "Not a Scala project",
			path:     "../testdata/deno",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scala := &runtime.Scala{Log: logger}
			if scala.Match(test.path) != test.expected {
				t.Errorf("expected %v, got %v", test.expected, scala.Match(test.path))
			}
		})
	}
}

func TestScalaGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     map[string]string
		expected []any
	}{
		{
			name: "Scala 3 project",
			path: "../testdata/scala",
			expected: []any{
				`ARG JAVA_VERSION=21`,
				`docker.io/sbtscala/sbt`,
				`sbt update`,
				`CMD`,
			},
		},
		{
			name: "Scala 2 project",
			path: "../testdata/scala-scala2",
			expected: []any{
				`ARG JAVA_VERSION=21`,
				`docker.io/sbtscala/sbt`,
			},
		},
		{
			name: "Scala project with install mounts",
			path: "../testdata/scala",
			data: map[string]string{"InstallMounts": `--mount=type=secret,id=_env,target=/app/.env \
    `},
			expected: []any{regexp.MustCompile(`--mount=type=secret,id=_env,target=/app/.env \\$`)},
		},
		{
			name:     "Not a Scala project",
			path:     "../testdata/deno",
			expected: []any{`ARG JAVA_VERSION=21`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scala := &runtime.Scala{Log: logger}
			var dockerfile []byte
			var err error
			if test.data != nil {
				dockerfile, err = scala.GenerateDockerfile(test.path, test.data)
			} else {
				dockerfile, err = scala.GenerateDockerfile(test.path)
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
