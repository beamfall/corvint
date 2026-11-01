//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fakeHost replaces the host identity for one test.
func fakeHost(t *testing.T, machine, boot string) {
	t.Helper()
	prev := hostIdentity
	hostIdentity = func() (string, string) { return machine, boot }
	t.Cleanup(func() { hostIdentity = prev })
}

func quarantinedProgram(t *testing.T, program string) (*Config, string, *readerEvidence) {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.Roles[0].Match.Labels = []string{"do-not-launch"}
	dir := ProgramDir(c, program)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	mark, err := publishReaderMarker(dir, program)
	if err != nil {
		t.Fatal(err)
	}
	return c, dir, mark
}

func openRefused(t *testing.T, c *Config, program string) {
	t.Helper()
	if d, err := Open(program, c, &fakeQueue{}, io.Discard); !errors.Is(err, ErrReaderQuiescence) {
		if d != nil {
			_ = d.Close()
		}
		t.Fatal("quarantined program opened", err)
	}
}

func openAdmitted(t *testing.T, c *Config, program string) []Event {
	t.Helper()
	d, err := Open(program, c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal("recovered program refused", err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	events, err := ReadEvents(ProgramDir(c, program), 50)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

// CAL-V0-227: a reboot of the same machine is the only automatic proof. The
// next start records it before removing the marker and reports it.
func TestCALV0227_RebootProofLiftsQuarantine(t *testing.T) {
	machine, boot1, boot2 := digestHex("m"), digestHex("b1"), digestHex("b2")
	fakeHost(t, machine, boot1)
	c, dir, mark := quarantinedProgram(t, "reboot")
	if mark.marker.Machine != machine || mark.marker.Boot != boot1 || mark.marker.Profile != readerMarkerProfile {
		t.Fatalf("marker lacks host evidence: %s", mark.raw)
	}
	openRefused(t, c, "reboot")
	if _, _, why := ReaderContainment(dir, "reboot"); !strings.Contains(why, "has not rebooted") || !strings.Contains(why, "dispatch recover-reader --program reboot") || !strings.Contains(why, "no reader pid or start time is recorded") {
		t.Fatal("same-boot diagnostic", why)
	}
	fakeHost(t, machine, boot2)
	if _, held, why := ReaderContainment(dir, "reboot"); !held || !strings.Contains(why, "has rebooted") {
		t.Fatal("status must report, never apply, the proof", held, why)
	}
	if _, err := os.Lstat(mark.path); err != nil {
		t.Fatal("status removed the marker", err)
	}
	events := openAdmitted(t, c, "reboot")
	if _, err := os.Lstat(mark.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("marker survived the reboot proof", err)
	}
	rec, err := ReadReaderRecovery(dir, "reboot-"+mark.marker.Run+".json")
	if err != nil || rec.Kind != RecoveryHostReboot || rec.MarkerBoot != boot1 || rec.CurrentBoot != boot2 || rec.Run != mark.marker.Run || rec.RequestID != "" {
		t.Fatalf("reboot record %+v %v", rec, err)
	}
	found := false
	for _, e := range events {
		if e.Kind == "reader-recovered" && e.Detail["kind"] == RecoveryHostReboot && e.Detail["run"] == mark.marker.Run {
			found = true
		}
	}
	if !found {
		t.Fatalf("no reader-recovered event: %+v", events)
	}
}

// CAL-V0-227 negatives: absent or foreign evidence is never a proof.
func TestCALV0227_NoProofWithoutEvidence(t *testing.T) {
	machine, boot1, boot2 := digestHex("m"), digestHex("b1"), digestHex("b2")
	cases := map[string]struct {
		markMachine, markBoot, nowMachine, nowBoot string
		legacy                                     bool
	}{
		"same boot":           {machine, boot1, machine, boot1, false},
		"other machine":       {machine, boot1, digestHex("other"), boot2, false},
		"marker without boot": {"", "", machine, boot2, false},
		"host without boot":   {machine, boot1, machine, "", false},
		"legacy marker":       {"", "", machine, boot2, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fakeHost(t, tc.markMachine, tc.markBoot)
			c, dir, mark := quarantinedProgram(t, "noproof")
			if tc.legacy {
				raw := []byte(`{"profile":"taskman-dispatch-reader-lifecycle/0","program":"noproof","run":"` + mark.marker.Run + `","lifecycle":"UNKNOWN"}`)
				if err := os.WriteFile(mark.path, raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			fakeHost(t, tc.nowMachine, tc.nowBoot)
			openRefused(t, c, "noproof")
			if _, err := os.Lstat(mark.path); err != nil {
				t.Fatal("marker removed without proof", err)
			}
			if _, err := os.Lstat(filepath.Join(dir, readerRecoveryDir)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("recorded a recovery without proof", err)
			}
			_, _, why := ReaderContainment(dir, "noproof")
			if !strings.Contains(why, "recover-reader") {
				t.Fatal("diagnostic omits the recovery", why)
			}
			if (tc.markBoot == "" || tc.nowBoot == "") && tc.nowBoot != "" && !strings.Contains(why, "no host boot identity") {
				t.Fatal("diagnostic hides the missing boot evidence", why)
			}
		})
	}
}

// CAL-V0-227: a reboot record that attests a different marker is not reused.
func TestCALV0227_RebootRecordMustMatchMarker(t *testing.T) {
	machine := digestHex("m")
	fakeHost(t, machine, digestHex("b1"))
	c, dir, mark := quarantinedProgram(t, "mismatch")
	stale := ReaderRecovery{Profile: ReaderRecoveryProfile, Program: "mismatch", Kind: RecoveryHostReboot, Run: mark.marker.Run, MarkerSHA256: digestHex("other"), MarkerModTime: "2026-01-01T00:00:00Z", RecordedAt: "2026-01-01T00:00:00Z"}
	if err := writeRecovery(dir, "reboot-"+mark.marker.Run+".json", stale); err != nil {
		t.Fatal(err)
	}
	fakeHost(t, machine, digestHex("b2"))
	openRefused(t, c, "mismatch")
	if _, err := os.Lstat(mark.path); err != nil {
		t.Fatal("marker removed under a mismatched record", err)
	}
}

func recoverReq(id string) ReaderRecoveryRequest {
	return ReaderRecoveryRequest{RequestID: id, Reason: "operator checked ps", Evidence: "ops-note-1"}
}

// CAL-V0-226: the operator attestation is recorded before the exact marker
// is removed, replays, and conflicts on a reused request ID.
func TestCALV0226_OperatorRecovery(t *testing.T) {
	fakeHost(t, digestHex("m"), digestHex("b1"))
	c, dir, mark := quarantinedProgram(t, "operator")
	openRefused(t, c, "operator")
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	rec, replayed, err := RecoverReader(c, "operator", recoverReq("req-1"), now)
	if err != nil || replayed || rec.Kind != RecoveryOperator || rec.Run != mark.marker.Run || rec.RequestID != "req-1" || rec.RecordedAt != "2026-10-14T12:00:00Z" {
		t.Fatalf("recover %+v %v %v", rec, replayed, err)
	}
	if _, err := os.Lstat(mark.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("marker kept", err)
	}
	stored, err := ReadReaderRecovery(dir, "operator-"+digestHex("req-1")[:32]+".json")
	if err != nil || *stored != *rec {
		t.Fatalf("stored record %+v %v", stored, err)
	}
	again, replayed, err := RecoverReader(c, "operator", recoverReq("req-1"), now.Add(time.Hour))
	if err != nil || !replayed || *again != *rec {
		t.Fatal("replay", again, replayed, err)
	}
	if _, _, err := RecoverReader(c, "operator", ReaderRecoveryRequest{RequestID: "req-1", Reason: "different", Evidence: "ops-note-1"}, now); wire.CodeOf(err) != wire.CodeRequestIDConflict {
		t.Fatal("changed replay accepted", err)
	}
	openAdmitted(t, c, "operator")
	// A later quarantine cannot reuse the request ID.
	mark2, err := publishReaderMarker(dir, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := RecoverReader(c, "operator", recoverReq("req-1"), now); wire.CodeOf(err) != wire.CodeRequestIDConflict {
		t.Fatal("request ID reused for a new marker", err)
	}
	if _, err := os.Lstat(mark2.path); err != nil {
		t.Fatal("conflict removed the marker", err)
	}
	if _, _, err := RecoverReader(c, "operator", recoverReq("req-2"), now); err != nil {
		t.Fatal(err)
	}
	openAdmitted(t, c, "operator")
	// Nothing to recover is refused.
	if _, _, err := RecoverReader(c, "operator", recoverReq("req-3"), now); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatal("recovered an absent quarantine", err)
	}
}

// CAL-V0-226: live evidence keeps the quarantine whatever the attestation.
func TestCALV0226_LiveEvidenceKeepsQuarantine(t *testing.T) {
	fakeHost(t, digestHex("m"), digestHex("b1"))
	c, dir, mark := quarantinedProgram(t, "live")
	if err := publishReaderLeader(dir, "live", mark.marker.Run, 4242, "start-1"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mustRead(t, mark.path), mark.raw) {
		t.Fatal("the leader record rewrote the marker")
	}
	if _, _, why := ReaderContainment(dir, "live"); !strings.Contains(why, "names reader pid 4242") {
		t.Fatal("diagnostic omits the recorded pid", why)
	}
	prev := readerProcessIdentity
	t.Cleanup(func() { readerProcessIdentity = prev })
	running := func(int) (string, error) { return "start-1", nil }
	cases := []struct {
		name     string
		identity func(int) (string, error)
		marker   []byte
	}{
		{"still running", running, mark.raw},
		{"identity unreadable", func(int) (string, error) { return "", errors.New("permission denied") }, mark.raw},
		// A marker that no longer decodes strictly does not skip the
		// live check.
		{"malformed marker", running, append(append([]byte(nil), mark.raw...), '\n')},
	}
	for _, tc := range cases {
		readerProcessIdentity = tc.identity
		if err := os.WriteFile(mark.path, tc.marker, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := RecoverReader(c, "live", recoverReq("req-"+strings.ReplaceAll(tc.name, " ", "-")), time.Now()); wire.CodeOf(err) != wire.CodeQuiescenceUnproved {
			t.Fatal(tc.name, "lifted the quarantine", err)
		}
		if _, err := os.Lstat(mark.path); err != nil {
			t.Fatal(tc.name, "removed the marker", err)
		}
		if _, err := os.Lstat(filepath.Join(dir, readerRecoveryDir)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal(tc.name, "recorded an attestation", err)
		}
	}
	if err := os.WriteFile(mark.path, mark.raw, 0o600); err != nil {
		t.Fatal(err)
	}
	// An unreadable leader record keeps the quarantine.
	leaderPath := filepath.Join(dir, readerLeaderName)
	good := mustRead(t, leaderPath)
	if err := os.WriteFile(leaderPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	readerProcessIdentity = func(int) (string, error) { return "start-2", nil }
	if _, _, err := RecoverReader(c, "live", recoverReq("req-bad-leader"), time.Now()); wire.CodeOf(err) != wire.CodeQuiescenceUnproved {
		t.Fatal("unreadable leader record lifted the quarantine", err)
	}
	if err := os.WriteFile(leaderPath, good, 0o600); err != nil {
		t.Fatal(err)
	}
	// A running dispatcher holds the program lock.
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	readerProcessIdentity = func(int) (string, error) { return "start-2", nil }
	if _, _, err := RecoverReader(c, "live", recoverReq("req-locked"), time.Now()); wire.CodeOf(err) != wire.CodeResourceCollision {
		t.Fatal("recovered under a held lock", err)
	}
	_ = lock.Close()
	// The recorded pid now names another process: the attestation stands.
	if _, _, err := RecoverReader(c, "live", recoverReq("req-gone"), time.Now()); err != nil {
		t.Fatal(err)
	}
	openAdmitted(t, c, "live")
	if _, err := os.Lstat(leaderPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("leader record kept after recovery", err)
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

// CAL-V0-229: recovery never attests or removes a newer marker or leader
// record version.
func TestCALV0229_RecoverRefusesNewerVersions(t *testing.T) {
	c, dir, mark := quarantinedProgram(t, "newer")
	newer := []byte(`{"profile":"taskman-dispatch-reader-lifecycle/2","program":"newer"}`)
	if err := os.WriteFile(mark.path, newer, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RecoverReader(c, "newer", recoverReq("req-newer"), time.Now()); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
		t.Fatal("newer marker attested", err)
	}
	if !bytes.Equal(mustRead(t, mark.path), newer) {
		t.Fatal("newer marker changed")
	}
	if err := os.WriteFile(mark.path, mark.raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, readerLeaderName), []byte(`{"profile":"taskman-dispatch-reader-leader/1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RecoverReader(c, "newer", recoverReq("req-newer-leader"), time.Now()); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
		t.Fatal("newer leader record ignored", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, readerRecoveryDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refusal recorded an attestation", err)
	}
}

// CAL-V0-226: removal takes the marker aside and checks it again. A marker
// replaced at that boundary is put back, and a marker left aside by an
// interrupted removal keeps the quarantine until it is attested.
func TestCALV0226_ClearTakesMarkerAside(t *testing.T) {
	c, dir, mark := quarantinedProgram(t, "aside")
	t.Cleanup(func() { readerTakeAsideHook = func(string) {} })
	readerTakeAsideHook = func(path string) {
		if err := os.WriteFile(path+".new", []byte("replacement"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".new", path); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := RecoverReader(c, "aside", recoverReq("req-raced"), time.Now()); err == nil {
		t.Fatal("removed a marker replaced at the removal boundary")
	}
	readerTakeAsideHook = func(string) {}
	if got := mustRead(t, mark.path); string(got) != "replacement" {
		t.Fatalf("replacement not restored: %q", got)
	}
	tomb := mark.path + readerClearingSuffix
	if _, err := os.Lstat(tomb); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("restored marker left aside", err)
	}
	// An interrupted removal: only the taken-aside marker remains.
	if err := os.Rename(mark.path, tomb); err != nil {
		t.Fatal(err)
	}
	if state, held, why := ReaderContainment(dir, "aside"); state != "UNKNOWN" || !held || !strings.Contains(why, "interrupted") {
		t.Fatal("taken-aside marker lifted the quarantine", state, held, why)
	}
	openRefused(t, c, "aside")
	rec, _, err := RecoverReader(c, "aside", recoverReq("req-tomb"), time.Now())
	if err != nil || rec.MarkerSHA256 != digestHex("replacement") {
		t.Fatalf("taken-aside recovery %+v %v", rec, err)
	}
	openAdmitted(t, c, "aside")
}

// CAL-V0-226: a malformed marker is attested by its exact bytes; a marker
// that is not a regular file is never removed.
func TestCALV0226_UnreadableMarkers(t *testing.T) {
	c, dir, mark := quarantinedProgram(t, "bad")
	if err := os.WriteFile(mark.path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	openRefused(t, c, "bad")
	rec, _, err := RecoverReader(c, "bad", recoverReq("req-bad"), time.Now())
	if err != nil || rec.Run != "" || rec.MarkerSHA256 != digestHex("{") {
		t.Fatalf("malformed marker recovery %+v %v", rec, err)
	}
	openAdmitted(t, c, "bad")
	if err := os.Mkdir(mark.path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RecoverReader(c, "bad", recoverReq("req-dir"), time.Now()); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatal("directory marker attested", err)
	}
	if fi, err := os.Lstat(mark.path); err != nil || !fi.IsDir() {
		t.Fatal("directory marker removed", err)
	}
	_ = dir
}

// CAL-V0-227 (V1-3635): the standalone work-state reader lifts its own
// quarantine by the same reboot proof, so it also reports it with a
// reader-recovered event; a lifted quarantine never goes unreported.
func TestCALV0227_StandaloneReaderRebootEmitsEvent(t *testing.T) {
	if !platformSupported {
		t.Skip("reader ownership is unsupported here")
	}
	const program = "work-state-reader"
	machine, boot1, boot2 := digestHex("m"), digestHex("b1"), digestHex("b2")
	fakeHost(t, machine, boot1)
	c, dir, mark := quarantinedProgram(t, program)
	fakeHost(t, machine, boot2)
	if _, err := standaloneStateCommand(context.Background(), c, []string{"/bin/sh", "-c", "exit 0"}); errors.Is(err, ErrReaderQuiescence) {
		t.Fatal("reboot proof refused", err)
	}
	if _, err := os.Lstat(mark.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("marker survived the reboot proof", err)
	}
	events, err := ReadEvents(dir, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Kind == "reader-recovered" && e.Program == program && e.Detail["kind"] == RecoveryHostReboot && e.Detail["run"] == mark.marker.Run && e.Seq > 0 {
			return
		}
	}
	t.Fatalf("no reader-recovered event: %+v", events)
}

// CAL-V0-227 (V1-3635 review): when the standalone reader cannot append its
// reader-recovered event, the quarantine marker stays and the read refuses;
// the next read finds the same HOST_REBOOT record, appends the event once and
// only then lifts the quarantine.
func TestCALV0227_StandaloneReaderKeepsQuarantineUntilEventLands(t *testing.T) {
	if !platformSupported {
		t.Skip("reader ownership is unsupported here")
	}
	const program = "work-state-reader"
	machine, boot1, boot2 := digestHex("m"), digestHex("b1"), digestHex("b2")
	fakeHost(t, machine, boot1)
	c, dir, mark := quarantinedProgram(t, program)
	fakeHost(t, machine, boot2)
	t.Cleanup(func() { appendEvent = appendEventLog })
	appendEvent = func(string, Event) error { return errors.New("event log unwritable") }
	argv := []string{"/bin/sh", "-c", "exit 0"}
	if _, err := standaloneStateCommand(context.Background(), c, argv); !errors.Is(err, ErrReaderQuiescence) {
		t.Fatalf("read with an unwritable event log: %v", err)
	}
	if _, err := os.Lstat(mark.path); err != nil {
		t.Fatal("the marker went although its event was not written", err)
	}
	if _, err := ReadReaderRecovery(dir, "reboot-"+mark.marker.Run+".json"); err != nil {
		t.Fatal("the reboot record was not kept for the retry", err)
	}
	appendEvent = appendEventLog
	for range 2 {
		if _, err := standaloneStateCommand(context.Background(), c, argv); errors.Is(err, ErrReaderQuiescence) {
			t.Fatal("retry refused", err)
		}
	}
	if _, err := os.Lstat(mark.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("marker survived the retried proof", err)
	}
	events, err := ReadEvents(dir, 50)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range events {
		if e.Kind == "reader-recovered" && e.Detail["run"] == mark.marker.Run {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("reader-recovered events = %d, want 1: %+v", count, events)
	}
}
