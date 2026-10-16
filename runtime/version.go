package runtime

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// versionSearchLimit bounds the minors and patches newestAllowed tries.
const versionSearchLimit = 40

// newestAllowed is the image tag of the newest version, up to maxMajor, that
// allows accepts: the major alone ("22") when its newest release is allowed as
// well, so that the image follows its fixes; the minor ("22.11") when that
// minor's newest is; the version itself otherwise. ok is false when none is.
//
// A project's constraint says the oldest version it runs on: that one is often
// out of support, and its image unpatched.
func newestAllowed(allows func(*semver.Version) bool, maxMajor uint64) (tag string, ok bool) {
	for major := int(maxMajor); major >= 0; major-- {
		for minor := versionSearchLimit; minor >= 0; minor-- {
			for patch := versionSearchLimit; patch >= 0; patch-- {
				if !allows(semver.New(uint64(major), uint64(minor), uint64(patch), "", "")) {
					continue
				}
				switch {
				case allows(semver.New(uint64(major), 9999, 9999, "", "")):
					return fmt.Sprint(major), true
				case allows(semver.New(uint64(major), uint64(minor), 9999, "", "")):
					return fmt.Sprintf("%d.%d", major, minor), true
				default:
					return fmt.Sprintf("%d.%d.%d", major, minor, patch), true
				}
			}
		}
	}
	return "", false
}

var (
	composerSinglePipeRe = regexp.MustCompile(`\s*\|\|?\s*`)
	composerTildeRe      = regexp.MustCompile(`~\s*(\d+\.\d+)(\s|,|$)`)
	composerStabilityRe  = regexp.MustCompile(`@\w+`)
)

// composerConstraint reads a Composer version constraint: "|" is "||", and
// "~8.2" allows 8.2 up to 9, not 8.3 as in semver.
func composerConstraint(s string) (*semver.Constraints, error) {
	s = composerStabilityRe.ReplaceAllString(strings.TrimSpace(s), "")
	s = composerSinglePipeRe.ReplaceAllString(s, " || ")
	s = composerTildeRe.ReplaceAllString(s, "^$1$2")
	c, err := semver.NewConstraint(s)
	if err != nil {
		return nil, fmt.Errorf("Composer constraint %q: %w", s, err)
	}
	return c, nil
}
