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

type Astro struct {
	Log *slog.Logger
}

func (d *Astro) Name() RuntimeName {
	return RuntimeNameAstro
}

func (d *Astro) Match(path string) bool {
	checkPaths := []string{
		filepath.Join(path, "astro.config.js"),
		filepath.Join(path, "astro.config.ts"),
		filepath.Join(path, "astro.config.mjs"),
		filepath.Join(path, "astro.config.mts"),
		filepath.Join(path, "astro.config.cjs"),
	}

	for _, p := range checkPaths {
		if _, err := os.Stat(p); err == nil {
			d.Log.Info("Detected Astro project")
			return true
		}
	}

	d.Log.Debug("Astro project not detected")
	return false
}

func (d *Astro) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	// Detect the Astro adapter/output mode from config file
	adapter := detectAstroAdapter(path, d.Log)

	var astroTmpl string
	switch adapter {
	case "node":
		d.Log.Info("Detected Astro Node.js SSR adapter")
		astroTmpl = astroNodeTemplate
	case "static":
		d.Log.Info("Detected Astro static output")
		astroTmpl = astroStaticTemplate
	default:
		d.Log.Info("No Astro adapter detected, using Node.js SSR template")
		astroTmpl = astroNodeTemplate
	}

	tmpl, err := template.New("Dockerfile").Parse(astroTmpl)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template: %w", err)
	}

	version, err := findNodeVersion(path, d.Log)
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

// detectAstroAdapter reads the astro config to identify whether this project
// uses @astrojs/node (SSR), or is a purely static site ("static" output).
func detectAstroAdapter(path string, log *slog.Logger) string {
	configFiles := []string{
		"astro.config.mjs",
		"astro.config.ts",
		"astro.config.js",
		"astro.config.mts",
		"astro.config.cjs",
	}

	for _, cf := range configFiles {
		fp := filepath.Join(path, cf)
		f, err := os.Open(fp)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			// Detect @astrojs/node adapter
			if strings.Contains(line, "@astrojs/node") {
				return "node"
			}
			// Detect output: 'static' or output: "static"
			if strings.Contains(line, "output") && strings.Contains(line, "static") {
				return "static"
			}
		}
	}

	// Check package.json for @astrojs/node or @astrojs/netlify etc.
	packageJSONPath := filepath.Join(path, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		content := string(data)
		if strings.Contains(content, "@astrojs/node") {
			return "node"
		}
	}

	return "node" // default to SSR node
}

// astroNodeTemplate: for Astro projects using @astrojs/node adapter (SSR mode)
var astroNodeTemplate = strings.TrimSpace(`
ARG NODE_VERSION={{.Version}}
ARG BUILDER=docker.io/library/node
ARG RUNNER=${BUILDER}:${NODE_VERSION}-slim
FROM ${RUNNER} AS base
RUN corepack enable

FROM base AS deps
WORKDIR /app
COPY package.json yarn.lock* package-lock.json* pnpm-lock.yaml* bun.lockb* ./
ARG INSTALL_CMD=
RUN --mount=type=cache,target=/root/.npm \
    --mount=type=cache,target=/root/.local/share/pnpm/store \
    --mount=type=cache,target=/usr/local/share/.cache/yarn \
    {{.InstallMounts}}if [ ! -z "${INSTALL_CMD}" ]; then sh -c "$INSTALL_CMD"; \
  elif [ -f yarn.lock ]; then yarn --frozen-lockfile; \
  elif [ -f package-lock.json ]; then npm ci; \
  elif [ -f bun.lockb ]; then npm i -g bun && bun install; \
  elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm i --frozen-lockfile; \
  else npm ci; fi

FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG BUILD_CMD=
RUN --mount=type=cache,target=/root/.astro \
    {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; \
  elif [ -f yarn.lock ]; then yarn build; \
  elif [ -f package-lock.json ]; then npm run build; \
  elif [ -f bun.lockb ]; then npm i -g bun && bun run build; \
  elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm run build; \
  else npm run build; fi

FROM base AS runtime
WORKDIR /app

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
ENV COREPACK_HOME=/app/.cache
RUN mkdir -p /app/.cache
RUN chown -R nonroot:nonroot /app

COPY --chown=nonroot:nonroot --from=builder /app/dist ./dist
COPY --chown=nonroot:nonroot --from=deps /app/node_modules ./node_modules
COPY --chown=nonroot:nonroot --from=builder /app/package.json ./

# To run as root instead, pass build arg: --build-arg USER=root
ARG USER=nonroot:nonroot
USER ${USER}

ENV HOST=0.0.0.0
ENV PORT=8080
EXPOSE ${PORT}

ENV NODE_ENV=production
CMD ["node", "./dist/server/entry.mjs"]
`)

// astroStaticTemplate: for Astro projects with output: "static" (pure SSG)
var astroStaticTemplate = strings.TrimSpace(`
ARG NODE_VERSION={{.Version}}
ARG BUILDER=docker.io/library/node
ARG RUNNER=docker.io/joseluisq/static-web-server:2-debian
FROM ${BUILDER}:${NODE_VERSION}-slim AS deps
RUN corepack enable
WORKDIR /app
COPY package.json yarn.lock* package-lock.json* pnpm-lock.yaml* bun.lockb* ./
ARG INSTALL_CMD=
RUN --mount=type=cache,target=/root/.npm \
    --mount=type=cache,target=/root/.local/share/pnpm/store \
    --mount=type=cache,target=/usr/local/share/.cache/yarn \
    {{.InstallMounts}}if [ ! -z "${INSTALL_CMD}" ]; then sh -c "$INSTALL_CMD"; \
  elif [ -f yarn.lock ]; then yarn --frozen-lockfile; \
  elif [ -f package-lock.json ]; then npm ci; \
  elif [ -f bun.lockb ]; then npm i -g bun && bun install; \
  elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm i --frozen-lockfile; \
  else npm ci; fi

FROM deps AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG BUILD_CMD=
RUN --mount=type=cache,target=/root/.astro \
    {{.BuildMounts}}if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; \
  elif [ -f yarn.lock ]; then yarn build; \
  elif [ -f package-lock.json ]; then npm run build; \
  elif [ -f bun.lockb ]; then npm i -g bun && bun run build; \
  elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm run build; \
  else npm run build; fi

FROM ${RUNNER} AS runtime

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*

COPY --from=builder /app/dist /public

ENV SERVER_PORT=8080
ENV SERVER_ROOT=/public
EXPOSE 8080
`)
