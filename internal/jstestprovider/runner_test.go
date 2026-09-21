//go:build darwin || linux

package jstestprovider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// TestRunE2E_CancellationSetsInfrastructure confirms an interrupted e2e run
// reports Infrastructure alongside Cancelled, mirroring RunUnit's
// unitBoundaryFailure cancelled case - otherwise the CLI's emit (which only
// checks Infrastructure != nil) would exit 0 on an interrupted e2e run while
// exiting 1 on an interrupted unit run.
func TestRunE2E_CancellationSetsInfrastructure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct {
		receipt Receipt
		err     error
	}, 1)
	go func() {
		receipt, err := RunE2E(ctx, E2EConfig{
			Config:     Config{Dir: t.TempDir(), RunnerName: "playwright", Timeout: 5 * time.Second},
			ServerArgv: []string{"/bin/sleep", "60"},
			TestArgv:   []string{"/bin/sleep", "60"},
		})
		done <- struct {
			receipt Receipt
			err     error
		}{receipt, err}
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("unexpected error: %v", result.err)
		}
		if !result.receipt.Cancelled {
			t.Fatalf("want Cancelled=true, got %+v", result.receipt)
		}
		if result.receipt.Infrastructure == nil {
			t.Fatalf("want non-nil Infrastructure on a cancelled e2e run, got %+v", result.receipt)
		}
		if result.receipt.Infrastructure.Reason != "cancelled" {
			t.Fatalf("want reason cancelled, got %q", result.receipt.Infrastructure.Reason)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunE2E did not return after cancellation")
	}
}

func TestRunE2E_AttestationRequiresExternalServerBeforeExecution(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	receipt, err := RunE2E(context.Background(), E2EConfig{
		Config:                 Config{Dir: t.TempDir(), RunnerName: "playwright"},
		ServerArgv:             []string{"/usr/bin/touch", marker},
		ApplicationAttestation: &ApplicationAttestationProvider{Argv: []string{"/bin/false"}, ConfigFile: "/missing"},
	})
	if err == nil || err.Error() != "application-attestation-requires-external-server" || receipt.Kind != "" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("managed server was started: %v", statErr)
	}
}

// TestRunUnit_StaleCallerOutputFileIsNotReadBack runs a stand-in npx that
// exits 0 without writing a report into a caller-supplied OutputFile that
// already holds an earlier run's report. The earlier report must not be
// published as this run's outcomes.
func TestRunUnit_StaleCallerOutputFileIsNotReadBack(t *testing.T) {
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "npx"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(bin, "npx"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	stale, err := os.ReadFile("testdata/vitest-mixed.json")
	if err != nil {
		t.Fatal(err)
	}
	outputFile := filepath.Join(t.TempDir(), "report.json")
	writeFile(t, outputFile, string(stale))

	receipt, err := RunUnit(context.Background(), UnitConfig{
		Config:     Config{Dir: t.TempDir(), RunnerName: "vitest", Timeout: 20 * time.Second},
		OutputFile: outputFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receipt.Tests) != 0 || receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "report-not-written" {
		t.Fatalf("want report-not-written and no outcomes, got tests=%d infrastructure=%+v", len(receipt.Tests), receipt.Infrastructure)
	}
}

// TestRunUnit_UnhandledErrorOutsideTestIsNotSilentlyGreen reproduces a real
// Vitest 5.0.0 run (empirically captured: a test that throws from a
// setTimeout callback after its own assertion passes) whose JSON report is
// entirely green - numFailedTests=0, every testResults[].status "passed",
// top-level success:true - while the process itself exits 1. Vitest's own
// success computation (node_modules/vitest/dist/chunks/index.B89dZ0-N.js:17814,
// `numFailedTestSuites === 0 && numFailedTests === 0`) never observes an
// error raised outside a test's own execution window, so the report carries
// no signal at all; only the exit status does. Per AGENTS.md invariant 2,
// this must abstain (infrastructure) rather than publish an all-passed
// receipt.
func TestRunUnit_UnhandledErrorOutsideTestIsNotSilentlyGreen(t *testing.T) {
	report, err := os.ReadFile("testdata/vitest-unhandled-error.json")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	outputFile := filepath.Join(t.TempDir(), "report.json")
	script := "#!/bin/sh\n/bin/cp '" + outputFile + ".src' '" + outputFile + "'\nexit 1\n"
	writeFile(t, outputFile+".src", string(report))
	writeFile(t, filepath.Join(bin, "npx"), script)
	if err := os.Chmod(filepath.Join(bin, "npx"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	receipt, err := RunUnit(context.Background(), UnitConfig{
		Config:     Config{Dir: t.TempDir(), RunnerName: "vitest", Timeout: 20 * time.Second},
		OutputFile: outputFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, test := range receipt.Tests {
		if test.State == StateFailed || test.State == StateInfrastructure {
			t.Fatalf("fixture is all-passed by construction, got failing outcome %+v", test)
		}
	}
	if receipt.Infrastructure == nil {
		t.Fatalf("want a non-nil Infrastructure abstention for an all-passed report backed by a nonzero exit, got %+v", receipt)
	}
}

// TestE2EBoundaryFailure_WaitNotCompleted mirrors unitBoundaryFailure's
// wait-not-completed case for the Playwright test process: a started run
// whose wait did not complete is an infrastructure failure, never parsed.
func TestE2EBoundaryFailure_WaitNotCompleted(t *testing.T) {
	failure := e2eBoundaryFailure(procgroup.Observation{Started: true, Stdout: []byte("{}")})
	if failure == nil || failure.Reason != "wait-not-completed" {
		t.Fatalf("want wait-not-completed, got %+v", failure)
	}
}
