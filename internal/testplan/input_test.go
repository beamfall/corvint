package testplan

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// TestDecodeInputRefusals covers every TCN-V0-002 refusal and bound.
func TestDecodeInputRefusals(t *testing.T) {
	valid := func(edits ...func(map[string]any)) string {
		return string(inputJSON(t, variation("V1", edits...)))
	}
	long := strings.Repeat("a", 257)
	cases := map[string]string{
		"not json":                 `{`,
		"not an object":            `[]`,
		"trailing data":            valid() + ` {}`,
		"duplicate top member":     `{"schema":"test-consolidation-input/0","schema":"test-consolidation-input/0","variations":[]}`,
		"unknown top member":       strings.Replace(valid(), `"schema"`, `"extra":1,"schema"`, 1),
		"wrong schema":             strings.Replace(valid(), InputSchema, "test-consolidation-input/1", 1),
		"short revision":           strings.Replace(valid(), `"schema"`, `"revision":"abc","schema"`, 1),
		"uppercase revision":       strings.Replace(valid(), `"schema"`, `"revision":"`+strings.ToUpper(fixedRevision)+`","schema"`, 1),
		"no variations":            `{"schema":"test-consolidation-input/0","variations":[]}`,
		"null variations":          `{"schema":"test-consolidation-input/0","variations":null}`,
		"variation not object":     `{"schema":"test-consolidation-input/0","variations":[1]}`,
		"unknown variation member": valid(set("note", "x")),
		"duplicate variation member": strings.Replace(valid(), `"variation_id":"V1"`,
			`"variation_id":"V1","variation_id":"V1"`, 1),
		"repeated variation_id":   string(inputJSON(t, variation("V1"), variation("V1"))),
		"identifier charset":      valid(set("user", "club admin")),
		"identifier too long":     valid(set("app", long)),
		"empty identifier":        valid(set("screen", "")),
		"empty user":              valid(set("user", "")),
		"pipe in identifier":      valid(set("setup", "a|b")),
		"spec dot segment":        valid(set("spec", "tests/./a.spec.ts")),
		"spec dotdot segment":     valid(set("spec", "../a.spec.ts")),
		"spec empty segment":      valid(set("spec", "tests//a.spec.ts")),
		"spec absolute":           valid(set("spec", "/tests/a.spec.ts")),
		"spec whitespace":         valid(set("spec", "tests/a b.spec.ts")),
		"spec pipe":               valid(set("spec", "tests/a|b.spec.ts")),
		"spec control":            valid(set("spec", "tests/a\u0007.spec.ts")),
		"action empty":            valid(set("action", []any{})),
		"action too long":         valid(set("action", repeatList("element:a", 65))),
		"assertion repeated":      valid(set("assertion", []any{"element:a", "element:a"})),
		"assertion empty":         valid(set("assertion", []any{})),
		"malformed fact":          valid(set("requires", []any{"no-equals"})),
		"empty fact key":          valid(set("changes", []any{"=v"})),
		"fact key too long":       valid(set("requires", []any{strings.Repeat("k", 129) + "=v"})),
		"one key twice":           valid(set("requires", []any{"slot=free", "slot=booked"})),
		"too many facts":          valid(set("changes", factList(65))),
		"destructive not bool":    valid(set("destructive", "yes")),
		"witnesses null":          valid(set("witnesses", nil)),
		"witness inferred":        valid(set("witnesses", []any{map[string]any{"test_id": "t1", "basis": "inferred"}})),
		"witness unknown member":  valid(set("witnesses", []any{map[string]any{"test_id": "t1", "basis": "declared", "x": "y"}})),
		"witness project charset": valid(set("witnesses", []any{map[string]any{"test_id": "t1", "project": "Desktop Chrome", "basis": "declared"}})),
		"witness without test_id": valid(set("witnesses", []any{map[string]any{"basis": "declared"}})),
		"too many witnesses":      valid(set("witnesses", witnessList(17))),
		"flow_id charset":         valid(set("flow_id", "a b")),
		"route not string":        valid(set("route", 3)),
		"invalid utf8":            strings.Replace(valid(), "club-admin", "club-\xffadmin", 1),
		"too deep":                strings.Replace(valid(), `"schema"`, `"x":[[[[[[[[[[1]]]]]]]]]],"schema"`, 1),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeInput([]byte(data)); code(err) != "test-plan-invalid-input" {
				t.Fatalf("got %v, want test-plan-invalid-input", err)
			}
		})
	}
	t.Run("input bound", func(t *testing.T) {
		big := append(inputJSON(t, variation("V1")), bytes.Repeat([]byte(" "), MaxInputBytes)...)
		if _, err := DecodeInput(big); code(err) != "test-plan-invalid-input" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("variation bound", func(t *testing.T) {
		rows := make([]map[string]any, MaxVariations+1)
		for i := range rows {
			rows[i] = map[string]any{"variation_id": "V" + strconv.Itoa(i)}
		}
		if _, err := DecodeInput(inputJSON(t, rows...)); code(err) != "test-plan-invalid-input" {
			t.Fatalf("got %v", err)
		}
	})
}

// TestDecodeInputMissingAnchorsAreNotRefusals: absent or null anchors are recorded, never refused,
// and explicit empty facts or false destructive are declarations (TCN-V0-002, TCN-V0-003).
func TestDecodeInputMissingAnchorsAreNotRefusals(t *testing.T) {
	in := decode(t, inputJSON(t,
		variation("V1", drop("changes"), set("destructive", nil), set("org", nil), drop("witnesses")),
		variation("V2", set("org", ""), set("flow_id", nil), set("route", nil)),
	))
	if got := strings.Join(in.Variations[0].Missing, ","); got != "org,changes,destructive" {
		t.Fatalf("missing = %q", got)
	}
	v2 := in.Variations[1]
	if len(v2.Missing) != 0 || v2.Requires == nil || len(v2.Requires) != 0 || v2.Changes == nil || *v2.Destructive || *v2.Org != "" {
		t.Fatalf("explicit declarations were not kept: %+v", v2)
	}
}

// TestInputOrderDoesNotChangeOutput permutes variations, set members, facts and witnesses and
// re-serialises the document; every output byte must stay the same (TCN-V0-002, TCN-V0-008).
func TestInputOrderDoesNotChangeOutput(t *testing.T) {
	base := []map[string]any{
		variation("V1", set("requires", []any{"a=1", "b=2"}), set("assertion", []any{"element:x", "element:y", "element:z"})),
		variation("V2", set("changes", []any{"a=2", "c=3"}), set("witnesses", []any{
			map[string]any{"test_id": "t2", "basis": "declared"}, map[string]any{"test_id": "t1", "project": "chromium", "basis": "reviewed"},
			map[string]any{"test_id": "t1", "project": "chromium", "basis": "declared"}})),
		variation("V3", set("requires", []any{"c=0"}), set("user", "member")),
		variation("V4", set("destructive", true)),
		variation("V5", drop("screen")),
		variation("V6", set("requires", []any{"a=1", "b=2"}), set("assertion", []any{"element:x", "element:y", "element:z"}), set("action", []any{"element:V1.act"})),
	}
	evidence := Evidence{Revision: fixedRevision, TestsDigest: "none", Documents: []testvaliditydoc.Document{
		e2eDocument("t1", "chromium", passing())}}
	reference := build(t, evidence, 8, base...)
	random := rand.New(rand.NewSource(681))
	for round := 0; round < 25; round++ {
		shuffled := make([]map[string]any, len(base))
		for i, v := range base {
			copied := map[string]any{}
			for k, value := range v {
				if list, ok := value.([]any); ok {
					list = append(make([]any, 0, len(list)), list...)
					random.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
					value = list
				}
				copied[k] = value
			}
			shuffled[i] = copied
		}
		random.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got := build(t, evidence, 8, shuffled...)
		if !bytes.Equal(got.JSON(), reference.JSON()) || !bytes.Equal(got.Table(), reference.Table()) {
			t.Fatalf("round %d: permuted input changed the output\n%s\n%s", round, got.Table(), reference.Table())
		}
	}
	// Re-encoding with indentation and different key order keeps the digest.
	var generic any
	if err := json.Unmarshal(inputJSON(t, base...), &generic); err != nil {
		t.Fatal(err)
	}
	indented, _ := json.MarshalIndent(generic, "", "  ")
	if decode(t, indented).Digest() != reference.InputDigest {
		t.Fatal("whitespace changed the input digest")
	}
}

func repeatList(item string, n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = item
	}
	return out
}

func factList(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = "k" + strconv.Itoa(i) + "=v"
	}
	return out
}

func witnessList(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = map[string]any{"test_id": "t" + strconv.Itoa(i), "basis": "declared"}
	}
	return out
}
