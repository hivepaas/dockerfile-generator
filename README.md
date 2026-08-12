# Dockerfile Generator

> [!NOTE]
> This project is based on and derived from [flexstack/new-dockerfile](https://github.com/flexstack/new-dockerfile) by FlexStack, Inc. (licensed under the MIT License), maintained and customized by [HivePaaS](https://github.com/hivepaas) for standalone Dockerfile generation.

`dockerfile-generator` is a CLI tool and Go package that automatically generates a configurable, production-ready Dockerfile based on your project's source code. It supports a wide range of languages and frameworks, including Next.js, Node.js, Python, Ruby, Java/Spring Boot, Go, Elixir/Phoenix, Rust, PHP, and more.

## Features

- [x] Automatically detect the runtime and framework used by your project
- [x] Use version managers like [asdf](https://github.com/asdf-vm), nvm, rbenv, and pyenv to install the correct version of the runtime
- [x] Make a best effort to detect any install, build, and start commands
- [x] Generate a Dockerfile with sensible defaults that are configurable via [Docker Build Args](https://docs.docker.com/build/guide/build-args/)
- [x] Support for a wide range of the most popular languages and frameworks including Next.js, Phoenix, Spring Boot, Django, and more
- [x] Use Debian Slim (and Alpine for Go) as the runtime image for optimal image size, performance, and security
- [x] Includes `wget` in the runtime image for adding health checks to services, e.g. `wget -nv -t1 --spider 'http://localhost:8080/healthz' || exit 1`
- [x] Includes `ca-certificates` in the runtime image to allow secure HTTPS connections
- [x] Use multi-stage builds to reduce the size of the final image
- [x] Run the application as a non-root user for better security
- [x] Supports multi-platform images that run on both x86 and ARM CPU architectures

## Supported Runtimes

- [Bun](#bun)
- [C / C++](#c--c)
- [C# / .NET](#c--net)
- [Dart](#dart)
- [Deno](#deno)
- [Elixir](#elixir)
- [Go](#go)
- [Java](#java)
- [Next.js](#nextjs)
- [Node.js](#nodejs)
- [PHP](#php)
- [Python](#python)
- [Ruby](#ruby)
- [Rust](#rust)
- [Scala](#scala)
- [Static](#static-html-css-js) (HTML, CSS, JS)
- [Zig](#zig)

## Installation

### CLI Tool

```sh
go install github.com/hivepaas/dockerfile-generator/cmd/dockerfile-gen@latest
```

### Go Package

```sh
go get github.com/hivepaas/dockerfile-generator
```

## CLI Usage

```sh
dockerfile-gen [options]
```

## CLI Options

- `--path` - Path to the project source code (default: `.`)
- `--write` - Write the generated Dockerfile to the project at the specified path (default: `false`)
- `--runtime` - Force a specific runtime, e.g. `node` (default: `auto`)
- `--quiet` - Disable all logging except for errors (default: `false`)
- `--help` - Show help

## CLI Examples

Print the generated Dockerfile to the console:
```sh
dockerfile-gen
```

Write a Dockerfile to the current directory:
```sh
dockerfile-gen --write
```

Write a Dockerfile to a specific directory:
```sh
dockerfile-gen > path/to/Dockerfile
```

Force a specific runtime:
```sh
dockerfile-gen --runtime next.js
```

List the supported runtimes:
```sh
dockerfile-gen --runtime list
```

## Read from Config file

In the CI use case, you might need a very common step for generating a `Dockerfile`. You can create a config file for the
CLI options. The default config file name is `dockerfile-gen.yaml`, and it should be in the root directory of your git
repository. Especially, there are multiple kinds of files, `dockerfile-gen` might not be able to it a correct one.

```yaml
runtime: go
```

And, the CLI option will overwrite the values from config file.

## How it Works

The tool searches for common files and directories in your project to determine the runtime and framework.
For example, if it finds a `package.json` file, it will assume the project is a Node.js project unless
a `next.config.js` file is present, in which case it will assume the project is a Next.js project.

From there, it will read any `.tool-versions` or other version manager files to determine the correct version
of the runtime to install. It will then make a best effort to detect any install, build, and start commands.
For example, a `serve`, `start`, `start:prod` command in a `package.json` file will be used as the start command.

Runtimes are matched against in the order they appear when you run `dockerfile-gen --runtime list`.

Read on to see runtime-specific examples and how to configure the generated Dockerfile.

## Runtime Documentation

### Bun

[Bun](https://bun.sh/) is a fast JavaScript all-in-one toolkit

#### Detected Files
  - `bun.lockb`
  - `bun.lock`
  - `bunfig.toml`

#### Version Detection
  - `.tool-versions` - `bun {VERSION}`
  - `.mise.toml` - `bun = "{VERSION}"`

#### Runtime Image
`oven/bun:${BUN_VERSION}-slim`

#### Build Args
  - `BUN_VERSION` - The version of Bun to install (default: `1`)
  - `INSTALL_CMD` - The command to install dependencies (default: `bun install`)
  - `BUILD_CMD` - The command to build the project (default: detected from `package.json`)
  - `START_CMD` - The command to start the project (default: detected from `package.json`)
  - `RUNNER` - The base runtime image (default: `docker.io/oven/bun:${BUN_VERSION}-slim`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Build Command

Detected in order of precedence:
  - `package.json` scripts: `"build:prod", "build:production", "build-prod", "build-production", "build"`

#### Start Command

Detected in order of precedence:
  - `package.json` scripts: `"serve", "start:prod", "start:production", "start-prod", "start-production", "preview", "start"`
  - `package.json` main/module file: `bun run ${mainFile}`

---

### C / C++

[C/C++](https://isocpp.org/) is a powerful, high-performance general-purpose programming language.

#### Detected Files
  - `CMakeLists.txt`
  - `Makefile`, `makefile`, `GNUmakefile`
  - `meson.build`
  - `main.cpp`, `main.cc`, `main.cxx`, `main.c`, `app.cpp`, `app.c`

#### Runtime Image
`alpine:latest`

#### Build Args
  - `BUILD_CMD` - The command to compile the executable (default: auto-detected from `CMakeLists.txt`, `Makefile`, or source files)
  - `RUNNER` - The base runtime image (default: `docker.io/library/alpine:latest`)
  - `APK_EXTRA_PKGS` - Extra APK packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Build Command
Detected in order of precedence:
  - `CMakeLists.txt`: `cmake -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build && cp $(find build -maxdepth 2 -type f -executable | head -n 1) /app_bin`
  - `Makefile`: `make && cp $(find . -maxdepth 1 -type f -executable ! -name "*.sh" ! -name "Makefile*" | head -n 1) /app_bin`
  - `main.cpp`: `g++ -O3 -o /app_bin main.cpp`
  - `main.c`: `gcc -O3 -o /app_bin main.c`

#### Start Command
`["/app/app"]`

---

### C# / .NET

[.NET](https://dotnet.microsoft.com/) is a free, cross-platform, open-source developer platform for building many different types of applications.

#### Detected Files
  - `*.csproj`, `*.fsproj`, `*.vbproj`
  - `*.sln`
  - `global.json`
  - `Directory.Build.props`

#### Version Detection
  - `.tool-versions` - `dotnet {VERSION}` or `dotnet-core {VERSION}`
  - `.mise.toml` - `dotnet = "{VERSION}"`
  - `global.json` - `"sdk": { "version": "{VERSION}" }`
  - Project file: `<TargetFramework>net{VERSION}</TargetFramework>`

#### Runtime Image
`mcr.microsoft.com/dotnet/aspnet:${DOTNET_VERSION}-alpine`

#### Build Args
  - `DOTNET_VERSION` - The version of the .NET SDK / Runtime (default: `8.0`)
  - `RESTORE_CMD` - The command to restore NuGet packages (default: `dotnet restore`)
  - `BUILD_CMD` - The command to publish the project (default: `dotnet publish -c Release -o /app/publish`)
  - `START_CMD` - The command to start the application (default: `dotnet {DLL_NAME}.dll`)
  - `RUNNER` - The base runtime image (default: `mcr.microsoft.com/dotnet/aspnet:${DOTNET_VERSION}-alpine`)
  - `APK_EXTRA_PKGS` - Extra APK packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Build Command
`dotnet publish -c Release -o /app/publish`

#### Start Command
`dotnet {DLL_NAME}.dll`

---

### Dart

[Dart](https://dart.dev/) is an approachable, portable, and productive language for high-quality apps on any platform.

#### Detected Files
  - `pubspec.yaml`
  - `pubspec.yml`

#### Version Detection
  - `.tool-versions` - `dart {VERSION}`
  - `.mise.toml` - `dart = "{VERSION}"`
  - `pubspec.yaml` - `sdk: '{VERSION}'`

#### Runtime Image
`debian:stable-slim`

#### Build Args
  - `DART_VERSION` - The version of Dart to install (default: `stable`)
  - `INSTALL_CMD` - The command to install dependencies (default: `dart pub get`)
  - `BUILD_CMD` - The command to compile the executable (default: `dart compile exe {MAIN_FILE} -o /app/bin/server`)
  - `RUNNER` - The base runtime image (default: `docker.io/library/debian:stable-slim`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Build Command
`dart compile exe {MAIN_FILE} -o /app/bin/server`

#### Start Command
`["/app/bin/server"]`

---

### Deno

[Deno](https://deno.com/) is a secure runtime for JavaScript with native TypeScript and JSX support

#### Detected Files
  - `deno.jsonc`
  - `deno.json`
  - `deno.lock`
  - `deps.ts`
  - `mod.ts`

#### Version Detection
  - `.tool-versions` - `deno {VERSION}`
  - `.mise.toml` - `deno = "{VERSION}"`

#### Runtime Image
`debian:stable-slim`

#### Build Args
  - `DENO_VERSION` - The version of Deno to install (default: `latest`)
  - `INSTALL_CMD` - The command to install dependencies (default: detected from `deno.jsonc` and source code)
  - `START_CMD` - The command to start the project (default: detected from `deno.jsonc` and source code)
  - `RUNNER` - The base runtime image (default: `docker.io/library/debian:stable-slim`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Install Command

Detected in order of precedence:
  - `deno.jsonc` tasks: `"cache"`
  - Main/module file: `deno cache ["mod.ts", "src/mod.ts", "main.ts", "src/main.ts", "index.ts", "src/index.ts]"`

#### Start Command

Detected in order of precedence:
  - `deno.jsonc` tasks: `"serve", "start:prod", "start:production", "start-prod", "start-production", "preview", "start"`
  - Main/module file: `deno run ["mod.ts", "src/mod.ts", "main.ts", "src/main.ts", "index.ts", "src/index.ts]"`
  
---

### Elixir

[Elixir](https://elixir-lang.org/) is a dynamic, functional language designed for building scalable and maintainable applications.

#### Detected Files
  - `mix.exs`

#### Version Detection
  - `.tool-versions` - `elixir {VERSION}`
  - `.tool-versions` - `erlang {VERSION}`
  - `.elixir-version` - `{VERSION}`
  - `.erlang-version` - `{VERSION}`
  - `.mise.toml` - `erlang = "{VERSION}"`

#### Runtime Image
`debian:stable-slim`

#### Build Args
  - `ELIXIR_VERSION` - The version of Elixir to install (default: `1.17`)
  - `OTP_VERSION` - The version of Erlang to install (default: `26.2.5`)
  - `BIN_NAME` - The name of the release binary (default: detected via app name in `mix.exs`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Start Command
`/app/bin/{BIN_NAME} start`

---

### Go

[Go](https://golang.org/) is an open-source programming language that makes it easy to build simple, reliable, and efficient software.

#### Detected Files
  - `go.mod`
  - `main.go`

#### Version Detection
  - `.tool-versions` - `golang {VERSION}`
  - `.mise.toml` - `go = "{VERSION}"`
  - `go.mod` - `go {VERSION}`

#### Runtime Image
`alpine:latest`

#### Build Args
  - `GO_VERSION` - The version of Go to install (default: `1.24`)
  - `TARGETOS` - The target OS for the build (default: `linux`)
  - `TARGETARCH` - The target architecture for the build (default: `amd64`)
  - `CGO_ENABLED` - Enable CGO for the build (default: `0`)
  - `GOPROXY` - The Go module proxy to use (default: `direct`)
  - `PACKAGE` - The package to compile e.g. `./cmd/http` (default: detected via `cmd` directory or `main.go`)
  - `RUNNER` - The base runtime image (default: `docker.io/library/alpine:latest`)
  - `APK_EXTRA_PKGS` - Extra APK packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Package Detection
  - Find the directory in `cmd` with a `.go` file
  - `main.go` file in the root directory

#### Install Command
`if [ -f go.mod ]; then go mod download && go mod tidy; fi`

#### Build Command
`CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /go/bin/app "${PACKAGE}"`

#### Start Command
`["/app/app"]`

---

### Java

[Java](https://www.java.com/) is a class-based, object-oriented programming language that is designed to have as few implementation dependencies as possible.

#### Detected Files
  - `pom.{xml,atom,clj,groovy,rb,scala,yml,yaml}`

#### Version Detection
JDK version:
  - `.tool-versions` - `java {VERSION}`
Maven version:
  - `.tool-versions` - `maven {VERSION}`

#### Runtime Image
`eclipse-temurin:${JAVA_VERSION}-jdk`

#### Build Args
  - `JAVA_VERSION` - The version of the JDK to install (default: `17`)
  - `MAVEN_VERSION` - The version of Maven to install (default: `3`)
  - `JAVA_OPTS` - The Java options to pass to the JVM (default: `-Xmx512m -Xms256m`)
  - `BUILD_CMD` - The command to build the project (default: best guess via source code)
  - `START_CMD` - The command to start the project (default: detected via source code)
  - `RUNNER` - The base runtime image (default: `docker.io/library/eclipse-temurin:${JAVA_VERSION}-jdk`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Install Command
- If Maven: `mvn install`

#### Build Command
- If Maven: `mvn -DoutputFile=target/mvn-dependency-list.log -B -DskipTests clean dependency:list install`

#### Start Command
- Default: `java $JAVA_OPTS -jar target/*jar`
- If Spring Boot: `java -Dserver.port=${PORT} $JAVA_OPTS -jar target/*jar`

---

### Next.js

[Next.js](https://nextjs.org/) is a React framework that enables functionality such as server-side rendering and generating static websites.

#### Detected Files
  - `next.config.{js,mjs,cjs,ts,mts}`
  - `next-env.d.ts`
  - `.next/`

#### Version Detection
  - `.tool-versions` - `nodejs {VERSION}`
  - `.nvmrc` - `v{VERSION}`
  - `.node-version` - `v{VERSION}`
  - `.mise.toml` - `node = "{VERSION}"`
  - `package.json` - `"engines": {"node": "{VERSION}"}`

#### Runtime Image
`node:${NODE_VERSION}-slim`

#### Build Args
  - `NODE_VERSION` - The version of Node.js to install (default: `lts`)
  - `RUNNER` - The base runtime image (default: `docker.io/library/node:${NODE_VERSION}-slim`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Install Command
```sh
if [ -f yarn.lock ]; then yarn --frozen-lockfile; \
elif [ -f package-lock.json ]; then npm ci; \
elif [ -f bun.lockb ]; then npm i -g bun && bun install; \
elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm i --frozen-lockfile; \
else echo "Lockfile not found." && exit 1; \
fi
```
#### Build Command
```sh
if [ -f yarn.lock ]; then yarn run build; \
elif [ -f package-lock.json ]; then npm run build; \
elif [ -f bun.lockb ]; then npm i -g bun && bun run build; \
elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm run build; \
else echo "Lockfile not found." && exit 1; \
fi
```

#### Start Command
- If `"output" :"standalone"`  in `next.config.js`: `HOSTNAME="0.0.0.0" node server.js`
- Otherwise `["node_modules/.bin/next", "start", "-H", "0.0.0.0"]`

---

### Node.js

[Node.js](https://nodejs.org/) is a JavaScript runtime built on Chrome's V8 JavaScript engine.

#### Detected Files
  - `yarn.lock`
  - `package-lock.json`
  - `pnpm-lock.yaml`

#### Version Detection
  - `.tool-versions` - `nodejs {VERSION}`
  - `.nvmrc` - `v{VERSION}`
  - `.node-version` - `v{VERSION}`

#### Runtime Image
`node:${NODE_VERSION}-slim`

#### Build Args
  - `NODE_VERSION` - The version of Node.js to install (default: `lts`)
  - `INSTALL_CMD` - The command to install dependencies (default: detected from source code)
  - `BUILD_CMD` - The command to build the project (default: detected from source code)
  - `START_CMD` - The command to start the project (default: detected from source code)
  - `RUNNER` - The base runtime image (default: `docker.io/library/node:${NODE_VERSION}-slim`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Install Command
- If Yarn: `yarn --frozen-lockfile`
- If npm: `npm ci`
- If pnpm: `corepack enable pnpm && pnpm i --frozen-lockfile`

#### Build Command
In order of precedence:
  - `package.json` scripts: `"build:prod", "build:production", "build-prod", "build-production", "build"`

#### Start Command
In order of precedence:
  - `package.json` scripts: `"serve", "start:prod", "start:production", "start-prod", "start-production", "preview", "start"`
  - `package.json` scripts search for regex matching: `^.*?\b(ts-)?node(mon)?\b.*?(index|main|server|client)\.([cm]?[tj]s)\b`
  - `package.json` main/module file: `node ${mainFile}`

---

### PHP

[PHP](https://www.php.net/) is a popular general-purpose scripting language that is especially suited to web development.

#### Detected Files
  - `composer.json`
  - `index.php`

#### Version Detection
  - `.tool-versions` - `php {VERSION}`
  - `composer.json` - `"php": "{VERSION}"`

#### Runtime Image
`php:${PHP_VERSION}-apache`

#### Build Args
  - `PHP_VERSION` - The version of PHP to install (default: `8.3`)
  - `INSTALL_CMD` - The command to install dependencies (default: detected via source code)
  - `BUILD_CMD` - The command to build the project (default: detected via source code)
  - `START_CMD` - The command to start the project (default: `apache2-foreground`)
  - `RUNNER` - The base runtime image (default: `docker.io/library/php:${PHP_VERSION}-apache`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Install Command
- If Composer: `composer update && composer install --prefer-dist --no-dev --optimize-autoloader --no-interaction`
- If `package.json` exists: composer install command + see Node.js install command

#### Build Command
- If `package.json` exists: see Node.js build command

#### Start Command
`apache2-foreground`

---

### Python

[Python](https://www.python.org/) is a high-level, interpreted programming language that is known for its readability and simplicity.

#### Detected Files
  - `requirements.txt`
  - `uv.lock`
  - `poetry.lock`
  - `Pipefile.lock`
  - `pyproject.toml`
  - `pdm.lock`
  - `main.py`
  - `app.py`
  - `application.py`
  - `app/__init__.py`
  - `filepath.Join(filepath.Base(path), "app.py")`
  - `filepath.Join(filepath.Base(path), "application.py")`
  - `filepath.Join(filepath.Base(path), "main.py")`
  - `filepath.Join(filepath.Base(path), "__init__.py")`

#### Version Detection
  - `.tool-versions` - `python {VERSION}`
  - `.python-version` - `{VERSION}`
  - `.mise.toml` - `python = "{VERSION}"`
  - `runtime.txt` - `python-{VERSION}`

#### Runtime Image
`python:${PYTHON_VERSION}-slim`

#### Build Args
  - `PYTHON_VERSION` - The version of Python to install (default: `3.12`)
  - `INSTALL_CMD` - The command to install dependencies (default: detected from source code)
  - `START_CMD` - The command to start the project (default: detected from source code)
  - `RUNNER` - The base runtime image (default: `docker.io/library/python:${PYTHON_VERSION}-slim`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Install Command
- If Poetry: `pip install poetry && poetry install --no-dev --no-ansi --no-root`
- If Pipenv: `pipenv install --dev --system --deploy`
- If uv: `pip install uv && uv sync --python-preference=only-system --no-cache --no-dev`
- If PDM: `pip install pdm && pdm install --prod`
- If `pyproject.toml` exists: `pip install --upgrade build setuptools && pip install .`
- If `requirements.txt` exists: `pip install -r requirements.txt`

#### Start Command
- If Django is detected: `python manage.py runserver 0.0.0.0:${PORT}`
- If FastAPI is detected: `fastapi run [main.py, app.py, application.py, app/main.py, app/application.py, app/__init__.py] --port ${PORT}`
- If `pyproject.toml` exists: `python -m ${projectName}`
- Otherwise: `python [main.py, app.py, application.py, app/main.py, app/application.py, app/__init__.py]`

---

### Ruby

[Ruby](https://www.ruby-lang.org/) is a dynamic, open-source programming language with a focus on simplicity and productivity.

#### Detected Files
  - `Gemfile`
  - `Gemfile.lock`
  - `config.ru`
  - `Rakefile`
  - `config/environment.rb`

#### Version Detection
  - `.tool-versions` - `ruby {VERSION}`
  - `.ruby-version` - `{VERSION}`
  - `.mise.toml` - `ruby = "{VERSION}"`
  - `Gemfile` - `ruby '{VERSION}'`

#### Runtime Image
`ruby:${RUBY_VERSION}-slim`

#### Build Args
  - `RUBY_VERSION` - The version of Ruby to install (default: `3.3`)
  - `INSTALL_CMD` - The command to install dependencies (default: detected from source code)
  - `BUILD_CMD` - The command to build the project (default: detected from source code)
  - `START_CMD` - The command to start the project (default: detected from source code)
  - `RUNNER` - The base runtime image (default: `docker.io/library/ruby:${RUBY_VERSION}-slim`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Install Command
- `bundle install`
- If `package.json` exists: `bundle install && [package manager install command]`

#### Build Command
- If Rails: `bundle exec rake assets:precompile`

#### Start Command
- If Rails: `bundle exec rails server -b 0.0.0.0 -p ${PORT}`
- If `config.ru` exists: `bundle exec rackup config.ru -p ${PORT}`
- If `config/environment.rb` exists: `bundle exec rails server -b`
- If `Rakefile` exists: `bundle exec rake`

---

### Rust

[Rust](https://www.rust-lang.org/) is a systems programming language that is known for its speed, memory safety, and parallelism.

#### Detected Files
  - `Cargo.toml`

#### Runtime Image
`debian:stable-slim`

#### Build Args
  - `TARGETOS` - The target OS for the build (default: `linux`)
  - `TARGETARCH` - The target architecture for the build (default: `amd64`)
  - `BIN_NAME` - The name of the release binary (default: detected via `Cargo.toml`)
  - `RUNNER` - The base runtime image (default: `docker.io/library/debian:stable-slim`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Build Command
```sh 
if [ "${TARGETARCH}" = "amd64" ]; then rustup target add x86_64-unknown-linux-gnu; else rustup target add aarch64-unknown-linux-gnu; fi
if [ "${TARGETARCH}" = "amd64" ]; then cargo zigbuild --release --target x86_64-unknown-linux-gnu; else cargo zigbuild --release --target aarch64-unknown-linux-gnu; fi
```

#### Start Command
Determined by the binary name in the `Cargo.toml` file
- `["/app/app"]`

---

### Zig

[Zig](https://ziglang.org/) is a general-purpose programming language and toolchain for maintaining robust, optimal, and reusable software.

#### Detected Files
  - `build.zig`
  - `build.zig.zon`
  - `src/main.zig`, `main.zig`

#### Version Detection
  - `.tool-versions` - `zig {VERSION}`
  - `.mise.toml` - `zig = "{VERSION}"`
  - `build.zig.zon` - `.minimum_zig_version = "{VERSION}"`

#### Runtime Image
`alpine:latest`

#### Build Args
  - `ZIG_VERSION` - The version of the Zig toolchain (default: `0.13.0`)
  - `BUILD_CMD` - The command to compile the project (default: `zig build -Doptimize=ReleaseSafe --prefix /app/out`)
  - `RUNNER` - The base runtime image (default: `docker.io/library/alpine:latest`)
  - `APK_EXTRA_PKGS` - Extra APK packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)

#### Build Command
`zig build -Doptimize=ReleaseSafe --prefix /app/out`

#### Start Command
`["/app/app"]`

---

### Scala

[Scala](https://www.scala-lang.org/) is a strong statically typed high-level general-purpose programming language that supports both object-oriented programming and functional programming.

#### Detected Files
  - `build.sbt`
  - `project/build.properties`
  - `project/plugins.sbt`
  - `src/main/scala/`

#### Version Detection
  - `build.sbt` - `scalaVersion := "{VERSION}"`
  - `.tool-versions` - `scala {VERSION}`
  - `.mise.toml` - `scala = "{VERSION}"`

#### Runtime Image
`eclipse-temurin:${JAVA_VERSION}-jdk`

#### Build Args
  - `JAVA_VERSION` - The Java version to run on (default: `21`)
  - `BUILD_CMD` - The command to build the project (default: auto-detected via `sbt stage`, `sbt assembly`, or `sbt compile package`)
  - `RUNNER` - The base runtime image (default: `docker.io/library/eclipse-temurin:${JAVA_VERSION}-jdk`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)
  - `USER` - The user to run the application as (default: `nonroot:nonroot`, or `root`)
  - `JAVA_OPTS` - JVM options to pass at runtime (default: empty)

#### Build Command
Detected in order of precedence:
  - If `sbt-native-packager` is in `project/`: `sbt stage`
  - If `sbt-assembly` is in `project/`: `sbt assembly`
  - Otherwise: `sbt compile package`

#### Start Command
```
if [ -d target/universal/stage/bin ]; then exec $(find target/universal/stage/bin ...) -Dhttp.port=${PORT};
else exec java ${JAVA_OPTS} -jar $(find target -name "*.jar" ...); fi
```

---

### Static (HTML, CSS, JS)

[Static Web Server](https://static-web-server.net/) is a cross-platform, high-performance & asynchronous web server for static files serving.
It is nearly as fast as Nginx and Lighttpd, but is [easily configurable with environment variables](https://static-web-server.net/configuration/environment-variables/).

#### Detected Files
  - `public/`
  - `static/`
  - `dist/`
  - `index.html`

#### Runtime Image
`joseluisq/static-web-server:${VERSION}-debian`

#### Build Args
  - `VERSION` - The version of the static web server to install (default: `2`)
  - `SERVER_ROOT` - The root directory of the server (default: detected from source code)
  - `RUNNER` - The base runtime image (default: `docker.io/joseluisq/static-web-server:${VERSION}-debian`)
  - `APT_EXTRA_PKGS` - Extra APT packages to install in runtime image (default: empty)

---

## Contributing

Read the [CONTRIBUTING.md](CONTRIBUTING.md) guide to learn how to contribute to this project.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.