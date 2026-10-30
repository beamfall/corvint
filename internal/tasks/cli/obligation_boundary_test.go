package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// boundarySet runs one ticket obligations set as actor with role.
func boundarySet(t *testing.T, root, id, req, actor, role, changes string) run {
	t.Helper()
	t.Setenv("CORVINT_TASKS_ACTOR", actor)
	defer t.Setenv("CORVINT_TASKS_ACTOR", "owner")
	return atm(t, root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, root, id),
		"--request-id", req, "--role", role, "--payload", `{"changes":`+changes+`}`)
}

// infraRepo is a separate Git repository with tracked code and no runnable
// test file; it returns its root.
func infraRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "infra")
	fixture.Write(t, filepath.Join(root, "deploy", "main.tf"), []byte("resource \"x\" \"y\" {}\n"))
	fixture.Write(t, filepath.Join(root, "deploy", "schema.sql"), []byte("create table t (id int);\n"))
	git(t, root, "init", "-q")
	git(t, root, "add", "deploy")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "infra")
	return mustEval(t, root)
}

func obligationsBoundaryRead(t *testing.T, root, id string) wire.Value {
	t.Helper()
	x := atm(t, root, nil, "ticket", "obligations", "boundary", "--target", id)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("obligations boundary: %s", x.stdout)
	}
	return x.res.Items[0]
}

// boundaryVerdicts maps each classified entry to verdict/reason.
func boundaryVerdicts(v wire.Value) map[string]string {
	out := map[string]string{}
	for _, e := range field(v, "entries").Arr {
		out[field(e, "id").Str] = field(e, "verdict").Str + "/" + field(e, "reason").Str
	}
	return out
}

func boundaryRepo(t *testing.T, v wire.Value, name string) wire.Value {
	t.Helper()
	for _, r := range field(v, "repositories").Arr {
		if (name == "" && field(r, "repository").Kind == wire.KindNull) || field(r, "repository").Str == name {
			return r
		}
	}
	t.Fatalf("no repository %q: %s", name, wire.Encode(v))
	return wire.Value{}
}

// TestTOLV0083_NoTestBoundaryScopeDecisionAndRescope: an obligation citing
// only code in a repository with no runnable test is NO_TEST_BOUNDARY with
// each repository's inventory; the scope decision refuses without a
// proposal, by a non-OWNER and by its proposer, and once accepted the
// completion counts exclude it and show lists it with its decision; adding
// a test file lists it as a re-scope candidate (TOL-V0-079..083, GitHub
// #1180).
func TestTOLV0083_NoTestBoundaryScopeDecisionAndRescope(t *testing.T) {
	infra := infraRepo(t)
	r, id, _ := obligationRepoWith(t, testRepoPolicy(map[string]string{"infra": infra}))
	seedObligations(t, r.Root, id)
	cites := `[{"id":"AC-1","state":null,"core":null,"reason":"cites the login spec","cites":[{"repository":null,"path":"e2e/login.spec.ts"}]},` +
		`{"id":"AC-2","state":null,"core":null,"reason":"cites the deploy module","cites":[{"repository":"infra","path":"deploy"}]},` +
		`{"id":"AC-4","state":null,"core":null,"reason":"cites a removed file","cites":[{"repository":"infra","path":"deploy/gone.tf"}]}]`
	if x := boundarySet(t, r.Root, id, "cites-1", "owner", "OWNER", cites); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("cites: %s", x.stdout)
	}
	v := obligationsBoundaryRead(t, r.Root, id)
	got := boundaryVerdicts(v)
	want := map[string]string{"AC-1": "TESTABLE/", "AC-2": "NO_TEST_BOUNDARY/NO_RUNNABLE_TESTS", "AC-4": "NO_TEST_BOUNDARY/NO_TRACKED_CODE", "AC-3": "UNKNOWN/NO_CITATIONS"}
	for k, w := range want {
		if got[k] != w {
			t.Fatalf("%s = %q, want %q: %s", k, got[k], w, wire.Encode(v))
		}
	}
	if strList(v, "noTestBoundary") != "AC-2,AC-4" {
		t.Fatalf("noTestBoundary: %s", wire.Encode(v))
	}
	if p := boundaryRepo(t, v, ""); field(p, "testFiles").Str != "1" || field(field(p, "kinds").Arr[0], "kind").Str != "javascript" {
		t.Fatalf("primary inventory: %s", wire.Encode(p))
	}
	if p := boundaryRepo(t, v, "infra"); field(p, "status").Str != "KNOWN" || field(p, "testFiles").Str != "0" || len(field(p, "examples").Arr) != 0 {
		t.Fatalf("infra inventory: %s", wire.Encode(p))
	}

	// Scope decision: no proposal, then a proposal by agent-1 that an
	// OPERATOR and the proposer itself cannot accept.
	accept := `[{"id":"AC-2","state":null,"core":null,"reason":"owner accepts","scope":{"action":"ACCEPT","core":false}}]`
	if x := boundarySet(t, r.Root, id, "acc-0", "owner", "OWNER", accept); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_SCOPE_NO_PROPOSAL:") {
		t.Fatalf("accept without a proposal: %s", x.stdout)
	}
	propose := `[{"id":"AC-2","state":null,"core":null,"reason":"no test provider in infra","scope":{"action":"PROPOSE","core":false,"evidence":"boundary report 2026-10-19"}}]`
	if x := boundarySet(t, r.Root, id, "prop-1", "agent-1", "OWNER", propose); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("propose: %s", x.stdout)
	}
	if ref := field(field(obligationsShow(t, r.Root, id), "reference"), "counts"); field(ref, "coreTotal").Str != "2" {
		t.Fatalf("a proposal changed core: %s", wire.Encode(ref))
	}
	if x := boundarySet(t, r.Root, id, "acc-op", "agent-1", "OPERATOR", accept); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "UNAUTHORIZED") {
		t.Fatalf("OPERATOR accept: %s", x.stdout)
	}
	if x := boundarySet(t, r.Root, id, "acc-self", "agent-1", "OWNER", accept); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_SCOPE_SELF_ACCEPT:") {
		t.Fatalf("self accept: %s", x.stdout)
	}
	if x := boundarySet(t, r.Root, id, "acc-1", "owner", "OWNER", accept); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("owner accept: %s", x.stdout)
	}
	if x := boundarySet(t, r.Root, id, "acc-2", "owner", "OWNER", accept); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_SCOPE_NO_PROPOSAL:") {
		t.Fatalf("second accept of an accepted decision: %s", x.stdout)
	}
	show := obligationsShow(t, r.Root, id)
	if c := field(field(show, "reference"), "counts"); field(c, "coreTotal").Str != "1" {
		t.Fatalf("accepted decision still counted in scope: %s", wire.Encode(c))
	}
	out := field(show, "outOfScope").Arr
	if len(out) != 1 || field(out[0], "id").Str != "AC-2" {
		t.Fatalf("outOfScope: %s", wire.Encode(show))
	}
	sc := field(out[0], "scope")
	if field(sc, "status").Str != "ACCEPTED" || field(field(sc, "proposedBy"), "id").Str != "agent-1" || field(field(sc, "acceptedBy"), "id").Str != "owner" ||
		field(sc, "reason").Str != "no test provider in infra" || field(sc, "evidence").Str != "boundary report 2026-10-19" || field(sc, "acceptedAt").Str == "" {
		t.Fatalf("scope decision: %s", wire.Encode(sc))
	}
	if len(field(obligationsBoundaryRead(t, r.Root, id), "rescopeCandidates").Arr) != 0 {
		t.Fatal("re-scope candidate before a test provider exists")
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("receipt audit is not consistent after a scope decision")
	}

	// A runnable test file appears in infra.
	fixture.Write(t, filepath.Join(infra, "deploy", "deploy_test.go"), []byte("package deploy\n"))
	git(t, infra, "add", "deploy")
	git(t, infra, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "first test")
	v = obligationsBoundaryRead(t, r.Root, id)
	rs := field(v, "rescopeCandidates").Arr
	if len(rs) != 1 || field(rs[0], "id").Str != "AC-2" || field(field(rs[0], "scope"), "status").Str != "ACCEPTED" {
		t.Fatalf("rescopeCandidates: %s", wire.Encode(v))
	}
	if p := boundaryRepo(t, v, "infra"); field(p, "testFiles").Str != "1" || field(p, "examples").Arr[0].Str != "deploy/deploy_test.go" {
		t.Fatalf("infra inventory after a test: %s", wire.Encode(p))
	}
}

// TestTOLV0082_RequireScopeDecisionRefusesPlainCore: under the opt-in
// policy a plain core change refuses APPROVAL_MISSING and the accepted
// scope decision still moves core; without the policy the plain change
// stays admitted (TOL-V0-082, TOL-V0-007).
func TestTOLV0082_RequireScopeDecisionRefusesPlainCore(t *testing.T) {
	plain := `[{"id":"AC-1","state":null,"core":false,"reason":"not core"}]`
	r0, id0, _ := obligationRepo(t)
	seedObligations(t, r0.Root, id0)
	if x := boundarySet(t, r0.Root, id0, "core-0", "owner", "OWNER", plain); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("plain core change without the policy: %s", x.stdout)
	}
	r, id, _ := obligationRepoWith(t, func(p wire.Value) {
		p.Obj.Set("obligations", wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(false)).Set("requireScopeDecision", wire.Bool(true))))
	})
	seedObligations(t, r.Root, id)
	x := boundarySet(t, r.Root, id, "core-1", "owner", "OWNER", plain)
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), wire.CodeApprovalMissing) || !strings.Contains(string(x.stdout), "OBLIGATION_SCOPE_DECISION_REQUIRED:") {
		t.Fatalf("plain core change under requireScopeDecision: %s", x.stdout)
	}
	if x := boundarySet(t, r.Root, id, "prop-1", "agent-1", "OWNER", `[{"id":"AC-1","state":null,"core":null,"reason":"r","scope":{"action":"PROPOSE","core":false,"evidence":"e"}}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("propose: %s", x.stdout)
	}
	if x := boundarySet(t, r.Root, id, "acc-1", "owner", "OWNER", `[{"id":"AC-1","state":null,"core":null,"reason":"ok","scope":{"action":"ACCEPT","core":false}}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("accept: %s", x.stdout)
	}
	if c := field(field(obligationsShow(t, r.Root, id), "reference"), "counts"); field(c, "coreTotal").Str != "1" {
		t.Fatalf("accepted decision did not move core: %s", wire.Encode(c))
	}
}

func worklistRead(t *testing.T, root string, args ...string) wire.Value {
	t.Helper()
	x := atm(t, root, nil, append([]string{"ticket", "obligations", "worklist"}, args...)...)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("obligations worklist: %s", x.stdout)
	}
	return x.res.Items[0]
}

func worklistIDs(v wire.Value, key string) string {
	var out []string
	for _, e := range field(v, key).Arr {
		s := field(e, "id").Str
		if r := field(e, "reason"); r.Kind == wire.KindString && key == "excluded" {
			s += ":" + r.Str
		}
		out = append(out, s)
	}
	return strings.Join(out, ",")
}

// TestTOLV0086_WorklistBatchCandidateAndNeedsText: two partial reviews
// citing one test file form one --next batch with their missing clauses,
// stub risks, line range and age; recording a candidate removes the item
// until it is reviewed again; a no-falsifiable-text finding is listed under
// needsObligationText (TOL-V0-084..088, GitHub #1182).
func TestTOLV0086_WorklistBatchCandidateAndNeedsText(t *testing.T) {
	defer cli.SetObligationWorklistNow(time.Date(2026, 10, 18, 13, 0, 0, 0, time.UTC))()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	empty := worklistRead(t, r.Root)
	if len(field(empty, "items").Arr) != 0 || strList(empty, "rank") != "priority,nearest" {
		t.Fatalf("worklist before any review: %s", wire.Encode(empty))
	}
	held := reviewFile(t, "2", "boundary", verdict("rev-a", "partial", "none", "misses the error branch"), verdict("rev-b", "correct", "on-path", "session store stubbed"))
	for i, ac := range []string{"AC-1", "AC-2"} {
		if x := reviewWitness(t, r.Root, id, "rv-"+ac, head, ac, held); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "HELD" {
			t.Fatalf("review %d: %s", i, x.stdout)
		}
	}
	noText := reviewFile(t, "2", "none", `{"reviewer":"rev-a","verdict":"partial","stubRisk":"none","reason":"the title says works","finding":"NO_FALSIFIABLE_TEXT"}`, verdict("rev-b", "correct", "none", "ok"))
	if x := reviewWitness(t, r.Root, id, "rv-AC-3", head, "AC-3", noText); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("no-text review: %s", x.stdout)
	}
	v := worklistRead(t, r.Root, "--target", id, "--next", "3")
	items := field(v, "items").Arr
	if len(items) != 2 || worklistIDs(v, "items") != "AC-1,AC-2" {
		t.Fatalf("batch: %s", wire.Encode(v))
	}
	it := items[0]
	if field(it, "verdict").Str != "partial" || field(it, "path").Str != "e2e/login.spec.ts" || field(it, "startLine").Str != "1" || field(it, "endLine").Str != "3" ||
		field(it, "reviewAgeDays").Str != "10" || field(it, "reviewedAt").Str != "2026-10-08T13:00:00Z" || field(it, "testRepository").Kind != wire.KindNull ||
		len(field(it, "missing").Arr) != 1 || field(field(it, "missing").Arr[0], "reason").Str != "misses the error branch" || strings.Join(stringsOf(field(it, "stubRisks")), "|") != "rev-b: on-path" {
		t.Fatalf("item: %s", wire.Encode(it))
	}
	if worklistIDs(v, "needsObligationText") != "AC-3" || strings.Contains(worklistIDs(v, "items"), "AC-3") {
		t.Fatalf("needsObligationText: %s", wire.Encode(v))
	}
	if x := atm(t, r.Root, nil, "ticket", "obligations", "worklist", "--rank", "priority,bogus"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("unknown rank key admitted: %s", x.stdout)
	}

	// A new witness candidate removes AC-1 until it is reviewed again.
	cand := `[{"id":"AC-1","state":null,"core":null,"reason":"new example","candidate":{"path":"e2e/login.spec.ts","startLine":"2","endLine":"2"}}]`
	if x := boundarySet(t, r.Root, id, "cand-1", "owner", "OWNER", cand); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("candidate: %s", x.stdout)
	}
	v = worklistRead(t, r.Root, "--target", id)
	if worklistIDs(v, "items") != "AC-2" || worklistIDs(v, "excluded") != "AC-1:CANDIDATE_AWAITING_REVIEW" {
		t.Fatalf("worklist with a candidate: %s", wire.Encode(v))
	}
	if x := reviewWitness(t, r.Root, id, "rv-AC-1b", head, "AC-1", held); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("re-review: %s", x.stdout)
	}
	v = worklistRead(t, r.Root, "--target", id)
	if worklistIDs(v, "items") != "AC-1,AC-2" || worklistIDs(v, "excluded") != "" {
		t.Fatalf("worklist after re-review: %s", wire.Encode(v))
	}
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		if field(e, "id").Str == "AC-1" && wire.Has(e, "candidate") {
			t.Fatalf("review left the candidate: %s", wire.Encode(e))
		}
	}

	// The reviewed example changes at HEAD: both items leave the queue until
	// a new review covers the new bytes.
	fixture.Write(t, filepath.Join(r.Root, "e2e", "login.spec.ts"), []byte(obligationSpec+"// changed\n"))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "change spec")
	v = worklistRead(t, r.Root, "--target", id)
	if worklistIDs(v, "items") != "" || worklistIDs(v, "excluded") != "AC-1:SUBJECT_CHANGED,AC-2:SUBJECT_CHANGED" {
		t.Fatalf("worklist after the subject changed: %s", wire.Encode(v))
	}
}

// TestTOLV0085_FindingNeedsShortfallVerdict: a no-falsifiable-text finding
// on a correct verdict is refused (TOL-V0-085).
func TestTOLV0085_FindingNeedsShortfallVerdict(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	bad := reviewFile(t, "2", "none", `{"reviewer":"rev-a","verdict":"correct","stubRisk":"none","reason":"r","finding":"NO_FALSIFIABLE_TEXT"}`, verdict("rev-b", "correct", "none", "ok"))
	if x := reviewWitness(t, r.Root, id, "rv-bad", head, "AC-1", bad); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("finding on a correct verdict admitted: %s", x.stdout)
	}
	empty := reviewFile(t, "2", "none", `{"reviewer":"rev-a","verdict":"correct","stubRisk":"none","reason":"r","finding":""}`, verdict("rev-b", "correct", "none", "ok"))
	if x := reviewWitness(t, r.Root, id, "rv-empty", head, "AC-1", empty); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "finding") {
		t.Fatalf("empty finding admitted: %s", x.stdout)
	}
}

// TestTOLV0081_GrantedOperatorCannotAccept: an OPERATOR row that explicitly
// grants OBLIGATIONS_SET admits a proposal, but ACCEPT stays OWNER-only even
// from an actor other than the proposer (TOL-V0-081).
func TestTOLV0081_GrantedOperatorCannotAccept(t *testing.T) {
	r, id, _ := obligationRepoWith(t, func(p wire.Value) {
		p.Obj.Set("roles", wire.ObjectValue(wire.NewObject().Set("OPERATOR", wire.Strings([]string{"OBLIGATIONS_SET"}))))
	})
	seedObligations(t, r.Root, id)
	propose := `[{"id":"AC-2","state":null,"core":null,"reason":"no test provider","scope":{"action":"PROPOSE","core":false,"evidence":"boundary report"}}]`
	if x := boundarySet(t, r.Root, id, "prop-op", "agent-1", "OPERATOR", propose); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("granted OPERATOR propose: %s", x.stdout)
	}
	accept := `[{"id":"AC-2","state":null,"core":null,"reason":"accept","scope":{"action":"ACCEPT","core":false}}]`
	if x := boundarySet(t, r.Root, id, "acc-op", "agent-2", "OPERATOR", accept); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "TOL-V0-081") {
		t.Fatalf("granted OPERATOR accept: %s", x.stdout)
	}
	if x := boundarySet(t, r.Root, id, "acc-owner", "owner", "OWNER", accept); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("owner accept: %s", x.stdout)
	}
}

// TestTOLV0086_WorklistTestRepositoryUnverified: an item reviewed from a
// declared test repository carries that repository, and a test checkout
// with no HEAD commit leaves it SUBJECT_UNVERIFIED, never SUBJECT_CHANGED
// (TOL-V0-086; Codex finding on this change).
func TestTOLV0086_WorklistTestRepositoryUnverified(t *testing.T) {
	app, appHead := separateTestRepo(t)
	r, id, _ := obligationRepoWith(t, testRepoPolicy(map[string]string{"app": app}))
	seedObligations(t, r.Root, id)
	held := reviewFile(t, "1", "none", verdict("rev-a", "partial", "none", "misses the error branch"))
	if x := reviewWitness(t, r.Root, id, "rv-app", appHead, "AC-1", held, "--test-repository", "app"); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "HELD" {
		t.Fatalf("test-repository review: %s", x.stdout)
	}
	v := worklistRead(t, r.Root, "--target", id)
	if worklistIDs(v, "items") != "AC-1" || field(field(v, "items").Arr[0], "testRepository").Str != "app" {
		t.Fatalf("test-repository item: %s", wire.Encode(v))
	}
	if err := os.Rename(app, app+".moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(app, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, app, "init", "-q")
	v = worklistRead(t, r.Root, "--target", id)
	if len(field(v, "items").Arr) != 0 || worklistIDs(v, "excluded") != "AC-1:SUBJECT_UNVERIFIED" {
		t.Fatalf("unborn test repository: %s", wire.Encode(v))
	}
}

func stringsOf(v wire.Value) []string {
	var out []string
	for _, x := range v.Arr {
		out = append(out, x.Str)
	}
	return out
}
