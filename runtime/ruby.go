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

	"github.com/Masterminds/semver/v3"

	"github.com/pelletier/go-toml/v2"
)

type Ruby struct {
	Log *slog.Logger
}

func (d *Ruby) Name() RuntimeName {
	return RuntimeNameRuby
}

func (d *Ruby) Match(path string) bool {
	checkPaths := []string{
		filepath.Join(path, "Gemfile"),
		filepath.Join(path, "Gemfile.lock"),
		filepath.Join(path, "Rakefile"),
		filepath.Join(path, "config.ru"),
		filepath.Join(path, "config/environment.rb"),
	}

	for _, p := range checkPaths {
		if _, err := os.Stat(p); err == nil {
			d.Log.Info("Detected Ruby project")
			return true
		}
	}

	d.Log.Debug("Ruby project not detected")
	return false
}

func (d *Ruby) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(rubyTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template")
	}

	// Parse version from go.mod
	version, err := findRubyVersion(path, d.Log)
	if err != nil {
		return nil, err
	}

	installCMD := "bundle install"
	packageManager := ""

	if _, err := os.Stat(filepath.Join(path, "package-lock.json")); err == nil {
		packageManager = "npm"
		installCMD = installCMD + " && npm ci"
	} else if _, err := os.Stat(filepath.Join(path, "pnpm-lock.yaml")); err == nil {
		packageManager = "pnpm"
		installCMD = installCMD + " && pnpm i --frozen-lockfile"
	} else if _, err := os.Stat(filepath.Join(path, "yarn.lock")); err == nil {
		packageManager = "yarn"
		installCMD = installCMD + " && yarn --frozen-lockfile"
	} else if hasBunLock(path) {
		packageManager = "bun"
		installCMD = installCMD + " && npm i -g bun && bun install"
	}

	nodeVersion := "lts"
	if packageManager != "" {
		d.Log.Info("Detected Node.js package manager: " + packageManager)
		if v, err := findNodeVersion(path, d.Log); err == nil {
			nodeVersion = *v
		}
	}

	buildCMD := ""
	startCMD := ""
	if IsRailsProject(path) {
		d.Log.Info("Detected Rails project")
		buildCMD = "bundle exec rails assets:precompile"
		startCMD = "bundle exec rails server -b 0.0.0.0 -p ${PORT}"
	} else {
		configFiles := []string{"config.ru", "Rakefile"}

		for _, fn := range configFiles {
			_, err := os.Stat(filepath.Join(path, fn))
			if err != nil {
				continue
			}

			switch fn {
			case "config.ru":
				d.Log.Info("Detected Rack project")
				// Puma serves config.ru, as rackup does with Rack 3 only when
				// the rackup gem is there
				if gemfileHas(path, "puma") {
					startCMD = "bundle exec puma -b tcp://0.0.0.0:${PORT} config.ru"
				} else {
					startCMD = "bundle exec rackup config.ru -o 0.0.0.0 -p ${PORT}"
				}
			case "Rakefile":
				d.Log.Info("Detected Rake project")
				startCMD = "bundle exec rake"
			}

			break
		}
	}

	d.Log.Info(
		fmt.Sprintf(`Detected defaults 
  Ruby version         : %s
  Node package manager : %s
  Install command      : %s
  Build command        : %s
  Start command        : %s

  Docker build arguments can supersede these defaults if provided.
  See https://github.com/hivepaas/dockerfile-generator#runtime-documentation`, *version, packageManager, installCMD, buildCMD, startCMD),
	)

	var buf bytes.Buffer
	templateData := map[string]string{
		"Version":     *version,
		"NodeVersion": nodeVersion,
		"InstallCMD":  safeCommand(installCMD),
		"BuildCMD":    safeCommand(buildCMD),
		"StartCMD":    safeCommand(startCMD),
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}
	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template")
	}

	return buf.Bytes(), nil
}

var rubyTemplate = strings.TrimSpace(`
ARG RUBY_VERSION={{.Version}}
ARG NODE_VERSION={{.NodeVersion}}
ARG BUILDER=docker.io/library/ruby
ARG RUNNER=${BUILDER}:${RUBY_VERSION}-slim
FROM docker.io/library/node:${NODE_VERSION}-slim AS node

FROM ${RUNNER} AS base
WORKDIR /app
ARG APT_EXTRA_PKGS=
# The libraries most apps' gems load: PostgreSQL's and MySQL's clients, YAML's
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates libpq5 libmariadb3 libyaml-0-2 ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
ENV RAILS_ENV=production \
    RACK_ENV=production \
    NODE_ENV=production \
    RAILS_LOG_TO_STDOUT=1 \
    RAILS_SERVE_STATIC_FILES=1 \
    BUNDLE_WITHOUT=development:test

FROM base AS build
# The compilers and headers gems with native extensions are built with
RUN apt-get update && apt-get install -y --no-install-recommends build-essential git pkg-config libpq-dev libmariadb-dev libyaml-dev && apt-get clean && rm -f /var/lib/apt/lists/*_*
# Node.js, for the assets an app builds with npm, pnpm or Yarn
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx \
    && if [ -f /usr/local/lib/node_modules/corepack/dist/corepack.js ]; then \
        ln -s ../lib/node_modules/corepack/dist/corepack.js /usr/local/bin/corepack; \
    else npm install -g --no-fund --no-audit corepack; fi \
    && corepack enable
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0

COPY . .
ARG INSTALL_CMD={{.InstallCMD}}
RUN --mount=type=cache,target=/root/.bundle/cache \
    {{.InstallMounts}}if [ ! -z "${INSTALL_CMD}" ]; then sh -c "$INSTALL_CMD"; fi
# Rails precompiles its assets without its secret: a placeholder, in this step only
ARG BUILD_CMD={{.BuildCMD}}
RUN {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then SECRET_KEY_BASE=assets-precompile-placeholder sh -c "$BUILD_CMD"; fi

FROM base AS runtime
COPY --from=build /usr/local/bundle /usr/local/bundle
COPY --from=build --chown=nonroot:nonroot /app /app

ENV PORT=8080
EXPOSE ${PORT}
# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

ARG START_CMD={{.StartCMD}}
ENV START_CMD=${START_CMD}
RUN if [ -z "${START_CMD}" ]; then echo "Unable to detect a container start command" && exit 1; fi
CMD ${START_CMD}
`)

func findRubyVersion(path string, log *slog.Logger) (*string, error) {
	version := ""
	versionFiles := []string{
		".tool-versions",
		".ruby-version",
		".mise.toml",
		"Gemfile",
	}

	for _, file := range versionFiles {
		fp := filepath.Join(path, file)
		_, err := os.Stat(fp)

		if err == nil {
			f, err := os.Open(fp)
			if err != nil {
				continue
			}

			defer f.Close()
			switch file {
			case ".tool-versions":
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.Contains(line, "ruby") {
						version = strings.Split(line, " ")[1]
						log.Info("Detected Ruby version in .tool-versions: " + version)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read .tool-versions file")
				}

			case ".ruby-version":
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line != "" {
						version = strings.TrimPrefix(line, "ruby-")
						log.Info("Detected Ruby version from .ruby-version: " + version)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read go.mod file")
				}

			case "Gemfile":
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					m := gemfileRubyRe.FindStringSubmatch(scanner.Text())
					if m == nil || strings.HasPrefix(m[1], ".") {
						continue
					}
					version = rubyVersionTag(m[1])
					log.Info("Detected Ruby version from Gemfile: " + version)
					break
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read Gemfile")
				}

			case ".mise.toml":
				var mise MiseToml
				if err := toml.NewDecoder(f).Decode(&mise); err != nil {
					return nil, fmt.Errorf("Failed to decode .mise.toml file")
				}
				rubyVersion, ok := mise.Tools["ruby"].(string)
				if !ok {
					versions, ok := mise.Tools["ruby"].([]string)
					if ok {
						rubyVersion = versions[0]
					}
				}
				if rubyVersion != "" {
					version = rubyVersion
					log.Info("Detected Python version in .mise.toml: " + version)
					break
				}
			}

			f.Close()
			if version != "" {
				break
			}
		}
	}

	if version == "" {
		version = "3.3"
		log.Info(fmt.Sprintf("No Ruby version detected. Using: %s", version))
	}

	return &version, nil
}

// gemfileRubyRe is the Gemfile's ruby line: ruby "3.3.6", ruby "~> 3.3".
var gemfileRubyRe = regexp.MustCompile(`^\s*ruby\s+['"]([^'"]+)['"]`)

// rubyVersionTag is the image tag for the Gemfile's Ruby: a version as it is,
// the newest a constraint allows within its oldest major otherwise - "~> 3.2"
// is Ruby 3's newest, not 3.2, out of support.
func rubyVersionTag(constraint string) string {
	constraint = strings.TrimSpace(constraint)
	if rubyExactVersionRe.MatchString(constraint) {
		return constraint
	}
	// RubyGems' "~> 3.2" is >= 3.2, < 4; "~> 3.2.1" is >= 3.2.1, < 3.3
	converted := rubyPessimisticRe.ReplaceAllStringFunc(constraint, func(m string) string {
		v := strings.TrimSpace(strings.TrimPrefix(m, "~>"))
		if strings.Count(v, ".") == 1 {
			return "^" + v
		}
		return "~" + v
	})
	c, err := semver.NewConstraint(converted)
	if err != nil {
		return constraint
	}
	for major := uint64(0); major <= 9; major++ {
		if tag, ok := newestAllowed(c.Check, major); ok {
			return tag
		}
	}
	return constraint
}

var (
	rubyExactVersionRe = regexp.MustCompile(`^\d+(\.\d+)*$`)
	rubyPessimisticRe  = regexp.MustCompile(`~>\s*[\d.]+`)
)

// IsRubyWebApp says whether a project is a Ruby web app: a Rails app, or a
// Gemfile beside a config.ru, Rack's - a package.json beside it builds its
// assets.
func IsRubyWebApp(path string) bool {
	if IsRailsProject(path) {
		return true
	}
	_, gemfileErr := os.Stat(filepath.Join(path, "Gemfile"))
	_, rackupErr := os.Stat(filepath.Join(path, "config.ru"))
	return gemfileErr == nil && rackupErr == nil
}

var railsGemRe = regexp.MustCompile(`(?m)^\s*gem\s+['"]rails['"]`)

// IsRailsProject says whether a project is a Rails app: its Gemfile has the
// rails gem, or it has Rails' config/environment.rb.
func IsRailsProject(path string) bool {
	if content, err := os.ReadFile(filepath.Join(path, "Gemfile")); err == nil && railsGemRe.Match(content) {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "config", "environment.rb"))
	return err == nil
}

// gemfileHas says whether the Gemfile names the gem.
func gemfileHas(path, gem string) bool {
	content, err := os.ReadFile(filepath.Join(path, "Gemfile"))
	if err != nil {
		return false
	}
	return regexp.MustCompile(`(?m)^\s*gem\s+['"]` + regexp.QuoteMeta(gem) + `['"]`).Match(content)
}
