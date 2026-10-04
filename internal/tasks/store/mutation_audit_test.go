package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
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

func mutateSelection(inv *transaction.Inventory) []string {
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, file := range inv.Files() {
		if strings.HasPrefix(file.Path, "intent/tickets/") || strings.HasPrefix(file.Path, "intent/releases/") {
			paths = append(paths, file.Path)
		}
	}
	return paths
}

// separateMutateAudits is Mutate's sequence before CAL-V0-070.
func separateMutateAudits(t *testing.T, repo *intent.Repository, id string) mutateAuditOutcome {
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
	if out.Inventory, err = inventory(repo); err != nil {
		out.Err, out.Inventory = "inventory: "+err.Error(), nil
		return out
	}
	if out.Canonical, err = reader.Audit(mutateSelection(out.Inventory)...); err != nil {
		out.Err, out.Canonical = "audit: "+err.Error(), nil
	}
	return out
}

// mergedMutateAudits is Mutate's sequence after CAL-V0-070.
func mergedMutateAudits(t *testing.T, repo *intent.Repository, id string) (mutateAuditOutcome, *journal.MutationAudit) {
	t.Helper()
	head, err := writerGuards(repo, transaction.Mutate)
	if err != nil {
		t.Fatal(err)
	}
	reader := journalReader(repo, head)
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
	if out.Inventory, err = inventory(repo, audit.Physical.Files); err != nil {
		out.Err, out.Inventory = "inventory: "+err.Error(), nil
		return out, audit
	}
	canonical, reused, err := audit.Canonical(mutateSelection(out.Inventory)...)
	if !reused {
		t.Fatal("Mutate's own selection was not reused")
	}
	if err != nil {
		out.Err = "audit: " + err.Error()
		return out, audit
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
				observed, observedErr := inventory(repo, audit.Physical.Files)
				if err != nil || observedErr != nil || !reflect.DeepEqual(fresh, observed) {
					t.Fatalf("observed inventory differs: %v %v", err, observedErr)
				}
				reused := 0
				for _, f := range fresh.Files() {
					if _, ok := audit.Physical.Files[f.Path]; ok {
						reused++
					}
				}
				if reused == 0 {
					t.Fatal("no inventory digest reused")
				}
				t.Logf("inventory digests reused: %d of %d", reused, len(fresh.Files()))
			}
		})
	}
}
