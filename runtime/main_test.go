package runtime_test

import (
	"log/slog"
	"testing"

	"github.com/hivepaas/dockerfile-generator/runtime"
)

type noopWriter struct{}

func (w *noopWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

var logger = slog.New(slog.NewJSONHandler(&noopWriter{}, nil))

func TestGetTemplate(t *testing.T) {
	runtimes := []runtime.RuntimeName{
		runtime.RuntimeNameGolang,
		runtime.RuntimeNameRuby,
		runtime.RuntimeNamePython,
		runtime.RuntimeNamePHP,
		runtime.RuntimeNameElixir,
		runtime.RuntimeNameJava,
		runtime.RuntimeNameRust,
		runtime.RuntimeNameNextJS,
		runtime.RuntimeNameBun,
		runtime.RuntimeNameDeno,
		runtime.RuntimeNameNode,
		runtime.RuntimeNameDotNet,
		runtime.RuntimeNameDart,
		runtime.RuntimeNameCpp,
		runtime.RuntimeNameZig,
		runtime.RuntimeNameScala,
		runtime.RuntimeNameAstro,
		runtime.RuntimeNameNuxt,
		runtime.RuntimeNameR,
		runtime.RuntimeNameStatic,
	}

	for _, r := range runtimes {
		tmpl := runtime.GetTemplate(r, "")
		if tmpl == "" {
			t.Errorf("GetTemplate(%q, \"\") returned empty template", r)
		}
	}

	// Test Java subkinds
	javaMaven := runtime.GetTemplate(runtime.RuntimeNameJava, "maven")
	javaGradle := runtime.GetTemplate(runtime.RuntimeNameJava, "gradle")
	if javaMaven == "" || javaGradle == "" || javaMaven == javaGradle {
		t.Errorf("Java templates mismatch: maven=%v, gradle=%v", javaMaven != "", javaGradle != "")
	}

	// Test NextJS subkinds
	nextServer := runtime.GetTemplate(runtime.RuntimeNameNextJS, "server")
	nextStandalone := runtime.GetTemplate(runtime.RuntimeNameNextJS, "standalone")
	if nextServer == "" || nextStandalone == "" || nextServer == nextStandalone {
		t.Errorf("NextJS templates mismatch: server=%v, standalone=%v", nextServer != "", nextStandalone != "")
	}

	// Test Astro subkinds
	astroNode := runtime.GetTemplate(runtime.RuntimeNameAstro, "node")
	astroStatic := runtime.GetTemplate(runtime.RuntimeNameAstro, "static")
	if astroNode == "" || astroStatic == "" || astroNode == astroStatic {
		t.Errorf("Astro templates mismatch: node=%v, static=%v", astroNode != "", astroStatic != "")
	}

	// Test Nuxt subkinds
	nuxtSSR := runtime.GetTemplate(runtime.RuntimeNameNuxt, "ssr")
	nuxtStatic := runtime.GetTemplate(runtime.RuntimeNameNuxt, "static")
	if nuxtSSR == "" || nuxtStatic == "" || nuxtSSR == nuxtStatic {
		t.Errorf("Nuxt templates mismatch: ssr=%v, static=%v", nuxtSSR != "", nuxtStatic != "")
	}

	// Test R subkinds
	rPlumber := runtime.GetTemplate(runtime.RuntimeNameR, "plumber")
	rShiny := runtime.GetTemplate(runtime.RuntimeNameR, "shiny")
	if rPlumber == "" || rShiny == "" || rPlumber == rShiny {
		t.Errorf("R templates mismatch: plumber=%v, shiny=%v", rPlumber != "", rShiny != "")
	}

	if tmpl := runtime.GetTemplate(runtime.RuntimeName("unknown"), ""); tmpl != "" {
		t.Errorf("GetTemplate(unknown, \"\") = %q, expected empty string", tmpl)
	}
}


