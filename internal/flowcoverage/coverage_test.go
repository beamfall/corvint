package flowcoverage

import (
	"context"
	jsonv1 "encoding/json"
	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/flowdocs"
	js "github.com/Beamfall/corvint/internal/jstestprovider"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	t                                  *testing.T
	root, source, revision, repository string
	d                                  Denominator
	r                                  Runs
	receipt                            js.Receipt
}

func (f *fixture) git(a ...string) string {
	f.t.Helper()
	c := exec.Command("git", a...)
	c.Dir = f.root
	b, e := c.CombinedOutput()
	if e != nil {
		f.t.Fatalf("git %v: %s %v", a, b, e)
	}
	return strings.TrimSpace(string(b))
}
func (f *fixture) put(p string, b []byte) {
	f.t.Helper()
	if e := os.MkdirAll(filepath.Dir(filepath.Join(f.root, p)), 0700); e != nil {
		f.t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(f.root, p), b, 0600); e != nil {
		f.t.Fatal(e)
	}
}
func (f *fixture) object(p string, v any) {
	f.t.Helper()
	b, e := Encode(v)
	if e != nil {
		f.t.Fatal(e)
	}
	f.put(p, b)
}
func (f *fixture) commit() string {
	f.git("add", "-A")
	f.git("commit", "--allow-empty", "-qm", "fixture")
	return f.git("rev-parse", "HEAD")
}
func (f *fixture) anchor(p string, line int) doccorpus.Anchor {
	b, e := os.ReadFile(filepath.Join(f.root, p))
	if e != nil {
		f.t.Fatal(e)
	}
	lines := strings.SplitAfter(string(b), "\n")
	return doccorpus.Anchor{Repository: f.repository, Revision: f.source, Path: p, Blob: f.git("rev-parse", f.source+":"+p), SHA256: digest(b), Start: line, End: line, SpanSHA256: digest([]byte(lines[line-1])), Authority: "syntax", Kind: "source", Reason: "synthetic unit fixture; no live execution claimed"}
}
func testID(r js.Receipt, t js.TestOutcome) string {
	b, _ := jsonv1.Marshal(struct {
		Identity js.Identity
		Project  *js.ProjectIdentity
		Anchor   *js.Anchor
		FullName string
	}{r.Identity, t.Project, t.Anchor, t.FullName})
	return digest(b)
}
func (f *fixture) publish() {
	f.t.Helper()
	for i := range f.receipt.Tests {
		f.receipt.Tests[i].ID = testID(f.receipt, f.receipt.Tests[i])
	}
	b, e := js.EncodeQualified(f.receipt)
	if e != nil {
		f.t.Fatal(e)
	}
	f.put("receipts/run.json", b)
	f.revision = f.commit()
	f.r.Revision = f.revision
	f.r.Runs[0].Ref = Ref{Path: "receipts/run.json", Revision: f.revision, SHA256: digest(b)}
	f.object("denominator.json", f.d)
	f.object("runs.json", f.r)
	f.commit()
}
func (f *fixture) compile() (*Result, error) {
	return Compile(context.Background(), f.root, Options{Denominator: "denominator.json", Receipts: "runs.json"})
}
func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: root}
	f.git("init", "-q")
	f.git("config", "user.email", "fixture@example.invalid")
	f.git("config", "user.name", "Fixture")
	for p, b := range map[string]string{"app/app.js": "function counter() { return 1; }\n", "test.cjs": "test('pass',()=>{});\ntest('control',()=>{});\n", "config.cjs": "module.exports={};\n", "package.json": "{}\n", "package-lock.json": "{}\n", "acceptance.md": "Fixture owner accepts pass and declared failing control.\n"} {
		f.put(p, []byte(b))
	}
	intent := appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: "counter", Revision: 1, Kind: "ui", Actor: "visitor", Preconditions: []string{}, Steps: []appflows.FlowStep{{StepID: "click", Action: "click counter"}}, Outcomes: []appflows.FlowOutcome{{OutcomeID: "one", Behavior: "counter is one", Matcher: "toHaveText", Locator: "#counter", Value: "1"}}, Variations: []appflows.FlowVariation{{VariationID: "counter.once", Preconditions: []string{}, Steps: []string{"click"}, ObservableFacts: []string{"counter visible"}, Outcomes: []string{"one"}, Projects: []string{}}}, Links: []appflows.FlowLink{}, Navigation: &appflows.FlowNavigation{PreconditionFlows: []string{}, Steps: []appflows.NavStep{{StepID: "click", State: "/", Locator: appflows.NavLocator{Role: "button", Name: "Increment"}, Expect: []string{"one"}}}}}
	f.object("flows/counter.json", intent)
	f.source = f.commit()
	var e error
	f.repository, e = contextindex.CorpusRepositoryID(context.Background(), f.root, f.source)
	if e != nil {
		t.Fatal(e)
	}
	g, e := flowdocs.Generate(context.Background(), f.root, flowdocs.Options{Revision: f.source, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	for p, b := range g.Files {
		f.put("generated/"+p, b)
	}
	genRevision := f.commit()
	acceptance := f.anchor("acceptance.md", 1)
	acceptance.Authority = "source-document"
	acceptance.Kind = "review"
	empty := Citation{Anchors: []doccorpus.Anchor{}, IDs: []string{}, Unavailable: "not available in synthetic fixture"}
	project := ""
	subject := ExactTest{File: "test.cjs", FullTitle: "pass", Project: &project, Anchor: f.anchor("test.cjs", 1), Assertions: []doccorpus.Anchor{f.anchor("test.cjs", 1)}}
	control := subject
	control.FullTitle = "control"
	control.Anchor = f.anchor("test.cjs", 2)
	control.Assertions = []doccorpus.Anchor{control.Anchor}
	f.d = Denominator{Schema: DenominatorSchema, Source: doccorpus.Repository{ID: f.repository, Revision: f.source}, Intents: IntentDirectory{Directory: "flows", Revision: f.source}, Inventory: Inventory{Kind: "generation", Ref: Ref{Path: "generated/generation.json", Revision: genRevision, SHA256: digest(g.Files["generation.json"])}}, Flows: []Mapping{{DocumentedID: g.Manifest.Flows[0].ID, IntentFlowIDs: []string{"counter"}}}, Variations: []Variation{{FlowID: "counter", ID: "counter.once", Citations: Citations{Docs: empty, Claims: empty, SourceBranches: empty, LegacyTests: empty, DownstreamFlows: empty}, Tests: []Test{{ExactTest: subject, Mode: "browser", Outcomes: []string{"one"}, Controls: []ExactTest{control}, Fixtures: []doccorpus.Anchor{}, PageObjects: []doccorpus.Anchor{}, Acceptance: acceptance}}}}}
	config := digest([]byte("module.exports={};\n"))
	test := subject.Anchor.SHA256
	packages := []string{"/fixture/package.json", "/fixture/package-lock.json"}
	combined := ""
	for _, p := range packages {
		combined += p + "=" + digest([]byte("{}\n")) + "\n"
	}
	build := digest([]byte("app.js=" + digest([]byte("function counter() { return 1; }\n")) + "\n"))
	f.receipt = js.Receipt{Profile: js.AttemptExternalProfile, Kind: "e2e", Identity: js.Identity{ConfigFile: "/fixture/config.cjs", ConfigDigest: config, ConfigInputDigests: map[string]string{"/fixture/config.cjs": config}, TestFileDigests: map[string]string{"/fixture/test.cjs": test}, PackageDigest: digest([]byte(combined)), NodeVersion: "v22.23.2", RunnerName: "playwright", RunnerVersion: "1.63.0", Environment: map[string]string{}, Argv: []string{"node", "playwright"}}, External: &js.ExternalLifecycle{ReadyURL: "http://127.0.0.1:8000", DeclaredAppIdentity: f.source, Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true, ConfigOverride: "synthetic fixture"}, AppBuildAtStart: js.AppBuildIdentity{Digest: build}, AppBuildAtPublish: js.AppBuildIdentity{Digest: build}, Schedule: &js.ExecutionSchedule{Workers: 1}}
	use := jsonv1.RawMessage(`{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)
	for i, name := range []string{"pass", "control"} {
		state, kind := js.StatePassed, "none"
		if i == 1 {
			state, kind = js.StateFailed, "assertion-or-test"
		}
		a := &js.Anchor{File: "/fixture/test.cjs", Line: i + 1}
		o := js.TestOutcome{Name: name, FullName: name, State: state, Anchor: a, Project: &js.ProjectIdentity{Name: "", Browser: "chromium", Device: "unknown", Use: use, ConfigDigest: config}, Attempts: []js.Attempt{{State: state, FailureKind: kind}}, AttemptDetails: []js.AttemptDetail{{State: state, Anchor: a}}}
		f.receipt.Tests = append(f.receipt.Tests, o)
		f.receipt.Schedule.Starts = append(f.receipt.Schedule.Starts, js.ExecutionStart{FullName: name, File: a.File, Line: a.Line, Project: ""})
	}
	f.r = Runs{Schema: RunsSchema, Directory: "receipts", Runs: []Run{{Kind: "planned", SourceRevision: f.source, TestRevision: f.source, Paths: map[string]string{"/fixture/config.cjs": "config.cjs", "/fixture/test.cjs": "test.cjs", "/fixture/package.json": "package.json", "/fixture/package-lock.json": "package-lock.json"}, PackagePaths: packages, BuildScope: "app", ApplicationIdentity: f.source, Environment: map[string]string{}}}}
	f.publish()
	return f
}
func TestFlowCoverageBoundProofAndWriteback(t *testing.T) {
	f := newFixture(t)
	r, e := f.compile()
	if e != nil {
		t.Fatal(e)
	}
	if !r.Report.Passed || r.Report.Rows[0].Status != "PROVEN" {
		t.Fatalf("status=%s reasons=%v observations=%v", r.Report.Rows[0].Status, r.Report.Rows[0].Reasons, r.Report.Rows[0].Observations[0].Reasons)
	}
	parent, _ := filepath.EvalSymlinks(t.TempDir())
	dir := filepath.Join(parent, "docs")
	if e = r.Write(dir); e != nil {
		t.Fatal(e)
	}
	if e = r.Write(dir); e == nil {
		t.Fatal("clobber")
	}
	diff, e := r.Check(dir)
	if e != nil || len(diff) != 0 {
		t.Fatal(diff, e)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "coverage-manifest.json"))
	if !strings.Contains(string(b), "source-generation.json") {
		t.Fatal("missing source manifest binding")
	}
	files, _ := r.Files()
	for p, b := range files {
		if strings.HasSuffix(p, "functional-overview.md") && !strings.Contains(string(b), "E2E coverage") {
			t.Fatal("coverage missing")
		}
	}
	f.put("app/new.js", []byte("new\n"))
	r, e = f.compile()
	if e != nil || r.Report.Rows[0].Status != "STALE" {
		t.Fatal(r, e)
	}
}
func TestFlowCoverageInventoryFreshness(t *testing.T) {
	for _, p := range []string{"receipts/manual.json", "flows/new.json", "generated/generation.json"} {
		t.Run(p, func(t *testing.T) {
			f := newFixture(t)
			f.put(p, []byte("{}"))
			r, e := f.compile()
			if e == nil && r.Report.Passed {
				t.Fatal("dirty inventory addition/change went green")
			}
			f.commit()
			r, e = f.compile()
			if e == nil && r.Report.Passed {
				t.Fatal("committed inventory addition/change went green")
			}
		})
	}
}
func TestFlowCoverageRetryAndInfrastructure(t *testing.T) {
	for _, kind := range []string{"configured", "schedule", "infrastructure", "build"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			switch kind {
			case "configured":
				f.receipt.Schedule.Starts[0].Retries = 1
			case "schedule":
				f.receipt.Schedule.Starts = f.receipt.Schedule.Starts[:1]
			case "infrastructure":
				f.receipt.Tests[1].Attempts[0].FailureKind = "infrastructure"
			case "build":
				f.r.Runs[0].BuildScope = ""
			}
			f.publish()
			r, e := f.compile()
			if e != nil {
				t.Fatal(e)
			}
			if r.Report.Passed || r.Report.Rows[0].Status != "MISSING_TEST" {
				t.Fatal(r.Report.Rows[0].Status, r.Report.Rows[0].Reasons)
			}
		})
	}
}
func TestFlowCoverageDenominatorOmissions(t *testing.T) {
	for _, kind := range []string{"flow", "variation", "project", "entry"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			switch kind {
			case "flow":
				f.d.Flows = nil
			case "variation":
				f.d.Variations = nil
			case "project":
				f.d.Variations[0].Tests[0].Project = nil
			case "entry":
				f.d.Variations[0].Tests[0].Controls = nil
			}
			f.object("denominator.json", f.d)
			f.commit()
			r, e := f.compile()
			if e == nil && r.Report.Passed {
				t.Fatal("incomplete denominator green")
			}
		})
	}
}

func TestFlowCoverageGroupHistoryAndPaging(t *testing.T) {
	for _, kind := range []string{"incompatible", "conflict", "historical", "metadata-source", "metadata-test", "relabel", "changed-test"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			old := f.receipt
			lifecycle := *old.External
			old.External = &lifecycle
			old.Tests = append([]js.TestOutcome{}, f.receipt.Tests...)
			for i := range old.Tests {
				old.Tests[i].Attempts = append([]js.Attempt{}, old.Tests[i].Attempts...)
				old.Tests[i].AttemptDetails = append([]js.AttemptDetail{}, old.Tests[i].AttemptDetails...)
			}
			old.Tests[0].State = js.StateFailed
			old.Tests[0].Attempts[0].State = js.StateFailed
			old.Tests[0].Attempts[0].FailureKind = "assertion-or-test"
			old.Tests[0].AttemptDetails[0].State = js.StateFailed
			extra := f.r.Runs[0]
			extra.Kind = "manual"
			if kind == "historical" {
				oldSource := f.source
				f.put("app/unrelated.js", []byte("// new build\n"))
				f.source = f.commit()
				f.d.Source.Revision = f.source
				f.r.Runs[0].SourceRevision = f.source
				f.r.Runs[0].ApplicationIdentity = f.source
				f.receipt.External.DeclaredAppIdentity = f.source
				build := digest([]byte("app.js=" + digest([]byte("function counter() { return 1; }\n")) + "\nunrelated.js=" + digest([]byte("// new build\n")) + "\n"))
				f.receipt.AppBuildAtStart.Digest = build
				f.receipt.AppBuildAtPublish.Digest = build
				extra.SourceRevision = oldSource
				g, e := flowdocs.Generate(context.Background(), f.root, flowdocs.Options{Revision: f.source, Scope: "app"})
				if e != nil {
					t.Fatal(e)
				}
				for p, b := range g.Files {
					f.put("generated/"+p, b)
				}
				f.d.Inventory.Revision = f.commit()
				f.d.Inventory.SHA256 = digest(g.Files["generation.json"])
			}
			if kind == "metadata-source" || kind == "metadata-test" || kind == "relabel" {
				f.put("unrelated.md", []byte("metadata-only commit\n"))
				metadata := f.commit()
				if kind == "metadata-source" {
					f.r.Runs[0].SourceRevision = metadata
					f.r.Runs[0].ApplicationIdentity = metadata
					f.receipt.External.DeclaredAppIdentity = metadata
				}
				if kind == "metadata-test" {
					f.r.Runs[0].TestRevision = metadata
				}
				if kind == "relabel" {
					f.r.Runs[0].ApplicationIdentity = "new-label"
					f.receipt.External.DeclaredAppIdentity = "new-label"
				}
			}
			if kind == "changed-test" {
				f.put("test.cjs", []byte("test('pass',()=>{ /* repaired assertion */ });\ntest('control',()=>{});\n"))
				f.source = f.commit()
				f.r.Runs[0].TestRevision = f.source
				bytes, _ := os.ReadFile(filepath.Join(f.root, "test.cjs"))
				f.receipt.Identity.TestFileDigests = map[string]string{"/fixture/test.cjs": digest(bytes)}
				contract := &f.d.Variations[0].Tests[0]
				contract.Anchor = f.anchor("test.cjs", 1)
				contract.Assertions = []doccorpus.Anchor{contract.Anchor}
				contract.Controls[0].Anchor = f.anchor("test.cjs", 2)
				contract.Controls[0].Assertions = []doccorpus.Anchor{contract.Controls[0].Anchor}
			}

			if kind == "incompatible" {
				f.receipt.Tests = f.receipt.Tests[:1]
				f.receipt.Schedule.Starts = f.receipt.Schedule.Starts[:1]
				old.Tests = old.Tests[1:]
				old.Schedule = &js.ExecutionSchedule{Workers: 1, Starts: []js.ExecutionStart{{FullName: "control", File: "/fixture/test.cjs", Line: 2}}}
				old.Identity.Environment = map[string]string{"MODE": "other"}
				extra.Environment = old.Identity.Environment
			}
			for i := range old.Tests {
				old.Tests[i].ID = testID(old, old.Tests[i])
			}
			b, e := js.EncodeQualified(old)
			if e != nil {
				t.Fatal(e)
			}
			f.put("receipts/manual.json", b)
			extra.Ref = Ref{Path: "receipts/manual.json", SHA256: digest(b)}
			f.publish()
			extra.Revision = f.r.Revision
			f.r.Runs = append(f.r.Runs, extra)
			f.object("runs.json", f.r)
			f.commit()
			r, e := f.compile()
			if e != nil {
				t.Fatal(e)
			}
			want := "FLAKY"
			if kind == "incompatible" {
				want = "MISSING_TEST"
			}
			if kind == "historical" || kind == "changed-test" {
				want = "PROVEN"
			}
			if r.Report.Rows[0].Status != want {
				t.Fatal(r.Report.Rows[0].Status, r.Report.Rows[0].Reasons)
			}
		})
	}
	f := newFixture(t)
	r, e := f.compile()
	if e != nil {
		t.Fatal(e)
	}
	r.Report.Rows = append(r.Report.Rows, Row{DocumentedID: "second", Status: "MISSING_TEST"})
	r.Report.Passed = false
	r.Report.Gaps = 1
	b, e := r.Page(0, 1)
	if e != nil {
		t.Fatal(e)
	}
	var p Page
	if e = decode(b, &p); e != nil || p.Passed || p.Gaps != 1 || p.Total != 2 || p.Next == nil {
		t.Fatal(p, e)
	}
}
func TestFlowCoverageExclusionAndEntryAuthority(t *testing.T) {
	for _, kind := range []string{"wrong-source", "proposed", "entry"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			set, e := appflows.LoadIntentsAt(context.Background(), f.root, "flows", f.source)
			if e != nil {
				t.Fatal(e)
			}
			flow := set.Flows[0]
			if kind == "proposed" {
				flow.Proposed = true
				f.object("flows/counter.json", flow)
				f.d.Intents.Revision = f.commit()
			}
			if kind == "entry" {
				flow.Navigation = nil
				f.object("flows/counter.json", flow)
				f.d.Intents.Revision = f.commit()
				f.d.Variations[0].Entry = &Entry{Acceptance: f.d.Variations[0].Tests[0].Acceptance}
			} else {
				matrix, _ := Encode(flow)
				revision := f.source
				if kind == "wrong-source" {
					revision = f.revision
				}
				sign := map[string]any{"schema": "application-flow-coverage-exclusion/1", "flow_id": "counter", "variation_id": "counter.once", "matrix_sha256": digest(matrix), "source_revision": revision, "author": "fixture owner", "rationale": "accepted fixture exclusion", "generated": false, "accepted": true}
				b, _ := Encode(sign)
				f.put("exclude.json", b)
				rev := f.commit()
				f.d.Variations[0].Exclusion = &Ref{Path: "exclude.json", Revision: rev, SHA256: digest(b)}
			}
			f.object("denominator.json", f.d)
			f.commit()
			r, e := f.compile()
			if kind == "entry" {
				if e == nil {
					t.Fatal("empty locator accepted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if r.Report.Passed {
				t.Fatal("authority gap went green")
			}
		})
	}
}

func TestFlowCoverageStagedAndCitedSource(t *testing.T) {
	f := newFixture(t)
	original, e := os.ReadFile(filepath.Join(f.root, "test.cjs"))
	if e != nil {
		t.Fatal(e)
	}
	f.put("test.cjs", []byte("changed\n"))
	f.git("add", "test.cjs")
	f.put("test.cjs", original)
	r, e := f.compile()
	if e != nil || r.Report.Rows[0].Status != "STALE" {
		t.Fatal("staged changed input", e)
	}
	f.git("reset", "--quiet", "HEAD", "test.cjs")
	f.put("app/app.js", []byte("function counter() { return 2; }\n"))
	r, e = f.compile()
	if e != nil || r.Report.Rows[0].Status != "STALE" {
		t.Fatal("generated source change", e)
	}
}
func TestFlowCoverageActualRetryAndAPI(t *testing.T) {
	f := newFixture(t)
	o := &f.receipt.Tests[0]
	o.State = js.StateFlaky
	o.Retries = 1
	o.Attempts = []js.Attempt{{State: js.StateFailed, Retry: 0, FailureKind: "assertion-or-test"}, {State: js.StatePassed, Retry: 1, FailureKind: "none"}}
	o.AttemptDetails = []js.AttemptDetail{{State: js.StateFailed, Retry: 0, Anchor: o.Anchor}, {State: js.StatePassed, Retry: 1, Anchor: o.Anchor}}
	f.receipt.Schedule.Starts = append(f.receipt.Schedule.Starts, js.ExecutionStart{File: o.Anchor.File, FullName: o.FullName, Line: o.Anchor.Line, Project: "", Retry: 1, Retries: 1})
	f.receipt.Schedule.Starts[0].Retries = 1
	f.publish()
	r, e := f.compile()
	if e != nil || r.Report.Rows[0].Status != "FLAKY" {
		t.Fatal("retry", e)
	}
	f = newFixture(t)
	set, e := appflows.LoadIntentsAt(context.Background(), f.root, "flows", f.source)
	if e != nil {
		t.Fatal(e)
	}
	flow := set.Flows[0]
	flow.Kind = "api"
	flow.Navigation.Steps[0].State = ""
	flow.Navigation.Steps[0].Locator = appflows.NavLocator{Method: "GET", Path: "/counter"}
	f.object("flows/counter.json", flow)
	f.d.Intents.Revision = f.commit()
	f.d.Variations[0].Tests[0].Mode = "api"
	f.object("denominator.json", f.d)
	f.commit()
	r, e = f.compile()
	if e != nil || r.Report.Rows[0].Status != "API_PROVEN" {
		t.Fatal("accepted API proof", e)
	}
	f.d.Variations[0].Tests[0].Mode = "browser"
	f.object("denominator.json", f.d)
	f.commit()
	if _, e = f.compile(); e == nil {
		t.Fatal("browser claim for accepted API")
	}
}
