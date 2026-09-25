package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// acceptanceFixture is the committed AFU-V1-039 application in testdata/flows/acceptance. repo/ holds
// the sources, Playwright specs and intents, whose REVIEW_ANCHOR placeholders are empty at commit A
// and name A at B; after-review/ is commit C, a change to the profile save span reviewed at A; and
// playwright-report.json is the run at C, ingested through `flows ingest`.
const acceptanceFixture = "testdata/flows/acceptance"

func acceptanceFiles(t *testing.T, dir, anchor string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = strings.ReplaceAll(string(raw), "REVIEW_ANCHOR", anchor)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// newAcceptanceFixture builds the fixture history and returns the root and the ingested evidence.
func newAcceptanceFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	shopGit(t, root, "init", "-q")
	shopWrite(t, root, acceptanceFiles(t, filepath.Join(acceptanceFixture, "repo"), ""))
	a := shopCommit(t, root, "A: application and intents")
	shopWrite(t, root, acceptanceFiles(t, filepath.Join(acceptanceFixture, "repo"), a))
	shopCommit(t, root, "B: review at A")
	shopWrite(t, root, acceptanceFiles(t, filepath.Join(acceptanceFixture, "after-review"), ""))
	head := shopCommit(t, root, "C: change the profile save span after review")
	report, err := filepath.Abs(filepath.Join(acceptanceFixture, "playwright-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The shared header's control names go-test subjects the report lacks, so it attaches to no record.
	header := shopIngestHeader(t, shopFixture{root: root, head: head})
	code, out, diagnostic := runFlowsCLI(root, append([]string{"ingest", "--format", "playwright-json", "--from", report}, header...)...)
	if code != 0 || strings.Count(out, "\n") != 3 {
		t.Fatalf("ingest %d %q %s", code, out, diagnostic)
	}
	evidence := filepath.Join(t.TempDir(), "runs.jsonl")
	if err = os.WriteFile(evidence, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	return root, evidence
}

func acceptanceCLI(t *testing.T, root string, v any, args ...string) string {
	t.Helper()
	code, out, diagnostic := runFlowsCLI(root, args...)
	if code != 0 {
		t.Fatalf("%v exited %d: %s", args, code, diagnostic)
	}
	if v != nil {
		if err := json.Unmarshal([]byte(out), v); err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
	}
	return out
}

// AFU-V1-039: over the committed fixture, the UI flow checkout and the API flow orders-api are
// verified; returns (declared, no test) is unmapped-flow and profile carries a stale-link, with
// verified run evidence, and both stay incomplete in map, gaps, navigate and docs.
func TestAFUV1039AcceptanceFixture(t *testing.T) {
	root, evidence := newAcceptanceFixture(t)
	want := map[string]struct {
		status string
		gaps   []string
		claim  string
	}{
		"checkout":   {"complete", []string{}, appflows.ClaimProven},
		"orders-api": {"complete", []string{}, appflows.ClaimProven},
		"profile":    {"incomplete", []string{"stale-link save-name"}, appflows.ClaimStale},
		"returns":    {"incomplete", []string{"unmapped-flow"}, appflows.ClaimUnproven},
	}

	var m appflows.MapReport
	acceptanceCLI(t, root, &m, "map", "--flows", "flows", "--evidence", evidence)
	if len(m.Flows) != len(want) {
		t.Fatalf("map has %d flows", len(m.Flows))
	}
	for _, f := range m.Flows {
		verified := !slices.ContainsFunc(f.Variations, func(v appflows.MapVariation) bool { return !v.Verified })
		if f.Status != want[f.FlowID].status || (f.Status == "complete" && !verified) {
			t.Errorf("map %s: status %s, every variation verified %v", f.FlowID, f.Status, verified)
		}
	}

	var g appflows.GapsReport
	acceptanceCLI(t, root, &g, "gaps", "--flows", "flows", "--evidence", evidence)
	if len(g.Flows) != len(want) {
		t.Fatalf("gaps has %d flows", len(g.Flows))
	}
	for _, f := range g.Flows {
		codes := []string{}
		for _, gap := range f.Gaps {
			codes = append(codes, strings.TrimSpace(gap.Code+" "+gap.Member))
		}
		if f.Status != want[f.FlowID].status || !slices.Equal(codes, want[f.FlowID].gaps) {
			t.Errorf("gaps %s: %s %v", f.FlowID, f.Status, codes)
		}
	}

	var n appflows.NavigationMap
	out := acceptanceCLI(t, root, &n, "navigate", "--flows", "flows", "--evidence", evidence)
	if len(n.Flows) != len(want) {
		t.Fatalf("navigate has %d flows", len(n.Flows))
	}
	for _, f := range n.Flows {
		if f.Status != want[f.FlowID].status {
			t.Errorf("navigate %s: %s", f.FlowID, f.Status)
		}
	}
	for _, step := range [][2]string{{"checkout", "pay"}, {"orders-api", "create-order"}} {
		if tr := navTransition(t, out, step[0], step[1]); tr.Verification != "verified" {
			t.Errorf("navigate %s/%s: %s", step[0], step[1], tr.Verification)
		}
	}
	for _, goal := range []string{"profile", "returns"} {
		var p appflows.NavigationPacket
		acceptanceCLI(t, root, &p, "navigate", "--flows", "flows", "--evidence", evidence, "--goal", goal)
		if len(p.Flows) != 1 || p.Flows[0].Status != "incomplete" {
			t.Errorf("navigate --goal %s: %+v", goal, p.Flows)
		}
	}

	acceptanceCLI(t, root, nil, "docs", "--flows", "flows", "--page", "flows.md", "--claims", "flows.claims.json", "--evidence", evidence)
	raw, err := os.ReadFile(filepath.Join(root, "flows.claims.json"))
	if err != nil {
		t.Fatal(err)
	}
	var claims appflows.DocClaims
	if err = json.Unmarshal(raw, &claims); err != nil || len(claims.Claims) != len(want) {
		t.Fatalf("claims %v %s", err, raw)
	}
	page, err := os.ReadFile(filepath.Join(root, "flows.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range claims.Claims {
		line := "  - `" + c.ID + "`: "
		if c.State != appflows.ClaimProven {
			line = "  - **" + c.State + ":** `" + c.ID + "`: "
		}
		if c.State != want[c.Flow].claim || !strings.Contains(string(page), line) {
			t.Errorf("docs %s: %s, page line %q present %v", c.ID, c.State, line, strings.Contains(string(page), line))
		}
	}
}
