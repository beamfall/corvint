package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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

func (s *serviceHome) runOptions(t *testing.T, open func(string, *dispatch.Config, dispatch.LaunchFence) (Controller, error)) RunOptions {
	t.Helper()
	root := s.root(t)
	return RunOptions{Host: s.h, Program: "site", Manifest: filepath.Join(root, manifestFile), Executable: s.exe, Open: open, Poll: 5 * time.Millisecond, Pulse: 5 * time.Millisecond, Retry: 5 * time.Millisecond}
}

// SERVICE500-003: an admitted launch holds F, so a concurrent stop cannot
// save its suppression until the launch is recorded and F released; the
// stop then completes ACKNOWLEDGED and later admissions refuse.
func TestSERVICE500_FenceSerializesStopWithAdmittedLaunch(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	_, ident := s.manifest(t)
	admit := s.runOptions(t, nil).admitter(root, ident, nil)
	release, err := admit()
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
	release()
	var a answer
	select {
	case a = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("stop did not progress after the fence was released")
	}
	if a.err != nil || field(t, a.out, "state") != "ACKNOWLEDGED" || field(t, a.out, "desired") != "STOPPED" {
		t.Fatalf("stop after fence release: %v", a.err)
	}
	if _, err := admit(); !errors.Is(err, errAdmission) {
		t.Fatalf("admission after STOPPED: %v", err)
	}
}

// SERVICE500-001/003: drain keeps the dispatcher supervising while its fence
// refuses launches, and the managed main moves DRAINING to STOPPED by CAS
// only once the saved ledger records no worker, then closes it.
func TestSERVICE500_DrainSupervisesThenSettlesStopped(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	s.writeLedger(t, "site.impl.0.a-1")
	opened, closed := make(chan struct{}, 8), make(chan struct{}, 8)
	fences := make(chan dispatch.LaunchFence, 8)
	open := func(_ string, _ *dispatch.Config, fence dispatch.LaunchFence) (Controller, error) {
		fences <- fence
		opened <- struct{}{}
		return &fakeController{opened: opened, closed: closed}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, s.runOptions(t, open)) }()
	defer func() {
		cancel()
		<-done
	}()
	var fence dispatch.LaunchFence
	select {
	case fence = <-fences:
	case <-time.After(10 * time.Second):
		t.Fatal("dispatcher did not open")
	}
	out, err := s.h.Stop("site", "drain-1", true)
	if err != nil || field(t, out, "state") != "ACKNOWLEDGED" || field(t, out, "desired") != "DRAINING" || field(t, out, "close") != "PENDING" || field(t, out, "drain") != "true" {
		t.Fatalf("drain: %v", err)
	}
	if _, err := fence(); !errors.Is(err, errAdmission) {
		t.Fatalf("draining fence admitted a launch: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if s.control(t).Desired != "DRAINING" || len(closed) != 0 {
		t.Fatal("drain settled or closed the dispatcher while a worker was recorded")
	}
	_, err = s.h.Stop("site", "drain-2", true)
	codeIs(t, err, wire.CodeResourceCollision)

	drained := s.control(t)
	s.writeLedger(t)
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
	fences := make(chan dispatch.LaunchFence, 8)
	open := func(_ string, _ *dispatch.Config, fence dispatch.LaunchFence) (Controller, error) {
		fences <- fence
		return &fakeController{opened: opened, closed: closed}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, s.runOptions(t, open)) }()
	defer func() {
		cancel()
		<-done
	}()
	var fence dispatch.LaunchFence
	select {
	case fence = <-fences:
	case <-time.After(10 * time.Second):
		t.Fatal("restarted main did not reopen to supervise the drain")
	}
	if _, err := fence(); !errors.Is(err, errAdmission) {
		t.Fatal("restarted main admitted a launch while DRAINING")
	}
	out, err := s.h.Resume("site", "resume-1")
	if err != nil || field(t, out, "desired") != "RUNNING" {
		t.Fatalf("resume during drain: %v", err)
	}
	release, err := fence()
	if err != nil {
		t.Fatalf("resumed fence refused: %v", err)
	}
	release()
	time.Sleep(50 * time.Millisecond)
	if len(closed) != 0 || len(fences) != 0 {
		t.Fatal("resume during drain restarted the dispatcher")
	}
}

// SERVICE500-003: the legacy stop file is observed PRESENT/ABSENT/UNKNOWN;
// only the main latches RUNNING R to DRAINING R+1, removal keeps the
// latch, UNKNOWN holds, and resume needs a fresh ABSENT.
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
	_, ident := s.manifest(t)
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
	admit := o.admitter(root, ident, &legacy)
	if status() != "ABSENT" {
		t.Fatal("absent legacy file not reported ABSENT")
	}
	if d := govern(); d.hold != "" || d.desired != "RUNNING" {
		t.Fatalf("absent legacy file changed control: %+v", d)
	}
	release, err := admit()
	if err != nil {
		t.Fatal(err)
	}
	release()

	if err := os.WriteFile(legacy, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if status() != "PRESENT" {
		t.Fatal("legacy file not reported PRESENT")
	}
	if _, err := admit(); !errors.Is(err, errAdmission) {
		t.Fatal("present legacy file admitted a launch")
	}
	before := s.control(t)
	if d := govern(); d.desired != "DRAINING" || !d.run {
		t.Fatalf("legacy presence did not latch DRAINING: %+v", d)
	}
	c := s.control(t)
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
	out, err := s.h.Resume("site", "resume-1")
	if err != nil || field(t, out, "desired") != "RUNNING" {
		t.Fatalf("resume after fresh ABSENT: %v", err)
	}
	release, err = admit()
	if err != nil {
		t.Fatal(err)
	}
	release()
}
