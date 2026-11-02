package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// AFP-V0-872 (V1-4052, V1-3831): --summary writes the affected-summary/1
// verdict: the plan's summary, its tier counts and the sorted tier-0 and
// tier-1 unit names, in a few hundred bytes, and --format text writes the
// same in a few lines. It restates the default plan's members exactly.
func TestAFPV0872AffectedSummaryVerdict(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	appendFile(t, filepath.Join(root, "core", "core.go"), "\n// edited\n")
	plan, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("plan exit %d: %s", code, stderr)
	}
	verdict, raw, stderr, code := runAffectedCLI(t, root, "--summary")
	if code != 0 {
		t.Fatalf("--summary exit %d: %s", code, stderr)
	}
	if len(raw) > 4096 || len(raw) >= len(affectedVerdictJSON(t, plan)) {
		t.Fatalf("--summary is %d bytes, plan %d", len(raw), len(affectedVerdictJSON(t, plan)))
	}
	if verdict["profile"] != "affected-summary/1" || !reflect.DeepEqual(verdict["summary"], plan["summary"]) || verdict["revision"] != plan["revision"] {
		t.Fatalf("verdict %v does not restate plan summary %v", verdict, plan["summary"])
	}
	planTiers := plan["tiers"].(map[string]any)
	tiers := verdict["tiers"].(map[string]any)
	if !reflect.DeepEqual(tiers["counts"], planTiers["counts"]) || tiers["summary"] != planTiers["summary"] {
		t.Fatalf("tiers %v, plan tiers %v", tiers, planTiers)
	}
	want := map[float64][]any{0: {}, 1: {}}
	for unit, tier := range planTiers["units"].(map[string]any) {
		if tier.(float64) < 2 {
			want[tier.(float64)] = append(want[tier.(float64)], unit)
		}
	}
	for tier, name := range map[float64]string{0: "tier0", 1: "tier1"} {
		sort.Slice(want[tier], func(i, j int) bool { return want[tier][i].(string) < want[tier][j].(string) })
		if !reflect.DeepEqual(tiers[name], want[tier]) {
			t.Fatalf("%s = %v, want %v", name, tiers[name], want[tier])
		}
	}
	if len(want[0]) == 0 {
		t.Fatal("fixture change has no tier-0 unit")
	}

	var text, textStderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--summary", "--format", "text"}, strings.NewReader(""), &text, &textStderr); code != 0 {
		t.Fatalf("text exit %d: %s", code, textStderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(text.String(), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "affected "+plan["summary"].(map[string]any)["line"].(string)+" revision=") ||
		lines[1] != "tiers "+planTiers["summary"].(string) || len(lines) != 2+len(want[0])+len(want[1]) || lines[2] != "tier0 "+want[0][0].(string) {
		t.Fatalf("text verdict:\n%s", text.String())
	}

	for _, options := range [][]string{{"--summary", "--full"}, {"--summary", "--summary"}, {"--summary=yes"}, {"--summary", "--by-path", "--path", "core/core.go"}} {
		if _, _, stderr, code := runAffectedCLI(t, root, options...); code != 2 || !strings.Contains(stderr, "--summary") {
			t.Fatalf("%v: exit %d %s", options, code, stderr)
		}
	}
}

func affectedVerdictJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
