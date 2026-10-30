package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// ANL-V0-060..062: in a two-repository layout `corvint affected --appmap-impact` run in the test
// repository accepts a report built in the application repository only when a declared
// --repository checkout carries its base and head, records that binding, fails closed on an
// uncommitted or ignored application path, and otherwise refuses naming the supported layout.
func TestANLV0060AffectedBindsReportToDeclaredRepository(t *testing.T) {
	app := appmapCLIRepo(t)
	tests := affectedFixtureRepository(t)
	testBase := strings.TrimSpace(affectedGit(t, tests, "rev-parse", "HEAD"))
	dir := t.TempDir()
	code, mapJSON, diag := runFlowsCLI(app, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	mapFile := filepath.Join(dir, "map.json")
	if err := os.WriteFile(mapFile, []byte(mapJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	appBase := strings.TrimSpace(affectedGit(t, app, "rev-parse", "HEAD"))
	commit := func(rel, text string) string {
		if err := os.WriteFile(filepath.Join(app, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, app, "add", "-A")
		affectedGit(t, app, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", rel)
		return strings.TrimSpace(affectedGit(t, app, "rev-parse", "HEAD"))
	}
	impact := func(name string) string {
		code, out, diag := runFlowsCLI(app, "appmap", "impact", "--map", mapFile, "--base", appBase, "--full")
		if code != 0 {
			t.Fatal(diag)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	routes, err := os.ReadFile(filepath.Join(app, "app", "routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	appHead := commit("app/routes.js", string(routes)+"\n")
	targeted := impact("targeted.json")
	declared := []string{"--repository", "web=" + app, "--repository-base", "web=" + appBase}
	nav := func(args ...string) map[string]any {
		t.Helper()
		receipt, _, stderr, code := runAffectedCLI(t, tests, append([]string{"--base", testBase, "--appmap-impact", targeted}, args...)...)
		if code != 0 {
			t.Fatal(stderr)
		}
		navs := receipt["advice"].(map[string]any)["app_navigation"].([]any)
		if len(navs) != 1 {
			t.Fatalf("app_navigation %v", navs)
		}
		return navs[0].(map[string]any)
	}
	reasons := func(row map[string]any) []string {
		out := []string{}
		for _, r := range row["reasons"].([]any) {
			out = append(out, r.(string))
		}
		return out
	}

	// Undeclared: the refusal names the supported layout and the report's own range.
	_, _, stderr, code := runAffectedCLI(t, tests, "--base", testBase, "--appmap-impact", targeted)
	if code == 0 || !strings.Contains(stderr, "appmap-invalid-query") || !strings.Contains(stderr, "--repository ID=DIR --repository-base ID="+appBase) || !strings.Contains(stderr, appHead) {
		t.Fatalf("undeclared report: code %d %s", code, stderr)
	}

	// Declared: the report is checked against that checkout and the binding is recorded.
	row := nav(declared...)
	bindings := row["bindings"].([]any)
	if row["verdict"] != "targeted" || len(bindings) != 1 || len(reasons(row)) != 0 {
		t.Fatalf("bound %v", row)
	}
	b := bindings[0].(map[string]any)
	if b["repository"] != "web" || b["base"] != appBase || b["head"] != appHead || b["impact_digest"] != row["impact_digests"].([]any)[0] {
		t.Fatalf("binding %v", b)
	}
	// A relative --repository source resolves against --root, as under e2e-safe.
	rel, err := filepath.Rel(tests, app)
	if err != nil {
		t.Fatal(err)
	}
	if row = nav("--repository", "web="+filepath.ToSlash(rel), "--repository-base", "web="+appBase); row["bindings"].([]any)[0].(map[string]any)["repository"] != "web" {
		t.Fatalf("relative source %v", row)
	}

	// An uncommitted application path the report never saw fails closed.
	if err := os.WriteFile(filepath.Join(app, "notes.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if row = nav(declared...); row["verdict"] != "full-suite-required" || !slices.Contains(reasons(row), "repository-worktree-not-covered") {
		t.Fatalf("dirty application checkout %v", row)
	}
	// A clean --root sharing both commits never hides the declared checkout's uncommitted path.
	clone := filepath.Join(t.TempDir(), "clone")
	affectedGit(t, dir, "clone", "-q", app, clone)
	receipt, _, stderr, code := runAffectedCLI(t, clone, append([]string{"--base", appBase, "--appmap-impact", targeted}, declared...)...)
	if code != 0 {
		t.Fatal(stderr)
	}
	if row = receipt["advice"].(map[string]any)["app_navigation"].([]any)[0].(map[string]any); row["verdict"] != "full-suite-required" ||
		!slices.Contains(reasons(row), "repository-worktree-not-covered") || row["bindings"].([]any)[0].(map[string]any)["repository"] != "web" {
		t.Fatalf("clean root sharing the commits %v", row)
	}
	if err := os.Remove(filepath.Join(app, "notes.txt")); err != nil {
		t.Fatal(err)
	}

	// A checkout at another HEAD, or a mismatched --repository-base, refuses with the reason.
	commit("later.txt", "later\n")
	if _, _, stderr, code = runAffectedCLI(t, tests, append([]string{"--base", testBase, "--appmap-impact", targeted}, declared...)...); code == 0 || !strings.Contains(stderr, "repository web is at HEAD") {
		t.Fatalf("stale checkout: code %d %s", code, stderr)
	}
	if _, _, stderr, code = runAffectedCLI(t, tests, "--base", testBase, "--appmap-impact", targeted, "--repository", "web="+app, "--repository-base", "web="+testBase); code == 0 || !strings.Contains(stderr, "appmap-invalid-query") {
		t.Fatalf("mismatched base: code %d %s", code, stderr)
	}

	// An application path the map does not place never yields an empty selection (ANL-V0-062).
	commit("handler.go", "package server\n")
	backend := impact("backend.json")
	receipt, _, stderr, code = runAffectedCLI(t, tests, append([]string{"--base", testBase, "--appmap-impact", backend}, declared...)...)
	if code != 0 {
		t.Fatal(stderr)
	}
	row = receipt["advice"].(map[string]any)["app_navigation"].([]any)[0].(map[string]any)
	if row["verdict"] != "full-suite-required" || !(slices.Contains(reasons(row), "impact-ignored-paths") || slices.Contains(reasons(row), "impact-unknown-paths")) {
		t.Fatalf("unplaced application path %v", row)
	}

	// Argument contract: a base without its checkout, or a checkout without a base, refuses.
	for _, args := range [][]string{
		{"--repository-base", "web=" + appBase},
		{"--repository", "web=" + app},
	} {
		if _, _, _, code := runAffectedCLI(t, tests, append([]string{"--base", testBase, "--appmap-impact", backend}, args...)...); code == 0 {
			t.Fatalf("%v was accepted", args)
		}
	}
}

// ANL-V0-063: single-root `impact` has no explicit-revision mode, and --app-root or --test-root
// beside --root and a PATH is no two-root impact; each refuses with a typed code that names the
// supported two-repository paths instead of answering over one root.
func TestANLV0063ImpactRefusesUnsupportedLayout(t *testing.T) {
	root := affectedFixtureRepository(t)
	for _, args := range [][]string{{"--app-root", root}, {"--test-root=" + root}, {"--at", "HEAD"}, {"--revision", "HEAD"}} {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root, "impact"}, append(args, "a.go")...), strings.NewReader(""), &stdout, &stderr)
		if code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported-impact-layout") || !strings.Contains(stderr.String(), "--test-root ALIAS=ABSOLUTE_ROOT") {
			t.Fatalf("%v: code %d stdout %q stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}
