//go:build darwin || linux

package testvaliditydoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// LPCV-V0-145, LPCV-V0-147: discovery's bound source reading places each test's declaration: an
// RSpec example and a JS/TS outcome anchored inside its body (a failure frame) both take the
// reader's file, declaring line and literal title path; an unplaced JS outcome names its title
// path source-unplaced.
func TestLPCVV0147DiscoveryPlacesLocation(t *testing.T) {
	root := discoveryRoot(t)
	writeAt(t, filepath.Join(root, "spec", "user_spec.rb"), rubySpecBody, time.Unix(1, 0))
	writeAt(t, filepath.Join(root, "spec", "spec_helper.rb"), rubyHelperBody, time.Unix(1, 0))
	writeAt(t, evidencePath(root, "corvint-ruby-test-provider-1.json"), string(encodeRuby(t, rubyDocument(t, "passed"))), time.Unix(10, 0))
	calls := 0
	saves := testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Findings: []testvalidity.SourceFinding{}, TitlePath: []string{"User", "saves"}}
	ruby := discoverRuby(t, root, fakeRubySources(&calls, "", map[int]testvalidity.ExampleSource{2: saves}))
	if got := ruby.Tests[0].Location; !reflect.DeepEqual(got, &Location{File: "spec/user_spec.rb", Line: 2, TitlePath: []string{"User", "saves"}, Absent: map[string]string{}}) {
		t.Fatalf("ruby location=%+v", got)
	}

	root = discoveryRoot(t)
	jsAnchoredReceipt(t, root, []string{filepath.Join(root, "src", "add.test.js")}, 3, 9)
	adds := testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: 1, Findings: []testvalidity.SourceFinding{}, EndLine: 4, TitlePath: []string{"add", "adds"}}
	js, err := DiscoverWithSources(root, fakeJSSources(&calls, "", map[int]testvalidity.ExampleSource{2: adds}))
	if err != nil {
		t.Fatal(err)
	}
	if got := js.Tests[0].Location; !reflect.DeepEqual(got, &Location{File: "src/add.test.js", Line: 2, TitlePath: []string{"add", "adds"}, Absent: map[string]string{}}) {
		t.Fatalf("placed js location=%+v", got)
	}
	if got := js.Tests[1].Location; got.File != "" || got.Absent["titlePath"] != LocationSourceUnplaced || got.Absent["file"] != LocationNotDeclaration {
		t.Fatalf("unplaced js location=%+v", got)
	}
}

// LPCV-V0-291: a Vitest outcome with no anchor, or with a failure frame outside its test file, is
// placed at its declaration when exactly one bound example's literal title path is its full name;
// two such examples name file and line declaration-ambiguous, and no match keeps the provider's
// reason. The projection is not moved by a title placement.
func TestLPCVV0291VitestDeclarationPlacedByTitle(t *testing.T) {
	root := discoveryRoot(t)
	writeAt(t, filepath.Join(root, "src", "add.test.js"), jsSpecBody, time.Unix(1, 0))
	sum := sha256.Sum256([]byte(jsSpecBody))
	digests := map[string]string{filepath.Join(root, "src", "add.test.js"): hex.EncodeToString(sum[:])}
	tests := []map[string]any{
		{"name": "adds", "fullName": "add adds", "state": "passed"},
		{"name": "adds", "fullName": "add adds", "state": "failed", "failureMessage": "boom", "anchor": map[string]any{"file": "/elsewhere/helper.test.js", "line": 7}},
		{"name": "twice", "fullName": "add twice", "state": "passed"},
		{"name": "gone", "fullName": "add gone", "state": "passed"},
	}
	data, err := json.Marshal(map[string]any{"receipt": map[string]any{"kind": "unit", "identity": map[string]any{"runnerName": "vitest", "testFileDigests": digests}, "tests": tests}})
	if err != nil {
		t.Fatal(err)
	}
	writeAt(t, evidencePath(root, "corvint-js-test-provider-1.json"), string(data), time.Unix(10, 0))
	example := func(line int, path ...string) testvalidity.ExampleSource {
		return testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: 1, Findings: []testvalidity.SourceFinding{}, EndLine: line + 1, TitlePath: path}
	}
	calls := 0
	sources := map[int]testvalidity.ExampleSource{2: example(2, "add", "adds"), 5: example(5, "add", "twice"), 6: example(6, "add", "twice")}
	plain, err := DiscoverWithSources(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	document, err := DiscoverWithSources(root, fakeJSSources(&calls, "", sources))
	if err != nil {
		t.Fatal(err)
	}
	want := &Location{File: "src/add.test.js", Line: 2, TitlePath: []string{"add", "adds"}, Absent: map[string]string{}}
	for index := range 2 {
		if got := document.Tests[index].Location; !reflect.DeepEqual(got, want) {
			t.Fatalf("test %d location=%+v", index, got)
		}
		if !reflect.DeepEqual(document.Tests[index].Projection, plain.Tests[index].Projection) {
			t.Fatalf("test %d projection moved by a title placement", index)
		}
	}
	if got := document.Tests[2].Location; got.File != "" || got.Absent["file"] != LocationDeclarationAmbiguous || got.Absent["line"] != LocationDeclarationAmbiguous {
		t.Fatalf("ambiguous location=%+v", got)
	}
	if got := document.Tests[3].Location; !reflect.DeepEqual(got, plain.Tests[3].Location) || got.Absent["file"] != LocationNotRecorded {
		t.Fatalf("unmatched location=%+v", got)
	}
	if calls != 1 {
		t.Fatalf("source read %d times, want once", calls)
	}
}
