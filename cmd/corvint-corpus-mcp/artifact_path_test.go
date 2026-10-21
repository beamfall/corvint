package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusMCPSession runs one stdio session (initialize, tools/list) and returns its exit code,
// stdout and stderr.
func corpusMCPSession(t *testing.T, root, artifact string) (int, string, string) {
	t.Helper()
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"conformance","version":"test"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--root", root, "--artifact", artifact, "--protocol-version", "2025-11-25"}, strings.NewReader(input), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestDCPV1205AbsoluteArtifactInsideRoot(t *testing.T) {
	root, _ := corpusServerFixture(t, "# Evidence\n\nOriginal source.\n")
	wantCode, want, wantErr := corpusMCPSession(t, root, "corpus.json")
	if wantCode != 0 || wantErr != "" || !strings.Contains(want, `"tools"`) {
		t.Fatalf("relative baseline: code %d stderr %q stdout %q", wantCode, wantErr, want)
	}
	code, got, stderr := corpusMCPSession(t, root, filepath.Join(root, "corpus.json"))
	if code != 0 || stderr != "" || got != want {
		t.Fatalf("absolute artifact: code %d stderr %q, stdout differs from relative: %v", code, stderr, got != want)
	}
}

func TestDCPV1206AbsoluteArtifactThroughSymlinkedRoot(t *testing.T) {
	root, _ := corpusServerFixture(t, "# Evidence\n\nOriginal source.\n")
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	// A symlinked ancestor of the root, like macOS /var -> /private/var.
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(filepath.Dir(realRoot), link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	linkedRoot := filepath.Join(link, filepath.Base(realRoot))
	_, want, _ := corpusMCPSession(t, realRoot, "corpus.json")
	for _, c := range []struct{ root, artifact string }{
		{linkedRoot, filepath.Join(realRoot, "corpus.json")},
		{realRoot, filepath.Join(linkedRoot, "corpus.json")},
	} {
		code, got, stderr := corpusMCPSession(t, c.root, c.artifact)
		if code != 0 || stderr != "" || got != want {
			t.Fatalf("root %s artifact %s: code %d stderr %q, stdout differs: %v", c.root, c.artifact, code, stderr, got != want)
		}
	}
}

func TestDCPV1207AbsoluteArtifactOutsideRootRefused(t *testing.T) {
	root, _ := corpusServerFixture(t, "# Evidence\n\nOriginal source.\n")
	outside := filepath.Join(t.TempDir(), "corpus.json")
	data, err := os.ReadFile(filepath.Join(root, "corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range []string{outside, root, filepath.Join(root, "..", filepath.Base(root)+"x", "corpus.json")} {
		code, stdout, stderr := corpusMCPSession(t, root, artifact)
		if code != 2 || stdout != "" || stderr != "corvint-corpus-mcp: --artifact: absolute path is not below --root\n" {
			t.Fatalf("artifact %s: code %d stdout %q stderr %q", artifact, code, stdout, stderr)
		}
	}
}

// corpusMCPMapSession is corpusMCPSession with one --map argument.
func corpusMCPMapSession(t *testing.T, root, mapPath string) (int, string, string) {
	t.Helper()
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"conformance","version":"test"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--root", root, "--artifact", "corpus.json", "--map", mapPath, "--protocol-version", "2025-11-25"}, strings.NewReader(input), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// V1-1998: an absolute --map below --root behaves like its relative spelling (DCP-V1-205,
// DCP-V1-206); one not below --root names --map (DCP-V1-207).
func TestDCPV1205AbsoluteMapInsideRoot(t *testing.T) {
	root, _ := mapPlanRoot(t)
	wantCode, want, wantErr := corpusMCPMapSession(t, root, "map-appmap.json")
	if wantCode != 0 || wantErr != "" || !strings.Contains(want, "corvint.map_plan") {
		t.Fatalf("relative baseline: code %d stderr %q stdout %q", wantCode, wantErr, want)
	}
	code, got, stderr := corpusMCPMapSession(t, root, filepath.Join(root, "map-appmap.json"))
	if code != 0 || stderr != "" || got != want {
		t.Fatalf("absolute map: code %d stderr %q, stdout differs from relative: %v", code, stderr, got != want)
	}
	outside := filepath.Join(t.TempDir(), "map-appmap.json")
	code, stdout, stderr := corpusMCPMapSession(t, root, outside)
	if code != 2 || stdout != "" || stderr != "corvint-corpus-mcp: --map: absolute path is not below --root\n" {
		t.Fatalf("outside map: code %d stdout %q stderr %q", code, stdout, stderr)
	}
}

// TestDCPV1242RelativeArtifactRefusalNamesArgumentAndRule checks that a relative --artifact that
// breaks the local input path rule exits 2 naming the argument and the rule, not the generic
// `repository unavailable` refusal (DCP-V1-242, V1-1228).
func TestDCPV1242RelativeArtifactRefusalNamesArgumentAndRule(t *testing.T) {
	root, _ := corpusServerFixture(t, "# Evidence\n\nOriginal source.\n")
	for _, c := range []struct{ artifact, rule string }{
		{"./corpus.json", "got a non-canonical path"},
		{"../corpus.json", "got a parent traversal"},
		{"docs//corpus.json", "got a non-canonical path"},
		{"docs\\corpus.json", "got a backslash, NUL, tab, CR or LF"},
	} {
		code, stdout, stderr := corpusMCPSession(t, root, c.artifact)
		want := "corvint-corpus-mcp: --artifact: local input paths must be canonical and relative to --root (" + c.rule + ")\n"
		if code != 2 || stdout != "" || stderr != want {
			t.Fatalf("artifact %q: code %d stdout %q stderr %q, want %q", c.artifact, code, stdout, stderr, want)
		}
	}
	// A canonical relative path that does not exist keeps the generic refusal.
	if code, _, stderr := corpusMCPSession(t, root, "missing.json"); code != 2 || stderr != "corvint-corpus-mcp: repository unavailable\n" {
		t.Fatalf("missing artifact: code %d stderr %q", code, stderr)
	}
}
