//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
	"github.com/Beamfall/corvint/internal/testevidence"
)

// LPCV-V0-051: a FIFO or device cannot supply a receipt or stall waiting for EOF.
func TestTestValidityRefusesSpecialReceipt(t *testing.T) {
	t.Parallel()
	fifo := filepath.Join(t.TempDir(), "receipt")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{fifo, "/dev/null"} {
		code, stdout, stderr := runTestValidityCLI(t, "--receipt", name)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "invalid-test-validity-receipt") {
			t.Fatalf("exit=%d stdout=%q stderr=%s", code, stdout, stderr)
		}
	}
}

// LPCV-V0-051: initial symlinks to regular receipts remain supported by the CLI.
func TestTestValidityReadsReceiptSymlink(t *testing.T) {
	t.Parallel()
	path := writeTestValidityInput(t, `{"receipt":{"kind":"unit","tests":[]}}`)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runTestValidityCLI(t, "--receipt", link)
	if code != 0 || !strings.Contains(stdout, "corvint-test-validity/0") || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
}

// LPCV-V0-053/MTV-V0-009: `--root R test-validity --discover` and the MCP
// tool's `discover` argument emit byte-identical documents, and discovery
// cannot be combined with a named receipt.
func TestTestValidityDiscoveryMatchesMCPDocument(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, ".corvint", "test-evidence", "run.json")
	for path, body := range map[string]string{
		filepath.Join(root, ".git", "HEAD"): "ref: refs/heads/main\n",
		evidence:                            `{"receipt":{"kind":"e2e","tests":[{"name":"flow","state":"failed"}]}}`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "test-validity", "--discover"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	registry, registryErr := testvaliditybridge.New(root)
	if registryErr != nil {
		t.Fatal(registryErr)
	}
	structured, _, toolFailure, callErr := registry.Call(context.Background(), testvaliditybridge.ToolTestValidity, []byte(`{"discover":true}`))
	if toolFailure != nil || callErr != nil {
		t.Fatalf("failure=%+v err=%v", toolFailure, callErr)
	}
	var cliDocument any
	if err := json.Unmarshal(stdout.Bytes(), &cliDocument); err != nil {
		t.Fatal(err)
	}
	cli, cliErr := gokernel.CanonicalJSON(cliDocument)
	mcp, mcpErr := gokernel.CanonicalJSON(structured["document"])
	if cliErr != nil || mcpErr != nil || !bytes.Equal(cli, mcp) || !bytes.Contains(mcp, []byte(`"evidence":".corvint/test-evidence/run.json"`)) {
		t.Fatalf("cli=%s\nmcp=%s", cli, mcp)
	}
	code, out, errText := runTestValidityCLI(t, "--discover", "--receipt", evidence)
	if code != 2 || out != "" || !strings.Contains(errText, "--discover") {
		t.Fatalf("exit=%d stdout=%q stderr=%s", code, out, errText)
	}
}

// LPCV-V0-055/LPCV-V0-053: a document the provider retention writes is the one
// `corvint test-validity --discover` then selects, ahead of an older
// operator-placed document, with its bound digest CURRENT.
func TestTestValidityDiscoverSelectsProducerRetainedDocument(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(root, "web", "adds.test.ts")
	older := filepath.Join(root, ".corvint", "test-evidence", "older.json")
	for path, body := range map[string]string{testFile: "test('adds')\n", older: `{"receipt":{"kind":"e2e","tests":[]}}`} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("test('adds')\n"))
	document := fmt.Sprintf("{\n  \"receipt\": {\"kind\": \"unit\", \"identity\": {\"testFileDigests\": {%q: %q}}, \"tests\": [{\"name\": \"adds\", \"state\": \"passed\"}]}\n}\n", testFile, hex.EncodeToString(digest[:]))
	name, err := testevidence.Retain(root, "corvint-js-test-provider", []byte(document))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "test-validity", "--discover"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var discovered struct {
		Discovery struct {
			Evidence  string `json:"evidence"`
			Freshness struct {
				State string `json:"state"`
			} `json:"freshness"`
		} `json:"discovery"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &discovered); err != nil {
		t.Fatal(err)
	}
	if discovered.Discovery.Evidence != ".corvint/test-evidence/"+name || discovered.Discovery.Freshness.State != "CURRENT" {
		t.Fatalf("retained=%s document=%s", name, stdout.String())
	}
}
