package testplan

import (
	"bytes"
	"strings"
	"testing"
)

// TestCheck covers TCN-V0-011: the exact table passes inside Markdown; any edit, reorder, added or
// dropped row, header problem, max-steps disagreement, changed evidence or INCOMPLETE plan fails.
func TestCheck(t *testing.T) {
	variations, evidence := workedExample()
	p := build(t, evidence, DefaultMaxSteps, variations...)
	table := string(p.Table())
	lines := strings.Split(strings.TrimSuffix(table, "\n"), "\n")
	markdown := func(body string) []byte {
		return []byte("# Plan\n\nReviewed by the team.\n\n" + body + "\nNotes after the table.\n| not part of it\n")
	}
	join := func(ls ...string) string { return strings.Join(ls, "\n") + "\n" }

	padded := []string{}
	for _, line := range lines {
		padded = append(padded, line+" \t ")
	}
	for name, file := range map[string][]byte{
		"exact":             []byte(table),
		"inside markdown":   markdown(table),
		"trailing spaces":   markdown(join(padded...)),
		"CRLF-free trailer": []byte(table + "\n\n"),
	} {
		if err := Check(p, file); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	edited := append([]string(nil), lines...)
	edited[3] = strings.Replace(edited[3], "V1, V4, V2", "V1, V2, V4", 1)
	reordered := append([]string(nil), lines...)
	reordered[3], reordered[4] = reordered[4], reordered[3]
	added := append(append([]string(nil), lines...), "| T003 | tests/e2e/teesheet.spec.ts | V9 | unique-context |")
	dropped := append(append([]string(nil), lines[:4]...), lines[5:]...)
	reason := append([]string(nil), lines...)
	reason[4] = strings.Replace(reason[4], "destructive-change", "unique-context", 1)
	other := build(t, Evidence{Revision: strings.Repeat("f", 40), TestsDigest: evidence.TestsDigest, Documents: evidence.Documents}, DefaultMaxSteps, variations...)
	withMap := evidence
	withMap.Maps = []MapView{}
	mapped := build(t, withMap, DefaultMaxSteps, variations...)
	four := build(t, evidence, 4, variations...)
	mismatches := map[string]struct {
		plan *Plan
		file []byte
	}{
		"edited row":          {p, markdown(join(edited...))},
		"reordered rows":      {p, markdown(join(reordered...))},
		"added row":           {p, markdown(join(added...))},
		"dropped row":         {p, markdown(join(dropped...))},
		"changed reason":      {p, markdown(join(reason...))},
		"edited header":       {p, markdown(strings.Replace(table, "status=COMPLETE", "status=INCOMPLETE", 1))},
		"max-steps disagrees": {four, markdown(table)},
		"changed revision":    {other, markdown(table)},
		"changed maps":        {mapped, markdown(table)},
		"missing column line": {p, markdown(join(append([]string{lines[0]}, lines[2:]...)...))},
		"cut by a blank line": {p, markdown(join(lines[:4]...) + "\n" + join(lines[4:]...))},
	}
	for name, c := range mismatches {
		err := Check(c.plan, c.file)
		if code(err) != "test-plan-mismatch" {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if err := Check(p, markdown(join(dropped...))); !strings.Contains(err.Error(), "variation V5 is missing") {
		t.Errorf("dropped row message: %v", err)
	}
	if err := Check(p, markdown(join(added...))); !strings.Contains(err.Error(), "variation V9 is extra") {
		t.Errorf("added row message: %v", err)
	}
	if err := Check(p, markdown(join(edited...))); !strings.Contains(err.Error(), "table line 4") {
		t.Errorf("edited row message: %v", err)
	}

	for name, file := range map[string][]byte{
		"no header":      markdown(join(lines[1:]...)),
		"doubled header": markdown(table + "\n" + table),
		"empty":          nil,
	} {
		if err := Check(p, file); code(err) != "test-plan-header-missing" {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if err := Check(p, append([]byte(table), bytes.Repeat([]byte("x"), MaxInputBytes)...)); code(err) != "test-plan-invalid-input" {
		t.Errorf("oversized file: got %v", err)
	}

	incomplete := build(t, evidence, DefaultMaxSteps, append(variations, variation("V7", drop("changes")))...)
	if err := Check(incomplete, incomplete.Table()); code(err) != "test-plan-incomplete" {
		t.Errorf("INCOMPLETE plan: got %v", err)
	}
}
