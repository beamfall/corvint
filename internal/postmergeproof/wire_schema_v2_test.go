// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

var frozenSchemaFilesV2 = []string{
	"process-proof.schema.json", "process-policy.schema.json", "process-preimages.schema.json",
	"process-qualification.schema.json", "native-graph-binding.schema.json",
}

// TestWireTableMatchesFrozenSchemas checks the closed decoder table against
// every $defs entry of the frozen PMR-V2 process schemas, which share defs.
func TestWireTableMatchesFrozenSchemas(t *testing.T) {
	var first map[string]any
	for _, name := range frozenSchemaFilesV2 {
		data, err := os.ReadFile("../../conformance/postmerge-runtime-v2/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatal(err)
		}
		defs, _ := schema["$defs"].(map[string]any)
		if first == nil {
			first = defs
		} else if !reflect.DeepEqual(first, defs) {
			t.Fatalf("%s $defs differ from %s", name, frozenSchemaFilesV2[0])
		}
	}
	if len(first) != len(wireTypesV2) {
		t.Fatalf("schema defines %d types, table has %d", len(first), len(wireTypesV2))
	}
	for typeName, raw := range first {
		def := raw.(map[string]any)
		fields, ok := wireTypesV2[typeName]
		if !ok {
			t.Errorf("table lacks %s", typeName)
			continue
		}
		if def["additionalProperties"] != false || def["type"] != "object" {
			t.Errorf("%s is not a closed object", typeName)
		}
		properties := def["properties"].(map[string]any)
		required := def["required"].([]any)
		if len(properties) != len(fields) || len(required) != len(fields) {
			t.Errorf("%s: member count differs", typeName)
		}
		for _, f := range fields {
			property, ok := properties[f.name].(map[string]any)
			if !ok || !slices.Contains(required, any(f.name)) {
				t.Errorf("%s.%s is not a required schema property", typeName, f.name)
				continue
			}
			if want := schemaFieldSpec(t, f.name, property); !sameFieldSpec(want, f) {
				t.Errorf("%s.%s: table %+v, schema %+v", typeName, f.name, f, want)
			}
		}
	}
}

func schemaFieldSpec(t *testing.T, name string, p map[string]any) fieldSpec {
	f := fieldSpec{name: name}
	if anyOf, ok := p["anyOf"].([]any); ok {
		if len(anyOf) != 2 || !reflect.DeepEqual(anyOf[1], map[string]any{"type": "null"}) {
			t.Fatalf("%s: unsupported anyOf", name)
		}
		f.nullable = true
		p = anyOf[0].(map[string]any)
	}
	integerBounds := func() {
		f.kind = kInt
		f.min, f.max = int64(p["minimum"].(float64)), maxInt64
		if p["maximum"].(float64) < 1e18 {
			f.max = int64(p["maximum"].(float64))
		}
	}
	switch {
	case p["$ref"] != nil:
		f.kind, f.ref = kObject, p["$ref"].(string)[len("#/$defs/"):]
	case p["const"] != nil:
		switch value := p["const"].(type) {
		case string:
			f.kind, f.constant = kString, value
		case bool:
			f.kind, f.constBool = kBool, &value
		case float64:
			v := int64(value)
			f.kind, f.constInt, f.min, f.max = kInt, &v, v, v
		}
	case p["enum"] != nil:
		f.kind = kString
		for _, value := range p["enum"].([]any) {
			f.enum = append(f.enum, value.(string))
		}
	case p["type"] == "string" && p["contentEncoding"] == "base64":
		f.kind = kBytes
	case p["type"] == "string" && p["pattern"] == "^[0-9a-f]{64}$":
		f.kind = kSHA
	case p["type"] == "string":
		f.kind = kString
	case p["type"] == "boolean":
		f.kind = kBool
	case p["type"] == "integer":
		integerBounds()
	case p["type"] == "array":
		items := p["items"].(map[string]any)
		f.kind = kStringArray
		if ref, ok := items["$ref"].(string); ok {
			f.kind, f.ref = kObjectArray, ref[len("#/$defs/"):]
		}
		f.maxItems = int(p["maxItems"].(float64))
		if minItems, ok := p["minItems"].(float64); ok {
			f.minItems = int(minItems)
		}
	default:
		t.Fatalf("%s: unsupported schema property %v", name, p)
	}
	return f
}

func sameFieldSpec(a, b fieldSpec) bool {
	if a.kind != b.kind || a.ref != b.ref || a.nullable != b.nullable || a.constant != b.constant ||
		!slices.Equal(a.enum, b.enum) || a.minItems != b.minItems || a.maxItems != b.maxItems {
		return false
	}
	if (a.constBool == nil) != (b.constBool == nil) || (a.constBool != nil && *a.constBool != *b.constBool) {
		return false
	}
	if (a.constInt == nil) != (b.constInt == nil) || (a.constInt != nil && *a.constInt != *b.constInt) {
		return false
	}
	return a.kind != kInt || (a.min == b.min && a.max == b.max)
}
