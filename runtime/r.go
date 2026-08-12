package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type R struct {
	Log *slog.Logger
}

func (d *R) Name() RuntimeName {
	return RuntimeNameR
}

func (d *R) Match(path string) bool {
	checkPaths := []string{
		filepath.Join(path, "renv.lock"),
		filepath.Join(path, "DESCRIPTION"),
		filepath.Join(path, "packrat", "packrat.lock"),
		filepath.Join(path, "plumber.R"),
		filepath.Join(path, "app.R"),
	}

	for _, p := range checkPaths {
		if _, err := os.Stat(p); err == nil {
			d.Log.Info("Detected R project")
			return true
		}
	}

	// Detect *.Rproj file
	if entries, err := os.ReadDir(path); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".Rproj") {
				d.Log.Info("Detected R project via .Rproj file")
				return true
			}
		}
	}

	d.Log.Debug("R project not detected")
	return false
}

func (d *R) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	appType := detectRAppType(path, d.Log)
	rVersion := detectRVersion(path, d.Log)

	var rTmpl string
	switch appType {
	case "shiny":
		d.Log.Info("Detected Shiny app")
		rTmpl = rShinyTemplate
	case "plumber":
		d.Log.Info("Detected Plumber API")
		rTmpl = rPlumberTemplate
	default:
		d.Log.Info("Detected generic R project, using Plumber template")
		rTmpl = rPlumberTemplate
	}

	tmpl, err := template.New("Dockerfile").Parse(rTmpl)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	templateData := map[string]string{
		"Version": rVersion,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}

	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// detectRAppType inspects the project to distinguish Shiny apps from Plumber APIs.
func detectRAppType(path string, log *slog.Logger) string {
	// Explicit plumber.R file → Plumber API
	if _, err := os.Stat(filepath.Join(path, "plumber.R")); err == nil {
		return "plumber"
	}

	// Shiny: app.R, or server.R + ui.R
	_, hasAppR := os.Stat(filepath.Join(path, "app.R"))
	_, hasServerR := os.Stat(filepath.Join(path, "server.R"))
	_, hasUiR := os.Stat(filepath.Join(path, "ui.R"))

	if hasAppR == nil || (hasServerR == nil && hasUiR == nil) {
		// Confirm by checking if the file imports shiny
		shinyFiles := []string{"app.R", "server.R", "global.R"}
		for _, sf := range shinyFiles {
			fp := filepath.Join(path, sf)
			f, err := os.Open(fp)
			if err != nil {
				continue
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "library(shiny)") || strings.Contains(line, "shinyApp(") {
					return "shiny"
				}
			}
		}
		return "shiny" // app.R / server.R present, assume shiny
	}

	// Check DESCRIPTION for Shiny dependency
	descPath := filepath.Join(path, "DESCRIPTION")
	if f, err := os.Open(descPath); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "shiny") {
				return "shiny"
			}
		}
	}

	return "plumber"
}

// detectRVersion reads R version from .r-version, renv.lock, or DESCRIPTION.
func detectRVersion(path string, log *slog.Logger) string {
	// .r-version file
	if data, err := os.ReadFile(filepath.Join(path, ".r-version")); err == nil {
		v := strings.TrimSpace(string(data))
		if v != "" {
			log.Info("Detected R version from .r-version: " + v)
			return v
		}
	}

	// renv.lock: {"R":{"Version":"4.4.1",...}}
	if data, err := os.ReadFile(filepath.Join(path, "renv.lock")); err == nil {
		content := string(data)
		// Simple scan: find "Version": "4.x.x" near "R"
		lines := strings.Split(content, "\n")
		inRBlock := false
		for _, line := range lines {
			if strings.Contains(line, `"R"`) && strings.Contains(line, "{") {
				inRBlock = true
			}
			if inRBlock && strings.Contains(line, `"Version"`) {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					v := strings.Trim(strings.TrimSpace(parts[1]), `",`)
					if v != "" {
						log.Info("Detected R version from renv.lock: " + v)
						return v
					}
				}
			}
			if inRBlock && strings.Contains(line, "}") {
				break
			}
		}
	}

	// DESCRIPTION: Depends: R (>= 4.2.0)
	if f, err := os.Open(filepath.Join(path, "DESCRIPTION")); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "Depends:") && strings.Contains(line, "R (>= ") {
				// Extract version from "R (>= 4.2.0)"
				start := strings.Index(line, "R (>= ")
				if start != -1 {
					rest := line[start+6:]
					end := strings.Index(rest, ")")
					if end != -1 {
						v := strings.TrimSpace(rest[:end])
						log.Info("Detected minimum R version from DESCRIPTION: " + v)
						return v
					}
				}
			}
		}
	}

	log.Info("Using default R version: 4.4.1")
	return "4.4.1"
}

// rPlumberTemplate: for R projects using Plumber to serve a REST API.
var rPlumberTemplate = strings.TrimSpace(`
ARG R_VERSION={{.Version}}
ARG BUILDER=docker.io/rocker/r-ver
ARG RUNNER=${BUILDER}:${R_VERSION}
FROM ${RUNNER} AS build
WORKDIR /app

# Install system dependencies for R packages
ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends \
    libcurl4-openssl-dev \
    libssl-dev \
    libxml2-dev \
    libsodium-dev \
    wget \
    ca-certificates \
    ${APT_EXTRA_PKGS} \
    && apt-get clean && rm -f /var/lib/apt/lists/*_*

# Install pak for fast package installation
RUN Rscript -e 'install.packages("pak", repos = "https://cloud.r-project.org")'

# Install renv if renv.lock is present, else install plumber via pak
COPY renv.lock* ./
RUN --mount=type=cache,target=/root/.cache/R \
    {{.InstallMounts}}if [ -f renv.lock ]; then \
      Rscript -e 'install.packages("renv", repos = "https://cloud.r-project.org"); renv::restore()'; \
    else \
      Rscript -e 'pak::pak("plumber")'; \
    fi

COPY . .

FROM ${RUNNER} AS runtime
WORKDIR /app

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends \
    libcurl4-openssl-dev \
    libssl-dev \
    libxml2-dev \
    wget \
    ca-certificates \
    ${APT_EXTRA_PKGS} \
    && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true

COPY --from=build /app /app
COPY --from=build /usr/local/lib/R /usr/local/lib/R

RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
RUN chown -R nonroot:nonroot /app

# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

ENV PORT=8080
EXPOSE ${PORT}

ARG PLUMBER_FILE=plumber.R
ENV PLUMBER_FILE=${PLUMBER_FILE}
CMD Rscript -e "pr <- plumber::plumb(Sys.getenv('PLUMBER_FILE', 'plumber.R')); pr\$run(host='0.0.0.0', port=as.integer(Sys.getenv('PORT', 8080)))"
`)

// rShinyTemplate: for R projects using Shiny to serve a web application.
var rShinyTemplate = strings.TrimSpace(`
ARG R_VERSION={{.Version}}
ARG BUILDER=docker.io/rocker/shiny
ARG RUNNER=${BUILDER}:${R_VERSION}
FROM ${RUNNER} AS build
WORKDIR /srv/shiny-server

# Install system dependencies
ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends \
    libcurl4-openssl-dev \
    libssl-dev \
    libxml2-dev \
    wget \
    ca-certificates \
    ${APT_EXTRA_PKGS} \
    && apt-get clean && rm -f /var/lib/apt/lists/*_*

# Install pak for fast package installation
RUN Rscript -e 'install.packages("pak", repos = "https://cloud.r-project.org")'

COPY renv.lock* ./
RUN --mount=type=cache,target=/root/.cache/R \
    {{.InstallMounts}}if [ -f renv.lock ]; then \
      Rscript -e 'install.packages("renv", repos = "https://cloud.r-project.org"); renv::restore()'; \
    else \
      Rscript -e 'pak::pak("shiny")'; \
    fi

COPY . .

FROM ${RUNNER} AS runtime
WORKDIR /srv/shiny-server

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends \
    libcurl4-openssl-dev \
    libssl-dev \
    libxml2-dev \
    wget \
    ca-certificates \
    ${APT_EXTRA_PKGS} \
    && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true

COPY --from=build /srv/shiny-server /srv/shiny-server
COPY --from=build /usr/local/lib/R /usr/local/lib/R

ENV PORT=3838
EXPOSE ${PORT}

# rocker/shiny uses shiny-server which reads from /srv/shiny-server
CMD ["/usr/bin/shiny-server"]
`)
