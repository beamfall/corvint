package contextindex

import "strings"

// commentSyntax is the lexical shape a marker scan needs for one file
// suffix: where comments begin and end, and which string forms must be
// skipped so a quoted "feature:x" never reads as a marker (GPK-V0-070).
type commentSyntax struct {
	line      []string
	block     [2]string
	quotes    string
	multiline []multilineString
}

// multilineString is a delimiter whose body may span lines; escapes says
// whether a backslash protects the next byte (a Go raw string does not).
type multilineString struct {
	delim   string
	escapes bool
}

var (
	cComments      = commentSyntax{line: []string{"//"}, block: [2]string{"/*", "*/"}, quotes: `"'`}
	goComments     = commentSyntax{line: []string{"//"}, block: [2]string{"/*", "*/"}, quotes: `"'`, multiline: []multilineString{{"`", false}}}
	webComments    = commentSyntax{line: []string{"//"}, block: [2]string{"/*", "*/"}, quotes: `"'`, multiline: []multilineString{{"`", true}}}
	rustComments   = commentSyntax{line: []string{"//"}, block: [2]string{"/*", "*/"}, quotes: `"`}
	tripleComments = commentSyntax{line: []string{"//"}, block: [2]string{"/*", "*/"}, quotes: `"'`, multiline: []multilineString{{`"""`, true}}}
	hashComments   = commentSyntax{line: []string{"#"}, quotes: `"'`}
	pyComments     = commentSyntax{line: []string{"#"}, quotes: `"'`, multiline: []multilineString{{`"""`, true}, {"'''", true}}}
)

// markerComments maps each indexed text suffix that has comments to its
// syntax. A suffix absent here (prose and data: .md is the exception through
// HTML comments) yields no markers at all.
var markerComments = map[string]commentSyntax{
	".go": goComments, ".mod": goComments,
	".ts": webComments, ".tsx": webComments, ".js": webComments, ".jsx": webComments, ".mjs": webComments, ".cjs": webComments,
	".rs": rustComments, ".swift": {line: []string{"//"}, block: [2]string{"/*", "*/"}, quotes: `"`, multiline: []multilineString{{`"""`, true}}},
	".cs": tripleComments, ".kt": tripleComments, ".kts": tripleComments, ".m": cComments,
	".py": pyComments, ".toml": pyComments,
	".sh": hashComments, ".yaml": hashComments, ".yml": hashComments, ".rb": hashComments,
	".sql": {line: []string{"--"}, block: [2]string{"/*", "*/"}, quotes: `"'`},
	".md":  {block: [2]string{"<!--", "-->"}}, ".mdx": {block: [2]string{"<!--", "-->"}},
}

// commentText returns text with every byte outside a comment replaced by a
// space and newlines kept, so a per-line marker scan over it reports the
// original line and byte column of each comment marker and nothing else.
func commentText(filePath, text string) string {
	syntax, ok := markerComments[pathExt(filePath)]
	if !ok {
		return ""
	}
	out := blankedBytes(text)
	for i := 0; i < len(text); {
		end, comment := syntax.next(text, i)
		if comment {
			copy(out[i:end], text[i:end])
		}
		i = end
	}
	return string(out)
}

// blankedBytes keeps byte offsets: a multibyte rune becomes several spaces.
func blankedBytes(text string) []byte {
	out := make([]byte, len(text))
	for i := range out {
		out[i] = ' '
	}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			out[i] = '\n'
		}
	}
	return out
}

// next consumes one token starting at i and reports its end and whether it
// is a comment. Tokens are a comment, a string, or a single code byte.
func (s commentSyntax) next(text string, i int) (int, bool) {
	rest := text[i:]
	for _, m := range s.multiline {
		if strings.HasPrefix(rest, m.delim) {
			return i + len(m.delim) + closeString(rest[len(m.delim):], m.delim, m.escapes, false), false
		}
	}
	for _, leader := range s.line {
		if strings.HasPrefix(rest, leader) {
			return i + commentLineEnd(rest), true
		}
	}
	if s.block[0] != "" && strings.HasPrefix(rest, s.block[0]) {
		return i + len(s.block[0]) + commentBlockEnd(rest[len(s.block[0]):], s.block[1]), true
	}
	if strings.IndexByte(s.quotes, rest[0]) >= 0 {
		return i + 1 + closeString(rest[1:], rest[:1], true, true), false
	}
	return i + 1, false
}

// closeString returns the length of a string body plus its closing
// delimiter. A single-line string also stops at a newline, so an unmatched
// quote (a Rust lifetime, a shell apostrophe) cannot swallow later lines.
func closeString(body, delim string, escapes, singleLine bool) int {
	for j := 0; j < len(body); j++ {
		switch {
		case escapes && body[j] == '\\':
			j++
		case singleLine && body[j] == '\n':
			return j
		case strings.HasPrefix(body[j:], delim):
			return j + len(delim)
		}
	}
	return len(body)
}

func commentLineEnd(rest string) int {
	if end := strings.IndexByte(rest, '\n'); end >= 0 {
		return end
	}
	return len(rest)
}

func commentBlockEnd(body, closer string) int {
	if end := strings.Index(body, closer); end >= 0 {
		return end + len(closer)
	}
	return len(body)
}
