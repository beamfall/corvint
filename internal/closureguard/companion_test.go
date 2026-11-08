package closureguard

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The companion `corvint-test` guards below moved here from cmd/corvint-test
// (V1-5071). The source-text halves of CTF-V0-007 and CCE-V0-015 stay in that
// package's TestCTFV0007FoldedGroupBoundary and TestCCEV0015CoreBoundary.

// CTF-V0-007, CTF-V0-010: the folded group entries are not linked into core
// `corvint`.
func TestCTFV0007CoreLinksNoFoldedGroup(t *testing.T) {
	_, foldedFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	foldedRoot := filepath.Dir(filepath.Dir(filepath.Dir(foldedFile)))
	command := exec.Command("go", "list", "-deps", "./cmd/corvint")
	command.Dir = foldedRoot
	out, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasSuffix(dep, "/internal/testrunner/cli") || strings.HasSuffix(dep, "/internal/behaviorfalsify/cli") ||
			strings.HasSuffix(dep, "/internal/playwrightminimize/cli") || strings.HasSuffix(dep, "/internal/testacceptance/cli") {
			t.Fatalf("core corvint imports %s", dep)
		}
	}
}

// SKH-V0-163: the stage 8 pipeline is linked only into the companion: core `corvint` and the
// corpus MCP server never import it, so core can only read its receipt. Each listing derives
// its own root and output so the import-graph classifier proves every call (AFP-V0-9600).
func TestSKHV0_163_OnlyTheCompanionImportsThePipeline(t *testing.T) {
	_, coreFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	coreRoot := filepath.Dir(filepath.Dir(filepath.Dir(coreFile)))
	core := exec.Command("go", "list", "-deps", "./cmd/corvint")
	core.Dir = coreRoot
	coreOut, coreErr := core.Output()
	if coreErr != nil {
		t.Fatalf("../corvint: %v", coreErr)
	}
	for _, dep := range strings.Fields(string(coreOut)) {
		if strings.HasSuffix(dep, "/internal/skhproof") {
			t.Fatalf("../corvint imports %s", dep)
		}
	}
	_, corpusFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	corpusRoot := filepath.Dir(filepath.Dir(filepath.Dir(corpusFile)))
	corpus := exec.Command("go", "list", "-deps", "./cmd/corvint-corpus-mcp")
	corpus.Dir = corpusRoot
	corpusOut, corpusErr := corpus.Output()
	if corpusErr != nil {
		t.Fatalf("../corvint-corpus-mcp: %v", corpusErr)
	}
	for _, dep := range strings.Fields(string(corpusOut)) {
		if strings.HasSuffix(dep, "/internal/skhproof") {
			t.Fatalf("../corvint-corpus-mcp imports %s", dep)
		}
	}
	_, companionFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	companionRoot := filepath.Dir(filepath.Dir(filepath.Dir(companionFile)))
	companion := exec.Command("go", "list", "-deps", "./cmd/corvint-test")
	companion.Dir = companionRoot
	companionOut, companionErr := companion.Output()
	if companionErr != nil || !strings.Contains(string(companionOut), "/internal/skhproof\n") {
		t.Fatalf("companion does not link the pipeline: %v", companionErr)
	}
}

// CCE-V0-015: core `corvint` never imports the companion or its package.
func TestCCEV0015CoreLinksNoCompanion(t *testing.T) {
	_, cceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	cceRoot := filepath.Dir(filepath.Dir(filepath.Dir(cceFile)))
	command := exec.Command("go", "list", "-deps", "./cmd/corvint")
	command.Dir = cceRoot
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		t.Fatalf("%v: %s", err, stderr.String())
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasSuffix(dep, "/internal/changee2e") || strings.HasSuffix(dep, "/cmd/corvint-test") {
			t.Fatalf("core corvint imports %s", dep)
		}
	}
}
