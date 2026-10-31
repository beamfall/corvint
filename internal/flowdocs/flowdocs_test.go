package flowdocs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func fixture(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Fixture")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	for p, v := range files {
		if e = os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0700); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(root, p), []byte(v), 0600); e != nil {
			t.Fatal(e)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	return root, git(t, root, "rev-parse", "HEAD")
}
func TestFlowDocsLexicalFalsePositives(t *testing.T) {
	cases := []struct {
		path, text    string
		flows, claims int
	}{
		{"app/test.rb", "# def Fake\ntext = <<~DOC\ndef Fake\n authorize(user)\nend\nDOC\nclass C\n def real\n  text = 'raise Secret'\n  # save!\n  authorize(user)\n end\nend\n", 1, 1},
		{"app/test.tsx", "/* function Fake() { save(); } */\nconst secret = `function Fake() {\n save();\n}`;\nexport function Real() {\n const pattern = /save()/;\n const text = 'authorize(user)';\n return ready && <div>\nfunction Fake() save();\n </div>;\n}\n", 1, 1},
		{"app/test.html", "<!-- <div ng-if='secret'> -->\n<script>const t = \"<div ng-if='bad'>\";</script>\n<div id='visible' ng-if='ready'>text ng-if='notattribute'</div>\n", 1, 1},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			ds, _ := extract(contextindex.Source{Path: c.path, Data: []byte(c.text)})
			if len(ds) != c.flows {
				t.Fatalf("declarations=%d expected=%d", len(ds), c.flows)
			}
			n := 0
			for _, d := range ds {
				n += len(d.observations)
			}
			if n != c.claims {
				t.Fatalf("observations=%d expected=%d: %+v", n, c.claims, ds)
			}
		})
	}
}
func TestFlowDocsScaleDeterminismSafetyAndGitlink(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 150; i++ {
		switch i % 3 {
		case 0:
			files[fmt.Sprintf("app/f%03d.rb", i)] = fmt.Sprintf("class C%d\n def run\n  authorize(user)\n  save!\n end\nend\n", i)
		case 1:
			files[fmt.Sprintf("app/f%03d.tsx", i)] = fmt.Sprintf("export function View%d() {\n fetch('/api');\n return ready && <div>OK</div>;\n}\n", i)
		case 2:
			files[fmt.Sprintf("app/f%03d.js", i)] = fmt.Sprintf("angular.module('app').controller('Controller%d', function() {});\n", i)
		}
	}
	root, rev := fixture(t, files)
	git(t, root, "update-index", "--add", "--cacheinfo", "160000,"+rev+",app/opaque")
	git(t, root, "commit", "-qm", "opaque gitlink")
	rev = git(t, root, "rev-parse", "HEAD")
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Manifest.Flows) != 150 || len(r.Files) != 1202 {
		t.Fatalf("scale counts: %d %d", len(r.Manifest.Flows), len(r.Files))
	}
	out := filepath.Join(root, "output")
	if e = Materialize(out, r); e != nil {
		t.Fatal(e)
	}
	before := git(t, root, "status", "--porcelain")
	check, e := CheckRevision(context.Background(), root, r.Files["generation.json"], rev, out)
	if e != nil || !check.Clean {
		t.Fatalf("determinism: %v", e)
	}
	if git(t, root, "status", "--porcelain") != before {
		t.Fatal("check mutated repository")
	}
	if len(r.Manifest.OpaqueGitlinks) != 1 || r.Manifest.OpaqueGitlinks[0].Path != "app/opaque" {
		t.Fatal("gitlink identity lost")
	}
	git(t, root, "add", "output")
	git(t, root, "commit", "-qm", "generated provider with opaque gitlink")
	finalized, e := Finalize(context.Background(), root, r, git(t, root, "rev-parse", "HEAD"), "output/provider.json")
	if e != nil {
		t.Fatal(e)
	}
	artifact, e := doccorpus.Build(context.Background(), root, finalized)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := doccorpus.Encode(artifact)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = doccorpus.Open(context.Background(), root, encoded); e != nil {
		t.Fatal(e)
	}
	for _, in := range finalized.Inputs {
		if in.Path == "app/opaque" || strings.HasPrefix(in.Path, "app/opaque/") {
			t.Fatal("gitlink contents became corpus input")
		}
	}
	foundOpaque := false
	for _, subject := range artifact.Subjects {
		if subject.ID == identity("gitlink", "app/opaque") {
			foundOpaque = subject.Evidence.State == "unknown" && strings.Contains(subject.Name, r.Manifest.OpaqueGitlinks[0].Commit)
		}
	}
	if !foundOpaque {
		t.Fatal("corpus lost opaque commit identity/uncertainty")
	}
	nextLink := git(t, root, "rev-parse", "HEAD")
	git(t, root, "update-index", "--cacheinfo", "160000,"+nextLink+",app/opaque")
	git(t, root, "commit", "-qm", "changed opaque gitlink identity")
	drift, e := CheckRevision(context.Background(), root, r.Files["generation.json"], git(t, root, "rev-parse", "HEAD"), out)
	if e != nil {
		t.Fatal(e)
	}
	if drift.Clean || len(drift.OpaqueChanges) != 1 || drift.OpaqueChanges[0].After != nextLink || len(drift.Added) > 0 || len(drift.Retired) > 0 {
		t.Fatal("opaque identity drift lost or nested contents inferred")
	}

	if e = Materialize(out, r); e == nil {
		t.Fatal("clobbered existing directory")
	}
	link := filepath.Join(root, "link")
	if e = os.Symlink(out, link); e != nil {
		t.Fatal(e)
	}
	if e = Materialize(filepath.Join(link, "nested"), r); e == nil {
		t.Fatal("followed symlink ancestor")
	}
	if e = os.WriteFile(filepath.Join(out, "provider.json"), []byte("human edit"), 0600); e != nil {
		t.Fatal(e)
	}
	diff, e := CompareOutput(out, r.Files)
	if e != nil || len(diff) != 1 || diff[0] != "provider.json" {
		t.Fatalf("missed changed bytes: %v %v", diff, e)
	}
	raw := strings.Replace(string(r.Files["generation.json"]), "Source declares", "Forged declares", 1)
	if _, e = Open(context.Background(), root, []byte(raw)); e == nil {
		t.Fatal("forged predecessor accepted")
	}
	t.Logf("150 flows; 1200 Markdown pages; %d provider claims; opaque gitlink; deterministic read-only check", len(r.Manifest.Provider.Claims))
}
func TestFlowDocsNoRawSourceSecrets(t *testing.T) {
	root, rev := fixture(t, map[string]string{"app/view.tsx": "export function View() {\n const secret = 'secret-canary-value';\n fetch('secret-canary-value');\n return ready && <div>secret-canary-value</div>;\n}\n"})
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	for name, raw := range r.Files {
		if strings.Contains(string(raw), "secret-canary-value") {
			t.Fatalf("literal leaked into %s", name)
		}
	}
}

func TestFlowDocsConstructsDependenciesAndRetirement(t *testing.T) {
	root, rev := fixture(t, map[string]string{
		"app/model.rb":     "class Model\n validates :name, presence: true\n def perform\n  authorize(user)\n  save!\n end\nend\n",
		"app/view.html":    "<div ng-controller='View' id='visible' ng-if='ready'>Visible</div>\n",
		"app/web.ts":       "export function first() {\n second();\n}\nexport function second() {\n throw new Error('private text');\n}\n",
		"app/directive.js": "angular.module('app').directive('counter', function() {\n fetch('/api');\n return {};\n});\n",
	})
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	kinds := map[string]int{}
	for _, d := range r.Manifest.Provider.Details {
		kinds[d.ClaimKind]++
	}
	if kinds["constraint"] < 2 || kinds["permission"] < 1 || kinds["side_effect"] < 1 || kinds["integration"] < 1 || kinds["behavior"] < 2 {
		t.Fatalf("construct gaps: %v", kinds)
	}
	dependencies := 0
	for _, rel := range r.Manifest.Provider.Relations {
		if rel.Type == "depends_on" {
			dependencies++
		}
	}
	if dependencies != 1 {
		t.Fatalf("static dependencies=%d", dependencies)
	}
	for _, p := range []string{"model.rb", "view.html", "web.ts", "directive.js"} {
		if e = os.WriteFile(filepath.Join(root, "app", p), []byte("\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	git(t, root, "add", "app")
	git(t, root, "commit", "-qm", "retired all declarations")
	checked, e := CheckRevision(context.Background(), root, r.Files["generation.json"], git(t, root, "rev-parse", "HEAD"), "")
	if e != nil {
		t.Fatal(e)
	}
	if checked.Clean || len(checked.Retired) == 0 || len(checked.Added) > 0 {
		t.Fatal("removals were not retained as retirement")
	}
}

func TestFlowDocsRevalidatedCorpusContext(t *testing.T) {
	ctx := context.Background()
	root, rev := fixture(t, map[string]string{"app/view.ts": "export function View() {\n fetch('/api');\n}\n"})
	generated, e := Generate(ctx, root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	ev := evidence(generated.Manifest.Flows[0].Paragraphs[0].Anchor)
	provider := doccorpus.ProviderRecord{Schema: doccorpus.AdoptionProviderSchema, ID: "original", Version: "1", Source: generated.Manifest.Source, Subjects: []doccorpus.Subject{
		{ID: "original:screen", Kind: "ui_surface", Name: "screen", Provider: "original", Evidence: ev},
		{ID: "original:test", Kind: "test", Name: "test", Provider: "original", Evidence: ev},
		{ID: "original:coverage", Kind: "coverage_definition", Name: "coverage", Provider: "original", Evidence: ev},
		{ID: "original:ticket", Kind: "ticket", Name: "ticket", Provider: "original", Evidence: ev},
		{ID: "original:intent", Kind: "intent_comparison", Name: "intent", Provider: "original", Evidence: ev},
	}, Claims: []doccorpus.Claim{{ID: "original:claim", Subject: "original:screen", Text: "Declared fixture behavior, not runtime proof", Provider: "original", Evidence: ev}}, Relations: []doccorpus.Relation{{ID: "original:join", From: "original:screen", To: "original:test", Type: "navigates", Provider: "original", Evidence: ev}}, Journeys: []doccorpus.Journey{{ID: "original:journey", Subject: "original:screen", Provider: "original", Status: "generated_not_verified", Cleanup: "not-run", Steps: []doccorpus.Step{{ID: "original:step", Action: "navigate", Operation: "click", Expected: "counter", Evidence: ev}}, Evidence: ev}}, Details: map[string]doccorpus.RecordDetails{
		"original:claim":    {ClaimKind: "behavior"},
		"original:join":     {TestLink: &doccorpus.TestLinkDetails{Role: "navigating", JoinConfidence: "declared", File: "app/view.ts", Title: "<script>alert(1)</script>", Project: "fixture"}},
		"original:coverage": {Coverage: &doccorpus.CoverageDetails{Definition: "fixture membership", Denominator: []string{"original:claim"}, Numerator: []string{}, Rule: "explicit-membership"}},
		"original:ticket":   {Ticket: &doccorpus.TicketDetails{ExternalID: "APP-1", Status: "open", History: []doccorpus.TicketHistory{{At: "2026-09-28T00:00:00Z", Revision: rev, Summary: "historical fixture"}}}},
		"original:intent":   {IntentComparison: &doccorpus.IntentComparisonDetails{ReportedIntent: []string{"original:screen"}, ObservedClaims: []string{"original:claim"}, Missing: []string{}, Conflicts: []string{}}},
	}, RestrictedFindings: []doccorpus.RestrictedFinding{{ID: "private-canary", Severity: "high", LocalDetailPath: "private/canary.txt", Detail: "private-canary-detail"}}}
	for _, name := range []string{"subjects", "claims", "relations", "journeys"} {
		provider.Capabilities = append(provider.Capabilities, doccorpus.CapabilityDeclaration{Name: name, State: "present", Reason: "declared fixture, not runtime proof"})
	}
	raw, e := doccorpus.Encode(provider)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "original.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "original.json")
	git(t, root, "commit", "-qm", "original provider")
	providerRev := git(t, root, "rev-parse", "HEAD")
	m, e := doccorpus.Inventory(ctx, root, rev, "app", "2026-09-28T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	m.Schema = doccorpus.ManifestSchemaV2
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "original", Kind: "records", Version: "1", Revision: providerRev, Record: "original.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "original.json", Revision: providerRev})
	m.Inputs = append(m.Inputs, doccorpus.Input{Path: "original.json", Revision: providerRev, Blob: git(t, root, "rev-parse", providerRev+":original.json"), SHA256: doccorpus.Digest(raw), Provider: "original", Purpose: "provider"})
	artifact, e := doccorpus.Build(ctx, root, m)
	if e != nil {
		t.Fatal(e)
	}
	corpus, e := doccorpus.Encode(artifact)
	if e != nil {
		t.Fatal(e)
	}
	r, e := Generate(ctx, root, Options{Revision: rev, Scope: "app", Corpus: corpus})
	if e != nil {
		t.Fatal(e)
	}
	text := ""
	for _, raw := range r.Files {
		text += string(raw)
	}
	for _, expected := range []string{"original:journey", "navigating", "declared", "APP-1", "explicit-membership", "high"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("lost context %s", expected)
		}
	}
	if strings.Contains(text, "private-canary") || strings.Contains(text, "private/canary.txt") {
		t.Fatal("restricted detail escaped")
	}
	for name, raw := range r.Files {
		if strings.HasSuffix(name, ".md") && strings.Contains(string(raw), "<script>") {
			t.Fatal("unescaped corpus title")
		}
	}
	if _, e = Open(ctx, root, r.Files["generation.json"]); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "generated.json"), r.Files["provider.json"], 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "generated.json")
	git(t, root, "commit", "-qm", "generated provider")
	finalized, e := Finalize(ctx, root, r, git(t, root, "rev-parse", "HEAD"), "generated.json")
	if e != nil {
		t.Fatal(e)
	}
	if len(finalized.Providers) != 3 || finalized.Repository.Revision != rev {
		t.Fatal("lost original corpus or source identity")
	}

	if e = os.WriteFile(filepath.Join(root, "app/view.ts"), []byte("export function View(props) {\n fetch('/api');\n}\n"), 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "app")
	git(t, root, "commit", "-qm", "changed imported binding")
	checked, e := CheckRevision(ctx, root, r.Files["generation.json"], git(t, root, "rev-parse", "HEAD"), "")
	if e != nil {
		t.Fatal(e)
	}
	imported := map[string]bool{}
	for _, f := range r.Manifest.Flows {
		for _, p := range f.Paragraphs {
			if p.Imported {
				imported[p.ID] = true
			}
		}
	}
	if len(imported) < 6 {
		t.Fatal("imported prose has no stable paragraphs")
	}
	for _, row := range checked.Bindings {
		if imported[row.ID] && row.State != "stale" {
			t.Fatalf("imported paragraph not stale: %s %s", row.ID, row.State)
		}
	}
}

func TestFlowDocsDeclarationTokensAreNotCalls(t *testing.T) {
	ds, _ := extract(contextindex.Source{Path: "app/functions.js", Data: []byte("function save() {}\nfunction authorize() { fetch('/api'); }\nfunction fetch() {}\n")})
	count := 0
	for _, d := range ds {
		for _, o := range d.observations {
			if d.name != "authorize" || o.key != "integration:fetch" {
				t.Fatalf("declaration token became call: %+v", o)
			}
			count++
		}
	}
	if count != 1 {
		t.Fatalf("lost one-line actual body call: %d", count)
	}
}
func TestFlowDocsAddedAndAllAnchorDrift(t *testing.T) {
	root, rev := fixture(t, map[string]string{"app/controller.rb": "class Controller\n def create\n  authorize(user)\n end\nend\n"})
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "app/controller.rb"), []byte("class Controller\n def create(params)\n  authorize(user)\n end\n def added\n end\nend\n"), 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "app")
	git(t, root, "commit", "-qm", "signature and added method")
	check, e := CheckRevision(context.Background(), root, r.Files["generation.json"], git(t, root, "rev-parse", "HEAD"), "")
	if e != nil {
		t.Fatal(e)
	}
	if check.Clean || len(check.Added) == 0 {
		t.Fatal("new flow did not produce drift")
	}
	id := r.Manifest.Provider.Claims[0].ID
	found := false
	for _, b := range check.Bindings {
		if b.ID == id {
			found = true
			if b.State != "stale" || len(b.Evidence) != 2 || b.Evidence[0].State != "fresh" || b.Evidence[1].State != "stale" {
				t.Fatalf("ignored declaration anchor: %+v", b)
			}
		}
	}
	if !found {
		t.Fatal("claim missing")
	}
}
func TestFlowDocsMultilineHTMLAttributeDrift(t *testing.T) {
	text := "<div\n id='counter'\n ng-if='ready\n && allowed'>Visible</div>\n"
	root, rev := fixture(t, map[string]string{"app/view.html": text})
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Manifest.Provider.Claims) != 1 {
		t.Fatal("missing HTML binding")
	}
	a := r.Manifest.Provider.Claims[0].Evidence.Anchors[0]
	if a.Start != 3 || a.End != 4 {
		t.Fatalf("wrong attribute extent: %+v", a)
	}
	if e = os.WriteFile(filepath.Join(root, "app/view.html"), []byte(strings.Replace(text, "allowed", "denied", 1)), 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "app")
	git(t, root, "commit", "-qm", "changed multiline attribute")
	check, e := CheckRevision(context.Background(), root, r.Files["generation.json"], git(t, root, "rev-parse", "HEAD"), "")
	if e != nil {
		t.Fatal(e)
	}
	stale := 0
	for _, b := range check.Bindings {
		if b.State == "stale" {
			stale++
		}
	}
	if check.Clean || stale != 2 {
		t.Fatalf("missed attribute drift: %d", stale)
	}
}

func TestFlowDocsDirectorySwapCannotEscape(t *testing.T) {
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	inside := filepath.Join(base, "inside")
	outside := filepath.Join(base, "outside")
	parked := filepath.Join(base, "parked")
	for _, p := range []string{inside, outside} {
		if e = os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e = os.WriteFile(filepath.Join(inside, "identity"), []byte("inside"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(outside, "identity"), []byte("outside"), 0600); e != nil {
		t.Fatal(e)
	}
	stop, done := make(chan struct{}), make(chan struct{})
	defer func() { close(stop); <-done }()
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if os.Rename(inside, parked) != nil {
				continue
			}
			if os.Symlink(outside, inside) == nil {
				os.Remove(inside)
			}
			os.Rename(parked, inside)
		}
	}()
	for i := 0; i < 200; i++ {
		r, e := openDirectory(inside)
		if e != nil {
			continue
		}
		b, e := readRoot(r, "identity", 64)
		r.Close()
		if e == nil && string(b) != "inside" {
			t.Fatal("symlink swap escaped opened ancestor identity")
		}
	}
}
func TestFlowDocsDiagramsReflectStaticCallInventory(t *testing.T) {
	root, rev := fixture(t, map[string]string{"app/functions.js": "function first() { fetch('/api'); }\nfunction second() { authorize(user); }\n"})
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	diagrams := []string{}
	for name, b := range r.Files {
		if strings.HasSuffix(name, "/flow-diagram.md") {
			diagrams = append(diagrams, string(b))
		}
	}
	if len(diagrams) != 2 || diagrams[0] == diagrams[1] {
		t.Fatal("identical placeholder diagrams")
	}
	text := strings.Join(diagrams, "\n")
	for _, part := range []string{"call fetch", "call authorize", "order unresolved"} {
		if !strings.Contains(text, part) {
			t.Fatalf("missing static diagram inventory %s", part)
		}
	}
}

func TestFlowDocsCorpusEnvelopeBounds(t *testing.T) {
	if encodingLimit(Manifest{}) != 256<<20 || encodingLimit(doccorpus.ProviderRecord{}) != 64<<20 || MaxOutputBytes != 320<<20 {
		t.Fatal("profile ceilings drifted")
	}
	// Reuse the measured 339-size boundary with sparse bytes rather than a second
	// large typed corpus fixture. Oversized inputs must fail before reading.
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	name := filepath.Join(root, "corpus.json")
	f, e := os.Create(name)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(93_134_239); e != nil {
		t.Fatal(e)
	}
	f.Close()
	r, e := openDirectory(root)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	st, e := r.Stat("corpus.json")
	if e != nil || st.Size() > int64(doccorpus.MaxCorpusBytes) || st.Size() <= int64(MaxBytes) {
		t.Fatal("339 retained corpus would exceed consumer bound")
	}
	// The actual bounded reader must admit bytes above the old64MiB ceiling.
	data, e := ReadCorpusFile(name)
	if e != nil || len(data) != 93_134_239 {
		t.Fatalf("339-sized input refused: %v", e)
	}
	data = nil
	f, e = os.OpenFile(name, os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(doccorpus.MaxCorpusBytes + 1); e != nil {
		t.Fatal(e)
	}
	f.Close()
	if _, e = ReadCorpusFile(name); e == nil {
		t.Fatal("oversized corpus admitted")
	}
	f, e = os.OpenFile(name, os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(MaxManifestBytes + 1); e != nil {
		t.Fatal(e)
	}
	f.Close()
	if _, e = ReadFile(name); e == nil {
		t.Fatal("oversized generation manifest admitted")
	}
}

func TestFlowDocsUnchangedRepeatedImportedSpan(t *testing.T) {
	root, rev := fixture(t, map[string]string{"app/view.ts": "export function View() {\n fetch('/api');\n fetch('/api');\n}\n"})
	ctx := context.Background()
	index, e := contextindex.BuildRevisionContext(ctx, root, rev)
	if e != nil {
		t.Fatal(e)
	}
	id, e := contextindex.CorpusRepositoryID(ctx, root, rev)
	if e != nil {
		t.Fatal(e)
	}
	a := anchor(doccorpus.Repository{ID: id, Revision: rev}, index.Sources["app/view.ts"], 2)
	auth, e := gitauth.Open(root, gitrun.NewDefaultBudget())
	if e != nil {
		t.Fatal(e)
	}
	release := auth.BeginObjectSession()
	defer release()
	scan := 0
	row, e := relocateAnchor(ctx, auth, index, doccorpus.Repository{ID: id, Revision: rev}, a, &scan)
	if e != nil || row.State != "fresh" || row.After.Start != 2 {
		t.Fatalf("unchanged repeated span: %+v %v", row, e)
	}
	if e = os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("unrelated\n"), 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "unrelated.txt")
	git(t, root, "commit", "-qm", "unrelated change")
	later := git(t, root, "rev-parse", "HEAD")
	index, e = contextindex.BuildRevisionContext(ctx, root, later)
	if e != nil {
		t.Fatal(e)
	}
	row, e = relocateAnchor(ctx, auth, index, doccorpus.Repository{ID: id, Revision: later}, a, &scan)
	if e != nil || row.State != "fresh" || row.After.Start != 2 || scan != 0 {
		t.Fatalf("untouched file in later commit: %+v %v", row, e)
	}
}

func TestFlowDocsSourceInventoryStillRefusesSymlink(t *testing.T) {
	root, _ := fixture(t, map[string]string{"app/view.js": "function View() {}\n"})
	if e := os.Symlink("view.js", filepath.Join(root, "app/link.js")); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "app/link.js")
	git(t, root, "commit", "-qm", "unsupported symlink")
	r, e := Generate(context.Background(), root, Options{Revision: git(t, root, "rev-parse", "HEAD"), Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sourceInventory(context.Background(), root, r.Manifest); e == nil {
		t.Fatal("non-gitlink unsupported entry admitted")
	}
}

// TestFlowDocsImportedSymbolAnchorReresolution checks `docs flows check`
// re-resolves an imported symbol anchor by its normalized region (AFU-V1-090).
func TestFlowDocsImportedSymbolAnchorReresolution(t *testing.T) {
	source := "export function View() {\n  // render\n  return fetch('/api');\n}\n"
	for _, tc := range []struct {
		name, next, state, symbol string
		start                     int
	}{
		{"AFU-V1-090 insertion above is fresh and moved", strings.Repeat("const pad = 1;\n", 20) + source, "fresh", doccorpus.SymbolAnchorMoved, 21},
		{"AFU-V1-090 reformat is fresh", "export function View() {\n    /* draw */\n    return fetch('/api');   // call\n}\n", "fresh", doccorpus.SymbolAnchorLive, 1},
		{"AFU-V1-090 one-token edit is stale and changed", strings.Replace(source, "'/api'", "'/v2'", 1), "stale", doccorpus.SymbolAnchorChanged, 1},
		{"AFU-V1-090 deletion is stale and gone", "export const other = 1;\n", "stale", doccorpus.SymbolAnchorGone, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, rev := fixture(t, map[string]string{"app/view.ts": source})
			ctx := context.Background()
			index, e := contextindex.BuildRevisionContext(ctx, root, rev)
			if e != nil {
				t.Fatal(e)
			}
			id, e := contextindex.CorpusRepositoryID(ctx, root, rev)
			if e != nil {
				t.Fatal(e)
			}
			src := index.Sources["app/view.ts"]
			a := anchor(doccorpus.Repository{ID: id, Revision: rev}, src, 1)
			a.End, a.Symbol, a.SpanSHA256 = 4, "View", doccorpus.Digest(src.Data)
			if e = os.WriteFile(filepath.Join(root, "app/view.ts"), []byte(tc.next), 0600); e != nil {
				t.Fatal(e)
			}
			git(t, root, "commit", "-qam", "edit")
			later := git(t, root, "rev-parse", "HEAD")
			index, e = contextindex.BuildRevisionContext(ctx, root, later)
			if e != nil {
				t.Fatal(e)
			}
			auth, e := gitauth.Open(root, gitrun.NewDefaultBudget())
			if e != nil {
				t.Fatal(e)
			}
			release := auth.BeginObjectSession()
			defer release()
			repository := doccorpus.Repository{ID: id, Revision: later}
			scan := 0
			row, e := revalidateImported(ctx, auth, index, doccorpus.NewSymbolResolver(auth, index, repository, nil), repository, a, &scan)
			if e != nil || row.State != tc.state || row.Symbol == nil || row.Symbol.State != tc.symbol {
				t.Fatalf("row: %+v symbol: %+v %v", row, row.Symbol, e)
			}
			if tc.start == 0 {
				if row.After != nil {
					t.Fatalf("gone must not invent a live location: %+v", row.After)
				}
			} else if row.After == nil || row.After.Start != tc.start || row.After.Symbol != "View" {
				t.Fatalf("after: %+v", row.After)
			}
		})
	}
}

// V1-2003: Finalize admits the corpus it builds like `docs corpus build`, so a
// query index over its bounds is refused with the capacity code (DCP-V1-196).
func TestFlowDocsFinalizeRefusesOverBoundIndex(t *testing.T) {
	ctx := context.Background()
	root, rev := fixture(t, map[string]string{"app/view.ts": "export function View() {\n fetch('/api');\n}\n"})
	r, e := Generate(ctx, root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "generated.json"), r.Files["provider.json"], 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "generated.json")
	git(t, root, "commit", "-qm", "generated provider")
	head := git(t, root, "rev-parse", "HEAD")
	if _, e = Finalize(ctx, root, r, head, "generated.json"); e != nil {
		t.Fatal(e)
	}
	defer doccorpus.SetIndexBoundsForTest(1, doccorpus.MaxIndexKeyBytes)()
	var coded *doccorpus.Error
	if _, e = Finalize(ctx, root, r, head, "generated.json"); !errors.As(e, &coded) || coded.Code != doccorpus.CapacityCode {
		t.Fatalf("over-bound index finalized: %v", e)
	}
}

// TestFlowDocsJavaScriptModuleExtensions (AFU-V1-043, V1-0461): .mjs and .cjs
// sources are JavaScript and produce flows like .js; a module-only scope must
// not silently report zero flows.
func TestFlowDocsJavaScriptModuleExtensions(t *testing.T) {
	root, rev := fixture(t, map[string]string{
		"tool/scan.mjs": "export function scan() {\n fetch('/api');\n}\n",
		"tool/load.cjs": "function load() {\n throw new Error('x');\n}\nmodule.exports = { load };\n",
	})
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "tool"})
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]string{}
	for _, f := range r.Manifest.Flows {
		names[f.Name] = f.Path
	}
	if len(names) != 2 || names["scan"] != "tool/scan.mjs" || names["load"] != "tool/load.cjs" {
		t.Fatalf("module flows=%v", names)
	}
}

// TestFlowDocsCallInventoryDriftIsStale (AFU-V1-045, V1-0461): a body edit
// that adds a call changes the rendered flow-diagram, so check must report
// the flow stale, while a pure line insertion above it stays fresh.
func TestFlowDocsCallInventoryDriftIsStale(t *testing.T) {
	root, rev := fixture(t, map[string]string{"app/flow.js": "function first() {\n if (ready) {\n  fetch('/api');\n }\n}\n"})
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Manifest.Flows) != 1 {
		t.Fatalf("flows=%d", len(r.Manifest.Flows))
	}
	for _, c := range r.Manifest.Flows[0].Calls {
		if c.Name == "if" || c.Name == "function" {
			t.Fatalf("control keyword listed as a call: %+v", r.Manifest.Flows[0].Calls)
		}
	}
	flowID := r.Manifest.Flows[0].ID
	state := func(source string) (bool, string) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(root, "app", "flow.js"), []byte(source), 0600); e != nil {
			t.Fatal(e)
		}
		git(t, root, "commit", "-qam", "edit")
		checked, e := CheckRevision(context.Background(), root, r.Files["generation.json"], git(t, root, "rev-parse", "HEAD"), "")
		if e != nil {
			t.Fatal(e)
		}
		for _, b := range checked.Bindings {
			if b.ID == flowID {
				return checked.Clean, b.State
			}
		}
		t.Fatal("flow binding missing")
		return false, ""
	}
	if clean, s := state("// moved\nfunction first() {\n if (ready) {\n  fetch('/api');\n }\n}\n"); !clean || s != "fresh" {
		t.Fatalf("relocation: clean=%v state=%s", clean, s)
	}
	if clean, s := state("// moved\nfunction first() {\n if (ready) {\n  fetch('/api');\n  render();\n }\n}\n"); clean || s != "stale" {
		t.Fatalf("added call: clean=%v state=%s", clean, s)
	}
}

// TestFlowDocsMemberKeywordCallsAreDrift (AFU-V1-045, V1-0461): the control
// keyword exclusion applies only to bare names. A member call that spells a
// keyword (`promise.catch(h)`, `iterator.return(v)`) is a real call, so
// adding one leaves the flow stale.
func TestFlowDocsMemberKeywordCallsAreDrift(t *testing.T) {
	for _, added := range []string{"  promise.catch(handle);\n", "  iterator.return(value);\n", "  promise\n    .catch(handle);\n"} {
		root, rev := fixture(t, map[string]string{"app/flow.js": "function first() {\n  load();\n}\n"})
		r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
		if e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(root, "app", "flow.js"), []byte("function first() {\n  load();\n"+added+"}\n"), 0600); e != nil {
			t.Fatal(e)
		}
		git(t, root, "commit", "-qam", "edit")
		checked, e := CheckRevision(context.Background(), root, r.Files["generation.json"], git(t, root, "rev-parse", "HEAD"), "")
		if e != nil {
			t.Fatal(e)
		}
		if checked.Clean {
			t.Fatalf("added member call %q left the flow fresh", added)
		}
	}
}

// TestFlowDocsPredecessorGeneratorManifestIsTypedStale (AFU-V1-045, V1-0461):
// a manifest that generator version 1, 2 or 3 wrote is refused with the typed
// flowdocs-generator-stale code at its own unchanged revision, by Open,
// check and generate --previous alike, instead of the generic rederivation
// mismatch; regenerating with the current generator is clean.
func TestFlowDocsPredecessorGeneratorManifestIsTypedStale(t *testing.T) {
	root, rev := fixture(t, map[string]string{"app/flow.js": "function first() {\n  if (ready) {\n    load();\n  }\n}\n"})
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	current := r.Files["generation.json"]
	for _, version := range []string{"1", "2", "3"} {
		predecessor := bytes.Replace(current, []byte(`"version":"`+Version+`"`), []byte(`"version":"`+version+`"`), 1)
		if bytes.Equal(predecessor, current) {
			t.Fatal("fixture did not carry the generator version")
		}
		var coded *doccorpus.Error
		if _, e := Open(context.Background(), root, predecessor); !errors.As(e, &coded) || coded.Code != StaleGeneratorCode {
			t.Fatalf("Open version %s: %v", version, e)
		}
		if _, e := CheckRevision(context.Background(), root, predecessor, rev, ""); !errors.As(e, &coded) || coded.Code != StaleGeneratorCode {
			t.Fatalf("CheckRevision version %s: %v", version, e)
		}
	}
	if checked, e := CheckRevision(context.Background(), root, current, rev, ""); e != nil || !checked.Clean {
		t.Fatalf("regenerated manifest: %v %+v", e, checked)
	}
}

// TestFlowDocsWebMemberCallsAreNotDeclarations (AFU-V1-043, V1-0461): a
// member-call arrow (messages.map(m => ...)) and a non-route HTTP-verb member
// read (headers.get("Content-Length")) are not flows, while direct and
// hook-wrapped arrows and literal route registrations still are.
func TestFlowDocsWebMemberCallsAreNotDeclarations(t *testing.T) {
	text := "const matched = messages.map((message) => message.id);\n" +
		"const size = response.headers.get(\"Content-Length\");\n" +
		"const cached = store.get('session', fallback);\n" +
		"const client = axios.get('/api/users');\n" +
		"export const handler = async (req) => {\n  save(req);\n};\n" +
		"const onClick = useCallback((event) => {\n  save(event);\n}, []);\n" +
		"const single = item => item.id;\n" +
		"app.get('/users', list);\n" +
		"router.post(\"/login\", login);\n" +
		"app.put('/items' /* path */, update);\n" +
		"app.delete('/items'\n  , remove);\n" +
		"const typed = (cb: () => void) => cb();\n" +
		"const defaulted = (x = getDefault()) => x;\n"
	ds, _ := extract(contextindex.Source{Path: "app/web.ts", Data: []byte(text)})
	got := []string{}
	for _, d := range ds {
		got = append(got, d.name)
	}
	want := []string{"binding:delete:/items", "binding:get:/users", "binding:post:/login", "binding:put:/items", "defaulted", "handler", "onClick", "single", "typed"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("declarations=%v want=%v", got, want)
	}
}

func TestAFUV1125FlowLimitIsTyped(t *testing.T) {
	functions := func(prefix string, n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "function %s%d() {\n  load();\n}\n", prefix, i)
		}
		return b.String()
	}
	root, rev := fixture(t, map[string]string{"app/a.js": functions("a", 200), "app/b.js": functions("b", 100), "edge/a.js": functions("e", MaxFlows)})
	_, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app"})
	var coded *doccorpus.Error
	if !errors.As(e, &coded) || coded.Code != FlowLimitCode {
		t.Fatalf("over the limit: %v", e)
	}
	for _, want := range []string{"300 flow candidates", fmt.Sprintf("MaxFlows limit of %d", MaxFlows), `scope "app"`} {
		if !strings.Contains(coded.Message, want) {
			t.Fatalf("message %q lacks %q", coded.Message, want)
		}
	}
	if r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "edge"}); e != nil || len(r.Manifest.Flows) != MaxFlows {
		t.Fatalf("exactly MaxFlows: %v", e)
	}
}

// TestFlowDocsGenericCallMaskedOnlyThroughTypeArguments (AFU-V1-043, V1-2415):
// a generic call's type-argument list (useMemo<T>(, useState<Map<K, V>>() is
// not JSX, so the statement it starts and the declarations after it stay
// code; a JSX element still blanks its text, also right after a keyword such
// as return, which is no generic callee.
func TestFlowDocsGenericCallMaskedOnlyThroughTypeArguments(t *testing.T) {
	text := "const value = useMemo<Item[]>(() => {\n  return persist(items);\n}, [items]);\n" +
		"const [state, setState] = useState<Map<string, Item>>(new Map());\n" +
		"const onSave = useCallback(() => {\n  save(state);\n}, [state]);\n" +
		"export function View() {\n  return <div>save(text) here</div>;\n}\n" +
		"export function Render() {\n  return<div>(required) save(text)</div>;\n}\n"
	ds, _ := extract(contextindex.Source{Path: "app/view.tsx", Data: []byte(text)})
	got := []string{}
	for _, d := range ds {
		calls := []string{}
		for _, o := range d.observations {
			calls = append(calls, o.key)
		}
		got = append(got, d.name+"="+strings.Join(calls, "+"))
	}
	want := []string{"Render=", "View=", "onSave=side_effect:save", "value=side_effect:persist"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("declarations=%v want=%v", got, want)
	}
}
