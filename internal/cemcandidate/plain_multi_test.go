package cemcandidate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// singleAdmittedStableSha256 is the stable.json digest an admitted
// single-form request over the neutral case produced before the multi-receipt
// form existed (recorded on origin/main 025120e1a), with the plan root fixed so
// the runner bytes are deterministic.
const singleAdmittedStableSha256 = "21dc2dbbf6f52c5ba8fb699c1ee50b13a386e6cddd29cc3cdc35eb9f6ea32f16"

// TestCEMV1574SingleFormOutputIsUnchanged: the single runnerPlan/runnerReceipt
// request still produces the exact map bytes it produced before the
// multi-receipt form (CEM-V1-574). The unadmitted single form is held byte for
// byte by TestCEMV1240PlainAssemblyReproducesNeutralCriterionCase.
func TestCEMV1574SingleFormOutputIsUnchanged(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, _ := plainFixture(t)
	docs := neutralRunnerDocs(t, r.Repository, r.Target)
	docs.plan.Request.Root = "/fixture/repo"
	docs.receipt.PlanSha256 = tr.Identity(docs.plan)
	if _, e := admit(t, r, out, mustJSON(t, docs.plan), mustJSON(t, docs.receipt)); e != nil {
		t.Fatalf("admitted single form refused: %v", e)
	}
	got, e := os.ReadFile(filepath.Join(out, "stable.json"))
	if e != nil {
		t.Fatal(e)
	}
	if d := tr.Digest(got); d != singleAdmittedStableSha256 {
		t.Fatalf("single-form stable.json sha256 %s, want %s:\n%s", d, singleAdmittedStableSha256, got)
	}
}

// multiRunnerCase is the two-profile fixture this producer reproduces and both
// the native and the portable reader check (CEM-V1-573, CEM-V1-577).
const multiRunnerCase = "interop/cem01-go/testdata/multi-runner"

func multiRunnerRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", multiRunnerCase)
}

// multiRunnerFixture builds the two-profile recipe repository, cites its Go
// hunks to SPEC.md line 5 and its JavaScript hunks to line 6, and returns a
// runners-form request whose criterion 0 is covered by the go-test receipt
// and criterion 1 by the node-test receipt, the bundle path and the case root.
func multiRunnerFixture(t *testing.T) (PlainRequest, string, string) {
	t.Helper()
	caseRoot := multiRunnerRoot(t)
	var expected struct {
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
	}
	readJSON(t, filepath.Join(caseRoot, "EXPECTED.json"), &expected)
	temp, repo, base, target := recipeRepository(t, caseRoot)
	if base != expected.BaseRevision || target != expected.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, expected.BaseRevision, expected.TargetRevision)
	}
	source, produced := producerMap(t, repo, base, target, map[string]string{"1": "5:5", "2": "6:6", "3": "6:6", "4": "5:5"})
	// Span offsets are bytes, so the earlier span is the line 5 citation.
	evidence := map[int64]string{}
	if len(source.Change.Evidence) == 2 {
		first, second := source.Change.Evidence[0], source.Change.Evidence[1]
		if first.Span.Start > second.Span.Start {
			first, second = second, first
		}
		evidence[5], evidence[6] = first.ID, second.ID
	}
	hunks := map[string][]string{}
	for _, h := range source.Change.Hunks {
		profile := "go-test"
		if strings.HasSuffix(h.Path, ".mjs") {
			profile = "node-test"
		}
		hunks[profile] = append(hunks[profile], h.ID)
	}
	if len(evidence) != 2 || evidence[5] == "" || evidence[6] == "" || len(hunks["go-test"]) != 2 || len(hunks["node-test"]) != 2 {
		t.Fatalf("producer map has evidence %v and hunks %v, want lines 5 and 6 and two hunks per profile", evidence, hunks)
	}
	inputs := filepath.Join(temp, "inputs")
	if e := os.Mkdir(inputs, 0o700); e != nil {
		t.Fatal(e)
	}
	ref := func(name string) FileRef {
		b, e := os.ReadFile(filepath.Join(caseRoot, "artifacts", name))
		if e != nil {
			t.Fatal(e)
		}
		return save(t, filepath.Join(inputs, name), b)
	}
	pairs := []RunnerPair{{ref("runner-plan-0.json"), ref("runner-receipt-0.json")}, {ref("runner-plan-1.json"), ref("runner-receipt-1.json")}}
	r := PlainRequest{Profile: PlainAssemblyProfile, Repository: repo, ExpectedBase: base, Target: target,
		SourceMap: save(t, filepath.Join(inputs, "source.json"), produced), Runners: pairs,
		RunnerAdmission: json.RawMessage(strconv.Quote(RunnerAdmissionProfile)),
		Criteria: []PlainCriterion{
			{CriterionEvidenceID: evidence[5], HunkIDs: hunks["go-test"], EvidenceIDs: []string{evidence[5]}, RunnerReceiptSha256s: []string{pairs[0].RunnerReceipt.Sha256}},
			{CriterionEvidenceID: evidence[6], HunkIDs: hunks["node-test"], EvidenceIDs: []string{evidence[6]}, RunnerReceiptSha256s: []string{pairs[1].RunnerReceipt.Sha256}},
		}}
	return r, filepath.Join(temp, "bundle"), caseRoot
}

// TestCEMV1573RunnersFormReproducesTwoProfileCase: the runners form admits a
// go-test and a node-test receipt, writes each pair at its own artifact
// paths, links each criterion to the receipt that covers it, and its map,
// which the native verifier ACCEPTs, equals the shared two-profile case byte
// for byte (CEM-V1-573).
func TestCEMV1573RunnersFormReproducesTwoProfileCase(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, caseRoot := multiRunnerFixture(t)
	result, e := AssemblePlain(context.Background(), r, out)
	if e != nil {
		t.Fatalf("AssemblePlain: %v", e)
	}
	if result.Verification.Outcome != "ACCEPT" || result.StablePath != filepath.Join(out, "stable.json") {
		t.Fatalf("result %+v", result)
	}
	for _, name := range []string{"stable.json", "artifacts/runner-plan-0.json", "artifacts/runner-receipt-0.json", "artifacts/runner-plan-1.json", "artifacts/runner-receipt-1.json"} {
		got, e := os.ReadFile(filepath.Join(out, name))
		if e != nil {
			t.Fatal(e)
		}
		want, e := os.ReadFile(filepath.Join(caseRoot, name))
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s differs from the two-profile case:\n%s\nwant:\n%s", name, got, want)
		}
	}
	raw, e := os.ReadFile(filepath.Join(out, "stable.json"))
	if e != nil {
		t.Fatal(e)
	}
	m, e := cw.ParseStable(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Receipts) != 2 || len(m.Links) != 2 {
		t.Fatalf("map has %d receipts and %d links, want 2 and 2", len(m.Receipts), len(m.Links))
	}
	for i, receipt := range m.Receipts {
		if receipt.Sha256 != r.Runners[i].RunnerReceipt.Sha256 || receipt.PlanSha256 != r.Runners[i].RunnerPlan.Sha256 || receipt.Profile != cw.StableRunnerReceiptProfile {
			t.Fatalf("receipt %d = %+v, want pair %d labelled %s", i, receipt, i, cw.StableRunnerReceiptProfile)
		}
		if got := m.Links[i].RunnerReceiptSha256s; len(got) != 1 || got[0] != receipt.Sha256 {
			t.Fatalf("criterion %d names receipts %v, want only receipt %d", i, got, i)
		}
	}
	recheckPinsAfterVerify(t, r, out+"-recheck")
}

// TestCEMV1575RunnerSetRefusals: each malformed runners request is refused
// with its closed code before any bundle exists (CEM-V1-575, CEM-V1-576).
func TestCEMV1575RunnerSetRefusals(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, caseRoot := multiRunnerFixture(t)
	single, _, _ := plainFixture(t)
	inputs := filepath.Dir(r.SourceMap.Path)
	r0, r1 := r.Runners[0].RunnerReceipt.Sha256, r.Runners[1].RunnerReceipt.Sha256
	// A second go-test pair: the same plan with a longer timeout, and the
	// receipt rebound to it.
	var plan tr.PlanDocument
	var receipt tr.ReceiptDocument
	readJSON(t, filepath.Join(caseRoot, "artifacts", "runner-plan-0.json"), &plan)
	readJSON(t, filepath.Join(caseRoot, "artifacts", "runner-receipt-0.json"), &receipt)
	plan.Request.TimeoutSeconds++
	receipt.PlanSha256 = tr.Identity(plan)
	second := RunnerPair{save(t, filepath.Join(inputs, "go-plan-2.json"), mustJSON(t, plan)), save(t, filepath.Join(inputs, "go-receipt-2.json"), mustJSON(t, receipt))}
	type refusal struct {
		base   PlainRequest
		mutate func(*PlainRequest)
		want   string
	}
	cases := map[string]refusal{
		"single-form-criterion-receipts": {single, func(p *PlainRequest) { p.Criteria[0].RunnerReceiptSha256s = []string{single.RunnerReceipt.Sha256} }, "runner-set-form"},
		"both-forms": {r, func(p *PlainRequest) {
			p.RunnerPlan, p.RunnerReceipt = p.Runners[0].RunnerPlan, p.Runners[0].RunnerReceipt
		}, "runner-set-form"},
		"criterion-without-receipts": {r, func(p *PlainRequest) { p.Criteria[1].RunnerReceiptSha256s = nil }, "runner-set-form"},
		"no-admission":               {r, func(p *PlainRequest) { p.RunnerAdmission = nil }, "runner-admission-required"},
		"empty-runners":              {r, func(p *PlainRequest) { p.Runners = []RunnerPair{} }, "runner-set-bound"},
		"runners-over-bound": {r, func(p *PlainRequest) {
			p.Runners = nil
			for i := 0; i <= MaxRunnerPairs; i++ {
				p.Runners = append(p.Runners, RunnerPair{FileRef{fmt.Sprintf("/absent/plan-%d.json", i), tr.Digest([]byte(fmt.Sprint("plan", i)))}, FileRef{fmt.Sprintf("/absent/receipt-%d.json", i), tr.Digest([]byte(fmt.Sprint("receipt", i)))}})
			}
		}, "runner-set-bound"},
		"criterion-receipts-empty":     {r, func(p *PlainRequest) { p.Criteria[0].RunnerReceiptSha256s = []string{} }, "runner-set-bound"},
		"duplicate-pair":               {r, func(p *PlainRequest) { p.Runners = append(p.Runners, p.Runners[1]) }, "runner-set-duplicate"},
		"duplicate-criterion-receipt":  {r, func(p *PlainRequest) { p.Criteria[0].RunnerReceiptSha256s = []string{r0, r0} }, "runner-set-duplicate"},
		"unresolved-criterion-receipt": {r, func(p *PlainRequest) { p.Criteria[1].RunnerReceiptSha256s = []string{strings.Repeat("0", 64)} }, "runner-set-unresolved"},
		"unused-receipt":               {r, func(p *PlainRequest) { p.Criteria[1].RunnerReceiptSha256s = []string{r0} }, "runner-set-unused"},
		"plans-swapped-between-pairs": {r, func(p *PlainRequest) {
			p.Runners[0].RunnerPlan, p.Runners[1].RunnerPlan = p.Runners[1].RunnerPlan, p.Runners[0].RunnerPlan
		}, "runner-receipt-tampered"},
		"same-profile-for-one-criterion": {r, func(p *PlainRequest) {
			p.Runners = append(p.Runners, second)
			p.Criteria[0].RunnerReceiptSha256s = []string{r0, second.RunnerReceipt.Sha256}
		}, "runner-profile-conflict"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := c.base
			p.Runners = append([]RunnerPair(nil), c.base.Runners...)
			p.Criteria = append([]PlainCriterion(nil), c.base.Criteria...)
			o := out + "-" + name
			c.mutate(&p)
			result, e := AssemblePlain(context.Background(), p, o)
			if e == nil || cemcode.CodeOf(e) != c.want || result.StablePath != "" {
				t.Fatalf("AssemblePlain = %+v, %v (code %q); want %q", result, e, cemcode.CodeOf(e), c.want)
			}
			if _, e := os.Lstat(o); !os.IsNotExist(e) {
				t.Fatalf("refusal left a bundle: %v", e)
			}
		})
	}
	// Two receipts of different profiles may cover one criterion.
	both := r
	both.Criteria = append([]PlainCriterion(nil), r.Criteria...)
	both.Criteria[0].RunnerReceiptSha256s = []string{r0, r1}
	if result, e := AssemblePlain(context.Background(), both, out+"-both"); e != nil || result.Verification.Outcome != "ACCEPT" {
		t.Fatalf("a criterion covered by go-test and node-test was refused: %+v, %v", result, e)
	}
}

// TestCEMV1578TasksBoundRequestHasNoRunnersForm: the Tasks-bound
// cem-stable-assembly/1 request, which writes no cem/1.0 map, keeps its
// closed field set, so a runners member is refused by the decoder.
func TestCEMV1578TasksBoundRequestHasNoRunnersForm(t *testing.T) {
	var s StableRequest
	if e := Decode([]byte(`{"profile":"`+StableAssemblyProfile+`","runners":[]}`), &s); e == nil || !strings.Contains(e.Error(), `unknown field "runners"`) {
		t.Fatalf("Decode = %v, want the runners member refused as unknown", e)
	}
}

// TestCEMV1575NamedMembersOfTheOtherFormAreRefused: a request decoded from
// JSON that names a member of the other runner form, even as null or an empty
// pin, is refused before any read, and a criterion naming more receipts than
// one link can hold is refused at the reader's link bound (CEM-V1-575).
func TestCEMV1575NamedMembersOfTheOtherFormAreRefused(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	r, out, _ := multiRunnerFixture(t)
	single, _, _ := plainFixture(t)
	decoded := func(t *testing.T, p PlainRequest, edit func(map[string]any)) PlainRequest {
		t.Helper()
		var doc map[string]any
		if e := json.Unmarshal(mustJSON(t, p), &doc); e != nil {
			t.Fatal(e)
		}
		edit(doc)
		raw, e := json.Marshal(doc)
		if e != nil {
			t.Fatal(e)
		}
		var q PlainRequest
		if e := Decode(raw, &q); e != nil {
			t.Fatalf("Decode: %v", e)
		}
		return q
	}
	criterion := func(doc map[string]any, i int) map[string]any {
		return doc["criteria"].([]any)[i].(map[string]any)
	}
	over := []string{}
	for i := 0; i <= MaxCriterionReceipts; i++ {
		over = append(over, tr.Digest([]byte(fmt.Sprint("receipt", i))))
	}
	cases := map[string]struct {
		base PlainRequest
		edit func(map[string]any)
	}{
		"single-with-null-runners":       {single, func(d map[string]any) { d["runners"] = nil }},
		"single-criterion-null-receipts": {single, func(d map[string]any) { criterion(d, 0)["runnerReceiptSha256s"] = nil }},
		"runners-with-empty-runner-plan": {r, func(d map[string]any) { d["runnerPlan"] = map[string]any{"path": "", "sha256": ""} }},
		"criterion-receipts-over-bound":  {r, func(d map[string]any) { criterion(d, 0)["runnerReceiptSha256s"] = over }},
	}
	want := map[string]string{"criterion-receipts-over-bound": "runner-set-bound"}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := decoded(t, c.base, c.edit)
			code := want[name]
			if code == "" {
				code = "runner-set-form"
			}
			o := out + "-" + name
			result, e := AssemblePlain(context.Background(), p, o)
			if e == nil || cemcode.CodeOf(e) != code || result.StablePath != "" {
				t.Fatalf("AssemblePlain = %+v, %v (code %q); want %q", result, e, cemcode.CodeOf(e), code)
			}
			if _, e := os.Lstat(o); !os.IsNotExist(e) {
				t.Fatalf("refusal left a bundle: %v", e)
			}
		})
	}
	// The JSON of a Go-built runners request names no single-form member, so
	// it decodes and assembles.
	p := decoded(t, r, func(map[string]any) {})
	if result, e := AssemblePlain(context.Background(), p, out+"-decoded"); e != nil || result.Verification.Outcome != "ACCEPT" {
		t.Fatalf("decoded runners request refused: %+v, %v", result, e)
	}
}
