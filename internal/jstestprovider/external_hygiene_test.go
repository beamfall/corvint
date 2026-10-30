package jstestprovider

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/runhygiene"
)

// browserLogTailMessage is the shape Playwright reports for a `/3` test whose action was still
// pending when the test timed out (V1-2620, CI runs 40163191708, 40196868668 and 40202373049):
// the reporter joins the timeout error with the action's error, whose "Browser logs" tail is the
// browser launch command line, including `--password-store=basic`.
const browserLogTailMessage = "\x1b[31mTest timeout of 30000ms exceeded.\x1b[39m\n" +
	"Error: locator.click: Test timeout of 30000ms exceeded.\n" +
	"Browser logs:\n\n" +
	"<launching> /Users/runner/Library/Caches/ms-playwright/chromium_headless_shell-1248/chrome-headless-shell-mac-arm64/chrome-headless-shell --disable-field-trial-config --no-first-run --password-store=basic --use-mock-keychain --headless --remote-debugging-pipe\n" +
	"<launched> pid=4242\n" +
	"Call log:\n" +
	"\x1b[2m  - waiting for getByTestId('slot').first()\x1b[22m\n" +
	"\x1b[2m    - waiting for element to be visible, enabled and stable\x1b[22m\n"

func browserLogTailOutcome() TestOutcome {
	anchor := &Anchor{File: "/fixture/e2e/book.spec.ts", Line: 8}
	artifacts := []FailureArtifact{{Name: "error-context", Path: "/tmp/corvint-playwright-1/output/book-chromium/error-context.md"}, {Name: "trace", Path: "/tmp/corvint-playwright-1/output/book-chromium/trace.zip"}}
	return TestOutcome{Name: "book", FullName: "book", State: StateTimedOut, Retries: 0, DurationMS: 30000, Anchor: anchor,
		FailureMessage: browserLogTailMessage, Artifacts: artifacts,
		Attempts:       []Attempt{{State: StateTimedOut, Retry: 0}},
		AttemptDetails: []AttemptDetail{{State: StateTimedOut, Retry: 0, DurationMS: 30000, FailureMessage: browserLogTailMessage, Anchor: anchor, Artifacts: artifacts}}}
}

// PWP-V3-010: the external row applies the native row's run-evidence hygiene before projection,
// so a browser launch command line in a timed-out action's message no longer refuses the
// qualified document, and the `/3` attempt entry stays equal to the last-attempt fields.
func TestPWPV3010ExternalBrowserLogTailScreened(t *testing.T) {
	unscreened := Receipt{Profile: AttemptExternalProfile, Tests: []TestOutcome{browserLogTailOutcome()}}
	if _, err := EncodeQualified(unscreened); err == nil || err.Error() != "qualified-document-secret-shaped" {
		t.Fatalf("the reporter's message must be secret-shaped as written, got %v", err)
	}
	tests := []TestOutcome{browserLogTailOutcome()}
	externalHygiene(tests)
	o := tests[0]
	if strings.Contains(o.FailureMessage, "password-store") || strings.Contains(o.FailureMessage, "<launching>") {
		t.Fatalf("launch command line kept: %q", o.FailureMessage)
	}
	for _, want := range []string{"Test timeout of 30000ms exceeded.", "Error: locator.click: Test timeout of 30000ms exceeded.", runhygiene.DroppedMarker, "waiting for getByTestId('slot').first()"} {
		if !strings.Contains(o.FailureMessage, want) {
			t.Fatalf("message lost %q: %q", want, o.FailureMessage)
		}
	}
	if o.AttemptDetails[0].FailureMessage != o.FailureMessage || !reflect.DeepEqual(o.AttemptDetails[0].Artifacts, o.Artifacts) || len(o.Artifacts) != 2 {
		t.Fatalf("attempt entry diverged from the last-attempt fields: %+v", o)
	}
	if err := ValidateAttemptDetails(tests); err != nil {
		t.Fatalf("PWP-V3-002 equality after hygiene: %v", err)
	}
	if _, err := EncodeQualified(Receipt{Profile: AttemptExternalProfile, Tests: tests}); err != nil {
		t.Fatalf("screened document refused: %v", err)
	}
	// Hygiene is idempotent, so a receipt re-read through the same row is unchanged.
	again := []TestOutcome{o}
	externalHygiene(again)
	if !reflect.DeepEqual(again[0], o) {
		t.Fatalf("second pass changed the outcome: %+v", again[0])
	}
}
