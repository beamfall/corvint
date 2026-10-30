package pythongrammar

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

type Token struct {
	Kind         byte
	Start, End   int
	line, column int
}

const (
	KindName byte = iota + 1
	KindNumber
	KindString
	KindSymbol
	KindNewline
	KindIndent
	KindDedent
)

type nativeLexer struct {
	Source                 []byte
	index, line, column    int
	delimiters             []byte
	indentation            []int
	altIndentation         []int
	atLineStart, continued bool
	Tokens                 []Token
}

type nativeDecorator struct {
	name         string
	line, column int
}

type Parser struct {
	subject        string
	Source         []byte
	Tokens         []Token
	collector      FactSink
	spans          DefinitionSpanSink
	definitions    []openDefinition
	definitionLine int
	contentEndLine int
	decorators     []nativeDecorator
	suites         []bool
	awaitMatch     bool
	awaitIndent    bool
	validateSyntax bool
}

// openDefinition is one definition whose suite has been entered and not yet
// closed. depth is the suite depth the definition statement itself sat at, so
// the dedent that returns the parser to it is the one that ends the body.
type openDefinition struct {
	startLine, depth int
}

// FactSink receives the grammar's facts. The grammar names only the subject it
// parsed; identity beyond that belongs to the caller, which closes over it.
type FactSink interface {
	Add(kind, subject, predicate, value string, line, column int) Failure
}

// DefinitionSpanSink is the optional half of FactSink for callers that need a
// definition's extent rather than only its position. A fact carries one line,
// which names where a definition starts and can never name where its suite
// ends; a collector that implements this interface is told both, and one that
// does not is unaffected -- the parser reports spans only when the assertion
// below succeeds.
type DefinitionSpanSink interface {
	DefinitionEnd(startLine, endLine int)
}

// SyntaxCheck rejects a lexed source whose shape the closed grammar treats as
// necessarily invalid. It is supplied by the caller rather than held here so
// this package — and every executable that reaches it for fact extraction
// alone — carries no syntax-validation route (ACP-009).
type SyntaxCheck func(*Parser) bool

// Balanced expressions outside the named fact grammar remain opaque.
// ParsePython312Subset parses the closed Python 3.12 subset, emitting facts to
// the sink. subject names the parsed unit in every fact it produces.
func ParsePython312Subset(subject string, source []byte, collector FactSink) Failure {
	return ParseChecked(subject, source, collector, nil)
}

// ParseChecked is ParsePython312Subset with an additional syntax check applied
// to the lexed source before the fact grammar runs. A nil check parses exactly
// as ParsePython312Subset does.
func ParseChecked(subject string, source []byte, collector FactSink, check SyntaxCheck) Failure {
	tokens, reason := LexPython312(source)
	if reason != "" {
		return reason
	}
	parser := Parser{subject: subject, Source: source, Tokens: tokens, collector: collector, validateSyntax: check != nil}
	if spans, reports := collector.(DefinitionSpanSink); reports {
		parser.spans = spans
	}
	if check != nil && check(&parser) {
		return failureMalformed
	}
	return parser.parse()
}

func LexPython312(source []byte) ([]Token, failure) {
	// Real Python holds above 4 source bytes per token, so one sized allocation
	// replaces the append growth ladder without over-allocating.
	lexer := nativeLexer{Source: source, line: 1, column: 1, indentation: make([]int, 1, 32), altIndentation: make([]int, 1, 32), atLineStart: true, Tokens: make([]Token, 0, len(source)/4+1)}
	for lexer.index < len(source) {
		if lexer.atLineStart {
			if reason := lexer.indent(); reason != "" {
				return nil, reason
			}
		}
		if lexer.index == len(source) {
			break
		}
		if reason := lexer.token(); reason != "" {
			return nil, reason
		}
	}
	if len(lexer.delimiters) != 0 {
		return nil, failureMalformed
	}
	// CPython's tokenizer terminates an unterminated final logical line, so a
	// source without a trailing newline is accepted rather than malformed.
	if len(lexer.Tokens) > 0 && lexer.Tokens[len(lexer.Tokens)-1].Kind != KindNewline {
		lexer.emit(KindNewline, lexer.index, lexer.index, lexer.line, lexer.column)
	}
	for len(lexer.indentation) > 1 {
		lexer.indentation = lexer.indentation[:len(lexer.indentation)-1]
		lexer.emit(KindDedent, lexer.index, lexer.index, lexer.line, lexer.column)
	}
	return lexer.Tokens, ""
}

func (lexer *nativeLexer) indent() failure {
	if len(lexer.delimiters) != 0 || lexer.continued {
		for lexer.index < len(lexer.Source) && nativeWhitespace(lexer.Source[lexer.index]) {
			lexer.advance(lexer.index + 1)
		}
		lexer.atLineStart, lexer.continued = false, false
		return ""
	}
	// altWidth counts every tab as one column. CPython compares indentation
	// at both tab sizes and raises TabError when the two orderings disagree,
	// so a layout that is consistent only at tab size 8 is not Python 3.12.
	start, width, altWidth := lexer.index, 0, 0
	for lexer.index < len(lexer.Source) {
		switch lexer.Source[lexer.index] {
		case ' ':
			width++
			altWidth++
		case '\t':
			width += 8 - width%8
			altWidth++
		case '\f':
			width, altWidth = 0, 0
		default:
			goto indentationDone
		}
		lexer.advance(lexer.index + 1)
	}
indentationDone:
	lexer.atLineStart = false
	if lexer.index == len(lexer.Source) || lexer.Source[lexer.index] == '\n' || lexer.Source[lexer.index] == '#' {
		return ""
	}
	current := lexer.indentation[len(lexer.indentation)-1]
	if width > current {
		if altWidth <= lexer.altIndentation[len(lexer.altIndentation)-1] {
			return failureMalformed
		}
		lexer.indentation = append(lexer.indentation, width)
		lexer.altIndentation = append(lexer.altIndentation, altWidth)
		if len(lexer.indentation)-1 > maxPythonDepth {
			return failureLimit
		}
		lexer.emit(KindIndent, start, lexer.index, lexer.line, 1)
		return ""
	}
	for len(lexer.indentation) > 1 && width < lexer.indentation[len(lexer.indentation)-1] {
		lexer.indentation = lexer.indentation[:len(lexer.indentation)-1]
		lexer.altIndentation = lexer.altIndentation[:len(lexer.altIndentation)-1]
		lexer.emit(KindDedent, start, lexer.index, lexer.line, 1)
	}
	if width != lexer.indentation[len(lexer.indentation)-1] {
		return failureMalformed
	}
	if altWidth != lexer.altIndentation[len(lexer.altIndentation)-1] {
		return failureMalformed
	}
	return ""
}

func (lexer *nativeLexer) token() failure {
	start, line, column := lexer.index, lexer.line, lexer.column
	value := lexer.Source[lexer.index]
	switch value {
	case ' ', '\t', '\f':
		lexer.advance(lexer.index + 1)
		return ""
	case '#':
		for lexer.index < len(lexer.Source) && lexer.Source[lexer.index] != '\n' {
			lexer.advance(lexer.index + 1)
		}
		return ""
	case '\n':
		lexer.advance(lexer.index + 1)
		if len(lexer.delimiters) == 0 {
			lexer.emit(KindNewline, start, start+1, line, column)
		}
		lexer.atLineStart = true
		return ""
	case '\\':
		if lexer.index+1 < len(lexer.Source) && lexer.Source[lexer.index+1] == '\n' {
			lexer.advance(lexer.index + 2)
			lexer.atLineStart, lexer.continued = true, true
			return ""
		}
		return failureMalformed
	}
	if quote, found := StringStart(lexer.Source, lexer.index); found {
		end, reason := lexer.string(start, quote)
		if reason != "" {
			return reason
		}
		lexer.advance(end)
		lexer.emit(KindString, start, end, line, column)
		return ""
	}
	if value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= utf8.RuneSelf {
		for lexer.index < len(lexer.Source) && (pythonWordByte(lexer.Source[lexer.index]) || lexer.Source[lexer.index] >= utf8.RuneSelf) {
			lexer.advance(lexer.index + 1)
		}
		lexer.emit(KindName, start, lexer.index, line, column)
		return ""
	}
	// A float may open with its decimal point: CPython's tokenizer emits `.1`
	// as one NUMBER, and only a digit after the dot distinguishes that from
	// attribute access, an ellipsis byte, or a relative-import dot -- all of
	// which are followed by a name, another dot, or a space. Starting the scan
	// on the dot keeps the existing greedy body, which already consumes the
	// fractional digits and any exponent or imaginary suffix.
	if value >= '0' && value <= '9' || value == '.' && numberFollowsDot(lexer.Source, lexer.index) {
		for lexer.index < len(lexer.Source) && (pythonWordByte(lexer.Source[lexer.index]) || lexer.Source[lexer.index] == '.') {
			lexer.advance(lexer.index + 1)
		}
		lexer.emit(KindNumber, start, lexer.index, line, column)
		return ""
	}
	if strings.ContainsRune("()[]{}:;,.@=+-*/%<>|&^~!", rune(value)) {
		if reason := lexer.delimiter(value); reason != "" {
			return reason
		}
		lexer.advance(lexer.index + 1)
		// ':=' is one token in CPython, so a header colon search never mistakes
		// an unparenthesized walrus for the end of a compound header.
		if value == ':' && lexer.index < len(lexer.Source) && lexer.Source[lexer.index] == '=' {
			lexer.advance(lexer.index + 1)
		}
		lexer.emit(KindSymbol, start, lexer.index, line, column)
		return ""
	}
	return failureMalformed
}

func (lexer *nativeLexer) string(start, quote int) (int, failure) {
	prefix := strings.ToLower(string(lexer.Source[start:quote]))
	if strings.Contains(prefix, "t") {
		return 0, failureUnsupported
	}
	if !nativeStringPrefix(prefix) {
		return 0, failureMalformed
	}
	mark := lexer.Source[quote]
	triple := quote+2 < len(lexer.Source) && lexer.Source[quote+1] == mark && lexer.Source[quote+2] == mark
	index := quote + 1
	if triple {
		index += 2
	}
	for index < len(lexer.Source) {
		if strings.Contains(prefix, "b") && lexer.Source[index] >= utf8.RuneSelf {
			return 0, failureMalformed
		}
		if lexer.Source[index] == '\\' {
			index += 2
			continue
		}
		if triple && index+2 < len(lexer.Source) && lexer.Source[index] == mark && lexer.Source[index+1] == mark && lexer.Source[index+2] == mark {
			return index + 3, ""
		}
		if !triple && lexer.Source[index] == mark {
			return index + 1, ""
		}
		if !triple && lexer.Source[index] == '\n' && !strings.Contains(prefix, "f") {
			return 0, failureMalformed
		}
		index++
	}
	return 0, failureMalformed
}

func nativeStringPrefix(prefix string) bool {
	switch prefix {
	case "", "r", "u", "b", "f", "br", "rb", "fr", "rf":
		return true
	}
	return false
}

func (lexer *nativeLexer) delimiter(value byte) failure {
	switch value {
	case '(', '[', '{':
		lexer.delimiters = append(lexer.delimiters, value)
	case ')', ']', '}':
		if len(lexer.delimiters) == 0 || !DelimiterPair(lexer.delimiters[len(lexer.delimiters)-1], value) {
			return failureMalformed
		}
		lexer.delimiters = lexer.delimiters[:len(lexer.delimiters)-1]
	}
	return ""
}

// DelimiterPair reports whether close is the delimiter matching open.
func DelimiterPair(open, close byte) bool {
	return open == '(' && close == ')' || open == '[' && close == ']' || open == '{' && close == '}'
}

func (lexer *nativeLexer) advance(end int) {
	for lexer.index < end {
		if lexer.Source[lexer.index] == '\n' {
			lexer.line, lexer.column = lexer.line+1, 1
		} else {
			lexer.column++
		}
		lexer.index++
	}
}

func (lexer *nativeLexer) emit(kind byte, start, end, line, column int) {
	lexer.Tokens = append(lexer.Tokens, Token{kind, start, end, line, column})
}

func nativeWhitespace(value byte) bool { return value == ' ' || value == '\t' || value == '\f' }

func (parser *Parser) parse() failure {
	joined := false
	for index := 0; index < len(parser.Tokens); {
		switch parser.Tokens[index].Kind {
		case KindNewline:
			joined = false
			index++
			continue
		case KindIndent:
			if !parser.awaitIndent {
				return failureMalformed
			}
			parser.awaitIndent = false
			parser.suites = append(parser.suites, parser.awaitMatch)
			index++
			continue
		case KindDedent:
			// A decorator binds only the definition that follows it in the same
			// suite; CPython rejects a dedent between the two.
			if parser.awaitIndent || len(parser.suites) == 0 || len(parser.decorators) != 0 {
				return failureMalformed
			}
			parser.suites = parser.suites[:len(parser.suites)-1]
			parser.closeDefinitions()
			index++
			continue
		}
		if parser.awaitIndent {
			return failureMalformed
		}
		end := parser.StatementEnd(index)
		statement := parser.Tokens[index:end]
		if len(statement) == 0 {
			// An empty statement comes from a leading or repeated ';'. CPython
			// rejects ';', ';;', 'x = 1;;' and ';x = 1' while accepting a single
			// trailing 'x = 1;', so an empty span is malformed rather than skippable.
			return failureMalformed
		}
		caseBlock := parser.blockStatement(statement, "case")
		if parser.inMatchSuite() != caseBlock {
			return failureMalformed
		}
		// CPython admits only simple statements after ';'. Accepting a
		// definition, decorator, or compound header there emitted facts for
		// source Python rejects, and gave two definitions one start line.
		if joined && parser.compoundStatement(statement) {
			return failureMalformed
		}
		if reason := parser.statement(statement); reason != "" {
			return reason
		}
		parser.awaitIndent = parser.nonInlineSuite(statement)
		parser.awaitMatch = parser.awaitIndent && parser.blockStatement(statement, "match")
		parser.contentEndLine = parser.statementEndLine(statement)
		parser.trackDefinition()
		index = end
		joined = index < len(parser.Tokens) && parser.Symbol(parser.Tokens[index], ";")
		if joined {
			index++
		}
	}
	if parser.awaitIndent || len(parser.suites) != 0 || len(parser.decorators) != 0 {
		return failureMalformed
	}
	return ""
}

func (parser *Parser) inMatchSuite() bool {
	return len(parser.suites) != 0 && parser.suites[len(parser.suites)-1]
}

func (parser *Parser) blockStatement(tokens []Token, word string) bool {
	if len(tokens) != 0 && tokens[len(tokens)-1].Kind == KindNewline {
		tokens = tokens[:len(tokens)-1]
	}
	if len(tokens) < 3 || !parser.Name(tokens[0], word) {
		return false
	}
	return parser.HeaderColon(tokens, 1) == len(tokens)-1
}

// compoundStatement reports whether a statement opens with a decorator,
// definition, or hard-keyword compound header. The soft keywords `match` and
// `case` stay out: `x = 1; match: int = 2` is a valid annotated assignment.
func (parser *Parser) compoundStatement(tokens []Token) bool {
	if parser.Symbol(tokens[0], "@") || parser.DefinitionStart(tokens) {
		return true
	}
	return parser.compound(tokens) && !parser.Name(tokens[0], "match") && !parser.Name(tokens[0], "case")
}

// trackDefinition records the extent of the definition the statement just
// parsed, if it was one. A block suite stays open until the dedent that closes
// it; an inline suite ("def f(): pass") is complete within its own statement,
// whose last content line contentEndLine already holds.
func (parser *Parser) trackDefinition() {
	startLine := parser.definitionLine
	parser.definitionLine = 0
	if parser.spans == nil || startLine == 0 {
		return
	}
	if !parser.awaitIndent {
		parser.spans.DefinitionEnd(startLine, parser.contentEndLine)
		return
	}
	parser.definitions = append(parser.definitions, openDefinition{startLine, len(parser.suites)})
}

// closeDefinitions reports every definition the dedent just left. A nested
// definition sits at a deeper suite depth than the one enclosing it, so the
// stack unwinds innermost first and one dedent can close several.
func (parser *Parser) closeDefinitions() {
	for len(parser.definitions) != 0 {
		open := parser.definitions[len(parser.definitions)-1]
		if open.depth < len(parser.suites) {
			return
		}
		parser.definitions = parser.definitions[:len(parser.definitions)-1]
		parser.spans.DefinitionEnd(open.startLine, parser.contentEndLine)
	}
}

// statementEndLine is the line the statement's last content token ends on.
// Blank and comment lines between two statements carry no token, so a suite's
// last content line is where its last statement ends rather than where the
// dedent that closes it was lexed.
func (parser *Parser) statementEndLine(statement []Token) int {
	for index := len(statement) - 1; index >= 0; index-- {
		if statement[index].Kind == KindNewline {
			continue
		}
		return parser.tokenEndLine(statement[index])
	}
	return parser.contentEndLine
}

// tokenEndLine is the line the token's last byte sits on. A triple-quoted
// string spans lines and a token records only where it started, so the two are
// not the same line for every token.
func (parser *Parser) tokenEndLine(token Token) int {
	return token.line + bytes.Count(parser.Source[token.Start:token.End], []byte{'\n'})
}

func (parser *Parser) nonInlineSuite(tokens []Token) bool {
	if len(tokens) > 0 && tokens[len(tokens)-1].Kind == KindNewline {
		tokens = tokens[:len(tokens)-1]
	}
	if !parser.DefinitionStart(tokens) && !parser.compound(tokens) {
		return false
	}
	for index := len(tokens) - 1; index >= 0; index-- {
		if parser.Symbol(tokens[index], ":") && parser.TopLevel(tokens, index) {
			return index == len(tokens)-1
		}
	}
	return false
}

func (parser *Parser) StatementEnd(start int) int {
	depth := 0
	for index := start; index < len(parser.Tokens); index++ {
		token := parser.Tokens[index]
		if token.Kind == KindNewline && depth == 0 {
			return index + 1
		}
		if token.Kind != KindSymbol {
			continue
		}
		switch parser.Text(token) {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ";":
			if depth == 0 {
				return index
			}
		}
	}
	return len(parser.Tokens)
}

func (parser *Parser) statement(tokens []Token) failure {
	if len(tokens) == 0 {
		return ""
	}
	if tokens[len(tokens)-1].Kind == KindNewline {
		tokens = tokens[:len(tokens)-1]
	}
	if len(tokens) == 0 {
		return ""
	}
	if parser.Symbol(tokens[0], "@") {
		return parser.decorator(tokens)
	}
	if len(parser.decorators) != 0 && !parser.DefinitionStart(tokens) {
		return failureMalformed
	}
	switch {
	case parser.Name(tokens[0], "import"):
		return parser.imports(tokens)
	case parser.Name(tokens[0], "from"):
		return parser.fromImport(tokens)
	case parser.DefinitionStart(tokens):
		return parser.definition(tokens)
	case parser.TypeAliasStart(tokens):
		return parser.typeAlias(tokens)
	case parser.compound(tokens):
		return parser.inlineSuite(tokens)
	default:
		return parser.directCall(tokens)
	}
}

func (parser *Parser) decorator(tokens []Token) failure {
	if len(tokens) == 1 {
		return failureMalformed
	}
	name, next, static, reason := parser.StaticCallee(tokens[1:])
	if reason != "" {
		return reason
	}
	if !static {
		return failureDynamic
	}
	rest := tokens[1:]
	if next == len(rest) {
		parser.decorators = append(parser.decorators, nativeDecorator{name, rest[0].line, rest[0].column})
		return ""
	}
	if !parser.Symbol(rest[next], "(") || parser.Matching(rest, next) != len(rest)-1 {
		return failureDynamic
	}
	if reason := parser.arguments(rest[next+1 : len(rest)-1]); reason != "" {
		return reason
	}
	parser.decorators = append(parser.decorators, nativeDecorator{name, rest[0].line, rest[0].column})
	return ""
}

func (parser *Parser) imports(tokens []Token) failure {
	for index := 1; index < len(tokens); {
		module, next, reason := parser.dotted(tokens, index, false)
		if reason != "" || module == "" {
			if reason != "" {
				return reason
			}
			return failureMalformed
		}
		if reason = staticModule(module); reason != "" {
			return reason
		}
		if next < len(tokens) && parser.Name(tokens[next], "as") {
			next++
			if next == len(tokens) || tokens[next].Kind != KindName {
				return failureMalformed
			}
			if reason = SyntaxName(parser.Text(tokens[next])); reason != "" {
				return reason
			}
			next++
		}
		if reason = parser.collector.Add("python.import.static", parser.subject, "imports", module, tokens[0].line, tokens[0].column); reason != "" {
			return reason
		}
		if next == len(tokens) {
			return ""
		}
		if !parser.Symbol(tokens[next], ",") || next+1 == len(tokens) {
			return failureMalformed
		}
		index = next + 1
	}
	return failureMalformed
}

func (parser *Parser) fromImport(tokens []Token) failure {
	marker := -1
	for index := 1; index < len(tokens); index++ {
		if parser.Name(tokens[index], "import") {
			marker = index
			break
		}
	}
	if marker < 2 || marker == len(tokens)-1 {
		return failureMalformed
	}
	module, next, reason := parser.dotted(tokens[1:marker], 0, true)
	if reason != "" {
		return reason
	}
	if next != marker-1 || module == "" {
		return failureMalformed
	}
	if reason = staticModule(module); reason != "" {
		return reason
	}
	index, parenthesized := marker+1, false
	if index < len(tokens) && parser.Symbol(tokens[index], "(") {
		index, parenthesized = index+1, true
	}
	for index < len(tokens) {
		if parenthesized && parser.Symbol(tokens[index], ")") {
			if index != len(tokens)-1 || index == marker+2 {
				return failureMalformed
			}
			return parser.collector.Add("python.import.static", parser.subject, "imports", module, tokens[0].line, tokens[0].column)
		}
		if parser.Symbol(tokens[index], "*") {
			// `*` is only ever the sole, unparenthesized, unaliased target.
			if parenthesized || index != marker+1 || index != len(tokens)-1 {
				return failureMalformed
			}
			return parser.collector.Add("python.import.static", parser.subject, "imports", module, tokens[0].line, tokens[0].column)
		} else {
			if tokens[index].Kind != KindName {
				return failureMalformed
			}
			if reason = SyntaxName(parser.Text(tokens[index])); reason != "" {
				return reason
			}
			index++
		}
		if index < len(tokens) && parser.Name(tokens[index], "as") {
			index++
			if index == len(tokens) || tokens[index].Kind != KindName {
				return failureMalformed
			}
			if reason = SyntaxName(parser.Text(tokens[index])); reason != "" {
				return reason
			}
			index++
		}
		if index == len(tokens) {
			if parenthesized {
				return failureMalformed
			}
			return parser.collector.Add("python.import.static", parser.subject, "imports", module, tokens[0].line, tokens[0].column)
		}
		// The closing parenthesis may follow the last name directly, as in
		// `from a import (b, c)`; the loop head closes the list.
		if parenthesized && parser.Symbol(tokens[index], ")") {
			continue
		}
		if !parser.Symbol(tokens[index], ",") {
			return failureMalformed
		}
		index++
	}
	return failureMalformed
}

func (parser *Parser) DefinitionStart(tokens []Token) bool {
	if len(tokens) == 0 {
		return false
	}
	return parser.Name(tokens[0], "def") || parser.Name(tokens[0], "class") || len(tokens) > 1 && parser.Name(tokens[0], "async") && parser.Name(tokens[1], "def")
}

func (parser *Parser) definition(tokens []Token) failure {
	nameIndex, kind := 1, "function"
	if parser.Name(tokens[0], "async") {
		nameIndex, kind = 2, "async-function"
	}
	if parser.Name(tokens[0], "class") {
		kind = "class"
	}
	if nameIndex == len(tokens) || tokens[nameIndex].Kind != KindName {
		return failureMalformed
	}
	name := parser.Text(tokens[nameIndex])
	if reason := SyntaxName(name); reason != "" {
		return reason
	}
	if parser.typeParameterDefault(tokens[nameIndex+1:]) {
		return failureUnsupported
	}
	if reason := parser.definitionHeader(kind, tokens[nameIndex+1:]); reason != "" {
		return reason
	}
	if reason := parser.collector.Add("python.definition", parser.subject, "defines", kind+":"+name, tokens[0].line, tokens[0].column); reason != "" {
		return reason
	}
	parser.definitionLine = tokens[0].line
	for _, decorator := range parser.decorators {
		if reason := parser.collector.Add("python.decorator", parser.subject, "decorates", decorator.name+"->"+kind+":"+name, decorator.line, decorator.column); reason != "" {
			return reason
		}
	}
	parser.decorators = nil
	return parser.inlineSuite(tokens)
}

func (parser *Parser) definitionHeader(kind string, tokens []Token) failure {
	index := 0
	if index < len(tokens) && parser.Symbol(tokens[index], "[") {
		end := parser.Matching(tokens, index)
		if end < 0 {
			return failureMalformed
		}
		if reason := parser.typeParameters(tokens[index+1 : end]); reason != "" {
			return reason
		}
		index = end + 1
	}
	if kind == "class" {
		if index < len(tokens) && parser.Symbol(tokens[index], "(") {
			end := parser.Matching(tokens, index)
			if end < 0 {
				return failureMalformed
			}
			if reason := parser.arguments(tokens[index+1 : end]); reason != "" {
				return reason
			}
			index = end + 1
		}
		if index == len(tokens) || !parser.Symbol(tokens[index], ":") {
			return failureMalformed
		}
		return ""
	}
	if index == len(tokens) || !parser.Symbol(tokens[index], "(") {
		return failureMalformed
	}
	end := parser.Matching(tokens, index)
	if end < 0 {
		return failureMalformed
	}
	if reason := parser.parameters(tokens[index+1 : end]); reason != "" {
		return reason
	}
	index = end + 1
	if parser.arrow(tokens, index) {
		index += 2
		colon := parser.HeaderColon(tokens, index)
		if colon == index {
			return failureMalformed
		}
		if colon < 0 {
			return failureMalformed
		}
		if reason := parser.expression(tokens[index:colon]); reason != "" {
			return reason
		}
		return ""
	}
	if index == len(tokens) || !parser.Symbol(tokens[index], ":") {
		return failureMalformed
	}
	return ""
}

func (parser *Parser) typeParameters(tokens []Token) failure {
	parts, reason := parser.CommaSeparated(tokens)
	if reason != "" || len(parts) == 0 {
		return failureMalformed
	}
	for _, part := range parts {
		index := 0
		for index < len(part) && parser.Symbol(part[index], "*") {
			index++
			if index > 2 {
				return failureMalformed
			}
		}
		if index == len(part) || part[index].Kind != KindName {
			return failureMalformed
		}
		if reason := SyntaxName(parser.Text(part[index])); reason != "" {
			return reason
		}
		index++
		if parser.HasTopLevel(part[index:], "=") {
			return failureUnsupported
		}
		if index == len(part) {
			continue
		}
		if !parser.Symbol(part[index], ":") {
			return failureMalformed
		}
		if reason := parser.expression(part[index+1:]); reason != "" {
			return reason
		}
	}
	return ""
}

func (parser *Parser) parameters(tokens []Token) failure {
	parts, reason := parser.CommaSeparated(tokens)
	if reason != "" {
		return reason
	}
	positional, positionalDefault := 0, false
	slash, star, bareStar, keywordOnly, keywordArguments := false, false, false, 0, false
	for index, part := range parts {
		kind, defaulted, reason := parser.parameter(part)
		if reason != "" {
			return reason
		}
		switch kind {
		case '/':
			if slash || star || positional == 0 {
				return failureMalformed
			}
			slash = true
		case '*':
			if star || keywordArguments {
				return failureMalformed
			}
			star, bareStar = true, true
		case 'v':
			if star || keywordArguments || defaulted {
				return failureMalformed
			}
			star = true
		case 'k':
			if keywordArguments || defaulted || index != len(parts)-1 {
				return failureMalformed
			}
			star, keywordArguments = true, true
		case 'p':
			if keywordArguments {
				return failureMalformed
			}
			if star {
				keywordOnly++
				continue
			}
			positional++
			if positionalDefault && !defaulted {
				return failureMalformed
			}
			positionalDefault = positionalDefault || defaulted
		}
	}
	if bareStar && keywordOnly == 0 {
		return failureMalformed
	}
	return ""
}

func (parser *Parser) parameter(tokens []Token) (byte, bool, failure) {
	if len(tokens) == 1 && parser.Symbol(tokens[0], "/") {
		return '/', false, ""
	}
	stars := 0
	for stars < len(tokens) && parser.Symbol(tokens[stars], "*") {
		stars++
	}
	if stars > 2 {
		return 0, false, failureMalformed
	}
	if stars == 1 && len(tokens) == 1 {
		return '*', false, ""
	}
	if stars == 2 && len(tokens) == 2 {
		return 0, false, failureMalformed
	}
	defaulted, reason := parser.namedParameter(tokens[stars:], stars == 0)
	if reason != "" {
		return 0, false, reason
	}
	if stars == 2 {
		return 'k', defaulted, ""
	}
	if stars == 1 {
		return 'v', defaulted, ""
	}
	return 'p', defaulted, ""
}

func (parser *Parser) namedParameter(tokens []Token, allowDefault bool) (bool, failure) {
	if len(tokens) == 0 || tokens[0].Kind != KindName {
		return false, failureMalformed
	}
	if reason := SyntaxName(parser.Text(tokens[0])); reason != "" {
		return false, reason
	}
	index, annotation, defaultAt := 1, -1, -1
	for index < len(tokens) {
		if defaultAt >= 0 || !parser.TopLevel(tokens, index) {
			index++
			continue
		}
		if parser.Symbol(tokens[index], ":") {
			if annotation >= 0 {
				return false, failureMalformed
			}
			annotation = index
		}
		if parser.Symbol(tokens[index], "=") && !parser.Comparison(tokens, index) {
			defaultAt = index
		}
		index++
	}
	if annotation < 0 && defaultAt < 0 {
		if len(tokens) == 1 {
			return false, ""
		}
		return false, failureMalformed
	}
	if annotation >= 0 {
		end := len(tokens)
		if defaultAt >= 0 {
			end = defaultAt
		}
		if reason := parser.expression(tokens[annotation+1 : end]); reason != "" {
			return false, reason
		}
	} else if defaultAt != 1 {
		return false, failureMalformed
	}
	if defaultAt < 0 {
		return false, ""
	}
	if !allowDefault {
		return false, failureMalformed
	}
	if reason := parser.expression(tokens[defaultAt+1:]); reason != "" {
		return false, reason
	}
	return true, ""
}

func (parser *Parser) arguments(tokens []Token) failure {
	parts, reason := parser.CommaSeparated(tokens)
	if reason != "" {
		return reason
	}
	sawKeyword, sawKeywordUnpack := false, false
	for _, part := range parts {
		stars := 0
		for stars < len(part) && stars < 2 && parser.Symbol(part[stars], "*") {
			stars++
		}
		keywordArgument := len(part) > 2 && part[0].Kind == KindName && parser.Symbol(part[1], "=") && !parser.Comparison(part, 1)
		switch {
		case stars == 2:
			sawKeywordUnpack = true
		case stars == 1:
			if sawKeywordUnpack {
				return failureMalformed
			}
		case keywordArgument:
			sawKeyword = true
		case sawKeyword || sawKeywordUnpack:
			return failureMalformed
		}
		if reason := parser.expression(part); reason != "" {
			return reason
		}
	}
	return ""
}

func (parser *Parser) CommaSeparated(tokens []Token) ([][]Token, failure) {
	if len(tokens) == 0 {
		return nil, ""
	}
	parts, start := make([][]Token, 0, 4), 0
	for index := range tokens {
		if !parser.Symbol(tokens[index], ",") || !parser.TopLevel(tokens, index) {
			continue
		}
		if index == start {
			return nil, failureMalformed
		}
		parts = append(parts, tokens[start:index])
		start = index + 1
	}
	if start < len(tokens) {
		parts = append(parts, tokens[start:])
	}
	return parts, ""
}

func (parser *Parser) expression(tokens []Token) failure {
	if len(tokens) == 0 {
		return failureMalformed
	}
	expectOperand, lastString, prevKeyword := true, false, ""
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if token.Kind == KindNewline || token.Kind == KindIndent || token.Kind == KindDedent {
			continue
		}
		if prevKeyword == "not~" && !parser.tokenIs(token, KindName, "in") {
			return failureMalformed
		}
		if !expectOperand && token.Kind == KindString && lastString {
			continue
		}
		lastString = expectOperand && token.Kind == KindString
		if parser.ExpressionBracket(token) {
			if !expectOperand && parser.Source[token.Start] == '{' {
				return failureMalformed
			}
			end := parser.Matching(tokens, index)
			if end < 0 {
				return failureMalformed
			}
			if !expectOperand && parser.Source[token.Start] == '[' && end == index+1 {
				return failureMalformed
			}
			if reason := parser.bracketInterior(colonBudget(parser.Source[token.Start], expectOperand), tokens[index+1:end]); reason != "" {
				return reason
			}
			expectOperand, prevKeyword = false, ""
			index = end
			continue
		}
		if expectOperand && parser.tokenIs(token, KindName, "lambda") {
			colon := parser.SpanColon(tokens, index+1)
			if colon < 0 {
				return failureMalformed
			}
			// A lambda's parameter list is CPython's `lambda_params`, which differs
			// from `params` only in carrying no annotation -- and an annotation
			// colon is unreachable, SpanColon having ended the span at the first
			// top-level colon. Defaults, `*args`, `**kwargs`, `/` and a bare `*`
			// are therefore admitted here exactly as a def admits them.
			if reason := parser.parameters(tokens[index+1 : colon]); reason != "" {
				return reason
			}
			index = colon
			prevKeyword = ""
			continue
		}
		if parser.KeywordOperator(token) {
			word := string(parser.Source[token.Start:token.End])
			switch {
			case word == "in" && prevKeyword == "not~":
				expectOperand, prevKeyword = true, "cmp"
			case word == "not" && prevKeyword == "is":
				expectOperand, prevKeyword = true, "cmp"
			case word == "not" && !expectOperand:
				expectOperand, prevKeyword = true, "not~"
			case expectOperand && !leadingKeywordAllowed(word, prevKeyword):
				return failureMalformed
			case word == "in":
				expectOperand, prevKeyword = true, "cmp"
			default:
				expectOperand, prevKeyword = true, word
			}
			continue
		}
		prevKeyword = ""
		if expectOperand && parser.EllipsisAt(tokens, index) {
			expectOperand = false
			index += 2
			continue
		}
		if expectOperand && parser.unaryPrefix(token) {
			continue
		}
		if expectOperand && parser.Operand(token) {
			expectOperand = false
			continue
		}
		if expectOperand || !parser.BinaryConnector(token) {
			return failureMalformed
		}
		for index+1 < len(tokens) && parser.BinaryConnector(tokens[index+1]) && tokens[index].End == tokens[index+1].Start {
			index++
		}
		expectOperand = true
	}
	if expectOperand {
		return failureMalformed
	}
	return ""
}

func (parser *Parser) ExpressionBracket(token Token) bool {
	if token.Kind != KindSymbol || token.End != token.Start+1 {
		return false
	}
	value := parser.Source[token.Start]
	return value == '(' || value == '[' || value == '{'
}

func (parser *Parser) Operand(token Token) bool {
	if token.Kind == KindNumber || token.Kind == KindString {
		return true
	}
	if token.Kind != KindName {
		return false
	}
	value := parser.Source[token.Start:token.End]
	if _, keyword := pythonKeywords[string(value)]; !keyword {
		return true
	}
	return string(value) == "True" || string(value) == "False" || string(value) == "None"
}

func (parser *Parser) unaryPrefix(token Token) bool {
	if token.Kind != KindSymbol || token.End != token.Start+1 {
		return false
	}
	value := parser.Source[token.Start]
	return value == '+' || value == '-' || value == '~' || value == '*'
}

func (parser *Parser) KeywordOperator(token Token) bool {
	if token.Kind != KindName {
		return false
	}
	switch string(parser.Source[token.Start:token.End]) {
	case "and", "or", "not", "in", "is", "if", "else", "for", "async":
		return true
	}
	return false
}

func (parser *Parser) BinaryConnector(token Token) bool {
	if token.Kind != KindSymbol || token.End > token.Start+2 {
		return false
	}
	return token.End == token.Start+2 || strings.IndexByte("+-*/%@&|^<>=!.", parser.Source[token.Start]) >= 0
}

func (parser *Parser) EllipsisAt(tokens []Token, index int) bool {
	if index+2 >= len(tokens) {
		return false
	}
	joined := parser.SymbolByte(tokens[index], '.') && parser.SymbolByte(tokens[index+1], '.') && parser.SymbolByte(tokens[index+2], '.')
	return joined && tokens[index].End == tokens[index+1].Start && tokens[index+1].End == tokens[index+2].Start
}

func (parser *Parser) SpanColon(tokens []Token, start int) int {
	for index := start; index < len(tokens); index++ {
		if parser.Symbol(tokens[index], ":") && parser.TopLevel(tokens, index) {
			return index
		}
	}
	return -1
}

// leadingKeywordAllowed gates keyword operators in operand position. Compound
// comparisons (`not in`, `is not`) are handled before this check; a completed
// comparison admits no further keyword operator.
func leadingKeywordAllowed(word, previous string) bool {
	if word == "not" {
		return previous != "cmp" && previous != "is"
	}
	return word == "for" && previous == "async"
}

func colonBudget(opener byte, display bool) int {
	if opener == '{' {
		return 1
	}
	if opener == '[' && !display {
		return 2
	}
	return 0
}

func (parser *Parser) bracketInterior(budget int, tokens []Token) failure {
	start, depth, lambdaSeen, colons := 0, 0, false, 0
	itemStart, itemClass, lastColon := 0, 0, false
	comprehension := parser.comprehensionInterior(tokens)
	for index := 0; index <= len(tokens); index++ {
		if index < len(tokens) && tokens[index].Kind == KindSymbol && tokens[index].End == tokens[index].Start+1 {
			switch parser.Source[tokens[index].Start] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
		}
		if index < len(tokens) && depth == 0 && parser.tokenIs(tokens[index], KindName, "lambda") {
			lambdaSeen = true
		}
		if index < len(tokens) && depth == 0 && lambdaSeen && parser.SymbolByte(tokens[index], ':') {
			lambdaSeen = false
			continue
		}
		comma := index < len(tokens) && depth == 0 && !lambdaSeen && parser.SymbolByte(tokens[index], ',')
		colon := index < len(tokens) && depth == 0 && !lambdaSeen && parser.SymbolByte(tokens[index], ':')
		if colon {
			colons++
		}
		if colon && colons > budget {
			return failureMalformed
		}
		if index < len(tokens) && !comma && !colon {
			continue
		}
		part := tokens[start:index]
		if len(part) == 0 && comma {
			return failureMalformed
		}
		if len(part) == 0 && colon && budget != 2 {
			return failureMalformed
		}
		if len(part) == 0 && index == len(tokens) && lastColon && budget != 2 {
			return failureMalformed
		}
		if len(part) > 0 {
			if reason := parser.expression(part); reason != "" {
				return reason
			}
		}
		if comma || index == len(tokens) {
			if budget == 1 && !comprehension && index > itemStart {
				class := parser.braceItemClass(tokens[itemStart:index], colons)
				if itemClass != 0 && class != itemClass {
					return failureMalformed
				}
				itemClass = class
			}
			colons, itemStart = 0, index+1
		}
		lastColon = colon
		start = index + 1
	}
	return ""
}

func (parser *Parser) comprehensionInterior(tokens []Token) bool {
	depth := 0
	for _, token := range tokens {
		if token.Kind == KindSymbol && token.End == token.Start+1 {
			switch parser.Source[token.Start] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
		}
		if depth == 0 && parser.tokenIs(token, KindName, "for") {
			return true
		}
	}
	return false
}

func (parser *Parser) braceItemClass(item []Token, colons int) int {
	if colons > 0 {
		return 1
	}
	stars := 0
	for _, token := range item {
		if token.Kind == KindNewline || token.Kind == KindIndent || token.Kind == KindDedent {
			continue
		}
		if parser.SymbolByte(token, '*') && stars < 2 {
			stars++
			continue
		}
		break
	}
	if stars == 2 {
		return 1
	}
	return 2
}

func (parser *Parser) tokenIs(token Token, kind byte, want string) bool {
	return token.Kind == kind && string(parser.Source[token.Start:token.End]) == want
}

func (parser *Parser) SymbolByte(token Token, want byte) bool {
	return token.Kind == KindSymbol && token.End == token.Start+1 && parser.Source[token.Start] == want
}

func (parser *Parser) arrow(tokens []Token, index int) bool {
	return index+1 < len(tokens) && parser.Symbol(tokens[index], "-") && parser.Symbol(tokens[index+1], ">") && tokens[index].End == tokens[index+1].Start
}

func (parser *Parser) Comparison(tokens []Token, index int) bool {
	if index > 0 && parser.Symbol(tokens[index-1], "=") && tokens[index-1].End == tokens[index].Start {
		return true
	}
	if index+1 < len(tokens) && parser.Symbol(tokens[index+1], "=") && tokens[index].End == tokens[index+1].Start {
		return true
	}
	if index > 0 && (parser.Symbol(tokens[index-1], "!") || parser.Symbol(tokens[index-1], "<") || parser.Symbol(tokens[index-1], ">") || parser.Symbol(tokens[index-1], ":")) && tokens[index-1].End == tokens[index].Start {
		return true
	}
	return false
}

func (parser *Parser) HeaderColon(tokens []Token, start int) int {
	for index := start; index < len(tokens); index++ {
		if parser.Symbol(tokens[index], ":") && parser.TopLevel(tokens[start:], index-start) {
			return index
		}
	}
	return -1
}

// TypeAliasStart reports whether a statement opening with the soft keyword
// `type` is a PEP 695 alias rather than an ordinary use of the builtin. CPython
// commits to `type_alias: "type" NAME [type_params] '=' expression` only on that
// shape, so `type(x)`, `type = 5` and `type(self).calls += 1` stay expressions.
func (parser *Parser) TypeAliasStart(tokens []Token) bool {
	if len(tokens) < 4 || !parser.Name(tokens[0], "type") || tokens[1].Kind != KindName {
		return false
	}
	assign := 2
	if parser.Symbol(tokens[assign], "[") {
		end := parser.Matching(tokens, assign)
		if end < 0 {
			return false
		}
		assign = end + 1
	}
	return assign < len(tokens) && parser.Symbol(tokens[assign], "=") && !parser.Comparison(tokens, assign)
}

func (parser *Parser) typeAlias(tokens []Token) failure {
	if len(tokens) < 4 || tokens[1].Kind != KindName {
		return failureMalformed
	}
	if reason := SyntaxName(parser.Text(tokens[1])); reason != "" {
		return reason
	}
	if parser.typeParameterDefault(tokens[2:]) {
		return failureUnsupported
	}
	if !parser.HasTopLevel(tokens[2:], "=") {
		return failureMalformed
	}
	return ""
}

func (parser *Parser) compound(tokens []Token) bool {
	if tokens[0].Kind != KindName || !parser.HasTopLevel(tokens[1:], ":") {
		return false
	}
	switch parser.Text(tokens[0]) {
	case "if", "elif", "else", "for", "while", "try", "except", "finally", "with", "match", "case":
		return true
	case "async":
		return len(tokens) > 1 && (parser.Name(tokens[1], "for") || parser.Name(tokens[1], "with"))
	default:
		return false
	}
}

func (parser *Parser) inlineSuite(tokens []Token) failure {
	if colon := parser.HeaderColon(tokens, 0); colon >= 0 && colon+1 < len(tokens) {
		return parser.directCall(tokens[colon+1:])
	}
	return ""
}

func (parser *Parser) directCall(tokens []Token) failure {
	if parser.validateSyntax && parser.DanglingUnary(tokens) {
		return failureMalformed
	}
	if len(tokens) != 0 && tokens[0].Kind == KindName && PythonKeyword(parser.Text(tokens[0])) {
		// directCall also parses an inline suite's body (the tokens after a
		// compound header's same-line colon), where CPython admits only a
		// simple statement -- the same restriction compoundStatement already
		// enforces after ';'. Accepting a nested definition or compound
		// header there emitted no fact but also no rejection, silently
		// succeeding on source Python does not parse.
		if parser.compoundStatement(tokens) {
			return failureMalformed
		}
		return ""
	}
	name, next, static, reason := parser.StaticCallee(tokens)
	if reason != "" || !static || next == len(tokens) || !parser.Symbol(tokens[next], "(") {
		return reason
	}
	if parser.Matching(tokens, next) != len(tokens)-1 {
		return ""
	}
	if reason := parser.bracketInterior(colonBudget('(', false), tokens[next+1:len(tokens)-1]); reason != "" {
		return reason
	}
	return parser.collector.Add("python.call.static", parser.subject, "calls", name, tokens[0].line, tokens[0].column)
}

// lambdaColon returns the index of the colon ending the parameter list of the
// lambda at start, or -1. It matches at the lambda's own bracket depth, so it
// finds that colon in a span the lambda does not sit at top level in.
func (parser *Parser) lambdaColon(tokens []Token, start int) int {
	depth := 0
	for index := start + 1; index < len(tokens); index++ {
		if tokens[index].Kind != KindSymbol {
			continue
		}
		switch parser.Text(tokens[index]) {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ":":
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func (parser *Parser) DanglingUnary(tokens []Token) bool {
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		// A lambda's parameter list is not an expression: its keyword-only `*`
		// marker wears exactly the shape this rule refuses, so judging it here
		// rejected `f(lambda a, *, b: a)`, which CPython accepts.
		if parser.Name(token, "lambda") {
			if colon := parser.lambdaColon(tokens, index); colon >= 0 {
				index = colon
			}
			continue
		}
		if !parser.unaryPrefix(token) {
			continue
		}
		if index+1 == len(tokens) {
			return true
		}
		next := tokens[index+1]
		if next.Kind == KindSymbol && strings.IndexByte(",)]}", parser.Source[next.Start]) >= 0 {
			return true
		}
	}
	return false
}

func (parser *Parser) StaticCallee(tokens []Token) (string, int, bool, failure) {
	if len(tokens) == 0 || tokens[0].Kind != KindName {
		return "", 0, false, ""
	}
	name := parser.Text(tokens[0])
	if !pythonName(name) {
		return "", 0, true, failureDynamic
	}
	index := 1
	for index+1 < len(tokens) && parser.Symbol(tokens[index], ".") && tokens[index+1].Kind == KindName {
		part := parser.Text(tokens[index+1])
		if !pythonName(part) {
			return "", 0, true, failureDynamic
		}
		name += "." + part
		index += 2
	}
	return name, index, true, ""
}

func (parser *Parser) dotted(tokens []Token, start int, relative bool) (string, int, failure) {
	index, module := start, ""
	if relative {
		for index < len(tokens) && parser.Symbol(tokens[index], ".") {
			module += "."
			index++
		}
	}
	if index == len(tokens) || tokens[index].Kind != KindName {
		return module, index, ""
	}
	for {
		part := parser.Text(tokens[index])
		if reason := SyntaxName(part); reason != "" {
			return "", index, reason
		}
		if module != "" && !strings.HasSuffix(module, ".") {
			module += "."
		}
		module += part
		index++
		if index == len(tokens) || !parser.Symbol(tokens[index], ".") {
			return module, index, ""
		}
		if index+1 == len(tokens) || tokens[index+1].Kind != KindName {
			return "", index, failureMalformed
		}
		module += "."
		index++
	}
}

func (parser *Parser) Matching(tokens []Token, open int) int {
	depth := 0
	for index := open; index < len(tokens); index++ {
		if tokens[index].Kind != KindSymbol {
			continue
		}
		switch parser.Text(tokens[index]) {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func (parser *Parser) HasTopLevel(tokens []Token, want string) bool {
	for index := range tokens {
		if parser.Symbol(tokens[index], want) && parser.TopLevel(tokens, index) {
			return true
		}
	}
	return false
}

func (parser *Parser) TopLevel(tokens []Token, target int) bool {
	depth := 0
	for index := 0; index < target; index++ {
		if tokens[index].Kind != KindSymbol {
			continue
		}
		switch parser.Text(tokens[index]) {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
	}
	return depth == 0
}

func (parser *Parser) typeParameterDefault(tokens []Token) bool {
	if len(tokens) == 0 || !parser.Symbol(tokens[0], "[") {
		return false
	}
	end := parser.Matching(tokens, 0)
	return end > 0 && parser.HasTopLevel(tokens[1:end], "=")
}

func (parser *Parser) Text(token Token) string {
	return string(parser.Source[token.Start:token.End])
}
func (parser *Parser) Name(token Token, want string) bool {
	return token.Kind == KindName && parser.Text(token) == want
}
func (parser *Parser) Symbol(token Token, want string) bool {
	return token.Kind == KindSymbol && parser.Text(token) == want
}
