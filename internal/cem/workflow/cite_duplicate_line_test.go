package workflow

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCiteDuplicatedLineNamesCopiesAndWiderUniqueSpanV12541 is V1-2541: a base
// line whose text appears twice in a changed file is refused as not stable
// even though no hunk edits it, because the frozen drift classifier matches
// span text and would read the citation as ambiguous. The refusal names both
// changed-file copies and a wider unique span that keeps the cited line, and
// that span then cites.
func TestCiteDuplicatedLineNamesCopiesAndWiderUniqueSpanV12541(t *testing.T) {
	t.Parallel()
	baseContent := "package resume\n\nfunc a() error {\n\tif x {\n\t\treturn nil\n\t}\n\treturn errA\n}\n\nfunc b() error {\n\tif y {\n\t\treturn nil\n\t}\n\treturn errB\n}\n\n// end\n"
	targetContent := strings.Replace(baseContent, "// end\n", "// END\n", 1)
	root, base, target := makeCiteSpanRepoFrom(t, baseContent, targetContent)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "12:12", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	message := cemcode.MessageOf(err)
	for _, want := range []string{
		"base lines 12:12 are edited by no hunk",
		"occurs 2 times in the changed file (at changed-file lines 5:5, 12:12)",
		"recovery: cite the wider unique span --lines 11:12, which keeps these lines",
		citeSpanRule,
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("refusal %q lacks %q", message, want)
		}
	}
	named := regexp.MustCompile(`--lines (\d+:\d+)`).FindAllStringSubmatch(message, -1)
	if len(named) == 0 {
		t.Fatalf("refusal %q names no span", message)
	}
	for _, match := range named {
		if _, err := session.Cite(ctx(), CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
			Lines: match[1], Relation: "implementation",
		}); err != nil {
			t.Fatalf("named span %s does not cite: %v (refusal %q)", match[1], err, message)
		}
	}
}

// TestOccurrenceLinesBoundsNamedCopiesV12541 names at most the limit and
// counts the rest.
func TestOccurrenceLinesBoundsNamedCopiesV12541(t *testing.T) {
	t.Parallel()
	if got := occurrenceLines([]byte("x\ny\nx\nx\n"), []byte("x\n"), 2); got != "1:1, 3:3, +1 more" {
		t.Fatalf("got %q", got)
	}
}

// TestCiteDuplicatedLineNamesAsymmetricWiderSpanV12541 is the review
// regression for DCW-V0-334: the only unique span that keeps cited line 4
// widens two lines up and one down, between two edited lines, so a search of
// one-sided and symmetric widenings alone would name no recovery.
func TestCiteDuplicatedLineNamesAsymmetricWiderSpanV12541(t *testing.T) {
	t.Parallel()
	baseContent := "E1\nA\nB\nX\nC\nE2\n-\nA\nB\nX\nD\n-\nQ\nB\nX\nC\n"
	targetContent := strings.Replace(strings.Replace(baseContent, "E1\n", "e1\n", 1), "E2\n", "e2\n", 1)
	root, base, target := makeCiteSpanRepoFrom(t, baseContent, targetContent)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "4:4", Relation: "implementation",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	message := cemcode.MessageOf(err)
	if want := "recovery: cite the wider unique span --lines 2:5, which keeps these lines"; !strings.Contains(message, want) {
		t.Fatalf("refusal %q lacks %q", message, want)
	}
	if _, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "2:5", Relation: "implementation",
	}); err != nil {
		t.Fatalf("named span 2:5 does not cite: %v (refusal %q)", err, message)
	}
}
