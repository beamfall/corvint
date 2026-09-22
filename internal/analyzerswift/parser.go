package analyzerswift

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"unicode/utf8"
)

const (
	appleRevision    = "8588cec3dedaef63bbff458e5e7c8bb6335de107"
	uiRevision       = "830a154d1d8f2f0b1b801a9a9a3b626bc74f6aa6"
	a11yBlob         = "fde6d493b79e38efc92029c3e8a51cd2606463f3"
	uiPackageBlob    = "33e29c099b140ff33449057930504f7b9c86ba94"
	coordinateVector = "swift-language=6.3\n" +
		"swiftpm-tools=6.0\n" +
		"xcode-object-version=77\n" +
		"apple-project-revision=" + appleRevision + "\n" +
		"apple-ui-revision=" + uiRevision + "\n" +
		"source-blob=" + a11yBlob + "\n" +
		"apple-ui-package-blob=" + uiPackageBlob + "\n"
	maxSwiftTokens = 4_096
	maxRawHashes   = 16
)

type swiftTokenKind uint8

const (
	swiftIdentifier swiftTokenKind = iota
	swiftLeftBrace
	swiftRightBrace
	swiftLeftParen
	swiftRightParen
	swiftColon
	swiftComma
	swiftNewline
	swiftString
	swiftAttribute
	swiftOther
)

// swiftToken offsets are uint16 and maxSourceSize is 65,536, so the margin
// against a wrap is exactly one byte: a body must end in LF, which puts the
// last addressable byte at 65,535 and keeps every producible offset in range.
// That is an accident of the LF requirement, not a designed bound, and a
// wrapped start offset would yield a valid-looking slice of the wrong bytes
// rather than a rejection. TestTokenOffsetsHoldAtMaximalSource enforces the
// coupling so raising maxSourceSize fails there instead of silently.
type swiftToken struct {
	start uint16
	end   uint16
	kind  swiftTokenKind
}

func (token swiftToken) text(body []byte) []byte { return body[token.start:token.end] }

func parseCoordinates(request Request, inputs []Input, coordinate []byte, sourceMatches, packageMatches bool) ([]Fact, string) {
	if !bytes.Equal(coordinate, []byte(coordinateVector)) || !sourceMatches || !packageMatches {
		return nil, "EXACT_BINDING_UNAVAILABLE"
	}
	coordinateInput := inputs[0]
	uiInput := inputs[2]
	values := []struct {
		kind, subject, predicate, value string
		related                         Input
	}{
		{"swift.language.coordinate", "swift", "declares-language", "6.3", Input{}},
		{"swiftpm.tools.coordinate", "swiftpm", "declares-tools-version", "6.0", Input{}},
		{"xcode.project.coordinate", "xcode", "declares-object-version", "77", Input{}},
		{"apple.project.revision", "beamfall-apple", "pins-revision", appleRevision, Input{}},
		{"swiftpm.package.dependency", "beamfall-apple", "depends-on", "beamfall-apple-ui@" + uiRevision, uiInput},
	}
	facts := make([]Fact, 0, len(values))
	for _, value := range values {
		facts = append(facts, newFact(request, coordinateInput, value.related, value.kind, value.subject, value.predicate, value.value, "beamfall-apple"))
	}
	return facts, ""
}

func parseSource(request Request, input Input, body []byte) ([]Fact, string) {
	tokens, reason := lexSwift(body)
	if reason != "" {
		return nil, reason
	}
	facts := make([]Fact, 0, 2)
	for index := 0; index < len(tokens); {
		if tokens[index].kind == swiftNewline {
			index++
			continue
		}
		next, reason := consumeAttributes(tokens, index)
		if reason != "" {
			return nil, reason
		}
		index = next
		token := tokens[index]
		if token.kind != swiftIdentifier {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		if string(token.text(body)) == "import" {
			module, next, ok := consumeImport(body, tokens, index)
			if !ok {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			facts = append(facts, newFact(request, input, Input{}, "swift.import.static", input.Path, "imports", module, input.Path))
			index = next
			continue
		}
		kind, name, next, ok := consumeDeclaration(body, tokens, index)
		if !ok {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		factKind, predicate := "swift.symbol.declaration", "declares-"+kind
		if kind == "extension" {
			factKind, predicate = "swift.symbol.extension", "extends"
		}
		facts = append(facts, newFact(request, input, Input{}, factKind, input.Path, predicate, name, request.CompilationUnitID))
		index = next
	}
	return facts, ""
}

func consumeImport(body []byte, tokens []swiftToken, index int) (string, int, bool) {
	if index+2 >= len(tokens) || tokens[index+1].kind != swiftIdentifier || swiftKeyword(tokens[index+1].text(body)) || tokens[index+2].kind != swiftNewline {
		return "", 0, false
	}
	return string(tokens[index+1].text(body)), index + 3, true
}

func consumeDeclaration(body []byte, tokens []swiftToken, index int) (string, string, int, bool) {
	var modifiers uint16
	for index < len(tokens) && tokens[index].kind == swiftIdentifier {
		modifier := swiftModifier(tokens[index].text(body))
		if modifier == 0 {
			break
		}
		if modifiers&modifier != 0 {
			return "", "", 0, false
		}
		modifiers |= modifier
		index++
	}
	if index+2 >= len(tokens) || tokens[index].kind != swiftIdentifier || !swiftDeclaration(tokens[index].text(body)) || tokens[index+1].kind != swiftIdentifier || swiftKeyword(tokens[index+1].text(body)) {
		return "", "", 0, false
	}
	kind, name := string(tokens[index].text(body)), string(tokens[index+1].text(body))
	index += 2
	if index < len(tokens) && tokens[index].kind == swiftColon {
		index++
		for {
			// An inherited type may carry its own attribute -- `@unchecked
			// Sendable`, `@retroactive Equatable` -- in the same closed form.
			next, reason := consumeAttributes(tokens, index)
			if reason != "" {
				return "", "", 0, false
			}
			index = next
			if index >= len(tokens) || tokens[index].kind != swiftIdentifier || swiftKeyword(tokens[index].text(body)) {
				return "", "", 0, false
			}
			index++
			if index >= len(tokens) || tokens[index].kind != swiftComma {
				break
			}
			index++
		}
	}
	if index >= len(tokens) || tokens[index].kind != swiftLeftBrace {
		return "", "", 0, false
	}
	depth := 0
	for index < len(tokens) {
		switch tokens[index].kind {
		case swiftLeftBrace:
			depth++
		case swiftRightBrace:
			depth--
			if depth == 0 {
				index++
				if index == len(tokens) {
					return kind, name, index, true
				}
				if tokens[index].kind != swiftNewline {
					return "", "", 0, false
				}
				return kind, name, index + 1, true
			}
			if depth < 0 {
				return "", "", 0, false
			}
		}
		index++
	}
	return "", "", 0, false
}

// consumeAttributes consumes a run of attribute clauses and any newlines that
// separate them, returning the index of the first token that is not part of the
// run. An attribute is closed grammar: `@` plus an identifier, optionally
// followed by one balanced parenthesised argument clause. Attributes are not
// fact-bearing under the frozen tuple matrix, so consuming them adds no fact;
// it only stops an ordinary Swift declaration from being rejected because it
// carries one. A run that reaches end of input, or that is followed by anything
// other than a declaration or import, is not an attributed declaration and
// rejects the whole request.
func consumeAttributes(tokens []swiftToken, index int) (int, string) {
	start := index
	for index < len(tokens) && tokens[index].kind == swiftAttribute {
		index++
		if index < len(tokens) && tokens[index].kind == swiftLeftParen {
			next, reason := consumeAttributeArguments(tokens, index)
			if reason != "" {
				return 0, reason
			}
			index = next
		}
		for index < len(tokens) && tokens[index].kind == swiftNewline {
			index++
		}
	}
	if index != start && (index == len(tokens) || tokens[index].kind != swiftIdentifier) {
		return 0, "UNSUPPORTED_SCHEMA"
	}
	return index, ""
}

// consumeAttributeArguments consumes one balanced parenthesised attribute
// argument clause. A brace inside the clause is outside the closed grammar:
// admitting it would let attribute payload perturb the brace depth that
// delimits a declaration body, so it rejects rather than being counted.
func consumeAttributeArguments(tokens []swiftToken, index int) (int, string) {
	depth := 0
	for index < len(tokens) {
		switch tokens[index].kind {
		case swiftLeftParen:
			depth++
		case swiftRightParen:
			depth--
			if depth == 0 {
				return index + 1, ""
			}
		case swiftLeftBrace, swiftRightBrace:
			return 0, "UNSUPPORTED_SCHEMA"
		}
		index++
	}
	return 0, "UNSUPPORTED_SCHEMA"
}

// scanSwiftAttribute reads the identifier of one `@` attribute and returns the
// offset just past it. `@` not followed by an identifier is not Swift at all.
func scanSwiftAttribute(body []byte, index int) (int, string) {
	next := index + 1
	if next >= len(body) || !asciiIdentifierStart(body[next]) {
		return 0, "MALFORMED_INPUT"
	}
	for next < len(body) && asciiIdentifierContinue(body[next]) {
		next++
	}
	return next, ""
}

// poundReason classifies a `#` run that does not open a raw string literal.
//
// A `#` run followed by an identifier is a real Swift construct the closed
// grammar does not implement: conditional compilation (`#if`, `#elseif`,
// `#else`, `#endif`, `#sourceLocation`), whose declaration set depends on a
// build configuration absent from the input, or a freestanding macro or pound
// literal (`#Preview`, `#available`, `#selector`, `#warning`) whose expansion
// is not present in the bytes. Both are UNSUPPORTED_SCHEMA, the closed reason
// for a construct outside the matrix. Neither is DYNAMIC_INPUT, which this
// family reserves for bytes whose meaning depends on a runtime value --
// string interpolation. Reporting them as dynamic input sent coverage triage
// looking for interpolation that is not there. The closed reason taxonomy in
// docs/specs/analyzer-candidate-profiles.md has no finer code, and adding one
// is an operator amendment, so these two causes still share a bucket.
//
// A `#` run long enough to be a raw-string delimiter but over maxRawHashes is
// a bounded-resource refusal, not a grammar refusal.
func poundReason(body []byte, index, hashes int) string {
	if index+hashes < len(body) && body[index+hashes] == '"' {
		return "LIMIT_EXCEEDED"
	}
	if index+hashes < len(body) && asciiIdentifierStart(body[index+hashes]) {
		return "UNSUPPORTED_SCHEMA"
	}
	return "MALFORMED_INPUT"
}

func lexSwift(body []byte) ([]swiftToken, string) {
	// An over-size body is a bounded-resource refusal, not malformed Swift:
	// reporting it as MALFORMED_INPUT sent triage looking for a lexical defect
	// in a file that is merely larger than the family's source ceiling.
	if len(body) > maxSourceSize {
		return nil, "LIMIT_EXCEEDED"
	}
	if len(body) == 0 || body[len(body)-1] != '\n' || !utf8.Valid(body) {
		return nil, "MALFORMED_INPUT"
	}
	tokens := make([]swiftToken, 0, minInt(256, len(body)))
	emit := func(token swiftToken) bool {
		if len(tokens) == maxSwiftTokens {
			return false
		}
		tokens = append(tokens, token)
		return true
	}
	for index := 0; index < len(body); {
		switch body[index] {
		case ' ':
			index++
		case '\n':
			if !emit(swiftToken{kind: swiftNewline}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		case '\r', '\t', 0:
			return nil, "MALFORMED_INPUT"
		case '/':
			if index+1 >= len(body) {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			switch body[index+1] {
			case '/':
				next, reason := skipLineComment(body, index+2)
				if reason != "" {
					return nil, reason
				}
				index = next
			case '*':
				next, reason := skipBlockComment(body, index+2)
				if reason != "" {
					return nil, reason
				}
				index = next
			default:
				if !emit(swiftToken{kind: swiftOther}) {
					return nil, "LIMIT_EXCEEDED"
				}
				index++
			}
		case '*':
			if index+1 < len(body) && body[index+1] == '/' {
				return nil, "MALFORMED_INPUT"
			}
			if !emit(swiftToken{kind: swiftOther}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		case '"':
			next, reason := scanSwiftString(body, index)
			if reason != "" {
				return nil, reason
			}
			if !emit(swiftToken{kind: swiftString}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index = next
		case '#':
			hashes := 0
			for index+hashes < len(body) && body[index+hashes] == '#' {
				hashes++
			}
			if hashes > maxRawHashes || index+hashes >= len(body) || body[index+hashes] != '"' {
				return nil, poundReason(body, index, hashes)
			}
			next, reason := scanSwiftRawString(body, index, hashes)
			if reason != "" {
				return nil, reason
			}
			if !emit(swiftToken{kind: swiftString}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index = next
		case '@':
			next, reason := scanSwiftAttribute(body, index)
			if reason != "" {
				return nil, reason
			}
			// The attribute name is never read: an attribute carries no tuple,
			// so the token records only that one was consumed, like a brace.
			if !emit(swiftToken{kind: swiftAttribute}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index = next
		case '(':
			if !emit(swiftToken{kind: swiftLeftParen}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		case ')':
			if !emit(swiftToken{kind: swiftRightParen}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		case '{':
			if !emit(swiftToken{kind: swiftLeftBrace}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		case '}':
			if !emit(swiftToken{kind: swiftRightBrace}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		case ':':
			if !emit(swiftToken{kind: swiftColon}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		case ',':
			if !emit(swiftToken{kind: swiftComma}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		default:
			if asciiIdentifierStart(body[index]) {
				start := index
				for index < len(body) && asciiIdentifierContinue(body[index]) {
					index++
				}
				if !emit(swiftToken{start: uint16(start), end: uint16(index), kind: swiftIdentifier}) {
					return nil, "LIMIT_EXCEEDED"
				}
				continue
			}
			if body[index] < 32 || body[index] > 126 {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			if !emit(swiftToken{kind: swiftOther}) {
				return nil, "LIMIT_EXCEEDED"
			}
			index++
		}
	}
	return tokens, ""
}

func skipLineComment(body []byte, index int) (int, string) {
	for index < len(body) && body[index] != '\n' {
		if body[index] < 32 || body[index] == 127 {
			return 0, "MALFORMED_INPUT"
		}
		index++
	}
	return index, ""
}

func skipBlockComment(body []byte, index int) (int, string) {
	depth := 1
	for index < len(body) {
		if index+1 < len(body) && body[index] == '/' && body[index+1] == '*' {
			depth++
			index += 2
			continue
		}
		if index+1 < len(body) && body[index] == '*' && body[index+1] == '/' {
			depth--
			index += 2
			if depth == 0 {
				return index, ""
			}
			continue
		}
		if body[index] == '\n' {
			index++
			continue
		}
		if body[index] < 32 || body[index] == 127 {
			return 0, "MALFORMED_INPUT"
		}
		index++
	}
	return 0, "MALFORMED_INPUT"
}

func scanSwiftString(body []byte, index int) (int, string) {
	multiline := index+2 < len(body) && bytes.Equal(body[index:index+3], []byte(`"""`))
	if multiline {
		index += 3
	} else {
		index++
	}
	for index < len(body) {
		if body[index] == '\\' {
			if index+1 >= len(body) {
				return 0, "MALFORMED_INPUT"
			}
			if body[index+1] == '(' {
				return 0, "DYNAMIC_INPUT"
			}
			index += 2
			continue
		}
		if multiline && index+2 < len(body) && bytes.Equal(body[index:index+3], []byte(`"""`)) {
			return index + 3, ""
		}
		if !multiline && body[index] == '"' {
			return index + 1, ""
		}
		if body[index] == '\n' {
			if !multiline {
				return 0, "MALFORMED_INPUT"
			}
			index++
			continue
		}
		index++
	}
	return 0, "MALFORMED_INPUT"
}

func scanSwiftRawString(body []byte, index, hashes int) (int, string) {
	start := index
	index += hashes
	if index+2 < len(body) && bytes.Equal(body[index:index+3], []byte(`"""`)) {
		return scanSwiftStringBody(body, index+3, hashes, true)
	}
	if index >= len(body) || body[index] != '"' {
		return 0, "MALFORMED_INPUT"
	}
	if start+hashes != index {
		return 0, "MALFORMED_INPUT"
	}
	return scanSwiftStringBody(body, index+1, hashes, false)
}

func scanSwiftStringBody(body []byte, index, hashes int, multiline bool) (int, string) {
	for index < len(body) {
		if isInterpolation(body, index, hashes) {
			return 0, "DYNAMIC_INPUT"
		}
		quotes := 1
		if multiline {
			quotes = 3
		}
		if index+quotes+hashes <= len(body) && bytes.Equal(body[index:index+quotes], bytes.Repeat([]byte{'"'}, quotes)) && bytes.Equal(body[index+quotes:index+quotes+hashes], bytes.Repeat([]byte{'#'}, hashes)) {
			return index + quotes + hashes, ""
		}
		if body[index] == '\n' {
			if !multiline {
				return 0, "MALFORMED_INPUT"
			}
			index++
			continue
		}
		if body[index] < 32 || body[index] == 127 {
			return 0, "MALFORMED_INPUT"
		}
		index++
	}
	return 0, "MALFORMED_INPUT"
}

func isInterpolation(body []byte, index, hashes int) bool {
	if body[index] != '\\' {
		return false
	}
	index++
	for count := 0; count < hashes; count++ {
		if index >= len(body) || body[index] != '#' {
			return false
		}
		index++
	}
	return index < len(body) && body[index] == '('
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func asciiIdentifierStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func asciiIdentifierContinue(value byte) bool {
	return asciiIdentifierStart(value) || value >= '0' && value <= '9'
}

func swiftModifier(value []byte) uint16 {
	switch string(value) {
	case "public":
		return 1 << 0
	case "private":
		return 1 << 1
	case "internal":
		return 1 << 2
	case "fileprivate":
		return 1 << 3
	case "open":
		return 1 << 4
	case "final":
		return 1 << 5
	case "indirect":
		return 1 << 6
	case "nonisolated":
		return 1 << 7
	}
	return 0
}

func swiftDeclaration(value []byte) bool {
	switch string(value) {
	case "class", "struct", "enum", "protocol", "actor", "extension":
		return true
	default:
		return false
	}
}

func swiftKeyword(value []byte) bool {
	switch string(value) {
	case "associatedtype", "case", "class", "deinit", "enum", "extension", "func", "import", "init", "inout", "let", "operator", "precedencegroup", "protocol", "repeat", "static", "struct", "subscript", "typealias", "var":
		return true
	default:
		return false
	}
}

func gitBlob(body []byte) string {
	hash := sha1.New()
	_, _ = hash.Write([]byte("blob " + strconv.Itoa(len(body)) + "\x00"))
	_, _ = hash.Write(body)
	return hex.EncodeToString(hash.Sum(nil))
}

func gitBlobMatches(body []byte, expected string) bool {
	hash := sha1.New()
	_, _ = hash.Write([]byte("blob " + strconv.Itoa(len(body)) + "\x00"))
	_, _ = hash.Write(body)
	sum := hash.Sum(nil)
	if len(expected) != len(sum)*2 {
		return false
	}
	for index, byteValue := range sum {
		if expected[index*2] != hexDigit(byteValue>>4) || expected[index*2+1] != hexDigit(byteValue&0x0f) {
			return false
		}
	}
	return true
}

func newFact(request Request, input, related Input, kind, subject, predicate, value, instance string) Fact {
	relatedHandle, relatedDigest := "-", "-"
	if related.Handle != "" {
		relatedHandle, relatedDigest = related.Handle, related.SHA256
	}
	target := `{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`
	fields := []string{
		request.Family, request.RequestID, request.ScopeID, request.CompilationUnitID, target,
		input.Handle, input.SHA256, relatedHandle, relatedDigest, kind, subject, predicate, value, instance,
	}
	return Fact{
		Kind: kind, InputHandle: input.Handle, RelatedHandle: relatedHandle, Subject: subject,
		Predicate: predicate, Value: value, InstanceID: instance, EvidenceSHA256: evidence(fields),
	}
}

func evidence(fields []string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var value [4]byte
	binary.BigEndian.PutUint32(value[:], uint32(len(fields)))
	_, _ = hash.Write(value[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(value[:], uint32(len(field)))
		_, _ = hash.Write(value[:])
		_, _ = hash.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
