package bridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/contextsummary"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// TestMCPV0110ImpactBudgetBytesIsCLIPathImpactBudget covers MCPV0-110: a
// budgetBytes call returns the receipt the CLI's --budget-bytes compiles for
// the same index, paths and limit; omitting it keeps the unbudgeted receipt;
// a value outside the CLI's packet range, explicit null, a fraction or a
// string is invalid-arguments.
func TestMCPV0110ImpactBudgetBytesIsCLIPathImpactBudget(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native impact is qualified only on Darwin and Linux")
	}
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	budgeted, callErr := registry.Call(ctx, ToolImpact, []byte(`{"paths":["internal/widget/widget.go"],"limit":3,"budgetBytes":2048}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	assertObservedBinding(t, budgeted)
	assertCanonicalObject(t, budgeted)
	snapshot, _, bridgeErr := registry.planningSnapshot(ctx, nil, "")
	if bridgeErr != nil {
		t.Fatal(bridgeErr)
	}
	budget := 2048
	want, compileErr := contextindex.EvalImpactSyntax(snapshot.index, []string{"internal/widget/widget.go"}, 3, &budget)
	if compileErr != nil {
		t.Fatal(compileErr)
	}
	if got, expected := canonicalBytes(t, budgeted.Receipt), canonicalBytes(t, want); !bytes.Equal(got, expected) {
		t.Fatalf("budgeted MCP impact differs from the CLI budget receipt:\nmcp=%s\ncli=%s", got, expected)
	}
	if request, _ := budgeted.Receipt["request"].(map[string]any); request == nil || request["budget_bytes"] == nil {
		t.Fatalf("budgeted receipt does not record budget_bytes: %#v", budgeted.Receipt["request"])
	}
	plain, callErr := registry.Call(ctx, ToolImpact, []byte(`{"paths":["internal/widget/widget.go"],"limit":3}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	unbudgeted, compileErr := contextindex.ImpactSyntax(snapshot.index, []string{"internal/widget/widget.go"}, 3)
	if compileErr != nil {
		t.Fatal(compileErr)
	}
	if got, expected := canonicalBytes(t, plain.Receipt), canonicalBytes(t, unbudgeted); !bytes.Equal(got, expected) {
		t.Fatal("omitting budgetBytes changed the unbudgeted impact receipt")
	}
	for _, value := range []string{"1215", "1000001", "null", "2048.5", `"2048"`, "true"} {
		if _, callErr := registry.Call(ctx, ToolImpact, []byte(`{"paths":["internal/widget/widget.go"],"budgetBytes":`+value+`}`)); callErr == nil || callErr.Code != "invalid-arguments" {
			t.Fatalf("budgetBytes %s error = %#v", value, callErr)
		}
	}
}

// TestMCPV0111ContextSummaryBytesIsCLISummary covers MCPV0-111: a
// summaryBytes call returns the CLI's evidence summary of the canonical
// receipt plus LF, naming that receipt's digest, with the MCP continuation;
// out-of-range values are invalid-arguments, and a receipt the summary cannot
// read abstains with the summary's own reason.
func TestMCPV0111ContextSummaryBytesIsCLISummary(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native context is qualified only on Darwin and Linux")
	}
	registry, err := NewTaskReview(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	full, callErr := registry.Call(ctx, ToolContext, []byte(`{"task":"change widget Value","limit":5}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	summarized, callErr := registry.Call(ctx, ToolContext, []byte(`{"task":"change widget Value","limit":5,"summaryBytes":4096}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	assertObservedBinding(t, summarized)
	assertCanonicalObject(t, summarized)
	raw := append(canonicalBytes(t, full.Receipt), '\n')
	want, summaryErr := contextsummary.Summarize(raw, 4096, contextsummary.MCPContinuation)
	if summaryErr != nil {
		t.Fatal(summaryErr)
	}
	if got := canonicalBytes(t, summarized.Receipt); !bytes.Equal(got, want) {
		t.Fatalf("MCP summary differs from the CLI projection:\nmcp=%s\ncli=%s", got, want)
	}
	digest := sha256.Sum256(raw)
	summary, _ := summarized.Receipt["summary"].(map[string]any)
	if summary == nil || summary["view"] != contextsummary.View || summary["packet_sha256"] != hex.EncodeToString(digest[:]) ||
		summary["continuation"] != contextsummary.MCPContinuation || summarized.Receipt["revision"] != summarized.Repository.TreeRevision {
		t.Fatalf("summary members = %#v", summarized.Receipt)
	}
	for _, value := range []string{"1023", "65537", "null", "4096.5", `"4096"`} {
		if _, callErr := registry.Call(ctx, ToolContext, []byte(`{"task":"change widget Value","summaryBytes":`+value+`}`)); callErr == nil || callErr.Code != "invalid-arguments" {
			t.Fatalf("summaryBytes %s error = %#v", value, callErr)
		}
	}
	if _, shapeErr := summarizedContext(map[string]any{"tool": "impact"}, 4096); abstentionReason(shapeErr) != "SUMMARY_SHAPE" {
		t.Fatalf("non-context receipt summary error = %v", shapeErr)
	}
}

func canonicalBytes(t *testing.T, value map[string]any) []byte {
	t.Helper()
	encoded, err := gokernel.CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
