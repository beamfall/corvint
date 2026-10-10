package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mutateAuditOutcome is what Mutate takes from its request lookup, inventory
// and canonical audit, with the phase of the first refusal.
type mutateAuditOutcome struct {
	Err       string
	Found     bool
	Entry     mutation.IndexEntry
	Ticket    string
	Inventory *transaction.Inventory
	Canonical *journal.Result
}

// separateMutateAudits is Mutate's sequence before CAL-V0-070. afterLookup,
// when given, runs between the lookup and the inventory.
func separateMutateAudits(t *testing.T, repo *intent.Repository, id string, afterLookup ...func()) mutateAuditOutcome {
	t.Helper()
	head, err := writerGuards(repo, transaction.Mutate)
	if err != nil {
		t.Fatal(err)
	}
	reader := journalReader(repo, head)
	index := journal.RequestIndex{Reader: reader}
	var out mutateAuditOutcome
	entry, found, err := index.Lookup(id)
	if err != nil {
		out.Err = "lookup: " + err.Error()
		return out
	}
	if found {
		out.Found, out.Entry, out.Ticket = true, entry, index.TicketID
		return out
	}
	for _, f := range afterLookup {
		f()
	}
	if out.Inventory, err = inventory(repo); err != nil {
		out.Err, out.Inventory = "inventory: "+err.Error(), nil
		return out
	}
	if out.Canonical, err = reader.Audit(mutationSelection(out.Inventory)...); err != nil {
		out.Err, out.Canonical = "audit: "+err.Error(), nil
	}
	return out
}

// mergedMutateAudits is Mutate's sequence after CAL-V0-070: a change watch,
// the merged audit, then observeMutation. It fails if an unchanged store that
// observeMutation accepts was observed again instead of reused, or if a
// refusal there did not come from the fresh passes.
func mergedMutateAudits(t *testing.T, repo *intent.Repository, id string) (mutateAuditOutcome, *journal.MutationAudit) {
	t.Helper()
	head, err := writerGuards(repo, transaction.Mutate)
	if err != nil {
		t.Fatal(err)
	}
	reader := journalReader(repo, head)
	watch, err := authority.WatchChanges(repo)
	if err != nil {
		t.Fatalf("change watch: %v", err)
	}
	defer watch.Close()
	var out mutateAuditOutcome
	audit, err := reader.AuditForMutation(id)
	if err != nil {
		out.Err = "lookup: " + err.Error()
		return out, audit
	}
	if audit.Found {
		out.Found, out.Entry, out.Ticket = true, audit.Entry, audit.TicketID
		return out, audit
	}
	var stages []string
	ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) { stages = append(stages, stage) })
	inv, paths, canonical, err := observeMutation(ctx, repo, reader, audit, watch)
	want := "audited,observed"
	if err != nil {
		want += ",fresh"
	}
	if got := strings.Join(stages, ","); got != want {
		t.Fatalf("stages %s, want %s", got, want)
	}
	if inv == nil {
		out.Err = "inventory: " + err.Error()
		return out, audit
	}
	out.Inventory = inv
	if err != nil {
		out.Err = "audit: " + err.Error()
		return out, audit
	}
	if _, reused, _ := audit.Canonical(paths...); !reused {
		t.Fatal("Mutate's own selection was not reused")
	}
	out.Canonical = canonical
	return out, audit
}

// CAL-V0-070: on a Mutate-built store, the merged audit with observed
// inventory digests gives Mutate the same refusal in the same phase, the same
// inventory and the same canonical Result as Lookup, a fresh inventory and a
// second Audit. A stray state directory is refused by the lookup capture in
// both flows, before an intent divergence the merged audit defers.
func TestCALV0070_MutateAuditSequenceEquivalence(t *testing.T) {
	t.Parallel()
	// One store; every edit is undone before the next case.
	repo := historyStore(t, 70)
	rewriteFile := func(t *testing.T, p string, edit func([]byte) []byte) func() {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		historyWrite(t, p, edit(append([]byte{}, raw...)))
		return func() { historyWrite(t, p, raw) }
	}
	ticket := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets")
	entries, err := os.ReadDir(ticket)
	if err != nil || len(entries) == 0 {
		t.Fatalf("tickets: %v", err)
	}
	ticket = filepath.Join(ticket, entries[0].Name())
	editTicket := func(t *testing.T) func() {
		return rewriteFile(t, ticket, func(raw []byte) []byte { return append(raw, '\n') })
	}
	unexpectedDir := func(t *testing.T) func() {
		p := filepath.Join(repo.StateDir, "receipts", "nested")
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return func() { os.Remove(p) }
	}
	cases := []struct {
		name     string
		id       string
		edit     func(*testing.T) func()
		want     string
		found    bool
		physical bool
	}{
		{name: "clean-absent", id: "absent", physical: true},
		{name: "clean-found-ticket", id: "history-create-3", found: true, physical: true},
		{name: "ticket-edited", id: "absent", edit: editTicket, want: "audit: INTENT_DIVERGED"},
		{name: "ticket-edited-found", id: "history-create-3", edit: editTicket, found: true},
		{name: "request-corrupt", id: "absent", want: "lookup: JOURNAL_FORKED", edit: func(t *testing.T) func() {
			p, _ := snapshot.RequestPath("history-60")
			return rewriteFile(t, filepath.Join(repo.StateDir, p), func([]byte) []byte { return []byte("{}\n") })
		}},
		{name: "reservations-edited", id: "absent", want: "lookup: JOURNAL_FORKED", edit: func(t *testing.T) func() {
			return rewriteFile(t, filepath.Join(repo.StateDir, "reservations.json"), func(raw []byte) []byte { return append(raw, '\n') })
		}},
		{name: "unexpected-state-dir", id: "absent", edit: unexpectedDir, want: "lookup: "},
		{name: "unexpected-state-dir-and-ticket-edited", id: "absent", want: "lookup: ", edit: func(t *testing.T) func() {
			undoDir, undoTicket := unexpectedDir(t), editTicket(t)
			return func() { undoTicket(); undoDir() }
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.edit != nil {
				defer tc.edit(t)()
			}
			baseline := separateMutateAudits(t, repo, tc.id)
			merged, audit := mergedMutateAudits(t, repo, tc.id)
			if !strings.HasPrefix(baseline.Err, tc.want) || (tc.want == "") != (baseline.Err == "") || baseline.Found != tc.found {
				t.Fatalf("fixture outcome: %+v", baseline)
			}
			if tc.found && baseline.Ticket == "" && tc.id == "history-create-3" {
				t.Fatal("found request lost its ticket")
			}
			if !reflect.DeepEqual(merged, baseline) {
				t.Fatalf("merged differs:\n got  %+v\n want %+v", merged, baseline)
			}
			if (audit.Physical.Files != nil) != tc.physical {
				t.Fatalf("physical publication: %d files", len(audit.Physical.Files))
			}
			if tc.physical && !tc.found {
				fresh, err := inventory(repo)
				if err != nil || !readUnchanged(fresh, audit.Physical.Files) {
					t.Fatalf("fresh inventory does not match the merged audit's reads: %v", err)
				}
				t.Logf("merged audit read %d of the inventory's %d files", len(audit.Physical.Files), len(fresh.Files()))
			}
		})
	}
}

// CAL-V0-070: an outside change made after Mutate's merged audit, at either
// observeMutation stage before its change check, is refused through the real
// Mutate with the code the separate passes raise on the changed store, and
// nothing is published. Reusing the audit there would have accepted a stale
// projection digest, or reached the pre-apply binding's SNAPSHOT_MOVED.
func TestCALV0070_MutateRefusesChangesAfterMergedAudit(t *testing.T) {
	t.Parallel()
	repo := historyStore(t, 70)
	stateFile := func(rel string) string { return filepath.Join(repo.StateDir, rel) }
	requestFile, ticketFile := mutationBoundaryFiles(t, repo)
	rewrite := func(t *testing.T, p string, raw []byte) func() {
		old, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		historyWrite(t, p, raw)
		return func() { historyWrite(t, p, old) }
	}
	appendLine := func(p string) func(*testing.T) func() {
		return func(t *testing.T) func() {
			old, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			return rewrite(t, p, append(old, '\n'))
		}
	}
	symlinkRequest := func(t *testing.T) func() {
		old, err := os.ReadFile(requestFile)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "request.json")
		historyWrite(t, target, old)
		if err = os.Remove(requestFile); err != nil {
			t.Fatal(err)
		}
		if err = os.Symlink(target, requestFile); err != nil {
			t.Fatal(err)
		}
		return func() {
			if err := os.Remove(requestFile); err != nil {
				t.Fatal(err)
			}
			historyWrite(t, requestFile, old)
		}
	}
	strayBarrier := func(t *testing.T) func() {
		p := stateFile("barrier.json")
		historyWrite(t, p, []byte("{}\n"))
		return func() { os.Remove(p) }
	}
	published := func(t *testing.T) string { return mutationPublished(t, repo) }
	cases := []struct {
		name, stage, want string
		change            func(*testing.T) func()
	}{
		{"request-overwritten", "audited", "JOURNAL_FORKED", func(t *testing.T) func() { return rewrite(t, requestFile, []byte("{}\n")) }},
		{"request-overwritten-after-observation", "observed", "JOURNAL_FORKED", func(t *testing.T) func() { return rewrite(t, requestFile, []byte("{}\n")) }},
		{"request-symlinked", "audited", "UNSUPPORTED_FILESYSTEM", symlinkRequest},
		{"ticket-edited", "audited", "INTENT_DIVERGED", appendLine(ticketFile)},
		{"ticket-edited-after-observation", "observed", "INTENT_DIVERGED", appendLine(ticketFile)},
		{"reservations-edited", "audited", "JOURNAL_FORKED", appendLine(stateFile("reservations.json"))},
		{"barrier-stray", "observed", "JOURNAL_FORKED", strayBarrier},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := "boundary-" + tc.name
			before := published(t)
			var stages []string
			undo := func() {}
			ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) {
				stages = append(stages, stage)
				if stage == tc.stage {
					undo = tc.change(t)
				}
			})
			rep, err := Mutate(ctx, repo, historyActor, historyCreate(id, "boundary "+tc.name), WallClock())
			undo()
			code := wire.CodeOf(err)
			// The separate passes with the same change made after their lookup.
			oracle := separateMutateAudits(t, repo, id, func() { undo = tc.change(t) })
			undo()
			if err == nil || (tc.want != "" && code != tc.want) || !strings.HasPrefix(oracle.Err, "inventory: "+code) && !strings.HasPrefix(oracle.Err, "audit: "+code) {
				t.Fatalf("Mutate: %v (report %+v); separate passes: %s", err, rep, oracle.Err)
			}
			t.Logf("refused %s, as the separate passes: %s", code, oracle.Err)
			if want := []string{"watched", "audited", "observed", "fresh"}; !reflect.DeepEqual(stages, want) {
				t.Fatalf("stages %v, want %v", stages, want)
			}
			if after := published(t); after != before {
				t.Fatalf("published: %s, before %s", after, before)
			}
			if p, _ := snapshot.RequestPath(id); fileExists(stateFile(p)) {
				t.Fatal("refused request was projected")
			}
		})
	}
	// A store changed before Mutate starts: the merged audit's refusal is
	// taken again without the watch, and a refusal it defers is raised by the
	// fresh passes.
	for _, tc := range []struct {
		name   string
		stages []string
		change func(*testing.T) func()
	}{
		{"request-overwritten-before", []string{"watched", "retry: "}, func(t *testing.T) func() { return rewrite(t, requestFile, []byte("{}\n")) }},
		{"ticket-edited-before", []string{"watched", "audited", "observed", "fresh"}, appendLine(ticketFile)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := "boundary-" + tc.name
			before := published(t)
			undo := tc.change(t)
			defer undo()
			var stages []string
			ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) { stages = append(stages, stage) })
			rep, err := Mutate(ctx, repo, historyActor, historyCreate(id, "boundary "+tc.name), WallClock())
			oracle := separateMutateAudits(t, repo, id)
			_, oracleErr, _ := strings.Cut(oracle.Err, ": ")
			if err == nil || oracleErr == "" || !strings.HasPrefix(oracleErr, wire.CodeOf(err)+":") {
				t.Fatalf("Mutate: %v (report %+v); separate passes: %s", err, rep, oracle.Err)
			}
			t.Logf("refused %s, as the separate passes: %s", wire.CodeOf(err), oracle.Err)
			if !stagesMatch(stages, tc.stages) {
				t.Fatalf("stages %v, want %v", stages, tc.stages)
			}
			if after := published(t); after != before {
				t.Fatalf("published: %s, before %s", after, before)
			}
			if p, _ := snapshot.RequestPath(id); fileExists(stateFile(p)) {
				t.Fatal("refused request was projected")
			}
		})
	}
	t.Run("unchanged-reuses", func(t *testing.T) {
		var stages []string
		ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) { stages = append(stages, stage) })
		rep, err := Mutate(ctx, repo, historyActor, historyCreate("boundary-unchanged", "boundary unchanged"), WallClock())
		if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("mutate: %+v %v", rep, err)
		}
		if want := []string{"watched", "audited", "observed"}; !reflect.DeepEqual(stages, want) {
			t.Fatalf("stages %v, want %v", stages, want)
		}
	})
}

// mutationBoundaryFiles is the request projection and the first ticket file
// that the CAL-V0-070 boundary tests change.
func mutationBoundaryFiles(t *testing.T, repo *intent.Repository) (request, ticket string) {
	t.Helper()
	request, _ = snapshot.RequestPath("history-60")
	tickets := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets")
	entries, err := os.ReadDir(tickets)
	if err != nil || len(entries) == 0 {
		t.Fatalf("tickets: %v", err)
	}
	return filepath.Join(repo.StateDir, request), filepath.Join(tickets, entries[0].Name())
}

// mutationPublished is what a refused Mutate must leave as it was.
func mutationPublished(t *testing.T, repo *intent.Repository) string {
	t.Helper()
	head, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := os.ReadDir(filepath.Join(repo.StateDir, "receipts"))
	if err != nil {
		t.Fatal(err)
	}
	_, stagingErr := os.Lstat(filepath.Join(repo.StateDir, "staging"))
	return fmt.Sprintf("head %s, %d receipts, staging %v", wire.Sum(head), len(receipts), !os.IsNotExist(stagingErr))
}

// stagesMatch compares Mutate's stages with want, where a "retry: " entry
// matches a retry stage with any refusal.
func stagesMatch(stages, want []string) bool {
	if len(stages) != len(want) {
		return false
	}
	for i, w := range want {
		if stages[i] != w && !(w == "retry: " && strings.HasPrefix(stages[i], w)) {
			return false
		}
	}
	return true
}

func fileExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}
