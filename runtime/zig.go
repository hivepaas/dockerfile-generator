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

type Zig struct {
	Log *slog.Logger
}

func (z *Zig) Name() RuntimeName {
	return RuntimeNameZig
}

func (z *Zig) Match(path string) bool {
	checkFiles := []string{
		filepath.Join(path, "build.zig"),
		filepath.Join(path, "build.zig.zon"),
		filepath.Join(path, "src", "main.zig"),
		filepath.Join(path, "main.zig"),
	}

	for _, f := range checkFiles {
		if _, err := os.Stat(f); err == nil {
			z.Log.Info("Detected Zig project via " + filepath.Base(f))
			return true
		}
	}

	z.Log.Debug("Zig project not detected")
	return false
}

func (z *Zig) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(zigTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template: %w", err)
	}

	version, err := findZigVersion(path, z.Log)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	templateData := map[string]string{
		"Version": *version,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}

	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

var zigTemplate = strings.TrimSpace(`
ARG ZIG_VERSION={{.Version}}
ARG BUILDER=docker.io/ziglang/zig
ARG RUNNER=docker.io/library/alpine:latest
FROM ${BUILDER}:${ZIG_VERSION} AS build
WORKDIR /app

COPY . .

ARG BUILD_CMD=
RUN --mount=type=cache,target=/root/.cache/zig \
    {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; \
    elif [ -f build.zig ]; then zig build -Doptimize=ReleaseSafe --prefix /app/out && cp $(find /app/out/bin -type f -executable | head -n 1) /app_bin 2>/dev/null || cp $(find zig-out/bin -type f -executable | head -n 1) /app_bin 2>/dev/null || true; \
    elif [ -f src/main.zig ]; then zig build-exe -O ReleaseSafe -femit-bin=/app_bin src/main.zig; \
    elif [ -f main.zig ]; then zig build-exe -O ReleaseSafe -femit-bin=/app_bin main.zig; \
    else echo "Unable to find Zig build recipe" && exit 1; fi

FROM ${RUNNER} AS runtime
WORKDIR /app

ARG APK_EXTRA_PKGS=
RUN apk add --no-cache ca-certificates tzdata wget ${APK_EXTRA_PKGS}
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

func findZigVersion(path string, log *slog.Logger) (*string, error) {
	version := ""

	// 1. Check .tool-versions
	toolVersionsPath := filepath.Join(path, ".tool-versions")
	if f, err := os.Open(toolVersionsPath); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "zig") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					version = cleanZigVersion(parts[1])
					log.Info("Detected Zig version in .tool-versions: " + version)
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
				if v, ok := mise.Tools["zig"].(string); ok && v != "" {
					version = cleanZigVersion(v)
					log.Info("Detected Zig version in .mise.toml: " + version)
				}
			}
			_ = f.Close()
		}
	}

	// 3. Check build.zig.zon for .minimum_zig_version
	if version == "" {
		zonPath := filepath.Join(path, "build.zig.zon")
		if data, err := os.ReadFile(zonPath); err == nil {
			minZigRe := regexp.MustCompile(`\.minimum_zig_version\s*=\s*"([^"]+)"`)
			matches := minZigRe.FindStringSubmatch(string(data))
			if len(matches) >= 2 {
				version = cleanZigVersion(matches[1])
				log.Info("Detected Zig version in build.zig.zon: " + version)
			}
		}
	}

	if version == "" {
		version = "0.13.0"
		log.Info("No Zig version detected. Using: " + version)
	}

	return &version, nil
}

func cleanZigVersion(v string) string {
	return strings.TrimSpace(v)
}
