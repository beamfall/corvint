package diagnostic

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestDiagnosticValuesScreenedAndBounded checks DRC-V0-008: a seeded secret and a 4 KiB value in
// both subject and evidence are screened and truncated with disclosure, a secret straddling the
// cut leaves no fragment, and evidence beyond 16 pairs is disclosed rather than dropped silently.
func TestDiagnosticValuesScreenedAndBounded(t *testing.T) {
	const secret = "password=hunter2-correct-horse"
	large := strings.Repeat("é", 2048)
	straddling := strings.Repeat("a", MaxValueBytes-32) + " " + secret + " " + secret
	refusal := Refusal{
		Subject: Subject{Kind: "value", Value: secret},
		Evidence: []Evidence{
			{Name: "read", Value: secret},
			{Name: "large", Value: large},
			{Name: "straddling", Value: straddling},
			{Name: "control", Value: "a\nb"},
		},
		SupportedFixes: []string{"worktree-impact.remove-path"},
	}
	emitted := refusal.Bounded()
	if strings.Contains(emitted.Subject.Value, "hunter2") || !strings.Contains(emitted.Subject.Value, "[REDACTED]") {
		t.Fatalf("subject not screened: %q", emitted.Subject.Value)
	}
	for _, pair := range emitted.Evidence {
		if strings.Contains(pair.Value, "hunt") || len(pair.Value) > MaxValueBytes || !utf8.ValidString(pair.Value) {
			t.Fatalf("evidence %q not screened or bounded: %q (%d bytes)", pair.Name, pair.Value, len(pair.Value))
		}
	}
	if value := emitted.Evidence[1].Value; !strings.HasSuffix(value, "bytes]") || !strings.Contains(value, "...[truncated ") {
		t.Fatalf("4 KiB evidence cut without disclosure: %q", value[len(value)-40:])
	}
	if emitted.Evidence[3].Value != "a\\u000ab" {
		t.Fatalf("control character not escaped: %q", emitted.Evidence[3].Value)
	}
	largeSubject := Refusal{Subject: Subject{Kind: "value", Value: large}, Terminal: "absent-evidence"}.Bounded()
	if len(largeSubject.Subject.Value) > MaxValueBytes || !strings.Contains(largeSubject.Subject.Value, "...[truncated ") {
		t.Fatalf("4 KiB subject not bounded with disclosure: %d bytes", len(largeSubject.Subject.Value))
	}
	if empty := (Refusal{Subject: Subject{Kind: "value"}, Terminal: "absent-evidence"}).Bounded(); empty.Subject.Value != EmptySubject {
		t.Fatalf("empty subject emitted as %q", empty.Subject.Value)
	}
	if refusal.Subject.Value != secret {
		t.Fatal("Bounded mutated its receiver")
	}

	many := make([]Evidence, MaxEvidence+4)
	for position := range many {
		many[position] = Evidence{Name: "count", Value: "1"}
	}
	bounded := Refusal{Subject: Subject{Kind: "argument", Value: "--limit"}, Evidence: many, Terminal: "absent-evidence"}.Bounded()
	last := bounded.Evidence[len(bounded.Evidence)-1]
	if len(bounded.Evidence) != MaxEvidence || last != (Evidence{Name: "evidence-truncated", Value: "5"}) {
		t.Fatalf("evidence bound: %d pairs, last %+v", len(bounded.Evidence), last)
	}
}

// TestDiagnosticValueCutKeepsControlEscapesWhole checks that the DRC-V0-008 cut never leaves a
// partial `\uXXXX` control escape before the disclosure, at every cut offset inside the escape.
func TestDiagnosticValueCutKeepsControlEscapesWhole(t *testing.T) {
	const escape = `\u000a`
	for shift := 0; shift < len(escape); shift++ {
		emitted := boundValue(strings.Repeat("a", shift) + strings.Repeat("\n", MaxValueBytes))
		kept, _, _ := strings.Cut(emitted, "...[truncated ")
		escapes := strings.TrimLeft(kept, "a")
		if escapes != strings.Repeat(escape, len(escapes)/len(escape)) {
			t.Fatalf("shift %d: cut splits a control escape: %q", shift, emitted[len(emitted)-40:])
		}
	}
}
