// SPDX-License-Identifier: Apache-2.0
package main

import (
	"regexp"
	"strings"
	"testing"
)

// CIN-V0-4625 / V1-4732: neither required jobs nor workflow completion may wait
// for advisory Darwin coverage. The control plane still triggers only on CI.
func TestCINV04625AdvisoryDarwinIsOutsideRequiredNeedsGraph(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	if regexp.MustCompile(`(?m)^  go-macos(?:-shard)?:`).MatchString(ci) {
		t.Fatal("Darwin jobs must run outside the CI workflow")
	}
	for _, line := range strings.Split(ci, "\n") {
		if strings.Contains(line, "needs") && strings.Contains(line, "go-macos") {
			t.Errorf("required CI graph waits for advisory Darwin: %s", line)
		}
	}
	control := readWorkflow(t, "ci-control-plane.yml")
	wantLine(t, "control-plane", control, "    workflows: [CI]")
	w := readWorkflow(t, "go-macos.yml")
	wantLine(t, "Darwin workflow", w, "name: Go macOS")
	wantLine(t, "Darwin workflow", w, "  pull_request:")
	if strings.Contains(w, "workflow_run:") || strings.Contains(w, "needs: [CI]") {
		t.Fatal("Darwin workflow must start independently of CI completion")
	}
	b := jobBlock(t, w, "go-macos")
	wantLine(t, "go-macos", b, "    needs: [cadence-plan, go-macos-shard]")
	wantLine(t, "go-macos", b, "    continue-on-error: true")
	b = jobBlock(t, w, "go-macos-shard")
	for _, line := range []string{
		"    needs: [cadence-plan, docs-plan, narrowing-verdict]",
		"    continue-on-error: true",
		"      fail-fast: false",
		"      max-parallel: 4",
		"        shard: [0, 1, 2, 3]",
		"    runs-on: macos-15",
		"          name: macos-selection",
		"          name: macos-selection-shard-${{ matrix.shard }}",
	} {
		wantLine(t, "go-macos-shard", b, line)
	}
	// Only the protected-base DOCS verifier and trusted MET verdict can narrow.
	docs := jobBlock(t, w, "docs-plan")
	for _, text := range []string{"ref: ${{ github.event.pull_request.base.sha || github.sha }}",
		"--base \"$PR_BASE\" --head \"$PR_HEAD\" --target \"$TESTED_MERGE\"",
		".mode == \"DOCS\" and .verified == true"} {
		if !strings.Contains(docs, text) {
			t.Errorf("independent docs-plan lost trusted binding %q", text)
		}
	}
	// Keep the verdict's trusted source and kill-switch admission identical.
	if jobBlock(t, w, "narrowing-verdict") != jobBlock(t, ci, "narrowing-verdict") {
		t.Fatal("independent Darwin verdict diverged from CI admission guards")
	}
}

func TestCINV04625DarwinWorkflowNeverAdmitsLinuxReuse(t *testing.T) {
	r := passedRun(7)
	r.Path = ".github/workflows/go-macos.yml"
	d := decide("merge_group", 4, []run{r}, prEvidence(evidence{}, 7, testTree, 4))
	if d.Mode != "FULL" || d.RunID != nil {
		t.Fatalf("advisory Darwin workflow admitted as Linux proof: %+v", d)
	}
}
