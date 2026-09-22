package extevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const obligationsCEM = `{"spec":"cem/0.2","baseRevision":"` + "1111111111111111111111111111111111111111" + `","excludedPath":".corvint/change.cem.json","patchSha256":"` + "2222222222222222222222222222222222222222222222222222222222222222" + `","evidence":[],"hunks":[
{"id":"hunk:sha256:aaaa","path":"pkg/main.go","disposition":"supported"},
{"id":"hunk:sha256:bbbb","path":"docs/readme.md","disposition":"unknown"}]}`

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
	document, err := Obligations([]byte(obligationsCEM), obligationsReceipt(false), 64)
	if err != nil {
		t.Fatal(err)
	}
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
	document, err := Obligations([]byte(obligationsCEM), obligationsReceipt(false), 64)
	if err != nil {
		t.Fatal(err)
	}
	rows := associationsOf(obligationsHunks(t, document)[1])
	if len(rows) != 1 || rows[0]["kind"] != "unknown" || rows[0]["code"] != "no-external-evidence" {
		t.Fatalf("hunk without evidence: %v", rows)
	}
	unavailable := bytes.Replace(obligationsReceipt(false), []byte(`"state":"loaded","reason":""`), []byte(`"state":"unavailable","reason":"missing file"`), 1)
	document, err = Obligations([]byte(obligationsCEM), unavailable, 64)
	if err != nil {
		t.Fatal(err)
	}
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
	document, err = Obligations([]byte(obligationsCEM), []byte(`{"ok":true,"mutates":false,"tool":"impact","context":{"profile":"impact"}}`), 64)
	if err != nil {
		t.Fatal(err)
	}
	rows = associationsOf(obligationsHunks(t, document)[0])
	if document["state"] != ObligationsUnknown || len(rows) != 1 || rows[0]["code"] != "no-external-section" {
		t.Fatalf("receipt without external: state %v rows %v", document["state"], rows)
	}
}

// EFO-V0-003, EFO-V0-007: the binding digests hash the raw inputs, hunk ids
// keep the CEM's ids and order, and the id is recomputable from the body.
func TestObligationsBindingAndIdentity(t *testing.T) {
	t.Parallel()
	cem, receipt := []byte(obligationsCEM), obligationsReceipt(false)
	document, err := Obligations(cem, receipt, 64)
	if err != nil {
		t.Fatal(err)
	}
	binding := document["binding"].(map[string]any)
	cemSum, impactSum := sha256.Sum256(cem), sha256.Sum256(receipt)
	if binding["cem_sha256"] != hex.EncodeToString(cemSum[:]) || binding["impact_sha256"] != hex.EncodeToString(impactSum[:]) {
		t.Fatalf("binding = %v", binding)
	}
	if binding["base_revision"] != strings.Repeat("1", 40) || binding["excluded_path"] != ".corvint/change.cem.json" {
		t.Fatalf("binding = %v", binding)
	}
	hunks := obligationsHunks(t, document)
	if len(hunks) != 2 || hunks[0]["hunk"] != "hunk:sha256:aaaa" || hunks[1]["hunk"] != "hunk:sha256:bbbb" || hunks[1]["disposition"] != "unknown" {
		t.Fatalf("hunks = %v", hunks)
	}
	id := document["id"].(string)
	delete(document, "id")
	body, err := gokernel.CanonicalJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if id != "obligations:sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("id %s is not the digest of the body", id)
	}
}

// EFO-V0-008: reversed receipt rows produce identical bytes, and the limit
// counts every omitted association.
func TestObligationsDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	encode := func(receipt []byte, limit int) []byte {
		document, err := Obligations([]byte(obligationsCEM), receipt, limit)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := gokernel.CanonicalJSON(document)
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
	delete(forwardDocument, "binding")
	delete(reversedDocument, "binding")
	delete(forwardDocument, "id")
	delete(reversedDocument, "id")
	forwardBody, _ := gokernel.CanonicalJSON(forwardDocument)
	reversedBody, _ := gokernel.CanonicalJSON(reversedDocument)
	if !bytes.Equal(forwardBody, reversedBody) {
		t.Fatalf("row order reached the output:\n%s\n%s", forwardBody, reversedBody)
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

// EFO-V0-002: a non-cem/0.2 document or a non-impact receipt is refused.
func TestObligationsRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	cases := map[string][2][]byte{
		"cem 0.1":        {[]byte(`{"spec":"cem/0.1","hunks":[]}`), obligationsReceipt(false)},
		"cem not json":   {[]byte(`{`), obligationsReceipt(false)},
		"affected":       {[]byte(obligationsCEM), []byte(`{"ok":true,"tool":"affected","context":{}}`)},
		"receipt broken": {[]byte(obligationsCEM), []byte(`[]`)},
	}
	for name, inputs := range cases {
		document, err := Obligations(inputs[0], inputs[1], 64)
		var kernelErr *gokernel.Error
		if document != nil || err == nil || !errors.As(err, &kernelErr) || kernelErr.Code != "invalid-obligations-input" {
			t.Errorf("%s: document %v err %v", name, document, err)
		}
	}
}

// EFO-V0-009: the sidecar carries no file body and names its untrusted text.
func TestObligationsPrivate(t *testing.T) {
	t.Parallel()
	document, err := Obligations([]byte(obligationsCEM), obligationsReceipt(false), 64)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("func StableValue")) || bytes.Contains(encoded, []byte(`"body"`)) {
		t.Fatalf("sidecar carries a body: %s", encoded)
	}
	fields := document["untrusted_text_fields"].([]any)
	if len(fields) != 3 || fields[1] != "hunks[].associations[].relation.rule" {
		t.Fatalf("untrusted fields = %v", fields)
	}
	if !strings.Contains(document["note"].(string), "never reads this sidecar") {
		t.Fatalf("note = %v", document["note"])
	}
}
