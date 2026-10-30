package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// TestProposeCitationsDCWV0803CitesASurvivingLineForEachMovedFile pins
// V1-3268: moving a package directory drafts no unknown row for either side of
// any moved file while a surviving BASE line names the old path, both sides of
// a move cite one span, no row cites a span in a deleted file or a line the
// change edits, and the real prepare and cite pre-flight admits every row,
// both before a map exists and against the prepared map.
func TestProposeCitationsDCWV0803CitesASurvivingLineForEachMovedFile(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	source := func(name string, lines int) string {
		return "package main\n\n// " + name + " serves.\nfunc " + name + "() int {\n" + numbered("\t// "+strings.ToLower(name), lines) + "\treturn 1\n}\n"
	}
	readme := "# Svc\n\n" + numbered("readme", 12)
	writeTree(t, root, map[string]string{
		"README.md":                 readme,
		"docs/specs/svc-v0.md":      "# Svc v0\n\n## Requirements\n\n- `SVC-V0-001`: The server package cmd/svc-server/ answers every request.\n- `SVC-V0-002`: It reads one config file.\n",
		"docs/SERVER.md":            "# Server\n\nThe entry point is cmd/svc-server/main.go.\n\nStart it with go run ./cmd/svc-server.\n",
		"cmd/svc-server/main.go":    source("Main", 6),
		"cmd/svc-server/handler.go": source("Handle", 8),
		"cmd/svc-server/config.go":  source("Config", 5),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, "internal/svc"), 0o777); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "mv", "cmd/svc-server", "internal/svc/cli")
	for _, name := range []string{"main.go", "handler.go", "config.go"} {
		file := filepath.Join(root, "internal/svc/cli", name)
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		writeTree(t, root, map[string]string{"internal/svc/cli/" + name: strings.Replace(string(data), "package main", "package cli", 1)})
	}
	writeTree(t, root, map[string]string{"README.md": strings.Replace(readme, "readme line 6\n", "readme line 6 amended\n", 1)})
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "move the server package")

	edited := diffHunks(t, root, base)
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	draft := stdout.String()
	rows := textLines(stdout.Bytes())
	if len(rows) != 7 || strings.Contains(draft, "\tunknown\t") {
		t.Fatalf("draft has %d rows or an unknown row, want 7 cited rows:\n%s\n%s", len(rows), draft, stderr.String())
	}
	cites := map[string]string{}
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		start, _ := strconv.Atoi(strings.Split(fields[2], ":")[0])
		if strings.HasPrefix(fields[1], "cmd/svc-server/") || strings.HasPrefix(fields[1], "internal/svc/") {
			t.Fatalf("row cites a span in a moved file: %q", row)
		}
		for _, hunk := range edited {
			if hunk.Path == fields[1] && start >= hunk.OldRange.Start && start < hunk.OldRange.Start+hunk.OldRange.Count {
				t.Fatalf("row cites a line the change edits: %q", row)
			}
		}
		cites[fields[0]] = fields[1] + ":" + fields[2]
	}
	plan := writeInput(t, draft)
	code, preStdout, preStderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan})
	if code != 0 || !strings.Contains(preStdout, "prepare PASS") || !strings.Contains(preStdout, "inputs-check PASS") {
		t.Fatalf("the prepared map does not admit the draft: code=%d\n%s\n%s", code, preStdout, preStderr)
	}

	// Both sides of each move cite one span, read through the prepared map.
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: testGit(t, root, "rev-parse", "HEAD")}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	var prepared struct {
		Hunks []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"hunks"`
	}
	if err := json.Unmarshal(data, &prepared); err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, hunk := range prepared.Hunks {
		byPath[hunk.Path] = cites[hunk.ID]
	}
	for _, name := range []string{"main.go", "handler.go", "config.go"} {
		old, moved := byPath["cmd/svc-server/"+name], byPath["internal/svc/cli/"+name]
		if old == "" || old != moved {
			t.Errorf("the sides of the %s move cite %q and %q, want one span:\n%s", name, old, moved, draft)
		}
	}

	// Against the prepared map the ordinal-keyed draft is the same.
	stdout.Reset()
	stderr.Reset()
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations with a map = %d, %v\n%s", code, err, stderr.String())
	}
	if rows := textLines(stdout.Bytes()); len(rows) != 7 || strings.Contains(stdout.String(), "\tunknown\t") {
		t.Fatalf("map draft:\n%s\n%s", stdout.String(), stderr.String())
	}
}

// TestMovedFilesDCWV0803PairsOnlyUnambiguousMoves: a deletion and an addition
// pair by their longest shared path suffix; two equally good partners leave
// both unpaired.
func TestMovedFilesDCWV0803PairsOnlyUnambiguousMoves(t *testing.T) {
	derived := map[string]*patch.Hunk{}
	for index, pair := range [][2]string{
		{"cmd/a/x/main.go", ""}, {"", "internal/a/x/main.go"},
		{"cmd/b/util.go", ""}, {"cmd/c/util.go", ""}, {"", "pkg/util.go"},
		{"edit.go", "edit.go"},
	} {
		hunk := &patch.Hunk{}
		if pair[0] != "" {
			hunk.OldPath = &pair[0]
		}
		if pair[1] != "" {
			hunk.NewPath = &pair[1]
		}
		derived[strconv.Itoa(index)] = hunk
	}
	moves := movedFiles(derived)
	if move := moves["internal/a/x/main.go"]; move.old != "cmd/a/x/main.go" || move.oldRoot != "cmd" || moves["cmd/a/x/main.go"] != move {
		t.Fatalf("main.go move = %+v", move)
	}
	if got := moves["cmd/a/x/main.go"].terms(); strings.Join(got, ",") != "cmd/a/x/main.go,cmd/a/x,cmd/a" {
		t.Fatalf("terms = %q", got)
	}
	if _, paired := moves["pkg/util.go"]; paired || len(moves) != 2 {
		t.Fatalf("an ambiguous move was paired: %+v", moves)
	}
	for text, want := range map[string]bool{
		"go run ./cmd/a.":          true,
		"see cmd/a/x/main.go here": true,
		"github.com/o/r/cmd/a\"":   true,
		"cmd/ab is another":        false,
		"xcmd/a":                   false,
		"cmd/a.go":                 false,
	} {
		if got := namesPathOrDirectory([]byte(text), "cmd/a"); got != want {
			t.Errorf("namesPathOrDirectory(%q) = %v, want %v", text, got, want)
		}
	}
}
