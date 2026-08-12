// A library for auto-generating Dockerfiles from project source code.
package dockerfile

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

// Creates a new Dockerfile generator. If no logger is provided, a default logger is created.
func New(log ...*slog.Logger) *Dockerfile {
	var logger *slog.Logger

	if len(log) > 0 {
		logger = log[0]
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	return &Dockerfile{
		log: logger,
	}
}

type Dockerfile struct {
	log *slog.Logger
}

// Generates a Dockerfile in-memory for the given path without writing to disk.
func (a *Dockerfile) Generate(path string, data ...map[string]string) ([]byte, runtime.Runtime, error) {
	r, err := a.MatchRuntime(path)
	if err != nil {
		return nil, nil, err
	}

	contents, err := r.GenerateDockerfile(path, data...)
	if err != nil {
		return nil, nil, err
	}

	return contents, r, nil
}

// Generates a Dockerfile for the given path and writes it to the same directory.
func (a *Dockerfile) Write(path string, data ...map[string]string) error {
	contents, r, err := a.Generate(path, data...)
	if err != nil {
		return err
	}

	// Write the Dockerfile to the same directory
	if err = os.WriteFile(filepath.Join(path, "Dockerfile"), contents, 0644); err != nil {
		return err
	}

	a.log.Info("Auto-generated Dockerfile for project using " + string(r.Name()))
	return nil
}

// Lists all runtimes that the Dockerfile generator can auto-generate.
func (a *Dockerfile) ListRuntimes() []runtime.Runtime {
	return []runtime.Runtime{
		&runtime.Golang{Log: a.log},
		&runtime.Rust{Log: a.log},
		&runtime.Ruby{Log: a.log},
		&runtime.Python{Log: a.log},
		&runtime.PHP{Log: a.log},
		&runtime.Java{Log: a.log},
		&runtime.Elixir{Log: a.log},
		&runtime.NextJS{Log: a.log},
		&runtime.Deno{Log: a.log},
		&runtime.Bun{Log: a.log},
		&runtime.Node{Log: a.log},
		&runtime.DotNet{Log: a.log},
		&runtime.Dart{Log: a.log},
		&runtime.Cpp{Log: a.log},
		&runtime.Zig{Log: a.log},
		&runtime.Scala{Log: a.log},
		&runtime.Static{Log: a.log},
	}
}

// Matches the runtime of the project at the given path.
func (a *Dockerfile) MatchRuntime(path string) (runtime.Runtime, error) {
	for _, r := range a.ListRuntimes() {
		if r.Match(path) {
			return r, nil
		}
	}

	return nil, ErrRuntimeNotFound
}

// Error returned when we could not auto-detect the runtime of the project.
var ErrRuntimeNotFound = fmt.Errorf("A Dockerfile was not detected in the project and we could not auto-generate one for you.")
