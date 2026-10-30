package dogfoodflow

import (
	"context"
	"io"
	"slices"
	"testing"
)

const (
	statusResourceLimit = `{"ok": false, "stableVerification": {"outcome": "UNSUPPORTED", "stage": "repository", "code": "unsupported-resource-limit"}}` + "\n"
	statusReject        = `{"ok": false, "stableVerification": {"outcome": "REJECT", "stage": "binding", "code": "hunk-mismatch"}}` + "\n"
	statusTimeout       = `{"ok": false, "stableVerification": {"outcome": "UNSUPPORTED", "stage": "repository", "code": "verification-timeout"}}` + "\n"
	statusAccept        = `{"ok": true, "stableVerification": {"outcome": "ACCEPT", "stage": "complete", "code": null}}` + "\n"
)

// scaledStatusRunner answers a cem status with first on the protocol argv and
// with scaled when --scaled-operations is present, recording every argv.
func scaledStatusRunner(calls *[][]string, first string, firstStatus int, scaled string) Runner {
	return Runner{Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
		*calls = append(*calls, slices.Clone(args))
		if slices.Contains(args, scaledOperationsFlag) {
			_, _ = io.WriteString(stdout, scaled)
			return 0
		}
		_, _ = io.WriteString(stdout, first)
		return firstStatus
	}}
}

// DCW-V0-373: the change flow runs cem status on the protocol argv first and
// reruns it once with --scaled-operations only after the stable verifier's
// unsupported-resource-limit refusal; any other result stands.
func TestDCWV0373StatusRetriesOnlyResourceLimit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		first       string
		firstStatus int
		calls       int
		want        string
	}{
		{"resource-limit", statusResourceLimit, 1, 2, "PRODUCED"},
		{"accept", statusAccept, 0, 1, "PRODUCED"},
		{"reject", statusReject, 1, 1, "NOT_PRODUCED"},
		{"timeout", statusTimeout, 1, 1, "NOT_PRODUCED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls [][]string
			evidence := t.TempDir()
			c := change{flow: flow{ctx: context.Background(), root: evidence}, evidence: evidence,
				options: ChangeOptions{Steps: scaledStatusRunner(&calls, tc.first, tc.firstStatus, statusAccept)}}
			args := statusArguments(evidence, "b", "t", 0, 0)
			c.runStatusStep(evidence+"/cem-status.json", args...)
			if len(calls) != tc.calls || slices.Contains(calls[0], scaledOperationsFlag) {
				t.Fatalf("calls=%q, want %d with the protocol argv first", calls, tc.calls)
			}
			if tc.calls == 2 && !slices.Equal(calls[1], append(slices.Clone(args), scaledOperationsFlag)) {
				t.Fatalf("retry argv=%q", calls[1])
			}
			if len(c.rows) != 1 || c.rows[0].name != "cem-status" || c.rows[0].status != tc.want {
				t.Fatalf("rows=%+v, want one cem-status %s", c.rows, tc.want)
			}
		})
	}
}

// DCW-V0-373: dogfood check reruns cem status with the scaled bound only when
// the base and tree verifiers agree on the protocol refusal, and both must
// agree on the scaled result: a base verifier without the flag fails closed.
func TestDCWV0373CheckScaledStatusNeedsBothVerifiers(t *testing.T) {
	refusesFlag := func(calls *[][]string) Runner {
		return Runner{Run: func(_ context.Context, _ string, args []string, stdout, stderr io.Writer) int {
			*calls = append(*calls, slices.Clone(args))
			if slices.Contains(args, scaledOperationsFlag) {
				_, _ = io.WriteString(stderr, "invalid-arguments\n")
				return 2
			}
			_, _ = io.WriteString(stdout, statusResourceLimit)
			return 1
		}}
	}
	for _, tc := range []struct {
		name      string
		baseFlag  bool
		agreed    bool
		outcome   string
		baseCalls int
	}{
		{"both-scaled", true, true, statusAccept, 2},
		{"base-without-flag", false, false, "", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var baseCalls, treeCalls [][]string
			base := scaledStatusRunner(&baseCalls, statusResourceLimit, 1, statusAccept)
			if !tc.baseFlag {
				base = refusesFlag(&baseCalls)
			}
			c := check{flow: flow{ctx: context.Background()}, options: CheckOptions{
				BaseVerifier: base, TreeVerifier: scaledStatusRunner(&treeCalls, statusResourceLimit, 1, statusAccept)}}
			result, agreed := c.verifyStatus(statusArguments(t.TempDir(), "b", "t", 0, 0)...)
			if agreed != tc.agreed || len(baseCalls) != tc.baseCalls || len(treeCalls) != 2 {
				t.Fatalf("agreed=%v base=%d tree=%d", agreed, len(baseCalls), len(treeCalls))
			}
			if tc.agreed && string(result.stdout) != tc.outcome {
				t.Fatalf("stdout=%q", result.stdout)
			}
		})
	}
}
