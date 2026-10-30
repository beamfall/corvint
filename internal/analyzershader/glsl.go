package analyzershader

import (
	"bytes"
	"strings"
)

// parseGLSL consumes every input with one forward line walk and one token walk.
// In particular, directive offsets and fact positions share sourceIndex; neither
// path rescans or copies a source prefix for each declaration.
func parseGLSL(request Request, input Input, source []byte, profile exactProfile, facts *factAccumulator) string {
	clean, reason := sanitize(source)
	if reason != "" {
		return reason
	}
	factory, reason := newFactFactory(request, input, source)
	if reason != "" {
		return reason
	}
	version := ""
	sawNonVersion := false
	macros := make(map[string]string)
	if reason := eachLine(clean, func(start, end int) string {
		trimmed := bytes.TrimSpace(clean[start:end])
		if len(trimmed) == 0 {
			return ""
		}
		if trimmed[0] != '#' {
			sawNonVersion = true
			return ""
		}
		if source[end-1] == '\\' {
			return "DYNAMIC_INPUT"
		}
		fields := strings.Fields(string(source[start:end]))
		if len(fields) < 2 {
			return "MALFORMED_INPUT"
		}
		switch fields[0] {
		case "#version":
			if sawNonVersion || version != "" || len(fields) != 3 || fields[2] != "es" {
				return "UNSUPPORTED_SCHEMA"
			}
			version = fields[1] + " " + fields[2]
			fact, failure := factory.sourceFact(start, end, "shader.glsl.version", input.Path, "declares-version", version, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			return facts.retain(fact)
		case "#include":
			sawNonVersion = true
			if len(fields) != 2 || !strings.HasPrefix(fields[1], "\"") || !strings.HasSuffix(fields[1], "\"") || len(fields[1]) < 3 {
				return "DYNAMIC_INPUT"
			}
			name := fields[1][1 : len(fields[1])-1]
			if credentialLike(name) {
				return "CREDENTIAL_INPUT"
			}
			if !logicalPath(name) {
				return "DYNAMIC_INPUT"
			}
			fact, failure := factory.sourceFact(start, end, "shader.glsl.include", input.Path, "includes", name, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			return facts.retain(fact)
		case "#define":
			sawNonVersion = true
			name, definition, failure := glslDefine(fields)
			if failure != "" {
				return failure
			}
			if prior, exists := macros[name]; exists {
				if prior == definition {
					return "DUPLICATE_VALUE"
				}
				return "CONFLICTING_VALUE"
			}
			macros[name] = definition
			fact, failure := factory.sourceFact(start, end, "shader.glsl.macro", input.Path, "defines", name, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			return facts.retain(fact)
		case "#if", "#ifdef", "#ifndef", "#elif", "#else", "#endif", "#pragma", "#extension":
			return "UNSUPPORTED_SCHEMA"
		default:
			return "UNKNOWN_FIELD"
		}
	}); reason != "" {
		return reason
	}
	if version != profile.sourceVersion {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	blankDirectives(clean)
	values, reason := tokens(clean)
	if reason != "" {
		return reason
	}
	entries := 0
	depth := 0
	for position := 0; position < len(values); position++ {
		value := values[position]
		if value.text == "{" {
			depth++
			continue
		}
		if value.text == "}" {
			depth--
			continue
		}
		if depth != 0 {
			continue
		}
		if value.text == "precision" {
			if !declarationBoundary(values, position) || position+3 >= len(values) || !precisionQualifier(values[position+1].text) || !glslIdentifier(values[position+2].text) || values[position+3].text != ";" {
				return "UNSUPPORTED_SCHEMA"
			}
			fact, failure := factory.sourceFact(value.start, values[position+3].end, "shader.glsl.precision", input.Path, "declares-precision", values[position+1].text+" "+values[position+2].text, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			if failure = facts.retain(fact); failure != "" {
				return failure
			}
		}
		if (value.text == "uniform" || value.text == "in" || value.text == "out") && (position == 0 || values[position-1].text != ")") {
			if !declarationBoundary(values, position) || position+3 >= len(values) || !glslIdentifier(values[position+1].text) || !glslIdentifier(values[position+2].text) || values[position+3].text != ";" {
				return "UNSUPPORTED_SCHEMA"
			}
			fact, failure := factory.sourceFact(value.start, values[position+3].end, "shader.glsl.interface", values[position+2].text, "declares-"+value.text, values[position+1].text, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			if failure = facts.retain(fact); failure != "" {
				return failure
			}
		}
		if value.text == "layout" {
			close, ok := matching(values, position+1, "(", ")")
			if !declarationBoundary(values, position) || !ok || close+4 >= len(values) || !layoutFields(values[position+2:close]) || !interfaceQualifier(values[close+1].text) || !glslIdentifier(values[close+2].text) || !glslIdentifier(values[close+3].text) || values[close+4].text != ";" {
				return "UNSUPPORTED_SCHEMA"
			}
			fact, failure := factory.sourceFact(value.start, values[close].end, "shader.glsl.layout", input.Path, "qualifies", values[close+1].text, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			if failure = facts.retain(fact); failure != "" {
				return failure
			}
			// A layout-qualified `in`/`out`/`uniform` declares the same interface
			// binding a plain one does; withholding shader.glsl.interface here left
			// the variable this layout qualifies unnamed anywhere in the output.
			interfaceFact, failure := factory.sourceFact(values[close+1].start, values[close+4].end, "shader.glsl.interface", values[close+3].text, "declares-"+values[close+1].text, values[close+2].text, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			if failure = facts.retain(interfaceFact); failure != "" {
				return failure
			}
		}
		if value.text == "void" && position+1 < len(values) && values[position+1].text == "main" {
			if !declarationBoundary(values, position) || position+4 >= len(values) || values[position+2].text != "(" || values[position+3].text != ")" || values[position+4].text != "{" {
				return "UNSUPPORTED_SCHEMA"
			}
			// GLSL has exactly one entry point per compilation unit; a second
			// `void main(){` is a duplicate-definition compile error, not a second
			// candidate entry.
			if entries > 0 {
				return "DUPLICATE_VALUE"
			}
			entries++
			fact, failure := factory.sourceFact(value.start, values[position+4].end, "shader.glsl.entry", input.Path, "declares-entry", profile.stage, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			if failure = facts.retain(fact); failure != "" {
				return failure
			}
		}
	}
	if entries == 0 {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	return ""
}

// glslDefine accepts only an object-like macro with no replacement or one
// identifier/decimal-integer replacement. Other preprocessing tokens are not
// parsed by this structural extractor and therefore fail closed before directive
// blanking.
func glslDefine(fields []string) (string, string, string) {
	if len(fields) < 2 || len(fields) > 3 || !glslIdentifier(fields[1]) {
		return "", "", "DYNAMIC_INPUT"
	}
	if len(fields) == 2 {
		return fields[1], "", ""
	}
	if !glslIdentifier(fields[2]) && !numericLiteral(fields[2]) {
		return "", "", "DYNAMIC_INPUT"
	}
	return fields[1], fields[2], ""
}

func eachLine(source []byte, visit func(start, end int) string) string {
	start := 0
	for start < len(source) {
		relative := bytes.IndexByte(source[start:], '\n')
		end := len(source)
		if relative >= 0 {
			end = start + relative
		}
		if reason := visit(start, end); reason != "" {
			return reason
		}
		if relative < 0 {
			return ""
		}
		start = end + 1
	}
	return ""
}

func matching(values []token, open int, left, right string) (int, bool) {
	if open >= len(values) || values[open].text != left {
		return 0, false
	}
	depth := 0
	for index := open; index < len(values); index++ {
		switch values[index].text {
		case left:
			depth++
		case right:
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func declarationBoundary(values []token, position int) bool {
	return position == 0 || values[position-1].text == ";" || values[position-1].text == "}"
}

func glslIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 || !(value[0] == '_' || value[0] >= 'A' && value[0] <= 'Z' || value[0] >= 'a' && value[0] <= 'z') {
		return false
	}
	for index := range value {
		if !identifierByte(value[index]) {
			return false
		}
	}
	return true
}

func precisionQualifier(value string) bool {
	return value == "highp" || value == "mediump" || value == "lowp"
}
func interfaceQualifier(value string) bool {
	return value == "in" || value == "out" || value == "uniform"
}
func layoutFields(values []token) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !(glslIdentifier(value.text) || numericLiteral(value.text) || value.text == "=" || value.text == ",") {
			return false
		}
	}
	return true
}
func numericLiteral(value string) bool {
	if len(value) == 0 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}
