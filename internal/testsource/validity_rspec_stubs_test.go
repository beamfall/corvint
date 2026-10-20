package testsource

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

var stubRules = []string{testvalidity.RuleRBStubDecidesAssertion, testvalidity.RuleRBMessageChainPrefixOnly,
	testvalidity.RuleRBFlagConditional, testvalidity.RuleRBFlagForcedNonDefault, testvalidity.RuleStubOnPath}

// stubBrief reads a spec with support and keeps, per example, the TSA-V0-431..438 rows (and
// STUB-ON-PATH, to pin the flag exclusion) and one COND row per flag condition.
func stubBrief(t *testing.T, name, text string, flags *testvalidity.FlagLookups) map[int][]string {
	t.Helper()
	examples, reason := RSpecExampleSourcesWithSupport(name, text, testvalidity.RubySupport{Unbound: "rspec-support-not-given", Flags: flags})
	if reason != "" {
		t.Fatalf("%s: uncertain %q", name, reason)
	}
	out := map[int][]string{}
	for line, e := range examples {
		rows := []string{}
		for _, f := range e.Findings {
			if slices.Contains(stubRules, f.Rule) {
				rows = append(rows, fmt.Sprintf("%s %s %s %s", f.Rule, f.Severity, f.Location, f.Cause))
			}
		}
		for _, c := range e.Conditions {
			rows = append(rows, fmt.Sprintf("COND %s %s %s default=%s", c.Location, c.Lookup, c.Qualifier, c.Default))
		}
		if len(rows) > 0 {
			out[line] = rows
		}
	}
	return out
}

const stubDecidesSpec = `RSpec.describe CancelOrder do
  let(:record) { create(:order) }
  subject(:run) { described_class.new(record).call }
  context 'when cancel fails' do
    before { allow(record).to receive(:cancel).and_return(false) }
    it 'does not cancel the order' do
      expect { run }.not_to change { record.reload.cancelled? }
    end
  end
  it 'reads a stubbed predicate' do
    allow(record).to receive(:cancelled?).and_return(false)
    expect { run }.not_to change { record.cancelled? }
  end
  it 'raises from save!' do
    allow(record).to receive(:save!).and_raise(ActiveRecord::RecordInvalid)
    expect { run }.not_to change(record, :status)
  end
  it 'calls the original' do
    allow(record).to receive(:cancel).and_call_original
    expect { run }.not_to change { record.reload.cancelled? }
  end
  it 'stubs a different receiver' do
    allow(gateway).to receive(:refund).and_return(false)
    expect { run }.not_to change { record.reload.cancelled? }
  end
  it 'asserts a positive change' do
    allow(record).to receive(:cancel).and_return(false)
    expect { run }.to change { record.reload.cancelled? }
  end
  it 'stubs after the assertion' do
    expect { run }.not_to change { record.reload.cancelled? }
    allow(record).to receive(:cancel).and_return(false)
  end
  it 'stubs an operation that is not a known write' do
    allow(record).to receive(:notify_owner).and_return(nil)
    expect { run }.not_to change { record.reload.cancelled? }
  end
  it 'stubs a predicate that is not observed' do
    allow(record).to receive(:cancellable?).and_return(false)
    expect { run }.not_to change { record.reload.cancelled? }
  end
  it 'continues the stub on the next line' do
    allow(record).to receive(:cancel)
      .and_wrap_original { |m| m.call }
    expect { run }.not_to change { record.reload.cancelled? }
  end
  it 'stubs with a block' do
    allow(record).to receive(:cancel) { false }
    expect { run }.not_to change { record.reload.cancelled? }
  end
  it 'observes a computed value' do
    allow(record).to receive(:cancel).and_return(false)
    expect { run }.not_to change { Order.where(id: record.id).first&.state }
  end
  it 'stubs a flag lookup' do
    allow(Feature).to receive(:enabled?).and_return(false)
    expect { run }.not_to change { Feature.enabled? }
  end
end
`

// TestStubDecidesAssertion traces TSA-V0-431 and TSA-V0-432: a stub replacing the observed write
// operation (in a hook or before the assertion) or fixing the observed value abstains with the
// stub as cause; a call-through, another receiver, a positive change, a later stub, an operation
// that is not a known write, an unobserved predicate, an unread continuation or block, a
// computed observation and a flag stub give no finding.
func TestStubDecidesAssertion(t *testing.T) {
	got := stubBrief(t, "spec/order_spec.rb", stubDecidesSpec, nil)
	wantExamples(t, got, map[int][]string{
		6:  {"RB-STUB-DECIDES-ASSERTION abstained spec/order_spec.rb:7 spec/order_spec.rb:5"},
		10: {"RB-STUB-DECIDES-ASSERTION abstained spec/order_spec.rb:12 spec/order_spec.rb:11"},
		14: {"RB-STUB-DECIDES-ASSERTION abstained spec/order_spec.rb:16 spec/order_spec.rb:15"},
		// A flag stub fixing the observed lookup is a condition, not a deciding stub (TSA-V0-435).
		55: {"RB-FLAG-CONDITIONAL advisory spec/order_spec.rb:56 spec/order_spec.rb:56",
			"COND spec/order_spec.rb:56 Feature.enabled? conditional on flag Feature.enabled?(*)=false default="},
	})
	examples, _ := RSpecExampleSources("spec/order_spec.rb", stubDecidesSpec)
	var finding testvalidity.SourceFinding
	for _, f := range examples[6].Findings {
		if f.Rule == testvalidity.RuleRBStubDecidesAssertion {
			finding = f
		}
	}
	for _, want := range []string{"spec/order_spec.rb:7", "spec/order_spec.rb:5", "allow(record).to receive(:cancel).and_return(false)",
		"record.reload.cancelled?", "replaces the operation record.cancel", "inject the failure below the operation", "abstains"} {
		if !strings.Contains(finding.Message, want) {
			t.Errorf("message %q lacks %q", finding.Message, want)
		}
	}
	p := testvalidity.ApplyExampleSource(testvalidity.Projection{}, examples[6], "spec/order_spec.rb:6")
	if p.Hygiene.State != testvalidity.HygieneAbstained || p.Hygiene.Reason != testvalidity.RuleRBStubDecidesAssertion {
		t.Errorf("hygiene %+v, want ABSTAINED %s", p.Hygiene, testvalidity.RuleRBStubDecidesAssertion)
	}
}

const chainSpec = `RSpec.describe Notifier do
  it 'asserts only the builder' do
    allow(Mailer).to receive_message_chain(:build, :deliver_now)
    subject.notify
    expect(Mailer).to have_received(:build)
  end
  it 'uses a dotted chain' do
    allow(Mailer).to receive_message_chain("build.deliver_now")
    subject.notify
    expect(Mailer).to have_received(:build).with(user)
  end
  it 'uses a hash tail' do
    allow(Mailer).to receive_message_chain(:build, deliver_now: true)
    subject.notify
    expect(Mailer).to have_received(:build)
  end
  it 'expects the whole chain' do
    expect(Mailer).to receive_message_chain(:build, :deliver_now)
    subject.notify
  end
  it 'also asserts the final message' do
    allow(Mailer).to receive_message_chain(:build, :deliver_now)
    subject.notify
    expect(Mailer).to have_received(:build)
    expect(mail).to have_received(:deliver_now)
  end
  it 'asserts nothing on the chain' do
    allow(Mailer).to receive_message_chain(:build, :deliver_now)
    expect(subject.notify).to be(true)
  end
  it 'asserts another receiver' do
    allow(Mailer).to receive_message_chain(:build, :deliver_now)
    subject.notify
    expect(Logger).to have_received(:build)
  end
  it 'uses a computed chain' do
    allow(Mailer).to receive_message_chain(*messages)
    subject.notify
    expect(Mailer).to have_received(:build)
  end
  context 'with the chain in a hook' do
    before { allow(Mailer).to receive_message_chain(:build, :render, :deliver_now) }
    it 'asserts a middle message' do
      subject.notify
      expect(Mailer).to have_received(:build)
    end
  end
end
`

// TestMessageChainPrefixOnly traces TSA-V0-433 and TSA-V0-434: a receive_message_chain stub
// (symbol, dotted string or hash tail, in the body or a hook) asserted only on a prefix abstains;
// an expect chain, an asserted final message, no prefix assertion, another receiver and a computed
// chain give no finding.
func TestMessageChainPrefixOnly(t *testing.T) {
	got := stubBrief(t, "spec/notifier_spec.rb", chainSpec, nil)
	wantExamples(t, got, map[int][]string{
		2:  {"RB-MESSAGE-CHAIN-PREFIX-ONLY abstained spec/notifier_spec.rb:5 spec/notifier_spec.rb:3"},
		7:  {"RB-MESSAGE-CHAIN-PREFIX-ONLY abstained spec/notifier_spec.rb:10 spec/notifier_spec.rb:8"},
		12: {"RB-MESSAGE-CHAIN-PREFIX-ONLY abstained spec/notifier_spec.rb:15 spec/notifier_spec.rb:13"},
		43: {"RB-MESSAGE-CHAIN-PREFIX-ONLY abstained spec/notifier_spec.rb:45 spec/notifier_spec.rb:42"},
	})
	examples, _ := RSpecExampleSources("spec/notifier_spec.rb", chainSpec)
	message := examples[2].Findings[len(examples[2].Findings)-1].Message
	for _, want := range []string{"build.deliver_now", "spec/notifier_spec.rb:3", "spec/notifier_spec.rb:5", "have_received(:build)", "final message deliver_now", "receive_message_chain(...)"} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q lacks %q", message, want)
		}
	}
}

const flagSpec = `RSpec.describe Checkout do
  before { allow(Flipper).to receive(:enabled?).and_return(false) }
  it 'forces the new flow' do
    allow(Flipper).to receive(:enabled?).with(:new_checkout).and_return(true)
    expect(subject.flow).to eq(:new)
  end
  it 'reads ENV' do
    allow(ENV).to receive(:fetch).and_call_original
    allow(ENV).to receive(:fetch).with("CHECKOUT_MODE").and_return("fast")
    expect(subject.mode).to eq("fast")
  end
  it 'forces a computed key' do
    allow(Feature).to receive(:enabled?).with(flag_name).and_return(true)
    expect(subject.flow).to eq(:new)
  end
  it 'forces a computed value' do
    allow(Feature).to receive(:enabled?).with(:beta).and_return(beta_on)
    expect(subject.flow).to eq(:new)
  end
  context 'with an inner override' do
    before { allow(Flipper).to receive(:enabled?).and_return(true) }
    it 'uses the inner value' do
      expect(subject.flow).to eq(:new)
    end
  end
  it 'asserts the flag itself' do
    allow(Feature).to receive(:enabled?).and_return(true)
    subject.call
    expect(Feature.enabled?).to eq(true)
  end
end
`

func checkoutFlags() *testvalidity.FlagLookups {
	return &testvalidity.FlagLookups{Schema: testvalidity.FlagLookupsSchema, Source: testvalidity.FlagLookupsPath,
		Lookups:  []testvalidity.FlagLookup{{Receiver: "Flipper", Method: "enabled?"}, {Receiver: "ENV", Method: "fetch"}, {Receiver: "Feature", Method: "enabled?"}},
		Defaults: map[string]string{"new_checkout": "false", "CHECKOUT_MODE": "fast", "beta": "false"}}
}

// TestFlagConditions traces TSA-V0-435..438: a flag lookup stub is a condition with its key,
// forced value and declared default; the innermost hook or the body wins; a call-through is not a
// condition; an unread key or value is unresolved; a forced value that differs from the default
// warns; a flag stub never gives STUB-ON-PATH, and a non-flag stub of the same shape still does.
func TestFlagConditions(t *testing.T) {
	const s = "spec/checkout_spec.rb"
	hook := "COND " + s + ":2 Flipper.enabled? conditional on flag Flipper.enabled?(*)=false default="
	got := stubBrief(t, s, flagSpec, checkoutFlags())
	wantExamples(t, got, map[int][]string{
		3: {"RB-FLAG-CONDITIONAL advisory " + s + ":2 " + s + ":2", "RB-FLAG-CONDITIONAL advisory " + s + ":4 " + s + ":4",
			"RB-FLAG-FORCED-NON-DEFAULT advisory " + s + ":4 " + s + ":4",
			hook, "COND " + s + ":4 Flipper.enabled? conditional on flag new_checkout=true default=false"},
		7: {"RB-FLAG-CONDITIONAL advisory " + s + ":2 " + s + ":2", "RB-FLAG-CONDITIONAL advisory " + s + ":9 " + s + ":9",
			hook, "COND " + s + ":9 ENV.fetch conditional on flag CHECKOUT_MODE=fast default=fast"},
		12: {"RB-FLAG-CONDITIONAL advisory " + s + ":2 " + s + ":2", "RB-FLAG-CONDITIONAL advisory " + s + ":13 " + s + ":13",
			hook, "COND " + s + ":13 Feature.enabled? conditional on flag <unresolved>=true default="},
		16: {"RB-FLAG-CONDITIONAL advisory " + s + ":2 " + s + ":2", "RB-FLAG-CONDITIONAL advisory " + s + ":17 " + s + ":17",
			hook, "COND " + s + ":17 Feature.enabled? conditional on flag beta=<unresolved> default=false"},
		22: {"RB-FLAG-CONDITIONAL advisory " + s + ":21 " + s + ":21",
			"COND " + s + ":21 Flipper.enabled? conditional on flag Flipper.enabled?(*)=true default="},
		26: {"RB-FLAG-CONDITIONAL advisory " + s + ":2 " + s + ":2", "RB-FLAG-CONDITIONAL advisory " + s + ":27 " + s + ":27",
			hook, "COND " + s + ":27 Feature.enabled? conditional on flag Feature.enabled?(*)=true default="},
	})

	// Without Feature on the list the same stub is a first-party collaborator again.
	only := &testvalidity.FlagLookups{Schema: testvalidity.FlagLookupsSchema, Lookups: []testvalidity.FlagLookup{{Receiver: "Flipper", Method: "enabled?"}}}
	if rows := stubBrief(t, s, flagSpec, only)[26]; !slices.Contains(rows, "STUB-ON-PATH advisory "+s+":27 "+s+":29") {
		t.Errorf("non-flag stub rows %q lack STUB-ON-PATH", rows)
	}

	// The built-in list, with no declaration, classifies the same lookups and knows no default.
	builtin := stubBrief(t, s, flagSpec, nil)
	if rows := builtin[3]; slices.ContainsFunc(rows, func(r string) bool { return strings.HasPrefix(r, testvalidity.RuleRBFlagForcedNonDefault) }) ||
		!slices.Contains(rows, "COND "+s+":4 Flipper.enabled? conditional on flag new_checkout=true default=") {
		t.Errorf("built-in rows %q", rows)
	}

	examples, _ := RSpecExampleSourcesWithSupport(s, flagSpec, testvalidity.RubySupport{Flags: checkoutFlags()})
	var conditional, nonDefault string
	for _, f := range examples[3].Findings {
		switch {
		case f.Rule == testvalidity.RuleRBFlagConditional && f.Location == s+":4":
			conditional = f.Message
		case f.Rule == testvalidity.RuleRBFlagForcedNonDefault:
			nonDefault = f.Message
		}
	}
	for _, want := range []string{s + ":4", "Flipper.enabled?", "conditional on flag new_checkout=true", "declared code default is false (.corvint/flag-lookups.json)"} {
		if !strings.Contains(conditional, want) {
			t.Errorf("conditional message %q lacks %q", conditional, want)
		}
	}
	if !strings.Contains(nonDefault, "differs from its declared default false") || !strings.Contains(nonDefault, "second example") {
		t.Errorf("non-default message %q", nonDefault)
	}
	p := testvalidity.ApplyExampleSource(testvalidity.Projection{}, examples[3], s+":3")
	if p.Hygiene.State != testvalidity.HygieneEligible {
		t.Errorf("flag conditions changed hygiene: %+v", p.Hygiene)
	}
	// Discovery serializes the example source as sourceRules, so the conditions reach it.
	data, err := json.Marshal(examples[3])
	if err != nil || !strings.Contains(string(data), `"conditions":[{"lookup":"Flipper.enabled?","key":"*","forced":"false","location":"spec/checkout_spec.rb:2"`) {
		t.Errorf("serialized source %s %v", data, err)
	}
	if data, _ := json.Marshal(examples[0]); strings.Contains(string(data), "conditions") {
		t.Errorf("a source without conditions serializes them: %s", data)
	}
}

// TestDecodeFlagLookups traces TSA-V0-439: a closed declaration decodes; each defect refuses
// tests-declaration-invalid.
func TestDecodeFlagLookups(t *testing.T) {
	got, err := DecodeFlagLookups([]byte(`{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"::Settings","method":"feature_on?"}],"defaults":{"billing.v2":"false"}}`))
	if err != nil || got.Source != testvalidity.FlagLookupsPath || got.Lookups[0].Receiver != "Settings" || got.Defaults["billing.v2"] != "false" {
		t.Fatalf("decode = %+v %v", got, err)
	}
	if _, ok := got.Match("::Settings", "feature_on?"); !ok {
		t.Errorf("declared lookup does not match")
	}
	for name, doc := range map[string]string{
		"unknown field": `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"A","method":"b"}],"extra":1}`,
		"trailing":      `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"A","method":"b"}]} {}`,
		"schema":        `{"schema":"corvint-flag-lookups/1","lookups":[{"receiver":"A","method":"b"}]}`,
		"no lookups":    `{"schema":"corvint-flag-lookups/0","lookups":[]}`,
		"receiver":      `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"flipper","method":"b"}]}`,
		"method":        `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"A","method":"b c"}]}`,
		"duplicate":     `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"A","method":"b"},{"receiver":"A","method":"b"}]}`,
		"key":           `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"A","method":"b"}],"defaults":{"a b":"1"}}`,
		"empty value":   `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"A","method":"b"}],"defaults":{"k":""}}`,
		"control":       `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"A","method":"b"}],"defaults":{"k":"a\nb"}}`,
		"not json":      `[`,
	} {
		if _, err := DecodeFlagLookups([]byte(doc)); !hasCode(err, "tests-declaration-invalid") {
			t.Errorf("%s: %v, want tests-declaration-invalid", name, err)
		}
	}
}

const flagDeclaration = `{"schema":"corvint-flag-lookups/0","lookups":[{"receiver":"Flipper","method":"enabled?"}],"defaults":{"new_checkout":"false"}}`

const flagJoinSpec = `RSpec.describe Checkout do
  it 'forces the new flow' do
    allow(Flipper).to receive(:enabled?).with(:new_checkout).and_return(true)
    expect(subject.flow).to eq(:new)
  end
  it 'cancels nothing' do
    allow(order).to receive(:cancel).and_return(false)
    expect { subject.call }.not_to change { order.reload.cancelled? }
  end
end
`

// TestSourceValidityFlagConditions traces TSA-V0-439 and TSA-V0-440 for tests validity: the
// committed declaration's defaults reach the test's conditions, a stub-decided example abstains,
// and an invalid declaration refuses.
func TestSourceValidityFlagConditions(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"spec/checkout_spec.rb": flagJoinSpec, testvalidity.FlagLookupsPath: flagDeclaration})
	report, err := SourceValidity(OpenRepo(context.Background(), root), SourceValidityOptions{Files: []string{"spec/checkout_spec.rb"}})
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]SourceValidityTest{}
	for _, test := range report.Tests {
		tests[test.Test] = test
	}
	forced := tests["spec/checkout_spec.rb:2"]
	if forced.Hygiene.State != testvalidity.HygieneEligible || len(forced.Conditions) != 1 ||
		forced.Conditions[0].Qualifier != "conditional on flag new_checkout=true" || forced.Conditions[0].Default != "false" ||
		forced.Conditions[0].DefaultFrom != testvalidity.FlagLookupsPath {
		t.Errorf("flag test %+v", forced)
	}
	if decided := tests["spec/checkout_spec.rb:6"]; decided.Hygiene.State != testvalidity.HygieneAbstained ||
		decided.Hygiene.Reason != testvalidity.RuleRBStubDecidesAssertion || decided.Conditions != nil {
		t.Errorf("stub-decided test %+v", decided)
	}

	bad := t.TempDir()
	commitFiles(t, bad, map[string]string{"spec/checkout_spec.rb": flagJoinSpec, testvalidity.FlagLookupsPath: `{"schema":"x"}`})
	if _, err := SourceValidity(OpenRepo(context.Background(), bad), SourceValidityOptions{Files: []string{"spec/checkout_spec.rb"}}); !hasCode(err, "tests-declaration-invalid") {
		t.Errorf("invalid declaration: %v", err)
	}
}

// TestExampleJoinFlagConditions traces TSA-V0-440 for tests example-join: the verdict carries the
// condition and the source cites the declaration it was read with.
func TestExampleJoinFlagConditions(t *testing.T) {
	root := t.TempDir()
	head := commitFiles(t, root, map[string]string{"spec/checkout_spec.rb": flagJoinSpec, testvalidity.FlagLookupsPath: flagDeclaration})
	document := &rubytestprovider.Document{
		RepositoryBefore: repoAt(head, "clean"), RepositoryAfter: repoAt(head, "clean"),
		Examples: []rubytestprovider.Example{{
			ID: "./spec/checkout_spec.rb[1:1]", RSpecID: "./spec/checkout_spec.rb[1:1]", File: "spec/checkout_spec.rb", Line: 2,
			Description: "Checkout forces the new flow", Status: "passed", SpecBlob: blobAt(t, root, head, "spec/checkout_spec.rb"),
		}},
	}
	record := joinOne(t, root, ExampleJoinOptions{Revision: head, Ruby: document}).Examples[0]
	if len(record.Verdict.Conditions) != 1 || record.Verdict.Conditions[0].Qualifier != "conditional on flag new_checkout=true" ||
		record.Verdict.Conditions[0].Default != "false" {
		t.Errorf("verdict %+v", record.Verdict)
	}
	if !slices.ContainsFunc(record.Source.Files, func(f ExampleJoinFile) bool {
		return f.Path == testvalidity.FlagLookupsPath && f.Blob == blobAt(t, root, head, testvalidity.FlagLookupsPath)
	}) {
		t.Errorf("source files %+v lack the declaration", record.Source.Files)
	}
}

const stubEdgeSpec = `RSpec.describe CancelOrder do
  before { allow(record).to receive(:destroy!).and_return(true) }
  it('keeps the record') { expect { run }.not_to change { record.reload.persisted? } }
  it 'asserts the builder' do
    allow(Mailer).to receive_message_chain(:build, deliver_now:true)
    subject.notify
    expect(Mailer).to have_received(:build)
  end
end
`

const flagEdgeSpec = `RSpec.describe Checkout do
  before { allow(Feature).to receive(:enabled?).with("beta").and_return(false) }
  it 'overrides with the other quote' do
    allow(Feature).to receive(:enabled?).with('beta').and_return(true)
    expect(subject.flow).to eq(:new)
  end
  it 'calls through in the body' do
    allow(Feature).to receive(:enabled?).with(:beta).and_call_original
    expect(subject.flow).to eq(:old)
  end
  it 'returns without parentheses' do
    allow(Flipper).to receive(:enabled?).with(:new_checkout).and_return true
    expect(subject.flow).to eq(:new)
  end
end
`

// TestStubRuleEdges traces TSA-V0-431, TSA-V0-433 and TSA-V0-437 on review edges: a one-line
// example and a compact hash tail are read; one key spelled with either quote is one condition
// whose body override wins; a body call-through removes a hook's forced value; and an and_return
// argument without parentheses is unresolved, never nil.
func TestStubRuleEdges(t *testing.T) {
	got := stubBrief(t, "spec/order_spec.rb", stubEdgeSpec, nil)
	wantExamples(t, got, map[int][]string{
		3: {"RB-STUB-DECIDES-ASSERTION abstained spec/order_spec.rb:3 spec/order_spec.rb:2"},
		4: {"RB-MESSAGE-CHAIN-PREFIX-ONLY abstained spec/order_spec.rb:7 spec/order_spec.rb:5"},
	})
	const s = "spec/checkout_spec.rb"
	got = stubBrief(t, s, flagEdgeSpec, checkoutFlags())
	wantExamples(t, got, map[int][]string{
		3: {"RB-FLAG-CONDITIONAL advisory " + s + ":4 " + s + ":4", "RB-FLAG-FORCED-NON-DEFAULT advisory " + s + ":4 " + s + ":4",
			"COND " + s + ":4 Feature.enabled? conditional on flag beta=true default=false"},
		11: {"RB-FLAG-CONDITIONAL advisory " + s + ":2 " + s + ":2", "RB-FLAG-CONDITIONAL advisory " + s + ":12 " + s + ":12",
			"COND " + s + ":2 Feature.enabled? conditional on flag beta=false default=false",
			"COND " + s + ":12 Flipper.enabled? conditional on flag new_checkout=<unresolved> default=false"},
	})
}
