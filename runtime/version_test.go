package runtime

import (
	"testing"
)

// The newest version a constraint allows, named as precisely as it allows:
// a major whose newest release it allows is the major alone.
func TestNodeVersionTag(t *testing.T) {
	cases := map[string]string{
		">=18":              "lts",
		"*":                 "lts",
		"^20.10.0":          "20",
		"20.x":              "20",
		">=18 <23":          "22",
		"^22 || ^24":        "24",
		"~20.10.0":          "20.10",
		"20.11.1":           "20.11.1",
		">=14.5.3 <=15.2.1": "15.2.1",
		"lts":               "",
	}
	for constraint, want := range cases {
		if got := nodeVersionTag(constraint); got != want {
			t.Errorf("nodeVersionTag(%q) = %q, want %q", constraint, got, want)
		}
	}
}

func TestNvmVersionTag(t *testing.T) {
	cases := map[string]string{
		"v22.11.0": "22.11.0",
		"22":       "22",
		"lts/*":    "lts",
		"lts/jod":  "jod",
		"node":     "current",
	}
	for line, want := range cases {
		if got := nvmVersionTag(line); got != want {
			t.Errorf("nvmVersionTag(%q) = %q, want %q", line, got, want)
		}
	}
}

// Composer's constraints: "|" is "||", and a tilde with two numbers allows
// up to the next major.
func TestComposerConstraint(t *testing.T) {
	cases := map[string]string{
		"^8.1":          "8",
		">=7.4":         "8",
		"^7.2 | ^8.0":   "8",
		"^7.4 || ~8.0":  "8",
		"~8.2":          "8",
		"~8.2.0":        "8.2",
		"8.2.*":         "8.2",
		">=8.1 <8.4":    "8.3",
		"^8.3@stable":   "8",
		">=7.1, <8.0.0": "7",
	}
	for constraint, want := range cases {
		c, err := composerConstraint(constraint)
		if err != nil {
			t.Errorf("composerConstraint(%q): %v", constraint, err)
			continue
		}
		if got, _ := newestAllowed(c.Check, 8); got != want {
			t.Errorf("newest PHP for %q = %q, want %q", constraint, got, want)
		}
	}
}

// The Gemfile's Ruby: a version as it is, the newest a constraint allows
// within its oldest major otherwise.
func TestRubyVersionTag(t *testing.T) {
	cases := map[string]string{
		"3.3.6":            "3.3.6",
		"3.3":              "3.3",
		"~> 3.2":           "3",
		"~> 3.2.1":         "3.2",
		">= 3.1":           "3",
		"~> 4.0":           "4",
		">= 3.1, < 3.4":    "3.3",
		"not a constraint": "not a constraint",
	}
	for constraint, want := range cases {
		if got := rubyVersionTag(constraint); got != want {
			t.Errorf("rubyVersionTag(%q) = %q, want %q", constraint, got, want)
		}
	}
}
