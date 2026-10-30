package corpusbridge

import (
	"context"
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

const evalCasesFixture = `{"schema":"corvint-corpus-eval-cases/1","origin":"labelled","cases":[
{"id":"search-p1","operation":"search","arguments":{"query":"Checkout step 1"},"expect":{"subjects":["native:file:docs/p1.md"]},"tags":["checkout"]},
{"id":"get-p1","operation":"get","arguments":{"id":"native:file:docs/p1.md"},"expect":{"subjects":["native:file:docs/p1.md"],"claims":["native:file:docs/p1.md:paragraph:3"]},"tags":["checkout","exact"]},
{"id":"locate-p2","operation":"locate","arguments":{"path":"docs/p2.md"},"expect":{"paths":["docs/p2.md"],"must_not":["native:file:docs/p2.md"]}},
{"id":"absent","operation":"search","arguments":{"query":"zebra"},"expect":{"abstain":true}},
{"id":"stale-case","operation":"get","arguments":{"id":"native:file:docs/p0.md"},"expect":{"subjects":["native:file:docs/p0.md","native:file:docs/gone.md"]},"origin":"synthetic"}
]}`

func evalRun(t *testing.T, root, cases string) *EvalReport {
	t.Helper()
	parsed, err := ParseEvalCases([]byte(cases))
	if err != nil {
		t.Fatal(err)
	}
	registry, rerr := New(root, "corpus.json")
	if rerr != nil {
		t.Fatal(rerr)
	}
	report, err := registry.Evaluate(context.Background(), parsed, "cases.json", []byte(cases))
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func evalCase(t *testing.T, report *EvalReport, id string) EvalResult {
	t.Helper()
	for _, c := range report.Cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("case %s missing", id)
	return EvalResult{}
}

// DCP-V1-251, DCP-V1-252, DCP-V1-253: every case is answered by the MCP tool path; hits,
// ranks, P@k/R@k, abstention, must_not, unresolved IDs, bytes and grouped denominators are
// reported.
func TestDCPV1251EvalScoresThroughTheMCPToolPath(t *testing.T) {
	root, _ := lazyFixture(t)
	report := evalRun(t, root, evalCasesFixture)
	registry, _ := New(root, "corpus.json")
	for _, c := range report.Cases {
		tool, _ := ToolFor(c.Operation)
		var arguments map[string]any
		for _, raw := range []string{evalCasesFixture} {
			var file EvalCases
			_ = json.Unmarshal([]byte(raw), &file)
			for _, fc := range file.Cases {
				if fc.ID == c.ID {
					_ = json.Unmarshal(fc.Arguments, &arguments)
				}
			}
		}
		data, _ := json.Marshal(arguments)
		structured, text, failure, protocol := registry.Call(context.Background(), tool, data)
		rows, _ := structured["results"].([]any)
		if failure != nil || protocol != nil || c.ReceiptBytes != len(text) || c.Returned != min(len(rows), c.K) || c.Tool != tool {
			t.Fatalf("%s not scored from the served receipt: %+v (served %d bytes, %d rows)", c.ID, c, len(text), len(rows))
		}
	}
	if c := evalCase(t, report, "search-p1"); c.Status != "pass" || !c.Hit || c.Rank == nil || *c.Rank < 1 || *c.Rank > c.Returned || !same(c.RecallAtK, 1, 1) || c.Origin != "labelled" {
		t.Fatalf("search-p1 = %+v", c)
	}
	if c := evalCase(t, report, "get-p1"); c.Status != "pass" || c.RecallAtK.N != 2 || c.RecallAtK.D != 2 || c.PrecisionAtK.D != c.Returned {
		t.Fatalf("get-p1 = %+v", c)
	}
	if c := evalCase(t, report, "locate-p2"); c.Status != "fail" || !c.Hit || len(c.MustNotViolations) != 1 || c.MustNotViolations[0] != "native:file:docs/p2.md" {
		t.Fatalf("locate-p2 must_not = %+v", c)
	}
	if c := evalCase(t, report, "absent"); c.Status != "pass" || c.Abstention != "correct" || !c.Abstained {
		t.Fatalf("absent = %+v", c)
	}
	c := evalCase(t, report, "stale-case")
	if c.Status != "fail" || len(c.Unresolved) != 1 || c.Unresolved[0] != (EvalUnresolved{Kind: "subject", ID: "native:file:docs/gone.md"}) || !same(c.RecallAtK, 1, 2) || c.Origin != "synthetic" {
		t.Fatalf("unresolved expected ID dropped or scored as found: %+v", c)
	}
	overall := report.Summary.Overall
	if overall.Cases != 5 || overall.Passed != 3 || overall.Failed != 2 || overall.Hit.D != 4 || !same(overall.Abstention, 1, 1) || overall.Unresolved != 1 || overall.MustNotViolations != 1 {
		t.Fatalf("overall = %+v", overall)
	}
	if g := report.Summary.ByOperation["search"]; g.Cases != 2 || strings.Join(g.CaseIDs, ",") != "absent,search-p1" {
		t.Fatalf("by_operation search = %+v", g)
	}
	if g := report.Summary.ByTag["checkout"]; g.Cases != 2 || !same(g.Hit, 2, 2) {
		t.Fatalf("by_tag checkout = %+v", g)
	}
	if g := report.Summary.ByOrigin["synthetic"]; g.Cases != 1 || g.CaseIDs[0] != "stale-case" {
		t.Fatalf("by_origin synthetic = %+v", g)
	}
	if report.Corpus.ArtifactSHA256 == "" || report.Corpus.RepositoryID == "" || report.Corpus.Profile.ID == "" || report.Identity.CasesSHA256 != doccorpus.Digest([]byte(evalCasesFixture)) {
		t.Fatalf("identity missing: %+v %+v", report.Corpus, report.Identity)
	}
}

// DCP-V1-250, DCP-V1-256: a malformed case file refuses whole, and synthetic or generated
// cases cannot be relabelled as human-labelled.
func TestDCPV1250EvalRefusesMalformedCaseFiles(t *testing.T) {
	head := `{"schema":"corvint-corpus-eval-cases/1","origin":"labelled","cases":[`
	for name, file := range map[string]string{
		"schema":         `{"schema":"x","origin":"labelled","cases":[{"id":"a","operation":"search","arguments":{"query":"q"},"expect":{"abstain":true}}]}`,
		"origin":         `{"schema":"corvint-corpus-eval-cases/1","origin":"human","cases":[{"id":"a","operation":"search","arguments":{"query":"q"},"expect":{"abstain":true}}]}`,
		"empty":          head + `]}`,
		"unknown member": head + `{"id":"a","operation":"search","arguments":{"query":"q"},"expect":{"abstain":true},"extra":1}]}`,
		"duplicate id":   head + `{"id":"a","operation":"search","expect":{"abstain":true}},{"id":"a","operation":"search","expect":{"abstain":true}}]}`,
		"operation":      head + `{"id":"a","operation":"render","expect":{"abstain":true}}]}`,
		"arguments":      head + `{"id":"a","operation":"search","arguments":[1],"expect":{"abstain":true}}]}`,
		"no expectation": head + `{"id":"a","operation":"search","expect":{}}]}`,
		"contradiction":  head + `{"id":"a","operation":"search","expect":{"abstain":true,"subjects":["s"]}}]}`,
		"duplicate item": head + `{"id":"a","operation":"search","expect":{"subjects":["s","s"]}}]}`,
		"relabelled":     `{"schema":"corvint-corpus-eval-cases/1","origin":"synthetic","cases":[{"id":"a","operation":"search","expect":{"abstain":true},"origin":"labelled"}]}`,
		"trailing":       head + `{"id":"a","operation":"search","expect":{"abstain":true}}]}{}`,
	} {
		if _, err := ParseEvalCases([]byte(file)); err == nil {
			t.Errorf("%s: case file admitted", name)
		}
	}
	synthetic := strings.Replace(evalCasesFixture, `"origin":"labelled"`, `"origin":"synthetic"`, 1)
	root, _ := lazyFixture(t)
	report := evalRun(t, root, synthetic)
	for _, c := range report.Cases {
		if c.Origin != "synthetic" {
			t.Fatalf("synthetic case relabelled: %+v", c)
		}
	}
	if report.CaseFile.Origin != "synthetic" || report.Summary.ByOrigin["synthetic"].Cases != 5 || len(report.Summary.ByOrigin) != 1 {
		t.Fatalf("synthetic origin not carried: %+v", report.Summary.ByOrigin)
	}
}

// DCP-V1-254: a rebuilt corpus of the same repository and profile is compared; every regressed
// case is listed and improvements never hide one.
func TestDCPV1254BaselineListsEveryRegression(t *testing.T) {
	root, _ := lazyFixture(t)
	baseline, err := doccorpus.Encode(evalRun(t, root, evalCasesFixture))
	if err != nil {
		t.Fatal(err)
	}
	same := evalRun(t, root, evalCasesFixture)
	if err := CompareEval(same, "baseline.json", baseline); err != nil || same.Regressed() || !same.Baseline.Comparable || same.Baseline.Unchanged != 5 {
		t.Fatalf("unchanged corpus regressed: %v %+v", err, same.Baseline)
	}
	// Rebuild without p1: same repository and profile, a new artifact.
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("rm", "-q", "docs/p1.md")
	git("-c", "user.name=test", "-c", "user.email=test@invalid", "commit", "-qm", "drop p1")
	m, err := doccorpus.Inventory(context.Background(), root, git("rev-parse", "HEAD"), "docs", "2026-09-20T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := doccorpus.Encode(a)
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	rebuilt := evalRun(t, root, evalCasesFixture)
	if err := CompareEval(rebuilt, "baseline.json", baseline); err != nil {
		t.Fatal(err)
	}
	c := rebuilt.Baseline
	ids := []string{}
	for _, change := range c.Regressed {
		ids = append(ids, change.ID)
	}
	if !c.Comparable || !rebuilt.Regressed() || strings.Join(ids, ",") != "search-p1,get-p1" || rebuilt.Corpus.ArtifactSHA256 == same.Corpus.ArtifactSHA256 {
		t.Fatalf("regressions not all listed: %+v", c)
	}
}

// DCP-V1-255: a changed case file or corpus identity is reported not comparable, never compared.
func TestDCPV1255ChangedIdentityIsNotComparable(t *testing.T) {
	root, _ := lazyFixture(t)
	report := evalRun(t, root, evalCasesFixture)
	baseline, _ := doccorpus.Encode(report)
	changed := strings.Replace(evalCasesFixture, `"tags":["checkout"]`, `"tags":["checkout","new"]`, 1)
	moved := evalRun(t, root, changed)
	if err := CompareEval(moved, "baseline.json", baseline); err != nil || moved.Baseline.Comparable || !moved.Regressed() || moved.Baseline.NotComparable[0] != "case file changed" || len(moved.Baseline.Regressed) != 0 {
		t.Fatalf("changed case file compared: %v %+v", err, moved.Baseline)
	}
	var other map[string]any
	if err := json.Unmarshal(baseline, &other); err != nil {
		t.Fatal(err)
	}
	other["comparison_identity"].(map[string]any)["profile_id"] = "another-profile"
	foreign, _ := json.Marshal(other)
	again := evalRun(t, root, evalCasesFixture)
	if err := CompareEval(again, "baseline.json", foreign); err != nil || again.Baseline.Comparable || again.Baseline.NotComparable[0] != "corpus identity changed" {
		t.Fatalf("foreign corpus compared: %v %+v", err, again.Baseline)
	}
	if err := CompareEval(again, "baseline.json", []byte(`{"schema":"other"}`)); err == nil {
		t.Fatal("malformed baseline admitted")
	}
}

func same(r EvalRatio, n, d int) bool { return r.N == n && r.D == d }
