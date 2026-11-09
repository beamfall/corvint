package workflow

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1710RefusalNamesUnstableBoundAndAcceptedInnerSpan pins CEM-V1-710
// (V1-3855): a cite-span-not-stable refusal names which bound of the span a
// hunk edits, or where inside it a hunk edits, and the longest accepted span
// inside it, which then cites as stated.
func TestCEMV1710RefusalNamesUnstableBoundAndAcceptedInnerSpan(t *testing.T) {
	t.Parallel()
	const base = "a\nb\nc\nd\ne\nf\n"
	for _, test := range []struct {
		name, target, lines, bound, inner string
	}{
		{"end bound", "a\nb\nc\nD\ne\nf\n", "1:4", "unstable bound: end line 4 removed or replaced by a hunk", "1:3"},
		{"start bound", "a\nB\nc\nd\ne\nf\n", "2:5", "unstable bound: start line 2 removed or replaced by a hunk", "3:5"},
		{"both bounds", "a\nB\nc\nd\nE\nf\n", "2:5", "unstable bound: start line 2 and end line 5 removed or replaced by a hunk", "3:4"},
		{"inner insertion", "a\nb\nc\nnew\nd\ne\nf\n", "2:5", "unstable inside: a hunk inserts lines before base line 4", "2:3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root, baseRev, target := makeCiteSpanRepoFrom(t, base, test.target)
			session := openSession(t, root)
			if _, err := session.Prepare(ctx(), PrepareOptions{Base: baseRev, Target: target}); err != nil {
				t.Fatal(err)
			}
			_, err := session.Cite(ctx(), CiteOptions{
				MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
				Lines: test.lines, Relation: "implementation",
			})
			if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
				t.Fatalf("got %v, want cite-span-not-stable", err)
			}
			message := cemcode.MessageOf(err)
			for _, want := range []string{citeSpanRule, test.bound, "accepted span inside it: --lines " + test.inner} {
				if !strings.Contains(message, want) {
					t.Fatalf("refusal %q lacks %q", message, want)
				}
			}
			if _, err := session.Cite(ctx(), CiteOptions{
				MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt",
				Lines: test.inner, Relation: "implementation",
			}); err != nil {
				t.Fatalf("accepted inner span %s does not cite: %v", test.inner, err)
			}
		})
	}
}

// TestCEMV1710BoundDiagnosis pins the CEM-V1-710 diagnosis order: removed
// bounds first, then the first removed or split line inside, else repeated
// text; and the longest inner unedited run, earlier on a tie.
func TestCEMV1710BoundDiagnosis(t *testing.T) {
	t.Parallel()
	edited := make([]bool, 8)
	split := make([]bool, 8)
	if got := citeSpanBounds(edited, split, 2, 4); !strings.Contains(got, "no hunk edits these lines") {
		t.Fatalf("unedited span diagnosis = %q", got)
	}
	edited[3] = true
	if got := citeSpanBounds(edited, split, 2, 5); got != "unstable inside: base line 3 removed or replaced by a hunk" {
		t.Fatalf("inner edit diagnosis = %q", got)
	}
	if start, end := innerUneditedRun(edited, split, 2, 5); start != 4 || end != 5 {
		t.Fatalf("inner run = %d:%d, want 4:5", start, end)
	}
	if start, end := innerUneditedRun(edited, split, 2, 4); start != 2 || end != 2 {
		t.Fatalf("tied inner run = %d:%d, want 2:2", start, end)
	}
	if start, end := innerUneditedRun(edited, split, 3, 3); start != 0 || end != 0 {
		t.Fatalf("fully edited inner run = %d:%d, want 0:0", start, end)
	}
}
