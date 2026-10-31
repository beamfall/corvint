package dogfoodflow

import (
	"reflect"
	"testing"
)

// TestVerifyArgumentsDCWV0384AdmitsOneQuotedRunAlternation pins V1-3131: a
// verify line whose one quoted word is a -run alternation of test names
// becomes a typed argv entry with that word as exactly one argument, while a
// single anchored name, a bracketed regex, a second quote and any shell
// operator outside the quotes stay v1 commands the recorder refuses (the
// anchored and double-quoted alternations DCW-V0-801 admits are pinned below).
func TestVerifyArgumentsDCWV0384AdmitsOneQuotedRunAlternation(t *testing.T) {
	got := verifyArguments([]byte("go test -count=1 ./internal/a -run 'TestA|TestB_c/sub' -v\n  go test ./x -run=TestA\n"))
	want := []string{
		"--verify-argv-json", `["go","test","-count=1","./internal/a","-run","TestA|TestB_c/sub","-v"]`,
		"--verify", "  go test ./x -run=TestA",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verifyArguments = %q, want %q", got, want)
	}
	if err := validateVerify(got); err != nil {
		t.Fatalf("admitted alternation refused: %v", err)
	}
	for _, line := range []string{
		"go test ./a -run '^TestA$'",
		"go test ./a -run 'Test[A-C]|TestD'",
		"go test ./a -run 'TestA'",
		"go test ./a -run 'TestA|TestB' -run 'TestC|TestD'",
		"go test ./a -run 'TestA|TestB'; rm -rf x",
		"go test ./a -run 'TestA|TestB' | tee log",
		"go test ./a -run 'TestA|$(id)'",
		"go test ./a -run 'TestA|TestB",
	} {
		args := verifyArguments([]byte(line + "\n"))
		if len(args) != 2 || args[0] != "--verify" {
			t.Errorf("%q became %q, want a v1 command", line, args)
			continue
		}
		if validateVerify(args) == nil {
			t.Errorf("%q passed the v1 grammar", line)
		}
	}
}

// TestVerifyArgumentsDCWV0801AdmitsAnchoredAndAttachedRunAlternation pins
// V1-3280: the -run alternation lanes write most, anchored as one group or
// per name, in double quotes or attached with =, is one typed argv word; an
// unquoted alternation stays refused and its quoted form is suggested; any
// other regex inside the quotes is still refused.
func TestVerifyArgumentsDCWV0801AdmitsAnchoredAndAttachedRunAlternation(t *testing.T) {
	for line, want := range map[string]string{
		"go test -count=1 -run '^(TestA|TestB)$' ./a":            `["go","test","-count=1","-run","^(TestA|TestB)$","./a"]`,
		"go test -run '^TestA$|^TestB_c/sub$' ./a":               `["go","test","-run","^TestA$|^TestB_c/sub$","./a"]`,
		"go test -run \"TestA|TestB\" ./a":                       `["go","test","-run","TestA|TestB","./a"]`,
		"go test -run='TestA|TestB' ./a":                         `["go","test","-run=TestA|TestB","./a"]`,
		"GOTOOLCHAIN=local go test ./a -run=\"^(TestA|TestB)$\"": `["GOTOOLCHAIN=local","go","test","./a","-run=^(TestA|TestB)$"]`,
	} {
		got := verifyArguments([]byte(line + "\n"))
		if len(got) != 2 || got[0] != "--verify-argv-json" || got[1] != want {
			t.Errorf("%q became %q, want argv %s", line, got, want)
			continue
		}
		if err := validateVerify(got); err != nil {
			t.Errorf("%q refused: %v", line, err)
		}
	}
	for _, line := range []string{
		"go test -run TestA|TestB ./a",
		"go test -run ^(TestA|TestB)$ ./a",
		"go test -run '^(TestA|TestB)' ./a",
		"go test -run '^(TestA|TestB)$|TestC' ./a",
		"go test -run '^TestA$|TestB' ./a",
		"go test -run '^(TestA|(TestB))$' ./a",
		"go test -run \"TestA|$HOME\" ./a",
		"go test -run \"TestA|TestB' ./a",
		"go test -run='TestA|TestB'x ./a",
	} {
		args := verifyArguments([]byte(line + "\n"))
		if len(args) != 2 || args[0] != "--verify" || validateVerify(args) == nil {
			t.Errorf("%q became %q, want a refused v1 command", line, args)
		}
	}
	for line, want := range map[string]string{
		"go test -run TestA|TestB ./a":           "go test -run 'TestA|TestB' ./a",
		"  go test ./a -run=^(TestA|TestB)$":     "go test ./a -run='^(TestA|TestB)$'",
		"go test -run ^TestA$|^TestB$ -v ./a":    "go test -run '^TestA$|^TestB$' -v ./a",
		"go test -run TestA ./a":                 "",
		"go test -run 'TestA|TestB' ./a":         "",
		"go test -run TestA|TestB ./a | tee log": "",
	} {
		if got := quotedRunSuggestion(line); got != want {
			t.Errorf("quotedRunSuggestion(%q) = %q, want %q", line, got, want)
		}
	}
	if got := quotedRunSuggestions(ChangeOptions{Verify: "go test ./b\ngo test -run TestA|TestB ./a"}); len(got) != 1 || got[0] != "line 2: go test -run 'TestA|TestB' ./a" {
		t.Errorf("quotedRunSuggestions = %q", got)
	}
}
