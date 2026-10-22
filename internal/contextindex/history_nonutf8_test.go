package contextindex

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestGPKV0072QueryLearningSkipsNonUTF8HistoryPaths commits a Latin-1 path
// (V1-0313). Query learning must answer from the commit's other paths instead
// of refusing the whole repository with unsupported-query-history, and the
// skipped path reaches no candidate.
func TestGPKV0072QueryLearningSkipsNonUTF8HistoryPaths(t *testing.T) {
	t.Parallel()
	root := nonUTF8Repository(t, true)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvalQuery(context.Background(), index, "fix MintToken in the token package", 10, nil)
	if err != nil {
		t.Fatalf("GPK-V0-072: query refused a repository whose history names a non-UTF-8 path: %v", err)
	}
	// The index exclusion still discloses the path; no result row names it.
	encoded, err := json.Marshal(result["results"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "latin/caf") {
		t.Fatalf("GPK-V0-072: the skipped path reached a result: %s", encoded)
	}
	learning, ok := result["learning"].(map[string]any)
	if !ok || learning["history_digest"] == nil || learning["history_commits_considered"] != 2 {
		t.Fatalf("GPK-V0-072: query did not learn from both commits: %v", result["learning"])
	}
}
