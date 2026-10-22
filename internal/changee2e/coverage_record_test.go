package changee2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CCE-V0-053 (V1-2412): the in-process static server records the paths it answers 200 or 304,
// a directory as its index.html, and nothing it answers 404.
func TestCCEV0053RecordingHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"index.html", "app/app.js"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(p)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := newCoverageRecorder()
	srv := httptest.NewServer(recordingHandler(http.FileServer(http.Dir(dir)), r.recordFunc()))
	defer srv.Close()
	for _, p := range []string{"/", "/app/app.js", "/app/./app.js", "/missing.js"} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if got := strings.Join(r.sorted(), ","); got != "app/app.js,index.html" {
		t.Fatalf("recorded %q", got)
	}
	if (*coverageRecorder)(nil).recordFunc() != nil {
		t.Fatal("a nil recorder must leave the pool unrecorded")
	}
}

// CCE-V0-053 (V1-2412): Prove records, for a compiled scenario, the application paths the head
// pool's static server served, attributed to the outcome screen and pinned to head blobs; the
// test tree, node_modules, unknown paths and the base pool are not recorded. CoverageFromProof
// turns the proof into an e2e-runtime-coverage/0 document that obligations consumes: the
// recorded path widens reach that reads UNKNOWN without it, and a contradicted pin refuses.
func TestCCEV0053CoverageRecorderFeedsReach(t *testing.T) {
	root, base := fixtureRepo(t)
	book := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, book)
	if sc.Screen != teesheetsScreen {
		t.Fatalf("scenario screen %q, want %s", sc.Screen, teesheetsScreen)
	}
	head := commitFiles(t, root, map[string]*string{"server/handler.go": text("package server\n")}, "backend")
	served := []string{"index.html", "app/components/teesheet.html", "server/handler.go", sc.Placement, proveConfigName, "node_modules/@playwright/test/index.js", "missing.js"}
	f := &fakeRunner{sides: map[string]fakeSide{
		head: {states: repeat("passed"), served: served},
		base: {states: repeat("failed"), message: assertionMessage(sc, `"Pending"`), line: sc.AssertionLine, served: []string{"app/app.js"}},
	}, controls: "killed"}
	e := f.env(root)
	e.recordCoverage = true
	p := e.proveScenario(bg, base, head, sc, 3)
	c := p.RecordedCoverage
	if c == nil || c.Unrecorded != "" || c.Screen != teesheetsScreen || strings.Join(c.Paths, ",") != "app/components/teesheet.html,index.html,server/handler.go" ||
		len(c.Pins) != 3 || len(c.Receipts) != 1 {
		t.Fatalf("recorded %+v", c)
	}
	for _, pin := range c.Pins {
		if want := gitTest(t, root, "rev-parse", head+":"+pin.Path); pin.Blob != want {
			t.Fatalf("pin %+v, want blob %s", pin, want)
		}
	}

	// Without the option nothing is recorded; a hosted test and an unqualified head pool say why.
	if p := f.env(root).proveScenario(bg, base, head, sc, 3); p.RecordedCoverage != nil {
		t.Fatalf("unrequested coverage %+v", p.RecordedCoverage)
	}
	hosted := sc
	hosted.Screen = ""
	if p := e.proveScenario(bg, base, head, hosted, 3); p.RecordedCoverage == nil || p.RecordedCoverage.Unrecorded != UnrecordedHosted {
		t.Fatalf("hosted %+v", p.RecordedCoverage)
	}
	f.sides[head] = fakeSide{reason: "provider-error", states: repeat("passed"), served: served}
	if p := e.proveScenario(bg, base, head, sc, 3); p.RecordedCoverage == nil || p.RecordedCoverage.Unrecorded != UnrecordedUnqualified || len(p.RecordedCoverage.Pins) != 0 {
		t.Fatalf("unqualified %+v", p.RecordedCoverage)
	}

	proof, err := json.Marshal(proveReport{Schema: ProveSchema, Base: base, Head: head, Digest: "sha256:proof", Scenarios: []scenarioProof{p, {RecordedCoverage: c}}})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := CoverageFromProof(proof)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), `"proof_digest": "sha256:proof"`) || !strings.Contains(string(doc), `"blob": "`) {
		t.Fatalf("coverage document %s", doc)
	}
	plain, err := BuildObligations(bg, diffOpts(root, book, head))
	if err != nil || plain.State != Unknown || len(plain.Obligations) != 0 {
		t.Fatalf("baseline %v: %s", err, plain.Encoded())
	}
	o := buildCoverage(t, root, book, head, string(doc))
	if o.State != Unknown || len(o.Obligations) != 1 || o.Obligations[0].Chain[1] != teesheetsScreen+" (runtime-coverage)" {
		t.Fatalf("recorded coverage did not widen reach: %s", o.Encoded())
	}
	stale := strings.Replace(string(doc), c.Pins[2].Blob, strings.Repeat("0", 40), 1)
	opts := diffOpts(root, book, head)
	opts.Coverage = writeCoverage(t, stale)
	if _, err := BuildObligations(bg, opts); refusalCode(err) != "e2e-coverage-invalid" {
		t.Fatalf("contradicted pin: %v", err)
	}
	if _, err := CoverageFromProof([]byte(`{"schema":"e2e-compile/0"}`)); refusalCode(err) != "e2e-coverage-invalid" {
		t.Fatalf("non-proof: %v", err)
	}
}

// CCE-V0-053 live: a real Playwright run against the in-process server records the head
// screen's served application files, each pinned to its head blob.
func TestCCEV0053LiveRecordsCoverage(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	modules, _ := liveToolchain(t)
	rep, err := Compile(bg, diffOpts(root, base, head))
	if err != nil || len(rep.Compiled()) != 1 {
		t.Fatalf("compile: %v", err)
	}
	p, _ := liveProveWith(t, root, modules, ProveOptions{Root: root, Base: base, Head: head, Scenarios: rep.Compiled(), RecordCoverage: true})
	requireVerdict(t, p, VerdictDiscriminates)
	c := p.RecordedCoverage
	if c == nil || c.Unrecorded != "" || c.Screen != teesheetsScreen || len(c.Paths) == 0 || len(c.Pins) != len(c.Paths) {
		t.Fatalf("recorded coverage = %+v, want pinned served paths on %s", c, teesheetsScreen)
	}
	for _, pin := range c.Pins {
		if want := gitTest(t, root, "rev-parse", head+":"+pin.Path); pin.Blob != want {
			t.Fatalf("pin %s = %s, want head blob %s", pin.Path, pin.Blob, want)
		}
	}
}
