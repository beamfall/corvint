package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ergReceiptForge rewrites receipt seq so that it posts forged instead of
// its review event: the forged event is published under its own digest, and
// the receipt's evidence post and ticket post (externalReviews head) are
// rehashed to name it. Every byte stays self-consistent; only the review
// transition is untrue. It returns the forged receipt bytes.
func ergReceiptForge(t *testing.T, repo *intent.Repository, seq uint64, forge func(*snapshot.ExternalReviewEvent)) []byte {
	t.Helper()
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.StateDir, "receipts", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rv, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	posts, _ := rv.Obj.Get("post")
	var old, forged wire.Digest
	var counters [2]wire.Count
	for _, p := range posts.Arr {
		at, _ := p.Obj.Get("path")
		if !strings.HasPrefix(at.Str, "evidence/") {
			continue
		}
		old = wire.Digest(strings.TrimPrefix(at.Str, "evidence/"))
		event, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(old)))
		if err != nil {
			t.Fatal(err)
		}
		e, err := snapshot.CanonicalExternalReviewEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		forge(e)
		counters = [2]wire.Count{e.ReviewGeneration, e.EventRevision}
		event, err = e.Encode()
		if err != nil {
			t.Fatal(err)
		}
		forged = wire.Sum(event)
		fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(forged)), event)
		p.Obj.Set("path", wire.String("evidence/"+string(forged))).Set("sha256", wire.String(string(forged))).Set("blobSha256", wire.String(string(forged)))
	}
	if old == "" {
		t.Fatalf("receipt %d posts no review event", seq)
	}
	pres, _ := rv.Obj.Get("pre")
	for _, p := range pres.Arr {
		if at, _ := p.Obj.Get("path"); at.Str == "evidence/"+string(old) {
			p.Obj.Set("path", wire.String("evidence/"+string(forged)))
		}
	}
	for _, p := range posts.Arr {
		at, _ := p.Obj.Get("path")
		if !strings.HasPrefix(at.Str, "intent/tickets/") {
			continue
		}
		rec, _ := p.Obj.Get("record")
		reviews, _ := rec.Obj.Get("externalReviews")
		for _, gate := range reviews.Obj.SortedKeys() {
			ref, _ := reviews.Obj.Get(gate)
			if head, _ := ref.Obj.Get("head"); head.Str == string(old) {
				ref.Obj.Set("head", wire.String(string(forged))).Set("generation", wire.String(string(counters[0]))).Set("revision", wire.String(string(counters[1])))
			}
		}
		p.Obj.Set("sha256", wire.String(string(wire.Sum(wire.EncodeFile(rec)))))
	}
	out := wire.EncodeFile(rv)
	fixture.Write(t, path, out)
	return out
}

// TestERGV0009_ForgedReviewEventsRefuseAtRecovery covers the crash boundary
// and the dispatcher read: a rehashed review event whose actor, counters,
// prior RETURN, subject generation, candidate tree or reviewer lease do not
// reproduce its receipt's transition refuses redo as JOURNAL_FORKED with the
// projection unchanged, and once settled leaves the dispatcher's gates
// unobserved rather than actionable (ERG-V0-006/-009). The lease cases run
// under a policy that requires a reviewer lease: a lease-bound record is the
// control, and stripping its lease to an operator attestation is a forgery.
func TestERGV0009_ForgedReviewEventsRefuseAtRecovery(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	cases := []struct {
		name  string
		forge func(*snapshot.ExternalReviewEvent)
		lease bool
	}{
		{"control", nil, false},
		{"actor", func(e *snapshot.ExternalReviewEvent) { e.ActorID = "someone-else" }, false},
		// Self-consistent request and event counters that skip the prior
		// reference (1,1): the ticket reference is rewritten to match.
		{"counters", func(e *snapshot.ExternalReviewEvent) {
			e.Request.ExpectedGeneration, e.Request.ExpectedRevision = "2", "2"
			e.ReviewGeneration, e.EventRevision = "2", "3"
		}, false},
		{"prior-return", func(e *snapshot.ExternalReviewEvent) {
			d := wire.Sum([]byte("not the prior RETURN"))
			e.Request.PriorReturn = &d
		}, false},
		// The subject's generation and TREE candidate must be the retained
		// BUILT submission's, not whatever the request states.
		{"subject-generation", func(e *snapshot.ExternalReviewEvent) { e.Request.Subject.Generation = "7" }, false},
		{"candidate-tree", func(e *snapshot.ExternalReviewEvent) { e.Request.Candidate.TreeOID = strings.Repeat("ab", 20) }, false},
		{"lease-control", nil, true},
		{"dropped-lease", func(e *snapshot.ExternalReviewEvent) {
			e.Request.ReviewerLease, e.TrustSource = nil, "OPERATOR_ATTESTED"
		}, true},
	}
	for _, tc := range cases {
		for _, settled := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/pending", true: "/settled"}[settled], func(t *testing.T) {
				root, tree := ergStore(t)
				if tc.lease {
					ergPolicyUpdateLease(t, root, "3", []string{"implement"}, []string{"implement"}, true)
				}
				id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
				local, err := wire.ParseTicketID("", id)
				if err != nil {
					t.Fatal(err)
				}
				repo, err := intent.Resolve(root)
				if err != nil {
					t.Fatal(err)
				}
				runOK := func(args ...string) run {
					t.Helper()
					r := atm(t, root, nil, args...)
					if r.res.Outcome != wire.OutcomeOK {
						t.Fatalf("%v: %s", args, r.stdout)
					}
					return r
				}
				headSeq := func() string {
					t.Helper()
					return field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
				}
				c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
				attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
				runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
				subject := headSeq()
				target := []string{"gate", "record", id, "--gate", ergGate, "--verdict", "RETURN", "--subject-receipt", subject,
					"--expected-generation", "0", "--expected-revision", "0", "--request-id", "review-1", "--reason", "TESTS:missing case",
					"--issued-at", "2026-10-04T12:00:00Z"}
				if tc.lease {
					target = append(target, "--reviewer-attempt", attempt)
				}
				switch tc.name {
				case "counters":
					runOK(target...)
					target = []string{"gate", "record", id, "--gate", ergGate, "--verdict", "PASS", "--subject-receipt", subject,
						"--expected-generation", "1", "--expected-revision", "1", "--request-id", "review-2", "--issued-at", "2026-10-04T12:01:00Z"}
				case "prior-return":
					runOK(target...)
					target = []string{"gate", "resubmit", id, "--gate", ergGate, "--author-attempt", attempt, "--subject-receipt", subject,
						"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:added the case", "--request-id", "resubmit-1",
						"--issued-at", "2026-10-04T12:01:00Z"}
				}
				_, before := ergGateView(t, root, id)
				headPath := filepath.Join(repo.StateDir, "head.json")
				ticketPath := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", local.Local+".json")
				preHead, err := os.ReadFile(headPath)
				if err != nil {
					t.Fatal(err)
				}
				preTicket, err := os.ReadFile(ticketPath)
				if err != nil {
					t.Fatal(err)
				}
				runOK(target...)
				seq, err := wire.ParseCount("headSeq", headSeq())
				if err != nil {
					t.Fatal(err)
				}
				forged := []byte(nil)
				if tc.forge != nil {
					forged = ergReceiptForge(t, repo, uint64(seq.Int()), tc.forge)
				}

				if settled {
					// The settled store names the forged receipt from its head
					// and projects the forged ticket record.
					if forged != nil {
						editJSONFile(t, headPath, func(v wire.Value) { v.Obj.Set("lastReceiptSha256", wire.String(string(wire.Sum(forged)))) })
						rv, _ := wire.Parse(forged)
						posts, _ := rv.Obj.Get("post")
						for _, p := range posts.Arr {
							if at, _ := p.Obj.Get("path"); strings.HasPrefix(at.Str, "intent/tickets/") {
								rec, _ := p.Obj.Get("record")
								fixture.Write(t, ticketPath, wire.EncodeFile(rec))
							}
						}
					}
					tickets, err := cli.ObserveTickets(root)
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, x := range tickets {
						if x.ID != id {
							continue
						}
						found = true
						if tc.forge == nil && (!x.GatesObserved || x.GateState(ergGate) == dispatch.GateNone) {
							t.Fatalf("control: the real verdict is not observed: %+v", x)
						}
						if tc.forge != nil && x.GatesObserved {
							t.Fatalf("a forged settled verdict is observable: %+v", x)
						}
					}
					if !found {
						t.Fatalf("the observation lost ticket %s", id)
					}
					if x := atm(t, root, nil, "receipt", "audit"); (x.res.Outcome == wire.OutcomeOK) != (tc.forge == nil) {
						t.Fatalf("receipt audit: %s", x.stdout)
					}
					return
				}

				// Crash after the receipt: rewind head and the projection.
				fixture.Write(t, headPath, preHead)
				fixture.Write(t, ticketPath, preTicket)
				// Reads refuse a pending receipt until a writer redoes it.
				if _, err := cli.ObserveTickets(root); wire.CodeOf(err) != wire.CodeRedoPending {
					t.Fatalf("pending observation: %v", err)
				}
				// Reads refuse while a receipt is pending, so the next writer
				// (an unrelated create) is the one that redoes it.
				x := atm(t, root, nil, "ticket", "create", "--request-id", "trigger", "--issued-at", "2026-10-04T12:02:00Z", "--payload",
					strings.Replace(createPayloadJSON, `"title":"Console ticket"`, `"title":"trigger"`, 1))
				if tc.forge == nil {
					if x.res.Outcome != wire.OutcomeOK {
						t.Fatalf("control redo: %s", x.stdout)
					}
					if _, state := ergGateView(t, root, id); state != dispatch.GateReturn {
						t.Fatalf("control: the redone verdict is not observed: %s", state)
					}
					return
				}
				if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeJournalForked) || !bytes.Contains(x.stdout, []byte("external review binding")) {
					t.Fatalf("a forged pending event was redone: %s", x.stdout)
				}
				for path, want := range map[string][]byte{headPath: preHead, ticketPath: preTicket} {
					if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
						t.Fatalf("%s changed after the refused redo: %v", path, err)
					}
				}
				if tickets, err := cli.ObserveTickets(root); err == nil {
					for _, x := range tickets {
						if x.ID == id && x.GatesObserved && x.GateState(ergGate) != before {
							t.Fatalf("the forged verdict became actionable: %s", x.GateState(ergGate))
						}
					}
				}
			})
		}
	}
}

func editJSONFile(t *testing.T, path string, edit func(wire.Value)) {
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
