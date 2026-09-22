package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerificationBudget(t *testing.T) {
	t.Run("CEM-GO-003 verification budget is a finite 30 minute hang detector", func(t *testing.T) {
		if verificationBudget != 30*time.Minute {
			t.Fatalf("verification budget=%v", verificationBudget)
		}
	})
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "cem-0.1", "patches", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNormativePatchFixtures(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"supported.patch", "topology.patch", "whitespace.patch", "line-ending.patch", "bytes.patch"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := parsePatch(fixture(t, name)); err != nil {
				t.Fatalf("parsePatch: %v", err)
			}
		})
	}
}

func TestNormativeInvalidPatchFixtures(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"invalid-surplus.patch", "invalid-header-create.patch", "invalid-header-delete.patch", "invalid-special-mode.patch", "invalid-mode-mismatch.patch"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := parsePatch(fixture(t, name)); err == nil {
				t.Fatal("invalid patch accepted")
			}
		})
	}
}

func TestStrictJSON(t *testing.T) {
	t.Parallel()
	var dst map[string]any
	for _, raw := range [][]byte{
		[]byte(`{"a":1,"a":2}`),
		[]byte(`{"a":"\ud800"}`),
		{0xff},
	} {
		if err := strictDecode(raw, &dst); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestWireIntegers(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{"start":1.0,"count":2e0}`, `{"start":0e1000000,"count":-0}`} {
		var r lineRange
		if err := strictDecode([]byte(raw), &r); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{`{"start":null,"count":0}`, `{"start":true,"count":0}`, `{"start":1.5,"count":0}`, `{"start":9007199254740992,"count":0}`} {
		var r lineRange
		if err := strictDecode([]byte(raw), &r); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestLiteralDiffPathContainingSeparator(t *testing.T) {
	t.Parallel()
	patch := []byte("diff --git a/docs/new b/x.txt b/docs/new b/x.txt\n--- a/docs/new b/x.txt\n+++ b/docs/new b/x.txt\n@@ -1 +1 @@\n-old\n+new\n")
	if _, err := parsePatch(patch); err != nil {
		t.Fatal(err)
	}
}

func TestSimilarityMetadataIsExact(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"similarity index junk 50%", "similarity index  50%", "dissimilarity index 50% 60%"} {
		patch := []byte("diff --git a/x b/y\n" + line + "\nrename from x\nrename to y\n--- a/x\n+++ b/y\n@@ -1 +1 @@\n-a\n+b\n")
		if _, err := parsePatch(patch); err == nil {
			t.Errorf("accepted %q", line)
		}
	}
}

func TestBinaryMarkerTextInHunkPayloadIsText(t *testing.T) {
	t.Parallel()
	payload := []byte("--- a/x\n+++ b/x\n@@ -1 +1 @@\n-Binary files a/x and b/x differ\n+GIT binary patch\n")
	if _, err := parsePatch(payload); err != nil {
		t.Fatalf("payload text rejected: %v", err)
	}
	for _, marker := range []string{"Binary files a/x and b/x differ", "GIT binary patch"} {
		if _, err := parsePatch([]byte("diff --git a/x b/x\n" + marker + "\n")); err == nil || err.code != "patch" {
			t.Errorf("%q: err=%v", marker, err)
		}
	}
}

func TestSimulateEmptyBase(t *testing.T) {
	t.Parallel()
	patch := []byte("--- a/empty.txt\n+++ b/empty.txt\n@@ -0,0 +1 @@\n+new\n")
	p, err := parsePatch(patch)
	if err != nil {
		t.Fatal(err)
	}
	out, simErr := simulate(context.Background(), nil, p.hunks)
	if simErr != nil || string(out) != "new\n" {
		t.Fatalf("out=%q err=%v", out, simErr)
	}
}

func TestExplicitEmptyTargetRejected(t *testing.T) {
	t.Parallel()
	_, err := parseArgs([]string{"verify", "--repository", "r", "--map", "m", "--patch", "p", "--target", ""})
	if err == nil || !err.operational {
		t.Fatalf("got %v", err)
	}
}

func TestStrictJSONSurrogateAfterEscapes(t *testing.T) {
	t.Parallel()
	var dst map[string]any
	for _, raw := range [][]byte{
		[]byte(`{"a":"escaped quote: \" then \ud800"}`),
		[]byte(`{"a":"backslashes: \\\\ then \udfff"}`),
		[]byte(`{"a":"valid pair first: \ud83d\ude00 then bad: \ud800"}`),
	} {
		if err := strictDecode(raw, &dst); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	for _, raw := range [][]byte{
		[]byte(`{"a":"literal replacement: �"}`),
		[]byte(`{"a":"escaped slash-u: \\ud800"}`),
		[]byte(`{"a":"valid pair: \ud83d\ude00"}`),
	} {
		if err := strictDecode(raw, &dst); err != nil {
			t.Fatalf("rejected %q: %v", raw, err)
		}
	}
}

func TestCanonicalIdentityUnicode(t *testing.T) {
	t.Parallel()
	e := evidence{
		BlobOID:    strings.Repeat("a", 40),
		Path:       "src/café.txt",
		Span:       span{Start: 0, End: 3},
		SpanSHA256: strings.Repeat("b", 64),
	}
	b := canonicalEvidence(e)
	if !bytes.Contains(b, []byte("café")) || bytes.Contains(b, []byte(`\u00e9`)) || bytes.HasSuffix(b, []byte("\n")) {
		t.Fatalf("non-canonical bytes: %q", b)
	}
}

func TestLFRecordOverflow(t *testing.T) {
	t.Parallel()
	b := bytes.Repeat([]byte("x\n"), maxRecords+1)
	if _, err := parsePatch(b); err == nil || err.code != "too-many-lines" {
		t.Fatalf("got %v", err)
	}
}

func TestOverlappingMatches(t *testing.T) {
	t.Parallel()
	first, count, err := firstTwoMatches(context.Background(), []byte("aaaaa"), []byte("aaaa"))
	if err != nil || first != 0 || count != 2 {
		t.Fatalf("first=%d count=%d err=%v", first, count, err)
	}
}

func TestEndToEndVerify(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.name", "CEM Test")
	gitRun(t, dir, "config", "user.email", "cem@example.invalid")
	base := []byte("Rule: value must be old.\n")
	if err := os.WriteFile(filepath.Join(dir, "rule.txt"), base, 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "rule.txt")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	baseOID := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	blobOID := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD:rule.txt"))
	patch := []byte("--- a/rule.txt\n+++ b/rule.txt\n@@ -1 +1 @@\n-Rule: value must be old.\n+Rule: value must be new.\n")
	parsed, parseErr := parsePatch(patch)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	e := evidence{Path: "rule.txt", BlobOID: blobOID, Span: span{Start: 6, End: 23}}
	e.SpanSHA256 = shaHex(base[e.Span.Start:e.Span.End])
	e.ID = "evidence:sha256:" + shaHex(canonicalEvidence(e))
	ph := parsed.hunks[0]
	m := cemMap{
		Spec: specVersion, BaseRevision: baseOID, PatchSHA256: shaHex(patch), Evidence: []evidence{e},
		Hunks: []mappedHunk{{ID: ph.ID, Path: "rule.txt", OldRange: ph.OldRange, NewRange: ph.NewRange,
			Disposition: "supported", Reason: "evidence-backed", Basis: []basis{{EvidenceID: e.ID, Relation: "specification"}}}},
	}
	mapBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	mapFile, patchFile := filepath.Join(dir, "map.json"), filepath.Join(dir, "change.patch")
	if err := os.WriteFile(mapFile, mapBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patchFile, patch, 0o600); err != nil {
		t.Fatal(err)
	}
	drift, verifyErr := verify(cliArgs{repository: dir, mapFile: mapFile, patchFile: patchFile})
	if verifyErr != nil || len(drift) != 0 {
		t.Fatalf("verify drift=%v err=%v", drift, verifyErr)
	}
	shared := filepath.Join(t.TempDir(), "shared")
	gitRun(t, dir, "clone", "-q", "--shared", ".", shared)
	if _, err := verify(cliArgs{repository: shared, mapFile: mapFile, patchFile: patchFile}); err == nil || !err.operational || err.code != "object-alternates" {
		t.Fatalf("CEM-GO-005 alternates repository: err=%v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	v := &verifier{ctx: ctx, repo: dir, blobs: map[string][]byte{}}
	items, driftErr := v.computeDrift(baseOID, []evidence{e})
	if driftErr != nil || len(items) != 1 || items[0].Status != "stable" {
		t.Fatalf("drift=%v err=%v", items, driftErr)
	}
}

func TestTargetSymlinkWithIdenticalBlobIsStable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.name", "CEM Test")
	gitRun(t, dir, "config", "user.email", "cem@example.invalid")
	content := []byte("target")
	path := filepath.Join(dir, "evidence")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "evidence")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	blobOID := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD:evidence"))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", path); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "evidence")
	gitRun(t, dir, "commit", "-q", "-m", "symlink")
	target := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v := &verifier{ctx: ctx, repo: dir, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	e := evidence{ID: "e", Path: "evidence", BlobOID: blobOID, Span: span{Start: 0, End: uint64(len(content))}, data: content}
	items, err := v.computeDrift(target, []evidence{e})
	if err != nil || len(items) != 1 || items[0].Status != "stable" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func TestGitReadsAreBatched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.name", "CEM Test")
	gitRun(t, dir, "config", "user.email", "cem@example.invalid")
	paths := make([]string, 400)
	for i := range paths {
		paths[i] = fmt.Sprintf("evidence-%03d", i)
		if err := os.WriteFile(filepath.Join(dir, paths[i]), []byte(fmt.Sprintf("%03d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, dir, "add", "--all")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	commit := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	v := &verifier{ctx: ctx, repo: dir, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	if err := v.preloadTrees(commit, paths); err != nil {
		t.Fatal(err)
	}
	oids := make([]string, 0, len(paths))
	for _, path := range paths {
		entry, _ := v.treeEntry(commit, path)
		oids = append(oids, entry.oid)
	}
	if err := v.preloadBlobs(oids); err != nil {
		t.Fatal(err)
	}
	if v.gitOps > 3 || len(v.blobs) != len(paths) {
		t.Fatalf("gitOps=%d blobs=%d", v.gitOps, len(v.blobs))
	}
}

func TestDeadlineChecksAndMissingObjectClassification(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := shaHexContext(ctx, []byte("data")); err == nil || !err.operational {
		t.Fatalf("hash err=%v", err)
	}
	if _, err := simulate(ctx, nil, nil); err == nil || !err.operational {
		t.Fatalf("simulate err=%v", err)
	}

	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	v := &verifier{ctx: ctx2, repo: dir, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	err := v.preloadBlobs([]string{strings.Repeat("0", 40)})
	if err == nil || !err.operational || err.code != "repository-io" {
		t.Fatalf("missing object err=%v", err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, b)
	}
	return string(b)
}

// TestDissimilarityWithoutRenameParses follows decision 0192: ALGORITHMS.md
// step 2 admits one similarity or dissimilarity line in any state-machine group.
func TestDissimilarityWithoutRenameParses(t *testing.T) {
	t.Parallel()
	patch := []byte("diff --git a/x b/x\ndissimilarity index 90%\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n")
	if _, err := parsePatch(patch); err != nil {
		t.Fatalf("rejected: %v", err)
	}
}

// TestInconsistentNewPositionRejectedAtParse keeps the parser-section rule in
// the parser, so it cannot depend on base resolution succeeding first.
func TestInconsistentNewPositionRejectedAtParse(t *testing.T) {
	t.Parallel()
	for _, patch := range []string{
		"--- a/x\n+++ b/x\n@@ -1 +5 @@\n-a\n+b\n",
		"diff --git a/x b/x\ndeleted file mode 100644\n--- a/x\n+++ /dev/null\n@@ -1 +0,0 @@\n-a\n@@ -2 +1,0 @@\n-b\n",
	} {
		if _, err := parsePatch([]byte(patch)); err == nil {
			t.Errorf("accepted %q", patch)
		}
	}
	twoHunkDelete := "diff --git a/x b/x\ndeleted file mode 100644\n--- a/x\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n@@ -3,2 +0,0 @@\n-c\n-d\n"
	if _, err := parsePatch([]byte(twoHunkDelete)); err != nil {
		t.Errorf("two-hunk delete rejected: %v", err)
	}
}
