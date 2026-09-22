package contextindex

import "strings"

// rubySymbols extracts the declarations of one Ruby source.
//
// There is no closed Ruby grammar in this repository the way there is for
// Python, so this is a lexical extractor: it blanks every comment, string,
// percent literal and heredoc body first, then matches declarations on what is
// left. The blanking pass is the whole point. A scanner that matches `def` and
// `class` against raw lines reports the contents of SQL heredocs, RSpec
// docstrings and commented-out code as declarations, and once those false
// symbols are in the index nothing distinguishes them from real ones.
//
// Every path that stops short of the end of the file, or that reaches the end
// with a lexical state it could not resolve, returns an ExtractionNote. A
// caller that sees no note may treat the returned symbols as the file's whole
// declaration set; that guarantee is what the notes buy.
func rubySymbols(source Source, text string) ([]Symbol, []ExtractionNote) {
	symbols := make([]Symbol, 0)
	notes := make([]ExtractionNote, 0)
	scanner := rubyScanner{}
	remaining := text
	lineNumber := 0

	for remaining != "" {
		lineNumber++
		if lineNumber > maxSourceLines {
			return symbols, append(notes, ExtractionNote{source.Path, noteLineCap})
		}
		line, rest, _ := strings.Cut(remaining, "\n")
		remaining = rest
		for _, declaration := range rubyDeclarations(scanner.scanLine(line)) {
			if len(symbols) >= maxSymbolsPerSource {
				return symbols, append(notes, ExtractionNote{source.Path, noteSymbolCap})
			}
			symbols = append(symbols, Symbol{Kind: declaration.kind, Name: declaration.name, Path: source.Path, BlobHash: source.BlobHash, Line: lineNumber})
		}
	}

	// The file was walked to its end, so the only remaining way to have missed
	// declarations is to have been inside something that never closed. Both
	// states swallowed every line after the opener.
	if scanner.inBlockComment {
		notes = append(notes, ExtractionNote{source.Path, noteUnterminatedComment})
	}
	if scanner.literal != nil || len(scanner.heredocs) > 0 {
		notes = append(notes, ExtractionNote{source.Path, noteUnterminatedString})
	}
	return symbols, notes
}

// rubyScanner carries the lexical state that outlives a physical line: a
// `=begin` block comment, a string or percent literal that has not closed, and
// the queue of heredoc bodies still owed to openers already seen. It is a value
// created per call, never shared, so rubySymbols is safe to call concurrently
// from the chunk walker.
type rubyScanner struct {
	inBlockComment bool
	literal        *rubyLiteral
	heredocs       []rubyHeredoc
}

// rubyLiteral is one string-like literal in progress. The four bracket pairs
// nest, which is why a percent literal tracks its own depth: `%w[a[b]c]` closes
// at the last bracket, not the first.
type rubyLiteral struct {
	opener, terminator byte
	depth              int
	interpolates       bool
}

// rubyHeredoc is one heredoc body owed by an opener on an earlier line.
// allowIndent records the `~`/`-` forms, whose terminator may be indented.
type rubyHeredoc struct {
	terminator  string
	allowIndent bool
}

// rubyStrip holds one physical line beside the copy in which literal and
// comment bytes have been replaced by spaces. The copy is allocated only once a
// line actually has something to blank, which most lines do not; blanking in
// place rather than deleting keeps every surviving byte at its original column,
// which the start-of-line constant rule depends on.
type rubyStrip struct {
	line    string
	blanked []byte
}

func (strip *rubyStrip) blank(from, to int) {
	if strip.blanked == nil {
		strip.blanked = []byte(strip.line)
	}
	for position := from; position < to && position < len(strip.blanked); position++ {
		strip.blanked[position] = ' '
	}
}

func (strip *rubyStrip) code() string {
	if strip.blanked == nil {
		return strip.line
	}
	return string(strip.blanked)
}

// scanLine returns the code-only form of one physical line and advances the
// scanner's carried state. A line that is entirely data returns "".
func (scanner *rubyScanner) scanLine(line string) string {
	// A heredoc body is data even when it is full of `def` and `class`; the
	// queue is drained in opening order because that is the order Ruby reads
	// the bodies of several heredocs opened on one line.
	if len(scanner.heredocs) > 0 {
		if rubyHeredocEnds(line, scanner.heredocs[0]) {
			scanner.heredocs = scanner.heredocs[1:]
		}
		return ""
	}
	if scanner.inBlockComment {
		if rubyBlockCommentMarker(line, "=end") {
			scanner.inBlockComment = false
		}
		return ""
	}
	// `=begin` is only a comment opener at column 0, and only outside a
	// literal: a heredoc or multi-line string may legitimately contain a line
	// beginning with it.
	if scanner.literal == nil && rubyBlockCommentMarker(line, "=begin") {
		scanner.inBlockComment = true
		return ""
	}

	strip := rubyStrip{line: line}
	index := 0
	afterValue := false
	if scanner.literal != nil {
		end, closed := scanner.literal.consume(line, 0)
		strip.blank(0, end)
		if !closed {
			return ""
		}
		scanner.literal = nil
		index, afterValue = end, true
	}

	for index < len(line) {
		character := line[index]

		if character == '#' {
			strip.blank(index, len(line))
			break
		}
		if character == '\'' || character == '"' || character == '`' {
			// Single quotes escape only \' and \\, but treating a backslash as
			// escaping whatever follows it terminates both forms identically.
			literal := rubyLiteral{terminator: character, interpolates: character != '\''}
			index, afterValue = scanner.openLiteral(&strip, &literal, index, index+1), true
			continue
		}
		if character == '%' {
			if literal, body, ok := rubyPercentLiteral(line, index, afterValue); ok {
				index, afterValue = scanner.openLiteral(&strip, &literal, index, body), true
				continue
			}
		}
		if character == '<' && index+1 < len(line) && line[index+1] == '<' {
			if heredoc, after, ok := rubyHeredocOpener(line, index); ok {
				scanner.heredocs = append(scanner.heredocs, heredoc)
				strip.blank(index, after)
				index, afterValue = after, true
				continue
			}
		}
		if character == '?' && !afterValue {
			if after, ok := rubyCharacterLiteral(line, index); ok {
				strip.blank(index, after)
				index, afterValue = after, true
				continue
			}
		}
		if character == '/' {
			if after, ok := rubyRegexLiteral(line, index, afterValue); ok {
				strip.blank(index, after)
				index, afterValue = after, true
				continue
			}
		}
		if rubyWordByte(character) {
			word, after := rubyWord(line, index)
			index, afterValue = after, !rubyExpressionKeywords[word]
			continue
		}
		index++
		afterValue = character == ')' || character == ']' || character == '}'
	}
	return strip.code()
}

// openLiteral consumes a literal whose marker starts at markerStart and whose
// body starts at bodyStart, blanking the whole span. A literal that does not
// close on this line is carried into the next, which is what makes a multi-line
// `%w` list or a `"..."` spanning lines safe rather than a source of false
// symbols from the lines it covers.
func (scanner *rubyScanner) openLiteral(strip *rubyStrip, literal *rubyLiteral, markerStart, bodyStart int) int {
	end, closed := literal.consume(strip.line, bodyStart)
	strip.blank(markerStart, end)
	if !closed {
		scanner.literal = literal
	}
	return end
}

// consume advances through a literal body, returning the index just past the
// closing delimiter and whether the literal closed on this line.
func (literal *rubyLiteral) consume(line string, index int) (int, bool) {
	for index < len(line) {
		character := line[index]
		switch {
		case character == '\\':
			index += 2
		case literal.interpolates && character == '#' && index+1 < len(line) && line[index+1] == '{':
			// `#` inside an interpolating literal is not a comment, and the
			// interpolation's own braces and quotes have to be stepped over or
			// `"#{row["id"]}"` closes at the wrong quote.
			index = rubyInterpolationEnd(line, index+2)
		case literal.opener != 0 && character == literal.opener:
			literal.depth++
			index++
		case character == literal.terminator:
			if literal.depth == 0 {
				return index + 1, true
			}
			literal.depth--
			index++
		default:
			index++
		}
	}
	return len(line), false
}

// rubyInterpolationEnd returns the index just past the `}` that closes a
// `#{...}` interpolation opened at index.
func rubyInterpolationEnd(line string, index int) int {
	depth := 0
	for index < len(line) {
		switch line[index] {
		case '\\':
			index++
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return index + 1
			}
			depth--
		case '\'', '"':
			quote := line[index]
			index++
			for index < len(line) && line[index] != quote {
				if line[index] == '\\' {
					index++
				}
				index++
			}
		}
		index++
	}
	return len(line)
}

// rubyBlockCommentMarker reports whether the line is a `=begin`/`=end` marker.
// Ruby recognises both only at column 0 and only as whole words, so `=beginner`
// is an ordinary expression.
func rubyBlockCommentMarker(line, marker string) bool {
	if !strings.HasPrefix(line, marker) {
		return false
	}
	rest := line[len(marker):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\r'
}

// rubyPercentLiteral recognises `%w[..]`, `%i(..)`, `%q{..}`, `%Q<..>`, `%r|..|`
// and the bare `%(..)`. It returns the literal and the index its body starts at.
func rubyPercentLiteral(line string, index int, afterValue bool) (rubyLiteral, int, bool) {
	cursor := index + 1
	interpolates := true
	if cursor < len(line) && rubyLetterByte(line[cursor]) {
		switch line[cursor] {
		case 'q', 'w', 'i', 's':
			interpolates = false
		case 'Q', 'W', 'I', 'r', 'x':
		default:
			return rubyLiteral{}, 0, false
		}
		cursor++
	} else if afterValue {
		// A bare `%` after a value is the modulo operator. Only the lettered
		// forms are accepted there, because `total % width` is overwhelmingly
		// more common in real code than `total %(...)`.
		return rubyLiteral{}, 0, false
	}
	if cursor >= len(line) || !rubyPercentDelimiter(line[cursor]) {
		return rubyLiteral{}, 0, false
	}
	opener, terminator := rubyDelimiterPair(line[cursor])
	return rubyLiteral{opener: opener, terminator: terminator, interpolates: interpolates}, cursor + 1, true
}

// rubyPercentDelimiter reports whether a byte may delimit a percent literal.
// `=` is excluded so `count %= 2` is not read as a literal.
func rubyPercentDelimiter(character byte) bool {
	if character >= 0x80 || character == '=' || character == ' ' || character == '\t' {
		return false
	}
	return !rubyWordByte(character)
}

// rubyDelimiterPair returns the nesting opener and the terminator for a percent
// literal's delimiter. Only the four bracket pairs nest; every other delimiter
// closes on its own repetition, so it has no opener.
func rubyDelimiterPair(delimiter byte) (byte, byte) {
	switch delimiter {
	case '[':
		return '[', ']'
	case '(':
		return '(', ')'
	case '{':
		return '{', '}'
	case '<':
		return '<', '>'
	}
	return 0, delimiter
}

// rubyHeredocOpener recognises `<<EOS`, `<<-EOS`, `<<~EOS` and their quoted
// forms, returning the heredoc and the index just past the opener.
func rubyHeredocOpener(line string, index int) (rubyHeredoc, int, bool) {
	cursor := index + 2
	marked := false
	if cursor < len(line) && (line[cursor] == '~' || line[cursor] == '-') {
		marked = true
		cursor++
	}
	if cursor < len(line) && (line[cursor] == '\'' || line[cursor] == '"') {
		quote := line[cursor]
		cursor++
		start := cursor
		for cursor < len(line) && line[cursor] != quote {
			cursor++
		}
		if cursor >= len(line) || cursor == start {
			return rubyHeredoc{}, 0, false
		}
		return rubyHeredoc{line[start:cursor], marked}, cursor + 1, true
	}
	tag, after := rubyWord(line, cursor)
	if tag == "" {
		return rubyHeredoc{}, 0, false
	}
	// Without `~` or `-`, `<<` is far more often the append operator than a
	// heredoc opener. Ruby requires the tag to touch the `<<`, and by universal
	// convention it is uppercase; both conditions together are what separate
	// `<<EOS` from `handlers << Handler`, whose space and lower risk of
	// misreading matter because a false heredoc swallows the rest of the file.
	if !marked && !rubyUpperByte(line[cursor]) && line[cursor] != '_' {
		return rubyHeredoc{}, 0, false
	}
	return rubyHeredoc{tag, marked}, after, true
}

// rubyHeredocEnds reports whether a line is the terminator of heredoc. Only the
// `~` and `-` forms permit an indented terminator; a plain `<<EOS` closes only
// on a line that is exactly its tag.
func rubyHeredocEnds(line string, heredoc rubyHeredoc) bool {
	candidate := strings.TrimRight(line, "\r")
	if heredoc.allowIndent {
		candidate = strings.TrimSpace(candidate)
	}
	return candidate == heredoc.terminator
}

// rubyCharacterLiteral matches `?a` and `?\n`. It earns its place only because
// `?"` and `?#` would otherwise open a string or a comment; the literal's value
// is never wanted.
func rubyCharacterLiteral(line string, index int) (int, bool) {
	if index+1 >= len(line) || line[index+1] == ' ' || line[index+1] == '\t' {
		return 0, false
	}
	if line[index+1] == '\\' {
		return min(index+3, len(line)), true
	}
	if index+2 < len(line) && rubyWordByte(line[index+2]) {
		// `?ab` is a ternary on a one-letter receiver, not a character.
		return 0, false
	}
	return index + 2, true
}

// rubyRegexLiteral resolves the `/` ambiguity conservatively. A regex is
// recognised only where a value may begin, or in the `split /pattern/` shape
// Ruby's own lexer keys on (space before the slash, none after), AND only when
// it closes on the same line. Requiring the close is what bounds the damage: a
// misread `/` costs the rest of one line, never the rest of the file, which is
// what a division carried forward as an open literal would cost.
//
// What this gives up: a regex spelled across several lines (the `/x` extended
// form) is scanned as code, so a `def`- or `class`-shaped fragment inside one
// can contribute a false symbol; and a regex passed as a bare argument with no
// space before it, or one whose closing slash is on another line, is read as
// division. Both were judged cheaper than the alternative, which is a whole
// file silently swallowed by a string opened inside an unrecognised regex.
func rubyRegexLiteral(line string, index int, afterValue bool) (int, bool) {
	spacedArgument := index > 0 && line[index-1] == ' ' &&
		index+1 < len(line) && line[index+1] != ' ' && line[index+1] != '='
	if afterValue && !spacedArgument {
		return 0, false
	}
	for cursor := index + 1; cursor < len(line); cursor++ {
		if line[cursor] == '\\' {
			cursor++
			continue
		}
		if line[cursor] == '/' {
			end := cursor + 1
			for end < len(line) && rubyLetterByte(line[end]) {
				end++
			}
			return end, true
		}
	}
	return 0, false
}

// rubyDeclaration is one declaration found on a stripped line.
type rubyDeclaration struct {
	kind, name string
}

// rubyExpressionKeywords are the keywords after which a value may begin, so a
// following `/` or `%` opens a literal instead of dividing. Method names are
// deliberately absent: guessing which identifiers take a bare regex argument
// would trade a bounded miss for an unbounded one.
var rubyExpressionKeywords = map[string]bool{
	"and": true, "begin": true, "case": true, "do": true, "else": true,
	"elsif": true, "ensure": true, "if": true, "in": true, "not": true,
	"or": true, "rescue": true, "return": true, "then": true, "unless": true,
	"until": true, "when": true, "while": true, "yield": true,
}

// rubyOperatorMethods are the method names Ruby spells with punctuation,
// ordered longest first so `<=>` is not read as `<` and `[]=` not as `[]`.
var rubyOperatorMethods = []string{
	"[]=", "<=>", "===",
	"**", "==", "=~", "!=", "!~", "[]", "<<", ">>", "<=", ">=", "+@", "-@",
	"<", ">", "+", "-", "*", "/", "%", "!", "~", "&", "|", "^",
}

// rubyDeclarations reports the declarations on one stripped line. A line may
// carry several: `def a; end; def b; end` is one line and two methods.
func rubyDeclarations(code string) []rubyDeclaration {
	var declarations []rubyDeclaration
	if constant, ok := rubyConstantAssignment(code); ok {
		declarations = append(declarations, constant)
	}
	for index := 0; index < len(code); index++ {
		if !rubyKeywordStart(code, index) {
			continue
		}
		word, after := rubyWord(code, index)
		switch word {
		case "def":
			if name, ok := rubyMethodName(code, after); ok {
				declarations = append(declarations, rubyDeclaration{"func", name})
			}
		case "class":
			if name, ok := rubyConstantPath(code, after); ok {
				declarations = append(declarations, rubyDeclaration{"type", name})
			}
		case "module":
			if name, ok := rubyConstantPath(code, after); ok {
				declarations = append(declarations, rubyDeclaration{"module", name})
			}
		}
		index = after - 1
	}
	return declarations
}

// rubyKeywordStart reports whether index can begin a declaration keyword.
// Excluding a preceding word byte is what keeps `definitely` from matching
// `def`; excluding `.`, `:`, `@` and `$` rules out `record.class`, `:class` and
// `@def`, which are a receiver call, a symbol and a variable, not declarations.
func rubyKeywordStart(code string, index int) bool {
	if code[index] < 'a' || code[index] > 'z' {
		return false
	}
	if index == 0 {
		return true
	}
	previous := code[index-1]
	return !rubyWordByte(previous) && previous != '.' && previous != ':' &&
		previous != '@' && previous != '$'
}

// rubyMethodName reads the name a `def` introduces, dropping any receiver:
// `def self.call`, `def logger.warn` and `def @io.rewind` name `call`, `warn`
// and `rewind`, because the index records what was defined, not where it was
// hung. The receiver is only consumed when a `.` follows it, so a `def` whose
// name this cannot parse reports nothing rather than reporting the receiver.
func rubyMethodName(code string, index int) (string, bool) {
	cursor, spaced := rubySkipSpaces(code, index)
	if !spaced {
		return "", false
	}
	receiver := rubySkipSigil(code, cursor)
	if word, after := rubyWord(code, receiver); word != "" && after < len(code) && code[after] == '.' {
		cursor = after + 1
	}
	for _, operator := range rubyOperatorMethods {
		if strings.HasPrefix(code[cursor:], operator) {
			return operator, true
		}
	}
	word, after := rubyWord(code, cursor)
	if word == "" {
		return "", false
	}
	return word + rubyMethodSuffix(code, after), true
}

// rubyMethodSuffix returns the `?`, `!` or `=` that belongs to a method name.
// The `=` counts only when it touches the identifier: `def foo=(value)` defines
// the setter `foo=`, while Ruby 3's endless `def foo = 42` defines `foo`.
func rubyMethodSuffix(code string, index int) string {
	if index >= len(code) {
		return ""
	}
	if code[index] == '?' || code[index] == '!' {
		return code[index : index+1]
	}
	if code[index] != '=' {
		return ""
	}
	if index+1 < len(code) && (code[index+1] == '=' || code[index+1] == '>' || code[index+1] == '~') {
		return ""
	}
	return "="
}

// rubyConstantPath reads the constant naming a class or module, after the space
// that must separate it from its keyword. Requiring an uppercase start is what
// makes `class << self` name nothing: a singleton class has no name to record.
func rubyConstantPath(code string, index int) (string, bool) {
	cursor, spaced := rubySkipSpaces(code, index)
	if !spaced {
		return "", false
	}
	return rubyConstantPathAt(code, cursor)
}

// rubyConstantPathAt reads `Foo` or `Foo::Bar` at index. The qualified form is
// kept whole because `Foo::Bar` and a nested `Bar` are different declarations
// to anyone searching the index.
func rubyConstantPathAt(code string, index int) (string, bool) {
	if index >= len(code) || !rubyUpperByte(code[index]) {
		return "", false
	}
	cursor := index
	for {
		_, after := rubyWord(code, cursor)
		cursor = after
		if cursor+2 < len(code) && code[cursor] == ':' && code[cursor+1] == ':' && rubyWordByte(code[cursor+2]) {
			cursor += 2
			continue
		}
		return code[index:cursor], true
	}
}

// rubyConstantAssignment matches a constant defined at the start of a line.
// Ruby has no keyword for it, so the position is the whole signal: requiring
// the constant to be the line's first token excludes `other.CONST`, an argument
// and a hash value, none of which define anything here.
func rubyConstantAssignment(code string) (rubyDeclaration, bool) {
	cursor, _ := rubySkipSpaces(code, 0)
	name, ok := rubyConstantPathAt(code, cursor)
	if !ok {
		return rubyDeclaration{}, false
	}
	cursor, _ = rubySkipSpaces(code, cursor+len(name))
	if cursor >= len(code) || code[cursor] != '=' {
		return rubyDeclaration{}, false
	}
	// `==`, `=>` and `=~` compare, key and match rather than assign. The
	// operator-assignments (`||=`, `+=`) never reach here: their own operator
	// sits between the name and the `=`, so the space skip stops short of it.
	if cursor+1 < len(code) && (code[cursor+1] == '=' || code[cursor+1] == '>' || code[cursor+1] == '~') {
		return rubyDeclaration{}, false
	}
	return rubyDeclaration{"var", name}, true
}

// rubySkipSigil steps over the `@`, `@@` or `$` that marks a variable used as a
// method's receiver, as in the `def @io.rewind` singleton form.
func rubySkipSigil(code string, index int) int {
	cursor := index
	if cursor < len(code) && code[cursor] == '$' {
		return cursor + 1
	}
	for cursor < len(code) && cursor < index+2 && code[cursor] == '@' {
		cursor++
	}
	return cursor
}

func rubySkipSpaces(code string, index int) (int, bool) {
	cursor := index
	for cursor < len(code) && (code[cursor] == ' ' || code[cursor] == '\t') {
		cursor++
	}
	return cursor, cursor > index
}

func rubyWord(text string, index int) (string, int) {
	if index >= len(text) || !rubyWordByte(text[index]) {
		return "", index
	}
	cursor := index
	for cursor < len(text) && rubyWordByte(text[cursor]) {
		cursor++
	}
	return text[index:cursor], cursor
}

// rubyWordByte treats every byte above ASCII as part of a word. Ruby permits
// non-ASCII identifiers, and folding the continuation bytes in keeps `définir`
// one token rather than a `déf`-shaped prefix that could match a keyword.
func rubyWordByte(character byte) bool {
	switch {
	case character >= 'a' && character <= 'z':
		return true
	case character >= 'A' && character <= 'Z':
		return true
	case character >= '0' && character <= '9':
		return true
	}
	return character == '_' || character >= 0x80
}

func rubyLetterByte(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

func rubyUpperByte(character byte) bool {
	return character >= 'A' && character <= 'Z'
}
