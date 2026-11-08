// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestSelectionReplayRunsOnce_AFPV09498 (AFP-V0-9498): two concurrent full
// corpus replays overran doc-gates' 30-minute cap on PR 145. The baseline
// replay must carry the AFP-V0-9501 parity assertions itself, except when it
// records results, and doc-gates must not also run the separate contract replay.
func TestSelectionReplayRunsOnce_AFPV09498(t *testing.T) {
	root := requireMake(t)
	replay := func(extra ...string) string {
		t.Helper()
		args := append([]string{"-s", "-n", "-C", root, "-f", "Makefile", "--no-print-directory", "selection-baseline-check"}, extra...)
		command := exec.Command("make", args...)
		// A make-driven CI run exports MAKEFLAGS, BASE and possibly the
		// recording output; none may leak into this dry run.
		command.Env = append(slices.DeleteFunc(os.Environ(), func(entry string) bool {
			return strings.HasPrefix(entry, "BASE=") || strings.HasPrefix(entry, "SELECTION_BASELINE_OUT=") || strings.HasPrefix(entry, "MAKELEVEL=")
		}), "MAKEFLAGS=")
		var stderr strings.Builder
		command.Stderr = &stderr
		out, err := command.Output()
		if err != nil {
			t.Fatalf("make -n selection-baseline-check: %v\n%s", err, stderr.String())
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, `"$d/selection-baseline" --check`) {
				return line
			}
		}
		t.Fatalf("make -n selection-baseline-check printed no replay line:\n%s", out)
		return ""
	}
	if line := replay(); !strings.Contains(line, `SELECTION_CONTRACT_CHECK=1 "$d/selection-baseline" --check`) {
		t.Fatalf("the doc-gates replay does not assert root-Go parity: %q", line)
	}
	if line := replay("SELECTION_BASELINE_OUT=out.json"); strings.Contains(line, "SELECTION_CONTRACT_CHECK") {
		t.Fatalf("a recording replay must keep the local dry-run manifests: %q", line)
	}
	makefile, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^DOC_GATE_TARGETS :=((?:.*\\\n)*.*)$`).FindSubmatch(makefile)
	if m == nil {
		t.Fatal("Makefile: no DOC_GATE_TARGETS list")
	}
	targets := strings.Fields(strings.ReplaceAll(string(m[1]), "\\", " "))
	var baseline, contract bool
	for _, target := range targets {
		baseline = baseline || target == "selection-baseline-check"
		contract = contract || target == "selection-contract-check"
	}
	if !baseline || contract {
		t.Fatalf("DOC_GATE_TARGETS must run selection-baseline-check and not selection-contract-check: baseline=%t contract=%t", baseline, contract)
	}
}
