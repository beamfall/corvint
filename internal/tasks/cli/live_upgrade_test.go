package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/service"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-130, CAL-V0-131 and CAL-V0-133: an attempt claimed by build N keeps
// heartbeating, renewing and releasing after build N+1 is installed in place
// by atomic rename, while a build N process that outlived the swap still
// reads and writes the shared store. The stamped build number is the seam.
func TestCALV0130_AttemptClaimedUnderBuildNContinuesUnderNPlus1(t *testing.T) {
	dir := t.TempDir()
	build := func(n string) string {
		t.Helper()
		out := filepath.Join(dir, "build-"+n, "corvint-tasks")
		c := exec.Command("go", "build", "-ldflags", "-X main.build="+n, "-o", out, "../../../cmd/corvint-tasks")
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v %s", n, err, b)
		}
		return out
	}
	buildN, buildNext := build("41"), build("42")
	installed := filepath.Join(dir, "bin", "corvint-tasks")
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		t.Fatal(err)
	}
	// CAL-V0-133: install writes a new file and renames it over the stable
	// path, so a running process keeps its own executable image.
	install := func(from string) {
		t.Helper()
		raw, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(installed+".new", raw, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(installed+".new", installed); err != nil {
			t.Fatal(err)
		}
	}
	r := fixture.TempRepo(t)
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	ok := func(binary string, args ...string) wire.Value {
		t.Helper()
		x := handoffCLIWithBinary(t, r.Root, binary, args...)
		if x.code != 0 || x.res.Outcome != wire.OutcomeOK || len(x.res.Items) == 0 {
			t.Fatalf("%v: %d %s", args, x.code, x.stdout)
		}
		return x.res.Items[0]
	}
	formats := func(v wire.Value) []string {
		var got []string
		for _, f := range field(v, "formats").Arr {
			got = append(got, f.Str)
		}
		return got
	}

	install(buildN)
	before := ok(installed, "version")
	if !strings.HasSuffix(field(before, "version").Str, "+build.41") || !slices.Equal(formats(before), cli.LiveFormats()) {
		t.Fatalf("build N version: %s", wire.Encode(before))
	}
	ok(installed, "init")
	id := field(ok(installed, "ticket", "create", "--request-id", "upgrade-create", "--payload", createPayloadJSON), "ticketId").Str
	claim := ok(installed, "claim", id, "--holder", "agent", "--request-id", "upgrade-claim")
	attempt, generation := field(claim, "attemptId").Str, field(claim, "generation").Str
	ok(installed, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "beat-n")

	install(buildNext)
	after := ok(installed, "version")
	if !strings.HasSuffix(field(after, "version").Str, "+build.42") || !slices.Equal(formats(after), formats(before)) {
		t.Fatalf("build N+1 version: %s", wire.Encode(after))
	}
	ok(installed, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "beat-next")
	renewed := ok(installed, "renew", "--attempt", attempt, "--generation", generation, "--request-id", "renew-next")
	if field(renewed, "generation").Str != generation {
		t.Fatalf("renew moved the generation: %s", wire.Encode(renewed))
	}
	// A build N process that outlived the swap reads build N+1's records.
	ok(buildN, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "beat-old")
	ok(installed, "release", "--attempt", attempt, "--generation", generation, "--request-id", "release-next")
	ok(installed, "receipt", "audit")
	ok(buildN, "receipt", "audit")
	if x := handoffCLIWithBinary(t, r.Root, installed, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "beat-after-release"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("released attempt still heartbeats: %s", x.stdout)
	}
}

// CAL-V0-131: a store whose format is not one this build knows is refused
// with UNSUPPORTED_VERSION before any lease verb writes, and is never read
// or migrated.
func TestCALV0131_OtherStoreFormatRefusesUnsupportedVersion(t *testing.T) {
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-12*time.Minute).Truncate(time.Second))
	a := claimed[0]
	stateDir := filepath.Join(root, ".git", "taskman")
	if err := os.Chmod(filepath.Join(stateDir, "VERSION"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(stateDir, "VERSION"), []byte(strings.Replace(snapshot.VersionBytes, "/0", "/1", 1)))
	journal := fixture.TreeSnapshot(t, stateDir)
	for _, args := range [][]string{
		{"attempt", "heartbeat", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "beat-other"},
		{"renew", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "renew-other"},
		{"release", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "release-other"},
	} {
		x := atm(t, root, nil, args...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedVersion) {
			t.Fatalf("%v: %+v", args, x.res)
		}
	}
	if after := fixture.TreeSnapshot(t, stateDir); !reflect.DeepEqual(after, journal) {
		t.Fatal("refused verbs wrote state")
	}
}

// CAL-V0-131 and CAL-V0-134: the reported set is sorted and unique, and it
// covers the store version and every record a live attempt, an adopted
// worker, a detached attempt runner or a supervised program owner that
// outlived a binary swap reads or writes.
func TestCALV0131_LiveFormatsCoverEveryLiveRecord(t *testing.T) {
	got := cli.LiveFormats()
	if !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Fatalf("formats not sorted and unique: %v", got)
	}
	for _, want := range []string{
		strings.TrimSpace(snapshot.VersionBytes), snapshot.ProfileHead, snapshot.ProfileReceipt, snapshot.ProfileAttempt,
		snapshot.ProfileReservations, snapshot.SupervisedProfile, mutation.Profile, mutation.OutcomeProfile,
		dispatch.StateProfile, "taskman-attempt-run-record/0", transaction.RunOutcomeProfile, service.ProfileName,
	} {
		if !slices.Contains(got, want) {
			t.Errorf("formats omit %s: %v", want, got)
		}
	}
}
