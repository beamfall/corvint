package lrfrepo

import "bytes"

// RequirementLayout is the line layout of a Markdown intent that the
// OCM-V0-001 Requirements rule reads: which lines are fenced, every unfenced
// `## Requirements` heading, and, only when there is exactly one, the line
// that ends its section and the requirement lines inside it. It carries no
// policy: the OCM verifier refuses what the dogfood coverage reader tolerates
// (V1-4754).
type RequirementLayout struct {
	// Fenced marks every line that opens, closes or sits inside a fenced
	// code block.
	Fenced []bool
	// Headings are the line numbers of every unfenced `## Requirements` line.
	Headings []int
	// End is the line number of the first unfenced level-2 heading after the
	// only heading, or the line count when there is none or not exactly one
	// heading.
	End int
	// Items are the requirement lines strictly between the only heading and
	// End, fenced or not, in line order; duplicates are kept.
	Items []RequirementLine
}

// RequirementLine is one requirement line: its zero-based line number and ID.
type RequirementLine struct {
	Line int
	ID   string
}

var requirementsHeading = []byte("## Requirements")

// ScanRequirementLayout reads the layout of count lines, where text returns a
// line's bytes without its ending. The caller decides how much line ending
// text strips, so each consumer keeps its own byte offsets.
func ScanRequirementLayout(count int, text func(int) []byte) RequirementLayout {
	layout := RequirementLayout{Fenced: MarkdownFenced(count, text), End: count}
	layout.Headings = headingLines(count, text, layout.Fenced, requirementsHeading)
	if len(layout.Headings) != 1 {
		return layout
	}
	layout.End = sectionEndLine(count, text, layout.Fenced, layout.Headings[0]+1)
	for number := layout.Headings[0] + 1; number < layout.End; number++ {
		if identity := RequirementLineID(text(number)); identity != "" {
			layout.Items = append(layout.Items, RequirementLine{Line: number, ID: identity})
		}
	}
	return layout
}

// RequirementLineID returns the ID a line declares in the OCM-V0-001
// requirement-line syntax, "- `ID`: " or "- **ID:** " or "- **ID.** " with
// the OCM-V0-952 ID grammar, or "" when it declares none.
func RequirementLineID(line []byte) string {
	identityStart := 3
	closingMarkers := [][]byte{[]byte("`: ")}
	if bytes.HasPrefix(line, []byte("- **")) {
		identityStart = 4
		closingMarkers = [][]byte{[]byte(":** "), []byte(".** ")}
	} else if !bytes.HasPrefix(line, []byte("- `")) {
		return ""
	}
	closing := -1
	for _, marker := range closingMarkers {
		if candidate := bytes.Index(line[identityStart:], marker); candidate >= 0 && (closing < 0 || candidate < closing) {
			closing = candidate
		}
	}
	if closing < 0 {
		return ""
	}
	identity := string(line[identityStart : identityStart+closing])
	if !requirementID.MatchString(identity) {
		return ""
	}
	return identity
}

// MarkdownFenced marks every line that opens, closes, or sits inside a
// CommonMark fenced code block: a run of at least three backticks or tildes
// indented at most three spaces opens one (a backtick fence's info string
// holds no backtick), and only a run of the same character at least as long,
// followed by nothing but spaces or tabs, closes it. An unclosed fence runs to
// the end of the document.
func MarkdownFenced(count int, text func(int) []byte) []bool {
	fenced := make([]bool, count)
	char, length := byte(0), 0
	for number := range fenced {
		line := text(number)
		fenced[number] = length > 0
		if length > 0 {
			if runChar, run, rest := markdownFenceRun(line); runChar == char && run >= length && len(bytes.Trim(rest, " \t")) == 0 {
				length = 0
			}
			continue
		}
		runChar, run, rest := markdownFenceRun(line)
		if run < 3 || runChar == '`' && bytes.IndexByte(rest, '`') >= 0 {
			continue
		}
		char, length = runChar, run
		fenced[number] = true
	}
	return fenced
}

// markdownFenceRun returns the backtick or tilde run that starts a line indented
// at most three spaces, its length, and the bytes after it.
func markdownFenceRun(line []byte) (byte, int, []byte) {
	indent := 0
	for indent < len(line) && indent < 4 && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(line) || line[indent] != '`' && line[indent] != '~' {
		return 0, 0, nil
	}
	char := line[indent]
	end := indent
	for end < len(line) && line[end] == char {
		end++
	}
	return char, end - indent, line[end:]
}

// headingLines returns the number of every unfenced line equal to heading.
func headingLines(count int, text func(int) []byte, fenced []bool, heading []byte) []int {
	headings := make([]int, 0, 1)
	for number := 0; number < count; number++ {
		if !fenced[number] && bytes.Equal(text(number), heading) {
			headings = append(headings, number)
		}
	}
	return headings
}

// sectionEndLine returns the number of the first unfenced level-2 heading at
// or after line from, or count.
func sectionEndLine(count int, text func(int) []byte, fenced []bool, from int) int {
	for number := from; number < count; number++ {
		if fenced[number] {
			continue
		}
		if line := text(number); bytes.Equal(line, []byte("##")) || bytes.HasPrefix(line, []byte("## ")) {
			return number
		}
	}
	return count
}
