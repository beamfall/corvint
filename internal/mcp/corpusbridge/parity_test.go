package corpusbridge

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

func TestCorpusCompleteReadAndTypedToolSchema(t *testing.T) {
	t.Run("DCP-V1-038 all CLI reads and typed capabilities", func(t *testing.T) {
		caps := []doccorpus.Capability{}
		for _, name := range []string{"info", "subjects", "claims", "relations", "journeys", "coverage", "gaps", "stability"} {
			caps = append(caps, doccorpus.Capability{Name: name, State: "present"})
		}
		r := &Registry{header: header{Schema: doccorpus.SchemaV2, Capabilities: caps}}
		seen := map[string]bool{}
		for _, tool := range r.Tools() {
			seen[tool.Name] = true
			op := tools[tool.Name]
			props := tool.InputSchema["properties"].(map[string]any)
			if props["retirement"] == nil {
				t.Fatal("trust policy schema missing")
			}
			if kind := doccorpus.OperationInput(op); kind != "" && props[kind] == nil {
				t.Fatal("typed required input missing", op)
			}
		}
		for name, op := range tools {
			if !seen[name] {
				t.Fatal("CLI operation absent MCP", op)
			}
		}
		r.header.Capabilities = []doccorpus.Capability{{Name: "subjects", State: "absent"}}
		if len(r.Tools()) != 0 {
			t.Fatal("absent capability exposed")
		}
	})
}

// RCF-V1-011, RCF-D2: pin the three promoted tools' advertised schemas, beyond
// the MTT-V1 table's property-name checks. Descriptions are prose, not wire types.
func TestRCFV1011DocsToolSchemas(t *testing.T) {
	for _, tool := range bridge.WithOutputSchemas(AllTools()) {
		if !rcfPromotedTool(tool.Name) {
			continue
		}
		t.Run(tool.Name, func(t *testing.T) {
			fields := map[string]string{}
			rcfSchemaFields("input", tool.InputSchema, fields)
			rcfSchemaFields("output", tool.OutputSchema, fields)
			rcfContract(t, tool.Name+"-schemas", fields)
			if err := rcfSchemaContract(tool.Name+"-schemas", map[string]any{
				"input": tool.InputSchema, "output": tool.OutputSchema,
			}, rcfReadContract(t, tool.Name+"-schemas")); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, name := range []string{"corvint.docs_search", "corvint.docs_get", "corvint.docs_info"} {
		found := false
		for _, tool := range AllTools() {
			found = found || tool.Name == name
		}
		if !found {
			t.Errorf("stable tool %s is missing", name)
		}
	}
}

func rcfPromotedTool(name string) bool {
	return name == "corvint.docs_search" || name == "corvint.docs_get" || name == "corvint.docs_info"
}

func rcfSchemaFields(path string, value any, fields map[string]string) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if key != "description" {
				rcfSchemaFields(path+"."+key, child, fields)
			}
		}
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		fields[path] = string(raw)
	}
}

// RCF-V1-011: optional fields cannot disappear unnoticed merely because a small
// runtime fixture omits them. Walk every serialized field reachable from the
// manifest, artifact and receipt types, including nested records and JSON tags.
// Runtime output tests below cover dynamically assembled maps and compact views.
func TestRCFV1011CorpusFormat(t *testing.T) {
	fields := map[string]string{
		"artifact.schema": doccorpus.SchemaV2,
		"manifest.schema": doccorpus.ManifestSchemaV2,
		"receipt.schema":  doccorpus.ReceiptSchema,
	}
	for _, value := range []any{doccorpus.Manifest{}, doccorpus.Artifact{}, doccorpus.Receipt{}, doccorpus.InputFreshness{}, doccorpus.RepositoryListing{}} {
		rcfTypeFields(reflect.TypeOf(value), fields, map[reflect.Type]bool{})
	}
	rcfContract(t, "corpus-format", fields)
}

func rcfTypeFields(typ reflect.Type, fields map[string]string, seen map[reflect.Type]bool) {
	if seen[typ] {
		return
	}
	seen[typ] = true
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		rcfTypeFields(typ.Elem(), fields, seen)
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			tag := field.Tag.Get("json")
			if !field.IsExported() || tag == "-" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "" {
				name = field.Name
			}
			fields[typ.String()+"."+name] = field.Type.String() + " json:" + tag
			rcfTypeFields(field.Type, fields, seen)
		}
	}
}

// RCF-V1-011: exercise the shared group used by both docs servers. Keep only
// field types plus each member's closed record variant; result order is not pinned.
// Reuse the /2 corpus fixture and the compact-view fixture's optional branches.
func TestRCFV1011DocsToolOutputs(t *testing.T) {
	root := boundFixture(t, 3, true)
	r, err := New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	group := Group{Registry: r}
	search, rpcErr := group.Call(t.Context(), SearchTool, map[string]any{"query": "Checkout", "receipt": "full"})
	if rpcErr != nil || search["isError"] != false {
		t.Fatalf("search fixture: %v %v", search, rpcErr)
	}
	id := search["structuredContent"].(map[string]any)["results"].([]any)[0].(map[string]any)["id"]
	cases := []struct {
		name, tool string
		args       map[string]any
	}{
		{"search-full", SearchTool, map[string]any{"query": "Checkout", "receipt": "full", "limit": 1}},
		{"search-compact", SearchTool, map[string]any{"query": "Checkout", "limit": 1}},
		{"search-compact-capabilities", SearchTool, map[string]any{"query": "Checkout", "include": []any{"capabilities"}, "offset": 1, "limit": 1}},
		{"search-claims", SearchTool, map[string]any{"query": "Checkout", "receipt": "full", "include": []any{"claims"}, "limit": 256}},
		{"search-miss", SearchTool, map[string]any{"query": "no-such-document"}},
		{"get", "corvint.docs_get", map[string]any{"id": id}},
		{"get-miss", "corvint.docs_get", map[string]any{"id": "no-such-id"}},
		{"info", "corvint.docs_info", map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, rpcErr := group.Call(t.Context(), tc.tool, tc.args)
			if rpcErr != nil || result["isError"] != false {
				t.Fatalf("call: %v %v", result, rpcErr)
			}
			rcfOutputContract(t, tc.name, result)
		})
	}
	// Exercise info's optional, dynamically named result fields through Query.
	// Work on a value copy; the registry's verified artifact stays unchanged.
	artifact := *r.artifact
	artifact.Manifest.Inputs = append([]doccorpus.Input{}, artifact.Manifest.Inputs...)
	artifact.Manifest.Inputs = append(artifact.Manifest.Inputs, doccorpus.Input{
		Path: "compiled.json", Purpose: "provider",
		AsOf: &doccorpus.InputAsOf{Source: "fixture", Kind: "timestamp", Value: "2026-11-18T00:00:00Z"},
	})
	artifact.Manifest.RepositoryInventory = &doccorpus.RepositoryInventory{}
	artifact.ImportParity = []doccorpus.ImportParity{{Provider: "fixture", SourceKind: "fixture"}}
	info, infoErr := doccorpus.Query(&artifact, doccorpus.Request{Operation: "info"}, "fresh", nil)
	if infoErr != nil {
		t.Fatal(infoErr)
	}
	encoded, encodeErr := doccorpus.Encode(info)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	var infoWire any
	if err := json.Unmarshal(encoded, &infoWire); err != nil {
		t.Fatal(err)
	}
	rcfOutputContract(t, "info-optional", infoWire)
	var full map[string]any
	if err := json.Unmarshal([]byte(compactFixture), &full); err != nil {
		t.Fatal(err)
	}
	compact, compactErr := CompactReceipt(full, true)
	if compactErr != nil {
		t.Fatal(compactErr)
	}
	rcfOutputContract(t, "compact-optional", compact)
	for _, name := range []string{SearchTool, "corvint.docs_get", "corvint.docs_info"} {
		failure, rpcErr := Failure(name, "corpus-refused", "fixture refusal")
		if rpcErr != nil {
			t.Fatal(rpcErr)
		}
		rcfOutputContract(t, "tool-failure", failure)
	}
}

func rcfOutputContract(t *testing.T, name string, value any) {
	t.Helper()
	var wire any
	if err := json.Unmarshal([]byte(mustJSON(t, value)), &wire); err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	var walk func(string, any)
	walk = func(path string, v any) {
		kind := fmt.Sprintf("%T", v)
		fields[path+" ("+kind+")"] = kind
		switch v := v.(type) {
		case map[string]any:
			for key, child := range v {
				switch key {
				case "details", "claim_audits", "claim_mentions", "result_repositories", "text_origins", "result_kinds", "result_ranks", "claim_kinds", "result_locations":
					if dict, ok := child.(map[string]any); ok {
						for _, entry := range dict {
							walk(path+"."+key+".*", entry)
						}
						continue
					}
				}
				walk(path+"."+key, child)
			}
		case []any:
			for _, child := range v {
				walk(path+"[]", child)
			}
		}
	}
	walk("$", wire)
	rcfContract(t, name+"-output", fields)
	if err := rcfOutputShape(name, rcfReadContract(t, name+"-output")).check("$", wire); err != nil {
		t.Fatal(err)
	}
}

// An explicit field list allows additive fields, but every existing field and
// type must remain. Goldens are reviewed inputs, never rewritten by a test run.
func rcfContract(t *testing.T, name string, got map[string]string) {
	t.Helper()
	want := rcfReadContract(t, name)
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value, ok := got[key]; !ok || value != want[key] {
			t.Errorf("%s: field %s: got %q (present=%t), want %q", name, key, value, ok, want[key])
		}
	}
}

func rcfReadContract(t *testing.T, name string) map[string]string {
	t.Helper()
	path := "testdata/rcf-contract/" + name + ".json"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil || len(want) == 0 {
		t.Fatalf("invalid contract %s: %v", path, err)
	}
	return want
}
