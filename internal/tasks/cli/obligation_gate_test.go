package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// withObligations applies base and then sets extra members on the policy's
// obligations object.
func withObligations(base func(wire.Value), set func(o *wire.Object)) func(wire.Value) {
	return func(p wire.Value) {
		base(p)
		set(field(p, "obligations").Obj)
	}
}

func commitTree(t *testing.T, dir, msg string) {
	t.Helper()
	git(t, dir, "add", "-A")
	git(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", msg)
}

// TestTOLV0101_ReviewedSubjectAtReviewedCommit: the worklist and boundary
// resolve a reviewed test file at the commit the review names, not at HEAD:
// a later edit leaves the subject RESOLVED at the reviewed commit; a
// repository that lacks that commit is REVIEWED_COMMIT_UNAVAILABLE and one
// that no longer resolves is TEST_REPOSITORY_UNRESOLVED (TOL-V0-101).
func TestTOLV0101_ReviewedSubjectAtReviewedCommit(t *testing.T) {
	app, appHead := separateTestRepo(t)
	r, id, _ := obligationRepoWith(t, testRepoPolicy(map[string]string{"app": app}))
	seedObligations(t, r.Root, id)
	held := reviewFile(t, "1", "none", verdict("rev-a", "partial", "none", "misses the error branch"))
	if x := reviewWitness(t, r.Root, id, "rv-app", appHead, "AC-1", held, "--test-repository", "app"); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "HELD" {
		t.Fatalf("test-repository review: %s", x.stdout)
	}
	subject := func(s wire.Value, status, reason string) {
		t.Helper()
		if field(s, "status").Str != status || field(s, "reason").Str != reason || field(s, "commit").Str != appHead ||
			field(s, "repository").Str != "app" || field(s, "path").Str != "e2e/login.spec.ts" {
			t.Fatalf("reviewedSubject want %s/%s: %s", status, reason, wire.Encode(s))
		}
	}
	v := worklistRead(t, r.Root, "--target", id)
	if worklistIDs(v, "items") != "AC-1" {
		t.Fatalf("worklist: %s", wire.Encode(v))
	}
	subject(field(field(v, "items").Arr[0], "reviewedSubject"), "RESOLVED", "")
	for _, e := range field(obligationsBoundaryRead(t, r.Root, id), "entries").Arr {
		if field(e, "id").Str == "AC-1" {
			subject(field(e, "reviewedSubject"), "RESOLVED", "")
		} else if wire.Has(e, "reviewedSubject") {
			t.Fatalf("unreviewed boundary entry has a subject: %s", wire.Encode(e))
		}
	}

	fixture.Write(t, filepath.Join(app, "e2e", "login.spec.ts"), []byte(obligationSpec+"// edited after review\n"))
	commitTree(t, app, "edit")
	v = worklistRead(t, r.Root, "--target", id)
	if worklistIDs(v, "excluded") != "AC-1:SUBJECT_CHANGED" {
		t.Fatalf("edited at HEAD: %s", wire.Encode(v))
	}
	subject(field(field(v, "excluded").Arr[0], "reviewedSubject"), "RESOLVED", "")

	if err := os.Rename(app, app+".old"); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(app, "e2e", "login.spec.ts"), []byte(obligationSpec))
	git(t, app, "init", "-q")
	commitTree(t, app, "unrelated history")
	v = worklistRead(t, r.Root, "--target", id)
	if worklistIDs(v, "excluded") != "AC-1:SUBJECT_CHANGED" && worklistIDs(v, "excluded") != "AC-1:SUBJECT_UNVERIFIED" && worklistIDs(v, "items") != "AC-1" {
		t.Fatalf("unrelated history: %s", wire.Encode(v))
	}
	var s wire.Value
	if len(field(v, "items").Arr) == 1 {
		s = field(field(v, "items").Arr[0], "reviewedSubject")
	} else {
		s = field(field(v, "excluded").Arr[0], "reviewedSubject")
	}
	subject(s, "UNAVAILABLE", "REVIEWED_COMMIT_UNAVAILABLE")

	if err := os.RemoveAll(app); err != nil {
		t.Fatal(err)
	}
	v = worklistRead(t, r.Root, "--target", id)
	var all []wire.Value
	all = append(append(all, field(v, "items").Arr...), field(v, "excluded").Arr...)
	if len(all) != 1 {
		t.Fatalf("unresolved repository: %s", wire.Encode(v))
	}
	subject(field(all[0], "reviewedSubject"), "UNAVAILABLE", "TEST_REPOSITORY_UNRESOLVED")
}

// TestTOLV0102_DeclaredCitesRequired: by default (requireDeclaredCites
// absent, owner decision 2026-10-19, V1-1963) and with an explicit true, a
// cite naming an undeclared repository is refused
// OBLIGATION_CITE_REPOSITORY_UNDECLARED and writes nothing, while declared
// and store-own (null repository) cites are admitted; an explicit false
// opts out to the TOL-V0-079 behavior that admits it (TOL-V0-102).
func TestTOLV0102_DeclaredCitesRequired(t *testing.T) {
	infra := infraRepo(t)
	bad := `[{"id":"AC-2","state":null,"core":null,"reason":"typo","cites":[{"repository":"infar","path":"deploy"}]}]`
	good := `[{"id":"AC-1","state":null,"core":null,"reason":"primary","cites":[{"repository":null,"path":"e2e/login.spec.ts"}]},` +
		`{"id":"AC-2","state":null,"core":null,"reason":"declared","cites":[{"repository":"infra","path":"deploy"}]}]`
	for _, tc := range []struct {
		name   string
		policy func(wire.Value)
	}{
		{"absent", testRepoPolicy(map[string]string{"infra": infra})},
		{"true", withObligations(testRepoPolicy(map[string]string{"infra": infra}), func(o *wire.Object) {
			o.Set("requireDeclaredCites", wire.Bool(true))
		})},
	} {
		r, id, _ := obligationRepoWith(t, tc.policy)
		seedObligations(t, r.Root, id)
		before := fixture.TreeSnapshot(t, r.StateDir)
		if x := boundarySet(t, r.Root, id, "cite-bad", "owner", "OWNER", bad); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_CITE_REPOSITORY_UNDECLARED:") {
			t.Fatalf("%s: undeclared cite admitted: %s", tc.name, x.stdout)
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
			t.Fatalf("%s: a refused cite wrote state", tc.name)
		}
		if x := boundarySet(t, r.Root, id, "cite-good", "owner", "OWNER", good); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%s: declared cites: %s", tc.name, x.stdout)
		}
	}

	r2, id2, _ := obligationRepoWith(t, withObligations(testRepoPolicy(map[string]string{"infra": infra}), func(o *wire.Object) {
		o.Set("requireDeclaredCites", wire.Bool(false))
	}))
	seedObligations(t, r2.Root, id2)
	if x := boundarySet(t, r2.Root, id2, "cite-bad", "owner", "OWNER", bad); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("explicit false refused an undeclared cite: %s", x.stdout)
	}
	if got := boundaryVerdicts(obligationsBoundaryRead(t, r2.Root, id2))["AC-2"]; got != "UNKNOWN/REPOSITORY_UNAVAILABLE" {
		t.Fatalf("undeclared cite verdict: %q", got)
	}
}

// TestTOLV0103_DeclaredTestFilePatterns: a declared pattern classifies a
// file the closed naming set does not, so a repository whose only test is
// such a file is TESTABLE and its inventory names the declared count
// (TOL-V0-103).
func TestTOLV0103_DeclaredTestFilePatterns(t *testing.T) {
	infra := infraRepo(t)
	pattern := wire.Array(wire.ObjectValue(wire.NewObject().Set("kind", wire.String("sql")).Set("pattern", wire.String("deploy/*.sql"))))
	r, id, _ := obligationRepoWith(t, withObligations(testRepoPolicy(map[string]string{"infra": infra}), func(o *wire.Object) {
		o.Set("testFilePatterns", pattern)
	}))
	seedObligations(t, r.Root, id)
	cites := `[{"id":"AC-2","state":null,"core":null,"reason":"cites the deploy module","cites":[{"repository":"infra","path":"deploy"}]}]`
	if x := boundarySet(t, r.Root, id, "cites-1", "owner", "OWNER", cites); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("cites: %s", x.stdout)
	}
	v := obligationsBoundaryRead(t, r.Root, id)
	if got := boundaryVerdicts(v)["AC-2"]; got != "TESTABLE/" {
		t.Fatalf("AC-2 = %q: %s", got, wire.Encode(v))
	}
	p := boundaryRepo(t, v, "infra")
	if field(p, "testFiles").Str != "1" || field(field(p, "kinds").Arr[0], "kind").Str != "sql" || field(p, "declaredPatterns").Str != "1" {
		t.Fatalf("infra inventory: %s", wire.Encode(p))
	}
	if wire.Has(boundaryRepo(t, v, ""), "declaredPatterns") != true {
		t.Fatalf("primary inventory lacks the declared count: %s", wire.Encode(v))
	}
}

func planFile(t *testing.T, id, tests string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plan.json")
	fixture.Write(t, p, []byte(`{"profile":"taskman-obligation-plan/0","ticketId":"`+id+`","tests":`+tests+`}`))
	return p
}

func planRun(t *testing.T, root, id, plan string, extra ...string) run {
	t.Helper()
	return atm(t, root, nil, append([]string{"ticket", "obligations", "plan", "--target", id, "--plan", plan}, extra...)...)
}

// TestTOLV0104_PlanCountsLedgerWitnesses: under --witnesses a plan counts
// reviewed and declared ledger witnesses of any test type, names each with
// its type and revision, and omits the new members without the switch
// (TOL-V0-104, GitHub #1202).
func TestTOLV0104_PlanCountsLedgerWitnesses(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	credit := reviewFile(t, "1", "none", verdict("rev-a", "correct", "none", "asserts the login redirect"))
	if x := reviewWitness(t, r.Root, id, "rv-1", head, "AC-1", credit); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "CREDITED" {
		t.Fatalf("review: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, declareArgs(id, "w-9", obligationRevision(t, r.Root, id), head, "AC-9")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("declare: %s", x.stdout)
	}
	plan := planFile(t, id, `[{"test":"a","project":"api","obligations":["AC-2"]}]`)
	plain := planRun(t, r.Root, id, plan)
	if plain.res.Outcome != wire.OutcomeOK || wire.Has(plain.res.Items[0], "ledgerWitnesses") {
		t.Fatalf("plain plan: %s", plain.stdout)
	}
	x := planRun(t, r.Root, id, plan, "--witnesses")
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "ok").Bool {
		t.Fatalf("witness plan: %s", x.stdout)
	}
	var got []string
	for _, w := range field(x.res.Items[0], "ledgerWitnesses").Arr {
		if field(w, "commit").Str != head || len(field(w, "eventSha256").Str) != 64 {
			t.Fatalf("witness revision: %s", wire.Encode(w))
		}
		got = append(got, field(w, "id").Str+":"+field(w, "source").Str+":"+field(w, "testType").Str+":"+field(w, "status").Str)
	}
	if strings.Join(got, ",") != "AC-1:REVIEWED:javascript:COVERED,AC-9:DECLARED:declared:COVERED" {
		t.Fatalf("ledgerWitnesses: %v", got)
	}
}

// TestTOLV0105_PlanStaleReviewedWitness: a reviewed witness whose subject
// changed at HEAD is a STALE_REVIEW finding that refuses the plan
// (TOL-V0-105).
func TestTOLV0105_PlanStaleReviewedWitness(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	credit := reviewFile(t, "1", "none", verdict("rev-a", "correct", "none", "asserts the login redirect"))
	if x := reviewWitness(t, r.Root, id, "rv-1", head, "AC-1", credit); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("review: %s", x.stdout)
	}
	fixture.Write(t, filepath.Join(r.Root, "e2e", "login.spec.ts"), []byte(obligationSpec+"// changed\n"))
	git(t, r.Root, "add", "e2e")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "change spec")
	plan := planFile(t, id, `[{"test":"a","project":"api","obligations":["AC-2"]}]`)
	x := planRun(t, r.Root, id, plan, "--witnesses")
	if x.res.Outcome == wire.OutcomeOK || field(x.res.Items[0], "ok").Bool || !strings.Contains(string(x.stdout), "STALE_REVIEW") {
		t.Fatalf("stale reviewed witness passed: %s", x.stdout)
	}
	f := field(x.res.Items[0], "findings").Arr
	if len(f) != 1 || field(f[0], "kind").Str != "STALE_REVIEW" || field(f[0], "id").Str != "AC-1" {
		t.Fatalf("findings: %s", x.stdout)
	}
	if w := field(x.res.Items[0], "ledgerWitnesses").Arr; len(w) != 1 || field(w[0], "status").Str != "REVIEW_STALE_AT_HEAD" {
		t.Fatalf("ledgerWitnesses: %s", x.stdout)
	}
}

// TestTOLV0106_UnassignedNamesConsideredWitness: an UNASSIGNED row names the
// witness considered and why it was insufficient (TOL-V0-106).
func TestTOLV0106_UnassignedNamesConsideredWitness(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	plan := planFile(t, id, `[{"test":"a","project":"api","obligations":["AC-1"]}]`)
	considered := func() wire.Value {
		t.Helper()
		x := planRun(t, r.Root, id, plan, "--witnesses")
		f := field(x.res.Items[0], "findings").Arr
		if x.res.Outcome == wire.OutcomeOK || len(f) != 1 || field(f[0], "kind").Str != "UNASSIGNED" || field(f[0], "id").Str != "AC-2" {
			t.Fatalf("plan: %s", x.stdout)
		}
		return field(f[0], "considered")
	}
	if c := considered(); field(c, "witness").Kind != wire.KindNull || field(c, "insufficient").Str != "NO_LEDGER_WITNESS" {
		t.Fatalf("no witness: %s", wire.Encode(c))
	}
	held := reviewFile(t, "1", "none", verdict("rev-a", "partial", "none", "misses the error branch"))
	if x := reviewWitness(t, r.Root, id, "rv-2", head, "AC-2", held); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "HELD" {
		t.Fatalf("held review: %s", x.stdout)
	}
	c := considered()
	w := field(c, "witness")
	if field(c, "insufficient").Str != "REVIEW_HELD" || field(w, "kind").Str != "REVIEWED" || field(w, "path").Str != "e2e/login.spec.ts" ||
		field(w, "commit").Str != head || field(w, "axes").Kind != wire.KindArray {
		t.Fatalf("held witness: %s", wire.Encode(c))
	}
	if x := planRun(t, r.Root, id, plan); wire.Has(field(x.res.Items[0], "findings").Arr[0], "considered") {
		t.Fatalf("plain plan names a considered witness: %s", x.stdout)
	}
}

func capabilityRead(t *testing.T, root string, args ...string) wire.Value {
	t.Helper()
	x := atm(t, root, nil, append([]string{"ticket", "obligations", "capability"}, args...)...)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("capability: %s", x.stdout)
	}
	return x.res.Items[0]
}

// TestTOLV0107_CapabilityAtEnvironmentBaseline: a capability query answers
// from the declared environment baseline, never HEAD, names the baseline
// revision, and abstains UNKNOWN when the baseline is undeclared or does
// not resolve (TOL-V0-107..109).
func TestTOLV0107_CapabilityAtEnvironmentBaseline(t *testing.T) {
	app, _ := separateTestRepo(t)
	r, _, head := obligationRepoWith(t, withObligations(testRepoPolicy(map[string]string{"app": app}), func(o *wire.Object) {
		o.Set("environmentBaselines", wire.ObjectValue(wire.NewObject().Set(".", wire.String("refs/heads/env")).Set("app", wire.String("refs/heads/missing"))))
	}))
	git(t, r.Root, "branch", "env", head)
	fixture.Write(t, filepath.Join(r.Root, "lib", "feature.go"), []byte("package lib\n\nfunc Feature() {}\n"))
	commitTree(t, r.Root, "feature")
	v := capabilityRead(t, r.Root, "--path", "lib")
	b := field(v, "baseline")
	if field(v, "status").Str != "ABSENT" || field(v, "reason").Str != "NOT_FOUND_AT_BASELINE" || field(b, "ref").Str != "refs/heads/env" ||
		field(b, "commit").Str != head || field(b, "repository").Kind != wire.KindNull {
		t.Fatalf("absent at baseline: %s", wire.Encode(v))
	}
	v = capabilityRead(t, r.Root, "--path", "e2e", "--contains", "AC-1 login")
	if field(v, "status").Str != "PRESENT" || strList(v, "matches") != "e2e/login.spec.ts" || field(v, "reason").Kind != wire.KindNull {
		t.Fatalf("contains at baseline: %s", wire.Encode(v))
	}
	if v = capabilityRead(t, r.Root, "--path", "e2e", "--contains", "not in the file"); field(v, "status").Str != "ABSENT" {
		t.Fatalf("absent text: %s", wire.Encode(v))
	}
	git(t, r.Root, "branch", "-f", "env", "HEAD")
	if v = capabilityRead(t, r.Root, "--path", "lib"); field(v, "status").Str != "PRESENT" || strList(v, "matches") != "lib/feature.go" ||
		field(field(v, "baseline"), "commit").Str != headOID(t, r.Root) {
		t.Fatalf("moved baseline: %s", wire.Encode(v))
	}
	if v = capabilityRead(t, r.Root, "--path", "e2e", "--repository", "app"); field(v, "status").Str != "UNKNOWN" || field(v, "reason").Str != "ENVIRONMENT_BASELINE_UNRESOLVED" ||
		field(field(v, "baseline"), "commit").Kind != wire.KindNull || field(field(v, "baseline"), "ref").Str != "refs/heads/missing" {
		t.Fatalf("unresolved baseline: %s", wire.Encode(v))
	}
	if v = capabilityRead(t, r.Root, "--path", "deploy", "--repository", "infra"); field(v, "status").Str != "UNKNOWN" || field(v, "reason").Str != "ENVIRONMENT_BASELINE_UNDECLARED" ||
		field(field(v, "baseline"), "ref").Kind != wire.KindNull {
		t.Fatalf("undeclared baseline: %s", wire.Encode(v))
	}
	for _, bad := range [][]string{{"--path", "/etc"}, {"--path", "../x"}, {"--path", "a//b"}, {"--path", "e2e", "--contains", ""}, {}} {
		if x := atm(t, r.Root, nil, append([]string{"ticket", "obligations", "capability"}, bad...)...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("bad capability args %v admitted: %s", bad, x.stdout)
		}
	}
}

// TestTOLV0107_CapabilityUndeclaredPrimary: with no environmentBaselines the
// primary repository abstains too, so HEAD never stands in for an
// environment (TOL-V0-107).
func TestTOLV0107_CapabilityUndeclaredPrimary(t *testing.T) {
	r, _, _ := obligationRepo(t)
	v := capabilityRead(t, r.Root, "--path", "e2e")
	if field(v, "status").Str != "UNKNOWN" || field(v, "reason").Str != "ENVIRONMENT_BASELINE_UNDECLARED" || len(field(v, "matches").Arr) != 0 {
		t.Fatalf("undeclared primary: %s", wire.Encode(v))
	}
}

// TestTOLV0108_CapabilityBoundsAndReadOnly: matches stop at 16 with
// truncated, a listing over 1,024 files reports truncated beside a match and
// abstains SEARCH_BOUND_EXCEEDED without one, and the read writes nothing
// (TOL-V0-108, TOL-V0-109).
func TestTOLV0108_CapabilityBoundsAndReadOnly(t *testing.T) {
	r, _, _ := obligationRepoWith(t, func(p wire.Value) {
		p.Obj.Set("obligations", wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(false)).
			Set("environmentBaselines", wire.ObjectValue(wire.NewObject().Set(".", wire.String("refs/heads/env"))))))
	})
	for i := 0; i < 20; i++ {
		fixture.Write(t, filepath.Join(r.Root, "many", fmt.Sprintf("f%02d.txt", i)), []byte("needle\n"))
	}
	for i := 0; i < 1030; i++ {
		body := "hay\n"
		switch i {
		case 0:
			body = "needle\n"
		case 1029:
			body = "late\n"
		}
		fixture.Write(t, filepath.Join(r.Root, "big", fmt.Sprintf("f%04d.txt", i)), []byte(body))
	}
	commitTree(t, r.Root, "bounds")
	git(t, r.Root, "branch", "env", "HEAD")
	before := fixture.TreeSnapshot(t, r.Root)
	for _, args := range [][]string{{"--path", "many"}, {"--path", "many", "--contains", "needle"}} {
		v := capabilityRead(t, r.Root, args...)
		if field(v, "status").Str != "PRESENT" || len(field(v, "matches").Arr) != 16 || !field(v, "truncated").Bool {
			t.Fatalf("16-match bound %v: %s", args, wire.Encode(v))
		}
	}
	if v := capabilityRead(t, r.Root, "--path", "big", "--contains", "late"); field(v, "status").Str != "UNKNOWN" ||
		field(v, "reason").Str != "SEARCH_BOUND_EXCEEDED" || len(field(v, "matches").Arr) != 0 || field(field(v, "baseline"), "commit").Kind != wire.KindString {
		t.Fatalf("file bound without a match: %s", wire.Encode(v))
	}
	if v := capabilityRead(t, r.Root, "--path", "big", "--contains", "needle"); field(v, "status").Str != "PRESENT" ||
		strList(v, "matches") != "big/f0000.txt" || !field(v, "truncated").Bool {
		t.Fatalf("file bound beside a match: %s", wire.Encode(v))
	}
	if after := fixture.TreeSnapshot(t, r.Root); !fixture.SameTree(before, after) {
		t.Fatal("capability read changed the repository")
	}
}

// TestTOLV0079_CitesBoundsReplaceAndDefault: cites admit 1..16 objects, a
// trailing "/" directory and, under an explicit requireDeclaredCites false,
// an undeclared label at write (read UNKNOWN);
// a later change's cites replace the entry's, and an entry without the
// member keeps its exact bytes (TOL-V0-079).
func TestTOLV0079_CitesBoundsReplaceAndDefault(t *testing.T) {
	// An undeclared label is admitted only under an explicit
	// requireDeclaredCites false (TOL-V0-102, V1-1963).
	r, id, _ := obligationRepoWith(t, func(p wire.Value) {
		p.Obj.Set("obligations", wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(false)).Set("requireDeclaredCites", wire.Bool(false))))
	})
	seedObligations(t, r.Root, id)
	plain := obligationsShow(t, r.Root, id)
	for _, e := range field(plain, "entries").Arr {
		if wire.Has(e, "cites") {
			t.Fatalf("seeded entry has cites: %s", wire.Encode(e))
		}
	}
	many := make([]string, 17)
	for i := range many {
		many[i] = `{"repository":null,"path":"e2e/f` + string(rune('a'+i)) + `.ts"}`
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	for i, bad := range []string{`[]`, `[` + strings.Join(many, ",") + `]`} {
		ch := `[{"id":"AC-1","state":null,"core":null,"reason":"r","cites":` + bad + `}]`
		if x := boundarySet(t, r.Root, id, "bad-"+string(rune('a'+i)), "owner", "OWNER", ch); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "cites") {
			t.Fatalf("cites %d admitted: %s", i, x.stdout)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("refused cites wrote state")
	}
	first := `[{"id":"AC-1","state":null,"core":null,"reason":"r","cites":[{"repository":null,"path":"e2e/"},{"repository":"nope","path":"x"}]}]`
	if x := boundarySet(t, r.Root, id, "c-1", "owner", "OWNER", first); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("cites with an undeclared label: %s", x.stdout)
	}
	if got := boundaryVerdicts(obligationsBoundaryRead(t, r.Root, id))["AC-1"]; got != "TESTABLE/" {
		t.Fatalf("directory cite with a slash: %q", got)
	}
	second := `[{"id":"AC-1","state":null,"core":null,"reason":"r","cites":[{"repository":"nope","path":"x"}]}]`
	if x := boundarySet(t, r.Root, id, "c-2", "owner", "OWNER", second); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("replacing cites: %s", x.stdout)
	}
	if got := boundaryVerdicts(obligationsBoundaryRead(t, r.Root, id))["AC-1"]; got != "UNKNOWN/REPOSITORY_UNAVAILABLE" {
		t.Fatalf("replaced cites still testable: %q", got)
	}
	for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
		if field(e, "id").Str == "AC-1" && len(field(e, "cites").Arr) != 1 {
			t.Fatalf("cites not replaced: %s", wire.Encode(e))
		}
	}
}

// TestTOLV0080_ScopeShape: scope needs a null core and a 1..512-byte
// non-blank evidence for PROPOSE (TOL-V0-080).
func TestTOLV0080_ScopeShape(t *testing.T) {
	r, id, _ := obligationRepo(t)
	seedObligations(t, r.Root, id)
	before := fixture.TreeSnapshot(t, r.StateDir)
	for i, bad := range []string{
		`[{"id":"AC-2","state":null,"core":false,"reason":"r","scope":{"action":"PROPOSE","core":false,"evidence":"e"}}]`,
		`[{"id":"AC-2","state":null,"core":null,"reason":"r","scope":{"action":"PROPOSE","core":false,"evidence":"  "}}]`,
		`[{"id":"AC-2","state":null,"core":null,"reason":"r","scope":{"action":"PROPOSE","core":false,"evidence":"` + strings.Repeat("e", 513) + `"}}]`,
		`[{"id":"AC-2","state":null,"core":null,"reason":"r","scope":{"action":"PROPOSE","core":false}}]`,
		`[{"id":"AC-2","state":null,"core":null,"reason":"r","scope":{"action":"DROP","core":false}}]`,
	} {
		if x := boundarySet(t, r.Root, id, "s-"+string(rune('a'+i)), "owner", "OWNER", bad); x.res.Outcome == wire.OutcomeOK || x.res.Outcome == "" {
			t.Fatalf("scope %d admitted: %s", i, x.stdout)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("refused scope wrote state")
	}
	ok := `[{"id":"AC-2","state":null,"core":null,"reason":"no provider","scope":{"action":"PROPOSE","core":false,"evidence":"` + strings.Repeat("e", 512) + `"}}]`
	if x := boundarySet(t, r.Root, id, "s-ok", "agent-1", "OWNER", ok); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("512-byte evidence: %s", x.stdout)
	}
}

// TestTOLV0084_CandidateShapeAndClear: a candidate needs a 1-based line
// range, is recorded with its issue time, and an entry without one keeps
// its exact bytes (TOL-V0-084).
func TestTOLV0084_CandidateShapeAndClear(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	for i, bad := range []string{`{"path":"e2e/login.spec.ts","startLine":"0","endLine":"1"}`, `{"path":"e2e/login.spec.ts","startLine":"3","endLine":"2"}`, `{"path":"e2e/login.spec.ts","startLine":"1"}`} {
		ch := `[{"id":"AC-1","state":null,"core":null,"reason":"r","candidate":` + bad + `}]`
		if x := boundarySet(t, r.Root, id, "cand-"+string(rune('a'+i)), "owner", "OWNER", ch); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("candidate %d admitted: %s", i, x.stdout)
		}
	}
	ch := `[{"id":"AC-1","state":null,"core":null,"reason":"r","candidate":{"path":"e2e/login.spec.ts","startLine":"1","endLine":"2"}}]`
	if x := boundarySet(t, r.Root, id, "cand-ok", "owner", "OWNER", ch); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("candidate: %s", x.stdout)
	}
	entry := func(ac string) wire.Value {
		for _, e := range field(obligationsShow(t, r.Root, id), "entries").Arr {
			if field(e, "id").Str == ac {
				return e
			}
		}
		t.Fatalf("no entry %s", ac)
		return wire.Value{}
	}
	if c := field(entry("AC-1"), "candidate"); field(c, "startLine").Str != "1" || field(c, "at").Str == "" {
		t.Fatalf("recorded candidate: %s", wire.Encode(c))
	}
	if wire.Has(entry("AC-2"), "candidate") {
		t.Fatal("an entry without a candidate gained one")
	}
	credit := reviewFile(t, "1", "none", verdict("rev-a", "correct", "none", "ok"))
	if x := reviewWitness(t, r.Root, id, "rv-1", head, "AC-1", credit); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("review: %s", x.stdout)
	}
	if wire.Has(entry("AC-1"), "candidate") {
		t.Fatal("a credit left the candidate")
	}
}
