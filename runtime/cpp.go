package runtime

import (
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type Cpp struct {
	Log *slog.Logger
}

func (c *Cpp) Name() RuntimeName {
	return RuntimeNameCpp
}

func (c *Cpp) Match(path string) bool {
	// Check for CMake, Makefile, Meson, or direct C/C++ source files
	directFiles := []string{
		"CMakeLists.txt",
		"meson.build",
		"configure.ac",
	}

	for _, f := range directFiles {
		if _, err := os.Stat(filepath.Join(path, f)); err == nil {
			c.Log.Info("Detected C/C++ project via " + f)
			return true
		}
	}

	// Check for Makefile only if not other runtimes (e.g. Ruby rakefile or Go Makefile)
	makefiles := []string{"Makefile", "makefile", "GNUmakefile"}
	for _, f := range makefiles {
		if _, err := os.Stat(filepath.Join(path, f)); err == nil {
			// If CMake or C/C++ files exist, it's C/C++
			cFiles, _ := filepath.Glob(filepath.Join(path, "*.c*"))
			if len(cFiles) > 0 {
				c.Log.Info("Detected C/C++ project via " + f)
				return true
			}
			// If src/*.cpp exists
			srcFiles, _ := filepath.Glob(filepath.Join(path, "src", "*.c*"))
			if len(srcFiles) > 0 {
				c.Log.Info("Detected C/C++ project via " + f + " and src/")
				return true
			}
		}
	}

	// Direct main C/C++ files
	cSourceFiles := []string{
		"main.cpp",
		"main.cc",
		"main.cxx",
		"main.c",
		"app.cpp",
		"app.c",
	}

	for _, f := range cSourceFiles {
		if _, err := os.Stat(filepath.Join(path, f)); err == nil {
			c.Log.Info("Detected C/C++ project via " + f)
			return true
		}
	}

	c.Log.Debug("C/C++ project not detected")
	return false
}

func (c *Cpp) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(cppTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template: %w", err)
	}

	buildCMD := detectCppBuildCmd(path, c.Log)
	c.Log.Info("Using C/C++ build command: " + buildCMD)

	var buf bytes.Buffer
	templateData := map[string]string{
		"BuildCMD": buildCMD,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}

	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

func detectCppBuildCmd(path string, log *slog.Logger) string {
	if _, err := os.Stat(filepath.Join(path, "CMakeLists.txt")); err == nil {
		return "cmake -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build && cp $(find build -maxdepth 2 -type f -executable | head -n 1) /app_bin"
	}

	if _, err := os.Stat(filepath.Join(path, "meson.build")); err == nil {
		return "meson setup build && ninja -C build && cp $(find build -maxdepth 2 -type f -executable | head -n 1) /app_bin"
	}

	makefiles := []string{"Makefile", "makefile", "GNUmakefile"}
	for _, f := range makefiles {
		if _, err := os.Stat(filepath.Join(path, f)); err == nil {
			return "make && cp $(find . -maxdepth 1 -type f -executable ! -name \"*.sh\" ! -name \"Makefile*\" | head -n 1) /app_bin"
		}
	}

	// Single file compilation
	if _, err := os.Stat(filepath.Join(path, "main.cpp")); err == nil {
		return "g++ -O3 -o /app_bin main.cpp"
	}
	if _, err := os.Stat(filepath.Join(path, "main.cc")); err == nil {
		return "g++ -O3 -o /app_bin main.cc"
	}
	if _, err := os.Stat(filepath.Join(path, "main.cxx")); err == nil {
		return "g++ -O3 -o /app_bin main.cxx"
	}
	if _, err := os.Stat(filepath.Join(path, "main.c")); err == nil {
		return "gcc -O3 -o /app_bin main.c"
	}

	return "make && cp $(find . -maxdepth 1 -type f -executable | head -n 1) /app_bin"
}

var cppTemplate = strings.TrimSpace(`
ARG BUILDPLATFORM=linux/amd64
ARG BUILDER=docker.io/library/alpine:latest
ARG RUNNER=docker.io/library/alpine:latest
FROM --platform=${BUILDPLATFORM} ${BUILDER} AS build
WORKDIR /src

RUN apk add --no-cache build-base cmake ninja git clang

COPY . .

ARG BUILD_CMD="{{.BuildCMD}}"
RUN --mount=type=cache,target=/root/.cache \
    {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; else cmake -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build && cp $(find build -maxdepth 2 -type f -executable | head -n 1) /app_bin; fi

FROM ${RUNNER} AS runtime
WORKDIR /app

ARG APK_EXTRA_PKGS=
RUN apk add --no-cache ca-certificates tzdata wget libstdc++ libgcc ${APK_EXTRA_PKGS}
RUN addgroup -S nonroot && adduser -S nonroot -G nonroot
RUN chown -R nonroot:nonroot /app

COPY --chown=nonroot:nonroot --from=build /app_bin /app/app

ENV PORT=8080
EXPOSE ${PORT}
# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

CMD ["/app/app"]
`)
