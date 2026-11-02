//go:build unix

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

func TestLiveVerdicts(t *testing.T) {
	t.Run("HLQ-V1-013 a host that never dispatched fails dispatch even when the harness ran the hook", func(t *testing.T) {
		s := liveSession{hostPID: 42, records: []liveRecord{{Phase: "start", PID: 7, Event: "user-prompt", Ancestry: []int{1000}}}}
		if status, detail := dispatchStatus(s); status != "FAIL" || !strings.Contains(detail, "dispatched=[]") {
			t.Fatalf("dispatch = %s %s", status, detail)
		}
		s.records[0].Ancestry = []int{99, 42}
		if status, detail := dispatchStatus(s); status != "PASS" || !strings.Contains(detail, "dispatched=[user-prompt]") {
			t.Fatalf("dispatch = %s %s", status, detail)
		}
	})
	t.Run("HLQ-V1-013 a held invocation that released itself or survived fails timeout and cancellation", func(t *testing.T) {
		held := liveRecord{Phase: "held", PID: 7, Event: "user-prompt", Ancestry: []int{42}}
		ended := liveSession{hostPID: 42, records: []liveRecord{held}, interrupted: true}
		if status, _ := timeoutStatus(ended); status != "PASS" {
			t.Fatalf("timeout = %s", status)
		}
		if status, _ := cancellationStatus(ended); status != "PASS" {
			t.Fatalf("cancellation = %s", status)
		}
		released := ended
		released.records = append([]liveRecord{held}, liveRecord{Phase: "released", PID: 7})
		survived := ended
		survived.heldAlive = true
		uninterrupted := ended
		uninterrupted.interrupted = false
		for name, s := range map[string]liveSession{"released": released, "survived": survived} {
			if status, _ := timeoutStatus(s); status != "FAIL" {
				t.Fatalf("%s timeout = %s", name, status)
			}
			if status, _ := cancellationStatus(s); status != "FAIL" {
				t.Fatalf("%s cancellation = %s", name, status)
			}
		}
		if status, _ := cancellationStatus(uninterrupted); status != "FAIL" {
			t.Fatalf("uninterrupted cancellation = %s", status)
		}
	})
	t.Run("HLQ-V1-014 dispatch verdict needs every live case and authority is never PASS", func(t *testing.T) {
		all := []result{{"dispatch", "PASS", ""}, {"timeout", "PASS", ""}, {"cancellation", "PASS", ""}, {"cleanup", "PASS", ""}}
		if got := liveDispatchVerdict(all); got != "PASS" {
			t.Fatalf("verdict = %s", got)
		}
		if got := liveDispatchVerdict(all[:3]); got != "NOT_RUN" {
			t.Fatalf("verdict = %s", got)
		}
		if got := liveDispatchVerdict(append(all[:3:3], result{"cleanup", "FAIL", ""})); got != "FAIL" {
			t.Fatalf("verdict = %s", got)
		}
		source := t.TempDir()
		os.MkdirAll(filepath.Join(source, "integrations"), 0o755)
		os.WriteFile(filepath.Join(source, "integrations/compatibility.json"), []byte(`{"entries":[{"host":"opencode","status":"FALLBACK"}]}`), 0o644)
		if got, _ := authorityVerdict(source, "opencode"); got != "FALLBACK" {
			t.Fatalf("authority = %s", got)
		}
		if got, _ := authorityVerdict(source, "unknown-host"); got != "UNSUPPORTED" {
			t.Fatalf("authority = %s", got)
		}
	})
	t.Run("HLQ-V1-015 an install-only run never yields a dispatch verdict", func(t *testing.T) {
		r := &runner{host: "claude-code", source: t.TempDir(), results: []result{{"install", "PASS", ""}, {"discovery", "PASS", ""}, {"upgrade", "PASS", ""}, {"uninstall", "PASS", ""}}}
		text := r.render()
		for _, want := range []string{"verdict\tinstallation\tPASS\t", "verdict\tdispatch\tNOT_RUN\tinstall-only evidence", "verdict\tauthority\tUNSUPPORTED\t"} {
			if !strings.Contains(text, want) {
				t.Fatalf("report lacks %q:\n%s", want, text)
			}
		}
	})
}

// TestLiveFixtureRun drives the live mode against a fake host that invokes the configured wrapper
// in its own process tree but enforces no timeout: dispatch and cancellation pass, timeout fails,
// so the dispatch verdict is FAIL and no paid or real host is involved (HLQ-V1-013, HLQ-V1-014).
func TestLiveFixtureRun(t *testing.T) {
	if _, err := exec.LookPath("perl"); err != nil {
		t.Skip("perl unavailable for wrapper timestamps")
	}
	directory := t.TempDir()
	host := filepath.Join(directory, "fakehost")
	execfile.WriteFile(host, []byte(`#!/bin/sh
if [ "$1" = "--version" ]; then echo "fakehost 9.8.7"; exit 0; fi
w=$(sed -n 's/.*"corvintBinary": *"\([^"]*\)".*/\1/p' opencode.json)
"$w" --root . harness event --event user-prompt --input - </dev/null
"$w" --root . harness event --event stop --input - </dev/null
exit 0
`), 0o755)
	corvint := filepath.Join(directory, "corvint")
	execfile.WriteFile(corvint, []byte("#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'Corvint 0.0.0-fixture'; fi\nexit 0\n"), 0o755)
	source := liveFixtureSource(t)
	l := &liveRunner{host: "opencode", hold: 2 * time.Second, grace: time.Second, bound: 30 * time.Second, pids: map[int]int{}, corvint: corvint, source: source, work: t.TempDir()}
	if err := l.prepare(host); err != nil {
		t.Fatal(err)
	}
	l.runOpenCode()
	text := l.render()
	for _, want := range []string{"hostVersion=9.8.7", "case\tdispatch\tPASS\t", "dispatched=[user-prompt,stop]", "case\ttimeout\tFAIL\t", "did not terminate the held invocation", "case\tcancellation\tPASS\t", "case\tcleanup\tPASS\t", "verdict\tinstallation\tNOT_RUN\t", "verdict\tdispatch\tFAIL\t", "verdict\tauthority\tFALLBACK\t"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report lacks %q:\n%s", want, text)
		}
	}
}

// liveFixtureSource is a source checkout holding the two repository files the live mode reads
// (the adapter manifest and the compatibility manifest), committed in its own git repository.
// A test confined to its declared read scopes cannot read the repository's .git, so the live
// mode's `git rev-parse HEAD` runs against this copy instead.
func liveFixtureSource(t *testing.T) string {
	t.Helper()
	repository, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	for _, relative := range []string{"integrations/opencode/package.json", "integrations/compatibility.json"} {
		data, err := os.ReadFile(filepath.Join(repository, relative))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(source, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, argv := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=HLQ", "-c", "user.email=hlq@localhost", "-c", "commit.gpgsign=false", "commit", "-qm", "source"}} {
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, argv...)...)
		command.Dir = source
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", argv, err, output)
		}
	}
	return source
}

func TestLiveUnsupportedHostIsNotRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	source, _ := filepath.Abs("../..")
	if code := run([]string{"--live", "--host", "claude-code", "--corvint", "/nonexistent", "--source", source}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	for _, want := range []string{"case\tdispatch\tNOT_RUN\tno model-free", "verdict\tdispatch\tNOT_RUN\t", "verdict\tauthority\tFALLBACK\t"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report lacks %q:\n%s", want, stdout.String())
		}
	}
}
