package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// ANL-V0-031: `corvint affected --appmap-impact` routes the navigation verdict into advice as a
// learned, advisory member: full-suite-required stays explicit (also an advice.unknown line), a
// targeted verdict lists spec files without changing checks, uncommitted paths the report cannot
// cover fail closed, and a report for another range, a repeated map or tampered content refuses.
func TestANLV0031AffectedRoutesImpactVerdict(t *testing.T) {
	root := appmapCLIRepo(t)
	code, mapJSON, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "map.json")
	if err := os.WriteFile(mapFile, []byte(mapJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	commit := func(rel, text string) string {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, root, "add", "-A")
		affectedGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", rel)
		return strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	}
	impact := func(name string) string {
		code, out, diag := runFlowsCLI(root, "appmap", "impact", "--map", mapFile, "--base", base, "--full")
		if code != 0 {
			t.Fatal(diag)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	advice := func(receipt map[string]any) (map[string]any, []any) {
		a := receipt["advice"].(map[string]any)
		navs, _ := a["app_navigation"].([]any)
		if len(navs) != 1 {
			t.Fatalf("app_navigation %v", a)
		}
		return navs[0].(map[string]any), a["unknown"].([]any)
	}
	hasUnknown := func(unknown []any, want string) bool {
		for _, u := range unknown {
			if strings.HasPrefix(u.(string), want) {
				return true
			}
		}
		return false
	}

	// A router change: targeted, spec files listed, checks untouched, no full-suite line.
	routes, err := os.ReadFile(filepath.Join(root, "app", "routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	commit("app/routes.js", string(routes)+"\n")
	targeted := impact("targeted.json")
	plain, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatal(stderr)
	}
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--appmap-impact", targeted)
	if code != 0 {
		t.Fatal(stderr)
	}
	nav, unknown := advice(receipt)
	if nav["verdict"] != "targeted" || nav["reported_verdict"] != "targeted" || nav["authority"] != "learned" || len(nav["spec_files"].([]any)) == 0 || hasUnknown(unknown, affectedAppNavigationUnknown) {
		t.Fatalf("targeted %v %v", nav, unknown)
	}
	if !equalJSON(t, plain["advice"].(map[string]any)["checks"], receipt["advice"].(map[string]any)["checks"]) || !equalJSON(t, plain["plan"], receipt["plan"]) {
		t.Fatal("the impact verdict changed checks or the plan")
	}

	// Uncommitted paths the report never saw: fail closed to full-suite-required.
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code = runAffectedCLI(t, root, "--base", base, "--appmap-impact", targeted)
	if code != 0 {
		t.Fatal(stderr)
	}
	if nav, unknown = advice(receipt); nav["verdict"] != "full-suite-required" || nav["reported_verdict"] != "targeted" || !hasUnknown(unknown, affectedAppNavigationUnknown+": admin: worktree-not-covered") {
		t.Fatalf("worktree %v %v", nav, unknown)
	}
	if err := os.Remove(filepath.Join(root, "notes.txt")); err != nil {
		t.Fatal(err)
	}

	// An unmapped template: the report's full-suite-required verdict stays explicit.
	commit("club.html", "<p></p>\n")
	full := impact("full.json")
	receipt, _, stderr, code = runAffectedCLI(t, root, "--base", base, "--appmap-impact", full)
	if code != 0 {
		t.Fatal(stderr)
	}
	if nav, unknown = advice(receipt); nav["verdict"] != "full-suite-required" || !hasUnknown(unknown, affectedAppNavigationUnknown+": admin: impact-unknown-paths") {
		t.Fatalf("full %v %v", nav, unknown)
	}

	// Refusals: a report for an older HEAD, a repeated map, a tampered verdict, no --base.
	if _, _, _, code = runAffectedCLI(t, root, "--base", base, "--appmap-impact", targeted); code == 0 {
		t.Fatal("a report for another HEAD was accepted")
	}
	if _, _, _, code = runAffectedCLI(t, root, "--base", base, "--appmap-impact", full, "--appmap-impact", full); code == 0 {
		t.Fatal("a repeated map was accepted")
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(dir, "tampered.json")
	if err := os.WriteFile(tampered, []byte(strings.Replace(string(raw), `"full-suite-required"`, `"targeted"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, code = runAffectedCLI(t, root, "--base", base, "--appmap-impact", tampered); code == 0 {
		t.Fatal("a tampered report was accepted")
	}
	// Trailing delimiters, or an authority other than learned under a recomputed digest, refuse.
	var forged appmap.ImpactReport
	if err = json.Unmarshal(raw, &forged); err != nil {
		t.Fatal(err)
	}
	forged.Authority, forged.Digest = "owner", ""
	body, err := json.Marshal(forged)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	forged.Digest = hex.EncodeToString(sum[:])
	if body, err = json.Marshal(forged); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{"trailing.json": append(bytes.TrimRight(raw, "\n"), '}'), "owner.json": body} {
		bad := filepath.Join(dir, name)
		if err = os.WriteFile(bad, content, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, code = runAffectedCLI(t, root, "--base", base, "--appmap-impact", bad); code == 0 {
			t.Fatalf("%s was accepted", name)
		}
	}
	if _, _, _, code = runAffectedCLI(t, root, "--appmap-impact", full); code == 0 {
		t.Fatal("--appmap-impact without --base was accepted")
	}
}

func equalJSON(t *testing.T, a, b any) bool {
	t.Helper()
	x, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(x) == string(y)
}

// ANL-V0-034: reports from several maps of one app aggregate into one app_navigation row (sorted
// digests, union of screens and spec files, the most conservative verdict); the same map twice
// refuses.
func TestANLV0034AffectedAggregatesMapsPerApp(t *testing.T) {
	root := appmapCLIRepo(t)
	dir := t.TempDir()
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	build := func(name string, args ...string) string {
		code, out, diag := runFlowsCLI(root, append([]string{"appmap", "build", "--manifest", "appmap.json"}, args...)...)
		if code != 0 {
			t.Fatal(diag)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	commit := func(rel, text string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, root, "add", "-A")
		affectedGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", rel)
	}
	impact := func(mapFile, name string) string {
		code, out, diag := runFlowsCLI(root, "appmap", "impact", "--map", mapFile, "--base", base, "--full")
		if code != 0 {
			t.Fatal(diag)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	first := build("first.json")
	routes, err := os.ReadFile(filepath.Join(root, "app", "routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	commit("app/routes.js", string(routes)+"\n")
	second := build("second.json")
	a, b := impact(first, "a.json"), impact(second, "b.json")
	row := func(args ...string) map[string]any {
		receipt, _, stderr, code := runAffectedCLI(t, root, append([]string{"--base", base}, args...)...)
		if code != 0 {
			t.Fatal(stderr)
		}
		navs := receipt["advice"].(map[string]any)["app_navigation"].([]any)
		if len(navs) != 1 {
			t.Fatalf("one row per app: %v", navs)
		}
		return navs[0].(map[string]any)
	}
	nav := row("--appmap-impact", a, "--appmap-impact", b)
	if nav["verdict"] != "targeted" || len(nav["map_digests"].([]any)) != 2 || len(nav["impact_digests"].([]any)) != 2 || len(nav["spec_files"].([]any)) == 0 {
		t.Fatalf("aggregate %v", nav)
	}
	commit("club.html", "<p></p>\n")
	c, d := impact(first, "c.json"), impact(second, "d.json")
	if nav = row("--appmap-impact", c, "--appmap-impact", d); nav["verdict"] != "full-suite-required" || nav["reported_verdict"] != "full-suite-required" {
		t.Fatalf("full aggregate %v", nav)
	}
	if _, _, _, code := runAffectedCLI(t, root, "--base", base, "--appmap-impact", c, "--appmap-impact", c); code == 0 {
		t.Fatal("the same map twice was accepted")
	}
}
