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

type Python struct {
	Log *slog.Logger
}

func (d *Python) Name() RuntimeName {
	return RuntimeNamePython
}

func (d *Python) Match(path string) bool {
	checkPaths := []string{
		filepath.Join(path, "requirements.txt"),
		filepath.Join(path, "poetry.lock"),
		filepath.Join(path, "uv.lock"),
		filepath.Join(path, "Pipfile.lock"),
		filepath.Join(path, "pyproject.toml"),
		filepath.Join(path, "pdm.lock"),
		filepath.Join(path, "main.py"),
		filepath.Join(path, "app.py"),
		filepath.Join(path, "application.py"),
		filepath.Join(path, "app/__init__.py"),
		filepath.Join(path, filepath.Base(path), "app.py"),
		filepath.Join(path, filepath.Base(path), "application.py"),
		filepath.Join(path, filepath.Base(path), "main.py"),
		filepath.Join(path, filepath.Base(path), "__init__.py"),
	}

	for _, p := range checkPaths {
		if _, err := os.Stat(p); err == nil {
			d.Log.Info("Detected Python project")
			return true
		}
	}

	d.Log.Debug("Python project not detected")
	return false
}

func (d *Python) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	tmpl, err := template.New("Dockerfile").Parse(pythonTemplate)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template")
	}

	// Parse version from go.mod
	version, err := findPythonVersion(path, d.Log)
	if err != nil {
		return nil, err
	}

	installCMD := ""
	packageManager := PythonPackageManagerPip
	if _, err := os.Stat(filepath.Join(path, "requirements.txt")); err == nil {
		d.Log.Info("Detected requirements.txt file")
		installCMD = "pip install --no-cache -r requirements.txt"
	} else if _, err := os.Stat(filepath.Join(path, "uv.lock")); err == nil {
		d.Log.Info("Detected a uv project")
		installCMD = "pip install uv && uv sync --python-preference=only-system --no-cache --no-dev"
		packageManager = PythonPackageManagerUv
	} else if _, err := os.Stat(filepath.Join(path, "poetry.lock")); err == nil {
		d.Log.Info("Detected a poetry project")
		installCMD = "pip install poetry && poetry install --only main --no-ansi --no-root"
		packageManager = PythonPackageManagerPoetry
	} else if _, err := os.Stat(filepath.Join(path, "Pipfile.lock")); err == nil {
		d.Log.Info("Detected a pipenv project")
		installCMD = "pip install pipenv && pipenv install --system --deploy"
		packageManager = PythonPackageManagerPipenv
	} else if _, err := os.Stat(filepath.Join(path, "pdm.lock")); err == nil {
		d.Log.Info("Detected a pdm project")
		installCMD = "pip install pdm && pdm install --prod"
		packageManager = PythonPackageManagerPdm
	} else if _, err := os.Stat(filepath.Join(path, "pyproject.toml")); err == nil {
		d.Log.Info("Detected a pyproject.toml file")
		installCMD = "pip install --upgrade build setuptools && pip install ."
	}

	startCMD := pythonStartCommand(path, d.Log)

	packagerInstructions := ""
	switch packageManager {
	case PythonPackageManagerPoetry:
		packagerInstructions = poetryInstructions
	case PythonPackageManagerUv:
		packagerInstructions = uvInstructions
	case PythonPackageManagerPdm:
		packagerInstructions = pdmInstructions
	}

	d.Log.Info(
		fmt.Sprintf(`Detected defaults 
  Python version       : %s
  Install command      : %s
  Start command        : %s

  Docker build arguments can supersede these defaults if provided.
  See https://github.com/hivepaas/dockerfile-generator#runtime-documentation`, *version, installCMD, startCMD),
	)

	var buf bytes.Buffer
	templateData := map[string]string{
		"Version":              *version,
		"InstallCMD":           safeCommand(installCMD),
		"StartCMD":             safeCommand(startCMD),
		"PackagerInstructions": packagerInstructions,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}
	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template")
	}

	return buf.Bytes(), nil
}

var pythonTemplate = strings.TrimSpace(`
ARG PYTHON_VERSION={{.Version}}
ARG BUILDER=docker.io/library/python
ARG RUNNER=${BUILDER}:${PYTHON_VERSION}-slim
FROM ${RUNNER}
WORKDIR /app
ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
RUN chown -R nonroot:nonroot /app
RUN mkdir -p /var/cache
RUN chown -R nonroot:nonroot /var/cache

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
{{ .PackagerInstructions }}

COPY --chown=nonroot:nonroot . .
ARG INSTALL_CMD={{.InstallCMD}}
RUN --mount=type=cache,target=/root/.cache/pip \
    --mount=type=cache,target=/var/cache/pypoetry \
    --mount=type=cache,target=/var/cache/uv \
    if [ ! -z "${INSTALL_CMD}" ]; then sh -c "$INSTALL_CMD";  fi

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

var poetryInstructions = `
ENV POETRY_NO_INTERACTION=1
ENV POETRY_VIRTUALENVS_CREATE=false
ENV POETRY_CACHE_DIR='/var/cache/pypoetry'
ENV POETRY_HOME='/usr/local'`

var pdmInstructions = `
ENV PDM_CHECK_UPDATE=false
# pdm installs into the project's virtual environment
ENV VIRTUAL_ENV=/app/.venv
ENV PATH="/app/.venv/bin:$PATH"`

var uvInstructions = `
# Set the UV_CACHE_DIR environment variable to a directory where uv will store its cache
ENV UV_CACHE_DIR='/var/cache/uv'
# Use the virtual environment automatically
ENV VIRTUAL_ENV=/app/.venv
ENV PATH="/app/.venv/bin:$PATH"`

func findPythonVersion(path string, log *slog.Logger) (*string, error) {
	version := ""
	versionFiles := []string{
		".tool-versions",
		".python-version",
		".mise.toml",
		"runtime.txt",
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
					if strings.Contains(line, "python") {
						version = strings.Split(line, " ")[1]
						log.Info("Detected Python version in .tool-versions: " + version)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read .tool-versions file")
				}

			case ".python-version":
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := scanner.Text()
					if line != "" {
						version = line
						log.Info("Detected Python version from .python-version: " + version)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read .python-version file")
				}

			case "runtime.txt":
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "python-") {
						version = strings.TrimPrefix(line, "python-")
						log.Info("Detected Python version from runtime.txt: " + version)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read runtime.txt file")
				}

			case ".mise.toml":
				var mise MiseToml
				if err := toml.NewDecoder(f).Decode(&mise); err != nil {
					return nil, fmt.Errorf("Failed to decode .mise.toml file")
				}
				pythonVersion, ok := mise.Tools["python"].(string)
				if !ok {
					versions, ok := mise.Tools["python"].([]string)
					if ok {
						pythonVersion = versions[0]
					}
				}
				if pythonVersion != "" {
					version = pythonVersion
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
		version = "3.12"
		log.Info(fmt.Sprintf("No Python version detected. Using %s.", version))
	}

	return &version, nil
}

func isDjangoProject(path string) *string {
	manageFiles := []string{"manage.py", "app/manage.py", filepath.Join(filepath.Base(path), "manage.py")}
	var managePy *string
	for _, file := range manageFiles {
		_, err := os.Stat(filepath.Join(path, file))
		if err == nil {
			managePy = &file
			break
		}
	}

	if managePy == nil {
		return nil
	}

	packagerFiles := []string{"requirements.txt", "pyproject.toml", "Pipfile"}

	for _, file := range packagerFiles {
		_, err := os.Stat(filepath.Join(path, file))
		if err == nil {
			f, err := os.Open(filepath.Join(path, file))
			if err != nil {
				return nil
			}

			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(strings.ToLower(line), "django") {
					return managePy
				}
			}

			f.Close()
		}
	}

	return nil
}

func isFastAPIProject(path string) bool {
	packagerFiles := []string{"requirements.txt", "pyproject.toml", "Pipfile"}

	for _, file := range packagerFiles {
		_, err := os.Stat(filepath.Join(path, file))
		if err == nil {
			f, err := os.Open(filepath.Join(path, file))
			if err != nil {
				return false
			}

			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(strings.ToLower(line), "fastapi") {
					return true
				}
			}

			f.Close()
		}
	}

	return false
}

type PythonPackageManager string

const (
	PythonPackageManagerPip    PythonPackageManager = "pip"
	PythonPackageManagerPoetry PythonPackageManager = "poetry"
	PythonPackageManagerUv     PythonPackageManager = "uv"
	PythonPackageManagerPipenv PythonPackageManager = "pipenv"
	PythonPackageManagerPdm    PythonPackageManager = "pdm"
)

// pythonStartCommand is the command that starts a Python app: Django's,
// FastAPI's or Flask's server, the package pyproject.toml names when it runs
// as a module, or the main file.
func pythonStartCommand(path string, log *slog.Logger) string {
	if managePy := isDjangoProject(path); managePy != nil {
		log.Info("Detected Django project")
		if pythonDependsOn(path, "gunicorn") {
			if wsgi := djangoWSGIModule(path, *managePy); wsgi != "" {
				chdir := ""
				if dir := filepath.Dir(*managePy); dir != "." {
					chdir = "--chdir " + filepath.ToSlash(dir) + " "
				}
				return "gunicorn " + chdir + wsgi + " --bind 0.0.0.0:${PORT}"
			}
		}
		return "python " + *managePy + " runserver 0.0.0.0:${PORT}"
	}

	isFastAPI := isFastAPIProject(path)
	if !isFastAPI {
		if cmd := pyprojectModuleCommand(path); cmd != "" {
			log.Info("Detected start command via pyproject.toml: " + cmd)
			return cmd
		}
	}

	base := filepath.Base(path)
	mainFiles := []string{
		"main.py",
		"app.py",
		"application.py",
		"app/main.py",
		"app/__init__.py",
		filepath.Join(base, "main.py"),
		filepath.Join(base, "app.py"),
		filepath.Join(base, "application.py"),
		filepath.Join(base, "__init__.py"),
	}
	for _, fn := range mainFiles {
		if _, err := os.Stat(filepath.Join(path, fn)); err != nil {
			continue
		}
		fn = filepath.ToSlash(fn)
		module := strings.TrimSuffix(strings.TrimSuffix(fn, ".py"), "/__init__")
		module = strings.ReplaceAll(module, "/", ".")
		var cmd string
		switch {
		case isFastAPI && !pythonDependsOnFastAPICLI(path) && pythonDependsOn(path, "uvicorn"):
			cmd = "uvicorn " + module + ":app --host 0.0.0.0 --port ${PORT}"
		case isFastAPI:
			cmd = "fastapi run " + fn + " --port ${PORT}"
		case pythonDependsOn(path, "flask") && pythonDependsOn(path, "gunicorn"):
			cmd = "gunicorn " + module + ":app --bind 0.0.0.0:${PORT}"
		default:
			cmd = "python " + fn
		}
		log.Info("Detected start command via main file: " + cmd)
		return cmd
	}
	return ""
}

// pyprojectModuleCommand runs the package pyproject.toml names as a module,
// when it is one: name/__main__.py, or src/name/__main__.py.
func pyprojectModuleCommand(path string) string {
	content, err := os.ReadFile(filepath.Join(path, "pyproject.toml"))
	if err != nil {
		return ""
	}
	var pyproject struct {
		Project struct {
			Name string `toml:"name"`
		} `toml:"project"`
		Tool struct {
			Poetry struct {
				Name string `toml:"name"`
			} `toml:"poetry"`
		} `toml:"tool"`
	}
	if err := toml.Unmarshal(content, &pyproject); err != nil {
		return ""
	}
	name := pyproject.Project.Name
	if name == "" {
		name = pyproject.Tool.Poetry.Name
	}
	module := strings.ToLower(strings.NewReplacer("-", "_", ".", "_").Replace(name))
	if module == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(path, module, "__main__.py")); err == nil {
		return "python -m " + module
	}
	if _, err := os.Stat(filepath.Join(path, "src", module, "__main__.py")); err == nil {
		return "PYTHONPATH=src python -m " + module
	}
	return ""
}

var djangoSettingsRe = regexp.MustCompile(`DJANGO_SETTINGS_MODULE["']\s*,\s*["']([\w.]+)\.settings["']`)

// djangoWSGIModule is the WSGI module beside the settings manage.py names:
// mysite.settings has mysite.wsgi.
func djangoWSGIModule(path, managePy string) string {
	content, err := os.ReadFile(filepath.Join(path, managePy))
	if err != nil {
		return ""
	}
	m := djangoSettingsRe.FindSubmatch(content)
	if m == nil {
		return ""
	}
	return string(m[1]) + ".wsgi"
}

// pythonDependsOn says whether requirements.txt, pyproject.toml or Pipfile
// names the package: as a name of its own, not a part of another.
func pythonDependsOn(path, name string) bool {
	re := regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_.-])` + regexp.QuoteMeta(name) + `([^A-Za-z0-9_.-]|$)`)
	for _, file := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		if content, err := os.ReadFile(filepath.Join(path, file)); err == nil && re.Match(content) {
			return true
		}
	}
	return false
}

var fastAPICLIRe = regexp.MustCompile(`(?i)fastapi\[[^\]]*standard|fastapi-cli`)

// pythonDependsOnFastAPICLI says whether the project has the fastapi command:
// fastapi[standard], or fastapi-cli.
func pythonDependsOnFastAPICLI(path string) bool {
	for _, file := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		if content, err := os.ReadFile(filepath.Join(path, file)); err == nil && fastAPICLIRe.Match(content) {
			return true
		}
	}
	return false
}
