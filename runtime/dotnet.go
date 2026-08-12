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

type DotNet struct {
	Log *slog.Logger
}

func (d *DotNet) Name() RuntimeName {
	return RuntimeNameDotNet
}

func (d *DotNet) Match(path string) bool {
	// Check for *.csproj, *.fsproj, *.vbproj, *.sln, global.json, Directory.Build.props
	directFiles := []string{
		"global.json",
		"Directory.Build.props",
	}

	for _, f := range directFiles {
		if _, err := os.Stat(filepath.Join(path, f)); err == nil {
			d.Log.Info("Detected .NET project via " + f)
			return true
		}
	}

	// Check for solution or project files in root
	matches, err := filepath.Glob(filepath.Join(path, "*.*proj"))
	if err == nil && len(matches) > 0 {
		d.Log.Info("Detected .NET project via project file")
		return true
	}

	slnMatches, err := filepath.Glob(filepath.Join(path, "*.sln"))
	if err == nil && len(slnMatches) > 0 {
		d.Log.Info("Detected .NET project via solution file")
		return true
	}

	// Search 1-2 levels down for *.csproj or *.fsproj
	subMatches, err := filepath.Glob(filepath.Join(path, "*", "*.*proj"))
	if err == nil && len(subMatches) > 0 {
		d.Log.Info("Detected .NET project in subdirectory")
		return true
	}

	d.Log.Debug(".NET project not detected")
	return false
}

func (d *DotNet) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(dotnetTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template: %w", err)
	}

	version, err := findDotnetVersion(path, d.Log)
	if err != nil {
		return nil, err
	}

	// Find project file name to determine the dll name
	dllName := findDotnetDllName(path, d.Log)
	startCMD := ""
	if dllName != "" {
		startCMD = fmt.Sprintf("dotnet %s.dll", dllName)
		startCMDJSON, _ := json.Marshal(startCMD)
		startCMD = string(startCMDJSON)
		d.Log.Info("Detected .NET start command: " + startCMD)
	}

	var buf bytes.Buffer
	templateData := map[string]string{
		"Version":  *version,
		"StartCMD": startCMD,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}

	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

var dotnetTemplate = strings.TrimSpace(`
ARG DOTNET_VERSION={{.Version}}
ARG BUILDER=mcr.microsoft.com/dotnet/sdk
ARG RUNNER=mcr.microsoft.com/dotnet/aspnet:${DOTNET_VERSION}-alpine
FROM ${BUILDER}:${DOTNET_VERSION} AS build
WORKDIR /source

COPY . .
ARG RESTORE_CMD=
RUN --mount=type=cache,target=/root/.nuget/packages \
    {{.InstallMounts}}if [ ! -z "${RESTORE_CMD}" ]; then sh -c "$RESTORE_CMD"; else dotnet restore; fi

ARG BUILD_CMD=
RUN --mount=type=cache,target=/root/.nuget/packages \
    {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; else dotnet publish -c Release -o /app/publish; fi

FROM ${RUNNER} AS runtime
WORKDIR /app

ARG APK_EXTRA_PKGS=
RUN if command -v apk >/dev/null 2>&1; then apk add --no-cache ca-certificates tzdata wget ${APK_EXTRA_PKGS}; \
    elif command -v apt-get >/dev/null 2>&1; then apt-get update && apt-get install -y --no-install-recommends wget ca-certificates && apt-get clean && rm -f /var/lib/apt/lists/*_*; fi
RUN if command -v addgroup >/dev/null 2>&1; then addgroup -S nonroot && adduser -S nonroot -G nonroot; \
    elif command -v groupadd >/dev/null 2>&1; then groupadd -r nonroot && useradd -r -g nonroot nonroot; fi
RUN chown -R nonroot:nonroot /app

COPY --chown=nonroot:nonroot --from=build /app/publish .

ENV PORT=8080
ENV ASPNETCORE_URLS=http://+:${PORT}
EXPOSE ${PORT}
# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

ARG START_CMD={{.StartCMD}}
ENV START_CMD=${START_CMD}
RUN if [ -z "${START_CMD}" ]; then echo "Unable to detect a container start command" && exit 1; fi
CMD ${START_CMD}
`)

func findDotnetVersion(path string, log *slog.Logger) (*string, error) {
	version := ""

	// 1. Check .tool-versions
	toolVersionsPath := filepath.Join(path, ".tool-versions")
	if f, err := os.Open(toolVersionsPath); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "dotnet") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					version = cleanDotnetVersion(parts[1])
					log.Info("Detected .NET version in .tool-versions: " + version)
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
				if v, ok := mise.Tools["dotnet"].(string); ok && v != "" {
					version = cleanDotnetVersion(v)
					log.Info("Detected .NET version in .mise.toml: " + version)
				}
			}
			_ = f.Close()
		}
	}

	// 3. Check global.json
	if version == "" {
		globalJsonPath := filepath.Join(path, "global.json")
		if data, err := os.ReadFile(globalJsonPath); err == nil {
			var globalConfig struct {
				Sdk struct {
					Version string `json:"version"`
				} `json:"sdk"`
			}
			if err := json.Unmarshal(data, &globalConfig); err == nil && globalConfig.Sdk.Version != "" {
				version = cleanDotnetVersion(globalConfig.Sdk.Version)
				log.Info("Detected .NET version in global.json: " + version)
			}
		}
	}

	// 4. Check *.csproj / *.fsproj / *.vbproj for <TargetFramework>netX.Y</TargetFramework>
	if version == "" {
		projectFiles, _ := filepath.Glob(filepath.Join(path, "*.*proj"))
		if len(projectFiles) == 0 {
			projectFiles, _ = filepath.Glob(filepath.Join(path, "*", "*.*proj"))
		}

		targetFrameworkRe := regexp.MustCompile(`<TargetFramework>net([0-9]+(?:\.[0-9]+)?)</TargetFramework>`)
		for _, pf := range projectFiles {
			if data, err := os.ReadFile(pf); err == nil {
				matches := targetFrameworkRe.FindStringSubmatch(string(data))
				if len(matches) >= 2 {
					version = cleanDotnetVersion(matches[1])
					log.Info(fmt.Sprintf("Detected .NET version in %s: %s", filepath.Base(pf), version))
					break
				}
			}
		}
	}

	if version == "" {
		version = "8.0"
		log.Info("No .NET version detected. Using: " + version)
	}

	return &version, nil
}

func cleanDotnetVersion(v string) string {
	v = strings.TrimSpace(v)
	// If 8.0.400 -> 8.0
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

func findDotnetDllName(path string, log *slog.Logger) string {
	// Look for *.*proj in root first
	projectFiles, _ := filepath.Glob(filepath.Join(path, "*.*proj"))
	if len(projectFiles) > 0 {
		base := filepath.Base(projectFiles[0])
		return strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Check subdirectories (e.g. src/MyApp/MyApp.csproj)
	subProjectFiles, _ := filepath.Glob(filepath.Join(path, "*", "*.*proj"))
	if len(subProjectFiles) > 0 {
		base := filepath.Base(subProjectFiles[0])
		return strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Check if directory name itself can be used
	abs, err := filepath.Abs(path)
	if err == nil {
		return filepath.Base(abs)
	}

	return ""
}
