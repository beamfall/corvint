package cli

import (
	"slices"
	"strings"
	"testing"
)

// TestTCLV0743_UnreadPathsAreNotObservedNotPresent pins the review finding
// on V1-3696: a touch path git cannot read at a commit is neither absent nor
// silently present; pathsAbsentAt returns it as unobserved, and the guard
// warns NOT_OBSERVED for it at the base and at the source commit.
func TestTCLV0743_UnreadPathsAreNotObservedNotPresent(t *testing.T) {
	git := func(args ...string) (string, bool) {
		switch args[len(args)-1] {
		case "present":
			return "present", true
		case "absent":
			return "", true
		}
		return "", false // git failed for "unread"
	}
	absent, unobserved := pathsAbsentAt(git, "c0ffee", []string{"present", "absent/", "unread"})
	if !slices.Equal(absent, []string{"absent/"}) || !slices.Equal(unobserved, []string{"unread"}) {
		t.Fatalf("absent %v unobserved %v", absent, unobserved)
	}
	w := unobservedWarning(unobserved, "the base origin/main at c0ffee")
	if !strings.Contains(w, "touchPaths unread could not be read at the base origin/main at c0ffee") || !strings.Contains(w, "NOT_OBSERVED") {
		t.Fatalf("warning %q", w)
	}
}
