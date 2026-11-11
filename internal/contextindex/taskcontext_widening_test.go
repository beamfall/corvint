package contextindex

import (
	"context"
	"math"
	"regexp"
	"slices"
	"strconv"
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

// lexicalStrengths reads each lexical or documentation row's BM25 from its
// evidence reason, in packet order; a relation row reads as +Inf so the
// strength order is checked only across the lexical fill.
func lexicalStrengths(t *testing.T, packet map[string]any) []float64 {
	t.Helper()
	strengths := make([]float64, 0)
	for _, row := range mapsFromAny(packet["results"]) {
		if row["kind"] != "lexical" && row["kind"] != "documentation" {
			strengths = append(strengths, math.Inf(1))
			continue
		}
		reason := mapsFromAny(row["evidence"])[0]["reason"].(string)
		match := bm25Reason.FindStringSubmatch(reason)
		if match == nil {
			t.Fatalf("reason carries no bm25: %q", reason)
		}
		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		strengths = append(strengths, value)
	}
	return strengths
}

var bm25Reason = regexp.MustCompile(`bm25 ([0-9.]+)`)
