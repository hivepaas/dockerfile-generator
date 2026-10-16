package dockerfile

import (
	"io"
	"log/slog"
	"testing"
)

func TestSupportedRuntimes(t *testing.T) {
	if len(SupportedRuntimes) != 20 {
		t.Fatalf("expected 20 supported runtimes, got %d", len(SupportedRuntimes))
	}
}

func TestParseRuntime(t *testing.T) {
	tests := []struct {
		input    string
		expected Runtime
		valid    bool
	}{
		{"go", RuntimeGolang, true},
		{"golang", RuntimeGolang, true},
		{"Go", RuntimeGolang, true},
		{"Python", RuntimePython, true},
		{"python", RuntimePython, true},
		{"Next.js", RuntimeNextJS, true},
		{"nextjs", RuntimeNextJS, true},
		{"next", RuntimeNextJS, true},
		{"Node", RuntimeNode, true},
		{"nodejs", RuntimeNode, true},
		{"C++", RuntimeCpp, true},
		{"c++", RuntimeCpp, true},
		{".net", RuntimeDotNet, true},
		{"dotnet", RuntimeDotNet, true},
		{"Rust", RuntimeRust, true},
		{"unknown_framework", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseRuntime(tt.input)
			if ok != tt.valid {
				t.Fatalf("ParseRuntime(%q) valid = %v, expected %v", tt.input, ok, tt.valid)
			}
			if got != tt.expected {
				t.Fatalf("ParseRuntime(%q) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetTemplate(t *testing.T) {
	for _, r := range SupportedRuntimes {
		tmpl := GetTemplate(r, "")
		if tmpl == "" {
			t.Errorf("GetTemplate(%q, \"\") returned empty template", r)
		}
	}

	// Test subkinds
	if tmpl := GetTemplate(RuntimeJava, "gradle"); tmpl == "" {
		t.Errorf("GetTemplate(RuntimeJava, gradle) returned empty template")
	}
}

func TestMatchRuntime(t *testing.T) {
	df := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	tests := []struct {
		path     string
		expected Runtime
	}{
		// composer.json comes before the lockfile of its assets' build
		{"testdata/php-laravel", RuntimePHP},
		{"testdata/php-composer", RuntimePHP},
		// index.php alone does not
		{"testdata/php-npm", RuntimeNode},
		{"testdata/nextjs", RuntimeNextJS},
		{"testdata/node", RuntimeNode},
		{"testdata/python", RuntimePython},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			r, err := df.MatchRuntime(tt.path)
			if err != nil {
				t.Fatalf("MatchRuntime(%q): %v", tt.path, err)
			}
			if r.Name() != tt.expected {
				t.Fatalf("MatchRuntime(%q) = %q, expected %q", tt.path, r.Name(), tt.expected)
			}
		})
	}
}
