// SPDX-License-Identifier: Apache-2.0
//go:build darwin || linux

package sessiontest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// AZU-V0-007 (selbatch fix round 2): two declared consumers with one runner
// argv launch it once, and the receipt keeps a terminal outcome for each
// consumer's unit, passing and failing together.
func TestAZUV0007SharedConsumerRunnerRunsOnce(t *testing.T) {
	for _, tc := range []struct {
		mode, result, outcome string
	}{
		{mode: "pass", result: ResultPass, outcome: OutcomePass},
		{mode: "fail", result: ResultFail, outcome: OutcomeFail},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			runner, m, _, dir := runnerFixture(t, tc.mode)
			script := filepath.Join(dir, "tools", "gate")
			if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
				t.Fatal(err)
			}
			body := "#!/bin/sh\n[ \"$1\" = --version ] && echo gate 1 && exit 0\necho run >> \"$FAKE_DIR/gate-runs\"\n[ \"$FAKE_MODE\" = pass ]\n"
			if err := execfile.WriteFile(script, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			units := []string{"declared:declared-consumer:web/e2e.mjs", "declared:declared-consumer:web/nav.mjs"}
			m.Scope, m.Units = ScopeNarrowed, nil
			runner.Declared.Languages = nil
			for _, unit := range units {
				m.Units = append(m.Units, Unit{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: unit})
				runner.Declared.Languages = append(runner.Declared.Languages, LanguageCommand{Language: unit[len("declared:"):], Argv: []string{"./tools/gate"}})
			}
			digest, _ := Digest(m)
			receipt := runner.Execute(context.Background(), m, digest, 0)
			if runs, _ := os.ReadFile(filepath.Join(dir, "gate-runs")); string(runs) != "run\n" {
				t.Fatalf("the shared runner ran %q, want once", runs)
			}
			if receipt.Result != tc.result || len(receipt.Invocations) != 1 || len(receipt.Invocations[0].Outcomes) != len(units) {
				t.Fatalf("receipt %s %+v", receipt.Result, receipt.Invocations)
			}
			for index, outcome := range receipt.Invocations[0].Outcomes {
				if outcome != (Outcome{Outcome: tc.outcome, Slice: SliceWhole, Unit: units[index]}) {
					t.Fatalf("outcome %d: %+v", index, outcome)
				}
			}
		})
	}
}
