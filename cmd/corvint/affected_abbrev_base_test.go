package main

import (
	"crypto/sha1"
	"encoding/hex"
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

// ACX-V0-854 (V1-3727): --base HEAD~1 and a remote-tracking ref resolve to the full commit id
// the receipt records; an unresolvable revision, an all-hex miss and a range share one hint.
func TestACXV0854RevisionBase(t *testing.T) {
	root := materializationFixture(t)
	head := strings.TrimSpace(materializationGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	materializationGit(t, root, "add", "second.txt")
	materializationGit(t, root, "commit", "-qm", "second")
	materializationGit(t, root, "update-ref", "refs/remotes/origin/main", head)
	for _, value := range []string{"HEAD~1", "origin/main", "HEAD^"} {
		invocation, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--base", value})
		if err != nil || invocation.Base != head {
			t.Fatalf("--base %s: base=%q err=%v, want %s", value, invocation.Base, err, head)
		}
	}
	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", "HEAD~1")
	if code != 0 || !strings.Contains(stdout, `"base":"`+head+`"`) && !strings.Contains(stdout, `"base": "`+head+`"`) {
		t.Fatalf("receipt does not record the full base: code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, value := range []string{"HEAD~9", "origin/missing", "0000000", "HEAD..main", "-p", ":/second", "a b"} {
		_, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--base", value})
		if err == nil || !strings.Contains(err.Error(), "git rev-parse --verify '"+value+"^{commit}'") || !strings.Contains(err.Error(), "full commit id") {
			t.Fatalf("--base %q: err=%v, want the uniform rev-parse hint", value, err)
		}
	}
}
