package golang_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// consumerUnknowns returns the plan's UNSELECTED_BINARY_CONSUMER entries.
func consumerUnknowns(plan affected.Plan) []affected.Unknown {
	var found []affected.Unknown
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownUnselectedBinaryConsumer {
			found = append(found, unknown)
		}
	}
	return found
}

// AFP-V0-762 (V1-1017): a declared consumer file that runs a command's built
// binary, such as a Node e2e script no plugin selects as a test, is named as
// an explicit unknown when the change reaches the command's build, and not
// for a test-only change of the command or a change elsewhere.
func TestDeclaredBinaryConsumerIsAnExplicitUnknown_AFPV0762(t *testing.T) {
	root := binaryExecRepository(t, `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["cmd/tool"]}}`)
	writeFiles(t, root, map[string]string{
		"web/test/e2e.mjs":           "// runs the built tool binary\n",
		affected.BinaryConsumersPath: `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/test/e2e.mjs":["cmd/tool"]}}`,
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("frontier = %v", result.Frontier)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"lib/lib.go", "cmd/tool/main.go"} {
		plan := affected.Select(graph, []string{dirty})
		got := consumerUnknowns(plan)
		if len(got) != 1 || got[0].Detail != "web/test/e2e.mjs" || got[0].Hint == "" || plan.Scope != affected.ScopeUnknown {
			t.Fatalf("%s: consumer unknowns = %+v scope %s, want web/test/e2e.mjs UNKNOWN", dirty, got, plan.Scope)
		}
		if _, ok := selected(plan, binaryExecModule+"runner"); !ok {
			t.Errorf("%s: declared package runner no longer selected", dirty)
		}
	}
	for _, dirty := range []string{"cmd/tool/main_test.go", "unrelated/unrelated.go"} {
		if got := consumerUnknowns(affected.Select(graph, []string{dirty})); len(got) != 0 {
			t.Errorf("%s: consumer unknowns = %+v, want none", dirty, got)
		}
	}
}

// AFP-V0-762: a consumer declaration that names no command, or is malformed,
// declares no consumer and raises the declaration frontier, while the packages
// declaration keeps its own edges.
func TestInvalidBinaryConsumerDeclaresNothing_AFPV0762(t *testing.T) {
	for name, declaration := range map[string]string{
		"not a command":    `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/e2e.mjs":["lib"]}}`,
		"empty entries":    `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/e2e.mjs":[]}}`,
		"escaping file":    `{"profile":"corvint-test-binary-consumers/0","consumers":{"../e2e.mjs":["cmd/tool"]}}`,
		"unsorted entries": `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/e2e.mjs":["lib","cmd/tool"]}}`,
		"wrong profile":    `{"profile":"corvint-test-binary-execs/0","consumers":{"web/e2e.mjs":["cmd/tool"]}}`,
		"unknown member":   `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/e2e.mjs":["cmd/tool"]},"x":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := binaryExecRepository(t, `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["cmd/tool"]}}`)
			writeFiles(t, root, map[string]string{affected.BinaryConsumersPath: declaration})
			result, err := golang.New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Frontier, []string{golang.FrontierBinaryExecsInvalid}) {
				t.Fatalf("frontier = %v, want %s", result.Frontier, golang.FrontierBinaryExecsInvalid)
			}
			for _, unit := range result.Units {
				if unit.ExecConsumers != nil {
					t.Errorf("%s keeps declared consumers %v", unit.ID, unit.ExecConsumers)
				}
				if unit.ID == binaryExecModule+"runner" && unit.Execs == nil {
					t.Errorf("the packages declaration lost its edge")
				}
			}
		})
	}
}

// AFP-V0-762 (Codex review): a change to both the command's dependency and its own test file
// still rebuilds the command, so the declared consumer is named although the command's
// witness is its direct test change.
func TestMixedChangeStillNamesTheBinaryConsumer_AFPV0762(t *testing.T) {
	root := binaryExecRepository(t, `{"profile":"corvint-test-binary-execs/0","packages":{"runner":["cmd/tool"]}}`)
	writeFiles(t, root, map[string]string{
		"web/test/e2e.mjs":           "// runs the built tool binary\n",
		affected.BinaryConsumersPath: `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/test/e2e.mjs":["cmd/tool"]}}`,
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range [][]string{{"lib/lib.go", "cmd/tool/main_test.go"}, {"cmd/tool/main.go", "cmd/tool/main_test.go"}} {
		if got := consumerUnknowns(affected.Select(graph, dirty)); len(got) != 1 || got[0].Detail != "web/test/e2e.mjs" {
			t.Errorf("%v: consumer unknowns = %+v, want web/test/e2e.mjs", dirty, got)
		}
	}
}
