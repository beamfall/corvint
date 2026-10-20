package testplan

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestStepFactsNeverConflictOrOrder: step facts with different values for one key share a test,
// never meet a requires fact or a changes fact in a precedence edge, and leave every plan byte
// except the input digest, the table header that carries it and table_digest unchanged
// (TCN-V0-013).
func TestStepFactsNeverConflictOrOrder(t *testing.T) {
	stepped := []map[string]any{
		variation("V1", set("step_facts", []any{"api=429"})),
		variation("V2", set("step_facts", []any{"api=5xx", "clock=frozen"})),
		variation("V3", set("step_facts", []any{"api=network-error"})),
		variation("V4", set("requires", []any{"api=ok"})),
		variation("V5", set("changes", []any{"api=down"})),
		// An edge from V5's change to this step fact would move V6 before V5.
		variation("V6", set("step_facts", []any{"api=ok"})),
	}
	p := build(t, Evidence{}, 8, stepped...)
	want := []string{"| T001 | tests/e2e/teesheet.spec.ts | V1, V2, V3, V4, V5, V6 | - |"}
	if got := rows(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}

	plain := make([]map[string]any, len(stepped))
	for i, v := range stepped {
		copied := map[string]any{}
		for k, value := range v {
			copied[k] = value
		}
		delete(copied, "step_facts")
		plain[i] = copied
	}
	q := build(t, Evidence{}, 8, plain...)
	if p.InputDigest == q.InputDigest {
		t.Fatal("step facts must be part of the input digest")
	}
	table := strings.Replace(string(p.Table()), p.InputDigest, q.InputDigest, 1)
	if table != string(q.Table()) {
		t.Fatalf("step facts changed the table beyond its input digest:\n%s\n%s", p.Table(), q.Table())
	}
	var withFacts, without map[string]any
	if err := json.Unmarshal(p.JSON(), &withFacts); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(q.JSON(), &without); err != nil {
		t.Fatal(err)
	}
	for _, digest := range []string{"input_digest", "table_digest"} {
		delete(withFacts, digest)
		delete(without, digest)
	}
	if !reflect.DeepEqual(withFacts, without) {
		t.Fatalf("step facts changed the JSON plan beyond its digests:\n%s\n%s", p.JSON(), q.JSON())
	}

	// The same values declared as requires open one compatible set each.
	p = build(t, Evidence{}, 8,
		variation("V1", set("requires", []any{"api=429"})),
		variation("V2", set("requires", []any{"api=5xx"})),
	)
	if got := rows(p); len(got) != 2 || !strings.HasSuffix(got[0], "conflicting-state |") {
		t.Fatalf("requires baseline rows:\n%s", strings.Join(got, "\n"))
	}
}

// TestStepFactsDecoding: step facts take the requires syntax and bounds; absent and empty are the
// same declaration with the same digest, null and every malformed set refuse, and permuting the
// members keeps the digest (TCN-V0-013).
func TestStepFactsDecoding(t *testing.T) {
	absent := decode(t, inputJSON(t, variation("V1")))
	empty := decode(t, inputJSON(t, variation("V1", set("step_facts", []any{}))))
	if absent.Variations[0].StepFacts != nil || empty.Variations[0].StepFacts != nil || absent.Digest() != empty.Digest() {
		t.Fatalf("absent and empty step facts differ: %q %q", absent.Digest(), empty.Digest())
	}
	one := decode(t, inputJSON(t, variation("V1", set("step_facts", []any{"b=2", "a=1"}))))
	other := decode(t, inputJSON(t, variation("V1", set("step_facts", []any{"a=1", "b=2"}))))
	if got := one.Variations[0].StepFacts; !reflect.DeepEqual(got, []string{"a=1", "b=2"}) || one.Digest() != other.Digest() {
		t.Fatalf("step facts not kept as a sorted set: %q", got)
	}
	if !bytes.Contains(canonicalBytes(t, one), []byte(`"step_facts":["a=1","b=2"]`)) {
		t.Fatalf("canonical input omits step facts: %s", canonicalBytes(t, one))
	}
	for name, value := range map[string]any{
		"null":           nil,
		"not an array":   "api=429",
		"malformed fact": []any{"no-equals"},
		"empty key":      []any{"=v"},
		"key too long":   []any{strings.Repeat("k", 129) + "=v"},
		"one key twice":  []any{"api=429", "api=5xx"},
		"too many facts": factList(65),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeInput(inputJSON(t, variation("V1", set("step_facts", value)))); code(err) != "test-plan-invalid-input" {
				t.Fatalf("got %v, want test-plan-invalid-input", err)
			}
		})
	}
}

// TestStepFactKeyAlsoRequiredRefuses: one variation naming a key in both requires and step_facts
// refuses, whatever the values; the same key in different variations does not (TCN-V0-014).
func TestStepFactKeyAlsoRequiredRefuses(t *testing.T) {
	for _, requires := range [][]any{{"api=429"}, {"api=ok"}, {"x=1", "api=ok"}} {
		data := inputJSON(t, variation("V1", set("requires", requires), set("step_facts", []any{"api=429"})))
		if _, err := DecodeInput(data); code(err) != "test-plan-invalid-input" {
			t.Fatalf("requires %v: got %v, want test-plan-invalid-input", requires, err)
		}
	}
	decode(t, inputJSON(t,
		variation("V1", set("requires", []any{"api=ok"})),
		variation("V2", set("step_facts", []any{"api=429"})),
		variation("V3", set("requires", []any{"api=ok"}), set("step_facts", []any{"clock=frozen"})),
	))
}

// TestStepFactsInDuplicateKey: variations that differ only in a step fact are not duplicates;
// equal step facts, absent or empty included, still class (TCN-V0-015).
func TestStepFactsInDuplicateKey(t *testing.T) {
	same := func(edits ...func(map[string]any)) []func(map[string]any) {
		return append([]func(map[string]any){set("action", []any{"element:V1.act"}), set("assertion", []any{"element:V1.see"})}, edits...)
	}
	p := build(t, Evidence{}, 8,
		variation("V1", same(set("step_facts", []any{"api=429"}))...),
		variation("V2", same(set("step_facts", []any{"api=5xx"}))...),
		variation("V3", same(set("step_facts", []any{"api=429"}))...),
		variation("V4", same()...),
		variation("V5", same(set("step_facts", []any{}))...),
	)
	want := []Duplicate{{"V3", "V1"}, {"V5", "V4"}}
	if !reflect.DeepEqual(p.Duplicates, want) {
		t.Fatalf("duplicates %+v\n%s", p.Duplicates, p.Table())
	}
	if got := rows(p); got[0] != "| T001 | tests/e2e/teesheet.spec.ts | V1, V2, V4 | - |" {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}
}

func canonicalBytes(t *testing.T, in *Input) []byte {
	t.Helper()
	data, err := json.Marshal(canonicalInput(in))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestStepFactsSplitDuplicateChangesPlan: a step fact on only one of two otherwise identical
// variations splits their duplicate class, so the plan changes beyond its digests; this is the
// TCN-V0-015 exception to the TCN-V0-013 output invariance.
func TestStepFactsSplitDuplicateChangesPlan(t *testing.T) {
	act, see := set("action", []any{"element:V1.act"}), set("assertion", []any{"element:V1.see"})
	before := build(t, Evidence{}, 8, variation("V1", act, see), variation("V2", act, see))
	after := build(t, Evidence{}, 8, variation("V1", act, see), variation("V2", act, see, set("step_facts", []any{"api=429"})))

	if want := []Duplicate{{"V2", "V1"}}; !reflect.DeepEqual(before.Duplicates, want) {
		t.Fatalf("before: duplicates %+v\n%s", before.Duplicates, before.Table())
	}
	wantBefore := []string{
		"| T001 | tests/e2e/teesheet.spec.ts | V1 | unique-context |",
		"| DUPLICATE V1 | tests/e2e/teesheet.spec.ts | V2 | duplicate |",
	}
	if got := rows(before); !reflect.DeepEqual(got, wantBefore) {
		t.Fatalf("before rows:\n%s", strings.Join(got, "\n"))
	}

	if len(after.Duplicates) != 0 {
		t.Fatalf("after: a step fact on one variation must split the class, got %+v", after.Duplicates)
	}
	wantAfter := []string{"| T001 | tests/e2e/teesheet.spec.ts | V1, V2 | - |"}
	if got := rows(after); !reflect.DeepEqual(got, wantAfter) {
		t.Fatalf("after rows:\n%s", strings.Join(got, "\n"))
	}
}
