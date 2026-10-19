//go:build darwin || linux

package testvaliditydoc

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/rubytestprovider"
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

const (
	inclusionSpecBody   = "RSpec.describe Widget do\n  it 'works' do\n  end\n  it_behaves_like 'a widget'\n  it_behaves_like 'a widget'\nend\n"
	inclusionSharedBody = "RSpec.shared_examples 'a widget' do\n  it 'reads' do\n  end\nend\n"
	inclusionShared     = "spec/support/widget_examples.rb"
)

// inclusionDocument is a receipt with one example of spec/widget_spec.rb, which binds that spec,
// and one shared example run through each of its two inclusions.
func inclusionDocument(t *testing.T, bindSpec bool) rubytestprovider.Document {
	t.Helper()
	document := rubyDocument(t)
	document.Helpers = append(document.Helpers, rubytestprovider.BoundFile{Path: inclusionShared, Blob: rubytestprovider.BlobID("sha1", []byte(inclusionSharedBody))})
	document.HelperSet = rubytestprovider.HelperSetDigest(document.Helpers)
	shared := rubytestprovider.BlobID("sha1", []byte(inclusionSharedBody))
	if bindSpec {
		document.Examples = append(document.Examples, rubytestprovider.Example{ID: "spec/widget_spec.rb:2", RSpecID: "./spec/widget_spec.rb[1:1]", File: "spec/widget_spec.rb", Line: 2, Status: "passed",
			SpecBlob: rubytestprovider.BlobID("sha1", []byte(inclusionSpecBody)), Helpers: document.HelperSet})
	}
	for _, id := range []string{"./spec/widget_spec.rb[1:2:1]", "./spec/widget_spec.rb[1:3:1]"} {
		document.Examples = append(document.Examples, rubytestprovider.Example{ID: inclusionShared + ":2", RSpecID: id, File: inclusionShared, Line: 2, Status: "passed",
			Description: "Widget behaves like a widget reads", SpecBlob: shared, Helpers: document.HelperSet})
	}
	return document
}

// TestDiscoverRubyInclusionJoin: an example whose receipt id names another spec than its file is
// read through that inclusion with the bound spec, shared file, description and helper set, each
// inclusion separately and anchored at its qualified reference; an including spec no example
// binds abstains rspec-inclusion-source-unbound (TSA-V0-088, TSA-V0-092).
func TestDiscoverRubyInclusionJoin(t *testing.T) {
	root := discoveryRoot(t)
	writeAt(t, filepath.Join(root, "spec", "widget_spec.rb"), inclusionSpecBody, time.Unix(1, 0))
	writeAt(t, filepath.Join(root, "spec", "spec_helper.rb"), rubyHelperBody, time.Unix(1, 0))
	writeAt(t, filepath.Join(root, "spec", "support", "widget_examples.rb"), inclusionSharedBody, time.Unix(1, 0))
	writeAt(t, evidencePath(root, "corvint-ruby-test-provider-1.json"), string(encodeRuby(t, inclusionDocument(t, true))), time.Unix(10, 0))

	located := testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: 1, Findings: []testvalidity.SourceFinding{}}
	var supports []testvalidity.RubySupport
	var asked []testvalidity.RSpecInclusion
	readers := RubyReaders{
		Examples: func(name, text string, support testvalidity.RubySupport) (map[int]testvalidity.ExampleSource, string) {
			supports = append(supports, support)
			return map[int]testvalidity.ExampleSource{2: located}, ""
		},
		Inclusion: func(q testvalidity.RSpecInclusion) (testvalidity.ExampleSource, string) {
			asked = append(asked, q)
			ref := q.Ref(q.ScopedID[1] + 2)
			if q.ScopedID[1] == 2 {
				refused := located
				refused.Findings = []testvalidity.SourceFinding{{Rule: testvalidity.RuleRBBlockNeverYielded, Severity: testvalidity.SeverityInvalid, Test: ref, Location: inclusionShared + ":3", Cause: "spec/widget_spec.rb:4"}}
				return refused, ref
			}
			return located, ref
		},
	}
	document, err := DiscoverWithReaders(root, func(string, string) (map[int]testvalidity.ExampleSource, string) {
		t.Fatal("the plain reader was called with Ruby readers bound")
		return nil, ""
	}, readers)
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 2 || len(supports) != 1 {
		t.Fatalf("inclusion reads %d, example reads %d, want 2 and 1", len(asked), len(supports))
	}
	wantFiles := map[string]string{"spec/spec_helper.rb": rubyHelperBody, inclusionShared: inclusionSharedBody}
	if !reflect.DeepEqual(supports[0].Files, wantFiles) || supports[0].Unbound != "" || !reflect.DeepEqual(asked[0].Support, supports[0]) {
		t.Fatalf("support = %+v / %+v", supports[0], asked[0].Support)
	}
	q := asked[0]
	if q.SpecPath != "spec/widget_spec.rb" || q.SpecText != inclusionSpecBody || q.SharedPath != inclusionShared || q.SharedText != inclusionSharedBody ||
		q.ExampleLine != 2 || q.Description != "Widget behaves like a widget reads" || !reflect.DeepEqual(q.ScopedID, []int{1, 2, 1}) || !reflect.DeepEqual(asked[1].ScopedID, []int{1, 3, 1}) {
		t.Fatalf("inclusion = %+v", q)
	}
	first, second := document.Tests[1].Projection.Hygiene, document.Tests[2].Projection.Hygiene
	if first.State != testvalidity.HygieneIneligible || !reflect.DeepEqual(first.Anchors, []string{"spec/widget_spec.rb:4>" + inclusionShared + ":2", inclusionShared + ":3", "spec/widget_spec.rb:4"}) {
		t.Fatalf("first inclusion = %+v", first)
	}
	if second.State != testvalidity.HygieneEligible || !reflect.DeepEqual(second.Anchors, []string{"spec/widget_spec.rb:5>" + inclusionShared + ":2"}) {
		t.Fatalf("second inclusion = %+v", second)
	}

	writeAt(t, evidencePath(root, "corvint-ruby-test-provider-1.json"), string(encodeRuby(t, inclusionDocument(t, false))), time.Unix(11, 0))
	asked = nil
	unbound, err := DiscoverWithReaders(root, func(string, string) (map[int]testvalidity.ExampleSource, string) { return nil, "" }, readers)
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 0 || unbound.Tests[0].Projection.Hygiene.State != testvalidity.HygieneAbstained || unbound.Tests[0].Projection.Hygiene.Reason != testvalidity.InclusionSourceUnbound {
		t.Fatalf("unbound spec: asked %d, %+v", len(asked), unbound.Tests[0].Projection.Hygiene)
	}
}
