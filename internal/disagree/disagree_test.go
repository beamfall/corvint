package disagree

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// RDS-V0-002: structuralChannel links the file declaring a task's identifier,
// one import hop in each direction from it, while excluding the subject path
// and any path the index does not itself carry as a source.
func TestStructuralChannelLinksDeclaringAndImportingFiles(t *testing.T) {
	index := &contextindex.Index{
		Module:  "example.test/mod",
		Symbols: []contextindex.Symbol{{Name: "Split", Path: "cache/demux.go"}},
		Sources: map[string]contextindex.Source{
			"cache/demux.go": {Path: "cache/demux.go"},
			"lib/util.go":    {Path: "lib/util.go"},
			"app/caller.go":  {Path: "app/caller.go"},
			// "ghost.go" is deliberately absent from Sources.
		},
		Imports: map[string]map[string]struct{}{
			"cache/demux.go": {"lib/util": {}},
			"app/caller.go":  {"example.test/mod/cache": {}},
			"ghost.go":       {"example.test/mod/cache": {}},
		},
	}

	rows := structuralChannel(index, "does Split handle empty keys", "", 10)
	byPath := map[string]string{}
	for _, row := range rows {
		byPath[row.path] = row.reason
	}
	if reason, ok := byPath["cache/demux.go"]; !ok || reason != "declares Split" {
		t.Fatalf("declaring file missing or mislabeled: %v", byPath)
	}
	if reason, ok := byPath["lib/util.go"]; !ok || reason != "imported by cache/demux.go" {
		t.Fatalf("forward import missing or mislabeled: %v", byPath)
	}
	if reason, ok := byPath["app/caller.go"]; !ok || reason != "imports example.test/mod/cache" {
		t.Fatalf("reverse import missing or mislabeled: %v", byPath)
	}
	if _, ok := byPath["ghost.go"]; ok {
		t.Fatalf("linked an importer the index does not carry as a source: %v", byPath)
	}

	excluded := structuralChannel(index, "does Split handle empty keys", "lib/util.go", 10)
	for _, row := range excluded {
		if row.path == "lib/util.go" {
			t.Fatalf("subject path was not excluded: %v", excluded)
		}
	}
}

// RDS-V0-002: a task whose text names no identifier the index declares votes
// nothing, rather than guessing at prose tokens.
func TestStructuralChannelAbstainsWithoutDeclaredIdentifiers(t *testing.T) {
	index := &contextindex.Index{
		Symbols: []contextindex.Symbol{{Name: "Split", Path: "cache/demux.go"}},
		Sources: map[string]contextindex.Source{"cache/demux.go": {Path: "cache/demux.go"}},
	}
	if rows := structuralChannel(index, "improve error handling generally", "", 10); rows != nil {
		t.Fatalf("voted without a declared identifier: %v", rows)
	}
}

// RDS-V0-002: a Python importer using a dotted specifier ("import pkg.helpers")
// links to the declaring file the same as a path-shaped Go import does.
// index.Imports records Python specifiers as dotted module names, never as
// paths, so importTargets must resolve them through
// contextindex.PythonImportCandidates or every dotted importer is dropped.
func TestStructuralChannelLinksDottedPythonImports(t *testing.T) {
	index := &contextindex.Index{
		Symbols: []contextindex.Symbol{{Name: "compute_total", Path: "pkg/helpers.py"}},
		Sources: map[string]contextindex.Source{
			"pkg/helpers.py": {Path: "pkg/helpers.py"},
			"pkg/report.py":  {Path: "pkg/report.py"},
			"pkg/cli.py":     {Path: "pkg/cli.py"},
		},
		Imports: map[string]map[string]struct{}{
			"pkg/report.py": {"pkg.helpers": {}},
			"pkg/cli.py":    {"pkg.helpers": {}},
		},
	}

	rows := structuralChannel(index, "fix compute_total rounding", "", 10)
	byPath := map[string]string{}
	for _, row := range rows {
		byPath[row.path] = row.reason
	}
	if reason, ok := byPath["pkg/report.py"]; !ok || reason != "imports pkg/helpers.py" {
		t.Fatalf("dotted importer pkg/report.py missing or mislabeled: %v", byPath)
	}
	if reason, ok := byPath["pkg/cli.py"]; !ok || reason != "imports pkg/helpers.py" {
		t.Fatalf("dotted importer pkg/cli.py missing or mislabeled: %v", byPath)
	}
}

// RDS-V0-003/RDS-V0-004/RDS-V0-006: compare's Jaccard threshold decides the
// categorical agreement, and the agreement decides the proposed state.
func TestCompareAgreementAndProposedState(t *testing.T) {
	tests := []struct {
		name                               string
		lexical, structural                []candidate
		wantAgreement, wantState, wantVote string
		wantJaccard                        float64
	}{
		{
			name:    "RDS-V0-006 no structural vote is uncertain",
			lexical: candidates("a.go", "b.go"), structural: nil,
			wantAgreement: "none", wantState: "uncertain", wantVote: "absent", wantJaccard: 0,
		},
		{
			name:    "RDS-V0-006 structural-only vote is uncertain",
			lexical: nil, structural: candidates("a.go"),
			wantAgreement: "none", wantState: "uncertain", wantVote: "present", wantJaccard: 0,
		},
		{
			name:    "RDS-V0-006 disjoint votes are unanswerable",
			lexical: candidates("a.go", "b.go"), structural: candidates("c.go", "d.go"),
			wantAgreement: "none", wantState: "unanswerable", wantVote: "present", wantJaccard: 0,
		},
		{
			name:    "RDS-V0-004 low overlap is uncertain",
			lexical: candidates("a.go", "b.go", "c.go"), structural: candidates("a.go", "x.go", "y.go"),
			wantAgreement: "low", wantState: "uncertain", wantVote: "present", wantJaccard: 0.2,
		},
		{
			name:    "RDS-V0-004 high overlap is answerable",
			lexical: candidates("a.go", "b.go"), structural: candidates("a.go", "b.go"),
			wantAgreement: "high", wantState: "answerable", wantVote: "present", wantJaccard: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := compare(tc.lexical, tc.structural, 10)
			if result["agreement"] != tc.wantAgreement || result["proposed_state"] != tc.wantState || result["structural_vote"] != tc.wantVote {
				t.Fatalf("compare() = %+v", result)
			}
			if result["jaccard_top_k"] != tc.wantJaccard {
				t.Fatalf("jaccard_top_k = %v, want %v", result["jaccard_top_k"], tc.wantJaccard)
			}
		})
	}
}

// RDS-V0-003: correlation is the Spearman coefficient over the shared paths
// alone; fewer than two shared paths admits no coefficient rather than a
// manufactured one.
func TestCorrelation(t *testing.T) {
	tests := []struct {
		name                string
		shared              []string
		lexical, structural map[string]int
		want                any
	}{
		{name: "RDS-V0-003 no overlap has no coefficient", want: nil},
		{
			name: "RDS-V0-003 one shared path has no coefficient", shared: []string{"a.go"},
			lexical: map[string]int{"a.go": 1}, structural: map[string]int{"a.go": 1}, want: nil,
		},
		{
			name: "RDS-V0-003 perfect positive correlation", shared: []string{"a.go", "b.go", "c.go"},
			lexical: ranks("a.go", "b.go", "c.go"), structural: ranks("a.go", "b.go", "c.go"), want: 1.0,
		},
		{
			name: "RDS-V0-003 positive non-extreme correlation", shared: []string{"a.go", "b.go", "c.go", "d.go"},
			lexical: ranks("a.go", "b.go", "c.go", "d.go"), structural: ranks("a.go", "d.go", "b.go", "c.go"), want: 0.4,
		},
		{
			name: "RDS-V0-003 zero correlation", shared: []string{"a.go", "b.go", "c.go", "d.go"},
			lexical: ranks("a.go", "b.go", "c.go", "d.go"), structural: ranks("c.go", "a.go", "d.go", "b.go"), want: 0.0,
		},
		{
			name: "RDS-V0-003 negative non-extreme correlation", shared: []string{"a.go", "b.go", "c.go", "d.go"},
			lexical: ranks("a.go", "b.go", "c.go", "d.go"), structural: ranks("c.go", "d.go", "a.go", "b.go"), want: -0.6,
		},
		{
			name: "RDS-V0-003 perfect negative correlation", shared: []string{"a.go", "b.go", "c.go"},
			lexical: ranks("a.go", "b.go", "c.go"), structural: ranks("c.go", "b.go", "a.go"), want: -1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := correlation(tc.shared, tc.lexical, tc.structural); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("correlation() = %v, want %v", got, tc.want)
			}
		})
	}
}

// RDS-V0-005: rankLinks orders by the number of distinct structural links,
// breaks ties by path, and truncates to limit.
func TestRankLinksOrdersByLinkCountThenPathAndTruncates(t *testing.T) {
	links := map[string]map[string]struct{}{
		"b.go": {"declares B": {}},
		"a.go": {"declares A": {}},
		"c.go": {"declares C": {}, "imports d.go": {}},
		"d.go": {"declares D": {}, "imported by c.go": {}},
	}
	tests := []struct {
		name  string
		limit int
		want  []candidate
	}{
		{
			name: "RDS-V0-005 link count then path tie-break with sorted reasons", limit: 10,
			want: []candidate{
				{path: "c.go", reason: "declares C; imports d.go"},
				{path: "d.go", reason: "declares D; imported by c.go"},
				{path: "a.go", reason: "declares A"},
				{path: "b.go", reason: "declares B"},
			},
		},
		{
			name: "RDS-V0-005 limit preserves deterministic winners", limit: 2,
			want: []candidate{
				{path: "c.go", reason: "declares C; imports d.go"},
				{path: "d.go", reason: "declares D; imported by c.go"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rankLinks(links, nil, tc.limit); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("rankLinks() = %v, want %v", got, tc.want)
			}
		})
	}
}

// RDS-V0-004: compare applies the literal 0.34 threshold. Signal's top-K cap
// makes 1/3 and 2/5 the nearest realizable ratios around it; a larger synthetic
// top proves equality belongs to the high side rather than only bounding it.
func TestCompareJaccardThresholdBoundary(t *testing.T) {
	tests := []struct {
		name                                string
		shared, lexicalOnly, structuralOnly int
		wantJaccard                         float64
		wantAgreement                       string
	}{
		{name: "RDS-V0-004 below threshold", shared: 16, lexicalOnly: 17, structuralOnly: 17, wantJaccard: 0.32, wantAgreement: "low"},
		{name: "RDS-V0-004 at threshold", shared: 17, lexicalOnly: 16, structuralOnly: 17, wantJaccard: 0.34, wantAgreement: "high"},
		{name: "RDS-V0-004 above threshold", shared: 18, lexicalOnly: 16, structuralOnly: 16, wantJaccard: 0.36, wantAgreement: "high"},
		{name: "RDS-V0-004 nearest capped ratio below", shared: 1, lexicalOnly: 1, structuralOnly: 1, wantJaccard: 0.3333, wantAgreement: "low"},
		{name: "RDS-V0-004 nearest capped ratio above", shared: 2, lexicalOnly: 2, structuralOnly: 1, wantJaccard: 0.4, wantAgreement: "high"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lexical, structural := overlappingCandidates(tc.shared, tc.lexicalOnly, tc.structuralOnly)
			signal := compare(lexical, structural, 50)
			if signal["jaccard_top_k"] != tc.wantJaccard || signal["agreement"] != tc.wantAgreement {
				t.Fatalf("compare() = %+v, want jaccard %v and agreement %q", signal, tc.wantJaccard, tc.wantAgreement)
			}
		})
	}
}

func candidates(paths ...string) []candidate {
	rows := make([]candidate, len(paths))
	for i, path := range paths {
		rows[i] = candidate{path: path, reason: "r"}
	}
	return rows
}

func ranks(paths ...string) map[string]int {
	result := make(map[string]int, len(paths))
	for i, path := range paths {
		result[path] = i + 1
	}
	return result
}

func overlappingCandidates(shared, lexicalOnly, structuralOnly int) ([]candidate, []candidate) {
	lexical := make([]candidate, 0, shared+lexicalOnly)
	structural := make([]candidate, 0, shared+structuralOnly)
	for i := 0; i < shared; i++ {
		row := candidate{path: fmt.Sprintf("shared-%02d.go", i)}
		lexical = append(lexical, row)
		structural = append(structural, row)
	}
	for i := 0; i < lexicalOnly; i++ {
		lexical = append(lexical, candidate{path: fmt.Sprintf("lexical-%02d.go", i)})
	}
	for i := 0; i < structuralOnly; i++ {
		structural = append(structural, candidate{path: fmt.Sprintf("structural-%02d.go", i)})
	}
	return lexical, structural
}

// RDS-V0-961/RDS-V0-963 (V1-3708): a Go import names a package once, so an
// import of a many-file package is one link to its representative source, not
// one per file, and the file declaring the named symbol is seeded at rank 1
// even when many importers tie on link count ahead of it by path.
func TestStructuralChannelSeedsDeclarerAheadOfPackageFanOut(t *testing.T) {
	index := &contextindex.Index{
		Module:  "example.test/mod",
		Symbols: []contextindex.Symbol{{Name: "EvalQuery", Path: "zz/eval/eval_query.go"}},
		Sources: map[string]contextindex.Source{
			"zz/eval/eval_query.go": {Path: "zz/eval/eval_query.go"},
			"lib/doc.go":            {Path: "lib/doc.go"},
			"lib/a.go":              {Path: "lib/a.go"},
			"lib/b.go":              {Path: "lib/b.go"},
			"lib/b_test.go":         {Path: "lib/b_test.go"},
		},
		Imports: map[string]map[string]struct{}{"zz/eval/eval_query.go": {"example.test/mod/lib": {}}},
	}
	for position := range 12 {
		importer := fmt.Sprintf("app/a%02d.go", position)
		index.Sources[importer] = contextindex.Source{Path: importer}
		index.Imports[importer] = map[string]struct{}{"example.test/mod/zz/eval": {}}
	}

	rows := structuralChannel(index, "where is EvalQuery defined", "", 20)
	if len(rows) == 0 || rows[0] != (candidate{path: "zz/eval/eval_query.go", reason: "declares EvalQuery"}) {
		t.Fatalf("declaring file is not seeded first: %v", rows)
	}
	forward := 0
	for _, row := range rows {
		if strings.HasPrefix(row.path, "lib/") {
			forward++
			if row.path != "lib/doc.go" || row.reason != "imported by zz/eval/eval_query.go" {
				t.Fatalf("package import did not resolve to its doc.go representative: %v", row)
			}
		}
	}
	if forward != 1 {
		t.Fatalf("package import linked %d files, want exactly one: %v", forward, rows)
	}
}

// RDS-V0-960: a file naming the declared symbol is a reference only where the
// name resolves structurally (same Go package, or an importer of a definer);
// the same word in an unrelated file, or an importer that never names it, is
// not a reference.
func TestStructuralChannelReferencesResolveStructurally(t *testing.T) {
	source := func(name, text string) contextindex.Source {
		return contextindex.Source{Path: name, Data: []byte(text)}
	}
	index := &contextindex.Index{
		Module:  "example.test/mod",
		Symbols: []contextindex.Symbol{{Name: "EvalQuery", Path: "ctx/eval_query.go"}},
		Sources: map[string]contextindex.Source{
			"ctx/eval_query.go":      source("ctx/eval_query.go", "func EvalQuery() {}"),
			"ctx/eval_query_test.go": source("ctx/eval_query_test.go", "EvalQuery(); EvalQuery()"),
			"ctx/other.go":           source("ctx/other.go", "EvalQueryish()"),
			"app/main.go":            source("app/main.go", "ctx.EvalQuery()"),
			"app/quiet.go":           source("app/quiet.go", "ctx.Other()"),
			"docs/notes.go":          source("docs/notes.go", "EvalQuery is described here"),
		},
		Imports: map[string]map[string]struct{}{
			"app/main.go":  {"example.test/mod/ctx": {}},
			"app/quiet.go": {"example.test/mod/ctx": {}},
		},
	}
	byPath := map[string]string{}
	for _, row := range structuralChannel(index, "EvalQuery", "", 10) {
		byPath[row.path] = row.reason
	}
	want := map[string]string{
		"ctx/eval_query.go":      "declares EvalQuery",
		"ctx/eval_query_test.go": "references EvalQuery",
		"app/main.go":            "imports example.test/mod/ctx; references EvalQuery",
		"app/quiet.go":           "imports example.test/mod/ctx",
	}
	if !reflect.DeepEqual(byPath, want) {
		t.Fatalf("structural channel = %v, want %v", byPath, want)
	}
}

// RDS-V0-962 (V1-3708): only identifier-shaped tokens vote. Prose words that
// happen to spell a declared name (`signal`, `where`) and all-capital acronyms
// stay silent unless quoted in backticks.
func TestIdentifierTokensDropProse(t *testing.T) {
	got := identifierTokens("where is the retriever signal for EvalQuery, CEM and `defined` or rank_links? Fetch it")
	want := map[string]struct{}{"EvalQuery": {}, "defined": {}, "rank_links": {}, "Fetch": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identifierTokens() = %v, want %v", got, want)
	}
	index := &contextindex.Index{Symbols: []contextindex.Symbol{
		{Name: "signal", Path: "a.go"}, {Name: "where", Path: "b.go"}, {Name: "CEM", Path: "c.go"},
	}}
	if rows := structuralChannel(index, "retriever disagreement answerability signal where CEM", "", 10); rows != nil {
		t.Fatalf("prose words voted structurally: %v", rows)
	}
}
