package jstestprovider

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// fakeExternalRun runs runExternal against a stand-in npx (no Playwright, no
// browser) and an httptest server that is always ready.
func fakeExternalRun(t *testing.T, script string, mutate func(*E2EConfig)) Receipt {
	t.Helper()
	bin, repo := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(bin, "npx"), "#!/bin/sh\n"+script+"\n")
	if err := os.Chmod(filepath.Join(bin, "npx"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	for _, name := range []string{"test.js", "config.js", "package.json", "lock.json"} {
		writeFile(t, filepath.Join(repo, name), "{}")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ready")) }))
	defer server.Close()
	cfg := E2EConfig{Config: Config{Dir: repo, TestFiles: []string{filepath.Join(repo, "test.js")}, ConfigFile: filepath.Join(repo, "config.js"),
		PackageJSON: filepath.Join(repo, "package.json"), Lockfile: filepath.Join(repo, "lock.json"), RunnerName: "playwright", RunnerVersion: "1.60.0", Timeout: 20 * time.Second},
		ExternalServer: true, AppIdentity: "fixture", ServerReadyURL: server.URL, ServerReadyLimit: 5 * time.Second}
	if mutate != nil {
		mutate(&cfg)
	}
	receipt, err := runExternal(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.External == nil || receipt.External.Runner == nil {
		t.Fatalf("PWP-V0-029: external receipt has no runner record: %+v", receipt.External)
	}
	return receipt
}

// PWP-V0-028, PWP-V0-029: console output past the 4 MiB bound is truncated,
// the runner runs to its own exit, and the tee receives every byte.
func TestPWPV0028ExternalConsoleOverBoundTruncatesAndTees(t *testing.T) {
	const size = externalOutputLimit + 1<<20
	script := "/usr/bin/head -c " + strconv.Itoa(size) + " /dev/zero\necho to-stderr >&2\nexit 0"
	var tee bytes.Buffer
	receipt := fakeExternalRun(t, script, func(cfg *E2EConfig) { cfg.RunnerOutput = &tee })
	runner := receipt.External.Runner
	if !runner.ExitObserved || runner.ExitStatus != 0 || runner.StopSignal != "" || runner.Signal != "" || !runner.OutputTruncated {
		t.Fatalf("runner record = %+v", runner)
	}
	if receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "report-not-written" {
		t.Fatalf("over-bound console must not fail the run, infrastructure = %+v", receipt.Infrastructure)
	}
	if tee.Len() != size+len("to-stderr\n") || !bytes.HasSuffix(tee.Bytes(), []byte("to-stderr\n")) {
		t.Fatalf("tee received %d bytes", tee.Len())
	}
}

// PWP-V0-028: without a tee nothing beyond the bounded capture is written and
// the record still reports the truncation.
func TestPWPV0028ExternalConsoleWithoutTee(t *testing.T) {
	receipt := fakeExternalRun(t, "/usr/bin/head -c "+strconv.Itoa(externalOutputLimit+10)+" /dev/zero >&2\nexit 4", nil)
	runner := receipt.External.Runner
	if runner.ExitStatus != 4 || !runner.OutputTruncated || receipt.Infrastructure == nil || receipt.Infrastructure.Reason == "output-overflow" {
		t.Fatalf("runner=%+v infrastructure=%+v", runner, receipt.Infrastructure)
	}
}

// PWP-V0-030, PWP-V0-029: a timed-out runner gets SIGINT first; one that exits
// on it records SIGINT and keeps the timeout failure.
func TestPWPV0030ExternalTimeoutInterruptsRunnerFirst(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "interrupted")
	t.Setenv("CORVINT_TEST_MARK", mark)
	script := `trap '/usr/bin/touch "$CORVINT_TEST_MARK"; exit 130' INT; trap 'exit 143' TERM; while :; do /bin/sleep 0.05; done`
	receipt := fakeExternalRun(t, script, func(cfg *E2EConfig) {
		cfg.Timeout = 300 * time.Millisecond
		cfg.InterruptGrace = 5 * time.Second
	})
	if _, err := os.Stat(mark); err != nil {
		t.Fatalf("runner did not see SIGINT: %v", err)
	}
	runner := receipt.External.Runner
	if runner.StopSignal != "SIGINT" || runner.ExitStatus != 130 || !runner.ExitObserved {
		t.Fatalf("runner record = %+v", runner)
	}
	if receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "timeout" {
		t.Fatalf("interrupted timeout must stay a timeout: %+v", receipt.Infrastructure)
	}
}

// PWP-V0-030: with no grace the earlier SIGTERM-first stop is unchanged.
func TestPWPV0030ExternalZeroGraceTerminatesFirst(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "interrupted")
	t.Setenv("CORVINT_TEST_MARK", mark)
	script := `trap '/usr/bin/touch "$CORVINT_TEST_MARK"; exit 130' INT; while :; do /bin/sleep 0.05; done`
	receipt := fakeExternalRun(t, script, func(cfg *E2EConfig) { cfg.Timeout = 300 * time.Millisecond })
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("zero grace sent SIGINT")
	}
	if runner := receipt.External.Runner; runner.StopSignal != "SIGTERM" || runner.ExitStatus != -1 || runner.Signal != "terminated" {
		t.Fatalf("runner record = %+v", runner)
	}
}

// PWP-V0-029: the runner record is a closed shape.
func TestPWPV0029RunnerRecordShape(t *testing.T) {
	r := qualifiedFixture(t)
	for _, valid := range []RunnerProcess{{ExitObserved: true}, {ExitStatus: -1, StopSignal: "SIGKILL", Signal: "killed", ExitObserved: true}, {ExitStatus: -1}} {
		r.External.Runner = &valid
		if err := runnerProcessShapeError(r); err != nil {
			t.Fatalf("%+v: %v", valid, err)
		}
	}
	for _, invalid := range []RunnerProcess{{StopSignal: "SIGHUP", ExitStatus: -1}, {ExitStatus: 2}, {ExitObserved: true, ExitStatus: 2, Signal: "killed"}, {ExitObserved: true, ExitStatus: -2}} {
		r.External.Runner = &invalid
		if err := runnerProcessShapeError(r); err == nil || err.Error() != "runner-process-invalid" {
			t.Fatalf("%+v accepted: %v", invalid, err)
		}
	}
}

// PWP-V0-031: the readiness rules the --server-ready-url help states. An owned
// server is ready below 500 with redirects followed; an external server only
// on 2xx with no redirect followed.
func TestPWPV0031ReadinessRulesByMode(t *testing.T) {
	for _, test := range []struct {
		status          int
		owned, external bool
	}{{200, true, true}, {204, true, true}, {302, true, false}, {404, true, false}, {500, false, false}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if test.status == http.StatusFound && r.URL.Path == "/" {
				http.Redirect(w, r, "/missing", http.StatusFound)
				return
			}
			w.WriteHeader(test.status)
		}))
		owned := waitReady(context.Background(), server.URL, 200*time.Millisecond) == nil
		external := externalReady(context.Background(), server.URL, 200*time.Millisecond) == nil
		server.Close()
		if owned != test.owned || external != test.external {
			t.Fatalf("status %d: owned=%v external=%v", test.status, owned, external)
		}
	}
}
