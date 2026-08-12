package dockerfile

import (
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

