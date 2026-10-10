package typescript

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// playwrightPureScope names the roots whose member reads cannot run code (TJAA-V0-018). A member
// read can invoke a getter or a Proxy trap, so a pure expression reads a member only of:
//   - the `devices` import from '@playwright/test', through a device-name key and then one scalar
//     descriptor field (userAgent, deviceScaleFactor, isMobile, hasTouch, defaultBrowserType) or
//     viewport/screen width/height, when every other occurrence of `devices` in the config is
//     that import, a `...devices[...]` spread or such a read; and
//   - a top-level `const` bound to a plain object literal: `key: value` properties only, with an
//     identifier or string key other than __proto__, each value a nested plain literal or an
//     expression proven primitive; and every other occurrence of the name in the config a read
//     chain that resolves through own keys to a primitive value, never written, updated,
//     deleted, called, destructured into, spread, aliased as an object or shadowed.
//
// Every other root (process.env, other imports, globals, `this`, call results) is refused. A
// config that names `eval`, has a `delete`, `++` or `--` token or a `\` escape in code (only an
// escaped identifier can hold one there) has no admitted roots, whatever the token's operand. Occurrences are found in the comment-stripped
// text, string and template contents included, so an unrecognized one fails closed.
type playwrightPureScope struct {
	devices  bool
	literals map[string]playwrightPlainValue
}

// playwrightPlainValue is one property value of a plain const literal: a nested plain literal
// (object non-nil) or a value proven primitive.
type playwrightPlainValue struct {
	object map[string]playwrightPlainValue
}

var (
	playwrightDevicesImport = regexp.MustCompile(`(?m)^[\t ]*import[\t ]*\{([^}]*)\}[\t ]*from[\t ]*('@playwright/test'|"@playwright/test")`)
	playwrightConstLiteral  = regexp.MustCompile(`(?m)^[\t ]*const[\t ]+([A-Za-z_$][A-Za-z0-9_$]*)[\t ]*=[\t ]*\{`)
	// playwrightDeviceScalars are descriptor fields whose value is a primitive.
	playwrightDeviceScalars = []string{"userAgent", "deviceScaleFactor", "isMobile", "hasTouch", "defaultBrowserType"}
	// playwrightObjectPrototypeKeys reach Object.prototype or a function instead of a descriptor.
	playwrightObjectPrototypeKeys = []string{"constructor", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf"}
)

// newPlaywrightPureScope derives the admitted member-read roots of a comment-stripped config.
func newPlaywrightPureScope(clean string) playwrightPureScope {
	scope := playwrightPureScope{literals: map[string]playwrightPlainValue{}}
	lexed, ok := playwrightLex(clean)
	if !ok || playwrightHasToken(clean, "eval") || playwrightCodeMutationToken(clean, lexed) {
		return scope
	}
	if match := playwrightDevicesImport.FindStringSubmatchIndex(clean); match != nil && lexed.topLevel(match[0]) {
		specifier := -1
		offset := match[2]
		for _, item := range strings.Split(clean[match[2]:match[3]], ",") {
			if strings.TrimSpace(item) == "devices" {
				specifier = offset + strings.Index(item, "devices")
			}
			offset += len(item) + 1
		}
		scope.devices = specifier >= 0 && playwrightDevicesSound(clean, lexed, specifier)
	}
	declared := map[string]int{}
	for _, match := range playwrightConstLiteral.FindAllStringSubmatchIndex(clean, -1) {
		declared[clean[match[2]:match[3]]]++
	}
	for _, match := range playwrightConstLiteral.FindAllStringSubmatchIndex(clean, -1) {
		name := clean[match[2]:match[3]]
		if declared[name] != 1 || name == "devices" || playwrightImpureKeywords[name] || !lexed.topLevel(match[0]) {
			continue
		}
		body, next, ok := playwrightBalancedValue(clean, match[1], '}')
		if !ok || !playwrightStatementEnds(clean, next) {
			continue
		}
		object, ok := playwrightPlainLiteral(body)
		if ok && playwrightLiteralSound(clean, lexed, name, match[2], object) {
			scope.literals[name] = playwrightPlainValue{object: object}
		}
	}
	return scope
}

// playwrightCodeMutationToken reports a `\` (an escaped identifier), `++`, `--` or `delete` in
// code, outside string, template and regular-expression content.
func playwrightCodeMutationToken(clean string, lexed playwrightLexed) bool {
	for index := 0; index < len(clean); index++ {
		if !lexed.code[index] {
			continue
		}
		switch clean[index] {
		case '\\':
			return true
		case '+', '-':
			if index+1 < len(clean) && clean[index+1] == clean[index] && lexed.code[index+1] {
				return true
			}
		}
	}
	for _, at := range playwrightTokenOffsets(clean, "delete") {
		if lexed.code[at] {
			return true
		}
	}
	return false
}

// memberRead reports whether reading keys from root is admitted.
func (scope playwrightPureScope) memberRead(root string, keys []string) bool {
	if root == "devices" {
		return scope.devices && playwrightDeviceRead(keys)
	}
	literal, ok := scope.literals[root]
	return ok && literal.resolves(keys)
}

// resolves reports whether keys name own properties down to a primitive value.
func (value playwrightPlainValue) resolves(keys []string) bool {
	for _, key := range keys {
		if value.object == nil {
			return false
		}
		child, ok := value.object[key]
		if !ok {
			return false
		}
		value = child
	}
	return len(keys) > 0 && value.object == nil
}

// playwrightDeviceRead admits a device-name key followed by a scalar field or by viewport/screen
// and width/height.
func playwrightDeviceRead(keys []string) bool {
	if len(keys) < 2 || strings.HasPrefix(keys[0], "__") || slices.Contains(playwrightObjectPrototypeKeys, keys[0]) {
		return false
	}
	if len(keys) == 2 {
		return slices.Contains(playwrightDeviceScalars, keys[1])
	}
	return len(keys) == 3 && (keys[1] == "viewport" || keys[1] == "screen") && (keys[2] == "width" || keys[2] == "height")
}

// playwrightPlainLiteral parses the body of a plain object literal.
func playwrightPlainLiteral(body string) (map[string]playwrightPlainValue, bool) {
	items, ok := playwrightSplitTopLevel(body)
	if !ok {
		return nil, false
	}
	object := map[string]playwrightPlainValue{}
	for index, item := range items {
		if item == "" && index == len(items)-1 && index > 0 {
			continue // trailing comma
		}
		if item == "" && len(items) == 1 {
			continue // empty literal
		}
		colon := playwrightTopLevelColon(item)
		if colon <= 0 || strings.HasPrefix(item, "...") {
			return nil, false // spread, method, accessor or shorthand
		}
		keyRaw := strings.TrimSpace(item[:colon])
		key, quoted := playwrightString(keyRaw)
		if !quoted {
			word, end := playwrightIdentifier(keyRaw, 0)
			if word == "" || end != len(keyRaw) {
				return nil, false // computed, numeric, accessor or method key
			}
			key = word
		}
		if _, duplicate := object[key]; duplicate || key == "__proto__" {
			return nil, false
		}
		value := strings.TrimSpace(item[colon+1:])
		if strings.HasPrefix(value, "{") {
			nested, exact := playwrightObject(value)
			if !exact || strings.HasSuffix(value, ";") {
				return nil, false
			}
			inner, ok := playwrightPlainLiteral(nested[1 : len(nested)-1])
			if !ok {
				return nil, false
			}
			object[key] = playwrightPlainValue{object: inner}
			continue
		}
		parser := &playwrightPureParser{raw: value}
		primitive, ok := parser.expression()
		if !ok || !primitive || parser.skip() != len(value) {
			return nil, false
		}
		object[key] = playwrightPlainValue{}
	}
	return object, true
}

// playwrightStatementEnds reports whether the const initializer ends at index: a `;`, the end of
// the file, or a line break before a new statement word.
func playwrightStatementEnds(raw string, index int) bool {
	for index < len(raw) && (raw[index] == ' ' || raw[index] == '\t') {
		index++
	}
	if index == len(raw) || raw[index] == ';' {
		return true
	}
	if raw[index] != '\n' && raw[index] != '\r' {
		return false
	}
	index = skipPlaywrightSpace(raw, index)
	if index == len(raw) {
		return true
	}
	word, _ := playwrightIdentifier(raw, index)
	return word != "" && word != "in" && word != "instanceof" && word != "as" && word != "satisfies"
}

// playwrightLiteralSound reports whether every occurrence of name other than its declaration is
// a read chain resolving to a primitive value of object.
func playwrightLiteralSound(raw string, lexed playwrightLexed, name string, declaration int, object map[string]playwrightPlainValue) bool {
	for _, at := range playwrightTokenOffsets(raw, name) {
		if at == declaration {
			continue
		}
		if !lexed.code[at] {
			return false
		}
		if previous := playwrightPreviousCode(raw, at); previous >= 0 && raw[previous] == '.' && (previous == 0 || raw[previous-1] != '.') {
			continue // a property name, not the binding
		}
		keys, end, ok := playwrightMemberChain(raw, at+len(name))
		if !ok || !(playwrightPlainValue{object: object}).resolves(keys) || !playwrightReadContext(raw, lexed, at, end) {
			return false
		}
	}
	return true
}

// playwrightDevicesSound reports whether every occurrence of `devices` other than the import
// specifier is a `...devices[...]` spread or an admitted read.
func playwrightDevicesSound(raw string, lexed playwrightLexed, specifier int) bool {
	for _, at := range playwrightTokenOffsets(raw, "devices") {
		if at == specifier {
			continue
		}
		if !lexed.code[at] {
			return false
		}
		keys, end, ok := playwrightMemberChain(raw, at+len("devices"))
		if !ok {
			return false
		}
		previous := playwrightPreviousCode(raw, at)
		if previous >= 2 && raw[previous-2:previous+1] == "..." {
			if len(keys) != 1 || !playwrightReadContext(raw, lexed, previous-2, end) {
				return false
			}
			continue
		}
		if !playwrightDeviceRead(keys) || !playwrightReadContext(raw, lexed, at, end) {
			return false
		}
	}
	return true
}

// playwrightReadContext reports whether the expression raw[start:end] is only read: the code
// before it does not update, delete, construct or spread it, the code after it does not assign,
// update, call or tag it, and no enclosing bracket group is a destructuring or for-in/of target.
func playwrightReadContext(raw string, lexed playwrightLexed, start, end int) bool {
	if previous := playwrightPreviousCode(raw, start); previous >= 0 {
		character := raw[previous]
		word := ""
		if character < utf8.RuneSelf && (character == '_' || character == '$' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
			from := previous
			for from > 0 && strings.IndexByte("_$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", raw[from-1]) >= 0 {
				from--
			}
			word = raw[from : previous+1]
		}
		switch {
		case word != "":
			if word != "return" && word != "typeof" && word != "void" {
				return false
			}
		case !strings.ContainsRune("([{,:?=!&|+-*/%<>^~;})", rune(character)):
			return false
		case (character == '+' || character == '-') && previous > 0 && raw[previous-1] == character:
			return false
		case character == '.':
			return false
		}
	}
	after := skipPlaywrightSpace(raw, end)
	if after < len(raw) {
		rest := raw[after:]
		if !strings.ContainsRune(",;)}]:?+-*/%<>&|^!=", rune(rest[0])) {
			return false
		}
		if playwrightAssignmentNext(rest) || strings.HasPrefix(rest, "++") || strings.HasPrefix(rest, "--") || rest[0] == '!' && !strings.HasPrefix(rest, "!=") {
			return false
		}
	}
	for group := lexed.parent[start]; group >= 0; group = lexed.parentOf[group] {
		if strings.IndexByte("([{", raw[group]) < 0 {
			continue
		}
		next := skipPlaywrightSpace(raw, lexed.closes[group]+1)
		if next < len(raw) && (raw[next] >= utf8.RuneSelf || playwrightAssignmentNext(raw[next:])) {
			return false
		}
		if word, _ := playwrightIdentifier(raw, next); word == "of" || word == "in" {
			return false
		}
	}
	return true
}

var playwrightAssignment = regexp.MustCompile(`^(?:=[^=>]|=$|(?:\*\*|>>>|<<|>>|&&|\|\||\?\?|[-+*/%&|^])=)`)

// playwrightAssignmentNext reports whether rest starts with an assignment operator.
func playwrightAssignmentNext(rest string) bool {
	return playwrightAssignment.MatchString(rest)
}

// playwrightMemberChain parses `.name`, `?.name`, `['key']` and `?.['key']` reads from index and
// returns the keys and the end. A computed key other than a string literal, a private name, an
// optional call or a malformed member is refused; `?.` before a digit is a conditional.
func playwrightMemberChain(raw string, index int) ([]string, int, bool) {
	keys := []string{}
	for {
		cursor := skipPlaywrightSpace(raw, index)
		rest := raw[cursor:]
		switch {
		case strings.HasPrefix(rest, "?.") && !(len(rest) > 2 && rest[2] >= '0' && rest[2] <= '9'):
			cursor = skipPlaywrightSpace(raw, cursor+2)
			if strings.HasPrefix(raw[cursor:], "[") {
				key, end, ok := playwrightComputedStringKey(raw, cursor)
				if !ok {
					return nil, index, false
				}
				keys, index = append(keys, key), end
				continue
			}
			word, end := playwrightIdentifier(raw, cursor)
			if word == "" {
				return nil, index, false
			}
			keys, index = append(keys, word), end
		case strings.HasPrefix(rest, ".") && !strings.HasPrefix(rest, "..."):
			cursor = skipPlaywrightSpace(raw, cursor+1)
			word, end := playwrightIdentifier(raw, cursor)
			if word == "" {
				return nil, index, false
			}
			keys, index = append(keys, word), end
		case strings.HasPrefix(rest, "["):
			key, end, ok := playwrightComputedStringKey(raw, cursor)
			if !ok {
				return nil, index, false
			}
			keys, index = append(keys, key), end
		default:
			return keys, index, true
		}
	}
}

func playwrightComputedStringKey(raw string, index int) (string, int, bool) {
	start := skipPlaywrightSpace(raw, index+1)
	if start >= len(raw) || raw[start] != '\'' && raw[start] != '"' {
		return "", index, false
	}
	end, ok := playwrightQuotedEnd(raw, start)
	if !ok {
		return "", index, false
	}
	key, ok := playwrightString(raw[start:end])
	close := skipPlaywrightSpace(raw, end)
	if !ok || close >= len(raw) || raw[close] != ']' {
		return "", index, false
	}
	return key, close + 1, true
}

// playwrightTokenOffsets returns every offset where name occurs as a whole identifier.
func playwrightTokenOffsets(raw, name string) []int {
	offsets := []int{}
	identifierRune := func(character rune) bool {
		return character == '_' || character == '$' || character == '\\' || character == utf8.RuneError || unicode.IsLetter(character) || unicode.IsDigit(character)
	}
	for from := 0; ; {
		index := strings.Index(raw[from:], name)
		if index < 0 {
			return offsets
		}
		at := from + index
		from = at + len(name)
		before, _ := utf8.DecodeLastRuneInString(raw[:at])
		after, _ := utf8.DecodeRuneInString(raw[at+len(name):])
		if at > 0 && identifierRune(before) || at+len(name) < len(raw) && identifierRune(after) {
			continue
		}
		offsets = append(offsets, at)
	}
}

func playwrightHasToken(raw, name string) bool {
	return len(playwrightTokenOffsets(raw, name)) != 0
}

// playwrightPreviousCode returns the index of the last non-space byte before index, or -1.
func playwrightPreviousCode(raw string, index int) int {
	for index--; index >= 0 && strings.IndexByte(" \t\r\n", raw[index]) >= 0; index-- {
	}
	return index
}

// playwrightLexed marks the code bytes of a comment-stripped module, the innermost bracket or
// template substitution enclosing each byte, and where each bracket closes.
type playwrightLexed struct {
	code     []bool
	parent   []int
	parentOf map[int]int
	closes   map[int]int
}

func (lexed playwrightLexed) topLevel(index int) bool {
	return lexed.code[index] && lexed.parent[index] < 0
}

// playwrightLex tokenizes quotes, templates (with nested substitutions), regular expression
// literals and brackets; it fails on an unbalanced module.
func playwrightLex(raw string) (playwrightLexed, bool) {
	lexed := playwrightLexed{code: make([]bool, len(raw)), parent: make([]int, len(raw)), parentOf: map[int]int{}, closes: map[int]int{}}
	stack := []int{}
	top := func() int {
		if len(stack) == 0 {
			return -1
		}
		return stack[len(stack)-1]
	}
	push := func(index int) {
		lexed.parentOf[index] = top()
		stack = append(stack, index)
	}
	for index := 0; index < len(raw); index++ {
		lexed.parent[index] = top()
		if top() >= 0 && raw[top()] == '`' {
			switch {
			case raw[index] == '\\':
				index++
				if index < len(raw) {
					lexed.parent[index] = top()
				}
			case raw[index] == '`':
				stack = stack[:len(stack)-1]
			case strings.HasPrefix(raw[index:], "${"):
				push(index)
				index++
				lexed.parent[index] = top()
			}
			continue
		}
		character := raw[index]
		lexed.code[index] = true
		switch character {
		case '\'', '"':
			end, ok := playwrightQuotedEnd(raw, index)
			if !ok {
				return lexed, false
			}
			lexed.code[index] = false
			for cursor := index + 1; cursor < end; cursor++ {
				lexed.parent[cursor] = top()
			}
			index = end - 1
		case '`':
			lexed.code[index] = false
			push(index)
		case '/':
			if !playwrightSlashStartsRegex(raw, index) {
				continue
			}
			class := false
			cursor := index + 1
			for ; cursor < len(raw) && (raw[cursor] != '/' || class); cursor++ {
				switch raw[cursor] {
				case '\\':
					cursor++
				case '[':
					class = true
				case ']':
					class = false
				case '\n', '\r':
					return lexed, false
				}
			}
			if cursor >= len(raw) {
				return lexed, false
			}
			for at := index; at <= cursor; at++ {
				lexed.code[at] = false
				lexed.parent[at] = top()
			}
			index = cursor
		case '{', '[', '(':
			push(index)
		case '}', ']', ')':
			open := top()
			if open < 0 {
				return lexed, false
			}
			if raw[open] == '$' && character == '}' {
				stack = stack[:len(stack)-1]
				lexed.code[index] = false
				continue
			}
			if !playwrightPair(raw[open], character) {
				return lexed, false
			}
			lexed.closes[open] = index
			stack = stack[:len(stack)-1]
		}
	}
	return lexed, len(stack) == 0
}
