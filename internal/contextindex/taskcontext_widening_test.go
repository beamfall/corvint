package contextindex

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

// contextSelectors renders coverage.critical or coverage.critical_missing as
// "relation path" strings in the order the packet carries them.
func contextSelectors(t *testing.T, coverage map[string]any, member string) []string {
	t.Helper()
	rendered := make([]string, 0)
	for _, item := range mapsFromAny(coverage[member]) {
		rendered = append(rendered, item["relation"].(string)+" "+item["path"].(string))
	}
	return rendered
}

// contextUnexamined maps each relation to its state and withheld count; a
// relation whose count is null maps to a nil count.
func contextUnexamined(t *testing.T, coverage map[string]any) (map[string]string, map[string]any) {
	t.Helper()
	states, withheld := map[string]string{}, map[string]any{}
	for _, item := range mapsFromAny(coverage["unexamined"]) {
		relation := item["relation"].(string)
		states[relation] = item["state"].(string)
		withheld[relation] = item["withheld"]
	}
	return states, withheld
}

func contextCoverage(t *testing.T, packet map[string]any) map[string]any {
	t.Helper()
	return packet["coverage"].(map[string]any)
}

// contextPairs renders the results as "kind path" strings in packet order.
func contextPairs(t *testing.T, packet map[string]any, skip ...string) []string {
	t.Helper()
	rendered := make([]string, 0)
	for _, row := range mapsFromAny(packet["results"]) {
		if slices.Contains(skip, row["id"].(string)) {
			continue
		}
		rendered = append(rendered, row["kind"].(string)+" "+row["id"].(string))
	}
	return rendered
}

func contextIntValue(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case float64:
		return int(number)
	}
	return -1
}

// TestTaskContextReservesInstructionsForUnrelatedVocabulary is TCP-V0-012's
// falsifier (a): the governing reservation never consults the task's
// vocabulary, so a task sharing no term with the instruction file's body or
// path still reserves it, by the literal precedence and not by relevance.
func TestTaskContextReservesInstructionsForUnrelatedVocabulary(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                    "module example.test/gov\n\ngo 1.27.0\n",
		"AGENTS.md":                 "Zorbulator wibfrast policy.\n",
		"CLAUDE.md":                 "Zorbulator wibfrast addendum.\n",
		"third_party/lib/AGENTS.md": "Nested instructions do not govern.\n",
		"widget/ledger.go":          "package widget\n\nfunc Ledger() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	task := "Reconcile the quantum widget ledger"
	t.Run("TCP-V0-008 disjoint vocabulary still reserves the instruction file", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, task, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		row := mapsFromAny(packet["results"])[0]
		if row["kind"] != "governing" || row["id"] != "AGENTS.md" || contextIntValue(row["score"]) != 1000 {
			t.Fatalf("first row = %v, want the governing reservation at 1000", row)
		}
		if row["action"] != "Read this project's standing instructions before changing anything." {
			t.Fatalf("governing action = %v", row["action"])
		}
		evidence := mapsFromAny(row["evidence"])
		if len(evidence) != 1 || evidence[0]["authority"] != "project-instructions" {
			t.Fatalf("governing evidence = %v, want one project-instructions row", evidence)
		}
		coverage := contextCoverage(t, packet)
		if coverage["governance"] != "reserved" {
			t.Fatalf("governance = %v", coverage["governance"])
		}
		if got := contextSelectors(t, coverage, "critical"); !slices.Equal(got, []string{"governing AGENTS.md"}) {
			t.Fatalf("critical = %v", got)
		}
		// CLAUDE.md is the sole applicable lower-precedence remainder. The
		// nested AGENTS.md is not a governing candidate.
		states, withheld := contextUnexamined(t, coverage)
		if states["governing"] != "examined" || contextIntValue(withheld["governing"]) != 1 {
			t.Fatalf("governing scope = %v / %v", states["governing"], withheld["governing"])
		}
	})
	t.Run("TCP-V0-008 nested instruction files never govern", func(t *testing.T) {
		nestedOnlyRoot := impactRepositoryWithFiles(t, map[string]string{
			"go.mod":                    "module example.test/nested\n\ngo 1.27.0\n",
			"third_party/lib/AGENTS.md": "Nested instructions do not govern.\n",
			"widget/ledger.go":          "package widget\n\nfunc Ledger() {}\n",
		})
		nestedOnly, err := Build(context.Background(), nestedOnlyRoot)
		if err != nil {
			t.Fatal(err)
		}
		packet, err := TaskContext(context.Background(), nestedOnly, task, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range mapsFromAny(packet["results"]) {
			if row["kind"] == "governing" {
				t.Fatalf("nested instruction file governed: %v", row)
			}
		}
		if governance := contextCoverage(t, packet)["governance"]; governance != "unresolved" {
			t.Fatalf("governance = %v, want unresolved", governance)
		}
	})
	t.Run("TCP-V0-008 an unread candidate reserves nothing and substitutes nothing", func(t *testing.T) {
		capped := *index
		capped.Sources = map[string]Source{}
		for path, source := range index.Sources {
			capped.Sources[path] = source
		}
		delete(capped.Sources, "AGENTS.md")
		capped.Exclusions = append(slices.Clone(index.Exclusions), Exclusion{"AGENTS.md", "over the source bound"})
		capped.Vocabulary = nil
		packet, err := TaskContext(context.Background(), &capped, task, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range mapsFromAny(packet["results"]) {
			if row["kind"] == "governing" {
				t.Fatalf("an unread candidate reserved a row: %v", row)
			}
		}
		coverage := contextCoverage(t, packet)
		states, _ := contextUnexamined(t, coverage)
		if states["governing"] != "capped" || coverage["governance"] != "unresolved" {
			t.Fatalf("capped scope = %v, governance = %v", states["governing"], coverage["governance"])
		}
		if got := contextSelectors(t, coverage, "critical"); len(got) != 0 {
			t.Fatalf("critical = %v, want no reservation at all", got)
		}
	})
}

func specMentionFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                 "module example.test/spec\n\ngo 1.27.0\n",
		"docs/specs/alpha-v0.md": "# Alpha\n\n- `ALP-001`: the alpha clause.\n- **`SHR-002`**: shared here.\n",
		"docs/specs/beta-v0.md":  "# Beta\n\n`SHR-002`. shared there too.\n",
		"docs/specs/README.md":   "- `RDM-001`: the readme clause the resolver must not read.\n",
		"src/alpha.go":           "package src\n\nfunc Alpha() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

// TestTaskContextAdmitsSpecsNamedByIdOrPath covers TCP-V0-009's resolver: an
// id with a defining clause, an ambiguous id, a named path, and the two
// negatives -- prose that looks like an id, and the README the pass excludes.
func TestTaskContextAdmitsSpecsNamedByIdOrPath(t *testing.T) {
	index := specMentionFixture(t)
	specRows := func(t *testing.T, task string) ([]map[string]any, map[string]any) {
		t.Helper()
		packet, err := TaskContext(context.Background(), index, task, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		rows := make([]map[string]any, 0)
		for _, row := range mapsFromAny(packet["results"]) {
			if row["kind"] == "spec-mentioned" {
				rows = append(rows, row)
			}
		}
		return rows, packet
	}
	t.Run("TCP-V0-009 a defining clause admits its spec", func(t *testing.T) {
		rows, packet := specRows(t, "apply ALP-001 to the alpha path")
		coverage := contextCoverage(t, packet)
		if len(rows) != 1 || rows[0]["id"] != "docs/specs/alpha-v0.md" {
			t.Fatalf("spec rows = %v", rows)
		}
		if contextIntValue(rows[0]["score"]) != 900 {
			t.Fatalf("score = %v, want 900", rows[0]["score"])
		}
		evidence := mapsFromAny(rows[0]["evidence"])[0]
		if evidence["reason"] != "defines ALP-001" || evidence["authority"] != "repository-spec" {
			t.Fatalf("evidence = %v", evidence)
		}
		if contextIntValue(evidence["line"]) != 3 {
			t.Fatalf("evidence line = %v, want the clause line", evidence["line"])
		}
		if coverage["governance"] != "spec-mentioned" {
			t.Fatalf("governance = %v", coverage["governance"])
		}
	})
	t.Run("TCP-V0-009 several definers admit the first and name the ambiguity", func(t *testing.T) {
		rows, packet := specRows(t, "apply SHR-002")
		coverage := contextCoverage(t, packet)
		if len(rows) != 1 || rows[0]["id"] != "docs/specs/alpha-v0.md" {
			t.Fatalf("spec rows = %v, want the first definer by path", rows)
		}
		if reason := mapsFromAny(rows[0]["evidence"])[0]["reason"]; reason != "defines SHR-002; ambiguous-definition (2 specs)" {
			t.Fatalf("reason = %v", reason)
		}
		// beta-v0.md is the definer the cap-fitting rule passed over. It still
		// carries the id, so the documentation class admits it, and TCP-V0-011 does
		// not count a candidate admitted under another relation as withheld.
		if !slices.Contains(contextPairs(t, packet), "documentation docs/specs/beta-v0.md") {
			t.Fatalf("the passed-over definer left the packet: %v", contextPairs(t, packet))
		}
		_, withheld := contextUnexamined(t, coverage)
		if contextIntValue(withheld["spec-mentioned"]) != 0 {
			t.Fatalf("spec-mentioned withheld = %v, want the admitted definer uncounted", withheld["spec-mentioned"])
		}
	})
	t.Run("TCP-V0-009 a named tracked spec path is admitted", func(t *testing.T) {
		rows, _ := specRows(t, "read docs/specs/beta-v0.md before changing anything")
		if len(rows) != 1 || rows[0]["id"] != "docs/specs/beta-v0.md" {
			t.Fatalf("spec rows = %v", rows)
		}
		if reason := mapsFromAny(rows[0]["evidence"])[0]["reason"]; reason != "the task names docs/specs/beta-v0.md" {
			t.Fatalf("reason = %v", reason)
		}
	})
	t.Run("TCP-V0-009 prose shaped like an id and the README admit nothing", func(t *testing.T) {
		for _, task := range []string{"conform to ISO-123 here", "apply RDM-001 to the tree"} {
			rows, packet := specRows(t, task)
			coverage := contextCoverage(t, packet)
			if len(rows) != 0 {
				t.Fatalf("%q admitted %v", task, rows)
			}
			states, _ := contextUnexamined(t, coverage)
			if states["spec-mentioned"] != "examined" || coverage["governance"] != "unresolved" {
				t.Fatalf("%q scope = %v, governance = %v", task, states["spec-mentioned"], coverage["governance"])
			}
		}
	})
}

// decoyFixture carries a title-overlap decoy spec that defines nothing and a
// defining spec whose only task link is the requirement id.
func decoyFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                  "module example.test/decoy\n\ngo 1.27.0\n",
		"AGENTS.md":               "Repository handler rules.\n",
		"handler/handler.go":      "package handler\n\nfunc Serve() {}\n",
		"handler/handler_test.go": "package handler\n\nfunc TestServe() {}\n",
		"docs/specs/nav-v0.md":    "# Nav\n\n- `NAV-007`: the nav clause.\n",
		"docs/specs/quokka-v0.md": "# Quokka\n\nThe quokka is described here.\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

// TestTaskContextEqualByteLexicalControlLosesOnlySpecMentionedRows is
// TCP-V0-012's falsifier (b): substituting the requirement id for an
// equal-byte token that defines nothing must lose the spec-mentioned rows and
// nothing else. The decoy spec shares the task's title vocabulary and is never
// admitted as spec-mentioned; it may still arrive lexically, in both packets.
func TestTaskContextEqualByteLexicalControlLosesOnlySpecMentionedRows(t *testing.T) {
	index := decoyFixture(t)
	const defining = "docs/specs/nav-v0.md"
	named, control := "Fix the quokka handler under NAV-007 today", "Fix the quokka handler under ZQX-449 today"
	if len(named) != len(control) {
		t.Fatalf("the control must be equal-byte: %d vs %d", len(named), len(control))
	}
	packets := map[string]map[string]any{}
	for label, task := range map[string]string{"named": named, "control": control} {
		packet, err := TaskContext(context.Background(), index, task, "handler/handler.go", maxLimit)
		if err != nil {
			t.Fatal(err)
		}
		packets[label] = packet
	}
	t.Run("TCP-V0-012 removing the id loses exactly the spec-mentioned rows", func(t *testing.T) {
		withID := contextPairs(t, packets["named"], defining)
		without := contextPairs(t, packets["control"], defining)
		if !slices.Equal(withID, without) {
			t.Fatalf("projected results differ:\n named   = %v\n control = %v", withID, without)
		}
		if !slices.Contains(contextPairs(t, packets["named"]), "spec-mentioned "+defining) {
			t.Fatalf("the named packet lost its spec row: %v", contextPairs(t, packets["named"]))
		}
		for _, row := range mapsFromAny(packets["control"]["results"]) {
			if row["kind"] == "spec-mentioned" {
				t.Fatalf("the control admitted a spec row: %v", row)
			}
		}
	})
	t.Run("TCP-V0-009 the title-overlap decoy is never spec-mentioned", func(t *testing.T) {
		for label, packet := range packets {
			for _, row := range mapsFromAny(packet["results"]) {
				if row["id"] == "docs/specs/quokka-v0.md" && row["kind"] == "spec-mentioned" {
					t.Fatalf("%s admitted the title-overlap decoy as a reserved spec row", label)
				}
			}
		}
	})
	t.Run("TCP-V0-011 coverage counts, selectors and withheld are exact", func(t *testing.T) {
		// Walked from the fixture, not from what the packets report. Six files
		// are tracked; the subject is never a result, so five paths can appear
		// and both packets carry all five -- the id only moves the defining
		// spec between `spec-mentioned` and `cochange`.
		wantResults := map[string][]string{
			"named": {
				"governing AGENTS.md", "spec-mentioned " + defining, "pair handler/handler_test.go",
				"cochange docs/specs/quokka-v0.md", "cochange go.mod",
			},
			// Under TCP-V0-014 "the" is a stop word, so the defining spec ("the
			// nav clause") has no lexical corroboration and the quokka spec,
			// which the task names, sorts above it among the co-change rows.
			"control": {
				"governing AGENTS.md", "pair handler/handler_test.go",
				"cochange docs/specs/quokka-v0.md", "cochange " + defining, "cochange go.mod",
			},
		}
		// `candidates` counts distinct admitted paths: every packet carries
		// all five and the limit omits nothing.
		const wantCandidates, wantIncluded, wantOmitted = 5, 5, 0
		// Every generator's output fits, so the only withholdings are the
		// co-change rows the reservations took over in place: AGENTS.md in both
		// packets, and the defining spec in the named one.
		// The `test` relation belongs to the retrieval shape (TCP-V0-015); with a
		// subject it is not-applicable and its withheld is null (-1 here).
		wantWithheld := map[string]map[string]int{
			"named":   {"cochange": 2, "test": -1},
			"control": {"cochange": 1, "test": -1},
		}
		wantCritical := map[string][]string{
			"named":   {"governing AGENTS.md", "spec-mentioned " + defining},
			"control": {"governing AGENTS.md"},
		}
		for _, label := range []string{"named", "control"} {
			coverage := contextCoverage(t, packets[label])
			if got := contextPairs(t, packets[label]); !slices.Equal(got, wantResults[label]) {
				t.Fatalf("%s results = %v, want %v", label, got, wantResults[label])
			}
			if got := contextIntValue(coverage["candidates"]); got != wantCandidates {
				t.Fatalf("%s candidates = %d, want %d", label, got, wantCandidates)
			}
			if got := contextIntValue(coverage["included_results"]); got != wantIncluded {
				t.Fatalf("%s included_results = %d, want %d", label, got, wantIncluded)
			}
			if got := contextIntValue(coverage["omitted_results"]); got != wantOmitted {
				t.Fatalf("%s omitted_results = %d, want %d", label, got, wantOmitted)
			}
			if coverage["budget_shortage"] != "none" {
				t.Fatalf("%s budget_shortage = %v, want none at a limit admitting every row", label, coverage["budget_shortage"])
			}
			if got := contextSelectors(t, coverage, "critical"); !slices.Equal(got, wantCritical[label]) {
				t.Fatalf("%s critical = %v, want %v", label, got, wantCritical[label])
			}
			if got := contextSelectors(t, coverage, "critical_missing"); len(got) != 0 {
				t.Fatalf("%s critical_missing = %v, want none at this limit", label, got)
			}
			_, withheld := contextUnexamined(t, coverage)
			for _, relation := range contextRelationOrder {
				if got := contextIntValue(withheld[relation]); got != wantWithheld[label][relation] {
					t.Fatalf("%s withheld[%s] = %d, want %d", label, relation, got, wantWithheld[label][relation])
				}
			}
		}
	})
}

// TestTaskContextReportsCriticalMissingAndSlotShortage is TCP-V0-012's
// falsifier (c): a limit below the reserved set names the exact selectors that
// did not fit, in the fixed order, and says the budget bound the packet.
func TestTaskContextReportsCriticalMissingAndSlotShortage(t *testing.T) {
	index := decoyFixture(t)
	const defining = "docs/specs/nav-v0.md"
	t.Run("TCP-V0-011 a limit of one carries the governing row alone", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, "Fix the quokka handler under NAV-007 today", "handler/handler.go", 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := contextPairs(t, packet); !slices.Equal(got, []string{"governing AGENTS.md"}) {
			t.Fatalf("results = %v", got)
		}
		coverage := contextCoverage(t, packet)
		if got := contextSelectors(t, coverage, "critical"); !slices.Equal(got, []string{"governing AGENTS.md"}) {
			t.Fatalf("critical = %v", got)
		}
		if got := contextSelectors(t, coverage, "critical_missing"); !slices.Equal(got, []string{"spec-mentioned " + defining}) {
			t.Fatalf("critical_missing = %v", got)
		}
		if coverage["budget_shortage"] != "slots" {
			t.Fatalf("budget_shortage = %v", coverage["budget_shortage"])
		}
	})
	t.Run("TCP-V0-011 a limit fitting the reserved set leaves nothing missing", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, "Fix the quokka handler under NAV-007 today", "handler/handler.go", 2)
		if err != nil {
			t.Fatal(err)
		}
		coverage := contextCoverage(t, packet)
		want := []string{"governing AGENTS.md", "spec-mentioned " + defining}
		if got := contextSelectors(t, coverage, "critical"); !slices.Equal(got, want) {
			t.Fatalf("critical = %v, want %v", got, want)
		}
		if got := contextSelectors(t, coverage, "critical_missing"); len(got) != 0 {
			t.Fatalf("critical_missing = %v", got)
		}
	})
}

// TestTaskContextProseIdentifierControlAdmitsNoDefinitions is TCP-V0-012's
// falsifier (d): an unbackticked lowercase prose word is not an identifier the
// `definition` slot may look up. The three prose definers are exactly what the
// unfiltered generator would carry, so the filter loses three definition rows
// and gains none. The shared identifier set is untouched, so the other slots
// read the same words.
func TestTaskContextProseIdentifierControlAdmitsNoDefinitions(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":           "module example.test/prose\n\ngo 1.27.0\n",
		"prose/render.go":  "package prose\n\nfunc render() {}\n",
		"prose/merge.go":   "package prose\n\nfunc merge() {}\n",
		"prose/collect.go": "package prose\n\nfunc collect() {}\n",
		"code/handler.go":  "package code\n\nfunc parseToken() {}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	prose := []string{"prose/collect.go", "prose/merge.go", "prose/render.go"}
	task := "render merge collect the report"
	t.Run("TCP-V0-010 the three prose definers are what the filter removes", func(t *testing.T) {
		// The same generator over the same task, with every identifier forced
		// eligible on the backticked ground: three definers, inside the slot
		// cap, so all three would be carried were the filter gone.
		compiler := newTaskContextCompiler(index, task, "")
		for position := range compiler.identifiers {
			compiler.identifiers[position].weight = 3
		}
		found := make([]string, 0)
		for _, row := range compiler.symbolRows() {
			found = append(found, row.path)
		}
		if !slices.Equal(found, prose) || len(found) > contextSymbolCap {
			t.Fatalf("unfiltered definers = %v (cap %d), want %v", found, contextSymbolCap, prose)
		}
	})
	t.Run("TCP-V0-010 prose words admit no definition row and add none", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, task, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		found := contextRowsByKind(t, packet)
		if len(found["definition"]) != 0 {
			t.Fatalf("definition rows = %v, want none: three lost, none gained", found["definition"])
		}
		// The freed slots may refill lexically: the prose files are still listed.
		for _, path := range prose {
			if !slices.Contains(found["lexical"], path) {
				t.Fatalf("%s left the packet entirely: %v", path, found)
			}
		}
	})
	t.Run("TCP-V0-010 an eligible identifier still reaches its definer", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, "trace the parseToken result", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		found := contextRowsByKind(t, packet)
		if !slices.Equal(found["definition"], []string{"code/handler.go"}) {
			t.Fatalf("definition rows = %v, want the camelCase identifier's definer", found["definition"])
		}
	})
	t.Run("TCP-V0-010 the shared identifier set is not narrowed", func(t *testing.T) {
		names := make([]string, 0)
		for _, identifier := range taskIdentifiers(task) {
			names = append(names, identifier.name)
		}
		for _, word := range []string{"render", "merge", "collect"} {
			if !slices.Contains(names, word) {
				t.Fatalf("taskIdentifiers lost %q: %v", word, names)
			}
			if definitionEligible(taskIdentifier{name: word, weight: 1}) {
				t.Fatalf("%q must not be definition-eligible", word)
			}
		}
		for _, code := range []string{"parseToken", "snake_case", "utf8"} {
			if !definitionEligible(taskIdentifier{name: code, weight: 1}) {
				t.Fatalf("%q must be definition-eligible", code)
			}
		}
		if !definitionEligible(taskIdentifier{name: "render", weight: 3}) {
			t.Fatal("a backticked prose word stays eligible")
		}
	})
}

// TestTaskContextKeepsTheSubjectOutUnderReservedRelations is TCP-V0-012's
// falsifier (f): the subject is never a result, including where it would
// itself be the governing instruction file or the spec defining the id.
func TestTaskContextKeepsTheSubjectOutUnderReservedRelations(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                  "module example.test/self\n\ngo 1.27.0\n",
		"AGENTS.md":               "The widget contract.\n",
		"docs/specs/widget-v0.md": "# Widget\n\n- `WID-001`: the widget clause.\n",
		"widget/widget.go":        "package widget\n\nfunc Widget() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, subject, absentRelation string
	}{
		{"TCP-V0-005 the subject is not its own governing row", "AGENTS.md", "governing"},
		{"TCP-V0-005 the subject is not its own spec-mentioned row", "docs/specs/widget-v0.md", "spec-mentioned"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			packet, err := TaskContext(context.Background(), index, "WID-001 widget rules need a look", item.subject, 20)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range mapsFromAny(packet["results"]) {
				if row["id"] == item.subject {
					t.Fatalf("the subject was admitted as %v", row["kind"])
				}
			}
			for _, selector := range contextSelectors(t, contextCoverage(t, packet), "critical") {
				if strings.HasSuffix(selector, " "+item.subject) {
					t.Fatalf("the subject was reserved: %v", selector)
				}
			}
			if item.absentRelation == "governing" {
				// AGENTS.md is the only instruction file, so skipping it as the
				// subject leaves no candidate and no substitute.
				if contextCoverage(t, packet)["governance"] != "spec-mentioned" {
					t.Fatalf("governance = %v", contextCoverage(t, packet)["governance"])
				}
			}
		})
	}
}

// TestTaskContextReportsUnexaminedScopePerRelation pins TCP-V0-011's
// `unexamined` shape: every relation in the fixed order, once, with the states
// the retrieval shape and an inapplicable pass report.
func TestTaskContextReportsUnexaminedScopePerRelation(t *testing.T) {
	index := specMentionFixture(t)
	t.Run("TCP-V0-011 the retrieval shape names the subject-dependent relations", func(t *testing.T) {
		packet, err := TaskContext(context.Background(), index, "where does the alpha live", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		coverage := contextCoverage(t, packet)
		report := mapsFromAny(coverage["unexamined"])
		relations := make([]string, 0, len(report))
		for _, item := range report {
			relations = append(relations, item["relation"].(string))
		}
		if !slices.Equal(relations, contextRelationOrder) {
			t.Fatalf("unexamined order = %v, want %v", relations, contextRelationOrder)
		}
		states, withheld := contextUnexamined(t, coverage)
		for _, relation := range []string{"pair", "reverse-import", "reference", "cochange", "sibling"} {
			if states[relation] != "subject-absent" || withheld[relation] != nil {
				t.Fatalf("%s = %v / %v, want subject-absent and an uncounted withheld", relation, states[relation], withheld[relation])
			}
		}
		// No id token and no named spec path: the resolver never ran.
		if states["spec-mentioned"] != "not-applicable" || withheld["spec-mentioned"] != nil {
			t.Fatalf("spec-mentioned = %v / %v", states["spec-mentioned"], withheld["spec-mentioned"])
		}
		if states["lexical"] != "examined" || withheld["lexical"] == nil {
			t.Fatalf("lexical = %v / %v", states["lexical"], withheld["lexical"])
		}
	})
	t.Run("TCP-V0-003 a promoted row is withheld for its own relation", func(t *testing.T) {
		// alpha-v0.md is a documentation match for "alpha" and the definer of
		// ALP-001: the reservation takes it over and names the relation it
		// removed, which stays counted against `documentation`.
		packet, err := TaskContext(context.Background(), index, "apply ALP-001 to the alpha path", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		row := mapsFromAny(packet["results"])[0]
		if row["kind"] != "spec-mentioned" || row["id"] != "docs/specs/alpha-v0.md" {
			t.Fatalf("first row = %v", row)
		}
		if summary := row["summary"].(string); !strings.HasSuffix(summary, "; also documentation") {
			t.Fatalf("summary = %q, want the removed relation named", summary)
		}
		if len(mapsFromAny(row["evidence"])) != 1 {
			t.Fatalf("a promoted reservation keeps one evidence row: %v", row["evidence"])
		}
		_, withheld := contextUnexamined(t, contextCoverage(t, packet))
		if contextIntValue(withheld["documentation"]) < 1 {
			t.Fatalf("documentation withheld = %v, want the promoted row counted", withheld["documentation"])
		}
	})
}

// TestTaskContextDisclosesAnUnparsedSubjectsSymbols is TCP-V0-011's
// `subject-symbols-incomplete` state: a subject the Python grammar refused has
// no symbols, so `reference` must not report `examined` over them, while a
// parsed subject in the same tree still does.
func TestTaskContextDisclosesAnUnparsedSubjectsSymbols(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"pkg/broken.py": "def broken_thing(:\n    return 1\n",
		"pkg/fine.py":   "def fine_thing():\n    return 2\n",
		"pkg/user.py":   "from pkg.fine import fine_thing\n\nfine_thing()\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("TCP-V0-011 an unparsed subject's reference state is subject-symbols-incomplete", func(t *testing.T) {
		if !slices.ContainsFunc(index.Unparsed, func(item Unparsed) bool { return item.Path == "pkg/broken.py" }) {
			t.Fatalf("fixture drifted: pkg/broken.py is not unparsed: %v", index.Unparsed)
		}
		for subject, want := range map[string]string{"pkg/broken.py": "subject-symbols-incomplete", "pkg/fine.py": "examined"} {
			packet, err := TaskContext(context.Background(), index, "Change the thing signature", subject, 20)
			if err != nil {
				t.Fatal(err)
			}
			states, withheld := contextUnexamined(t, contextCoverage(t, packet))
			if states["reference"] != want || withheld["reference"] == nil {
				t.Fatalf("%s reference = %v / %v, want %s with a counted withheld", subject, states["reference"], withheld["reference"], want)
			}
		}
	})
}

// TestTaskContextAmendedPacketIsByteIdenticalAcrossRuns is TCP-V0-012's
// falsifier (e): two runs over the same pinned tree, history window, task,
// subject and limit render the same bytes, reservations and coverage members
// included (TCP-V0-007).
func TestTaskContextAmendedPacketIsByteIdenticalAcrossRuns(t *testing.T) {
	index := decoyFixture(t)
	t.Run("TCP-V0-012 identical inputs render identical bytes", func(t *testing.T) {
		rendered := make([][]byte, 2)
		for run := range rendered {
			packet, err := TaskContext(context.Background(), index, "Fix the quokka handler under NAV-007 today", "handler/handler.go", 20)
			if err != nil {
				t.Fatal(err)
			}
			if rendered[run], err = CanonicalJSON(packet); err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Equal(rendered[0], rendered[1]) {
			t.Fatalf("packets differ:\n%s\n%s", rendered[0], rendered[1])
		}
		if !bytes.Contains(rendered[0], []byte(`"governance":"reserved"`)) {
			t.Fatalf("the rendered packet lost its governance member: %s", rendered[0])
		}
	})
}

// TestTaskContextRepeatedSpecReportsNoSlotShortage is TCP-V0-009/011:
// deduplication precedes the cap, so naming a fourth id whose only definer is
// a spec the packet already carries reports no budget shortage.
func TestTaskContextRepeatedSpecReportsNoSlotShortage(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                 "module example.test/repeat\n\ngo 1.27.0\n",
		"docs/specs/alpha-v0.md": "# Alpha\n\n- `ALP-001`: the alpha clause.\n- `ZED-001`: the zed clause.\n",
		"docs/specs/beta-v0.md":  "# Beta\n\n- `BET-001`: the beta clause.\n",
		"docs/specs/gamma-v0.md": "# Gamma\n\n- `GAM-001`: the gamma clause.\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	// ZED-001 sorts last, so its definer arrives with all three slots taken.
	packet, err := TaskContext(context.Background(), index, "Apply ALP-001, BET-001, GAM-001 and ZED-001", "", maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	found := contextRowsByKind(t, packet)
	want := []string{"docs/specs/alpha-v0.md", "docs/specs/beta-v0.md", "docs/specs/gamma-v0.md"}
	if !slices.Equal(found["spec-mentioned"], want) {
		t.Fatalf("spec rows = %v, want %v", found["spec-mentioned"], want)
	}
	coverage := contextCoverage(t, packet)
	if coverage["budget_shortage"] != "none" {
		t.Fatalf("budget_shortage = %v, want none: the repeat is a duplicate, not a turned-away candidate", coverage["budget_shortage"])
	}
	if got := contextSelectors(t, coverage, "critical_missing"); len(got) != 0 {
		t.Fatalf("critical_missing = %v, want none", got)
	}
	if _, withheld := contextUnexamined(t, coverage); contextIntValue(withheld["spec-mentioned"]) != 0 {
		t.Fatalf("withheld[spec-mentioned] = %v, want 0", withheld["spec-mentioned"])
	}
}

func TestTaskContextPlacesDocumentationAfterFiveCodeRows(t *testing.T) {
	t.Run("TCP-V0-013", func(t *testing.T) {
		files := map[string]string{
			"go.mod":     "module example.test/documentation\n\ngo 1.27.0\n",
			"code/01.go": "package code\n\n// needle\n",
			"code/02.go": "package code\n\n// needle needle\n",
			"code/03.go": "package code\n\n// needle signal\n",
			"code/04.go": "package code\n\n// signal signal signal\n",
			"code/05.go": "package code\n\n// needle signal signal\n",
			"code/06.go": "package code\n\n// needle needle needle needle\n",
			"code/07.go": "package code\n\n// signal\n",
			"docs/a.md":  "needle\n",
			"docs/b.mdx": "needle needle\n",
			"docs/c.rst": "needle signal\n",
			"docs/d.txt": "signal signal signal\n",
		}
		index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
		if err != nil {
			t.Fatal(err)
		}
		packet, err := TaskContext(context.Background(), index, "needle signal", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		// TCP-V0-013 fixes the placement (five code rows, two documentation rows,
		// the rest of each class); the order inside a class is the BM25 score.
		wantKinds := []string{
			"lexical", "lexical", "lexical", "lexical", "lexical",
			"documentation", "documentation",
			"lexical", "lexical",
			"documentation", "documentation",
		}
		got := contextPairs(t, packet)
		gotKinds := make([]string, 0, len(got))
		for _, pair := range got {
			gotKinds = append(gotKinds, strings.SplitN(pair, " ", 2)[0])
		}
		if !slices.Equal(gotKinds, wantKinds) {
			t.Fatalf("TCP-V0-013 placement = %v, want %v", got, wantKinds)
		}
		if !strings.HasSuffix(got[5], "docs/c.rst") || !strings.HasSuffix(got[6], "docs/d.txt") {
			t.Fatalf("TCP-V0-013 documentation quota = %v, want docs/c.rst then docs/d.txt", got[5:7])
		}
		for _, row := range mapsFromAny(packet["results"]) {
			reason := mapsFromAny(row["evidence"])[0]["reason"].(string)
			if row["kind"] == "documentation" && !strings.Contains(reason, "documentation") {
				t.Fatalf("documentation reason = %q, want the class named", reason)
			}
		}
		states, withheld := contextUnexamined(t, contextCoverage(t, packet))
		if states["documentation"] != "examined" || contextIntValue(withheld["documentation"]) != 0 {
			t.Fatalf("documentation scope = %v / %v", states["documentation"], withheld["documentation"])
		}
	})
}

func TestTaskContextSearchesRstDocumentation(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":          "module example.test/rst\n\ngo 1.27.0\n",
		"docs/stream.rst": "streaming response generators\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := index.Sources["docs/stream.rst"]; !ok {
		t.Fatal("docs/stream.rst was not admitted as searchable text")
	}
	for _, symbol := range index.Symbols {
		if symbol.Path == "docs/stream.rst" {
			t.Fatalf(".rst admission unexpectedly extracted symbol %+v", symbol)
		}
	}
	packet, err := TaskContext(context.Background(), index, "streaming generators", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := contextPairs(t, packet); !slices.Contains(got, "documentation docs/stream.rst") {
		t.Fatalf("searchable .rst rows = %v, want documentation docs/stream.rst", got)
	}
}

// TestTaskContextAmbiguityCountsSpecsNotClauses is TCP-V0-009: a spec
// restating an id's defining clause defines it once, so `ambiguous-definition`
// counts specs and is absent when only one spec defines the id.
func TestTaskContextAmbiguityCountsSpecsNotClauses(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                "module example.test/ambig\n\ngo 1.27.0\n",
		"docs/specs/dup-v0.md":  "# Dup\n\n- `DUP-001`: stated once.\n- `SHR-001`: shared here.\n- `DUP-001`: restated verbatim.\n",
		"docs/specs/echo-v0.md": "# Echo\n\n- `SHR-001`: shared there.\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	packet, err := TaskContext(context.Background(), index, "Apply DUP-001 and SHR-001", "", maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	for _, row := range mapsFromAny(packet["results"]) {
		if row["kind"] != "spec-mentioned" {
			continue
		}
		reasons[row["id"].(string)] = mapsFromAny(row["evidence"])[0]["reason"].(string)
	}
	if got := reasons["docs/specs/dup-v0.md"]; got != "defines DUP-001" {
		t.Fatalf("repeated clause reason = %q, want %q", got, "defines DUP-001")
	}
	if got := reasons["docs/specs/echo-v0.md"]; got != "defines SHR-001; ambiguous-definition (2 specs)" {
		t.Fatalf("shared id reason = %q", got)
	}
}

// TestTaskContextSubjectSpecPathActivatesThePass is TCP-V0-009: the pass runs
// when a candidate id token or tracked spec path appears in the task or the
// subject, while path admission reads the task alone -- so a subject naming
// only a spec path is examined and admits nothing through this relation.
func TestTaskContextSubjectSpecPathActivatesThePass(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":               "module example.test/subjectspec\n\ngo 1.27.0\n",
		"docs/specs/nav-v0.md": "# Nav\n\n- `NAV-007`: the nav clause.\n",
		"notes/plan.md":        "The rollout follows docs/specs/nav-v0.md throughout.\n",
		"notes/other.md":       "The rollout follows nothing in particular.\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	for subject, want := range map[string]string{"notes/plan.md": "examined", "notes/other.md": "not-applicable"} {
		packet, err := TaskContext(context.Background(), index, "Advance the rollout", subject, maxLimit)
		if err != nil {
			t.Fatal(err)
		}
		coverage := contextCoverage(t, packet)
		if states, _ := contextUnexamined(t, coverage); states["spec-mentioned"] != want {
			t.Fatalf("subject %s spec-mentioned state = %q, want %q", subject, states["spec-mentioned"], want)
		}
		if found := contextRowsByKind(t, packet); len(found["spec-mentioned"]) != 0 {
			t.Fatalf("subject %s admitted %v: path admission reads the task alone", subject, found["spec-mentioned"])
		}
	}
}
