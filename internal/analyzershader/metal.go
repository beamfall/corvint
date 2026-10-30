package analyzershader

import (
	"bytes"
	"strings"
)

func parseMetal(request Request, input Input, source []byte, profile exactProfile, facts *factAccumulator) string {
	clean, reason := sanitize(source)
	if reason != "" {
		return reason
	}
	factory, reason := newFactFactory(request, input, source)
	if reason != "" {
		return reason
	}
	if reason := eachLine(clean, func(start, end int) string {
		if !bytes.HasPrefix(bytes.TrimSpace(clean[start:end]), []byte{'#'}) {
			return ""
		}
		fields := strings.Fields(string(source[start:end]))
		if len(fields) != 2 || fields[0] != "#include" {
			return "UNSUPPORTED_SCHEMA"
		}
		if fields[1] == "<metal_stdlib>" {
			return ""
		}
		if !strings.HasPrefix(fields[1], "\"") || !strings.HasSuffix(fields[1], "\"") || len(fields[1]) < 3 {
			return "DYNAMIC_INPUT"
		}
		name := fields[1][1 : len(fields[1])-1]
		if credentialLike(name) {
			return "CREDENTIAL_INPUT"
		}
		if !logicalPath(name) {
			return "DYNAMIC_INPUT"
		}
		fact, failure := factory.sourceFact(start, end, "shader.metal.include", input.Path, "includes", name, request.CompilationUnitID)
		if failure != "" {
			return failure
		}
		return facts.retain(fact)
	}); reason != "" {
		return reason
	}
	blankDirectives(clean)
	values, reason := tokens(clean)
	if reason != "" {
		return reason
	}
	stageEntries := 0
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
		if depth == 0 && value.text == "using" {
			if !declarationBoundary(values, position) || position+3 >= len(values) || values[position+1].text != "namespace" || !glslIdentifier(values[position+2].text) || values[position+3].text != ";" {
				return "UNSUPPORTED_SCHEMA"
			}
			fact, failure := factory.sourceFact(value.start, values[position+3].end, "shader.metal.namespace", input.Path, "uses-namespace", values[position+2].text, request.CompilationUnitID)
			if failure != "" {
				return failure
			}
			if failure = facts.retain(fact); failure != "" {
				return failure
			}
		}
		if depth == 0 {
			if stage, known := metalStage(value.text); known {
				if !declarationBoundary(values, position) {
					return "UNSUPPORTED_SCHEMA"
				}
				parameters, body, ok := metalFunctionBody(values, position)
				if !ok {
					return "UNSUPPORTED_SCHEMA"
				}
				if stage == profile.stage {
					stageEntries++
					fact, failure := factory.sourceFact(value.start, values[body].end, "shader.metal.entry", values[position+2].text, "declares-entry", stage, request.CompilationUnitID)
					if failure != "" {
						return failure
					}
					if failure = facts.retain(fact); failure != "" {
						return failure
					}
					if failure = retainMetalAttributes(values[position+3:parameters], factory, facts); failure != "" {
						return failure
					}
				}
			}
		}
	}
	if stageEntries == 0 {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	return ""
}

func metalStage(value string) (string, bool) {
	switch value {
	case "vertex", "fragment":
		return value, true
	case "kernel":
		return "compute", true
	default:
		return "", false
	}
}

// metalFunctionBody accepts only a complete stage function declaration. A
// prototype, field declaration, or attribute-like adjacency is terminal.
func metalFunctionBody(values []token, start int) (int, int, bool) {
	if start+3 >= len(values) || !glslIdentifier(values[start+1].text) || !glslIdentifier(values[start+2].text) || values[start+3].text != "(" {
		return 0, 0, false
	}
	close, ok := matching(values, start+3, "(", ")")
	if !ok || close+1 >= len(values) || values[close+1].text != "{" {
		return 0, 0, false
	}
	return close, close + 1, true
}

func retainMetalAttributes(values []token, factory factFactory, facts *factAccumulator) string {
	for position := 0; position < len(values); position++ {
		if values[position].text != "[" || position+1 == len(values) || values[position+1].text != "[" {
			continue
		}
		end, attribute, ok := metalAttribute(values, position)
		if !ok {
			return "UNSUPPORTED_SCHEMA"
		}
		if attribute != "buffer" && attribute != "texture" && attribute != "sampler" && attribute != "stage_in" {
			position = end
			continue
		}
		fact, reason := factory.sourceFact(values[position].start, values[end].end, "shader.metal.attribute", factory.input.Path, "uses-attribute", attribute, factory.request.CompilationUnitID)
		if reason != "" {
			return reason
		}
		if reason = facts.retain(fact); reason != "" {
			return reason
		}
		position = end
	}
	return ""
}

func metalAttribute(values []token, start int) (int, string, bool) {
	if start+3 >= len(values) || values[start].text != "[" || values[start+1].text != "[" || !glslIdentifier(values[start+2].text) {
		return 0, "", false
	}
	attribute := values[start+2].text
	if attribute == "stage_in" || attribute == "vertex_id" || attribute == "position" || attribute == "thread_position_in_grid" || attribute == "thread_position_in_threadgroup" {
		if start+4 < len(values) && values[start+3].text == "]" && values[start+4].text == "]" {
			return start + 4, attribute, true
		}
		return 0, "", false
	}
	if attribute != "buffer" && attribute != "texture" && attribute != "sampler" {
		return 0, "", false
	}
	if start+7 >= len(values) || values[start+3].text != "(" || !numericLiteral(values[start+4].text) || values[start+5].text != ")" || values[start+6].text != "]" || values[start+7].text != "]" {
		return 0, "", false
	}
	return start + 7, attribute, true
}
