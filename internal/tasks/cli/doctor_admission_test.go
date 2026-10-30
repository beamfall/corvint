//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func writeAdmission(t *testing.T, programDir string, rec dispatch.AdmissionRecord) {
	t.Helper()
	if err := os.MkdirAll(programDir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(programDir, dispatch.AdmissionFile), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func admissionFixture(at time.Time, pressure string, heldTicks, deepTicks int) dispatch.AdmissionRecord {
	since := at.Add(-time.Minute)
	waitSince := at.Add(-2 * time.Minute)
	q := dispatch.RoleQueue{Role: "impl", Cap: 1, CapMax: 3, EffectiveCap: 1, Running: 1, Waiting: 4, DeepTicks: deepTicks, OldestWaitTicket: "ticket:a:q:old", OldestWaitSince: &waitSince}
	if deepTicks > 0 {
		q.DeepSince = &since
	}
	return dispatch.AdmissionRecord{Profile: dispatch.AdmissionProfile, Program: "prog", At: at, TickSeconds: 30, Pressure: pressure, GlobalCap: 4, Running: 2,
		Roles: []dispatch.RoleQueue{q},
		Held:  []dispatch.HeldRunner{{Role: "runner", Ticket: "ticket:a:q:r1", Pool: "lanes", Fence: dispatch.FencePlanWindow, Detail: "outside the plan window", Free: 2, FreeMembers: []string{"m1", "m2"}, Ticks: heldTicks, Since: since}}}
}

// TQD-V0-043: a run-only ticket held beside free pool members for the
// threshold's ticks is RUNNABLE_HELD_IDLE with the rule named; fewer ticks,
// a missing, stale or unreadable observation report nothing.
func TestTQDV0043_RunnableHeldIdle(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	prog := filepath.Join(t.TempDir(), "prog")
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`"}]}`)
	if x, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "RUNNABLE_HELD_IDLE")) != 0 || strings.Contains(strings.Join(x.res.Warnings, "\n"), "queue observation") {
		t.Fatalf("missing observation: %s", x.stdout)
	}
	now := time.Now().UTC().Truncate(time.Second)
	writeAdmission(t, prog, admissionFixture(now, dispatch.AdmissionPressureUnknown, 3, 0))
	_, item := doctorOK(t, root, "--config", cfg)
	got := findingsOf(item, "RUNNABLE_HELD_IDLE")
	if len(got) != 1 || field(got[0], "who").Str != "prog/runner/ticket:a:q:r1" || field(got[0], "firstSeen").Str != now.Add(-time.Minute).Format(time.RFC3339) {
		t.Fatalf("finding: %s", wire.Encode(item))
	}
	for _, want := range []string{"PLAN_WINDOW (outside the plan window)", "3 tick(s)", "pool lanes had 2 free member(s): m1, m2"} {
		if !strings.Contains(field(got[0], "detail").Str, want) {
			t.Fatalf("detail lacks %q: %s", want, field(got[0], "detail").Str)
		}
	}
	if !strings.Contains(field(got[0], "remedy").Str, "bypassFocus") {
		t.Fatalf("remedy: %s", wire.Encode(got[0]))
	}
	writeAdmission(t, prog, admissionFixture(now, dispatch.AdmissionPressureCalm, 2, 0))
	if _, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "RUNNABLE_HELD_IDLE")) != 0 {
		t.Fatalf("below threshold: %s", wire.Encode(item))
	}
	low := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","thresholds":{"runnableHeldIdleTicks":"2"},"dispatchers":[{"programDir":"`+prog+`"}]}`)
	if _, item := doctorOK(t, root, "--config", low); len(findingsOf(item, "RUNNABLE_HELD_IDLE")) != 1 {
		t.Fatalf("configured threshold: %s", wire.Encode(item))
	}
	writeAdmission(t, prog, admissionFixture(now.Add(-time.Hour), dispatch.AdmissionPressureCalm, 9, 9))
	x, item := doctorOK(t, root, "--config", cfg)
	if len(findingsOf(item, "RUNNABLE_HELD_IDLE"))+len(findingsOf(item, "ROLE_QUEUE_DEEP")) != 0 || !strings.Contains(strings.Join(x.res.Warnings, "\n"), "queue observation is stale") {
		t.Fatalf("stale: %s", x.stdout)
	}
	if err := os.WriteFile(filepath.Join(prog, dispatch.AdmissionFile), []byte(`{"profile":"taskman-dispatch-admission/9"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if x, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "RUNNABLE_HELD_IDLE")) != 0 || !strings.Contains(strings.Join(x.res.Warnings, "\n"), "queue observation unreadable") {
		t.Fatalf("unreadable: %s", x.stdout)
	}
	// A FIFO or a symbolic link in place of the observation is refused
	// without blocking on it or following it.
	path := filepath.Join(prog, dispatch.AdmissionFile)
	good := filepath.Join(prog, "good.json")
	writeAdmission(t, prog, admissionFixture(now, dispatch.AdmissionPressureCalm, 3, 0))
	if err := os.Rename(path, good); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(good, path); err != nil {
		t.Fatal(err)
	}
	if x, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "RUNNABLE_HELD_IDLE")) != 0 || !strings.Contains(strings.Join(x.res.Warnings, "\n"), "queue observation unreadable") {
		t.Fatalf("symlink: %s", x.stdout)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if x, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "RUNNABLE_HELD_IDLE")) != 0 || !strings.Contains(strings.Join(x.res.Warnings, "\n"), "not a regular file") {
		t.Fatalf("fifo: %s", x.stdout)
	}
}

// TQD-V0-044: a role whose queue stays deeper than its cap at CALM pressure
// for the threshold's ticks is ROLE_QUEUE_DEEP with depth, cap, oldest wait
// and headroom; UNKNOWN, RAISED or unconfigured pressure abstains.
func TestTQDV0044_RoleQueueDeep(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	prog := filepath.Join(t.TempDir(), "prog")
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`"}]}`)
	now := time.Now().UTC().Truncate(time.Second)
	writeAdmission(t, prog, admissionFixture(now, dispatch.AdmissionPressureCalm, 1, 3))
	_, item := doctorOK(t, root, "--config", cfg)
	got := findingsOf(item, "ROLE_QUEUE_DEEP")
	if len(got) != 1 || field(got[0], "who").Str != "prog/impl" || field(got[0], "firstSeen").Str != now.Add(-time.Minute).Format(time.RFC3339) {
		t.Fatalf("finding: %s", wire.Encode(item))
	}
	for _, want := range []string{"queue depth 5 (1 running, 4 waiting)", "effective cap 1 (cap 1, capMax 3)", "3 tick(s)", "oldest wait ticket:a:q:old since " + now.Add(-2*time.Minute).Format(time.RFC3339), "headroom: the global cap allows 2 more session(s) (2 of 4 running)"} {
		if !strings.Contains(field(got[0], "detail").Str, want) {
			t.Fatalf("detail lacks %q: %s", want, field(got[0], "detail").Str)
		}
	}
	full := admissionFixture(now, dispatch.AdmissionPressureCalm, 1, 3)
	full.Running = 4
	writeAdmission(t, prog, full)
	if _, item := doctorOK(t, root, "--config", cfg); !strings.Contains(field(findingsOf(item, "ROLE_QUEUE_DEEP")[0], "detail").Str, "global cap is full (4 of 4 running)") {
		t.Fatalf("full headroom: %s", wire.Encode(item))
	}
	writeAdmission(t, prog, admissionFixture(now, dispatch.AdmissionPressureCalm, 1, 2))
	if _, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "ROLE_QUEUE_DEEP")) != 0 {
		t.Fatalf("below threshold: %s", wire.Encode(item))
	}
	low := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","thresholds":{"roleQueueDeepTicks":"2"},"dispatchers":[{"programDir":"`+prog+`"}]}`)
	if _, item := doctorOK(t, root, "--config", low); len(findingsOf(item, "ROLE_QUEUE_DEEP")) != 1 {
		t.Fatalf("configured threshold: %s", wire.Encode(item))
	}
	for _, p := range []string{dispatch.AdmissionPressureUnknown, dispatch.AdmissionPressureNotConfigured, dispatch.AdmissionPressureRaised} {
		writeAdmission(t, prog, admissionFixture(now, p, 1, 3))
		if _, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "ROLE_QUEUE_DEEP")) != 0 {
			t.Fatalf("%s pressure must abstain: %s", p, wire.Encode(item))
		}
	}
}

// DSG-V0-005: each unmatched work state of the observation is one
// WORK_STATE_NO_ROLE_MATCH finding, and an idle streak of an hour or more
// beside free capacity is IDLE_WITH_CAPACITY with the deferral reasons; a
// shorter streak reports nothing.
func TestDSGV0005_DoctorNoRoleMatchAndIdle(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	prog := filepath.Join(t.TempDir(), "prog")
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`"}]}`)
	now := time.Now().UTC().Truncate(time.Second)
	rec := admissionFixture(now, dispatch.AdmissionPressureCalm, 1, 0)
	idleSince := now.Add(-61 * time.Minute)
	rec.UnmatchedStates = []dispatch.UnmatchedState{{State: "X~2", Tickets: 2, Example: "ticket:a:q:x"}}
	rec.IdleTicks, rec.IdleSince, rec.IdleAlertSeconds = 122, &idleSince, 3600
	rec.IdleDeferred = []dispatch.DeferralCount{{Reason: "NO_ROLE_MATCH", Tickets: 2}, {Reason: "WORK_STATE_HELD", Tickets: 1}}
	writeAdmission(t, prog, rec)
	_, item := doctorOK(t, root, "--config", cfg)
	nm := findingsOf(item, "WORK_STATE_NO_ROLE_MATCH")
	if len(nm) != 1 || field(nm[0], "who").Str != "prog/X~2" || !strings.Contains(field(nm[0], "detail").Str, `"X~2" of 2 ticket(s)`) || !strings.Contains(field(nm[0], "remedy").Str, "holdStates") {
		t.Fatalf("no role match: %s", wire.Encode(item))
	}
	idle := findingsOf(item, "IDLE_WITH_CAPACITY")
	if len(idle) != 1 || field(idle[0], "who").Str != "prog" || field(idle[0], "firstSeen").Str != idleSince.Format(time.RFC3339) || !strings.Contains(field(idle[0], "detail").Str, "NO_ROLE_MATCH=2, WORK_STATE_HELD=1") || !strings.Contains(field(idle[0], "detail").Str, "2 of 4 sessions") {
		t.Fatalf("idle: %s", wire.Encode(item))
	}
	short := now.Add(-59 * time.Minute)
	rec.UnmatchedStates, rec.IdleSince = nil, &short
	writeAdmission(t, prog, rec)
	if _, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "IDLE_WITH_CAPACITY"))+len(findingsOf(item, "WORK_STATE_NO_ROLE_MATCH")) != 0 {
		t.Fatalf("short idle: %s", wire.Encode(item))
	}
	// The record's own threshold governs: 30 minutes reports the same
	// streak, and an alert turned off reports none.
	rec.IdleAlertSeconds = 1800
	writeAdmission(t, prog, rec)
	if _, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "IDLE_WITH_CAPACITY")) != 1 {
		t.Fatalf("configured threshold: %s", wire.Encode(item))
	}
	rec.IdleAlertSeconds = 0
	writeAdmission(t, prog, rec)
	if _, item := doctorOK(t, root, "--config", cfg); len(findingsOf(item, "IDLE_WITH_CAPACITY")) != 0 {
		t.Fatalf("alert off: %s", wire.Encode(item))
	}
}
