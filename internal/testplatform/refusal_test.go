package testplatform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/doccorpus"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// V1-2486: the helper skips only on Windows and only for a listed refusal.
func TestWindowsRefusalSkipOnlyAppliesToAListedRefusalOnWindows(t *testing.T) {
	refusal := fmt.Errorf("run git: %w", errors.New("process-platform-unsupported"))
	for _, goos := range []string{"darwin", "linux"} {
		if reason, skip := windowsRefusalSkip(goos, refusal); skip || reason != "" {
			t.Fatalf("%s: skip=%v reason=%q, want no skip on a supported platform", goos, skip, reason)
		}
	}
	for _, accepted := range []error{
		errors.New("process-platform-unsupported"),
		fmt.Errorf("run git: %v", errors.New("process-platform-unsupported")),
		errors.Join(errors.New("exit status 1"), fmt.Errorf("cleanup: %w", errors.New("owned process-group cleanup is unsupported on this platform"))),
	} {
		if _, skip := windowsRefusalSkip("windows", accepted); !skip {
			t.Fatalf("windows: %q did not skip, want the exact production refusal to skip", accepted)
		}
	}
	reason, skip := windowsRefusalSkip("windows", refusal)
	if !skip || !strings.Contains(reason, "process-platform-unsupported") || !strings.Contains(reason, "internal/procgroup/process.go") {
		t.Fatalf("windows: skip=%v reason=%q, want a skip quoting the refusal and its source", skip, reason)
	}
	for _, err := range []error{
		nil,
		errors.New("exit status 1"),
		errors.New("open x: Access is denied."),
		// Refusal text inside a path is not the refusal.
		errors.New(`exec: "C:\process-platform-unsupported\git.exe": executable file not found in %PATH%`),
		fmt.Errorf("lookup git: %w", errors.New(`C:\process-platform-unsupported\git.exe: file does not exist`)),
		// Refusal text in front of an unrelated cause is not the refusal.
		fmt.Errorf("process-platform-unsupported: %w", errors.New("exit status 1")),
		fmt.Errorf("after worker platform unsupported retry: %w", errors.New("Access is denied.")),
		// A longer token that only extends a refusal is not the refusal.
		errors.New("run git: process-platform-unsupported-new"),
	} {
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
		if !strings.Contains(string(source), `"`+refusal.Message+`"`) {
			t.Fatalf("%s no longer carries the refusal %q", refusal.Source, refusal.Message)
		}
	}
}

// V1-2522, V1-2523: the CEM update-lock and Tasks platform refusals, as their
// callers render them, skip on Windows only.
func TestCEMLockAndTasksRefusalsSkipOnlyOnWindows(t *testing.T) {
	lock := fmt.Errorf("cem prepare: %w", cemcode.New(cemcode.PublishFailed,
		"update lock for %q cannot be opened: %v", ".corvint/change.cem.json", errors.New("serialized CEM map updates are unsupported on this platform")))
	tasks := taskswire.Errorf(taskswire.CodeUnsupported, "platform", "Corvint Tasks is unsupported on this platform")
	for _, err := range []error{lock, tasks} {
		for _, goos := range []string{"darwin", "linux"} {
			if reason, skip := windowsRefusalSkip(goos, err); skip || reason != "" {
				t.Fatalf("%s: %v skipped with %q, want no skip on a supported platform", goos, err, reason)
			}
		}
		if _, skip := windowsRefusalSkip("windows", err); !skip {
			t.Fatalf("windows: %v did not skip", err)
		}
	}
	// The pre-V1-2522 message dropped the platform cause and must not skip.
	opaque := cemcode.New(cemcode.PublishFailed, "update lock for %q cannot be opened", ".corvint/change.cem.json")
	if _, skip := windowsRefusalSkip("windows", opaque); skip {
		t.Fatalf("windows: %v skipped without the platform refusal", opaque)
	}
	// A MALFORMED drive-letter PathText is not a platform refusal.
	malformed := taskswire.Errorf(taskswire.CodeMalformed, "", "PathText must be an absolute path")
	if _, skip := windowsRefusalSkip("windows", malformed); skip {
		t.Fatalf("windows: %v skipped", malformed)
	}
}

// PRS-V1-022 (V1-2484): the docs corpus and receipt-read platform refusals, as
// their callers render them, skip on Windows only; the input-unavailable code
// they replaced does not.
func TestDocsCorpusAndReceiptRefusalsSkipOnlyOnWindows(t *testing.T) {
	corpus := &doccorpus.Error{Code: doccorpus.PlatformUnsupportedCode, Message: "docs corpus reads are unsupported on this platform"}
	receipt := fmt.Errorf("cannot read --receipt: %w", testvaliditydoc.ErrPlatformUnsupported)
	location := &testvaliditydoc.DiscoveryError{Code: "invalid-test-evidence-location", Message: testvaliditydoc.ErrPlatformUnsupported.Error()}
	for _, err := range []error{corpus, receipt, location} {
		for _, goos := range []string{"darwin", "linux"} {
			if reason, skip := windowsRefusalSkip(goos, err); skip || reason != "" {
				t.Fatalf("%s: %v skipped with %q, want no skip on a supported platform", goos, err, reason)
			}
		}
		if _, skip := windowsRefusalSkip("windows", err); !skip {
			t.Fatalf("windows: %v did not skip", err)
		}
	}
	opaque := &doccorpus.Error{Code: "corpus-input-unavailable", Message: "input missing, unsafe or over bound"}
	if _, skip := windowsRefusalSkip("windows", opaque); skip {
		t.Fatalf("windows: %v skipped without the platform refusal", opaque)
	}
}
