// SPDX-License-Identifier: Apache-2.0

package rubylinecov

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/rubydialect"
)

// Sources gives the line-level analysis both immutable sides of a diff and
// the Ruby syntax check of the new side. A nil member forbids narrowing.
type Sources struct {
	Old, New func(path string) ([]byte, error)
	// Check reports whether src parses under the index's Ruby.
	Check func(path string, src []byte) error
}

// AnalyzeLines maps a diff to the tests whose measured line coverage reached
// the edited lines (LPCV-V0-7605 as amended by decision 0601). An edit of
// executable lines selects the tests that hit them, so an edit of lines no
// test hit selects none. A line Ruby does not count (when, else, a
// continuation line) and an insertion select every test of the enclosing
// called method and its body lines. Load-time reach, a block-structure
// change, a new side that does not parse or cannot be checked, and every
// fallback of Analyze keep the static plan with the reason reported.
// Callers apply it only to a qualified family's effective plan.
func AnalyzeLines(ix *linecov.Index, diffs []linecov.FileDiff, src Sources) linecov.Effect {
	e := linecov.Effect{Tests: map[int32]bool{}, Whole: map[string]bool{}, Touched: map[string]bool{}}
	fail := func(format string, args ...any) linecov.Effect {
		return linecov.Effect{Fallback: fmt.Sprintf(format, args...)}
	}
	testFiles := map[string]bool{}
	load := int32(-1)
	for i, t := range ix.Tests {
		if t.Package == Load {
			load = int32(i)
		} else {
			testFiles[t.Package] = true
		}
	}
	for _, d := range diffs {
		if !strings.HasSuffix(d.Path, ".rb") {
			return fail("non-Ruby input %s", d.Path)
		}
		if testFiles[d.Path] || IsTestFile(d.Path) {
			e.Whole[d.Path] = true
			continue
		}
		if dir, _, _ := strings.Cut(d.Path, "/"); dir == "spec" || dir == "test" {
			return fail("test helper %s", d.Path)
		}
		if d.OldOID == "" || d.NewOID == "" {
			return fail("added or deleted file %s", d.Path)
		}
		f, ok := ix.FileByPath(d.Path)
		if !ok {
			return fail("uninstrumented file %s", d.Path)
		}
		if len(ix.TestsOfFile(d.Path)) == 0 {
			return fail("unexecuted file %s", d.Path)
		}
		if f.OID != d.OldOID {
			return fail("index records %s at another blob", d.Path)
		}
		if src.Old == nil || src.New == nil {
			return fail("source blobs unavailable for %s", d.Path)
		}
		oldSrc, err := src.Old(d.Path)
		if err != nil {
			return fail("old side of %s unreadable: %v", d.Path, err)
		}
		newSrc, err := src.New(d.Path)
		if err != nil {
			return fail("new side of %s unreadable: %v", d.Path, err)
		}
		if src.Check == nil {
			return fail("no Ruby syntax check for %s", d.Path)
		}
		if err := src.Check(d.Path, newSrc); err != nil {
			return fail("the new side of %s does not parse: %v", d.Path, err)
		}
		if why := sameStructure(string(oldSrc), string(newSrc), d.Hunks, d.NewHunks); why != "" {
			return fail("edit changes Ruby block structure %s: %s", d.Path, why)
		}
		for _, h := range d.Hunks {
			if h.End < h.Start { // an insertion after line h.Start
				if !selectHunk(ix, f, h, e.Tests) {
					return fail("insertion outside a called method body %s:%d", d.Path, h.Start)
				}
				continue
			}
			for line := h.Start; line <= h.End; line++ {
				if b, ok := lineBlock(ix, f.Path, line); ok {
					for _, t := range b.Tests {
						e.Tests[t] = true
					}
					continue
				}
				// Ruby does not count this line: the statement it belongs to is
				// owned by the enclosing method.
				if !selectHunk(ix, f, linecov.Hunk{Start: line, End: line}, e.Tests) {
					return fail("edit of an uncounted line outside a called method body %s:%d", d.Path, line)
				}
			}
		}
	}
	if load >= 0 && e.Tests[load] {
		return fail("the change reaches load-time code")
	}
	return e
}

// lineBlock is the executable-line block of path:line, if Ruby counts it.
func lineBlock(ix *linecov.Index, path string, line int) (linecov.Block, bool) {
	for _, b := range ix.BlocksAtLine(path, int32(line)) {
		if b.StartCol == 0 && int(b.Start) == line && int(b.End) == line {
			return b, true
		}
	}
	return linecov.Block{}, false
}

// sameStructure reports why an edit may change how Ruby nests the code, or
// "" when it cannot: both sides lex completely, every line outside the hunks
// keeps its code text (no string, heredoc or comment boundary moved), and
// each hunk keeps its sequence of block keywords, brackets and line
// continuations. It is lexical, so it may refuse a harmless edit; the syntax
// check covers a malformed one.
func sameStructure(oldSrc, newSrc string, oldHunks, newHunks []linecov.Hunk) string {
	if len(oldHunks) != len(newHunks) {
		return "unpaired hunks"
	}
	for _, s := range []string{oldSrc, newSrc} {
		x := rubydialect.Symbols(s, 100000, 100000)
		if x.Stop != "" || x.UnterminatedComment || x.UnterminatedString {
			return "the lexer did not reach a clean end"
		}
	}
	oldCode, newCode := rubydialect.CodeLines(oldSrc), rubydialect.CodeLines(newSrc)
	region := func(code []string, h linecov.Hunk) []string {
		var tokens []string
		for line := h.Start; line <= h.End; line++ {
			if line >= 1 && line <= len(code) {
				tokens = append(tokens, structureTokens(code[line-1])...)
			}
		}
		return tokens
	}
	o, n := 1, 1
	for i := range oldHunks {
		oh, nh := oldHunks[i], newHunks[i]
		// The first changed line of a side with no lines is the one after Start.
		oFirst, nFirst := oh.Start, nh.Start
		if oh.End < oh.Start {
			oFirst++
		}
		if nh.End < nh.Start {
			nFirst++
		}
		for o < oFirst && n < nFirst {
			if line(oldCode, o) != line(newCode, n) {
				return fmt.Sprintf("line %d no longer lexes as before", n)
			}
			o, n = o+1, n+1
		}
		if o != oFirst || n != nFirst {
			return "misaligned hunks"
		}
		if strings.Join(region(oldCode, oh), " ") != strings.Join(region(newCode, nh), " ") {
			return fmt.Sprintf("keywords, brackets or continuations differ at line %d", nh.Start)
		}
		o, n = max(o, oh.End+1), max(n, nh.End+1)
	}
	for o <= len(oldCode) || n <= len(newCode) {
		if line(oldCode, o) != line(newCode, n) {
			return fmt.Sprintf("line %d no longer lexes as before", n)
		}
		o, n = o+1, n+1
	}
	return ""
}

func line(code []string, n int) string {
	if n < 1 || n > len(code) {
		return "\x00"
	}
	return code[n-1]
}

// structureKeywords open, divide or close a Ruby block.
var structureKeywords = map[string]bool{
	"BEGIN": true, "END": true, "__END__": true, "begin": true, "case": true, "class": true, "def": true, "do": true,
	"else": true, "elsif": true, "end": true, "ensure": true, "for": true, "if": true, "in": true, "module": true,
	"rescue": true, "then": true, "unless": true, "until": true, "when": true, "while": true,
}

// structureTokens lists one code line's block keywords and brackets in order,
// with markers for a line that continues onto the next or from the previous.
func structureTokens(code string) []string {
	var tokens []string
	trimmed := strings.TrimSpace(code)
	if strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "&.") {
		tokens = append(tokens, "<lead")
	}
	for i := 0; i < len(code); {
		c := code[i]
		switch {
		case rubydialect.WordByte(c):
			word, next := rubydialect.Word(code, i)
			if next <= i {
				next = i + 1
			}
			// A method call or symbol named like a keyword (x.class, :end) is
			// still listed: the comparison only needs to be conservative.
			if structureKeywords[word] {
				tokens = append(tokens, word)
			}
			i = next
		case strings.ContainsRune("()[]{}|;", rune(c)):
			tokens = append(tokens, string(c))
			i++
		default:
			i++
		}
	}
	if trimmed != "" && strings.ContainsRune(`\,.+-*/%&|^=<>!?:(`, rune(trimmed[len(trimmed)-1])) {
		tokens = append(tokens, "cont>")
	}
	for _, word := range []string{" and", " or", " not"} {
		if strings.HasSuffix(trimmed, word) {
			tokens = append(tokens, "cont>")
		}
	}
	return tokens
}
