package breakagemap

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/extevidence"
)

// TestBreakageEndpointSpanChange compares each distinct EEP /3 endpoint span declared on the
// selected source between the base and the target, beside the manifest span, words the provider
// unknown for narrowed endpoints, and refuses a /2 record carrying a span by field
// (BKM-V0-093, BKM-V0-099).
func TestBreakageEndpointSpanChange(t *testing.T) {
	m, bindings, base := fixture(t)
	var record extevidence.Record1
	if err := json.Unmarshal(m.Providers[0], &record); err != nil {
		t.Fatal(err)
	}
	v3 := record
	v3.Schema = extevidence.Schema3
	v3.Relations = nil
	for _, span := range [][2]int{{2, 2}, {1, 1}, {1, 1}, {1, 99}} {
		rel := record.Relations[0]
		rel.From.Span = &extevidence.Span1{Start: span[0], End: span[1]}
		v3.Relations = append(v3.Relations, rel)
	}
	raw, err := json.Marshal(v3)
	if err != nil {
		t.Fatal(err)
	}
	m.Providers = []json.RawMessage{raw}
	r, err := Compile(context.Background(), encode(t, m), bindings, "api:pkg/api.go:Changed", base)
	if err != nil {
		t.Fatal(err)
	}
	want := []SpanChange{{1, 1, "declared-span-unchanged"}, {1, 99, "span-unavailable"}, {2, 2, "declared-span-changed"}}
	if !reflect.DeepEqual(r.EndpointSpanChanges, want) || r.DeclaredSpanChange != "declared-span-changed" {
		t.Fatalf("endpoint spans = %+v declared = %s", r.EndpointSpanChanges, r.DeclaredSpanChange)
	}
	narrowed, generic := false, false
	for _, u := range r.Unknowns {
		narrowed = narrowed || strings.Contains(u.Reason, "narrow some endpoints to a span or declared symbol")
		generic = generic || strings.Contains(u.Reason, "bind files/entities, not the selected symbol")
	}
	if !narrowed || generic {
		t.Fatalf("narrowed provider unknown: %+v", r.Unknowns)
	}
	if r, err = Compile(context.Background(), encode(t, m), bindings, "api:pkg/api.go:Changed", ""); err != nil || r.EndpointSpanChanges != nil {
		t.Fatalf("no --base still compared endpoint spans: %+v %v", r.EndpointSpanChanges, err)
	}
	v2 := v3
	v2.Schema = extevidence.Schema2
	raw2, err := json.Marshal(v2)
	if err != nil {
		t.Fatal(err)
	}
	m.Providers = []json.RawMessage{raw2}
	if _, err := Decode(encode(t, m)); err == nil || !strings.Contains(err.Error(), "endpoint field span that needs external-evidence-provider/3") {
		t.Fatalf("/2 span refusal = %v", err)
	}
}

// TestBreakageDeclaredSpanUnavailable reports span-unavailable when the base blob is shorter than
// the declared span, and keeps an unsupported-language file change separate (BKM-V0-092).
func TestBreakageDeclaredSpanUnavailable(t *testing.T) {
	m, bindings, _ := fixture(t)
	base := m.Repositories[1].Commit
	put(t, bindings["client"], "flow.json", "{\"flow_id\":\"checkout\",\"declared\":true}\nappended\nmore\n")
	commit(t, bindings["client"])
	m.Repositories[1] = pinRepo(t, "client", bindings["client"])
	for i, s := range m.Sources {
		if s.Repository == "client" {
			m.Sources[i] = pinSource(t, m.Repositories[1], bindings["client"], s.Path)
			if s.Path == "flow.json" {
				m.Sources[i].Start, m.Sources[i].End = 3, 3
			}
		}
	}
	m.Providers = nil
	r, err := Compile(context.Background(), encode(t, m), bindings, "client:flow.json:checkout", base)
	if err != nil || r.Change != "declaration-unresolved-file-changed" || r.DeclaredSpanChange != "span-unavailable" {
		t.Fatalf("change=%s span=%s %v", r.Change, r.DeclaredSpanChange, err)
	}
}
