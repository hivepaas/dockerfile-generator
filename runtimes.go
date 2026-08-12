package dockerfile

import (
	"strings"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

// Runtime represents a supported programming language, framework, or environment.
type Runtime = runtime.RuntimeName

const (
	RuntimeGolang Runtime = runtime.RuntimeNameGolang
	RuntimeRuby   Runtime = runtime.RuntimeNameRuby
	RuntimePython Runtime = runtime.RuntimeNamePython
	RuntimePHP    Runtime = runtime.RuntimeNamePHP
	RuntimeElixir Runtime = runtime.RuntimeNameElixir
	RuntimeJava   Runtime = runtime.RuntimeNameJava
	RuntimeRust   Runtime = runtime.RuntimeNameRust
	RuntimeNextJS Runtime = runtime.RuntimeNameNextJS
	RuntimeBun    Runtime = runtime.RuntimeNameBun
	RuntimeDeno   Runtime = runtime.RuntimeNameDeno
	RuntimeNode   Runtime = runtime.RuntimeNameNode
	RuntimeDotNet Runtime = runtime.RuntimeNameDotNet
	RuntimeDart   Runtime = runtime.RuntimeNameDart
	RuntimeCpp    Runtime = runtime.RuntimeNameCpp
	RuntimeZig    Runtime = runtime.RuntimeNameZig
	RuntimeScala  Runtime = runtime.RuntimeNameScala
	RuntimeAstro  Runtime = runtime.RuntimeNameAstro
	RuntimeNuxt   Runtime = runtime.RuntimeNameNuxt
	RuntimeR      Runtime = runtime.RuntimeNameR
	RuntimeStatic Runtime = runtime.RuntimeNameStatic
)

// SupportedRuntimes is the list of all supported languages, frameworks, and runtimes.
var SupportedRuntimes = []Runtime{
	RuntimeGolang,
	RuntimeRuby,
	RuntimePython,
	RuntimePHP,
	RuntimeElixir,
	RuntimeJava,
	RuntimeRust,
	RuntimeNextJS,
	RuntimeBun,
	RuntimeDeno,
	RuntimeNode,
	RuntimeDotNet,
	RuntimeDart,
	RuntimeCpp,
	RuntimeZig,
	RuntimeScala,
	RuntimeAstro,
	RuntimeNuxt,
	RuntimeR,
	RuntimeStatic,
}

// ParseRuntime parses a string into a supported Runtime (case-insensitive).
func ParseRuntime(s string) (Runtime, bool) {
	for _, r := range SupportedRuntimes {
		if strings.EqualFold(string(r), s) {
			return r, true
		}
	}

	// Handle common aliases
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "golang", "go":
		return RuntimeGolang, true
	case "next", "nextjs", "next.js":
		return RuntimeNextJS, true
	case "c++", "cpp":
		return RuntimeCpp, true
	case "dotnet", ".net":
		return RuntimeDotNet, true
	case "nodejs", "node":
		return RuntimeNode, true
	}

	return "", false
}

// GetTemplate returns the Dockerfile template for the given runtime and optional subkind.
func GetTemplate(name Runtime, subkind string) string {
	return runtime.GetTemplate(name, subkind)
}


