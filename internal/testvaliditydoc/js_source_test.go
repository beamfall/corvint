//go:build darwin || linux

package testvaliditydoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const jsSpecBody = "describe('add', () => {\n  it('adds', () => {\n    expect(add(1, 2)).toBe(3);\n  });\n  it.skip('later', () => { expect(add(0, 0)).toBe(0); });\n});\n"

// jsAnchoredReceipt writes the JS test file and a receipt binding its digest under each key, with
// one outcome per anchor line (0 means no anchor).
func jsAnchoredReceipt(t *testing.T, root string, keys []string, lines ...int) {
	t.Helper()
	writeAt(t, filepath.Join(root, "src", "add.test.js"), jsSpecBody, time.Unix(1, 0))
	sum := sha256.Sum256([]byte(jsSpecBody))
	digests := map[string]string{}
	for _, key := range keys {
		digests[key] = hex.EncodeToString(sum[:])
	}
	tests := []map[string]any{}
	for i, line := range lines {
		test := map[string]any{"name": "t" + string(rune('a'+i)), "state": "passed"}
		if line != 0 {
			test["anchor"] = map[string]any{"file": "src/add.test.js", "line": line}
		}
		tests = append(tests, test)
	}
	data, err := json.Marshal(map[string]any{"receipt": map[string]any{"kind": "unit", "identity": map[string]any{"testFileDigests": digests}, "tests": tests}})
	if err != nil {
		t.Fatal(err)
	}
	writeAt(t, evidencePath(root, "corvint-js-test-provider-1.json"), string(data), time.Unix(10, 0))
}

// fakeJSSources serves sources for src/add.test.js with reason, and counts its calls.
func fakeJSSources(calls *int, reason string, sources map[int]testvalidity.ExampleSource) ExampleSourceReader {
	return func(name, text string) (map[int]testvalidity.ExampleSource, string) {
		*calls++
		if name != "src/add.test.js" || text != jsSpecBody {
			return map[int]testvalidity.ExampleSource{}, "unexpected-input"
		}
		return sources, reason
	}
}

// TestDiscoverJSSourceRules: discovery binds a JS/TS outcome's anchor to one digest key, reads the
// file once, and places the example at the anchor line or around a failure frame; a placed example
// takes the JS strength and hygiene and a sourceRules member; an unplaced outcome, a reader
// abstention and an edited file abstain; not-a-js-test-file, no anchor, an ambiguous key and no
// reader keep the projection (TSA-V0-082, TSA-V0-083).
func TestDiscoverJSSourceRules(t *testing.T) {
	root := discoveryRoot(t)
	key := filepath.Join(root, "src", "add.test.js")
	jsAnchoredReceipt(t, root, []string{key}, 2, 3, 5, 9, 0)
	plain := discover(t, root)

	calls := 0
	adds := testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: 1, Findings: []testvalidity.SourceFinding{}, ExpectationAnchors: []string{"src/add.test.js:3"}, EndLine: 4}
	skipped := testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: 1, EndLine: 5,
		Findings: []testvalidity.SourceFinding{{Rule: testvalidity.RuleJSSkipped, Severity: testvalidity.SeverityInvalid, Test: "src/add.test.js:5", Location: "src/add.test.js:5", Cause: "src/add.test.js:5"}}}
	sources := map[int]testvalidity.ExampleSource{2: adds, 5: skipped}
	document, err := DiscoverWithSources(root, fakeJSSources(&calls, "", sources))
	if err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	type row struct{ hygiene, reason, strength string }
	got := []row{}
	for _, test := range document.Tests {
		got = append(got, row{test.Projection.Hygiene.State, test.Projection.Hygiene.Reason, test.Projection.Strength.Reason})
	}
	plainStrength := plain.Tests[4].Projection.Strength.Reason
	want := []row{
		{testvalidity.HygieneEligible, testvalidity.HygieneReasonJSExampleRead, testvalidity.StrengthReasonBodyExpectations},
		{testvalidity.HygieneEligible, testvalidity.HygieneReasonJSExampleRead, testvalidity.StrengthReasonBodyExpectations},
		{testvalidity.HygieneIneligible, testvalidity.RuleJSSkipped, testvalidity.StrengthReasonBodyExpectations},
		{testvalidity.HygieneAbstained, testvalidity.HygieneReasonJSExampleUnplaced, plainStrength},
		{plain.Tests[4].Projection.Hygiene.State, plain.Tests[4].Projection.Hygiene.Reason, plainStrength},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projections\n%v\nwant\n%v", got, want)
	}
	if anchors := document.Tests[1].Projection.Hygiene.Anchors; !reflect.DeepEqual(anchors, []string{"src/add.test.js:2"}) {
		t.Fatalf("failure-frame anchors %q", anchors)
	}
	if document.Tests[4].SourceRules != nil || !reflect.DeepEqual(document.Tests[4], plain.Tests[4]) {
		t.Fatalf("unanchored outcome changed: %+v", document.Tests[4])
	}
	data, err := json.Marshal(document.Tests[0])
	if err != nil || !strings.Contains(string(data), `"sourceRules":{"read":"located","expectations":1,"findings":[]}`) {
		t.Fatalf("sourceRules member: %s %v", data, err)
	}
	again, _ := DiscoverWithSources(root, fakeJSSources(&calls, "", sources))
	if !reflect.DeepEqual(again, document) {
		t.Fatalf("a repeated discovery differs")
	}

	if abstained, _ := DiscoverWithSources(root, fakeJSSources(&calls, "source-unparsed", nil)); abstained.Tests[0].Projection.Hygiene.State != testvalidity.HygieneAbstained ||
		abstained.Tests[0].Projection.Hygiene.Reason != "source-unparsed" {
		t.Fatalf("reader abstained=%+v", abstained.Tests[0].Projection.Hygiene)
	}
	for name, read := range map[string]ExampleSourceReader{
		"not a test file": fakeJSSources(&calls, testvalidity.ExampleSourceNotJSTestFile, nil),
		"no reader":       nil,
	} {
		kept, _ := DiscoverWithSources(root, read)
		if !reflect.DeepEqual(kept, plain) {
			t.Fatalf("%s changed the document", name)
		}
	}

	writeAt(t, key, jsSpecBody+"// edited\n", time.Unix(20, 0))
	calls = 0
	edited, _ := DiscoverWithSources(root, fakeJSSources(&calls, "", sources))
	if calls != 0 || edited.Tests[0].Projection.Hygiene.State != testvalidity.HygieneAbstained || edited.Tests[0].Projection.Hygiene.Reason != testvalidity.HygieneReasonJSSourceUnbound {
		t.Fatalf("edited file: calls=%d %+v", calls, edited.Tests[0].Projection.Hygiene)
	}

	ambiguous := discoveryRoot(t)
	jsAnchoredReceipt(t, ambiguous, []string{filepath.Join(ambiguous, "src", "add.test.js"), filepath.Join(ambiguous, "other", "src", "add.test.js")}, 2)
	before := discover(t, ambiguous)
	if after, _ := DiscoverWithSources(ambiguous, fakeJSSources(&calls, "", sources)); calls != 0 || !reflect.DeepEqual(after, before) {
		t.Fatalf("ambiguous key: calls=%d changed=%v", calls, !reflect.DeepEqual(after, before))
	}
}
