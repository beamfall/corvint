package testsource

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// flowRows keeps the TSA-V0-120..130 rows of an example brief.
func flowRows(rows []string) []string {
	out := []string{}
	for _, row := range rows {
		for _, rule := range []string{testvalidity.RuleStubOnPath, testvalidity.RuleStubNeverInvoked, testvalidity.RuleAssertedValueNotFromSubject} {
			if strings.HasPrefix(row, rule+" ") {
				out = append(out, row)
			}
		}
	}
	return out
}

func flowBrief(got map[int][]string) map[int][]string {
	out := map[int][]string{}
	for line, rows := range got {
		if f := flowRows(rows); len(f) > 0 {
			out[line] = f
		}
	}
	return out
}

const stubPathSpec = `RSpec.describe Checkout do
  let(:gateway) { PaymentGateway.new }
  it 'returns the stubbed total' do
    allow(PriceCalculator).to receive(:total).and_return(42)
    result = subject.call
    expect(result.total).to eq(42)
  end
  it 'asserts the stub directly' do
    allow(TaxTable).to receive(:rate).and_return(0.2)
    subject.call
    expect(TaxTable.rate).to eq(0.2)
  end
  it 'stubs an HTTP client' do
    allow(Faraday).to receive(:get).and_return(response_body)
    expect(subject.call).to eq(response_body)
  end
  it 'stubs the clock' do
    allow(Time).to receive(:now).and_return(frozen)
    expect(subject.stamp).to eq(frozen)
  end
  it 'stubs a queue method' do
    allow(Mailer).to receive(:deliver_later).and_return(true)
    expect(subject.notify).to eq(true)
  end
  it 'stubs a first-party value the subject does not return' do
    allow(PriceCalculator).to receive(:total).and_return(42)
    expect(subject.call.count).to eq(3)
  end
  it 'stubs an unresolved receiver' do
    allow(calculator).to receive(:total).and_return(42)
    expect(subject.call).to eq(42)
  end
  it 'stubs a let-built collaborator' do
    allow(gateway).to receive(:charge).and_return(:ok)
    expect(subject.pay(gateway)).to eq(:ok)
  end
end
`

// TestStubOnPathRSpec traces TSA-V0-120..123: a first-party stub whose value the subject's result
// carries, or that the assertion calls, is on-path and advisory; a boundary or queue stub, a value
// the assertion does not check and an unresolved receiver give no finding.
func TestStubOnPathRSpec(t *testing.T) {
	got := flowBrief(exampleBrief(t, "spec/checkout_spec.rb", stubPathSpec))
	wantExamples(t, got, map[int][]string{
		3:  {"STUB-ON-PATH advisory spec/checkout_spec.rb:4 spec/checkout_spec.rb:6"},
		8:  {"STUB-ON-PATH advisory spec/checkout_spec.rb:9 spec/checkout_spec.rb:11"},
		33: {"STUB-ON-PATH advisory spec/checkout_spec.rb:34 spec/checkout_spec.rb:35"},
	})
}

const neverInvokedSpec = `RSpec.describe Report do
  let(:unused_client) { double }
  it 'stubs an object it never passes' do
    store = Store.new
    allow(store).to receive(:fetch).and_return([])
    expect(subject.rows).to eq([])
  end
  it 'stubs an object it passes' do
    store = Store.new
    allow(store).to receive(:fetch).and_return([])
    expect(subject.rows(store)).to eq([])
  end
  it 'stubs a let double nobody receives' do
    allow(unused_client).to receive(:get).and_return(1)
    expect(subject.rows).to eq([])
  end
  it 'stubs a let it hands on elsewhere' do
    allow(handed).to receive(:get).and_return(1)
    expect(subject.rows).to eq([])
  end
  it 'expects a message on a fresh object' do
    store = Store.new
    expect(store).to receive(:fetch)
    subject.rows
  end
  it 'stubs a constant' do
    allow(Store).to receive(:fetch).and_return([])
    expect(subject.rows).to eq([])
  end
end
`

// TestStubNeverInvokedRSpec traces TSA-V0-124: a silent stub on an object the example builds and
// never mentions again is reported as advisory; a passed object, a message expectation, an
// unresolved receiver and a constant are not.
func TestStubNeverInvokedRSpec(t *testing.T) {
	got := flowBrief(exampleBrief(t, "spec/report_spec.rb", neverInvokedSpec))
	wantExamples(t, got, map[int][]string{
		3:  {"STUB-NEVER-INVOKED advisory spec/report_spec.rb:5 spec/report_spec.rb:4"},
		13: {"STUB-NEVER-INVOKED advisory spec/report_spec.rb:14 spec/report_spec.rb:2"},
	})
}

const valueOriginSpec = `RSpec.describe Invoice do
  let(:invoice) { create(:invoice, total: 10) }
  let(:html) { Capybara.string('<p>hi</p>') }
  it 'asserts on a factory it built' do
    subject.call
    expect(invoice.total).to eq(10)
  end
  it 'asserts on a reloaded record' do
    subject.call
    expect(invoice.reload.total).to eq(20)
  end
  it 'asserts on a value the subject receives' do
    subject.call(invoice)
    expect(invoice.total).to eq(10)
  end
  it 'asserts on the subject' do
    expect(subject.total).to eq(10)
  end
  it 'asserts on a self-built page' do
    expect(html).to have_content('hi')
  end
  it 'asserts on a literal' do
    expect(1).to eq(1)
  end
  it 'asserts on an unknown call' do
    value = compute(3)
    expect(value).to eq(3)
  end
  it 'enqueues the job itself' do
    InvoiceJob.perform_later(7)
    expect(InvoiceJob).to have_been_enqueued.with(7)
  end
  it 'enqueues inside the block' do
    expect { InvoiceJob.perform_later(7) }.to have_enqueued_job(InvoiceJob).with(7)
  end
  it 'expects the subject to enqueue' do
    expect { subject.call }.to have_enqueued_job(InvoiceJob).with(7)
  end
  it 'enqueues different arguments' do
    InvoiceJob.perform_later(8)
    subject.call
    expect(InvoiceJob).to have_been_enqueued.with(7)
  end
  it 'enqueues a sidekiq job itself' do
    InvoiceWorker.perform_in(5, 7)
    expect(InvoiceWorker).to have_enqueued_sidekiq_job(7)
  end
end
`

// TestAssertedValueNotFromSubjectRSpec traces TSA-V0-126, TSA-V0-128 and TSA-V0-129: a value only
// the test built, and an enqueue expectation the test's own enqueue meets, are advisory findings;
// a reloaded record, a value handed to the subject, the subject's result, a literal and an
// unresolved call are not reported.
func TestAssertedValueNotFromSubjectRSpec(t *testing.T) {
	got := flowBrief(exampleBrief(t, "spec/invoice_spec.rb", valueOriginSpec))
	wantExamples(t, got, map[int][]string{
		4:  {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory spec/invoice_spec.rb:6 spec/invoice_spec.rb:2"},
		19: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory spec/invoice_spec.rb:20 spec/invoice_spec.rb:3"},
		29: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory spec/invoice_spec.rb:31 spec/invoice_spec.rb:30"},
		33: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory spec/invoice_spec.rb:34 spec/invoice_spec.rb:34"},
		44: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory spec/invoice_spec.rb:46 spec/invoice_spec.rb:45"},
	})
}

// TestFlowFindingsAdvisoryAndDeterministic traces TSA-V0-120 and TSA-V0-130: through the validity
// command every flow finding is advisory and leaves its test ELIGIBLE, and two reads of the same
// text give identical files and tests.
func TestFlowFindingsAdvisoryAndDeterministic(t *testing.T) {
	files := map[string]string{
		"spec/checkout_spec.rb": stubPathSpec, "spec/report_spec.rb": neverInvokedSpec,
		"spec/invoice_spec.rb": valueOriginSpec, "test/checkout.test.ts": jsStubPathSpec,
		"test/banner.test.ts": jsValueOriginSpec,
	}
	root := t.TempDir()
	commitFiles(t, root, files)
	names := slices.Sorted(maps.Keys(files))
	read := func() SourceValidityReport {
		report, err := SourceValidity(OpenRepo(context.Background(), root), SourceValidityOptions{Files: names})
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	report := read()
	if again := read(); !reflect.DeepEqual(report, again) {
		t.Fatal("two reads differ")
	}
	flagged := map[string]bool{}
	for _, file := range report.Files {
		found := false
		for _, f := range file.Findings {
			if len(flowRows([]string{f.Rule + " "})) == 0 {
				continue
			}
			if f.Severity != testvalidity.SeverityAdvisory {
				t.Fatalf("%s: %s at %s is %s", file.Path, f.Rule, f.Location, f.Severity)
			}
			flagged[f.Test], found = true, true
		}
		if !found {
			t.Fatalf("%s: no flow findings reached the command", file.Path)
		}
	}
	for _, test := range report.Tests {
		if flagged[test.Test] && test.Hygiene.State != testvalidity.HygieneEligible {
			t.Fatalf("%s hygiene %s", test.Test, test.Hygiene.State)
		}
	}
}

// TestStubOnPathMessages traces TSA-V0-122: an on-path finding names the stubbed symbol, the
// assertion line and the action line.
func TestStubOnPathMessages(t *testing.T) {
	for _, c := range []struct{ name, text, location, want string }{
		{"spec/checkout_spec.rb", stubPathSpec, "spec/checkout_spec.rb:4", "PriceCalculator.total is stubbed, and the assertion at spec/checkout_spec.rb:6 checks the stub's return value after the action at spec/checkout_spec.rb:5"},
		{"test/checkout.test.ts", jsStubPathSpec, "test/checkout.test.ts:10", "totalFor is stubbed, and the assertion at test/checkout.test.ts:12 checks the stub's return value after the action at test/checkout.test.ts:11"},
	} {
		examples, reason := ExampleSources(c.name, c.text)
		if reason != "" {
			t.Fatalf("%s: %s", c.name, reason)
		}
		found := false
		for _, e := range examples {
			for _, f := range e.Findings {
				if f.Rule == testvalidity.RuleStubOnPath && f.Location == c.location {
					found = true
					if !strings.Contains(f.Message, c.want) {
						t.Fatalf("%s: message %q lacks %q", c.location, f.Message, c.want)
					}
				}
			}
		}
		if !found {
			t.Fatalf("%s: no STUB-ON-PATH finding", c.location)
		}
	}
}

const flowReviewSpec = `RSpec.describe Invoice do
  it 'negates on a built value' do
    invoice = build(:invoice, total: 10)
    expect(invoice.total).not_to eq(0)
  end
  it 'interpolates the subject' do
    rendered = "#{subject.call}"
    expect(rendered).to eq("42")
  end
  it 'hands the value to the subject in an assertion' do
    invoice = build(:invoice, total: 0)
    expect(subject.update(invoice)).to be_truthy
    expect(invoice.total).to eq(42)
  end
  it 'stubs a boundary client it never passes' do
    client = Faraday.new
    allow(client).to receive(:get)
    expect(subject.call).to be_truthy
  end
  it 'negates a stubbed value' do
    allow(PriceCalculator).to receive(:total).and_return(42)
    expect(subject.call.total).not_to eq(42)
  end
  it 'still reports a built value read on its own' do
    invoice = build(:invoice, total: 10)
    expect(invoice.total).to eq(10)
  end
  it 'reads a stubbed object in an interpolation' do
    store = Store.new
    allow(store).to receive(:fetch).and_return([])
    expect(subject.rows("#{store.fetch}")).to eq([])
  end
end
`

// TestFlowReviewRSpec traces the #1184 review regressions for TSA-V0-122, TSA-V0-124 and
// TSA-V0-129: a negated assertion is neither on-path nor an origin finding, an interpolated
// subject call is not a test-built value, a value handed to the subject inside an expectation
// escapes, a boundary receiver is never a never-invoked stub, and a stubbed name read in an
// interpolated subject argument is a use, so the stub is on-path rather than never invoked.
func TestFlowReviewRSpec(t *testing.T) {
	got := flowBrief(exampleBrief(t, "spec/invoice_spec.rb", flowReviewSpec))
	wantExamples(t, got, map[int][]string{
		24: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory spec/invoice_spec.rb:26 spec/invoice_spec.rb:25"},
		28: {"STUB-ON-PATH advisory spec/invoice_spec.rb:30 spec/invoice_spec.rb:31"},
	})
}
