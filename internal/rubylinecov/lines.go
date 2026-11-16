// SPDX-License-Identifier: Apache-2.0

package rubylinecov

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/rubydialect"
)

// Sources gives the line-level analysis both immutable sides of a diff, the
// Ruby syntax check of the new side and Ruby's own reading of both. A nil
// member forbids narrowing.
type Sources struct {
	Old, New func(path string) ([]byte, error)
	// Check reports whether src parses under the index's Ruby.
	Check func(path string, src []byte) error
	// Lex is how the index's Ruby itself reads src, a line an entry: the
	// Code, Inside and Open of rubydialect.LineStates (see RubyLexer).
	Lex func(path string, src []byte) ([]rubydialect.LineState, error)
}

// AnalyzeLines maps a diff to the tests whose measured line coverage reached
// the edited lines (LPCV-V0-7605 as amended by decision 0601). A line's hits
// bound its reach only when Ruby cannot evaluate part of it, or bind a name
// other lines read, without counting it. So a hunk selects the tests that hit
// its lines, and none when no test hit them, only when every old line is a
// counted line holding one whole statement, every new line holds one whole
// statement, both sides bind the same names, and neither side of the file may
// hold a percent literal a blank delimits (see unbounded). Any other hunk,
// and an insertion, selects every test of the enclosing called method and
// its body lines, or falls back outside one.
//
// Every one of those readings rests on the lexer of internal/rubydialect,
// which is an approximation of Ruby: it reads a percent literal delimited by
// a blank or a newline, two divisions on one line and a quote in `$'` as
// code or as the wrong literal, and an edit of such text can change a method
// the edit is not in. So a file is narrowed only when Ruby's own lexer reads
// both of its sides as the lexer does, line by line (see misread). Load-time
// reach, a block-structure change, a side Ruby reads differently, a new side
// that does not parse or cannot be checked, and every fallback of Analyze
// keep the static plan with the reason reported. Callers apply it only to a
// qualified family's effective plan.
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
		if src.Lex == nil {
			return fail("no Ruby lexer for %s", d.Path)
		}
		oldSide, newSide := readSide(string(oldSrc)), readSide(string(newSrc))
		if why := oldSide.misread(src.Lex, d.Path, oldSrc); why != "" {
			return fail("Ruby reads the old side of %s differently from the lexer: %s", d.Path, why)
		}
		if why := newSide.misread(src.Lex, d.Path, newSrc); why != "" {
			return fail("Ruby reads the new side of %s differently from the lexer: %s", d.Path, why)
		}
		if why := sameStructure(oldSide, newSide, d.Hunks, d.NewHunks); why != "" {
			return fail("edit changes Ruby block structure %s: %s", d.Path, why)
		}
		for i, h := range d.Hunks {
			if h.End < h.Start { // an insertion after line h.Start
				if !selectHunk(ix, f, h, e.Tests) {
					return fail("insertion outside a called method body %s:%d", d.Path, h.Start)
				}
				continue
			}
			// The tests that hit the old lines are always part of the reach.
			uncounted := 0
			for line := h.Start; line <= h.End; line++ {
				b, ok := lineBlock(ix, f.Path, line)
				if !ok && uncounted == 0 {
					uncounted = line
				}
				for _, t := range b.Tests {
					e.Tests[t] = true
				}
			}
			why, at := "", h.Start
			if uncounted != 0 {
				// Ruby does not count this line: the statement it belongs to is
				// owned by the enclosing method.
				why, at = "an uncounted line", uncounted
			} else {
				why, at = unbounded(oldSide, newSide, h, d.NewHunks[i])
			}
			if why == "" {
				continue
			}
			if !selectHunk(ix, f, h, e.Tests) {
				if load >= 0 && e.Tests[load] {
					return fail("the change reaches load-time code")
				}
				return fail("edit of %s outside a called method body %s:%d", why, d.Path, at)
			}
		}
	}
	if load >= 0 && e.Tests[load] {
		return fail("the change reaches load-time code")
	}
	return e
}

// side is one side of a diff as the reach rule reads it.
type side struct {
	raw   []string
	state []rubydialect.LineState
	// depth[i] is the bracket depth before line i+1; balanced is false when
	// the depth goes negative or does not return to zero.
	depth    []int
	balanced bool
	// blankDelimited is true when the side may hold a percent literal
	// delimited by a space or tab (see holdsBlankDelimited).
	blankDelimited bool
}

func readSide(src string) side {
	s := side{raw: strings.Split(src, "\n"), state: rubydialect.LineStates(src), balanced: true, blankDelimited: holdsBlankDelimited(src)}
	s.depth = make([]int, len(s.state)+1)
	depth := 0
	for i, st := range s.state {
		s.depth[i] = depth
		for j := 0; j < len(st.Code); j++ {
			switch st.Code[j] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
			if depth < 0 {
				s.balanced = false
			}
		}
	}
	s.depth[len(s.state)] = depth
	if depth != 0 {
		s.balanced = false
	}
	return s
}

// misread reports where Ruby's own lexer reads src differently from the
// side, or "" when both agree on every line: the same bytes are code, and the
// line begins and ends inside a literal, a heredoc body or a block comment
// for both. Blanks at a line's end are not compared, because a line that is
// all literal text is empty for one reader and blank for the other. Each
// literal or comment begins with a byte that is not a blank (a quote, a
// percent sign, a slash, `?`, `<<`, `#`), so two readings that agree on a
// line's text agree on which of its bytes are code.
func (s side) misread(lex func(string, []byte) ([]rubydialect.LineState, error), path string, src []byte) string {
	ruby, err := lex(path, src)
	if err != nil {
		return err.Error()
	}
	if len(ruby) != len(s.state) {
		return fmt.Sprintf("%d lines against %d", len(ruby), len(s.state))
	}
	for i, ours := range s.state {
		theirs := ruby[i]
		if strings.TrimRight(ours.Code, " \t\r") != strings.TrimRight(theirs.Code, " \t\r") || ours.Inside != theirs.Inside || ours.Open != theirs.Open {
			return fmt.Sprintf("line %d", i+1)
		}
	}
	return ""
}

// lexScript prints how Ruby's own lexer (Ripper) reads the source on its
// standard input, in the form rubydialect.LineStates gives: one output line a
// source line, holding `I` or `-` (the line begins inside a literal, a heredoc
// body or a block comment), `O` or `-` (it ends inside one or owes a heredoc
// body), then the line with every comment, string, percent literal, regexp,
// character literal and heredoc opener replaced by spaces, and empty when the
// whole line is such text. It exits non-zero on a source Ruby does not lex to
// the end, and on string content it finds outside every literal it tracked.
const lexScript = `require 'ripper'
src = STDIN.binmode.read
lines = src.split("\n", -1)
out = lines.map(&:dup)
inside = Array.new(lines.size, false)
open = Array.new(lines.size, false)
carry = lambda do |l1, l2|
  (l1...l2).each { |l| open[l - 1] = true if l <= lines.size }
  (l1 + 1..l2).each { |l| inside[l - 1] = true if l <= lines.size }
end
blank = lambda do |l1, c1, l2, c2|
  (l1..l2).each do |l|
    line = out[l - 1] or next
    from = l == l1 ? c1 : 0
    to = l == l2 ? [c2, line.bytesize].min : line.bytesize
    if l != l1 && l != l2
      out[l - 1] = +''
    elsif to > from
      line[from...to] = ' ' * (to - from)
    end
  end
  carry.(l1, l2)
end
whole = ->(l1, l2) { (l1..l2).each { |l| out[l - 1] = +'' if l <= lines.size } }
tokens = Ripper.lex(src.dup.force_encoding('UTF-8'), '-', 1, raise_errors: true)
depth, start, slash = 0, nil, false
heredocs, body = 0, nil
tokens.each do |(l, c), event, text, _|
  text = text.b
  case event
  when :on_comment
    blank.(l, c, l, c + text.chomp.bytesize) if depth == 0
  when :on_embdoc_beg, :on_embdoc, :on_embdoc_end
    whole.(l, l)
    open[l - 1] = true unless event == :on_embdoc_end
    inside[l - 1] = true unless event == :on_embdoc_beg
  when :on_tstring_beg, :on_backtick, :on_qwords_beg, :on_words_beg, :on_qsymbols_beg, :on_symbols_beg, :on_regexp_beg, :on_symbeg
    next if event == :on_symbeg && text == ':'
    if depth == 0
      start = [l, text.start_with?(':') ? c + 1 : c]
      slash = text == '/'
    end
    depth += 1
  when :on_tstring_end, :on_regexp_end, :on_label_end
    depth -= 1
    abort 'a literal ends that never began' if depth < 0
    if depth == 0
      width = event == :on_tstring_end || event == :on_regexp_end && slash ? text.bytesize : 1
      blank.(start[0], start[1], l, c + width)
    end
  when :on_tstring_content, :on_embexpr_beg
    abort "string content outside a literal at line #{l}" if depth == 0 && heredocs == 0
  when :on_heredoc_beg
    blank.(l, c, l, c + text.bytesize) if depth == 0
    body ||= l + 1
    heredocs += 1
  when :on_heredoc_end
    abort 'a heredoc ends that never began' if heredocs == 0
    whole.(body, l)
    carry.(body - 1, l)
    heredocs -= 1
    body = heredocs > 0 ? l + 1 : nil
    open[l - 1] = true if heredocs > 0
  when :on_CHAR
    blank.(l, c, l, c + text.bytesize) if depth == 0
  when :on___end__
    whole.(l + 1, lines.size)
  end
end
abort 'a literal or heredoc is left open' if depth != 0 || heredocs != 0
STDOUT.binmode.write(out.each_with_index.map { |line, i| (inside[i] ? 'I' : '-') + (open[i] ? 'O' : '-') + line }.join("\n"))
`

// RubyLexer returns the Sources.Lex that asks the interpreter ruby, run with
// env, how Ruby itself reads a source. The caller answers for ruby being the
// index's Ruby.
func RubyLexer(ctx context.Context, ruby string, env []string) func(path string, src []byte) ([]rubydialect.LineState, error) {
	return func(path string, src []byte) ([]rubydialect.LineState, error) {
		cmd := exec.CommandContext(ctx, ruby, "-W0", "-e", lexScript)
		cmd.Env = env
		cmd.Stdin = bytes.NewReader(src)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			message, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
			return nil, fmt.Errorf("Ruby does not lex %s: %v %s", path, err, message)
		}
		lines := strings.Split(stdout.String(), "\n")
		states := make([]rubydialect.LineState, len(lines))
		for i, line := range lines {
			if len(line) < 2 {
				return nil, fmt.Errorf("Ruby's reading of %s:%d is malformed", path, i+1)
			}
			states[i] = rubydialect.LineState{Code: line[2:], Inside: line[0] == 'I', Open: line[1] == 'O'}
		}
		return states, nil
	}
}

// codes is every line's code text.
func (s side) codes() []string {
	codes := make([]string, len(s.state))
	for i, st := range s.state {
		codes[i] = st.Code
	}
	return codes
}

// structure lists line n's block keywords and brackets in order, with
// markers for a line that continues from the previous or onto the next.
func (s side) structure(n int) []string {
	var tokens []string
	if s.leads(n) {
		tokens = append(tokens, "<lead")
	}
	tokens = append(tokens, structureTokens(s.state[n-1].Code)...)
	if s.continues(n) {
		tokens = append(tokens, "cont>")
	}
	return tokens
}

// code is the trimmed code text of line n, "" outside the file.
func (s side) code(n int) string {
	if n < 1 || n > len(s.state) {
		return ""
	}
	return strings.TrimSpace(s.state[n-1].Code)
}

// whole reports why line n is not one whole statement, or "" when it is:
// code that begins and ends on the line outside every bracket and literal,
// with no block keyword, no second statement and no continuation from the
// previous line or onto the next. Ruby then counts the line whenever it
// evaluates any part of it. The probe of Ruby 3.3.8 behind this rule: a
// one-line `when C then B` counts only when B runs although C is evaluated,
// a line inside a multi-line literal counts only for a lambda it holds, and
// a multi-line statement is counted on a line other than its first.
func (s side) whole(n int) string {
	if n < 1 || n > len(s.state) {
		return "a line outside the file"
	}
	code := s.code(n)
	switch {
	case !s.balanced:
		return "a line of a file with unbalanced brackets"
	case s.state[n-1].Inside || s.state[n-1].Open:
		return "a line inside or opening a multi-line literal"
	case code == "":
		return "a blank, comment or data line"
	case s.depth[n-1] != 0 || s.depth[n] != 0:
		return "a line inside or opening a multi-line bracket"
	case s.leads(n) || s.continues(n):
		return "a line of a multi-line statement"
	}
	for i := 0; i < len(code); {
		switch {
		case rubydialect.WordByte(code[i]):
			word, next := rubydialect.Word(code, i)
			if next <= i {
				next = i + 1
			}
			// A word after one dot is a method name (x.class), not a keyword.
			called := i > 0 && code[i-1] == '.' && (i < 2 || code[i-2] != '.')
			if blockKeyword(word) && !called {
				return "a line with a block keyword"
			}
			i = next
		case code[i] == ';':
			return "a line with a second statement"
		default:
			i++
		}
	}
	for m := n - 1; m >= 1; m-- {
		if s.code(m) != "" || s.state[m-1].EndsLiteral {
			if s.continues(m) {
				return "a line of a multi-line statement"
			}
			break
		}
	}
	for m := n + 1; m <= len(s.state); m++ {
		if s.code(m) != "" || s.state[m-1].StartsLiteral {
			if s.leads(m) {
				return "a line of a multi-line statement"
			}
			break
		}
	}
	return ""
}

// leads and continues apply to a line whose first or last code byte is not
// a blanked literal: a literal there is a value, whatever precedes it.
func (s side) leads(n int) bool { return !s.state[n-1].StartsLiteral && leads(s.code(n)) }

func (s side) continues(n int) bool { return !s.state[n-1].EndsLiteral && continues(s.code(n)) }

// leads reports whether trimmed code continues the previous line: it starts
// with a dot, a logical operator or the word `and` or `or`.
func leads(code string) bool {
	for _, prefix := range []string{".", "&.", "&&", "||"} {
		if strings.HasPrefix(code, prefix) {
			return true
		}
	}
	word, _ := rubydialect.Word(code, 0)
	return word == "and" || word == "or"
}

// operandKeywords take an operand after them, so a line ending in one
// continues onto the next.
var operandKeywords = []string{"and", "or", "not", "if", "unless", "while", "until", "in", "when", "case", "elsif", "rescue"}

// continues reports whether trimmed code continues onto the next line: it
// ends with an operator, a backslash or a keyword that takes an operand. A
// trailing `?` or `!` directly after a word is a method name's suffix. The
// keyword is the line's last word whatever precedes it: Ruby needs no blank
// between a bracket or a number and a keyword (`f(1)and`, `1or`). A word
// after one dot is a method name (x.in).
func continues(code string) bool {
	if code == "" {
		return false
	}
	last := code[len(code)-1]
	if last == '?' || last == '!' {
		return len(code) < 2 || !rubydialect.WordByte(code[len(code)-2])
	}
	if strings.ContainsRune(`\,.+-*/%&|^=<>:(~`, rune(last)) {
		return true
	}
	start := len(code)
	for start > 0 && rubydialect.WordByte(code[start-1]) {
		start--
	}
	word := code[start:]
	if start > 0 && code[start-1] == '.' && (start < 2 || code[start-2] != '.') {
		return false
	}
	for _, keyword := range operandKeywords {
		if word == keyword || numberLed(word) && strings.HasSuffix(afterNumber(word), keyword) {
			return true
		}
	}
	return false
}

// numberLed reports whether a word starts with a digit. Ruby ends a number
// where a keyword starts, so `1if` is the number 1 and the keyword `if`.
func numberLed(word string) bool { return word != "" && word[0] >= '0' && word[0] <= '9' }

// unbounded reports why the hits of a replaced or deleted hunk's old lines do
// not bound the edit's reach, and the line, or "" when they do: neither side
// of the file may hold a percent literal a blank delimits, every old and new
// line is one whole statement, and both sides bind the same names in the
// same order. A name a line binds (an assigned local, a pattern or named
// capture, a numbered block parameter) changes how other lines parse and run
// even when the line itself never executes.
//
// A file that may hold a blank-delimited percent literal (see
// holdsBlankDelimited) bounds no hunk, whatever its lines say: Ruby closes
// such a literal at the next blank, which may be the line's indentation, a
// blank inside it or the blank at its end, so no part of a line's text tells
// what the line declares (`p 1, % a ` ends the literal; `p 1, % a` runs it
// onto the next line and swallows an assignment there).
func unbounded(oldSide, newSide side, oldHunk, newHunk linecov.Hunk) (string, int) {
	if oldSide.blankDelimited || newSide.blankDelimited {
		return "a line of a file that may hold a blank-delimited percent literal", oldHunk.Start
	}
	var oldBinds, newBinds []string
	for n := oldHunk.Start; n <= oldHunk.End; n++ {
		if why := oldSide.whole(n); why != "" {
			return why, n
		}
		sig, ok := binding(oldSide.raw[n-1], oldSide.state[n-1].Code)
		if !ok {
			return "a line that may bind a block parameter or a capture", n
		}
		if sig != "" {
			oldBinds = append(oldBinds, sig)
		}
	}
	for n := newHunk.Start; n <= newHunk.End; n++ {
		if why := newSide.whole(n); why != "" {
			return why + " on the new side", oldHunk.Start
		}
		sig, ok := binding(newSide.raw[n-1], newSide.state[n-1].Code)
		if !ok {
			return "a line that may bind a block parameter or a capture on the new side", oldHunk.Start
		}
		if sig != "" {
			newBinds = append(newBinds, sig)
		}
	}
	if strings.Join(oldBinds, "\n") != strings.Join(newBinds, "\n") {
		return "a line whose bindings change", oldHunk.Start
	}
	return "", 0
}

// binding is the signature of the names one source line can bind, taken
// from the line's own text so that nothing which tells two lines apart is
// dropped: "" when the line has no assignment and no `=>`; its text through
// the last assignment operator when it assigns; and its whole text when it
// holds `=>`, whose target follows the operator. A sigil, a dot, a bracket,
// a comma, a number, a comment mark and a byte Ruby reads as part of a name
// (a no-break space) all stay as written, because each decides what a line
// declares: `x = h`, `@x = h`, `foo.x = h`, `foo x = h` and `foo # x = h`
// differ, as do `foo 1 => x` and `foo => x`, and `foo [1] => x` and
// `foo[1] => x`.
//
// The text is squeezed: the blanks at either end are cut and each other run
// of spaces and tabs is read as one space. That is the only comparison there
// is, and unbounded calls it only for a file in which no side may hold a
// percent literal a blank delimits; there a run only separates tokens or
// sits inside a string, regexp or comment body, where its length changes a
// value but never a name. Two lines so share a signature only when their
// squeezed text through the operator, or all of it on a `=>` line, is the
// same, and then they declare the same names. Text after the last assignment
// operator, and a line with neither operator, is not compared at all.
//
// Operators are found in the raw line, so an assignment inside an
// interpolation counts, and one inside a string or a comment only makes the
// comparison stricter: such text is compared as written, behind its quote or
// `#`, and never read as code. ok is false for a line that can bind without
// an operator the signature would show: a named capture in either spelling,
// or a numbered or implicit block parameter in its code or an interpolation.
func binding(raw, code string) (sig string, ok bool) {
	if strings.Contains(raw, "(?<") || strings.Contains(raw, "(?'") {
		return "", false
	}
	params := []string{code}
	if strings.Contains(raw, "#{") {
		params = append(params, raw)
	}
	for _, text := range params {
		for i := 0; i < len(text); {
			word, next := rubydialect.Word(text, i)
			if next <= i {
				i++
				continue
			}
			if word == "it" || len(word) == 2 && word[0] == '_' && word[1] >= '1' && word[1] <= '9' {
				return "", false
			}
			i = next
		}
	}
	last, rocket := -1, false
	at := func(i int) byte {
		if i < 0 || i >= len(raw) {
			return 0
		}
		return raw[i]
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '=' {
			continue
		}
		prev, next := at(i-1), at(i+1)
		switch {
		case next == '>':
			rocket = true
		case next == '=' || next == '~' || prev == '=' || prev == '!':
			// ==, ===, =~ and != compare.
		case (prev == '<' || prev == '>') && at(i-2) != prev:
			// <= and >= compare; <<= and >>= assign.
		default:
			last = i
		}
	}
	switch {
	case rocket:
		return squeeze(raw), true
	case last >= 0:
		return squeeze(raw[:last+1]), true
	}
	return "", true
}

// holdsBlankDelimited reports whether src may hold a percent literal
// delimited by a space or tab: a `%`, at most one letter, then a space or
// tab. Ruby 3.3.8 accepts that delimiter for a bare, q, Q, s, r and x
// literal; any letter is matched, and so is text in a string or a comment
// and the modulo operator written with a blank after it, because the lexer
// does not read such a literal and so cannot tell code from text around it.
// Where Ruby does read such a literal, misread already keeps the static
// plan; this rule is the one that holds without Ruby's reading.
func holdsBlankDelimited(src string) bool {
	for i := strings.IndexByte(src, '%'); i >= 0; {
		j := i + 1
		if j < len(src) && ('a' <= src[j]|0x20 && src[j]|0x20 <= 'z') {
			j++
		}
		if j < len(src) && (src[j] == ' ' || src[j] == '\t') {
			return true
		}
		next := strings.IndexByte(src[i+1:], '%')
		if next < 0 {
			break
		}
		i += 1 + next
	}
	return false
}

// squeeze trims text and puts one space for each run of spaces and tabs
// inside it. Every other byte is kept: Ruby reads a no-break space, like any
// byte above ASCII, as part of a name.
func squeeze(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == '\t' }), " ")
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
func sameStructure(oldSide, newSide side, oldHunks, newHunks []linecov.Hunk) string {
	if len(oldHunks) != len(newHunks) {
		return "unpaired hunks"
	}
	for _, s := range []side{oldSide, newSide} {
		x := rubydialect.Symbols(strings.Join(s.raw, "\n"), 100000, 100000)
		if x.Stop != "" || x.UnterminatedComment || x.UnterminatedString {
			return "the lexer did not reach a clean end"
		}
	}
	oldCode, newCode := oldSide.codes(), newSide.codes()
	region := func(s side, h linecov.Hunk) []string {
		var tokens []string
		for n := h.Start; n <= h.End; n++ {
			if n >= 1 && n <= len(s.state) {
				tokens = append(tokens, s.structure(n)...)
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
		if strings.Join(region(oldSide, oh), " ") != strings.Join(region(newSide, nh), " ") {
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

// blockKeyword reports whether word is a block keyword or a number with one
// glued to its end (`1if`, `2do`, `0x1fif`).
func blockKeyword(word string) bool {
	if structureKeywords[word] {
		return true
	}
	if numberLed(word) {
		rest := afterNumber(word)
		for keyword := range structureKeywords {
			if strings.HasSuffix(rest, keyword) {
				return true
			}
		}
	}
	return false
}

// afterNumber is the part of a number-led word that follows a number with a
// radix prefix (0x, 0b, 0o, 0d in either case): Ruby takes that radix's
// digits and underscores as far as they go, so `0xdef` is one number and
// `0x1fif` is 0x1f and `if`. A word without a radix prefix is returned
// whole, so a keyword is looked for at its end; that may read a decimal
// number's exponent as part of a keyword, which only refuses more lines.
func afterNumber(word string) string {
	if len(word) < 2 || word[0] != '0' {
		return word
	}
	var digit func(c byte) bool
	switch word[1] | 0x20 {
	case 'x':
		digit = func(c byte) bool { return '0' <= c && c <= '9' || 'a' <= c|0x20 && c|0x20 <= 'f' }
	case 'b':
		digit = func(c byte) bool { return c == '0' || c == '1' }
	case 'o':
		digit = func(c byte) bool { return '0' <= c && c <= '7' }
	case 'd':
		digit = func(c byte) bool { return '0' <= c && c <= '9' }
	default:
		return word
	}
	i := 2
	for i < len(word) && (digit(word[i]) || word[i] == '_') {
		i++
	}
	return word[i:]
}

// structureTokens lists one code line's block keywords and brackets in
// order.
func structureTokens(code string) []string {
	var tokens []string
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
			if blockKeyword(word) {
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
	return tokens
}
