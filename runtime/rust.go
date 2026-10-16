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

	"github.com/pelletier/go-toml"
)

type Rust struct {
	Log *slog.Logger
}

func (d *Rust) Name() RuntimeName {
	return RuntimeNameRust
}

func (d *Rust) Match(path string) bool {
	checkPaths := []string{
		filepath.Join(path, "Cargo.toml"),
	}

	for _, p := range checkPaths {
		if _, err := os.Stat(p); err == nil {
			d.Log.Info("Detected Rust project")
			return true
		}
	}

	d.Log.Debug("rust project not detected")
	return false
}

func (d *Rust) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(rustlangTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template")
	}

	var binName string
	// Parse the Cargo.toml file to get the binary name
	cargoTomlPath := filepath.Join(path, "Cargo.toml")
	if _, err := os.Stat(cargoTomlPath); err == nil {
		f, err := os.Open(cargoTomlPath)
		if err != nil {
			return nil, fmt.Errorf("Failed to open Cargo.toml")
		}

		defer f.Close()

		var cargoTOML map[string]interface{}
		if err := toml.NewDecoder(f).Decode(&cargoTOML); err != nil {
			return nil, fmt.Errorf("Failed to decode Cargo.toml")
		}

		checkBins := []string{"bin", "lib", "package"}
		var ok bool
		var pkg map[string]interface{}
		for _, bin := range checkBins {
			// [[bin]]
			// [lib]
			// [package]
			if bin == "bin" {
				if pkgs, ok := cargoTOML[bin].([]map[string]interface{}); ok {
					if len(pkgs) > 0 {
						d.Log.Info("Detected binary in Cargo.toml via [[bin]]")
						pkg = pkgs[0]
						break
					}
				}
			} else if pkg, ok = cargoTOML[bin].(map[string]interface{}); ok {
				d.Log.Info("Detected binary in Cargo.toml via [" + bin + "]")
				break
			}
		}

		if binName, ok = pkg["name"].(string); !ok {
			d.Log.Warn("Failed to get binary name from Cargo.toml")
		} else {
			d.Log.Info("Detected binary name: " + binName)
		}
	}

	var buf bytes.Buffer
	templateData := map[string]string{
		"BinName": binName,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}
	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template")
	}

	return buf.Bytes(), nil
}

var rustlangTemplate = strings.TrimSpace(`
ARG BUILDER=docker.io/messense/cargo-zigbuild
ARG RUNNER=docker.io/library/debian:stable-slim
FROM --platform=${BUILDPLATFORM} ${BUILDER}:latest AS build
WORKDIR /app
COPY . .

# The platform the image is built for, which BuildKit sets: the build runs on
# the builder's own, and cross-compiles for it, to a path that does not depend
# on the platform.
ARG TARGETARCH
ARG BIN_NAME={{.BinName}}
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/local/cargo/git \
    case "${TARGETARCH}" in arm64) target=aarch64-unknown-linux-gnu ;; *) target=x86_64-unknown-linux-gnu ;; esac \
    && rustup target add "${target}" \
    && cargo zigbuild --release --target "${target}" \
    && cp "target/${target}/release/${BIN_NAME}" /app_bin

FROM ${RUNNER} AS runtime
WORKDIR /app

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
RUN chown -R nonroot:nonroot /app

COPY --chown=nonroot:nonroot --from=build /app_bin ./app

# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

ENV PORT=8080
EXPOSE ${PORT}
CMD ["/app/app"]
`)
