package diagnostic

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestDiagnosticEnvelopeFields freezes one refusal per DRC-V0 shape and fails a planted
// out-of-vocabulary kind and a planted qualifier concatenated into subject.value.
func TestDiagnosticEnvelopeFields(t *testing.T) {
	fixtures := []struct {
		name    string
		refusal Refusal
		want    string
	}{
		{"repairable", Refusal{Subject: Subject{Kind: "value", Value: "pkg/a.go"}, SupportedFixes: []string{"worktree-impact.remove-path"}},
			`{"subject":{"kind":"value","value":"pkg/a.go"},"evidence":[],"supported_fixes":["worktree-impact.remove-path"]}`},
		{"terminal", Refusal{Subject: Subject{Kind: "repository-state", Value: "captured-revision-index"}, Terminal: "absent-evidence"},
			`{"subject":{"kind":"repository-state","value":"captured-revision-index"},"evidence":[],"supported_fixes":[],"terminal":"absent-evidence"}`},
		{"measured", Refusal{Subject: Subject{Kind: "value", Value: "pkg/a.go"}, Evidence: []Evidence{{Name: "revision", Value: "abc123"}}, SupportedFixes: []string{"worktree-impact.remove-path"}},
			`{"subject":{"kind":"value","value":"pkg/a.go"},"evidence":[{"name":"revision","value":"abc123"}],"supported_fixes":["worktree-impact.remove-path"]}`},
		{"unmeasured", Refusal{Subject: Subject{Kind: "argument", Value: "--limit"}, Evidence: []Evidence{}, SupportedFixes: []string{"impact.use-tracked-path-profile"}},
			`{"subject":{"kind":"argument","value":"--limit"},"evidence":[],"supported_fixes":["impact.use-tracked-path-profile"]}`},
	}
	for _, fixture := range fixtures {
		if err := fixture.refusal.Validate(); err != nil {
			t.Fatalf("%s: %v", fixture.name, err)
		}
		if got := marshal(t, fixture.refusal); !sameFields(t, got, fixture.want) {
			t.Fatalf("%s emitted %s, want %s", fixture.name, got, fixture.want)
		}
	}

	planted := fixtures[2].refusal
	planted.Subject.Kind = "path"
	if planted.Validate() == nil {
		t.Fatal("a subject kind outside the vocabulary validated")
	}
	concatenated := fixtures[2].refusal
	concatenated.Subject.Value = "pkg/a.go@abc123"
	if sameFields(t, marshal(t, concatenated), fixtures[2].want) {
		t.Fatal("a qualifier concatenated into subject.value matched the frozen expectation")
	}
	for _, broken := range []Refusal{
		{Subject: Subject{Kind: "value", Value: "a\nb"}, SupportedFixes: []string{"x"}},
		{Subject: Subject{Kind: "value", Value: "a"}},
		{Subject: Subject{Kind: "value", Value: "a"}, Terminal: "gave-up"},
		{Subject: Subject{Kind: "value", Value: "a"}, SupportedFixes: []string{"x"}, Terminal: "absent-evidence"},
	} {
		if broken.Validate() == nil {
			t.Fatalf("broken refusal validated: %+v", broken)
		}
	}

	var wrapped *Error
	cause := errors.New("refused")
	if !errors.As(error(&Error{Err: cause, Refusal: fixtures[0].refusal}), &wrapped) || !errors.Is(wrapped, cause) || wrapped.Error() != "refused" {
		t.Fatal("Error does not wrap its cause")
	}
}

func marshal(t *testing.T, refusal Refusal) string {
	t.Helper()
	data, err := json.Marshal(refusal)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// sameFields compares two JSON objects member by member, independent of key order.
func sameFields(t *testing.T, got, want string) bool {
	t.Helper()
	var left, right map[string]any
	if json.Unmarshal([]byte(got), &left) != nil || json.Unmarshal([]byte(want), &right) != nil {
		t.Fatalf("unparseable JSON: %s / %s", got, want)
	}
	return jsonEqual(left, right)
}

func jsonEqual(left, right any) bool {
	leftData, _ := json.Marshal(left)
	rightData, _ := json.Marshal(right)
	return string(leftData) == string(rightData)
}
