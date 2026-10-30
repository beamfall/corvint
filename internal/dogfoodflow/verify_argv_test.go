package dogfoodflow

import (
	"reflect"
	"testing"
)

// TestVerifyArgumentsDCWV0384AdmitsOneQuotedRunAlternation pins V1-3131: a
// verify line whose one quoted word is a -run alternation of test names
// becomes a typed argv entry with that word as exactly one argument, while an
// anchored, grouped or bracketed regex, a second quote, double quotes and any
// shell operator outside the quotes stay v1 commands the recorder refuses.
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
		"go test ./a -run '^(TestA|TestB)$'",
		"go test ./a -run '^TestA$'",
		"go test ./a -run 'Test[A-C]|TestD'",
		"go test ./a -run 'TestA'",
		"go test ./a -run 'TestA|TestB' -run 'TestC|TestD'",
		"go test ./a -run \"TestA|TestB\"",
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
