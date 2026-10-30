package corpusbridge

import (
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
)

// searchRankingFixture1665 is the committed GitHub #1665 fixture: a small
// source tree whose Ruby service and model a records provider owns, supplied
// subjects and claims reproducing the issue's examples, and a docs corpus eval
// case file tagged by the issue row each case reproduces.
const searchRankingFixture1665 = "testdata/search-ranking-gh1665"

// DCS-V1-486 (GitHub #1665): every case of the fixture passes through the MCP
// tool path, deterministically, and the served docs_search and docs_claims
// rows carry their rank groups, so a tie the issue showed as an ID-ordered
// run is visible to the caller.
func TestDCSV1486GH1665RankingEvalFixture(t *testing.T) {
	root := searchRankingCorpus(t, searchRankingFixture1665)
	cases, err := os.ReadFile(filepath.Join(searchRankingFixture1665, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := evalRun(t, root, string(cases))
	for _, c := range report.Cases {
		if c.Status != "pass" {
			t.Errorf("%s: %s (rank %v, unresolved %v, returned %v)", c.ID, c.Status, c.Rank, c.Unresolved, c.ReturnedRows)
		}
	}
	for tag, want := range map[string]int{"row1": 1, "row2": 2, "row4": 1, "row5": 4} {
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
	registry, rerr := New(root, "corpus.json")
	if rerr != nil {
		t.Fatal(rerr)
	}
	for tool, arguments := range map[string]string{"corvint.docs_claims": `{"query":"admin","limit":10}`, "corvint.docs_search": `{"query":"admin","include":["claims"],"limit":10}`} {
		structured, _, failure, protocol := registry.Call(context.Background(), tool, []byte(arguments))
		if failure != nil || protocol != nil {
			t.Fatalf("%s: %v %v", tool, failure, protocol)
		}
		raw, err := json.Marshal(structured)
		if err != nil {
			t.Fatal(err)
		}
		var served struct {
			Results     []map[string]any `json:"results"`
			ResultRanks map[string]struct {
				Rank  int     `json:"rank"`
				Group int     `json:"group"`
				Tier  string  `json:"tier"`
				Score float64 `json:"score"`
			} `json:"result_ranks"`
		}
		if err := json.Unmarshal(raw, &served, json.RejectUnknownMembers(false)); err != nil {
			t.Fatal(err)
		}
		groups := map[int]int{}
		for _, r := range served.ResultRanks {
			groups[r.Group]++
		}
		t.Logf("%s admin: %d results, %d ranked, groups %v", tool, len(served.Results), len(served.ResultRanks), groups)
		if len(served.Results) < 3 || len(served.ResultRanks) != len(served.Results) || groups[1] < 2 {
			t.Fatalf("%s should serve a rank per result and a visible tie group: %d results, %d ranked, groups %v", tool, len(served.Results), len(served.ResultRanks), groups)
		}
	}
}
