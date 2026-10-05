package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fakeLaunchd is an in-memory launchd user domain. It never runs launchctl.
type fakeLaunchd struct {
	mu          sync.Mutex
	domain      string
	registered  map[string]string
	calls       []string
	unreachable bool
	fail        map[string]int // verb -> exit code
}

func (f *fakeLaunchd) Run(_ context.Context, argv []string) (int, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(argv, " "))
	if len(argv) < 3 || argv[0] != "launchctl" {
		return 64, nil, nil
	}
	if code, ok := f.fail[argv[1]]; ok {
		return code, []byte("injected failure"), nil
	}
	switch argv[1] {
	case "print":
		if argv[2] == f.domain {
			if f.unreachable {
				return 5, nil, nil
			}
			return 0, nil, nil
		}
		label := strings.TrimPrefix(argv[2], f.domain+"/")
		if _, ok := f.registered[label]; ok {
			return 0, nil, nil
		}
		return launchdAbsentExit, nil, nil
	case "bootstrap":
		if argv[2] != f.domain || len(argv) != 4 {
			return 64, nil, nil
		}
		if _, err := os.Stat(argv[3]); err != nil {
			return 5, nil, nil
		}
		f.registered[strings.TrimSuffix(filepath.Base(argv[3]), ".plist")] = argv[3]
		return 0, nil, nil
	case "bootout":
		label := strings.TrimPrefix(argv[2], f.domain+"/")
		if _, ok := f.registered[label]; !ok {
			return 3, nil, nil
		}
		delete(f.registered, label)
		return 0, nil, nil
	}
	return 64, nil, nil
}

func (f *fakeLaunchd) effects() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.Contains(c, " bootstrap ") || strings.Contains(c, " bootout ") {
			n++
		}
	}
	return n
}

func (f *fakeLaunchd) isRegistered(label string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.registered[label]
	return ok
}

type serviceHome struct {
	h                           Host
	mgr                         *fakeLaunchd
	home, profile, config, work string
	exe                         string
	cfg                         dispatch.Config
}

// newServiceHome builds a disposable temp HOME with an executable, a
// dispatch config and a service profile. Temporary-root refusal is lifted
// only for the test's own temp tree.
func newServiceHome(t *testing.T) *serviceHome {
	t.Helper()
	saved := temporaryRoots
	temporaryRoots = nil
	t.Cleanup(func() { temporaryRoots = saved })
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &serviceHome{home: home, work: filepath.Join(home, "project"), exe: filepath.Join(home, "bin", "corvint-tasks"), config: filepath.Join(home, "config", "dispatch.json"), profile: filepath.Join(home, "config", "service.json")}
	for _, d := range []string{s.work, filepath.Dir(s.exe), filepath.Dir(s.config)} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(s.exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.cfg = dispatch.Config{Profile: dispatch.ConfigProfile, StateDir: filepath.Join(home, "dispatch-state"), WorkRoot: s.work, TickSeconds: 1, GlobalCap: 1, KillGraceSeconds: 1, Hosts: map[string]dispatch.Host{"sh": {Argv: []string{s.exe, "{prompt}"}}}, Roles: []dispatch.Role{{Name: "impl", Host: "sh", Cap: 1, Match: &dispatch.Match{}, Prompt: "work on {ticketLocal} as {holder}", IdleSeconds: 30, WallSeconds: 60}}, Backoff: dispatch.Backoff{ParkAfter: 2}}
	s.writeConfig(t)
	raw, err := Profile{Executable: s.exe, DispatchConfig: s.config, WorkRoot: s.work, Helpers: []Helper{}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.profile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	uid := os.Getuid()
	s.mgr = &fakeLaunchd{domain: "gui/" + strconv.Itoa(uid), registered: map[string]string{}, fail: map[string]int{}}
	s.h = Host{UID: uid, Home: home, GOOS: "darwin", Manager: s.mgr, QueueID: func(string) (string, error) { return "queue:acme:main", nil }, Sleep: func(time.Duration) {}}
	return s
}

func (s *serviceHome) writeConfig(t *testing.T) {
	t.Helper()
	raw, err := json.Marshal(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dispatch.DecodeConfig(raw); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.config, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (s *serviceHome) root(t *testing.T) string {
	t.Helper()
	root, err := s.h.StateRoot("site")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func (s *serviceHome) install(t *testing.T, request string, replace bool) (*wire.Object, error) {
	t.Helper()
	return s.h.Install(InstallRequest{Program: "site", Config: s.profile, RequestID: request, Replace: replace})
}

func field(t *testing.T, o *wire.Object, key string) string {
	t.Helper()
	v, ok := o.Get(key)
	if !ok {
		t.Fatalf("result has no %s", key)
	}
	switch v.Kind {
	case wire.KindString:
		return v.Str
	case wire.KindBool:
		return strconv.FormatBool(v.Bool)
	}
	return ""
}

func (s *serviceHome) control(t *testing.T) *Control {
	t.Helper()
	c, err := s.h.readControl(s.root(t))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (s *serviceHome) manifest(t *testing.T) (*Manifest, wire.Digest) {
	t.Helper()
	m, raw, err := s.h.readManifest(s.root(t))
	if err != nil {
		t.Fatal(err)
	}
	return m, wire.Sum(raw)
}

func codeIs(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || wire.CodeOf(err) != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestSERVICE500_InstallJournalReplayAndConflict(t *testing.T) {
	s := newServiceHome(t)
	out, err := s.install(t, "install-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "phase") != "INSTALLED" || field(t, out, "replayed") != "false" || field(t, out, "desired") != "RUNNING" || field(t, out, "generation") != "1" {
		t.Fatalf("install result %s", wire.EncodeFile(wire.ObjectValue(out)))
	}
	m, ident := s.manifest(t)
	u := m.Units[0]
	if !s.mgr.isRegistered(u.Label) || s.h.unitFile(u) != fileOwned {
		t.Fatal("unit not published and registered")
	}
	if c := s.control(t); c.Desired != "RUNNING" || c.ManifestIdentity != ident || c.Revision.Int() != 1 {
		t.Fatalf("control %+v", c)
	}
	for _, p := range []string{s.root(t), filepath.Join(s.root(t), operationDir)} {
		if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o700 {
			t.Fatalf("%s is not private", p)
		}
	}
	effects := s.mgr.effects()
	again, err := s.install(t, "install-1", false)
	if err != nil || field(t, again, "replayed") != "true" || field(t, again, "phase") != "INSTALLED" || s.mgr.effects() != effects {
		t.Fatalf("replay re-executed: %v", err)
	}
	same, err := s.install(t, "install-2", false)
	if err != nil || field(t, same, "phase") != "NO_CHANGE" || s.mgr.effects() != effects {
		t.Fatalf("identical install changed state: %v", err)
	}
	if ops, _ := s.h.operations(s.root(t)); len(ops) != 1 {
		t.Fatalf("NO_CHANGE install took a journal slot: %d records", len(ops))
	}
	_, err = s.install(t, "install-2", true)
	codeIs(t, err, wire.CodeRequestIDConflict)
	_, err = s.h.Stop("site", "install-1", false)
	codeIs(t, err, wire.CodeRequestIDConflict)
	_, err = s.install(t, "install-1", true)
	codeIs(t, err, wire.CodeRequestIDConflict)
	s.cfg.TickSeconds = 2
	s.writeConfig(t)
	_, err = s.install(t, "install-3", false)
	codeIs(t, err, wire.CodeResourceCollision)
	if _, now := s.manifest(t); now != ident || s.mgr.effects() != effects {
		t.Fatal("refused install changed the installation")
	}
}

func TestSERVICE500_ReplaceRestoresDesiredAndRollsBack(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	old, oldIdent := s.manifest(t)
	if _, err := s.h.Stop("site", "stop-1", false); err != nil {
		t.Fatal(err)
	}
	s.cfg.TickSeconds = 2
	s.writeConfig(t)
	out, err := s.install(t, "replace-1", true)
	if err != nil {
		t.Fatal(err)
	}
	next, ident := s.manifest(t)
	if field(t, out, "generation") != "2" || next.Previous == nil || *next.Previous != oldIdent {
		t.Fatalf("replacement lineage %s", wire.EncodeFile(wire.ObjectValue(out)))
	}
	if s.mgr.isRegistered(old.Units[0].Label) || s.h.unitFile(old.Units[0]) != fileAbsent || !s.mgr.isRegistered(next.Units[0].Label) {
		t.Fatal("old unit kept or new unit missing")
	}
	if c := s.control(t); c.Desired != "STOPPED" || c.ManifestIdentity != ident {
		t.Fatalf("replacement did not restore the prior desired state: %+v", c)
	}

	// A failed registration of generation 3 restores generation 2 exactly.
	s.cfg.TickSeconds = 3
	s.writeConfig(t)
	// When the restore itself fails the record stays ROLLBACK_REQUIRED and
	// the same request id finishes the rollback.
	s.mgr.fail["bootstrap"] = 5
	_, err = s.install(t, "replace-2", true)
	if err == nil || !strings.Contains(err.Error(), "ROLLBACK_REQUIRED") {
		t.Fatalf("want held rollback, got %v", err)
	}
	if st, _ := s.h.Status("site"); !strings.Contains(string(wire.EncodeFile(wire.ObjectValue(st))), "replace-2 ROLLBACK_REQUIRED") {
		t.Fatal("status does not report the unfinished rollback")
	}
	delete(s.mgr.fail, "bootstrap")
	_, err = s.install(t, "replace-2", true)
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("want completed rollback, got %v", err)
	}
	codeIs(t, err, wire.CodeRestored)
	if !s.mgr.isRegistered(next.Units[0].Label) {
		t.Fatal("prior registration not restored")
	}
	if _, now := s.manifest(t); now != ident {
		t.Fatal("rollback changed the committed manifest")
	}
	if s.h.unitFile(next.Units[0]) != fileOwned {
		t.Fatal("prior unit bytes not restored")
	}
	if c := s.control(t); c.Desired != "STOPPED" || c.ManifestIdentity != ident {
		t.Fatalf("rollback lost control: %+v", c)
	}
	replay, err := s.install(t, "replace-2", true)
	if err != nil || field(t, replay, "phase") != "ROLLED_BACK" || field(t, replay, "replayed") != "true" {
		t.Fatalf("rolled-back replay %v", err)
	}
}

func TestSERVICE500_FreshRegisterFailureRemovesPublishedUnit(t *testing.T) {
	s := newServiceHome(t)
	s.mgr.fail["bootstrap"] = 5
	_, err := s.install(t, "install-1", false)
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("want rollback, got %v", err)
	}
	codeIs(t, err, wire.CodeRestored)
	if _, _, err := s.h.readManifest(s.root(t)); !absent(err) {
		t.Fatal("manifest committed by a failed install")
	}
	entries, _ := os.ReadDir(filepath.Join(s.home, "Library", "LaunchAgents"))
	if len(entries) != 0 {
		t.Fatalf("published unit left behind: %v", entries)
	}
	if _, err := s.h.readControl(s.root(t)); !absent(err) {
		t.Fatal("control written by a failed install")
	}
}

func TestSERVICE500_ForeignUnitAndUnreachableManagerRefuse(t *testing.T) {
	s := newServiceHome(t)
	p, _, err := s.h.readProfile(s.profile)
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.h.facts("site", s.root(t), p, wire.SizeOf(1), nil)
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := build(p, f)
	if err != nil {
		t.Fatal(err)
	}
	s.mgr.registered[m.Units[0].Label] = m.Units[0].Path
	_, err = s.install(t, "install-1", false)
	codeIs(t, err, wire.CodeResourceCollision)
	delete(s.mgr.registered, m.Units[0].Label)
	if err := os.WriteFile(m.Units[0].Path, []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = s.install(t, "install-2", false)
	codeIs(t, err, wire.CodeResourceCollision)
	if raw, _ := os.ReadFile(m.Units[0].Path); string(raw) != "foreign" {
		t.Fatal("foreign unit overwritten")
	}
	os.Remove(m.Units[0].Path)
	s.mgr.unreachable = true
	_, err = s.install(t, "install-3", false)
	codeIs(t, err, wire.CodeCapabilityUnavailable)
	if s.mgr.effects() != 0 {
		t.Fatal("refused install reached the manager")
	}
}

func TestSERVICE500_UninstallKeepsControlAndJournal(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	m, _ := s.manifest(t)
	s.mgr.fail["bootout"] = 5
	_, err := s.h.Uninstall("site", "remove-1")
	codeIs(t, err, wire.CodeUncertainEffect)
	_, err = s.install(t, "install-2", false)
	codeIs(t, err, wire.CodeResourceCollision)
	delete(s.mgr.fail, "bootout")
	out, err := s.h.Uninstall("site", "remove-1")
	if err != nil || field(t, out, "phase") != "REMOVED" || field(t, out, "desired") != "STOPPED" {
		t.Fatalf("uninstall %v", err)
	}
	if s.mgr.isRegistered(m.Units[0].Label) || s.h.unitFile(m.Units[0]) != fileAbsent {
		t.Fatal("unit left registered or on disk")
	}
	if _, _, err := s.h.readManifest(s.root(t)); !absent(err) {
		t.Fatal("manifest kept")
	}
	if c := s.control(t); c.Desired != "STOPPED" {
		t.Fatal("control not retained as STOPPED")
	}
	effects := s.mgr.effects()
	again, err := s.h.Uninstall("site", "remove-1")
	if err != nil || field(t, again, "replayed") != "true" || s.mgr.effects() != effects {
		t.Fatalf("uninstall replay %v", err)
	}
	_, err = s.h.Uninstall("site", "remove-2")
	codeIs(t, err, wire.CodeResourceCollision)
	// Reinstalling keeps the retained STOPPED state until resume.
	if _, err := s.install(t, "install-3", false); err != nil {
		t.Fatal(err)
	}
	if c := s.control(t); c.Desired != "STOPPED" {
		t.Fatalf("reinstall started a stopped program: %+v", c)
	}
	if _, err := s.h.Resume("site", "resume-1"); err != nil {
		t.Fatal(err)
	}
	if c := s.control(t); c.Desired != "RUNNING" {
		t.Fatal("resume did not publish RUNNING")
	}
}

func TestSERVICE500_StopResumeFenceAndConflicts(t *testing.T) {
	s := newServiceHome(t)
	_, err := s.h.Stop("site", "stop-0", false)
	codeIs(t, err, wire.CodeResourceCollision)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	_, err = s.h.Resume("site", "resume-0")
	codeIs(t, err, wire.CodeResourceCollision)

	// A live dispatcher owner that is not the fenced managed main (no
	// bound pulse names it) keeps stop PENDING and blocks resume.
	m, _ := s.manifest(t)
	dir := dispatchDir(m)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	id, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil || id == "" {
		t.Skip("process identity is not observable here")
	}
	lock := filepath.Join(dir, "lock")
	if err := os.WriteFile(lock, []byte(fmt.Sprintf("%d %s\n", os.Getpid(), id)), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := s.h.Stop("site", "stop-1", false)
	if err != nil || field(t, out, "state") != "PENDING" || field(t, out, "close") != "PENDING" || field(t, out, "desired") != "STOPPED" || field(t, out, "workers") != "PRESERVED" {
		t.Fatalf("stop with live owner %v", err)
	}
	rev := s.control(t).Revision
	if out, err := s.h.Stop("site", "stop-1", false); err != nil || field(t, out, "revision") != string(rev) {
		t.Fatalf("stop replay advanced revision: %v", err)
	}
	_, err = s.h.Resume("site", "resume-1")
	codeIs(t, err, wire.CodeResourceCollision)
	os.Remove(lock)
	out, err = s.h.Stop("site", "stop-2", false)
	if err != nil || field(t, out, "state") != "ACKNOWLEDGED" || field(t, out, "close") != "OBSERVED" {
		t.Fatalf("stop without owner %v", err)
	}
	// Resume requires the pinned bytes.
	s.cfg.TickSeconds = 5
	s.writeConfig(t)
	_, err = s.h.Resume("site", "resume-1")
	codeIs(t, err, wire.CodeResourceCollision)
	s.cfg.TickSeconds = 1
	s.writeConfig(t)
	out, err = s.h.Resume("site", "resume-1")
	if err != nil || field(t, out, "desired") != "RUNNING" || field(t, out, "restartDebt") != "NOT_OBSERVED" {
		t.Fatalf("resume %v", err)
	}
	if out, err := s.h.Resume("site", "resume-1"); err != nil || field(t, out, "replayed") != "true" {
		t.Fatalf("resume replay %v", err)
	}
	_, err = s.h.Stop("site", "resume-1", false)
	codeIs(t, err, wire.CodeRequestIDConflict)

	// A delayed retry of an older resume after a later stop replays: it
	// never republishes RUNNING over the operator's stop.
	if out, err := s.h.Stop("site", "stop-3", false); err != nil || field(t, out, "state") != "ACKNOWLEDGED" {
		t.Fatalf("stop after resume %v", err)
	}
	rev = s.control(t).Revision
	out, err = s.h.Resume("site", "resume-1")
	if err != nil || field(t, out, "replayed") != "true" || field(t, out, "desired") != "STOPPED" {
		t.Fatalf("delayed resume retry %v", err)
	}
	if c := s.control(t); c.Desired != "STOPPED" || c.Revision != rev {
		t.Fatalf("delayed resume retry changed control: %+v", c)
	}
	// The same holds for an older stop after a later resume.
	if _, err := s.h.Resume("site", "resume-2"); err != nil {
		t.Fatal(err)
	}
	out, err = s.h.Stop("site", "stop-2", false)
	if err != nil || field(t, out, "replayed") != "true" || field(t, out, "state") != "SUPERSEDED" || s.control(t).Desired != "RUNNING" {
		t.Fatalf("delayed stop retry %v", err)
	}
	// A control change interrupted before its ledger write is recorded
	// from control by the next request.
	if err := os.Remove(filepath.Join(s.root(t), requestsFile)); err != nil {
		t.Fatal(err)
	}
	if out, err := s.h.Resume("site", "resume-2"); err != nil || field(t, out, "replayed") != "true" {
		t.Fatalf("resume replay after ledger loss %v", err)
	}
}

// The ledger keeps the most recent maxControlRequests ids in order.
func TestSERVICE500_ControlRequestLedgerIsBounded(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	rs := []controlRequest{}
	var err error
	for i := 0; i < maxControlRequests+3; i++ {
		if rs, err = s.h.rememberRequest(root, rs, fmt.Sprintf("req-%03d", i), wire.Sum([]byte(strconv.Itoa(i)))); err != nil {
			t.Fatal(err)
		}
	}
	back, err := s.h.controlRequests(root, nil)
	if err != nil || len(back) != maxControlRequests || back[0].ID != "req-003" {
		t.Fatalf("ledger %d %v", len(back), err)
	}
}

func TestSERVICE500_HistoryCapRefusesBeforeEffect(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-0", false); err != nil {
		t.Fatal(err)
	}
	m, _ := s.manifest(t)
	for i := 1; i < maxOperations-uninstallReserve; i++ {
		op := &Operation{Kind: "INSTALL", RequestID: fmt.Sprintf("filler-%03d", i), Program: "site", Phase: "NO_CHANGE", RequestSha256: wire.Sum([]byte(strconv.Itoa(i))), Next: m, Actions: []Action{{Kind: "NO_CHANGE_PRESERVE_CONTROL_DEBT_WORKERS"}}, Completed: 1, Published: []string{}}
		raw, err := EncodeOperation(*op)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(s.root(t), operationDir, operationName(op.RequestID)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := s.install(t, "install-next", false)
	codeIs(t, err, wire.CodeLimitExceeded)
	if _, err := s.install(t, "install-0", false); err != nil {
		t.Fatalf("replay must still answer at the cap: %v", err)
	}
	// The reserved headroom still lets the operator remove the service.
	if out, err := s.h.Uninstall("site", "uninstall-1"); err != nil || field(t, out, "phase") != "REMOVED" {
		t.Fatalf("uninstall at the install cap: %v", err)
	}
}

// SERVICE500-001: a manifest commit that renamed before its readback
// failed is undone by rollback, so control stays bound and uninstall works.
func TestSERVICE500_RollbackRestoresCommittedManifest(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	_, ident := s.manifest(t)
	s.cfg.TickSeconds = 2
	s.writeConfig(t)
	s.mgr.fail["bootstrap"] = 5
	if _, err := s.install(t, "replace-1", true); err == nil || !strings.Contains(err.Error(), "ROLLBACK_REQUIRED") {
		t.Fatalf("want held rollback, got %v", err)
	}
	delete(s.mgr.fail, "bootstrap")
	root := s.root(t)
	ops, err := s.h.operations(root)
	if err != nil {
		t.Fatal(err)
	}
	var held *Operation
	for _, o := range ops {
		if o.RequestID == "replace-1" {
			held = o
		}
	}
	nextRaw, err := held.Next.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(root, manifestFile, nextRaw); err != nil {
		t.Fatal(err)
	}
	_, err = s.install(t, "replace-1", true)
	codeIs(t, err, wire.CodeRestored)
	if _, now := s.manifest(t); now != ident {
		t.Fatal("rollback left the uncommitted manifest")
	}
	if c := s.control(t); c.ManifestIdentity != ident {
		t.Fatalf("control unbound after rollback: %+v", c)
	}
	if out, err := s.h.Uninstall("site", "uninstall-1"); err != nil || field(t, out, "phase") != "REMOVED" {
		t.Fatalf("uninstall after rollback: %v", err)
	}

	// A fresh install's manifest is removed exactly.
	op := &Operation{Next: held.Next}
	if err := writeAtomic(root, manifestFile, nextRaw); err != nil {
		t.Fatal(err)
	}
	if err := s.h.restoreManifest(root, op); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.h.readManifest(root); !absent(err) {
		t.Fatal("fresh rollback left a manifest")
	}
	if err := writeAtomic(root, manifestFile, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	codeIs(t, s.h.restoreManifest(root, op), wire.CodeResourceCollision)
}

// SERVICE500-001: an invalid profile is refused before the registry exists.
func TestSERVICE500_RefusedInstallCreatesNoRegistry(t *testing.T) {
	s := newServiceHome(t)
	_, err := s.h.Install(InstallRequest{Program: "site", Config: filepath.Join(s.home, "config", "missing.json"), RequestID: "install-1"})
	if err == nil {
		t.Fatal("missing profile accepted")
	}
	if _, err := os.Stat(filepath.Join(s.home, ".local")); !absent(err) {
		t.Fatal("refused install created registry state")
	}
}

// SERVICE500-001: a per-unit drop-in in a system-wide systemd --user root
// is observed, so the unit is not adopted.
func TestSERVICE500_SystemWideDropInsAreObserved(t *testing.T) {
	sys := t.TempDir()
	saved := systemdUserDropInRoots
	systemdUserDropInRoots = []string{sys}
	t.Cleanup(func() { systemdUserDropInRoots = saved })
	m := &Manifest{Manager: "systemd-user"}
	u := Unit{Path: filepath.Join(t.TempDir(), "org.corvint.x.site.service")}
	if dropIns(m, u) {
		t.Fatal("drop-in reported without one")
	}
	if err := os.Mkdir(filepath.Join(sys, "org.corvint.x.site.service.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !dropIns(m, u) {
		t.Fatal("system-wide drop-in not observed")
	}
}

func TestSERVICE500_StatusAndDispatchServiceArePureReads(t *testing.T) {
	s := newServiceHome(t)
	out, err := s.h.Status("site")
	if err != nil || field(t, out, "installed") != "ABSENT" || field(t, out, "desired") != "ABSENT" {
		t.Fatalf("absent status %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.home, ".local")); !absent(err) {
		t.Fatal("status created state")
	}
	if _, ok := s.h.DispatchService("site", s.cfg.StateDir); ok {
		t.Fatal("absent service reported in dispatch status")
	}
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	calls := len(s.mgr.calls)
	o, ok := s.h.DispatchService("site", s.cfg.StateDir)
	if !ok || field(t, o, "desired") != "RUNNING" || field(t, o, "controller") != "ABSENT" || len(s.mgr.calls) != calls {
		t.Fatal("dispatch service object wrong or queried the manager")
	}
	if _, ok := s.h.DispatchService("site", "/elsewhere"); ok {
		t.Fatal("service reported for another state dir")
	}
	out, err = s.h.Status("site")
	if err != nil || field(t, out, "installed") != "PRESENT" || field(t, out, "controlBound") != "true" || field(t, out, "operations") != "1" {
		t.Fatalf("installed status %v %s", err, wire.EncodeFile(wire.ObjectValue(out)))
	}
	if _, err := s.h.Status("Bad Name"); err == nil {
		t.Fatal("invalid program accepted")
	}
}

func TestSERVICE500_SystemdRegistrationParsing(t *testing.T) {
	m := &Manifest{Manager: "systemd-user", Domain: "user/501"}
	u := Unit{Label: "org.corvint.x.site", Path: "/home/a/.config/systemd/user/org.corvint.x.site.service"}
	for out, want := range map[string]string{
		"LoadState=not-found\nActiveState=inactive\nUnitFileState=\nFragmentPath=\n":                     regAbsent,
		"LoadState=loaded\nActiveState=active\nUnitFileState=enabled\nFragmentPath=" + u.Path + "\n":     regPresent,
		"LoadState=loaded\nActiveState=active\nUnitFileState=enabled\nFragmentPath=/etc/other.service\n": regUnknown,
		"LoadState=loaded\nActiveState=inactive\nUnitFileState=disabled\nFragmentPath=" + u.Path + "\n":  regAbsent,
		"LoadState=loaded\nActiveState=activating\nUnitFileState=disabled\nFragmentPath=\n":              regUnknown,
	} {
		h := Host{Manager: cannedManager(out)}
		if got := h.query(m, u); got != want {
			t.Errorf("%q: got %s want %s", out, got, want)
		}
	}
	if got := (Host{Manager: failingManager{}}).query(m, u); got != regUnknown {
		t.Fatal("manager error must be UNKNOWN")
	}
}

type cannedManager string

func (c cannedManager) Run(context.Context, []string) (int, []byte, error) {
	return 0, []byte(c), nil
}

type failingManager struct{}

func (failingManager) Run(context.Context, []string) (int, []byte, error) {
	return -1, nil, errors.New("unavailable")
}

// fakeController is an in-process dispatcher stand-in.
type fakeController struct {
	opened, closed chan struct{}
	// gate, when set, holds Close (and so the main loop's next pulse)
	// until the test closes it.
	gate chan struct{}
}

func (f *fakeController) Run(ctx context.Context, _ int) error {
	<-ctx.Done()
	return nil
}
func (f *fakeController) Close() error {
	if f.gate != nil {
		<-f.gate
	}
	f.closed <- struct{}{}
	return nil
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestSERVICE500_ManagedMainFollowsControlAndPins(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	opened, closed := make(chan struct{}, 8), make(chan struct{}, 8)
	// The gate keeps the first Close, and so the RUNNING pulse, in place
	// until stop has answered; without it a loaded host can let the main
	// publish IDLE first, and stop then rightly reports close OBSERVED.
	gate := make(chan struct{})
	open := func(program string, c *dispatch.Config, _ dispatch.LaunchFence) (Controller, error) {
		if program != "site" || c.WorkRoot != s.work {
			return nil, errors.New("wrong binding")
		}
		opened <- struct{}{}
		return &fakeController{opened: opened, closed: closed, gate: gate}, nil
	}
	opts := RunOptions{Host: s.h, Program: "site", Manifest: filepath.Join(root, manifestFile), Executable: s.exe, Open: open, Poll: 5 * time.Millisecond, Pulse: 5 * time.Millisecond, Retry: 5 * time.Millisecond}
	bad := opts
	bad.Manifest = filepath.Join(s.home, "manifest.json")
	codeIs(t, Run(context.Background(), bad), wire.CodeUnsupported)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opts) }()
	recv := func(ch chan struct{}, what string) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for %s", what)
		}
	}
	pulse := func(want string) func() bool {
		return func() bool {
			raw, err := s.h.readPrivate(filepath.Join(root, pulseFile), maxPulse)
			if err != nil {
				return false
			}
			p, err := DecodePulse(raw)
			return err == nil && p.State == want
		}
	}
	recv(opened, "dispatcher open")
	waitFor(t, "RUNNING pulse", pulse("RUNNING"))
	if st, _ := s.h.pulseState(root, ""); st != "RUNNING" {
		t.Fatalf("pulse state %s", st)
	}
	out, err := s.h.Stop("site", "stop-1", false)
	// Suppression is durable and no fenced launch can follow it, so stop
	// is ACKNOWLEDGED; the dispatcher's close is not yet observed.
	if err != nil || field(t, out, "state") != "ACKNOWLEDGED" || field(t, out, "close") != "PENDING" {
		t.Fatalf("stop with a live RUNNING pulse: %v", err)
	}
	close(gate)
	recv(closed, "dispatcher close after STOPPED")
	waitFor(t, "IDLE pulse", pulse("IDLE"))
	if _, err := s.h.Resume("site", "resume-1"); err != nil {
		t.Fatal(err)
	}
	recv(opened, "dispatcher reopen after resume")
	s.cfg.TickSeconds = 7
	s.writeConfig(t)
	recv(closed, "dispatcher close on config drift")
	waitFor(t, "HOLD pulse", pulse("HOLD"))
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("managed main did not return")
	}
	if len(opened) != 0 {
		t.Fatal("dispatcher reopened while pins drifted")
	}
}

func TestSERVICE500_FencePruningBoundsSettledHistory(t *testing.T) {
	d := debtFixture()
	for g := 1; g <= 128; g++ {
		d.Fences[strconv.Itoa(g)] = "INTENTIONAL_STOP"
	}
	obs := FailureObservation{Generation: "129", Outcome: "FAILED", BootID: "boot", Termination: "PROVED_TERMINATED", Now: 1, TimeKnown: true, IdentityKnown: true}
	if _, state, _ := ChargeFailure(d, obs); state != "SERVICE_RESTART_HOLD" {
		t.Fatalf("full fence store must hold, got %s", state)
	}
	h := HealthObservation{BootID: "boot", Generation: "129", Desired: "RUNNING", State: "HEALTHY", TimeKnown: true, PinnedValid: true, PulseTimely: true, TickHealthy: true, OwnershipSettled: true}
	for now := uint64(0); now <= 600; now += 20 {
		h.Now = now
		d = ObserveHealth(d, h)
	}
	if len(d.Fences) != 0 || d.FenceFloor != 128 {
		t.Fatalf("settled fences not pruned: %d floor %d", len(d.Fences), d.FenceFloor)
	}
	obs.Now = 700
	if _, state, err := ChargeFailure(d, obs); err != nil || state != "BACKOFF" {
		t.Fatalf("pruned store must charge again: %s %v", state, err)
	}
	stale := obs
	stale.Generation = "7"
	if _, state, _ := ChargeFailure(d, stale); state != "HOLD" {
		t.Fatalf("outcome below the floor must hold, got %s", state)
	}
	opaque := debtFixture()
	opaque.Fences["a"] = "FAILED"
	opaque.Fences["0"] = "FAILED"
	pruneFences(&opaque, 1<<40)
	if len(opaque.Fences) != 2 {
		t.Fatal("opaque generations must never be pruned")
	}
}

// SERVICE500-003: a stop saved while Open is taking ownership is observed
// before the dispatcher runs: the opened controller closes without Run.
func TestSERVICE500_ManagedMainRechecksControlAfterOpen(t *testing.T) {
	s := newServiceHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	ran, closed := make(chan struct{}, 8), make(chan struct{}, 8)
	open := func(string, *dispatch.Config, dispatch.LaunchFence) (Controller, error) {
		if _, err := s.h.Stop("site", "stop-1", false); err != nil {
			return nil, err
		}
		return &racingController{ran: ran, closed: closed}, nil
	}
	opts := RunOptions{Host: s.h, Program: "site", Manifest: filepath.Join(root, manifestFile), Executable: s.exe, Open: open, Poll: 5 * time.Millisecond, Pulse: 5 * time.Millisecond, Retry: 5 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opts) }()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("opened controller was not closed after STOPPED")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(ran) != 0 {
		t.Fatal("dispatcher ran after STOPPED was saved during Open")
	}
}

type racingController struct{ ran, closed chan struct{} }

func (r *racingController) Run(ctx context.Context, _ int) error {
	r.ran <- struct{}{}
	<-ctx.Done()
	return nil
}
func (r *racingController) Close() error {
	r.closed <- struct{}{}
	return nil
}
