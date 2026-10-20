package testvaliditydoc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	rubySpecBody   = "RSpec.describe User do\n  it 'saves' do\n  end\nend\n"
	rubyHelperBody = "RSpec.configure { }\n"
)

func rubyDocument(t *testing.T, statuses ...string) rubytestprovider.Document {
	t.Helper()
	repository := &jstestprovider.ApplicationRepositoryIdentity{RootCommit: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40), DirtyState: "clean"}
	helpers := []rubytestprovider.BoundFile{{Path: "spec/spec_helper.rb", Blob: rubytestprovider.BlobID("sha1", []byte(rubyHelperBody))}}
	document := rubytestprovider.Document{
		Profile: rubytestprovider.Profile, Runner: "rspec", ProjectDirectory: ".", ObjectFormat: "sha1",
		RepositoryBefore: repository, RepositoryAfter: repository, HelperBasis: rubytestprovider.HelperBasis,
		Helpers: helpers, HelperSet: rubytestprovider.HelperSetDigest(helpers), Examples: []rubytestprovider.Example{}, JUnit: rubytestprovider.JUnit{State: "NOT_REQUESTED"},
	}
	for index, status := range statuses {
		line := index + 2
		document.Examples = append(document.Examples, rubytestprovider.Example{
			ID: "spec/user_spec.rb:" + string(rune('0'+line)), File: "spec/user_spec.rb", Line: line, Description: "User saves " + status,
			Status: status, SpecBlob: rubytestprovider.BlobID("sha1", []byte(rubySpecBody)), Helpers: document.HelperSet,
		})
	}
	return document
}

func encodeRuby(t *testing.T, document rubytestprovider.Document) []byte {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// LPCV-V0-093: the Ruby document decodes closed and every axis is recomputed
// from its observations; strength is never claimed, and hygiene is UNSUPPORTED
// except that a pending example is INELIGIBLE (TSA-V0-070).
func TestLPCVV0093RubyDocumentDecodesClosedAndProjects(t *testing.T) {
	input, err := Decode(encodeRuby(t, rubyDocument(t, "passed", "failed", "pending")))
	if err != nil {
		t.Fatal(err)
	}
	document := Project(input)
	if document.Source != "corvint-ruby-test-provider" || document.Kind != "rspec" || document.Tier != "preview" || document.Promotable == nil || *document.Promotable || len(document.Tests) != 3 {
		t.Fatalf("document=%+v", document)
	}
	want := []string{testvalidity.ExecutionPassed, testvalidity.ExecutionFailed, testvalidity.ExecutionSkipped}
	for index, test := range document.Tests {
		projection := test.Projection
		if test.ID != "spec/user_spec.rb:"+string(rune('2'+index)) || projection.Association.State != testvalidity.AssociationAssociated || projection.Association.Anchors[0] != test.ID {
			t.Fatalf("association=%+v", test)
		}
		hygiene, reason := testvalidity.HygieneUnsupported, "ruby-hygiene-not-produced"
		if index == 2 {
			hygiene, reason = testvalidity.HygieneIneligible, testvalidity.HygieneReasonRSpecPending
		}
		if projection.Execution.State != want[index] || projection.Hygiene.State != hygiene || projection.Hygiene.Reason != reason || projection.Strength.State != testvalidity.StrengthNotMeasured {
			t.Fatalf("projection=%+v", projection)
		}
	}
	if document.Run.Execution.State != testvalidity.ExecutionFailed {
		t.Fatalf("run=%+v", document.Run)
	}

	infrastructure := rubyDocument(t, "passed")
	infrastructure.Infrastructure = &rubytestprovider.Infrastructure{Reason: "timeout", Detail: "bounded"}
	input, err = Decode(encodeRuby(t, infrastructure))
	if err != nil {
		t.Fatal(err)
	}
	if document := Project(input); document.Tests[0].Projection.Execution.State != testvalidity.ExecutionInfrastructure || document.Run.Execution.State != testvalidity.ExecutionInfrastructure {
		t.Fatalf("infrastructure=%+v", document)
	}

	data := encodeRuby(t, rubyDocument(t, "passed"))
	for name, mutated := range map[string]string{
		"unknown member":      strings.Replace(string(data), `"runner":"rspec"`, `"runner":"rspec","verdict":"valid"`, 1),
		"carried helper set":  strings.Replace(string(data), `"helperSet":"sha256:`, `"helperSet":"sha256:0`, 1),
		"open status":         strings.Replace(string(data), `"status":"passed"`, `"status":"flaky"`, 1),
		"unbound and bound":   strings.Replace(string(data), `"status":"passed"`, `"status":"passed","specBlobUnbound":"spec-file-outside-snapshot"`, 1),
		"git-relative helper": strings.Replace(string(data), `"path":"spec/spec_helper.rb"`, `"path":".git/config"`, 1),
	} {
		if _, err := Decode([]byte(mutated)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// LPCV-V0-092: the provider's per-file snapshot bound equals the bound
// discovery reads a retained spec or helper file under.
func TestLPCVV0092SnapshotBoundMatchesDiscoveryRead(t *testing.T) {
	if rubytestprovider.MaxSpecFileBytes != MaxInputBytes {
		t.Fatalf("provider bound %d, discovery bound %d", rubytestprovider.MaxSpecFileBytes, MaxInputBytes)
	}
}

// LPCV-V0-135: the selection, runtime and shared-example members are
// optional and closed; a document without them decodes and projects exactly
// as before, and carrying them does not move any projected axis.
func TestLPCVV0135RubyOptionalMembersDecodeClosed(t *testing.T) {
	legacy := encodeRuby(t, rubyDocument(t, "passed", "failed"))
	for _, member := range []string{"selection", "selectionWithheld", "runtime", "sharedExample", "sharedExampleAbsent"} {
		if strings.Contains(string(legacy), `"`+member+`"`) {
			t.Fatalf("legacy document carries %s", member)
		}
	}
	input, err := Decode(legacy)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(Project(input))
	if err != nil {
		t.Fatal(err)
	}

	carried := rubyDocument(t, "passed", "failed")
	container := strings.Repeat("ab", 32)
	carried.Selection = []rubytestprovider.SelectorOutcome{
		{Selector: "spec/user_spec.rb:2", Kind: rubytestprovider.SelectorLine, Outcome: rubytestprovider.SelectorMatched, Matched: 1},
		{Selector: "spec/user_spec.rb[1:9]", Kind: rubytestprovider.SelectorID, Outcome: rubytestprovider.SelectorNoExample},
	}
	carried.Runtime = &rubytestprovider.Runtime{Basis: rubytestprovider.RuntimeBasis, Ruby: rubytestprovider.RuntimeFact{Value: "3.3.5"}, Bundler: rubytestprovider.RuntimeFact{Unknown: "not-reported"}, ProbeContainer: rubytestprovider.RuntimeFact{Value: container}, Image: rubytestprovider.RuntimeFact{Unknown: "not-observable-inside-prefix"}}
	carried.Examples[0].SharedExample = &rubytestprovider.SharedExample{Origin: carried.Examples[0].ID, IncludedAt: []string{"spec/user_spec.rb:9"}}
	carried.Examples[1].SharedExampleAbsent = "not-reported"
	data := encodeRuby(t, carried)
	input, err = Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(Project(input))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("optional members moved the projection:\n%s\n%s", before, after)
	}

	for name, mutated := range map[string]string{
		"selection member":     strings.Replace(string(data), `"matched":1`, `"matched":1,"examples":[]`, 1),
		"runtime member":       strings.Replace(string(data), `"basis":"exec-prefix-probe"`, `"basis":"exec-prefix-probe","host":"x"`, 1),
		"runtime fact member":  strings.Replace(string(data), `"value":"3.3.5"`, `"value":"3.3.5","source":"host"`, 1),
		"shared member":        strings.Replace(string(data), `"includedAt":[`, `"group":"x","includedAt":[`, 1),
		"host basis":           strings.Replace(string(data), `"basis":"exec-prefix-probe"`, `"basis":"host"`, 1),
		"open outcome":         strings.Replace(string(data), `"outcome":"NO_EXAMPLE"`, `"outcome":"SKIPPED"`, 1),
		"open shared absence":  strings.Replace(string(data), `"sharedExampleAbsent":"not-reported"`, `"sharedExampleAbsent":"maybe"`, 1),
		"traversal selector":   strings.Replace(string(data), `"selector":"spec/user_spec.rb:2"`, `"selector":"../user_spec.rb:2"`, 1),
		"image without digest": strings.Replace(string(data), `{"unknown":"not-observable-inside-prefix"}`, `{"value":"`+container+`"}`, 1),
	} {
		if mutated == string(data) {
			t.Fatalf("%s: mutation did not apply", name)
		}
		if _, err := Decode([]byte(mutated)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
