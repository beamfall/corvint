package unplannedread

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func worktree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"planned.go":              "package main\n",
		"internal/deep/hidden.go": strings.Repeat("x", 512),
		".gitignore":              ".corvint/\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// URE-V0-001: a read of a packet path, or of a file under a packet directory,
// is planned; anything else project-relative is unplanned and sized.
func TestClassifyPlannedAndUnplanned(t *testing.T) {
	root := worktree(t)
	packet := []string{"planned.go", "internal/deep"}

	event, ok := Classify(packet, "Read", map[string]any{"file_path": "planned.go"}, root)
	if !ok || !event.Planned || event.Path != "" || event.Bytes != 0 {
		t.Fatalf("exact packet path should be a pathless planned row: %+v ok=%v", event, ok)
	}

	event, ok = Classify(packet, "Read", map[string]any{"file_path": filepath.Join(root, "internal/deep/hidden.go")}, root)
	if !ok || !event.Planned {
		t.Fatalf("directory prefix should count as planned: %+v ok=%v", event, ok)
	}

	event, ok = Classify([]string{"planned.go"}, "Read", map[string]any{"file_path": "internal/deep/hidden.go"}, root)
	if !ok || event.Planned || event.Path != "internal/deep/hidden.go" || event.Bytes != 512 || !event.SizeKnown {
		t.Fatalf("unplanned read should be sized and pathed: %+v ok=%v", event, ok)
	}
	if event.PacketDigest == "" {
		t.Fatal("unplanned row should name the packet it was judged against")
	}

	if _, ok := Classify(packet, "Read", map[string]any{"file_path": "/etc/hosts"}, root); ok {
		t.Fatal("a path outside the worktree is not classified")
	}
	if _, ok := Classify(packet, "Edit", map[string]any{"file_path": "planned.go"}, root); ok {
		t.Fatal("a write tool is not a read")
	}
}

// URE-V0-001: on a case-insensitive volume a case-aliased read is judged and
// ranked under the spelling its directory stores; an absent suffix keeps its own.
func TestClassifyStoredSpelling(t *testing.T) {
	root := worktree(t)
	if _, err := os.Lstat(filepath.Join(root, "PLANNED.GO")); err != nil {
		t.Skip("case-sensitive volume: no case alias exists")
	}
	event, ok := Classify([]string{"planned.go"}, "Read", map[string]any{"file_path": filepath.Join(root, "PLANNED.GO")}, root)
	if !ok || !event.Planned {
		t.Fatalf("a case alias of a packet path must be planned: %+v ok=%v", event, ok)
	}
	event, ok = Classify(nil, "Read", map[string]any{"file_path": "INTERNAL/Deep/HIDDEN.GO"}, root)
	if !ok || event.Path != "internal/deep/hidden.go" {
		t.Fatalf("an unplanned alias must carry the stored spelling: %+v ok=%v", event, ok)
	}
	event, ok = Classify(nil, "Read", map[string]any{"file_path": "INTERNAL/Missing.go"}, root)
	if !ok || event.Path != "internal/Missing.go" {
		t.Fatalf("an absent component keeps the caller's spelling: %+v ok=%v", event, ok)
	}
}

// URE-V0-001: absence never substitutes for resolved project containment.
func TestClassifyAbsentPathContainment(t *testing.T) {
	root, outside := worktree(t), t.TempDir()
	for name, target := range map[string]string{
		"escape":          outside,
		"contained":       root,
		"dangling-escape": filepath.Join(outside, "missing.go"),
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		path string
		want string
	}{
		{"escape/missing.go", ""},
		{"contained/absent/deep.go", "absent/deep.go"},
		{"dangling-escape", ""},
	} {
		t.Run(test.path, func(t *testing.T) {
			event, ok := Classify(nil, "Read", map[string]any{"file_path": test.path}, root)
			if ok != (test.want != "") || event.Path != test.want || event.SizeKnown || event.Bytes != 0 {
				t.Fatalf("event=%+v ok=%v, want path %q with unknown size", event, ok, test.want)
			}
		})
	}
}

// URE-V0-002: the bounded Bash heuristic recognizes cat/sed/head/tail file
// operands and nothing else.
func TestBashHeuristic(t *testing.T) {
	root := worktree(t)
	cases := map[string]string{
		"cat internal/deep/hidden.go":                       "internal/deep/hidden.go",
		"sed -n '1,20p' internal/deep/hidden.go":            "internal/deep/hidden.go",
		"head -n 5 internal/deep/hidden.go":                 "internal/deep/hidden.go",
		"git status --short | tail internal/deep/hidden.go": "internal/deep/hidden.go",
	}
	for command, want := range cases {
		event, ok := Classify(nil, "Bash", map[string]any{"command": command}, root)
		if !ok || event.Path != want {
			t.Fatalf("%q: got %+v ok=%v, want path %q", command, event, ok, want)
		}
	}
	for _, command := range []string{"go test ./...", "rm -rf internal/deep", "cat -n"} {
		if _, ok := Classify(nil, "Bash", map[string]any{"command": command}, root); ok {
			t.Fatalf("%q should not classify as a read", command)
		}
	}
}

// URE-V0-002: an operand the whitespace split cannot read literally abstains
// rather than inventing an unplanned read of a path no call opened.
func TestBashHeuristicNeverInventsPath(t *testing.T) {
	root := worktree(t)
	if event, ok := Classify(nil, "Bash", map[string]any{"command": "tail -n +5 internal/deep/hidden.go"}, root); !ok || event.Path != "internal/deep/hidden.go" {
		t.Fatalf("tail +N: got %+v ok=%v", event, ok)
	}
	for _, command := range []string{
		"cat < internal/deep/hidden.go",
		"cat <<EOF",
		"cat $FILE",
		"cat *.go",
		`cat "my file.txt"`,
		"sed 's/a b c/d/' internal/deep/hidden.go",
		"sed -n -e 's/a/b/' -e 's/c/d/p' internal/deep/hidden.go",
		// A flag value or script the split took for an operand names no file.
		"sed -i '' 's/x/y/' planned.go",
		"head -c 1k planned.go",
	} {
		if event, ok := Classify(nil, "Bash", map[string]any{"command": command}, root); ok {
			t.Fatalf("%q invented read %+v", command, event)
		}
	}
}

// URE-V0-002: a segment after an unclosed quote, a heredoc, or a directory
// change is not a command whose operand is relative to the worktree root.
func TestBashLaterSegmentContext(t *testing.T) {
	root := worktree(t)
	if event, ok := Classify(nil, "Bash", map[string]any{"command": "echo 'a|b'; cat planned.go"}, root); !ok || event.Path != "planned.go" {
		t.Fatalf("a closed quote keeps later segments literal: got %+v ok=%v", event, ok)
	}
	for _, command := range []string{
		"git commit -F - <<EOF\nfix: cleanup\ncat planned.go\nEOF",
		"python3 -c 'import os\ncat planned.go\n'",
		"cd internal && cat planned.go",
		"(pushd internal; head planned.go)",
	} {
		if event, ok := Classify(nil, "Bash", map[string]any{"command": command}, root); ok {
			t.Fatalf("%q invented read %+v", command, event)
		}
	}
}

// URE-V0-002: a relative operand resolves from the post-tool payload's cwd,
// which the host moves after a `cd`, never silently from the worktree root.
func TestHookResolvesRelativeOperandFromPayloadCwd(t *testing.T) {
	root := worktree(t)
	if err := os.WriteFile(filepath.Join(root, "internal", "planned.go"), []byte("package internal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Enable(root); err != nil {
		t.Fatal(err)
	}
	for _, cwd := range []string{filepath.Join(root, "internal"), t.TempDir()} {
		HookPostTool(root, nil, map[string]any{"cwd": cwd, "tool_name": "Bash", "tool_input": map[string]any{"command": "cat planned.go"}})
	}
	// The payload cwd is taken after the command, so a later `cd` moves it.
	HookPostTool(root, nil, map[string]any{"cwd": root, "tool_name": "Bash", "tool_input": map[string]any{"command": "cat planned.go; cd internal"}})
	digest, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if digest.Unplanned != 1 || digest.TopPaths[0].Path != "internal/planned.go" {
		t.Fatalf("want one read of internal/planned.go and an outside-cwd abstention: %+v", digest)
	}
}

// URE-V0-003, URE-V0-004: a marker never enables a ledger git would track, so
// a marker committed to a clone without the ignore entries writes nothing.
func TestLedgerRequiresIgnoreEntries(t *testing.T) {
	for name, ignore := range map[string]string{"none": "", "ledger only": ".corvint/unplanned-reads.jsonl\n"} {
		t.Run(name, func(t *testing.T) {
			root := worktree(t)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(ignore), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := Enable(root); err == nil {
				t.Fatal("enable must refuse a ledger git would track")
			}
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".corvint", markerName), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			HookPostTool(root, nil, map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": "planned.go"}})
			RecordPacket(root, "session-a", []string{"planned.go"})
			if entries, _ := os.ReadDir(filepath.Join(root, ".corvint")); len(entries) != 1 {
				t.Fatalf("an unignored ledger was written: %d entries", len(entries))
			}
		})
	}
}

// URE-V0-003: without the opt-in marker nothing is written.
func TestAppendDisabledIsNoOp(t *testing.T) {
	root := worktree(t)
	event, _ := Classify(nil, "Read", map[string]any{"file_path": "planned.go"}, root)
	if err := Append(root, event); err != nil {
		t.Fatalf("disabled append must be a silent no-op: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint", ledgerName)); !os.IsNotExist(err) {
		t.Fatalf("disabled append created a file: %v", err)
	}
}

// URE-V0-004: with the marker present rows append and the file stays under
// the self-observation ledger's own byte cap.
func TestAppendEnabledRotatesAtCap(t *testing.T) {
	root := worktree(t)
	if err := Enable(root); err != nil {
		t.Fatal(err)
	}
	event, _ := Classify(nil, "Read", map[string]any{"file_path": "internal/deep/hidden.go"}, root)
	if err := Append(root, event); err != nil {
		t.Fatal(err)
	}
	existing, err := os.ReadFile(filepath.Join(root, ".corvint", ledgerName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", ledgerName), bytes.Repeat(existing, maxFileBytes/len(existing)+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, event); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", ledgerName))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > maxFileBytes {
		t.Fatalf("ledger grew past the cap: %d bytes", len(data))
	}
	if len(data) == 0 || !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatal("ledger should retain whole newest rows")
	}
	if err := Disable(root); err != nil {
		t.Fatal(err)
	}
	if Enabled(root) {
		t.Fatal("disable must remove the marker")
	}
}

type countingReaderAt struct {
	io.ReaderAt
	bytesRead int
}

func (reader *countingReaderAt) ReadAt(buffer []byte, offset int64) (int, error) {
	count, err := reader.ReaderAt.ReadAt(buffer, offset)
	reader.bytesRead += count
	return count, err
}

// URE-V0-004: repairing an externally oversized ledger preserves the prior
// oldest-first truncation semantics without reading more than the retained half.
func TestAppendRepairReadIsBounded(t *testing.T) {
	root := worktree(t)
	if err := Enable(root); err != nil {
		t.Fatal(err)
	}
	oldRow := []byte("{\"old\":true}\n")
	oversized := bytes.Repeat(oldRow, maxFileBytes*4/len(oldRow)+1)
	path := filepath.Join(root, ".corvint", ledgerName)
	if err := os.WriteFile(path, oversized, 0o600); err != nil {
		t.Fatal(err)
	}
	encoded := []byte("{\"new\":true}")

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := &countingReaderAt{ReaderAt: file}
	previous, err := readPreviousRows(reader, int64(len(oversized)), len(encoded)+1)
	if err != nil {
		t.Fatal(err)
	}
	if reader.bytesRead > maxFileBytes/2 {
		t.Fatalf("repair read %d bytes, want at most %d", reader.bytesRead, maxFileBytes/2)
	}
	wantPrevious := keepNewest(oversized, maxFileBytes/2)
	if !bytes.Equal(previous, wantPrevious) {
		t.Fatal("bounded repair changed oldest-first truncation semantics")
	}

	if err := appendRow(root, encoded); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append(append(wantPrevious, encoded...), '\n')
	if !bytes.Equal(got, want) {
		t.Fatal("repaired ledger differs from prior append semantics")
	}
	if len(got) > maxFileBytes {
		t.Fatalf("repaired ledger grew past the cap: %d bytes", len(got))
	}
}

// URE-V0-005: the digest counts, sums, ranks and computes the unplanned share.
func TestDigestMath(t *testing.T) {
	root := worktree(t)
	if err := Enable(root); err != nil {
		t.Fatal(err)
	}
	unplanned, _ := Classify(nil, "Read", map[string]any{"file_path": "internal/deep/hidden.go"}, root)
	small, _ := Classify(nil, "Grep", map[string]any{"path": "planned.go"}, root)
	plannedRow, _ := Classify([]string{"planned.go"}, "Read", map[string]any{"file_path": "planned.go"}, root)
	for _, event := range []Event{unplanned, small, plannedRow, plannedRow} {
		if err := Append(root, event); err != nil {
			t.Fatal(err)
		}
	}
	digest, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if digest.Unplanned != 2 || digest.Planned != 2 {
		t.Fatalf("counts: %+v", digest)
	}
	if digest.TotalBytes != 512+13 {
		t.Fatalf("total bytes: %d", digest.TotalBytes)
	}
	if digest.Ratio() != "0.500" {
		t.Fatalf("ratio: %s", digest.Ratio())
	}
	if digest.Tools["Read"] != 1 || digest.Tools["Grep"] != 1 {
		t.Fatalf("per-tool counts: %+v", digest.Tools)
	}
	if len(digest.TopPaths) != 2 || digest.TopPaths[0].Path != "internal/deep/hidden.go" {
		t.Fatalf("top paths: %+v", digest.TopPaths)
	}
	var rendered bytes.Buffer
	if err := Render(root, 120, &rendered); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "ratio=0.500") {
		t.Fatalf("render: %q", rendered.String())
	}
}

// URE-V0-006: the hook never returns an error, enabled or not, whatever the
// payload shape.
func TestHookPostToolNeverErrors(t *testing.T) {
	root := worktree(t)
	payloads := []map[string]any{
		{},
		{"tool_name": "Read"},
		{"tool_name": 7, "tool_input": map[string]any{"file_path": "planned.go"}},
		{"tool_name": "Read", "tool_input": map[string]any{"file_path": "internal/deep/hidden.go"}},
		{"tool_name": "Read", "tool_input": map[string]any{"file_path": strings.Repeat("a/", 4000) + "b.go"}},
		// Valid absent components whose JSON escaping exceeds the row bound.
		{"tool_name": "Read", "tool_input": map[string]any{"file_path": strings.Repeat("\x01/", 400) + "b.go"}},
	}
	for _, enabled := range []bool{false, true} {
		if enabled {
			if err := Enable(root); err != nil {
				t.Fatal(err)
			}
		}
		for index, payload := range payloads {
			if err := HookPostTool(root, []string{"planned.go"}, payload); err != nil {
				t.Fatalf("enabled=%v payload %d returned %v", enabled, index, err)
			}
		}
	}
	digest, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if digest.Unplanned != 1 {
		t.Fatalf("only the one well-formed unplanned read should be recorded: %+v", digest)
	}
	if digest.LastError == "" {
		t.Fatal("the dropped oversize-row error should surface as LastError")
	}
}

// URE-V0-005, URE-V0-006: a hook drop in one process reaches a digest rendered
// by a fresh process, as a reason code and count that name no path.
func TestDroppedHookErrorSurfacesAcrossProcesses(t *testing.T) {
	if readerRoot := os.Getenv("URE_DIGEST_READER_ROOT"); readerRoot != "" {
		if err := Render(readerRoot, 120, os.Stdout); err != nil {
			t.Fatal(err)
		}
		return
	}
	root := worktree(t)
	if err := Enable(root); err != nil {
		t.Fatal(err)
	}
	oversized := strings.Repeat("\x01/", 400) + "b.go"
	for range 2 {
		HookPostTool(root, nil, map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": oversized}})
	}
	reader := exec.Command(os.Args[0], "-test.run=^TestDroppedHookErrorSurfacesAcrossProcesses$", "-test.count=1")
	reader.Env = append(os.Environ(), "URE_DIGEST_READER_ROOT="+root)
	output, err := reader.CombinedOutput()
	if err != nil {
		t.Fatalf("reader process: %v\n%s", err, output)
	}
	rendered := string(output)
	if !strings.Contains(rendered, "unplanned=0 planned=0") {
		t.Fatalf("error rows must not count as reads: %q", rendered)
	}
	if !strings.Contains(rendered, "LAST-ERROR reason=row-exceeds-bound count=2") {
		t.Fatalf("the dropped error should reach another process's digest: %q", rendered)
	}
	if strings.Contains(rendered, "b.go") {
		t.Fatalf("the error row must name no path: %q", rendered)
	}
}

// URE-V0-008, URE-V0-009: a post-tool read is judged only against a packet
// recorded for the same session; with none it abstains, and packet rows never
// count as reads.
func TestSessionPacketDenominator(t *testing.T) {
	root := worktree(t)
	if err := Enable(root); err != nil {
		t.Fatal(err)
	}
	read := func(path string) map[string]any {
		return map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": path}}
	}
	HookPostToolSession(root, "session-a", read("planned.go"))
	if digest, _ := Read(root); digest.Unplanned+digest.Planned != 0 {
		t.Fatalf("a read with no recorded packet must abstain: %+v", digest)
	}
	RecordPacket(root, "session-a", []string{"planned.go", "internal/deep/"})
	HookPostToolSession(root, "session-a", read("internal/deep/hidden.go"))
	HookPostToolSession(root, "session-b", read("planned.go"))
	RecordPacket(root, "session-a", []string{"internal/deep"})
	HookPostToolSession(root, "session-a", read("planned.go"))
	digest, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if digest.Planned != 1 || digest.Unplanned != 1 || digest.TopPaths[0].Path != "planned.go" {
		t.Fatalf("want one planned and one unplanned read against the newest same-session packet: %+v", digest)
	}
	ledger, _ := os.ReadFile(filepath.Join(root, ".corvint", ledgerName))
	if bytes.Contains(ledger, []byte("internal/deep\"")) || bytes.Contains(ledger, []byte("session-a")) {
		t.Fatalf("packet rows must carry hashes, not packet paths or session ids: %s", ledger)
	}
}

// URE-V0-008: a refused packet row supersedes the session's older packet, so
// later reads abstain instead of scoring against a stale planned set.
func TestRefusedPacketAbstains(t *testing.T) {
	root := worktree(t)
	if err := Enable(root); err != nil {
		t.Fatal(err)
	}
	RecordPacket(root, "session-a", []string{"planned.go"})
	oversized := []string{"internal/deep"}
	for index := 0; index < 200; index++ {
		oversized = append(oversized, "internal/deep/file"+strings.Repeat("x", index%7))
	}
	RecordPacket(root, "session-a", oversized)
	HookPostToolSession(root, "session-a", map[string]any{"tool_name": "Read", "tool_input": map[string]any{"file_path": "internal/deep/hidden.go"}})
	digest, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if digest.Unplanned+digest.Planned != 0 || digest.LastError != "row-exceeds-bound" {
		t.Fatalf("a read after a refused packet must abstain and the refusal surface: %+v", digest)
	}
}

// URE-V0-004, URE-V0-005, URE-V0-009: a symlinked .corvint or ledger never
// moves a write, a previous-row read, a digest or a packet lookup outside the worktree.
func TestSymlinkedLedgerIsRefused(t *testing.T) {
	event := Event{Tool: "Read", Path: "planned.go"}
	t.Run("ledger", func(t *testing.T) {
		root, outside := worktree(t), t.TempDir()
		secret := filepath.Join(outside, "secret")
		outsideRows := `{"kind":"packet","session":"` + shortHash("session-a") + `","packet":"outside","paths_sha256":[]}` + "\n" +
			`{"tool":"Read","path":"outside.go","bytes":7}` + "\n"
		if err := os.WriteFile(secret, []byte(outsideRows), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Enable(root); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(secret, filepath.Join(root, ".corvint", ledgerName)); err != nil {
			t.Fatal(err)
		}
		if err := Append(root, event); err == nil {
			t.Fatal("append through a symlinked ledger must fail")
		}
		if data, _ := os.ReadFile(filepath.Join(root, ".corvint", ledgerName)); bytes.Contains(data, []byte("planned.go")) {
			t.Fatalf("ledger was replaced through the link: %q", data)
		}
		if digest, err := Read(root); err == nil {
			t.Fatalf("a digest through a symlinked ledger must fail: %+v", digest)
		}
		if packet, found := newestPacket(root, shortHash("session-a")); found {
			t.Fatalf("packet lookup through a symlinked ledger must abstain: %+v", packet)
		}
	})
	t.Run("directory", func(t *testing.T) {
		root, outside := worktree(t), t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, ".corvint")); err != nil {
			t.Fatal(err)
		}
		if err := Enable(root); err == nil {
			t.Fatal("enable through a symlinked .corvint must fail")
		}
		if err := os.WriteFile(filepath.Join(outside, markerName), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Append(root, event); err == nil {
			t.Fatal("append through a symlinked .corvint must fail")
		}
		if entries, _ := os.ReadDir(outside); len(entries) != 1 {
			t.Fatalf("writes escaped the worktree: %d entries", len(entries))
		}
	})
}

// URE-V0-009: session lookup reads only the retained ledger tail. A complete
// packet there is usable; a packet outside the bounded tail remains unknown.
func TestNewestPacketBoundsOversizeLedger(t *testing.T) {
	t.Run("URE-V0-009 bounded tail", func(t *testing.T) {
		root := worktree(t)
		if err := Enable(root); err != nil {
			t.Fatal(err)
		}
		encode := func(session, digest string) []byte {
			t.Helper()
			row, err := json.Marshal(packetRow{
				Kind:         packetKind,
				Session:      shortHash(session),
				PacketDigest: digest,
				PathHashes:   []string{shortHash("planned.go")},
			})
			if err != nil {
				t.Fatal(err)
			}
			return append(row, '\n')
		}
		paddingRow := []byte("{\"kind\":\"read\"}\n")
		ledger := append(encode("older-session", "older"), bytes.Repeat(paddingRow, maxFileBytes/len(paddingRow)+1)...)
		ledger = append(ledger, encode("tail-session", "tail")...)
		if err := os.WriteFile(filepath.Join(root, ".corvint", ledgerName), ledger, 0o600); err != nil {
			t.Fatal(err)
		}

		packet, found := newestPacket(root, shortHash("tail-session"))
		if !found || packet.PacketDigest != "tail" {
			t.Fatalf("newest complete tail packet not found: %+v found=%v", packet, found)
		}
		if _, found := newestPacket(root, shortHash("older-session")); found {
			t.Fatal("packet outside the bounded tail must remain unknown")
		}
	})
}
