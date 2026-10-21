package contextindex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandedRangeImpactPreservesDefaultSemantics(t *testing.T) {
	t.Parallel()
	t.Run("ERI-V0-004 default-semantic-bytes", func(t *testing.T) {
		for _, objectFormat := range []string{"sha1", "sha256"} {
			t.Run(objectFormat, func(t *testing.T) {
				root := impactRepository(t, objectFormat)
				base := commitRangeFixture(t, root)
				index, err := Build(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				for _, limit := range []int{1, 20, 50} {
					original, err := RangeImpact(context.Background(), index, base, limit)
					if err != nil {
						t.Fatal(err)
					}
					expanded, err := ExpandedRangeImpact(context.Background(), index, base, limit)
					if err != nil {
						t.Fatal(err)
					}
					if expanded["profile"] != expandedRangeImpactProfile || expanded["request"].(map[string]any)["rangeProfile"] != "expanded-256" {
						t.Fatalf("missing explicit experimental identity: %v", expanded)
					}
					expanded["profile"] = rangeImpactProfile
					delete(expanded["request"].(map[string]any), "rangeProfile")
					if err := stabilizePacketBytes(expanded); err != nil {
						t.Fatal(err)
					}
					want, _ := CanonicalJSON(original)
					got, _ := CanonicalJSON(expanded)
					if !bytes.Equal(got, want) {
						t.Fatalf("limit %d changed semantic bytes:\n%s\n%s", limit, want, got)
					}
				}
			})
		}
	})
}

func TestExpandedRangeImpactBoundaries(t *testing.T) {
	t.Parallel()
	for _, count := range []int{0, 1, 100, 101, 140, 256, 257} {
		t.Run(fmt.Sprintf("ERI-V0-001 %d-paths", count), func(t *testing.T) {
			root := impactRepository(t, "")
			base := testGit(t, root, "rev-parse", "HEAD")
			for index := 0; index < count; index++ {
				writeTestFile(t, root, fmt.Sprintf("notes/%03d.txt", index), fmt.Sprintf("range member %d\n", index))
			}
			if count != 0 {
				testGit(t, root, "add", ".")
				testGit(t, root, "commit", "-qm", "capacity boundary")
			}
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := ExpandedRangeImpact(context.Background(), index, base, 20)
			if count == 257 {
				assertRangeErrorCode(t, err, "unsupported-impact-range")
				if receipt != nil || !strings.Contains(err.Error(), "256-path bound") {
					t.Fatalf("oversized range returned receipt=%v error=%v", receipt, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if receipt["range"].(map[string]any)["changedPathCount"] != count || receipt["omissions"].(map[string]any)["count"] != count {
					t.Fatalf("incomplete non-Go membership: %v", receipt)
				}
			}
			_, err = RangeImpact(context.Background(), index, base, 20)
			if count > 100 {
				assertRangeErrorCode(t, err, "unsupported-impact-range")
				if !strings.Contains(err.Error(), "100-path bound") {
					t.Fatalf("default bound changed: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExpandedRangeImpactGlobalCoverage(t *testing.T) {
	t.Parallel()
	t.Run("ERI-V0-002 complete-membership-before-limit", func(t *testing.T) {
		root := impactRepository(t, "")
		base := testGit(t, root, "rev-parse", "HEAD")
		for index := 0; index < 102; index++ {
			writeTestFile(t, root, fmt.Sprintf("internal/capacity/item%03d.go", index), fmt.Sprintf("package capacity\n\nfunc Item%d() {}\n", index))
		}
		for index := 0; index < 38; index++ {
			writeTestFile(t, root, fmt.Sprintf("notes/%03d.txt", index), fmt.Sprintf("omitted native Go member %d\n", index))
		}
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "complete mixed range")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := ExpandedRangeImpact(context.Background(), index, base, 20)
		if err != nil {
			t.Fatal(err)
		}
		coverage := receipt["coverage"].(map[string]any)
		if receipt["state"] != "BUDGETED" || coverage["requested_results"] != 102 || coverage["included_results"] != 20 || coverage["omitted_results"] != 82 || len(anySlice(coverage["critical"])) != 102 || len(anySlice(coverage["critical_missing"])) != 82 {
			t.Fatalf("global coverage lost candidates: %v", coverage)
		}
		if receipt["range"].(map[string]any)["changedPathCount"] != 140 || receipt["omissions"].(map[string]any)["count"] != 38 {
			t.Fatalf("mixed membership lost paths: %v", receipt)
		}
		if !anyContains(anySlice(coverage["uncertainty"]), "38 non-Go changed paths") || !anyContains(anySlice(coverage["uncertainty"]), "82 ranked impact results") {
			t.Fatalf("omissions lost their uncertainty: %v", coverage)
		}
		verification := anySlice(receipt["verification"])
		if len(verification) != 2 || !anyStringIn(verification, "go test ./internal/capacity/...") || !anyStringIn(verification, "make gate") {
			t.Fatalf("verification is not globally deduplicated: %v", verification)
		}
	})
}

func TestExpandedRangeImpactGlobalAuthority(t *testing.T) {
	t.Parallel()
	t.Run("ERI-V0-002 whole-change-authority", func(t *testing.T) {
		root := impactRepository(t, "")
		base := testGit(t, root, "rev-parse", "HEAD")
		writeTestFile(t, root, "docs/adr/9999-self-minted.md", "# Self minted\n\nstatus: accepted\n")
		writeTestFile(t, root, "testing/scenarios.yaml", "scenarios:\n  - id: minted-token\n    area: auth\n    summary: Self-declared scenario.\n    status: shipped\n")
		writeTestFile(t, root, "internal/token/token.go", "package token\n\n// ADR-0012 ADR-9999 feature:stream-token scenario:minted-token\nfunc MintToken() string { return \"changed\" }\n")
		for index := 0; index < 110; index++ {
			writeTestFile(t, root, fmt.Sprintf("notes/%03d.txt", index), fmt.Sprintf("authority spacing %d\n", index))
		}
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "whole-change authority")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := ExpandedRangeImpact(context.Background(), index, base, 50)
		if err != nil {
			t.Fatal(err)
		}
		if stringIn(resultKeys(receipt), "decision:docs/adr/9999-self-minted.md") || !stringIn(resultKeys(receipt), "decision:docs/adr/0012-token.md") {
			t.Fatalf("whole-change decision authority changed: %v", resultKeys(receipt))
		}
		ledger := map[string]string{}
		for _, result := range mapsFromAny(receipt["results"]) {
			for _, raw := range anySlice(result["evidence"]) {
				item := raw.(map[string]any)
				if item["reason"] == "canonical ledger record selected by exact changed-hunk marker" {
					ledger[stringValue(result["kind"])+":"+stringValue(result["id"])] = stringValue(item["authority"]) + "/" + stringValue(item["confidence"])
				}
			}
		}
		if ledger["scenario:minted-token"] != UnverifiedLedgerAuthority+"/low" || ledger["feature:stream-token"] != "canonical-ledger/authoritative" {
			t.Fatalf("whole-change ledger authority changed: %v", ledger)
		}
		uncertainty := anySlice(receipt["coverage"].(map[string]any)["uncertainty"])
		if !anyContains(uncertainty, "ADR-9999 cited by a changed hunk is authored by the same change set") || !anyContains(uncertainty, "scenario:minted-token is declared by testing/scenarios.yaml, which the same change set authors") {
			t.Fatalf("withheld authority lost its reasons: %v", uncertainty)
		}
	})
}

func TestExpandedRangeImpactRejectsUnsupportedMembers(t *testing.T) {
	t.Parallel()
	t.Run("ERI-V0-003 unsupported-members-beyond-default-cap", func(t *testing.T) {
		for _, name := range []string{"binary", "delete", "mode", "unsafe", "excluded-go", "invalid-go"} {
			t.Run("ERI-V0-003 "+name, func(t *testing.T) {
				root := impactRepository(t, "")
				base := testGit(t, root, "rev-parse", "HEAD")
				for index := 0; index < 100; index++ {
					writeTestFile(t, root, fmt.Sprintf("notes/%03d.txt", index), fmt.Sprintf("supported member %d\n", index))
				}
				switch name {
				case "binary":
					writeTestFile(t, root, "z.bin", "\x00binary\n")
				case "delete":
					testGit(t, root, "rm", "internal/token/other.go")
				case "mode":
					if err := os.Chmod(filepath.Join(root, "internal/token/other.go"), 0o755); err != nil {
						t.Fatal(err)
					}
				case "unsafe":
					writeTestFile(t, root, "z unsafe.txt", "unsafe range member\n")
				case "excluded-go":
					writeTestFile(t, root, "vendor/new.go", "package vendor\n")
				case "invalid-go":
					writeTestFile(t, root, "internal/token/broken.go", "package token\nfunc broken(\n")
				}
				testGit(t, root, "add", "-A")
				testGit(t, root, "commit", "-qm", "unsupported member")
				index, err := Build(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				receipt, err := ExpandedRangeImpact(context.Background(), index, base, 20)
				assertRangeErrorCode(t, err, "unsupported-impact-range")
				if receipt != nil {
					t.Fatalf("unsupported member yielded partial receipt: %v", receipt)
				}
			})
		}
	})
}

func TestExpandedRangeImpactPreservesSnapshotRefusal(t *testing.T) {
	t.Parallel()
	t.Run("ERI-V0-005 snapshot-drift", func(t *testing.T) {
		root := impactRepository(t, "")
		base := commitRangeFixture(t, root)
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "notes.txt", "changed after capture\n")
		for _, compile := range []func(context.Context, *Index, string, int) (map[string]any, error){RangeImpact, ExpandedRangeImpact} {
			receipt, err := compile(context.Background(), index, base, 20)
			assertRangeErrorCode(t, err, "impact-range-drift")
			if receipt != nil {
				t.Fatal("snapshot drift yielded a receipt")
			}
		}
	})
}

// TestRangeImpactRefusesOverBoundBeforeCopyDetection pins ERI-V0-060
// (V1-1881): a range whose rename-free lower bound already exceeds the path
// bound refuses with the exact diff's error without running the copy-detecting
// diff, and every case the bound cannot decide still reaches the exact diff.
func TestRangeImpactRefusesOverBoundBeforeCopyDetection(t *testing.T) {
	type outcome struct {
		err        error
		exactCalls int
	}
	run := func(t *testing.T, index *Index, base string, expanded bool) outcome {
		t.Helper()
		calls := 0
		readRangeChangesFor = func(ctx context.Context, index *Index, baseTree string) ([]rangeChange, error) {
			calls++
			return readRangeChanges(ctx, index, baseTree)
		}
		t.Cleanup(func() { readRangeChangesFor = readRangeChanges })
		var err error
		if expanded {
			_, err = ExpandedRangeImpact(context.Background(), index, base, 20)
		} else {
			_, err = RangeImpact(context.Background(), index, base, 20)
		}
		readRangeChangesFor = readRangeChanges
		return outcome{err: err, exactCalls: calls}
	}
	exactError := func(t *testing.T, index *Index, base string, pathLimit int) error {
		t.Helper()
		baseTree := testGit(t, index.Root, "rev-parse", base+"^{tree}")
		changes, err := readRangeChanges(context.Background(), index, baseTree)
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) <= pathLimit {
			t.Fatalf("fixture range has %d exact paths, want more than %d", len(changes), pathLimit)
		}
		return rangePathBoundError(pathLimit)
	}
	commitAdded := func(t *testing.T, root string, count int, prefix string) {
		t.Helper()
		for item := 0; item < count; item++ {
			writeTestFile(t, root, fmt.Sprintf("%s/%03d.txt", prefix, item), fmt.Sprintf("range member %s %d\n", prefix, item))
		}
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "range "+prefix)
	}

	for _, objectFormat := range []string{"sha1", "sha256"} {
		t.Run("over-bound-additions/"+objectFormat, func(t *testing.T) {
			root := impactRepository(t, objectFormat)
			base := testGit(t, root, "rev-parse", "HEAD")
			commitAdded(t, root, 101, "notes")
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			got := run(t, index, base, false)
			want := exactError(t, index, base, maxImpactPaths)
			assertRangeErrorCode(t, got.err, "unsupported-impact-range")
			if got.err.Error() != want.Error() || got.exactCalls != 0 {
				t.Fatalf("over-bound range: error %v (want %v), exact diff calls %d (want 0)", got.err, want, got.exactCalls)
			}
		})
	}
	t.Run("expanded-profile-over-bound", func(t *testing.T) {
		root := impactRepository(t, "")
		base := testGit(t, root, "rev-parse", "HEAD")
		commitAdded(t, root, 257, "notes")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		got := run(t, index, base, true)
		want := exactError(t, index, base, maxExpandedRangePaths)
		if got.err == nil || got.err.Error() != want.Error() || got.exactCalls != 0 {
			t.Fatalf("expanded over-bound range: error %v (want %v), exact diff calls %d (want 0)", got.err, want, got.exactCalls)
		}
	})
	t.Run("renames-collapse-under-bound", func(t *testing.T) {
		root := impactRepository(t, "")
		commitAdded(t, root, 60, "before")
		base := testGit(t, root, "rev-parse", "HEAD")
		testGit(t, root, "mv", "before", "after")
		testGit(t, root, "commit", "-qm", "rename sixty")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		got := run(t, index, base, false)
		if got.exactCalls != 1 {
			t.Fatalf("120 rename-free entries collapsing to 60 renames skipped the exact diff: calls %d", got.exactCalls)
		}
		if got.err != nil && strings.Contains(got.err.Error(), "path bound") {
			t.Fatalf("in-bound rename range refused on the bound: %v", got.err)
		}
	})
	unsafeSource := strings.Repeat("an unchanged copy source line\n", 40)
	for _, copied := range []bool{false, true} {
		t.Run(fmt.Sprintf("unsafe-unchanged-base-path/copied=%t", copied), func(t *testing.T) {
			root := impactRepository(t, "")
			writeTestFile(t, root, "docs/has space.txt", unsafeSource)
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "unsafe base path")
			base := testGit(t, root, "rev-parse", "HEAD")
			if copied {
				writeTestFile(t, root, "notes/copy.txt", unsafeSource)
			}
			commitAdded(t, root, 101, "notes")
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			got := run(t, index, base, false)
			if !copied {
				if got.exactCalls != 0 || got.err == nil || got.err.Error() != exactError(t, index, base, maxImpactPaths).Error() {
					t.Fatalf("an unsafe base path no addition copies blocked the lower bound: error %v, exact diff calls %d", got.err, got.exactCalls)
				}
				return
			}
			// The exact diff names the unsafe copy source before it counts.
			if got.exactCalls != 1 || got.err == nil || !strings.Contains(got.err.Error(), "unsafe for verification planning") {
				t.Fatalf("a copied unsafe source did not defer to the exact diff: error %v, exact diff calls %d", got.err, got.exactCalls)
			}
		})
	}
}

// ERI-V0-060: a repository whose remote is a promisor only through
// remote.<name>.promisor or remote.<name>.partialclonefilter may lack the base
// blobs the exact diff reads. The over-bound shortcut must not replace the
// repository-object-unavailable refusal and its recovery with the bound refusal.
func TestRangeImpactOverBoundKeepsMissingObjectRefusalForPromisorRemotes(t *testing.T) {
	for _, key := range []string{"remote.origin.promisor", "remote.origin.partialclonefilter"} {
		t.Run(key, func(t *testing.T) {
			source := impactRepository(t, "")
			for item := 0; item < 101; item++ {
				writeTestFile(t, source, fmt.Sprintf("before/%03d.txt", item), fmt.Sprintf("base member %d\n%s", item, strings.Repeat("shared body line\n", 8)))
			}
			testGit(t, source, "add", ".")
			testGit(t, source, "commit", "-qm", "base members")
			base := testGit(t, source, "rev-parse", "HEAD")
			testGit(t, source, "rm", "-rq", "before")
			for item := 0; item < 101; item++ {
				writeTestFile(t, source, fmt.Sprintf("after/%03d.txt", item), fmt.Sprintf("head member %d\n%s", item, strings.Repeat("shared body line\n", 8)))
			}
			testGit(t, source, "add", ".")
			testGit(t, source, "commit", "-qm", "move members")
			testGit(t, source, "config", "uploadpack.allowFilter", "true")
			root := filepath.Join(t.TempDir(), "clone")
			testGit(t, t.TempDir(), "clone", "-q", "--filter=blob:none", "file://"+source, root)
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			// Keep only the named promisor key and make lazy fetches fail, so
			// the base blobs stay missing.
			testGitOptional(t, root, "config", "--unset", "extensions.partialclone")
			if key == "remote.origin.promisor" {
				testGit(t, root, "config", "--unset", "remote.origin.partialclonefilter")
			} else {
				testGit(t, root, "config", "--unset", "remote.origin.promisor")
			}
			testGit(t, root, "config", "remote.origin.url", "file://"+filepath.Join(t.TempDir(), "gone"))
			calls := 0
			readRangeChangesFor = func(ctx context.Context, index *Index, baseTree string) ([]rangeChange, error) {
				calls++
				return readRangeChanges(ctx, index, baseTree)
			}
			t.Cleanup(func() { readRangeChangesFor = readRangeChanges })
			_, err = RangeImpact(context.Background(), index, base, 20)
			var refusal *Error
			if calls != 1 || !errors.As(err, &refusal) || refusal.Code != "repository-object-unavailable" {
				t.Fatalf("promisor remote via %s: error %v, exact diff calls %d; want repository-object-unavailable from the exact diff", key, err, calls)
			}
		})
	}
}
