//go:build darwin || linux

package journal

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mutationOutcome is everything store.Mutate takes from its journal audits:
// the phase and text of the first refusal, the request proof, and the
// canonical Result it plans from.
type mutationOutcome struct {
	Err      string
	Found    bool
	Entry    mutation.IndexEntry
	Ticket   string
	Identity Identity
	Result   *Result
}

// separateMutationAudits is store.Mutate before CAL-V0-070: RequestIndex's
// lookup audit, then a second complete audit of the observed selection.
func separateMutationAudits(r Reader, id string, lim limits, paths []string) mutationOutcome {
	lookup, err := r.audit(nil, id, lim, false)
	var out mutationOutcome
	if lookup != nil {
		out.Identity = lookup.Identity
	}
	if err != nil {
		out.Err = "lookup: " + err.Error()
		return out
	}
	if lookup.request != nil {
		out.Found, out.Entry, out.Ticket = true, lookup.request.Entry, lookup.requestTicket
		return out
	}
	out.Result, err = r.audit(paths, "", lim, true)
	if err != nil {
		out.Err, out.Result = "audit: "+err.Error(), nil
	}
	return out
}

func mergedMutationAudit(t *testing.T, r Reader, id string, lim limits, paths []string) (mutationOutcome, *MutationAudit) {
	t.Helper()
	m, err := r.auditForMutation(id, lim)
	out := mutationOutcome{Identity: m.Identity}
	if err != nil {
		out.Err = "lookup: " + err.Error()
		return out, m
	}
	if m.Found {
		out.Found, out.Entry, out.Ticket = true, m.Entry, m.TicketID
		return out, m
	}
	result, reused, err := m.Canonical(paths...)
	if !reused {
		t.Fatalf("observed selection not reused: %v", paths)
	}
	if err != nil {
		out.Err = "audit: " + err.Error()
		return out, m
	}
	out.Result = result
	return out, m
}

// observedIntentPaths is the selection store.Mutate derives from its
// inventory: queue, policy and every regular ticket and release file.
func observedIntentPaths(t *testing.T, repo *fixture.Repo) []string {
	t.Helper()
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, dir := range []string{"tickets", "releases"} {
		entries, err := os.ReadDir(filepath.Join(repo.IntentDir, dir))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") {
				paths = append(paths, "intent/"+dir+"/"+e.Name())
			}
		}
	}
	return paths
}

// CAL-V0-070: the merged mutation audit and pinned-parent reads return the
// same refusal, in the same phase, and the same Result as the separate audits
// with per-file opens, on clean, tampered, missing, non-regular and moving
// inputs.
func TestCALV0070_MergedMutationAuditEquivalence(t *testing.T) {
	requestR, _ := snapshot.RequestPath("R")
	other, _ := snapshot.RequestPath("other")
	stray, _ := snapshot.RequestPath("stray")
	state := func(repo *fixture.Repo, p string) string { return filepath.Join(repo.StateDir, p) }
	corruptRequest := func(t *testing.T, repo *fixture.Repo) { fixture.Write(t, state(repo, other), []byte("{}\n")) }
	editTicket := func(t *testing.T, repo *fixture.Repo) {
		edited := fixture.Ticket("A")
		edited.Title = "stable human edit"
		fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "A.json"), edited.Encode())
	}
	// swapOther replaces the other request projection once, between the first
	// capture and its reads, and returns the restoration.
	swapOther := func(symlink bool) func(*testing.T, *fixture.Repo) func() {
		return func(t *testing.T, repo *fixture.Repo) func() {
			p := state(repo, other)
			raw := read(t, p)
			remove(t, p)
			if symlink {
				copyPath := filepath.Join(t.TempDir(), "copy.json")
				fixture.Write(t, copyPath, raw)
				if err := os.Symlink(copyPath, p); err != nil {
					t.Fatal(err)
				}
			}
			return func() {
				_ = os.Remove(p)
				fixture.Write(t, p, raw)
			}
		}
	}
	small := limits{scan: wire.MaxArchiveScanEntries, selected: 1}
	const (
		forked    = "lookup: " + wire.CodeJournalForked
		filesys   = "lookup: " + wire.CodeUnsupportedFilesystem
		diverged  = "audit: " + wire.CodeIntentDiverged
		overspent = "audit: " + wire.CodeLimitExceeded
	)
	cases := []struct {
		name string
		id   string
		lim  *limits
		edit func(*testing.T, *fixture.Repo)
		race func(*testing.T, *fixture.Repo) func()
		// want is the baseline's refusal phase and code; "" plans or replays.
		want  string
		found bool
		// physical: settled with every projection agreeing, so digests publish
		physical bool
	}{
		{name: "clean-absent", id: "absent", physical: true},
		{name: "clean-found", id: "R", found: true, physical: true},
		{name: "receipt-rebound", id: "absent", want: forked, edit: func(t *testing.T, repo *fixture.Repo) {
			rewrite(t, repo, 3, func(v wire.Value) { v.Obj.Set("requestId", str("rebound")) })
		}},
		{name: "receipt-bytes", id: "absent", want: "lookup: " + wire.CodeMalformed, edit: func(t *testing.T, repo *fixture.Repo) {
			p := state(repo, "receipts/000000000003.json")
			raw := read(t, p)
			fixture.Write(t, p, append(raw[:len(raw)-1:len(raw)-1], ' ', '\n'))
		}},
		{name: "receipt-missing", id: "absent", want: forked, edit: func(t *testing.T, repo *fixture.Repo) { remove(t, state(repo, "receipts/000000000003.json")) }},
		{name: "request-missing", id: "absent", want: forked, edit: func(t *testing.T, repo *fixture.Repo) { remove(t, state(repo, other)) }},
		{name: "request-corrupt", id: "absent", want: forked, edit: corruptRequest},
		{name: "request-corrupt-found", id: "R", want: forked, edit: corruptRequest},
		{name: "request-stray", id: "absent", want: forked, edit: func(t *testing.T, repo *fixture.Repo) {
			fixture.Write(t, state(repo, stray), requestBytes("stray", 9, wire.Sum([]byte("stray")), false))
		}},
		{name: "request-symlink", id: "absent", want: filesys, edit: func(t *testing.T, repo *fixture.Repo) { swapOther(true)(t, repo) }},
		{name: "request-fifo", id: "absent", want: filesys, edit: func(t *testing.T, repo *fixture.Repo) {
			remove(t, state(repo, other))
			if err := syscall.Mkfifo(state(repo, other), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "request-removed-after-capture", id: "absent", want: forked, race: swapOther(false)},
		{name: "request-symlinked-after-capture", id: "absent", want: filesys, race: swapOther(true)},
		{name: "ticket-edited", id: "absent", want: diverged, edit: editTicket},
		{name: "ticket-edited-found", id: "R", found: true, edit: editTicket},
		{name: "ticket-missing", id: "absent", want: diverged, edit: func(t *testing.T, repo *fixture.Repo) {
			remove(t, filepath.Join(repo.IntentDir, "tickets", "A.json"))
		}},
		{name: "ticket-stray", id: "absent", want: diverged, edit: func(t *testing.T, repo *fixture.Repo) {
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "B.json"), fixture.Ticket("B").Encode())
		}},
		{name: "ticket-symlink", id: "absent", want: filesys, edit: func(t *testing.T, repo *fixture.Repo) {
			if err := os.Symlink(filepath.Join(repo.IntentDir, "tickets", "A.json"), filepath.Join(repo.IntentDir, "tickets", "C.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "queue-edited", id: "absent", want: diverged, edit: func(t *testing.T, repo *fixture.Repo) {
			p := filepath.Join(repo.IntentDir, "queue.json")
			raw := read(t, p)
			fixture.Write(t, p, append(raw[:len(raw):len(raw)], '\n'))
		}},
		{name: "budget", id: "absent", want: overspent, lim: &small},
		{name: "budget-found", id: "R", found: true, lim: &small},
		{name: "budget-then-request-corrupt", id: "absent", want: forked, lim: &small, edit: corruptRequest},
		{name: "budget-before-ticket-edited", id: "absent", want: overspent, lim: &small, edit: editTicket},
		{name: "pending", id: "absent", want: "lookup: " + wire.CodeRedoPending, edit: func(t *testing.T, repo *fixture.Repo) {
			path, _ := snapshot.RequestPath("late")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes("late", 5, wire.Sum([]byte("late")), false)}, "late", true, false, false)
		}},
	}
	defer func() { pinnedReads = true }()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, r := setup(t)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()}, "", true, true, false)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{requestR: requestBytes("R", 3, wire.Sum([]byte("R")), false)}, "R", true, true, false)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{other: requestBytes("other", 4, wire.Sum([]byte("other")), false)}, "other", true, true, false)
			if tc.edit != nil {
				tc.edit(t, repo)
			}
			lim := profileLimits
			if tc.lim != nil {
				lim = *tc.lim
			}
			paths := observedIntentPaths(t, repo)
			stateBefore, intentBefore := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentDir)
			run := func(pinned, merged bool) (mutationOutcome, *MutationAudit) {
				pinnedReads = pinned
				defer func() { pinnedReads = true }()
				rr := r
				if tc.race != nil {
					var restore func()
					rr.afterCapture = func() {
						if restore == nil {
							restore = tc.race(t, repo)
						}
					}
					defer func() { restore() }()
				}
				if merged {
					return mergedMutationAudit(t, rr, tc.id, lim, paths)
				}
				return separateMutationAudits(rr, tc.id, lim, paths), nil
			}
			baseline, _ := run(false, false)
			pinned, _ := run(true, false)
			merged, m := run(true, true)
			mergedPerFile, _ := run(false, true)
			if tc.race == nil {
				fixture.AssertUntouched(t, repo, stateBefore, intentBefore, "mutation audit")
			}

			if !strings.HasPrefix(baseline.Err, tc.want) || (tc.want == "") != (baseline.Err == "") || baseline.Found != tc.found {
				t.Fatalf("fixture outcome: %+v", baseline)
			}
			for name, got := range map[string]mutationOutcome{"pinned": pinned, "merged": merged, "merged per-file": mergedPerFile} {
				if tc.race != nil {
					// Each run's restoration rewrites the file, so the
					// observed inventory identity legitimately differs.
					got.Identity, baseline.Identity = Identity{}, Identity{}
				}
				if !reflect.DeepEqual(got, baseline) {
					t.Fatalf("%s differs:\n got  %+v\n want %+v", name, got, baseline)
				}
			}
			// Digests are published only for a settled audit every projection
			// agrees with, and then equal the bytes a fresh read returns.
			if (m.Physical.Files != nil) != tc.physical || m.Physical.Cleanup != nil {
				t.Fatalf("physical publication: %d files, cleanup %v", len(m.Physical.Files), m.Physical.Cleanup)
			}
			native := r.Source.(Native)
			for path, file := range m.Physical.Files {
				actual, err := native.Read(path, wire.MaxEvidenceBlobBytes)
				if err != nil || file.Bytes != len(actual) || file.Sha256 != wire.Sum(actual) {
					t.Fatalf("physical parity %s: %+v %v", path, file, err)
				}
			}
			if baseline.Err == "" && !baseline.Found {
				for name, set := range map[string][]string{
					"subset":    paths[:len(paths)-1],
					"superset":  append(append([]string{}, paths...), "intent/tickets/Z.json"),
					"unchecked": append(append([]string{}, paths...), "intent/../x"),
				} {
					if _, ok, _ := m.Canonical(set...); ok {
						t.Fatalf("%s selection reused", name)
					}
				}
			}
			if baseline.Found {
				if _, ok, _ := m.Canonical(paths...); ok {
					t.Fatal("found request reused for planning")
				}
			}
		})
	}
}
