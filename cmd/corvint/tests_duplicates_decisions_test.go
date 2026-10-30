package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTestsDuplicatesDecisionsCLI proves --decisions FILE resolves a gate pair from outside the
// repository, names its source, and refuses an unreadable file (TSA-V0-660, TSA-V0-662).
func TestTestsDuplicatesDecisionsCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	base := testsCommit(t, root, map[string]string{"e2e/cart.spec.ts": testsCartSpec})
	testsCommit(t, root, map[string]string{"e2e/basket.spec.ts": strings.Replace(testsCartSpec, "adds an item", "basket gains", 1)})
	gate := []string{"--root", root, "tests", "duplicates", "--max-line-tests", "2", "--base", base, "--gate", "V1-2842"}
	if code, _, _ := runCLI(t, gate...); code != 1 {
		t.Fatalf("gate without decisions: code=%d", code)
	}
	file := filepath.Join(t.TempDir(), "review.json")
	decision := `{"schema":"corvint-test-duplicate-decisions/0","decisions":[{"decision":"distinct-from",` +
		`"test":"e2e/basket.spec.ts :: basket gains","other":"e2e/cart.spec.ts :: adds an item","reason":"guest basket","reviewer":"ada"}]}`
	if err := os.WriteFile(file, []byte(decision), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errb := runCLI(t, append(gate, "--decisions", file)...)
	if code != 0 || !strings.Contains(out, `"decisionSource": "caller:`+file+`"`) {
		t.Fatalf("gate with caller decision: code=%d out=%s err=%s", code, out, errb)
	}
	if code, _, errb := runCLI(t, append(gate, "--decisions", file+".absent")...); code != 2 || !strings.Contains(errb, "tests-decisions-invalid") {
		t.Fatalf("unreadable decision file: code=%d err=%s", code, errb)
	}
}
