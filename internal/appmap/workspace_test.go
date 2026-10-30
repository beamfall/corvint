package appmap

import (
	"context"
	"flag"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// parseRoots binds the shared root flags, parses args and resolves them against root.
func parseRoots(t *testing.T, root string, args ...string) (*Roots, error) {
	t.Helper()
	f := flag.NewFlagSet("roots", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	roots := BindRootFlags(f)
	if err := f.Parse(args); err != nil {
		return nil, err
	}
	return roots, roots.Resolve(root)
}

// WSD-V0-004: --workspace FILE, or the declaration discovered under --root, binds the same roots
// as --repo, --manifest-repo and --repo-revision, so the build is the flag build byte for byte;
// a default revision for an alias the build reads nothing from is unused, not refused; the
// workspace never merges with the flags, and --no-workspace skips discovery.
func TestWSDV0004BuildWorkspace(t *testing.T) {
	ctx := context.Background()
	app, appRev, e2e, e2eRev := twoRoots(t, true)
	flagged, err := parseRoots(t, app, "--repo", "e2e="+e2e, "--manifest-repo", "e2e", "--repo-revision", "e2e="+e2eRev)
	if err != nil {
		t.Fatal(err)
	}
	want, err := BuildRoots(ctx, app, "appmap.json", appRev, *flagged)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(filepath.Join(app, ".corvint"), e2e)
	if err != nil {
		t.Fatal(err)
	}
	decl := `{"profile":"corvint-workspace/0","manifest_repo":"e2e","repos":{"e2e":{"root":"` + filepath.ToSlash(rel) + `","revision":"` + e2eRev + `"},` +
		`"docs":{"root":"` + filepath.ToSlash(t.TempDir()) + `","revision":"main"}}}`
	named := filepath.Join(t.TempDir(), "workspace.json")
	writeFile(t, filepath.Dir(named), "workspace.json", strings.Replace(decl, filepath.ToSlash(rel), filepath.ToSlash(e2e), 1))
	writeFile(t, app, ".corvint/workspace.json", decl)
	for name, args := range map[string][]string{"named": {"--workspace", named}, "discovered": nil} {
		roots, err := parseRoots(t, app, args...)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if roots.Repos["e2e"] != e2e || roots.ManifestRepo != "e2e" || roots.Revisions["e2e"] != e2eRev || roots.Workspace == "" {
			t.Fatalf("%s: roots %+v", name, roots)
		}
		got, err := BuildRoots(ctx, app, "appmap.json", appRev, *roots)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.Digest != want.Digest || !reflect.DeepEqual(got.Roots, want.Roots) {
			t.Fatalf("%s: workspace build differs from the flag build\n%+v\n%+v", name, got.Roots, want.Roots)
		}
	}
	if roots, err := parseRoots(t, app, "--no-workspace"); err != nil || roots.Aliased() || roots.Workspace != "" {
		t.Fatalf("--no-workspace: %+v %v", roots, err)
	}
	if roots, err := parseRoots(t, app, "--repo", "e2e="+e2e); err != nil || roots.Workspace != "" || roots.ManifestRepo != "" {
		t.Fatalf("flags skip discovery: %+v %v", roots, err)
	}
	for _, args := range [][]string{
		{"--workspace", named, "--repo", "e2e=" + e2e},
		{"--workspace", named, "--manifest-repo", "e2e"},
		{"--workspace", named, "--no-workspace"},
	} {
		if _, err := parseRoots(t, app, args...); err == nil || !strings.Contains(err.Error(), "does not combine") {
			t.Fatalf("%v: %v", args, err)
		}
	}
	// A discovered declaration that is not valid refuses rather than being skipped.
	writeFile(t, app, ".corvint/workspace.json", `{"profile":"corvint-workspace/0","repos":{"e2e":{}}}`)
	if _, err := parseRoots(t, app); codeOf(err) != "workspace-invalid" {
		t.Fatalf("invalid discovered workspace: %v", err)
	}
	// An explicit --repo-revision for an alias the build does not read still refuses.
	if _, err := BuildRoots(ctx, app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e, "docs": t.TempDir()}, ManifestRepo: "e2e", Revisions: map[string]string{"docs": "main"}}); codeOf(err) != "appmap-root-unavailable" {
		t.Fatalf("flag revision for an unread alias: %v", err)
	}
}
