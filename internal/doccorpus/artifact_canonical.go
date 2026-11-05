// SPDX-License-Identifier: Apache-2.0

package doccorpus

import (
	"encoding/json/jsontext"
	"strconv"
)

// appendCanonicalRecord is a typed equivalent of EncodeTo for the two dominant
// split members (DCR-V1-8001). It uses the JSON encoder's own string quoting,
// preserves declaration order, and reuses one worker-local buffer. The closed
// decoder still runs first; this only replaces the subsequent byte comparison.
// Other record types continue through EncodeTo.
func appendCanonicalRecord(b []byte, value any) ([]byte, bool) {
	switch v := value.(type) {
	case *Claim:
		b = append(b, '{')
		b = canonicalStrings(b, "id", v.ID, "subject", v.Subject, "text", v.Text, "provider", v.Provider)
		b = append(b, `,"evidence":`...)
		b = canonicalEvidence(b, &v.Evidence)
	case *Subject:
		b = append(b, '{')
		b = canonicalStrings(b, "id", v.ID, "kind", v.Kind, "name", v.Name, "provider", v.Provider)
		b = append(b, `,"evidence":`...)
		b = canonicalEvidence(b, &v.Evidence)
		if len(v.AlternateIDs) > 0 {
			b = append(b, `,"alternate_ids":`...)
			b = canonicalStringArray(b, v.AlternateIDs)
		}
	default:
		return b, false
	}
	return append(b, '}'), true
}

// Names are fixed ASCII JSON names from the types above. Decoded strings are
// valid UTF-8, so AppendQuote cannot fail here; it has the exact default JSON
// escaping policy used by EncodeTo, including controls, HTML and non-ASCII.
func canonicalStrings(b []byte, pairs ...string) []byte {
	for i := 0; i < len(pairs); i += 2 {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '"')
		b = append(b, pairs[i]...)
		b = append(b, `":`...)
		b, _ = jsontext.AppendQuote(b, pairs[i+1])
	}
	return b
}

func canonicalStringArray(b []byte, values []string) []byte {
	b = append(b, '[')
	for i, s := range values {
		if i > 0 {
			b = append(b, ',')
		}
		b, _ = jsontext.AppendQuote(b, s)
	}
	return append(b, ']')
}

func canonicalEvidence(b []byte, e *Evidence) []byte {
	b = append(b, '{')
	b = canonicalStrings(b, "reported_trust", e.ReportedTrust, "derivation", e.Derivation,
		"trust", e.Trust, "state", e.State, "freshness", e.Freshness)
	b = append(b, `,"anchors":[`...)
	for i := range e.Anchors {
		if i > 0 {
			b = append(b, ',')
		}
		a := &e.Anchors[i]
		b = append(b, '{')
		b = canonicalStrings(b, "repository", a.Repository, "revision", a.Revision,
			"path", a.Path, "blob", a.Blob, "sha256", a.SHA256)
		b = append(b, `,"start_line":`...)
		b = strconv.AppendInt(b, int64(a.Start), 10)
		b = append(b, `,"end_line":`...)
		b = strconv.AppendInt(b, int64(a.End), 10)
		b = append(b, ',')
		b = canonicalStrings(b, "span_sha256", a.SpanSHA256, "symbol", a.Symbol,
			"region", a.Region, "authority", a.Authority, "evidence_kind", a.Kind, "reason", a.Reason)
		b = append(b, '}')
	}
	b = append(b, `],`...)
	b = canonicalStrings(b, "unknown", e.Unknown)
	b = append(b, `,"limitations":`...)
	b = canonicalStringArray(b, e.Limitations)
	return append(b, '}')
}
