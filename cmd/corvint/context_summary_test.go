package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// evidenceSummaryFiles is the fixture the default-wire golden was captured
// over; the tree it commits is content-addressed, so the golden is stable.
var evidenceSummaryFiles = map[string]string{
	"AGENTS.md":              "# Agents\n\nFollow docs/specs/demux-v0.md before changing cache.\n",
	"docs/specs/demux-v0.md": "# Demux V0\n\n## Requirements\n\n- `DMX-V0-001`: Split keeps empty keys.\n- `DMX-V0-002`: Join is the inverse of Split.\n\n## Non-goals\n\nNone.\n",
	"go.mod":                 "module example.test/ctx\n\ngo 1.27.0\n",
	"cache/demux.go":         "package cache\n\n// Split keeps empty keys.\nfunc Split(key string) string { return key }\n",
	"cache/demux_test.go":    "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
	"cache/join.go":          "package cache\n\nfunc Join(key string) string { return Split(key) }\n",
	"cache/keys.go":          "package cache\n\nfunc EmptyKeys() []string { return []string{Split(\"\")} }\n",
	"cache/latin1.txt":       "caf\xe9 keys\n",
}

const evidenceSummaryTask = "does `Split` keep empty keys per DMX-V0-001"

func evidenceSummaryRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range evidenceSummaryFiles {
		writeSummaryFixture(t, root, name, content)
	}
	gitFixture(t, root, "init", "-q")
	gitFixture(t, root, "add", "-A")
	gitFixture(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fixture")
	return root
}

func writeSummaryFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitFixture(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v", arguments, err)
	}
	return strings.TrimSpace(string(output))
}

func runContextCommand(t *testing.T, arguments ...string) (int, []byte, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.Bytes(), stderr.String()
}

func decodeObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	return value
}

// TestContextDefaultWireIsTheGolden pins TCP-V0-024's compatibility claim: the
// default invocation prints the bytes captured from the base commit's binary
// (1894b9e5) over this fixture, so the opt-in views leave the wire unchanged.
func TestContextDefaultWireIsTheGolden(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	code, stdout, stderr := runContextCommand(t, "--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	golden, err := os.ReadFile("testdata/context-default-wire.golden")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout, golden) {
		t.Fatalf("default context wire drifted from the base golden:\n%s\nwant\n%s", stdout, golden)
	}
}

func TestContextSummaryKeepsIdentityCoverageAndCriticalRows(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	base := []string{"--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go"}
	_, full, _ := runContextCommand(t, base...)
	packet := decodeObject(t, full)
	results := packet["results"].([]any)
	critical := len(packet["coverage"].(map[string]any)["critical"].([]any))
	if critical == 0 || len(results) < critical+2 {
		t.Fatalf("fixture needs critical rows and ordinary rows: %s", full)
	}
	sum := sha256.Sum256(full)
	for _, budget := range []int{contextSummaryMinBytes, 1400, 1800, 2400, contextSummaryDefaultBytes} {
		code, stdout, stderr := runContextCommand(t, append(base, "--summary", "--summary-bytes", fmt.Sprint(budget))...)
		if code != 0 {
			if strings.Contains(stderr, `"summary-budget"`) && strings.Contains(stderr, "critical row") {
				continue
			}
			t.Fatalf("budget %d: exit %d: %s", budget, code, stderr)
		}
		if len(stdout) > budget {
			t.Fatalf("budget %d: summary is %d bytes", budget, len(stdout))
		}
		view := decodeObject(t, stdout)
		for _, member := range []string{"coverage", "request", "revision", "schema_version", "state", "subject", "tool", "ok", "mutates"} {
			if fmt.Sprint(view[member]) != fmt.Sprint(packet[member]) {
				t.Fatalf("budget %d: member %s changed: %v != %v", budget, member, view[member], packet[member])
			}
		}
		summary := view["summary"].(map[string]any)
		shown := int(summary["results_shown"].(float64))
		if summary["packet_sha256"] != hex.EncodeToString(sum[:]) || int(summary["results_total"].(float64)) != len(results) || shown+int(summary["results_omitted"].(float64)) != len(results) || summary["evidence_complete"] != "UNKNOWN" {
			t.Fatalf("budget %d: summary totals = %v", budget, summary)
		}
		if shown < critical {
			t.Fatalf("budget %d: summary dropped a critical row: %s", budget, stdout)
		}
		for index, value := range view["results"].([]any) {
			row := value.(map[string]any)
			original := results[index].(map[string]any)
			evidence := original["evidence"].([]any)[0].(map[string]any)
			want := "cv1:" + packet["revision"].(string) + ":" + evidence["blob_hash"].(string) + ":all:" + evidence["path"].(string)
			if row["id"] != original["id"] || row["kind"] != original["kind"] || row["authority"] != evidence["authority"] || row["trust"] != evidence["trust"] || row["blob_hash"] != evidence["blob_hash"] || row["handle"] != want {
				t.Fatalf("budget %d: row %d = %v, packet row %v", budget, index, row, original)
			}
		}
	}
	code, _, stderr := runContextCommand(t, append(base, "--summary", "--summary-bytes", "1023")...)
	if code != 2 || !strings.Contains(stderr, `"summary-budget"`) {
		t.Fatalf("budget below the floor: exit %d: %s", code, stderr)
	}
}

func TestContextSummaryTruncationReportsTotalsAndRefusesToDropCriticalRows(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	base := []string{"--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go", "--summary"}
	_, full, _ := runContextCommand(t, base...)
	complete := decodeObject(t, full)["summary"].(map[string]any)
	if complete["results_omitted"].(float64) != 0 {
		t.Fatalf("default budget should hold the fixture: %v", complete)
	}
	truncated := false
	refused := false
	for budget := contextSummaryMinBytes; budget < len(full); budget += 64 {
		code, stdout, stderr := runContextCommand(t, append(base, "--summary-bytes", fmt.Sprint(budget))...)
		if code != 0 {
			refused = refused || strings.Contains(stderr, "through the last critical row, which need")
			if stdout != nil && len(stdout) > 0 {
				t.Fatalf("refusal printed a partial summary: %s", stdout)
			}
			continue
		}
		summary := decodeObject(t, stdout)["summary"].(map[string]any)
		if summary["results_omitted"].(float64) > 0 {
			truncated = true
			if !strings.Contains(summary["continuation"].(string), "results[results_shown:]") || summary["results_total"] != complete["results_total"] {
				t.Fatalf("truncated summary lacks its continuation route: %v", summary)
			}
		}
	}
	if !truncated || !refused {
		t.Fatalf("sweep did not observe both truncation (%v) and critical refusal (%v)", truncated, refused)
	}
}

func TestContextExpandReturnsExactPinnedBytes(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	tree := gitFixture(t, root, "rev-parse", "HEAD^{tree}")
	blob := gitFixture(t, root, "rev-parse", "HEAD:docs/specs/demux-v0.md")
	source := []byte(evidenceSummaryFiles["docs/specs/demux-v0.md"])
	writeSummaryFixture(t, root, "docs/specs/demux-v0.md", "dirty worktree bytes must never be returned\n")
	for _, test := range []struct {
		handle, text string
		complete     bool
	}{
		{"cv1:" + tree + ":" + blob + ":all:docs/specs/demux-v0.md", string(source), true},
		{"cv1:" + tree + ":" + blob[:7] + ":5-6:docs/specs/demux-v0.md", "- `DMX-V0-001`: Split keeps empty keys.\n- `DMX-V0-002`: Join is the inverse of Split.\n", false},
	} {
		code, stdout, stderr := runContextCommand(t, "--root", root, "context", "--expand", test.handle)
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", test.handle, code, stderr)
		}
		view := decodeObject(t, stdout)
		selection := view["selection"].(map[string]any)
		sourceMember := view["source"].(map[string]any)
		textSum := sha256.Sum256([]byte(test.text))
		if selection["text"] != test.text || selection["sha256"] != hex.EncodeToString(textSum[:]) || selection["complete"] != test.complete {
			t.Fatalf("%s: selection = %v", test.handle, selection)
		}
		object := sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(source))), source...))
		if sourceMember["blob"] != hex.EncodeToString(object[:]) || sourceMember["blob_verified"] != true || sourceMember["tree"] != tree || view["mutates"] != false {
			t.Fatalf("%s: source = %v", test.handle, sourceMember)
		}
	}
}

func TestContextExpandRefusesWithoutSubstitutingContent(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	tree := gitFixture(t, root, "rev-parse", "HEAD^{tree}")
	blob := gitFixture(t, root, "rev-parse", "HEAD:cache/demux.go")
	other := gitFixture(t, root, "rev-parse", "HEAD:cache/join.go")
	latin := gitFixture(t, root, "rev-parse", "HEAD:cache/latin1.txt")
	handle := func(tree, blob, lines, path string) string {
		return "cv1:" + tree + ":" + blob + ":" + lines + ":" + path
	}
	cases := map[string]string{
		"no-prefix":           "ev1:" + tree + ":" + blob + ":all:cache/demux.go",
		"short-tree":          handle(tree[:12], blob, "all", "cache/demux.go"),
		"upper-tree":          handle(strings.ToUpper(tree), blob, "all", "cache/demux.go"),
		"short-blob":          handle(tree, blob[:6], "all", "cache/demux.go"),
		"traversal":           handle(tree, blob, "all", "../outside/demux.go"),
		"inner-traversal":     handle(tree, blob, "all", "cache/../cache/demux.go"),
		"absolute":            handle(tree, blob, "all", "/etc/passwd"),
		"leading-dash":        handle(tree, blob, "all", "-cache"),
		"control":             handle(tree, blob, "all", "cache/demux.go\n"),
		"non-utf8":            handle(tree, blob, "all", "cache/\xff.go"),
		"oversize":            handle(tree, blob, "all", strings.Repeat("a/", 2200)+"x"),
		"huge-range":          handle(tree, blob, "1-999999999", "cache/demux.go"),
		"overflow-range":      handle(tree, blob, "1-99999999999999999999", "cache/demux.go"),
		"reversed-range":      handle(tree, blob, "3-2", "cache/demux.go"),
		"zero-range":          handle(tree, blob, "0-2", "cache/demux.go"),
		"blob-of-other-path":  handle(tree, other, "all", "cache/demux.go"),
		"directory":           handle(tree, blob, "all", "cache"),
		"missing-path":        handle(tree, blob, "all", "cache/absent.go"),
		"missing-object":      handle(tree, "0000000", "all", "cache/demux.go"),
		"non-utf8-source":     handle(tree, latin, "all", "cache/latin1.txt"),
		"over-max-bytes":      handle(tree, blob, "all", "cache/demux.go") + "\x00max=8",
		"stale-other-tree":    handle(strings.Repeat("a", len(tree)), blob, "all", "cache/demux.go"),
		"empty-handle-string": "",
	}
	want := map[string]string{
		"no-prefix": "invalid-handle", "short-tree": "invalid-handle", "upper-tree": "invalid-handle", "short-blob": "invalid-handle",
		"traversal": "invalid-handle", "inner-traversal": "invalid-handle", "absolute": "invalid-handle", "leading-dash": "invalid-handle",
		"control": "invalid-handle", "non-utf8": "invalid-handle", "oversize": "invalid-handle", "huge-range": "invalid-handle",
		"overflow-range": "invalid-handle", "reversed-range": "invalid-handle", "zero-range": "invalid-handle",
		"blob-of-other-path": "invalid-handle", "directory": "invalid-handle", "missing-path": "missing-handle",
		"missing-object": "missing-handle", "non-utf8-source": "unsupported-text", "over-max-bytes": "expand-budget",
		"stale-other-tree": "stale-handle", "empty-handle-string": "argument",
	}
	for name, text := range cases {
		arguments := []string{"--root", root, "context", "--expand", text}
		if before, found := strings.CutSuffix(text, "\x00max=8"); found {
			arguments = []string{"--root", root, "context", "--expand", before, "--max-bytes", "8"}
		}
		code, stdout, stderr := runContextCommand(t, arguments...)
		if code != 2 || len(stdout) != 0 {
			t.Fatalf("%s: exit %d stdout %q", name, code, stdout)
		}
		if want[name] != "argument" && !strings.Contains(stderr, `"code": "`+want[name]+`"`) {
			t.Fatalf("%s: stderr %s, want %s", name, stderr, want[name])
		}
	}
}

func TestContextExpandRefusesAStaleHandleAfterHeadMoves(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	_, summary, _ := runContextCommand(t, "--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go", "--summary")
	handle := decodeObject(t, summary)["results"].([]any)[0].(map[string]any)["handle"].(string)
	if code, _, stderr := runContextCommand(t, "--root", root, "context", "--expand", handle); code != 0 {
		t.Fatalf("fresh handle refused: %s", stderr)
	}
	writeSummaryFixture(t, root, "cache/keys.go", "package cache\n")
	gitFixture(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qam", "move head")
	code, stdout, stderr := runContextCommand(t, "--root", root, "context", "--expand", handle)
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"code": "stale-handle"`) {
		t.Fatalf("stale handle: exit %d stdout %q stderr %s", code, stdout, stderr)
	}
}

// TestContextExpandRefusesAnAmbiguousAbbreviatedBlob writes two blobs whose
// object IDs share their first seven hex digits; the handle naming that prefix
// refuses instead of resolving it through the current tree.
func TestContextExpandRefusesAnAmbiguousAbbreviatedBlob(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	seen := map[string]string{}
	var first, second, prefix string
	for index := 0; first == ""; index++ {
		content := fmt.Sprintf("collision candidate %d\n", index)
		object := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content)))
		key := hex.EncodeToString(object[:])[:7]
		if earlier, ok := seen[key]; ok {
			first, second, prefix = earlier, content, key
		}
		seen[key] = content
	}
	writeSummaryFixture(t, root, "collide.txt", first)
	writeSummaryFixture(t, root, "other.txt", second)
	gitFixture(t, root, "add", "-A")
	gitFixture(t, root, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "collide")
	tree := gitFixture(t, root, "rev-parse", "HEAD^{tree}")
	code, stdout, stderr := runContextCommand(t, "--root", root, "context", "--expand", "cv1:"+tree+":"+prefix+":all:collide.txt")
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"code": "ambiguous-handle"`) {
		t.Fatalf("ambiguous prefix %s: exit %d stdout %q stderr %s", prefix, code, stdout, stderr)
	}
	full := gitFixture(t, root, "rev-parse", "HEAD:collide.txt")
	if code, _, stderr := runContextCommand(t, "--root", root, "context", "--expand", "cv1:"+tree+":"+full+":all:collide.txt"); code != 0 {
		t.Fatalf("full blob identity refused: %s", stderr)
	}
}

func TestContextSummaryAndExpandAreReadOnly(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	before := repositoryListing(t, root)
	_, summary, _ := runContextCommand(t, "--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go", "--summary")
	handle := decodeObject(t, summary)["results"].([]any)[1].(map[string]any)["handle"].(string)
	runContextCommand(t, "--root", root, "context", "--expand", handle)
	runContextCommand(t, "--root", root, "context", "--expand", handle+"x")
	runContextCommand(t, "--root", root, "context", "--task", evidenceSummaryTask, "--summary", "--summary-bytes", "1024")
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
		t.Fatalf(".corvint written by a read verb: %v", err)
	}
	if after := repositoryListing(t, root); after != before {
		t.Fatalf("summary or expand wrote to the repository:\n%s\n%s", before, after)
	}
}

func TestParseContextViewArguments(t *testing.T) {
	t.Parallel()
	root := evidenceSummaryRepository(t)
	options, _, err := parseTaskContextInvocation([]string{"--root", root, "context", "--task", "t", "--summary", "--summary-bytes", "2048"})
	if err != nil || !options.summary || options.summaryBytes != 2048 {
		t.Fatalf("summary options = %+v, %v", options, err)
	}
	options, _, err = parseTaskContextInvocation([]string{"--root", root, "context", "--expand", "cv1:x", "--max-bytes", "10"})
	if err != nil || options.expand != "cv1:x" || options.maxBytes != 10 {
		t.Fatalf("expand options = %+v, %v", options, err)
	}
	for _, arguments := range [][]string{
		{"context", "--expand", "cv1:x", "--task", "t"},
		{"context", "--expand", "cv1:x", "--summary"},
		{"context", "--expand", "cv1:x", "--limit", "3"},
		{"context", "--task", "t", "--max-bytes", "10"},
		{"context", "--task", "t", "--summary-bytes", "2048"},
		{"context", "--summary"},
	} {
		if _, isContext, err := parseTaskContextInvocation(append([]string{"--root", root}, arguments...)); !isContext || err == nil {
			t.Fatalf("%v: isContext=%v err=%v", arguments, isContext, err)
		}
	}
}

// TestSourceViewNativeSourceDigestsAreFrozen resolves ESV-V0-005: the
// manifest's native digests bind the exact source-view implementation files.
func TestSourceViewNativeSourceDigestsAreFrozen(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../benchmarks/selfuse-batch/source-views-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		CurrentState struct {
			NativeSourceDigests struct {
				Files []struct{ Path, SHA256 string } `json:"files"`
			} `json:"nativeSourceDigests"`
		} `json:"currentState"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	files := manifest.CurrentState.NativeSourceDigests.Files
	if len(files) != 2 {
		t.Fatalf("native source digests = %+v", files)
	}
	for _, file := range files {
		source, err := os.ReadFile(filepath.Join("../..", filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(source)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatalf("%s changed without re-freezing its native source digest (%x)", file.Path, sum)
		}
	}
}
