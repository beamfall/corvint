// SPDX-License-Identifier: Apache-2.0

package flowreports

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/runevidence"
)

// storeDir is the run evidence store of the fixture repository.
func storeDir(t *testing.T, root string) string {
	t.Helper()
	return runevidence.Locate(git(t, root, "rev-parse", "--absolute-git-dir"))
}

// plant writes one record file into the store by hand, named name (its content ID unless the test
// damages it), and, with tests, one log line naming it and those tests; without tests no log is
// written, so the store lists the record by file and cannot name its tests.
func plant(t *testing.T, root, name string, data []byte, tests []string) {
	t.Helper()
	dir := storeDir(t, root)
	if err := os.MkdirAll(filepath.Join(dir, "records"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "records", name+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if tests == nil {
		return
	}
	line, err := json.Marshal(runevidence.Entry{ID: name, Profile: runevidence.ProfileReceipt, Tree: "t", Commit: "c",
		Instances: []string{}, Tests: tests, Bytes: int64(len(data)), WrittenAt: "2026-11-16T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.OpenFile(filepath.Join(dir, "log.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	if _, err := log.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
}

// AFL-V1-006 over the run evidence store: a stored record that is not a receipt or
// test-run-evidence/0 document this build reads (malformed, or an unknown profile such as
// corvint-test-run/9), whatever its name, or a skipped record whose tests the store cannot name,
// refuses the whole report with exit 2 and nothing on stdout, as a bad --evidence file does. A record
// edited by hand that still parses (its bytes no longer hash to its name) and whose tests the log
// names stays the RVD-V1-010 case: skipped, and the tests it held are marked run-evidence-corrupt.
func TestAFLV1006StoreRecordRefusedWhole_V15623(t *testing.T) {
	checkoutFile := []string{"playwright:e2e/checkout.spec.ts"}
	unknown := []byte(`{"profile":"corvint-test-run/9","source":{"head":"h","tree":"t"}}` + "\n")
	malformed := []byte("{not json\n")
	for name, plantIt := range map[string]func(t *testing.T, fx shop){
		"unknown profile named in the log": func(t *testing.T, fx shop) { plant(t, fx.root, runevidence.ID(unknown), unknown, checkoutFile) },
		"malformed record named in the log": func(t *testing.T, fx shop) {
			plant(t, fx.root, runevidence.ID(malformed), malformed, checkoutFile)
		},
		"unknown profile with no log": func(t *testing.T, fx shop) { plant(t, fx.root, runevidence.ID(unknown), unknown, nil) },
		"damaged record with no log": func(t *testing.T, fx shop) {
			plant(t, fx.root, strings.Repeat("b", 64), receipt(t, fx.head, fx.tree, "PASS", "e2e/checkout.spec.ts"), nil)
		},
		"malformed bytes under another record's name": func(t *testing.T, fx shop) {
			plant(t, fx.root, strings.Repeat("d", 64), malformed, checkoutFile)
		},
		"bad record beside a valid one": func(t *testing.T, fx shop) {
			record(t, fx.root, receipt(t, fx.head, fx.tree, "PASS", "e2e/search.spec.ts"))
			plant(t, fx.root, runevidence.ID(unknown), unknown, checkoutFile)
		},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newShop(t)
			plantIt(t, fx)
			code, out, diagnostic := report(t, fx.root, "status", "--flows", "flows")
			if code != 2 || out != "" || !strings.Contains(diagnostic, `"code":"invalid-arguments"`) || !strings.Contains(diagnostic, runevidence.ReasonCorrupt) {
				t.Fatalf("status over the store: %d %q %s", code, out, diagnostic)
			}
			status(t, fx.root, "--no-store") // the refusal is the store's, not the repository's
		})
	}
	t.Run("damaged record whose tests the log names", func(t *testing.T) {
		fx := newShop(t)
		plant(t, fx.root, strings.Repeat("c", 64), receipt(t, fx.head, fx.tree, "PASS", "e2e/checkout.spec.ts"), checkoutFile)
		r, _ := status(t, fx.root)
		if got := row(t, r, "checkout", "checkout.basic"); got[0].Result != appflows.ResultNotObserved || got[0].Reason != runevidence.ReasonCorrupt {
			t.Fatalf("the test the damaged file-level record held: %+v", got)
		}
		if got := row(t, r, "search", "search.basic"); got[0].Reason != "" {
			t.Fatalf("a test the damaged record never held is marked: %+v", got)
		}
		if r.RunEvidence == nil || !slices.Equal(r.RunEvidence.CorruptRecords, []string{strings.Repeat("c", 64)}) {
			t.Fatalf("run_evidence %+v", r.RunEvidence)
		}
	})
}

// testLink is a declared test link of a variation, reviewed at anchor ("" leaves it unreviewed).
func testLink(from, path, key, anchor string) appflows.FlowLink {
	return appflows.FlowLink{From: from, Basis: "declared", Target: appflows.LinkTarget{Type: "test", Path: path, TestKey: key}, ReviewedAt: anchor}
}

// AFL-V1-001: only accepted, non-stale links attach a test. The checkout test link goes stale when
// its spec file changes after the review, and a link of search.none was never reviewed; flows map
// shows both, and status attaches neither.
func TestAFLV1001OnlyReviewedLinksAttachTests_V15623(t *testing.T) {
	const noneKey = "search.spec.ts > none"
	fx := newShop(t, func(intent *appflows.FlowIntent) {
		if intent.FlowID == "search" {
			intent.Links = append(intent.Links, testLink("search.none", "e2e/search.spec.ts", noneKey, ""))
		}
	})
	write(t, fx.root, map[string]string{"e2e/checkout.spec.ts": "test(\"pays\", () => { expect(1).toBe(1); });\n"})
	head := commit(t, fx.root, "D: change the checkout spec after review")
	set, err := appflows.LoadIntentsAt(context.Background(), fx.root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	links, err := appflows.EvaluateLinks(context.Background(), fx.root, set, head)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, l := range links {
		if l.Target.Type == "test" {
			states[l.From+" "+l.Target.TestKey] = l.ReviewState
		}
	}
	want := map[string]string{"checkout.basic " + checkoutKey: appflows.ReviewStale, "search.basic " + searchKey: appflows.ReviewReviewed, "search.none " + noneKey: appflows.ReviewUnreviewed}
	if fmt.Sprint(states) != fmt.Sprint(want) {
		t.Fatalf("fixture review states %v, want %v", states, want)
	}
	r, _ := status(t, fx.root, "--no-store")
	for _, v := range [][3]string{{"checkout", "checkout.basic", appflows.GapNoTest}, {"search", "search.none", appflows.GapNoTest}} {
		for _, f := range r.Flows {
			for _, variation := range f.Variations {
				if f.FlowID == v[0] && variation.VariationID == v[1] && (len(variation.Tests) != 0 || variation.Code != v[2]) {
					t.Errorf("%s/%s attaches a link that is not reviewed: %+v", v[0], v[1], variation)
				}
			}
		}
	}
	if got := row(t, r, "search", "search.basic"); len(got) != 1 || got[0].TestKey != searchKey {
		t.Fatalf("the reviewed link: %+v", got)
	}
	if r.Totals.Tests != 1 {
		t.Fatalf("totals %+v count links that are not reviewed", r.Totals)
	}
}

// AFL-V1-007 (reach-unknown): a commit that changes only paths whose reach cannot be resolved (no
// owning unit in the impact graph, no link targets them) names every flow and every variation with
// reason reach-unknown, never an empty list; status --base keeps exactly those flows and variations.
func TestAFLV1007UnknownReachNamesEveryFlow_V15623(t *testing.T) {
	fx := newShop(t)
	write(t, fx.root, map[string]string{"src/components/Cart.vue": "<template><div/></template>\n", "db/schema.sql": "create table t (id int);\n"})
	commit(t, fx.root, "D: change paths with no owning unit")
	code, out, diagnostic := report(t, fx.root, "impact", "--flows", "flows", "--base", fx.head)
	if code != 0 {
		t.Fatalf("impact %d %s", code, diagnostic)
	}
	var impact appflows.ImpactReport
	if err := json.Unmarshal([]byte(out), &impact); err != nil {
		t.Fatal(err)
	}
	var impacted []string
	for _, f := range impact.Flows {
		if f.Reason != appflows.ReasonReachUnknown {
			t.Errorf("flow %s has reason %q, want %s", f.FlowID, f.Reason, appflows.ReasonReachUnknown)
		}
		for _, v := range f.Variations {
			impacted = append(impacted, f.FlowID+"/"+v)
		}
	}
	all := []string{"checkout/checkout.basic", "search/search.basic", "search/search.none"}
	if !slices.Equal(impacted, all) {
		t.Fatalf("impact names %v, want every flow and variation %v", impacted, all)
	}
	if !slices.Equal(impact.ReachUnknown, []string{"db/schema.sql", "src/components/Cart.vue"}) || impact.Graph.Scope != "UNKNOWN" {
		t.Fatalf("reach_unknown %v scope %s", impact.ReachUnknown, impact.Graph.Scope)
	}
	r, _ := status(t, fx.root, "--base", fx.head)
	var reported []string
	for _, f := range r.Flows {
		if f.Reason != appflows.ReasonReachUnknown {
			t.Errorf("status flow %s has reason %q", f.FlowID, f.Reason)
		}
		for _, v := range f.Variations {
			reported = append(reported, f.FlowID+"/"+v.VariationID)
		}
	}
	if !slices.Equal(reported, all) || r.Totals.Variations != 3 {
		t.Fatalf("status --base %v totals %+v, want %v", reported, r.Totals, all)
	}
}

// RVD-V1-007 bound: the store's records and the named --evidence files are bounded separately, as
// RunEvidenceFor bounds them, so a full store does not refuse a one-record evidence file; the named
// files together keep their own bound.
func TestAFLV1002NamedEvidenceBoundIsSeparateFromStore_V15623(t *testing.T) {
	fx := newShop(t)
	outcomes := make([]any, 0, 8192)
	for index := range 8192 {
		file := fmt.Sprintf("e2e/f%04d.spec.ts", index)
		outcomes = append(outcomes, map[string]any{"outcome": "PASS", "unit": "typescript:playwright:" + file, "testId": "playwright:" + file})
	}
	raw, err := json.Marshal(map[string]any{"profile": "corvint-test-run/0", "source": map[string]any{"head": fx.head, "tree": fx.tree, "paths": []any{}},
		"invocations": []any{map[string]any{"adapter": "typescript", "version": "1.50.0", "outcomes": outcomes}}})
	if err != nil {
		t.Fatal(err)
	}
	record(t, fx.root, append(raw, '\n'))
	evidence := file(t, "one.jsonl", ingested(t, checkoutKey, fx.head, fx.tree, "passed"))
	r, _ := status(t, fx.root, "--evidence", evidence)
	if got := row(t, r, "checkout", "checkout.basic"); got[0].Result != "passed" || got[0].Granularity != "test" {
		t.Fatalf("the named record beside a full store: %+v", got)
	}
	gaps, _, diagnostic := report(t, fx.root, "gaps", "--flows", "flows", "--evidence", evidence)
	if gaps != 0 {
		t.Fatalf("gaps on the same inputs: %d %s", gaps, diagnostic)
	}
	var half bytes.Buffer
	for range 4097 {
		half.Write(ingested(t, searchKey, fx.head, fx.tree, "passed"))
	}
	first, second := file(t, "a.jsonl", half.Bytes()), file(t, "b.jsonl", half.Bytes())
	code, out, diagnostic := report(t, fx.root, "status", "--flows", "flows", "--no-store", "--evidence", first, "--evidence", second)
	if code != 2 || out != "" || !strings.Contains(diagnostic, appflows.BoundRecords) {
		t.Fatalf("8194 named records: %d %q %s", code, out, diagnostic)
	}
}

// slicedReceipt is receipt with the unit's slice.
func slicedReceipt(t *testing.T, head, tree, outcome, file, slice string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"profile": "corvint-test-run/0",
		"source":  map[string]any{"head": head, "tree": tree, "paths": []any{}},
		"invocations": []any{map[string]any{"adapter": "typescript", "version": "1.50.0", "outcomes": []any{
			map[string]any{"outcome": outcome, "unit": "typescript:playwright:" + file, "testId": "playwright:" + file, "slice": slice},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// AFL-V1-002, invariant 2: a receipt row names no Playwright project, so it cannot say which of a
// variation's several projects ran; each of them stays NOT_OBSERVED with reason
// project-not-recorded instead of showing one PASS for all. A single-project variation keeps the
// file outcome, and a sliced unit's slice is carried onto the row.
func TestAFLV1002ProjectlessReceiptRow_V15623(t *testing.T) {
	fx := newShop(t, func(intent *appflows.FlowIntent) {
		for index := range intent.Variations {
			if intent.Variations[index].VariationID == "search.basic" {
				intent.Variations[index].Projects = []string{"chromium", "firefox"}
			}
		}
	})
	record(t, fx.root, receipt(t, fx.head, fx.tree, "PASS", "e2e/search.spec.ts"))
	record(t, fx.root, slicedReceipt(t, fx.head, fx.tree, "PASS", "e2e/checkout.spec.ts", "1/2"))
	r, _ := status(t, fx.root)
	got := row(t, r, "search", "search.basic")
	if len(got) != 2 {
		t.Fatalf("search.basic rows %+v", got)
	}
	for _, test := range got {
		if test.Result != appflows.ResultNotObserved || test.Reason != appflows.ReasonProjectNotRecorded || test.Granularity != "" {
			t.Errorf("%s @%s from a project-less row: %+v", test.TestKey, test.Project, test)
		}
	}
	if got := row(t, r, "checkout", "checkout.basic"); got[0].Result != "passed" || got[0].Granularity != "file" || got[0].Slice != "1/2" || got[0].Reason != "" {
		t.Fatalf("single-project variation from a sliced receipt row: %+v", got)
	}
}
