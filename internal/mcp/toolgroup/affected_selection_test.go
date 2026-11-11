// SPDX-License-Identifier: Apache-2.0
package toolgroup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/selectionswitch"
)

// selectionGroup is an affected group over a bare root whose planner counts
// its runs and whose environment holds value for CORVINT_SELECTION.
func selectionGroup(t *testing.T, value string) (Affected, *int) {
	t.Helper()
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding, ok := rootbind.Bind(root)
	if !ok {
		t.Fatal("bind")
	}
	runs := 0
	runner := func(_ context.Context, _, base string, stdout, _ io.Writer) int {
		runs++
		_, _ = fmt.Fprintf(stdout, "{\"base\":%q,\"profile\":\"affected-summary/1\"}\n", base)
		return 0
	}
	lookup := func(name string) (string, bool) {
		if name != selectionswitch.Variable {
			t.Fatalf("looked up %s", name)
		}
		return value, value != ""
	}
	return Affected{Root: root, Binding: binding, Run: runner, Lookup: lookup}, &runs
}

// MTT-V1-008: an unknown CORVINT_SELECTION value selects every test and
// carries the unknown-value reason naming the value; full and full:go select
// every test (all Go tests among them) with the kill-switch reason; none of
// them runs the planner, and an unset switch plans as before.
func TestMTTV1008AffectedToolReadsTheKillSwitch(t *testing.T) {
	for _, tc := range []struct {
		value, reason string
		runners       []any
	}{
		{"fastest", selectionswitch.ReasonUnknownValue, []any{}},
		{"full", selectionswitch.ReasonKillSwitch, []any{}},
		{"full:go", selectionswitch.ReasonKillSwitch, []any{"go"}},
	} {
		group, runs := selectionGroup(t, tc.value)
		result, failure := group.Call(context.Background(), AffectedTool, map[string]any{"base": "HEAD~1"})
		if failure != nil || result["isError"] != false {
			t.Fatalf("%s: %#v %#v", tc.value, result, failure)
		}
		structured, _ := result["structuredContent"].(map[string]any)
		selection, _ := structured["selection"].(map[string]any)
		if structured["profile"] != AffectedSelectAllProfile || structured["scope"] != "FULL" || structured["selectAll"] != true ||
			structured["reason"] != tc.reason || structured["base"] != "HEAD~1" || selection["value"] != tc.value || !reflect.DeepEqual(selection["runners"], tc.runners) {
			t.Fatalf("%s: %#v", tc.value, structured)
		}
		if hint, _ := structured["hint"].(string); hint == "" || strings.Contains(hint, "\n") {
			t.Fatalf("%s: hint %q is not one line", tc.value, hint)
		}
		if tc.value == "fastest" && !strings.Contains(structured["detail"].(string), `"fastest"`) {
			t.Fatalf("unknown value not named: %q", structured["detail"])
		}
		if *runs != 0 {
			t.Fatalf("%s ran the planner %d times", tc.value, *runs)
		}
	}
	group, runs := selectionGroup(t, "")
	result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
	if failure != nil || result["structuredContent"].(map[string]any)["profile"] != "affected-summary/1" || *runs != 1 {
		t.Fatalf("unset switch: %#v %#v runs=%d", result, failure, *runs)
	}
}
