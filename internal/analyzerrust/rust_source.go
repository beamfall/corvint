package analyzerrust

import "strings"

// itemPredicate maps a Rust item keyword to the predicate its declaration
// emits. `impl` is absent deliberately: an impl block has no single declared
// name, so naming one would be an inference rather than a reading.
var itemPredicate = map[string]string{
	"fn": "declares-fn", "struct": "declares-struct", "enum": "declares-enum",
	"trait": "declares-trait", "union": "declares-union", "type": "declares-type",
	"const": "declares-const", "static": "declares-static",
}

// dynamicMacros are external-input directives. Like Go's //go:embed they name
// content the analyzer was not given, so they reject the request as
// DYNAMIC_INPUT rather than producing a fact about bytes nobody supplied.
var dynamicMacros = map[string]bool{
	"include": true, "include_str": true, "include_bytes": true,
}

// pathHeads are the path-qualifier keywords admitted inside a use path.
var pathHeads = map[string]bool{"crate": true, "self": true, "super": true}

// parseSource extracts the closed static forms from one Rust source input.
// Unlisted forms are inert: they yield no fact rather than rejecting, matching
// the JavaScript and Ruby source rules. Rejection is reserved for lexical
// failure, unsupported bytes, and dynamic external input.
func parseSource(in Input, content []byte, c *factCollector) string {
	if !strings.HasSuffix(in.Path, ".rs") {
		return "UNSUPPORTED_SCHEMA"
	}
	tokens, why := tokenize(content)
	if why != "" {
		return why
	}
	s := &sourceScan{tokens: tokens, in: in, c: c}
	for s.at < len(s.tokens) {
		if why := s.step(); why != "" {
			return why
		}
	}
	return ""
}

type sourceScan struct {
	tokens  []token
	at      int
	in      Input
	c       *factCollector
	pending bool // the item now being read is preceded by #[test]
}

func (s *sourceScan) peek(offset int) token {
	if s.at+offset >= len(s.tokens) {
		return token{kind: tokenPunct, text: ""}
	}
	return s.tokens[s.at+offset]
}

func (s *sourceScan) step() string {
	current := s.peek(0)
	if current.kind == tokenPunct && current.text == "#" {
		return s.attribute()
	}
	// An item body or terminator ends the item a pending #[test] annotated,
	// whether or not that item was a fn this scanner reads.
	if current.kind == tokenPunct && (current.text == ";" || current.text == "{" || current.text == "}") {
		s.pending = false
	}
	if current.kind != tokenIdent {
		s.at++
		return ""
	}
	next := s.peek(1)
	if next.kind == tokenPunct && next.text == "!" {
		return s.macroInvocation(current.text)
	}
	switch current.text {
	case "mod":
		s.pending = false
		return s.moduleDecl()
	case "use":
		s.pending = false
		return s.useDecl()
	case "extern":
		s.pending = false
		return s.externCrate()
	}
	if predicate, ok := itemPredicate[current.text]; ok {
		return s.itemDecl(current.text, predicate)
	}
	s.at++
	return ""
}

// attribute reads #[...] and #![...]. Only the attribute's leading path is
// retained; arguments are never a fact value, so no derive list, cfg predicate,
// or arbitrary token payload can become one.
func (s *sourceScan) attribute() string {
	open := 1
	if t := s.peek(open); t.kind == tokenPunct && t.text == "!" {
		open++
	}
	if t := s.peek(open); !(t.kind == tokenPunct && t.text == "[") {
		s.at++
		return ""
	}
	name := s.peek(open + 1)
	if name.kind != tokenIdent && name.kind != tokenRawIdent {
		s.at++
		return ""
	}
	s.at += open
	s.skipTokenTree()
	if name.text == "test" {
		s.pending = true
	}
	if why := s.emit("rust.attribute", "carries-attribute", name.text); why != "" {
		return why
	}
	if name.text == "cfg" || name.text == "cfg_attr" {
		return s.emit("rust.cfg.unevaluated", "declares-conditional-compilation", name.text)
	}
	return ""
}

// macroInvocation rejects the dynamic external-input directives and otherwise
// treats a macro call as inert: its body is arbitrary token soup and cannot
// yield a closed fact. A macro call requires a delimiter after its bang, so the
// not-operator in `include != other` is never mistaken for one.
func (s *sourceScan) macroInvocation(name string) string {
	open := 2
	if name == "macro_rules" {
		macroName := s.peek(open)
		if macroName.kind != tokenIdent && macroName.kind != tokenRawIdent {
			s.at++
			return ""
		}
		open++
	}
	delimiter := s.peek(open)
	if delimiter.kind != tokenPunct ||
		(delimiter.text != "(" && delimiter.text != "[" && delimiter.text != "{") {
		s.at++
		return ""
	}
	if dynamicMacros[name] {
		return "DYNAMIC_INPUT"
	}
	s.pending = false
	s.at += open
	s.skipTokenTree()
	return ""
}

func (s *sourceScan) skipTokenTree() {
	depth := 0
	for s.at < len(s.tokens) {
		switch s.tokens[s.at].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
		s.at++
		if depth == 0 {
			return
		}
	}
}

// moduleDecl reads `mod NAME;` and `mod NAME {`. A `mod` token can only appear
// as a whole identifier, so a name merely ending in "mod" never matches.
func (s *sourceScan) moduleDecl() string {
	name := s.peek(1)
	if name.kind != tokenIdent && name.kind != tokenRawIdent {
		s.at++
		return ""
	}
	terminator := s.peek(2)
	if terminator.kind != tokenPunct || (terminator.text != ";" && terminator.text != "{") {
		s.at++
		return ""
	}
	s.at += 3
	return s.emit("rust.module", "declares-module", name.text)
}

// useDecl reads a use declaration up to its semicolon. A simple ::-separated
// path becomes an import fact; a grouped, glob, or aliased tree is not a closed
// atom, so it records an explicit unevaluated marker naming only its head
// segment rather than silently emitting nothing.
func (s *sourceScan) useDecl() string {
	s.at++
	segments := make([]string, 0, 8)
	simple, terminated, colons := true, false, 0
	for s.at < len(s.tokens) {
		t := s.tokens[s.at]
		if t.kind == tokenPunct && t.text == ";" {
			s.at++
			terminated = true
			break
		}
		switch {
		case t.kind == tokenIdent || t.kind == tokenRawIdent:
			if t.text == "as" {
				simple = false
			} else {
				// Every segment after the first needs exactly one `::`; the
				// first may carry a leading `::` or none.
				simple = simple && (colons == 2 || (colons == 0 && len(segments) == 0))
				segments = append(segments, t.text)
			}
			colons = 0
		case t.kind == tokenPunct && t.text == ":":
			colons++
			// A separator is exactly two adjacent colon bytes.
			simple = simple && colons <= 2 && (colons == 1 || s.tokens[s.at-1].start+1 == t.start)
		default:
			simple = false
		}
		s.at++
	}
	// An unterminated tree or a trailing separator is not a closed atom.
	simple = simple && terminated && colons == 0
	if len(segments) == 0 {
		return ""
	}
	if !simple || !validUsePath(segments) {
		return s.emit("rust.use.unevaluated", "imports-unevaluated", segments[0])
	}
	return s.emit("rust.import.static", "imports", strings.Join(segments, "::"))
}

// validUsePath admits only ident segments, with crate/self/super permitted as
// qualifiers. crate and self may head the path; super may repeat but only while
// the leading qualifier run has not ended. A segment that is any other keyword
// is not a nameable path.
func validUsePath(segments []string) bool {
	leading := true
	for i, segment := range segments {
		if leading && pathHeads[segment] {
			if i == 0 || segment == "super" {
				continue
			}
			return false
		}
		leading = false
		if rustKeywords[segment] || !rustIdent(segment) {
			return false
		}
	}
	return true
}

func (s *sourceScan) externCrate() string {
	if t := s.peek(1); t.kind != tokenIdent || t.text != "crate" {
		s.at++
		return ""
	}
	name := s.peek(2)
	if name.kind != tokenIdent && name.kind != tokenRawIdent {
		s.at++
		return ""
	}
	s.at += 3
	return s.emit("rust.extern.crate", "links-crate", name.text)
}

// itemDecl reads one named item. `const fn` and `static mut` are skipped at
// their qualifier so the declaration is attributed to the real item keyword,
// and an anonymous form such as `const _:` yields nothing.
func (s *sourceScan) itemDecl(keyword, predicate string) string {
	if keyword == "const" && s.at > 0 && typePositionConst(s.tokens[s.at-1]) {
		s.at++
		return ""
	}
	offset := 1
	if keyword == "static" {
		if t := s.peek(offset); t.kind == tokenIdent && t.text == "mut" {
			offset++
		}
	}
	name := s.peek(offset)
	if name.kind == tokenIdent && (name.text == "fn" || rustKeywords[name.text]) {
		s.at++
		return ""
	}
	if name.kind != tokenIdent && name.kind != tokenRawIdent {
		s.at++
		return ""
	}
	isTest := s.pending && keyword == "fn"
	s.pending = false
	s.at += offset + 1
	if why := s.emit("rust.declaration", predicate, name.text); why != "" {
		return why
	}
	if isTest {
		return s.emit("rust.test", "declares-test", name.text)
	}
	return ""
}

// typePositionConst reports whether `const` follows `*`, `<`, or `,`: a raw
// pointer type or a const-generic parameter, never a const item.
func typePositionConst(previous token) bool {
	return previous.kind == tokenPunct && (previous.text == "*" || previous.text == "<" || previous.text == ",")
}

// emit records one source fact. The subject is the input's logical path and the
// instance identity is path::value, so the same name declared in two files
// stays two distinct facts.
func (s *sourceScan) emit(kind, predicate, value string) string {
	if value == "" || !rustFactValue(value) {
		return ""
	}
	return s.c.add(s.in.Handle, kind, s.in.Path, predicate, value, s.in.Path+"::"+value)
}

// rustIdent is RUST_IDENT: an ASCII Rust identifier. Non-ASCII identifiers are
// outside this candidate's closed grammar and are rejected by the lexer, so a
// name reaching here is already ASCII.
func rustIdent(value string) bool {
	if value == "" || len(value) > MaxIdentifierBytes {
		return false
	}
	if !isIdentStart(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		if !isIdentContinue(value[i]) {
			return false
		}
	}
	return true
}

// rustFactValue admits an identifier or a ::-separated path of identifiers.
func rustFactValue(value string) bool {
	if len(value) > MaxFactFieldBytes {
		return false
	}
	for _, segment := range strings.Split(value, "::") {
		if !rustIdent(segment) {
			return false
		}
	}
	return true
}
