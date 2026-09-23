package mcp20260728

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"testing"
)

// officialSchemaEnv names a local copy of the pinned official 2026-07-28 schema.
// The default run stays offline and skips; supplying the file executes the
// schema against live corvint-mcp traffic after its digest matches the pin.
const (
	officialSchemaEnv    = "CORVINT_MCP_OFFICIAL_SCHEMA"
	officialSchemaSHA256 = "ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203"
)

func TestServerTrafficMatchesOfficialSchema(t *testing.T) {
	path := os.Getenv(officialSchemaEnv)
	if path == "" {
		t.Skip(officialSchemaEnv + " unset: official-schema validation NOT_RUN")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if got := hex.EncodeToString(digest[:]); got != officialSchemaSHA256 {
		t.Fatalf("official schema sha256=%s want %s", got, officialSchemaSHA256)
	}
	schema := officialSchema{defs: object(t, object(t, decodeNumbers(t, raw))["$defs"])}
	root := fixtureRepository(t)
	base, target := cemFixture(t, root, ".corvint/change.cem.json", "pkg/value.go", 1)
	client := startServer(t, root)
	defer client.close(t)
	exchanges := []struct {
		requestDef, responseDef, method string
		params                          map[string]any
	}{
		{"DiscoverRequest", "DiscoverResultResponse", "server/discover", map[string]any{"_meta": requestMeta()}},
		{"ListToolsRequest", "ListToolsResultResponse", "tools/list", map[string]any{"_meta": requestMeta()}},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.status", "arguments": map[string]any{}}},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.query", "arguments": map[string]any{"task": "Identify the active work queue"}}},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.impact", "arguments": map[string]any{"paths": []any{"pkg/value.go"}}}},
		{"CallToolRequest", "JSONRPCErrorResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.impact", "arguments": map[string]any{"paths": []any{"../escape.go"}}}},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.context", "arguments": map[string]any{"task": "change the fixture Value", "subject": "pkg/value.go"}}},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.cem.report", "arguments": map[string]any{"map": ".corvint/change.cem.json", "expectedBase": base, "target": target}}},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.cem.report", "arguments": map[string]any{"map": "missing.cem.json", "expectedBase": base, "target": target}}},
		{"CallToolRequest", "JSONRPCErrorResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.cem.report", "arguments": map[string]any{"map": "../escape.cem.json", "expectedBase": base, "target": target}}},
		{"JSONRPCRequest", "JSONRPCErrorResponse", "corvint/unknown", map[string]any{"_meta": requestMeta()}},
	}
	for index, exchange := range exchanges {
		id := 300 + index
		sent := decodeNumbers(t, mustJSON(t, request(id, exchange.method, exchange.params)))
		if err := schema.check(map[string]any{"$ref": "#/$defs/" + exchange.requestDef}, sent, exchange.requestDef); err != nil {
			t.Fatalf("request %d does not match %s: %v", id, exchange.requestDef, err)
		}
		response := client.call(t, id, exchange.method, exchange.params)
		if err := schema.check(map[string]any{"$ref": "#/$defs/" + exchange.responseDef}, response, exchange.responseDef); err != nil {
			t.Fatalf("response %d does not match %s: %v\n%s", id, exchange.responseDef, err, canonicalJSON(response))
		}
	}
	forged := map[string]any{"jsonrpc": "1.0", "id": json.Number("1"), "result": map[string]any{}}
	if schema.check(map[string]any{"$ref": "#/$defs/DiscoverResultResponse"}, forged, "forged") == nil {
		t.Fatal("official-schema checker accepted a forged response")
	}
}

// officialSchema checks the JSON Schema 2020-12 keywords the pinned schema
// uses. An unknown keyword is an error, so a schema revision that needs more
// cannot pass by being ignored; format stays an annotation, as 2020-12 defaults.
type officialSchema struct{ defs map[string]any }

// schemaKeywords is filled in init because its rules recurse through check.
var schemaKeywords map[string]func(officialSchema, any, any, string) error

func init() {
	schemaKeywords = map[string]func(officialSchema, any, any, string) error{
		"$schema": skipAnnotation, "description": skipAnnotation, "format": skipAnnotation,
		"title": skipAnnotation, "default": skipAnnotation, "examples": skipAnnotation,
		"$ref":                 officialSchema.checkRef,
		"type":                 checkType,
		"const":                checkConst,
		"enum":                 checkEnum,
		"properties":           officialSchema.checkProperties,
		"required":             checkRequired,
		"additionalProperties": officialSchema.checkAdditional,
		"items":                officialSchema.checkItems,
		"maxItems":             checkMaxItems,
		"minimum":              checkMinimum,
		"maximum":              checkMaximum,
		"anyOf":                officialSchema.checkAnyOf,
		"allOf":                officialSchema.checkAllOf,
	}
}

func (schema officialSchema) check(node, value any, at string) error {
	if flag, isBool := node.(bool); isBool {
		if !flag {
			return fmt.Errorf("%s: false schema", at)
		}
		return nil
	}
	body, isObject := node.(map[string]any)
	if !isObject {
		return fmt.Errorf("%s: schema node is %T", at, node)
	}
	keywords := make([]string, 0, len(body))
	for keyword := range body {
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)
	for _, keyword := range keywords {
		rule, known := schemaKeywords[keyword]
		if !known {
			return fmt.Errorf("%s: unsupported schema keyword %q", at, keyword)
		}
		argument := body[keyword]
		if keyword == "additionalProperties" {
			argument = []any{body["properties"], argument}
		}
		if err := rule(schema, argument, value, at); err != nil {
			return err
		}
	}
	return nil
}

func skipAnnotation(officialSchema, any, any, string) error { return nil }

func (schema officialSchema) checkRef(argument, value any, at string) error {
	name, local := strings.CutPrefix(fmt.Sprint(argument), "#/$defs/")
	target, defined := schema.defs[name]
	if !local || !defined {
		return fmt.Errorf("%s: unresolved $ref %v", at, argument)
	}
	return schema.check(target, value, at+">"+name)
}

func checkType(_ officialSchema, argument, value any, at string) error {
	names, isList := argument.([]any)
	if !isList {
		names = []any{argument}
	}
	for _, name := range names {
		if matchesType(fmt.Sprint(name), value) {
			return nil
		}
	}
	return fmt.Errorf("%s: %T is not %v", at, value, argument)
}

func matchesType(name string, value any) bool {
	number, isNumber := value.(json.Number)
	integral := isNumber && isInteger(number)
	checks := map[string]bool{
		"object": isMap(value), "array": isList(value), "null": value == nil,
		"string": isString(value), "boolean": isBool(value),
		"number": isNumber, "integer": integral,
	}
	return checks[name]
}

func isMap(value any) bool    { _, ok := value.(map[string]any); return ok }
func isList(value any) bool   { _, ok := value.([]any); return ok }
func isString(value any) bool { _, ok := value.(string); return ok }
func isBool(value any) bool   { _, ok := value.(bool); return ok }

func isInteger(number json.Number) bool {
	parsed, ok := new(big.Float).SetString(number.String())
	return ok && parsed.IsInt()
}

func checkConst(_ officialSchema, argument, value any, at string) error {
	if !sameJSON(argument, value) {
		return fmt.Errorf("%s: %v is not const %v", at, value, argument)
	}
	return nil
}

func checkEnum(_ officialSchema, argument, value any, at string) error {
	options, _ := argument.([]any)
	for _, option := range options {
		if sameJSON(option, value) {
			return nil
		}
	}
	return fmt.Errorf("%s: %v is not in enum %v", at, value, argument)
}

func (schema officialSchema) checkProperties(argument, value any, at string) error {
	properties, _ := argument.(map[string]any)
	body, isObject := value.(map[string]any)
	if !isObject {
		return nil
	}
	for name, property := range properties {
		field, present := body[name]
		if !present {
			continue
		}
		if err := schema.check(property, field, at+"."+name); err != nil {
			return err
		}
	}
	return nil
}

func checkRequired(_ officialSchema, argument, value any, at string) error {
	names, _ := argument.([]any)
	body, isObject := value.(map[string]any)
	for _, name := range names {
		if _, present := body[fmt.Sprint(name)]; isObject && !present {
			return fmt.Errorf("%s: missing required %v", at, name)
		}
	}
	return nil
}

// checkAdditional receives [properties, additionalProperties].
func (schema officialSchema) checkAdditional(argument, value any, at string) error {
	pair := argument.([]any)
	declared, _ := pair[0].(map[string]any)
	body, isObject := value.(map[string]any)
	if !isObject {
		return nil
	}
	for name, field := range body {
		if _, known := declared[name]; known {
			continue
		}
		if err := schema.check(pair[1], field, at+"."+name); err != nil {
			return err
		}
	}
	return nil
}

func (schema officialSchema) checkItems(argument, value any, at string) error {
	elements, _ := value.([]any)
	for index, element := range elements {
		if err := schema.check(argument, element, fmt.Sprintf("%s[%d]", at, index)); err != nil {
			return err
		}
	}
	return nil
}

func checkMaxItems(_ officialSchema, argument, value any, at string) error {
	elements, isArray := value.([]any)
	limit, _ := argument.(json.Number).Int64()
	if isArray && int64(len(elements)) > limit {
		return fmt.Errorf("%s: %d items exceed maxItems %d", at, len(elements), limit)
	}
	return nil
}

func checkMinimum(_ officialSchema, argument, value any, at string) error {
	if compareNumbers(value, argument) < 0 {
		return fmt.Errorf("%s: %v below minimum %v", at, value, argument)
	}
	return nil
}

func checkMaximum(_ officialSchema, argument, value any, at string) error {
	if compareNumbers(value, argument) > 0 {
		return fmt.Errorf("%s: %v above maximum %v", at, value, argument)
	}
	return nil
}

// compareNumbers orders value against bound and reports 0 for a non-number
// value, which the numeric keywords ignore.
func compareNumbers(value, bound any) int {
	number, isNumber := value.(json.Number)
	if !isNumber {
		return 0
	}
	left, _ := new(big.Float).SetString(number.String())
	right, _ := new(big.Float).SetString(bound.(json.Number).String())
	return left.Cmp(right)
}

func (schema officialSchema) checkAnyOf(argument, value any, at string) error {
	branches, _ := argument.([]any)
	for _, branch := range branches {
		if schema.check(branch, value, at) == nil {
			return nil
		}
	}
	return fmt.Errorf("%s: no anyOf branch matches", at)
}

func (schema officialSchema) checkAllOf(argument, value any, at string) error {
	branches, _ := argument.([]any)
	for _, branch := range branches {
		if err := schema.check(branch, value, at); err != nil {
			return err
		}
	}
	return nil
}

func sameJSON(left, right any) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftBytes, rightBytes)
}

func decodeNumbers(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
