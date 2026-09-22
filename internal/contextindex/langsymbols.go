package contextindex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file adds lexical symbol extraction for the C-family languages the index
// previously could not see at all: Rust, C#, Swift and Kotlin.
//
// It deliberately does NOT parse these languages. goSymbols (parse.go:262) sets
// the house precedent: a line scanner that recognises declarations by their
// leading keywords. A line scanner is honest here in a way a half-written
// parser would not be, because its failure mode is a missed declaration rather
// than a confidently wrong tree.
//
// What a raw line scanner gets wrong is context: `// fn not_real()` and
// `let sql = "class Foo {}"` both look like declarations to a regex. So every
// line is first reduced to its code, with comments and string literals elided,
// by a small carried-state scanner. That scanner is the only part of this file
// that has to be right about lexing, and it is shared by all four languages.

// braceDialect names the lexical differences among the four languages. Each
// field exists because at least one of them disagrees with the others; a flag
// that every dialect set the same way would just be inlined behaviour.
type braceDialect struct {
	// nestedBlockComments distinguishes Rust, Swift and Kotlin, where `/* /* */ */`
	// is one comment, from C#, where the first `*/` closes it. Getting this
	// backwards for C# would silently swallow the code after the inner close.
	nestedBlockComments bool
	// lifetimes marks Rust, where a leading `'` is far more often a lifetime
	// (`&'a str`) than a character literal. Treating `'a` as an unterminated
	// char literal would elide the rest of nearly every generic signature.
	lifetimes bool
	// charLiterals marks the dialects that spell a character `'x'`. Swift has
	// no such literal, so a stray apostrophe there must not open one.
	charLiterals bool
	// tripleQuoted marks Swift, Kotlin and C# 11 multi-line string literals.
	tripleQuoted bool
	// verbatimAt marks C# `@"..."`, where `""` is an escaped quote and `\` is
	// not an escape at all.
	verbatimAt bool
	// rawStringPrefix is the character that, immediately before a run of `#`
	// and a quote, opens a raw string: `r` for Rust's `r#"..."#`, `#` for
	// Swift's `#"..."#`. Empty when the dialect has none.
	rawStringPrefix byte
}

var (
	rustDialect = braceDialect{
		nestedBlockComments: true, lifetimes: true, charLiterals: true,
		rawStringPrefix: 'r',
	}
	csharpDialect = braceDialect{
		charLiterals: true, tripleQuoted: true, verbatimAt: true,
	}
	swiftDialect = braceDialect{
		nestedBlockComments: true, tripleQuoted: true, rawStringPrefix: '#',
	}
	kotlinDialect = braceDialect{
		nestedBlockComments: true, charLiterals: true, tripleQuoted: true,
	}
)

// braceState is the lexical state carried from one line to the next. A file
// that ends with any of these still set ended mid-construct, which is what the
// unterminated notes report.
type braceState struct {
	commentDepth int
	inTriple     bool
	inVerbatim   bool
	// rawHashes is the number of `#` that must follow the closing quote, or
	// -1 when no raw string is open. Zero is a legitimate value (`r"..."`),
	// which is why absence cannot be spelled as 0.
	rawHashes int
	// depth is the brace nesting of code seen so far. Swift, Kotlin and Rust
	// use it to separate a type member from a function local, which otherwise
	// share a spelling.
	depth int
}

func newBraceState() *braceState { return &braceState{rawHashes: -1} }

// open reports whether the scanner ended a file mid-construct, and which kind.
// A file cannot legally end this way, so reaching here means either the source
// is broken or the scanner mis-lexed it. Both are worth a note: in either case
// the lines the scanner swallowed were never examined for declarations.
func (state *braceState) open() (string, bool) {
	switch {
	case state.commentDepth > 0:
		return noteUnterminatedComment, true
	case state.inTriple || state.inVerbatim || state.rawHashes >= 0:
		return noteUnterminatedString, true
	}
	return "", false
}

// codeOnly reduces one line to the code it contains, replacing each elided
// comment or string literal with a single space. A space rather than nothing,
// so that `foo"bar"baz` cannot fuse into one token that never appeared.
func codeOnly(line string, state *braceState, dialect braceDialect) string {
	var kept strings.Builder
	kept.Grow(len(line))
	position := 0
	for position < len(line) {
		// A carried-over construct consumes the head of the line before any
		// code can start, so these are resolved before ordinary scanning.
		if state.commentDepth > 0 {
			position = consumeBlockComment(line, position, state, dialect)
			continue
		}
		if state.rawHashes >= 0 {
			position = consumeRawString(line, position, state)
			continue
		}
		if state.inTriple {
			position = consumeTriple(line, position, state)
			continue
		}
		if state.inVerbatim {
			position = consumeVerbatim(line, position, state)
			continue
		}
		consumed, elided, done := scanCode(line, position, state, dialect)
		if done {
			break
		}
		if elided {
			kept.WriteByte(' ')
		} else {
			kept.WriteString(line[position:consumed])
		}
		position = consumed
	}
	return kept.String()
}

// scanCode advances past one construct starting at position in code context.
// It reports where the construct ends, whether it was elided rather than kept,
// and whether the rest of the line is a comment.
func scanCode(line string, position int, state *braceState, dialect braceDialect) (int, bool, bool) {
	character := line[position]
	switch {
	case hasAt(line, position, "//"):
		return 0, false, true
	case hasAt(line, position, "/*"):
		state.commentDepth = 1
		return position + 2, true, false
	case dialect.tripleQuoted && hasAt(line, position, `"""`):
		state.inTriple = true
		return consumeTriple(line, position+3, state), true, false
	case dialect.verbatimAt && hasAt(line, position, `@"`):
		state.inVerbatim = true
		return consumeVerbatim(line, position+2, state), true, false
	case dialect.rawStringPrefix != 0 && character == dialect.rawStringPrefix:
		if start, hashes, ok := rawStringOpener(line, position); ok {
			state.rawHashes = hashes
			return consumeRawString(line, start, state), true, false
		}
	case character == '"':
		return consumeString(line, position+1, '"'), true, false
	case character == '\'':
		return scanQuote(line, position, dialect), true, false
	case character == '{':
		state.depth++
	case character == '}':
		if state.depth > 0 {
			state.depth--
		}
	}
	// Multi-byte runes must advance by their whole width or the next pass
	// would resume inside a rune and compare continuation bytes as ASCII.
	_, width := utf8.DecodeRuneInString(line[position:])
	return position + width, false, false
}

// scanQuote resolves the one genuinely ambiguous character in these dialects.
// In Rust `'a` opens a lifetime and `'a'` a character; the two differ only in
// what follows the identifier. Swift has no character literal at all, so an
// apostrophe there is ordinary text.
func scanQuote(line string, position int, dialect braceDialect) int {
	if dialect.lifetimes && isLifetime(line, position) {
		return position + 1
	}
	if !dialect.charLiterals {
		return position + 1
	}
	return consumeString(line, position+1, '\'')
}

// isLifetime reports whether the quote at position opens a Rust lifetime rather
// than a character literal. `'a'` is a character: an identifier of exactly one
// character closed by a quote. Anything else beginning with an identifier
// character -- `'a`, `'static`, `'_` -- is a lifetime.
func isLifetime(line string, position int) bool {
	rest := line[position+1:]
	first, width := utf8.DecodeRuneInString(rest)
	if width == 0 || !(first == '_' || unicode.IsLetter(first)) {
		return false
	}
	return !(len(rest) > width && rest[width] == '\'')
}

// consumeString advances past a single-line quoted literal, honouring
// backslash escapes. An unterminated one ends at the line break: these
// dialects do not continue a plain quoted string across lines, so the literal
// is simply broken and the next line resumes as code rather than swallowing
// the remainder of the file.
func consumeString(line string, position int, quote byte) int {
	for position < len(line) {
		switch line[position] {
		case '\\':
			position += 2
			continue
		case quote:
			return position + 1
		}
		position++
	}
	return len(line)
}

func consumeBlockComment(line string, position int, state *braceState, dialect braceDialect) int {
	for position < len(line) {
		switch {
		case hasAt(line, position, "*/"):
			state.commentDepth--
			position += 2
			if state.commentDepth == 0 {
				return position
			}
		case dialect.nestedBlockComments && hasAt(line, position, "/*"):
			state.commentDepth++
			position += 2
		default:
			position++
		}
	}
	return len(line)
}

func consumeTriple(line string, position int, state *braceState) int {
	for position < len(line) {
		if hasAt(line, position, `"""`) {
			state.inTriple = false
			return position + 3
		}
		position++
	}
	return len(line)
}

// consumeVerbatim advances through a C# `@"..."` literal, where a doubled quote
// is an escaped quote and a single one closes the literal.
func consumeVerbatim(line string, position int, state *braceState) int {
	for position < len(line) {
		if line[position] == '"' {
			if hasAt(line, position, `""`) {
				position += 2
				continue
			}
			state.inVerbatim = false
			return position + 1
		}
		position++
	}
	return len(line)
}

// consumeRawString advances through a raw string, which ends only at a quote
// followed by exactly the run of `#` that opened it. Raw strings carry no
// escapes, which is the entire point of them.
func consumeRawString(line string, position int, state *braceState) int {
	for position < len(line) {
		if line[position] == '"' && countHashes(line, position+1) >= state.rawHashes {
			closed := position + 1 + state.rawHashes
			state.rawHashes = -1
			return closed
		}
		position++
	}
	return len(line)
}

// rawStringOpener recognises `r"`, `r#"`, `r##"` (Rust) and `#"`, `##"`
// (Swift). It reports the offset just past the opening quote and the number of
// hashes the closer must match.
func rawStringOpener(line string, position int) (int, int, bool) {
	hashes := countHashes(line, position+1)
	quote := position + 1 + hashes
	if quote >= len(line) || line[quote] != '"' {
		return 0, 0, false
	}
	// Swift spells its raw string `#"`, so the prefix character is itself the
	// first hash and must not also be counted as the marker.
	if line[position] == '#' {
		return quote + 1, hashes + 1, true
	}
	return quote + 1, hashes, true
}

func countHashes(line string, position int) int {
	count := 0
	for position+count < len(line) && line[position+count] == '#' {
		count++
	}
	return count
}

func hasAt(line string, position int, prefix string) bool {
	return strings.HasPrefix(line[position:], prefix)
}

// codeToken is one lexical unit of a code-only line: an identifier, or a single
// punctuation character. Nothing here needs to tell `<` from `<=`, so operators
// are not glued together.
type codeToken struct {
	text string
	// identifier distinguishes a name from punctuation without re-inspecting
	// the first byte at every use site.
	identifier bool
}

// tokenize splits a code-only line. Backticks are dropped so that Swift's
// escaped identifiers (“ `default` “) arrive as ordinary names.
func tokenize(line string) []codeToken {
	result := make([]codeToken, 0, 16)
	position := 0
	for position < len(line) {
		character, width := utf8.DecodeRuneInString(line[position:])
		switch {
		case unicode.IsSpace(character):
			position += width
		case character == '`':
			position += width
		case isIdentifierStart(character):
			start := position
			for position < len(line) {
				next, nextWidth := utf8.DecodeRuneInString(line[position:])
				if !isIdentifierPart(next) {
					break
				}
				position += nextWidth
			}
			result = append(result, codeToken{line[start:position], true})
		default:
			result = append(result, codeToken{line[position : position+width], false})
			position += width
		}
	}
	return result
}

func isIdentifierStart(character rune) bool {
	return character == '_' || character == '$' || unicode.IsLetter(character)
}

func isIdentifierPart(character rune) bool {
	return isIdentifierStart(character) || unicode.IsNumber(character)
}

// skipModifiers advances past the leading declaration modifiers of a line, and
// past the bracketed groups that attach to them: Rust's `pub(crate)`, C#'s
// `[Attribute(...)]`, Swift's `@available(...)`. It returns the index of the
// first token that is not part of that prelude, and whether any modifier was
// actually seen -- which C# uses as its precision guard.
func skipModifiers(tokens []codeToken, modifiers map[string]bool) (int, bool) {
	position, seen := 0, false
	for position < len(tokens) {
		current := tokens[position]
		switch {
		case current.identifier && modifiers[current.text]:
			seen = true
			position++
			// A modifier may be qualified, as in `pub(crate)` or
			// `internal(set)`; the qualifier belongs to the modifier.
			if position < len(tokens) && tokens[position].text == "(" {
				position = skipGroup(tokens, position, "(", ")")
			}
		case current.text == "@" || current.text == "[":
			seen = true
			position = skipAttribute(tokens, position)
		default:
			return position, seen
		}
	}
	return position, seen
}

// skipAttribute advances past one attribute: `@name(args)` in Swift, or a
// `[Name(args)]` group in C#.
func skipAttribute(tokens []codeToken, position int) int {
	if tokens[position].text == "[" {
		return skipGroup(tokens, position, "[", "]")
	}
	position++
	if position < len(tokens) && tokens[position].identifier {
		position++
	}
	// A dotted attribute name, `@objc.something`, keeps its qualifier.
	for position+1 < len(tokens) && tokens[position].text == "." && tokens[position+1].identifier {
		position += 2
	}
	if position < len(tokens) && tokens[position].text == "(" {
		position = skipGroup(tokens, position, "(", ")")
	}
	return position
}

// skipGroup advances past a balanced bracket group, returning the index just
// past its close. An unbalanced group runs to the end of the line, which is the
// correct reading for a declaration whose argument list wraps.
func skipGroup(tokens []codeToken, position int, open, close string) int {
	depth := 0
	for position < len(tokens) {
		switch tokens[position].text {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return position + 1
			}
		}
		position++
	}
	return len(tokens)
}

// declaration is one recognised declaration on a line.
type declaration struct {
	kind, name string
}

// languageSpec is everything that differs between the four languages once the
// lexing is shared: which words introduce a declaration, which merely qualify
// one, and the one dialect-specific rule that keyword tables cannot express.
type languageSpec struct {
	dialect   braceDialect
	modifiers map[string]bool
	// keywords maps a declaration keyword to the symbol kind it introduces.
	keywords map[string]string
	// memberOnly names the keywords whose symbols are only meaningful outside
	// a function body -- `let`, `var`, `val`, `const`. Emitting them from any
	// depth would bury every real declaration under an avalanche of locals.
	memberOnly map[string]bool
	// maxMemberDepth is the deepest brace nesting at which a memberOnly
	// keyword still names a member rather than a local.
	maxMemberDepth int
	// extra runs after the keyword table finds nothing, for declarations that
	// no leading keyword announces -- notably C# methods and properties.
	extra func(tokens []codeToken, position int, sawModifier bool, enclosing string) (declaration, bool)
}

// declarationOf finds the declaration a code-only line introduces, if any.
// inFunction reports whether the line sits inside a function body, where a
// `let`/`var`/`const` names a local rather than a member.
func (spec languageSpec) declarationOf(line string, depth int, inFunction bool, enclosing string) (declaration, bool) {
	tokens := tokenize(line)
	if len(tokens) == 0 {
		return declaration{}, false
	}
	position, sawModifier := skipModifiers(tokens, spec.modifiers)
	for position < len(tokens) {
		current := tokens[position]
		if !current.identifier {
			break
		}
		kind, isKeyword := spec.keywords[current.text]
		if !isKeyword {
			break
		}
		// A keyword that qualifies another -- `const fn`, `enum class`,
		// `data class` -- yields to it. Only the innermost keyword names the
		// construct, so scanning continues rather than emitting the outer one.
		if position+1 < len(tokens) {
			if _, nextIsKeyword := spec.keywords[tokens[position+1].text]; nextIsKeyword {
				position++
				continue
			}
		}
		// A value declaration is a member outside a function body and a local
		// inside one. Brace depth alone cannot tell them apart: a Rust
		// associated const in an `impl` block and a local in a top-level `fn`
		// both sit at depth 1. So the function-body flag is the primary rule,
		// and the depth bound remains as a backstop for the blocks that are
		// neither -- a Swift closure or a property initialiser.
		if spec.memberOnly[current.text] && (inFunction || depth > spec.maxMemberDepth) {
			return declaration{}, false
		}
		return spec.nameAfter(tokens, position, kind, current.text)
	}
	if spec.extra != nil {
		return spec.extra(tokens, position, sawModifier, enclosing)
	}
	return declaration{}, false
}

// nameAfter reads the identifier a declaration keyword introduces. Some
// keywords name themselves: Swift's `init` and `deinit` take no name, and a
// Kotlin `companion object` may have none.
func (spec languageSpec) nameAfter(tokens []codeToken, position int, kind, keyword string) (declaration, bool) {
	if selfNamed[keyword] {
		return declaration{kind, keyword}, true
	}
	next := position + 1
	if next >= len(tokens) {
		return declaration{}, false
	}
	// `macro_rules! name` puts its bang between the keyword and the name.
	if tokens[next].text == "!" {
		next++
	}
	if next >= len(tokens) || !tokens[next].identifier {
		return declaration{}, false
	}
	// A delegate is spelled like a method signature -- `delegate bool Name(..)`
	// -- so the identifier straight after the keyword is its RETURN TYPE, not
	// its name. Reading it positionally reported every delegate as a type
	// called `bool` or `void`.
	if parenNamed[keyword] {
		if name, ok := csharpMethodName(tokens[next:], ""); ok {
			return declaration{kind, name}, true
		}
		return declaration{}, false
	}
	// An event declares `event Action<string> Log;`, so like a field its name
	// trails its type. Reading the token straight after the keyword named
	// every event after the delegate type it carries.
	if trailingNamed[keyword] {
		if name, ok := csharpFieldName(tokens[next:]); ok {
			return declaration{kind, name}, true
		}
		return declaration{}, false
	}
	name, _ := qualifiedName(tokens, next)
	// A dotted name means different things either side of this line. For a
	// namespace or a type it IS the name, so `namespace A.B.C` is one module.
	// For a function or a value it is a Kotlin extension receiver -- in
	// `fun RecordingBridge.loadfileCount()` the declared name is the last
	// segment and `RecordingBridge` is the type being extended. Taking the
	// first segment, as a positional read does, named every extension function
	// after its receiver instead. Go's method symbols drop their receiver the
	// same way (parse.go:262), so the last segment is also the spelling a
	// caller searching this index will type.
	if kind == "func" || kind == "var" {
		if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
			name = name[dot+1:]
		}
	}
	// A Rust `impl` block and a Swift `extension` name an existing type rather
	// than declaring one, but they are the only readable anchor for the
	// methods beneath them, so they are reported under their own kind.
	return declaration{kind, name}, true
}

// parenNamed lists the keywords whose name sits before a parameter list rather
// than straight after the keyword.
var parenNamed = map[string]bool{"delegate": true}

// trailingNamed lists the keywords whose name follows a type rather than the
// keyword itself.
var trailingNamed = map[string]bool{"event": true}

// qualifiedName joins a dotted name into the single identifier it denotes, so
// `namespace Serilog.Tests.Core` is one module rather than the first of its
// three segments. It returns the index just past the name.
func qualifiedName(tokens []codeToken, position int) (string, int) {
	name := tokens[position].text
	position++
	for position+1 < len(tokens) && tokens[position].text == "." && tokens[position+1].identifier {
		name += "." + tokens[position+1].text
		position += 2
	}
	return name, position
}

// selfNamed lists the keywords that are their own symbol name.
var selfNamed = map[string]bool{"init": true, "deinit": true, "subscript": true}

func words(list ...string) map[string]bool {
	result := make(map[string]bool, len(list))
	for _, word := range list {
		result[word] = true
	}
	return result
}

var rustSpec = languageSpec{
	dialect: rustDialect,
	modifiers: words(
		"pub", "async", "unsafe", "extern", "default", "move", "ref",
	),
	keywords: map[string]string{
		"fn": "func", "struct": "type", "enum": "type", "trait": "type",
		"union": "type", "type": "type", "mod": "module",
		"macro_rules": "macro", "const": "var", "static": "var",
		"impl": "impl",
	},
	memberOnly:     words("const", "static"),
	maxMemberDepth: 1,
}

var swiftSpec = languageSpec{
	dialect: swiftDialect,
	modifiers: words(
		"public", "private", "internal", "fileprivate", "open", "static",
		"final", "override", "required", "convenience", "mutating",
		"nonmutating", "lazy", "weak", "unowned", "dynamic", "optional",
		"indirect", "distributed", "nonisolated", "package",
	),
	keywords: map[string]string{
		"func": "func", "class": "type", "struct": "type", "enum": "type",
		"protocol": "type", "actor": "type", "extension": "type",
		"typealias": "type", "associatedtype": "type",
		"init": "func", "deinit": "func", "subscript": "func",
		"var": "var", "let": "var",
	},
	memberOnly:     words("var", "let"),
	maxMemberDepth: 1,
}

var kotlinSpec = languageSpec{
	dialect: kotlinDialect,
	modifiers: words(
		"public", "private", "protected", "internal", "open", "final",
		"abstract", "override", "sealed", "data", "inner", "annotation",
		"companion", "suspend", "inline", "infix", "operator", "external",
		"lateinit", "tailrec", "vararg", "actual", "expect", "const",
		"value", "crossinline", "noinline", "reified",
	),
	keywords: map[string]string{
		"fun": "func", "class": "type", "interface": "type", "object": "type",
		"typealias": "type", "enum": "type",
		"val": "var", "var": "var",
	},
	memberOnly:     words("val", "var"),
	maxMemberDepth: 1,
}

var csharpSpec = languageSpec{
	dialect: csharpDialect,
	modifiers: words(
		"public", "private", "protected", "internal", "static", "abstract",
		"virtual", "override", "sealed", "async", "partial", "extern",
		"unsafe", "new", "readonly", "const", "volatile", "required",
		"file", "implicit", "explicit", "ref", "unchecked",
	),
	keywords: map[string]string{
		"class": "type", "struct": "type", "interface": "type",
		"enum": "type", "record": "type", "delegate": "type",
		"namespace": "module", "event": "var",
	},
	// C# member depth varies with whether the file uses a block-scoped or a
	// file-scoped namespace, so depth cannot separate members from locals here.
	// csharpMember uses the modifier prelude for that instead.
	maxMemberDepth: -1,
	extra:          csharpMember,
}

// csharpMember recognises the two C# declarations that no keyword introduces:
// methods and properties. Both are spelled as a type followed by a name, which
// is also how a local variable and a method call are spelled, so the rule is
// anchored on the modifier prelude.
//
// Requiring a modifier is a deliberate trade. It costs the interface members
// and top-level statements that carry none; it buys immunity from every method
// call and local declaration inside a method body, which are vastly more
// numerous. A rule that matched `Foo(` anywhere would report call sites as
// declarations, which is the failure this file exists to avoid.
func csharpMember(tokens []codeToken, position int, sawModifier bool, enclosing string) (declaration, bool) {
	if !sawModifier || position >= len(tokens) {
		return declaration{}, false
	}
	remaining := tokens[position:]
	if name, ok := csharpMethodName(remaining, enclosing); ok {
		return declaration{"func", name}, true
	}
	if name, ok := csharpPropertyName(remaining); ok {
		return declaration{"var", name}, true
	}
	if name, ok := csharpFieldName(remaining); ok {
		return declaration{"var", name}, true
	}
	return declaration{}, false
}

// csharpFieldName recognises `private readonly int count;` and
// `public const string Name = "x";`: a type, a name, and a terminator, with
// neither a parameter list to make it a method nor a brace to make it a
// property. It runs last so those two claim their shapes first.
func csharpFieldName(tokens []codeToken) (string, bool) {
	terminator := -1
	for position, current := range tokens {
		if current.text == "(" || current.text == "{" {
			return "", false
		}
		if current.text == ";" || current.text == "=" {
			terminator = position
			break
		}
	}
	// A field runs to a terminator, and needs a type and a name before it.
	if terminator < 2 {
		return "", false
	}
	name := tokens[terminator-1]
	if !name.identifier || csharpStatementKeywords[name.text] {
		return "", false
	}
	hasType := false
	for _, current := range tokens[:terminator-1] {
		if current.identifier {
			hasType = true
			break
		}
	}
	if !hasType {
		return "", false
	}
	return name.text, true
}

// csharpMethodName finds the identifier immediately before the parameter list.
// A method needs a return type before that name; a constructor has none, and is
// recognised instead by matching the enclosing type's name.
func csharpMethodName(tokens []codeToken, enclosing string) (string, bool) {
	open := -1
	for position, current := range tokens {
		if current.text == "(" {
			open = position
			break
		}
		// A `=` before any parenthesis means this is an assignment, not a
		// signature: `private int count = Compute();` is a field.
		if current.text == "=" {
			return "", false
		}
	}
	if open < 1 {
		return "", false
	}
	name := tokens[open-1]
	if !name.identifier || csharpStatementKeywords[name.text] {
		return "", false
	}
	// A generic method's name sits before its type parameters, `Foo<T>(`.
	if name.text == ">" {
		return "", false
	}
	hasReturnType := false
	for _, current := range tokens[:open-1] {
		if current.identifier {
			hasReturnType = true
			break
		}
	}
	if hasReturnType || (enclosing != "" && name.text == enclosing) {
		return name.text, true
	}
	return "", false
}

// csharpPropertyName recognises `public int Total { get; set; }`: a type, a
// name, and a brace, with no parameter list to make it a method.
func csharpPropertyName(tokens []codeToken) (string, bool) {
	brace := -1
	for position, current := range tokens {
		switch current.text {
		case "(", "=", ";":
			return "", false
		case "{":
			brace = position
		}
		if brace >= 0 {
			break
		}
	}
	if brace < 2 {
		return "", false
	}
	name := tokens[brace-1]
	if !name.identifier || csharpStatementKeywords[name.text] {
		return "", false
	}
	if !tokens[brace-2].identifier && tokens[brace-2].text != ">" && tokens[brace-2].text != "]" {
		return "", false
	}
	return name.text, true
}

// csharpStatementKeywords are the words that introduce a statement whose shape
// is otherwise indistinguishable from a signature.
var csharpStatementKeywords = words(
	"if", "while", "for", "foreach", "switch", "catch", "lock", "using",
	"return", "fixed", "do", "else", "try", "finally", "yield", "when",
	"nameof", "typeof", "sizeof", "checked", "stackalloc", "throw", "get",
	"set", "add", "remove", "init", "where", "select", "from",
)

// braceSymbols is the shared driver for all four languages. It walks the file
// once, reducing each line to its code and asking the language spec what that
// line declares.
//
// Every early exit from this walk appends a note. That is the whole contract of
// this function: a caller that receives symbols and no notes may rely on the
// walk having reached the end of the file, and a caller that receives notes
// knows exactly which limit it hit.
func braceSymbols(source Source, text string, spec languageSpec) ([]Symbol, []ExtractionNote) {
	symbols := make([]Symbol, 0)
	notes := make([]ExtractionNote, 0)
	note := func(reason string) {
		notes = append(notes, ExtractionNote{source.Path, reason})
	}

	state := newBraceState()
	enclosing := ""
	// functionBodyDepth is the brace depth at which the innermost enclosing
	// function body began, or -1 when the walk is not inside one.
	functionBodyDepth := -1
	lines := strings.Split(text, "\n")
	scanned := lines
	if len(scanned) > maxSourceLines {
		scanned = scanned[:maxSourceLines]
		note(noteLineCap)
	}
	for offset, line := range scanned {
		depth := state.depth
		if functionBodyDepth >= 0 && depth < functionBodyDepth {
			functionBodyDepth = -1
		}
		code := codeOnly(line, state, spec.dialect)
		if strings.TrimSpace(code) == "" {
			continue
		}
		found, ok := spec.declarationOf(code, depth, functionBodyDepth >= 0, enclosing)
		if !ok {
			continue
		}
		// A function whose line also opens a brace begins a body, and every
		// value declaration inside it is a local until the depth falls back.
		if found.kind == "func" && state.depth > depth && functionBodyDepth < 0 {
			functionBodyDepth = depth + 1
		}
		// A type declaration becomes the enclosing name for the members below
		// it, which is how a C# constructor is told from a method.
		if found.kind == "type" {
			enclosing = found.name
		}
		// `impl` and Swift `extension` anchor the methods beneath them but do
		// not declare a new symbol, so they set the enclosing name and stop.
		if found.kind == "impl" {
			enclosing = found.name
			continue
		}
		if len(symbols) >= maxSymbolsPerSource {
			note(noteSymbolCap)
			break
		}
		symbols = append(symbols, Symbol{Kind: found.kind, Name: found.name, Path: source.Path, BlobHash: source.BlobHash, Line: offset + 1})
	}
	if reason, unresolved := state.open(); unresolved {
		note(reason)
	}
	return symbols, notes
}

func rustSymbols(source Source, text string) ([]Symbol, []ExtractionNote) {
	return braceSymbols(source, text, rustSpec)
}

func swiftSymbols(source Source, text string) ([]Symbol, []ExtractionNote) {
	return braceSymbols(source, text, swiftSpec)
}

func kotlinSymbols(source Source, text string) ([]Symbol, []ExtractionNote) {
	return braceSymbols(source, text, kotlinSpec)
}

func csharpSymbols(source Source, text string) ([]Symbol, []ExtractionNote) {
	return braceSymbols(source, text, csharpSpec)
}

// languageSymbols dispatches on file extension. It returns false for a suffix
// with no extractor, so the caller can tell "this language has no symbol
// support" from "this file declares nothing", which are very different facts.
func languageSymbols(source Source, text string) ([]Symbol, []ExtractionNote, bool) {
	extractor, known := symbolExtractors[strings.ToLower(pathExt(source.Path))]
	if !known {
		return nil, nil, false
	}
	symbols, notes := extractor(source, normalizeLineEndings(text))
	return symbols, notes, true
}

// normalizeLineEndings rewrites a classic-Mac source, whose lines end in a bare
// carriage return, so the line scanners see its lines.
//
// Every extractor here splits on "\n". A CR-only file therefore arrives as a
// single enormous line: the scanner reads one declaration, misses every other
// one, and -- because it did reach the end of the file -- reports no note at
// all. That is precisely the silent-truncation shape these extractors exist to
// avoid, so it is corrected rather than merely recorded. CRLF already works,
// because the trailing CR lexes as whitespace.
//
// The check is two substring scans and allocates nothing for the ordinary file;
// no source in a 26,559-file corpus took this branch.
func normalizeLineEndings(text string) string {
	if !strings.Contains(text, "\r") || strings.Contains(text, "\n") {
		return text
	}
	return strings.ReplaceAll(text, "\r", "\n")
}

var symbolExtractors = map[string]func(Source, string) ([]Symbol, []ExtractionNote){
	".rs":    rustSymbols,
	".cs":    csharpSymbols,
	".swift": swiftSymbols,
	".kt":    kotlinSymbols,
	".kts":   kotlinSymbols,
	".rb":    rubySymbols,
	".ts":    webSymbols,
	".tsx":   webSymbols,
	".js":    webSymbols,
	".jsx":   webSymbols,
	".mjs":   webSymbols,
	".cjs":   webSymbols,
}

// pathExt is path.Ext over the base name, kept local so this file does not
// depend on the caller having already split the path.
func pathExt(value string) string {
	if slash := strings.LastIndexByte(value, '/'); slash >= 0 {
		value = value[slash+1:]
	}
	if dot := strings.LastIndexByte(value, '.'); dot >= 0 {
		return value[dot:]
	}
	return ""
}
