package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/changee2e"
	"github.com/Beamfall/corvint/internal/skhproof"
)

// SKH-V0-174: `corvint-test e2e how-accept` writes the candidate only to a new caller-named
// path, refuses an existing file or symlink and an incomplete draft, and prints the receipt; with
// no recorded live run the receipt blocks as infrastructure.
func TestSKHV0_174_HowAcceptCLI(t *testing.T) {
	dir := t.TempDir()
	lines := []string{"import { test } from '@playwright/test';", "test('ships', async ({ page }) => {", appmap.DraftGuardLine,
		"  await page.goto('/orders/1');", "});"}
	answer := func(status string) string {
		sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
		data, _ := json.Marshal(map[string]any{"schema": appmap.HowSchema, "map_revision": strings.Repeat("a", 40), "draft": appmap.SKDraft{
			Status: status, Label: "PROPOSAL", GuardLine: 3, Lines: lines, Digest: hex.EncodeToString(sum[:])}})
		p := filepath.Join(dir, status+".json")
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	ctx := context.Background()
	complete, target := answer(appmap.HowComplete), filepath.Join(dir, "how.spec.ts")
	var out bytes.Buffer
	if err := run(ctx, []string{"e2e", "how-accept", "--answer", complete, "--write", target}, &out); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(target)
	if err != nil || bytes.Contains(written, []byte("test.fixme(")) || bytes.Count(written, []byte("\n")) != len(lines)-1 {
		t.Fatalf("candidate = %q, %v", written, err)
	}
	var r skhproof.Receipt
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.Status != skhproof.Blocked || r.Code != skhproof.CodeInfrastructure ||
		r.Transform == nil || r.Transform.CandidateSHA == "" {
		t.Fatalf("receipt = %s (%v)", out.Bytes(), err)
	}
	// An existing file or a symlink is never overwritten.
	link := filepath.Join(dir, "link.spec.ts")
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), link); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{target, link} {
		var r *changee2e.Refusal
		if err := run(ctx, []string{"e2e", "how-accept", "--answer", complete, "--write", p}, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-out-unsafe" {
			t.Fatalf("%s: %v", p, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(dir, "elsewhere")); err == nil {
		t.Fatal("symlink target written")
	}
	var refusal *skhproof.Refusal
	partial := answer(appmap.HowIncomplete)
	if err := run(ctx, []string{"e2e", "how-accept", "--answer", partial, "--write", filepath.Join(dir, "p.spec.ts")}, &bytes.Buffer{}); !errors.As(err, &refusal) || refusal.Code != skhproof.CodeGenerateIncomplete {
		t.Fatalf("partial write: %v", err)
	}
	out.Reset()
	if err := run(ctx, []string{"e2e", "how-accept", "--answer", partial}, &out); err != nil || !strings.Contains(out.String(), `"code":"generate-incomplete"`) {
		t.Fatalf("partial receipt: %v %s", err, out.Bytes())
	}
	for _, args := range [][]string{{"e2e", "how-accept"}, {"e2e", "how-accept", "--answer", complete, "extra"}, {"e2e", "how-accept", "--bogus"}} {
		if err := run(ctx, args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "how-accept --answer FILE") {
			t.Fatalf("%v: %v", args, err)
		}
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"schema":"site-knowledge-proof-evidence/0","x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx, []string{"e2e", "how-accept", "--answer", complete, "--evidence", bad}, &bytes.Buffer{}); !errors.As(err, &refusal) || refusal.Code != "skh-evidence-invalid" {
		t.Fatalf("bad evidence: %v", err)
	}
}

// SKH-V0-163: the stage 8 pipeline is linked only into the companion: core `corvint` and the
// corpus MCP server never import it, so core can only read its receipt.
func TestSKHV0_163_OnlyTheCompanionImportsThePipeline(t *testing.T) {
	for _, target := range []string{"../corvint", "../corvint-corpus-mcp"} {
		out, err := exec.Command("go", "list", "-deps", target).Output()
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		for _, dep := range strings.Fields(string(out)) {
			if strings.HasSuffix(dep, "/internal/skhproof") {
				t.Fatalf("%s imports %s", target, dep)
			}
		}
	}
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil || !strings.Contains(string(out), "/internal/skhproof\n") {
		t.Fatalf("companion does not link the pipeline: %v", err)
	}
}
