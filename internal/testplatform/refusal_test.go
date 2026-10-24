package testplatform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// V1-2486: the helper skips only on Windows and only for a listed refusal.
func TestWindowsRefusalSkipOnlyAppliesToAListedRefusalOnWindows(t *testing.T) {
	refusal := fmt.Errorf("run git: %w", errors.New("process-platform-unsupported"))
	for _, goos := range []string{"darwin", "linux"} {
		if reason, skip := windowsRefusalSkip(goos, refusal); skip || reason != "" {
			t.Fatalf("%s: skip=%v reason=%q, want no skip on a supported platform", goos, skip, reason)
		}
	}
	reason, skip := windowsRefusalSkip("windows", refusal)
	if !skip || !strings.Contains(reason, "process-platform-unsupported") || !strings.Contains(reason, "internal/procgroup/process.go") {
		t.Fatalf("windows: skip=%v reason=%q, want a skip quoting the refusal and its source", skip, reason)
	}
	for _, err := range []error{nil, errors.New("exit status 1"), errors.New("open x: Access is denied.")} {
		if reason, skip := windowsRefusalSkip("windows", err); skip {
			t.Fatalf("windows: %v skipped with %q, want only a listed refusal to skip", err, reason)
		}
	}
}

// V1-2486: on the platform running this test, other than Windows, the helper
// returns without skipping even for a listed refusal.
func TestSkipWindowsRefusalDoesNothingOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this proves the helper is inert off Windows")
	}
	ran := false
	t.Run("refusal", func(t *testing.T) {
		SkipWindowsRefusal(t, errors.New(PlatformRefusals[0].Message))
		ran = true
	})
	if !ran {
		t.Fatalf("SkipWindowsRefusal skipped on %s", runtime.GOOS)
	}
}

// V1-2486: each listed refusal is still the exact text its production source
// carries, so a reworded refusal fails here instead of silently not matching.
func TestPlatformRefusalsMatchTheirSources(t *testing.T) {
	for _, refusal := range PlatformRefusals {
		source, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(refusal.Source)))
		if err != nil {
			t.Fatalf("%s: %v", refusal.Source, err)
		}
		if !strings.Contains(string(source), `"`+refusal.Message) {
			t.Fatalf("%s no longer carries the refusal %q", refusal.Source, refusal.Message)
		}
	}
}
