package contextindex

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestPerPathRangeImpactNamesEachRefusedPath pins ERI-V0-264 (V1-2796): the
// default profile refuses the whole range on its smallest failing path, and
// the per-path profile names each refused path with its rule while analysing
// the rest.
func TestPerPathRangeImpactNamesEachRefusedPath(t *testing.T) {
	t.Parallel()
	root := impactRepository(t, "")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "internal/token/token.go", "package token\n\n// feature:stream-token scenario:revoked-token\nfunc MintToken() string { return \"changed\" }\n")
	writeTestFile(t, root, "notes/a b.txt", "unsafe name\n")
	writeTestFile(t, root, "internal/v@2/v.go", "package v\n\nfunc Versioned() {}\n")
	writeTestFile(t, root, "pkg/@scope/c++.md", "admitted characters\n")
	testGit(t, root, "rm", "-q", "internal/auth/comment_only.go")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "mixed range")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	_, strict := RangeImpact(context.Background(), index, base, 50)
	assertRangeErrorCode(t, strict, "unsupported-impact-range")
	if strict == nil || !strings.HasSuffix(strict.Error(), `"notes/a b.txt"`) {
		t.Fatalf("strict refusal %v must name the unsafe path", strict)
	}

	receipt, err := PerPathRangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	rules := make(map[string]string)
	for _, row := range mapsFromAny(receipt["refusedPaths"]) {
		rules[stringValue(row["path"])] = stringValue(row["rule"])
		if !strings.Contains(stringValue(row["detail"]), stringValue(row["path"])) {
			t.Errorf("detail %q does not name %s", row["detail"], row["path"])
		}
	}
	want := map[string]string{
		"internal/auth/comment_only.go": "DELETED",
		"internal/v@2/v.go":             "GO_PACKAGE_DIRECTORY_AT",
		"notes/a b.txt":                 "UNSAFE_PATH",
	}
	if len(rules) != len(want) {
		t.Fatalf("refused paths %v, want %v", rules, want)
	}
	for refusedPath, rule := range want {
		if rules[refusedPath] != rule {
			t.Errorf("%s rule %q, want %q", refusedPath, rules[refusedPath], rule)
		}
	}
	coverage := receipt["coverage"].(map[string]any)
	if critical, ok := coverage["critical"].([]any); !ok || len(critical) != 1 || critical[0] != "path:internal/token/token.go" {
		t.Fatalf("critical %v, want only the admitted Go path", coverage["critical"])
	}
	omitted := false
	for _, row := range mapsFromAny(receipt["omissions"].(map[string]any)["samples"]) {
		omitted = omitted || row["path"] == "pkg/@scope/c++.md"
	}
	if !omitted {
		t.Fatalf("admitted '@'/'+' path is not carried as a non-Go omission: %v", receipt["omissions"])
	}
	uncertain := false
	for _, line := range coverage["uncertainty"].([]any) {
		uncertain = uncertain || strings.Contains(stringValue(line), "3 changed paths were refused per path")
	}
	if !uncertain {
		t.Fatalf("uncertainty does not name the refused paths: %v", receipt["uncertainty"])
	}
}

// TestRangeImpactRefusesGoPackageDirectoryWithAt pins ERI-V0-263: '@' is an
// admitted path character, but a Go package directory carrying it would turn
// the planned go test pattern into a version query, so the default profile
// refuses it by name.
func TestRangeImpactRefusesGoPackageDirectoryWithAt(t *testing.T) {
	t.Parallel()
	root := impactRepository(t, "")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "internal/v@2/v.go", "package v\n\nfunc Versioned() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "at directory")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, refusal := RangeImpact(context.Background(), index, base, 50)
	assertRangeErrorCode(t, refusal, "unsupported-impact-path")
	if refusal == nil || !strings.HasSuffix(refusal.Error(), "internal/v@2/v.go") {
		t.Fatalf("refusal %v must name the Go path", refusal)
	}
}

// TestPerPathRangeImpactKeepsBytesWithoutRefusals pins ERI-V0-264: with no
// refused path the per-path receipt is byte-identical to RangeImpact's.
func TestPerPathRangeImpactKeepsBytesWithoutRefusals(t *testing.T) {
	t.Parallel()
	root := impactRepository(t, "")
	base := commitRangeFixture(t, root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	strict, err := RangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	perPath, err := PerPathRangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := CanonicalJSON(strict)
	right, _ := CanonicalJSON(perPath)
	if string(left) != string(right) {
		t.Fatalf("per-path receipt bytes differ without refusals:\n%s\n%s", left, right)
	}
}

// TestPerPathRangeImpactRefusesPathsPastTheBoundEachAlone pins ERI-V0-285: a
// range over the 100-path bound keeps the first 100 changed paths in path
// order and names every later one PATH_BOUND, so one large commit does not
// hide the analysable part. RangeImpact still refuses the whole range.
func TestPerPathRangeImpactRefusesPathsPastTheBoundEachAlone(t *testing.T) {
	t.Parallel()
	root := impactRepository(t, "")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "internal/token/token.go", "package token\n\n// feature:stream-token scenario:revoked-token\nfunc MintToken() string { return \"changed\" }\n")
	for position := 0; position < maxImpactPaths; position++ {
		writeTestFile(t, root, fmt.Sprintf("zz/generated-%03d.txt", position), fmt.Sprintf("generated %d\n", position))
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "large range")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, strict := RangeImpact(context.Background(), index, base, 50)
	if strict == nil || !strings.Contains(strict.Error(), "100-path bound") {
		t.Fatalf("strict refusal %v, want the whole-range path bound", strict)
	}
	receipt, err := PerPathRangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	rows := mapsFromAny(receipt["refusedPaths"])
	if len(rows) != 1 || rows[0]["path"] != "zz/generated-099.txt" || rows[0]["rule"] != "PATH_BOUND" ||
		!strings.Contains(stringValue(rows[0]["detail"]), "100-path bound") {
		t.Fatalf("refused paths %v, want only the last path in path order as PATH_BOUND", rows)
	}
	coverage := receipt["coverage"].(map[string]any)
	if critical, ok := coverage["critical"].([]any); !ok || len(critical) != 1 || critical[0] != "path:internal/token/token.go" {
		t.Fatalf("critical %v, want the admitted Go path analysed", coverage["critical"])
	}
}

// TestPerPathRangeImpactRefusesAPathPastTheHunkBoundAlone pins ERI-V0-285: a
// path whose hunks would cross the range hunk bound is refused alone as
// HUNK_BOUND, and a later path that fits is still analysed.
func TestPerPathRangeImpactRefusesAPathPastTheHunkBoundAlone(t *testing.T) {
	t.Parallel()
	root := impactRepository(t, "")
	lines := 7 * (maxRangeHunks + 2)
	var before, after strings.Builder
	before.WriteString("package big\n")
	after.WriteString("package big\n")
	for line := 0; line < lines; line++ {
		before.WriteString("//\n")
		if line%7 == 0 {
			after.WriteString("// changed\n")
		} else {
			after.WriteString("//\n")
		}
	}
	writeTestFile(t, root, "internal/big/big.go", before.String())
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "big file")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "internal/big/big.go", after.String())
	writeTestFile(t, root, "internal/token/token.go", "package token\n\n// feature:stream-token scenario:revoked-token\nfunc MintToken() string { return \"changed\" }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "many hunks")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, strict := RangeImpact(context.Background(), index, base, 50)
	if strict == nil || !strings.Contains(strict.Error(), "hunk bound") {
		t.Fatalf("strict refusal %v, want the whole-range hunk bound", strict)
	}
	receipt, err := PerPathRangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	rows := mapsFromAny(receipt["refusedPaths"])
	if len(rows) != 1 || rows[0]["path"] != "internal/big/big.go" || rows[0]["rule"] != "HUNK_BOUND" {
		t.Fatalf("refused paths %v, want internal/big/big.go as HUNK_BOUND", rows)
	}
	coverage := receipt["coverage"].(map[string]any)
	if critical, ok := coverage["critical"].([]any); !ok || len(critical) != 1 || critical[0] != "path:internal/token/token.go" {
		t.Fatalf("critical %v, want the later fitting Go path analysed", coverage["critical"])
	}
}
