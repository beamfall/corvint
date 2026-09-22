package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type acceptingTraceAuthority struct{}

func TestStoredRowSecretPatternParityCorpus(t *testing.T) {
	type parityCase struct {
		Name  string `json:"name"`
		Text  string `json:"text"`
		Match bool   `json:"match"`
	}
	var corpus struct {
		Baseline   []parityCase `json:"baseline"`
		WriterOnly []parityCase `json:"writer_only"`
	}
	raw, err := os.ReadFile("../../internal/secretscreen/testdata/parity.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, test := range corpus.Baseline {
		t.Run(test.Name, func(t *testing.T) {
			if got := traceSecretRE.MatchString(test.Text); got != test.Match {
				t.Fatalf("stored-row pattern match for %q = %v, want %v", test.Text, got, test.Match)
			}
		})
	}
	for _, test := range corpus.WriterOnly {
		t.Run(test.Name, func(t *testing.T) {
			if traceSecretRE.MatchString(test.Text) {
				t.Fatalf("stored-row reader unexpectedly rejected writer-only shape %q", test.Text)
			}
		})
	}
}

func (acceptingTraceAuthority) repositoryIdentity() (string, string, error) {
	return "sha1", strings.Repeat("1", 40), nil
}
func (acceptingTraceAuthority) qualifyCommit(string, string) error  { return nil }
func (acceptingTraceAuthority) qualifyPaths(string, []string) error { return nil }
func (acceptingTraceAuthority) finish() error                       { return nil }

func gitTest(t testing.TB, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Corvint", "GIT_AUTHOR_EMAIL=corvint@example.invalid", "GIT_COMMITTER_NAME=Corvint", "GIT_COMMITTER_EMAIL=corvint@example.invalid")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

func plantedRepository(t testing.TB) (string, string) {
	t.Helper()
	root := t.TempDir()
	return plantedRepositoryAt(t, root)
}

func plantedRepositoryAt(t testing.TB, root string) (string, string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".context-corvint/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", ".gitignore", "tracked.txt")
	gitTest(t, root, "commit", "-q", "-m", "planted")
	head := gitTest(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".context-corvint", "traces"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root, head
}

func plantedManifest(t testing.TB) traceManifest {
	t.Helper()
	raw := manifestBytes(t, []any{traceSource("local-trace-v1", "0", ".context-corvint/traces")})
	manifest, err := parseTraceManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func plantedTraceRow(t testing.TB, revision string) []byte {
	t.Helper()
	basis := map[string]any{
		"changed_paths": []any{"tracked.txt"}, "opened_paths": []any{"tracked.txt"},
		"outcome": "passed", "revision": revision, "schema_version": json.Number("1"),
		"task": "planted task", "verification": []any{"go test ./..."},
	}
	raw, err := canonical(basis, false)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	basis["trace_id"] = hex.EncodeToString(digest[:])
	row, err := canonical(basis, true)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func mutateTraceRow(t testing.TB, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	value, err := decodeTrace(bytes.TrimSuffix(raw, []byte{'\n'}))
	if err != nil {
		t.Fatal(err)
	}
	trace := value.(map[string]any)
	mutate(trace)
	basis, err := canonical(cloneWithout(trace, "trace_id"), false)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(basis)
	trace["trace_id"] = hex.EncodeToString(digest[:])
	result, err := canonical(trace, false)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTraceRowsRejectSecretCommandAndForbiddenPath(t *testing.T) {
	revision := strings.Repeat("1", 40)
	valid := plantedTraceRow(t, revision)
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"secret", func(trace map[string]any) { trace["task"] = "inspect token=abcdefghijklmnop" }},
		{"command", func(trace map[string]any) { trace["verification"] = []any{"go test; whoami"} }},
		{"forbidden-path", func(trace map[string]any) { trace["opened_paths"] = []any{".git/config"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := mutateTraceRow(t, valid, test.mutate)
			if _, err := validateTraceRow(raw, revision, acceptingTraceAuthority{}); err == nil {
				t.Fatal("hostile row accepted")
			}
		})
	}
}

func TestTraceCorpusEmptyAndPositive(t *testing.T) {
	root, head := plantedRepository(t)
	manifest := plantedManifest(t)
	empty, err := validateTraceCorpus(root, manifest)
	if err != nil || empty.files != 0 || empty.rows != 0 {
		t.Fatalf("empty = %+v, %v", empty, err)
	}
	tracePath := filepath.Join(root, ".context-corvint", "traces", head+".jsonl")
	if err := os.WriteFile(tracePath, plantedTraceRow(t, head), 0o600); err != nil {
		t.Fatal(err)
	}
	positive, err := validateTraceCorpus(root, manifest)
	if err != nil || positive.files != 1 || positive.rows != 1 || positive.outcomes[head]["passed"] != 1 {
		t.Fatalf("positive = %+v, %v", positive, err)
	}
}

func TestTraceCorpusClassifiesMalformedWrongAndUnreachable(t *testing.T) {
	root, head := plantedRepository(t)
	manifest := plantedManifest(t)
	traceDirectory := filepath.Join(root, ".context-corvint", "traces")
	validPath := filepath.Join(traceDirectory, head+".jsonl")
	if err := os.WriteFile(validPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	malformed, err := validateTraceCorpus(root, manifest)
	if err != nil || malformed.sources[0].terminal["SOURCE_INVALID_SCHEMA"] != 1 {
		t.Fatalf("malformed classification = %+v, %v", malformed, err)
	}
	if err := os.Remove(validPath); err != nil {
		t.Fatal(err)
	}
	wrong := strings.Repeat("0", len(head))
	if err := os.WriteFile(filepath.Join(traceDirectory, wrong+".jsonl"), plantedTraceRow(t, wrong), 0o600); err != nil {
		t.Fatal(err)
	}
	wrongCorpus, err := validateTraceCorpus(root, manifest)
	if err != nil || wrongCorpus.sources[0].terminal["SOURCE_INVALID_IDENTITY"] != 1 {
		t.Fatalf("wrong revision classification = %+v, %v", wrongCorpus, err)
	}
	if err := os.Remove(filepath.Join(traceDirectory, wrong+".jsonl")); err != nil {
		t.Fatal(err)
	}
	tree := gitTest(t, root, "rev-parse", "HEAD^{tree}")
	unreachable := gitTest(t, root, "commit-tree", tree, "-m", "unreachable")
	if err := os.WriteFile(filepath.Join(traceDirectory, unreachable+".jsonl"), plantedTraceRow(t, unreachable), 0o600); err != nil {
		t.Fatal(err)
	}
	unreachableCorpus, err := validateTraceCorpus(root, manifest)
	if err != nil || unreachableCorpus.sources[0].terminal["SOURCE_INVALID_IDENTITY"] != 1 {
		t.Fatalf("unreachable classification = %+v, %v", unreachableCorpus, err)
	}
}

func TestTraceCorpusPartialInvalidMemberByteEquality(t *testing.T) {
	root, oldHead := plantedRepository(t)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "tracked.txt")
	gitTest(t, root, "commit", "-q", "-m", "second")
	newHead := gitTest(t, root, "rev-parse", "HEAD")
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.WriteFile(filepath.Join(directory, oldHead+".jsonl"), plantedTraceRow(t, oldHead), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, newHead+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestRaw := manifestBytes(t, []any{traceSource("local-trace-v1", "0", ".context-corvint/traces")})
	manifest, err := parseTraceManifest(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := validateTraceCorpus(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.sources) != 1 || len(corpus.sources[0].members) != 1 || corpus.sources[0].terminal["SOURCE_INVALID_SCHEMA"] != 1 {
		t.Fatalf("partial corpus = %+v", corpus.sources)
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(expected); err != nil {
		t.Fatalf("partial expected snapshot invalid: %v", err)
	}
	assertTraceBlackBox(t, root, manifestRaw, expected)
}

func TestTraceCorpusMultipleConfiguredSourcesSameRevision(t *testing.T) {
	root, head := plantedRepository(t)
	extra := filepath.Join(root, ".context-corvint", "traces-extra")
	if err := os.MkdirAll(extra, 0o700); err != nil {
		t.Fatal(err)
	}
	first := plantedTraceRow(t, head)
	second := mutateTraceRow(t, first, func(trace map[string]any) { trace["task"] = "second configured source" })
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", head+".jsonl"), first, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extra, head+".jsonl"), append(second, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestRaw := manifestBytes(t, []any{
		traceSource("local-trace-v1", "0", ".context-corvint/traces"),
		traceSource("local-trace-v1", "1", ".context-corvint/traces-extra"),
	})
	manifest, err := parseTraceManifest(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := validateTraceCorpus(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.sources) != 2 || len(corpus.sources[0].members) != 1 || len(corpus.sources[1].members) != 1 {
		t.Fatalf("multi-source corpus = %+v", corpus.sources)
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(expected); err != nil {
		t.Fatalf("multi-source expected snapshot invalid: %v", err)
	}
	assertTraceBlackBox(t, root, manifestRaw, expected)
}

func TestTraceCorpusStoreChangedAfterRetry(t *testing.T) {
	root, _ := plantedRepository(t)
	markerA := filepath.Join(root, ".context-corvint", "traces", "a")
	markerB := filepath.Join(root, ".context-corvint", "traces", "b")
	if err := os.WriteFile(markerA, []byte("marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	authority, err := newClosedGitAuthority(root)
	if err != nil {
		t.Fatal(err)
	}
	hook := func(_ string, attempt int) error {
		if attempt == 0 {
			return os.Rename(markerA, markerB)
		}
		return os.Rename(markerB, markerA)
	}
	manifest := plantedManifest(t)
	corpus, err := validateTraceCorpusWithAuthorityAndHook(root, manifest, authority, hook)
	if err != nil {
		t.Fatal(err)
	}
	if corpus.sources[0].overrideCode != "STORE_CHANGED" {
		t.Fatalf("changed store = %+v", corpus.sources[0])
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(expected); err != nil {
		t.Fatalf("changed-store expected snapshot invalid: %v", err)
	}
}

func TestTraceCorpusEntryLimit(t *testing.T) {
	root, _ := plantedRepository(t)
	directory := filepath.Join(root, ".context-corvint", "traces")
	for index := 0; index < 1_001; index++ {
		name := filepath.Join(directory, "ignored-"+strconv.Itoa(index))
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifestRaw := manifestBytes(t, []any{traceSource("local-trace-v1", "0", ".context-corvint/traces")})
	manifest, err := parseTraceManifest(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := validateTraceCorpus(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if corpus.sources[0].overrideCode != "TRACE_STORE_BOUND" || corpus.sources[0].overrideLimit != "1000" {
		t.Fatalf("entry bound = %+v", corpus.sources[0])
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(expected); err != nil {
		t.Fatalf("limit expected snapshot invalid: %v", err)
	}
	assertTraceBlackBox(t, root, manifestRaw, expected)
}

func assertTraceBlackBox(t testing.TB, root string, manifestRaw, expected []byte) {
	t.Helper()
	inputs := t.TempDir()
	manifestPath := filepath.Join(inputs, "manifest.json")
	snapshotPath := filepath.Join(inputs, "snapshot.json")
	if err := os.WriteFile(manifestPath, manifestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, expected, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"verify", "--root", root, "--manifest", manifestPath, "--snapshot", snapshotPath}, &stdout, &stderr); exit != 0 || stderr.Len() != 0 || stdout.String() != `{"profile":"corvint-dashboard-trace-adapter-conformance/0","status":"PASS"}`+"\n" {
		t.Fatalf("black-box exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestTraceBlackBoxByteEquality(t *testing.T) {
	root, head := plantedRepository(t)
	manifestRaw := manifestBytes(t, []any{traceSource("local-trace-v1", "0", ".context-corvint/traces")})
	manifest, err := parseTraceManifest(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", head+".jsonl"), plantedTraceRow(t, head), 0o600); err != nil {
		t.Fatal(err)
	}
	corpus, err := validateTraceCorpus(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(expected); err != nil {
		t.Fatalf("expected snapshot invalid: %v", err)
	}
	inputs := t.TempDir()
	manifestPath := filepath.Join(inputs, "trace-manifest.json")
	snapshotPath := filepath.Join(inputs, "trace-snapshot.json")
	if err := os.WriteFile(manifestPath, manifestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, expected, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := run([]string{"verify", "--root", root, "--manifest", manifestPath, "--snapshot", snapshotPath}, &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 || stdout.String() != `{"profile":"corvint-dashboard-trace-adapter-conformance/0","status":"PASS"}`+"\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if err := os.WriteFile(snapshotPath, frozenValid, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := run([]string{"verify", "--root", root, "--manifest", manifestPath, "--snapshot", snapshotPath}, &stdout, &stderr); exit != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "TRACE_SNAPSHOT_MISMATCH") {
		t.Fatalf("mismatch exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestTraceBlackBoxEmptyAndUTF8Root(t *testing.T) {
	root, _ := plantedRepositoryAt(t, filepath.Join(t.TempDir(), "café"))
	manifestRaw := manifestBytes(t, []any{traceSource("local-trace-v1", "0", ".context-corvint/traces")})
	manifest, err := parseTraceManifest(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := validateTraceCorpus(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil {
		t.Fatal(err)
	}
	inputs := t.TempDir()
	manifestPath := filepath.Join(inputs, "manifest.json")
	snapshotPath := filepath.Join(inputs, "snapshot.json")
	if err := os.WriteFile(manifestPath, manifestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, expected, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"verify", "--root", root, "--manifest", manifestPath, "--snapshot", snapshotPath}, &stdout, &stderr); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestTraceAuthorityRejectsMissingTerminalBlob(t *testing.T) {
	root, head := plantedRepository(t)
	blob := gitTest(t, root, "rev-parse", "HEAD:tracked.txt")
	objectPath := filepath.Join(root, ".git", "objects", blob[:2], blob[2:])
	if err := os.Remove(objectPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", head+".jsonl"), plantedTraceRow(t, head), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateTraceCorpus(root, plantedManifest(t)); err == nil {
		t.Fatal("missing terminal blob accepted")
	}
}

func TestCatFileUsesLongestSortedPrefixes(t *testing.T) {
	objectIDs := make([]string, 50_001)
	for index := range objectIDs {
		objectIDs[index] = strings.Repeat("0", 32) + hex.EncodeToString([]byte{
			byte(index >> 24), byte(index >> 16), byte(index >> 8), byte(index),
		})
	}
	prefixes, err := longestCatFilePrefixes(objectIDs)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefixes) != 2 || len(prefixes[0]) != 50_000 || len(prefixes[1]) != 1 {
		t.Fatalf("prefix lengths = %v, %v", len(prefixes[0]), len(prefixes[1]))
	}
	if &prefixes[0][0] != &objectIDs[0] || &prefixes[1][0] != &objectIDs[50_000] {
		t.Fatal("prefixes did not preserve the exact sorted ledger")
	}
	if _, err := longestCatFilePrefixes([]string{strings.Repeat("a", 4_194_304)}); rejectionCode(err) != rejectSize {
		t.Fatalf("over-bound single ID rejection = %v", err)
	}
}
