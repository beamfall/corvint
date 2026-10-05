package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// acceptGate is an executable COMMAND gate that prints the report file, the
// way a G2 gate runs `corvint tests accept` and captures its report.
func acceptGate(report string) wire.Value {
	expected := wire.NewObject().Set("exitCode", wire.String("0")).Set("reducer", wire.Null())
	return wire.ObjectValue(wire.NewObject().Set("gateId", wire.String("accept")).Set("kind", wire.String("COMMAND")).
		Set("argv", wire.Strings([]string{"/bin/cat", report})).Set("cwd", wire.String("WORKTREE")).Set("env", wire.Strings(nil)).
		Set("timeoutSeconds", wire.String("30")).Set("expected", wire.ObjectValue(expected)).Set("evidence", wire.Strings(nil)).
		Set("inputs", wire.Strings(nil)).Set("sharedResource", wire.Null()).Set("reusable", wire.Bool(false)).Set("required", wire.Bool(false)))
}

// TestERGV0002_EvidenceCandidatesNeedAnArtifactLink drives EVIDENCE
// candidates, evidence references and the issue 394 producer mapping through
// the native writer (ERG-V0-002, ERG-V0-010). A caller digest, an
// acceptance report and an evidence reference refuse until an executable
// gate run of the subject attempt retained exactly those bytes; then
// `--from-acceptance` records rejected as RETURN and accepted as PASS on the
// report's EVIDENCE candidate, a blocked report supports no verdict, and a
// verdict that contradicts the linked report refuses in the writer, not
// only in the CLI. The workState exposes the EVIDENCE candidate, and the
// receipt audit replays every event against its link.
func TestERGV0002_EvidenceCandidatesNeedAnArtifactLink(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gated := filepath.Join(dir, "gate-output.json")
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, acceptGate(gated))
	id := planTicket(t, root, "accepted", "P1", `["src/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	refused := func(why, code string, args ...string) {
		t.Helper()
		x := atm(t, root, nil, args...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, code) {
			t.Fatalf("%s was not refused with %s: %s", why, code, x.stdout)
		}
	}
	report := func(name, verdict string) (path, artifact string) {
		t.Helper()
		raw := []byte(`{"schema":"corvint-new-e2e-assessment/1","verdict":"` + verdict + `","reasons":["provider-freshness-unknown"]}` + "\n")
		path = filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return path, string(wire.Sum(raw)) + ":" + strconv.Itoa(len(raw))
	}

	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-a")
	subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	// The store's checkout holds untracked queue files, so the gate runs in
	// a clean linked worktree at the candidate.
	clean := filepath.Join(dir, "clean")
	git(t, root, "worktree", "add", "--detach", clean, "HEAD")
	gateRun := func(req, path string) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err == nil {
			err = os.WriteFile(gated, raw, 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
		runOK("gate", "run", "--attempt", attempt, "--generation", gen, "--gate", "accept", "--request-id", req, "--worktree", clean)
	}
	record := func(req, generation, revision string, extra ...string) []string {
		return append([]string{"gate", "record", id, "--gate", ergGate, "--subject-receipt", subject,
			"--expected-generation", generation, "--expected-revision", revision, "--request-id", req}, extra...)
	}

	rejected, rejectedArtifact := report("rejected", "rejected")
	accepted, acceptedArtifact := report("accepted", "accepted")
	blocked, blockedArtifact := report("blocked", "blocked")
	why := "--reason"
	refused("an unlinked acceptance report", wire.CodeMissingEvidence, record("early", "0", "0", "--from-acceptance", rejected, why, "TESTS:surviving control")...)
	refused("a caller-supplied candidate digest", wire.CodeMissingEvidence, record("caller", "0", "0", "--verdict", "RETURN", "--candidate-evidence", rejectedArtifact, why, "TESTS:surviving control")...)
	refused("an unlinked evidence reference", wire.CodeMissingEvidence, record("ref", "0", "0", "--verdict", "RETURN", "--evidence", "report="+rejectedArtifact, why, "TESTS:surviving control")...)
	plain := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(plain, []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused("a non-report file", wire.CodeMalformed, record("plain", "0", "0", "--from-acceptance", plain)...)

	gateRun("run-rejected", rejected)
	refused("a PASS on a linked rejected report", wire.CodeMalformed, record("contradict", "0", "0", "--verdict", "PASS", "--candidate-evidence", rejectedArtifact)...)
	refused("--verdict conflicting with --from-acceptance", wire.CodeMalformed, record("conflict", "0", "0", "--verdict", "PASS", "--from-acceptance", rejected)...)
	runOK(record("g2-return", "0", "0", "--from-acceptance", rejected, why, "TESTS:surviving control")...)
	v, state := ergGateView(t, root, id)
	sha, n := rejectedArtifact[:64], rejectedArtifact[65:]
	if state != dispatch.GateReturn || v.Candidate != (dispatch.GateCandidate{Kind: "EVIDENCE", Sha256: sha, Bytes: n}) || v.Subject.ReceiptSeq != subject {
		t.Fatalf("RETURN on the linked rejected report: %s %+v", state, v)
	}
	items := runOK("gate", "state", id).res.Items
	candidate, _ := items[0].Obj.Get("candidate")
	if field(candidate, "kind").Str != "EVIDENCE" || field(candidate, "sha256").Str != sha || field(candidate, "bytes").Str != n {
		t.Fatalf("gate state candidate: %s", wire.EncodeFile(items[0]))
	}

	gateRun("run-blocked", blocked)
	refused("a blocked report through the CLI", wire.CodeMissingEvidence, record("blocked-cli", "1", "1", "--from-acceptance", blocked)...)
	refused("a verdict on a linked blocked report", wire.CodeMissingEvidence, record("blocked-writer", "1", "1", "--verdict", "PASS", "--candidate-evidence", blockedArtifact)...)

	// A new report is a new candidate: the RETURN on the old one needs the
	// author's explicit resubmission on the new linked artifact (ERG-V0-006).
	gateRun("run-accepted", accepted)
	resubmit := func(req, artifact string) []string {
		return []string{"gate", "resubmit", id, "--gate", ergGate, "--author-attempt", attempt, "--subject-receipt", subject,
			"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:control now killed", "--request-id", req,
			"--candidate-evidence", artifact}
	}
	refused("a resubmission on an unlinked artifact", wire.CodeMissingEvidence, resubmit("g2-resubmit-x", string(wire.Sum([]byte("x")))+":1")...)
	runOK(resubmit("g2-resubmit", acceptedArtifact)...)
	pass := record("g2-pass", "2", "2", "--from-acceptance", accepted, "--evidence", "earlier="+rejectedArtifact, "--issued-at", "2026-10-05T12:00:00Z")
	runOK(pass...)
	v, state = ergGateView(t, root, id)
	if state != dispatch.GatePass || v.Generation != "2" || v.Revision != "3" || v.Candidate.Sha256 != acceptedArtifact[:64] {
		t.Fatalf("PASS on the linked accepted report: %s %+v", state, v)
	}
	if x := atm(t, root, nil, pass...); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeCompleted {
		t.Fatalf("an identical retry did not replay: %s", x.stdout)
	}
	refused("a retry with another candidate", wire.CodeRequestIDConflict, record("g2-pass", "2", "2", "--verdict", "PASS", "--evidence", "earlier="+rejectedArtifact, "--issued-at", "2026-10-05T12:00:00Z")...)
	runOK("receipt", "audit")
}

// TestERGV0002_ForgedArtifactEventsRefuseAtRecovery covers the crash
// boundary and the settled read for artifact links (ERG-V0-002, -010): a
// rehashed review event that names an unlinked EVIDENCE candidate, an
// unlinked evidence reference, or a verdict its linked acceptance report
// does not support refuses redo as JOURNAL_FORKED with the projection
// unchanged, and once settled fails the receipt audit and leaves the
// dispatcher's gates unobserved. The receipt fold, not only the writer,
// enforces the link and the issue 394 mapping.
func TestERGV0002_ForgedArtifactEventsRefuseAtRecovery(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	// The unlinked artifact is retained under evidence/, so only the missing
	// link, not missing bytes, refuses it.
	unlinkedRaw := []byte("never produced by a gate run")
	unlinked := wire.Sum(unlinkedRaw)
	cases := []struct {
		name  string
		forge func(*snapshot.ExternalReviewEvent)
	}{
		{"control", nil},
		{"unlinked-candidate", func(e *snapshot.ExternalReviewEvent) {
			e.Request.Candidate = snapshot.ExternalReviewCandidate{Kind: "EVIDENCE", Sha256: unlinked, Bytes: "28"}
		}},
		{"unlinked-evidence", func(e *snapshot.ExternalReviewEvent) {
			e.Request.Evidence = append(e.Request.Evidence, snapshot.GateEvidence{Label: "forged", Sha256: unlinked, Bytes: "28"})
		}},
		{"contradicted-verdict", func(e *snapshot.ExternalReviewEvent) {
			pass := "PASS"
			e.Request.Verdict, e.Request.Reasons = &pass, []snapshot.ExternalReviewReason{}
		}},
	}
	for _, tc := range cases {
		for _, settled := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/pending", true: "/settled"}[settled], func(t *testing.T) {
				root, tree := ergStore(t)
				// A gate argv entry is at most 128 bytes, which a subtest's
				// TempDir path can exceed.
				dir, err := os.MkdirTemp("", "erg")
				if err == nil {
					t.Cleanup(func() { os.RemoveAll(dir) })
					dir, err = filepath.EvalSymlinks(dir)
				}
				if err != nil {
					t.Fatal(err)
				}
				gated := filepath.Join(dir, "gate-output.json")
				raw := []byte(`{"schema":"corvint-new-e2e-assessment/1","verdict":"rejected","reasons":["provider-freshness-unknown"]}` + "\n")
				if err := os.WriteFile(gated, raw, 0o600); err != nil {
					t.Fatal(err)
				}
				ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, acceptGate(gated))
				id := planTicket(t, root, "accepted", "P1", `["src/"]`)
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
				fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(unlinked)), unlinkedRaw)
				c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
				attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
				runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-a")
				subject := headSeq()
				clean := filepath.Join(dir, "clean")
				git(t, root, "worktree", "add", "--detach", clean, "HEAD")
				runOK("gate", "run", "--attempt", attempt, "--generation", gen, "--gate", "accept", "--request-id", "run-rejected", "--worktree", clean)

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
				report := filepath.Join(dir, "report.json")
				if err := os.WriteFile(report, raw, 0o600); err != nil {
					t.Fatal(err)
				}
				runOK("gate", "record", id, "--gate", ergGate, "--subject-receipt", subject, "--expected-generation", "0", "--expected-revision", "0",
					"--request-id", "g2-return", "--from-acceptance", report, "--reason", "TESTS:surviving control", "--issued-at", "2026-10-05T12:00:00Z")
				seq, err := wire.ParseCount("headSeq", headSeq())
				if err != nil {
					t.Fatal(err)
				}
				var forged []byte
				if tc.forge != nil {
					forged = ergReceiptForge(t, repo, uint64(seq.Int()), tc.forge)
				}

				if settled {
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
					if tc.forge == nil && err != nil {
						t.Fatal(err)
					}
					for _, x := range tickets {
						if x.ID == id && (tc.forge == nil) != (x.GatesObserved && x.GateState(ergGate) == dispatch.GateReturn) {
							t.Fatalf("settled observation: %+v", x)
						}
					}
					if x := atm(t, root, nil, "receipt", "audit"); (x.res.Outcome == wire.OutcomeOK) != (tc.forge == nil) {
						t.Fatalf("receipt audit: %s", x.stdout)
					}
					return
				}

				fixture.Write(t, headPath, preHead)
				fixture.Write(t, ticketPath, preTicket)
				x := atm(t, root, nil, "ticket", "create", "--request-id", "trigger", "--issued-at", "2026-10-05T12:02:00Z", "--payload",
					strings.Replace(createPayloadJSON, `"title":"Console ticket"`, `"title":"trigger"`, 1))
				if tc.forge == nil {
					if x.res.Outcome != wire.OutcomeOK {
						t.Fatalf("control redo: %s", x.stdout)
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
			})
		}
	}
}
