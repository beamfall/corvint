package delta

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDeltaRecordsMatchPublishedSchema(t *testing.T) {
	t.Run("DLT-V0-008 emitted records and decision vectors satisfy protocol/delta/schema.json", testDeltaRecordsMatchPublishedSchema)
}

func testDeltaRecordsMatchPublishedSchema(t *testing.T) {
	raw, err := os.ReadFile("../../protocol/delta/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	var records [][]byte
	vectorsRaw, err := os.ReadFile("../../conformance/delta-v0/decision-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name   string `json:"name"`
		Record Record `json:"record"`
	}
	if err := json.Unmarshal(vectorsRaw, &vectors, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		r := v.Record
		r.finalize()
		encoded, err := r.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, encoded)
	}
	root, base, head := fixtureMerge(t)
	for _, o := range []Options{
		{Base: base, Head: head, Build: "163", WorkKeyPattern: `TASK-[0-9]+`},
		{Base: head, Head: head, Build: "163"},
	} {
		r, err := Compile(context.Background(), root, o)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := r.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, encoded)
	}
	for i, encoded := range records {
		var instance any
		if err := json.Unmarshal(encoded, &instance); err != nil {
			t.Fatal(err)
		}
		if err := validateSchema(schema, instance, "$"); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	// The validator must refuse, not ignore, a record outside the schema.
	var bad map[string]any
	if err := json.Unmarshal(records[0], &bad); err != nil {
		t.Fatal(err)
	}
	bad["decision"] = "maybe"
	if validateSchema(schema, bad, "$") == nil {
		t.Fatal("schema validator admitted an invalid decision")
	}
	bad["decision"], bad["changedPaths"] = "findings", []any{"../escape"}
	if validateSchema(schema, bad, "$") == nil {
		t.Fatal("schema validator admitted a traversal path")
	}
}

// validateSchema checks the closed draft 2020-12 keyword subset the published
// delta schema uses and refuses any other keyword instead of ignoring it.
func validateSchema(schema map[string]any, value any, at string) error {
	for key, rule := range schema {
		switch key {
		case "$schema", "$id", "$comment", "properties":
		case "type":
			if !schemaType(rule.(string), value) {
				return fmt.Errorf("%s: want type %s", at, rule)
			}
		case "enum":
			found := false
			for _, option := range rule.([]any) {
				found = found || reflect.DeepEqual(option, value)
			}
			if !found {
				return fmt.Errorf("%s: %v not in enum", at, value)
			}
		case "pattern":
			if s, ok := value.(string); ok && !schemaPattern(rule.(string), s) {
				return fmt.Errorf("%s: %q does not match %s", at, s, rule)
			}
		case "minLength":
			if s, ok := value.(string); ok && float64(utf8.RuneCountInString(s)) < rule.(float64) {
				return fmt.Errorf("%s: shorter than %v", at, rule)
			}
		case "minimum":
			if n, ok := value.(float64); ok && n < rule.(float64) {
				return fmt.Errorf("%s: below %v", at, rule)
			}
		case "maxItems":
			if a, ok := value.([]any); ok && float64(len(a)) > rule.(float64) {
				return fmt.Errorf("%s: more than %v items", at, rule)
			}
		case "items":
			if a, ok := value.([]any); ok {
				for i, item := range a {
					if err := validateSchema(rule.(map[string]any), item, fmt.Sprintf("%s[%d]", at, i)); err != nil {
						return err
					}
				}
			}
		case "required":
			if o, ok := value.(map[string]any); ok {
				for _, name := range rule.([]any) {
					if _, present := o[name.(string)]; !present {
						return fmt.Errorf("%s: missing %s", at, name)
					}
				}
			}
		case "additionalProperties":
			if rule != false {
				return fmt.Errorf("%s: unsupported additionalProperties %v", at, rule)
			}
			if o, ok := value.(map[string]any); ok {
				properties, _ := schema["properties"].(map[string]any)
				for name := range o {
					if _, declared := properties[name]; !declared {
						return fmt.Errorf("%s: undeclared member %s", at, name)
					}
				}
			}
		case "anyOf":
			var failures []string
			for _, option := range rule.([]any) {
				err := validateSchema(option.(map[string]any), value, at)
				if err == nil {
					failures = nil
					break
				}
				failures = append(failures, err.Error())
			}
			if failures != nil {
				return fmt.Errorf("%s: no anyOf branch: %s", at, strings.Join(failures, "; "))
			}
		default:
			return fmt.Errorf("%s: unsupported schema keyword %s", at, key)
		}
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		if o, ok := value.(map[string]any); ok {
			for name, member := range o {
				if rule, declared := properties[name]; declared {
					if err := validateSchema(rule.(map[string]any), member, at+"."+name); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func schemaType(want string, value any) bool {
	switch want {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "integer":
		n, ok := value.(float64)
		return ok && n == float64(int64(n))
	}
	return false
}

// schemaPattern evaluates ECMA leading negative lookaheads, which RE2 lacks,
// as separate anchored refusals before matching the remaining pattern.
func schemaPattern(pattern, s string) bool {
	rest := strings.TrimPrefix(pattern, "^")
	for strings.HasPrefix(rest, "(?!") {
		depth, end := 0, -1
		for i := 0; i < len(rest) && end < 0; i++ {
			switch rest[i] {
			case '\\':
				i++
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					end = i
				}
			}
		}
		if regexp.MustCompile("^" + rest[3:end]).MatchString(s) {
			return false
		}
		rest = rest[end+1:]
	}
	return regexp.MustCompile("^" + rest).MatchString(s)
}
