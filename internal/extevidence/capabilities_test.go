package extevidence

import (
	"context"
	"strings"
	"testing"
)

// TestCapabilitiesNegotiation pins EEP-TR-012 and EEP-TR-013 under `impact`:
// an absent member changes nothing, a declared set that covers the record
// loads, and one that omits the record schema or a used evidence kind closes
// the provider as `unsupported` with a Core-authored reason.
func TestCapabilitiesNegotiation(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	ctx, root := context.Background(), indexRoot(repo.index())
	base := fixture(t, repo.head)
	declare := func(members map[string]any) []byte {
		return mutate(t, base, func(record map[string]any) { record["capabilities"] = members })
	}
	cases := []struct {
		name   string
		data   []byte
		state  string
		reason string
	}{
		{"absent", base, StateLoaded, ""},
		{"sufficient", declare(map[string]any{"schemas": []string{Schema}, "evidence_kinds": []string{"declared", "observed", "inferred", "learned"}}), StateLoaded, ""},
		{"kinds-undeclared", declare(map[string]any{"schemas": []string{Schema}}), StateLoaded, ""},
		{"missing-kind", declare(map[string]any{"evidence_kinds": []string{"declared", "observed", "learned"}}), StateUnsupported, "provider mockdocs declares capabilities without evidence kind inferred"},
		{"missing-schema", declare(map[string]any{"schemas": []string{Schema1}}), StateUnsupported, "provider mockdocs declares capabilities without schema " + Schema},
		{"empty-schemas", declare(map[string]any{"schemas": []string{}}), StateUnsupported, "provider mockdocs declares capabilities without schema " + Schema},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := decodeRecord(ctx, root, provider{}, tc.data)
			if entry.state != tc.state {
				t.Fatalf("state = %s (%s), want %s", entry.state, entry.reason, tc.state)
			}
			if tc.reason != "" && entry.reason != tc.reason {
				t.Errorf("reason = %q, want %q", entry.reason, tc.reason)
			}
		})
	}
	record, err := Decode(base)
	if err != nil || record.Capabilities != nil {
		t.Errorf("absent member must decode as nil capabilities: %v %v", record.Capabilities, err)
	}
	section := Section(ctx, repo.index(), []string{writeRecord(t, t.TempDir(), "provider.json", cases[3].data)}, nil, []string{"pkg/main.go"}, 10)
	row := section["providers"].([]any)[0].(map[string]any)
	if row["state"] != StateUnsupported || row["reason"] != cases[3].reason || row["id"] != "" {
		t.Errorf("unsupported row must be closed: %v", row)
	}
	for _, key := range []string{"results", "downstream", "verification", "unknowns"} {
		if len(section[key].([]any)) != 0 {
			t.Errorf("%s must be empty for an unsupported provider", key)
		}
	}
}

// TestCapabilitiesDecodeStrict pins that the member is validated like every
// other: an unknown member inside it, a duplicate, or a non-identifier makes
// the record invalid (EEP-TR-012).
func TestCapabilitiesDecodeStrict(t *testing.T) {
	t.Parallel()
	base := fixture(t, strings.Repeat("1", 40))
	cases := map[string]struct {
		members map[string]any
		want    string
	}{
		"unknown-member": {map[string]any{"schemas": []string{Schema}, "tools": []string{}}, "unknown field"},
		"duplicate":      {map[string]any{"evidence_kinds": []string{"observed", "observed"}}, "duplicate"},
		"non-identifier": {map[string]any{"schemas": []string{""}}, "capabilities.schemas[0]"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			data := mutate(t, base, func(record map[string]any) { record["capabilities"] = tc.members })
			if _, err := Decode(data); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
