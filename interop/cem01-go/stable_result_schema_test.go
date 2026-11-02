package main

// CEM-V1-843 and CEM-V1-844 portable conformance: the complete portable
// results of the public receipt-authentication case and of a sealed
// Tasks-bound S0E case validate against both committed RESULT.schema.json
// copies as they are, with a closed draft 2020-12 validator that refuses every
// keyword it does not implement (V1-2521, V1-2520, decision 0589). The
// validator is the portable twin of the one in internal/cem/verify.

import (
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
)

// portableSchemaCopies are the two byte-identical cem/1.0 stable result
// schemas, relative to the stable protocol root.
var portableSchemaCopies = []string{"RESULT.schema.json", "repository-envelope-packet/RESULT.schema.json"}

func loadPortableStableSchema(t *testing.T, rel string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(neutralStableRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func portableResultObject(t *testing.T, r stableResult) any {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestCEMV1843PortableReceiptAuthenticationResultsValidate runs the public
// receipt-authentication case (the VERIFIED ACCEPT, five axis cases and
// thirteen refusals) through both portable commands and validates every
// complete result against both committed schema copies; the sealed const
// NOT_OBSERVED predecessor at axes.authentication refuses the ACCEPT.
func TestCEMV1843PortableReceiptAuthenticationResultsValidate(t *testing.T) {
	c := loadReceiptAuthPortableCase(t)
	results := map[string]any{}
	run := func(name string, raw []byte, trusted string, want receiptAuthPortableMutation) {
		keys := c.trusted(t, trusted)
		canonical, exit := runStableCanonicalTrusted(context.Background(), c.repo, raw, c.base, c.target, c.dir, nil, keys)
		if exit != want.Exit || canonical.Outcome != want.Outcome || canonical.Axes["authentication"] != want.Authentication {
			b, _ := json.Marshal(canonical)
			t.Fatalf("%s canonical exit=%d result=%s, want %d %s authentication %s", name, exit, b, want.Exit, want.Outcome, want.Authentication)
		}
		results[name+"/canonical"] = portableResultObject(t, canonical)
		stable, exit := runStableTrusted(context.Background(), c.repo, raw, c.base, c.target, c.dir, keys)
		if exit != want.Exit || stable.Outcome != want.Outcome || stable.Axes["authentication"] != want.Authentication {
			b, _ := json.Marshal(stable)
			t.Fatalf("%s stable exit=%d result=%s, want %d %s authentication %s", name, exit, b, want.Exit, want.Outcome, want.Authentication)
		}
		results[name+"/stable"] = portableResultObject(t, stable)
	}
	run("accept", c.raw, c.expect.Accept.TrustedKeys, receiptAuthPortableMutation{Exit: c.expect.Accept.Exit, Outcome: "ACCEPT", Authentication: "VERIFIED"})
	seenAxes, seenIssues := map[string]bool{"VERIFIED": true}, map[string]bool{}
	for _, m := range append(append([]receiptAuthPortableMutation{}, c.expect.AxisCases...), c.expect.Refusals...) {
		run(m.Name, c.mutate(t, m.Set), m.TrustedKeys, m)
		seenAxes[m.Authentication] = true
		if m.IssueCode != "" {
			seenIssues[m.IssueCode] = true
		}
	}
	for _, want := range []string{"NOT_OBSERVED", "UNTRUSTED_KEY", "VERIFIED", "FAILED"} {
		if !seenAxes[want] {
			t.Fatalf("the public case never yields authentication %s", want)
		}
	}
	for _, want := range []string{"attestation-malformed", "attestation-key-mismatch", "attestation-subject-mismatch", "attestation-signature-invalid"} {
		if !seenIssues[want] {
			t.Fatalf("the public case never yields issue code %s", want)
		}
	}
	for _, rel := range portableSchemaCopies {
		schema := loadPortableStableSchema(t, rel)
		for name, v := range results {
			if err := portableSchemaValidate(schema, schema, v, "$"); err != nil {
				t.Fatalf("%s rejects the portable %s result: %v", rel, name, err)
			}
		}
		sealed := loadPortableStableSchema(t, rel)
		sealed["properties"].(map[string]any)["axes"].(map[string]any)["properties"].(map[string]any)["authentication"] = map[string]any{"const": "NOT_OBSERVED"}
		if err := portableSchemaValidate(sealed, sealed, results["accept/canonical"], "$"); err == nil || !strings.HasPrefix(err.Error(), "$.axes.authentication") {
			t.Fatalf("the sealed axes.authentication const of %s on the VERIFIED accept: %v", rel, err)
		}
	}
}

// TestCEMV1844PortableTasksBoundResultsValidate validates every sealed
// full-result case of the S0E packet and the live portable result of the
// Tasks-bound ledger-336-evidence case against both committed schema copies
// as they are, and the plain neutral-criterion ACCEPT line beside them.
func TestCEMV1844PortableTasksBoundResultsValidate(t *testing.T) {
	var doc struct {
		Cases []s0eFullCase `json:"cases"`
	}
	s0eJSON(t, "FULL-RESULT-CASES.json", &doc)
	if len(doc.Cases) != 56 {
		t.Fatalf("expected 56 cases, got %d", len(doc.Cases))
	}
	neutralLine, err := os.ReadFile(filepath.Join(stableCaseFiles(t, "neutral-criterion"), "results", "accept.json"))
	if err != nil {
		t.Fatal(err)
	}
	var neutral any
	if err := json.Unmarshal(neutralLine, &neutral); err != nil {
		t.Fatal(err)
	}
	var live any
	if stableOwnerSupported() && (runtime.Version() == s0eFrozenTuple || s0eTupleBinding(t, runtime.Version()) == "") {
		r := newS0eRun(t)
		s0eSetups[52](r)
		got, code := runStableCanonical(r.ctx, r.repo, r.raw, s0eBase, r.target, r.artifacts, r.hooks)
		if code != 0 || got.Outcome != "ACCEPT" || len(got.References.CriterionBindings) == 0 {
			b, _ := json.Marshal(got)
			t.Fatalf("ledger-336-evidence exit=%d result=%s, want a Tasks-bound ACCEPT", code, b)
		}
		live = portableResultObject(t, got)
	} else {
		t.Log("NOT_RUN: the live Tasks-bound run needs qualified process containment and a bound structural tuple; the sealed cases are still validated")
	}
	for _, rel := range portableSchemaCopies {
		schema := loadPortableStableSchema(t, rel)
		if err := portableSchemaValidate(schema, schema, neutral, "$"); err != nil {
			t.Fatalf("%s rejects the sealed neutral-criterion ACCEPT line: %v", rel, err)
		}
		bound := 0
		for _, c := range doc.Cases {
			if err := portableSchemaValidate(schema, schema, c.ExpectedResult, "$"); err != nil {
				t.Fatalf("%s rejects sealed case %s: %v", rel, c.ID, err)
			}
			refs, _ := c.ExpectedResult["references"].(map[string]any)
			if bindings, _ := refs["criterionBindings"].([]any); len(bindings) > 0 {
				bound++
			}
		}
		if bound == 0 {
			t.Fatal("no sealed case carries a Tasks-bound criterion")
		}
		if live != nil {
			if err := portableSchemaValidate(schema, schema, live, "$"); err != nil {
				t.Fatalf("%s rejects the live Tasks-bound portable result: %v", rel, err)
			}
		}
	}
}

// TestPortableStableSchemaValidatorRefusesUnknownKeywords keeps the portable
// test validator closed, like its native twin.
func TestPortableStableSchemaValidatorRefusesUnknownKeywords(t *testing.T) {
	schema := map[string]any{"dependentRequired": map[string]any{}}
	if err := portableSchemaValidate(schema, schema, map[string]any{}, "$"); err == nil || !strings.Contains(err.Error(), "unsupported schema keyword") {
		t.Fatalf("unknown keyword: %v", err)
	}
	for name, c := range map[string]struct {
		schema map[string]any
		value  any
	}{
		"const":      {map[string]any{"const": "a"}, "b"},
		"enum":       {map[string]any{"enum": []any{"a"}}, "b"},
		"anyOf":      {map[string]any{"anyOf": []any{map[string]any{"const": "a"}, map[string]any{"type": "object"}}}, "b"},
		"required":   {map[string]any{"required": []any{"a"}}, map[string]any{}},
		"additional": {map[string]any{"additionalProperties": false, "properties": map[string]any{}}, map[string]any{"a": 1.0}},
		"pattern":    {map[string]any{"pattern": "^[0-9a-f]{2}$"}, "zz"},
	} {
		if portableSchemaValidate(c.schema, c.schema, c.value, "$") == nil {
			t.Fatalf("%s: the validator admitted an invalid value", name)
		}
	}
}

// portableSchemaValidate checks the closed draft 2020-12 keyword subset the stable
// result schema uses and refuses any other keyword instead of ignoring it.
// root resolves local "#/$defs/NAME" references; keywords run in sorted order
// so the first reported failure is deterministic.
func portableSchemaValidate(root, schema map[string]any, value any, at string) error {
	keys := make([]string, 0, len(schema))
	for key := range schema {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := portableSchemaKeyword(root, schema, key, schema[key], value, at); err != nil {
			return err
		}
	}
	return nil
}

func portableSchemaKeyword(root, schema map[string]any, key string, rule, value any, at string) error {
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
		return portableSchemaValidate(root, def, value, at)
	case "type":
		types, ok := rule.([]any)
		if !ok {
			types = []any{rule}
		}
		for _, want := range types {
			if portableSchemaTypeOf(want.(string), value) {
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
					if err := portableSchemaValidate(root, sub.(map[string]any), member, at+"."+name); err != nil {
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
			if err := portableSchemaValidate(root, sub, item, fmt.Sprintf("%s[%d]", at, i)); err != nil {
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
		match, err := portableSchemaPattern(rule.(string))
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
			if err := portableSchemaValidate(root, option.(map[string]any), value, at); err != nil {
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
		if portableSchemaValidate(root, rule.(map[string]any), value, at) == nil {
			return fmt.Errorf("%s: matches a not schema", at)
		}
	case "if":
		branch := "else"
		if portableSchemaValidate(root, rule.(map[string]any), value, at) == nil {
			branch = "then"
		}
		if sub, ok := schema[branch].(map[string]any); ok {
			return portableSchemaValidate(root, sub, value, at)
		}
	default:
		return fmt.Errorf("%s: unsupported schema keyword %s", at, key)
	}
	return nil
}

var portableSchemaUnicodeEscape = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

// portableSchemaPattern compiles an ECMA-262 pattern of the schema with RE2. RE2 has
// no lookahead, so leading negative lookaheads right after "^" (the only form
// the schema uses) are peeled off and each is checked as an anchored prefix
// that must not match; any other unsupported syntax is an error.
func portableSchemaPattern(pattern string) (func(string) bool, error) {
	pattern = portableSchemaUnicodeEscape.ReplaceAllString(pattern, `\x{$1}`)
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

func portableSchemaTypeOf(want string, value any) bool {
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
