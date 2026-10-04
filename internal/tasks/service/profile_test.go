package service

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"reflect"
	"strings"
	"testing"
)

func serviceFixture(t *testing.T, manager string) (Profile, InstallationFacts, *Manifest) {
	t.Helper()
	p := Profile{Executable: "/Applications/Corvint/bin/corvint-tasks", DispatchConfig: "/Users/alice/config/dispatch.json", WorkRoot: "/Users/alice/project", Helpers: []Helper{}}
	f := InstallationFacts{State: "VERIFIED", Manager: manager, Program: "site", QueueID: "queue:acme:main", UID: "501", Generation: "1", ConfigRoot: "/Users/alice/config", StateRoot: "/Users/alice/state", UnitRoot: "/Users/alice/units", ManifestPath: "/Users/alice/state/site/manifest.json", DispatchStateRoot: "/Users/alice/state/dispatch/site", CanonicalStore: p.WorkRoot, ConfigWorkRoot: p.WorkRoot, ConfigStateRoot: "/Users/alice/state/dispatch/site", RootsSafe: true, ManagerReachable: true, HelperExecutables: map[string]FileFacts{}}
	if manager == "launchd" {
		f.Domain = "gui/501"
	} else {
		f.Domain = "user/501"
	}
	f.Executable = FileFacts{State: "VERIFIED", Path: p.Executable, DeclaredPath: p.Executable, Owner: "0", Mode: 0755, Regular: true, Executable: true, SafeAncestors: true, NoReplacementSymlink: true, Sha256: wire.Sum([]byte("exe"))}
	f.DispatchConfig = FileFacts{State: "VERIFIED", Path: p.DispatchConfig, DeclaredPath: p.DispatchConfig, Owner: "501", Mode: 0600, Regular: true, SafeAncestors: true, NoReplacementSymlink: true, Sha256: wire.Sum([]byte("dispatch"))}
	raw, err := p.Encode()
	if err != nil {
		t.Fatal("initialize profile", err)
	}
	back, err := DecodeProfile(raw)
	if err != nil {
		t.Fatal("initialize decode", err)
	}
	again, err := back.Encode()
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("canonical fixture", err)
	}
	m, err := BuildManifest(p, f)
	if err != nil {
		t.Fatal("initialize manifest", err)
	}
	if _, err = m.Encode(); err != nil {
		t.Fatal(err)
	}
	return p, f, m
}
func serviceOwned(m Manifest) []ExistingUnit {
	a := []ExistingUnit{}
	for _, u := range m.Units {
		a = append(a, ExistingUnit{Label: u.Label, Path: u.Path, Sha256: wire.Sum(u.Raw), Ownership: "OWNED", Registration: "PRESENT", NoSymlink: true, NoDropIns: true})
	}
	return a
}
func TestIssue500_ProfileClosedAndBounded(t *testing.T) {
	p, f, m := serviceFixture(t, "launchd")
	raw, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string][]byte{"extra": bytes.Replace(raw, []byte(`"profile":`), []byte(`"extra":null,"profile":`), 1), "duplicate": bytes.Replace(raw, []byte(`"helpers":[]`), []byte(`"helpers":[],"helpers":[]`), 1), "missing": bytes.Replace(raw, []byte(`"legacyStopFile":null,`), nil, 1), "overbytes": bytes.Repeat([]byte("x"), 64*wire.KiB+1)} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeProfile(bad); err == nil {
				t.Fatal("closed bound accepted")
			}
		})
	}
	for name, spoil := range map[string]func(*Profile){"relative": func(p *Profile) { p.WorkRoot = "relative" }, "temporary": func(p *Profile) { p.WorkRoot = "/tmp/site" }, "control": func(p *Profile) { p.Executable += "\n" }, "helpers": func(p *Profile) { p.Helpers = make([]Helper, 9) }, "foreground": func(p *Profile) {
		p.Helpers = []Helper{{ID: "h", Cwd: p.WorkRoot, Argv: []string{p.Executable}, Env: map[string]string{}}}
	}} {
		t.Run(name, func(t *testing.T) {
			q, _, _ := serviceFixture(t, "launchd")
			spoil(&q)
			if _, err := q.Encode(); err == nil {
				t.Fatal("invalid profile")
			}
		})
	}
	for name, spoil := range map[string]func(*InstallationFacts){"manager": func(f *InstallationFacts) { f.ManagerReachable = false }, "root": func(f *InstallationFacts) { f.RootsSafe = false }, "domain": func(f *InstallationFacts) { f.Domain = "system" }, "owner": func(f *InstallationFacts) { f.Executable.Owner = "999" }, "writable": func(f *InstallationFacts) { f.DispatchConfig.Mode = 0666 }, "symlink": func(f *InstallationFacts) { f.Executable.NoReplacementSymlink = false }, "store": func(f *InstallationFacts) { f.ConfigWorkRoot = "/other" }} {
		t.Run(name, func(t *testing.T) {
			q, ff, _ := serviceFixture(t, "launchd")
			spoil(&ff)
			if _, err := BuildManifest(q, ff); err == nil {
				t.Fatal("unknown/foreign observed facts accepted")
			}
		})
	}
	again, err := BuildManifest(p, f)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.Encode()
	b, _ := again.Encode()
	if !bytes.Equal(a, b) {
		t.Fatal("manifest nondeterministic")
	}
	v, err := wire.Parse(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.Obj.Get("config"); !ok {
		t.Fatal("closed helper/config snapshot absent")
	}
}
func debtFixture() Debt { return Debt{BootID: "boot", Fences: map[string]string{}} }
func TestIssue500_FiniteRestartDebtAndReset(t *testing.T) {
	d := debtFixture()
	times := []uint64{0, 30, 90, 210, 450, 750}
	deadlines := []uint64{30, 90, 210, 450, 750}
	for i, now := range times {
		o := FailureObservation{Generation: string(rune('a' + i)), Outcome: "FAILED", BootID: "boot", Termination: "PROVED_TERMINATED", Now: now, TimeKnown: true, IdentityKnown: true}
		next, state, err := ChargeFailure(d, o)
		if err != nil || next.Failures != uint8(i+1) {
			t.Fatalf("failure%d: %v", i+1, err)
		}
		if i < 5 {
			if state != "BACKOFF" || next.EligibleAfter != deadlines[i] {
				t.Fatal("backoff ladder")
			}
		} else if state != "SERVICE_RESTART_HOLD" {
			t.Fatal("sixth failure not HOLD")
		}
		replay, rstate, err := ChargeFailure(next, o)
		if err != nil || rstate != "REPLAY" || !reflect.DeepEqual(replay, next) {
			t.Fatal("duplicate outcome charged")
		}
		o.Outcome = "different"
		if _, _, err = ChargeFailure(next, o); err == nil {
			t.Fatal("generation conflict accepted")
		}
		d = next
	}
	for _, now := range []uint64{751, 1000000} {
		if _, state := Eligibility(d, "boot", now, true); state != "SERVICE_RESTART_HOLD" {
			t.Fatal("generation7 admitted")
		}
	}
	for _, outcome := range []string{"INTENTIONAL_STOP", "IDLE_WRAPPER_RESTART"} {
		prior := debtFixture()
		prior.Failures = 3
		prior.Delay = 120
		prior.EligibleAfter = 999
		got, state, err := ChargeFailure(prior, FailureObservation{Generation: "idle", Outcome: outcome, BootID: "boot", Termination: "PROVED_TERMINATED", TimeKnown: true, IdentityKnown: true})
		if err != nil || state != "NO_CHARGE" || got.Failures != 3 || got.Delay != 120 || got.EligibleAfter != 999 {
			t.Fatal("intentional stop/idle changed debt", err)
		}
	}
	five := debtFixture()
	five.Failures = 5
	five.Delay = 300
	five.EligibleAfter = 750
	healthy := HealthObservation{BootID: "boot", Generation: "live", Desired: "RUNNING", State: "HEALTHY", TimeKnown: true, PinnedValid: true, PulseTimely: true, TickHealthy: true, OwnershipSettled: true}
	measured := five
	for now := uint64(0); now <= 590; now += 10 {
		healthy.Now = now
		measured = ObserveHealth(measured, healthy)
	}
	healthy.Now = 599
	measured = ObserveHealth(measured, healthy)
	if measured.Failures != 5 {
		t.Fatal("599 seconds reset")
	}
	healthy.Now = 600
	measured = ObserveHealth(measured, healthy)
	if measured.Failures != 0 || len(measured.Fences) != len(five.Fences) {
		t.Fatal("600 healthy seconds failed reset")
	}
	for name, spoil := range map[string]func(*HealthObservation){"stopped": func(o *HealthObservation) { o.Desired = "STOPPED" }, "missing-pulse": func(o *HealthObservation) { o.PulseTimely = false }, "tick": func(o *HealthObservation) { o.TickHealthy = false }, "boot": func(o *HealthObservation) { o.BootID = "other" }} {
		t.Run(name, func(t *testing.T) {
			o := healthy
			spoil(&o)
			if got := ObserveHealth(five, o); got.Failures != 5 || got.HealthySince != nil {
				t.Fatal("unproved healthy reset")
			}
		})
	}
	newBoot, state := Eligibility(five, "new-boot", 10, true)
	if state != "BACKOFF" || newBoot.EligibleAfter != 310 || newBoot.Failures != 5 {
		t.Fatal("boot debt shortened")
	}
	if _, state = Eligibility(five, "", 1000, false); state != "HOLD" {
		t.Fatal("unknown time admitted")
	}
	// Explicit resume preserves worker identity and native debt in STOPPED and
	// already RUNNING states; only transient/helper retirement is a prerequisite.
	for _, desired := range []string{"STOPPED", "RUNNING"} {
		c := Control{Program: "site", ManifestIdentity: wire.Sum([]byte("manifest")), Revision: "1", Desired: desired}
		native := map[string]string{"retry": "6", "park": "true", "attempt": "same", "budget": "42"}
		f := ResumeFacts{Control: ControlFacts{FenceHeld: true, Fresh: true, PinnedValid: true, Published: true, ObservedRevision: "1", Legacy: "ABSENT"}, Trees: []OwnedTree{{Identity: "live-worker", Kind: "WORKER", State: "LIVE", Ownership: "VERIFIED"}, {Identity: "controller-transient", Kind: "TRANSIENT", State: "RETIRED", Ownership: "VERIFIED"}, {Identity: "helper-tree", Kind: "HELPER", State: "RETIRED", Ownership: "VERIFIED"}}, LaunchIntentsReconciled: true, AllResetsPublished: true, NativeSentinels: native}
		debts := map[string]Debt{"main": d, "helper": five}
		p, err := Resume(c, "resume-1", debts, f)
		if err != nil || p.State != "RESUMED" || p.Debts["main"].Failures != 0 || !reflect.DeepEqual(p.NativeSentinels, native) || len(p.PreservedWorkers) != 1 || p.PreservedWorkers[0] != "live-worker" || p.Launch {
			t.Fatalf("worker-preserving resume %+v %v", p, err)
		}
		if debts["main"].Failures != 6 {
			t.Fatal("input debt mutated")
		}
		f.Control.ObservedRevision = p.Control.Revision
		f.Operation = p.Operation
		replay, err := Resume(p.Control, "resume-1", p.Debts, f)
		if err != nil || replay.Control.Revision != p.Control.Revision {
			t.Fatal("resume replay increments revision")
		}
		f.Operation = nil
		f.Control.ObservedRevision = c.Revision
		f.AllResetsPublished = false
		pending, err := Resume(c, "resume-2", debts, f)
		if err != nil || pending.State != "RESUME_PENDING" || pending.Debts["main"].Failures != 6 {
			t.Fatal("partial reset acknowledged")
		}
		f.AllResetsPublished = true
		f.Trees[1].State = "UNKNOWN"
		pending, err = Resume(c, "resume-3", debts, f)
		if err != nil || pending.State != "RESUME_PENDING" || pending.Launch {
			t.Fatal("unknown transient tree admitted")
		}
	}
}
func TestIssue500_ControlLaunchFencePlan(t *testing.T) {
	c := Control{Program: "site", ManifestIdentity: wire.Sum([]byte("manifest")), Revision: "1", Desired: "RUNNING"}
	f := ControlFacts{FenceHeld: true, Fresh: true, PinnedValid: true, ObservedRevision: "1", Legacy: "ABSENT", IntentStates: []string{"CHECKED_SAVED"}}
	if p := PlanLaunch(c, f); !p.Launch {
		t.Fatal("valid hypothetical launch")
	}
	bad := f
	bad.FenceHeld = false
	if PlanLaunch(c, bad).Launch {
		t.Fatal("reread without fence admitted")
	}
	stop := f
	stop.Published = true
	stop.IntentStates = []string{"UNKNOWN"}
	stop.CommittedWorkers = []string{"pre-stop-worker"}
	p, err := Suppress(c, "stop-1", "STOPPED", stop)
	if err != nil || p.State != "SUPPRESSION_PENDING" || p.Launch {
		t.Fatal("ambiguous launch acknowledged")
	}
	if PlanLaunch(p.Control, f).Launch {
		t.Fatal("stale RUNNING tick admitted")
	}
	stop.IntentStates = []string{"COMMITTED", "PROVED_NO_EFFECT"}
	p, err = Suppress(c, "stop-1", "STOPPED", stop)
	if err != nil || p.State != "SUPPRESSION_ACKNOWLEDGED" || len(p.PreservedWorkers) != 1 {
		t.Fatal("settled pre-stop launch missing")
	}
	stop.ObservedRevision = p.Control.Revision
	again, err := Suppress(p.Control, "stop-1", "STOPPED", stop)
	if err != nil || again.Control.Revision != p.Control.Revision {
		t.Fatal("lost reply replay")
	}
	if _, err = Suppress(p.Control, "stop-1", "DRAINING", stop); err == nil {
		t.Fatal("same request conflict")
	}
	stop.FenceHeld = false
	if _, err = Suppress(c, "x", "STOPPED", stop); err == nil {
		t.Fatal("stop without fence")
	}
}
func TestIssue500_LegacyPresenceLatchRevision(t *testing.T) {
	if LegacyPresence(LegacyObservation{ParentVerified: true, FileState: "ENOENT"}) != "ABSENT" {
		t.Fatal("verified absent")
	}
	for _, state := range []string{"EACCES", "IO_ERROR", "SYMLINK", "UNKNOWN"} {
		if LegacyPresence(LegacyObservation{ParentVerified: true, FileState: state}) != "UNKNOWN" {
			t.Fatal("legacy unknown")
		}
	}
	if LegacyPresence(LegacyObservation{FileState: "ENOENT"}) != "UNKNOWN" {
		t.Fatal("unknown parent")
	}
	c := Control{Program: "site", ManifestIdentity: wire.Sum([]byte("manifest")), Revision: "1", Desired: "RUNNING"}
	f := ControlFacts{FenceHeld: true, Fresh: true, PinnedValid: true, Published: true, ObservedRevision: "1", Legacy: "PRESENT", IntentStates: []string{"PROVED_NO_EFFECT"}}
	p := LegacyLatch(c, "MAIN", "legacy-1", f)
	if p.Control.Desired != "DRAINING" || p.Control.Revision != "2" {
		t.Fatal("main did not latch")
	}
	if h := LegacyLatch(c, "HELPER", "x", f); h.Control != c || len(h.Actions) != 0 {
		t.Fatal("helper wrote")
	}
	f.Legacy = "ABSENT"
	f.ObservedRevision = "2"
	if again := LegacyLatch(p.Control, "MAIN", "x", f); again.Control.Desired != "DRAINING" {
		t.Fatal("removal cleared latch")
	}
	stopped := c
	stopped.Desired = "STOPPED"
	f.Legacy = "PRESENT"
	f.ObservedRevision = "1"
	if p := LegacyLatch(stopped, "MAIN", "x", f); p.Control.Desired != "STOPPED" {
		t.Fatal("presence overwrote STOPPED")
	}
	f.ObservedRevision = "0"
	if LegacyLatch(c, "MAIN", "x", f).State != "HOLD" {
		t.Fatal("stale legacy observation")
	}
	if _, err := Resume(c, "resume", map[string]Debt{"main": debtFixture()}, ResumeFacts{Control: f}); err == nil {
		t.Fatal("PRESENT resume accepted")
	}
	if strings.Contains(ProfileName, "default") {
		t.Fatal("optional profile lost")
	}
}

func TestIssue500_ResumeOperationReplayAndPartialPublication(t *testing.T) {
	c := Control{Program: "site", ManifestIdentity: wire.Sum([]byte("manifest")), Revision: "1", Desired: "STOPPED"}
	d := debtFixture()
	d.Failures = 3
	d.Delay = 120
	d.EligibleAfter = 500
	debts := map[string]Debt{"main": d, "helper": d}
	f := ResumeFacts{Control: ControlFacts{FenceHeld: true, Fresh: true, PinnedValid: true, Published: true, ObservedRevision: "1", Legacy: "ABSENT"}, LaunchIntentsReconciled: true, AllResetsPublished: true, Trees: []OwnedTree{{Identity: "worker", Kind: "WORKER", State: "LIVE", Ownership: "VERIFIED"}}}
	first, err := Resume(c, "R", debts, f)
	if err != nil || first.State != "RESUMED" || first.Operation == nil || !first.Operation.Complete {
		t.Fatal("initialized resume", first, err)
	}
	charged, state, err := ChargeFailure(first.Debts["main"], FailureObservation{Generation: "later", Outcome: "FAILED", BootID: "boot", Termination: "PROVED_TERMINATED", Now: 600, TimeKnown: true, IdentityKnown: true})
	if err != nil || state != "BACKOFF" || charged.Failures != 1 {
		t.Fatal("reached post-resume charge", state, err)
	}
	later := map[string]Debt{"main": charged, "helper": first.Debts["helper"]}
	f.Control.ObservedRevision = first.Control.Revision
	f.Operation = first.Operation
	replay, err := Resume(first.Control, "R", later, f)
	if err != nil || replay.State != "RESUMED_REPLAY" || len(replay.Actions) != 0 || !reflect.DeepEqual(replay.Debts, later) || replay.Control != first.Control {
		t.Fatal("completed replay erased later debt", replay, err)
	}
	f.Operation = nil
	if _, err = Resume(first.Control, "R", later, f); err == nil {
		t.Fatal("missing replay evidence admitted")
	}
	f.Control.ObservedRevision = c.Revision
	f.AllResetsPublished = false
	partial, err := Resume(c, "partial", debts, f)
	if err != nil || partial.Operation == nil || partial.State != "RESUME_PENDING" {
		t.Fatal("initial partial", err)
	}
	op := cloneResumeOperation(partial.Operation)
	op.PublishedUnits["helper"] = true
	mixed := map[string]Debt{"main": d, "helper": op.AfterDebts["helper"]}
	f.Operation = op
	pending, err := Resume(c, "partial", mixed, f)
	if err != nil || pending.State != "RESUME_PENDING" || !reflect.DeepEqual(pending.Debts, mixed) {
		t.Fatal("partial reconciliation", err)
	}
	resets := 0
	for _, a := range pending.Actions {
		if a.Kind == "CHECKED_PUBLISH_SERVICE_DEBT_RESET" {
			resets++
			if a.Label != "main" {
				t.Fatal("republished completed unit")
			}
		}
	}
	if resets != 1 {
		t.Fatal("missing remainder reset")
	}
	f.AllResetsPublished = true
	completed, err := Resume(c, "partial", mixed, f)
	if err != nil || completed.State != "RESUMED" || !completed.Operation.Complete || completed.Debts["main"].Failures != 0 {
		t.Fatal("partial finish", err)
	}
	mixed["main"] = charged
	if _, err = Resume(c, "partial", mixed, f); err == nil {
		t.Fatal("conflicting pre-debt overwritten")
	}
}
