package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

// batchDocument is the SBQ-V0-004 wire, read back with the operation receipts
// left as raw bytes so parity can be asserted byte-for-byte.
type batchDocument struct {
	OK         bool   `json:"ok"`
	Mutates    bool   `json:"mutates"`
	Tool       string `json:"tool"`
	Profile    string `json:"profile"`
	Snapshot   struct{ Tree, Commit string }
	Operations []struct {
		ID      string          `json:"id"`
		Verb    string          `json:"verb"`
		OK      bool            `json:"ok"`
		Context json.RawMessage `json:"context"`
		Error   json.RawMessage `json:"error"`
	} `json:"operations"`
}

func batchRepository(t *testing.T) string {
	t.Helper()
	return batchRepositoryWithModule(t, "example.test/batch")
}

// batchRepositoryWithModule builds the fixture at one module path. A module
// without a slash is the one that makes `.go` impact refuse with
// unsupported-impact-repository. The .gitignore entry is what lets the
// self-observation ledger be written at all, so a stray append is observable.
func batchRepositoryWithModule(t *testing.T, module string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".gitignore": ".corvint/\n.context-corvint/\n",
		"go.mod":     "module " + module + "\n\ngo 1.27.0\n",
		"AGENTS.md": "# Project instructions\n\nThe roadmap is the only active work queue.\n" +
			"Run the required workflow gates before taking a ticket.\n" +
			"Orient with `script/orient.sh`, then take the next roadmap ticket.\n",
		"script/orient.sh":      "#!/bin/sh\nexit 0\n",
		"cache/demux.go":        "package cache\n\n// Split keeps the demux key.\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go":   "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		"cache/reader.go":       "package cache\n\nfunc Read(key string) string { return Split(key) }\n",
		"testing/features.yaml": "features: []\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "batch fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func runBatchForTest(t *testing.T, root, request string) (int, []byte, []byte) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "batch"}, strings.NewReader(request), &stdout, &stderr)
	return code, stdout.Bytes(), stderr.Bytes()
}

func decodeBatchDocument(t *testing.T, stdout []byte) batchDocument {
	t.Helper()
	var document batchDocument
	if err := json.Unmarshal(stdout, &document); err != nil {
		t.Fatalf("decode batch document: %v: %s", err, stdout)
	}
	return document
}

func refusalCode(t *testing.T, stderr []byte) string {
	t.Helper()
	var refusal map[string]any
	if err := json.Unmarshal(stderr, &refusal); err != nil {
		t.Fatalf("decode refusal: %v: %s", err, stderr)
	}
	code, _ := refusal["code"].(string)
	return code
}

func refusalMessage(t *testing.T, stderr []byte) string {
	t.Helper()
	var refusal map[string]any
	if err := json.Unmarshal(stderr, &refusal); err != nil {
		t.Fatalf("decode refusal: %v: %s", err, stderr)
	}
	message, _ := refusal["error"].(string)
	return message
}

// standaloneContextMember returns the "context" member a standalone verb emits,
// as the exact bytes it wrote.
func standaloneContextMember(t *testing.T, root string, arguments ...string) json.RawMessage {
	t.Helper()
	var stdout, stderr bytes.Buffer
	full := append([]string{"--root", root}, arguments...)
	if code := runContext(context.Background(), full, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("standalone %v exit %d: %s", arguments, code, stderr.String())
	}
	var payload struct {
		Context json.RawMessage `json:"context"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode standalone payload: %v: %s", err, stdout.String())
	}
	return payload.Context
}

// standalonePacket returns the whole document the `context` verb writes; that
// packet is the receipt a batch context operation must reproduce.
func standalonePacket(t *testing.T, root string, arguments ...string) json.RawMessage {
	t.Helper()
	var stdout, stderr bytes.Buffer
	full := append([]string{"--root", root}, arguments...)
	if code := runContext(context.Background(), full, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("standalone %v exit %d: %s", arguments, code, stderr.String())
	}
	return json.RawMessage(bytes.TrimSuffix(stdout.Bytes(), []byte{'\n'}))
}

func corvintListing(t *testing.T, root string) string {
	t.Helper()
	entries := make([]string, 0, 16)
	base := filepath.Join(root, ".corvint")
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, relativeErr := filepath.Rel(base, path)
		if relativeErr != nil {
			return relativeErr
		}
		entries = append(entries, fmt.Sprintf("%s %d", filepath.ToSlash(relative), info.Size()))
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	sort.Strings(entries)
	return strings.Join(entries, "\n")
}

// corvintDigest hashes the name, size and full content of every file under
// `.corvint/`, so an append that leaves the file list unchanged — the
// self-observation ledger row the standalone impact verb writes on an
// unsupported- refusal — is still caught.
func corvintDigest(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	base := filepath.Join(root, ".corvint")
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, relativeErr := filepath.Rel(base, path)
		if relativeErr != nil {
			return relativeErr
		}
		fmt.Fprintf(digest, "\n%s %d\n", filepath.ToSlash(relative), info.Size())
		if info.IsDir() {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		digest.Write(content)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// batchQueryRows builds count structurally valid query operations.
func batchQueryRows(count int) string {
	rows := make([]string, 0, count)
	for index := 0; index < count; index++ {
		rows = append(rows, fmt.Sprintf(`{"id":"o%d","verb":"query","task":"demux"}`, index))
	}
	return `{"operations":[` + strings.Join(rows, ",") + `]}`
}

func TestBatchRefusesMalformedRequestsBeforeReading(t *testing.T) {
	t.Parallel()
	// SBQ-V0-001 and the request bounds half of SBQ-V0-006. The fixture has no
	// snapshot, so an invalid-arguments refusal (rather than
	// unsupported-batch-snapshot) is itself the proof that nothing was read.
	root := batchRepository(t)
	// A valid request padded past the stdin bound: the refusal must come from
	// the bound itself, not from JSON that would fail to parse anyway.
	valid := `{"operations":[{"id":"a","verb":"query","task":"t"}]}`
	oversize := valid + strings.Repeat(" ", gokernel.MaxInputBytes+1-len(valid))
	for _, test := range []struct{ name, request string }{
		{"unknown member", `{"operations":[{"id":"a","verb":"query","task":"t"}],"limit":3}`},
		{"unknown operation member", `{"operations":[{"id":"a","verb":"query","task":"t","subject":"cache/demux.go"}]}`},
		{"unknown verb", `{"operations":[{"id":"a","verb":"prove","task":"t"}]}`},
		{"duplicate id", `{"operations":[{"id":"a","verb":"query","task":"t"},{"id":"a","verb":"query","task":"u"}]}`},
		{"null scalar", `{"operations":[{"id":"a","verb":"query","task":"t","limit":null}]}`},
		{"seventeen operations", batchQueryRows(17)},
		{"oversize stdin", oversize},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := runBatchForTest(t, root, test.request)
			if code != 2 || len(stdout) != 0 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if got := refusalCode(t, stderr); got != "invalid-arguments" {
				t.Fatalf("code=%q stderr=%s", got, stderr)
			}
			if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
				t.Fatalf("refusal touched .corvint: %v", err)
			}
		})
	}
}

func TestBatchNullPathsIsStructural(t *testing.T) {
	t.Parallel()
	// An explicit JSON null for "paths" is a structural invalid-arguments
	// refusal, like the other pointer-decoded members; an absent "paths" is
	// unchanged: impact's own runtime refusal, not the batch parse.
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[{"id":"a","verb":"impact","paths":null}]}`)
	if code != 2 || len(stdout) != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if got := refusalCode(t, stderr); got != "invalid-arguments" {
		t.Fatalf("code=%q stderr=%s", got, stderr)
	}
	code, stdout, stderr = runBatchForTest(t, root, `{"operations":[{"id":"a","verb":"impact"}]}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	document := decodeBatchDocument(t, stdout)
	if len(document.Operations) != 1 || document.Operations[0].OK {
		t.Fatalf("absent paths should still be impact's own refusal: %s", stdout)
	}
}

func TestBatchValidatesTheRequestBeforeResolvingTheRoot(t *testing.T) {
	t.Parallel()
	// SBQ-V0-001: "refused before any repository read" includes the .git stat
	// resolveExplicitRoot performs, so a malformed request against a directory
	// that is no repository still reports the request's own refusal.
	root := t.TempDir()
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[{"id":"a","verb":"prove","task":"t"}]}`)
	if code != 2 || len(stdout) != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if got := refusalCode(t, stderr); got != "invalid-arguments" {
		t.Fatalf("code=%q stderr=%s", got, stderr)
	}
	if message := refusalMessage(t, stderr); !strings.Contains(message, "unrecognized verb") {
		t.Fatalf("the root was resolved before the request was validated: %s", stderr)
	}
	// The root is still resolved once the request validates.
	_, _, stderr = runBatchForTest(t, root, `{"operations":[{"id":"a","verb":"query","task":"t"}]}`)
	if message := refusalMessage(t, stderr); !strings.Contains(message, "not a Git repository") {
		t.Fatalf("valid request skipped root resolution: %s", stderr)
	}
}

func TestBatchAcceptsTheSixteenOperationBound(t *testing.T) {
	t.Parallel()
	// SBQ-V0-001: sixteen is admitted; the seventeen-operation refusal above is
	// the bound, not an off-by-one.
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	rows := make([]string, 0, 16)
	for index := 0; index < 16; index++ {
		rows = append(rows, fmt.Sprintf(`{"id":"o%d","verb":"impact","paths":["cache/demux.go"]}`, index))
	}
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[`+strings.Join(rows, ",")+`]}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	document := decodeBatchDocument(t, stdout)
	if len(document.Operations) != 16 {
		t.Fatalf("operations = %d", len(document.Operations))
	}
	for _, operation := range document.Operations {
		if !operation.OK {
			t.Fatalf("operation %s failed: %s", operation.ID, operation.Error)
		}
	}
}

func TestBatchRefusesOneOperationsArgumentsAndContinues(t *testing.T) {
	t.Parallel()
	// SBQ-V0-004 and the amended failure mode: an argument the standalone verb
	// refuses at its own parse (an empty task, an out-of-range budget_bytes) is
	// that operation's invalid-arguments error and never aborts the batch.
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[
		{"id":"first","verb":"impact","paths":["cache/demux.go"]},
		{"id":"empty","verb":"context","task":""},
		{"id":"last","verb":"impact","paths":["cache/reader.go"]}]}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	document := decodeBatchDocument(t, stdout)
	if len(document.Operations) != 3 {
		t.Fatalf("operations = %s", stdout)
	}
	first, failing, last := document.Operations[0], document.Operations[1], document.Operations[2]
	if !first.OK || !last.OK || len(first.Context) == 0 || len(last.Context) == 0 {
		t.Fatalf("surrounding operations = %s", stdout)
	}
	if failing.ID != "empty" || failing.OK || len(failing.Context) != 0 {
		t.Fatalf("failing operation = %s", stdout)
	}
	var refusal map[string]any
	if err := json.Unmarshal(failing.Error, &refusal); err != nil {
		t.Fatal(err)
	}
	if code, _ := refusal["code"].(string); code != "invalid-arguments" {
		t.Fatalf("refusal = %s", failing.Error)
	}
	if message, _ := refusal["message"].(string); !strings.Contains(message, "the following arguments are required: --task") {
		t.Fatalf("refusal = %s", failing.Error)
	}
	// The out-of-range byte budget is the same class of per-operation refusal.
	code, stdout, stderr = runBatchForTest(t, root, `{"operations":[
		{"id":"budget","verb":"query","task":"does Split keep empty demux keys","budget_bytes":1}]}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	document = decodeBatchDocument(t, stdout)
	if len(document.Operations) != 1 || document.Operations[0].OK {
		t.Fatalf("budget operation = %s", stdout)
	}
	if err := json.Unmarshal(document.Operations[0].Error, &refusal); err != nil {
		t.Fatal(err)
	}
	if code, _ := refusal["code"].(string); code != "invalid-arguments" {
		t.Fatalf("budget refusal = %s", document.Operations[0].Error)
	}
	if message, _ := refusal["message"].(string); !strings.Contains(message, "budget_bytes must be between") {
		t.Fatalf("budget refusal = %s", document.Operations[0].Error)
	}
}

func TestBatchRefusesWithoutSnapshot(t *testing.T) {
	t.Parallel()
	// SBQ-V0-002: no build fallback, and no self-observation append despite the
	// unsupported- prefix (SBQ-V0-005, invariant 4).
	root := batchRepository(t)
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[{"id":"a","verb":"query","task":"does Split keep empty demux keys"}]}`)
	if code != 2 || len(stdout) != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if got := refusalCode(t, stderr); got != "unsupported-batch-snapshot" {
		t.Fatalf("code=%q stderr=%s", got, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint", "self-observations.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("snapshot refusal wrote the self-observation ledger: %v", err)
	}
}

func TestBatchMatchesStandaloneVerbsAtOneSnapshot(t *testing.T) {
	t.Parallel()
	// SBQ-V0-003: each operation's receipt is byte-equal to the standalone
	// verb's own at the same snapshot, across both query intents, context and
	// impact.
	root := batchRepository(t)
	addTrackedRustImpactFixture(t, root)
	runIndexForTest(t, root, false)
	repositoryTask := "does Split keep empty demux keys"
	operationsTask := authorityStartPrompt
	if intent := contextindex.QueryIntent(repositoryTask); intent != "repository" {
		t.Fatalf("fixture repository task classified as %q", intent)
	}
	if intent := contextindex.QueryIntent(operationsTask); intent != "project-operations" {
		t.Fatalf("fixture project-operations task classified as %q", intent)
	}
	request, err := json.Marshal(map[string]any{"operations": []map[string]any{
		{"id": "repository", "verb": "query", "task": repositoryTask},
		{"id": "operations", "verb": "query", "task": operationsTask},
		{"id": "packet", "verb": "context", "task": repositoryTask, "subject": "cache/demux.go"},
		{"id": "reach", "verb": "impact", "paths": []string{"cache/demux.go"}},
		{"id": "unruled", "verb": "impact", "paths": []string{"crates/ignore/src/dir.rs"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runBatchForTest(t, root, string(request))
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	document := decodeBatchDocument(t, stdout)
	if !document.OK || document.Mutates || document.Tool != "batch" || document.Profile != batchProfile {
		t.Fatalf("envelope = %s", stdout)
	}
	index, hit, err := contextindex.LoadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("load snapshot: hit=%v err=%v", hit, err)
	}
	if document.Snapshot.Tree != index.Revision || document.Snapshot.Commit != index.CommitRevision {
		t.Fatalf("snapshot identity = %+v", document.Snapshot)
	}
	want := map[string]json.RawMessage{
		"repository": standaloneContextMember(t, root, "query", "--task", repositoryTask),
		"operations": standaloneContextMember(t, root, "query", "--task", operationsTask),
		"packet":     standalonePacket(t, root, "context", "--task", repositoryTask, "--subject", "cache/demux.go"),
		"reach":      standaloneContextMember(t, root, "impact", "cache/demux.go"),
		"unruled":    standaloneContextMember(t, root, "impact", "crates/ignore/src/dir.rs"),
	}
	if len(document.Operations) != len(want) {
		t.Fatalf("operations = %s", stdout)
	}
	for _, operation := range document.Operations {
		if !operation.OK {
			t.Fatalf("operation %s failed: %s", operation.ID, operation.Error)
		}
		if !bytes.Equal(operation.Context, want[operation.ID]) {
			t.Fatalf("operation %s receipt differs from the standalone verb\nbatch      = %s\nstandalone = %s",
				operation.ID, operation.Context, want[operation.ID])
		}
	}
}

func TestBatchContinuesPastAFailingOperation(t *testing.T) {
	t.Parallel()
	// SBQ-V0-004: a per-operation refusal carries the standalone verb's error
	// and never stops the operation after it; the batch still exits 0.
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[
		{"id":"first","verb":"impact","paths":["cache/demux.go"]},
		{"id":"missing","verb":"impact","paths":["cache/absent.go"]},
		{"id":"last","verb":"context","task":"does Split keep empty demux keys"}]}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	document := decodeBatchDocument(t, stdout)
	if len(document.Operations) != 3 {
		t.Fatalf("operations = %s", stdout)
	}
	first, failing, last := document.Operations[0], document.Operations[1], document.Operations[2]
	if first.ID != "first" || !first.OK || last.ID != "last" || !last.OK || len(last.Context) == 0 {
		t.Fatalf("surrounding operations = %s", stdout)
	}
	if failing.ID != "missing" || failing.OK || len(failing.Context) != 0 {
		t.Fatalf("failing operation = %s", stdout)
	}
	var refusal map[string]any
	if err := json.Unmarshal(failing.Error, &refusal); err != nil {
		t.Fatal(err)
	}
	message, _ := refusal["message"].(string)
	if !strings.Contains(message, "impact paths are not tracked at revision") {
		t.Fatalf("refusal = %s", failing.Error)
	}
	// contextindex.Error values without a code omit the member, exactly as
	// emitError renders the standalone refusal.
	if _, present := refusal["code"]; present {
		t.Fatalf("codeless refusal gained a code member: %s", failing.Error)
	}
}

func TestBatchWritesNothing(t *testing.T) {
	t.Parallel()
	// SBQ-V0-005: read-only on every path, including the snapshot directory the
	// index verb owns.
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	beforeGit, beforeCorvint := repositoryListing(t, root), corvintListing(t, root)
	beforeDigest := corvintDigest(t, root)
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[
		{"id":"query","verb":"query","task":"does Split keep empty demux keys"},
		{"id":"packet","verb":"context","task":"does Split keep empty demux keys","subject":"cache/demux.go"},
		{"id":"reach","verb":"impact","paths":["cache/demux.go"]},
		{"id":"refused","verb":"impact","paths":["cache/absent.go"]}]}`)
	if code != 0 || len(stdout) == 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	if after := repositoryListing(t, root); after != beforeGit {
		t.Fatalf("batch changed the worktree:\n%s\n%s", beforeGit, after)
	}
	if after := corvintListing(t, root); after != beforeCorvint {
		t.Fatalf("batch changed .corvint:\n%s\n%s", beforeCorvint, after)
	}
	if after := corvintDigest(t, root); after != beforeDigest {
		t.Fatalf("batch changed the content under .corvint: %s -> %s", beforeDigest, after)
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint", "self-observations.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("batch wrote the self-observation ledger: %v", err)
	}
}

func TestBatchWritesNoUnsupportedObservation(t *testing.T) {
	t.Parallel()
	// SBQ-V0-005: batch never appends the self-observation ledger, including on
	// the unsupported- coded operation refusal the standalone impact verb does
	// observe (invariant 4).
	root := batchRepositoryWithModule(t, "batchfixture")
	runIndexForTest(t, root, false)
	ledger := filepath.Join(root, ".corvint", "self-observations.jsonl")
	// Control: the standalone verb's append lands in this fixture, so the same
	// append from the batch path would be visible to the digest below.
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "impact", "cache/demux.go"},
		strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("standalone impact exit %d: %s", code, stderr.String())
	}
	before, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("standalone impact wrote no ledger, so this fixture proves nothing: %v", err)
	}
	beforeDigest := corvintDigest(t, root)
	code, batchStdout, batchStderr := runBatchForTest(t, root, `{"operations":[{"id":"reach","verb":"impact","paths":["cache/demux.go"]}]}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, batchStderr)
	}
	document := decodeBatchDocument(t, batchStdout)
	if len(document.Operations) != 1 || document.Operations[0].OK {
		t.Fatalf("operations = %s", batchStdout)
	}
	var refusal map[string]any
	if err := json.Unmarshal(document.Operations[0].Error, &refusal); err != nil {
		t.Fatal(err)
	}
	if code, _ := refusal["code"].(string); code != "unsupported-impact-repository" {
		t.Fatalf("refusal = %s", document.Operations[0].Error)
	}
	if after := corvintDigest(t, root); after != beforeDigest {
		t.Fatalf("batch changed the content under .corvint: %s -> %s", beforeDigest, after)
	}
	if after, err := os.ReadFile(ledger); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("batch appended the self-observation ledger: err=%v\n%s\n%s", err, before, after)
	}
}

// possessedFrom reads the evidence rows one operation's receipt carries and
// renders them as a possession list, so a fixture possesses exactly what the
// snapshot actually pinned.
func possessedFrom(t *testing.T, receipt json.RawMessage) []map[string]any {
	t.Helper()
	var packet struct {
		Results []struct {
			Evidence []struct {
				Path     string `json:"path"`
				BlobHash string `json:"blob_hash"`
			} `json:"evidence"`
		} `json:"results"`
	}
	if err := json.Unmarshal(receipt, &packet); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	seen := make(map[string]struct{})
	entries := make([]map[string]any, 0)
	for _, result := range packet.Results {
		for _, row := range result.Evidence {
			key := row.Path + "\x00" + row.BlobHash
			if _, duplicate := seen[key]; duplicate || row.BlobHash == "" {
				continue
			}
			seen[key] = struct{}{}
			entries = append(entries, map[string]any{"path": row.Path, "blob_hash": row.BlobHash})
		}
	}
	return entries
}

func batchDelta(t *testing.T, stdout []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(stdout, &document); err != nil {
		t.Fatalf("decode batch document: %v", err)
	}
	delta, _ := document["delta"].(map[string]any)
	return delta
}

func TestBatchRefusesMalformedPossessed_SBQ007(t *testing.T) {
	t.Parallel()
	// SBQ-V0-007(b),(c): shape, the 32-entry cap, a repeated (path, blob_hash)
	// pair and the byte bound are all refused before any repository read. The
	// fixture has no snapshot, so invalid-arguments rather than
	// unsupported-batch-snapshot is itself the proof nothing was read.
	root := batchRepository(t)
	entries := func(count int, hash func(int) string) string {
		rows := make([]string, 0, count)
		for index := 0; index < count; index++ {
			rows = append(rows, fmt.Sprintf(`{"path":"cache/demux.go","blob_hash":%q}`, hash(index)))
		}
		return strings.Join(rows, ",")
	}
	distinct := func(index int) string { return fmt.Sprintf("h%d", index) }
	wrap := func(list string) string {
		return `{"operations":[{"id":"a","verb":"query","task":"t","possessed":[` + list + `]}]}`
	}
	// A 32-entry request whose encoded length is exactly the bound is accepted
	// by SBQ-V0-007's own caps and refused only by the byte bound one byte
	// later, so the two limits are distinguishable.
	underBound := wrap(entries(maxPossessedEntries, distinct))
	atBound := underBound[:len(underBound)-1] + strings.Repeat(" ", gokernel.MaxInputBytes-len(underBound)) + "}"
	for _, test := range []struct{ name, request string }{
		{"null possessed", `{"operations":[{"id":"a","verb":"query","task":"t","possessed":null}]}`},
		{"object possessed", `{"operations":[{"id":"a","verb":"query","task":"t","possessed":{}}]}`},
		{"scalar possessed", `{"operations":[{"id":"a","verb":"query","task":"t","possessed":3}]}`},
		{"thirty-three entries", wrap(entries(maxPossessedEntries+1, distinct))},
		{"missing blob_hash", wrap(`{"path":"cache/demux.go"}`)},
		{"non-string path", wrap(`{"path":7,"blob_hash":"h"}`)},
		{"empty path", wrap(`{"path":"","blob_hash":"h"}`)},
		{"unrecognized entry member", wrap(`{"path":"p","blob_hash":"h","kind":"path"}`)},
		{"negative line", wrap(`{"path":"p","blob_hash":"h","line":-1}`)},
		{"repeated pair", wrap(`{"path":"p","blob_hash":"h"},{"path":"p","blob_hash":"h"}`)},
		{"one byte over the bound", atBound + " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := runBatchForTest(t, root, test.request)
			if code != 2 || len(stdout) != 0 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if got := refusalCode(t, stderr); got != "invalid-arguments" {
				t.Fatalf("code=%q stderr=%s", got, stderr)
			}
			if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
				t.Fatalf("refusal touched .corvint: %v", err)
			}
		})
	}
	t.Run("exact byte bound and large fields accepted", func(t *testing.T) {
		runIndexForTest(t, root, false)
		if len(atBound) != 131072 {
			t.Fatal(len(atBound))
		}
		large := wrap(entries(32, func(i int) string { return distinct(i) + strings.Repeat("x", 3000) }))
		for _, request := range []string{underBound, atBound, large} {
			code, _, stderr := runBatchForTest(t, root, request)
			if code != 0 {
				t.Fatalf("%d bytes: exit=%d stderr=%s", len(request), code, stderr)
			}
		}
	})
	t.Run("one path under several hashes is legal", func(t *testing.T) {
		runIndexForTest(t, root, false)
		code, _, stderr := runBatchForTest(t, root, wrap(`{"path":"cache/demux.go","blob_hash":"h1"},{"path":"cache/demux.go","blob_hash":"h2"}`))
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr)
		}
	})
}

func TestBatchPossessionIgnoresLine_SBQ007a(t *testing.T) {
	t.Parallel()
	// SBQ-V0-007(a): `line` is validated and ignored, so a stored line and one
	// dropped after validation are indistinguishable on the wire.
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	base := `{"operations":[{"id":"i","verb":"impact","paths":["cache/demux.go"],"possessed":[{"path":"cache/demux.go","blob_hash":"h"%s}]}]}`
	codeWithout, without, stderr := runBatchForTest(t, root, fmt.Sprintf(base, ""))
	codeWith, with, _ := runBatchForTest(t, root, fmt.Sprintf(base, `,"line":42`))
	if codeWithout != 0 || codeWith != 0 {
		t.Fatalf("exit=%d/%d stderr=%s", codeWithout, codeWith, stderr)
	}
	if !bytes.Equal(without, with) {
		t.Fatalf("line changed the wire\nwithout = %s\nwith    = %s", without, with)
	}
}

func TestBatchPossessionSuppressedResults_SBQ010b(t *testing.T) {
	t.Parallel()
	// SBQ-V0-009(f) and SBQ-V0-010(b): a fully possessed impact operation keeps
	// every coverage count and its pre-suppression `state`, gains
	// coverage.suppressed_results and exactly one appended uncertainty line, and
	// reports the departed results under delta.suppressed.
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	baseline := `{"operations":[{"id":"i","verb":"impact","paths":["cache/demux.go"]}]}`
	code, stdout, stderr := runBatchForTest(t, root, baseline)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	before := decodeBatchDocument(t, stdout).Operations[0].Context
	possessed := possessedFrom(t, before)
	if len(possessed) == 0 {
		t.Fatalf("fixture impact receipt carries no evidence: %s", before)
	}
	request, err := json.Marshal(map[string]any{"operations": []map[string]any{
		{"id": "i", "verb": "impact", "paths": []string{"cache/demux.go"}, "possessed": possessed},
	}})
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runBatchForTest(t, root, string(request))
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	var pre, post struct {
		State    string         `json:"state"`
		Results  []any          `json:"results"`
		Coverage map[string]any `json:"coverage"`
	}
	if err := json.Unmarshal(before, &pre); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(decodeBatchDocument(t, stdout).Operations[0].Context, &post); err != nil {
		t.Fatal(err)
	}
	if post.State != pre.State {
		t.Fatalf("state = %q, want the pre-suppression %q", post.State, pre.State)
	}
	for _, count := range []string{"requested_results", "included_results", "omitted_results",
		"authoritative_results", "advisory_results"} {
		if fmt.Sprint(post.Coverage[count]) != fmt.Sprint(pre.Coverage[count]) {
			t.Fatalf("%s moved: %v -> %v", count, pre.Coverage[count], post.Coverage[count])
		}
	}
	suppressedCount := len(pre.Results) - len(post.Results)
	if suppressedCount == 0 {
		t.Fatalf("nothing suppressed: %s", stdout)
	}
	if fmt.Sprint(post.Coverage["suppressed_results"]) != fmt.Sprint(suppressedCount) {
		t.Fatalf("suppressed_results = %v, want %d", post.Coverage["suppressed_results"], suppressedCount)
	}
	line := fmt.Sprintf("%d results suppressed by caller possession", suppressedCount)
	uncertainty, _ := post.Coverage["uncertainty"].([]any)
	if len(uncertainty) == 0 || uncertainty[len(uncertainty)-1] != line {
		t.Fatalf("uncertainty = %v, want %q appended", uncertainty, line)
	}
	delta := batchDelta(t, stdout)
	entries, _ := delta["suppressed"].([]any)
	if len(entries) != suppressedCount {
		t.Fatalf("delta.suppressed = %v", delta["suppressed"])
	}
	first, _ := entries[0].(map[string]any)
	if first["operation"] != "i" || first["reason"] != "possessed-retained" ||
		first["profile"] != batchProfile || first["recover"] != "resend the operation without `possessed`" {
		t.Fatalf("delta.suppressed[0] = %v", first)
	}
	binding, _ := delta["binding"].(map[string]any)
	for _, member := range []string{"tree", "commit", "engine", "status_sha256"} {
		if value, _ := binding[member].(string); value == "" {
			t.Fatalf("delta.binding.%s empty: %v", member, binding)
		}
	}
	if _, present := delta["fallback"]; present {
		t.Fatalf("clean fixture took a fallback: %v", delta["fallback"])
	}
}

func TestBatchPossessionMixedWorktreeFallback_SBQ010d(t *testing.T) {
	t.Parallel()
	// SBQ-V0-010(d): the mixed-worktree fallback is batch-wide but still yields
	// exactly one delta.fallback entry per possessing operation and none for an
	// operation that supplied no `possessed`; every entry of a fallback
	// operation is echoed under delta.ignored.
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	if err := os.WriteFile(filepath.Join(root, "cache", "reader.go"),
		[]byte("package cache\n\nfunc Read(key string) string { return Split(key) + \"!\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	possessed := []map[string]any{{"path": "cache/demux.go", "blob_hash": "h"}}
	request, err := json.Marshal(map[string]any{"operations": []map[string]any{
		{"id": "p1", "verb": "impact", "paths": []string{"cache/demux.go"}, "possessed": possessed},
		{"id": "n1", "verb": "impact", "paths": []string{"cache/demux.go"}},
		{"id": "p2", "verb": "context", "task": "does Split keep empty demux keys", "possessed": possessed},
		{"id": "n2", "verb": "context", "task": "does Split keep empty demux keys"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runBatchForTest(t, root, string(request))
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	delta := batchDelta(t, stdout)
	entries, _ := delta["fallback"].([]any)
	if len(entries) != 2 {
		t.Fatalf("delta.fallback = %v", delta["fallback"])
	}
	for position, want := range []string{"p1", "p2"} {
		row, _ := entries[position].(map[string]any)
		if row["operation"] != want || row["reason"] != "mixed-worktree" {
			t.Fatalf("delta.fallback[%d] = %v", position, row)
		}
	}
	ignored, _ := delta["ignored"].([]any)
	if len(ignored) != 2 {
		t.Fatalf("delta.ignored = %v", delta["ignored"])
	}
	for _, operation := range decodeBatchDocument(t, stdout).Operations {
		if bytes.Contains(operation.Context, []byte(`"suppressed_results"`)) {
			t.Fatalf("operation %s gained suppressed_results under a fallback: %s", operation.ID, operation.Context)
		}
	}
}

func TestBatchPossessionTablesAndFailedOperations_SBQ008h(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	for _, missing := range []string{"tracked", "skipped", "both", "neither"} {
		t.Run(missing, func(t *testing.T) {
			index, hit, err := loadSnapshot(context.Background(), root)
			if err != nil || !hit {
				t.Fatalf("load: %v %v", hit, err)
			}
			if missing == "tracked" || missing == "both" {
				index.Tracked = nil
			}
			if missing == "skipped" || missing == "both" {
				index.Skipped = nil
			}
			ops, err := parseBatchRequest([]byte(`{"operations":[{"id":"p","verb":"impact","paths":["cache/demux.go"],"possessed":[]},{"id":"u","verb":"impact","paths":["cache/demux.go"]}]}`))
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := emitBatchReceipt(&stdout, &stderr, batchReceipt(context.Background(), root, index, ops))
			document := decodeBatchDocument(t, stdout.Bytes())
			if code != 0 || !document.Operations[1].OK {
				t.Fatalf("exit=%d stdout=%s", code, stdout.Bytes())
			}
			if missing == "neither" {
				if !document.Operations[0].OK {
					t.Fatal(string(document.Operations[0].Error))
				}
			} else if document.Operations[0].OK || !bytes.Contains(document.Operations[0].Error, []byte(`"code":"unsupported-possession-tables"`)) {
				t.Fatal(string(stdout.Bytes()))
			}
			delta := batchDelta(t, stdout.Bytes())
			if len(delta) != 1 || delta["binding"] == nil {
				t.Fatalf("verdict lists on table refusal: %v", delta)
			}
		})
	}
	code, stdout, stderr := runBatchForTest(t, root, `{"operations":[{"id":"p","verb":"query","task":"","possessed":[]}]}`)
	if code != 0 || batchDelta(t, stdout)["binding"] == nil {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// Compare binding before bytes: a changed input names its own member instead of
// being misreported as nondeterministic receipt serialization.
func batchBindingDifference(left, right map[string]any) string {
	for _, key := range []string{"tree", "commit", "engine", "status_sha256", "trace_digests"} {
		if !reflect.DeepEqual(left[key], right[key]) {
			return key
		}
	}
	return ""
}

func TestPossessionReplayAtBoundQuintuple_SBQ010f(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	request := `{"operations":[{"id":"q","verb":"query","task":"Split demux","possessed":[]},{"id":"a","verb":"query","task":"take the next roadmap ticket"},{"id":"c","verb":"context","task":"Split demux","possessed":[]},{"id":"i","verb":"impact","paths":["cache/demux.go"],"possessed":[]}]}`
	run := func() ([]byte, map[string]any) {
		t.Helper()
		code, stdout, stderr := runBatchForTest(t, root, request)
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr)
		}
		for _, op := range decodeBatchDocument(t, stdout).Operations {
			if !op.OK && op.ID != "a" {
				t.Fatalf("%s: %s", op.ID, op.Error)
			}
		}
		delta := batchDelta(t, stdout)
		if len(delta) != 1 {
			t.Fatalf("empty possession produced lists: %v", delta)
		}
		return stdout, delta["binding"].(map[string]any)
	}
	first, binding := run()
	second, replay := run()
	if member := batchBindingDifference(binding, replay); member != "" {
		t.Fatalf("binding differs: %s", member)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("equal binding produced unequal receipt")
	}
	traces := binding["trace_digests"].([]any)
	if len(traces) != 1 || traces[0].(map[string]any)["operation"] != "q" {
		t.Fatalf("ledger reading operations = %v", traces)
	}
	for _, member := range []string{"tree", "commit", "engine", "status_sha256", "trace_digests"} {
		changed := make(map[string]any)
		for k, v := range binding {
			changed[k] = v
		}
		changed[member] = "changed"
		if got := batchBindingDifference(binding, changed); got != member {
			t.Fatalf("wanted %s drift, got %s", member, got)
		}
	}
	_, err := tracerecordrepo.Record(context.Background(), root, tracerecordrepo.Input{
		Task: "Split demux", OpenedPaths: []string{"cache/demux.go"}, ChangedPaths: []string{"cache/reader.go"}, Verification: []string{"go test ./..."}, Outcome: "passed",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, changed := run()
	if got := batchBindingDifference(binding, changed); got != "trace_digests" {
		t.Fatalf("changed ledger: %s", got)
	}
	if err := os.WriteFile(filepath.Join(root, "cache/reader.go"), []byte("package cache\n"), 0644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runBatchForTest(t, root, request)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	dirty := batchDelta(t, stdout)["binding"].(map[string]any)
	if got := batchBindingDifference(changed, dirty); got != "status_sha256" {
		t.Fatalf("changed dirty state: %s", got)
	}
}

func TestBatchTraceDigestProjection_SBQ010f(t *testing.T) {
	t.Parallel()
	a := trace.Record{SchemaVersion: 1, Revision: "tree", TraceID: "a", Task: "task", OpenedPaths: []string{}, Outcome: "passed"}
	b := a
	b.TraceID = "b"
	got := batchTraceDigest([]trace.Record{b, a}, "ready")
	if got != batchTraceDigest([]trace.Record{a, b}, "ready") {
		t.Fatal("record order changed digest")
	}
	preimage := `{"records":[{"opened_paths":[],"outcome":"passed","revision":"tree","schema_version":1,"task":"task","trace_id":"a"},{"opened_paths":[],"outcome":"passed","revision":"tree","schema_version":1,"task":"task","trace_id":"b"}],"state":"ready"}`
	if want := fmt.Sprintf("%x", sha256.Sum256([]byte(preimage))); got != want {
		t.Fatalf("projection digest=%s want=%s", got, want)
	}
	b.TraceID = "c"
	if got == batchTraceDigest([]trace.Record{a, b}, "ready") {
		t.Fatal("TraceID was not bound")
	}
	if got == batchTraceDigest([]trace.Record{a, b}, "blocked-mixed-worktree") {
		t.Fatal("state was not bound")
	}
}

func TestBatchPossessionEvidenceByteSavings_SBQ010i(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	operations := []map[string]any{
		{"id": "q", "verb": "query", "task": "Split demux", "limit": 10},
		{"id": "c", "verb": "context", "task": "Split demux", "subject": "cache/demux.go", "limit": 10},
		{"id": "i", "verb": "impact", "paths": []string{"cache/demux.go"}, "limit": 10},
	}
	run := func() []byte {
		t.Helper()
		request, err := json.Marshal(map[string]any{"operations": operations})
		if err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := runBatchForTest(t, root, string(request))
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr)
		}
		return stdout
	}
	baseline := decodeBatchDocument(t, run())
	for i, op := range baseline.Operations {
		if !op.OK {
			t.Fatalf("baseline %s: %s", op.ID, op.Error)
		}
		operations[i]["possessed"] = possessedFrom(t, op.Context)
	}
	treatmentBytes := run()
	treatment := decodeBatchDocument(t, treatmentBytes)
	delta := batchDelta(t, treatmentBytes)
	suppressed := make(map[string]bool)
	for _, raw := range delta["suppressed"].([]any) {
		row := raw.(map[string]any)
		suppressed[fmt.Sprint(row["operation"], "\x00", row["kind"], "\x00", row["id"])] = true
	}
	numerator, denominator := 0, 0
	for i, op := range baseline.Operations {
		var before, after struct {
			Results []struct {
				Kind, ID string
				Evidence json.RawMessage
			}
			Coverage map[string]json.RawMessage
		}
		if err := json.Unmarshal(op.Context, &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(treatment.Operations[i].Context, &after); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before.Coverage["critical_missing"], after.Coverage["critical_missing"]) {
			t.Fatal("critical_missing changed")
		}
		for _, result := range before.Results {
			denominator += len(result.Evidence)
			if suppressed[op.ID+"\x00"+result.Kind+"\x00"+result.ID] {
				numerator += len(result.Evidence)
			}
		}
	}
	if denominator == 0 || 100*numerator/denominator < 30 {
		t.Fatalf("evidence savings %d/%d below 30%%", numerator, denominator)
	}
	t.Logf("tree=%s commit=%s evidence_bytes=%d suppressed_evidence_bytes=%d savings_percent=%d", baseline.Snapshot.Tree, baseline.Snapshot.Commit, denominator, numerator, 100*numerator/denominator)
}

func TestBatchTraceCaptureBetweenOperationsAndFailure_SBQ010f(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	index, hit, err := loadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("load=%v err=%v", hit, err)
	}
	collector := &possessionCollector{}
	operation := batchOperation{id: "first", verb: "query", task: "Split demux", limit: 10, supplied: true}
	first := batchOperationRow(context.Background(), root, index, operation, collector)
	if first["ok"] != true {
		t.Fatal(first)
	}
	_, err = tracerecordrepo.Record(context.Background(), root, tracerecordrepo.Input{Task: "Split demux", OpenedPaths: []string{"cache/demux.go"}, ChangedPaths: []string{"cache/reader.go"}, Verification: []string{"go test ./..."}, Outcome: "passed"})
	if err != nil {
		t.Fatal(err)
	}
	operation.id = "second"
	second := batchOperationRow(context.Background(), root, index, operation, collector)
	if second["ok"] != true {
		t.Fatal(second)
	}
	if len(collector.traces) != 2 {
		t.Fatal(collector.traces)
	}
	before, after := collector.traces[0].(map[string]any), collector.traces[1].(map[string]any)
	if before["operation"] != "first" || after["operation"] != "second" || before["digest"] == after["digest"] {
		t.Fatalf("capture not bound at each read: %v", collector.traces)
	}
	records, state, err := tracerecordrepo.Read(context.Background(), root, index)
	if err != nil {
		t.Fatal(err)
	}
	if after["digest"] != batchTraceDigest(records, state) {
		t.Fatalf("second capture differs from consumed store: %v", after)
	}
	writeRecordStore(t, root, []byte("malformed\n"))
	operation.id = "failed"
	failed := batchOperationRow(context.Background(), root, index, operation, collector)
	if failed["ok"] != false {
		t.Fatal(failed)
	}
	captured := collector.traces[2].(map[string]any)
	if captured["operation"] != "failed" || captured["state"] != "" || captured["digest"] != batchTraceDigest(nil, "") {
		t.Fatalf("failed Read result not captured: %v", captured)
	}
	if !collector.supplied || collector.delta(index)["binding"] == nil {
		t.Fatal("failed possessing read lost binding")
	}
}

func TestBatchPossessedUnboundedFieldsAndIgnoredInteger_SBQ007(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	base := `{"operations":[{"id":"i","verb":"impact","paths":["cache/demux.go"],"possessed":[{"path":"absent","blob_hash":"h"%s}]}]}`
	_, without, _ := runBatchForTest(t, root, fmt.Sprintf(base, ""))
	for _, line := range []string{"0", "-0", strings.Repeat("9", 1000)} {
		code, with, stderr := runBatchForTest(t, root, fmt.Sprintf(base, `,"line":`+line))
		if code != 0 || !bytes.Equal(without, with) {
			t.Fatalf("line changed receipt: exit=%d stderr=%s", code, stderr)
		}
	}
	entries := make([]map[string]any, 32)
	for i := range entries {
		entries[i] = map[string]any{"path": fmt.Sprint(i) + strings.Repeat("p", 4200), "blob_hash": "h"}
	}
	encoded, _ := json.Marshal(map[string]any{"operations": []map[string]any{{"id": "i", "verb": "impact", "paths": []string{"cache/demux.go"}, "possessed": entries}}})
	code, _, stderr := runBatchForTest(t, filepath.Join(root, "nonexistent"), string(encoded))
	if len(encoded) <= 131072 || code != 2 || refusalCode(t, stderr) != "invalid-arguments" {
		t.Fatalf("bytes=%d code=%d stderr=%s", len(encoded), code, stderr)
	}
}

func TestBatchInvalidationOrderingAndRehydration_SBQ009(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	operations := []map[string]any{
		{"id": "z", "verb": "impact", "paths": []string{"cache/demux.go"}, "possessed": []map[string]any{{"path": "cache/demux.go", "blob_hash": "forged"}}},
		{"id": "a", "verb": "impact", "paths": []string{"cache/demux.go"}, "possessed": []map[string]any{{"path": "cache/demux.go", "blob_hash": "forged"}}},
	}
	run := func() []byte {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"operations": operations})
		code, out, err := runBatchForTest(t, root, string(raw))
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, err)
		}
		return out
	}
	first := run()
	invalidated := batchDelta(t, first)["invalidated"].([]any)
	if len(invalidated) == 0 {
		t.Fatal("forged hash did not invalidate")
	}
	seen := make(map[string]bool)
	last := ""
	for _, raw := range invalidated {
		row := raw.(map[string]any)
		key := fmt.Sprint(row["kind"], "\x00", row["id"], "\x00", row["path"])
		if seen[key] || key < last || row["verdict"] != "stale" {
			t.Fatal(invalidated)
		}
		seen[key] = true
		last = key
	}
	operations[0], operations[1] = operations[1], operations[0]
	if !reflect.DeepEqual(invalidated, batchDelta(t, run())["invalidated"]) {
		t.Fatal("invalidated depends on operation order")
	}
	for _, operation := range operations {
		delete(operation, "possessed")
	}
	full := run()
	if batchDelta(t, full) != nil {
		t.Fatal("unpossessed request retained delta")
	}
	before, after := decodeBatchDocument(t, first), decodeBatchDocument(t, full)
	if !bytes.Equal(before.Operations[0].Context, after.Operations[0].Context) {
		t.Fatal("unsuppressed stale receipt differs from full receipt")
	}
}

func TestBatchPossessionExactQueryBudget_SBQ010c(t *testing.T) {
	t.Parallel()
	root := batchRepository(t)
	runIndexForTest(t, root, false)
	index, hit, err := loadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("load=%v err=%v", hit, err)
	}
	budget := 10000
	op := batchOperation{id: "q", verb: "query", task: "Split demux", limit: 10, budget: &budget}
	var baseline map[string]any
	for attempt := 0; attempt < 8; attempt++ {
		baseline, err = runBatchOperation(context.Background(), root, index, op)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := contextindex.CanonicalJSON(baseline)
		if err != nil {
			t.Fatal(err)
		}
		if budget == len(encoded) {
			break
		}
		budget = len(encoded)
	}
	encoded, err := contextindex.CanonicalJSON(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != budget {
		t.Fatalf("fixture did not compile at budget: %d != %d", len(encoded), budget)
	}
	for _, entry := range possessedFrom(t, encoded) {
		op.possessed = append(op.possessed, contextindex.PossessedEntry{Path: entry["path"].(string), BlobHash: entry["blob_hash"].(string)})
	}
	op.supplied = true
	collector := &possessionCollector{}
	row := batchOperationRow(context.Background(), root, index, op, collector)
	if row["ok"] != true || len(collector.suppressed) == 0 {
		t.Fatalf("suppression=%v row=%v", collector.suppressed, row)
	}
	receipt := row["context"].(map[string]any)
	coverage := receipt["coverage"].(map[string]any)
	final, err := contextindex.CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if coverage["packet_bytes"] != len(final) || coverage["budget_bytes"] != budget || coverage["within_budget"] != (len(final) <= budget) {
		t.Fatalf("final=%d coverage=%v", len(final), coverage)
	}
	if len(collector.delta(index)) < 2 {
		t.Fatal("fixture has no separate delta bytes")
	}
}
