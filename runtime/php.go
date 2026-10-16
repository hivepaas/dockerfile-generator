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
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/semver/v3"
)

type PHP struct {
	Log *slog.Logger
}

func (d *PHP) Name() RuntimeName {
	return RuntimeNamePHP
}

func (d *PHP) Match(path string) bool {
	checkPaths := []string{
		filepath.Join(path, "composer.json"),
		filepath.Join(path, "index.php"),
	}

	for _, p := range checkPaths {
		if _, err := os.Stat(p); err == nil {
			d.Log.Info("Detected PHP project")
			return true
		}
	}

	d.Log.Debug("PHP project not detected")
	return false
}

func (d *PHP) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(phpTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template")
	}

	// Parse version from go.mod
	version, err := findPHPVersion(path, d.Log)
	if err != nil {
		return nil, err
	}

	startCMD := "apache2-foreground"
	// A framework's front controller is public/index.php: the code beside it is not
	// to be served
	documentRoot := "/var/www/html"
	if _, err := os.Stat(filepath.Join(path, "public", "index.php")); err == nil {
		d.Log.Info("Detected public/index.php: serving public/")
		documentRoot = "/var/www/html/public"
	}
	installCMD := ""
	if _, err := os.Stat(filepath.Join(path, "composer.json")); err == nil {
		d.Log.Info("Detected composer.json file")
		// The build's image is not the one the app runs on: its extensions are
		// the runtime image's business, and so is its PHP when composer.lock
		// fixed the packages, which composer's platform check verifies as the
		// app starts
		ignore := "'--ignore-platform-req=ext-*'"
		if _, err := os.Stat(filepath.Join(path, "composer.lock")); err == nil {
			ignore = "--ignore-platform-reqs"
		}
		installCMD = "composer install --prefer-dist --no-dev --optimize-autoloader --no-interaction " + ignore
	}

	packageManager := ""
	npmInstallCMD := ""
	if _, err := os.Stat(filepath.Join(path, "package-lock.json")); err == nil {
		packageManager = "npm"
		npmInstallCMD = "npm ci"
	} else if _, err := os.Stat(filepath.Join(path, "pnpm-lock.yaml")); err == nil {
		packageManager = "pnpm"
		npmInstallCMD = "corepack enable pnpm && pnpm i --frozen-lockfile"
	} else if _, err := os.Stat(filepath.Join(path, "yarn.lock")); err == nil {
		packageManager = "yarn"
		npmInstallCMD = "yarn --frozen-lockfile"
	} else if hasBunLock(path) {
		// The build's image has Node.js, not Bun
		packageManager = "bun"
		npmInstallCMD = "npm i -g bun && bun install"
	}

	if npmInstallCMD != "" {
		d.Log.Info("Detected package-lock.json, pnpm-lock.yaml, yarn.lock, bun.lock or bun.lockb file")
		if installCMD == "" {
			installCMD = npmInstallCMD
		} else {
			installCMD = fmt.Sprintf("%s && %s", installCMD, npmInstallCMD)
		}
	}

	buildCMD := ""
	if packageManager != "" {
		f, err := os.Open(filepath.Join(path, "package.json"))
		if err != nil {
			return nil, fmt.Errorf("Failed to open package.json file")
		}

		defer f.Close()

		var packageJSON map[string]interface{}
		if err := json.NewDecoder(f).Decode(&packageJSON); err != nil {
			return nil, fmt.Errorf("Failed to decode package.json file")
		}

		scripts, ok := packageJSON["scripts"].(map[string]interface{})
		if ok {
			d.Log.Info("Detected scripts in package.json")
			buildCommands := []string{"build:prod", "build:production", "build-prod", "build-production", "build"}
			for _, cmd := range buildCommands {
				if _, ok := scripts[cmd].(string); ok {
					corepack := ""
					if packageManager == "pnpm" {
						corepack = "corepack enable pnpm &&"
					}
					buildCMD = fmt.Sprintf("%s%s run %s", corepack, packageManager, cmd)
					d.Log.Info("Detected build command in package.json: " + buildCMD)
					break
				}
			}
		}

		f.Close()
	}

	d.Log.Info(
		fmt.Sprintf(`Detected defaults 
  PHP version     : %s
  Install command : %s
  Build command   : %s
  Start command   : %s

  Docker build arguments can supersede these defaults if provided.
  See https://github.com/hivepaas/dockerfile-generator#runtime-documentation`, *version, installCMD, buildCMD, startCMD),
	)

	var buf bytes.Buffer
	templateData := map[string]string{
		"Version":      *version,
		"DocumentRoot": documentRoot,
		"Extensions":   strings.Join(phpExtensions(path, d.Log), " "),
		"InstallCMD":   safeCommand(installCMD),
		"BuildCMD":     safeCommand(buildCMD),
		"StartCMD":     safeCommand(startCMD),
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}
	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template")
	}

	return buf.Bytes(), nil
}

var phpTemplate = strings.TrimSpace(`
ARG PHP_VERSION={{.Version}}
ARG BUILDER=docker.io/library/composer
ARG RUNNER=docker.io/library/php:${PHP_VERSION}-apache
FROM ${BUILDER}:lts as build
RUN apk add --no-cache nodejs npm
# pnpm and Yarn run through Corepack, which Alpine's Node.js does not include
RUN npm install -g --no-fund --no-audit corepack && corepack enable
WORKDIR /app
COPY . .

ARG INSTALL_CMD={{.InstallCMD}}
ARG BUILD_CMD={{.BuildCMD}}
RUN --mount=type=cache,target=/root/.composer/cache \
    {{.InstallMounts}}if [ ! -z "${INSTALL_CMD}" ]; then sh -c "$INSTALL_CMD"; fi
RUN {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; fi

FROM ${RUNNER} AS runtime

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
# PHP's production settings: errors are logged, not shown to the visitor
RUN cp "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"
# PHP's extensions: the database drivers, OPcache, and those composer.json and
# composer.lock require. One the image has already is left as it is.
ARG PHP_EXTENSIONS="{{.Extensions}}"
COPY --from=docker.io/mlocati/php-extension-installer:2 /usr/bin/install-php-extensions /usr/local/bin/
RUN if [ -n "${PHP_EXTENSIONS}" ]; then install-php-extensions ${PHP_EXTENSIONS}; fi

ENV PORT=8080
EXPOSE ${PORT}
RUN sed -i "s/80/${PORT}/g" /etc/apache2/sites-available/000-default.conf /etc/apache2/ports.conf
ARG DOCUMENT_ROOT={{.DocumentRoot}}
ENV APACHE_DOCUMENT_ROOT=${DOCUMENT_ROOT}
# The site is served from APACHE_DOCUMENT_ROOT, with .htaccess's rewrites; dot
# files (.env), Composer's files and vendor/ are not served
RUN sed -i 's!/var/www/html!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/sites-available/000-default.conf \
    && a2enmod rewrite \
    && printf '%s\n' \
        'RedirectMatch 404 "/\.(?!well-known/)"' \
        '<Directory /var/www/html/vendor>' '    Require all denied' '</Directory>' \
        '<FilesMatch "^(composer\.(json|lock)|auth\.json)$">' '    Require all denied' '</FilesMatch>' \
        > /etc/apache2/conf-enabled/hivepaas-hidden.conf
COPY --from=build --chown=nonroot:nonroot /app /var/www/html

# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

ARG START_CMD={{.StartCMD}}
ENV START_CMD=${START_CMD}
RUN if [ -z "${START_CMD}" ]; then echo "Unable to detect a container start command" && exit 1; fi
CMD ${START_CMD}
`)

func findPHPVersion(path string, log *slog.Logger) (*string, error) {
	version := ""
	versionFiles := []string{
		".tool-versions",
		"composer.json",
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
					if strings.Contains(line, "php") {
						version = strings.Split(line, " ")[1]
						log.Info("Detected PHP version in .tool-versions: " + version)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read .tool-versions file")
				}

			case "composer.json":
				var composerJSON map[string]interface{}
				err := json.NewDecoder(f).Decode(&composerJSON)
				if err != nil {
					return nil, fmt.Errorf("Failed to read composer.json file")
				}
				version = composerPHPVersion(path, composerJSON, log)
			}

			f.Close()
			if version != "" {
				break
			}
		}
	}

	if version == "" {
		version = "8.3"
		log.Info(fmt.Sprintf("No PHP version detected. Using: %s", version))
	}

	return &version, nil
}

// phpDefaultExtensions are installed for every PHP app: its database drivers,
// which no package requires, and OPcache.
var phpDefaultExtensions = []string{"opcache", "pdo_mysql", "pdo_pgsql"}

// phpCoreExtensions are part of PHP itself, never installed.
var phpCoreExtensions = map[string]bool{
	"core": true, "date": true, "hash": true, "json": true, "libxml": true, "pcre": true,
	"random": true, "reflection": true, "spl": true, "standard": true,
}

var phpExtensionNameRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// phpExtensions are the extensions an app needs: the defaults, and the ext-*
// that composer.json and composer.lock's packages require.
func phpExtensions(path string, log *slog.Logger) []string {
	required := map[string]bool{}
	collect := func(require map[string]interface{}) {
		for name := range require {
			ext, ok := strings.CutPrefix(strings.ToLower(name), "ext-")
			if !ok {
				continue
			}
			ext = strings.ReplaceAll(ext, "-", "_")
			if ext == "zend_opcache" {
				ext = "opcache"
			}
			if phpExtensionNameRe.MatchString(ext) && !phpCoreExtensions[ext] {
				required[ext] = true
			}
		}
	}

	var composerJSON struct {
		Require map[string]interface{} `json:"require"`
	}
	if content, err := os.ReadFile(filepath.Join(path, "composer.json")); err == nil {
		if json.Unmarshal(content, &composerJSON) == nil {
			collect(composerJSON.Require)
		}
	}
	var lock struct {
		Packages []struct {
			Require map[string]interface{} `json:"require"`
		} `json:"packages"`
	}
	if content, err := os.ReadFile(filepath.Join(path, "composer.lock")); err == nil {
		if json.Unmarshal(content, &lock) == nil {
			for _, pkg := range lock.Packages {
				collect(pkg.Require)
			}
		}
	}

	for _, ext := range phpDefaultExtensions {
		required[ext] = true
	}
	extensions := slices.Sorted(maps.Keys(required))
	log.Info("PHP extensions: " + strings.Join(extensions, " "))
	return extensions
}

// hasBunLock tells whether a project has Bun's lockfile: bun.lock, or bun.lockb
// before Bun 1.2.
func hasBunLock(path string) bool {
	for _, name := range []string{"bun.lock", "bun.lockb"} {
		if _, err := os.Stat(filepath.Join(path, name)); err == nil {
			return true
		}
	}
	return false
}

// phpLatestMajor is the newest major PHP has released: a constraint that
// allows it takes its newest minor, the image's "8".
const phpLatestMajor = 8

// composerPHPVersion is the PHP that composer.json and composer.lock run on:
// config.platform.php, the version the project resolves its dependencies for,
// or else the newest that composer.json and every locked package allow. A
// lock written on a newer PHP than composer.json's minimum often needs it.
func composerPHPVersion(path string, composerJSON map[string]interface{}, log *slog.Logger) string {
	if config, ok := composerJSON["config"].(map[string]interface{}); ok {
		if platform, ok := config["platform"].(map[string]interface{}); ok {
			if php, ok := platform["php"].(string); ok {
				if v, err := semver.NewVersion(php); err == nil {
					log.Info("Detected PHP version in composer.json's config.platform: " + php)
					return fmt.Sprintf("%d.%d", v.Major(), v.Minor())
				}
			}
		}
	}

	var constraints []*semver.Constraints
	if require, ok := composerJSON["require"].(map[string]interface{}); ok {
		if php, ok := require["php"].(string); ok {
			if c, err := composerConstraint(php); err == nil {
				constraints = append(constraints, c)
			}
		}
	}
	if len(constraints) == 0 {
		return ""
	}
	locked := lockedPHPConstraints(path)
	constraints = append(constraints, locked...)

	tag, ok := newestAllowed(func(v *semver.Version) bool {
		for _, c := range constraints {
			if !c.Check(v) {
				return false
			}
		}
		return true
	}, phpLatestMajor)
	if !ok {
		log.Warn("No PHP version satisfies composer.json and composer.lock together")
		return ""
	}
	log.Info(fmt.Sprintf("Detected PHP version from composer.json and %d locked packages: %s", len(locked), tag))
	return tag
}

// lockedPHPConstraints are the PHP versions composer.lock's packages require -
// not its development ones, which are not installed.
func lockedPHPConstraints(path string) []*semver.Constraints {
	content, err := os.ReadFile(filepath.Join(path, "composer.lock"))
	if err != nil {
		return nil
	}
	var lock struct {
		Packages []struct {
			Require map[string]string `json:"require"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(content, &lock); err != nil {
		return nil
	}
	var constraints []*semver.Constraints
	for _, pkg := range lock.Packages {
		if php, ok := pkg.Require["php"]; ok {
			if c, err := composerConstraint(php); err == nil {
				constraints = append(constraints, c)
			}
		}
	}
	return constraints
}

var gteVersionRe = regexp.MustCompile(`^>=\s*([\d.]+)`)
var rangeVersionRe = regexp.MustCompile(`^([\d.]+)\s*-\s*([\d.]+)`)
var tildeVersionRe = regexp.MustCompile(`^~\s*([\d.]+)`)
var caretVersionRe = regexp.MustCompile(`^\^([\d.]+)`)
var exactVersionRe = regexp.MustCompile(`^([\d.]+)`)
