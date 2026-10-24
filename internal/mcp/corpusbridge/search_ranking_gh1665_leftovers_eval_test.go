package corpusbridge

import (
	"os"
	"path/filepath"
	"testing"
)

// searchRankingFixture1665Leftovers is the committed fixture for the GitHub
// #1665 re-test leftovers: questions naming a flow or a symbol among rarer
// question words, and identifier queries a short feature name ties.
const searchRankingFixture1665Leftovers = "testdata/search-ranking-gh1665-leftovers"

// DCS-V1-546 (GitHub #1665): every case of the leftovers fixture passes
// through the MCP tool path, deterministically.
func TestDCSV1546GH1665LeftoversRankingEvalFixture(t *testing.T) {
	root := searchRankingCorpus(t, searchRankingFixture1665Leftovers)
	cases, err := os.ReadFile(filepath.Join(searchRankingFixture1665Leftovers, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := evalRun(t, root, string(cases))
	for _, c := range report.Cases {
		t.Logf("%s: %s (rank %v, returned %v)", c.ID, c.Status, c.Rank, c.ReturnedRows)
		if c.Status != "pass" {
			t.Errorf("%s: %s (rank %v, unresolved %v, returned %v)", c.ID, c.Status, c.Rank, c.Unresolved, c.ReturnedRows)
		}
	}
	for tag, want := range map[string]int{"question": 3, "identifier": 5} {
		g := report.Summary.ByTag[tag]
		t.Logf("%s: %d of %d passed", tag, g.Passed, g.Cases)
		if g.Cases != want {
			t.Errorf("%s: %d cases, want %d", tag, g.Cases, want)
		}
	}
	again := evalRun(t, root, string(cases))
	for i, c := range report.Cases {
		if c.Status != again.Cases[i].Status || (c.Rank == nil) != (again.Cases[i].Rank == nil) || c.Rank != nil && *c.Rank != *again.Cases[i].Rank {
			t.Fatalf("%s not deterministic: %+v vs %+v", c.ID, c, again.Cases[i])
		}
	}
}
