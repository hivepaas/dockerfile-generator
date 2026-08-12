package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"
)

type Dart struct {
	Log *slog.Logger
}

func (d *Dart) Name() RuntimeName {
	return RuntimeNameDart
}

func (d *Dart) Match(path string) bool {
	checkFiles := []string{
		filepath.Join(path, "pubspec.yaml"),
		filepath.Join(path, "pubspec.yml"),
	}

	for _, f := range checkFiles {
		if _, err := os.Stat(f); err == nil {
			d.Log.Info("Detected Dart project via " + filepath.Base(f))
			return true
		}
	}

	d.Log.Debug("Dart project not detected")
	return false
}

func (d *Dart) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(dartTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template: %w", err)
	}

	version, err := findDartVersion(path, d.Log)
	if err != nil {
		return nil, err
	}

	mainFile := findDartMainFile(path, d.Log)
	d.Log.Info("Using Dart main file: " + mainFile)

	var buf bytes.Buffer
	templateData := map[string]string{
		"Version":  *version,
		"MainFile": mainFile,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}

	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

var dartTemplate = strings.TrimSpace(`
ARG DART_VERSION={{.Version}}
ARG BUILDER=docker.io/library/dart
ARG RUNNER=docker.io/library/debian:stable-slim
FROM ${BUILDER}:${DART_VERSION} AS build
WORKDIR /app

# Resolve app dependencies
COPY pubspec.* ./
ARG INSTALL_CMD="dart pub get"
RUN --mount=type=cache,target=/root/.pub-cache \
    {{.InstallMounts}}if [ ! -z "${INSTALL_CMD}" ]; then sh -c "$INSTALL_CMD"; fi

# Copy app source code and compile AOT native binary
COPY . .
RUN --mount=type=cache,target=/root/.pub-cache \
    dart pub get --offline

ARG BUILD_CMD=
RUN --mount=type=cache,target=/root/.pub-cache \
    {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; else dart compile exe {{.MainFile}} -o /app/bin/server; fi

FROM ${RUNNER} AS runtime
WORKDIR /app

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
RUN chown -R nonroot:nonroot /app

COPY --chown=nonroot:nonroot --from=build /app/bin/server /app/bin/server

ENV PORT=8080
EXPOSE ${PORT}
# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

CMD ["/app/bin/server"]
`)

func findDartVersion(path string, log *slog.Logger) (*string, error) {
	version := ""

	// 1. Check .tool-versions
	toolVersionsPath := filepath.Join(path, ".tool-versions")
	if f, err := os.Open(toolVersionsPath); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "dart") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					version = cleanDartVersion(parts[1])
					log.Info("Detected Dart version in .tool-versions: " + version)
					break
				}
			}
		}
		_ = f.Close()
	}

	// 2. Check .mise.toml
	if version == "" {
		misePath := filepath.Join(path, ".mise.toml")
		if f, err := os.Open(misePath); err == nil {
			var mise MiseToml
			if err := toml.NewDecoder(f).Decode(&mise); err == nil {
				if v, ok := mise.Tools["dart"].(string); ok && v != "" {
					version = cleanDartVersion(v)
					log.Info("Detected Dart version in .mise.toml: " + version)
				}
			}
			_ = f.Close()
		}
	}

	// 3. Check pubspec.yaml for sdk version constraint
	if version == "" {
		pubspecPath := filepath.Join(path, "pubspec.yaml")
		if _, err := os.Stat(pubspecPath); err != nil {
			pubspecPath = filepath.Join(path, "pubspec.yml")
		}

		if data, err := os.ReadFile(pubspecPath); err == nil {
			sdkRe := regexp.MustCompile(`sdk:\s*['"]?(?:>=|\^|~|>)?\s*([0-9]+(?:\.[0-9]+)?)`)
			matches := sdkRe.FindStringSubmatch(string(data))
			if len(matches) >= 2 {
				version = cleanDartVersion(matches[1])
				log.Info("Detected Dart version in pubspec.yaml: " + version)
			}
		}
	}

	if version == "" {
		version = "stable"
		log.Info("No Dart version detected. Using: " + version)
	}

	return &version, nil
}

func cleanDartVersion(v string) string {
	v = strings.TrimSpace(v)
	// If 3.4.0 -> 3.4
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

func findDartMainFile(path string, log *slog.Logger) string {
	candidates := []string{
		"bin/server.dart",
		"bin/main.dart",
		"bin/app.dart",
		"server.dart",
		"main.dart",
	}

	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(path, c)); err == nil {
			return c
		}
	}

	// Check any .dart file in bin/
	binFiles, err := filepath.Glob(filepath.Join(path, "bin", "*.dart"))
	if err == nil && len(binFiles) > 0 {
		return filepath.Join("bin", filepath.Base(binFiles[0]))
	}

	return "bin/server.dart"
}
