package contextindex

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// specCitedFixture is a spec-routed implementation task's repository in
// miniature (V1-1269): the spec cites a command verb, a make target, a tool
// directory, a library file and a sibling document, none of which shares
// the task's words, while lexical decoys do.
func specCitedFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":    "module example.test/cited\n\ngo 1.27.0\n",
		"AGENTS.md": "Standing instructions.\n",
		"Makefile":  "gate:\n\tgo test ./...\n",
		"docs/specs/runner-v0.md": "# Runner\n\nExists: `tool plan --base`, `tools/select`, `make gate`.\n\n" +
			"- `RUN-V0-001`: the runner reuses `tool plan` and `make gate` through `tools/select/`.\n" +
			"- `RUN-V0-002`: `lib/kind.go:12` types results; see `docs/other.md` and `missing/path.go`.\n" +
			"- `RUN-V0-003`: `tool plan` again, and `lib/extra.go`.\n",
		"docs/other.md":         "# Other\n",
		"cmd/tool/plan.go":      "package main\n\nfunc plan() {}\n",
		"tools/select/main.go":  "package main\n\nfunc main() {}\n",
		"lib/kind.go":           "package lib\n\nfunc Kind() {}\n",
		"lib/extra.go":          "package lib\n\nfunc Extra() {}\n",
		"decoy/runner_tests.go": "package decoy\n\n// implement the runner tests once and type each result\nfunc Run() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

const specCitedTask = "Implement the runner per RUN-V0-001: run the selected tests once and type each result."

// TestTaskContextReservesTheCodeTheRoutedSpecCites_TCPV0990: a task routed to
// a spec carries the code that spec cites, most-cited first, as reserved
// `spec-cited` rows after the other reservations, and names every candidate
// the cap held back with its reason (V1-1269).
func TestTaskContextReservesTheCodeTheRoutedSpecCites_TCPV0990(t *testing.T) {
	t.Parallel()
	packet, err := TaskContext(context.Background(), specCitedFixture(t), specCitedTask, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	got := contextPairs(t, packet)
	want := []string{
		"governing AGENTS.md", "spec-mentioned docs/specs/runner-v0.md",
		"spec-cited cmd/tool/plan.go", "spec-cited tools/select/main.go", "spec-cited Makefile",
	}
	if len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
		t.Fatalf("results = %v, want prefix %v", got, want)
	}
	for _, row := range mapsFromAny(packet["results"])[2:5] {
		evidence := mapsFromAny(row["evidence"])[0]
		if evidence["authority"] != "document-reference" || evidence["trust"] != TrustProjectAuthority || row["score"] != 825 {
			t.Fatalf("spec-cited row = %v", row)
		}
		if !strings.HasPrefix(evidence["reason"].(string), "docs/specs/runner-v0.md cites `") {
			t.Fatalf("spec-cited reason = %v", evidence["reason"])
		}
	}
	if reason := mapsFromAny(packet["results"])[2]["evidence"].([]any)[0].(map[string]any)["reason"]; reason != "docs/specs/runner-v0.md cites `tool plan --base` (3 citations)" {
		t.Fatalf("most-cited reason = %v", reason)
	}
	coverage := contextCoverage(t, packet)
	omitted := coverage["omitted_spec_cited"].(map[string]any)
	samples := mapsFromAny(omitted["samples"])
	paths := make([]string, 0, len(samples))
	for _, sample := range samples {
		paths = append(paths, sample["path"].(string))
	}
	// Equal counts keep first-citation order; a cited document, an untracked path and the
	// spec itself are never candidates.
	if contextIntValue(omitted["count"]) != 2 || !slices.Equal(paths, []string{"lib/kind.go", "lib/extra.go"}) {
		t.Fatalf("omitted_spec_cited = %v", omitted)
	}
	if samples[0]["reason"] != "docs/specs/runner-v0.md cites `lib/kind.go:12` (1 citations); omitted: spec-cited cap 3" {
		t.Fatalf("omitted reason = %v", samples[0]["reason"])
	}
	critical := contextSelectors(t, coverage, "critical")
	for _, selector := range []string{"spec-cited cmd/tool/plan.go", "spec-cited Makefile", "spec-cited tools/select/main.go"} {
		if !slices.Contains(critical, selector) {
			t.Fatalf("critical = %v, want %s", critical, selector)
		}
	}
	report := mapsFromAny(coverage["unexamined"])
	relations := make([]string, 0, len(report))
	for _, item := range report {
		relations = append(relations, item["relation"].(string))
	}
	if !slices.Equal(relations, contextRankOrder) {
		t.Fatalf("unexamined order = %v, want %v", relations, contextRankOrder)
	}
	states, withheld := contextUnexamined(t, coverage)
	if states[specCitedRelation] != "examined" || contextIntValue(withheld[specCitedRelation]) != 2 {
		t.Fatalf("spec-cited state = %s withheld = %v", states[specCitedRelation], withheld[specCitedRelation])
	}
	if coverage["budget_shortage"] != "slots" {
		t.Fatalf("budget_shortage = %v, want slots", coverage["budget_shortage"])
	}
}

// TestTaskContextSpecCitedRowsTheLimitCutsAreCriticalMissing_TCPV0991: the
// reserved spec-cited rows cost positions of `--limit` like every other
// reservation, and the ones the limit cuts are named in `critical_missing`.
func TestTaskContextSpecCitedRowsTheLimitCutsAreCriticalMissing_TCPV0991(t *testing.T) {
	t.Parallel()
	packet, err := TaskContext(context.Background(), specCitedFixture(t), specCitedTask, "", 3)
	if err != nil {
		t.Fatal(err)
	}
	got := contextPairs(t, packet)
	if !slices.Equal(got, []string{"governing AGENTS.md", "spec-mentioned docs/specs/runner-v0.md", "spec-cited cmd/tool/plan.go"}) {
		t.Fatalf("results = %v", got)
	}
	missing := contextSelectors(t, contextCoverage(t, packet), "critical_missing")
	if !slices.Equal(missing, []string{"spec-cited Makefile", "spec-cited tools/select/main.go"}) {
		t.Fatalf("critical_missing = %v", missing)
	}
}

// TestTaskContextWithoutASpecCitationKeepsItsReceipt_TCPV0993: a task that
// routes to no spec, or to a spec citing no tracked code, runs no spec-cited
// pass: its receipt lists the unchanged relation order and carries no
// `omitted_spec_cited` member, so its bytes are those before the relation.
func TestTaskContextWithoutASpecCitationKeepsItsReceipt_TCPV0993(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		index *Index
		task  string
	}{
		{"no spec route", specCitedFixture(t), "run the selected tests once and type each result"},
		{"spec citing nothing", specMentionFixture(t), "change ALP-001"},
	} {
		packet, err := TaskContext(context.Background(), test.index, test.task, "", 20)
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
			t.Fatalf("%s: unexamined order = %v, want %v", test.name, relations, contextRelationOrder)
		}
		if _, present := coverage["omitted_spec_cited"]; present {
			t.Fatalf("%s: omitted_spec_cited present: %v", test.name, coverage["omitted_spec_cited"])
		}
		for _, pair := range contextPairs(t, packet) {
			if strings.HasPrefix(pair, specCitedRelation+" ") {
				t.Fatalf("%s: spec-cited row %s", test.name, pair)
			}
		}
	}
}

// TestTaskContextOmittedSpecCitedNamesOnlyUncarriedPaths_TCPV0992: a
// candidate the cap held back that another relation carries is a result, not
// an omission, so `omitted_spec_cited` and the relation's `withheld` count
// leave it out.
func TestTaskContextOmittedSpecCitedNamesOnlyUncarriedPaths_TCPV0992(t *testing.T) {
	t.Parallel()
	packet, err := TaskContext(context.Background(), specCitedFixture(t), specCitedTask+" Also see `lib/extra.go`.", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	pairs := contextPairs(t, packet)
	if !slices.Contains(pairs, "mentioned lib/extra.go") || !slices.Contains(pairs, "lexical lib/kind.go") {
		t.Fatalf("results = %v, want lib/extra.go and lib/kind.go carried by other relations", pairs)
	}
	coverage := contextCoverage(t, packet)
	if _, present := coverage["omitted_spec_cited"]; present {
		t.Fatalf("omitted_spec_cited = %v, want absent: every held-back candidate is a result", coverage["omitted_spec_cited"])
	}
	if states, withheld := contextUnexamined(t, coverage); states[specCitedRelation] != "examined" || contextIntValue(withheld[specCitedRelation]) != 0 {
		t.Fatalf("spec-cited state = %s withheld = %v, want examined and 0", states[specCitedRelation], withheld[specCitedRelation])
	}
}
