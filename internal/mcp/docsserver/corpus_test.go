// SPDX-License-Identifier: Apache-2.0
package docsserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// The corpus artifact rules moved here from corvint mcp's corpus group with
// the server split (MTT-V1-004, MTT-V1-005); MMR-V0-014 and MMR-V0-017 hold
// unchanged on corvint-docs mcp.

// corpusRepository is a committed repository with source.md, a cache
// directory and a built corpus artifact at corpus.json.
func corpusRepository(t *testing.T) string {
	t.Helper()
	root := repository(t)
	if err := os.MkdirAll(filepath.Join(root, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source.md"), []byte("# Evidence\n\nOriginal source.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "keep.go"), []byte("package cache\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "corpus source")
	writeArtifact(t, root, "corpus.json")
	return root
}

func writeArtifact(t *testing.T, root, name string) {
	t.Helper()
	revision, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := doccorpus.Inventory(context.Background(), root, strings.TrimSpace(string(revision)), "source.md", "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := doccorpus.Build(context.Background(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	data, err := doccorpus.Encode(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// pipeSession is one running server that answers requests one at a time.
type pipeSession struct {
	t       *testing.T
	input   io.WriteCloser
	decoder *json.Decoder
	stderr  *bytes.Buffer
	done    chan int
}

func startSession(t *testing.T, arguments []string) *pipeSession {
	t.Helper()
	inputReader, input := io.Pipe()
	outputReader, output := io.Pipe()
	session := &pipeSession{t: t, input: input, decoder: json.NewDecoder(outputReader), stderr: &bytes.Buffer{}, done: make(chan int, 1)}
	go func() {
		exit := Serve(context.Background(), "test", append([]string{"--protocol-version", "2025-11-25"}, arguments...), inputReader, output, session.stderr)
		_ = output.Close()
		session.done <- exit
	}()
	session.raw(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	if _, err := io.WriteString(input, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"); err != nil {
		t.Fatal(err)
	}
	return session
}

func (session *pipeSession) raw(line string) map[string]any {
	if _, err := io.WriteString(session.input, line+"\n"); err != nil {
		session.t.Fatal(err)
	}
	var response map[string]any
	if err := session.decoder.Decode(&response); err != nil {
		session.t.Fatalf("%v (stderr %q)", err, session.stderr.String())
	}
	return response
}

func (session *pipeSession) call(id int, name string, arguments map[string]any) map[string]any {
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}})
	if err != nil {
		session.t.Fatal(err)
	}
	result, _ := session.raw(string(raw))["result"].(map[string]any)
	return result
}

func (session *pipeSession) close() int {
	_ = session.input.Close()
	return <-session.done
}

// refused reports whether result is the corpus-refused tool error and, when
// reason is set, names it.
func refused(result map[string]any, reason string) bool {
	structured, _ := result["structuredContent"].(map[string]any)
	message, _ := structured["error"].(string)
	return result["isError"] == true && structured["code"] == corpusRefused && structured["profile"] == "corvint-corpus-mcp-tool-error/0" &&
		(reason == "" || strings.Contains(message, "("+reason+")"))
}

var search = map[string]any{"query": "Original"}

// MMR-V0-014: a missing artifact is noted, not a startup failure: corpus
// tools answer corpus-refused until it can be opened, and a stale one is
// refused, while help keeps serving.
func TestMMRV0014CorpusArtifactMissingAndStale(t *testing.T) {
	root := corpusRepository(t)
	missing := startSession(t, []string{"--root", root, "--corpus-artifact", "later.json"})
	if result := missing.call(1, "corvint.docs_search", search); !refused(result, "") {
		t.Fatalf("missing artifact: %#v", result)
	}
	if result := missing.call(2, "corvint.help", map[string]any{}); result["isError"] != false {
		t.Fatalf("help stopped serving: %#v", result)
	}
	writeArtifact(t, root, "later.json")
	if result := missing.call(3, "corvint.docs_search", search); result["isError"] != false {
		t.Fatalf("artifact built after startup is not opened: %#v", result)
	}
	if exit := missing.close(); exit != 0 || !strings.Contains(missing.stderr.String(), "corvint-docs mcp: note: --corpus-artifact unavailable (") {
		t.Fatalf("exit=%d stderr=%q", exit, missing.stderr.String())
	}

	stale := startSession(t, []string{"--root", root, "--corpus-artifact", filepath.Join(root, "corpus.json")})
	if result := stale.call(1, "corvint.docs_search", search); result["isError"] != false {
		t.Fatalf("absolute artifact path below the root: %#v", result)
	}
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := stale.call(2, "corvint.docs_search", search); !refused(result, "") {
		t.Fatalf("stale artifact: %#v", result)
	}
	if exit := stale.close(); exit != 0 || stale.stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stale.stderr.String())
	}
}

// MMR-V0-017: the root identity is pinned at startup: a retry after a missing
// artifact never opens one under a root replaced since, an artifact opened at
// startup is refused once its root is replaced, and one removed after it
// opened is corpus-refused, never the registry's unavailable-input code.
func TestMMRV0017CorpusRootPinnedAndArtifactRemoved(t *testing.T) {
	replace := func(root string) {
		t.Helper()
		other := corpusRepository(t)
		if err := os.Rename(root, root+".replaced"); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(other, root); err != nil {
			t.Fatal(err)
		}
	}
	root := corpusRepository(t)
	missing := startSession(t, []string{"--root", root, "--corpus-artifact", "later.json"})
	if result := missing.call(1, "corvint.docs_search", search); !refused(result, "corpus-unavailable") {
		t.Fatalf("missing artifact: %#v", result)
	}
	replace(root)
	writeArtifact(t, root, "later.json")
	if result := missing.call(2, "corvint.docs_search", search); !refused(result, "root-changed") {
		t.Fatalf("retry bound the replacement repository: %#v", result)
	}
	if exit := missing.close(); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, missing.stderr.String())
	}

	opened := corpusRepository(t)
	session := startSession(t, []string{"--root", opened, "--corpus-artifact", "corpus.json"})
	if result := session.call(1, "corvint.docs_search", search); result["isError"] != false {
		t.Fatalf("opened artifact: %#v", result)
	}
	replace(opened)
	if result := session.call(2, "corvint.docs_search", search); !refused(result, "root-changed") {
		t.Fatalf("opened artifact under a replaced root: %#v", result)
	}
	if exit := session.close(); exit != 0 || session.stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, session.stderr.String())
	}

	removed := corpusRepository(t)
	session = startSession(t, []string{"--root", removed, "--corpus-artifact", "corpus.json"})
	if result := session.call(1, "corvint.docs_search", search); result["isError"] != false {
		t.Fatalf("opened artifact: %#v", result)
	}
	if err := os.Remove(filepath.Join(removed, "corpus.json")); err != nil {
		t.Fatal(err)
	}
	for id, name := range []string{"corvint.docs_search", "corvint.docs_info"} {
		arguments := map[string]any{}
		if name == "corvint.docs_search" {
			arguments = search
		}
		if result := session.call(id+2, name, arguments); !refused(result, "corpus-unavailable") {
			t.Fatalf("%s after removal: %#v", name, result)
		}
	}
	if exit := session.close(); exit != 0 || session.stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, session.stderr.String())
	}
}

// MCPV0-950 with MMR-V0-014: a subdirectory root binds its toplevel before
// the corpus artifact does, so an absolute artifact under the toplevel and a
// relative one under the declared subdirectory both open; one outside the
// root refuses before any MCP byte.
func TestMCPV0950CorpusArtifactUnderASubdirectoryRoot(t *testing.T) {
	root := corpusRepository(t)
	subdirectory := filepath.Join(root, "cache")
	writeArtifact(t, root, filepath.Join("cache", "nested.json"))
	for name, artifact := range map[string]string{
		"absolute under the toplevel":     filepath.Join(root, "corpus.json"),
		"relative under the subdirectory": "nested.json",
	} {
		session := startSession(t, []string{"--root", subdirectory, "--corpus-artifact", artifact})
		if result := session.call(1, "corvint.docs_search", search); result["isError"] != false {
			t.Fatalf("%s: %#v", name, result)
		}
		if exit := session.close(); exit != 0 || session.stderr.Len() != 0 {
			t.Fatalf("%s: exit=%d stderr=%q", name, exit, session.stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if exit := Serve(context.Background(), "test", []string{"--root", subdirectory, "--corpus-artifact", "/elsewhere/corpus.json"}, bytes.NewReader(nil), &stdout, &stderr); exit != 2 ||
		stderr.String() != "corvint-docs mcp: invalid arguments: --corpus-artifact: absolute path is not below --root\n" {
		t.Fatalf("outside artifact exit=%d stderr=%q", exit, stderr.String())
	}
}
