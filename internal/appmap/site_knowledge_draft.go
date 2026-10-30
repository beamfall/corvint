package appmap

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Stage 7 (V1-1904): the how draft. From a how answer the generator copies the primary test's
// file at the map revision, line for line, without the other tests of that file, and inserts the
// AMSP-V0-008 guard after the test's declaration. Every line carries its evidence; a line it
// cannot copy is a marked TODO and the draft is never COMPLETE. Nothing is synthesised
// (SKH-V0-070..074, SKH-V0-153..160).

// DraftGuardLine is the one line the stage 8 companion's recorded transform removes (SKH-V0-088).
const DraftGuardLine = `  test.fixme(true, "how draft: a proposal copied from mined tests; only the stage 8 companion removes this line");`

const (
	// GapSourceFramework abstains a draft whose test or setup was mined from another framework.
	GapSourceFramework = "source-framework-unsupported"
	// GapSecretScreened marks a source line that holds secret-shaped text and is not copied.
	GapSecretScreened = "secret-screened"

	DraftBasisCopied = "copied"
	DraftBasisGuard  = "guard"
	DraftBasisTodo   = "todo"
)

// SKDraftLine is the evidence of one draft line: the source line it copies, the guard, or a TODO
// with the gap it marks.
type SKDraftLine struct {
	Line   int    `json:"line"`
	Basis  string `json:"basis"`
	Path   string `json:"path,omitempty"`
	Source int    `json:"source_line,omitempty"`
	Blob   string `json:"blob,omitempty"`
	Gap    string `json:"gap,omitempty"`
}

// SKDraft is the generated test: lines, their evidence map, the guard line and the digest of the
// guarded bytes (the lines joined by newlines, with a final newline).
type SKDraft struct {
	Status       string   `json:"status"`
	Label        string   `json:"label"`
	Gaps         []HowGap `json:"gaps"`
	Source       string   `json:"source,omitempty"`
	ProposedPath string   `json:"proposed_path,omitempty"`
	// ProposedRepo names the map root alias the proposed path belongs to when the test was read
	// from an aliased root (SKH-V0-189); absent for --root.
	ProposedRepo string        `json:"proposed_repo,omitempty"`
	GuardLine    int           `json:"guard_line,omitempty"`
	Lines        []string      `json:"lines"`
	Evidence     []SKDraftLine `json:"evidence"`
	Digest       string        `json:"digest,omitempty"`
}

// HowDraft is the how answer for goal with the stage 7 draft in its head: `flows appmap plan
// --request GOAL --draft --source DIR` and the corvint.map_plan MCP tool with sources
// (SKH-V0-073). It reads only committed sources and writes nothing.
func HowDraft(ctx context.Context, m *Map, sources []string, goal string, conditions []string, trust []TestTrust, ledger *EdgeLedger, o Options) ([]byte, error) {
	a, err := howCompute(ctx, m, sources, goal, conditions, trust, ledger, o)
	if err != nil {
		return nil, err
	}
	d, err := a.draft(ctx, o.Root, goal)
	if err != nil {
		return nil, err
	}
	return render(append(a.head, field{"draft", d}), a.sections, a.budget)
}

// jsSource reports a JavaScript or TypeScript source path.
func jsSource(p string) bool {
	switch path.Ext(p) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	}
	return false
}

// draftPath places the draft beside its primary test (the AMAP-V0-013 closest-spec rule), keeping
// the test's spec suffix so the suite's testMatch selects it.
func draftPath(testPath, goal string) string {
	base := path.Base(testPath)
	suffix := path.Ext(base)
	for _, marker := range []string{".spec.", ".test.", ".e2e."} {
		if i := strings.Index(base, marker); i >= 0 {
			suffix = base[i:]
			break
		}
	}
	return path.Join(path.Dir(testPath), "how-"+digest([]byte(goal))[:12]+suffix)
}

// bodyOpener matches a line ending in the opening brace of an arrow or function callback body.
var bodyOpener = regexp.MustCompile(`(=>|\))\s*\{\s*(//.*)?$`)

func (a *howAnswer) draft(ctx context.Context, root, goal string) (SKDraft, error) {
	d := SKDraft{Label: howProposalLabel, Gaps: []HowGap{}, Lines: []string{}, Evidence: []SKDraftLine{}}
	abstain := func(g ...HowGap) (SKDraft, error) {
		d.Status, d.Gaps = HowAbstained, append(d.Gaps, g...)
		return d, nil
	}
	t := a.test
	if t == nil {
		return abstain(a.gaps...)
	}
	d.Source = t.ID
	// A test or setup mined from another framework is never copied across (SKH-V0-070).
	if t.Framework != "playwright" || !jsSource(t.Path) {
		at := t.Anchor
		return abstain(HowGap{Code: GapSourceFramework, Detail: "test " + t.ID + ": " + t.Framework, Anchor: &at})
	}
	for _, s := range a.setup {
		if !jsSource(s.Anchor.Path) {
			at := s.Anchor
			return abstain(HowGap{Code: GapSourceFramework, Detail: "setup " + s.Setup + ": " + s.Kind, Anchor: &at})
		}
	}
	// The test is re-read from the root its sources came from (SKH-V0-189).
	src, rev := repo{ctx: ctx, root: root}, a.revision
	if sr := a.k.roots[t.Anchor.Repo]; sr != nil {
		src, rev = sr.r, sr.rev
	} else if t.Anchor.Repo != "" {
		return d, rootUnavailable(t.Anchor.Repo, "was not read for this answer")
	}
	e, data, err := src.readOne(rev, t.Path, maxTestFileBytes)
	if err != nil {
		return d, err
	}
	if e.oid != t.Anchor.Blob {
		return d, fmt.Errorf("%s changed between reads at %s", t.Path, rev)
	}
	source := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	skip := map[int]bool{}
	for _, o := range a.k.Tests {
		if o.Path == t.Path && o.ID != t.ID {
			for n := o.Anchor.Start; n <= o.Anchor.End; n++ {
				skip[n] = true
			}
		}
	}
	// The guard goes directly after the line that opens the test's callback body, which may sit
	// below a multi-line declaration; with no such line inside the test's span the draft abstains
	// rather than place the guard inside the argument list (SKH-V0-178).
	opener := 0
	for n := t.Anchor.Start; n <= t.Anchor.End && n <= len(source); n++ {
		if bodyOpener.MatchString(source[n-1]) {
			opener = n
			break
		}
	}
	if opener == 0 {
		at := t.Anchor
		return abstain(HowGap{Code: GapSourceFramework, Detail: "test " + t.ID + ": callback body opener not located", Anchor: &at})
	}
	opaque, screened := map[int]SKStep{}, map[int]bool{}
	for _, s := range t.Steps {
		if s.Kind == StepOpaque {
			opaque[s.Anchor.Start] = s
		}
		for n := s.Anchor.Start; s.Screened && n <= s.Anchor.End; n++ {
			screened[n] = true
		}
	}
	gaps := append([]HowGap{}, a.gaps...)
	emit := func(text, basis string, n int, gap string) {
		d.Lines = append(d.Lines, text)
		ev := SKDraftLine{Line: len(d.Lines), Basis: basis, Gap: gap}
		if n > 0 {
			ev.Path, ev.Source, ev.Blob = t.Path, n, e.oid
		}
		d.Evidence = append(d.Evidence, ev)
	}
	todo := func(indent, code, detail string, n int) {
		emit(indent+"// TODO "+code+": "+commentSafe.Replace(detail), DraftBasisTodo, n, code)
	}
	for i, line := range source {
		n := i + 1
		if skip[n] {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if s, ok := opaque[n]; ok {
			todo(indent, GapNoPageObject, s.Reason+": "+s.Call+" (resolve or import it)", n)
		}
		if screened[n] || secretscreen.MatchString(line) {
			todo(indent, GapSecretScreened, t.Path+":"+strconv.Itoa(n)+" is not copied", n)
			gaps = append(gaps, HowGap{Code: GapSecretScreened, Detail: t.Path + ":" + strconv.Itoa(n)})
			continue
		}
		emit(line, DraftBasisCopied, n, "")
		if n != opener {
			continue
		}
		emit(DraftGuardLine, DraftBasisGuard, 0, "")
		d.GuardLine = len(d.Lines)
		// Each answer gap a copied line cannot close, and each condition, is a marked TODO.
		for _, g := range a.gaps {
			if g.Code != GapNoPageObject {
				todo("  ", g.Code, g.Detail, 0)
			}
		}
		for _, c := range a.conditions {
			if c.Status != ConditionCandidates {
				continue // already a no-setup-for-precondition gap
			}
			ids := []string{}
			for _, cand := range c.Candidates {
				ids = append(ids, cand.Setup)
			}
			todo("  ", GapNoSetup, "arrange "+strconv.Quote(c.Condition)+" through "+strings.Join(ids, ", "), 0)
			gaps = append(gaps, HowGap{Code: GapNoSetup, Detail: "condition-not-arranged: " + c.Condition})
		}
	}
	d.Gaps = sortedGaps(gaps)
	d.ProposedPath, d.ProposedRepo = draftPath(t.Path, goal), t.Anchor.Repo
	d.Digest = digest([]byte(strings.Join(d.Lines, "\n") + "\n"))
	d.Status = HowComplete
	for _, ev := range d.Evidence {
		if ev.Basis == DraftBasisTodo {
			d.Status = HowIncomplete
		}
	}
	if len(d.Gaps) > 0 {
		d.Status = HowIncomplete
	}
	return d, nil
}

func sortedGaps(g []HowGap) []HowGap {
	out := append([]HowGap{}, g...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Code < out[j-1].Code; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
