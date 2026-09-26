package contextindex

import (
	"context"
	"strings"
	"testing"
)

// evalFloorQuery runs one query against the shared evaluation repository and
// returns the packet state, the number of rendered results, and the abstention
// reason.
func evalFloorQuery(t *testing.T, index *Index, text string) (string, int, string) {
	t.Helper()
	packet, err := EvalQuery(context.Background(), index, text, 10, nil)
	if err != nil {
		t.Fatalf("EvalQuery(%q): %v", text, err)
	}
	results := mapsFromAny(packet["results"])
	reason := ""
	if abstention, ok := packet["abstention"].(map[string]any); ok {
		reason = stringValue(abstention["reason"])
	}
	return stringValue(packet["state"]), len(results), reason
}

// TestEvalQueryRelevanceFloor pins the floor in both directions: a task the
// repository cannot answer is withdrawn even though ranking produced results
// for it, and a task it can answer is untouched.
func TestEvalQueryRelevanceFloor(t *testing.T) {
	index, err := Build(context.Background(), evalQueryRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name, text, state string
		results           string
		reason            string
	}{{
		// "expired" matches the session-revocation summary and nothing else
		// does. One word of six is the background rate of English in a code
		// repository, not evidence that this repository answers the task.
		name: "one-word accident abstains", text: "renew my expired passport before the embassy appointment",
		state: "OUT_OF_SCOPE", results: "empty", reason: "below-relevance-floor",
	}, {
		name: "unrelated task abstains", text: "how do I bake sourdough bread at high altitude",
		state: "OUT_OF_SCOPE", results: "empty",
	}, {
		name: "supported task answers", text: "session expiry device revocation enforcement",
		state: "READY", results: "present", reason: "none",
	}, {
		// A query offering fewer than two words to match is held to what it
		// offers, so a bare identifier lookup still answers.
		name: "single-word query answers", text: "revocation",
		state: "READY", results: "present", reason: "none",
	}} {
		t.Run(testCase.name, func(t *testing.T) {
			state, count, reason := evalFloorQuery(t, index, testCase.text)
			if state != testCase.state {
				t.Fatalf("state = %q, want %q (results=%d reason=%q)", state, testCase.state, count, reason)
			}
			if testCase.results == "empty" && count != 0 {
				t.Fatalf("results = %d, want 0", count)
			}
			if testCase.results == "present" && count == 0 {
				t.Fatal("results = 0, want at least one")
			}
			if testCase.reason != "" && reason != testCase.reason {
				t.Fatalf("abstention reason = %q, want %q", reason, testCase.reason)
			}
		})
	}
}

// TestEvalStrongestSupportCountsQueryWords pins the two properties the floor
// depends on: support is counted in words of the query as written rather than
// in the stems terms() derives from them, and a result carried into the packet
// behind another result contributes no support of its own.
func TestEvalStrongestSupportCountsQueryWords(t *testing.T) {
	ordered := evalOrderedTerms("autumn leaves rendering parity")
	supportIndex := map[string]map[string]struct{}{
		"decision:docs/adr/0001.md": intersectionSet([]string{"leaves", "leave"}),
		"feature:parity-render":     intersectionSet([]string{"rendering", "parity"}),
	}
	stemOnly := []map[string]any{{"kind": "decision", "id": "docs/adr/0001.md"}}
	if support := evalStrongestSupport(stemOnly, supportIndex, ordered); support != 1 {
		t.Fatalf("stem variants counted as %d words, want 1", support)
	}
	carried := []map[string]any{{"kind": "symbol", "id": "internal/render/render.go:Draw"}}
	if support := evalStrongestSupport(carried, supportIndex, ordered); support != 0 {
		t.Fatalf("unscored carried result counted as %d words, want 0", support)
	}
	twoWords := []map[string]any{{"kind": "feature", "id": "parity-render"}}
	if support := evalStrongestSupport(twoWords, supportIndex, ordered); support != 2 {
		t.Fatalf("two matched words counted as %d, want 2", support)
	}
}

// GPK-V0-039, GPK-V0-044.
func TestEvalQueryLearnedPathCannotLiftRelevanceFloorAbstention(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/learnedfloor\n\ngo 1.27.0\n")
	writeTestFile(t, root, "internal/dashboard/authority/types.go", "package authority\n\ntype TraceCode string\n")
	path := "internal/trace/codec.go"
	writeTestFile(t, root, path, "package trace\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	query := "trace codec zebra quokka"
	absent, err := NewQueryTraceSnapshot("absent", nil)
	if err != nil {
		t.Fatal(err)
	}
	state, count, reason, _ := evalFloorQueryWithSnapshot(t, index, query, absent)
	if state != "OUT_OF_SCOPE" || count != 0 || reason != "below-relevance-floor" {
		t.Fatalf("without trace: state=%q results=%d reason=%q", state, count, reason)
	}
	ready, err := NewQueryTraceSnapshot("ready", []QueryTrace{{
		TraceID: strings.Repeat("a", 64), Task: "trace codec zebra quokka cleanup", Outcome: "passed", ChangedPaths: []string{path},
	}})
	if err != nil {
		t.Fatal(err)
	}
	state, count, reason, matched := evalFloorQueryWithSnapshot(t, index, query, ready)
	if matched != 1 {
		t.Fatalf("matched local traces=%d, want 1", matched)
	}
	if state != "OUT_OF_SCOPE" || count != 0 || reason != "below-relevance-floor" {
		t.Fatalf("with trace: state=%q results=%d reason=%q", state, count, reason)
	}
}

func evalFloorQueryWithSnapshot(t *testing.T, index *Index, text string, snapshot QueryTraceSnapshot) (string, int, string, int) {
	t.Helper()
	packet, err := EvalQuery(context.Background(), index, text, 10, nil, snapshot)
	if err != nil {
		t.Fatalf("EvalQuery(%q): %v", text, err)
	}
	results := mapsFromAny(packet["results"])
	reason := ""
	if abstention, ok := packet["abstention"].(map[string]any); ok {
		reason = stringValue(abstention["reason"])
	}
	learning := packet["learning"].(map[string]any)
	matched, _ := learning["matched_local_traces"].(int)
	return stringValue(packet["state"]), len(results), reason, matched
}

// GPK-V0-039, GPK-V0-040. The floor is judged on the ranking's emitted packet,
// before the packet budget narrows it. A budget that drops the only row resting
// on two query words ships the surviving one-word rows as BUDGETED with the
// omission counted, never as a below-relevance-floor withdrawal: the task is
// answerable, and the byte budget is the cause the receipt already names.
func TestEvalQueryRelevanceFloorPrecedesPacketBudget(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/floorbudget\n\ngo 1.27.0\n")
	writeTestFile(t, root, "internal/a/s.go", "package a\n\nfunc Revocation() bool { return true }\n")
	writeTestFile(t, root, "docs/adr/0001-session.md", "# Session\n\nStatus: accepted\n\nSession.\n")
	// The long path makes the two-word row the one a small budget cannot fit.
	supporting := "internal/" + strings.Repeat("deep", 60) + "/revoke.go"
	writeTestFile(t, root, supporting, "package deep\n\nfunc SessionRevocation() bool { return true }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	budget := 2000
	packet, err := EvalQuery(context.Background(), index, "session revocation", 10, &budget)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range mapsFromAny(packet["results"]) {
		if strings.HasPrefix(stringValue(result["id"]), supporting) {
			t.Fatalf("budget %d kept the supporting row; the fixture no longer exercises the ordering", budget)
		}
	}
	abstention := packet["abstention"].(map[string]any)
	omitted, _ := packet["coverage"].(map[string]any)["omitted_results"].(int)
	if packet["state"] != "BUDGETED" || len(mapsFromAny(packet["results"])) == 0 || omitted == 0 || abstention["reason"] != "none" {
		t.Fatalf("state=%v results=%d omitted=%d abstention=%v, want BUDGETED rows with the omission counted",
			packet["state"], len(mapsFromAny(packet["results"])), omitted, abstention)
	}

	// The surviving rows alone fail the floor: without the supporting file the
	// same query is withdrawn.
	testGit(t, root, "rm", "-q", supporting)
	testGit(t, root, "commit", "-qm", "drop the supporting row")
	unsupported, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if state, count, reason := evalFloorQuery(t, unsupported, "session revocation"); state != "OUT_OF_SCOPE" || count != 0 || reason != "below-relevance-floor" {
		t.Fatalf("without the supporting row: state=%q results=%d reason=%q", state, count, reason)
	}
}

// GPK-V0-066. Records outrank symbols only as answers to the task. Two
// features that each match one query word fill a limit-1 packet that fails the
// floor, while a symbol resting on four query words was never admitted; the
// packet is compiled from the confident symbols instead of being withdrawn.
// Without that symbol the same query is still withdrawn.
func TestEvalQueryUnsupportedRecordsYieldToSupportedSymbols(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/recordfloor\n\ngo 1.27.0\n")
	writeTestFile(t, root, "testing/features.yaml", "features:\n"+
		"  - id: plugin-music\n    area: plugins\n    summary: Music library.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: plugin-podcasts\n    area: plugins\n    summary: Podcast feeds.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios: []\n")
	supporting := "internal/plugin/trust.go"
	writeTestFile(t, root, supporting, "package plugin\n\ntype PluginTrustRoots struct{}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	query := "validate plugin trust roots at model loader construction"

	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := EvalQuery(context.Background(), index, query, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	abstention := packet["abstention"].(map[string]any)
	if len(results) != 1 || results[0]["id"] != supporting+":PluginTrustRoots" || packet["state"] != "READY" || abstention["reason"] != "none" {
		t.Fatalf("state=%v abstention=%v results=%v, want the supporting symbol READY", packet["state"], abstention, results)
	}

	testGit(t, root, "rm", "-q", supporting)
	testGit(t, root, "commit", "-qm", "drop the supporting symbol")
	unsupported, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if state, count, reason := evalFloorQuery(t, unsupported, query); state != "OUT_OF_SCOPE" || count != 0 || reason != "below-relevance-floor" {
		t.Fatalf("without the supporting symbol: state=%q results=%d reason=%q", state, count, reason)
	}
}

// GPK-V0-068. A limit-1 packet that emits one feature while the limit omits a
// competitive feature resting on a query word the emitted one does not rest on
// has chosen between two readings of the task by score alone, so it asks for
// widening instead of claiming READY. Once the limit admits both, the packet is
// READY again.
func TestEvalQueryLimitOmittingCompetingRecordNeedsWidening(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitignore", ".context-corvint/\n")
	writeTestFile(t, root, "go.mod", "module example.test/competing\n\ngo 1.27.0\n")
	writeTestFile(t, root, "testing/features.yaml", "features:\n"+
		"  - id: access-request-grant\n    area: auth\n    summary: Per-title access requests and grants.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: hls-transcode\n    area: playback\n    summary: HLS transcode ladder.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "testing/scenarios.yaml", "scenarios: []\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	query := "carry hls priority in the signed stream grant instead of the request header"

	for _, tc := range []struct {
		limit          int
		state, reason  string
		active         bool
		wantResultsLen int
	}{
		{limit: 1, state: "NEEDS_WIDENING", reason: "omitted-competing-record", active: true, wantResultsLen: 1},
		{limit: 2, state: "READY", reason: "none", active: false, wantResultsLen: 2},
	} {
		packet, err := EvalQuery(context.Background(), index, query, tc.limit, nil)
		if err != nil {
			t.Fatal(err)
		}
		results := mapsFromAny(packet["results"])
		abstention := packet["abstention"].(map[string]any)
		if len(results) != tc.wantResultsLen || results[0]["id"] != "access-request-grant" || packet["state"] != tc.state || abstention["reason"] != tc.reason || abstention["active"] != tc.active {
			t.Fatalf("limit %d: state=%v abstention=%v results=%v, want %s/%s", tc.limit, packet["state"], abstention, results, tc.state, tc.reason)
		}
	}
}
