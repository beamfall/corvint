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
// CEM-V1-622). Each result carries the CEM-V1-083 plain criterion binding, so
// it validates against the committed copies only because CEM-V1-881 admits that
// form; nothing is overlaid (V1-2615).
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
		schema := loadStableSchema(t, filepath.Join(top, rel))
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
		legacySchema := loadStableSchema(t, filepath.Join(top, rel))
		defs := legacySchema["$defs"].(map[string]any)
		props := defs["runnerReceipt"].(map[string]any)["properties"].(map[string]any)
		props["profile"] = map[string]any{"const": wire.StableRunnerReceiptProfile}
		if err := schemaValidate(legacySchema, legacySchema, results[wire.StableOpaqueReceiptProfile], "$"); err == nil || !strings.Contains(err.Error(), "runnerReceipts[0].profile") {
			t.Fatalf("the pre-V1-2107 profile const of %s on opaque/0: %v", rel, err)
		}
		// The sealed predecessor criterionBinding held only the Tasks form; it
		// must refuse the plain binding, which is the gap V1-2615 closed.
		tasksOnly := loadStableSchema(t, filepath.Join(top, rel))
		tasksDefs := tasksOnly["$defs"].(map[string]any)
		alts, ok := tasksDefs["criterionBinding"].(map[string]any)["anyOf"].([]any)
		if !ok || len(alts) != 2 {
			t.Fatalf("%s criterionBinding is not anyOf the Tasks and plain forms", rel)
		}
		tasksDefs["criterionBinding"] = alts[0]
		if err := schemaValidate(tasksOnly, tasksOnly, results[wire.StableRunnerReceiptProfile], "$"); err == nil || !strings.HasPrefix(err.Error(), "$.references.criterionBindings[0]") {
			t.Fatalf("the pre-V1-2615 Tasks-only criterionBinding of %s on a plain binding: %v", rel, err)
		}
		extra := withPlainBindingMember(t, results[wire.StableRunnerReceiptProfile], "ticketId", "ticket:a:b:c")
		if err := schemaValidate(schema, schema, extra, "$"); err == nil || !strings.HasPrefix(err.Error(), "$.references.criterionBindings[0]") {
			t.Fatalf("%s on a plain binding with an extra member: %v", rel, err)
		}
	}
}

// withPlainBindingMember returns a deep copy of a verify result whose first
// criterion binding also carries member.
func withPlainBindingMember(t *testing.T, v any, member, value string) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	out["references"].(map[string]any)["criterionBindings"].([]any)[0].(map[string]any)[member] = value
	return out
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

// stableSchemaNode returns the object at an RFC 6901 pointer of a decoded
// schema copy, so a test can restore one sealed predecessor node.
func stableSchemaNode(t *testing.T, schema map[string]any, pointer string) (parent map[string]any, key string) {
	t.Helper()
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	node := schema
	for _, token := range tokens[:len(tokens)-1] {
		next, ok := node[token].(map[string]any)
		if !ok {
			t.Fatalf("schema pointer %s: %s is not an object", pointer, token)
		}
		node = next
	}
	return node, tokens[len(tokens)-1]
}

// stableResultObject round-trips a verify result through its JSON encoding.
func stableResultObject(t *testing.T, result StableResult) any {
	t.Helper()
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestCEMV1843ReceiptAuthenticationResultsValidate proves every complete
// native verify result of the public receipt-authentication case (the
// VERIFIED ACCEPT, the five axis cases and the thirteen refusals) validates
// against both committed RESULT.schema.json copies as they are, and that the
// sealed predecessor nodes (const NOT_OBSERVED at axes.authentication and at
// the runner receipt echo, the issue code enum without the attestation codes,
// the two artifact kind enums without receipt-attestation) each refuse them,
// which is the gap V1-2521 closed (CEM-V1-840..843, decision 0589).
func TestCEMV1843ReceiptAuthenticationResultsValidate(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	c := loadReceiptAuthCase(t)
	repo, base, target := c.materialize(t)
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root, TrustedKeys: c.trusted(t, c.expect.Accept.TrustedKeys)}
	accept, exit := Stable(context.Background(), c.raw, options)
	if exit != 0 || accept.Outcome != "ACCEPT" || accept.Axes["authentication"] != AuthenticationVerified {
		t.Fatalf("Stable exit=%d result=%s, want the VERIFIED ACCEPT", exit, stableResultDebug(accept))
	}
	results := map[string]any{"accept": stableResultObject(t, accept)}
	axes := map[string]string{"accept": AuthenticationVerified}
	issues := map[string]string{}
	for _, m := range append(append([]receiptAuthMutation{}, c.expect.AxisCases...), c.expect.Refusals...) {
		o := options
		o.TrustedKeys = c.trusted(t, m.TrustedKeys)
		result, exit := Stable(context.Background(), c.mutate(t, m.Set), o)
		if exit != m.Exit || result.Outcome != m.Outcome || result.Axes["authentication"] != m.Authentication {
			t.Fatalf("%s: Stable exit=%d result=%s, want %d %s authentication %s", m.Name, exit, stableResultDebug(result), m.Exit, m.Outcome, m.Authentication)
		}
		results[m.Name] = stableResultObject(t, result)
		axes[m.Name] = m.Authentication
		if m.IssueCode != "" {
			issues[m.Name] = m.IssueCode
		}
	}
	seenAxes, seenIssues := map[string]bool{}, map[string]bool{}
	for _, v := range axes {
		seenAxes[v] = true
	}
	for _, v := range issues {
		seenIssues[v] = true
	}
	for _, want := range []string{AuthenticationNotObserved, AuthenticationUntrusted, AuthenticationVerified, AuthenticationFailed} {
		if !seenAxes[want] {
			t.Fatalf("the public case never yields authentication %s", want)
		}
	}
	for _, want := range []string{"attestation-malformed", "attestation-key-mismatch", "attestation-subject-mismatch", "attestation-signature-invalid"} {
		if !seenIssues[want] {
			t.Fatalf("the public case never yields issue code %s", want)
		}
	}
	echo, _ := results["accept"].(map[string]any)["references"].(map[string]any)["runnerReceipts"].([]any)[0].(map[string]any)["authentication"].(map[string]any)
	if echo["method"] != "dsse-ed25519" {
		t.Fatalf("the accept result echoes runner receipt authentication %v, want the dsse-ed25519 object", echo)
	}
	top := repositoryRoot(t)
	for _, rel := range stableSchemaCopies {
		schema := loadStableSchema(t, filepath.Join(top, rel))
		for name, v := range results {
			if err := schemaValidate(schema, schema, v, "$"); err != nil {
				t.Fatalf("%s rejects the %s result (authentication %s): %v", rel, name, axes[name], err)
			}
		}
		// Each sealed predecessor node, restored on its own, refuses the
		// result that CEM-V1-084 makes both consumers emit.
		predecessors := []struct {
			pointer string
			node    any
			result  string
			at      string
		}{
			{"/properties/axes/properties/authentication", map[string]any{"const": "NOT_OBSERVED"}, "accept", "$.axes.authentication"},
			{"/$defs/runnerReceipt/properties/authentication", map[string]any{"const": "NOT_OBSERVED"}, "accept", "$.references.runnerReceipts[0].authentication"},
			{"/$defs/artifact/properties/kind", nil, "accept", "$.references.artifacts[2].kind"},
			{"/properties/artifactChecks/items/properties/kind", nil, "accept", "$.artifactChecks[2].kind"},
		}
		for name, code := range issues {
			if strings.HasPrefix(code, "attestation-") {
				predecessors = append(predecessors, struct {
					pointer string
					node    any
					result  string
					at      string
				}{"/properties/issueCodes/items", nil, name, "$.issueCodes[0]"})
			}
		}
		for _, p := range predecessors {
			sealed := loadStableSchema(t, filepath.Join(top, rel))
			parent, key := stableSchemaNode(t, sealed, p.pointer)
			node := p.node
			if node == nil {
				var kept []any
				for _, v := range parent[key].(map[string]any)["enum"].([]any) {
					if !strings.Contains(v.(string), "attestation") {
						kept = append(kept, v)
					}
				}
				node = map[string]any{"enum": kept}
			}
			parent[key] = node
			if err := schemaValidate(sealed, sealed, results[p.result], "$"); err == nil || !strings.HasPrefix(err.Error(), p.at) {
				t.Fatalf("the sealed predecessor of %s at %s on the %s result: %v", rel, p.pointer, p.result, err)
			}
		}
	}
}

// TestCEMV1844TasksBoundResultsValidate proves the sealed Tasks-bound results
// keep validating after the CEM-V1-881 widening: every full-result case of
// the S0E packet validates against both committed copies as they are, the
// ACCEPT cases carry only Tasks-form criterion bindings (the plain branch
// alone refuses them, the Tasks branch alone admits them), and the plain
// neutral-criterion ACCEPT line validates beside them (CEM-V1-844, V1-2520,
// decision 0589). The live Tasks-bound run is TestCEMV1844TasksBoundLiveResultValidates.
func TestCEMV1844TasksBoundResultsValidate(t *testing.T) {
	t.Parallel()
	packet := loadS0EPacket(t)
	if len(packet.cases) != 56 {
		t.Fatalf("public packet has %d cases, want 56", len(packet.cases))
	}
	top := repositoryRoot(t)
	neutralLine, err := os.ReadFile(filepath.Join(top, neutralCaseRoot, "results", "accept.json"))
	if err != nil {
		t.Fatal(err)
	}
	var neutral any
	if err := json.Unmarshal(neutralLine, &neutral); err != nil {
		t.Fatal(err)
	}
	for _, rel := range stableSchemaCopies {
		schema := loadStableSchema(t, filepath.Join(top, rel))
		tasksOnly := loadStableSchema(t, filepath.Join(top, rel))
		plainOnly := loadStableSchema(t, filepath.Join(top, rel))
		for _, s := range []map[string]any{tasksOnly, plainOnly} {
			alts, ok := s["$defs"].(map[string]any)["criterionBinding"].(map[string]any)["anyOf"].([]any)
			if !ok || len(alts) != 2 {
				t.Fatalf("%s criterionBinding is not anyOf the Tasks and plain forms", rel)
			}
		}
		tasksOnly["$defs"].(map[string]any)["criterionBinding"] = tasksOnly["$defs"].(map[string]any)["criterionBinding"].(map[string]any)["anyOf"].([]any)[0]
		plainOnly["$defs"].(map[string]any)["criterionBinding"] = plainOnly["$defs"].(map[string]any)["criterionBinding"].(map[string]any)["anyOf"].([]any)[1]
		if err := schemaValidate(schema, schema, neutral, "$"); err != nil {
			t.Fatalf("%s rejects the sealed neutral-criterion ACCEPT line: %v", rel, err)
		}
		if err := schemaValidate(tasksOnly, tasksOnly, neutral, "$"); err == nil || !strings.HasPrefix(err.Error(), "$.references.criterionBindings[0]") {
			t.Fatalf("the Tasks-only criterionBinding of %s on the plain line: %v", rel, err)
		}
		bound := 0
		for _, c := range packet.cases {
			if err := schemaValidate(schema, schema, c.ExpectedResult, "$"); err != nil {
				t.Fatalf("%s rejects sealed case %s: %v", rel, c.ID, err)
			}
			refs, _ := c.ExpectedResult["references"].(map[string]any)
			bindings, _ := refs["criterionBindings"].([]any)
			if len(bindings) == 0 {
				continue
			}
			bound++
			if err := schemaValidate(tasksOnly, tasksOnly, c.ExpectedResult, "$"); err != nil {
				t.Fatalf("the Tasks-only criterionBinding of %s rejects sealed case %s: %v", rel, c.ID, err)
			}
			if err := schemaValidate(plainOnly, plainOnly, c.ExpectedResult, "$"); err == nil || !strings.HasPrefix(err.Error(), "$.references.criterionBindings[0]") {
				t.Fatalf("the plain-only criterionBinding of %s on sealed case %s: %v", rel, c.ID, err)
			}
		}
		if bound == 0 {
			t.Fatal("no sealed case carries a Tasks-bound criterion")
		}
	}
}
