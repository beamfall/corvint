package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// stableSchemaCopies are the two byte-identical cem/1.0 stable result schemas.
var stableSchemaCopies = []string{
	"protocol/cem-1.0/stable/RESULT.schema.json",
	"protocol/cem-1.0/stable/repository-envelope-packet/RESULT.schema.json",
}

// TestCEMV1622OpaqueReceiptResultValidates proves the complete verify result
// for the public neutral case with its runner receipt labelled opaque/0 (the
// label of an unadmitted fixture map) validates against both RESULT.schema.json
// copies, that the legacy-labelled result still validates, and that the
// pre-V1-2107 legacy const and an unknown label are refused (CEM-V1-621,
// CEM-V1-622). The committed schema's criterionBinding definition still lacks
// the CEM-V1-083 plain form these results carry; that gap (V1-2615) is pinned
// here and the rest of each result is validated with only that one definition
// overlaid (CEM-V1-622).
func TestCEMV1622OpaqueReceiptResultValidates(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	c := loadNeutralCase(t)
	repo, base, target := c.materialize(t)
	legacy := []byte(`"profile":"` + wire.StableRunnerReceiptProfile + `"`)
	if bytes.Count(c.raw, legacy) != 1 {
		t.Fatal("public case does not carry exactly one legacy runner label")
	}
	results := map[string]any{}
	for _, profile := range []string{wire.StableRunnerReceiptProfile, wire.StableOpaqueReceiptProfile} {
		raw := bytes.Replace(c.raw, legacy, []byte(`"profile":"`+profile+`"`), 1)
		result, exit := Stable(context.Background(), raw, StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root})
		if exit != 0 || result.Outcome != "ACCEPT" {
			t.Fatalf("Stable %s: exit=%d result=%s, want ACCEPT", profile, exit, stableResultDebug(result))
		}
		b, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		if got := receiptProfiles(v); !reflect.DeepEqual(got, []string{profile}) {
			t.Fatalf("result runner receipt profiles %v, want [%s]", got, profile)
		}
		results[profile] = v
	}
	top := repositoryRoot(t)
	for _, rel := range stableSchemaCopies {
		committed := loadStableSchema(t, filepath.Join(top, rel))
		for profile, v := range results {
			err := schemaValidate(committed, committed, v, "$")
			if err == nil || !strings.HasPrefix(err.Error(), "$.references.criterionBindings[0]: ") || !strings.Contains(err.Error(), "evidenceId") {
				t.Fatalf("%s on the %s verify result: %v, want only the pinned plain criterion binding gap", rel, profile, err)
			}
		}
		schema := withPlainCriterionBinding(loadStableSchema(t, filepath.Join(top, rel)))
		for profile, v := range results {
			if err := schemaValidate(schema, schema, v, "$"); err != nil {
				t.Fatalf("%s rejects the %s verify result: %v", rel, profile, err)
			}
		}
		unknown := withReceiptProfile(t, results[wire.StableOpaqueReceiptProfile], "opaque/1")
		if err := schemaValidate(schema, schema, unknown, "$"); err == nil || !strings.Contains(err.Error(), "runnerReceipts[0].profile") {
			t.Fatalf("%s on the unknown runner receipt label opaque/1: %v", rel, err)
		}
		// The sealed predecessor node admitted only the legacy label; it must
		// refuse the opaque/0 result, which is the defect V1-2107 fixed.
		legacySchema := withPlainCriterionBinding(loadStableSchema(t, filepath.Join(top, rel)))
		defs := legacySchema["$defs"].(map[string]any)
		props := defs["runnerReceipt"].(map[string]any)["properties"].(map[string]any)
		props["profile"] = map[string]any{"const": wire.StableRunnerReceiptProfile}
		if err := schemaValidate(legacySchema, legacySchema, results[wire.StableOpaqueReceiptProfile], "$"); err == nil || !strings.Contains(err.Error(), "runnerReceipts[0].profile") {
			t.Fatalf("the pre-V1-2107 profile const of %s on opaque/0: %v", rel, err)
		}
	}
}

// withPlainCriterionBinding overlays the schema's criterionBinding definition
// with anyOf the committed Tasks form and the CEM-V1-083 plain form (exactly
// id, criterionIndex, criterionSha256 and evidenceId), the one definition the
// committed schema does not yet carry.
func withPlainCriterionBinding(schema map[string]any) map[string]any {
	defs := schema["$defs"].(map[string]any)
	tasks := defs["criterionBinding"].(map[string]any)
	tasksProps := tasks["properties"].(map[string]any)
	defs["criterionBinding"] = map[string]any{"anyOf": []any{tasks, map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"id", "criterionIndex", "criterionSha256", "evidenceId"},
		"properties": map[string]any{
			"id":              tasksProps["id"],
			"criterionIndex":  tasksProps["criterionIndex"],
			"criterionSha256": tasksProps["criterionSha256"],
			"evidenceId":      map[string]any{"type": "string", "pattern": "^evidence:sha256:[0-9a-f]{64}$"},
		},
	}}}
	return schema
}

// TestStableSchemaValidatorRefusesUnknownKeywords keeps the test validator
// closed: a keyword it does not implement fails instead of passing silently.
func TestStableSchemaValidatorRefusesUnknownKeywords(t *testing.T) {
	t.Parallel()
	schema := map[string]any{"dependentRequired": map[string]any{}}
	if err := schemaValidate(schema, schema, map[string]any{}, "$"); err == nil || !strings.Contains(err.Error(), "unsupported schema keyword") {
		t.Fatalf("unknown keyword: %v", err)
	}
	for name, c := range map[string]struct {
		schema map[string]any
		value  any
	}{
		"type":        {map[string]any{"type": "integer"}, 1.5},
		"const":       {map[string]any{"const": "a"}, "b"},
		"pattern":     {map[string]any{"pattern": "^[0-9a-f]{2}$"}, "zz"},
		"maxLength":   {map[string]any{"maxLength": 1.0}, "ab"},
		"lookahead":   {map[string]any{"pattern": "^(?!/)(?!.*\\\\)[^/]+(?:/[^/]+)*$"}, "/abs"},
		"lookahead2":  {map[string]any{"pattern": "^(?!/)(?!.*\\\\)[^/]+(?:/[^/]+)*$"}, "a\\b"},
		"control":     {map[string]any{"pattern": "^[^\\u0000-\\u001f\\u007f]*$"}, "a\tb"},
		"uniqueItems": {map[string]any{"uniqueItems": true}, []any{"a", "a"}},
		"prefixItems": {map[string]any{"prefixItems": []any{map[string]any{"type": "string"}}}, []any{1.0}},
		"oneOf":       {map[string]any{"oneOf": []any{map[string]any{}, map[string]any{}}}, "x"},
		"not":         {map[string]any{"not": map[string]any{}}, "x"},
		"if/then":     {map[string]any{"if": map[string]any{"const": "x"}, "then": map[string]any{"type": "integer"}}, "x"},
		"required":    {map[string]any{"required": []any{"a"}}, map[string]any{}},
		"additional":  {map[string]any{"additionalProperties": false, "properties": map[string]any{}}, map[string]any{"a": 1.0}},
	} {
		if schemaValidate(c.schema, c.schema, c.value, "$") == nil {
			t.Fatalf("%s: the validator admitted an invalid value", name)
		}
	}
}

func loadStableSchema(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func receiptProfiles(result any) []string {
	refs, _ := result.(map[string]any)["references"].(map[string]any)
	receipts, _ := refs["runnerReceipts"].([]any)
	var out []string
	for _, r := range receipts {
		out = append(out, fmt.Sprint(r.(map[string]any)["profile"]))
	}
	return out
}

// withReceiptProfile returns a deep copy of result whose runner receipts carry
// profile.
func withReceiptProfile(t *testing.T, result any, profile string) any {
	t.Helper()
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	for _, r := range v.(map[string]any)["references"].(map[string]any)["runnerReceipts"].([]any) {
		r.(map[string]any)["profile"] = profile
	}
	return v
}

// schemaValidate checks the closed draft 2020-12 keyword subset the stable
// result schema uses and refuses any other keyword instead of ignoring it.
// root resolves local "#/$defs/NAME" references; keywords run in sorted order
// so the first reported failure is deterministic.
func schemaValidate(root, schema map[string]any, value any, at string) error {
	keys := make([]string, 0, len(schema))
	for key := range schema {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := schemaKeyword(root, schema, key, schema[key], value, at); err != nil {
			return err
		}
	}
	return nil
}

func schemaKeyword(root, schema map[string]any, key string, rule, value any, at string) error {
	obj, isObj := value.(map[string]any)
	arr, isArr := value.([]any)
	str, isStr := value.(string)
	num, isNum := value.(float64)
	switch key {
	case "$schema", "$id", "$defs", "title", "description", "then", "else":
		return nil
	case "$ref":
		ref, _ := rule.(string)
		name, ok := strings.CutPrefix(ref, "#/$defs/")
		def, found := root["$defs"].(map[string]any)[name].(map[string]any)
		if !ok || !found {
			return fmt.Errorf("%s: unsupported $ref %q", at, ref)
		}
		return schemaValidate(root, def, value, at)
	case "type":
		types, ok := rule.([]any)
		if !ok {
			types = []any{rule}
		}
		for _, want := range types {
			if schemaTypeOf(want.(string), value) {
				return nil
			}
		}
		return fmt.Errorf("%s: want type %v", at, rule)
	case "const":
		if !reflect.DeepEqual(rule, value) {
			return fmt.Errorf("%s: %v is not const %v", at, value, rule)
		}
	case "enum":
		for _, option := range rule.([]any) {
			if reflect.DeepEqual(option, value) {
				return nil
			}
		}
		return fmt.Errorf("%s: %v not in enum %v", at, value, rule)
	case "properties":
		if isObj {
			for name, member := range obj {
				if sub, declared := rule.(map[string]any)[name]; declared {
					if err := schemaValidate(root, sub.(map[string]any), member, at+"."+name); err != nil {
						return err
					}
				}
			}
		}
	case "required":
		if isObj {
			for _, name := range rule.([]any) {
				if _, present := obj[name.(string)]; !present {
					return fmt.Errorf("%s: missing %s", at, name)
				}
			}
		}
	case "additionalProperties":
		if rule != false {
			return fmt.Errorf("%s: unsupported additionalProperties %v", at, rule)
		}
		if isObj {
			declared, _ := schema["properties"].(map[string]any)
			for name := range obj {
				if _, ok := declared[name]; !ok {
					return fmt.Errorf("%s: undeclared member %s", at, name)
				}
			}
		}
	case "items", "prefixItems":
		if !isArr {
			return nil
		}
		prefix, _ := schema["prefixItems"].([]any)
		for i, item := range arr {
			var sub map[string]any
			switch {
			case key == "prefixItems" && i < len(prefix):
				sub = prefix[i].(map[string]any)
			case key == "items" && i >= len(prefix):
				sub = rule.(map[string]any)
			default:
				continue
			}
			if err := schemaValidate(root, sub, item, fmt.Sprintf("%s[%d]", at, i)); err != nil {
				return err
			}
		}
	case "minItems", "maxItems":
		if isArr && ((key == "minItems" && float64(len(arr)) < rule.(float64)) || (key == "maxItems" && float64(len(arr)) > rule.(float64))) {
			return fmt.Errorf("%s: %d items violates %s %v", at, len(arr), key, rule)
		}
	case "uniqueItems":
		if isArr && rule == true {
			for i := range arr {
				for j := i + 1; j < len(arr); j++ {
					if reflect.DeepEqual(arr[i], arr[j]) {
						return fmt.Errorf("%s: items %d and %d repeat", at, i, j)
					}
				}
			}
		}
	case "minLength", "maxLength":
		if isStr {
			n := float64(utf8.RuneCountInString(str))
			if (key == "minLength" && n < rule.(float64)) || (key == "maxLength" && n > rule.(float64)) {
				return fmt.Errorf("%s: length %v violates %s %v", at, n, key, rule)
			}
		}
	case "minimum", "maximum":
		if isNum && ((key == "minimum" && num < rule.(float64)) || (key == "maximum" && num > rule.(float64))) {
			return fmt.Errorf("%s: %v violates %s %v", at, num, key, rule)
		}
	case "pattern":
		match, err := schemaPattern(rule.(string))
		if err != nil {
			return fmt.Errorf("%s: unsupported pattern %q: %v", at, rule, err)
		}
		if isStr && !match(str) {
			return fmt.Errorf("%s: %q does not match %s", at, str, rule)
		}
	case "allOf", "anyOf", "oneOf":
		matched := 0
		var failures []string
		for _, option := range rule.([]any) {
			if err := schemaValidate(root, option.(map[string]any), value, at); err != nil {
				failures = append(failures, err.Error())
			} else {
				matched++
			}
		}
		options := len(rule.([]any))
		if (key == "allOf" && matched != options) || (key == "anyOf" && matched == 0) || (key == "oneOf" && matched != 1) {
			return fmt.Errorf("%s: %s matched %d of %d: %s", at, key, matched, options, strings.Join(failures, "; "))
		}
	case "not":
		if schemaValidate(root, rule.(map[string]any), value, at) == nil {
			return fmt.Errorf("%s: matches a not schema", at)
		}
	case "if":
		branch := "else"
		if schemaValidate(root, rule.(map[string]any), value, at) == nil {
			branch = "then"
		}
		if sub, ok := schema[branch].(map[string]any); ok {
			return schemaValidate(root, sub, value, at)
		}
	default:
		return fmt.Errorf("%s: unsupported schema keyword %s", at, key)
	}
	return nil
}

var schemaUnicodeEscape = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

// schemaPattern compiles an ECMA-262 pattern of the schema with RE2. RE2 has
// no lookahead, so leading negative lookaheads right after "^" (the only form
// the schema uses) are peeled off and each is checked as an anchored prefix
// that must not match; any other unsupported syntax is an error.
func schemaPattern(pattern string) (func(string) bool, error) {
	pattern = schemaUnicodeEscape.ReplaceAllString(pattern, `\x{$1}`)
	var refused []*regexp.Regexp
	if rest, ok := strings.CutPrefix(pattern, "^"); ok {
		for strings.HasPrefix(rest, "(?!") {
			depth, end := 0, -1
			for i := 0; i < len(rest) && end < 0; i++ {
				switch rest[i] {
				case '\\':
					i++
				case '(':
					depth++
				case ')':
					if depth--; depth == 0 {
						end = i
					}
				}
			}
			if end < 0 {
				return nil, fmt.Errorf("unbalanced lookahead")
			}
			re, err := regexp.Compile("^(?:" + rest[len("(?!"):end] + ")")
			if err != nil {
				return nil, err
			}
			refused = append(refused, re)
			rest = rest[end+1:]
		}
		pattern = "^" + rest
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return func(s string) bool {
		for _, r := range refused {
			if r.MatchString(s) {
				return false
			}
		}
		return re.MatchString(s)
	}, nil
}

func schemaTypeOf(want string, value any) bool {
	switch v := value.(type) {
	case nil:
		return want == "null"
	case bool:
		return want == "boolean"
	case string:
		return want == "string"
	case float64:
		return want == "number" || (want == "integer" && v == math.Trunc(v) && !math.IsInf(v, 0))
	case []any:
		return want == "array"
	case map[string]any:
		return want == "object"
	}
	return false
}
