package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// manifestScopesRepository commits files Markdown files under each of dirs in one commit, written
// by one git fast-import stream so a 20,000-file fixture stays cheap; the commit is then checked out.
func manifestScopesRepository(t *testing.T, files int, dirs ...string) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	var stream bytes.Buffer
	stream.WriteString("commit refs/heads/main\ncommitter t <t@example.invalid> 946684800 +0000\ndata 7\nfixture\n")
	for _, dir := range dirs {
		for i := 0; i < files; i++ {
			body := fmt.Sprintf("# %s %d\n\nNote %d.\n", dir, i, i)
			fmt.Fprintf(&stream, "M 100644 inline %s/d%03d/f%05d.md\ndata %d\n%s\n", dir, i/100, i, len(body), body)
		}
	}
	command := exec.Command("git", "fast-import", "--quiet")
	command.Dir, command.Stdin = root, &stream
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fast-import: %v %s", err, out)
	}
	cemGit(t, root, "checkout", "-q", "-f", "main")
	return root, cemGit(t, root, "rev-parse", "HEAD")
}

func decodeCorpusManifest(t *testing.T, out string) doccorpus.Manifest {
	t.Helper()
	var m doccorpus.Manifest
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDCPV1160SchemaTwoManifestOfTwentyThousandFilesBuilds(t *testing.T) {
	// DCP-V1-160, DCP-V1-161, DCP-V1-163: two scopes of 10,000 files each yield one /2 manifest
	// with both scopes and 20,000 inputs, which `docs corpus build` accepts.
	start := time.Now()
	root, rev := manifestScopesRepository(t, 10000, "a", "b")
	t.Logf("fixture: %s", time.Since(start))
	start = time.Now()
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "manifest", "--schema", "2", "--revision", rev, "--scope", "b", "--scope", "a", "--timestamp", "2026-10-19T00:00:00Z")
	if code != 0 {
		t.Fatalf("manifest: %d %s", code, stderr)
	}
	t.Logf("manifest: %s", time.Since(start))
	m := decodeCorpusManifest(t, out)
	if m.Schema != doccorpus.ManifestSchemaV2 || len(m.Scopes) != 2 || m.Scopes[0].Path != "a" || m.Scopes[1].Path != "b" || len(m.Inputs) != 20000 {
		t.Fatalf("schema %s scopes %+v inputs %d", m.Schema, m.Scopes, len(m.Inputs))
	}
	cemWrite(t, root, "corpus-input.json", out)
	start = time.Now()
	code, built, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--manifest", "corpus-input.json")
	if code != 0 {
		t.Fatalf("build: %d %s", code, stderr)
	}
	t.Logf("build: %s", time.Since(start))
	if !strings.Contains(built, `"schema":"`+doccorpus.SchemaV2+`"`) && !strings.Contains(built, `"schema": "`+doccorpus.SchemaV2+`"`) {
		t.Fatal("build did not emit a /2 artifact")
	}
	// DCP-V1-162: the default /1 inventory of the same scope names its bound, scope and count.
	code, _, stderr = corpusCLI(t, root, "docs", "corpus", "manifest", "--revision", rev, "--scope", "a", "--timestamp", "2026-10-19T00:00:00Z")
	for _, want := range []string{"corpus-refused", "oversized scope: a holds 10000 tracked files", doccorpus.ManifestSchema + " bound of 4096", "--schema 2 admits 100000"} {
		if code != 2 || !strings.Contains(stderr, want) {
			t.Fatalf("/1 refusal %d lacks %q: %s", code, want, stderr)
		}
	}
}

func TestDCPV1160DefaultSingleScopeUnchanged(t *testing.T) {
	// DCP-V1-160: without --schema, one --scope emits the same /1 single-scope manifest as before.
	root, rev := manifestScopesRepository(t, 3, "docs", "src")
	args := []string{"docs", "corpus", "manifest", "--revision", rev, "--scope", "docs", "--timestamp", "2026-10-19T00:00:00Z"}
	code, out, stderr := corpusCLI(t, root, args...)
	if code != 0 {
		t.Fatal(stderr)
	}
	want, err := doccorpus.Inventory(context.Background(), root, rev, "docs", "2026-10-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := doccorpus.Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	if out != string(raw) {
		t.Fatal("CLI manifest differs from the single-scope inventory")
	}
	m := decodeCorpusManifest(t, out)
	if m.Schema != doccorpus.ManifestSchema || len(m.Scopes) != 1 || m.Scopes[0] != (doccorpus.Scope{Path: "docs", Revision: rev}) || len(m.Inputs) != 3 {
		t.Fatalf("%+v", m)
	}
	_, explicit, _ := corpusCLI(t, root, append(args, "--schema", "1")...)
	if explicit != out {
		t.Fatal("--schema 1 differs from the default")
	}
}

func TestDCPV1160SchemaFlagValues(t *testing.T) {
	// DCP-V1-160: --schema admits 1 or 2 and only on manifest.
	root, rev := manifestScopesRepository(t, 1, "docs")
	for _, args := range [][]string{
		{"docs", "corpus", "manifest", "--revision", rev, "--scope", "docs", "--timestamp", "2026-10-19T00:00:00Z", "--schema", "3"},
		{"docs", "corpus", "manifest", "--revision", rev, "--scope", "docs", "--timestamp", "2026-10-19T00:00:00Z", "--schema", "2", "--schema", "2"},
		{"docs", "corpus", "build", "--manifest", "corpus-input.json", "--schema", "2"},
	} {
		if code, _, _ := corpusCLI(t, root, args...); code != 2 {
			t.Fatalf("%v admitted: %d", args, code)
		}
	}
}

func TestDCPV1161ScopesSortedAndOverlapRefused(t *testing.T) {
	// DCP-V1-161: scope order does not change the manifest; duplicate and nested scopes are refused.
	root, rev := manifestScopesRepository(t, 2, "docs", "src", "src/sub")
	manifest := func(scopes ...string) (int, string, string) {
		args := []string{"docs", "corpus", "manifest", "--revision", rev, "--timestamp", "2026-10-19T00:00:00Z"}
		for _, s := range scopes {
			args = append(args, "--scope", s)
		}
		return corpusCLI(t, root, args...)
	}
	code, forward, stderr := manifest("docs", "src")
	if code != 0 {
		t.Fatal(stderr)
	}
	_, reverse, _ := manifest("src", "docs")
	if forward != reverse {
		t.Fatal("scope argument order changed the manifest")
	}
	m := decodeCorpusManifest(t, forward)
	if m.Schema != doccorpus.ManifestSchema || len(m.Scopes) != 2 || m.Scopes[0].Path != "docs" || m.Scopes[1].Path != "src" || len(m.Inputs) != 6 {
		t.Fatalf("%+v", m)
	}
	cemWrite(t, root, "corpus-input.json", forward)
	if code, _, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--manifest", "corpus-input.json"); code != 0 {
		t.Fatalf("multi-scope /1 manifest not built: %s", stderr)
	}
	for _, c := range []struct {
		scopes []string
		want   string
	}{
		{[]string{"docs", "docs"}, "duplicate scope docs"},
		{[]string{"src/sub", "src"}, "overlapping scopes: src contains src/sub"},
		{[]string{"docs", "missing"}, "empty scope: missing holds no tracked files"},
	} {
		code, _, stderr := manifest(c.scopes...)
		if code != 2 || !strings.Contains(stderr, "corpus-refused") || !strings.Contains(stderr, c.want) {
			t.Fatalf("%v: %d %s", c.scopes, code, stderr)
		}
	}
}
