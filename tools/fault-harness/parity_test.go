// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// TestGoParityWithOracle_FIH_V1_011 builds tools/affected-edit-oracle as it is
// and runs one AEO-V0 mutation case through it and through the harness Go
// adapter, on the same synthetic repository, pin and planner. The miss
// verdicts and the missing units must match for a planner that selects every
// killed unit, one that drops one of them and one that selects nothing.
func TestGoParityWithOracle_FIH_V1_011(t *testing.T) {
	ctx := context.Background()
	repo := newFixtureRepo(t)
	repo.writeGoModule()
	pin := repo.commit("fixture")
	oracle := filepath.Join(t.TempDir(), "affected-edit-oracle")
	if out, err := command(ctx, ".", 10*time.Minute, "go", "build", "-buildvcs=false", "-o", oracle, "../affected-edit-oracle"); err != nil {
		t.Fatalf("build oracle: %v\n%s", err, out)
	}
	planner, mode := fakePlanner(t)
	corpus := filepath.Join(t.TempDir(), "corpus.json")
	tc := map[string]any{
		"id": "leaf-value", "repo": "corvint", "baseSha": pin, "editClass": "body",
		"edit":       map[string]string{"path": "leaf.go", "operation": "replace-literal", "before": "return 1", "content": "return (1)"},
		"mustSelect": []string{"go:oracle.invalid/leaf"}, "maySelect": []string{"go:oracle.invalid/leaf/other"},
		"why": "Value is asserted by TestValue and, through Double, by TestDouble.",
		"mutation": map[string]any{
			"edit":     map[string]string{"path": "leaf.go", "operation": "replace-literal", "before": "return 1", "content": "return 2"},
			"commands": []map[string]any{{"argv": []string{"go", "test", "-json", "-count=1", "./..."}, "format": "go-json"}},
		},
	}
	body, _ := json.Marshal(map[string]any{"profile": aeoProfile, "cases": []any{tc}})
	if err := os.WriteFile(corpus, body, 0o600); err != nil {
		t.Fatal(err)
	}
	leaf, other := "go:oracle.invalid/leaf", "go:oracle.invalid/leaf/other"
	for _, plan := range []struct {
		units   []string
		safety  string
		missing []string
	}{
		{[]string{leaf, other}, "PASS", nil},
		{[]string{leaf}, "SAFETY_FAIL", []string{other}},
		{nil, "SAFETY_FAIL", []string{leaf, other}},
	} {
		setMode(t, mode, plan.units...)
		// The oracle, unchanged.
		oracleOut := filepath.Join(t.TempDir(), "oracle.json")
		out, _ := command(ctx, ".", 10*time.Minute, oracle, "--corpus", corpus, "--root", repo.root, "--workspace", repo.root, "--clones", t.TempDir(), "--corvint", planner, "--mutate", "--allow-over", "--out", oracleOut)
		var oracleReport struct {
			Cases []struct {
				Safety struct {
					Status  string   `json:"status"`
					Missing []string `json:"missing"`
					Detail  string   `json:"detail"`
				} `json:"safety"`
			} `json:"cases"`
		}
		raw, err := os.ReadFile(oracleOut)
		if err != nil || json.Unmarshal(raw, &oracleReport) != nil || len(oracleReport.Cases) != 1 {
			t.Fatalf("oracle report: %v\n%s", err, out)
		}
		o := oracleReport.Cases[0].Safety
		// The harness Go adapter, through its parity command.
		var w bytes.Buffer
		harnessOut := filepath.Join(t.TempDir(), "parity.json")
		code := run(ctx, []string{"parity", "--corpus", corpus, "--root", repo.root, "--clones", t.TempDir(), "--corvint", planner, "--out", harnessOut}, &w)
		var parity struct {
			Cases []parityRow `json:"cases"`
		}
		raw, err = os.ReadFile(harnessOut)
		if err != nil || json.Unmarshal(raw, &parity) != nil || len(parity.Cases) != 1 {
			t.Fatalf("harness report: %v %s", err, w.String())
		}
		h := parity.Cases[0]
		if o.Status != plan.safety || h.Safety != plan.safety || !reflect.DeepEqual(o.Missing, plan.missing) && len(o.Missing)+len(plan.missing) > 0 || !reflect.DeepEqual(h.Missing, plan.missing) {
			t.Fatalf("plan %v: oracle %+v, harness %+v", plan.units, o, h)
		}
		if wantCode := map[bool]int{true: 0, false: 1}[plan.safety == "PASS"]; code != wantCode {
			t.Fatalf("plan %v: exit %d", plan.units, code)
		}
	}
	if status := repo.git("status", "--porcelain"); status != "" || repo.git("rev-parse", "HEAD") != pin {
		t.Fatalf("source written: %q", status)
	}
}
