package cli_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TRM-V0-005 (V1-3189): widen on a live attempt under an OPERATOR admission
// barrier stays refused PAUSED, and the refusal says widen is an admission
// and names the recovery; lifting the barrier lets the same widen through.
func TestTRMV0005_WidenUnderBarrierNamesAdmissionAndRecovery(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "scoped", "P1", `["src/scoped/"]`)
	attempt, generation := claimed(t, handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "claim"))
	if x := handoffCLI(t, root, "pause", "--request-id", "pause", "--role", "OPERATOR"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pause: %s", x.stdout)
	}
	w := handoffCLI(t, root, "widen", "--attempt", attempt, "--generation", generation, "--request-id", "widen-paused", "--scope", "extra/x.go")
	if !hasCode(w.res, wire.CodePaused) {
		t.Fatalf("widen under barrier: want PAUSED: %s", w.stdout)
	}
	for _, want := range []string{"widen admits new paths to a live attempt's reservation", "corvint-tasks unpause", "corvint-tasks pause", "TRM-V0-006"} {
		if !strings.Contains(string(w.stdout), want) {
			t.Fatalf("PAUSED widen detail lacks %q: %s", want, w.stdout)
		}
	}
	if x := handoffCLI(t, root, "unpause", "--request-id", "unpause", "--role", "OPERATOR"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("unpause: %s", x.stdout)
	}
	if x := handoffCLI(t, root, "widen", "--attempt", attempt, "--generation", generation, "--request-id", "widen", "--scope", "extra/x.go"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("widen after unpause: %s", x.stdout)
	}
}

// TRM-V0-006 and TRM-V0-007 (V1-3190): after claim, widen and release, one
// claim restores the widened scope with its provenance; a path never
// admitted to the ticket is still refused OUT_OF_SCOPE. The resume path is
// measured: the widen route takes two mutations, the restoring claim one.
func TestTRMV0006_ResumedClaimRestoresWidenedScope(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "scoped", "P1", `["src/scoped/"]`)
	attempt, generation := claimed(t, handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "claim-1"))
	if x := handoffCLI(t, root, "widen", "--attempt", attempt, "--generation", generation, "--request-id", "widen-1", "--scope", "extra/x.go"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("widen: %s", x.stdout)
	}
	release := func(gen, request string) {
		t.Helper()
		if x := handoffCLI(t, root, "release", "--attempt", attempt, "--generation", gen, "--request-id", request); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release: %s", x.stdout)
		}
	}
	release(generation, "release-1")

	// Before: the widen route resumes in two mutations, claim then widen.
	before := 0
	resumed := handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "claim-2")
	before++
	a2, g2 := claimed(t, resumed)
	if x := handoffCLI(t, root, "widen", "--attempt", a2, "--generation", g2, "--request-id", "widen-2", "--scope", "extra/x.go"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("widen-2: %s", x.stdout)
	}
	before++
	widenedScope := scopePaths(t, root, attempt)
	release(g2, "release-2")

	never := handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "claim-never", "--scope", "extra/x.go", "never/y.go")
	if !hasCode(never.res, wire.CodeOutOfScope) {
		t.Fatalf("never-admitted path: want OUT_OF_SCOPE: %s", never.stdout)
	}
	if rejected := field(field(never.res.Items[0], "scopeRefusal"), "rejected"); len(rejected.Arr) != 1 || rejected.Arr[0].Str != "never/y.go" {
		t.Fatalf("rejected %s", wire.Encode(rejected))
	}

	// After: one claim restores the widened scope.
	after := 0
	c := handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "claim-3", "--scope", "extra/x.go")
	after++
	a3, g3 := claimed(t, c)
	if a3 != attempt {
		t.Fatalf("resumed claim is attempt %s, want the retried %s", a3, attempt)
	}
	restored := field(c.res.Items[0], "restoredScope")
	if field(restored, "fromGeneration").Str != g2 || len(field(restored, "paths").Arr) != 1 || field(restored, "paths").Arr[0].Str != "extra/x.go" {
		t.Fatalf("restoredScope %s", wire.Encode(restored))
	}
	if got := scopePaths(t, root, attempt); !slices.Equal(got, widenedScope) {
		t.Fatalf("restored scope %v, want the widened %v", got, widenedScope)
	}
	if source := field(field(c.res.Items[0], "scope"), "source").Str; source != "DECLARED" {
		t.Fatalf("restored scope source %q", source)
	}
	assertRestoreReceipt(t, root, field(c.res.Items[0], "receipt").Str, attempt, g2, g3)
	if before != 2 || after != 1 {
		t.Fatalf("resume command count before %d after %d", before, after)
	}
	t.Logf("TRM-V0-006 resume path: %d mutations before, %d after", before, after)
}

// TRM-V0-800 to TRM-V0-802 (V1-3191): under an OPERATOR admission barrier
// one named ticket is claimed and its attempt widened with the explicit,
// request-bound --admit-under-barrier, the barrier record stays as it was,
// and every other admission stays refused PAUSED.
func TestTRMV0800_AdmitUnderBarrierNamesOneTicket(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	named := planTicket(t, root, "named", "P1", `["src/named/"]`)
	other := planTicket(t, root, "other", "P1", `["src/other/"]`)
	if x := handoffCLI(t, root, "pause", "--request-id", "pause", "--role", "OPERATOR"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pause: %s", x.stdout)
	}
	barrier := func() wire.BarrierRef {
		t.Helper()
		b := handoffCLI(t, root, "queue", "status").res.Snapshot.Barrier
		if b == nil {
			t.Fatal("barrier lifted")
		}
		return *b
	}
	before := barrier()
	plain := handoffCLI(t, root, "claim", named, "--holder", "builder", "--request-id", "plain")
	if !hasCode(plain.res, wire.CodePaused) || !strings.Contains(string(plain.stdout), "claim --admit-under-barrier") {
		t.Fatalf("plain claim under barrier: want PAUSED naming the route: %s", plain.stdout)
	}
	if next := handoffCLI(t, root, "claim", "--next", "--holder", "builder", "--request-id", "next-admit", "--admit-under-barrier"); !hasCode(next.res, wire.CodeMalformed) {
		t.Fatalf("claim --next --admit-under-barrier: want MALFORMED: %s", next.stdout)
	}
	attempt, generation := claimed(t, handoffCLI(t, root, "claim", named, "--holder", "builder", "--request-id", "admit", "--admit-under-barrier"))
	if x := handoffCLI(t, root, "claim", named, "--holder", "builder", "--request-id", "admit"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("replay without the flag reused the exempted request: %s", x.stdout)
	}
	w := handoffCLI(t, root, "widen", "--attempt", attempt, "--generation", generation, "--request-id", "widen-plain", "--scope", "extra/x.go")
	if !hasCode(w.res, wire.CodePaused) || !strings.Contains(string(w.stdout), "--admit-under-barrier") {
		t.Fatalf("plain widen under barrier: want PAUSED naming the route: %s", w.stdout)
	}
	if x := handoffCLI(t, root, "widen", "--attempt", attempt, "--generation", generation, "--request-id", "widen-admit", "--scope", "extra/x.go", "--admit-under-barrier"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("exempted widen: %s", x.stdout)
	}
	if got := scopePaths(t, root, attempt); !slices.Contains(got, "PATH:extra/x.go") {
		t.Fatalf("exempted widen scope %v", got)
	}
	for _, args := range [][]string{{"claim", other, "--holder", "builder", "--request-id", "other"}, {"claim", "--next", "--holder", "builder", "--request-id", "next"}} {
		if x := handoffCLI(t, root, args...); !hasCode(x.res, wire.CodePaused) {
			t.Fatalf("%v during the exemption: want PAUSED: %s", args, x.stdout)
		}
	}
	if after := barrier(); after != before {
		t.Fatalf("barrier changed: %+v, was %+v", after, before)
	}
	if x := handoffCLI(t, root, "receipt", "audit"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("receipt audit: %s", x.stdout)
	}
}

func claimed(t *testing.T, x run) (attempt, generation string) {
	t.Helper()
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("claim: %s", x.stdout)
	}
	return field(x.res.Items[0], "attemptId").Str, field(x.res.Items[0], "generation").Str
}

func scopePaths(t *testing.T, root, attempt string) []string {
	t.Helper()
	a := readAttempt(t, root, attempt)
	out := []string{}
	for _, r := range a.Scope.Resources {
		out = append(out, r.Class+":"+r.Key)
	}
	slices.Sort(out)
	return out
}

func readAttempt(t *testing.T, root, attempt string) *snapshot.Attempt {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "attempts", attempt+".json"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := snapshot.DecodeAttempt(raw)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// assertRestoreReceipt checks the ADMIT receipt binds the ended generation
// it restored from (Pre) and records the new generation naming it (Post).
func assertRestoreReceipt(t *testing.T, root, name, attempt, fromGeneration, generation string) {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", name))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		t.Fatal(err)
	}
	path := "attempts/" + attempt + ".json"
	pre := slices.ContainsFunc(rc.Pre, func(p snapshot.PreEntry) bool { return p.Path == path && p.Sha256 != nil })
	post := slices.ContainsFunc(rc.Post, func(p snapshot.PostEntry) bool { return p.Path == path })
	if !pre || !post || rc.Generation == nil || string(*rc.Generation) != generation {
		t.Fatalf("receipt %s does not bind the restored attempt: %+v", name, rc)
	}
	a := readAttempt(t, root, attempt)
	if n := len(a.PriorGenerations); n == 0 || string(a.PriorGenerations[n-1].Generation) != fromGeneration {
		t.Fatalf("prior generations %+v lack %s", a.PriorGenerations, fromGeneration)
	}
}
