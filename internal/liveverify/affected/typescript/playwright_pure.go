package typescript

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// playwrightImpureKeywords start expressions or statements that can run code, write state or
// suspend evaluation, or are not values at all.
var playwrightImpureKeywords = map[string]bool{
	"async": true, "await": true, "break": true, "case": true, "catch": true, "class": true, "const": true,
	"continue": true, "debugger": true, "default": true, "delete": true, "do": true, "else": true,
	"export": true, "extends": true, "finally": true, "for": true, "function": true, "if": true,
	"import": true, "in": true, "instanceof": true, "let": true, "new": true, "return": true,
	"super": true, "switch": true, "throw": true, "try": true, "typeof": true, "var": true, "void": true,
	"while": true, "with": true, "yield": true,
}

// playwrightBinaryOperator is one admitted binary operator. A coercing operator can convert an
// object operand through its toString, valueOf, Symbol.toPrimitive or Symbol.hasInstance, so it
// admits only operands proven primitive; the others never call user code.
type playwrightBinaryOperator struct {
	token      string
	precedence int
	coercing   bool
	word       bool
}

// playwrightBinaryOperators are matched longest first.
var playwrightBinaryOperators = []playwrightBinaryOperator{
	{token: "instanceof", precedence: 7, coercing: true, word: true},
	{token: ">>>", precedence: 8, coercing: true}, {token: "===", precedence: 6}, {token: "!==", precedence: 6},
	{token: "??", precedence: 1}, {token: "||", precedence: 1}, {token: "&&", precedence: 2},
	{token: "==", precedence: 6, coercing: true}, {token: "!=", precedence: 6, coercing: true},
	{token: "<=", precedence: 7, coercing: true}, {token: ">=", precedence: 7, coercing: true},
	{token: "<<", precedence: 8, coercing: true}, {token: ">>", precedence: 8, coercing: true},
	{token: "**", precedence: 11, coercing: true}, {token: "in", precedence: 7, coercing: true, word: true},
	{token: "|", precedence: 3, coercing: true}, {token: "^", precedence: 4, coercing: true},
	{token: "&", precedence: 5, coercing: true}, {token: "<", precedence: 7, coercing: true},
	{token: ">", precedence: 7, coercing: true}, {token: "+", precedence: 9, coercing: true},
	{token: "-", precedence: 9, coercing: true}, {token: "*", precedence: 10, coercing: true},
	{token: "/", precedence: 10, coercing: true}, {token: "%", precedence: 10, coercing: true},
}

// playwrightPureMaxDepth bounds recursion on adversarial nesting.
const playwrightPureMaxDepth = 128

// playwrightPureExpression reports whether evaluating raw cannot run repository code or write
// state, so a non-identity `use` value cannot alter a devices descriptor or another project's
// identity (TJAA-V0-018). It admits literals, identifiers, member reads (including optional
// chaining), array literals, object literals of `key: value` or shorthand properties, and the
// non-coercing operators `===`, `!==`, `&&`, `||`, `??`, `?:`, `!`, `typeof` and `void` over those.
// Template substitutions, computed member and property keys, unary `+`, `-`, `~` and every other
// binary operator convert their operands, which can call a user toString, valueOf,
// Symbol.toPrimitive or Symbol.hasInstance, so they admit only operands proven primitive: literals,
// templates, `process.env.NAME` reads and the results of operators over those. It refuses every
// call (including tagged templates and optional calls), assignment, update, delete, new, await,
// yield, import, function, arrow and class expressions, spread, method or accessor definitions, the
// comma operator, regular expression literals and anything it does not recognize.
func playwrightPureExpression(raw string) bool {
	parser := &playwrightPureParser{raw: raw}
	_, ok := parser.expression()
	return ok && parser.skip() == len(raw)
}

type playwrightPureParser struct {
	raw   string
	pos   int
	depth int
}

func (p *playwrightPureParser) skip() int {
	p.pos = skipPlaywrightSpace(p.raw, p.pos)
	return p.pos
}

func (p *playwrightPureParser) peek(token string) bool {
	return strings.HasPrefix(p.raw[p.skip():], token)
}

func (p *playwrightPureParser) accept(token string) bool {
	if !p.peek(token) {
		return false
	}
	p.pos += len(token)
	return true
}

// expression parses a conditional expression; primitive reports a value proven primitive.
func (p *playwrightPureParser) expression() (primitive, ok bool) {
	if p.depth++; p.depth > playwrightPureMaxDepth {
		return false, false
	}
	defer func() { p.depth-- }()
	test, ok := p.binary(0)
	if !ok || !p.conditionalMark() {
		return test, ok
	}
	p.pos++
	consequent, ok := p.expression()
	if !ok || !p.accept(":") {
		return false, false
	}
	alternate, ok := p.expression()
	return consequent && alternate, ok
}

// conditionalMark reports a `?` that starts a conditional, not `??` or optional chaining.
func (p *playwrightPureParser) conditionalMark() bool {
	rest := p.raw[p.skip():]
	return strings.HasPrefix(rest, "?") && !strings.HasPrefix(rest, "??") && (!strings.HasPrefix(rest, "?.") || len(rest) > 2 && rest[2] >= '0' && rest[2] <= '9')
}

func (p *playwrightPureParser) binary(minimum int) (primitive, ok bool) {
	left, ok := p.unary()
	for ok {
		operator, found := p.binaryOperator()
		if !found || operator.precedence <= minimum {
			return left, true
		}
		p.pos += len(operator.token)
		next := operator.precedence
		if operator.token == "**" {
			next-- // right-associative
		}
		right, rightOK := p.binary(next)
		if !rightOK || operator.coercing && !(left && right) {
			return false, false
		}
		switch operator.token {
		case "&&", "||", "??":
			left = left && right
		default:
			left = true
		}
	}
	return false, false
}

func (p *playwrightPureParser) binaryOperator() (playwrightBinaryOperator, bool) {
	rest := p.raw[p.skip():]
	for _, operator := range playwrightBinaryOperators {
		if !strings.HasPrefix(rest, operator.token) {
			continue
		}
		if operator.word {
			if word, _ := playwrightIdentifier(rest, 0); word != operator.token {
				continue
			}
		}
		return operator, true
	}
	return playwrightBinaryOperator{}, false
}

func (p *playwrightPureParser) unary() (primitive, ok bool) {
	if p.depth++; p.depth > playwrightPureMaxDepth {
		return false, false
	}
	defer func() { p.depth-- }()
	rest := p.raw[p.skip():]
	if word, end := playwrightIdentifier(rest, 0); word == "typeof" || word == "void" {
		p.pos += end
		_, ok := p.unary()
		return true, ok
	}
	switch {
	case strings.HasPrefix(rest, "!"):
		p.pos++
		_, ok := p.unary()
		return true, ok
	case strings.HasPrefix(rest, "+"), strings.HasPrefix(rest, "-"), strings.HasPrefix(rest, "~"):
		p.pos++
		operand, ok := p.unary()
		return true, ok && operand
	}
	return p.postfix()
}

// postfix parses a primary expression and its member reads. Only `process.env.NAME` is a member
// read proven primitive: Node keeps every process.env value a string.
func (p *playwrightPureParser) postfix() (primitive, ok bool) {
	start := p.skip()
	primitive, ok = p.primary()
	if !ok {
		return false, false
	}
	chain := []string{}
	if word, end := playwrightIdentifier(p.raw, start); end == p.pos && word == "process" {
		chain = append(chain, word)
	}
	for {
		rest := p.raw[p.skip():]
		switch {
		case strings.HasPrefix(rest, "?.") && !(len(rest) > 2 && rest[2] >= '0' && rest[2] <= '9'):
			p.pos += 2
			if p.peek("[") {
				if !p.computedKey() {
					return false, false
				}
				primitive, chain = false, nil
				continue
			}
			fallthrough
		case strings.HasPrefix(rest, ".") && !strings.HasPrefix(rest, "..."):
			if !strings.HasPrefix(rest, "?.") {
				p.pos++
			}
			word, end := playwrightIdentifier(p.raw, p.skip())
			if word == "" {
				return false, false // optional call, private name or malformed member
			}
			p.pos = end
			if chain != nil {
				chain = append(chain, word)
			}
			primitive = len(chain) == 3 && chain[1] == "env"
		case strings.HasPrefix(rest, "["):
			if !p.computedKey() {
				return false, false
			}
			primitive, chain = false, nil
		case strings.HasPrefix(rest, "("), strings.HasPrefix(rest, "`"), strings.HasPrefix(rest, "'"), strings.HasPrefix(rest, "\""):
			return false, false // call or tagged template
		default:
			return primitive, true
		}
	}
}

// computedKey parses `[expr]`; converting the key to a property key can call user code, so the
// key must be proven primitive.
func (p *playwrightPureParser) computedKey() bool {
	if !p.accept("[") {
		return false
	}
	key, ok := p.expression()
	return ok && key && p.accept("]")
}

func (p *playwrightPureParser) primary() (primitive, ok bool) {
	rest := p.raw[p.skip():]
	if rest == "" {
		return false, false
	}
	switch character := rest[0]; {
	case character == '\'' || character == '"':
		end, ok := playwrightQuotedEnd(p.raw, p.pos)
		p.pos = end
		return true, ok
	case character == '`':
		return true, p.template()
	case character >= '0' && character <= '9' || character == '.' && len(rest) > 1 && rest[1] >= '0' && rest[1] <= '9':
		end := 1
		for end < len(rest) && (rest[end] == '.' || rest[end] == '_' || rest[end] >= '0' && rest[end] <= '9' || rest[end] >= 'a' && rest[end] <= 'z' || rest[end] >= 'A' && rest[end] <= 'Z') {
			end++
		}
		p.pos += end
		return true, true
	case character == '(':
		p.pos++
		inner, ok := p.expression()
		return inner, ok && p.accept(")")
	case character == '[':
		return false, p.arrayLiteral()
	case character == '{':
		return false, p.objectLiteral()
	}
	word, end := playwrightIdentifier(rest, 0)
	if word == "" || playwrightImpureKeywords[word] {
		return false, false
	}
	p.pos += end
	// true, false and null are reserved literals; undefined, NaN and Infinity can be shadowed.
	return word == "true" || word == "false" || word == "null", true
}

// template parses a template literal whose substitutions are proven primitive.
func (p *playwrightPureParser) template() bool {
	for p.pos++; p.pos < len(p.raw); p.pos++ {
		switch {
		case p.raw[p.pos] == '\\':
			p.pos++
		case p.raw[p.pos] == '`':
			p.pos++
			return true
		case strings.HasPrefix(p.raw[p.pos:], "${"):
			p.pos += 2
			substitution, ok := p.expression()
			if !ok || !substitution || !p.peek("}") {
				return false
			}
		}
	}
	return false
}

func (p *playwrightPureParser) arrayLiteral() bool {
	p.pos++
	for !p.accept("]") {
		if p.accept(",") {
			continue // elision
		}
		if _, ok := p.expression(); !ok {
			return false
		}
		if !p.accept(",") && !p.peek("]") {
			return false
		}
	}
	return true
}

func (p *playwrightPureParser) objectLiteral() bool {
	p.pos++
	for !p.accept("}") {
		rest := p.raw[p.skip():]
		shorthand := ""
		switch {
		case strings.HasPrefix(rest, "["):
			if !p.computedKey() {
				return false
			}
		case strings.HasPrefix(rest, "'"), strings.HasPrefix(rest, "\""), rest != "" && rest[0] >= '0' && rest[0] <= '9':
			if _, ok := p.primary(); !ok {
				return false
			}
		default:
			word, end := playwrightIdentifier(rest, 0)
			if word == "" {
				return false // spread or malformed property
			}
			p.pos += end
			shorthand = word
		}
		if p.accept(":") {
			if _, ok := p.expression(); !ok {
				return false
			}
		} else if shorthand == "" || playwrightImpureKeywords[shorthand] || !p.peek(",") && !p.peek("}") {
			return false // method, accessor or malformed property
		}
		if !p.accept(",") && !p.peek("}") {
			return false
		}
	}
	return true
}

func skipPlaywrightSpace(raw string, index int) int {
	for index < len(raw) && strings.IndexByte(" \t\r\n", raw[index]) >= 0 {
		index++
	}
	return index
}

// playwrightIdentifier returns the identifier or keyword at index and its end, or end == index.
func playwrightIdentifier(raw string, index int) (string, int) {
	end := index
	for end < len(raw) {
		character, size := utf8.DecodeRuneInString(raw[end:])
		if character != '_' && character != '$' && !unicode.IsLetter(character) && (end == index || !unicode.IsDigit(character)) {
			break
		}
		end += size
	}
	return raw[index:end], end
}

// playwrightQuotedEnd returns the index after the single- or double-quoted string at index.
func playwrightQuotedEnd(raw string, index int) (int, bool) {
	quote := raw[index]
	for cursor := index + 1; cursor < len(raw); cursor++ {
		switch raw[cursor] {
		case '\\':
			cursor++
		case '\n', '\r':
			return 0, false
		case quote:
			return cursor + 1, true
		}
	}
	return 0, false
}
