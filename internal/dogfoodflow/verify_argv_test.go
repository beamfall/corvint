package dogfoodflow

import (
	"reflect"
	"strings"
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
		"go test ./a -run 'Test.*|TestD'",
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

// TestVerifyArgumentsDCWV0804AdmitsQuotedEnvironmentAssignments covers V1-3378:
// a verify line led by quoted NAME=value assignments is recorded as a typed
// argv whose first words are those assignments verbatim, and any other shell
// syntax after or inside them stays refused with the unsupported-verify-syntax
// remediation.
func TestVerifyArgumentsDCWV0804AdmitsQuotedEnvironmentAssignments(t *testing.T) {
	for line, want := range map[string]string{
		"HEAVY_WHOLE_REASON='x' script/run.sh go test -count=1 ./internal/a":           `["HEAVY_WHOLE_REASON=x","script/run.sh","go","test","-count=1","./internal/a"]`,
		"HEAVY_WHOLE_REASON='whole package; 60 KB of tests' script/run.sh go test ./a": `["HEAVY_WHOLE_REASON=whole package; 60 KB of tests","script/run.sh","go","test","./a"]`,
		`A="two words" GOTOOLCHAIN=local B='' go test ./a`:                             `["A=two words","GOTOOLCHAIN=local","B=","go","test","./a"]`,
		"HEAVY_WHOLE_REASON='r' script/run.sh go test -run 'TestA|TestB' ./a":          `["HEAVY_WHOLE_REASON=r","script/run.sh","go","test","-run","TestA|TestB","./a"]`,
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
	// An unquoted assignment keeps the unchanged v1 command form.
	if got := verifyArguments([]byte("GOTOOLCHAIN=local go test ./a\n")); len(got) != 2 || got[0] != "--verify" {
		t.Errorf("unquoted assignment became %q, want a v1 command", got)
	}
	for _, line := range []string{
		"HEAVY_WHOLE_REASON='x' script/run.sh go test ./a | tee log",
		"HEAVY_WHOLE_REASON='x' script/run.sh go test -run '^TestA$' ./a",
		`A="$HOME" go test ./a`,
		"A=\"`id`\" go test ./a",
		`A="x\" go test ./a`,
		"A='it's' go test ./a",
		"A='x'y go test ./a",
		"1A='x' go test ./a",
		"go test ./a A='x'",
		"A='x'",
	} {
		args := verifyArguments([]byte(line + "\n"))
		if err := validateVerify(args); args[0] != "--verify" || err == nil || !strings.Contains(err.Error(), "unsupported shell syntax") {
			t.Errorf("%q became %q (%v), want a refused v1 command", line, args, err)
		}
	}
	if got := quotedRunFix(""); got != "" {
		t.Errorf("quotedRunFix(\"\") = %q", got)
	}
}

// DCW-V0-991 (V1-3642): a quoted -run word may hold bracket classes of name
// characters, alone or in an alternation, and is recorded as one argv word;
// any other quoted -run word is refused with a reason naming its first
// unadmitted character, or its shape when every character is admitted.
func TestVerifyArgumentsDCWV0991AdmitsQuotedBracketClass(t *testing.T) {
	for line, want := range map[string]string{
		"go test -run 'TestX88[45]_' ./a":           `["go","test","-run","TestX88[45]_","./a"]`,
		"go test -run=\"TestX[0-9]$x\" ./a":         "",
		"go test -run '^(TestX88[45]_|TestY)$' ./a": `["go","test","-run","^(TestX88[45]_|TestY)$","./a"]`,
		"go test ./a -run=\"Test[AB]c\"":            `["go","test","./a","-run=Test[AB]c"]`,
		"go test -run '[45]_' ./a":                  `["go","test","-run","[45]_","./a"]`,
		"go test -run 'TestA[45]|TestB[1]' -v ./a":  `["go","test","-run","TestA[45]|TestB[1]","-v","./a"]`,
	} {
		got := verifyArguments([]byte(line + "\n"))
		if want == "" {
			if got[0] != "--verify" || validateVerify(got) == nil {
				t.Errorf("%q became %q, want refused", line, got)
			}
			continue
		}
		if len(got) != 2 || got[0] != "--verify-argv-json" || got[1] != want {
			t.Errorf("%q became %q, want argv %s", line, got, want)
			continue
		}
		if err := validateVerify(got); err != nil {
			t.Errorf("%q refused: %v", line, err)
		}
	}
	for line, want := range map[string]string{
		"go test -run 'TestX[^4]' ./a":   `is not a single name with a bracket class`,
		"go test -run 'TestX.*' ./a":     `holds "*" at byte 6`,
		"go test -run 'Test[a b]' ./a":   `holds " " at byte 6`,
		"go test -run '^TestA$' ./a":     `is not a single name with a bracket class`,
		"go test -run 'TestX[]' ./a":     `is not a single name with a bracket class`,
		"go test -run 'TestA|TestB' ./a": "",
		"go test -run TestA ./a":         "",
	} {
		got := quotedRunRefusal(line)
		if want == "" && got != "" || want != "" && (!strings.Contains(got, want) || !strings.Contains(got, "DCW-V0-991")) {
			t.Errorf("quotedRunRefusal(%q) = %q, want %q", line, got, want)
		}
	}
}
