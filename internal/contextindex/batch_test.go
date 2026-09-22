package contextindex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// possessionIndex is the loaded-snapshot fixture SBQ-V0-008 verdicts against:
// one clean admitted source, one dirty admitted source, one tracked path the
// index never admitted, and one directory visible only as a tracked prefix.
func possessionIndex() *Index {
	return &Index{
		Revision: "tree", CommitRevision: "commit", StatusSHA256: "status",
		Skipped: map[string]struct{}{"modules/only/sub": {}},
		Tracked: map[string]struct{}{
			"cache/demux.go": {}, "cache/dirty.go": {}, "script/run": {}, "docs/specs/note.md": {},
		},
		Sources: map[string]Source{
			"cache/demux.go":     {Path: "cache/demux.go", BlobHash: "aaa"},
			"cache/dirty.go":     {Path: "cache/dirty.go", BlobHash: "bbb"},
			"docs/specs/note.md": {Path: "docs/specs/note.md", BlobHash: "ccc"},
		},
		DirtyPaths: []string{"cache/dirty.go"},
	}
}

func TestPossessionVerdictPrecedence_SBQ008(t *testing.T) {
	// SBQ-V0-008(a)-(f): one verdict per entry, matched by literal path with no
	// normalization, case folding or Git process.
	index := possessionIndex()
	for _, test := range []struct{ name, path, hash, want string }{
		{"gitlink", "modules/only/sub", "aaa", VerdictUnframable},
		{"submodule-only ancestor", "modules/only", "aaa", VerdictUnframable},
		{"directory", "docs/specs", "aaa", VerdictUnframable},
		{"tracked but never admitted", "script/run", "aaa", VerdictUnsupported},
		{"dirty outranks a matching hash", "cache/dirty.go", "bbb", VerdictDirty},
		{"forged hash", "cache/demux.go", "zzz", VerdictStale},
		{"equal hash", "cache/demux.go", "aaa", VerdictRetained},
		{"prefix lookalike", "docs/spec", "aaa", VerdictAbsent},
		{"absolute", "/cache/demux.go", "aaa", VerdictAbsent},
		{"parent escape", "../cache/demux.go", "aaa", VerdictAbsent},
		{"case fold", "Cache/Demux.go", "aaa", VerdictAbsent},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := PossessionVerdict(index, PossessedEntry{Path: test.path, BlobHash: test.hash})
			if got != test.want {
				t.Fatalf("verdict(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestPossessionRefusesTablelessIndex_SBQ008h(t *testing.T) {
	// SBQ-V0-008(h): a table-less index would silence (b) and send every path
	// to (f), so the operation fails closed instead of verdicting.
	index := possessionIndex()
	index.Tracked = nil
	_, err := ApplyPossession(index, map[string]any{"results": []any{}}, []PossessedEntry{{Path: "a", BlobHash: "b"}})
	contextError, ok := err.(*Error)
	if !ok || contextError.Code != "unsupported-possession-tables" {
		t.Fatalf("err = %v", err)
	}
	if _, control := ApplyPossession(possessionIndex(), possessionReceipt(), nil); control != nil {
		t.Fatalf("control index refused: %v", control)
	}
}

// possessionReceipt is a query-shaped receipt over the fixture: one result
// whose single row is possessible, one two-row result, and one critical result.
func possessionReceipt() map[string]any {
	return map[string]any{
		"state": "READY", "request": map[string]any{"limit": 10},
		"results": []any{
			map[string]any{"kind": "path", "id": "cache/demux.go", "evidence": []any{
				map[string]any{"path": "cache/demux.go", "blob_hash": "aaa"},
			}},
			map[string]any{"kind": "path", "id": "docs/specs/note.md", "evidence": []any{
				map[string]any{"path": "docs/specs/note.md", "blob_hash": "ccc"},
				map[string]any{"path": "cache/demux.go", "blob_hash": "aaa"},
			}},
			map[string]any{"kind": "path", "id": "cache/critical.go", "evidence": []any{
				map[string]any{"path": "cache/demux.go", "blob_hash": "aaa"},
			}},
		},
		"verification": []any{},
		"coverage": map[string]any{
			"requested_results": 3, "included_results": 3, "omitted_results": 0,
			"critical": []any{"path:cache/critical.go"}, "critical_missing": []any{},
			"budget_bytes": nil, "packet_bytes": 0, "within_budget": true, "uncertainty": []any{},
		},
	}
}

func TestSuppressionAndInvalidationMatching_SBQ009(t *testing.T) {
	// SBQ-V0-009(a)-(f): an exact pair suppresses, a path-only mismatch stays
	// in the packet and invalidates, a critical result is never suppressed, and
	// a partly matched result survives whole.
	index := possessionIndex()
	index.DirtyPaths = nil
	receipt := possessionReceipt()
	outcome, err := ApplyPossession(index, receipt, []PossessedEntry{
		{Path: "cache/demux.go", BlobHash: "aaa"},
		{Path: "docs/specs/note.md", BlobHash: "forged"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Fallback != "" {
		t.Fatalf("fallback = %q", outcome.Fallback)
	}
	if len(outcome.Suppressed) != 1 || outcome.Suppressed[0].ID != "cache/demux.go" || outcome.Suppressed[0].Rows != 1 {
		t.Fatalf("suppressed = %+v", outcome.Suppressed)
	}
	if len(outcome.Invalidated) != 1 || outcome.Invalidated[0].ID != "docs/specs/note.md" ||
		outcome.Invalidated[0].Path != "docs/specs/note.md" || outcome.Invalidated[0].Verdict != VerdictStale {
		t.Fatalf("invalidated = %+v", outcome.Invalidated)
	}
	if kept := mapsFromAny(receipt["results"]); len(kept) != 2 ||
		stringValue(kept[0]["id"]) != "docs/specs/note.md" || stringValue(kept[1]["id"]) != "cache/critical.go" {
		t.Fatalf("results = %+v", kept)
	}
}

func TestSuppressionSkipsEmptyHashAndEmptyEvidence_SBQ009cd(t *testing.T) {
	// SBQ-V0-009(c),(d): a row with an empty blob_hash (an unguarded Sources
	// miss) matches nothing, and a result with no evidence row is not
	// suppressible.
	receipt := map[string]any{
		"state": "READY",
		"results": []any{
			map[string]any{"kind": "path", "id": "hashless", "evidence": []any{
				map[string]any{"path": "cache/demux.go", "blob_hash": ""},
			}},
			map[string]any{"kind": "path", "id": "rowless", "evidence": []any{}},
		},
		"coverage": map[string]any{"critical": []any{}, "uncertainty": []any{}, "packet_bytes": 0},
	}
	clean := possessionIndex()
	clean.DirtyPaths = nil
	outcome, err := ApplyPossession(clean, receipt, []PossessedEntry{{Path: "cache/demux.go", BlobHash: "aaa"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Suppressed) != 0 || len(outcome.Invalidated) != 0 {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestPossessionFallbackReasons_SBQ010d(t *testing.T) {
	// SBQ-V0-010(d): a whole-operation fallback disables suppression and echoes
	// every entry, and unsupported-possession wins over mixed-worktree.
	possessed := []PossessedEntry{{Path: "cache/demux.go", BlobHash: "aaa"}, {Path: "script/run", BlobHash: "ddd"}}
	for _, test := range []struct {
		name  string
		build func() (*Index, map[string]any, []PossessedEntry)
		want  string
	}{
		{"budget compacted", func() (*Index, map[string]any, []PossessedEntry) {
			receipt := possessionReceipt()
			receipt["request"] = map[string]any{"budget_bytes": 4096, "omitted_by_budget": 2}
			return possessionIndex(), receipt, possessed[:1]
		}, FallbackBudgetCompacted},
		{"mixed worktree", func() (*Index, map[string]any, []PossessedEntry) {
			return possessionIndex(), possessionReceipt(), possessed[:1]
		}, FallbackMixedWorktree},
		{"unsupported possession wins", func() (*Index, map[string]any, []PossessedEntry) {
			receipt := possessionReceipt()
			receipt["results"] = append(anySlice(receipt["results"]), map[string]any{
				"kind": "path", "id": "script/run", "evidence": []any{
					map[string]any{"path": "script/run", "blob_hash": "ddd"},
				},
			})
			return possessionIndex(), receipt, possessed
		}, FallbackUnsupportedPossession},
	} {
		t.Run(test.name, func(t *testing.T) {
			index, receipt, entries := test.build()
			before := len(mapsFromAny(receipt["results"]))
			outcome, err := ApplyPossession(index, receipt, entries)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Fallback != test.want {
				t.Fatalf("fallback = %q, want %q", outcome.Fallback, test.want)
			}
			if len(outcome.Ignored) != len(entries) || len(outcome.Suppressed) != 0 {
				t.Fatalf("outcome = %+v", outcome)
			}
			coverage := receipt["coverage"].(map[string]any)
			if _, present := coverage["suppressed_results"]; present {
				t.Fatalf("fallback operation gained suppressed_results: %+v", coverage)
			}
			if after := len(mapsFromAny(receipt["results"])); after != before {
				t.Fatalf("results = %d, want %d", after, before)
			}
		})
	}
}

func TestSuppressionFreezesCoverageCounts_SBQ010b(t *testing.T) {
	// SBQ-V0-010(b): every coverage count and `state` freeze at their
	// pre-suppression values; suppressed_results alone carries the difference
	// and uncertainty gains exactly one appended line.
	index := possessionIndex()
	index.DirtyPaths = nil
	receipt := possessionReceipt()
	receipt["coverage"].(map[string]any)["critical"] = []any{}
	outcome, err := ApplyPossession(index, receipt, []PossessedEntry{
		{Path: "cache/demux.go", BlobHash: "aaa"},
		{Path: "docs/specs/note.md", BlobHash: "ccc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Suppressed) != 3 {
		t.Fatalf("suppressed = %+v", outcome.Suppressed)
	}
	coverage := receipt["coverage"].(map[string]any)
	if coverage["included_results"] != 3 || coverage["requested_results"] != 3 || coverage["omitted_results"] != 0 {
		t.Fatalf("counts moved: %+v", coverage)
	}
	if receipt["state"] != "READY" {
		t.Fatalf("state = %v", receipt["state"])
	}
	if coverage["suppressed_results"] != 3 {
		t.Fatalf("suppressed_results = %v", coverage["suppressed_results"])
	}
	lines := anySlice(coverage["uncertainty"])
	if len(lines) != 1 || lines[0] != "3 results suppressed by caller possession" {
		t.Fatalf("uncertainty = %+v", lines)
	}
	if len(anySlice(receipt["results"])) != 0 {
		t.Fatalf("results = %+v", receipt["results"])
	}
}

func TestSuppressionRestabilizesWithinBudget_SBQ010c(t *testing.T) {
	// SBQ-V0-010(c): within_budget is reported truthfully after suppression,
	// never assumed to stay true, and packet_bytes reaches its fixed point over
	// the narrowed receipt.
	index := possessionIndex()
	index.DirtyPaths = nil
	receipt := possessionReceipt()
	coverage := receipt["coverage"].(map[string]any)
	coverage["critical"] = []any{}
	coverage["budget_bytes"] = 10
	if _, err := ApplyPossession(index, receipt, []PossessedEntry{
		{Path: "cache/demux.go", BlobHash: "aaa"}, {Path: "docs/specs/note.md", BlobHash: "ccc"},
	}); err != nil {
		t.Fatal(err)
	}
	if coverage["within_budget"] != false {
		t.Fatalf("within_budget = %v over a 10-byte budget", coverage["within_budget"])
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if coverage["packet_bytes"] != len(encoded) {
		t.Fatalf("packet_bytes = %v, encoded = %d", coverage["packet_bytes"], len(encoded))
	}
}

func TestStabilizeReceiptRefusesMistypedCoverage_SBQ010c(t *testing.T) {
	// SBQ-V0-010(c): the exported wrapper returns a contextindex.Error where
	// stabilizePacketBytes' own unchecked assertion would panic.
	for _, test := range []struct {
		name    string
		receipt map[string]any
	}{
		{"absent", map[string]any{}},
		{"scalar", map[string]any{"coverage": 3}},
		{"null", map[string]any{"coverage": nil}},
		{"typed nil map", map[string]any{"coverage": map[string]any(nil)}},
		{"array", map[string]any{"coverage": []any{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := StabilizeReceipt(test.receipt)
			if _, ok := err.(*Error); !ok {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestInvalidatedSortsByKindIDPath_SBQ010f(t *testing.T) {
	// SBQ-V0-010(f): delta.invalidated is the one list exempt from operation
	// order and sorts by (kind, id, path) alone.
	entries := []InvalidatedResult{
		{Kind: "path", ID: "b", Path: "z"},
		{Kind: "learned-path", ID: "a", Path: "y"},
		{Kind: "path", ID: "b", Path: "a"},
	}
	SortInvalidated(entries)
	if entries[0].Kind != "learned-path" || entries[1].Path != "a" || entries[2].Path != "z" {
		t.Fatalf("order = %+v", entries)
	}
}

func TestPossessionSkippedTreeRoundTrip_SBQ008(t *testing.T) {
	root := testRepository(t)
	writeTestFile(t, root, ".gitignore", ".corvint/\n")
	writeTestFile(t, root, "script/run", "#!/bin/sh\nexit 0\n")
	writeTestFile(t, root, "docs/specs/generated.md", "<!-- Code generated by test; DO NOT EDIT. -->\n")
	writeTestFile(t, root, "docs/specs/note.md", "# Link target\n")
	if err := os.Symlink("note.md", filepath.Join(root, "docs/specs/link.md")); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", ".")
	commit := testGit(t, root, "rev-parse", "HEAD")
	testGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+commit+",modules/only/sub")
	testGit(t, root, "commit", "-qm", "possession paths")
	_, err := Build(context.Background(), root)
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "repository-probe-failed" {
		t.Fatalf("live gitlink index lost private status refusal: %v", err)
	}
	// EAF-V0-007 refuses live index gitlinks. A staged removal retains the
	// committed non-blob tree evidence without requiring submodule traversal.
	testGit(t, root, "update-index", "--force-remove", "modules/only/sub")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := index.Tracked["modules/only/sub"]; ok {
		t.Fatal("gitlink became tracked blob")
	}
	if _, ok := index.Skipped["modules/only/sub"]; !ok {
		t.Fatal("gitlink not recorded")
	}
	for _, exclusion := range index.Exclusions {
		if exclusion.Path == "script/run" {
			t.Fatal("suffix-only drop gained exclusion")
		}
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	loaded, hit, err := LoadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("load=%v err=%v", hit, err)
	}
	linkHash := testGit(t, root, "rev-parse", "HEAD:docs/specs/link.md")
	loaded.DirtyPaths = append(loaded.DirtyPaths, "modules/only/sub", "script/run", "docs/specs/generated.md")
	for _, entry := range []struct{ path, hash, want string }{
		{"modules/only/sub", commit, VerdictUnframable},
		{"modules/only", commit, VerdictUnframable},
		{"modules", commit, VerdictUnframable},
		{"script/run", "forged", VerdictUnsupported},
		{"docs/specs/generated.md", "forged", VerdictUnsupported},
		{"docs/specs/link.md", linkHash, VerdictRetained},
		{"docs/specs/link.md", "forged", VerdictStale},
	} {
		if got := PossessionVerdict(loaded, PossessedEntry{Path: entry.path, BlobHash: entry.hash}); got != entry.want {
			t.Fatalf("%s=%s want %s", entry.path, got, entry.want)
		}
	}
}

func TestPossessionPerVerbAccounting_SBQ010abc(t *testing.T) {
	for _, verb := range []string{"query", "impact", "context"} {
		for _, all := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/all=%v", verb, all), func(t *testing.T) {
				index := possessionIndex()
				index.DirtyPaths = nil
				receipt := possessionReceipt()
				receipt["mode"] = verb
				receipt["exclusions"] = map[string]any{"count": 1, "samples": []any{"unchanged"}}
				coverage := receipt["coverage"].(map[string]any)
				coverage["uncertainty"] = []any{"2 results omitted by ranking"}
				if verb == "context" {
					for _, key := range []string{"requested_results", "budget_bytes", "packet_bytes", "within_budget", "uncertainty"} {
						delete(coverage, key)
					}
					coverage["candidates"] = 5
					coverage["governance"] = 1
					coverage["unexamined"] = 2
					coverage["budget_shortage"] = false
					coverage["critical"] = []any{map[string]any{"relation": "path", "path": "cache/critical.go"}}
					delete(receipt, "verification")
					delete(receipt, "exclusions")
				}
				if all {
					coverage["critical"] = []any{}
				}
				frozen := make(map[string]any)
				for k, v := range coverage {
					frozen[k] = v
				}
				outcome, err := ApplyPossession(index, receipt, []PossessedEntry{{Path: "cache/demux.go", BlobHash: "aaa"}, {Path: "docs/specs/note.md", BlobHash: "ccc"}})
				if err != nil {
					t.Fatal(err)
				}
				want := 2
				if all {
					want = 3
				}
				if len(outcome.Suppressed) != want || coverage["suppressed_results"] != want || receipt["state"] != "READY" {
					t.Fatalf("outcome=%+v receipt=%v", outcome, receipt)
				}
				for key, before := range frozen {
					if key == "packet_bytes" || key == "within_budget" || key == "uncertainty" {
						continue
					}
					if !reflect.DeepEqual(before, coverage[key]) {
						t.Fatalf("%s changed", key)
					}
				}
				lines := anySlice(coverage["uncertainty"])
				if verb == "context" {
					if len(lines) != 1 {
						t.Fatalf("context lines=%v", lines)
					}
					for _, key := range []string{"packet_bytes", "within_budget", "budget_bytes"} {
						if _, ok := coverage[key]; ok {
							t.Fatalf("context gained %s", key)
						}
					}
				} else {
					if len(lines) != 2 || lines[0] != "2 results omitted by ranking" {
						t.Fatalf("lines=%v", lines)
					}
					if coverage["budget_bytes"] != nil || coverage["within_budget"] != true {
						t.Fatalf("unbounded coverage=%v", coverage)
					}
					bytes, err := CanonicalJSON(receipt)
					if err != nil {
						t.Fatal(err)
					}
					if len(bytes) != coverage["packet_bytes"] {
						t.Fatalf("packet bytes=%v actual=%d", coverage["packet_bytes"], len(bytes))
					}
					if !reflect.DeepEqual(receipt["exclusions"], map[string]any{"count": 1, "samples": []any{"unchanged"}}) {
						t.Fatal("exclusions changed")
					}
				}
				if lines[len(lines)-1] != fmt.Sprintf("%d results suppressed by caller possession", want) {
					t.Fatal(lines)
				}
			})
		}
	}
}

func TestPossessionVerificationRetainedPathsFirst_SBQ010b(t *testing.T) {
	rows := func(prefix string, n int) []map[string]any {
		result := make([]map[string]any, n)
		for i := range result {
			result[i] = map[string]any{"evidence": []any{map[string]any{"path": fmt.Sprintf("%s/%02d.go", prefix, i)}}}
		}
		return result
	}
	survivors, suppressed := rows("z", 2), rows("a", 25)
	paths := possessionVerificationPaths(survivors, suppressed)
	if len(paths) != 20 || paths[0] != "z/00.go" || paths[1] != "z/01.go" || paths[19] != "a/17.go" {
		t.Fatal(paths)
	}
}

func TestPossessionMultipleHashesAndInvalidationRows_SBQ009(t *testing.T) {
	index := possessionIndex()
	index.DirtyPaths = nil
	tests := []struct {
		name                    string
		entries                 []PossessedEntry
		suppressed, invalidated int
	}{
		{"any equal hash wins", []PossessedEntry{{"cache/demux.go", "forged"}, {"cache/demux.go", "aaa"}, {"docs/specs/note.md", "ccc"}}, 2, 0},
		{"two paths invalidate one result", []PossessedEntry{{"cache/demux.go", "forged"}, {"cache/demux.go", "other"}, {"docs/specs/note.md", "forged"}}, 0, 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ApplyPossession(index, possessionReceipt(), test.entries)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Suppressed) != test.suppressed || len(result.Invalidated) != test.invalidated {
				t.Fatalf("outcome=%+v", result)
			}
			if test.invalidated != 0 {
				count := 0
				for _, item := range result.Invalidated {
					if item.ID == "docs/specs/note.md" {
						count++
					}
				}
				if count != 2 {
					t.Fatalf("two-row result invalidations=%d", count)
				}
			}
		})
	}
	receipt := possessionReceipt()
	receipt["results"] = []any{map[string]any{"kind": "path", "id": "script/run", "evidence": []any{map[string]any{"path": "script/run", "blob_hash": ""}}}}
	result, err := ApplyPossession(index, receipt, []PossessedEntry{{"script/run", "h"}})
	if err != nil || result.Fallback != FallbackUnsupportedPossession || len(result.Ignored) != 1 {
		t.Fatalf("hashless unsupported fallback=%+v err=%v", result, err)
	}
	result, err = ApplyPossession(index, possessionReceipt(), []PossessedEntry{{"script/run", "h"}, {"absent", "h"}, {"modules/only/sub", "h"}})
	if err != nil || result.Fallback != "" || len(result.Ignored) != 3 {
		t.Fatalf("unmatched paths=%+v err=%v", result, err)
	}
}

func TestPossessionSnapshotEngineIdentity_SBQ010f(t *testing.T) {
	// Engine memoization is process-local. Model a second binary's already
	// computed identity while keeping the repository/tree and payload fixed.
	root := testRepository(t)
	writeTestFile(t, root, ".gitignore", ".corvint/\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "ignore snapshots")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	original := LoadedEngineID()
	defer func() { engineDigest = original }()
	var loaded []*Index
	var identities []string
	for _, engineID := range []string{original, "0000000000000001"} {
		engineDigest = engineID
		if _, err := WriteSnapshot(index); err != nil {
			t.Fatal(err)
		}
		snapshot, hit, err := LoadSnapshot(context.Background(), root)
		if err != nil || !hit {
			t.Fatalf("engine %s load=%v err=%v", engineID, hit, err)
		}
		loaded = append(loaded, snapshot)
		identities = append(identities, LoadedEngineID())
	}
	if !reflect.DeepEqual(loaded[0], loaded[1]) || identities[0] == identities[1] {
		t.Fatalf("same payload did not distinguish engine: %v", identities)
	}
}
