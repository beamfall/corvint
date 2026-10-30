package gokernel

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// V1-1143 documents the current limitation, not a new authority contract. A
// repository without CEM/OCM artifacts still receives successful event receipts;
// the frontier degradation neither diagnoses missing artifacts nor blocks Stop.
func TestFrontierUnavailableWithoutAdopterArtifacts(t *testing.T) {
	root := testRepository(t)
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
		t.Fatalf("fixture must start without Corvint artifacts: %v", err)
	}
	before := runGit(t, root, "status", "--porcelain=v1", "--untracked-files=all")
	for _, tc := range []struct{ event, input string }{
		{"session-start", `{}`},
		{"user-prompt", `{"task":"inspect source"}`},
		{"file-change", `{"paths":["src/main.go"]}`},
		{"post-tool", `{"changedPaths":["src/main.go"]}`},
		{"stop", `{}`},
		{"session-end", `{}`},
	} {
		t.Run(tc.event, func(t *testing.T) {
			req := request(root, tc.event, tc.input)
			// Retrieval is outside this test's scope; supply its successful seam
			// so index-backed events reach receipt construction too.
			called := false
			req.IndexedContext = func(context.Context, string, string, map[string]any, int) (map[string]any, error) {
				called = true
				return map[string]any{}, nil
			}
			result, err := HandleEvent(req)
			if err != nil {
				t.Fatal(err)
			}
			if called != (tc.event == "user-prompt" || tc.event == "file-change") {
				t.Fatalf("unexpected indexed-context invocation: %v", called)
			}
			if result["ok"] != true || result["support"] != "FALLBACK" || result["mutates"] != false {
				t.Fatalf("missing artifacts must not fail the event: %#v", result)
			}
			codes, ok := result["degradations"].([]any)
			if !ok || !slices.Contains(codes, any("frontier-authority-unavailable")) {
				t.Fatalf("expected string degradation, got %#v", result["degradations"])
			}
			if tc.event == "stop" {
				frontier := result["frontier"].(map[string]any)
				if frontier["state"] != "UNAVAILABLE" || frontier["shouldContinue"] != false || frontier["reason"] != "frontier-authority-unavailable" {
					t.Fatalf("unavailable authority must not claim closure or block Stop: %#v", frontier)
				}
			}
		})
	}
	if after := runGit(t, root, "status", "--porcelain=v1", "--untracked-files=all"); after != before {
		t.Fatalf("event reads changed the repository: before=%q after=%q", before, after)
	}
}
