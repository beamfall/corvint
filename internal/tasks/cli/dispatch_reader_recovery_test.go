//go:build darwin || linux

package cli_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const recoveryRun = "0123456789abcdef0123456789abcdef"

func recoveryMarker(machine, boot string) []byte {
	s := `{"profile":"taskman-dispatch-reader-lifecycle/1","program":"prog","run":"` + recoveryRun + `","lifecycle":"UNKNOWN"`
	if machine != "" {
		s += `,"machine":"` + machine + `"`
	}
	if boot != "" {
		s += `,"boot":"` + boot + `"`
	}
	return []byte(s + "}")
}

func recoveryLeader(pid int, start string) []byte {
	return []byte(`{"profile":"taskman-dispatch-reader-leader/0","program":"prog","run":"` + recoveryRun + `","readerPid":` + strconv.Itoa(pid) + `,"readerStart":"` + start + `"}`)
}

// quarantinedCLI writes marker raw and proves the refused start names the
// recovery command.
func quarantinedCLI(t *testing.T, raw []byte) (root, config, dir string) {
	t.Helper()
	root, _ = leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	planTicket(t, root, "reader", "P1", `["src/"]`)
	config, dir = readerCLIConfig(t, root, `printf '{}'`)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(dir, "reader-lifecycle.json"), raw)
	r := atm(t, root, nil, "dispatch", "--once", "--program", "prog", "--config", config)
	if r.res.Outcome != wire.OutcomeError || !containsReaderCode(r.res.Codes, wire.CodeQuiescenceUnproved) || !strings.Contains(string(r.stdout)+string(r.stderr), "dispatch recover-reader --program prog") {
		t.Fatalf("quarantined start: %s %s", r.stdout, r.stderr)
	}
	return root, config, dir
}

func startsCleanly(t *testing.T, root, config, dir string) {
	t.Helper()
	r := atm(t, root, nil, "dispatch", "--once", "--program", "prog", "--config", config)
	if r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("recovered start: %s %s", r.stdout, r.stderr)
	}
	if _, err := os.Lstat(filepath.Join(dir, "reader-lifecycle.json")); !os.IsNotExist(err) {
		t.Fatal("reader marker left after a clean start", err)
	}
}

func recoverArgs(config, id string) []string {
	return []string{"dispatch", "recover-reader", "--program", "prog", "--config", config, "--reason", "ps shows no reader or descendant", "--evidence", "ops-ticket-17", "--request-id", id}
}

// CAL-V0-226 and CAL-V0-228: an UNKNOWN marker refuses the start and names
// the operator recovery, which records the attestation, replays, refuses a
// reused request ID on a later quarantine, and lets the next start run.
func TestCALV0226_DispatchRecoverReaderOperator(t *testing.T) {
	root, config, dir := quarantinedCLI(t, recoveryMarker("", ""))
	r := atm(t, root, nil, recoverArgs(config, "recover-1")...)
	item := r.res.Items
	if r.res.Outcome != wire.OutcomeOK || len(item) != 1 || field(item[0], "kind").Str != dispatch.RecoveryOperator || field(item[0], "run").Str != recoveryRun || field(item[0], "replayed").Bool || field(item[0], "readerQuarantined").Bool {
		t.Fatalf("recover-reader: %s %s", r.stdout, r.stderr)
	}
	sum := sha256.Sum256([]byte("recover-1"))
	rec, err := dispatch.ReadReaderRecovery(dir, "operator-"+hex.EncodeToString(sum[:])[:32]+".json")
	if err != nil || rec.Reason != "ps shows no reader or descendant" || rec.Evidence != "ops-ticket-17" {
		t.Fatalf("attestation %+v %v", rec, err)
	}
	r = atm(t, root, nil, recoverArgs(config, "recover-1")...)
	if r.res.Outcome != wire.OutcomeOK || !field(r.res.Items[0], "replayed").Bool {
		t.Fatalf("replay: %s", r.stdout)
	}
	startsCleanly(t, root, config, dir)
	fixture.Write(t, filepath.Join(dir, "reader-lifecycle.json"), recoveryMarker("", ""))
	if r := atm(t, root, nil, recoverArgs(config, "recover-1")...); !hasCode(r.res, wire.CodeRequestIDConflict) {
		t.Fatalf("reused request ID: %s", r.stdout)
	}
	for _, bad := range [][]string{
		{"dispatch", "recover-reader", "--program", "prog", "--config", config, "--evidence", "e", "--request-id", "r"},
		{"dispatch", "recover-reader", "--program", "prog", "--config", config, "--reason", "x", "--request-id", "r"},
		{"dispatch", "recover-reader", "--program", "prog", "--config", config, "--reason", "x", "--evidence", "e"},
	} {
		if r := atm(t, root, nil, bad...); r.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%v accepted: %s", bad, r.stdout)
		}
	}
	if _, err := os.Lstat(filepath.Join(dir, "reader-lifecycle.json")); err != nil {
		t.Fatal("refused recovery removed the marker", err)
	}
}

// CAL-V0-227: a marker that names this machine and an earlier boot is lifted
// by the next start, which records the proof; no operator step is needed.
func TestCALV0227_DispatchRecoverReaderByReboot(t *testing.T) {
	machine, boot := dispatch.ReaderHostIdentity()
	if machine == "" || boot == "" {
		t.Skip("NOT_OBSERVED: this host reports no machine or boot identity")
	}
	earlier := sha256.Sum256([]byte("an earlier boot"))
	root, config, dir := quarantinedCLI(t, recoveryMarker(machine, boot))
	if _, err := os.Lstat(filepath.Join(dir, "reader-recoveries")); !os.IsNotExist(err) {
		t.Fatal("same-boot refusal recorded a recovery", err)
	}
	fixture.Write(t, filepath.Join(dir, "reader-lifecycle.json"), recoveryMarker(machine, hex.EncodeToString(earlier[:])))
	r := atm(t, root, nil, "dispatch", "status", "--program", "prog", "--config", config)
	if r.res.Outcome != wire.OutcomeOK || !strings.Contains(field(r.res.Items[0], "readerDiagnostic").Str, "has rebooted") {
		t.Fatalf("status: %s", r.stdout)
	}
	startsCleanly(t, root, config, dir)
	rec, err := dispatch.ReadReaderRecovery(dir, "reboot-"+recoveryRun+".json")
	if err != nil || rec.Kind != dispatch.RecoveryHostReboot || rec.CurrentBoot != boot {
		t.Fatalf("reboot record %+v %v", rec, err)
	}
	events, err := dispatch.ReadEvents(dir, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		found = found || (e.Kind == "reader-recovered" && e.Detail["kind"] == dispatch.RecoveryHostReboot)
	}
	if !found {
		t.Fatal("no reader-recovered event")
	}
}

// CAL-V0-226 negative: live evidence keeps the refusal. A marker naming a
// running leader, or a dispatcher holding the program lock, is not lifted.
func TestCALV0226_DispatchRecoverReaderLiveEvidence(t *testing.T) {
	self := os.Getpid()
	start, err := supervisor.ProcessIdentity(self)
	if err != nil || start == "" {
		t.Skip("NOT_OBSERVED: own process identity unreadable", err)
	}
	root, config, dir := quarantinedCLI(t, recoveryMarker("", ""))
	fixture.Write(t, filepath.Join(dir, "reader-leader.json"), recoveryLeader(self, start))
	r := atm(t, root, nil, recoverArgs(config, "recover-live")...)
	if !hasCode(r.res, wire.CodeQuiescenceUnproved) || !strings.Contains(string(r.stdout), "still running") {
		t.Fatalf("live leader recovered: %s", r.stdout)
	}
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "reader-leader.json")); err != nil {
		t.Fatal(err)
	}
	if r := atm(t, root, nil, recoverArgs(config, "recover-locked")...); !hasCode(r.res, wire.CodeResourceCollision) {
		t.Fatalf("recovered under a running dispatcher: %s", r.stdout)
	}
	if _, err := os.Lstat(filepath.Join(dir, "reader-lifecycle.json")); err != nil {
		t.Fatal("refusal removed the marker", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "reader-recoveries")); !os.IsNotExist(err) {
		t.Fatal("refusal recorded an attestation", err)
	}
}

// CAL-V0-228: the verbose help names the recovery and its proof rules.
func TestCALV0228_DispatchHelpNamesRecovery(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"dispatch", "--help", "--verbose"}, {"dispatch", "recover-reader", "--help", "--verbose"}} {
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK || !strings.Contains(string(r.stdout), "recover-reader") || !strings.Contains(string(r.stdout), "reboot") {
			t.Fatalf("%v: %s", args, r.stdout)
		}
	}
}
