package main

import (
	"encoding/json"
	"io/fs"
	"maps"
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
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
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
	code, out, diagnostic := runFlowsCLI(root, append([]string{"ingest", "--format", "playwright-json", "--from", report}, acceptanceIngestHeader(t, root, head)...)...)
	if code != 0 || strings.Count(out, "\n") != 3 {
		t.Fatalf("ingest %d %q %s", code, out, diagnostic)
	}
	evidence := filepath.Join(t.TempDir(), "runs.jsonl")
	if err = os.WriteFile(evidence, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	return root, evidence
}

// acceptanceIngestHeader is the run header of the Playwright run at head. It declares no --control.
func acceptanceIngestHeader(t *testing.T, root, head string) []string {
	t.Helper()
	digest := strings.Repeat("a", 64)
	return []string{"--run-id", "run-acceptance", "--runner-version", "1.50.0", "--source-commit", head, "--source-tree", shopGit(t, root, "rev-parse", "HEAD^{tree}"),
		"--source-clean", "--build-artifact-digest", digest, "--environment-id", "ci", "--environment-digest", digest, "--fixture-id", "seed", "--fixture-digest", digest,
		"--cleanup", "done"}
}

// sortedIDs returns the sorted IDs a surface reports, one per item.
func sortedIDs[T any](items []T, id func(T) string) []string {
	ids := []string{}
	for _, item := range items {
		ids = append(ids, id(item))
	}
	slices.Sort(ids)
	return ids
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

	flows := slices.Sorted(maps.Keys(want))

	var m appflows.MapReport
	acceptanceCLI(t, root, &m, "map", "--flows", "flows", "--evidence", evidence)
	if ids := sortedIDs(m.Flows, func(f appflows.MapFlow) string { return f.FlowID }); !slices.Equal(ids, flows) {
		t.Fatalf("map flows %v", ids)
	}
	for _, f := range m.Flows {
		verified := len(f.Variations) > 0 && !slices.ContainsFunc(f.Variations, func(v appflows.MapVariation) bool { return !v.Verified })
		if f.Status != want[f.FlowID].status || (f.Status == "complete" && !verified) {
			t.Errorf("map %s: status %s, every variation verified %v", f.FlowID, f.Status, verified)
		}
		// profile's stale link does not reach its variation: verification is per evidence pair.
		if f.FlowID == "profile" && !verified {
			t.Errorf("map profile: variation not verified: %+v", f.Variations)
		}
	}

	var g appflows.GapsReport
	acceptanceCLI(t, root, &g, "gaps", "--flows", "flows", "--evidence", evidence)
	if ids := sortedIDs(g.Flows, func(f appflows.GapFlow) string { return f.FlowID }); !slices.Equal(ids, flows) {
		t.Fatalf("gaps flows %v", ids)
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
	if ids := sortedIDs(n.Flows, func(f appflows.NavFlow) string { return f.FlowID }); !slices.Equal(ids, flows) {
		t.Fatalf("navigate flows %v", ids)
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
	// Current behavior, pinned: step verification is evidence-only, so profile's save-name step is
	// verified although its source link is stale and the flow is incomplete. The stale link shows only in
	// the flow status, gaps and the docs claim. returns has no evidence, so its step is unverified.
	if tr := navTransition(t, out, "profile", "save-name"); tr.Verification != "verified" {
		t.Errorf("navigate profile/save-name: %s", tr.Verification)
	}
	if tr := navTransition(t, out, "returns", "request-return"); tr.Verification != "unverified" {
		t.Errorf("navigate returns/request-return: %s", tr.Verification)
	}
	for _, goal := range []string{"profile", "returns"} {
		p := decodePacket(t, acceptanceCLI(t, root, nil, "navigate", "--flows", "flows", "--evidence", evidence, "--goal", goal))
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
	if err = json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("claims %v %s", err, raw)
	}
	if ids := sortedIDs(claims.Claims, func(c appflows.DocClaim) string { return c.Flow }); !slices.Equal(ids, flows) {
		t.Fatalf("docs claim flows %v", ids)
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
