package runtime

import (
	"bufio"
	"bytes"
	"encoding/json"
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

type Scala struct {
	Log *slog.Logger
}

func (s *Scala) Name() RuntimeName {
	return RuntimeNameScala
}

func (s *Scala) Match(path string) bool {
	checkFiles := []string{
		filepath.Join(path, "build.sbt"),
		filepath.Join(path, "project", "build.properties"),
		filepath.Join(path, "project", "plugins.sbt"),
		filepath.Join(path, "build.scala"),
	}

	for _, f := range checkFiles {
		if _, err := os.Stat(f); err == nil {
			s.Log.Info("Detected Scala project via " + filepath.Base(f))
			return true
		}
	}

	if _, err := os.Stat(filepath.Join(path, "src", "main", "scala")); err == nil {
		s.Log.Info("Detected Scala project via src/main/scala")
		return true
	}

	s.Log.Debug("Scala project not detected")
	return false
}

func (s *Scala) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(scalaTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template: %w", err)
	}

	javaVersion := findScalaJavaVersion(path, s.Log)
	scalaVersion := findScalaVersion(path, s.Log)
	s.Log.Info(fmt.Sprintf("Using Scala version: %s, Java version: %s", *scalaVersion, *javaVersion))

	startCMD := detectScalaStartCmd(path, s.Log)
	startCMDJSON, _ := json.Marshal(startCMD)
	startCMD = string(startCMDJSON)

	var buf bytes.Buffer
	templateData := map[string]string{
		"JavaVersion":  *javaVersion,
		"ScalaVersion": *scalaVersion,
		"StartCMD":     startCMD,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}

	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

func detectScalaStartCmd(path string, log *slog.Logger) string {
	return `sh -c 'if [ -d target/universal/stage/bin ]; then exec $(find target/universal/stage/bin -maxdepth 1 -type f -executable | head -n 1) -Dhttp.port=${PORT}; else exec java ${JAVA_OPTS} -jar $(find target -name "*.jar" ! -name "*-sources.jar" ! -name "*-javadoc.jar" | head -n 1); fi'`
}

var scalaTemplate = strings.TrimSpace(`
ARG JAVA_VERSION={{.JavaVersion}}
ARG BUILDER=docker.io/sbtscala/sbt
FROM ${BUILDER}:latest AS build
WORKDIR /app

COPY project project
COPY build.sbt ./
RUN --mount=type=cache,target=/root/.sbt \
    --mount=type=cache,target=/root/.ivy2/cache \
    --mount=type=cache,target=/root/.cache/coursier \
    {{.InstallMounts}}sbt update || true

COPY . .

ARG BUILD_CMD=
RUN --mount=type=cache,target=/root/.sbt \
    --mount=type=cache,target=/root/.ivy2/cache \
    --mount=type=cache,target=/root/.cache/coursier \
    {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; \
    elif grep -rq "sbt-native-packager" project 2>/dev/null; then sbt stage; \
    elif grep -rq "sbt-assembly" project 2>/dev/null; then sbt assembly; \
    else sbt compile package; fi

ARG RUNNER=docker.io/library/eclipse-temurin:${JAVA_VERSION}-jdk
FROM ${RUNNER} AS runtime
WORKDIR /app

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
RUN chown -R nonroot:nonroot /app

COPY --chown=nonroot:nonroot --from=build /app/target /app/target

ENV PORT=8080
EXPOSE ${PORT}
# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

ARG JAVA_OPTS=
ENV JAVA_OPTS=${JAVA_OPTS}
ARG START_CMD={{.StartCMD}}
ENV START_CMD=${START_CMD}
RUN if [ -z "${START_CMD}" ]; then echo "Unable to detect a container start command" && exit 1; fi
CMD ${START_CMD}
`)

func findScalaVersion(path string, log *slog.Logger) *string {
	version := ""

	// 1. Check build.sbt
	sbtPath := filepath.Join(path, "build.sbt")
	if data, err := os.ReadFile(sbtPath); err == nil {
		scalaVerRe := regexp.MustCompile(`scalaVersion\s*:=\s*"([^"]+)"`)
		matches := scalaVerRe.FindStringSubmatch(string(data))
		if len(matches) >= 2 {
			version = matches[1]
			log.Info("Detected Scala version in build.sbt: " + version)
		}
	}

	// 2. Check .tool-versions
	if version == "" {
		toolVersionsPath := filepath.Join(path, ".tool-versions")
		if f, err := os.Open(toolVersionsPath); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "scala") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						version = parts[1]
						log.Info("Detected Scala version in .tool-versions: " + version)
						break
					}
				}
			}
			_ = f.Close()
		}
	}

	// 3. Check .mise.toml
	if version == "" {
		misePath := filepath.Join(path, ".mise.toml")
		if f, err := os.Open(misePath); err == nil {
			var mise MiseToml
			if err := toml.NewDecoder(f).Decode(&mise); err == nil {
				if v, ok := mise.Tools["scala"].(string); ok && v != "" {
					version = v
					log.Info("Detected Scala version in .mise.toml: " + version)
				}
			}
			_ = f.Close()
		}
	}

	if version == "" {
		version = "3.4.2"
		log.Info("No Scala version detected. Using: " + version)
	}

	return &version
}

func findScalaJavaVersion(path string, log *slog.Logger) *string {
	version := ""

	// Check .tool-versions for java
	toolVersionsPath := filepath.Join(path, ".tool-versions")
	if f, err := os.Open(toolVersionsPath); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "java") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					version = parts[1]
					break
				}
			}
		}
		_ = f.Close()
	}

	if version == "" {
		version = "21"
	}

	// Extract major version
	if strings.Contains(version, ".") {
		parts := strings.Split(version, ".")
		if len(parts) > 0 {
			version = parts[0]
		}
	}

	return &version
}
