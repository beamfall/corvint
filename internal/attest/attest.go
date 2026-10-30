// Package attest wraps a prove document (see cmd/corvint/prove.go and
// docs/specs/falsifiable-packet-v0.md, requirement FPK-V0-012) as an in-toto
// Statement v1 (https://in-toto.io/Statement/v1), and signs that statement
// inside a DSSE envelope (https://github.com/secure-systems-lab/dsse) so an
// external gate can consume it.
package attest

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
)

// PredicateType is the in-toto predicateType URI for a falsifiable-packet/0
// proof document.
const PredicateType = "https://corvint-context.dev/attestation/falsifiable-packet/0"

// statementType is the fixed in-toto Statement v1 "_type" value.
const statementType = "https://in-toto.io/Statement/v1"

// proveProfile is the "profile" value a prove document must carry to be
// wrapped by Statement. It matches proveProfile in cmd/corvint/prove.go.
const proveProfile = "falsifiable-packet/0"

// Subject is one in-toto Statement subject: a name and a set of digests
// keyed by algorithm (values are lowercase hex, except "gitCommit" which is
// the raw commit oid).
type Subject struct {
	Name   string
	Digest map[string]string
}

// Statement builds an in-toto Statement v1 whose predicate is proveDocument
// parsed as JSON, and whose predicateType is PredicateType. It returns
// canonical JSON: object keys sorted at every level (plain byte order), no
// insignificant whitespace, UTF-8, and no trailing newline.
//
// Statement rejects a proveDocument that does not parse as JSON, is not a
// JSON object, does not have "profile":"falsifiable-packet/0", or does not
// carry a non-empty string "revision". It also rejects a repeated member name
// at any depth and invalid UTF-8, which the decoder would resolve last-wins or
// replace with U+FFFD, so the predicate would not be the document unchanged.
func Statement(proveDocument []byte, subjects []Subject) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(proveDocument))
	decoder.UseNumber()
	var predicate any
	if err := decoder.Decode(&predicate); err != nil {
		return nil, fmt.Errorf("attest: parse prove document: %w", err)
	}
	// More reports false before a stray '}' or ']', so only io.EOF proves the
	// document ended.
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("attest: prove document has trailing data after the JSON value")
	}
	if !jsontext.Value(proveDocument).IsValid() {
		return nil, errors.New("attest: prove document repeats a member name or is not valid UTF-8")
	}
	object, ok := predicate.(map[string]any)
	if !ok {
		return nil, errors.New("attest: prove document must be a JSON object")
	}
	if profile, _ := object["profile"].(string); profile != proveProfile {
		return nil, fmt.Errorf("attest: prove document must have \"profile\":%q", proveProfile)
	}
	if revision, ok := object["revision"].(string); !ok || revision == "" {
		return nil, errors.New("attest: prove document must have a non-empty \"revision\"")
	}

	subjectList := make([]any, len(subjects))
	for index, subject := range subjects {
		subjectList[index] = subjectToAny(subject)
	}

	statement := map[string]any{
		"_type":         statementType,
		"subject":       subjectList,
		"predicateType": PredicateType,
		"predicate":     object,
	}
	return canonicalMarshal(statement)
}

func subjectToAny(subject Subject) map[string]any {
	digest := make(map[string]any, len(subject.Digest))
	for algorithm, value := range subject.Digest {
		digest[algorithm] = value
	}
	return map[string]any{"name": subject.Name, "digest": digest}
}
