package analyzerkotlinandroid

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode"
)

// scanCanonicalJSON is deliberately non-retaining. json.Unmarshal happens only
// after this bounded structural pass has rejected deep, token-heavy, and long
// ordinary-string requests.
// scanCanonicalJSON returns "" for an acceptable payload, LIMIT_EXCEEDED when
// a depth/token/string bound is crossed, and NONCANONICAL_REQUEST for invalid
// syntax — the closed matrix assigns parser-bound violations to LIMIT_EXCEEDED.
func scanCanonicalJSON(raw []byte) string {
	depth, tokens, start := 0, 0, 0
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '{', '[':
			depth++
			tokens++
			if depth > 8 {
				return "LIMIT_EXCEEDED"
			}
		case '}', ']':
			depth--
			if depth < 0 {
				return "NONCANONICAL_REQUEST"
			}
		case ',', ':':
			tokens++
		case '"':
			start = index
			index++
			for index < len(raw) && raw[index] != '"' {
				if raw[index] == '\\' {
					index++
				}
				index++
			}
			if index >= len(raw) {
				return "NONCANONICAL_REQUEST"
			}
			if index-start-1 > 1_398_104 {
				return "LIMIT_EXCEEDED"
			}
			tokens++
		}
		if tokens > 4096 {
			return "LIMIT_EXCEEDED"
		}
	}
	if depth != 0 {
		return "NONCANONICAL_REQUEST"
	}
	// Bounds are enforced first: a request over the depth/token/string caps is
	// LIMIT_EXCEEDED even where Go's own validator would call it invalid.
	if !json.Valid(raw) {
		return "NONCANONICAL_REQUEST"
	}
	return ""
}

type token struct {
	text                             string
	line, column, endLine, endColumn int
	quoted                           bool
	// dynamic marks a quoted token whose content interpolates ($name or
	// ${...}); its true runtime value is bytes the analyzer was never given,
	// so a consumer must refuse the fact rather than assert this token's
	// literal text as the value.
	dynamic bool
}

// literalText returns a quoted token's text for use as a fact value, or
// rejects the whole request with DYNAMIC_INPUT when the token interpolates:
// the candidate never invents a fact about bytes the interpolation would
// supply at runtime.
func literalText(t token) (string, string) {
	if t.dynamic {
		return "", "DYNAMIC_INPUT"
	}
	return t.text, ""
}

func kotlinTokens(source []byte) ([]token, string) {
	if !bytes.Equal(bytes.ToValidUTF8(source, []byte("?")), source) {
		return nil, "MALFORMED_INPUT"
	}
	result := make([]token, 0, len(source)/8)
	line, column := 1, 1
	advance := func(b byte) {
		if b == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	for index := 0; index < len(source); {
		b := source[index]
		if b == ' ' || b == '\t' || b == '\r' {
			advance(b)
			index++
			continue
		}
		if b == '\n' {
			startLine, startColumn := line, column
			advance(b)
			index++
			result = append(result, token{text: "\n", line: startLine, column: startColumn, endLine: line, endColumn: column})
			continue
		}
		if index+1 < len(source) && source[index : index+2][0] == '/' && source[index : index+2][1] == '/' {
			for index < len(source) && source[index] != '\n' {
				advance(source[index])
				index++
			}
			continue
		}
		if index+1 < len(source) && source[index] == '/' && source[index+1] == '*' {
			startLine, depth := line, 1
			advance(source[index])
			advance(source[index+1])
			index += 2
			for index < len(source) && depth > 0 {
				if index+1 < len(source) && source[index] == '/' && source[index+1] == '*' {
					depth++
					advance(source[index])
					advance(source[index+1])
					index += 2
					continue
				}
				if index+1 < len(source) && source[index] == '*' && source[index+1] == '/' {
					depth--
					advance(source[index])
					advance(source[index+1])
					index += 2
					continue
				}
				advance(source[index])
				index++
			}
			if depth != 0 || line-startLine > 16384 {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			continue
		}
		if b == '"' {
			startLine, startColumn, raw := line, column, index+2 < len(source) && bytes.Equal(source[index:index+3], []byte("\"\"\""))
			quote := 1
			if raw {
				quote = 3
			}
			for n := 0; n < quote; n++ {
				advance(source[index])
				index++
			}
			start := index
			dynamic := false
			closed := false
			for index < len(source) {
				if !raw && source[index] == '\\' {
					advance(source[index])
					index++
					if index >= len(source) {
						break
					}
					advance(source[index])
					index++
					continue
				}
				if source[index] == '$' && index+1 < len(source) && (source[index+1] == '{' || isIdentStart(source[index+1])) {
					dynamic = true
				}
				if (raw && index+2 < len(source) && bytes.Equal(source[index:index+3], []byte("\"\"\""))) || (!raw && source[index] == '"') {
					text := string(source[start:index])
					for n := 0; n < quote; n++ {
						advance(source[index])
						index++
					}
					result = append(result, token{text: text, line: startLine, column: startColumn, endLine: line, endColumn: column, quoted: true, dynamic: dynamic})
					closed = true
					break
				}
				advance(source[index])
				index++
			}
			if !closed {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			continue
		}
		if isIdentStart(b) {
			start, sl, sc := index, line, column
			for index < len(source) && isIdentPart(source[index]) {
				advance(source[index])
				index++
			}
			result = append(result, token{text: string(source[start:index]), line: sl, column: sc, endLine: line, endColumn: column})
			continue
		}
		if b >= '0' && b <= '9' {
			start, sl, sc := index, line, column
			for index < len(source) && ((source[index] >= '0' && source[index] <= '9') || source[index] == '_') {
				advance(source[index])
				index++
			}
			result = append(result, token{text: string(source[start:index]), line: sl, column: sc, endLine: line, endColumn: column})
			continue
		}
		startLine, startColumn := line, column
		advance(b)
		index++
		result = append(result, token{text: string(b), line: startLine, column: startColumn, endLine: line, endColumn: column})
	}
	return result, ""
}
func isIdentStart(b byte) bool { return b == '_' || unicode.IsLetter(rune(b)) }
func isIdentPart(b byte) bool  { return isIdentStart(b) || (b >= '0' && b <= '9') }
func isIdentifier(value string) bool {
	if value == "" || !isIdentStart(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		if !isIdentPart(value[index]) {
			return false
		}
	}
	return !map[string]bool{"class": true, "fun": true, "val": true, "var": true, "package": true, "import": true}[value]
}

func kotlinFacts(request Request, input Input, source []byte) ([]Fact, string) {
	if !strings.HasSuffix(input.Path, ".kt") {
		return nil, "UNSUPPORTED_SCHEMA"
	}
	tokens, reason := kotlinTokens(source)
	if reason != "" {
		return nil, reason
	}
	output := make([]Fact, 0, 64)
	add := func(at token, kind, subject, predicate, value string) {
		output = append(output, factAt(request, input, at.line, at.column, at.endLine, at.endColumn, kind, subject, predicate, value))
	}
	for index := 0; index < len(tokens); index++ {
		current := tokens[index]
		if current.text == "package" || current.text == "import" {
			end := index + 1
			var pieces []string
			for end < len(tokens) && tokens[end].text != "\n" && tokens[end].text != ";" {
				if tokens[end].quoted || (!isIdentifier(tokens[end].text) && tokens[end].text != "." && tokens[end].text != "as") {
					return nil, "UNSUPPORTED_SCHEMA"
				}
				pieces = append(pieces, tokens[end].text)
				end++
			}
			value := strings.Join(pieces, "")
			if !qualified(value) {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			valueToken := joinedToken(tokens, index+1, end, value)
			if current.text == "package" {
				add(valueToken, "kotlin.package", input.Path, "declares-package", value)
			} else {
				add(valueToken, "kotlin.import.static", input.Path, "imports", value)
			}
			continue
		}
		if current.text == "@" && index+1 < len(tokens) && isIdentifier(tokens[index+1].text) {
			add(tokens[index+1], "kotlin.annotation", input.Path, "uses-annotation", tokens[index+1].text)
			continue
		}
		if current.text == "class" || current.text == "interface" || current.text == "object" || current.text == "fun" || current.text == "val" || current.text == "var" {
			for next := index + 1; next < len(tokens) && next < index+12; next++ {
				if isIdentifier(tokens[next].text) {
					add(tokens[next], "kotlin.declaration", input.Path, "declares-"+current.text, tokens[next].text)
					break
				}
			}
			continue
		}
		if current.text == "context" && index+1 < len(tokens) && tokens[index+1].text == "(" {
			add(current, "kotlin.context.receiver", input.Path, "declares-context-receiver", "present")
			continue
		}
		if isIdentifier(current.text) && index+1 < len(tokens) && tokens[index+1].text == "<" {
			add(current, "kotlin.type.generic", input.Path, "uses-generic", current.text)
		}
		if isIdentifier(current.text) && index+1 < len(tokens) && tokens[index+1].text == "?" {
			add(current, "kotlin.type.nullable", input.Path, "uses-nullable", current.text)
		}
		if isIdentifier(current.text) && index+1 < len(tokens) && tokens[index+1].text == "(" && !map[string]bool{"if": true, "for": true, "while": true, "when": true, "fun": true, "context": true}[current.text] {
			add(current, "kotlin.call.static", input.Path, "calls", current.text)
		}
	}
	if len(output) == 0 {
		return nil, "EXACT_BINDING_UNAVAILABLE"
	}
	return output, ""
}

func gradleFacts(request Request, input Input, source []byte) ([]Fact, string) {
	tokens, reason := kotlinTokens(source)
	if reason != "" {
		return nil, reason
	}
	output := make([]Fact, 0, 64)
	add := func(at token, kind, subject, predicate, value string) {
		output = append(output, factAt(request, input, at.line, at.column, at.endLine, at.endColumn, kind, subject, predicate, value))
	}
	for index := 0; index < len(tokens); index++ {
		current := tokens[index]
		if current.text == "id" && index+4 < len(tokens) && tokens[index+1].text == "(" && tokens[index+2].quoted && tokens[index+3].text == ")" && tokens[index+4].text == "version" && index+5 < len(tokens) && tokens[index+5].quoted {
			pluginID, reason := literalText(tokens[index+2])
			if reason != "" {
				return nil, reason
			}
			version, reason := literalText(tokens[index+5])
			if reason != "" {
				return nil, reason
			}
			add(claimedToken(tokens[index+5]), "android.gradle.plugin", pluginID, "pins-version", version)
			continue
		}
		if current.text == "alias" && index+2 < len(tokens) && tokens[index+1].text == "(" {
			if reference, ok := gradleCatalogReference(tokens, index+2); ok {
				add(reference, "android.gradle.plugin.alias", input.Path, "uses-version-catalog", reference.text)
			}
		}
		if input.Family == "android.gradle.settings" && (current.text == "include" || current.text == "includeBuild") && index+2 < len(tokens) && tokens[index+1].text == "(" && tokens[index+2].quoted {
			value, reason := literalText(tokens[index+2])
			if reason != "" {
				return nil, reason
			}
			add(claimedToken(tokens[index+2]), "android.gradle.project", input.Path, current.text, value)
		}
		if current.text == "namespace" || current.text == "compileSdk" || current.text == "minSdk" || current.text == "targetSdk" {
			if index+2 < len(tokens) && (tokens[index+1].text == "=" || tokens[index+1].text == "(") {
				value := tokens[index+2]
				if value.quoted || isIdentifier(value.text) || (value.text >= "0" && value.text <= "9") {
					text, reason := literalText(value)
					if reason != "" {
						return nil, reason
					}
					add(claimedToken(value), "android.gradle.android", input.Path, "sets-"+current.text, text)
				}
			}
		}
		if isDependency(current.text) && index+2 < len(tokens) && tokens[index+1].text == "(" {
			value := tokens[index+2]
			if value.quoted {
				text, reason := literalText(value)
				if reason != "" {
					return nil, reason
				}
				add(claimedToken(value), "android.gradle.dependency", input.Path, current.text, text)
			} else if reference, ok := gradleDependencyReference(tokens, index+2); ok {
				add(reference, "android.gradle.dependency.alias", input.Path, current.text, reference.text)
				add(reference, "android.gradle.dependency", input.Path, current.text, reference.text)
			}
		}
		if (current.text == "srcDir" || current.text == "srcDirs") && index+2 < len(tokens) && tokens[index+1].text == "(" && tokens[index+2].quoted {
			text, reason := literalText(tokens[index+2])
			if reason != "" {
				return nil, reason
			}
			add(claimedToken(tokens[index+2]), "android.gradle.source-set", input.Path, "uses-source-directory", text)
		}
	}
	if len(output) == 0 {
		return nil, "EXACT_BINDING_UNAVAILABLE"
	}
	return output, ""
}

func joinedToken(tokens []token, start, end int, value string) token {
	result := tokens[start]
	result.text = value
	result.endLine, result.endColumn = tokens[end-1].endLine, tokens[end-1].endColumn
	return result
}

func claimedToken(value token) token {
	if !value.quoted {
		return value
	}
	value.column++
	value.endColumn--
	return value
}

func gradleDependencyReference(tokens []token, start int) (token, bool) {
	if reference, ok := gradleCatalogReference(tokens, start); ok {
		return reference, true
	}
	if start+2 < len(tokens) && tokens[start].text == "platform" && tokens[start+1].text == "(" {
		return gradleCatalogReference(tokens, start+2)
	}
	return token{}, false
}

// gradleCatalogReference retains the complete static dotted version-catalog
// coordinate (for example libs.androidx.activity.compose), including the
// source span from the initial libs token to the final identifier.
func gradleCatalogReference(tokens []token, start int) (token, bool) {
	if start >= len(tokens) || tokens[start].text != "libs" {
		return token{}, false
	}
	reference := tokens[start]
	for index := start + 1; index+1 < len(tokens) && tokens[index].text == "." && isIdentifier(tokens[index+1].text); index += 2 {
		reference.text += "." + tokens[index+1].text
		reference.endLine, reference.endColumn = tokens[index+1].endLine, tokens[index+1].endColumn
	}
	return reference, reference.text != "libs"
}
func isDependency(value string) bool {
	switch value {
	case "implementation", "api", "compileOnly", "runtimeOnly", "debugImplementation", "testImplementation", "androidTestImplementation", "kapt", "ksp":
		return true
	default:
		return false
	}
}
func propertiesFacts(request Request, input Input, content []byte) ([]Fact, string) {
	lines := bytes.Split(content, []byte("\n"))
	output := make([]Fact, 0, 4)
	for lineIndex, line := range lines {
		pair := bytes.SplitN(line, []byte("="), 2)
		if len(pair) != 2 {
			continue
		}
		key, value := string(pair[0]), string(pair[1])
		switch key {
		case "distributionUrl":
			if strings.Contains(value, "gradle-9.6.1-bin.zip") {
				output = append(output, factFromLiteral(request, input, lineIndex+1, line, []byte("9.6.1"), "gradle.distribution", "gradle", "pins-version", "9.6.1"))
			}
		case "versionName":
			output = append(output, factFromLiteral(request, input, lineIndex+1, line, pair[1], "android.project.version", "beamfall-android", "pins-version", value))
		case "versionCode":
			output = append(output, factFromLiteral(request, input, lineIndex+1, line, pair[1], "android.project.version-code", "beamfall-android", "pins-code", value))
		}
	}
	if len(output) == 0 {
		return nil, "EXACT_BINDING_UNAVAILABLE"
	}
	return output, ""
}

// catalogSection closes the `gradle/libs.versions.toml` table set. KA-V0's
// closed fact matrix carries a tuple only for `versions` and `plugins` rows;
// `libraries` and `bundles` are part of the KA-V0-003 pinned projection, so
// they are admitted against their own closed row form and are deliberately
// fact-free. Every other table name is an unsupported input schema.
func catalogSection(name string) bool {
	switch name {
	case "versions", "libraries", "plugins", "bundles":
		return true
	default:
		return false
	}
}

// catalogRow classifies one closed catalog row. An empty kind is an admitted
// row that carries no tuple; ok=false is a row that is not the closed form for
// its table.
func catalogRow(section, value string) (kind, predicate, literal string, ok bool) {
	switch section {
	case "versions":
		if !quotedLiteral(value) {
			return "", "", "", false
		}
		return "android.version.catalog", "pins-version", strings.Trim(value, "\""), true
	case "plugins":
		idIndex := strings.Index(value, "id = \"")
		if idIndex < 0 {
			return "", "", "", false
		}
		rest := value[idIndex+len("id = \""):]
		end := strings.Index(rest, "\"")
		if end < 0 {
			return "", "", "", false
		}
		return "android.version.catalog.plugin", "declares-plugin", rest[:end], true
	case "libraries":
		return "", "", "", strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}")
	case "bundles":
		return "", "", "", value == "[" || (strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"))
	default:
		return "", "", "", false
	}
}

func quotedLiteral(value string) bool {
	return len(value) > 1 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")
}

// catalogBundleElement admits one element line of the multi-line bundle array.
func catalogBundleElement(text string) bool {
	if !strings.HasPrefix(text, "\"") {
		return false
	}
	return strings.HasSuffix(text, "\"") || strings.HasSuffix(text, "\",")
}

func catalogFacts(request Request, input Input, content []byte) ([]Fact, string) {
	lines := bytes.Split(content, []byte("\n"))
	section := ""
	inBundle := false
	output := make([]Fact, 0, 32)
	for index, line := range lines {
		text := strings.TrimSpace(string(line))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if inBundle {
			if text == "]" {
				inBundle = false
				continue
			}
			if !catalogBundleElement(text) {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			continue
		}
		if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
			section = text[1 : len(text)-1]
			if !catalogSection(section) {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			continue
		}
		if section == "" {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		pieces := strings.SplitN(text, "=", 2)
		if len(pieces) != 2 {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		key, value := strings.TrimSpace(pieces[0]), strings.TrimSpace(pieces[1])
		kind, predicate, literal, ok := catalogRow(section, value)
		if !ok {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		inBundle = value == "["
		if kind == "" {
			continue
		}
		output = append(output, factFromLiteral(request, input, index+1, line, []byte(literal), kind, key, predicate, literal))
	}
	if inBundle {
		return nil, "UNSUPPORTED_SCHEMA"
	}
	if len(output) == 0 {
		return nil, "EXACT_BINDING_UNAVAILABLE"
	}
	return output, ""
}

// factFromLiteral locates literal within the row's value side only. literal is
// always carved out of the trimmed text after the row's first "=" (catalogRow
// never inspects the key), so searching the whole raw line let a key that
// happened to contain the same bytes -- e.g. `kotlin17 = "17"` -- win the
// match and report the key's column instead of the actual literal's.
func factFromLiteral(request Request, input Input, lineNumber int, line, literal []byte, kind, subject, predicate, value string) Fact {
	rhs, offset := line, 0
	if equals := bytes.IndexByte(line, '='); equals >= 0 {
		rhs, offset = line[equals+1:], equals+1
	}
	start := offset + bytes.Index(rhs, literal)
	return factAt(request, input, lineNumber, start+1, lineNumber, start+len(literal)+1, kind, subject, predicate, value)
}
