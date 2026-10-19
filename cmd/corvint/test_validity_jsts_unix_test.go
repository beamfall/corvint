//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
)

const jsDiscoverSpec = `describe('add', () => {
  it('adds', () => {
    expect(add(1, 2)).toBe(3);
  });
  it.skip('later', () => { expect(add(0, 0)).toBe(0); });
});
`

// TestTestValidityDiscoverJSSourceRules: `corvint test-validity --discover` over an anchored,
// digest-bound JS receipt reads the test file with the JS/TS reader: the skipped example is
// INELIGIBLE with JS-SKIPPED, the clean one ELIGIBLE with the JS read reason and its body
// expectations, and the MCP tool emits the same document (TSA-V0-073, TSA-V0-082).
func TestTestValidityDiscoverJSSourceRules(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "src", "add.test.js")
	sum := sha256.Sum256([]byte(jsDiscoverSpec))
	tests := []map[string]any{}
	for _, line := range []int{2, 5} {
		tests = append(tests, map[string]any{"name": "t", "state": "passed", "anchor": map[string]any{"file": "src/add.test.js", "line": line}})
	}
	receipt, err := json.Marshal(map[string]any{"receipt": map[string]any{
		"kind": "unit", "identity": map[string]any{"testFileDigests": map[string]string{source: hex.EncodeToString(sum[:])}}, "tests": tests,
	}})
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		filepath.Join(root, ".git", "HEAD"): "ref: refs/heads/main\n",
		source:                              jsDiscoverSpec,
		filepath.Join(root, ".corvint", "test-evidence", "corvint-js-test-provider-1.json"): string(receipt),
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
	type axis struct{ State, Reason string }
	var out struct {
		Tests []struct {
			Projection  struct{ Hygiene, Strength axis }
			SourceRules *struct {
				Read         string
				Expectations int
				Findings     []struct{ Rule string }
			}
		}
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil || len(out.Tests) != 2 {
		t.Fatalf("document %s: %v", stdout.String(), err)
	}
	clean, skipped := out.Tests[0], out.Tests[1]
	if clean.Projection.Hygiene != (axis{"ELIGIBLE", "js-example-source-read"}) || clean.Projection.Strength.Reason != "example-body-expectations" ||
		clean.SourceRules == nil || clean.SourceRules.Read != "located" || clean.SourceRules.Expectations != 1 || len(clean.SourceRules.Findings) != 0 {
		t.Errorf("clean %+v %+v", clean.Projection, clean.SourceRules)
	}
	if skipped.Projection.Hygiene != (axis{"INELIGIBLE", "JS-SKIPPED"}) || skipped.SourceRules == nil || len(skipped.SourceRules.Findings) != 1 || skipped.SourceRules.Findings[0].Rule != "JS-SKIPPED" {
		t.Errorf("skipped %+v %+v", skipped.Projection, skipped.SourceRules)
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
	if cliErr != nil || mcpErr != nil || !bytes.Equal(cli, mcp) {
		t.Fatalf("cli=%s\nmcp=%s", cli, mcp)
	}
}
