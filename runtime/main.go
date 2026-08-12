package runtime

import "strings"

// An interface that all runtimes must implement.
type Runtime interface {
	// Returns the name of the runtime.
	Name() RuntimeName
	// Returns true if the runtime can be used for the given path.
	Match(path string) bool
	// Generates a Dockerfile for the given path.
	GenerateDockerfile(path string, data ...map[string]string) ([]byte, error)
}

type RuntimeName string

const (
	RuntimeNameGolang RuntimeName = "go"
	RuntimeNameRuby   RuntimeName = "ruby"
	RuntimeNamePython RuntimeName = "python"
	RuntimeNamePHP    RuntimeName = "php"
	RuntimeNameElixir RuntimeName = "elixir"
	RuntimeNameJava   RuntimeName = "java"
	RuntimeNameRust   RuntimeName = "rust"
	RuntimeNameNextJS RuntimeName = "nextjs"
	RuntimeNameBun    RuntimeName = "bun"
	RuntimeNameDeno   RuntimeName = "deno"
	RuntimeNameNode   RuntimeName = "node"
	RuntimeNameDotNet RuntimeName = "dotnet"
	RuntimeNameDart   RuntimeName = "dart"
	RuntimeNameCpp    RuntimeName = "cpp"
	RuntimeNameZig    RuntimeName = "zig"
	RuntimeNameScala  RuntimeName = "scala"
	RuntimeNameAstro  RuntimeName = "astro"
	RuntimeNameNuxt   RuntimeName = "nuxt"
	RuntimeNameR      RuntimeName = "r"
	RuntimeNameStatic RuntimeName = "static"
)

// GetTemplate returns the Dockerfile template for the given runtime name and optional subkind.
// If subkind is empty, the default template for the runtime is returned.
func GetTemplate(name RuntimeName, subkind string) string {
	subkind = strings.ToLower(strings.TrimSpace(subkind))

	switch name {
	case RuntimeNameGolang:
		return golangTemplate
	case RuntimeNameRuby:
		return rubyTemplate
	case RuntimeNamePython:
		return pythonTemplate
	case RuntimeNamePHP:
		return phpTemplate
	case RuntimeNameElixir:
		return elixirTemplate
	case RuntimeNameJava:
		switch subkind {
		case "gradle", "gradlew":
			return javaGradleTemplate
		case "maven", "mvn", "pom", "":
			return javaMavenTemplate
		default:
			return javaMavenTemplate
		}
	case RuntimeNameRust:
		return rustlangTemplate
	case RuntimeNameNextJS:
		switch subkind {
		case "standalone":
			return nextJSStandaloneTemplate
		case "server", "":
			return nextJSServerTemplate
		default:
			return nextJSServerTemplate
		}
	case RuntimeNameBun:
		return bunTemplate
	case RuntimeNameDeno:
		return denoTemplate
	case RuntimeNameNode:
		return nodeTemplate
	case RuntimeNameDotNet:
		return dotnetTemplate
	case RuntimeNameDart:
		return dartTemplate
	case RuntimeNameCpp:
		return cppTemplate
	case RuntimeNameZig:
		return zigTemplate
	case RuntimeNameScala:
		return scalaTemplate
	case RuntimeNameAstro:
		switch subkind {
		case "static", "ssg":
			return astroStaticTemplate
		case "node", "ssr", "":
			return astroNodeTemplate
		default:
			return astroNodeTemplate
		}
	case RuntimeNameNuxt:
		switch subkind {
		case "static", "ssg":
			return nuxtStaticTemplate
		case "ssr", "server", "":
			return nuxtSSRTemplate
		default:
			return nuxtSSRTemplate
		}
	case RuntimeNameR:
		switch subkind {
		case "shiny", "app":
			return rShinyTemplate
		case "plumber", "api", "":
			return rPlumberTemplate
		default:
			return rPlumberTemplate
		}
	case RuntimeNameStatic:
		return staticTemplate
	default:
		return ""
	}
}

