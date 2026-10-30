package appmap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// observedFixture builds the page-object fixture of TestANLV0049 (an unbound page object that a
// spec and a workflow import) and returns it with its map.
func observedFixture(t *testing.T, complete bool) prefixFixture {
	t.Helper()
	root, _ := fixtureRepo(t)
	files := map[string]string{
		"e2e/pages/table.page.ts":  "import { Page } from '@playwright/test';\nexport class TablePage {\n  constructor(private readonly page: Page) {}\n  async sort() {\n    await this.page.getByTestId('sort').click();\n  }\n}\n",
		"e2e/specs/table.spec.ts":  "import { test } from '@playwright/test';\nimport { TablePage } from '../pages/table.page';\ntest('sorts', async ({ page }) => {\n  await new TablePage(page).sort();\n});\n",
		"e2e/pages/orphan.page.ts": "export class Orphan {}\n",
	}
	for p, body := range files {
		if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if complete {
		gitTest(t, root, "rm", "-q", "e2e/specs/unresolved.spec.ts", "e2e/specs/alias.spec.ts")
	}
	rev := commitAll(t, root, "unbound page object")
	f := prefixFixture{m: build(t, root, rev), root: root, rev: rev}
	if po := f.m.file("e2e/pages/table.page.ts"); po == nil || po.Role != rolePageObject || po.Screen != "" || len(po.Methods) == 0 {
		t.Fatalf("fixture page object is bound or has no methods: %+v", po)
	}
	return f
}

// observedRecords records one trace whose sort() action ran on the home screen and one whose action
// ran on a URL the map does not declare, then reads them back through LoadUIRecords.
func observedRecords(t *testing.T, f prefixFixture) *UIRecords {
	t.Helper()
	trace := func(url string) map[string][]string {
		return map[string][]string{"trace.trace": {
			`{"type":"before","callId":"call@1","pageId":"page@1","method":"click","params":{"selector":"#sort"},"stack":[{"file":"/ci/repo/e2e/pages/table.page.ts","line":5,"column":5},{"file":"/ci/repo/e2e/specs/table.spec.ts","line":4,"column":3}]}`,
			`{"type":"frame-snapshot","snapshot":{"callId":"call@1","snapshotName":"before@call@1","frameUrl":"` + url + `","pageId":"page@1","isMainFrame":true,"timestamp":1}}`,
		}}
	}
	raw, err := ObserveUIRecords(f.m, nil, "", []UITraceInput{
		{TestKey: "table.spec.ts:sorts", File: writeUITraceZip(t, trace("http://app.test/#!/home"))},
		{TestKey: "table.spec.ts:elsewhere", File: writeUITraceZip(t, trace("http://app.test/#!/not-declared"))},
	}, f.rev)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "records.json")
	if err = os.WriteFile(name, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	rec, err := LoadUIRecords(name)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func observedImpact(t *testing.T, f prefixFixture, rec *UIRecords, base string) (ImpactReport, []byte) {
	t.Helper()
	raw, err := ImpactWithRecords(context.Background(), f.m, nil, rec, base, Options{Root: f.root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var r ImpactReport
	if err = json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return r, raw
}

// OUE-V0-071, OUE-V0-073, OUE-V0-074 (GH #1829): a page-object edit whose method an observed
// trace bound to a screen selects that screen (basis observed-binding) and the observing tests,
// with the records revision, digest and trace provenance; without --records the same change
// places no screen.
func TestOUEV0071ObservedBindingSelectsScreensAndTests(t *testing.T) {
	f := observedFixture(t, true)
	rec := observedRecords(t, f)
	base := f.rev
	impactCommit(t, f, map[string]string{"e2e/pages/table.page.ts": "export class TablePage { v = 2 }\n"})

	plain, _ := observedImpact(t, f, nil, base)
	if len(plain.Screens) != 0 || plain.Verdict != VerdictNoNavigation || plain.RecordsDigest != "" || plain.Observed != nil {
		t.Fatalf("baseline without records changed: %+v", plain)
	}
	if _, ok := plain.Omitted["observed"]; ok {
		t.Fatal("omitted.observed present without records")
	}

	r, raw := observedImpact(t, f, rec, base)
	if r.Verdict != VerdictTargeted || len(r.Unknown) != 0 {
		t.Fatalf("observed binding did not target: %+v", r)
	}
	if len(r.Screens) != 1 || r.Screens[0].ID != "screen:admin:app.home" || r.Screens[0].Basis != BasisObservedBinding ||
		!reflect.DeepEqual(r.Screens[0].Paths, []string{"e2e/pages/table.page.ts"}) {
		t.Fatalf("observed screen: %+v", r.Screens)
	}
	if !reflect.DeepEqual(r.TestKeys, []string{"table.spec.ts:elsewhere", "table.spec.ts:sorts"}) {
		t.Fatalf("observing tests not selected: %v", r.TestKeys)
	}
	if !slices.Contains(r.SpecFiles, "e2e/specs/table.spec.ts") {
		t.Fatalf("importing spec lost: %v", r.SpecFiles)
	}
	if r.RecordsRevision != rec.Revision || r.RecordsDigest != rec.Digest || len(r.Observed) != 1 {
		t.Fatalf("records provenance: %+v", r)
	}
	ob := r.Observed[0]
	if ob.Path != "e2e/pages/table.page.ts" || !reflect.DeepEqual(ob.Screens, []string{"screen:admin:app.home"}) ||
		!reflect.DeepEqual(ob.UnmappedScreens, []string{"url:/not-declared"}) || len(ob.Traces) != 2 || len(ob.Bindings) == 0 {
		t.Fatalf("observed entry: %+v", ob)
	}
	// The report stays a complete, self-verifying application-map-impact/0 document.
	name := filepath.Join(t.TempDir(), "impact.json")
	if err := os.WriteFile(name, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadImpactReport(name); err != nil {
		t.Fatalf("impact report with observed bindings does not load: %v", err)
	}
	again, _ := observedImpact(t, f, rec, base)
	if again.Digest != r.Digest {
		t.Fatal("impact with records is not deterministic")
	}
}

// OUE-V0-072: an observed binding only adds. Behind an incomplete test import graph the changed
// page object keeps its unknown and full-suite-required; an unbound page object keeps
// test-support-unreached.
func TestOUEV0072ObservedBindingNeverRemovesUnknown(t *testing.T) {
	f := observedFixture(t, false)
	rec := observedRecords(t, f)
	base := f.rev
	impactCommit(t, f, map[string]string{"e2e/pages/table.page.ts": "export class TablePage { v = 2 }\n"})
	r, _ := observedImpact(t, f, rec, base)
	if r.Verdict != VerdictFullSuite || impactReasons(r)["e2e/pages/table.page.ts"] != "test-import-graph-incomplete" {
		t.Fatalf("observed binding narrowed an unknown: %+v", r)
	}
	if len(r.Screens) != 1 || r.Screens[0].Basis != BasisObservedBinding {
		t.Fatalf("observed screen not added beside the unknown: %+v", r.Screens)
	}

	f = observedFixture(t, true)
	rec = observedRecords(t, f)
	base = f.rev
	impactCommit(t, f, map[string]string{"e2e/pages/orphan.page.ts": "export class Orphan { v = 2 }\n"})
	r, _ = observedImpact(t, f, rec, base)
	if r.Verdict != VerdictFullSuite || impactReasons(r)["e2e/pages/orphan.page.ts"] != "test-support-unreached" || len(r.Observed) != 0 {
		t.Fatalf("unbound page object narrowed with records: %+v", r)
	}
}

// OUE-V0-070: records built without a map, or for another application, refuse.
func TestOUEV0070RecordsBindingRefusals(t *testing.T) {
	f := observedFixture(t, true)
	rec := observedRecords(t, f)
	for name, mutate := range map[string]func(*UIRecords){
		"no-map":    func(r *UIRecords) { r.MapDigest = "" },
		"no-app":    func(r *UIRecords) { r.App = "" },
		"other-app": func(r *UIRecords) { r.App = "other" },
	} {
		c := *rec
		mutate(&c)
		_, err := ImpactWithRecords(context.Background(), f.m, nil, &c, f.rev, Options{Root: f.root})
		if codeOf(err) != "appmap-invalid-query" {
			t.Fatalf("%s: want appmap-invalid-query, got %v", name, err)
		}
	}
}
