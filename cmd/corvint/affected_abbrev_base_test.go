package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// AFP-V0-230: an unambiguous abbreviated --base resolves to the full commit id in the root
// repository, an unknown or too-short prefix is refused with the full-id hint, and --root is
// accepted after the verb (once).
func TestAFPV0230AbbreviatedBaseAndTrailingRoot(t *testing.T) {
	root := materializationFixture(t)
	full := strings.TrimSpace(materializationGit(t, root, "rev-parse", "HEAD"))

	for name, arguments := range map[string][]string{
		"leading root":  {"--root", root, "affected", "--base", full[:10]},
		"trailing root": {"affected", "--base", full[:10], "--root", root},
		"equals root":   {"affected", "--root=" + root, "--base", full[:7]},
	} {
		invocation, handled, err := parseAffectedInvocation(arguments)
		if err != nil || !handled || invocation.Base != full {
			t.Fatalf("%s: base=%q handled=%v err=%v", name, invocation.Base, handled, err)
		}
	}

	for name, arguments := range map[string][]string{
		"unknown prefix": {"--root", root, "affected", "--base", "0000000"},
		"too short":      {"--root", root, "affected", "--base", full[:6]},
		"uppercase":      {"--root", root, "affected", "--base", strings.ToUpper(full[:10])},
	} {
		if _, _, err := parseAffectedInvocation(arguments); err == nil || !strings.Contains(err.Error(), "full commit id") {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	if _, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--root", root, "--base", full}); err == nil || !strings.Contains(err.Error(), "--root may be given once") {
		t.Fatalf("duplicate root: %v", err)
	}
	invocation, _, err := parseAffectedInvocation([]string{"affected", "--path=--root", "--root", root})
	if err != nil || len(invocation.PathList) != 1 || invocation.PathList[0] != "--root" {
		t.Fatalf("option value named --root was consumed: %+v %v", invocation, err)
	}
}

// AFP-V0-232: a prefix that abbreviates two commits refuses with ambiguous-affected-base naming
// both, while each commit's full id and a longer unique prefix still resolve to the full id.
func TestAFPV0232AmbiguousAbbreviatedBase(t *testing.T) {
	root := materializationFixture(t)
	tree := strings.TrimSpace(materializationGit(t, root, "rev-parse", "HEAD^{tree}"))
	// A birthday search over generated commit bodies finds two whose ids share seven hex digits.
	body := func(n int) string {
		return fmt.Sprintf("tree %s\nauthor T <t@example.invalid> 0 +0000\ncommitter T <t@example.invalid> 0 +0000\n\nambiguous %d\n", tree, n)
	}
	id := func(content string) string {
		sum := sha1.Sum([]byte(fmt.Sprintf("commit %d\x00%s", len(content), content)))
		return hex.EncodeToString(sum[:])
	}
	seen := map[string]int{}
	first, second := -1, -1
	for n := 0; n < 1<<20 && first < 0; n++ {
		prefix := id(body(n))[:7]
		if earlier, ok := seen[prefix]; ok {
			first, second = earlier, n
		}
		seen[prefix] = n
	}
	if first < 0 {
		t.Fatal("no seven-digit collision found")
	}
	scratch := t.TempDir()
	var written []string
	for _, n := range []int{first, second} {
		path := filepath.Join(scratch, fmt.Sprint(n))
		if err := os.WriteFile(path, []byte(body(n)), 0o644); err != nil {
			t.Fatal(err)
		}
		object := strings.TrimSpace(materializationGit(t, root, "hash-object", "-t", "commit", "-w", path))
		if object != id(body(n)) {
			t.Fatalf("hash-object wrote %s, want %s", object, id(body(n)))
		}
		written = append(written, object)
	}
	prefix := written[0][:7]
	_, handled, err := parseAffectedInvocation([]string{"--root", root, "affected", "--base", prefix})
	var coded *gokernel.Error
	if !handled || !errors.As(err, &coded) || coded.Code != affectedAmbiguousBaseCode ||
		!strings.Contains(coded.Message, written[0]) || !strings.Contains(coded.Message, written[1]) {
		t.Fatalf("ambiguous prefix %s: handled=%v err=%v", prefix, handled, err)
	}
	for _, object := range written {
		for _, value := range []string{object, object[:39]} {
			invocation, _, err := parseAffectedInvocation([]string{"affected", "--base", value, "--root", root})
			if err != nil || invocation.Base != object {
				t.Fatalf("--base %s: base=%q err=%v", value, invocation.Base, err)
			}
		}
	}
}

// TSP-V0-720 (V1-3350): affected --base and test --base resolve a revision expression to the
// full id of the commit it names, which the affected receipt and the test manifest record; an
// expression naming no commit, a range and an option-like value are refused, and an all-hex
// value keeps the AFP-V0-230 prefix rules.
func TestTSPV0720BaseRevisionExpressionResolves(t *testing.T) {
	root, first := selfBinaryRepository(t)
	commitSelfBinaryFiles(t, root, map[string]string{"notes.txt": "second\n"})
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	affectedGit(t, root, "branch", "pinned", first)
	for value, want := range map[string]string{"HEAD": head, "HEAD^": first, "HEAD~1": first, "pinned": first, "pinned^{commit}": first, "@": head} {
		invocation, handled, err := parseAffectedInvocation([]string{"--root", root, "affected", "--base", value})
		if err != nil || !handled || invocation.Base != want {
			t.Fatalf("affected --base %s: base=%q err=%v, want %s", value, invocation.Base, err, want)
		}
	}
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", "HEAD~1")
	if rangeMember, _ := receipt["range"].(map[string]any); code != 0 || rangeMember["base"] != first {
		t.Fatalf("affected --base HEAD~1: exit %d range %v: %s", code, receipt["range"], stderr)
	}
	for _, value := range []string{"HEAD~9", "nosuchbranch", "HEAD..HEAD", "-HEAD", "HEAD:notes.txt", "HEAD^ x"} {
		if _, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--base", value}); err == nil {
			t.Fatalf("affected --base %q was admitted", value)
		}
	}
	for _, value := range []string{"ABCDEF0", "abcdef"} {
		if validBaseRevision(value) {
			t.Fatalf("all-hex %q read as a revision expression", value)
		}
	}
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	var stdout, testStderr bytes.Buffer
	runContext(context.Background(), []string{"--root", root, "test", "--base", "HEAD~1", "--dry-run", "--manifest", manifest}, strings.NewReader(""), &stdout, &testStderr)
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("test --base HEAD~1 wrote no manifest: %v\n%s", err, testStderr.String())
	}
	var written struct {
		Base string `json:"base"`
	}
	if err := json.Unmarshal(raw, &written); err != nil || written.Base != first {
		t.Fatalf("test --base HEAD~1 manifest base %q (%v), want %s", written.Base, err, first)
	}
	stdout.Reset()
	testStderr.Reset()
	if code := runContext(context.Background(), []string{"--root", root, "test", "--base", "HEAD~9", "--dry-run"}, strings.NewReader(""), &stdout, &testStderr); code != 2 || !strings.Contains(testStderr.String(), "does not name a commit") {
		t.Fatalf("test --base HEAD~9: exit %d: %s", code, testStderr.String())
	}
}
