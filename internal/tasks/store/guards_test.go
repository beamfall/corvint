package store_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func editJSON(t *testing.T, path string, edit func(wire.Value)) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	edit(v)
	fixture.Write(t, path, wire.EncodeFile(v))
}

func refuseUnchanged(t *testing.T, repo *intent.Repository, code string) {
	t.Helper()
	before := storeDigest(t, repo)
	report, err := store.Mutate(context.Background(), repo, operator(), envelope("refused", mutation.OpCreate, "", "", createPayload("must not commit")), now(t))
	if wire.CodeOf(err) != code && (err != nil || !report.Outcome.HasCode(code)) {
		t.Fatalf("want %s; got report=%+v err=%v", code, report, err)
	}
	if report.Receipt != "" || storeDigest(t, repo) != before {
		t.Fatal("refusal changed store or intent")
	}
}

func TestTMV0007_AS35_WriterRejectsProjectionDrift(t *testing.T) {
	for _, field := range []string{"title", "acceptanceCriteria", "queue", "policy"} {
		t.Run(field, func(t *testing.T) {
			repo, _ := initialized(t)
			created := mutate(t, repo, envelope("original", mutation.OpCreate, "", "", createPayload("original")))
			id, err := wire.ParseTicketID("ticketId", created.Ticket)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", id.Local+".json")
			if field == "queue" || field == "policy" {
				path = filepath.Join(repo.PrimaryWorktree, intent.Dir, field+".json")
			}
			editJSON(t, path, func(v wire.Value) {
				switch field {
				case "title":
					v.Obj.Set(field, str("manual edit"))
				case "acceptanceCriteria":
					v.Obj.Set(field, wire.Strings([]string{"different acceptance"}))
				default:
					v.Obj.Set("revision", str("2"))
				}
			})
			// A new, unrelated CREATE must also refuse queue-wide divergence.
			refuseUnchanged(t, repo, wire.CodeIntentDiverged)
		})
	}
}

func TestTMV0007_AS35_WriterObservesPrimaryBranch(t *testing.T) {
	for _, head := range []string{"ref: refs/heads/other\n", strings.Repeat("a", 40) + "\n", "missing", "symlink"} {
		t.Run(head, func(t *testing.T) {
			repo, _ := initialized(t)
			path := filepath.Join(repo.CommonDir, "HEAD")
			if head == "missing" || head == "symlink" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if head == "symlink" {
					target := filepath.Join(t.TempDir(), "HEAD")
					fixture.Write(t, target, []byte("ref: refs/heads/main\n"))
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				fixture.Write(t, path, []byte(head))
			}
			refuseUnchanged(t, repo, wire.CodeIntentBranchMismatch)
		})
	}
}

// Keep receipt 3 pending while receipt 2 is the settled head. Corruption of
// genesis then exercises history beyond the old final-receipt-only check.
func pendingMutation(t *testing.T) *intent.Repository {
	t.Helper()
	repo, _ := initialized(t)
	mutate(t, repo, envelope("first", mutation.OpCreate, "", "", createPayload("first")))
	path := filepath.Join(repo.StateDir, "head.json")
	head, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	created := mutate(t, repo, envelope("pending", mutation.OpCreate, "", "", createPayload("pending")))
	id, err := wire.ParseTicketID("", created.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, path, head)
	if err := os.Remove(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", id.Local+".json")); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestTMV0009_AS11_WriterGuardsBeforeRedo(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, kind := range []string{"version", "primary", "barrier", "restore", "restore-directory", "branch"} {
			t.Run(kind+map[bool]string{false: "/settled", true: "/pending"}[pending], func(t *testing.T) {
				var repo *intent.Repository
				if pending {
					repo = pendingMutation(t)
				} else {
					repo, _ = initialized(t)
				}
				code := wire.CodeUnsupported
				switch kind {
				case "version":
					fixture.Write(t, filepath.Join(repo.StateDir, "VERSION"), []byte("taskman-state/9\n"))
				case "primary":
					editJSON(t, filepath.Join(repo.StateDir, "head.json"), func(v wire.Value) { v.Obj.Set("primaryWorktree", str("/private/tmp/other")) })
				case "barrier":
					code = wire.CodePaused
					fixture.Write(t, filepath.Join(repo.StateDir, "barrier.json"), wire.EncodeFile(obj("profile", str("taskman-barrier/0"), "queueId", str(fixture.QueueID), "scope", str("ALL"), "reason", str("EMERGENCY"), "actor", str("tester"), "sinceSeq", str("1"), "since", str(issued))))
				case "restore":
					code = wire.CodeRestoreIncomplete
					fixture.Write(t, filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE"), []byte("incomplete\n"))
				case "restore-directory":
					code = wire.CodeRestoreIncomplete
					if err := os.Mkdir(filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE"), 0700); err != nil {
						t.Fatal(err)
					}
				case "branch":
					code = wire.CodeIntentBranchMismatch
					fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/other\n"))
				}
				refuseUnchanged(t, repo, code)
			})
		}
	}
}

func TestTMV0009_AS11_RedoRequiresCompleteJournalProof(t *testing.T) {
	for _, kind := range []string{"scope", "generation", "request", "history"} {
		t.Run(kind, func(t *testing.T) {
			repo := pendingMutation(t)
			path := filepath.Join(repo.StateDir, "receipts", "000000000003.json")
			if kind == "history" {
				path = filepath.Join(repo.StateDir, "receipts", "000000000001.json")
			}
			if kind == "generation" {
				prior := filepath.Join(repo.StateDir, "receipts", "000000000002.json")
				editJSON(t, prior, func(v wire.Value) { v.Obj.Set("headGeneration", str("1")) })
				raw, err := os.ReadFile(prior)
				if err != nil {
					t.Fatal(err)
				}
				digest := wire.Sum(raw)
				editJSON(t, filepath.Join(repo.StateDir, "head.json"), func(v wire.Value) {
					v.Obj.Set("generation", str("1"))
					v.Obj.Set("lastReceiptSha256", str(string(digest)))
				})
				editJSON(t, path, func(v wire.Value) { v.Obj.Set("prev", str(string(digest))) })
			}
			editJSON(t, path, func(v wire.Value) {
				switch kind {
				case "scope":
					v.Obj.Set("ticketId", str("ticket:elsewhere:main:AT-0003"))
				case "generation":
					v.Obj.Set("headGeneration", str("0"))
				case "request":
					v.Obj.Set("requestId", str("forged-request"))
				case "history":
					v.Obj.Set("recordedAt", str("2026-09-01T00:00:00Z"))
				}
			})
			before := storeDigest(t, repo)
			report, err := store.Mutate(context.Background(), repo, operator(), envelope("next", mutation.OpCreate, "", "", createPayload("never")), now(t))
			if err == nil {
				t.Fatalf("invalid %s accepted: %+v", kind, report)
			}
			if storeDigest(t, repo) != before {
				t.Fatalf("invalid %s caused recovery writes", kind)
			}
		})
	}
}

func TestTMV0006_AS03_ReplayPreservesIdentityAndAllowsStableDivergence(t *testing.T) {
	repo, _ := initialized(t)
	env := envelope("retry", mutation.OpCreate, "", "", createPayload("original"))
	first := mutate(t, repo, env)
	id, err := wire.ParseTicketID("", first.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	editJSON(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", id.Local+".json"), func(v wire.Value) { v.Obj.Set("title", str("manual")) })
	fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/other\n"))
	before := storeDigest(t, repo)
	replay := mutate(t, repo, env)
	if !replay.Outcome.Replayed || replay.Ticket != first.Ticket || replay.Receipt != "" {
		t.Fatalf("identity lost: first=%+v replay=%+v", first, replay)
	}
	conflict := mutate(t, repo, envelope("retry", mutation.OpCreate, "", "", createPayload("changed")))
	if !conflict.Outcome.HasCode(wire.CodeRequestIDConflict) {
		t.Fatalf("conflict: %+v", conflict)
	}
	if storeDigest(t, repo) != before {
		t.Fatal("replay/conflict wrote")
	}
}

func TestTMV0006_AS03_ActorBindingBeforeReplayAndRedo(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "replay", true: "redo"}[pending], func(t *testing.T) {
			var repo *intent.Repository
			if pending {
				repo = pendingMutation(t)
			} else {
				repo, _ = initialized(t)
				mutate(t, repo, envelope("pending", mutation.OpCreate, "", "", createPayload("pending")))
			}
			before := storeDigest(t, repo)
			report, err := store.Mutate(context.Background(), repo, mutation.Binding{ID: "someone-else", Role: "OWNER"}, envelope("pending", mutation.OpCreate, "", "", createPayload("pending")), now(t))
			if err != nil || report.Outcome.Outcome != mutation.OutcomeUnauthorized {
				t.Fatalf("binding: %+v %v", report, err)
			}
			if storeDigest(t, repo) != before {
				t.Fatal("actor mismatch caused writes")
			}
		})
	}
}

func TestTMV0006_AS03_ForgedRequestNeverReplays(t *testing.T) {
	repo, _ := initialized(t)
	env := envelope("retry", mutation.OpCreate, "", "", createPayload("original"))
	mutate(t, repo, env)
	path, err := snapshot.RequestPath("retry")
	if err != nil {
		t.Fatal(err)
	}
	editJSON(t, filepath.Join(repo.StateDir, path), func(v wire.Value) { v.Obj.Set("mutationSha256", str(string(wire.Sum([]byte("forged"))))) })
	before := storeDigest(t, repo)
	report, err := store.Mutate(context.Background(), repo, operator(), env, now(t))
	if wire.CodeOf(err) != wire.CodeJournalForked || report.Outcome.Replayed {
		t.Fatalf("forged entry trusted: %+v %v", report, err)
	}
	if storeDigest(t, repo) != before {
		t.Fatal("forged request caused writes")
	}
}

func TestTMV0009_AS11_InvalidInitLeavesCorrectableInput(t *testing.T) {
	for _, kind := range []string{"role", "request", "branch", "detached"} {
		t.Run(kind, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
			repo, err := intent.Resolve(r.Root)
			if err != nil {
				t.Fatal(err)
			}
			actor := operator()
			request := "bad"
			switch kind {
			case "role":
				actor.Role = "INVALID"
			case "request":
				request = ""
			case "branch":
				fixture.Write(t, filepath.Join(r.CommonDir, "HEAD"), []byte("ref: refs/heads/other\n"))
			case "detached":
				fixture.Write(t, filepath.Join(r.CommonDir, "HEAD"), []byte(strings.Repeat("a", 40)+"\n"))
			}
			before := fixture.TreeSnapshot(t, r.IntentDir)
			bad, err := store.Init(context.Background(), repo, actor, request, now(t))
			if err == nil && bad.Outcome.Outcome == mutation.OutcomeCompleted {
				t.Fatal("invalid init committed")
			}
			if _, err := os.Lstat(repo.StateDir); !os.IsNotExist(err) {
				t.Fatalf("refused init created state: %v", err)
			}
			if !reflect.DeepEqual(fixture.TreeSnapshot(t, r.IntentDir), before) {
				t.Fatal("refused init changed intent")
			}
			fixture.Write(t, filepath.Join(r.CommonDir, "HEAD"), []byte("ref: refs/heads/main\n"))
			good, err := store.Init(context.Background(), repo, operator(), "valid", now(t))
			if err != nil || good.Receipt == "" {
				t.Fatalf("corrected init failed: %+v %v", good, err)
			}
		})
	}
}

func TestTMV0007_AS29_LinkedCallerUsesPrimaryHEAD(t *testing.T) {
	repo, _ := initialized(t)
	gitdir := filepath.Join(repo.CommonDir, "worktrees", "linked")
	linked := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "linked")
	fixture.Write(t, filepath.Join(gitdir, "commondir"), []byte("../..\n"))
	fixture.Write(t, filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/other\n"))
	fixture.Write(t, filepath.Join(linked, ".git"), []byte("gitdir: "+gitdir+"\n"))
	resolved, err := intent.Resolve(linked)
	if err != nil {
		t.Fatal(err)
	}
	report := mutate(t, resolved, envelope("linked", mutation.OpCreate, "", "", createPayload("linked caller")))
	if report.Receipt == "" {
		t.Fatalf("primary main refused: %+v", report)
	}
	fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/other\n"))
	fixture.Write(t, filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/main\n"))
	refuseUnchanged(t, resolved, wire.CodeIntentBranchMismatch)
}

func TestTMV0016_AS27_AdmissionBarrierAllowsNativeMutation(t *testing.T) {
	repo, _ := initialized(t)
	var files []archive.FileEntry
	var dirs []string
	err := filepath.Walk(repo.StateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(repo.StateDir, path)
		if err != nil {
			return err
		}
		if rel == "." || rel == "staging" {
			return nil
		}
		if info.IsDir() {
			dirs = append(dirs, rel)
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, archive.FileEntry{Path: rel, Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	read := func(root, path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	queue := read(filepath.Join(repo.PrimaryWorktree, intent.Dir), "queue.json")
	policy := read(filepath.Join(repo.PrimaryWorktree, intent.Dir), "policy.json")
	for path, raw := range map[string][]byte{"intent/queue.json": queue, "intent/policy.json": policy} {
		files = append(files, archive.FileEntry{Path: path, Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))})
	}
	inv, err := transaction.NewInventory(files, dirs)
	if err != nil {
		t.Fatal(err)
	}
	result := transaction.Model(transaction.Request{Operation: transaction.Pause, QueueID: fixture.QueueID, RequestID: "pause", Actor: operator()}, transaction.Input{Inventory: inv, Head: read(repo.StateDir, "head.json"), Queue: queue, Policy: policy, Reservations: read(repo.StateDir, "reservations.json"), Premise: transaction.FixtureNoRuntime, Branch: "main", Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now(t)})
	if result.Plan == nil {
		t.Fatalf("pause fixture: %+v", result)
	}
	// Materialize a complete, journalled ADMISSION fixture. This does not expose
	// or qualify a production pause writer.
	for _, a := range result.Plan.Artifacts() {
		path := filepath.Join(repo.StateDir, a.Target)
		if strings.HasPrefix(a.Target, "intent/") {
			path = filepath.Join(repo.PrimaryWorktree, intent.Dir, strings.TrimPrefix(a.Target, "intent/"))
		}
		fixture.Write(t, path, a.Data)
	}
	report := mutate(t, repo, envelope("admission", mutation.OpCreate, "", "", createPayload("allowed")))
	if report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" {
		t.Fatalf("ADMISSION blocked ticket mutation: %+v", report)
	}
}
