//go:build corvint_development

package contextindex

import (
	"context"
	"fmt"
	"testing"
)

// EAF-V0-006: an opt-in development qualification screen, not a released contract.
// Its current padding failure remains a promotion blocker; default gates do not run it.
func TestUncertaintyPaddingDevelopmentScreen(t *testing.T) {
	t.Run("EAF-V0-006", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_TERMS", "")
		t.Setenv("CORVINT_CONTEXT_FRAME_RELATION", "")
		task := "Does `httpRetry` honour `maxAttempts` and `backoffBase` inside the `retry_loop`?"
		for _, scenario := range []struct {
			name               string
			padding            int
			positive, neighbor bool
		}{
			{"baseline", 43, false, false},
			{"unrelated-test", 42, false, true},
			{"unrelated-files", 44, false, false},
			{"genuine-support", 43, true, false},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				files := map[string]string{
					"go.mod":               "module example.test/retry\n\ngo 1.27.0\n",
					"net/loop.go":          "package net\n// retry inside the loop\nfunc loop() {}\n",
					"unrelated/catalog.go": "package unrelated\n// External log field: httpRetry\nfunc Catalog() {}\n",
					"unrelated/schema.go":  "package unrelated\n// External log field: httpRetry\nfunc Schema() {}\n",
				}
				if scenario.neighbor {
					files["net/loop_test.go"] = "package net\nfunc TestLoop() { loop() }\n"
				}
				if scenario.positive {
					files["net/loop.go"] = "package net\nfunc retry_loop() { httpRetry(maxAttempts, backoffBase) }\n"
				}
				for i := 0; i < scenario.padding; i++ {
					files[fmt.Sprintf("padding/p%02d.go", i)] = fmt.Sprintf("package padding\nconst Unrelated%d = 1\n", i)
				}
				index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
				if err != nil {
					t.Fatal(err)
				}
				compiler := newTaskContextCompiler(index, task, "")
				candidates := compiler.compile(20)
				packet, err := TaskContext(context.Background(), index, task, "", 20)
				if err != nil {
					t.Fatal(err)
				}
				answer := packet["coverage"].(map[string]any)["answerability"].(map[string]any)
				wantState, wantVerdict := "NO_CANDIDATES", "unsupported-conjunction"
				if scenario.positive {
					wantState, wantVerdict = "READY", "supported"
				}
				if packet["state"] != wantState || answer["verdict"] != wantVerdict {
					t.Errorf("development screen %s: state=%v verdict=%v known=%v; want %s/%s", scenario.name, packet["state"], answer["verdict"], answer["known"], wantState, wantVerdict)
				}
				if !scenario.positive && relationRows(candidates) != 0 {
					t.Errorf("unrelated fixture acquired a rescue relation")
				}
				if scenario.positive && (answer["support"] != 3 || answer["supporting_sources"] != 1) {
					t.Errorf("genuine evidence did not supply support: %v", answer)
				}
				t.Logf("sources=%d rescueRelations=%d candidates=%d state=%v answer=%v", len(index.vocabulary().Paths), relationRows(candidates), len(candidates), packet["state"], answer)
			})
		}
	})
}
