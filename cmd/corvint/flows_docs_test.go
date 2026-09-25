package main

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

var docsArgs = []string{"docs", "--flows", "flows", "--page", "docs/flows.md", "--claims", "docs/flows.claims.json", "--docs-root", "docs"}

const docsGuide = "# Guide\n\nPaying completes the order.\n<!-- corvint-claim flow=checkout variation=checkout.happy outcome=paid -->\n"

// docsEvidence writes run evidence at HEAD: checkout with the given outcome, search flaky and
// profile.edit from an older commit.
func docsEvidence(t *testing.T, fx shopFixture, checkout string) string {
	t.Helper()
	at := appflows.RunSource{Commit: shopGit(t, fx.root, "rev-parse", "HEAD"), Tree: shopGit(t, fx.root, "rev-parse", "HEAD^{tree}"), Clean: true}
	atB := appflows.RunSource{Commit: fx.b, Tree: shopGit(t, fx.root, "rev-parse", fx.b+"^{tree}"), Clean: true}
	control := func(key string) []appflows.NegativeControl {
		return []appflows.NegativeControl{{TestKey: key, Expected: "failed", Observed: "failed"}}
	}
	records := []appflows.TestRunEvidence{
		shopRecord("checkout.spec.ts > pays", "chromium", at, "done", []string{checkout}, control("checkout.spec.ts > declines")),
		shopRecord("search.spec.ts > finds", "chromium", at, "done", []string{"failed", "passed"}, control("search.spec.ts > empty")),
		shopRecord("profile.spec.ts > edits", "", atB, "done", []string{"passed"}, []appflows.NegativeControl{}),
	}
	var lines bytes.Buffer
	for _, r := range records {
		line, err := appflows.EncodeRunEvidence(r)
		if err != nil {
			t.Fatal(err)
		}
		lines.Write(line)
	}
	name := filepath.Join(t.TempDir(), "runs.jsonl")
	if err := os.WriteFile(name, lines.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

// newDocsFixture is the shop fixture plus an anchored guide and an unanchored FAQ, with the page and
// sidecar rendered from evidence at that commit and then committed.
func newDocsFixture(t *testing.T) shopFixture {
	t.Helper()
	fx := newShopFixture(t)
	shopWrite(t, fx.root, map[string]string{"docs/guide.md": docsGuide, "docs/faq.md": "# FAQ\n\nNo anchor here.\n"})
	shopCommit(t, fx.root, "D: hand-written docs")
	code, out, diagnostic := runFlowsCLI(fx.root, append(docsArgs, "--evidence", docsEvidence(t, fx, "passed"))...)
	if code != 0 || out != "docs/flows.md\ndocs/flows.claims.json\n" {
		t.Fatalf("render %d %q %s", code, out, diagnostic)
	}
	fx.head = shopCommit(t, fx.root, "E: rendered docs")
	return fx
}

func docsCheck(t *testing.T, fx shopFixture, checkout string, extra ...string) (int, appflows.DocCheck, string) {
	t.Helper()
	args := append(append(slices.Clone(docsArgs), "--check", "--evidence", docsEvidence(t, fx, checkout)), extra...)
	code, out, diagnostic := runFlowsCLI(fx.root, args...)
	var report appflows.DocCheck
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("check report %d %q %s: %v", code, out, diagnostic, err)
	}
	return code, report, diagnostic
}

func failureCodes(report appflows.DocCheck) []string {
	codes := []string{}
	for _, f := range report.Failures {
		codes = append(codes, f.Code+" "+f.Claim+f.Path)
	}
	return codes
}

// AFU-V1-030 AFU-V1-031: the fixed template and sidecar match the goldens, every claim that is not
// PROVEN is marked, and regeneration at a later commit is byte-identical.
func TestAFUV1030DocsRenderGoldenAndByteStable(t *testing.T) {
	fx := newDocsFixture(t)
	for name, golden := range map[string]string{"docs/flows.md": "docs.golden.md", "docs/flows.claims.json": "docs.claims.golden.json"} {
		got, err := os.ReadFile(filepath.Join(fx.root, name))
		want, wantErr := os.ReadFile(filepath.Join("testdata", "flows", golden))
		if err != nil || wantErr != nil || string(got) != string(want) {
			t.Errorf("%s golden mismatch (%v %v); got:\n%s", name, err, wantErr, got)
		}
	}
	page, _ := os.ReadFile(filepath.Join(fx.root, "docs", "flows.md"))
	for _, line := range []string{"  - `checkout/checkout.happy/paid`: paid visible\n", "  - **CONTRADICTED:** `search/search.basic/listed`",
		"  - **STALE:** `profile/profile.edit/saved`", "  - **UNPROVEN:** `profile/profile.view/shown`", "  - **UNPROVEN:** `returns/returns.basic/refunded`"} {
		if !strings.Contains(string(page), line) {
			t.Errorf("page lacks %q", line)
		}
	}
	code, out, diagnostic := runFlowsCLI(fx.root, append(docsArgs, "--evidence", docsEvidence(t, fx, "passed"))...)
	if code != 0 || shopGit(t, fx.root, "status", "--porcelain") != "" {
		t.Fatalf("regeneration at a later commit changed bytes: %d %s %s %s", code, out, diagnostic, shopGit(t, fx.root, "status", "--porcelain"))
	}
	for _, bad := range [][]string{
		{"docs", "--flows", "flows", "--page", "docs/a.md", "--claims", "docs/a.md"},
		{"docs", "--flows", "flows", "--page", "/tmp/a.md", "--claims", "docs/a.json"},
		{"docs", "--flows", "flows", "--page", "docs/a.md", "--claims", "docs/a.json", "--waivers", "w.json"},
		{"docs", "--flows", "flows", "--page", "docs/a.md", "--claims", "docs/a.json", "--docs-root", "absent"},
	} {
		if code, _, _ = runFlowsCLI(fx.root, bad...); code != 2 {
			t.Errorf("%v exited %d", bad, code)
		}
	}
}

// AFU-V1-018 AFU-V1-032 AFU-V1-033: a passing check is read-only and reports the unanchored
// documents in one coverage row; losing PROVEN or editing the page fails it.
func TestAFUV1032DocsCheckDrift(t *testing.T) {
	fx := newDocsFixture(t)
	before := flowSnapshot(t, fx.root)
	code, report, diagnostic := docsCheck(t, fx, "passed")
	if code != 0 || report.Status != "pass" || len(report.Failures) != 0 || report.Revision != fx.head {
		t.Fatalf("clean check %d %+v %s", code, report, diagnostic)
	}
	c := report.Coverage
	if c == nil || c.Row != "unanchored-documents" || c.Value != 1 || c.Denominator != 2 || c.Revision != fx.head || !slices.Equal(c.Paths, []string{"docs/faq.md"}) ||
		!strings.Contains(c.Limitation, "not the surrounding prose") {
		t.Fatalf("coverage row %+v", c)
	}
	code, report, diagnostic = docsCheck(t, fx, "failed")
	want := []string{"claim-lost-proven checkout/checkout.happy/paiddocs/flows.md", "claim-lost-proven docs/guide.md:checkout/checkout.happy/paiddocs/guide.md",
		"rendered-bytes-differ docs/flows.md", "rendered-bytes-differ docs/flows.claims.json"}
	if code != 2 || report.Status != "fail" || !slices.Equal(failureCodes(report), want) || !strings.Contains(diagnostic, "flow-docs-check-failed") {
		t.Fatalf("lost PROVEN %d %v %s", code, failureCodes(report), diagnostic)
	}
	if after := flowSnapshot(t, fx.root); !maps.Equal(before, after) {
		t.Fatal("flows docs --check changed the repository")
	}
	shopWrite(t, fx.root, map[string]string{"docs/flows.md": "# Hand edited\n"})
	shopCommit(t, fx.root, "F: hand edit the generated page")
	code, report, _ = docsCheck(t, fx, "passed")
	if code != 2 || !slices.Equal(failureCodes(report), []string{"rendered-bytes-differ docs/flows.md"}) {
		t.Fatalf("hand-edited page %d %v", code, failureCodes(report))
	}
}

// AFU-V1-032: an unexpired committed waiver naming the claim excuses the lost PROVEN; an expired one
// does not.
func TestAFUV1032WaiverExpiry(t *testing.T) {
	fx := newDocsFixture(t)
	waivers := func(expires string) string {
		return `{"schema":"flow-doc-waivers/0","waivers":[` +
			`{"claim":"checkout/checkout.happy/paid","reason":"payment sandbox outage","reviewer":"docs owner","expires":"` + expires + `"},` +
			`{"claim":"docs/guide.md:checkout/checkout.happy/paid","reason":"payment sandbox outage","reviewer":"docs owner","expires":"` + expires + `"}]}` + "\n"
	}
	shopWrite(t, fx.root, map[string]string{"flow-doc-waivers.json": waivers("2099-12-31")})
	shopCommit(t, fx.root, "F: waive checkout")
	code, report, diagnostic := docsCheck(t, fx, "failed", "--waivers", "flow-doc-waivers.json")
	if code != 0 || report.Status != "pass" || len(report.Waived) != 2 {
		t.Fatalf("unexpired waiver %d %v %v %s", code, failureCodes(report), report.Waived, diagnostic)
	}
	shopWrite(t, fx.root, map[string]string{"flow-doc-waivers.json": waivers("2020-01-01")})
	shopCommit(t, fx.root, "G: waiver expired")
	code, report, _ = docsCheck(t, fx, "failed", "--waivers", "flow-doc-waivers.json")
	if code != 2 || len(report.Waived) != 0 || len(report.Failures) != 4 || !strings.Contains(report.Failures[0].Detail, "expired on 2020-01-01") {
		t.Fatalf("expired waiver %d %+v", code, report)
	}
	shopWrite(t, fx.root, map[string]string{"flow-doc-waivers.json": `{"schema":"flow-doc-waivers/0","waivers":[{"claim":"c","reason":"r","reviewer":"x","expires":"tomorrow"}]}`})
	shopCommit(t, fx.root, "H: invalid waiver")
	if code, _, _ := runFlowsCLI(fx.root, append(slices.Clone(docsArgs), "--check", "--waivers", "flow-doc-waivers.json")...); code != 2 {
		t.Fatalf("invalid waiver accepted: %d", code)
	}
}

// AFU-V1-033: an unanchored document never fails, even when it changes; an unknown anchor fails the
// check and refuses the render.
func TestAFUV1033AnchoredMarkdown(t *testing.T) {
	fx := newDocsFixture(t)
	shopWrite(t, fx.root, map[string]string{"docs/faq.md": "# FAQ\n\nChanged prose that claims checkout never fails.\n"})
	shopCommit(t, fx.root, "F: edit unanchored prose")
	if code, report, diagnostic := docsCheck(t, fx, "passed"); code != 0 || report.Status != "pass" {
		t.Fatalf("an unanchored document failed the check: %d %v %s", code, failureCodes(report), diagnostic)
	}
	shopWrite(t, fx.root, map[string]string{"docs/bad.md": "# Bad\n<!-- corvint-claim flow=checkout variation=checkout.happy outcome=refunded -->\n"})
	shopCommit(t, fx.root, "G: unknown anchor")
	code, report, _ := docsCheck(t, fx, "passed")
	if code != 2 || !slices.Equal(failureCodes(report), []string{"unknown-anchor checkout/checkout.happy/refundeddocs/bad.md"}) || report.Failures[0].Line != 2 {
		t.Fatalf("unknown anchor %d %+v", code, report.Failures)
	}
	if report.Coverage.Denominator != 3 || !slices.Equal(report.Coverage.Paths, []string{"docs/bad.md", "docs/faq.md"}) {
		t.Fatalf("coverage %+v", report.Coverage)
	}
	before := flowSnapshot(t, fx.root)
	code, _, diagnostic := runFlowsCLI(fx.root, append(docsArgs, "--evidence", docsEvidence(t, fx, "passed"))...)
	if code != 2 || !strings.Contains(diagnostic, "unknown-anchor") || !maps.Equal(before, flowSnapshot(t, fx.root)) {
		t.Fatalf("render with an unknown anchor %d %s", code, diagnostic)
	}
}

// AFU-V1-032: an unexpired waiver for an anchored claim that is no longer rendered keeps its
// committed form for the byte comparison, so the sidecar still matches.
func TestAFUV1032WaiverKeepsUnrenderedClaim(t *testing.T) {
	fx := newDocsFixture(t)
	shopWrite(t, fx.root, map[string]string{"docs/guide.md": "# Guide\n\nPaying completes the order.\n",
		"flow-doc-waivers.json": `{"schema":"flow-doc-waivers/0","waivers":[{"claim":"docs/guide.md:checkout/checkout.happy/paid","reason":"guide rewrite","reviewer":"docs owner","expires":"2099-12-31"}]}` + "\n"})
	shopCommit(t, fx.root, "F: drop the guide anchor under a waiver")
	code, report, diagnostic := docsCheck(t, fx, "passed", "--waivers", "flow-doc-waivers.json")
	if code != 0 || report.Status != "pass" || !slices.Equal(report.Waived, []string{"docs/guide.md:checkout/checkout.happy/paid"}) {
		t.Fatalf("waived unrendered claim %d %v %v %s", code, failureCodes(report), report.Waived, diagnostic)
	}
}

// AFU-V1-033: a symlinked Markdown document under the docs root is not read; it counts as unanchored
// and neither the check nor the render fails.
func TestAFUV1033NonRegularMarkdownSkipped(t *testing.T) {
	fx := newDocsFixture(t)
	if err := os.Symlink("guide.md", filepath.Join(fx.root, "docs", "link.md")); err != nil {
		t.Fatal(err)
	}
	shopCommit(t, fx.root, "F: symlinked Markdown")
	code, report, diagnostic := docsCheck(t, fx, "passed")
	if code != 0 || report.Status != "pass" || report.Coverage.Denominator != 3 || !slices.Equal(report.Coverage.Paths, []string{"docs/faq.md", "docs/link.md"}) {
		t.Fatalf("symlinked Markdown %d %v %+v %s", code, failureCodes(report), report.Coverage, diagnostic)
	}
	if code, out, diagnostic := runFlowsCLI(fx.root, append(docsArgs, "--evidence", docsEvidence(t, fx, "passed"))...); code != 0 {
		t.Fatalf("render with symlinked Markdown %d %s %s", code, out, diagnostic)
	}
}
