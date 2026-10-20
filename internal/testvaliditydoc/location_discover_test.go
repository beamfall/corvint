//go:build darwin || linux

package testvaliditydoc

import (
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
