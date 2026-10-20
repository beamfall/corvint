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
