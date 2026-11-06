// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// CEP-V0-8600 audits the only per-sample metric reading another arm.
func TestCEPV08600MergeClearsStaleCrossArmMetrics(t *testing.T) {
	const key = "cb_file_baseline_precision_length_matched"
	for _, tc := range []struct {
		name              string
		baseline, context arm
		present           bool
		want              float64
	}{
		{"replaced baseline", arm{Ranked: []string{"miss"}}, arm{Ranked: []string{"gold"}}, true, 0},
		{"replaced answer", arm{Ranked: []string{"gold", "miss"}}, arm{Ranked: []string{"gold", "miss"}}, true, 0.5},
		{"empty baseline", arm{}, arm{Ranked: []string{"gold"}}, true, 0},
		{"errored baseline", arm{Error: "failed"}, arm{Ranked: []string{"gold"}}, false, 0},
		{"abstained answer", arm{Ranked: []string{"gold"}}, arm{}, false, 0},
		{"errored answer", arm{Ranked: []string{"gold"}}, arm{Error: "failed"}, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			save := func(name string, arms map[string]arm) string {
				detail := sampleReport{ID: "cb-merge", TaskType: contextBenchTask, Repo: "o/n", Gold: []string{"gold"}, Arms: arms, Metrics: map[string]metrics{}}
				for name := range arms {
					detail.Metrics[name] = metrics{key: 0.75}
				}
				data, err := json.Marshal(storedReport{SamplesSHA256: "s", Limit: 10, Details: []sampleReport{detail}})
				if err != nil {
					t.Fatal(err)
				}
				p := filepath.Join(t.TempDir(), name+".json")
				if err := os.WriteFile(p, data, 0600); err != nil {
					t.Fatal(err)
				}
				return p
			}
			first := save("first", map[string]arm{"context": {Ranked: []string{"old"}}, lengthMatchedBaseline: {Ranked: []string{"old"}}})
			second := save("second", map[string]arm{"context": tc.context, lengthMatchedBaseline: tc.baseline})
			report, err := resummarize(options{summaries: []string{first, second}})
			if err != nil {
				t.Fatal(err)
			}
			detail := report["details"].([]sampleReport)[0]
			got, present := detail.Metrics["context"][key]
			if present != tc.present || present && got != tc.want {
				t.Fatalf("metric=%v present=%v, want %v present=%v", got, present, tc.want, tc.present)
			}
			if _, present := detail.Metrics[lengthMatchedBaseline][key]; present {
				t.Fatal("baseline kept a stale cross-arm metric")
			}
		})
	}
}
