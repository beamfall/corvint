package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
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

func candidatePacket(t *testing.T) (string, map[string]string, []byte) {
	t.Helper()
	packet, err := filepath.Abs(filepath.Join("..", "..", "protocol", "cem-1.0"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(packet, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if shaHex(raw) != "18de09b6d6ede40d34d973381372f97a5cf490a006da8a23312021f16d3282e5" {
		t.Fatal("candidate manifest changed")
	}
	var manifest struct{ ArtifactSHA256 map[string]string }
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for path, digest := range manifest.ArtifactSHA256 {
		b, e := os.ReadFile(filepath.Join(packet, path))
		if e != nil || shaHex(b) != digest {
			t.Fatalf("packet digest %s: %v", path, e)
		}
	}
	return packet, manifest.ArtifactSHA256, raw
}
func TestCandidateNormativePacket(t *testing.T) {
	packet, _, manifestRaw := candidatePacket(t)
	var manifest struct {
		Author, Timestamp, BaseMessage, ContentMessage, TargetMessage string
		Repositories                                                  []struct{ Name, ObjectFormat, BaseRevision, ContentRevision, TargetRevision, Map string }
		Cases                                                         []struct {
			Name, Repository, Map, OmitArtifact, ChangeArtifact, Integrity, Target, ArtifactDirectory string
			Exit                                                                                      int
		}
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "consumer")
	cmd := exec.Command("go", "build", "-o", exe, ".")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	type binding struct{ repo, base, target, content string }
	repos := map[string]binding{}
	for _, r := range manifest.Repositories {
		repo := t.TempDir()
		run := func(args ...string) string {
			t.Helper()
			cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
			cmd.Env = append(cleanGitEnvironment(), append([]string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}, commitEnv(t, manifest.Author, manifest.Timestamp)...)...)
			b, e := cmd.Output()
			if e != nil {
				t.Fatalf("git %v: %v", args, e)
			}
			return strings.TrimSpace(string(b))
		}
		run("init", "-q", "--object-format="+r.ObjectFormat)
		for _, name := range []string{"app.txt", "rule.txt"} {
			b, e := os.ReadFile(filepath.Join(packet, "repository/base", name))
			if e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(repo, name), b, 0600); e != nil {
				t.Fatal(e)
			}
		}
		run("add", ".")
		run("commit", "-q", "-m", manifest.BaseMessage)
		if got := run("rev-parse", "HEAD"); got != r.BaseRevision {
			t.Fatalf("base %s != %s", got, r.BaseRevision)
		}
		if e := os.WriteFile(filepath.Join(repo, "app.txt"), []byte("after\n"), 0600); e != nil {
			t.Fatal(e)
		}
		run("add", ".")
		run("commit", "-q", "-m", manifest.ContentMessage)
		if got := run("rev-parse", "HEAD"); got != r.ContentRevision {
			t.Fatalf("content %s != %s", got, r.ContentRevision)
		}
		b, e := os.ReadFile(filepath.Join(packet, r.Map))
		if e != nil {
			t.Fatal(e)
		}
		if e := os.Mkdir(filepath.Join(repo, ".corvint"), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(repo, candidateSidecar), b, 0600); e != nil {
			t.Fatal(e)
		}
		run("add", ".")
		run("commit", "-q", "-m", manifest.TargetMessage)
		if got := run("rev-parse", "HEAD"); got != r.TargetRevision {
			t.Fatalf("target %s != %s", got, r.TargetRevision)
		}
		repos[r.Name] = binding{repo, r.BaseRevision, r.TargetRevision, r.ContentRevision}
	}
	for _, c := range manifest.Cases {
		t.Run(c.Name, func(t *testing.T) {
			r := repos[c.Repository]
			artifacts := t.TempDir()
			artifactDirectory := c.ArtifactDirectory
			if artifactDirectory == "" {
				artifactDirectory = "artifacts"
			}
			if c.Target == "content" {
				r.target = r.content
			}
			if e := os.Mkdir(filepath.Join(artifacts, artifactDirectory), 0700); e != nil {
				t.Fatal(e)
			}
			for _, name := range []string{"runner-plan.json", "runner-receipt.json", "tasks-capture.json", "tasks-verification.json", "tasks-claimed-ticket.json"} {
				p := "artifacts/" + name
				if p == c.OmitArtifact {
					continue
				}
				b, e := os.ReadFile(filepath.Join(packet, p))
				if e != nil {
					t.Fatal(e)
				}
				if p == c.ChangeArtifact {
					b = append(b, ' ')
				}
				if e := os.WriteFile(filepath.Join(artifacts, artifactDirectory, name), b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			cmd := exec.Command(exe, "verify-candidate", "--repository", r.repo, "--map", filepath.Join(packet, c.Map), "--expected-base", r.base, "--target", r.target, "--artifacts", artifacts)
			out, e := cmd.Output()
			status := 0
			if e != nil {
				if ee, ok := e.(*exec.ExitError); ok {
					status = ee.ExitCode()
				} else {
					t.Fatal(e)
				}
			}
			var got candidateResult
			if e := json.Unmarshal(out, &got); e != nil {
				t.Fatalf("output %s %v", out, e)
			}
			if status != c.Exit || got.Integrity != c.Integrity {
				t.Fatalf("exit %d want %d: %s", status, c.Exit, out)
			}
			if got.ReferenceIntegrity != "REFERENCE_INTEGRITY_ONLY" || len(got.Limits) != 8 {
				t.Fatalf("limits: %s", out)
			}
			for _, v := range got.Limits {
				if v != "NOT_OBSERVED" {
					t.Fatalf("assurance %s", out)
				}
			}
		})
	}
}
func TestCandidateStrictAdditiveBoundary(t *testing.T) {
	packet, _, _ := candidatePacket(t)
	raw, e := os.ReadFile(filepath.Join(packet, "maps/sha1-valid.json"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e := decodeMap(raw); e == nil {
		t.Fatal("legacy accepted candidate")
	}
	m, shapeErr := decodeCandidate(raw)
	if shapeErr != nil || m.Spec != candidateSpec {
		t.Fatalf("candidate %v", shapeErr)
	}
	for _, mutation := range []struct{ name, old, new string }{{"fraction", "\"criterionIndex\": 0", "\"criterionIndex\": 0.0"}, {"exponent", "\"criterionIndex\": 0", "\"criterionIndex\": 0e0"}, {"casefold", "\"ticketId\":", "\"TicketId\":"}, {"null", "\"criterionIndex\": 0", "\"criterionIndex\": null"}, {"new-profile", candidateSpec, "cem/1.0-experimental.2"}} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := bytes.Replace(raw, []byte(mutation.old), []byte(mutation.new), 1)
			if bytes.Equal(raw, changed) {
				t.Fatal("mutation missed")
			}
			if _, e := decodeCandidate(changed); e == nil {
				t.Fatal("accepted mutation")
			}
		})
	}
	if candidateTokens([]byte(strings.Repeat("[", 65)+"0"+strings.Repeat("]", 65))) == nil {
		t.Fatal("depth accepted")
	}
}
func TestCandidateArtifactBounds(t *testing.T) {
	dir := t.TempDir()
	root, e := os.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	b := []byte("opaque failed native record")
	if e := os.WriteFile(filepath.Join(dir, "record"), b, 0600); e != nil {
		t.Fatal(e)
	}
	a := []candidateArtifact{{"runner-receipt", "record", shaHex(b)}}
	if e := candidateArtifacts(context.Background(), root, a); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("record", filepath.Join(dir, "link")); e != nil {
		t.Fatal(e)
	}
	a[0].Path = "link"
	if candidateArtifacts(context.Background(), root, a) == nil {
		t.Fatal("link accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a[0].Path = "record"
	if candidateArtifacts(ctx, root, a) == nil {
		t.Fatal("cancel accepted")
	}
	if e := os.WriteFile(filepath.Join(dir, "record"), []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if candidateArtifacts(context.Background(), root, a) == nil {
		t.Fatal("changed artifact accepted")
	}
}

func TestCandidateRejectsForgedLooseObject(t *testing.T) {
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	path := filepath.Join(repo, "record")
	if e := os.WriteFile(path, []byte("original"), 0600); e != nil {
		t.Fatal(e)
	}
	oid := strings.TrimSpace(gitRun(t, repo, "hash-object", "-w", "record"))
	v := &verifier{ctx: context.Background(), repo: repo}
	if _, e := v.candidateObject("blob", oid, 1024); e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	if _, e := writer.Write([]byte("blob 8\x00tampered")); e != nil {
		t.Fatal(e)
	}
	if e := writer.Close(); e != nil {
		t.Fatal(e)
	}
	object := filepath.Join(repo, ".git", "objects", oid[:2], oid[2:])
	if e := os.Chmod(object, 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(object, encoded.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := v.candidateObject("blob", oid, 1024); e == nil || e.code != "object-integrity" {
		t.Fatalf("forged object: %v", e)
	}
}
func TestCandidateRepositoryRefusesAmbientAuthority(t *testing.T) {
	for _, kind := range []string{"alternates", "attributes", "shallow", "config-include"} {
		t.Run(kind, func(t *testing.T) {
			repo := t.TempDir()
			gitRun(t, repo, "init", "-q")
			if e := candidateRepository(repo); e != nil {
				t.Fatal(e)
			}
			path := ""
			switch kind {
			case "alternates":
				path = ".git/objects/info/alternates"
			case "attributes":
				path = ".git/info/attributes"
			case "shallow":
				path = ".git/shallow"
			case "config-include":
				f, e := os.OpenFile(filepath.Join(repo, ".git/config"), os.O_APPEND|os.O_WRONLY, 0600)
				if e != nil {
					t.Fatal(e)
				}
				_, e = f.WriteString("\n[include]\npath = /outside/config\n")
				_ = f.Close()
				if e != nil {
					t.Fatal(e)
				}
			}
			if path != "" {
				if e := os.WriteFile(filepath.Join(repo, path), []byte("untrusted\n"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if candidateRepository(repo) == nil {
				t.Fatal("ambient authority accepted")
			}
		})
	}
}

func TestCandidateScalarKindsBeforeHashing(t *testing.T) {
	packet, _, _ := candidatePacket(t)
	raw, e := os.ReadFile(filepath.Join(packet, "maps/sha1-valid.json"))
	if e != nil {
		t.Fatal(e)
	}
	var original any
	if e := json.Unmarshal(raw, &original); e != nil {
		t.Fatal(e)
	}
	// Discover every scalar position, including reference-array elements, rather
	// than exercising only criterionIndex and leaving zero-value string coercions.
	var paths [][]any
	var collect func(any, []any)
	collect = func(value any, path []any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				collect(child, append(append([]any{}, path...), key))
			}
		case []any:
			for i, child := range v {
				collect(child, append(append([]any{}, path...), i))
			}
		default:
			paths = append(paths, path)
		}
	}
	collect(original, nil)
	for _, path := range paths {
		for _, replacement := range []any{nil, true, map[string]any{}, []any{}, float64(0), ""} {
			var value any
			_ = json.Unmarshal(raw, &value)
			parent := value
			for _, part := range path[:len(path)-1] {
				switch key := part.(type) {
				case string:
					parent = parent.(map[string]any)[key]
				case int:
					parent = parent.([]any)[key]
				}
			}
			var old any
			switch key := path[len(path)-1].(type) {
			case string:
				old = parent.(map[string]any)[key]
			case int:
				old = parent.([]any)[key]
			}
			if fmt.Sprintf("%T", old) == fmt.Sprintf("%T", replacement) {
				continue
			}
			switch key := path[len(path)-1].(type) {
			case string:
				parent.(map[string]any)[key] = replacement
			case int:
				parent.([]any)[key] = replacement
			}
			changed, _ := json.Marshal(value)
			var root map[string]json.RawMessage
			_ = json.Unmarshal(changed, &root)
			if candidateScalarTypes(root) {
				t.Fatalf("scalar kind admitted at %v replacement %T", path, replacement)
			}
			if _, e := decodeCandidate(changed); e == nil {
				t.Fatalf("scalar decoded at %v replacement %T", path, replacement)
			}
		}
	}
	for _, token := range []string{"null", "true", "false", "\"0\"", "{}", "[]", "0.0", "0e0", "-0", "01", "9007199254740992"} {
		if _, ok := candidateWireInteger(json.RawMessage(token)); ok {
			t.Fatalf("integer %s admitted", token)
		}
	}
	for _, token := range []string{"0", "255", "9007199254740991"} {
		if _, ok := candidateWireInteger(json.RawMessage(token)); !ok {
			t.Fatalf("valid canonical integer %s refused", token)
		}
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(raw, &root)
	var criteria []map[string]json.RawMessage
	_ = json.Unmarshal(root["criterionBindings"], &criteria)
	for key := range criteria[0] {
		saved := criteria[0][key]
		if key == "id" {
			continue
		}
		for _, token := range []string{"null", "true", "{}", "[]"} {
			criteria[0][key] = json.RawMessage(token)
			if _, e := candidateCanonicalRecord(criteria[0]); e == nil {
				t.Fatalf("canonicalization admitted %s:%s", key, token)
			}
		}
		criteria[0][key] = saved
	}
}

// CEM 0.3: independently authored from the public algorithm contract.
func Test03IndependentStructuralPredicates(t *testing.T) {
	tests := []struct {
		name, reason, before, after string
		want                        bool
	}{
		{"formatter-only", "formatter-only", "package p\nfunc f( ) { }\n", "package p\n\nfunc f() {}\n", true},
		{"formatter-semantic", "formatter-only", "package p\nfunc f() int {return 1}\n", "package p\nfunc f() int {return 2}\n", false},
		{"rename-receiver", "rename", "package p\ntype box int\nfunc(old box) Value() int{return int(old)}\n", "package p\ntype box int\nfunc(next box) Value() int{return int(next)}\n", true},
		{"rename-label", "rename", "package p\nfunc f(){old: for { break old }}\n", "package p\nfunc f(){next: for { break next }}\n", true},
		{"rename-local", "rename", "package p\nfunc f() int { old := 1; return old }\n", "package p\nfunc f() int { next := 1; return next }\n", true},
		{"rename-comment", "rename", "package p\n// old is local; oldish stays.\nvar old = 1\n", "package p\n// next is local; oldish stays.\nvar next = 1\n", true},
		{"rename-exported", "rename", "package p\nvar Old = 1\n", "package p\nvar Next = 1\n", false},
		{"rename-existing", "rename", "package p\nvar old, next = 1, 2\n", "package p\nvar next, next = 1, 2\n", false},
		{"rename-selector", "rename", "package p\nvar old = 1\nfunc f(){ _ = value.old }\n", "package p\nvar next = 1\nfunc f(){ _ = value.next }\n", false},
		{"rename-field", "rename", "package p\ntype box struct{ old int }\n", "package p\ntype box struct{ next int }\n", false},
		{"rename-method", "rename", "package p\ntype box int\nfunc(box) old(){}\n", "package p\ntype box int\nfunc(box) next(){}\n", false},
		{"rename-directive", "rename", "package p\n//go:linkname old external\nvar old int\n", "package p\n//go:linkname next external\nvar next int\n", false},
		{"rename-two-pairs", "rename", "package p\nvar old, other int\n", "package p\nvar next, different int\n", false},
		{"move-functions", "move", "package p\nfunc a(){}\nfunc b(){}\n", "package p\nfunc b(){}\nfunc a(){}\n", true},
		{"move-vars", "move", "package p\nvar a=first()\nvar b=second()\n", "package p\nvar b=second()\nvar a=first()\n", false},
		{"move-init", "move", "package p\nfunc init(){ a() }\nfunc init(){ b() }\n", "package p\nfunc init(){ b() }\nfunc init(){ a() }\n", false},
		{"move-semantic", "move", "package p\nfunc a(){ x() }\nfunc b(){}\n", "package p\nfunc b(){}\nfunc a(){ y() }\n", false},
		{"import-order", "import-reorder", "package p\nimport(\n \"strings\"\n \"fmt\"\n)\n", "package p\nimport(\n \"fmt\"\n \"strings\"\n)\n", true},
		{"import-alias", "import-reorder", "package p\nimport(\n s \"strings\"\n \"fmt\"\n)\n", "package p\nimport(\n \"fmt\"\n x \"strings\"\n)\n", false},
		{"import-semantic", "import-reorder", "package p\nimport(\n \"strings\"\n \"fmt\"\n)\nvar x=1\n", "package p\nimport(\n \"fmt\"\n \"strings\"\n)\nvar x=2\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, ok := parseGo03([]byte(tc.before))
			if !ok {
				t.Fatal("invalid before")
			}
			b, ok := parseGo03([]byte(tc.after))
			if !ok {
				t.Fatal("invalid after")
			}
			got := false
			switch tc.reason {
			case "formatter-only":
				x, e := format.Source([]byte(tc.before))
				y, f := format.Source([]byte(tc.after))
				got = e == nil && f == nil && bytes.Equal(x, y)
			case "rename":
				got = rename03(a, b)
			case "move":
				got = move03(a, b)
			case "import-reorder":
				got = importReorder03(a, b)
			}
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
func testMap03() map[string]any {
	return map[string]any{"spec": spec03, "baseRevision": strings.Repeat("a", 40), "patchSha256": strings.Repeat("b", 64), "excludedPath": sidecar03, "evidence": []any{}, "hunks": []any{map[string]any{"id": "hunk:sha256:" + strings.Repeat("c", 64), "path": "sample.go", "oldRange": map[string]any{"start": 1, "count": 5}, "newRange": map[string]any{"start": 1, "count": 5}, "disposition": "mechanical", "reason": "import-reorder", "basis": []any{}}}}
}
func Test03WitnessRetentionAndBoundaries(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string]any)
		code string
	}{
		{"absent", func(h map[string]any) {}, ""},
		{"coverage", func(h map[string]any) {
			h["coverage"] = map[string]any{"profileSha256": strings.Repeat("a", 64), "testRun": "fixture\u0085\u2028", "mode": "set", "state": "covered", "covered": []any{map[string]any{"start": 1, "count": 1}}}
		}, ""},
		{"null", func(h map[string]any) { h["coverage"] = nil }, "invalid-field"},
		{"uncompiled-residual", func(h map[string]any) {
			h["discriminates"] = map[string]any{"treeRevision": strings.Repeat("1", 40), "selectionSha256": strings.Repeat("2", 64), "mutants": 2, "killed": 1, "survived": 0, "survivors": []any{}, "bounds": map[string]any{"maxHunks": 1, "maxMutants": 1, "wallTimeSeconds": 1}, "state": "discriminates", "detail": ""}
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := testMap03()
			h := m["hunks"].([]any)[0].(map[string]any)
			tc.edit(h)
			raw, _ := json.Marshal(m)
			got, e := decode03(raw)
			if tc.code != "" {
				if e == nil || e.code != tc.code {
					t.Fatalf("error %v", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			again, _ := json.Marshal(got)
			var retained map[string]any
			if e := json.Unmarshal(again, &retained); e != nil {
				t.Fatal(e)
			}
			want, _ := json.Marshal(m)
			actual, _ := json.Marshal(retained)
			if string(want) != string(actual) {
				t.Fatalf("lost values or presence\n%s\n%s", want, actual)
			}
		})
	}
	for _, tc := range []struct {
		s    string
		want bool
	}{{strings.Repeat("x", 256), true}, {strings.Repeat("x", 257), false}, {strings.Repeat("é", 128), true}, {strings.Repeat("é", 129), false}, {"x\t", false}, {"x\u007f", false}, {"x\u0085", true}, {"x\u2028", true}} {
		if textBound03(tc.s, 1, 256) != tc.want {
			t.Fatal("text boundary")
		}
	}
}
func Test03StrictJSONIsSeparateFromLegacy(t *testing.T) {
	for _, s := range []string{`{"n":-0}`, `{"n":1e0}`, `{"n":1.0}`, `{"n":9007199254740992}`, `{"n":1,"n":2}`, `{"n":"\ud800"}`} {
		if strictJSON03([]byte(s)) == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	for _, s := range []string{`{"n":0}`, `{"n":9007199254740991}`, `{"n":true}`} {
		if e := strictJSON03([]byte(s)); e != nil {
			t.Fatal(e)
		}
	}
	for _, n := range []int{63, 64} {
		s := `{"extra":` + strings.Repeat("[", n) + `0` + strings.Repeat("]", n) + `}`
		e := strictJSON03([]byte(s))
		if (e == nil) != (n == 63) {
			t.Fatalf("depth %d: %v", n, e)
		}
	}
	m := testMap03()
	h := m["hunks"].([]any)[0].(map[string]any)
	h["newRange"] = map[string]any{"start": uint64(maxWireInteger), "count": 1}
	raw, _ := json.Marshal(m)
	if _, e := decode03(raw); e != nil {
		t.Fatalf("invented early endpoint rejection: %v", e)
	}
}

func Test03GitDirectChildControls(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("proposal refuses Windows process scope")
	}
	for _, tc := range []struct {
		name, script, want string
		limit              int
		cancel             bool
	}{
		{"normal", "#!/bin/sh\nprintf 'ok'\n", "", 64, false},
		{"output-limit", "#!/bin/sh\nprintf '0123456789abcdef'\n", "unsupported-resource-limit", 8, false},
		// exec replaces the shell: this test admits exactly one child, not descendants.
		{"cancel", "#!/bin/sh\nexec /bin/sleep 30\n", "git-timeout", 64, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tool := filepath.Join(root, "git")
			if e := os.WriteFile(tool, []byte(tc.script), 0700); e != nil {
				t.Fatal(e)
			}
			t.Setenv("PATH", root)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				timer := time.AfterFunc(100*time.Millisecond, cancel)
				defer timer.Stop()
			}
			v := &verifier{ctx: ctx, repo: root}
			start := time.Now()
			out, e := v.git03(tc.limit, "version")
			if tc.want == "" {
				if e != nil || string(out) != "ok" {
					t.Fatalf("out=%q error=%v", out, e)
				}
			} else if e == nil || !e.operational || e.code != tc.want {
				t.Fatalf("error=%v want operational %s", e, tc.want)
			}
			if time.Since(start) > 5*time.Second {
				t.Fatal("direct child control exceeded bound")
			}
		})
	}
}

func Test03DiffFailureBoundary(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("proposal refuses Windows process scope")
	}
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, "git"), []byte("#!/bin/sh\nexit 1\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", root)
	for _, tc := range []struct {
		name, want            string
		diff, cancel, exhaust bool
	}{
		{"generic-read", "git-read-failed", false, false, false},
		{"canonical-diff", "git-diff-failed", true, false, false},
		{"diff-resource", "unsupported-resource-limit", true, false, true},
		{"diff-timeout", "git-timeout", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			v := &verifier{ctx: ctx, repo: root}
			if tc.exhaust {
				v.gitOps = maxGitOps
			}
			var e *cemError
			if tc.diff {
				_, e = v.patch03(strings.Repeat("a", 40), strings.Repeat("b", 40))
			} else {
				_, e = v.git03(64, "cat-file", "blob", strings.Repeat("a", 40))
			}
			if e == nil || !e.operational || e.code != tc.want {
				t.Fatalf("error=%v want operational %s", e, tc.want)
			}
		})
	}
}

func TestStableArtifactBoundary(t *testing.T) {
	root := t.TempDir()
	root, e := filepath.EvalSymlinks(root)
	if e != nil {
		t.Fatal(e)
	}
	b := []byte(`{"status":"FAILED","authentication":"FORGED"}`)
	if e := os.WriteFile(filepath.Join(root, "receipt"), b, 0600); e != nil {
		t.Fatal(e)
	}
	a := stableArtifact{"runner-receipt", "receipt", shaHex(b)}
	checks, err := checkStableArtifacts(context.Background(), root, []stableArtifact{a})
	if err != nil || len(checks) != 1 {
		t.Fatalf("opaque integrity %v %v", checks, err)
	}
	a.SHA256 = strings.Repeat("0", 64)
	checks, err = checkStableArtifacts(context.Background(), root, []stableArtifact{a})
	if err == nil || err.code != "artifact-digest-mismatch" || len(checks) != 0 {
		t.Fatalf("mismatch %v %v", checks, err)
	}
	if e := os.Symlink("receipt", filepath.Join(root, "link")); e != nil {
		t.Fatal(e)
	}
	a.Path = "link"
	a.SHA256 = shaHex(b)
	_, err = checkStableArtifacts(context.Background(), root, []stableArtifact{a})
	if err == nil || err.code != "artifact-unavailable" {
		t.Fatalf("symlink %v", err)
	}
	if e := os.Symlink(root, filepath.Join(root, "dirlink")); e != nil {
		t.Fatal(e)
	}
	a.Path = "dirlink/receipt"
	_, err = checkStableArtifacts(context.Background(), root, []stableArtifact{a})
	if err == nil || err.code != "artifact-unavailable" {
		t.Fatalf("directory symlink %v", err)
	}
	a.Path = "../receipt"
	_, err = checkStableArtifacts(context.Background(), root, []stableArtifact{a})
	if err == nil {
		t.Fatal("escape allowed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.Path = "receipt"
	_, err = checkStableArtifacts(ctx, root, []stableArtifact{a})
	if err == nil || err.code != "verification-timeout" {
		t.Fatalf("cancel %v", err)
	}
}
func TestStableSeparateProfileAndArgumentPrecedence(t *testing.T) {
	for _, s := range []string{"cem/0.1", "cem/0.3", "cem/1.0-candidate"} {
		_, e := decodeStable([]byte(`{"spec":"` + s + `"}`))
		if e == nil || e.code != "unsupported-spec" {
			t.Fatalf("profile %s: %v", s, e)
		}
	}
	for _, args := range [][]string{{"--repository", "/missing", "--map", "/missing", "--bogus", "x"}, {"--repository", "/missing", "--map", "/missing", "--map", "/other"}, {"--repository", "relative", "--map", "/missing"}} {
		r, n := executeStableCLI(context.Background(), args)
		if n != 2 || r.Stage != "arguments" || r.Code == nil || *r.Code != "invalid-arguments" || r.MapSHA256 != nil {
			t.Fatalf("arguments %v %v", r, n)
		}
	}
	r, n := executeStableCLI(context.Background(), []string{"--repository", "/missing", "--map", "/missing"})
	if n != 2 || r.Stage != "input" {
		t.Fatalf("input precedence %v", r)
	}
	b, _ := json.Marshal(newStableResult("base", "target"))
	var obj map[string]json.RawMessage
	_ = json.Unmarshal(b, &obj)
	if len(obj) != 21 {
		t.Fatalf("result keys %d", len(obj))
	}
}

func TestStableArtifactsRequireTwoCompletePasses(t *testing.T) {
	a, b := []byte("first"), []byte("second")
	refs := []stableArtifact{{"runner-receipt", "a", shaHex(a)}, {"runner-plan", "b", shaHex(b)}}
	for _, mode := range []string{"success", "changed-second-pass", "unavailable-second-pass", "late-first-pass"} {
		t.Run(mode, func(t *testing.T) {
			calls := []string{}
			read := func(_ context.Context, _ string, r stableArtifact) ([]byte, *cemError) {
				calls = append(calls, r.Path)
				n := len(calls)
				if mode == "unavailable-second-pass" && n == 4 {
					return nil, operational("artifact-unavailable")
				}
				if mode == "late-first-pass" && n == 2 {
					return nil, operational("artifact-resource-limit")
				}
				if mode == "changed-second-pass" && n == 4 {
					return []byte("changed"), nil
				}
				if r.Path == "a" {
					return a, nil
				}
				return b, nil
			}
			checks, e := checkStableArtifactReads(context.Background(), "/explicit", refs, read)
			if mode == "success" {
				if e != nil || len(checks) != 2 || strings.Join(calls, ",") != "a,b,a,b" {
					t.Fatalf("success %v %v %v", checks, e, calls)
				}
			} else {
				if e == nil || len(checks) != 0 {
					t.Fatalf("partial success escaped %v %v", checks, e)
				}
				if mode == "changed-second-pass" && (e.operational || e.code != "artifact-changed-during-verification") {
					t.Fatalf("change %v", e)
				}
			}
		})
	}
}

func TestStableSidecarFailureStates(t *testing.T) {
	for _, tc := range []struct{ name, want, stage, issue string }{{"missing", "ABSENT", "verification", "patch-digest-mismatch"}, {"equal", "EXACT", "verification", "patch-digest-mismatch"}, {"mismatch", "MISMATCH", "binding", "excluded-artifact-mismatch"}, {"symlink", "UNSUPPORTED_KIND", "binding", "excluded-artifact-mismatch"}, {"executable", "UNSUPPORTED_KIND", "binding", "excluded-artifact-mismatch"}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			gitRun(t, repo, "init", "-q")
			write := func(p string, b []byte, mode os.FileMode) {
				t.Helper()
				if e := os.MkdirAll(filepath.Dir(filepath.Join(repo, p)), 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(filepath.Join(repo, p), b, mode); e != nil {
					t.Fatal(e)
				}
			}
			commit := func() string {
				gitRun(t, repo, "add", ".")
				gitRun(t, repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
				return strings.TrimSpace(gitRun(t, repo, "rev-parse", "HEAD"))
			}
			write("app.txt", []byte("a\n"), 0600)
			base := commit()
			m := stableMap{Spec: stableSpec, BaseRevision: base, PatchSHA256: strings.Repeat("0", 64), ExcludedPath: sidecar03, Evidence: []evidence{}, Hunks: []hunk03{{mappedHunk: mappedHunk{ID: "hunk:sha256:" + strings.Repeat("1", 64), Path: "app.txt", OldRange: lineRange{1, 1}, NewRange: lineRange{1, 1}, Disposition: "unknown", Reason: "no-evidence", Basis: []basis{}}}}, stableReferences: emptyStableReferences()}
			raw, e := json.Marshal(m)
			if e != nil {
				t.Fatal(e)
			}
			write("app.txt", []byte("b\n"), 0600)
			switch tc.name {
			case "equal":
				write(sidecar03, raw, 0600)
			case "mismatch":
				write(sidecar03, []byte("different"), 0600)
			case "executable":
				write(sidecar03, raw, 0700)
			case "symlink":
				if e := os.MkdirAll(filepath.Dir(filepath.Join(repo, sidecar03)), 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink("app.txt", filepath.Join(repo, sidecar03)); e != nil {
					t.Fatal(e)
				}
			}
			target := commit()
			r, n := runStable(context.Background(), repo, raw, base, target, t.TempDir())
			if n != 1 || r.Sidecar != tc.want || r.Stage != tc.stage || len(r.IssueCodes) != 1 || r.IssueCodes[0] != tc.issue || r.Outcome != "REJECT" || r.Accept == nil || *r.Accept {
				t.Fatalf("sidecar %s exit=%d result=%+v", tc.name, n, r)
			}
			if len(r.ArtifactChecks) != 0 || r.Axes["referenceIntegrity"] != "NOT_CHECKED" {
				t.Fatalf("unearned artifact success %+v", r)
			}
			if tc.stage == "binding" {
				expected := newStableResult(base, target)
				no := false
				expected.Accept = &no
				expected.Outcome = "REJECT"
				expected.Stage = "binding"
				expected.IssueCodes = []string{"excluded-artifact-mismatch"}
				expected.Assurance = string03("structural-only")
				expected.MapSHA256 = string03(shaHex(raw))
				expected.Sidecar = tc.want
				expected.Hunks = m.Hunks
				expected.References = m.stableReferences
				expected.Axes["changeIntegrity"] = "FAILED"
				expected.Axes["coverageValidation"] = "ABSENT"
				expected.Axes["discriminationValidation"] = "ABSENT"
				want, _ := json.Marshal(expected)
				got, _ := json.Marshal(r)
				if !bytes.Equal(want, got) {
					t.Fatalf("full 21-key failure mismatch\nwant=%s\ngot=%s", want, got)
				}
			}
		})
	}
}

// Both source WriterTo and destination ReaderFrom dispatch must obey the cap.
func TestCandidateBufferCopyLimits(t *testing.T) {
	for _, writerTo := range []bool{false, true} {
		for _, size := range []int{0, 7, 8, 9, 65537} {
			t.Run(fmt.Sprintf("writerTo=%t/size=%d", writerTo, size), func(t *testing.T) {
				b := &candidateBuffer{limit: 8}
				var source io.Reader = strings.NewReader(strings.Repeat("x", size))
				if !writerTo {
					source = io.LimitReader(source, int64(size))
				}
				n, err := io.Copy(b, source)
				if size <= 8 {
					if err != nil || n != int64(size) || string(b.Bytes()) != strings.Repeat("x", size) {
						t.Fatalf("n=%d err=%v bytes=%d", n, err, len(b.Bytes()))
					}
				} else if !errors.Is(err, errSizeLimit) || n > 8 || len(b.Bytes()) > 8 {
					t.Fatalf("overflow n=%d err=%v retained=%d", n, err, len(b.Bytes()))
				}
			})
		}
	}
	t.Run("successive-writes", func(t *testing.T) {
		b := &candidateBuffer{limit: 8}
		for _, s := range []string{"123", "45678"} {
			if n, e := b.Write([]byte(s)); n != len(s) || e != nil {
				t.Fatalf("write=%d,%v", n, e)
			}
		}
		if n, e := b.Write([]byte("9")); n != 0 || !errors.Is(e, errSizeLimit) || string(b.Bytes()) != "12345678" {
			t.Fatalf("overflow=%d,%v bytes=%q", n, e, b.Bytes())
		}
	})
}

func TestCandidateGitDirectChildOutputLimits(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("direct-child fixture uses POSIX shell")
	}
	for _, tc := range []struct {
		name                 string
		stdout, stderr, exit int
		want                 string
	}{
		{"empty", 0, 0, 0, ""}, {"stdout-exact", 8, 0, 0, ""}, {"stdout-overflow", 9, 0, 0, "unsupported-resource-limit"},
		{"stderr-exact", 8, 65536, 0, ""}, {"stderr-overflow", 8, 65537, 0, "unsupported-resource-limit"},
		{"stdout-overflow-child-failure", 9, 0, 7, "unsupported-resource-limit"}, {"stderr-overflow-child-failure", 0, 65537, 7, "unsupported-resource-limit"},
		{"within-bound-child-failure", 8, 16, 7, "repository-io"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			// Shell builtins only: one direct child, no background processes/descendants.
			script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'\nprintf '%%s' '%s' >&2\nexit %d\n", strings.Repeat("o", tc.stdout), strings.Repeat("e", tc.stderr), tc.exit)
			if err := os.WriteFile(filepath.Join(root, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", root)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			v := &verifier{ctx: ctx, repo: root}
			out, err := v.candidateGit(8, "version")
			if tc.want == "" {
				if err != nil || string(out) != strings.Repeat("o", tc.stdout) {
					t.Fatalf("bytes=%d err=%v", len(out), err)
				}
			} else if err == nil || !err.operational || err.code != tc.want || out != nil {
				t.Fatalf("bytes=%d err=%v want=%s", len(out), err, tc.want)
			}
		})
	}
}
