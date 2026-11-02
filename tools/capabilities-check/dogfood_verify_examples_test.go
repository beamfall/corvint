package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

// DCW-V0-732 (NEW-B4, NEW-B5, V1-3862): every command example in the
// docs/DOGFOOD.md DOGFOOD_VERIFY_FILE row before "Refused" is a line the
// dogfood verify grammar admits, and every one after it is refused, so the
// documented canonical, admission-wrapped, subtest and quoted-alternation forms
// and the refused forms match the checker.
func TestDCWV0732DogfoodVerifyExamplesMatchTheChecker(t *testing.T) {
	data, err := os.ReadFile("../../docs/DOGFOOD.md")
	if err != nil {
		t.Fatal(err)
	}
	var row string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "| `DOGFOOD_VERIFY_FILE` |") {
			row = line
		}
	}
	admitted, refused, found := strings.Cut(row, "Refused")
	refused, _, _ = strings.Cut(refused, " | ")
	if !found {
		t.Fatalf("docs/DOGFOOD.md DOGFOOD_VERIFY_FILE row has no Refused examples: %q", row)
	}
	examples := regexp.MustCompile("`((?:[^`\\s]+ )?go test [^`]+)`")
	count := func(text string, admit bool) int {
		matches := examples.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			err := dogfoodflow.ValidateVerifyLine(match[1])
			if admit && err != nil || !admit && err == nil {
				t.Errorf("documented %s example %q: ValidateVerifyLine = %v", map[bool]string{true: "admitted", false: "refused"}[admit], match[1], err)
			}
		}
		return len(matches)
	}
	if got := count(admitted, true); got < 5 {
		t.Errorf("admitted examples = %d, want the canonical, wrapped, subtest and two alternation forms", got)
	}
	if got := count(refused, false); got < 2 {
		t.Errorf("refused examples = %d, want the unquoted alternation and the anchored subtest", got)
	}
}
