// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// RCF-V1-011: these underlying kinds are reviewed inputs, independent of the
// reflected declarations. A named type's stable spelling cannot hide retyping.
func rcfCorpusScalarKinds() map[string]reflect.Kind {
	return map[string]reflect.Kind{
		"bool": reflect.Bool, "string": reflect.String, "int": reflect.Int,
		"int64": reflect.Int64, "float64": reflect.Float64, "uint8": reflect.Uint8,
		"jstestreceipt.ExecutionState": reflect.String,
	}
}

func rcfCorpusScalarContract(types map[reflect.Type]bool, want map[string]reflect.Kind) error {
	got := map[string]reflect.Kind{}
	for typ := range types {
		kind, pinned := want[typ.String()]
		if !pinned {
			switch typ.Kind() {
			case reflect.Struct, reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
				// The field walker visits contained types (including map keys).
				continue
			case reflect.Interface:
				// Receipt.results is intentionally dynamic. Its scalar leaves are
				// NOT_PROVEN here; TestRCFV1011DocsToolOutputs checks its fixtures.
				if typ == reflect.TypeFor[any]() {
					continue
				}
			}
			return fmt.Errorf("unproven scalar form %s (%s)", typ, typ.Kind())
		}
		got[typ.String()] = typ.Kind()
		if typ.Kind() != kind {
			return fmt.Errorf("scalar %s: underlying kind got %s, want %s", typ, typ.Kind(), kind)
		}
		if err := rcfScalarEncoding(typ, kind); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("scalar kinds: got %v, want %v", got, want)
	}
	return nil
}

func rcfScalarEncoding(typ reflect.Type, kind reflect.Kind) error {
	// Only method-free scalar encodings are certified. Custom value or pointer
	// encoders need a separately reviewed contract, even if samples happen to pass.
	if typ.NumMethod() != 0 || reflect.PointerTo(typ).NumMethod() != 0 {
		return fmt.Errorf("unproven scalar methods on %s", typ)
	}
	var samples []any
	var wires []string
	switch kind {
	case reflect.Bool:
		samples, wires = []any{false, true}, []string{"false", "true"}
	case reflect.String:
		samples, wires = []any{"", "passed", "quote\"\n"}, []string{`""`, `"passed"`, `"quote\"\n"`}
	case reflect.Int, reflect.Int64:
		samples, wires = []any{0, 7, -7}, []string{"0", "7", "-7"}
	case reflect.Uint8:
		samples, wires = []any{0, 7}, []string{"0", "7"}
	case reflect.Float64:
		samples, wires = []any{0.0, 1.5, -1.5}, []string{"0", "1.5", "-1.5"}
	default:
		return fmt.Errorf("unproven scalar encoding %s (%s)", typ, kind)
	}
	for i, sample := range samples {
		value := reflect.New(typ)
		value.Elem().Set(reflect.ValueOf(sample).Convert(typ))
		for _, input := range []any{value.Elem().Interface(), value.Interface()} {
			for _, encode := range []func(any) ([]byte, error){json.Marshal, doccorpus.Encode} {
				raw, err := encode(input)
				if err != nil || strings.TrimSpace(string(raw)) != wires[i] {
					return fmt.Errorf("scalar %s JSON: got %s (%v), want %s", typ, raw, err, wires[i])
				}
			}
		}
	}
	return nil
}

type rcfRetypedState int
type rcfUnknownState bool
type rcfCustomState string

func (*rcfCustomState) MarshalJSON() ([]byte, error) { return []byte(`7`), nil }

func TestRCFV1011ScalarContractClosedForms(t *testing.T) {
	// A numeric declaration retains its named identity in the old field map.
	// The independent string-kind pin must reject it, including through containers.
	for _, typ := range []reflect.Type{
		reflect.TypeFor[rcfRetypedState](),
		reflect.TypeFor[*rcfRetypedState](),
		reflect.TypeFor[[]rcfRetypedState](),
		reflect.TypeFor[[1]rcfRetypedState](),
		reflect.TypeFor[map[rcfRetypedState]string](),
	} {
		t.Run(typ.String(), func(t *testing.T) {
			record := reflect.StructOf([]reflect.StructField{{Name: "State", Type: typ, Tag: `json:"state"`}})
			fields, seen := map[string]string{}, map[reflect.Type]bool{}
			rcfTypeFields(record, fields, seen)
			if got := fields[record.String()+".state"]; got != typ.String()+" json:state" {
				t.Fatalf("named field identity: %s", got)
			}
			want := map[string]reflect.Kind{"corpusbridge.rcfRetypedState": reflect.String, "string": reflect.String}
			if err := rcfCorpusScalarContract(seen, want); err == nil || !strings.Contains(err.Error(), "underlying kind got int, want string") {
				t.Fatalf("named scalar retyping was not rejected: %v", err)
			}
		})
	}
	for _, typ := range []reflect.Type{
		reflect.TypeFor[rcfUnknownState](), // Unknown named scalar, even a familiar bool kind.
		reflect.TypeFor[complex128](),      // Unknown scalar encoding fails closed.
		reflect.TypeFor[rcfCustomState](),  // Pointer encoder changes JSON without retyping.
	} {
		t.Run(typ.String(), func(t *testing.T) {
			want := rcfCorpusScalarKinds()
			if typ == reflect.TypeFor[rcfCustomState]() {
				want[typ.String()] = reflect.String
			}
			if err := rcfCorpusScalarContract(map[reflect.Type]bool{typ: true}, want); err == nil || !strings.Contains(err.Error(), "unproven scalar") {
				t.Fatalf("unknown form was not rejected: %v", err)
			}
		})
	}
}

func TestRCFV1011AttemptStateJSON(t *testing.T) {
	for _, state := range []jstestreceipt.ExecutionState{
		jstestreceipt.StatePassed, jstestreceipt.StateFailed, jstestreceipt.StateSkipped,
		jstestreceipt.StateFlaky, jstestreceipt.StateTimedOut, jstestreceipt.StateInterrupted,
		jstestreceipt.StateInfrastructure,
	} {
		for _, encode := range []func(any) ([]byte, error){json.Marshal, doccorpus.Encode} {
			raw, err := encode(jstestreceipt.Attempt{State: state})
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]any
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			if value, ok := wire["state"].(string); !ok || value != string(state) {
				t.Fatalf("attempt state: got %s, want JSON string %q", raw, state)
			}
		}
	}
}
