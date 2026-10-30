package analyzerrust

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// tokenKind is the closed token vocabulary this candidate recognizes. Comments,
// strings, and literals are consumed but never produced: they are inert, so no
// fact can be derived from text inside them.
type tokenKind uint8

const (
	tokenIdent tokenKind = iota
	tokenRawIdent
	tokenPunct
	tokenLifetime
	tokenLiteral
)

type token struct {
	kind  tokenKind
	text  string
	start int
}

// lexer is a bounded lexical-state scanner over one Rust source input. It
// allocates only the token slice; every text field aliases the source bytes.
type lexer struct {
	src    []byte
	offset int
	tokens []token
	depth  int
	open   [MaxNestingDepth]byte
}

// rustKeywords is the Rust 2024 reserved set. A raw identifier (r#fn) is never
// a keyword; a bare spelling in this set is never an ordinary name.
var rustKeywords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "const": true,
	"continue": true, "crate": true, "dyn": true, "else": true, "enum": true,
	"extern": true, "false": true, "fn": true, "for": true, "if": true,
	"impl": true, "in": true, "let": true, "loop": true, "match": true,
	"mod": true, "move": true, "mut": true, "pub": true, "ref": true,
	"return": true, "self": true, "Self": true, "static": true, "struct": true,
	"super": true, "trait": true, "true": true, "type": true, "union": true,
	"unsafe": true, "use": true, "where": true, "while": true,
	// Reserved but unused; a source using one is still lexically closed.
	"abstract": true, "become": true, "box": true, "do": true, "final": true,
	"gen": true, "macro": true, "override": true, "priv": true, "try": true,
	"typeof": true, "unsized": true, "virtual": true, "yield": true,
}

// tokenize consumes the whole source or rejects it. Every unterminated or
// unsupported construct fails the entire request; nothing is partially scanned.
func tokenize(src []byte) ([]token, string) {
	if len(src) > MaxSourceBytes {
		return nil, "LIMIT_EXCEEDED"
	}
	if !utf8.Valid(src) {
		return nil, "MALFORMED_INPUT"
	}
	if i := indexAnyByte(src, "\r\x00"); i >= 0 {
		return nil, "MALFORMED_INPUT"
	}
	l := &lexer{src: src, tokens: make([]token, 0, 256)}
	if why := l.skipShebang(); why != "" {
		return nil, why
	}
	for l.offset < len(l.src) {
		if why := l.step(); why != "" {
			return nil, why
		}
		if len(l.tokens) > MaxSourceTokens {
			return nil, "LIMIT_EXCEEDED"
		}
	}
	if l.depth != 0 {
		return nil, "MALFORMED_INPUT"
	}
	return l.tokens, ""
}

// skipShebang consumes a leading #! line only when it is a real shebang. In
// Rust a file-leading `#!` followed by `[` is an inner attribute, not a
// shebang, and must be lexed rather than skipped.
func (l *lexer) skipShebang() string {
	if !strings.HasPrefix(string(l.src), "#!") {
		return ""
	}
	rest := l.src[2:]
	if len(rest) > 0 && rest[0] == '[' {
		return ""
	}
	end := indexByte(l.src, '\n')
	if end < 0 {
		l.offset = len(l.src)
		return ""
	}
	l.offset = end + 1
	return ""
}

func (l *lexer) step() string {
	c := l.src[l.offset]
	switch {
	case c == ' ' || c == '\t' || c == '\n':
		l.offset++
		return ""
	case c == '/':
		return l.comment()
	case c == '"':
		return l.stringLiteral(l.offset)
	case c == '\'':
		return l.quote()
	case c == 'r' || c == 'b' || c == 'c':
		return l.prefixed()
	case isIdentStart(c) || c >= utf8.RuneSelf:
		return l.identifier()
	case c >= '0' && c <= '9':
		return l.number()
	}
	return l.punctuation()
}

// comment handles //, ///, //! line comments and Rust's NESTED block comments.
// Rust permits /* /* */ */; a single-level scan would terminate at the first
// */ and treat the remaining comment body as source.
func (l *lexer) comment() string {
	if l.offset+1 >= len(l.src) {
		return l.punctuation()
	}
	switch l.src[l.offset+1] {
	case '/':
		end := indexByte(l.src[l.offset:], '\n')
		if end < 0 {
			l.offset = len(l.src)
			return ""
		}
		l.offset += end + 1
		return ""
	case '*':
		return l.blockComment()
	}
	return l.punctuation()
}

func (l *lexer) blockComment() string {
	depth := 0
	i := l.offset
	for i < len(l.src) {
		if i+1 < len(l.src) && l.src[i] == '/' && l.src[i+1] == '*' {
			depth++
			if depth > MaxNestingDepth {
				return "LIMIT_EXCEEDED"
			}
			i += 2
			continue
		}
		if i+1 < len(l.src) && l.src[i] == '*' && l.src[i+1] == '/' {
			depth--
			i += 2
			if depth == 0 {
				l.offset = i
				return ""
			}
			continue
		}
		i++
	}
	return "MALFORMED_INPUT"
}

// prefixed disambiguates the r/b/c prefixes. `r#"` opens a raw string while
// `r#name` is a raw identifier, and the two are distinguished only by what
// follows the run of hashes.
func (l *lexer) prefixed() string {
	start := l.offset
	rest := l.src[l.offset:]
	switch {
	case hasPrefix(rest, "br#") || hasPrefix(rest, "cr#"):
		return l.rawString(start, l.offset+2)
	case hasPrefix(rest, "br\"") || hasPrefix(rest, "cr\""):
		return l.rawString(start, l.offset+2)
	case hasPrefix(rest, "b\"") || hasPrefix(rest, "c\""):
		return l.stringLiteral(start)
	case hasPrefix(rest, "b'"):
		l.offset++
		return l.charLiteral(start)
	case hasPrefix(rest, "r\""):
		return l.rawString(start, l.offset+1)
	case hasPrefix(rest, "r#"):
		return l.rawHashPrefix(start)
	}
	return l.identifier()
}

// rawHashPrefix resolves the r# ambiguity: a run of hashes followed by a quote
// is a raw string, and a single hash followed by an identifier start is a raw
// identifier. Anything else is not a closed form.
func (l *lexer) rawHashPrefix(start int) string {
	i := l.offset + 1
	for i < len(l.src) && l.src[i] == '#' {
		i++
	}
	if i < len(l.src) && l.src[i] == '"' {
		return l.rawString(start, l.offset+1)
	}
	if i == l.offset+2 && i < len(l.src) && isIdentStart(l.src[i]) {
		l.offset = i
		for l.offset < len(l.src) && isIdentContinue(l.src[l.offset]) {
			l.offset++
		}
		name := string(l.src[start+2 : l.offset])
		if name == "crate" || name == "self" || name == "super" || name == "Self" {
			return "MALFORMED_INPUT"
		}
		l.tokens = append(l.tokens, token{kind: tokenRawIdent, text: name, start: start})
		return ""
	}
	return "MALFORMED_INPUT"
}

// rawString consumes r"..." / r#"..."# with a variable hash count. The
// terminator is a quote followed by exactly the opening number of hashes; a
// shorter run inside the body does not close it.
func (l *lexer) rawString(start, hashStart int) string {
	i := hashStart
	hashes := 0
	for i < len(l.src) && l.src[i] == '#' {
		hashes++
		i++
	}
	if hashes > MaxRawStringHashes {
		return "LIMIT_EXCEEDED"
	}
	if i >= len(l.src) || l.src[i] != '"' {
		return "MALFORMED_INPUT"
	}
	i++
	closing := `"` + strings.Repeat("#", hashes)
	for search := i; search < len(l.src); {
		end := indexString(l.src[search:], closing)
		if end < 0 {
			return "MALFORMED_INPUT"
		}
		end += search
		after := end + len(closing)
		if after == len(l.src) || l.src[after] != '#' {
			l.offset = after
			l.tokens = append(l.tokens, token{kind: tokenLiteral, text: "", start: start})
			return ""
		}
		search = end + 1
	}
	return "MALFORMED_INPUT"
}

// stringLiteral consumes a normal or byte string. Escapes are validated
// structurally: a backslash always consumes its escapee, so a trailing \" can
// never be mistaken for a terminator.
func (l *lexer) stringLiteral(start int) string {
	i := l.offset
	for i < len(l.src) && l.src[i] != '"' {
		i++
	}
	if i >= len(l.src) {
		return "MALFORMED_INPUT"
	}
	i++
	for i < len(l.src) {
		switch l.src[i] {
		case '\\':
			if i+1 >= len(l.src) {
				return "MALFORMED_INPUT"
			}
			i += 2
			continue
		case '"':
			l.offset = i + 1
			l.tokens = append(l.tokens, token{kind: tokenLiteral, text: "", start: start})
			return ""
		}
		i++
	}
	return "MALFORMED_INPUT"
}

// quote resolves Rust's char-versus-lifetime ambiguity at a bare '. 'a' is a
// char literal, 'a is a lifetime, and 'outer: is a loop label. The decision
// needs lookahead past the identifier, because both start identically.
func (l *lexer) quote() string {
	start := l.offset
	rest := l.src[l.offset+1:]
	if len(rest) == 0 {
		return "MALFORMED_INPUT"
	}
	if rest[0] != '\\' && isIdentStart(rest[0]) {
		end := 0
		for end < len(rest) && isIdentContinue(rest[end]) {
			end++
		}
		// A quote directly after a one-character name closes a char literal;
		// otherwise the name is a lifetime or label.
		if !(end == 1 && end < len(rest) && rest[end] == '\'') {
			l.offset = l.offset + 1 + end
			l.tokens = append(l.tokens, token{kind: tokenLifetime,
				text: string(rest[:end]), start: start})
			return ""
		}
	}
	return l.charLiteral(start)
}

func (l *lexer) charLiteral(start int) string {
	i := l.offset + 1
	if i >= len(l.src) {
		return "MALFORMED_INPUT"
	}
	if l.src[i] == '\\' {
		if i+1 >= len(l.src) || l.src[i+1] == '\n' {
			return "MALFORMED_INPUT"
		}
		i += 2
		for i < len(l.src) && l.src[i] != '\'' {
			if l.src[i] == '\n' {
				return "MALFORMED_INPUT"
			}
			i++
		}
	} else {
		r, size := utf8.DecodeRune(l.src[i:])
		if r == '\n' {
			return "MALFORMED_INPUT"
		}
		i += size
	}
	if i >= len(l.src) || l.src[i] != '\'' {
		return "MALFORMED_INPUT"
	}
	l.offset = i + 1
	l.tokens = append(l.tokens, token{kind: tokenLiteral, text: "", start: start})
	return ""
}

func (l *lexer) identifier() string {
	start := l.offset
	if l.src[l.offset] >= utf8.RuneSelf {
		return "UNSUPPORTED_SCHEMA"
	}
	for l.offset < len(l.src) && isIdentContinue(l.src[l.offset]) {
		l.offset++
	}
	l.tokens = append(l.tokens, token{kind: tokenIdent,
		text: string(l.src[start:l.offset]), start: start})
	return ""
}

// number consumes a numeric literal including its suffix and separators. Its
// text is discarded: a literal is never a fact value.
func (l *lexer) number() string {
	start := l.offset
	for l.offset < len(l.src) {
		c := l.src[l.offset]
		if isIdentContinue(c) || c == '.' && l.offset+1 < len(l.src) && l.src[l.offset+1] >= '0' && l.src[l.offset+1] <= '9' {
			l.offset++
			continue
		}
		if (c == '+' || c == '-') && l.offset > start {
			previous := l.src[l.offset-1]
			if previous == 'e' || previous == 'E' {
				l.offset++
				continue
			}
		}
		break
	}
	l.tokens = append(l.tokens, token{kind: tokenLiteral, text: "", start: start})
	return ""
}

// punctuation tracks the open delimiter stack so an unbalanced or mismatched
// source rejects rather than producing facts from a truncated item.
func (l *lexer) punctuation() string {
	c := l.src[l.offset]
	if kind := strings.IndexByte("{[(", c); kind >= 0 {
		if l.depth == MaxNestingDepth {
			return "LIMIT_EXCEEDED"
		}
		l.open[l.depth] = byte(kind)
		l.depth++
	}
	if kind := strings.IndexByte("}])", c); kind >= 0 {
		if l.depth == 0 || l.open[l.depth-1] != byte(kind) {
			return "MALFORMED_INPUT"
		}
		l.depth--
	}
	if strings.IndexByte("{}[]()<>:;,.=&|!?+-*/%^~@#$", c) < 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	l.tokens = append(l.tokens, token{kind: tokenPunct, text: string(c), start: l.offset})
	l.offset++
	return ""
}

func isIdentStart(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_'
}

func isIdentContinue(c byte) bool {
	return isIdentStart(c) || c >= '0' && c <= '9'
}

func hasPrefix(b []byte, prefix string) bool {
	return len(b) >= len(prefix) && string(b[:len(prefix)]) == prefix
}

func indexByte(b []byte, c byte) int {
	for i := 0; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func indexAnyByte(b []byte, set string) int {
	for i := 0; i < len(b); i++ {
		if strings.IndexByte(set, b[i]) >= 0 {
			return i
		}
	}
	return -1
}

func indexString(b []byte, s string) int {
	return bytes.Index(b, []byte(s))
}
