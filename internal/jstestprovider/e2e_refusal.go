package jstestprovider

import (
	"encoding/json"
	"strings"
)

// E2ERefusalSchema names the typed pre-run e2e refusal document
// (PWP-V0-034). It is written to stdout in place of a receipt when an e2e
// request is refused before any server, runner or browser starts.
const E2ERefusalSchema = "corvint-playwright-e2e-refusal/0"

// E2ERefusal is a pre-run e2e refusal with one closed refusal code and an
// optional detail. Error keeps the earlier "code" or "code: detail" text.
type E2ERefusal struct {
	Code   string
	Detail string
	// Tuple names the failing runtime tuple field and its admitted values
	// when the refusal is a tuple refusal (PWP-V0-040); nil otherwise.
	Tuple *RuntimeTupleMismatch
}

func (e *E2ERefusal) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

func refuseE2E(code string) error { return &E2ERefusal{Code: code} }

// e2eRefusalDocument is the closed refusal document shape.
type e2eRefusalDocument struct {
	Schema                  string                `json:"schema"`
	Code                    string                `json:"code"`
	Message                 string                `json:"message"`
	QualifiedRunnerVersions []string              `json:"qualifiedRunnerVersions"`
	RuntimeTuple            *RuntimeTupleMismatch `json:"runtimeTuple,omitempty"`
}

// EncodeE2ERefusal returns the canonical refusal document: compact
// encoding/json bytes in declared field order plus LF. It always names the
// exact runner versions external mode admits.
func EncodeE2ERefusal(r *E2ERefusal) []byte {
	data, _ := json.Marshal(e2eRefusalDocument{Schema: E2ERefusalSchema, Code: r.Code, Message: r.Error(), QualifiedRunnerVersions: QualifiedPlaywrightVersions(), RuntimeTuple: r.Tuple})
	return append(data, '\n')
}

// unqualifiedVersionDetail names the admitted runner versions in the
// version refusal message (PWP-V0-034).
func unqualifiedVersionDetail() string {
	return "qualified runner versions: " + strings.Join(qualifiedPlaywrightVersions, ", ")
}

// detail is the refusal text naming the failing tuple field and the exact
// values the shared table admits for it (PWP-V0-040).
func (m *RuntimeTupleMismatch) detail() string {
	return m.Field + " not admitted for " + m.Profile + "; admitted " + m.Field + " values: " + strings.Join(m.Admitted, ", ")
}
