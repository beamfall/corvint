package toolgroup

import (
	"encoding/json"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

// The closed -32602 reason codes of a tools/call (MCPV0-952).
const (
	reasonUnknownParam    = "unknown-param"
	reasonMissingToolName = "missing-tool-name"
	reasonUnknownTool     = "unknown-tool"
	reasonUnknownArgument = "unknown-argument"
	reasonMissingArgument = "missing-argument"
	reasonInvalidType     = "invalid-type"
	reasonInvalidEnum     = "invalid-enum"
	reasonInvalidConst    = "invalid-const"
	reasonPattern         = "pattern-mismatch"
	reasonOutOfRange      = "out-of-range"
	reasonDuplicateItem   = "duplicate-item"
	// reasonInvalidArgument is a rule the advertised schema does not state, such
	// as a path that resolves outside the repository.
	reasonInvalidArgument = "invalid-argument"
)

// ListParamsRefusal is the -32602 of a tools/list request whose params are not
// `_meta` and an empty cursor (every server lists one page), or nil.
func ListParamsRefusal(params map[string]any) *protocol.RPCError {
	if params == nil {
		return protocol.InvalidParamsAt("Invalid params", "missing-param", "params")
	}
	for _, key := range slices.Sorted(maps.Keys(params)) {
		if key != "_meta" && key != "cursor" {
			return protocol.InvalidParamsAt("Invalid params", reasonUnknownParam, key)
		}
	}
	if cursor, present := params["cursor"]; present {
		if value, valid := cursor.(string); !valid || value != "" {
			return protocol.InvalidParamsAt("Invalid params", "invalid-cursor", "cursor")
		}
	}
	return nil
}

// WithInvalidParamsData gives a tools/call -32602 that carries no data the
// reason and offending field its params show against the listed tools
// (MCPV0-952). It reads only the request and the listed schemas, so it never
// changes which calls are refused, and every server that lists the same tools
// answers the same data (MMR-V0-013).
func WithInvalidParamsData(tools func() []bridge.ToolDescriptor, params map[string]any, failure *protocol.RPCError) *protocol.RPCError {
	if failure == nil || failure.Code != protocol.CodeInvalidParams || failure.Data != nil {
		return failure
	}
	failure.Data = invalidParamsData(tools(), params)
	return failure
}

func invalidParamsData(tools []bridge.ToolDescriptor, params map[string]any) protocol.InvalidParamsData {
	for _, key := range slices.Sorted(maps.Keys(params)) {
		if key != "_meta" && key != "name" && key != "arguments" {
			return protocol.InvalidParamsData{Reason: reasonUnknownParam, Field: key, Allowed: []any{"_meta", "arguments", "name"}}
		}
	}
	name, _ := params["name"].(string)
	if name == "" {
		return protocol.InvalidParamsData{Reason: reasonMissingToolName, Field: "name"}
	}
	arguments := map[string]any{}
	if raw, present := params["arguments"]; present {
		object, isObject := raw.(map[string]any)
		if !isObject {
			return protocol.InvalidParamsData{Reason: reasonInvalidType, Field: "arguments", Rule: "object"}
		}
		arguments = object
	}
	var names []string
	for _, tool := range tools {
		if tool.Name == name {
			if data, found := diagnose(tool.InputSchema, arguments, "arguments"); found {
				return data
			}
			return protocol.InvalidParamsData{Reason: reasonInvalidArgument, Field: "arguments"}
		}
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return protocol.InvalidParamsData{Reason: reasonUnknownTool, Field: "name", Allowed: stringsAsAny(names)}
}

// diagnose returns the first rule of schema that value breaks, in a fixed
// order: type, const and enum, bounds, pattern, then object members in sorted
// order and array items in index order.
func diagnose(schema map[string]any, value any, field string) (protocol.InvalidParamsData, bool) {
	if branches, ok := schema["oneOf"].([]any); ok {
		var first *protocol.InvalidParamsData
		for _, branch := range branches {
			branchSchema, _ := branch.(map[string]any)
			data, found := diagnose(branchSchema, value, field)
			if !found {
				return protocol.InvalidParamsData{}, false
			}
			if first == nil {
				first = &data
			}
		}
		if first != nil {
			return *first, true
		}
	}
	if kind, ok := schema["type"].(string); ok && !hasType(value, kind) {
		return protocol.InvalidParamsData{Reason: reasonInvalidType, Field: field, Rule: kind}, true
	}
	if constant, ok := schema["const"]; ok && !sameJSON(constant, value) {
		return protocol.InvalidParamsData{Reason: reasonInvalidConst, Field: field, Allowed: []any{constant}}, true
	}
	if enum, ok := schema["enum"].([]any); ok && !slices.ContainsFunc(enum, func(allowed any) bool { return sameJSON(allowed, value) }) {
		return protocol.InvalidParamsData{Reason: reasonInvalidEnum, Field: field, Allowed: enum}, true
	}
	switch typed := value.(type) {
	case string:
		length := float64(utf8.RuneCountInString(typed))
		if rule, broken := bounds(schema, "minLength", "maxLength", length); broken {
			return protocol.InvalidParamsData{Reason: reasonOutOfRange, Field: field, Rule: rule}, true
		}
		if pattern, ok := schema["pattern"].(string); ok {
			if compiled, err := regexp.Compile(pattern); err == nil && !compiled.MatchString(typed) {
				return protocol.InvalidParamsData{Reason: reasonPattern, Field: field, Rule: pattern}, true
			}
		}
	case json.Number, float64:
		number, _ := numberOf(typed)
		if rule, broken := bounds(schema, "minimum", "maximum", number); broken {
			return protocol.InvalidParamsData{Reason: reasonOutOfRange, Field: field, Rule: rule}, true
		}
	case []any:
		if rule, broken := bounds(schema, "minItems", "maxItems", float64(len(typed))); broken {
			return protocol.InvalidParamsData{Reason: reasonOutOfRange, Field: field, Rule: rule}, true
		}
		for index, item := range typed {
			itemField := field + "[" + strconv.Itoa(index) + "]"
			if unique, _ := schema["uniqueItems"].(bool); unique && slices.ContainsFunc(typed[:index], func(earlier any) bool { return sameJSON(earlier, item) }) {
				return protocol.InvalidParamsData{Reason: reasonDuplicateItem, Field: itemField}, true
			}
			if items, ok := schema["items"].(map[string]any); ok {
				if data, found := diagnose(items, item, itemField); found {
					return data, true
				}
			}
		}
	case map[string]any:
		properties, _ := schema["properties"].(map[string]any)
		if schema["additionalProperties"] == false {
			allowed := slices.Sorted(maps.Keys(properties))
			for _, key := range slices.Sorted(maps.Keys(typed)) {
				if _, declared := properties[key]; !declared {
					return protocol.InvalidParamsData{Reason: reasonUnknownArgument, Field: field + "." + key, Allowed: stringsAsAny(allowed)}, true
				}
			}
		}
		required, _ := schema["required"].([]any)
		for _, key := range required {
			if name, _ := key.(string); name != "" {
				if _, present := typed[name]; !present {
					return protocol.InvalidParamsData{Reason: reasonMissingArgument, Field: field + "." + name}, true
				}
			}
		}
		for _, key := range slices.Sorted(maps.Keys(typed)) {
			if property, ok := properties[key].(map[string]any); ok {
				if data, found := diagnose(property, typed[key], field+"."+key); found {
					return data, true
				}
			}
		}
	}
	return protocol.InvalidParamsData{}, false
}

// bounds reports the first of the inclusive lower and upper bound keywords
// that value breaks, as `minimum N` or `maximum N`.
func bounds(schema map[string]any, lower, upper string, value float64) (string, bool) {
	if limit, ok := numberOf(schema[lower]); ok && value < limit {
		return lower + " " + strconv.FormatFloat(limit, 'f', -1, 64), true
	}
	if limit, ok := numberOf(schema[upper]); ok && value > limit {
		return upper + " " + strconv.FormatFloat(limit, 'f', -1, 64), true
	}
	return "", false
}

func numberOf(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	}
	return 0, false
}

func hasType(value any, kind string) bool {
	switch kind {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "number":
		_, ok := numberOf(value)
		return ok
	case "integer":
		number, ok := numberOf(value)
		if typed, isNumber := value.(json.Number); isNumber {
			_, err := typed.Int64()
			return err == nil
		}
		return ok && number == math.Trunc(number)
	}
	return true
}

func sameJSON(left, right any) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftBytes) == string(rightBytes)
}

func stringsAsAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}
