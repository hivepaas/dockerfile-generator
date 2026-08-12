package runtime_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

func TestCppMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "C++ project with CMake",
			path:     "../testdata/cpp-cmake",
			expected: true,
		},
		{
			name:     "C project with Makefile",
			path:     "../testdata/cpp-makefile",
			expected: true,
		},
		{
			name:     "C++ project single file",
			path:     "../testdata/cpp-simple",
			expected: true,
		},
		{
			name:     "Not a C++ project",
			path:     "../testdata/deno",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cpp := &runtime.Cpp{Log: logger}
			if cpp.Match(test.path) != test.expected {
				t.Errorf("expected %v, got %v", test.expected, cpp.Match(test.path))
			}
		})
	}
}

func TestCppGenerateDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     map[string]string
		expected []any
	}{
		{
			name:     "C++ project with CMake",
			path:     "../testdata/cpp-cmake",
			expected: []any{`cmake -B build -DCMAKE_BUILD_TYPE=Release`, `CMD ["/app/app"]`},
		},
		{
			name:     "C project with Makefile",
			path:     "../testdata/cpp-makefile",
			expected: []any{`make && cp $(find . -maxdepth 1`, `CMD ["/app/app"]`},
		},
		{
			name:     "C++ project simple main.cpp",
			path:     "../testdata/cpp-simple",
			expected: []any{`g++ -O3 -o /app_bin main.cpp`, `CMD ["/app/app"]`},
		},
		{
			name: "C++ project with build mounts",
			path: "../testdata/cpp-cmake",
			data: map[string]string{"BuildMounts": `--mount=type=secret,id=_env,target=/app/.env \
    `},
			expected: []any{regexp.MustCompile(`--mount=type=secret,id=_env,target=/app/.env \\$`)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cpp := &runtime.Cpp{Log: logger}
			var dockerfile []byte
			var err error
			if test.data != nil {
				dockerfile, err = cpp.GenerateDockerfile(test.path, test.data)
			} else {
				dockerfile, err = cpp.GenerateDockerfile(test.path)
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
