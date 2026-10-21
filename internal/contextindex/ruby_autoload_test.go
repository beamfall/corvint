package contextindex

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// rubyAutoloadIssueFiles is the GitHub #1077 fixture: a model and a controller
// that names it with no require, plus a JS module whose test imports it, so a
// direct import and a named-constant reference can be ranked side by side.
func rubyAutoloadIssueFiles() map[string]string {
	return map[string]string{
		"app/models/thing.rb":                       "class Thing < ApplicationRecord\n  def label = name\nend\n",
		"app/models/application_record.rb":          "class ApplicationRecord < ActiveRecord::Base\n  self.abstract_class = true\nend\n",
		"app/controllers/things_controller.rb":      "class ThingsController < ApplicationController\n  def show\n    @thing = Thing.find(params[:id])\n  end\nend\n",
		"app/controllers/application_controller.rb": "class ApplicationController < ActionController::Base\nend\n",
		"app/controllers/other_controller.rb":       "class OtherController < ApplicationController\n  def show\n    render plain: \"Thing\".length\n  end\nend\n",
		"spec/models/thing_spec.rb":                 "RSpec.describe Thing do\n  it { expect(Thing.new).to be_valid }\nend\n",
		"web/util.js":                               "export const util = 1;\n",
		"web/util.test.js":                          "import { util } from './util';\n",
		"web/types.ts":                              "export type Shape = { id: number };\n",
		"web/types.test.ts":                         "import type { Shape } from './types';\nexport const shape: Shape = { id: 1 };\n",
	}
}

func rubyResultRows(receipt map[string]any) map[string]map[string]any {
	rows := map[string]map[string]any{}
	for _, item := range mapsFromAny(receipt["results"]) {
		rows[stringValue(item["id"])] = item
	}
	return rows
}

func rubyUnknownCount(receipt map[string]any, code string) int {
	for _, row := range mapsFromAny(receipt["unknowns"]) {
		if row["code"] == code {
			count, _ := row["count"].(int)
			return count
		}
	}
	return 0
}

// TestRubyAutoloadNamedConstantReverseReference: NGI-V0-016, NGI-V0-018.
func TestRubyAutoloadNamedConstantReverseReference(t *testing.T) {
	t.Parallel()
	files := rubyAutoloadIssueFiles()
	receipt := registryImpact(t, files, "app/models/thing.rb")
	rows := rubyResultRows(receipt)
	controller, ok := rows["app/controllers/things_controller.rb"]
	if !ok {
		t.Fatalf("controller naming Thing is missing: %#v", receipt["results"])
	}
	if controller["kind"] != "reference" || controller["score"] != rubyConstantReferenceScore {
		t.Fatalf("controller row = %#v", controller)
	}
	evidenceRows := mapsFromAny(controller["evidence"])
	if len(evidenceRows) != 1 || evidenceRows[0]["line"] != 3 || evidenceRows[0]["confidence"] != "medium" || evidenceRows[0]["authority"] != "syntax" {
		t.Fatalf("controller evidence = %#v", evidenceRows)
	}
	spec, ok := rows["spec/models/thing_spec.rb"]
	if !ok || spec["kind"] != "test" || spec["score"] != rubyConstantTestScore || len(mapsFromAny(spec["evidence"])) != 2 {
		t.Fatalf("spec row = %#v", spec)
	}
	if _, wrong := rows["app/controllers/other_controller.rb"]; wrong {
		t.Fatal("a string literal naming Thing must not be a reverse reference")
	}
	if rubyUnknownCount(receipt, "ruby-constant-name-literal") != 1 {
		t.Fatalf("string class name not counted: %#v", receipt["unknowns"])
	}
	// NGI-V0-036: a mapped path has a named reverse-reference rule, so only
	// the dynamic-dispatch baseline stays.
	if rubyUnknownCount(receipt, "reverse-import-rule-unavailable") != 0 || rubyUnknownCount(receipt, "dynamic-dispatch-unresolved") != 1 {
		t.Fatalf("baseline frontier = %#v", receipt["unknowns"])
	}
	if controller["advisory"] != true || spec["advisory"] != true {
		t.Fatalf("convention rows must be advisory: %#v %#v", controller, spec)
	}
	// A direct import outranks every named-constant reference.
	imported := registryImpact(t, files, "web/util.js")
	importRow := rubyResultRows(imported)["web/util.test.js"]
	if importRow == nil || importRow["kind"] != "reverse-import" {
		t.Fatalf("import row = %#v", imported["results"])
	}
	importConfidence := mapsFromAny(importRow["evidence"])[0]["confidence"]
	if importRow["score"].(int) <= rubyConstantReferenceScore || importConfidence != "high" {
		t.Fatalf("import %v/%v must outrank the constant reference", importRow["score"], importConfidence)
	}
	// A type-only test import (the weakest import, 550) still outranks every
	// named-constant reference, so a mixed request under a result limit keeps
	// the import before any convention reference.
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	mixed, err := ImpactSyntax(index, []string{"app/models/thing.rb", "web/types.ts"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	order := map[string]int{}
	for position, item := range mapsFromAny(mixed["results"]) {
		order[stringValue(item["id"])] = position
	}
	typeOnly := rubyResultRows(mixed)["web/types.test.ts"]
	if typeOnly == nil || typeOnly["kind"] != "reverse-import" || typeOnly["score"] != 550 {
		t.Fatalf("type-only test import row = %#v", typeOnly)
	}
	if typeOnly["score"].(int) <= rubyConstantReferenceScore ||
		order["web/types.test.ts"] > order["app/controllers/things_controller.rb"] ||
		order["web/types.test.ts"] > order["spec/models/thing_spec.rb"] {
		t.Fatalf("a type-only import must rank above every constant reference: %v", order)
	}
	report, _ := receipt["ruby_autoload"].(map[string]any)
	if report == nil || report["roots_source"] != "default" ||
		!reflect.DeepEqual(report["roots"], []any{"app/controllers", "app/models"}) {
		t.Fatalf("ruby_autoload report = %#v", receipt["ruby_autoload"])
	}
	again := registryImpact(t, files, "app/models/thing.rb")
	first, _ := CanonicalJSON(receipt)
	second, _ := CanonicalJSON(again)
	if string(first) != string(second) {
		t.Fatal("ruby autoload impact is not byte-identical across runs")
	}
}

// TestRubyAutoloadNamespacedConstant: NGI-V0-016, NGI-V0-018.
func TestRubyAutoloadNamespacedConstant(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"app/services/billing/invoice.rb": "module Billing\n  class Invoice\n  end\nend\n",
		"app/services/billing/charge.rb":  "module Billing\n  class Charge\n    def run\n      if ready?\n        Invoice.new\n      end\n    end\n  end\nend\n",
		"app/services/invoice.rb":         "class Invoice\nend\n",
		"app/jobs/sync_job.rb":            "class SyncJob\n  def perform = ::Billing::Invoice.call\nend\n",
		"app/jobs/top_job.rb":             "class TopJob\n  def perform\n    Invoice.new\n  end\nend\n",
		"app/jobs/compact_job.rb":         "class Billing::CompactJob\n  def perform\n    Invoice.new\n  end\nend\n",
		"app/models/billing/line.rb":      "class Billing::Invoice::Line\nend\n",
	}
	receipt := registryImpact(t, files, "app/services/billing/invoice.rb")
	rows := rubyResultRows(receipt)
	for _, want := range []string{"app/services/billing/charge.rb", "app/jobs/sync_job.rb", "app/models/billing/line.rb"} {
		if row, ok := rows[want]; !ok || row["kind"] != "reference" {
			t.Fatalf("%s missing as a reference: %#v", want, receipt["results"])
		}
	}
	if line := mapsFromAny(rows["app/services/billing/charge.rb"]["evidence"])[0]["line"]; line != 5 {
		t.Fatalf("namespace-relative reference line = %v", line)
	}
	// A top-level `Invoice` names ::Invoice, and a compact `class
	// Billing::CompactJob` does not open Billing lexically, so neither
	// reaches Billing::Invoice.
	for _, absent := range []string{"app/jobs/top_job.rb", "app/jobs/compact_job.rb"} {
		if _, wrong := rows[absent]; wrong {
			t.Fatalf("%s must not reference Billing::Invoice", absent)
		}
	}
	top := rubyResultRows(registryImpact(t, files, "app/services/invoice.rb"))
	for _, want := range []string{"app/jobs/top_job.rb", "app/jobs/compact_job.rb"} {
		if _, ok := top[want]; !ok {
			t.Fatalf("%s must reference ::Invoice", want)
		}
	}
	if _, wrong := top["app/services/billing/charge.rb"]; wrong {
		t.Fatal("Billing::Charge's Invoice resolves to Billing::Invoice, not ::Invoice")
	}
}

// TestRubyAutoloadInflections: NGI-V0-017, NGI-V0-021.
func TestRubyAutoloadInflections(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"config/initializers/inflections.rb": "ActiveSupport::Inflector.inflections(:en) do |inflect|\n  inflect.acronym \"API\"\nend\n",
		"config/initializers/zeitwerk.rb":    "Rails.autoloaders.main.inflector.inflect(\n  \"html_parser\" => \"HTMLParser\",\n)\n",
		"app/clients/api_client.rb":          "class APIClient\nend\n",
		"app/parsers/html_parser.rb":         "class HTMLParser\nend\n",
		"app/jobs/fetch_job.rb":              "class FetchJob\n  def perform\n    APIClient.get(HTMLParser.new)\n  end\nend\n",
		"app/jobs/wrong_job.rb":              "class WrongJob\n  def perform\n    ApiClient.get\n  end\nend\n",
	}
	receipt := registryImpact(t, files, "app/clients/api_client.rb")
	rows := rubyResultRows(receipt)
	if _, ok := rows["app/jobs/fetch_job.rb"]; !ok {
		t.Fatalf("acronym inflection not applied: %#v", receipt["results"])
	}
	if _, wrong := rows["app/jobs/wrong_job.rb"]; wrong {
		t.Fatal("ApiClient is not the APIClient constant under the API acronym")
	}
	if _, ok := rubyResultRows(registryImpact(t, files, "app/parsers/html_parser.rb"))["app/jobs/fetch_job.rb"]; !ok {
		t.Fatal("inflect override not applied")
	}
	report := receipt["ruby_autoload"].(map[string]any)
	want := []any{
		map[string]any{"word": "api", "constant": "API", "kind": "acronym", "path": "config/initializers/inflections.rb", "line": 2},
		map[string]any{"word": "html_parser", "constant": "HTMLParser", "kind": "inflect", "path": "config/initializers/zeitwerk.rb", "line": 1},
	}
	if !reflect.DeepEqual(report["inflections"], want) {
		t.Fatalf("inflections = %#v", report["inflections"])
	}
	if rubyUnknownCount(receipt, "ruby-autoload-config-unresolved") != 0 {
		t.Fatalf("literal inflections must resolve: %#v", receipt["unknowns"])
	}

	files["config/initializers/more.rb"] = "ActiveSupport::Inflector.inflections { |i| i.acronym(name) }\nRails.autoloaders.main.collapse(\"app/x\")\n"
	files["config/application.rb"] = "config.autoload_paths << Rails.root.join(\"extras\")\n"
	if count := rubyUnknownCount(registryImpact(t, files, "app/clients/api_client.rb"), "ruby-autoload-config-unresolved"); count != 3 {
		t.Fatalf("non-literal acronym, collapse and autoload_paths = %d, want 3", count)
	}
}

// TestRubyAutoloadDeclaration: NGI-V0-017.
func TestRubyAutoloadDeclaration(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		RubyAutoloadDeclarationPath:  `{"schema": "ruby-autoload/0", "roots": ["src", "app/models"], "inflections": {"oauth": "OAuth"}}`,
		"src/oauth/token.rb":         "module OAuth\n  class Token\n  end\nend\n",
		"src/session.rb":             "class Session\n  def token\n    OAuth::Token.new\n  end\nend\n",
		"app/models/user.rb":         "class User\nend\n",
		"lib/tools/user_importer.rb": "class UserImporter\n  def run = User.all\nend\n",
		"config/application.rb":      "config.autoload_paths << Rails.root.join(\"src\")\n",
	}
	receipt := registryImpact(t, files, "src/oauth/token.rb")
	if _, ok := rubyResultRows(receipt)["src/session.rb"]; !ok {
		t.Fatalf("declared root/inflection not applied: %#v", receipt["results"])
	}
	report := receipt["ruby_autoload"].(map[string]any)
	if report["roots_source"] != "declared" || report["declaration"] != RubyAutoloadDeclarationPath ||
		!reflect.DeepEqual(report["roots"], []any{"app/models", "src"}) {
		t.Fatalf("report = %#v", report)
	}
	if rubyUnknownCount(receipt, "ruby-autoload-config-unresolved") != 0 {
		t.Fatal("a declaration stands in for autoload_paths configuration")
	}
	// lib is not a declared root, so lib/tools/user_importer.rb is unmapped.
	unmapped := registryImpact(t, files, "lib/tools/user_importer.rb")
	if rubyUnknownCount(unmapped, "ruby-constant-unmapped") != 1 {
		t.Fatalf("unmapped path = %#v", unmapped["unknowns"])
	}
	if _, ok := rubyResultRows(registryImpact(t, files, "app/models/user.rb"))["lib/tools/user_importer.rb"]; !ok {
		t.Fatal("an unmapped file still references a mapped constant")
	}

	for name, body := range map[string]string{
		"unknown member": `{"schema": "ruby-autoload/0", "roots": ["src"], "collapse": []}`,
		"escaping root":  `{"schema": "ruby-autoload/0", "roots": ["../src"]}`,
		"parent root":    `{"schema": "ruby-autoload/0", "roots": ["..", "src"]}`,
		"bad constant":   `{"schema": "ruby-autoload/0", "roots": ["src"], "inflections": {"oauth": "o-auth"}}`,
		"duplicate":      `{"schema": "ruby-autoload/0", "schema": "ruby-autoload/0", "roots": ["src"]}`,
	} {
		files[RubyAutoloadDeclarationPath] = body
		invalid := registryImpact(t, files, "src/oauth/token.rb")
		if rubyUnknownCount(invalid, "ruby-autoload-declaration-invalid") != 1 || len(rubyResultRows(invalid)) != 1 {
			t.Fatalf("%s: invalid declaration must map nothing: %#v %#v", name, invalid["results"], invalid["unknowns"])
		}
		if invalid["ruby_autoload"].(map[string]any)["roots_source"] != "declaration-invalid" {
			t.Fatalf("%s: report = %#v", name, invalid["ruby_autoload"])
		}
	}
}

// TestRubyAutoloadMetaprogrammingStaysUnknown: NGI-V0-019.
func TestRubyAutoloadMetaprogrammingStaysUnknown(t *testing.T) {
	files := map[string]string{
		"app/models/thing.rb":    "class Thing\nend\n",
		"app/models/owner.rb":    "class Owner < ApplicationRecord\n  belongs_to :thing\n  has_many :items, class_name: \"Thing\"\nend\n",
		"app/jobs/dynamic.rb":    "class Dynamic\n  def run(name)\n    name.constantize.find(1)\n    Object.const_get(\"Thing\")\n    self.class.send(:thing_finder)\n    # Thing.find in a comment\n    \"Thing\".safe_constantize\n  end\nend\n",
		"app/jobs/heredoc.rb":    "class Heredoc\n  SQL = <<~SQL\n    SELECT * FROM Thing\n  SQL\n  def label = { Thing: 1, :Thing => 2 }\nend\n",
		"app/jobs/unbalanced.rb": "module Jobs\n  class Unbalanced\n    def run\n      Thing.find\n    end\n",
	}
	receipt := registryImpact(t, files, "app/models/thing.rb")
	rows := rubyResultRows(receipt)
	for _, absent := range []string{"app/models/owner.rb", "app/jobs/dynamic.rb", "app/jobs/heredoc.rb"} {
		if _, wrong := rows[absent]; wrong {
			t.Fatalf("%s must stay unknown, not a reverse reference: %#v", absent, rows[absent])
		}
	}
	// An unbalanced file is resolved against every namespace it opens.
	if _, ok := rows["app/jobs/unbalanced.rb"]; !ok {
		t.Fatal("an unbalanced file must still report its references")
	}
	if count := rubyUnknownCount(receipt, "ruby-constant-lookup-dynamic"); count != 4 {
		t.Fatalf("constantize/const_get/send/safe_constantize sites = %d, want 4", count)
	}
	// "Thing" (class_name), "Thing" (const_get), "Thing" (safe_constantize) and :thing.
	if count := rubyUnknownCount(receipt, "ruby-constant-name-literal"); count != 4 {
		t.Fatalf("string/association names = %d, want 4: %#v", count, receipt["unknowns"])
	}
}

// TestRubyAutoloadDefaultOn: NGI-V0-020, NGI-V0-033. A default request
// naming a Ruby path selects the syntax profile, so its receipt is the syntax
// receipt; a request with no Ruby path keeps the historical default bytes.
func TestRubyAutoloadDefaultOn(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, rubyAutoloadIssueFiles()))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"app/models/thing.rb"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	if receipt["language_profile"] != "non-go-syntax-v0" || receipt["ruby_autoload"] == nil {
		t.Fatalf("a default Ruby request must carry the syntax profile: %#v", receipt)
	}
	if row := rubyResultRows(receipt)["app/controllers/things_controller.rb"]; row == nil || row["advisory"] != true {
		t.Fatalf("default receipt must carry the advisory controller row: %#v", receipt["results"])
	}
	syntax, err := ImpactSyntax(index, []string{"app/models/thing.rb"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	defaultBytes, _ := CanonicalJSON(receipt)
	syntaxBytes, _ := CanonicalJSON(syntax)
	if string(defaultBytes) != string(syntaxBytes) {
		t.Fatal("a default Ruby request must equal the syntax-profile receipt")
	}
	web, err := Impact(index, []string{"web/util.js"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"ruby_autoload", "language_profile", "unknowns"} {
		if _, present := web[member]; present {
			t.Fatalf("a default request with no Ruby path must not carry %s", member)
		}
	}
	webSyntax, err := ImpactSyntax(index, []string{"web/util.js"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := webSyntax["ruby_autoload"]; present {
		t.Fatal("a request with no Ruby path must not carry the Ruby member")
	}
}

// TestRubyAutoloadEndlessDef: NGI-V0-018. Ruby 3's endless `def` opens no block.
func TestRubyAutoloadEndlessDef(t *testing.T) {
	if !rubyEndlessDef("def label = name", 3) || !rubyEndlessDef("def self.x(a) = a", 3) || rubyEndlessDef("def x=(v)", 3) ||
		rubyEndlessDef("def ==(other)", 3) || rubyEndlessDef("def run", 3) {
		t.Fatal("endless def detection")
	}
}

// TestRubyAutoloadNestingBound: NGI-V0-018. Every block, not only a
// `module`/`class`, counts against the nesting bound, so a file nesting past
// it is resolved as unbalanced (never fewer references) in linear time.
func TestRubyAutoloadNestingBound(t *testing.T) {
	nested := func(depth int, body string) string {
		return "module Jobs\n" + strings.Repeat("begin\n", depth) + body + strings.Repeat("end\n", depth) + "end\nTail.new\n"
	}
	within := scanRubyConstantSource(Source{Data: []byte(nested(rubyNestingBound.limit-1, "Thing.new\n"))}, func(string) bool { return false })
	if !within.balanced {
		t.Fatal("nesting at the bound must stay balanced")
	}
	past := scanRubyConstantSource(Source{Data: []byte(nested(rubyNestingBound.limit, "Thing.new\n"))}, func(string) bool { return false })
	if past.balanced {
		t.Fatal("begin blocks past the nesting bound must mark the file unbalanced")
	}
	for _, source := range []*rubyConstantSource{within, past} {
		names := map[string][]string{}
		for _, reference := range source.references {
			names[reference.name] = reference.stack
		}
		if !reflect.DeepEqual(names["Thing"], []string{"Jobs"}) || !reflect.DeepEqual(names["Tail"], []string{}) {
			t.Fatalf("lexical stacks after deep nesting = %#v", names)
		}
	}
	// A deep file (well inside the source-size bound) scans in linear time
	// and, resolved as unbalanced, still reports its reference.
	files := map[string]string{
		"app/models/thing.rb": "class Thing\nend\n",
		"app/jobs/deep.rb":    nested(60000, "Thing.new\n"),
	}
	if _, ok := rubyResultRows(registryImpact(t, files, "app/models/thing.rb"))["app/jobs/deep.rb"]; !ok {
		t.Fatal("a file nesting past the bound must still report its reference")
	}
}

// TestRubyAutoloadQualifiedClassInNamespace: NGI-V0-018. `class A::B` inside
// `module N` reopens N::A::B only when N::A exists; otherwise it reopens the
// top-level A::B, and its body resolves in that class and then in N.
func TestRubyAutoloadQualifiedClassInNamespace(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"app/services/billing/invoice.rb":      "module Billing\n  class Invoice\n  end\nend\n",
		"app/services/billing/invoice/line.rb": "module Billing\n  class Invoice\n    class Line\n    end\n  end\nend\n",
		"app/patches/admin_invoice.rb":         "module Admin\n  class Billing::Invoice\n    def extra\n      Line.new\n    end\n  end\nend\n",
	}
	receipt := registryImpact(t, files, "app/services/billing/invoice.rb")
	row, ok := rubyResultRows(receipt)["app/patches/admin_invoice.rb"]
	if !ok || row["kind"] != "reference" || mapsFromAny(row["evidence"])[0]["line"] != 2 {
		t.Fatalf("reopening ::Billing::Invoice from module Admin must be a reference: %#v", receipt["results"])
	}
	if _, ok := rubyResultRows(registryImpact(t, files, "app/services/billing/invoice/line.rb"))["app/patches/admin_invoice.rb"]; !ok {
		t.Fatal("Line inside the reopened Billing::Invoice resolves to Billing::Invoice::Line")
	}
	files["app/models/admin/billing.rb"] = "module Admin\n  module Billing\n  end\nend\n"
	shadowed := rubyResultRows(registryImpact(t, files, "app/services/billing/invoice.rb"))
	if _, wrong := shadowed["app/patches/admin_invoice.rb"]; wrong {
		t.Fatal("with Admin::Billing present, class Billing::Invoice opens Admin::Billing::Invoice")
	}
}

// TestRubyAutoloadLocalConstantShadows: NGI-V0-018. A constant the file
// assigns in a namespace shadows an autoloaded constant of the same name
// there, and the assignment is not itself a reference.
func TestRubyAutoloadLocalConstantShadows(t *testing.T) {
	files := map[string]string{
		"app/models/thing.rb":   "class Thing\nend\n",
		"app/jobs/shadow.rb":    "module Jobs\n  Thing = Struct.new(:id)\n  def self.run\n    Thing.new\n  end\nend\n",
		"app/jobs/plain.rb":     "module Jobs\n  def self.run\n    Thing.new\n  end\nend\n",
		"app/jobs/top_level.rb": "Thing = Struct.new(:id)\n",
	}
	rows := rubyResultRows(registryImpact(t, files, "app/models/thing.rb"))
	for _, absent := range []string{"app/jobs/shadow.rb", "app/jobs/top_level.rb"} {
		if _, wrong := rows[absent]; wrong {
			t.Fatalf("%s must not reference the autoloaded ::Thing", absent)
		}
	}
	if _, ok := rows["app/jobs/plain.rb"]; !ok {
		t.Fatal("an unshadowed Thing in module Jobs still resolves to ::Thing")
	}
	source := scanRubyConstantSource(Source{Data: []byte(files["app/jobs/shadow.rb"])}, func(name string) bool { return name == "Thing" })
	for _, reference := range source.references {
		if reference.name == "Thing" && reference.line == 2 {
			t.Fatal("a constant assignment must not be collected as a reference")
		}
	}
	if !source.local["Jobs::Thing"] {
		t.Fatalf("local definitions = %v", source.local)
	}
}

// TestRubyAutoloadQualifiedAndOrAssignmentsDefine is V1-1714 (NGI-V0-018): a
// qualified `A::B = value` and a rooted `::X = value` define the constant in
// its resolved namespace, never reference it; the namespace a qualified
// target names is still a reference. `Name ||= value` reads and keeps an
// existing ::Name, so it stays a reference and defines nothing.
func TestRubyAutoloadQualifiedAndOrAssignmentsDefine(t *testing.T) {
	known := func(name string) bool { return name == "Thing" || name == "Admin" || name == "Admin::Thing" }
	for name, test := range map[string]struct {
		text    string
		defined string
		refs    []string
	}{
		"qualified":        {"Admin::Thing = Struct.new(:id)\n", "Admin::Thing", []string{"Admin"}},
		"rooted":           {"module Jobs\n  ::Thing = Struct.new(:id)\nend\n", "Thing", nil},
		"rooted qualified": {"module Jobs\n  ::Admin::Thing = 1\nend\n", "Admin::Thing", []string{"Admin"}},
		"or-assign":        {"module Jobs\n  Thing ||= Struct.new(:id)\nend\n", "", []string{"Thing"}},
		"nested qualified": {"module Admin\n  Admin::Thing = 1\nend\n", "Admin::Thing", []string{"Admin"}},
	} {
		source := scanRubyConstantSource(Source{Data: []byte(test.text)}, known)
		if test.defined == "" && source.local["Jobs::Thing"] || test.defined != "" && !source.local[test.defined] {
			t.Errorf("%s: local definitions = %v, want %s", name, source.local, test.defined)
		}
		refs := []string{}
		for _, reference := range source.references {
			if !reference.defined && reference.name != "Struct" {
				refs = append(refs, reference.name)
			}
		}
		if strings.Join(refs, ",") != strings.Join(test.refs, ",") {
			t.Errorf("%s: references = %v, want %v", name, refs, test.refs)
		}
	}
	files := map[string]string{
		"app/models/thing.rb":   "class Thing\nend\n",
		"app/jobs/rooted.rb":    "module Jobs\n  ::Thing = Struct.new(:id)\nend\n",
		"app/jobs/or_assign.rb": "module Jobs\n  Thing ||= Struct.new(:id)\n  def self.run\n    Thing.new\n  end\nend\n",
		"app/jobs/plain.rb":     "module Jobs\n  def self.run\n    Thing.new\n  end\nend\n",
	}
	rows := rubyResultRows(registryImpact(t, files, "app/models/thing.rb"))
	if _, wrong := rows["app/jobs/rooted.rb"]; wrong {
		t.Error("app/jobs/rooted.rb must not reference the autoloaded ::Thing")
	}
	for _, present := range []string{"app/jobs/plain.rb", "app/jobs/or_assign.rb"} {
		if _, ok := rows[present]; !ok {
			t.Errorf("%s: Thing (read by ||= or plain use) in module Jobs still resolves to ::Thing", present)
		}
	}
}
