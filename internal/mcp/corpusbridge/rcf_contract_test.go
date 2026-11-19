// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// Reconstruct the reviewed schemas, rather than treating their leaves as a
// subset. This also catches empty objects and keywords we do not understand.
func rcfSchemaContract(name string, got any, fields map[string]string) error {
	want := map[string]any{}
	for path, raw := range fields {
		parts := strings.Split(path, ".")
		object := want
		for _, part := range parts[:len(parts)-1] {
			if object[part] == nil {
				object[part] = map[string]any{}
			}
			object = object[part].(map[string]any)
		}
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return err
		}
		object[parts[len(parts)-1]] = value
	}
	// Normalize Go numeric/slice representations as well as schema descriptions.
	raw, err := json.Marshal(got)
	if err != nil {
		return err
	}
	var wire any
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	if !reflect.DeepEqual(rcfWithoutDescriptions(wire, true), rcfWithoutDescriptions(want, true)) {
		return fmt.Errorf("%s: schema differs from the pinned contract", name)
	}
	return nil
}

func rcfWithoutDescriptions(value any, names bool) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range v {
			if key == "description" && !names {
				continue
			}
			if names {
				out[key] = rcfWithoutDescriptions(child, false)
				continue
			}
			// Only these keywords contain schemas. Values in const/enum and
			// unknown keywords are preserved verbatim, including description keys.
			switch key {
			case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
				out[key] = rcfWithoutDescriptions(child, true)
			case "allOf", "anyOf", "oneOf", "prefixItems", "items", "additionalItems",
				"additionalProperties", "contains", "not", "if", "then", "else",
				"propertyNames", "unevaluatedItems", "unevaluatedProperties", "contentSchema":
				out[key] = rcfWithoutDescriptions(child, false)
			default:
				out[key] = child
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = rcfWithoutDescriptions(child, false)
		}
		return out
	default:
		return value
	}
}

// The old union remains an assertion, but cannot certify individual records.
// This tree requires every field in every member. Only the explicitly reviewed
// fixture variants below may differ; unknown fields, types and variants fail.
type rcfShape struct {
	kind     string
	fields   map[string]*rcfShape
	variants []*rcfShape
	counts   []int
	literal  string
}

func (s *rcfShape) at(path string) *rcfShape {
	for _, part := range strings.Split(strings.ReplaceAll(path, "[]", ".[]"), ".") {
		if s.fields == nil {
			s.fields = map[string]*rcfShape{}
		}
		if s.fields[part] == nil {
			s.fields[part] = &rcfShape{}
		}
		s = s.fields[part]
	}
	return s
}

func (s *rcfShape) without(names ...string) *rcfShape {
	c := *s
	c.fields = maps.Clone(s.fields)
	for _, name := range names {
		delete(c.fields, name)
	}
	return &c
}

func rcfOutputShape(name string, fields map[string]string) *rcfShape {
	root := &rcfShape{}
	for field, kind := range fields {
		path, _, _ := strings.Cut(field, " (")
		node := root.at(path)
		if node.kind != "" && node.kind != kind {
			// An unreviewed union is never accepted merely because either type
			// appeared elsewhere. The live-location variant is resolved below.
			node.kind = "unreviewed union"
		} else {
			node.kind = kind
		}
	}
	if name == "search-claims" || name == "get" {
		node := root.at("$.structuredContent.results[]")
		node.variants = []*rcfShape{node.without("subject", "text"), node.without("name", "kind")}
	}
	if name == "info-optional" {
		node := root.at("$.results[]")
		node.variants = []*rcfShape{
			node.without("import_parity", "restricted_summaries", "scope"),
			node.without("builder", "manifest_sha256", "profile_sha256", "profile", "built_at", "validation", "inputs_as_of", "repositories"),
		}
		root.at("$.results").counts = []int{1, 1}
	}
	if name == "compact-optional" {
		// The fixture has two plain and two envelope anchors. Counts prevent
		// dropping an optional field from silently changing a member's variant.
		node := root.at("$.anchors[]")
		node.variants = []*rcfShape{node.without("anchor_state"), node.without()}
		root.at("$.anchors").counts = []int{2, 2}
		node = root.at("$.results[]")
		node.variants = []*rcfShape{node.without("count"), node.without("evidence")}
		root.at("$.results").counts = []int{2, 1}
		node = root.at("$.result_ranks.*")
		node.variants = []*rcfShape{node.without("match_anchor"), node.without()}
		root.at("$.result_ranks").counts = []int{1, 1}
		node = root.at("$.symbol_anchors[]")
		for _, state := range []string{"resolved", "moved", "unresolved"} {
			variant := node.without("live")
			variant.fields["state"] = &rcfShape{kind: "string", literal: state}
			switch state {
			case "resolved":
				variant.fields["live"] = &rcfShape{kind: "string"}
			case "moved":
				live := *node.fields["live"]
				live.kind = "map[string]interface {}"
				variant.fields["live"] = &live
			}
			// Copies must not inherit variants appended in earlier iterations.
			variant.variants = nil
			node.variants = append(node.variants, variant)
		}
		root.at("$.symbol_anchors").counts = []int{1, 1, 1}
	}
	return root.at("$")
}

func (s *rcfShape) check(path string, value any) error {
	if s == nil {
		return fmt.Errorf("%s: unreviewed form %T", path, value)
	}
	if len(s.variants) > 0 {
		for _, variant := range s.variants {
			if variant.check(path, value) == nil {
				return nil
			}
		}
		return fmt.Errorf("%s: no reviewed record variant matches %v", path, value)
	}
	kind := s.kind
	if kind == "" && s.fields["*"] != nil {
		kind = "map[string]interface {}" // Wildcard dictionaries omit their own union entry.
	}
	if fmt.Sprintf("%T", value) != kind {
		return fmt.Errorf("%s: got %T, want %s", path, value, kind)
	}
	if s.literal != "" && value != s.literal {
		return fmt.Errorf("%s: got %v, want %s", path, value, s.literal)
	}
	switch v := value.(type) {
	case map[string]any:
		if member := s.fields["*"]; member != nil {
			values := make([]any, 0, len(v))
			for _, child := range v {
				values = append(values, child)
			}
			return s.checkMembers(path+".*", member, values)
		}
		if len(v) != len(s.fields) {
			return fmt.Errorf("%s: got %d fields, want %d", path, len(v), len(s.fields))
		}
		for key, field := range s.fields {
			child, ok := v[key]
			if !ok {
				return fmt.Errorf("%s: missing %s", path, key)
			}
			if err := field.check(path+"."+key, child); err != nil {
				return err
			}
		}
	case []any:
		return s.checkMembers(path+"[]", s.fields["[]"], v)
	}
	return nil
}

func (s *rcfShape) checkMembers(path string, member *rcfShape, values []any) error {
	counts := make([]int, len(s.counts))
	for i, child := range values {
		if err := member.check(fmt.Sprintf("%s member %d", path, i), child); err != nil {
			return err
		}
		if len(counts) > 0 {
			for j, variant := range member.variants {
				if variant.check(path, child) == nil {
					counts[j]++
				}
			}
		}
	}
	if !reflect.DeepEqual(counts, s.counts) && len(s.counts) > 0 {
		return fmt.Errorf("%s: variant counts %v, want %v", path, counts, s.counts)
	}
	return nil
}

func TestRCFV1011SchemaComparatorRejectsRestrictions(t *testing.T) {
	fields := rcfReadContract(t, "corvint.docs_search-schemas")
	for _, keyword := range []string{"pattern", "unknownConstraint"} {
		t.Run(keyword, func(t *testing.T) {
			for _, tool := range bridge.WithOutputSchemas(AllTools()) {
				if tool.Name != SearchTool {
					continue
				}
				query := tool.InputSchema["properties"].(map[string]any)["query"].(map[string]any)
				query[keyword] = "^Checkout$"
				if err := rcfSchemaContract(tool.Name, map[string]any{"input": tool.InputSchema, "output": tool.OutputSchema}, fields); err == nil {
					t.Fatalf("accepted new schema keyword %s", keyword)
				}
			}
		})
	}
}

func TestRCFV1011SchemaComparatorClosedForms(t *testing.T) {
	fields := map[string]string{
		"input.type":                        `"object"`,
		"input.properties.description.type": `"string"`,
		"input.properties.kind.oneOf":       `[{"type":"string"},{"type":"number"}]`,
	}
	for _, tc := range []struct {
		name  string
		input string
		valid bool
	}{
		{"descriptions", `{"type":"object","description":"prose","properties":{"description":{"type":"string","description":"prose"},"kind":{"oneOf":[{"type":"string","description":"prose"},{"type":"number"}]}}}`, true},
		{"property named description", `{"type":"object","properties":{"description":{"type":"number"},"kind":{"oneOf":[{"type":"string"},{"type":"number"}]}}}`, false},
		{"empty unknown keyword", `{"type":"object","unknownConstraint":{},"properties":{"description":{"type":"string"},"kind":{"oneOf":[{"type":"string"},{"type":"number"}]}}}`, false},
		{"nested array constraint", `{"type":"object","properties":{"description":{"type":"string"},"kind":{"oneOf":[{"type":"string","pattern":"^Checkout$"},{"type":"number"}]}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input any
			if err := json.Unmarshal([]byte(tc.input), &input); err != nil {
				t.Fatal(err)
			}
			err := rcfSchemaContract(tc.name, map[string]any{"input": input}, fields)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t, error=%v", tc.valid, err)
			}
		})
	}
	for _, keyword := range []string{"const", "enum", "unknownConstraint"} {
		t.Run(keyword+" description value", func(t *testing.T) {
			var before, after any = map[string]any{"description": "before"}, map[string]any{"description": "after"}
			if keyword == "enum" {
				before, after = []any{before}, []any{after}
			}
			raw, err := json.Marshal(before)
			if err != nil {
				t.Fatal(err)
			}
			fields := map[string]string{"input." + keyword: string(raw)}
			got := map[string]any{"input": map[string]any{keyword: after}}
			if err := rcfSchemaContract(keyword, got, fields); err == nil {
				t.Fatal("description in a constraint value was discarded")
			}
		})
	}
}

func TestRCFV1011OutputComparatorChecksEveryMember(t *testing.T) {
	shape := rcfOutputShape("compact-optional", rcfReadContract(t, "compact-optional-output"))
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		valid  bool
	}{
		{"original", func(map[string]any) {}, true},
		{"reordered", func(v map[string]any) {
			for _, key := range []string{"results", "symbol_anchors", "anchors"} {
				a := v[key].([]any)
				for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
					a[i], a[j] = a[j], a[i]
				}
			}
		}, true},
		{"unresolved reason removed", func(v map[string]any) {
			delete(v["symbol_anchors"].([]any)[2].(map[string]any), "reason")
		}, false},
		{"one reason retyped", func(v map[string]any) {
			v["symbol_anchors"].([]any)[2].(map[string]any)["reason"] = false
		}, false},
		{"unknown state", func(v map[string]any) {
			v["symbol_anchors"].([]any)[2].(map[string]any)["state"] = "future-state"
		}, false},
		{"wrong live variant", func(v map[string]any) {
			v["symbol_anchors"].([]any)[1].(map[string]any)["live"] = "a2"
		}, false},
		{"unknown member", func(v map[string]any) {
			v["results"] = append(v["results"].([]any), []any{"unknown-record-form"})
		}, false},
		{"one nested field removed", func(v map[string]any) {
			delete(v["results"].([]any)[0].(map[string]any)["evidence"].(map[string]any), "anchors")
		}, false},
		{"one primitive retyped", func(v map[string]any) {
			v["limitations"].([]any)[0] = false
		}, false},
		{"one dictionary field removed", func(v map[string]any) {
			delete(v["result_ranks"].(map[string]any)["x"].(map[string]any), "rank")
		}, false},
		{"optional dictionary field removed", func(v map[string]any) {
			delete(v["result_ranks"].(map[string]any)["y"].(map[string]any), "match_anchor")
		}, false},
		{"optional anchor field removed", func(v map[string]any) {
			delete(v["anchors"].([]any)[0].(map[string]any), "anchor_state")
		}, false},
		{"unknown field", func(v map[string]any) {
			v["results"].([]any)[0].(map[string]any)["future_field"] = "unknown"
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var full map[string]any
			if err := json.Unmarshal([]byte(compactFixture), &full); err != nil {
				t.Fatal(err)
			}
			compact, err := CompactReceipt(full, true)
			if err != nil {
				t.Fatal(err)
			}
			// The contract is for JSON, whose decoded numbers are float64.
			if err := json.Unmarshal([]byte(mustJSON(t, compact)), &compact); err != nil {
				t.Fatal(err)
			}
			tc.mutate(compact)
			err = shape.check("$", compact)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t, error=%v", tc.valid, err)
			}
		})
	}
}
