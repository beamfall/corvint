package dogfoodflow

import (
	"strings"
	"testing"
)

// TestDCWV0722InputsCheckNamesTheRefusedLineItQuotes (V1-4105): with two
// refused lines whose commands sort in the opposite order to the file, the
// refusal names the first refused line and quotes that line's command, not
// the first refused command in sorted order.
func TestDCWV0722InputsCheckNamesTheRefusedLineItQuotes(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	verify := writeInput(t, "go version\nzz go test ./pre *\naa go test ./pre ;\n")
	code, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: verify})
	if code != 2 || len(labels) != 0 {
		t.Fatalf("code=%d steps=%q stdout=%q stderr=%q", code, labels, stdout, stderr)
	}
	if !strings.Contains(stderr, "  input: --verify "+verify+" line 2\n") {
		t.Errorf("stderr does not name line 2:\n%s", stderr)
	}
	if !strings.Contains(stderr, `"zz go test ./pre *"`) || strings.Contains(stderr, `"aa go test ./pre ;"`) {
		t.Errorf("stderr does not quote line 2's command alone:\n%s", stderr)
	}
}

// TestDCWV0722InputsCheckHintsQuotingForAnUnquotedCaret (V1-4105): an
// unquoted anchored -run name is refused with the quoted form that admits it.
func TestDCWV0722InputsCheckHintsQuotingForAnUnquotedCaret(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	verify := writeInput(t, "go version\ngo test -run ^TestA ./pre\n")
	code, _, stderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: verify})
	if code != 2 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{
		"  input: --verify " + verify + " line 2\n",
		"the -run word ^TestA is unquoted, and ^ and $ are admitted only inside quotes: write -run '^TestA$' for the one exact name, or -run TestA for every test whose name contains TestA (DCW-V0-722)",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	for _, line := range []string{"go test -run TestA ./pre", "go test -run '^TestA$' ./pre"} {
		if caretRunRefusal(line) != "" {
			t.Errorf("caretRunRefusal(%q) names a refusal", line)
		}
	}
}
