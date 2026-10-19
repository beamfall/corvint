//go:build darwin || linux

package testvaliditydoc

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// fakeRubySources returns a reader that serves sources for the declaring lines it holds, with
// reason, and counts its calls.
func fakeRubySources(calls *int, reason string, sources map[int]testvalidity.ExampleSource) ExampleSourceReader {
	return func(name, text string) (map[int]testvalidity.ExampleSource, string) {
		*calls++
		if name != "spec/user_spec.rb" || text != rubySpecBody {
			return map[int]testvalidity.ExampleSource{}, "unexpected-input"
		}
		return sources, reason
	}
}

func discoverRuby(t *testing.T, root string, read ExampleSourceReader) Document {
	t.Helper()
	document, err := DiscoverWithSources(root, read)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

// TestDiscoverRubySourceRules: discovery reads a bound spec file once through the injected reader
// and recomputes each located example's hygiene and strength, with the facts as the sourceRules
// member; an invalid finding is INELIGIBLE; an unlocated example, a reader that abstains and an
// edited spec file are ABSTAINED; without a reader the projection and bytes are unchanged
// (TSA-V0-068, TSA-V0-069, V1-1787).
func TestDiscoverRubySourceRules(t *testing.T) {
	root := discoveryRoot(t)
	writeAt(t, filepath.Join(root, "spec", "user_spec.rb"), rubySpecBody, time.Unix(1, 0))
	writeAt(t, filepath.Join(root, "spec", "spec_helper.rb"), rubyHelperBody, time.Unix(1, 0))
	writeAt(t, evidencePath(root, "corvint-ruby-test-provider-1.json"), string(encodeRuby(t, rubyDocument(t, "passed", "passed"))), time.Unix(10, 0))

	calls := 0
	located := testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: 0, Findings: []testvalidity.SourceFinding{}}
	document := discoverRuby(t, root, fakeRubySources(&calls, "", map[int]testvalidity.ExampleSource{2: located}))
	if calls != 1 {
		t.Fatalf("reader calls %d, want 1", calls)
	}
	first, second := document.Tests[0].Projection, document.Tests[1].Projection
	if first.Hygiene.State != testvalidity.HygieneEligible || first.Strength.State != testvalidity.StrengthNotMeasured || first.Strength.Reason != testvalidity.StrengthReasonNoBodyExpectation {
		t.Fatalf("located=%+v", first)
	}
	if second.Hygiene.State != testvalidity.HygieneAbstained || second.Hygiene.Reason != testvalidity.HygieneReasonRSpecExampleUnplaced {
		t.Fatalf("unlocated=%+v", second.Hygiene)
	}
	data, err := json.Marshal(document.Tests[0])
	if err != nil || !strings.Contains(string(data), `"sourceRules":{"read":"located","expectations":0,"findings":[]}`) {
		t.Fatalf("sourceRules member: %s %v", data, err)
	}

	memoized := located
	memoized.Expectations = 1
	memoized.Findings = []testvalidity.SourceFinding{{Rule: testvalidity.RuleMemoizedBlockExpectation, Severity: testvalidity.SeverityInvalid, Test: "spec/user_spec.rb:2", Location: "spec/user_spec.rb:2", Cause: "spec/user_spec.rb:1"}}
	refused := discoverRuby(t, root, fakeRubySources(&calls, "", map[int]testvalidity.ExampleSource{2: memoized})).Tests[0]
	if refused.Projection.Hygiene.State != testvalidity.HygieneIneligible || refused.Projection.Hygiene.Reason != testvalidity.HygieneReasonMemoizedBlockExpectation {
		t.Fatalf("memoized=%+v", refused.Projection.Hygiene)
	}

	if abstained := discoverRuby(t, root, fakeRubySources(&calls, "rspec-reader-unbalanced", nil)).Tests[0]; abstained.Projection.Hygiene.State != testvalidity.HygieneAbstained ||
		abstained.Projection.Hygiene.Reason != "rspec-reader-unbalanced" || abstained.SourceRules == nil || abstained.SourceRules.Read != "rspec-reader-unbalanced" {
		t.Fatalf("reader abstained=%+v", abstained)
	}

	plain := discover(t, root)
	if plain.Tests[0].SourceRules != nil || plain.Tests[0].Projection.Hygiene.State != testvalidity.HygieneUnsupported {
		t.Fatalf("no reader=%+v", plain.Tests[0])
	}
	if data, _ := json.Marshal(plain); strings.Contains(string(data), "sourceRules") {
		t.Fatalf("no-reader bytes carry sourceRules")
	}

	writeAt(t, filepath.Join(root, "spec", "user_spec.rb"), rubySpecBody+"# edited\n", time.Unix(20, 0))
	calls = 0
	stale := discoverRuby(t, root, fakeRubySources(&calls, "", map[int]testvalidity.ExampleSource{2: located})).Tests[0]
	if calls != 0 || stale.Projection.Hygiene.State != testvalidity.HygieneAbstained || stale.Projection.Hygiene.Reason != testvalidity.HygieneReasonRSpecSourceUnbound {
		t.Fatalf("edited spec: calls=%d %+v", calls, stale.Projection.Hygiene)
	}
	if !reflect.DeepEqual(stale.Projection.Hygiene.Anchors, []string{"spec/user_spec.rb:2"}) {
		t.Fatalf("anchors=%+v", stale.Projection.Hygiene.Anchors)
	}
}
