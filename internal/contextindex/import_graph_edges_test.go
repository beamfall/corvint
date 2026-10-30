package contextindex

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// graphSummary reduces records to comparable edge and uncertainty strings.
func graphSummary(t *testing.T, rows []ImportGraphRecord) (edges []string, unknowns []string) {
	t.Helper()
	for _, row := range rows {
		switch row.Type {
		case "edge":
			edges = append(edges, row.From+" -> "+row.To+" "+row.Kind+" "+row.Evidence["specifier"].(string)+
				" line="+jsonString(t, row.Evidence["line"])+" authority="+row.Evidence["authority"].(string))
		case "uncertainty":
			unknowns = append(unknowns, jsonString(t, row.Unknown))
		}
	}
	return edges, unknowns
}

func jsonString(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func containsAll(t *testing.T, label string, got []string, want ...string) {
	t.Helper()
	for _, item := range want {
		found := false
		for _, value := range got {
			if strings.Contains(value, item) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: missing %s in\n%s", label, item, strings.Join(got, "\n"))
		}
	}
}

// NGI-V0-306: literal Angular component resources resolve, relative to the
// component, to indexed files; non-literal or unresolvable ones stay uncertain.
func TestImportGraphAngularResources(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"web/app/c.component.ts": "import { Component } from '@angular/core';\n" +
			"// @Component({templateUrl: './commented.html'})\n" +
			"@Component({\n" +
			"  selector: 'app-c',\n" +
			"  templateUrl: './c.component.html',\n" +
			"  styleUrls: ['./c.component.css', \"../shared/theme.scss\"],\n" +
			"})\n" +
			"export class C {}\n",
		"web/app/c.component.html": "<p>c</p>\n",
		"web/app/c.component.css":  "p {}\n",
		"web/app/commented.html":   "<p>no</p>\n",
		"web/shared/theme.scss":    "p {}\n",
		"web/app/d.component.ts": "@Component({\n" +
			"  templateUrl: TEMPLATE,\n" +
			"  styleUrl: `./${name}.css`,\n" +
			"})\n" +
			"export class D {}\n" +
			"@Component({ templateUrl: './missing.html', styleUrls: ['./d.css' + suffix, '/abs.css'] })\n" +
			"export class E {}\n",
		"web/app/s.component.ts": "@Component({ template: '<p></p>', styleUrl: './c.component.css' })\nexport class S {}\n",
		// A decorator spelled inside a nested template literal is string data.
		"web/app/n.component.ts": "const text = `${`@Component({templateUrl: './c.component.html'})`}`;\n",
		// Quoted member names are the same properties as identifier names.
		"web/app/q.component.ts": "@Component({ 'templateUrl': './c.component.html', \"styleUrls\": ['./c.component.css'] })\nexport class Q {}\n",
		"web/app/r.component.ts": "@Component({\n  \"styleUrl\": STYLE,\n})\nexport class R {}\n",
	})
	revision := strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))
	index, err := BuildImportGraph(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	rows, first := graphRecords(t, index, "")
	_, second := graphRecords(t, index, "")
	if !bytes.Equal(first, second) {
		t.Fatal("graph bytes differ")
	}
	edges, unknowns := graphSummary(t, rows)
	want := []string{
		"web/app/c.component.ts -> web/app/c.component.css angular-style ./c.component.css line=6 authority=syntax",
		"web/app/q.component.ts -> web/app/c.component.css angular-style ./c.component.css line=1 authority=syntax",
		"web/app/s.component.ts -> web/app/c.component.css angular-style ./c.component.css line=1 authority=syntax",
		"web/app/c.component.ts -> web/app/c.component.html angular-template ./c.component.html line=5 authority=syntax",
		"web/app/q.component.ts -> web/app/c.component.html angular-template ./c.component.html line=1 authority=syntax",
		"web/app/c.component.ts -> web/shared/theme.scss angular-style ../shared/theme.scss line=6 authority=syntax",
	}
	if !reflect.DeepEqual(edges, want) {
		t.Fatalf("edges:\n%s\nwant:\n%s", strings.Join(edges, "\n"), strings.Join(want, "\n"))
	}
	for _, row := range rows {
		if row.Type == "edge" && (row.Evidence["blob_hash"] != index.Sources[row.From].BlobHash || row.Revision != revision) {
			t.Fatalf("unbound edge: %+v", row)
		}
	}
	containsAll(t, "unknowns", unknowns,
		`"code":"angular-resource-nonliteral","language":"typescript","line":2,"path":"web/app/d.component.ts","property":"templateUrl"`,
		`"code":"angular-resource-nonliteral","language":"typescript","line":3,"path":"web/app/d.component.ts","property":"styleUrl"`,
		`"candidate":"web/app/missing.html","code":"angular-resource-unresolved","language":"typescript","line":6,"path":"web/app/d.component.ts","property":"templateUrl"`,
		`"code":"angular-resource-nonliteral","language":"typescript","line":6,"path":"web/app/d.component.ts","property":"styleUrls"`,
		`"code":"angular-resource-unresolved","language":"typescript","line":6,"path":"web/app/d.component.ts","property":"styleUrls"`,
		`"code":"angular-resource-nonliteral","language":"typescript","line":2,"path":"web/app/r.component.ts","property":"styleUrl"`,
		`"code":"string-keyed-di-unresolved"`,
	)
	for _, value := range unknowns {
		if strings.Contains(value, "angular-resource") && (strings.Contains(value, "commented.html") || strings.Contains(value, "n.component.ts")) {
			t.Fatalf("commented or string decorator read: %s", value)
		}
	}
	// NGI-V0-308: filters remove only edges, never uncertainties.
	filtered, _ := graphRecords(t, index, "angular-template")
	filteredEdges, filteredUnknowns := graphSummary(t, filtered)
	if !reflect.DeepEqual(filteredEdges, want[3:5]) || !reflect.DeepEqual(filteredUnknowns, unknowns) {
		t.Fatalf("filtered edges %v unknowns changed=%v", filteredEdges, !reflect.DeepEqual(filteredUnknowns, unknowns))
	}
}

// NGI-V0-307: a Zeitwerk constant reference with exactly one candidate file is
// a convention edge; zero or several candidates are uncertainty naming them.
func TestImportGraphRubyConstants(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"app/models/application_record.rb": "class ApplicationRecord\nend\n",
		"app/models/user.rb":               "class User < ApplicationRecord\nend\n",
		"app/models/report.rb":             "class Report; end\n",
		"lib/report.rb":                    "class Report; end\n",
		"app/services/billing/tax_rate.rb": "module Billing\n  class TaxRate\n  end\nend\n",
		"app/services/billing/invoice_builder.rb": "module Billing\n  class InvoiceBuilder\n    def run\n" +
			"      User.find(1)\n      Billing::TaxRate.call\n      Report.new\n      Missing::Thing.call\n      User.first\n    end\n  end\nend\n",
		"spec/models/user_spec.rb": "RSpec.describe User do\nend\n",
	})
	revision := strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))
	index, err := BuildImportGraph(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	rows, original := graphRecords(t, index, "")
	edges, unknowns := graphSummary(t, rows)
	want := []string{
		"app/models/user.rb -> app/models/application_record.rb ruby-constant ApplicationRecord line=1 authority=convention",
		"app/services/billing/invoice_builder.rb -> app/models/user.rb ruby-constant User line=4 authority=convention",
		"spec/models/user_spec.rb -> app/models/user.rb ruby-constant User line=1 authority=convention",
		"app/services/billing/invoice_builder.rb -> app/services/billing/tax_rate.rb ruby-constant Billing::TaxRate line=5 authority=convention",
	}
	if !reflect.DeepEqual(edges, want) {
		t.Fatalf("edges:\n%s\nwant:\n%s", strings.Join(edges, "\n"), strings.Join(want, "\n"))
	}
	for _, row := range rows {
		if row.Type == "edge" && row.Evidence["convention"] != "zeitwerk" {
			t.Fatalf("edge not marked convention-derived: %+v", row)
		}
	}
	containsAll(t, "unknowns", unknowns,
		`"candidates":["app/models/report.rb","lib/report.rb"],"code":"ruby-constant-ambiguous","constants":["Report"],"count":1,"language":"ruby","line":6,"path":"app/services/billing/invoice_builder.rb"`,
		`"candidates":[],"code":"ruby-constant-unresolved","constants":["Missing::Thing"],"count":1,"language":"ruby","line":7,"path":"app/services/billing/invoice_builder.rb"`,
		`"candidates":[],"code":"ruby-constant-unresolved","constants":["RSpec"],"count":1,"language":"ruby","line":1,"path":"spec/models/user_spec.rb"`,
		`"code":"ruby-autoload-not-import"`,
	)
	// An implicit namespace (no billing.rb) is neither an edge nor an unknown.
	for _, value := range unknowns {
		if strings.Contains(value, `"constants":["Billing"]`) {
			t.Fatalf("implicit namespace reported: %s", value)
		}
	}
	// NGI-V0-308: an import-only filter keeps every Ruby uncertainty.
	filtered, _ := graphRecords(t, index, "import")
	filteredEdges, filteredUnknowns := graphSummary(t, filtered)
	if len(filteredEdges) != 0 || !reflect.DeepEqual(filteredUnknowns, unknowns) {
		t.Fatalf("import filter: edges=%v unknowns changed=%v", filteredEdges, !reflect.DeepEqual(filteredUnknowns, unknowns))
	}

	// NGI-V0-307, NGI-V0-301: removing the duplicate at a later commit makes
	// Report an edge there, while the pinned commit keeps its bytes.
	testGit(t, root, "rm", "-q", "lib/report.rb")
	testGit(t, root, "commit", "-qm", "drop duplicate")
	writeTestFile(t, root, "app/models/user.rb", "dirty\n")
	later, err := BuildImportGraph(t.Context(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	laterRows, _ := graphRecords(t, later, "ruby-constant")
	laterEdges, _ := graphSummary(t, laterRows)
	containsAll(t, "later", laterEdges, "app/services/billing/invoice_builder.rb -> app/models/report.rb ruby-constant Report line=6")
	pinned, err := BuildImportGraph(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, again := graphRecords(t, pinned, ""); !bytes.Equal(original, again) {
		t.Fatal("later HEAD or dirty bytes changed the pinned graph")
	}
}

// NGI-V0-307: an unbalanced file resolves a reference against several
// namespaces, so even a single candidate file stays uncertain.
func TestImportGraphRubyIncompleteResolution(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"app/models/admin/user.rb": "module Admin\n  class User\n  end\nend\n",
		"app/models/user.rb":       "class User\nend\n",
		"app/services/broken.rb":   "module Admin\n  def run\n    User.call\n",
		// One mapped candidate still does not make an unbalanced file's
		// resolution complete.
		"app/models/widget.rb":   "class Widget\nend\n",
		"app/services/opened.rb": "class Opened\n  def run\n    Widget.call\n",
	})
	index, err := BuildImportGraph(t.Context(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := graphRecords(t, index, "ruby-constant")
	edges, unknowns := graphSummary(t, rows)
	for _, edge := range edges {
		if strings.HasPrefix(edge, "app/services/broken.rb") || strings.HasPrefix(edge, "app/services/opened.rb") {
			t.Fatalf("edge from an unbalanced file: %s", edge)
		}
	}
	containsAll(t, "unknowns", unknowns,
		`"candidates":["app/models/admin/user.rb","app/models/user.rb"],"code":"ruby-constant-ambiguous","constants":["Admin::User","User"]`,
		`"candidates":["app/models/widget.rb"],"code":"ruby-constant-ambiguous","constants":["Widget"],"count":1,"language":"ruby","line":3,"path":"app/services/opened.rb"`)
}

// NGI-V0-308: the closed kinds filter admits the new kinds once each.
func TestImportGraphKindsClosedSet(t *testing.T) {
	kinds, err := ImportGraphKinds("ruby-constant,angular-template,angular-style,type-import,import")
	if err != nil || len(kinds) != 5 {
		t.Fatalf("kinds=%v err=%v", kinds, err)
	}
	for _, value := range []string{"ruby-constant,ruby-constant", "angular", "ruby", "angular-template,", "Ruby-constant"} {
		if _, err := ImportGraphKinds(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
