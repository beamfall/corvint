package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// observeE2EConfig runs the e2e flag parsing and returns the config RunE2E
// would receive, without running anything.
func observeE2EConfig(t *testing.T, args ...string) (jstestprovider.E2EConfig, error) {
	t.Helper()
	var observed jstestprovider.E2EConfig
	stop := errors.New("stopped before running")
	previous := runE2EReceipt
	t.Cleanup(func() { runE2EReceipt = previous })
	runE2EReceipt = func(_ context.Context, cfg jstestprovider.E2EConfig) (jstestprovider.Receipt, error) {
		observed = cfg
		return jstestprovider.Receipt{}, stop
	}
	err := runE2E(args)
	if errors.Is(err, stop) {
		err = nil
	}
	return observed, err
}

// PWP-V0-028, PWP-V0-030: the runner flags are external-only, the grace has a
// documented 10s default, and a negative grace refuses.
func TestPWPV0030InterruptGraceFlag(t *testing.T) {
	for _, args := range [][]string{{"--runner-output", "-"}, {"--interrupt-grace", "1s"}} {
		if _, err := observeE2EConfig(t, args...); err == nil || err.Error() != "runner-control-requires-external-server" {
			t.Fatalf("%v outside external mode: %v", args, err)
		}
	}
	if _, err := observeE2EConfig(t, "--external-server", "--interrupt-grace=-1s"); err == nil || err.Error() != "interrupt-grace-invalid" {
		t.Fatalf("negative grace: %v", err)
	}
	for want, args := range map[time.Duration][]string{
		defaultInterruptGrace:   {"--external-server"},
		0:                       {"--external-server", "--interrupt-grace", "0"},
		1500 * time.Millisecond: {"--external-server", "--interrupt-grace", "1.5s"},
	} {
		cfg, err := observeE2EConfig(t, args...)
		if err != nil || cfg.InterruptGrace != want || cfg.RunnerOutput != nil {
			t.Fatalf("%v: grace=%s output=%v err=%v", args, cfg.InterruptGrace, cfg.RunnerOutput, err)
		}
	}
	if defaultInterruptGrace != 10*time.Second {
		t.Fatalf("documented default changed: %s", defaultInterruptGrace)
	}
	if cfg, err := observeE2EConfig(t); err != nil || cfg.InterruptGrace != 0 {
		t.Fatalf("owned-server mode carries a grace: %s %v", cfg.InterruptGrace, err)
	}
}

// PWP-V0-028: --runner-output names a file the runner console is copied to.
func TestPWPV0028RunnerOutputFlagWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runner.log")
	if err := os.WriteFile(path, []byte("earlier run"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := observeE2EConfig(t, "--external-server", "--runner-output", path)
	if err != nil || cfg.RunnerOutput == nil {
		t.Fatalf("runner output not wired: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || len(data) != 0 {
		t.Fatalf("runner output was not truncated: %q %v", data, err)
	}
	if _, err := observeE2EConfig(t, "--external-server", "--runner-output", filepath.Join(t.TempDir(), "missing", "runner.log")); err == nil || !strings.HasPrefix(err.Error(), "runner-output-unavailable") {
		t.Fatalf("unopenable runner output: %v", err)
	}
	var stderr bytes.Buffer
	output, closeOutput, err := openRunnerOutput(path, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("console\n")); err != nil {
		t.Fatal(err)
	}
	closeOutput()
	if data, _ := os.ReadFile(path); string(data) != "console\n" || stderr.Len() != 0 {
		t.Fatalf("file=%q stderr=%q", data, stderr.String())
	}
	failing := &runnerOutput{w: failingSink{}}
	if _, err := failing.Write([]byte("x")); err == nil {
		t.Fatal("failing sink accepted a write")
	}
	if _, err := failing.Write([]byte("y")); !errors.Is(err, errRunnerOutputStopped) {
		t.Fatalf("stopped sink: %v", err)
	}
	reportRunnerOutput(failing, nil, &stderr)
	if !strings.HasPrefix(stderr.String(), "runner-output-incomplete") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

type failingSink struct{}

func (failingSink) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// PWP-V0-031: the readiness help states each mode's rule as the code applies it.
func TestPWPV0031ServerReadyURLHelpMatchesReadiness(t *testing.T) {
	var usage bytes.Buffer
	printUsage(&usage)
	text := usage.String()
	for _, want := range []string{
		"without --external-server the owned server is ready\non any status below 500 and redirects are followed",
		"with --external-server only a 2xx answer is\nready and a redirect is not followed",
		"owned server, any status < 500 (redirects followed); --external-server, 2xx only (no redirects)",
		"URL polled until it answers 2xx; a redirect is not followed",
		"--interrupt-grace (default 10s; 0 sends SIGTERM first)",
		"--runner-output PATH",
		".corvint/.gitignore",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("usage omits %q", want)
		}
	}
	if strings.Contains(text, "URL polled until it answers with status < 500") {
		t.Error("usage still promises the owned-server rule for every mode")
	}
}
