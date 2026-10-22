package workflow

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1322RepeatedTableSeparatorNamesUniqueSpansV11531 is V1-1531: a
// Markdown table separator repeats in the file, so a span on it is refused
// as not stable even though no hunk edits it; the refusal states the rule
// and names the nearest base spans whose text stays unique, and each named
// span then cites.
func TestCEMV1322RepeatedTableSeparatorNamesUniqueSpansV11531(t *testing.T) {
	t.Parallel()
	baseContent := "# Spec\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\nmiddle text\n\n| c | d |\n|---|---|\n| 3 | 4 |\n\nend\n"
	targetContent := strings.Replace(baseContent, "| 3 | 4 |\n", "| 3 | 4 |\n| 5 | 6 |\n", 1)
	root, base, target := makeCiteSpanRepoFrom(t, baseContent, targetContent)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
		Lines: "10:10", Relation: "specification",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
		t.Fatalf("got %v, want cite-span-not-stable", err)
	}
	message := cemcode.MessageOf(err)
	for _, want := range []string{citeSpanRule, "occurs 2 times in the changed file", "nearest stable base spans: --lines "} {
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
			Lines: match[1], Relation: "specification",
		}); err != nil {
			t.Fatalf("named span %s does not cite: %v (refusal %q)", match[1], err, message)
		}
	}
}
