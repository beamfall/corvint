package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// writeLedger saves a dispatcher ledger recording the given worker ids.
func (s *serviceHome) writeLedger(t *testing.T, workers ...string) {
	t.Helper()
	m, _ := s.manifest(t)
	dir := dispatchDir(m)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	l := dispatch.Ledger{Profile: dispatch.StateProfile, Program: "site", Workers: []*dispatch.Worker{}, Backoff: map[string]*dispatch.BackoffState{}}
	for _, id := range workers {
		l.Workers = append(l.Workers, &dispatch.Worker{ID: id, Role: "impl", State: "RUNNING"})
	}
	raw, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(dir, "state.json", raw); err != nil {
		t.Fatal(err)
	}
	if got, err := dispatch.LoadLedger(dir, "site"); err != nil || len(got.Workers) != len(workers) {
		t.Fatalf("fixture ledger not loadable: %v", err)
	}
}

func (s *serviceHome) runOptions(t *testing.T, open func(string, *dispatch.Config, dispatch.LaunchControl) (Controller, error)) RunOptions {
	t.Helper()
	root := s.root(t)
	return RunOptions{Host: s.h, Program: "site", Manifest: filepath.Join(root, manifestFile), Executable: s.exe, Open: open, Poll: 5 * time.Millisecond, Pulse: 5 * time.Millisecond, Retry: 5 * time.Millisecond}
}

// launchControl opens the managed main's launch control for a test.
func (s *serviceHome) launchControl(t *testing.T, legacy *string) *launchControl {
	t.Helper()
	_, ident := s.manifest(t)
	lc, err := s.runOptions(t, nil).control(s.root(t), ident, legacy)
	if err != nil {
		t.Fatal(err)
	}
	return lc
}

func (s *serviceHome) intent(t *testing.T) string {
	t.Helper()
	o, err := s.h.Status("site")
	if err != nil {
		t.Fatal(err)
	}
	return field(t, o, "launchIntent")
}

// boundaryController stands in for a controlled dispatcher: its Run reports
// a tick boundary every few milliseconds with the test's recorded and
// settled facts, and returns dispatch.ErrSettled when the control settles it.
type boundaryController struct {
	control           dispatch.LaunchControl
	recorded, settled *atomic.Bool
	closed            chan struct{}
}

func (b *boundaryController) Run(ctx context.Context, _ int) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Millisecond):
		}
		if b.control.Boundary(b.recorded.Load(), b.recorded.Load() && b.settled.Load()) {
			return dispatch.ErrSettled
		}
	}
}
func (b *boundaryController) Close() error {
	b.closed <- struct{}{}
	return nil
}

// SERVICE500-003: an admitted launch holds F, so a concurrent stop cannot
// save its suppression until the launch is recorded and F released; the
// stop then completes ACKNOWLEDGED and later admissions refuse.
func TestSERVICE500_FenceSerializesStopWithAdmittedLaunch(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	lc := s.launchControl(t, nil)
	release, err := lc.Admit("site.impl.0.a-1")
	if err != nil {
		t.Fatalf("RUNNING control refused admission: %v", err)
	}
	type answer struct {
		out *wire.Object
		err error
	}
	done := make(chan answer, 1)
	go func() {
		out, err := s.h.Stop("site", "stop-1", false)
		done <- answer{out, err}
	}()
	select {
	case a := <-done:
		t.Fatalf("stop completed while a launch held the fence: %v", a.err)
	case <-time.After(100 * time.Millisecond):
	}
	if s.control(t).Desired != "RUNNING" {
		t.Fatal("stop saved suppression while a launch held the fence")
	}
	release(true)
	var a answer
	select {
	case a = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("stop did not progress after the fence was released")
	}
	if a.err != nil || field(t, a.out, "state") != "ACKNOWLEDGED" || field(t, a.out, "desired") != "STOPPED" {
		t.Fatalf("stop after fence release: %v", a.err)
	}
	if s.intent(t) != "ABSENT" {
		t.Fatal("a recorded launch left its intent")
	}
	if _, err := lc.Admit("site.impl.0.a-2"); !errors.Is(err, errAdmission) {
		t.Fatalf("admission after STOPPED: %v", err)
	}
}

// SERVICE500-003: a launch released unrecorded (its ledger save failed
// after spawn) leaves a durable UNRESOLVED intent: no launch is admitted,
// a stop is not acknowledged and a drain does not settle until a recorded
// tick boundary of the same dispatcher resolves it.
func TestSERVICE500_UnrecordedLaunchBlocksAckAndSettlement(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	lc := s.launchControl(t, nil)
	release, err := lc.Admit("site.impl.0.a-1")
	if err != nil {
		t.Fatal(err)
	}
	release(false)
	if s.intent(t) != "UNRESOLVED" {
		t.Fatal("an unrecorded launch left no durable intent")
	}
	if _, err := lc.Admit("site.impl.0.a-2"); !errors.Is(err, errAdmission) {
		t.Fatalf("admission with an unresolved intent: %v", err)
	}
	out, err := s.h.Stop("site", "drain-1", true)
	if err != nil || field(t, out, "state") != "PENDING" || field(t, out, "desired") != "DRAINING" {
		t.Fatalf("drain with an unresolved intent: %v", err)
	}
	drained := s.control(t)
	if lc.Boundary(false, true) || s.control(t).Desired != "DRAINING" || s.intent(t) != "UNRESOLVED" {
		t.Fatal("an unrecorded boundary resolved the intent or settled the drain")
	}
	other := s.launchControl(t, nil)
	if other.Boundary(true, true) || s.control(t).Desired != "DRAINING" || s.intent(t) != "UNRESOLVED" {
		t.Fatal("another dispatcher's boundary resolved the intent")
	}
	if lc.Boundary(true, false) || s.control(t).Desired != "DRAINING" || s.intent(t) != "ABSENT" {
		t.Fatal("a recorded unsettled boundary did not resolve the intent alone")
	}
	if !lc.Boundary(true, true) {
		t.Fatal("a recorded settled boundary did not settle the drain")
	}
	if c := s.control(t); c.Desired != "STOPPED" || c.Revision.Int() != drained.Revision.Int()+1 || c.LastRequest != "drain-1" {
		t.Fatalf("settlement was not one CAS step keeping the drain request: %+v", c)
	}
	out, err = s.h.Stop("site", "drain-1", true)
	if err != nil || field(t, out, "replayed") != "true" || field(t, out, "state") != "ACKNOWLEDGED" {
		t.Fatalf("drain replay after resolution: %v", err)
	}
}

// SERVICE500-003: an intent left by a crashed dispatcher is resolved at the
// next open only when the saved ledger records its effect; otherwise it
// stays UNRESOLVED and keeps admission refused and stop PENDING.
func TestSERVICE500_LeftIntentResolvesOnlyFromLedger(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	m, ident := s.manifest(t)
	cfg := &dispatch.Config{StateDir: m.DispatchStateRoot}
	release, err := s.launchControl(t, nil).Admit("site.impl.0.a-1")
	if err != nil {
		t.Fatal(err)
	}
	release(false) // the dispatcher crashed before its ledger save
	s.writeLedger(t)
	o := s.runOptions(t, nil)
	if !o.boundAfterOpen(root, ident, cfg) || s.intent(t) != "UNRESOLVED" {
		t.Fatal("an intent the ledger does not record was resolved")
	}
	if _, err := s.launchControl(t, nil).Admit("site.impl.0.b-1"); !errors.Is(err, errAdmission) {
		t.Fatalf("admission with a left intent: %v", err)
	}
	if out, err := s.h.Stop("site", "stop-1", false); err != nil || field(t, out, "state") != "PENDING" {
		t.Fatalf("stop with a left intent: %v", err)
	}
	if _, err := s.h.Resume("site", "resume-1"); err != nil {
		t.Fatal(err)
	}
	s.writeLedger(t, "site.impl.0.a-1")
	if !o.boundAfterOpen(root, ident, cfg) || s.intent(t) != "ABSENT" {
		t.Fatal("an intent the ledger records was not resolved at open")
	}
	release, err = s.launchControl(t, nil).Admit("site.impl.0.b-1")
	if err != nil {
		t.Fatalf("admission after resolution: %v", err)
	}
	release(true)
}

// SERVICE500-001/003: drain keeps the dispatcher supervising while its
// control refuses launches; only a recorded, settled tick boundary moves
// DRAINING to STOPPED by CAS, which ends the dispatcher without a hold.
func TestSERVICE500_DrainSupervisesThenSettlesStopped(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	closed := make(chan struct{}, 8)
	controls := make(chan dispatch.LaunchControl, 8)
	var recorded, settled atomic.Bool
	recorded.Store(true)
	open := func(_ string, _ *dispatch.Config, control dispatch.LaunchControl) (Controller, error) {
		controls <- control
		return &boundaryController{control: control, recorded: &recorded, settled: &settled, closed: closed}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, s.runOptions(t, open)) }()
	defer func() {
		cancel()
		<-done
	}()
	var control dispatch.LaunchControl
	select {
	case control = <-controls:
	case <-time.After(10 * time.Second):
		t.Fatal("dispatcher did not open")
	}
	waitFor(t, "RUNNING pulse", func() bool { st, _ := s.h.pulseState(root, ""); return st == "RUNNING" })
	out, err := s.h.Stop("site", "drain-1", true)
	if err != nil || field(t, out, "state") != "ACKNOWLEDGED" || field(t, out, "desired") != "DRAINING" || field(t, out, "close") != "PENDING" || field(t, out, "drain") != "true" {
		t.Fatalf("drain: %v", err)
	}
	if _, err := control.Admit("site.impl.0.a-1"); !errors.Is(err, errAdmission) {
		t.Fatalf("draining control admitted a launch: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if s.control(t).Desired != "DRAINING" || len(closed) != 0 {
		t.Fatal("drain settled or closed the dispatcher before a settled boundary")
	}
	_, err = s.h.Stop("site", "drain-2", true)
	codeIs(t, err, wire.CodeResourceCollision)

	drained := s.control(t)
	settled.Store(true)
	waitFor(t, "settled STOPPED", func() bool {
		c, err := s.h.readControl(root)
		return err == nil && c.Desired == "STOPPED"
	})
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("dispatcher not closed after drain settled")
	}
	if c := s.control(t); c.Revision.Int() != drained.Revision.Int()+1 || c.LastRequest != "drain-1" || c.LastRequestSha256 != drained.LastRequestSha256 {
		t.Fatalf("settlement was not one CAS step keeping the drain request: %+v", c)
	}
	waitFor(t, "IDLE pulse", func() bool { st, _ := s.h.pulseState(root, ""); return st == "IDLE" })
	if _, p := s.h.pulseState(root, ""); p == nil || p.Hold != "" {
		t.Fatalf("settlement left a hold: %+v", p)
	}
	out, err = s.h.Stop("site", "drain-1", true)
	if err != nil || field(t, out, "replayed") != "true" || field(t, out, "state") != "ACKNOWLEDGED" || field(t, out, "desired") != "STOPPED" || field(t, out, "close") != "OBSERVED" {
		t.Fatalf("drain replay after settlement: %v", err)
	}
	_, err = s.h.Stop("site", "drain-3", true)
	codeIs(t, err, wire.CodeResourceCollision)
}

// SERVICE500-003: a managed main restarted during a drain reopens to
// supervise without admitting launches, and a concurrent resume wins.
func TestSERVICE500_ResumeDuringDrainWins(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	s.writeLedger(t, "site.impl.0.a-1")
	if out, err := s.h.Stop("site", "drain-1", true); err != nil || field(t, out, "state") != "ACKNOWLEDGED" || field(t, out, "close") != "PENDING" {
		t.Fatalf("drain with no dispatcher: %v", err)
	}
	opened, closed := make(chan struct{}, 8), make(chan struct{}, 8)
	controls := make(chan dispatch.LaunchControl, 8)
	open := func(_ string, _ *dispatch.Config, control dispatch.LaunchControl) (Controller, error) {
		controls <- control
		return &fakeController{opened: opened, closed: closed}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, s.runOptions(t, open)) }()
	defer func() {
		cancel()
		<-done
	}()
	var control dispatch.LaunchControl
	select {
	case control = <-controls:
	case <-time.After(10 * time.Second):
		t.Fatal("restarted main did not reopen to supervise the drain")
	}
	if _, err := control.Admit("site.impl.0.b-1"); !errors.Is(err, errAdmission) {
		t.Fatal("restarted main admitted a launch while DRAINING")
	}
	out, err := s.h.Resume("site", "resume-1")
	if err != nil || field(t, out, "desired") != "RUNNING" {
		t.Fatalf("resume during drain: %v", err)
	}
	if control.Boundary(true, true) || s.control(t).Desired != "RUNNING" {
		t.Fatal("a settled boundary stopped a resumed control")
	}
	release, err := control.Admit("site.impl.0.b-1")
	if err != nil {
		t.Fatalf("resumed control refused: %v", err)
	}
	release(true)
	time.Sleep(50 * time.Millisecond)
	if len(closed) != 0 || len(controls) != 0 {
		t.Fatal("resume during drain restarted the dispatcher")
	}
}

// SERVICE500-003: the legacy stop file is observed PRESENT/ABSENT/UNKNOWN;
// only the main latches RUNNING R to DRAINING R+1 (at its poll or at a
// refused admission under the same F), removal keeps the latch, UNKNOWN
// holds, and resume needs a fresh ABSENT.
func TestSERVICE500_LegacyStopFileLatchesDrain(t *testing.T) {
	s := newServiceHome(t)
	legacy := filepath.Join(s.home, "legacy", "stop")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := Profile{Executable: s.exe, DispatchConfig: s.config, WorkRoot: s.work, LegacyStopFile: &legacy, Helpers: []Helper{}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.profile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	s.writeLedger(t, "site.impl.0.a-1")
	status := func() string {
		t.Helper()
		o, err := s.h.Status("site")
		if err != nil {
			t.Fatal(err)
		}
		return field(t, o, "legacyStopFile")
	}
	o := s.runOptions(t, nil)
	govern := func() desiredRun {
		t.Helper()
		var stamp exeStamp
		d := o.observe(root, &stamp)
		if !d.run || d.legacy == nil || *d.legacy != legacy {
			t.Fatalf("observation %+v", d)
		}
		return o.govern(root, d)
	}
	lc := s.launchControl(t, &legacy)
	if status() != "ABSENT" {
		t.Fatal("absent legacy file not reported ABSENT")
	}
	if d := govern(); d.hold != "" || d.desired != "RUNNING" {
		t.Fatalf("absent legacy file changed control: %+v", d)
	}
	release, err := lc.Admit("site.impl.0.b-1")
	if err != nil {
		t.Fatal(err)
	}
	release(true)

	// PRESENT at admission latches under the same F, so removing the file
	// before the main's next poll does not lose the stop.
	if err := os.WriteFile(legacy, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if status() != "PRESENT" {
		t.Fatal("legacy file not reported PRESENT")
	}
	before := s.control(t)
	if _, err := lc.Admit("site.impl.0.b-2"); !errors.Is(err, errAdmission) {
		t.Fatal("present legacy file admitted a launch")
	}
	c := s.control(t)
	if c.Desired != "DRAINING" || c.Revision.Int() != before.Revision.Int()+1 || c.LastRequest != "legacy-stop-r"+strconv.FormatInt(c.Revision.Int(), 10) {
		t.Fatalf("admission latch was not RUNNING R to DRAINING R+1: %+v", c)
	}
	if err := os.Remove(legacy); err != nil {
		t.Fatal(err)
	}
	if d := govern(); d.desired != "DRAINING" || !d.run || s.control(t).Revision != c.Revision {
		t.Fatalf("removal before the poll released the admission latch: %+v", d)
	}
	out, err := s.h.Resume("site", "resume-0")
	if err != nil || field(t, out, "desired") != "RUNNING" {
		t.Fatalf("resume after removal: %v", err)
	}

	// The main's poll latches too.
	if err := os.WriteFile(legacy, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	before = s.control(t)
	if d := govern(); d.desired != "DRAINING" || !d.run {
		t.Fatalf("legacy presence did not latch DRAINING: %+v", d)
	}
	c = s.control(t)
	if c.Desired != "DRAINING" || c.Revision.Int() != before.Revision.Int()+1 || c.LastRequest != "legacy-stop-r"+strconv.FormatInt(c.Revision.Int(), 10) {
		t.Fatalf("latch was not RUNNING R to DRAINING R+1: %+v", c)
	}
	_, err = s.h.Resume("site", "resume-1")
	codeIs(t, err, wire.CodeResourceCollision)

	// Removal keeps the latch.
	if err := os.Remove(legacy); err != nil {
		t.Fatal(err)
	}
	if d := govern(); d.desired != "DRAINING" || s.control(t).Revision != c.Revision {
		t.Fatal("removing the legacy file released the latch")
	}

	// A symlink is UNKNOWN: it holds and refuses resume.
	if err := os.Symlink(filepath.Join(s.home, "elsewhere"), legacy); err != nil {
		t.Fatal(err)
	}
	if status() != "UNKNOWN" {
		t.Fatal("symlinked legacy file not reported UNKNOWN")
	}
	if d := govern(); d.hold == "" {
		t.Fatal("UNKNOWN legacy presence did not hold")
	}
	_, err = s.h.Resume("site", "resume-1")
	codeIs(t, err, wire.CodeUncertainEffect)
	if err := os.Remove(legacy); err != nil {
		t.Fatal(err)
	}
	out, err = s.h.Resume("site", "resume-1")
	if err != nil || field(t, out, "desired") != "RUNNING" {
		t.Fatalf("resume after fresh ABSENT: %v", err)
	}
	release, err = lc.Admit("site.impl.0.b-3")
	if err != nil {
		t.Fatal(err)
	}
	release(true)
}
