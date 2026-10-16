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
)

type Java struct {
	Log *slog.Logger
}

func (d *Java) Name() RuntimeName {
	return RuntimeNameJava
}

func (d *Java) Match(path string) bool {
	checkPaths := []string{
		filepath.Join(path, "build.gradle"),
		filepath.Join(path, "gradlew"),
		filepath.Join(path, "pom.xml"),
		filepath.Join(path, "pom.atom"),
		filepath.Join(path, "pom.clj"),
		filepath.Join(path, "pom.groovy"),
		filepath.Join(path, "pom.rb"),
		filepath.Join(path, "pom.scala"),
		filepath.Join(path, "pom.yml"),
		filepath.Join(path, "pom.yaml"),
	}

	for _, p := range checkPaths {
		if _, err := os.Stat(p); err == nil {
			d.Log.Info("Detected Java project")
			return true
		}
	}

	d.Log.Debug("Java project not detected")
	return false
}

func (d *Java) GenerateDockerfile(path string, data ...map[string]string) ([]byte, error) {
	version, err := findJDKVersion(path, d.Log)
	if err != nil {
		return nil, err
	}

	tpl := javaMavenTemplate
	startCMD := "java $JAVA_OPTS -jar " + javaMavenJar
	buildCMD := ""
	gradleVersion := ""

	_, wrapperErr := os.Stat(filepath.Join(path, "gradlew"))
	_, groovyErr := os.Stat(filepath.Join(path, "build.gradle"))
	_, kotlinErr := os.Stat(filepath.Join(path, "build.gradle.kts"))
	if wrapperErr == nil || groovyErr == nil || kotlinErr == nil {
		gv, err := findGradleVersion(path, d.Log)
		if err != nil {
			return nil, err
		}

		gradleVersion = *gv
		tpl = javaGradleTemplate
		// The project's own Gradle, or the image's when it has no wrapper
		gradle := "gradle"
		if wrapperErr == nil {
			gradle = "./gradlew"
		}
		buildCMD = gradle + " clean build -x check -x test"
		startCMD = "java $JAVA_OPTS -jar " + javaGradleJar
	}

	mavenVersion := ""
	for _, file := range pomFiles {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			mv, err := findMavenVersion(path, d.Log)
			if err != nil {
				return nil, err
			}

			mavenVersion = *mv
			buildCMD = "mvn -DoutputFile=target/mvn-dependency-list.log -B -DskipTests clean dependency:list install"
			break
		}
	}

	if isSpringBootApp(path) {
		d.Log.Info("Detected Spring Boot application")
		startCMD = "java -Dserver.port=${PORT} $JAVA_OPTS -jar " + javaMavenJar
		if gradleVersion != "" {
			startCMD = "java $JAVA_OPTS -jar -Dserver.port=${PORT} " + javaGradleJar
		}
	}

	if isWildflySwarmApp(path) {
		d.Log.Info("Detected Wildfly Swarm application")
		startCMD = "java -Dswarm.http.port=${PORT} $JAVA_OPTS -jar " + javaMavenJar
	}

	d.Log.Info(
		fmt.Sprintf(`Detected defaults 
  JDK version     : %s
  Maven version   : %s
  Gradle version  : %s
  Build command   : %s
  Start command   : %s

  Docker build arguments can supersede these defaults if provided.
  See https://github.com/hivepaas/dockerfile-generator#runtime-documentation`, *version, mavenVersion, gradleVersion, buildCMD, startCMD),
	)

	var buf bytes.Buffer

	tmpl, err := template.New("Dockerfile").Parse(tpl)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse template")
	}

	if buildCMD != "" {
		buildCMDJSON, _ := json.Marshal(buildCMD)
		buildCMD = string(buildCMDJSON)
	}

	if startCMD != "" {
		startCMDJSON, _ := json.Marshal(startCMD)
		startCMD = string(startCMDJSON)
	}
	templateData := map[string]string{
		"Version":       *version,
		"GradleVersion": gradleVersion,
		"MavenVersion":  mavenVersion,
		"BuildCMD":      buildCMD,
		"StartCMD":      startCMD,
	}
	if len(data) > 0 {
		maps.Copy(templateData, data[0])
	}
	if err := tmpl.Option("missingkey=zero").Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("Failed to execute template")
	}

	return buf.Bytes(), nil
}

var javaMavenTemplate = strings.TrimSpace(`
ARG JAVA_VERSION={{.Version}}
ARG MAVEN_VERSION={{.MavenVersion}}
ARG RUNNER=docker.io/library/eclipse-temurin:${JAVA_VERSION}-jdk
FROM maven:${MAVEN_VERSION}-eclipse-temurin-${JAVA_VERSION} AS build
WORKDIR /app

COPY pom.xml* pom.atom* pom.clj* pom.groovy* pom.rb* pom.scala* pom.yml* pom.yaml* .
# The dependencies, cached apart from the code; a project of several modules
# fetches them as it builds
RUN --mount=type=cache,target=/root/.m2 mvn -B dependency:go-offline || true

COPY . .
# The build skips the tests, as a deployment is not where they run
ARG BUILD_CMD={{.BuildCMD}}
RUN --mount=type=cache,target=/root/.m2 if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; fi
# The jar the build made, among those beside it: not its sources, its
# documentation or its tests
RUN jar="$(ls -1 target/*.jar 2>/dev/null | grep -v -E -- '-(sources|javadoc|tests)\.jar$' | head -n 1)" \
    && if [ -n "${jar}" ]; then cp "${jar}" /app/app.jar; fi

FROM ${RUNNER} AS runtime
WORKDIR /app
VOLUME /tmp

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
RUN chown -R nonroot:nonroot /app

COPY --from=build --chown=nonroot:nonroot /app/target/*.jar /app/target/
COPY --from=build --chown=nonroot:nonroot /app/app.ja[r] /app/

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

var javaGradleTemplate = strings.TrimSpace(`
ARG JAVA_VERSION={{.Version}}
ARG GRADLE_VERSION={{.GradleVersion}}
ARG BUILDER=docker.io/library/gradle
ARG RUNNER=docker.io/library/eclipse-temurin:${JAVA_VERSION}-jdk
FROM ${BUILDER}:${GRADLE_VERSION}-jdk${JAVA_VERSION} AS build
WORKDIR /app

COPY . .

ARG BUILD_CMD={{.BuildCMD}}
RUN --mount=type=cache,target=/root/.gradle if [ ! -z "${BUILD_CMD}" ]; then sh -c "$BUILD_CMD"; fi
# The jar the build made, among those beside it: not its sources, its
# documentation, its tests or Gradle's plain jar
RUN jar="$(ls -1 build/libs/*.jar 2>/dev/null | grep -v -E -- '-(sources|javadoc|tests|plain)\.jar$' | head -n 1)" \
    && if [ -n "${jar}" ]; then cp "${jar}" /app/app.jar; fi

FROM ${RUNNER} AS runtime
WORKDIR /app
VOLUME /tmp

ARG APT_EXTRA_PKGS=
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates ${APT_EXTRA_PKGS} && apt-get clean && rm -f /var/lib/apt/lists/*_*
RUN update-ca-certificates 2>/dev/null || true
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot
RUN chown -R nonroot:nonroot /app

COPY --from=build --chown=nonroot:nonroot /app/build/libs/*.jar /app/build/libs/
COPY --from=build --chown=nonroot:nonroot /app/app.ja[r] /app/

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

func findJDKVersion(path string, log *slog.Logger) (*string, error) {
	version := ""
	versionFiles := []string{".tool-versions"}

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
					if strings.Contains(line, "java") {
						versionString := strings.Split(line, " ")[1]
						regexpVersion := regexp.MustCompile(`\d+`)
						version = regexpVersion.FindString(versionString)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read .tool-versions file")
				}

				log.Info("Detected JDK version in .tool-versions: " + version)
			}

			f.Close()
			if version != "" {
				break
			}
		}
	}

	if version == "" {
		version = "17"
	}

	return &version, nil
}

func findGradleVersion(path string, log *slog.Logger) (*string, error) {
	version := ""
	versionFiles := []string{".tool-versions"}

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
					if strings.Contains(line, "gradle") {
						version = strings.Split(line, " ")[1]
						log.Info("Detected Gradle version in .tool-versions: " + version)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read .tool-versions file")
				}
			}

			f.Close()
			if version != "" {
				break
			}
		}
	}

	if version == "" {
		version = "8"
		log.Info(fmt.Sprintf("No Gradle version detected. Using: %s", version))
	}

	return &version, nil
}

func findMavenVersion(path string, log *slog.Logger) (*string, error) {
	version := ""
	versionFiles := []string{".tool-versions"}

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
					if strings.Contains(line, "maven") {
						version = strings.Split(line, " ")[1]
						log.Info("Detected Maven version in .tool-versions: " + version)
						break
					}
				}

				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("Failed to read .tool-versions file")
				}
			}

			f.Close()
			if version != "" {
				break
			}
		}
	}

	if version == "" {
		version = "3"
		log.Info(fmt.Sprintf("No Maven version detected. Using: %s", version))
	}

	return &version, nil
}

func isSpringBootApp(path string) bool {
	checkFiles := append([]string{}, pomFiles...)
	checkFiles = append(checkFiles, "build.gradle")

	for _, file := range checkFiles {
		pomXML, err := os.Open(filepath.Join(path, file))
		if err != nil {
			continue
		}

		defer pomXML.Close()
		scanner := bufio.NewScanner(pomXML)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "org.springframework.boot") {
				return true
			}
		}
	}

	return false
}

func isWildflySwarmApp(path string) bool {
	for _, file := range pomFiles {
		pomXML, err := os.Open(filepath.Join(path, file))
		if err != nil {
			continue
		}

		defer pomXML.Close()
		scanner := bufio.NewScanner(pomXML)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "wildfly-swarm") {
				return true
			} else if strings.Contains(line, "org.wildfly.swarm") {
				return true
			}
		}
	}

	return false
}

// javaMavenJar and javaGradleJar are the jar the build picks among those it
// makes, which it copies to app.jar: the start command is not a shell's, and
// would not run a $(...) to find it.
const (
	javaMavenJar  = "app.jar"
	javaGradleJar = "app.jar"
)

var pomFiles = []string{
	"pom.xml",
	"pom.atom",
	"pom.clj",
	"pom.groovy",
	"pom.rb",
	"pom.scala",
	"pom.yml",
	"pom.yaml",
}
