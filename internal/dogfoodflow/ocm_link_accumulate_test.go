package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"
)

// OCM-V0-940: a later plan row for an obligation an earlier row of the same
// scope linked runs with --accumulate, so a multi-file plan keeps every row.
func TestOCMV0940LinkPlanAccumulatesRepeatedObligation(t *testing.T) {
	var calls [][]string
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		calls = append(calls, append([]string(nil), args...))
		return 0
	}}
	var stderr bytes.Buffer
	c := &change{flow: flow{ctx: context.Background(), root: t.TempDir(), stderr: &stderr}, options: ChangeOptions{Steps: steps}}
	c.evidence = c.root
	c.linkPlan = []byte("intent.md\tREQ-001\t1\ta_test.go\t1\n" +
		"other.md\tREQ-001\t1\tc_test.go\t1\n" +
		"intent.md\tREQ-002\t1\ta_test.go\t2\n" +
		"intent.md\tREQ-001\t1\tb_test.go\t1\n")
	c.runOCMLinks(".corvint/intent.ocm.json", "intent.md")
	if len(calls) != 3 {
		t.Fatalf("calls = %v", calls)
	}
	for index, want := range []bool{false, false, true} {
		if got := slices.Contains(calls[index], "--accumulate"); got != want {
			t.Fatalf("row %d accumulate = %v, want %v: %s", index, got, want, strings.Join(calls[index], " "))
		}
	}
}
