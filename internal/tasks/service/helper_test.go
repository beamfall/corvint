package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fakeSystemd is an in-memory systemd --user manager. It never runs
// systemctl.
type fakeSystemd struct {
	mu         sync.Mutex
	unitRoot   string
	registered map[string]string // label -> fragment path
	calls      []string
}

func (f *fakeSystemd) Run(_ context.Context, argv []string) (int, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(argv, " "))
	if len(argv) < 3 || argv[0] != "systemctl" || argv[1] != "--user" {
		return 64, nil, nil
	}
	switch argv[2] {
	case "show":
		if len(argv) == 5 && argv[4] == "Version" {
			return 0, []byte("Version=255\n"), nil
		}
		label := strings.TrimSuffix(argv[len(argv)-1], ".service")
		if p, ok := f.registered[label]; ok {
			return 0, []byte("LoadState=loaded\nActiveState=active\nUnitFileState=enabled\nFragmentPath=" + p + "\n"), nil
		}
		return 0, []byte("LoadState=not-found\nActiveState=inactive\nUnitFileState=\nFragmentPath=\n"), nil
	case "daemon-reload":
		return 0, nil, nil
	case "enable", "disable":
		if len(argv) != 5 || argv[3] != "--now" {
			return 64, nil, nil
		}
		label := strings.TrimSuffix(argv[4], ".service")
		if argv[2] == "disable" {
			delete(f.registered, label)
			return 0, nil, nil
		}
		path := filepath.Join(f.unitRoot, argv[4])
		if _, err := os.Stat(path); err != nil {
			return 5, nil, nil
		}
		f.registered[label] = path
		return 0, nil, nil
	}
	return 64, nil, nil
}

func (f *fakeSystemd) labels() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []string{}
	for l := range f.registered {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// newHelperHome is a linux (systemd --user) service home whose profile
// declares one helper, web.
func newHelperHome(t *testing.T) (*serviceHome, *fakeSystemd) {
	t.Helper()
	s := newServiceHome(t)
	helper := filepath.Join(s.home, "bin", "web-helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := Profile{Executable: s.exe, DispatchConfig: s.config, WorkRoot: s.work, Helpers: []Helper{{ID: "web", Cwd: s.work, Argv: []string{helper, "--port", "8080"}, Env: map[string]string{"MODE": "test"}, Foreground: true, StopWithProgram: true}}}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.profile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	sd := &fakeSystemd{unitRoot: filepath.Join(s.home, ".config", "systemd", "user"), registered: map[string]string{}}
	s.h.GOOS, s.h.Manager = "linux", sd
	return s, sd
}

// fenceHeld reports whether F is held by another open file description.
func fenceHeld(root string) bool {
	f, err := os.OpenFile(filepath.Join(root, fenceFile), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return false
	}
	defer f.Close()
	return tryLock(f) != nil
}

// fakeTree is a helper spawner that starts no process. It records what
// was durable and whether F was held at each lifecycle step.
type fakeTree struct {
	root string

	mu                           sync.Mutex
	quiet, startErr, verifyErr   error
	retireErr                    error
	autoExit                     bool
	gate                         chan struct{}
	starts, retires              int
	fenceAtStart, fenceAtRetire  []bool
	intentAtStart, recordAtStart []string
	recordAtRetire               []string
	argv, env                    [][]string
	dir                          []string
}

type fakeProc struct {
	t      *fakeTree
	pid    int
	exited chan struct{}
	once   sync.Once
}

func (f *fakeTree) Quiet() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.quiet
}

func (f *fakeTree) state(h Host) (intent, record string) {
	in, _, err := h.readHelperIntent(f.root, "web")
	switch {
	case err != nil:
		intent = "UNKNOWN"
	case in == nil:
		intent = "ABSENT"
	default:
		intent = in.State
	}
	rec, err := h.readHelperRecord(f.root, "site", "web")
	if err != nil {
		return intent, "UNKNOWN"
	}
	return intent, rec.State
}

func (f *fakeTree) Start(cmd *exec.Cmd) (helperProc, error) {
	held := fenceHeld(f.root)
	intent, record := f.state(Host{UID: os.Getuid()})
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fenceAtStart = append(f.fenceAtStart, held)
	f.intentAtStart = append(f.intentAtStart, intent)
	f.recordAtStart = append(f.recordAtStart, record)
	f.argv = append(f.argv, append([]string{cmd.Path}, cmd.Args...))
	f.env = append(f.env, cmd.Env)
	f.dir = append(f.dir, cmd.Dir)
	if f.startErr != nil {
		return nil, f.startErr
	}
	f.starts++
	_, _ = cmd.Stdout.(*os.File).Write([]byte("helper says hello\n"))
	p := &fakeProc{t: f, pid: 40000 + f.starts, exited: make(chan struct{})}
	return p, nil
}

func (p *fakeProc) PID() int                { return p.pid }
func (p *fakeProc) Exited() <-chan struct{} { return p.exited }
func (p *fakeProc) exit()                   { p.once.Do(func() { close(p.exited) }) }

func (p *fakeProc) Verify() (string, error) {
	p.t.mu.Lock()
	err, auto := p.t.verifyErr, p.t.autoExit
	p.t.mu.Unlock()
	if err != nil {
		return "", err
	}
	if auto {
		p.exit()
	}
	return "start:1", nil
}

func (p *fakeProc) Retire(time.Duration) (string, error) {
	p.t.mu.Lock()
	gate := p.t.gate
	p.t.mu.Unlock()
	if gate != nil {
		<-gate
	}
	held := fenceHeld(p.t.root)
	_, record := p.t.state(Host{UID: os.Getuid()})
	p.exit()
	p.t.mu.Lock()
	defer p.t.mu.Unlock()
	p.t.retires++
	p.t.fenceAtRetire = append(p.t.fenceAtRetire, held)
	p.t.recordAtRetire = append(p.t.recordAtRetire, record)
	return "exited", p.t.retireErr
}

func (f *fakeTree) count() (starts, retires int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts, f.retires
}

// steppingClock advances one uptime hour per observation, so restart
// backoff never delays the next spawn.
func steppingClock() func() (string, uint64, bool) {
	var now atomic.Uint64
	return func() (string, uint64, bool) { return "boot-1", now.Add(3600), true }
}

// runHelper starts a run-helper wrapper for web with the fake tree.
func (s *serviceHome) runHelper(t *testing.T, tree *fakeTree, clock func() (string, uint64, bool)) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	o := HelperOptions{RunOptions: s.runOptions(t, nil), Helper: "web", RetireBound: time.Second, spawner: func() (helperSpawner, error) { return tree, nil }, clock: clock, getenv: func(k string) string {
		return map[string]string{"HOME": s.home, "PATH": "/usr/bin:/bin", "SECRET_TOKEN": "leak"}[k]
	}}
	go func() { done <- RunHelper(ctx, o) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			if err := <-done; err != nil {
				t.Errorf("run-helper returned %v", err)
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func (s *serviceHome) helperRecord(t *testing.T) helperRecord {
	t.Helper()
	rec, err := s.h.readHelperRecord(s.root(t), "site", "web")
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func (s *serviceHome) helperIntent(t *testing.T) *helperIntent {
	t.Helper()
	in, _, err := s.h.readHelperIntent(s.root(t), "web")
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func (s *serviceHome) helperPulse(t *testing.T) *Pulse {
	raw, err := os.ReadFile(filepath.Join(s.root(t), helperFile("web", helperPulseSuffix)))
	if err != nil {
		return nil
	}
	p, err := DecodePulse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &p
}

func TestSERVICE500_HelperInstallAcceptedOnLinuxRefusedElsewhere(t *testing.T) {
	s, sd := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	m, _ := s.manifest(t)
	if pin, ok := m.HelperExecutables["web"]; !ok || pin.Sha256 == "" {
		t.Fatal("helper executable not pinned in the manifest")
	}
	if len(m.Units) != 2 || len(sd.labels()) != 2 {
		t.Fatalf("want a main and a helper unit, got %d units, registered %v", len(m.Units), sd.labels())
	}
	var helperUnitText string
	for _, u := range m.Units {
		if strings.HasSuffix(u.Label, ".helper-web") {
			raw, err := os.ReadFile(u.Path)
			if err != nil {
				t.Fatal(err)
			}
			helperUnitText = string(raw)
		}
	}
	for _, want := range []string{`"service" "run-helper"`, `"--helper" "web"`, "KillMode=control-group", "StandardOutput=null"} {
		if !strings.Contains(helperUnitText, want) {
			t.Fatalf("helper unit lacks %s:\n%s", want, helperUnitText)
		}
	}
	if strings.Contains(helperUnitText, "8080") {
		t.Fatal("helper argv must come from the manifest, not unit text")
	}

	d := newServiceHome(t)
	raw, err := os.ReadFile(s.profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.profile, []byte(strings.ReplaceAll(string(raw), s.home, d.home)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.home, "bin", "web-helper"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = d.install(t, "install-1", false)
	codeIs(t, err, wire.CodeUnsupported)
	if _, err := os.Stat(filepath.Join(d.home, ".local")); !absent(err) || d.mgr.effects() != 0 {
		t.Fatal("refused helper install created state or touched the manager")
	}
}

func TestSERVICE500_HelperSpawnsUnderFenceAndAcksAfterVerify(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	tree := &fakeTree{root: root, gate: make(chan struct{})}
	stop := s.runHelper(t, tree, steppingClock())
	var release sync.Once
	open := func() { release.Do(func() { close(tree.gate) }) }
	t.Cleanup(open) // runs before the wrapper's cleanup, so a failure cannot wedge it
	waitFor(t, "helper RUNNING", func() bool { return s.helperRecord(t).State == "RUNNING" })
	in := s.helperIntent(t)
	if in == nil || in.State != "RUNNING" || in.PID != 40001 || in.Identity != "start:1" || in.Generation != 1 {
		t.Fatalf("acknowledged intent %+v", in)
	}
	tree.mu.Lock()
	if !tree.fenceAtStart[0] || tree.intentAtStart[0] != "SPAWNING" || tree.recordAtStart[0] != "STARTING" {
		t.Fatalf("start must follow a durable SPAWNING intent under F: fence=%v intent=%s record=%s", tree.fenceAtStart[0], tree.intentAtStart[0], tree.recordAtStart[0])
	}
	env := strings.Join(tree.env[0], "\n")
	if !strings.Contains(env, "MODE=test") || !strings.Contains(env, "PATH=/usr/bin:/bin") || strings.Contains(env, "SECRET_TOKEN") {
		t.Fatalf("helper environment %q", env)
	}
	if tree.dir[0] != s.work || tree.argv[0][2] != "--port" {
		t.Fatalf("helper exec %v in %s", tree.argv[0], tree.dir[0])
	}
	tree.mu.Unlock()
	waitFor(t, "RUNNING pulse", func() bool { p := s.helperPulse(t); return p != nil && p.State == "RUNNING" })

	// Stop: the tree is retired outside F; until it is, the close stays
	// PENDING and resume refuses.
	out, err := s.h.Stop("site", "stop-1", false)
	if err != nil || field(t, out, "close") != "PENDING" || field(t, out, "state") != "ACKNOWLEDGED" {
		t.Fatalf("stop with a live helper tree: %v %s", err, wire.EncodeFile(wire.ObjectValue(out)))
	}
	_, err = s.h.Resume("site", "resume-0")
	codeIs(t, err, wire.CodeUncertainEffect)
	open()
	waitFor(t, "helper RETIRED", func() bool { return s.helperIntent(t) == nil && s.helperRecord(t).State == "RETIRED" })
	tree.mu.Lock()
	if tree.fenceAtRetire[0] || tree.recordAtRetire[0] != "RUNNING" {
		t.Fatalf("retirement must run outside F: fence=%v", tree.fenceAtRetire[0])
	}
	tree.mu.Unlock()
	if rec := s.helperRecord(t); rec.Debt.Failures != 0 {
		t.Fatalf("an intentional stop charged restart debt: %+v", rec.Debt)
	}
	out, err = s.h.Stop("site", "stop-2", false)
	if err != nil || field(t, out, "close") != "OBSERVED" {
		t.Fatalf("stop after retirement: %v", err)
	}
	stop()
	logs, err := s.h.readLogStatus(root, "helper-web")
	if err != nil || logs["stdout"].Written != uint64(len("helper says hello\n")) {
		t.Fatalf("helper stdout not logged: %v %+v", err, logs)
	}
	if starts, retires := tree.count(); starts != 1 || retires != 1 {
		t.Fatalf("starts=%d retires=%d", starts, retires)
	}
}

func TestSERVICE500_HelperVerifyFailureRetiresWithoutAck(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	tree := &fakeTree{root: s.root(t), verifyErr: errors.New("leader escaped its group")}
	// A fixed clock: the charged failure's backoff blocks a second start.
	s.runHelper(t, tree, func() (string, uint64, bool) { return "boot-1", 100, true })
	waitFor(t, "unverified tree retired", func() bool {
		_, retires := tree.count()
		return retires == 1 && s.helperIntent(t) == nil
	})
	tree.mu.Lock()
	if tree.recordAtRetire[0] != "STARTING" {
		t.Fatalf("an unverified tree was acknowledged: %s", tree.recordAtRetire[0])
	}
	tree.mu.Unlock()
	rec := s.helperRecord(t)
	if rec.State != "RETIRED" || rec.Debt.Failures != 1 || rec.Debt.EligibleAfter != 130 || !strings.Contains(rec.LastExit, "not verified") {
		t.Fatalf("record %+v", rec)
	}
	time.Sleep(50 * time.Millisecond)
	if starts, _ := tree.count(); starts != 1 {
		t.Fatalf("restarted during backoff: %d starts", starts)
	}
}

func TestSERVICE500_HelperUnprovedRetirementHoldsUntilOperatorResume(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	tree := &fakeTree{root: root, retireErr: errors.New("descendants remained at the retirement bound"), autoExit: true}
	stop := s.runHelper(t, tree, steppingClock())
	waitFor(t, "helper HOLD", func() bool { return s.helperRecord(t).State == "HOLD" })
	if in := s.helperIntent(t); in == nil {
		t.Fatal("unproved retirement removed the intent")
	}
	if rec := s.helperRecord(t); rec.Debt.Failures != 0 {
		t.Fatalf("an unproved retirement charged debt: %+v", rec.Debt)
	}
	tree.mu.Lock()
	tree.retireErr = nil
	tree.mu.Unlock()
	time.Sleep(50 * time.Millisecond)
	if starts, _ := tree.count(); starts != 1 {
		t.Fatalf("restarted over an unretired tree: %d starts", starts)
	}
	waitFor(t, "HOLD pulse", func() bool { p := s.helperPulse(t); return p != nil && p.State == "HOLD" })
	stop()

	out, err := s.h.Stop("site", "stop-1", false)
	if err != nil || field(t, out, "close") != "PENDING" {
		t.Fatalf("stop over an unretired helper tree: %v", err)
	}
	_, err = s.h.Resume("site", "resume-1")
	codeIs(t, err, wire.CodeUncertainEffect)
	_, err = s.h.Uninstall("site", "uninstall-1")
	codeIs(t, err, wire.CodeUncertainEffect)

	// The operator verifies that no process remains and removes the intent.
	if err := os.Remove(filepath.Join(root, helperFile("web", helperIntentSuffix))); err != nil {
		t.Fatal(err)
	}
	out, err = s.h.Resume("site", "resume-2")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := out.Get("helperDebtReset"); len(v.Arr) != 1 || v.Arr[0].Str != "web" || field(t, out, "restartDebt") != "NOT_OBSERVED" {
		t.Fatalf("resume result %s", wire.EncodeFile(wire.ObjectValue(out)))
	}
	if rec := s.helperRecord(t); rec.State != "RETIRED" || rec.Hold != "" || rec.Generation != 1 {
		t.Fatalf("record after resume %+v", rec)
	}
}

func TestSERVICE500_HelperExistingIntentHolds(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	_, ident := s.manifest(t)
	if _, err := writeHelperIntent(s.root(t), helperIntent{Helper: "web", ManifestSha256: ident, Token: "00", Generation: 4, State: "RUNNING", PID: 7, Identity: "x"}); err != nil {
		t.Fatal(err)
	}
	tree := &fakeTree{root: s.root(t)}
	s.runHelper(t, tree, steppingClock())
	waitFor(t, "HOLD pulse", func() bool {
		p := s.helperPulse(t)
		return p != nil && p.State == "HOLD" && strings.Contains(p.Hold, "not proved retired")
	})
	if starts, _ := tree.count(); starts != 0 || s.helperIntent(t).Generation != 4 {
		t.Fatal("a wrapper spawned over an earlier generation's intent")
	}
}

func TestSERVICE500_HelperSingleController(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	f, err := os.OpenFile(filepath.Join(root, helperFile("web", helperLockSuffix)), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := tryLock(f); err != nil {
		t.Fatal(err)
	}
	tree := &fakeTree{root: root}
	stop := s.runHelper(t, tree, steppingClock())
	time.Sleep(60 * time.Millisecond)
	// The refused duplicate exits without writing any helper state or the
	// owner's log counters.
	stop()
	if starts, _ := tree.count(); starts != 0 {
		t.Fatal("a second controller spawned")
	}
	for _, name := range []string{helperFile("web", helperIntentSuffix), helperFile("web", helperRecordSuffix), helperFile("web", helperPulseSuffix), filepath.Join(logDir, helperUnit("web"), logStatusFile)} {
		if _, err := os.Stat(filepath.Join(root, name)); !absent(err) {
			t.Fatalf("a non-controlling wrapper wrote %s", name)
		}
	}
	s.runHelper(t, tree, steppingClock())
	f.Close()
	waitFor(t, "controller takes over", func() bool { return s.helperRecord(t).State == "RUNNING" })
}

// An interruption that arrives while the wrapper waits for F starts
// nothing once F is acquired.
func TestSERVICE500_HelperInterruptedWhileFencedStartsNothing(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	f, err := os.OpenFile(filepath.Join(root, fenceFile), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := tryLock(f); err != nil {
		t.Fatal(err)
	}
	released := false
	t.Cleanup(func() {
		if !released {
			f.Close()
		}
	})
	clocked := make(chan struct{}, 16)
	base := steppingClock()
	tree := &fakeTree{root: root}
	stop := s.runHelper(t, tree, func() (string, uint64, bool) {
		select {
		case clocked <- struct{}{}:
		default:
		}
		return base()
	})
	select {
	case <-clocked:
	case <-time.After(10 * time.Second):
		t.Fatal("wrapper never reached its spawn")
	}
	time.Sleep(50 * time.Millisecond)
	stopped := make(chan struct{})
	go func() { stop(); close(stopped) }()
	time.Sleep(50 * time.Millisecond)
	f.Close()
	released = true
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("wrapper did not stop")
	}
	if starts, _ := tree.count(); starts != 0 {
		t.Fatal("an interrupted wrapper spawned after acquiring F")
	}
	if _, err := os.Stat(filepath.Join(root, helperFile("web", helperIntentSuffix))); !absent(err) {
		t.Fatal("an interrupted wrapper wrote a helper intent")
	}
}

// A SPAWNING helper claim seen under F is an unresolved launch: stop saves
// the suppression but does not acknowledge it, on first answer or replay.
func TestSERVICE500_StopDoesNotAcknowledgeUnresolvedHelperLaunch(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	_, ident := s.manifest(t)
	if _, err := writeHelperIntent(s.root(t), helperIntent{Helper: "web", ManifestSha256: ident, Token: "00", Generation: 1, State: "SPAWNING"}); err != nil {
		t.Fatal(err)
	}
	for _, request := range []string{"stop-1", "stop-1"} {
		out, err := s.h.Stop("site", request, false)
		if err != nil || field(t, out, "state") != "PENDING" || field(t, out, "desired") != "STOPPED" {
			t.Fatalf("stop over an unresolved helper launch: %v %s", err, wire.EncodeFile(wire.ObjectValue(out)))
		}
	}
}

func TestSERVICE500_HelperRestartDebtChargesAndHolds(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	tree := &fakeTree{root: s.root(t), autoExit: true}
	stop := s.runHelper(t, tree, steppingClock())
	waitFor(t, "SERVICE_RESTART_HOLD", func() bool { return s.helperRecord(t).State == "HOLD" })
	stop()
	rec := s.helperRecord(t)
	if rec.Debt.Failures != 6 || !strings.Contains(rec.Hold, "SERVICE_RESTART_HOLD") || s.helperIntent(t) != nil {
		t.Fatalf("record %+v", rec)
	}
	if starts, retires := tree.count(); starts != 6 || retires != 6 {
		t.Fatalf("starts=%d retires=%d", starts, retires)
	}

	// A record retry for the same generation (its intent still present
	// after a partly published record) is a REPLAY: no second charge.
	_, ident := s.manifest(t)
	if _, err := writeHelperIntent(s.root(t), helperIntent{Helper: "web", ManifestSha256: ident, Token: "t", Generation: rec.Generation, State: "RUNNING"}); err != nil {
		t.Fatal(err)
	}
	w := &helperWrapper{o: HelperOptions{RunOptions: s.runOptions(t, nil), Helper: "web"}, root: s.root(t), ctx: context.Background()}
	settled := &helperSettlement{gen: rec.Generation, token: "t", exit: "exited", reason: "helper exited", charge: true, termination: "PROVED_TERMINATED"}
	if err := w.record(settled, "boot-1", 1<<40, true); err != nil || s.helperIntent(t) != nil {
		t.Fatalf("replay %v", err)
	}
	if again := s.helperRecord(t); again.Debt.Failures != 6 || again.State != "HOLD" || again.Hold != rec.Hold {
		t.Fatalf("replay changed the record: %+v", again)
	}

	// Resume resets helper debt; an unknown clock then holds before any start.
	if _, err := s.h.Stop("site", "stop-1", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.h.Resume("site", "resume-1"); err != nil {
		t.Fatal(err)
	}
	if rec := s.helperRecord(t); rec.Debt.Failures != 0 || rec.State != "RETIRED" || rec.Debt.FenceFloor != 6 {
		t.Fatalf("debt after resume %+v", rec.Debt)
	}
	unknown := &fakeTree{root: s.root(t)}
	s.runHelper(t, unknown, func() (string, uint64, bool) { return "", 0, false })
	waitFor(t, "HOLD on unknown time", func() bool {
		p := s.helperPulse(t)
		return p != nil && p.State == "HOLD" && strings.Contains(p.Hold, "restart debt HOLD")
	})
	if starts, _ := unknown.count(); starts != 0 {
		t.Fatal("started with an unknown boot clock")
	}
}

func TestSERVICE500_HelperStartFailureChargesWithoutTree(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	tree := &fakeTree{root: s.root(t), startErr: errors.New("exec format error")}
	s.runHelper(t, tree, func() (string, uint64, bool) { return "boot-1", 100, true })
	waitFor(t, "start failure recorded", func() bool {
		r := s.helperRecord(t)
		return r.Debt.Failures == 1 && s.helperIntent(t) == nil
	})
	if r := s.helperRecord(t); r.State != "RETIRED" || r.Debt.Fences["1"] != "FAILED" {
		t.Fatalf("record %+v", r)
	}
}

func TestSERVICE500_HelperQuietRefusesLeftoverChild(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	tree := &fakeTree{root: s.root(t), quiet: errors.New("a live child of this wrapper remains")}
	s.runHelper(t, tree, steppingClock())
	waitFor(t, "HOLD pulse", func() bool { p := s.helperPulse(t); return p != nil && p.State == "HOLD" })
	if starts, _ := tree.count(); starts != 0 {
		t.Fatal("spawned beside a leftover child")
	}
}

func TestSERVICE500_HelperStatusIsPureRead(t *testing.T) {
	s, sd := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	before := listTree(t, s.home)
	calls := len(sd.calls)
	out, err := s.h.Status("site")
	if err != nil {
		t.Fatal(err)
	}
	text := string(wire.EncodeFile(wire.ObjectValue(out)))
	for _, want := range []string{`"helperTrees":[]`, `"id":"web"`, `"tree":"ABSENT"`, `"state":"RETIRED"`, `"restartDebt"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("status lacks %s: %s", want, text)
		}
	}
	if after := listTree(t, s.home); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatal("status changed state")
	}
	for _, c := range sd.calls[calls:] {
		if !strings.HasPrefix(c, "systemctl --user show ") {
			t.Fatalf("status ran a manager effect: %s", c)
		}
	}
	// With logs and an intent present status reports them without text.
	logs := openUnitLogs(root, helperUnit("web"), "stdout", "stderr")
	_, _ = logs.streams["stdout"].Write([]byte("token=abc\n"))
	logs.close()
	_, ident := s.manifest(t)
	if _, err := writeHelperIntent(root, helperIntent{Helper: "web", ManifestSha256: ident, Token: "00", Generation: 1, State: "SPAWNING"}); err != nil {
		t.Fatal(err)
	}
	out, err = s.h.Status("site")
	if err != nil {
		t.Fatal(err)
	}
	text = string(wire.EncodeFile(wire.ObjectValue(out)))
	if strings.Contains(text, "token=abc") || !strings.Contains(text, `"excerpt":"WITHHELD"`) || !strings.Contains(text, `"helperTrees":["web"]`) || !strings.Contains(text, `"tree":"SPAWNING"`) {
		t.Fatalf("status %s", text)
	}
}

func listTree(t *testing.T, dir string) []string {
	t.Helper()
	out := []string{}
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		out = append(out, p+" "+fi.ModTime().String()+" "+fi.Mode().String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSERVICE500_HelperRecordIsBounded(t *testing.T) {
	root := t.TempDir()
	r := helperRecord{Profile: HelperRecordName, Program: "site", Helper: "web", State: "HOLD", Hold: strings.Repeat("h", 20000), LastExit: strings.Repeat("x", 20000), Debt: Debt{Fences: map[string]string{}}}
	if err := writeHelperRecord(root, r); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(root, helperFile("web", helperRecordSuffix)))
	if err != nil || fi.Size() > maxHelperRecord {
		t.Fatalf("record size %v %v", fi, err)
	}
}

func TestSERVICE500_RunHelperRefusesUnsupportedAndForeignManifest(t *testing.T) {
	s := newServiceHome(t)
	root := s.root(t)
	o := HelperOptions{RunOptions: s.runOptions(t, nil), Helper: "web"}
	codeIs(t, RunHelper(context.Background(), o), wire.CodeUnsupported)
	o.GOOS = "linux"
	o.Manifest = filepath.Join(root, "other.json")
	codeIs(t, RunHelper(context.Background(), o), wire.CodeUnsupported)
	o.Manifest, o.Helper = filepath.Join(root, manifestFile), "Bad"
	codeIs(t, RunHelper(context.Background(), o), wire.CodeMalformed)
}

func TestSERVICE500_HelperReinstallWaitsForRetiredTree(t *testing.T) {
	s, sd := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	tree := &fakeTree{root: s.root(t), gate: make(chan struct{})}
	s.runHelper(t, tree, steppingClock())
	var release sync.Once
	open := func() { release.Do(func() { close(tree.gate) }) }
	t.Cleanup(open)
	waitFor(t, "helper RUNNING", func() bool { return s.helperRecord(t).State == "RUNNING" })
	p, err := DecodeProfile(mustRead(t, s.profile))
	if err != nil {
		t.Fatal(err)
	}
	p.Helpers[0].Env["MODE"] = "next"
	raw, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.profile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	before := sd.labels()
	// The replace saves STOPPED, finds the tree unretired and rolls back
	// before any unit changes.
	_, err = s.install(t, "install-2", true)
	if err == nil || !strings.Contains(err.Error(), "helper tree is not proved retired") {
		t.Fatalf("replace over a live helper tree: %v", err)
	}
	if s.helperIntent(t) == nil || strings.Join(sd.labels(), ",") != strings.Join(before, ",") {
		t.Fatal("units changed while the helper tree was live")
	}
	if m, _ := s.manifest(t); m.Generation.Uint64() != 1 {
		t.Fatal("the refused replace committed")
	}
	// The rollback restored the prior control, so the wrapper may still run
	// or may have restarted the helper; a retry with a real wait succeeds
	// once retirement is unblocked.
	open()
	s.h.Sleep = func(time.Duration) { time.Sleep(20 * time.Millisecond) }
	if _, err := s.install(t, "install-3", true); err != nil {
		t.Fatalf("retry after retirement: %v", err)
	}
	if m, _ := s.manifest(t); m.Generation.Uint64() != 2 {
		t.Fatal("replace did not commit")
	}
	if _, retires := tree.count(); retires < 1 {
		t.Fatal("replace committed without retiring the helper tree")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// A settlement retry never overwrites a record that a resume reconciled
// after the intent was removed, and an outcome whose intent vanished
// before it was recorded holds instead of being written.
func TestSERVICE500_HelperSettlementRetryKeepsResumedRecord(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	tree := &fakeTree{root: root, autoExit: true}
	stop := s.runHelper(t, tree, steppingClock())
	waitFor(t, "SERVICE_RESTART_HOLD", func() bool { return s.helperRecord(t).State == "HOLD" })
	stop()
	gen := s.helperRecord(t).Generation
	if _, err := s.h.Stop("site", "stop-1", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.h.Resume("site", "resume-1"); err != nil {
		t.Fatal(err)
	}
	resumed := s.helperRecord(t)
	if resumed.State != "RETIRED" || resumed.Debt.Failures != 0 {
		t.Fatalf("resume %+v", resumed)
	}
	w := &helperWrapper{o: HelperOptions{RunOptions: s.runOptions(t, nil), Helper: "web"}, root: root, ctx: context.Background()}
	// Recorded before the intent removal failed: the retry finds the
	// intent gone and leaves the resumed record alone.
	recorded := &helperSettlement{gen: gen, token: "t", exit: "exited", reason: "helper exited", charge: true, termination: "PROVED_TERMINATED", recorded: true}
	if err := w.record(recorded, "boot-1", 1<<40, true); err != nil {
		t.Fatal(err)
	}
	// Never recorded and its intent is gone: HOLD, nothing written.
	unrecorded := &helperSettlement{gen: gen, token: "t", exit: "exited", reason: "helper exited", charge: true, termination: "PROVED_TERMINATED"}
	if err := w.record(unrecorded, "boot-1", 1<<40, true); wire.CodeOf(err) != wire.CodeUncertainEffect {
		t.Fatalf("unrecorded settlement without its intent: %v", err)
	}
	if again := s.helperRecord(t); !reflect.DeepEqual(again, resumed) {
		t.Fatalf("a settlement retry overwrote the resumed record:\n%+v\n%+v", again, resumed)
	}
}

// resumePartial applies request's helper resets under F and stops before
// the control write: the state an interrupted or failed resume leaves.
func (s *serviceHome) resumePartial(t *testing.T, request string) {
	t.Helper()
	root := s.root(t)
	m, ident := s.manifest(t)
	c := s.control(t)
	rs, err := s.h.controlRequests(root, c)
	if err != nil {
		t.Fatal(err)
	}
	unfence, err := s.h.fence(root)
	if err != nil {
		t.Fatal(err)
	}
	defer unfence()
	if _, _, _, err := s.h.resumeHelpers(root, m, *c, rs, request, wire.Sum([]byte(request+"\nRESUME\n"+string(ident)))); err != nil {
		t.Fatal(err)
	}
}

func (s *serviceHome) chargeHelper(t *testing.T, failures uint8) {
	t.Helper()
	rec := s.helperRecord(t)
	rec.Generation++
	rec.Debt.Failures, rec.Debt.EligibleAfter, rec.LastExit = failures, 1000, "exit status 1"
	if err := writeHelperRecord(s.root(t), rec); err != nil {
		t.Fatal(err)
	}
}

// A resume interrupted after its helper resets is reconciled by its own
// retry, refused when debt or control changed after it, and superseded by
// a later resume so that its retry never erases debt charged since.
func TestSERVICE500_ResumeJournalNeverErasesLaterHelperDebt(t *testing.T) {
	s, _ := newHelperHome(t)
	if _, err := s.install(t, "install-1", false); err != nil {
		t.Fatal(err)
	}
	root := s.root(t)
	journal := filepath.Join(root, resumeOperationFile)
	stop := func(request string) {
		t.Helper()
		if _, err := s.h.Stop("site", request, false); err != nil {
			t.Fatal(err)
		}
	}
	stop("stop-1")
	s.chargeHelper(t, 3)

	// Same-request retry completes the partial reset.
	before := *s.control(t)
	s.resumePartial(t, "resume-a")
	if rec := s.helperRecord(t); rec.Debt.Failures != 0 || *s.control(t) != before {
		t.Fatalf("partial resume: record %+v control %+v", rec, *s.control(t))
	}
	if _, err := os.Stat(journal); err != nil {
		t.Fatalf("partial resume left no journal: %v", err)
	}
	out, err := s.h.Resume("site", "resume-a")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := out.Get("helperDebtReset"); len(v.Arr) != 1 || v.Arr[0].Str != "web" || field(t, out, "desired") != "RUNNING" {
		t.Fatalf("reconciled retry %s", wire.EncodeFile(wire.ObjectValue(out)))
	}
	if _, err := os.Stat(journal); !os.IsNotExist(err) {
		t.Fatalf("completed resume kept its journal: %v", err)
	}

	// Debt charged after the journaled reset is kept and refuses the retry.
	stop("stop-2")
	s.chargeHelper(t, 2)
	s.resumePartial(t, "resume-b")
	s.chargeHelper(t, 4)
	_, err = s.h.Resume("site", "resume-b")
	codeIs(t, err, wire.CodeResourceCollision)
	if rec := s.helperRecord(t); rec.Debt.Failures != 4 {
		t.Fatalf("a refused retry changed debt: %+v", rec.Debt)
	}

	// A retry after control changed is refused.
	s.chargeHelper(t, 1)
	s.resumePartial(t, "resume-c")
	stop("stop-3")
	_, err = s.h.Resume("site", "resume-c")
	codeIs(t, err, wire.CodeResourceCollision)

	// A later resume supersedes the unfinished one; after new debt and a
	// stop, the superseded retry is a conflict and erases nothing.
	s.chargeHelper(t, 5)
	s.resumePartial(t, "resume-d")
	if _, err := s.h.Resume("site", "resume-e"); err != nil {
		t.Fatal(err)
	}
	s.chargeHelper(t, 2)
	stop("stop-4")
	_, err = s.h.Resume("site", "resume-d")
	codeIs(t, err, wire.CodeRequestIDConflict)
	if rec := s.helperRecord(t); rec.Debt.Failures != 2 {
		t.Fatalf("a superseded retry erased later debt: %+v", rec.Debt)
	}
	if out, err := s.h.Resume("site", "resume-e"); err != nil || field(t, out, "replayed") != "true" {
		t.Fatalf("completed resume did not replay: %v", err)
	}
	if rec := s.helperRecord(t); rec.Debt.Failures != 2 {
		t.Fatalf("a completed replay erased later debt: %+v", rec.Debt)
	}
}
