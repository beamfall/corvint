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
	"github.com/Beamfall/corvint/internal/skhproof"
)

// SKH-V0-184: `corvint-test e2e how-evidence` records a hosted prove report and a validity report
// for the committed candidate, and how-accept assesses the recorded evidence past infrastructure.
func TestSKHV0_184_HowEvidenceCLI(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	dir := t.TempDir()
	lines := []string{"import { test, expect } from '@playwright/test';", "test('ships', async ({ page }) => {", appmap.DraftGuardLine,
		"  await page.goto('/orders/1');", "  await expect(page.getByText('Shipped')).toBeVisible();", "});"}
	const proposed = "e2e/how-0123456789ab.spec.ts"
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	answer, _ := json.Marshal(map[string]any{"schema": appmap.HowSchema, "map_revision": strings.Repeat("a", 40), "draft": appmap.SKDraft{
		Status: appmap.HowComplete, Label: "PROPOSAL", ProposedPath: proposed, GuardLine: 3, Lines: lines, Digest: hex.EncodeToString(sum[:])}})
	answerPath := filepath.Join(dir, "answer.json")
	if err := os.WriteFile(answerPath, answer, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := filepath.Join(dir, "repo")
	if err := run(ctx, []string{"e2e", "how-accept", "--answer", answerPath, "--write", filepath.Join(dir, "candidate.spec.ts")}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	candidate, err := os.ReadFile(filepath.Join(dir, "candidate.spec.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "e2e"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, proposed), candidate, 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.WriteFile(filepath.Join(repo, "big.txt"), bytes.Repeat([]byte("x"), maxCandidateBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-qm", "candidate")
	head := git("rev-parse", "HEAD")
	write := func(name string, doc any) string {
		data, _ := json.Marshal(doc)
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	passed := []any{map[string]any{"state": "passed"}, map[string]any{"state": "passed"}}
	prove := write("prove.json", map[string]any{"schema": skhproof.ProveSchema, "head": head, "retries": 0, "scenarios": []any{map[string]any{
		"placement": proposed, "hosted": true, "verdict": "DISCRIMINATES",
		"head":     map[string]any{"application_revision": head, "test_revision": head, "qualified": true, "stability": "DETERMINISTIC", "outcomes": passed},
		"controls": []any{map[string]any{"id": "control:omitted-event:step-1", "status": "killed"}}}}})
	validity := write("validity.json", map[string]any{"schema": skhproof.ValiditySchema, "revision": head,
		"files": []any{map[string]any{"path": proposed, "findings": []any{}}},
		"tests": []any{map[string]any{"test": proposed + ":2", "hygiene": map[string]any{"state": "ELIGIBLE"}, "strength": map[string]any{}}}})

	var out bytes.Buffer
	if err := run(ctx, []string{"e2e", "how-evidence", "--root", repo, "--answer", answerPath, "--prove", prove, "--validity", validity}, &out); err != nil {
		t.Fatal(err)
	}
	evidence := write("evidence.json", json.RawMessage(out.Bytes()))
	out.Reset()
	if err := run(ctx, []string{"e2e", "how-accept", "--answer", answerPath, "--evidence", evidence}, &out); err != nil {
		t.Fatal(err)
	}
	var r skhproof.Receipt
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.Code != skhproof.CodeAcceptanceBlocked || r.Evidence != "recorded" {
		t.Fatalf("receipt = %s (%v)", out.Bytes(), err)
	}

	// A blob over the read bound is refused, not returned.
	if data, err := committedFile(ctx, repo)(head, "big.txt"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("oversized blob: %d bytes, %v", len(data), err)
	}

	// A missing report or a missing flag is refused before anything is read back.
	var refusal *skhproof.Refusal
	err = run(ctx, []string{"e2e", "how-evidence", "--root", repo, "--answer", answerPath, "--prove", filepath.Join(dir, "absent.json"), "--validity", validity}, &bytes.Buffer{})
	if !errors.As(err, &refusal) || refusal.Code != "skh-evidence-invalid" {
		t.Errorf("absent prove report: %v", err)
	}
	if err := run(ctx, []string{"e2e", "how-evidence", "--answer", answerPath}, &bytes.Buffer{}); err == nil {
		t.Errorf("missing --prove and --validity were accepted")
	}
}
