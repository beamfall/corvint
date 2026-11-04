package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// DCW-V0-803 amended by DWC-V0-926: a move may cite a surviving path or
// basename anchor; directory-only matches cannot justify a draft.
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
		"README.md": readme,
		// An untouched document names README.md: DCW-V0-393 never drafts the
		// edited README's own lines (batch integration with DCW-V0-803).
		"docs/INDEX.md":             "# Index\n\nStart with README.md.\n",
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

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	// DWC-V0-926/927 supersede unanchored same-file, move, sibling and seal
	// guesses. Preserve the full scenario and check every remaining cite with
	// the CEM verifier; missing anchors must be explicit unknown rows.
	assertAnchoredDraft(t, root, base, stdout.String())

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
	for _, tc := range []struct {
		text, term string
		want       bool
	}{
		{"go run ./cmd/a.", "cmd/a", true},
		{"see cmd/a/x/main.go here", "cmd/a", true},
		{"github.com/o/r/cmd/a\"", "cmd/a", true},
		{"cmd/ab is another", "cmd/a", false},
		{"xcmd/a", "cmd/a", false},
		{"cmd/a.go", "cmd/a", false},
		// A parent directory or another module names a different file.
		{"see tools/cmd/svc/main.go", "cmd/svc/main.go", false},
		{"go run ../cmd/a", "cmd/a", false},
		{"github.com/other/cmd/a", "cmd/a", false},
		{"xgithub.com/o/r/cmd/a", "cmd/a", false},
		{"tools/cmd/svc/main.go and cmd/svc/main.go", "cmd/svc/main.go", true},
	} {
		if got := namesPathOrDirectory([]byte(tc.text), tc.term, "github.com/o/r"); got != tc.want {
			t.Errorf("namesPathOrDirectory(%q, %q) = %v, want %v", tc.text, tc.term, got, tc.want)
		}
	}
	if namesPathOrDirectory([]byte("github.com/o/r/cmd/a"), "cmd/a", "") {
		t.Error("a module-qualified path matched without a BASE go.mod")
	}
}

// TestProposeCitationsDCWV0803DraftsDistinctSpansForAMoveAlone pins the
// review findings on V1-3268: when a move's two hunks are the change's only
// hunks, the two sides cite distinct naming lines, so the uniform-span guard
// leaves no unknown row; and a line naming tools/cmd/svc/main.go, a different
// file, is never cited for cmd/svc/main.go.
func TestProposeCitationsDCWV0803DraftsDistinctSpansForAMoveAlone(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{
		"go.mod":                "module example.com/svc\n\ngo 1.22\n",
		"docs/a-tools.md":       "# Tools\n\nThe stub tools/cmd/svc/main.go is unrelated.\n",
		"docs/b-server.md":      "# Server\n\nThe entry point is cmd/svc/main.go.\n",
		"docs/c-layout.md":      "# Layout\n\nThe command lives in cmd/svc.\n",
		"cmd/svc/main.go":       "package main\n\nfunc main() {}\n",
		"tools/cmd/svc/main.go": "package main\n\n// stub\nfunc main() {}\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o777); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "mv", "cmd/svc", "internal/svc")
	testGit(t, root, "commit", "-q", "-m", "move the command")

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	// DWC-V0-926/927 supersede unanchored same-file, move, sibling and seal
	// guesses. Preserve the full scenario and check every remaining cite with
	// the CEM verifier; missing anchors must be explicit unknown rows.
	assertAnchoredDraft(t, root, base, stdout.String())

}
