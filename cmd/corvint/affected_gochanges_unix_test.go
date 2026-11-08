// SPDX-License-Identifier: Apache-2.0
//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-3323: the default compact JSON exposes the comparison, --path stays
// conservative, and a second plan over different dirty bytes cannot reuse a
// comment-only classification through the graph cache.
func TestAffectedCommentOnlyJSONAndCache_AFP_V0_3323(t *testing.T) {
	root, base := sourceSliceFixture(t)
	file := filepath.Join(root, "u", "u.go")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	plan := func(args ...string) affectedCompactPlan {
		t.Helper()
		argv := append([]string{"--root", root, "affected", "--format", "json"}, args...)
		code, stdout, stderr := runCLI(t, argv...)
		if code != 0 {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
		var receipt struct {
			Plan affectedCompactPlan `json:"plan"`
		}
		if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
			t.Fatal(err)
		}
		return receipt.Plan
	}
	selected := func(plan affectedCompactPlan, id string) bool {
		for _, item := range plan.Selected {
			if item.UnitID == id {
				return true
			}
		}
		return false
	}
	if err := os.WriteFile(file, append(append([]byte(nil), body...), []byte("\n// context\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--base", base}, {"--base", base, "--no-cache"}, {"--base", base}} {
		got := plan(args...)
		if len(got.GoChanges) != 1 || got.GoChanges[0] != (affected.GoChange{Path: "u/u.go", Kind: affected.GoCommentOnly}) || !selected(got, "go:example.test/m/u") || selected(got, "go:example.test/m/q") {
			t.Fatalf("comment-only CLI plan: %+v", got)
		}
	}
	if err := os.WriteFile(file, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := plan("--path", "u/u.go"); len(got.GoChanges) != 0 || !selected(got, "go:example.test/m/q") {
		t.Fatalf("path-only plan must fail closed: %+v", got)
	}
	if err := os.WriteFile(file, []byte("package u\nvar Last string\nfunc U() string { return \"changed\" }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := plan("--base", base); len(got.GoChanges) != 0 || !selected(got, "go:example.test/m/q") {
		t.Fatalf("code edit reused comment classification: %+v", got)
	}
}
