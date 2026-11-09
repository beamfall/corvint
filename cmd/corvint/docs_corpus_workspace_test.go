package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// corpusWorkspaceRepos commits an application checkout and a separate end-to-end test checkout
// and declares both in app/.corvint/workspace.json.
func corpusWorkspaceRepos(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	app, tests := filepath.Join(parent, "app"), filepath.Join(parent, "tests")
	cemWrite(t, app, "docs/login.md", "# Login\n\nUsers sign in with a password.\n")
	cemWrite(t, tests, "pages/login.md", "# Login page\n\nThe login page object opens the form.\n")
	cemWrite(t, tests, "pages/empty.md", "")
	for _, root := range []string{app, tests} {
		cemGit(t, root, "init", "-q")
		cemGit(t, root, "config", "user.name", "Corpus test")
		cemGit(t, root, "config", "user.email", "corpus@example.invalid")
		cemGit(t, root, "add", ".")
		cemGit(t, root, "commit", "-qm", "init")
	}
	cemWrite(t, app, ".corvint/workspace.json", `{"profile":"corvint-workspace/0","repos":{"app":{"root":".."},"e2e":{"root":"../../tests"}}}`)
	return app, tests
}

func corpusCLICode(t *testing.T, root string, args ...string) (string, string) {
	t.Helper()
	code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus"}, args...)...)
	if code == 0 {
		return out, ""
	}
	var envelope struct{ Code string }
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil || envelope.Code == "" {
		t.Fatalf("%v: exit %d, stderr %s", args, code, stderr)
	}
	return "", envelope.Code
}

// TestDCPV1604DocsCorpusWorkspaceManifest: manifest reads the discovered, named or --repo
// workspace alike, scopes each alias by ALIAS:PATH, refuses an unscoped alias and a workspace
// combined with --repo, and with --no-workspace stays the single-root manifest byte for byte
// (DCP-V1-603, DCP-V1-604).
func TestDCPV1604DocsCorpusWorkspaceManifest(t *testing.T) {
	t.Parallel()
	app, tests := corpusWorkspaceRepos(t)
	revision := cemGit(t, app, "rev-parse", "HEAD")
	base := []string{"manifest", "--revision", revision, "--scope", "docs", "--timestamp", "2026-10-28T00:00:00Z"}
	discovered, code := corpusCLICode(t, app, append(base, "--scope", "e2e:pages")...)
	if code != "" || !strings.Contains(discovered, `"alias":"e2e"`) || !strings.Contains(discovered, `"root":"../tests"`) {
		t.Fatalf("discovered workspace manifest: %s %s", code, discovered)
	}
	named, _ := corpusCLICode(t, app, append(base, "--scope", "e2e:pages", "--workspace", filepath.Join(app, ".corvint/workspace.json"))...)
	flagged, _ := corpusCLICode(t, app, append(base, "--scope", "e2e:pages", "--repo", "e2e="+tests)...)
	if named != discovered || flagged != discovered {
		t.Fatalf("named or --repo workspace differs from the discovered one:\n%s\n%s\n%s", discovered, named, flagged)
	}
	single, code := corpusCLICode(t, app, append(base, "--no-workspace")...)
	m, err := doccorpus.InventoryScopes(context.Background(), app, revision, []string{"docs"}, doccorpus.ManifestSchema, "2026-10-28T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := doccorpus.Encode(m)
	if code != "" || single != string(want) {
		t.Fatalf("--no-workspace manifest is not the single-root manifest: %s", code)
	}
	for name, c := range map[string]struct {
		args []string
		code string
	}{
		"unscoped alias":          {base, "corpus-refused"},
		"workspace with --repo":   {append(base, "--workspace", filepath.Join(app, ".corvint/workspace.json"), "--repo", "e2e="+tests), "invalid-arguments"},
		"workspace with --no":     {append(base, "--workspace", filepath.Join(app, ".corvint/workspace.json"), "--no-workspace"), "invalid-arguments"},
		"malformed --repo":        {append(base, "--repo", "E2E="+tests), "invalid-arguments"},
		"extract without alias":   {[]string{"extract", "--profile", "p.json", "--revision", revision, "--repository", "e2e", "--no-workspace"}, "invalid-arguments"},
		"trace --id and --path":   {[]string{"trace", "--artifact", "corpus.json", "--id", "x", "--path", "docs/login.md"}, "invalid-arguments"},
		"trace qualifier no path": {[]string{"trace", "--artifact", "corpus.json", "--id", "x", "--repository", "e2e"}, "invalid-arguments"},
	} {
		if _, code := corpusCLICode(t, app, c.args...); code != c.code {
			t.Fatalf("%s: code %q, want %q", name, code, c.code)
		}
	}
}

// TestDCPV1604DocsCorpusWorkspaceTraceAndGaps: a corpus built from the workspace manifest answers
// trace and gaps for a path in either checkout, takes a matching named workspace, and refuses
// one whose alias it does not declare (DCP-V1-602, DCP-V1-604).
func TestDCPV1604DocsCorpusWorkspaceTraceAndGaps(t *testing.T) {
	t.Parallel()
	app, tests := corpusWorkspaceRepos(t)
	revision := cemGit(t, app, "rev-parse", "HEAD")
	manifest, code := corpusCLICode(t, app, "manifest", "--revision", revision, "--scope", "docs", "--scope", "e2e:pages", "--timestamp", "2026-10-28T00:00:00Z")
	if code != "" {
		t.Fatal(code)
	}
	cemWrite(t, app, "corpus-input.json", manifest)
	artifact, code := corpusCLICode(t, app, "build", "--manifest", "corpus-input.json")
	if code != "" {
		t.Fatal(code)
	}
	cemWrite(t, app, "corpus.json", artifact)
	type receipt struct {
		State          string
		PathResolution struct{ ID string } `json:"path_resolution"`
		Results        []struct {
			ID, Subject, Kind string
		}
	}
	read := func(op string, args ...string) receipt {
		t.Helper()
		out, code := corpusCLICode(t, app, append([]string{op, "--artifact", "corpus.json"}, args...)...)
		if code != "" {
			t.Fatalf("%v: %s", args, code)
		}
		var r receipt
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := read("trace", "--path", "docs/login.md"); r.State != "ready" || r.PathResolution.ID != "native:file:docs/login.md" {
		t.Fatalf("primary trace: %+v", r)
	}
	if r := read("trace", "--path", "pages/login.md", "--repository", "e2e", "--workspace", filepath.Join(app, ".corvint/workspace.json")); r.State != "ready" || r.PathResolution.ID != "native:file:e2e:pages/login.md" {
		t.Fatalf("test repository trace: %+v", r)
	}
	if r := read("gaps", "--path", "pages/empty.md", "--repository", "e2e"); len(r.Results) != 1 || r.Results[0].Subject != "native:file:e2e:pages/empty.md" {
		t.Fatalf("test repository gaps: %+v", r)
	}
	if err := os.MkdirAll(app+"-web", 0700); err != nil {
		t.Fatal(err)
	}
	if _, code := corpusCLICode(t, app, "info", "--artifact", "corpus.json", "--repo", "e2e="+tests); code != "" {
		t.Fatalf("matching --repo workspace: %q", code)
	}
	if _, code := corpusCLICode(t, app, "info", "--artifact", "corpus.json", "--repo", "e2e="+tests, "--repo", "web="+app+"-web"); code != "corpus-refused" {
		t.Fatalf("undeclared alias: %q, want corpus-refused", code)
	}
}
