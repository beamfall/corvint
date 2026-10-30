//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestDSGV0012_HandoverFailureAndInterruption covers the handover paths
// that need no second build (GitHub #1815): a worker records the build that
// launched it; a pending request ends Run; a failed exec leaves this
// dispatcher running on its own build with the worker untouched; a request
// whose build changed is refused; and a start that finds an EXEC phase the
// old process never completed reports the interrupted upgrade, discards a
// stale request and adopts the recorded worker.
func TestDSGV0012_HandoverFailureAndInterruption(t *testing.T) {
	c := testConfig(t, "sleep 30")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("tick: %v running %d", err, d.Running())
	}
	w := d.ledger.Workers[0]
	self := SelfBuild()
	if got := WorkerBuild(d.dir, w.ID); got != self || got.Sha256 == "" {
		t.Fatalf("worker build = %+v, want %+v (DSG-V0-010)", got, self)
	}

	// Without EnableUpgrade a request is ignored.
	target := c.Hosts["sh"].Argv[0]
	sum, err := BuildDigest(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteUpgradeRequest(d.dir, UpgradeRequest{Build: target, Sha256: sum}); err != nil {
		t.Fatal(err)
	}
	if err := WriteUpgradeRequest(d.dir, UpgradeRequest{Build: target, Sha256: sum}); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second request: %v, want ErrExist", err)
	}
	if tmps, _ := filepath.Glob(filepath.Join(d.dir, ".tmp-upgrade-*")); len(tmps) != 0 {
		t.Fatalf("a refused request left %v", tmps)
	}
	if err := d.Run(ctx, 1); err != nil {
		t.Fatalf("run without upgrade enabled: %v", err)
	}

	if err := d.EnableUpgrade([]string{"dispatch", "--program", "prog"}); err != nil {
		t.Fatal(err)
	}
	st, err := ReadUpgradeState(d.dir)
	if err != nil || st.Phase != UpgradeRunning || st.PID != os.Getpid() || st.Sha256 != self.Sha256 {
		t.Fatalf("enabled state: %+v %v", st, err)
	}
	if err := d.Run(ctx, 1); !errors.Is(err, ErrUpgradeRequested) {
		t.Fatalf("run with a pending request: %v", err)
	}
	var gotPath string
	var gotArgv, gotEnv []string
	d.execImage = func(path string, argv, env []string) error {
		gotPath, gotArgv, gotEnv = path, argv, env
		if st, err := ReadUpgradeState(d.dir); err != nil || st.Phase != UpgradeExec || st.Sha256 != sum || st.FromSha256 != self.Sha256 {
			t.Errorf("state at exec: %+v %v", st, err)
		}
		return errors.New("exec refused")
	}
	if err := d.Handover(ctx); err == nil {
		t.Fatal("a failed exec returned no error")
	}
	if gotPath != target || !slices.Equal(gotArgv, []string{target, "dispatch", "--program", "prog"}) {
		t.Fatalf("exec %q %q", gotPath, gotArgv)
	}
	if !slices.ContainsFunc(gotEnv, func(kv string) bool { return strings.HasPrefix(kv, handoverEnv+"=") }) {
		t.Fatal("exec env names no handed-over lock")
	}
	if st, err := ReadUpgradeState(d.dir); err != nil || st.Phase != UpgradeFailed || st.Sha256 != self.Sha256 {
		t.Fatalf("failed state: %+v %v", st, err)
	}
	if len(eventsWith(t, d, "alert", "code", "UPGRADE_FAILED")) != 1 || len(eventsWith(t, d, "upgrading", "toSha256", sum)) != 1 {
		t.Fatal("missing upgrading or UPGRADE_FAILED event")
	}
	if _, err := os.Lstat(UpgradeRequestPath(d.dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("request not consumed: %v", err)
	}
	if err := d.Run(ctx, 1); err != nil || d.Running() != 1 || d.ledger.Workers[0].PID != w.PID {
		t.Fatalf("after a failed exec: %v running %d", err, d.Running())
	}

	// A request whose build no longer has its digest is refused.
	if err := WriteUpgradeRequest(d.dir, UpgradeRequest{Build: target, Sha256: strings.Repeat("0", 64)}); err != nil {
		t.Fatal(err)
	}
	if err := d.Handover(ctx); err == nil || len(eventsWith(t, d, "alert", "code", "UPGRADE_BUILD_CHANGED")) != 1 {
		t.Fatalf("changed build: %v", err)
	}

	// A configuration the new image could not start on here is refused.
	d.configRead = func() ([]byte, fs.FileInfo, error) { return nil, nil, errors.New("config gone") }
	if err := WriteUpgradeRequest(d.dir, UpgradeRequest{Build: target, Sha256: sum}); err != nil {
		t.Fatal(err)
	}
	if err := d.Handover(ctx); err == nil || len(eventsWith(t, d, "alert", "code", "UPGRADE_CONFIG_CHANGED")) != 1 {
		t.Fatalf("changed config: %v", err)
	}
	d.configRead = nil

	// A process that recorded EXEC and died before the new image started:
	// the next start (another pid, no inherited lock) reports it once.
	st, _ = ReadUpgradeState(d.dir)
	st.Phase, st.PID = UpgradeExec, 0x7ffffff0
	if err := writeUpgradeState(d.dir, st); err != nil {
		t.Fatal(err)
	}
	if err := WriteUpgradeRequest(d.dir, UpgradeRequest{Build: target, Sha256: sum}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d2, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	if len(eventsWith(t, d2, "alert", "code", "UPGRADE_INTERRUPTED")) != 1 || len(eventsWith(t, d2, "alert", "code", "UPGRADE_REQUEST_DISCARDED")) != 1 {
		t.Fatal("missing UPGRADE_INTERRUPTED or UPGRADE_REQUEST_DISCARDED")
	}
	if st, err := ReadUpgradeState(d2.dir); err != nil || st.Phase != UpgradeInterrupted {
		t.Fatalf("interrupted state: %+v %v", st, err)
	}
	if d2.Running() != 1 || d2.ledger.Workers[0].PID != w.PID || d2.ledger.Workers[0].LeaderIdentity != w.LeaderIdentity {
		t.Fatalf("the interrupted upgrade lost the worker: %+v", d2.ledger.Workers)
	}
	if _, err := os.Lstat(UpgradeRequestPath(d2.dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale request kept")
	}
}
