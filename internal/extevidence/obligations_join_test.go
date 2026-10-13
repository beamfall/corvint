package extevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// obligationsTestHunks are the change hunks the shared impact join reads; the
// stable sidecar passes a verified cem/1.0 map's hunks the same way.
var obligationsTestHunks = []wire.Hunk{
	{ID: "hunk:sha256:aaaa", Path: "pkg/main.go", Disposition: "supported"},
	{ID: "hunk:sha256:bbbb", Path: "docs/readme.md", Disposition: "unknown"},
}

// joined runs the shared impact join over the test hunks and returns its
// state and hunk rows in the sidecar's member names.
func joined(t *testing.T, receipt []byte, limit int) map[string]any {
	t.Helper()
	join, err := joinImpact(receipt, obligationsTestHunks, limit)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"state": join.state, "state_reason": join.reason, "providers": join.providers, "hunks": join.hunks}
}

func obligationsReceipt(reverse bool) []byte {
	rows := []string{
		`{"authority":"external-provider","provider":"mockdocs","entity":"mockdocs:cap-stable-value","kind":"capability","summary":"Exposes the stable value.","path":"pkg/main.go","relation":{"from":"path:pkg/main.go","to":"mockdocs:cap-stable-value","type":"implements","evidence":"declared","rule":"capability-map","reference":"docs/capabilities.md#stable-value"},"verification":"verified","reason":"changed path pkg/main.go joined by declared relation implements"}`,
		`{"authority":"external-provider","provider":"mockdocs","entity":"mockdocs:cap-other","kind":"capability","summary":"Other.","path":"pkg/main.go","relation":{"from":"path:pkg/main.go","to":"mockdocs:cap-other","type":"implements","evidence":"declared","rule":"capability-map","reference":"docs/capabilities.md#other"},"verification":"verified","reason":"changed path pkg/main.go joined by declared relation implements"}`,
	}
	if reverse {
		rows[0], rows[1] = rows[1], rows[0]
	}
	return []byte(`{"ok":true,"mutates":false,"tool":"impact","context":{"profile":"impact","external":{"schema_version":1,"authority":"external-provider",
"providers":[{"id":"mockdocs","revision":"2026-09-18.1","state":"loaded","reason":"","freshness":"equal","source":"p.json","sha256":"00"}],
"results":[` + strings.Join(rows, ",") + `],
"downstream":[{"authority":"external-provider","provider":"mockdocs","entity":"mockdocs:journey-first-run","kind":"journey","summary":"A user reads the value once.","relation":{"from":"mockdocs:cap-stable-value","to":"mockdocs:journey-first-run","type":"mockdocs:enables","evidence":"inferred","rule":"journey-graph","reference":"docs/journeys.md#first-run"},"verification":"unsupported","reason":"one inferred relation mockdocs:enables from result entity mockdocs:cap-stable-value"}],
"verification":[{"authority":"external-provider","provider":"mockdocs","entity":"mockdocs:cap-stable-value","kind":"capability","summary":"Exposes the stable value.","path":"pkg/main_test.go","relation":{"from":"path:pkg/main_test.go","to":"mockdocs:cap-stable-value","type":"verifies","evidence":"observed","rule":"test-run","reference":"ci/run/1234"},"verification":"verified","reason":"observed relation verifies from path pkg/main_test.go to listed entity mockdocs:cap-stable-value"}],
"unknowns":[],"omitted":{},"untrusted_text_fields":[]}}}`)
}

func obligationsHunks(t *testing.T, document map[string]any) []map[string]any {
	t.Helper()
	raw := document["hunks"].([]any)
	hunks := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		hunks = append(hunks, entry.(map[string]any))
	}
	return hunks
}

func associationsOf(hunk map[string]any) []map[string]any {
	raw := hunk["associations"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		out = append(out, entry.(map[string]any))
	}
	return out
}

// EFO-V0-004, EFO-V0-005: a hunk joins entities on its path, one-hop
// obligations, and required tests; each carries the reference-only members.
func TestObligationsHunkAssociations(t *testing.T) {
	t.Parallel()
	document := joined(t, obligationsReceipt(false), 64)
	if document["state"] != ObligationsComplete {
		t.Fatalf("state = %v", document["state"])
	}
	rows := associationsOf(obligationsHunks(t, document)[0])
	kinds := []string{}
	for _, row := range rows {
		kinds = append(kinds, row["kind"].(string))
	}
	if strings.Join(kinds, ",") != "entity,entity,obligation,test" {
		t.Fatalf("kinds = %v", kinds)
	}
	test := rows[3]
	if test["path"] != "pkg/main_test.go" || test["entity"] != "mockdocs:cap-stable-value" {
		t.Fatalf("test row = %v", test)
	}
	for _, row := range rows {
		relation := row["relation"].(map[string]any)
		if row["authority"] != Authority || row["provider"] != "mockdocs" || relation["reference"] == "" || row["freshness"] != FreshnessEqual || row["verification"] == "" {
			t.Fatalf("row lacks its reference-only members: %v", row)
		}
		for _, forbidden := range []string{"closes", "justifies", "score", "confidence"} {
			if _, present := row[forbidden]; present {
				t.Fatalf("row carries %s: %v", forbidden, row)
			}
		}
	}
}

// EFO-V0-006: a hunk without evidence, a provider that did not load, and a
// receipt without an external section are each an explicit unknown.
func TestObligationsExplicitUnknowns(t *testing.T) {
	t.Parallel()
	document := joined(t, obligationsReceipt(false), 64)
	rows := associationsOf(obligationsHunks(t, document)[1])
	if len(rows) != 1 || rows[0]["kind"] != "unknown" || rows[0]["code"] != "no-external-evidence" {
		t.Fatalf("hunk without evidence: %v", rows)
	}
	unavailable := bytes.Replace(obligationsReceipt(false), []byte(`"state":"loaded","reason":""`), []byte(`"state":"unavailable","reason":"missing file"`), 1)
	document = joined(t, unavailable, 64)
	if document["state"] != ObligationsPartial {
		t.Fatalf("state = %v", document["state"])
	}
	for _, hunk := range obligationsHunks(t, document) {
		rows := associationsOf(hunk)
		last := rows[len(rows)-1]
		if last["kind"] != "unknown" || last["code"] != "provider-unavailable" || last["provider"] != "mockdocs" {
			t.Fatalf("hunk %v lacks the provider unknown: %v", hunk["hunk"], rows)
		}
	}
	document = joined(t, []byte(`{"ok":true,"mutates":false,"tool":"impact","context":{"profile":"impact"}}`), 64)
	rows = associationsOf(obligationsHunks(t, document)[0])
	if document["state"] != ObligationsUnknown || len(rows) != 1 || rows[0]["code"] != "no-external-section" {
		t.Fatalf("receipt without external: state %v rows %v", document["state"], rows)
	}
}

// EFO-V0-008: reversed receipt rows produce identical bytes, and the limit
// counts every omitted association.
func TestObligationsDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	encode := func(receipt []byte, limit int) []byte {
		encoded, err := gokernel.CanonicalJSON(joined(t, receipt, limit))
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	forward, reversed := encode(obligationsReceipt(false), 64), encode(obligationsReceipt(true), 64)
	var forwardDocument, reversedDocument map[string]any
	if err := json.Unmarshal(forward, &forwardDocument); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(reversed, &reversedDocument); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(forward, reversed) {
		t.Fatalf("row order reached the output:\n%s\n%s", forward, reversed)
	}
	var bounded map[string]any
	if err := json.Unmarshal(encode(obligationsReceipt(false), 1), &bounded); err != nil {
		t.Fatal(err)
	}
	first := obligationsHunks(t, bounded)[0]
	if len(associationsOf(first)) != 1 || first["omitted"].(float64) != 3 {
		t.Fatalf("limit 1: %v", first)
	}
}

// CEM-V1-210, CEM-V1-217: the retired external-frontier-obligations/0 route
// (an invocation without the stable context) admits no map. A retired cem/0.x
// map refuses with unsupported-spec and the recovery line; a cem/1.0 map or
// other bytes refuse with invalid-obligations-input naming the stable route. The shared join refuses a receipt that is not an impact
// receipt (EFO-V0-002).
func TestCEMV1210FrozenObligationsAdmitNoMap(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		cem  []byte
		code string
	}{
		"cem 0.2":  {[]byte(`{"spec":"cem/0.2","hunks":[]}`), cemcode.UnsupportedSpec},
		"cem 0.1":  {[]byte(`{"spec":"cem/0.1","hunks":[]}`), cemcode.UnsupportedSpec},
		"cem 1.0":  {[]byte(`{"spec":"cem/1.0"}`), "invalid-obligations-input"},
		"not json": {[]byte(`{`), "invalid-obligations-input"},
	}
	for name, test := range cases {
		document, err := StableObligations(context.Background(), test.cem, obligationsReceipt(false), 64, StableObligationsOptions{})
		var kernelErr *gokernel.Error
		if document != nil || err == nil || !errors.As(err, &kernelErr) || kernelErr.Code != test.code {
			t.Errorf("%s: document %v err %v", name, document, err)
			continue
		}
		if test.code == cemcode.UnsupportedSpec && !strings.Contains(kernelErr.Message, wire.RetiredSpecFix) {
			t.Errorf("%s: refusal lacks the recovery line: %v", name, err)
		}
	}
	for name, receipt := range map[string][]byte{
		"affected":       []byte(`{"ok":true,"tool":"affected","context":{}}`),
		"receipt broken": []byte(`[]`),
	} {
		_, err := joinImpact(receipt, obligationsTestHunks, 64)
		var kernelErr *gokernel.Error
		if !errors.As(err, &kernelErr) || kernelErr.Code != "invalid-obligations-input" {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

// EFO-V0-009: the joined rows carry no file body, and the sidecar names its
// untrusted text.
func TestObligationsPrivate(t *testing.T) {
	t.Parallel()
	encoded, err := gokernel.CanonicalJSON(joined(t, obligationsReceipt(false), 64))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("func StableValue")) || bytes.Contains(encoded, []byte(`"body"`)) {
		t.Fatalf("join carries a body: %s", encoded)
	}
	if len(obligationsUntrusted) != 3 || obligationsUntrusted[1] != "hunks[].associations[].relation.rule" {
		t.Fatalf("untrusted fields = %v", obligationsUntrusted)
	}
}
