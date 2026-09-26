package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func now(t *testing.T) wire.Timestamp {
	t.Helper()
	ts, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		t.Fatalf("timestamp: %v", err)
	}
	return ts
}

func operator() mutation.Binding { return mutation.Binding{ID: "tester", Role: "OWNER"} }

// initialized builds a repository whose intent store the operator authored,
// then runs the genesis transaction against it.
func initialized(t *testing.T) (*intent.Repository, *store.Report) {
	t.Helper()
	repo := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(repo.IntentDir, "policy.json"), fixture.PolicyBytes())
	resolved, err := intent.Resolve(repo.Root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	report, err := store.Init(context.Background(), resolved, operator(), "req-init", now(t))
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	return resolved, report
}

// TestTMV0009_AS11_InitCommitsGenesisReceipt is the §5.2 commit point for the
// genesis transaction: the receipt is linked in, and the head and every post
// destination exist after it.
func TestTMV0009_AS11_InitCommitsGenesisReceipt(t *testing.T) {
	repo, report := initialized(t)
	if report.Receipt != "000000000001.json" {
		t.Fatalf("receipt = %q, want 000000000001.json", report.Receipt)
	}
	for _, rel := range []string{
		"VERSION", "head.json", "reservations.json",
		filepath.Join("receipts", "000000000001.json"),
	} {
		if _, err := os.Stat(filepath.Join(repo.StateDir, rel)); err != nil {
			t.Errorf("%s not published: %v", rel, err)
		}
	}
	version, err := os.ReadFile(filepath.Join(repo.StateDir, "VERSION"))
	if err != nil || string(version) != "taskman-state/0\n" {
		t.Errorf("VERSION = %q, %v", version, err)
	}
}

// TestTMV0010_AS29_InitCreatesTheStateDirectories checks that genesis creates
// exactly the §3.4 directories this slice needs and no lane directory it has
// not built.
func TestTMV0010_AS29_InitCreatesTheStateDirectories(t *testing.T) {
	repo, report := initialized(t)
	want := []string{"taskman", "receipts", "evidence", "pinned", "requests", "staging"}
	if len(report.Directories) != len(want) {
		t.Fatalf("directories = %v, want %v", report.Directories, want)
	}
	for i, d := range want {
		if report.Directories[i] != d {
			t.Errorf("directory %d = %q, want %q", i, report.Directories[i], d)
		}
	}
	for _, absent := range []string{"attempts", "effects", "worktrees"} {
		if _, err := os.Stat(filepath.Join(repo.StateDir, absent)); err == nil {
			t.Errorf("%s exists; the runtime slice has not been built", absent)
		}
	}
}

// TestTMV0009_AS11_InitLeavesNoUnassignedStageSlot is the §5.6 rule that a
// staging slot is never left occupied without a live descriptor: a leftover
// slot makes every later reader refuse the store.
func TestTMV0009_AS11_InitLeavesNoUnassignedStageSlot(t *testing.T) {
	repo, _ := initialized(t)
	entries, err := os.ReadDir(filepath.Join(repo.StateDir, "staging"))
	if err != nil {
		t.Fatalf("read staging: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("staging still holds %v; every slot a transaction uses is cleared", names)
	}
}

// TestTMV0002_AS10_InitIsRefusedOnAnInitializedStore checks that a second
// genesis reports the model's BLOCKED refusal and writes nothing, rather than
// failing as a filesystem error.
func TestTMV0002_AS10_InitIsRefusedOnAnInitializedStore(t *testing.T) {
	repo, _ := initialized(t)
	before := readTree(t, repo.StateDir)

	report, err := store.Init(context.Background(), repo, operator(), "req-again", now(t))
	if err != nil {
		t.Fatalf("second init returned an error rather than a refusal: %v", err)
	}
	if report.Outcome.Outcome != mutation.OutcomeBlocked {
		t.Errorf("outcome = %q, want %q", report.Outcome.Outcome, mutation.OutcomeBlocked)
	}
	if report.Receipt != "" {
		t.Errorf("a refused init reported receipt %q", report.Receipt)
	}
	if after := readTree(t, repo.StateDir); after != before {
		t.Error("a refused init changed the store")
	}
}

// TestTMV0008_AS07_InitRequiresTheOperatorsIntentStore checks that a missing
// queue or policy is reported as the operator's own missing input, and that
// nothing is created before it is.
func TestTMV0008_AS07_InitRequiresTheOperatorsIntentStore(t *testing.T) {
	repo := fixture.TempRepo(t)
	resolved, err := intent.Resolve(repo.Root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err = store.Init(context.Background(), resolved, operator(), "req", now(t)); err == nil {
		t.Fatal("init succeeded with no intent store")
	}
	if _, err = os.Stat(resolved.StateDir); err == nil {
		t.Error("the state dir was created before the operator's input was validated")
	}
}

// TestTMV0009_AS35_InitDoesNotOverwriteAnEditedProjection is the §5.2 redo
// rule: a post destination holding neither the pre nor the post state is
// never overwritten, so a concurrent or manual edit survives.
func TestTMV0009_AS35_InitDoesNotOverwriteAnEditedProjection(t *testing.T) {
	repo := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(repo.IntentDir, "policy.json"), fixture.PolicyBytes())
	resolved, err := intent.Resolve(repo.Root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// A reservations file that is not what genesis computes stands in for a
	// state destination at a third value.
	fixture.Write(t, filepath.Join(resolved.StateDir, "reservations.json"), []byte("{}\n"))
	edited, err := os.ReadFile(filepath.Join(resolved.StateDir, "reservations.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_, _ = store.Init(context.Background(), resolved, operator(), "req", now(t))
	after, err := os.ReadFile(filepath.Join(resolved.StateDir, "reservations.json"))
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != string(edited) {
		t.Errorf("edited file was overwritten: %q -> %q", edited, after)
	}
}

// TestTMV0001_AS29_InitRecordsAnUnauthenticatedBinding checks decision 0003's
// honesty rule: the local-operator binding is recorded, and no coverage axis
// is promoted to observed by admitting the real premise.
func TestTMV0001_AS29_InitRecordsAnUnauthenticatedBinding(t *testing.T) {
	_, report := initialized(t)
	for name, got := range map[string]string{
		"actorAuthentication":         report.Coverage.ActorAuthentication,
		"administrativeAuthorization": report.Coverage.AdministrativeAuthorization,
		"durability":                  report.Coverage.Durability,
		"runtimeQualification":        report.Coverage.RuntimeQualification,
		"inventoryObservation":        report.Coverage.InventoryObservation,
	} {
		if got != "NOT_OBSERVED" {
			t.Errorf("%s = %q, want NOT_OBSERVED: the premise grants nothing", name, got)
		}
	}
}

// readTree renders the state dir as a stable string so a test can assert
// that a refused command changed nothing.
func readTree(t *testing.T, root string) string {
	t.Helper()
	out := ""
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		out += path + "\n"
		if !info.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out += string(wire.Sum(raw)) + "\n"
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

// TestCTSV0001_InitRefusesOverExistingRecords pins V1-0323: a genesis over a
// ticket or release record would leave that record without a journal
// afterimage, so init refuses with INTENT_DIVERGED and creates no state dir.
func TestCTSV0001_InitRefusesOverExistingRecords(t *testing.T) {
	for _, dir := range []string{intent.TicketsDir, intent.ReleasesDir} {
		t.Run(dir, func(t *testing.T) {
			repo := fixture.TempRepo(t)
			fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), fixture.QueueBytes())
			fixture.Write(t, filepath.Join(repo.IntentDir, "policy.json"), fixture.PolicyBytes())
			fixture.Write(t, filepath.Join(repo.IntentDir, dir, "X-1.json"), []byte("{}\n"))
			resolved, err := intent.Resolve(repo.Root)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			report, err := store.Init(context.Background(), resolved, operator(), "req", now(t))
			if err != nil {
				t.Fatalf("init returned an error rather than a refusal: %v", err)
			}
			if report.Outcome.Outcome != mutation.OutcomeBlocked || len(report.Outcome.Codes) != 1 || report.Outcome.Codes[0] != wire.CodeIntentDiverged {
				t.Fatalf("outcome = %+v, want BLOCKED with INTENT_DIVERGED", report.Outcome)
			}
			if report.Receipt != "" {
				t.Errorf("a refused init reported receipt %q", report.Receipt)
			}
			if _, err := os.Lstat(resolved.StateDir); !os.IsNotExist(err) {
				t.Errorf("a refused init created the state dir: %v", err)
			}
		})
	}
}
