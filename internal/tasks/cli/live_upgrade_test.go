package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// survivorFixture is the build N runner's command: it records its pid in $1,
// then waits until the finish file $2 exists. It has a lifetime of its own,
// independent of the runner and of the finish file: it also ends once $4
// seconds have passed or its directory $3 is gone, so a runner killed before
// it retired the command cannot leave the loop running. Its date and sleep
// commands close fd 3, so a pipe on fd 3 reaches EOF when the loop exits.
const survivorFixture = `end=$(( $(date +%s 3>&-) + $4 )); echo $$ > "$1"
while [ ! -f "$2" ] && [ -d "$3" ] && [ "$(date +%s 3>&-)" -lt "$end" ]; do sleep 0.05 3>&-; done`

// CAL-V0-130, CAL-V0-131 and CAL-V0-133: an attempt claimed by build N keeps
// heartbeating, renewing and releasing after build N+1 is installed in place
// by atomic rename. A build N attempt runner started from the installed path
// before the swap stays alive across it, heartbeats into the store build N+1
// has written, records its outcome and exits cleanly. Rolling back to build N
// the same way keeps an attempt claimed under build N+1 working. The stamped
// build number is the seam.
func TestCALV0130_AttemptClaimedUnderBuildNContinuesUnderNPlus1(t *testing.T) {
	// Tasks imports no Core package (decision 0397), so this mirrors the
	// darwin || linux build constraint of internal/groupreap's owner.
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the build N attempt runner needs the owned process group API")
	}
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
	// The survivor: a build N attempt runner, started from the installed
	// path, whose command waits until the test lets it finish.
	started, finish := filepath.Join(dir, "started"), filepath.Join(dir, "finish")
	survivor := exec.Command(installed, "run", "--attempt", attempt, "--generation", generation, "--timeout", "300", "--",
		"/bin/sh", "-c", survivorFixture, "survivor", started, finish, dir, "120")
	survivor.Dir = r.Root
	var survivorOut, survivorErr bytes.Buffer
	survivor.Stdout, survivor.Stderr = &survivorOut, &survivorErr
	// A descendant that outlives the runner cannot hold Wait on its pipes.
	survivor.WaitDelay = 5 * time.Second
	if err := survivor.Start(); err != nil {
		t.Fatal(err)
	}
	// Cleanup never hangs the package and never signals a process group
	// itself: the runner owns its command's group and retires it, while
	// that group's leader is unreaped, when it gets SIGTERM. The runner is
	// signalled only while it is unreaped, SIGKILL follows if it does not
	// stop, and each wait is bounded.
	runner := watchSurvivor(survivor)
	t.Cleanup(func() {
		if err := runner.retire(10*time.Second, 10*time.Second); err != nil {
			t.Error(err)
		}
	})
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(20 * time.Millisecond) {
		if raw, err := os.ReadFile(started); err == nil && bytes.HasSuffix(raw, []byte("\n")) {
			break
		}
		select {
		case <-runner.done:
			t.Fatalf("build N runner exited before its command started: %v %s %s", runner.waitErr, survivorOut.Bytes(), survivorErr.Bytes())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("build N runner never started its command")
		}
	}

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
	// A build N binary still on disk reads build N+1's records.
	ok(buildN, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "beat-old")
	// The survivor kept running through the swap and the build N+1 writes:
	// its command ends now, and it heartbeats once more, records its outcome
	// and exits 0. Its first heartbeat preceded the swap, so a count of two
	// or more includes one written after it.
	select {
	case <-runner.done:
		t.Fatalf("build N runner exited during the swap: %v %s %s", runner.waitErr, survivorOut.Bytes(), survivorErr.Bytes())
	default:
	}
	fixture.Write(t, finish, nil)
	select {
	case <-runner.done:
		if runner.waitErr != nil {
			t.Fatalf("build N runner: %v %s %s", runner.waitErr, survivorOut.Bytes(), survivorErr.Bytes())
		}
	case <-time.After(time.Minute):
		t.Fatal("build N runner did not finish")
	}
	res, err := wire.DecodeResult(survivorOut.Bytes())
	if err != nil || res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
		t.Fatalf("build N runner envelope: %v %s", err, survivorOut.Bytes())
	}
	item := res.Items[0]
	beats, _ := strconv.Atoi(field(item, "heartbeats").Str)
	if beats < 2 || field(item, "exitStatus").Str != "0" || field(item, "lostLease").Kind != wire.KindNull || field(item, "outcomeReceipt").Str == "" {
		t.Fatalf("build N runner did not heartbeat and record after the swap: %s", wire.Encode(item))
	}
	ok(installed, "release", "--attempt", attempt, "--generation", generation, "--request-id", "release-next")
	ok(installed, "receipt", "audit")
	ok(buildN, "receipt", "audit")
	if x := handoffCLIWithBinary(t, r.Root, installed, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "beat-after-release"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("released attempt still heartbeats: %s", x.stdout)
	}

	// Rollback is the same procedure run the other way (CAL-V0-130): with
	// equal formats, an attempt claimed under build N+1 heartbeats, renews
	// and releases under build N reinstalled by rename.
	backID := field(ok(installed, "ticket", "create", "--request-id", "rollback-create", "--payload", createPayloadJSON), "ticketId").Str
	backClaim := ok(installed, "claim", backID, "--holder", "agent", "--request-id", "rollback-claim")
	backAttempt, backGeneration := field(backClaim, "attemptId").Str, field(backClaim, "generation").Str
	ok(installed, "attempt", "heartbeat", "--attempt", backAttempt, "--generation", backGeneration, "--request-id", "rollback-beat-next")
	install(buildN)
	rolledBack := ok(installed, "version")
	if !strings.HasSuffix(field(rolledBack, "version").Str, "+build.41") || !slices.Equal(formats(rolledBack), formats(after)) {
		t.Fatalf("rolled-back build N version: %s", wire.Encode(rolledBack))
	}
	ok(installed, "attempt", "heartbeat", "--attempt", backAttempt, "--generation", backGeneration, "--request-id", "rollback-beat-n")
	if renewed := ok(installed, "renew", "--attempt", backAttempt, "--generation", backGeneration, "--request-id", "rollback-renew-n"); field(renewed, "generation").Str != backGeneration {
		t.Fatalf("rollback renew moved the generation: %s", wire.Encode(renewed))
	}
	ok(installed, "release", "--attempt", backAttempt, "--generation", backGeneration, "--request-id", "rollback-release-n")
	ok(installed, "receipt", "audit")
	ok(buildNext, "receipt", "audit")
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

// CAL-V0-130 and CAL-V0-131: rollback across different format sets is
// unsupported. Build N+1 here changed the store format, so its store carries
// a newer VERSION; build N refuses its lease verbs and its receipt audit with
// UNSUPPORTED_VERSION and leaves the store bytes unchanged. No drain, restore
// or conversion makes the store readable to build N. (A single record at a
// newer version is refused by its decoder, as the CAL-V0-131 format table
// shows; a hand-edited record cannot stand in for one here, because the
// journal afterimage check reports it as JOURNAL_FORKED first.)
func TestCALV0130_RollbackAcrossDifferentFormatsRefuses(t *testing.T) {
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-12*time.Minute).Truncate(time.Second))
	a := claimed[0]
	stateDir := filepath.Join(root, ".git", "taskman")
	if err := os.Chmod(filepath.Join(stateDir, "VERSION"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(stateDir, "VERSION"), []byte(strings.Replace(snapshot.VersionBytes, "/0", "/1", 1)))
	written := fixture.TreeSnapshot(t, stateDir)
	for _, args := range [][]string{
		{"attempt", "heartbeat", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "beat-rollback"},
		{"renew", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "renew-rollback"},
		{"release", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "release-rollback"},
		{"receipt", "audit"},
	} {
		x := atm(t, root, nil, args...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedVersion) {
			t.Fatalf("%v: %+v", args, x.res)
		}
	}
	if after := fixture.TreeSnapshot(t, stateDir); !reflect.DeepEqual(after, written) {
		t.Fatal("refused rollback wrote state")
	}
}

// outputOnlyProfiles are written only to stdout and never decoded again, so
// they are not part of the CAL-V0-131 format set.
var outputOnlyProfiles = []string{
	"taskman-critical-path/0", "taskman-dispatch-run/0", "taskman-dispatch-status/0", "taskman-dispatch-unpark/0",
	"taskman-lease-timing/0", "taskman-plan-selected/0", "taskman-plan-summary/0", "taskman-plan/0", "taskman-pool-status/0",
	"taskman-priority-first/0",
	"taskman-user-service-install/0", "taskman-user-service-resume/0", "taskman-user-service-run-helper/0",
	"taskman-user-service-run/0", "taskman-user-service-status/0", "taskman-user-service-stop/0",
	"taskman-user-service-uninstall/0",
}

// adoptOnlyProfiles are older versions this build only adopts once and never
// writes, such as a drained taskman-dispatch-state/0 ledger (CAL-V0-132,
// proposed amendment); they are not part of the CAL-V0-131 format set.
var adoptOnlyProfiles = []string{"taskman-dispatch-state/0"}

// CAL-V0-131 and CAL-V0-134: the reported set is sorted and unique, holds the
// store version, and holds every profile the tasks packages name except the
// output-only ones. A new persisted or decoded profile that is not added to
// LiveFormats fails here, as does a stale entry in either list.
func TestCALV0131_LiveFormatsCoverEveryDecodedProfile(t *testing.T) {
	got := cli.LiveFormats()
	if !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != len(got) {
		t.Fatalf("formats not sorted and unique: %v", got)
	}
	version := strings.TrimSpace(snapshot.VersionBytes)
	if !slices.Contains(got, version) {
		t.Fatalf("formats omit the store version %s", version)
	}
	literal := regexp.MustCompile(`"(taskman-[a-z0-9-]+/[0-9]+)"`)
	named := map[string]string{}
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		// LiveFormats' own literals do not count as a decoder naming them.
		if filepath.ToSlash(p) == "../cli/cli.go" {
			at := bytes.Index(raw, []byte("func LiveFormats() []string {"))
			end := bytes.Index(raw[at:], []byte("\n}\n"))
			raw = append(append([]byte{}, raw[:at]...), raw[at+end:]...)
		}
		for _, m := range literal.FindAllSubmatch(raw, -1) {
			named[string(m[1])] = p
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for profile, p := range named {
		lists := 0
		for _, in := range []bool{slices.Contains(got, profile), slices.Contains(outputOnlyProfiles, profile), slices.Contains(adoptOnlyProfiles, profile)} {
			if in {
				lists++
			}
		}
		if lists != 1 {
			t.Errorf("%s (%s) must be in exactly one of LiveFormats, outputOnlyProfiles and adoptOnlyProfiles", profile, p)
		}
	}
	for _, profile := range slices.Concat(got, outputOnlyProfiles, adoptOnlyProfiles) {
		if _, ok := named[profile]; !ok && profile != version {
			t.Errorf("%s is listed but no tasks source names it", profile)
		}
	}
}
