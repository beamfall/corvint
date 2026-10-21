package testsource

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// accessorBrief reads a spec against an application file set and keeps, per example, the
// TSA-V0-444 rows.
func accessorBrief(t *testing.T, name, text string, app *testvalidity.RubyApp) map[int][]string {
	t.Helper()
	examples, reason := RSpecExampleSourcesWithSupport(name, text, testvalidity.RubySupport{App: app})
	if reason != "" {
		t.Fatalf("%s: uncertain %q", name, reason)
	}
	out := map[int][]string{}
	for line, e := range examples {
		for _, f := range e.Findings {
			if f.Rule == testvalidity.RuleRBAssertTargetNeverWritten {
				out[line] = append(out[line], fmt.Sprintf("%s %s %s %s", f.Rule, f.Severity, f.Location, f.Cause))
			}
		}
	}
	return out
}

const accessorWidget = `class Widget
  attr_accessor :flag
  attr_reader :state

  def initialize(owner = nil)
    @owner = owner
  end

  def activate
    @state = :flagged
    log_change
  end

  def flag?
    @state == :flagged
  end

  private

  def log_change
    @changed = true
  end
end
`

const accessorLedger = `class Ledger < ApplicationRecord
  belongs_to :account
  attr_accessor :posted

  def post
    @posted_at = Time.current
  end
end
`

const accessorSchema = `ActiveRecord::Schema[7.1].define(version: 1) do
  create_table "ledgers" do |t|
    t.datetime "posted_at"
  end
  create_table "accounts" do |t|
    t.boolean "flag"
  end
end
`

func accessorApp(extra map[string]string) *testvalidity.RubyApp {
	files := map[string]string{"app/models/widget.rb": accessorWidget, "app/models/ledger.rb": accessorLedger, "db/schema.rb": accessorSchema}
	for path, text := range extra {
		files[path] = text
	}
	return testvalidity.RubyAppFiles(files)
}

const accessorSpec = `RSpec.describe Widget do
  let(:widget) { described_class.new }
  it 'flags the widget' do
    widget.activate
    expect(widget.flag).to be_nil
  end
  it 'flags a local widget' do
    w = Widget.new(owner)
    w.activate
    expect(w.flag?).to be(true)
    expect(w.flag).to eq(false)
  end
  context 'with a named subject' do
    subject(:named) { Widget.new }
    before { subject.activate }
    it 'reads the accessor through the subject alias' do
      expect(named.flag).to be_nil
    end
  end
  describe 'the implicit subject' do
    it 'reads the accessor' do
      subject.activate
      expect(subject.flag).to be_falsey
    end
  end
  it 'asserts a default without exercising anything' do
    expect(widget.flag).to be_nil
  end
  it 'asserts a predicate' do
    widget.activate
    expect(widget.flag?).to be(true)
  end
end

RSpec.describe Ledger do
  it 'posts the ledger' do
    ledger = Ledger.new
    ledger.post
    expect(ledger.posted).to be_nil
  end
end
`

// TestAssertTargetNeverWritten traces TSA-V0-443..446 and TSA-V0-449: an assertion of a plain
// accessor that no code reachable from the exercised methods writes abstains, located at the
// assertion with the first exercised call as cause, through a let, a local, a named subject and
// its alias, the implicit subject, and a record whose schema has no such column; an example that
// exercises nothing and an assertion of the predicate give no finding.
func TestAssertTargetNeverWritten(t *testing.T) {
	const s = "spec/widget_spec.rb"
	got := accessorBrief(t, s, accessorSpec, accessorApp(nil))
	wantExamples(t, got, map[int][]string{
		3:  {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":5 " + s + ":4"},
		7:  {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":11 " + s + ":9"},
		16: {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":17 " + s + ":15"},
		21: {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":23 " + s + ":22"},
		36: {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":39 " + s + ":38"},
	})
	examples, _ := RSpecExampleSourcesWithSupport(s, accessorSpec, testvalidity.RubySupport{App: accessorApp(nil)})
	var finding testvalidity.SourceFinding
	for _, f := range examples[3].Findings {
		if f.Rule == testvalidity.RuleRBAssertTargetNeverWritten {
			finding = f
		}
	}
	for _, want := range []string{"assertion target is never written on the exercised path", s + ":5", "expect(widget.flag).to be_nil",
		"widget.flag", "attr_accessor :flag", "app/models/widget.rb:2", "Widget#activate", s + ":4",
		"sibling predicate Widget#flag? at app/models/widget.rb:14", "widget.flag?", "hint:", "abstains"} {
		if !strings.Contains(finding.Message, want) {
			t.Errorf("message %q lacks %q", finding.Message, want)
		}
	}
	for _, f := range examples[36].Findings {
		if f.Rule == testvalidity.RuleRBAssertTargetNeverWritten && (!strings.Contains(f.Message, "Ledger#post") || strings.Contains(f.Message, "sibling")) {
			t.Errorf("record message %q", f.Message)
		}
	}
	p := testvalidity.ApplyExampleSource(testvalidity.Projection{}, examples[3], s+":3")
	if p.Hygiene.State != testvalidity.HygieneAbstained || p.Hygiene.Reason != testvalidity.RuleRBAssertTargetNeverWritten {
		t.Errorf("hygiene %+v, want ABSTAINED %s", p.Hygiene, testvalidity.RuleRBAssertTargetNeverWritten)
	}
	// No application source reads nothing (TSA-V0-448).
	if got := accessorBrief(t, s, accessorSpec, nil); len(got) != 0 {
		t.Errorf("without application source: %v", got)
	}
}

const accessorWriters = `class Switch
  attr_accessor :flag
  attr_writer :mode

  def set_ivar
    @flag = true
  end

  def set_self
    self.flag = true
  end

  def set_call
    self.flag=(true)
  end

  def set_or
    @flag ||= true
  end

  def set_peer
    @peer.flag = true
  end

  def set_nested
    helper
  end

  def set_symbol
    items.each(&method(:helper_by_symbol))
  end

  def noop
    @other = 1
  end

  private

  def helper
    @flag = false
  end

  def helper_by_symbol(item)
    @flag = item
  end
end
`

const accessorCallback = `class Account < ApplicationRecord
  attr_accessor :marked
  before_save :mark

  def close
    @closed = true
  end

  private

  def mark
    @marked = true
  end
end
`

const accessorProfile = `class Profile < ApplicationRecord
  attr_accessor :verified

  def reopen(params)
    assign_attributes(params)
  end

  def touch_up
    @touched = true
  end
end
`

const accessorStarted = `class Starter
  attr_accessor :flag

  def initialize
    @flag = false
  end

  def run
    @ran = true
  end
end
`

const accessorWriterSpec = `RSpec.describe Switch do
  let(:switch) { described_class.new }
  it 'writes the ivar' do
    switch.set_ivar
    expect(switch.flag).to be(true)
  end
  it 'assigns through self' do
    switch.set_self
    expect(switch.flag).to be(true)
  end
  it 'calls the setter' do
    switch.set_call
    expect(switch.flag).to be(true)
  end
  it 'or-assigns' do
    switch.set_or
    expect(switch.flag).to be(true)
  end
  it 'assigns an attribute of another object' do
    switch.set_peer
    expect(switch.flag).to be_nil
  end
  it 'writes in a helper' do
    switch.set_nested
    expect(switch.flag).to be(false)
  end
  it 'writes in a method passed by symbol' do
    switch.set_symbol
    expect(switch.flag).to eq(1)
  end
  it 'assigns in the example' do
    switch.noop
    switch.flag = true
    expect(switch.flag).to be(true)
  end
  context 'with a hook that assigns' do
    before { switch.flag = false }
    it 'reads the assigned value' do
      switch.noop
      expect(switch.flag).to be(false)
    end
  end
  it 'writes nothing' do
    switch.noop
    expect(switch.flag).to be_nil
  end
end

RSpec.describe Account do
  it 'marks in a callback' do
    account = Account.new
    account.close
    expect(account.marked).to be(true)
  end
end

RSpec.describe Profile do
  it 'mass-assigns the record' do
    profile = Profile.new
    profile.reopen(params)
    expect(profile.verified).to be(true)
  end
  it 'touches the record' do
    profile = Profile.new
    profile.touch_up
    expect(profile.verified).to be(true)
  end
end

RSpec.describe Starter do
  it 'starts unflagged in the constructor' do
    starter = Starter.new
    starter.run
    expect(starter.flag).to be(false)
  end
  it 'passes the value to new' do
    widget = Widget.new(flag: true)
    widget.activate
    expect(widget.flag).to be(true)
  end
end
`

// TestAssertTargetWritten traces TSA-V0-446: each writer form reachable from the exercised method
// (an instance variable assignment, self assignment, a setter call, an operator assignment, an
// attribute assignment on another receiver, a helper reached by name or by symbol), a spec-side
// assignment in the body or a hook, a record callback, a record mass assignment, the constructor
// and a hash argument to `.new` all give no finding; the control examples that write nothing
// still do.
func TestAssertTargetWritten(t *testing.T) {
	const s = "spec/switch_spec.rb"
	app := accessorApp(map[string]string{"app/models/switch.rb": accessorWriters, "app/models/account.rb": accessorCallback,
		"app/models/profile.rb": accessorProfile, "app/services/starter.rb": accessorStarted, "db/schema.rb": `ActiveRecord::Schema.define(version: 1) do
  create_table "accounts" do |t|
    t.datetime "closed_at"
  end
end
`})
	got := accessorBrief(t, s, accessorWriterSpec, app)
	wantExamples(t, got, map[int][]string{
		43: {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":45 " + s + ":44"},
		63: {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":66 " + s + ":65"},
	})
}

const accessorUncertain = `class Override
  attr_accessor :flag

  def flag
    @state
  end

  def run
    @state = 1
  end
end

class Mixed
  include Flaggable
  attr_accessor :flag

  def run
    @state = 1
  end
end

class Dynamic
  attr_accessor :flag

  def run(name)
    public_send(name, true)
  end
end

class Yielding
  attr_accessor :flag

  def run
    Notifier.call(self)
  end
end

class Defined
  attr_accessor :flag
  define_method(:run) { @flag = true }
end

class Sub < Base
  attr_accessor :flag

  def run
    @state = 1
  end
end
`

const accessorUncertainSpec = `RSpec.describe Widget do
  it 'reads a database column' do
    account = Account.new
    account.close
    expect(account.flag).to be_nil
  end
  it 'reads an overridden reader' do
    o = Override.new
    o.run
    expect(o.flag).to be_nil
  end
  it 'reads an unresolved class' do
    m = Missing.new
    m.run
    expect(m.flag).to be_nil
  end
  it 'reads a class with a module' do
    m = Mixed.new
    m.run
    expect(m.flag).to be_nil
  end
  it 'reads a class that defines methods dynamically' do
    d = Defined.new
    d.run
    expect(d.flag).to be_nil
  end
  it 'calls through public_send' do
    d = Dynamic.new
    d.run(:x)
    expect(d.flag).to be_nil
  end
  it 'passes self out' do
    y = Yielding.new
    y.run
    expect(y.flag).to be_nil
  end
  it 'reads an unresolved superclass' do
    s = Sub.new
    s.run
    expect(s.flag).to be_nil
  end
  it 'builds through a factory' do
    widget = build(:widget)
    widget.activate
    expect(widget.flag).to be_nil
  end
  it 'passes the receiver as an argument' do
    widget = Widget.new
    Processor.call(widget)
    expect(widget.flag).to be_nil
  end
  it 'stubs the receiver' do
    widget = Widget.new
    allow(widget).to receive(:activate)
    widget.activate
    expect(widget.flag).to be_nil
  end
  it 'calls an unresolved method' do
    widget = Widget.new
    widget.save
    expect(widget.flag).to be_nil
  end
  context 'with another let on the receiver' do
    let(:widget) { Widget.new }
    let(:result) { widget.activate }
    it 'reads through the other let' do
      result
      widget.activate
      expect(widget.flag).to be_nil
    end
  end
end
`

const accessorAroundSpec = `RSpec.describe Widget do
  let(:widget) { described_class.new }
  around { |example| widget.flag = true; example.run }
  it 'is written in an around hook' do
    widget.activate
    expect(widget.flag).to be(true)
  end
end
`

// TestAssertTargetUncertain traces TSA-V0-445 and TSA-V0-447: a reader a database column could
// back, an overridden reader, an unresolved class or superclass, a class-level include or
// define_method, a public_send or self escaping the reachable code, a factory-built receiver, a
// receiver passed out or stubbed, an unresolved exercised method, another let using the receiver,
// an around hook, a schema-less record and a missing application source give no finding.
func TestAssertTargetUncertain(t *testing.T) {
	const s = "spec/uncertain_spec.rb"
	// One file declares every uncertain class; each class's autoload path reads it.
	files := map[string]string{"app/models/widget.rb": accessorWidget, "app/models/ledger.rb": accessorLedger, "db/schema.rb": accessorSchema,
		"app/models/account.rb": "class Account < ApplicationRecord\n  attr_accessor :flag\n\n  def close\n    @closed = true\n  end\nend\n"}
	for _, class := range []string{"Override", "Mixed", "Dynamic", "Yielding", "Defined", "Sub"} {
		files["app/models/"+underscore(class)+".rb"] = accessorUncertain
	}
	app := testvalidity.RubyAppFiles(files)
	if got := accessorBrief(t, s, accessorUncertainSpec, app); len(got) != 0 {
		t.Errorf("uncertain readings gave findings: %v", got)
	}
	if got := accessorBrief(t, s, accessorAroundSpec, app); len(got) != 0 {
		t.Errorf("around hook gave findings: %v", got)
	}
	// A record whose schema is not committed could have the column.
	delete(files, "db/schema.rb")
	if got := accessorBrief(t, "spec/widget_spec.rb", accessorSpec, testvalidity.RubyAppFiles(files)); len(got[36]) != 0 {
		t.Errorf("schema-less record: %v", got[36])
	}
	// A second candidate file for one constant is ambiguous.
	files["lib/widget.rb"] = accessorWidget
	if got := accessorBrief(t, "spec/widget_spec.rb", accessorSpec, testvalidity.RubyAppFiles(files)); len(got[3]) != 0 {
		t.Errorf("ambiguous class file: %v", got[3])
	}
	for in, want := range map[string]string{"Widget": "widget", "HTTPClient": "http_client", "OAuth2Token": "o_auth2_token", "UserProfile": "user_profile"} {
		if got := underscore(in); got != want {
			t.Errorf("underscore(%s) = %s, want %s", in, got, want)
		}
	}
}

// TestSourceValidityAccessorNeverWritten traces TSA-V0-448: tests validity reads the committed
// application class under the spec's prefix and the example abstains with the finding's rule.
func TestSourceValidityAccessorNeverWritten(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"spec/widget_spec.rb": accessorSpec, "app/models/widget.rb": accessorWidget,
		"app/models/ledger.rb": accessorLedger, "db/schema.rb": accessorSchema})
	report, err := SourceValidity(OpenRepo(context.Background(), root), SourceValidityOptions{Files: []string{"spec/widget_spec.rb"}})
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]SourceValidityTest{}
	for _, test := range report.Tests {
		tests[test.Test] = test
	}
	if got := tests["spec/widget_spec.rb:3"]; got.Hygiene.State != testvalidity.HygieneAbstained || got.Hygiene.Reason != testvalidity.RuleRBAssertTargetNeverWritten {
		t.Errorf("never-written test %+v", got)
	}
	// The example that exercises nothing has no finding, so the report does not list it.
	if got, ok := tests["spec/widget_spec.rb:26"]; ok {
		t.Errorf("default-value test %+v", got)
	}
}

const accessorMulti = `class Multi
  attr_accessor :flag

  def ivars
    @flag, @state = true, :active
  end

  def selfs
    self.flag, other = true, 1
  end

  def nested
    (@state, @flag), rest = [1, 2], 3
  end

  def noop
    @other = 1
  end
end
`

const accessorBuilt = `class Built
  attr_accessor :flag

  def self.new(*args)
    super.tap { |b| b.flag = true }
  end

  def run
    @ran = true
  end
end
`

const accessorSingleton = `class Single
  attr_accessor :flag

  class << self
    def new
      allocate
    end
  end

  def run
    @ran = true
  end
end
`

const accessorBuiltChild = `class BuiltChild < Built
  attr_accessor :mode

  def run
    @ran = true
  end
end
`

const accessorWithheldSpec = `RSpec.describe Widget do
  let(:widget) { described_class.new }
  def seed_flag
    widget.flag = true
  end
  it 'passes a string key to new' do
    w = Widget.new("flag" => true)
    w.activate
    expect(w.flag).to be(true)
  end
  it 'passes a quoted symbol key to new' do
    w = Widget.new("flag": true)
    w.activate
    expect(w.flag).to be(true)
  end
  it 'passes attributes to a record' do
    ledger = Ledger.new(attrs)
    ledger.post
    expect(ledger.posted).to be(true)
  end
  context 'with a hook that calls a helper' do
    before { seed_flag }
    it 'reads the seeded flag' do
      widget.activate
      expect(widget.flag).to be(true)
    end
  end
  it 'calls a helper in the body' do
    seed_flag
    widget.activate
    expect(widget.flag).to be(true)
  end
  context 'with a let that calls a helper' do
    let(:seeded) { seed_flag }
    it 'reads the seeded flag' do
      seeded
      widget.activate
      expect(widget.flag).to be(true)
    end
  end
  it 'assigns two targets' do
    widget.flag, other = true, 1
    widget.activate
    expect(widget.flag).to be(true)
  end
  it 'still reads an unwritten flag' do
    widget.activate
    expect(widget.flag).to be_nil
  end
end

RSpec.describe Multi do
  it 'assigns ivars together' do
    m = Multi.new
    m.ivars
    expect(m.flag).to be(true)
  end
  it 'assigns through self together' do
    m = Multi.new
    m.selfs
    expect(m.flag).to be(true)
  end
  it 'assigns a nested target' do
    m = Multi.new
    m.nested
    expect(m.flag).to eq(2)
  end
  it 'writes nothing' do
    m = Multi.new
    m.noop
    expect(m.flag).to be_nil
  end
end

RSpec.describe Built do
  it 'builds through a custom new' do
    b = Built.new
    b.run
    expect(b.flag).to be(true)
  end
  it 'builds through a singleton new' do
    s = Single.new
    s.run
    expect(s.flag).to be_nil
  end
  it 'builds a subclass of a custom new' do
    c = BuiltChild.new
    c.run
    expect(c.flag).to be(true)
  end
end
`

// TestAssertTargetWithheld traces TSA-V0-446 and TSA-V0-447: a string-literal key passed to
// `.new`, any argument to a record's `.new`, a hook, body or let that calls a spec helper, a
// multiple assignment of the reader in the class or the example, a class-level `new` on the class
// or a superclass, a class another committed file reopens or patches, an application that cannot
// be read whole, and an incomplete support set all give no finding; the control examples that
// write nothing still do.
func TestAssertTargetWithheld(t *testing.T) {
	const s = "spec/withheld_spec.rb"
	files := map[string]string{"app/models/multi.rb": accessorMulti, "app/models/built.rb": accessorBuilt,
		"app/models/single.rb": accessorSingleton, "app/models/built_child.rb": accessorBuiltChild}
	got := accessorBrief(t, s, accessorWithheldSpec, accessorApp(files))
	wantExamples(t, got, map[int][]string{
		46: {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":48 " + s + ":47"},
		68: {"RB-ASSERT-TARGET-NEVER-WRITTEN abstained " + s + ":71 " + s + ":70"},
	})
	// An incomplete support set could hold a helper that writes the receiver.
	incomplete := testvalidity.RubySupport{Unbound: "rspec-support-not-given", App: accessorApp(files)}
	examples, _ := RSpecExampleSourcesWithSupport(s, accessorWithheldSpec, incomplete)
	for line, e := range examples {
		for _, f := range e.Findings {
			if f.Rule == testvalidity.RuleRBAssertTargetNeverWritten {
				t.Errorf("incomplete support set: finding at example %d", line)
			}
		}
	}
	// Reopenings and patches of Widget withhold example 3; of Ledger, example 36.
	const w = "spec/widget_spec.rb"
	for _, c := range []struct {
		name          string
		extra         map[string]string
		widget, ledgr bool
	}{
		{"a reopening class line", map[string]string{"lib/widget_extensions.rb": "class Widget\n  def activate\n    self.flag = true\n  end\nend\n"}, false, true},
		{"a namespaced reopening", map[string]string{"app/models/concerns/patch.rb": "module Patches\n  class ::Widget\n  end\nend\n"}, false, true},
		{"a class_eval initializer", map[string]string{"config/initializers/widget.rb": "Widget.class_eval do\n  attr_writer :flag\nend\n"}, false, true},
		{"a patch after the class", map[string]string{"app/models/widget.rb": accessorWidget + "Widget.prepend(Flagging)\n"}, false, true},
		{"a reopened record", map[string]string{"lib/ledger_patch.rb": "class Ledger\n  before_validation { self.posted = true }\nend\n"}, true, false},
		{"a subclass elsewhere", map[string]string{"app/models/gadget.rb": "class Gadget < Widget\nend\n"}, true, true},
		{"an application that does not read whole", nil, false, false},
	} {
		app := accessorApp(c.extra)
		if c.extra == nil {
			app.ReadAll = func() bool { return false }
		}
		got := accessorBrief(t, w, accessorSpec, app)
		if len(got[3]) == 1 != c.widget || len(got[36]) == 1 != c.ledgr {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

const accessorBudgetSpec = `RSpec.describe Widget do
  let(:widget) { described_class.new }
  it 'flags the widget' do
    widget.activate
    expect(widget.flag).to be_nil
  end
end
`

// TestSourceValidityAccessorBudget traces TSA-V0-447 and TSA-V0-448: every spec prefix of one
// command draws on one application-read budget, so once a prefix spends it the next prefix reads
// no application source and gives no finding; and a committed config/ file that patches the class
// withholds the finding through the whole-application read.
func TestSourceValidityAccessorBudget(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{
		"a/spec/widget_spec.rb": accessorBudgetSpec, "a/app/models/widget.rb": accessorWidget,
		"b/spec/widget_spec.rb": accessorBudgetSpec, "b/app/models/widget.rb": accessorWidget,
		"c/spec/widget_spec.rb": accessorBudgetSpec, "c/app/models/widget.rb": accessorWidget,
		"c/config/initializers/widget.rb": "Widget.class_eval do\n  attr_writer :flag\nend\n"})
	repo := OpenRepo(context.Background(), root)
	abstained := func(budget *readBudget) map[string]bool {
		t.Helper()
		report, err := sourceValidity(repo, SourceValidityOptions{Files: []string{"a/spec/widget_spec.rb", "b/spec/widget_spec.rb", "c/spec/widget_spec.rb"}}, budget)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, test := range report.Tests {
			if test.Hygiene.State == testvalidity.HygieneAbstained && test.Hygiene.Reason == testvalidity.RuleRBAssertTargetNeverWritten {
				out[test.Test] = true
			}
		}
		return out
	}
	both := map[string]bool{"a/spec/widget_spec.rb:3": true, "b/spec/widget_spec.rb:3": true}
	if got := abstained(newReadBudget()); !maps.Equal(got, both) {
		t.Errorf("full budget: %v, want %v", got, both)
	}
	// One file, or the bytes of one file, is spent by prefix a; prefix b reads nothing.
	size := len(accessorWidget) + 128
	for _, budget := range []*readBudget{{files: 1, bytes: maxTotalBytes}, {files: MaxFiles, bytes: size}} {
		if got := abstained(budget); !maps.Equal(got, map[string]bool{"a/spec/widget_spec.rb:3": true}) {
			t.Errorf("budget %+v: %v", *budget, got)
		}
	}
	if got := abstained(&readBudget{}); len(got) != 0 {
		t.Errorf("empty budget: %v", got)
	}
}
