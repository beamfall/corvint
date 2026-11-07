// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// DCW-V0-7401: a quoted or bare caret has identical literal argv semantics.
func TestDCWV07401CaretAndVerifyDraft(t *testing.T) {
	for _, pattern := range []string{"^TestExample$", "'^TestExample$'", `"^TestExample$"`} {
		args := verifyLineArguments("go test -run " + pattern + " ./example")
		if err := validateVerify(args); err != nil || args[0] != "--verify-argv-json" {
			t.Fatalf("%s: %v %v", pattern, args, err)
		}
	}
	t.Setenv("DOGFOOD_VERIFY_FILE", "")
	t.Setenv("DOGFOOD_VERIFY", "go test -run TestA|TestB ./example\ngo test -run ^TestExample$ ./example\necho nope; exit 1")
	var out bytes.Buffer
	p := &proposal{flow: flow{stderr: &out}}
	p.draftVerify()
	if !strings.Contains(out.String(), "VERIFY: go test -run 'TestA|TestB' ./example") || !strings.Contains(out.String(), "VERIFY: go test -run ^TestExample$ ./example") || strings.Contains(out.String(), "echo nope") {
		t.Fatal(out.String())
	}
}

// DCW-V0-7403: the string-only aggregate receipt cannot carry typed argv.
func TestDCWV07403AggregateArgvRefusedBeforePrepare(t *testing.T) {
	files := map[string]string{"pre_test.go": "package pre\nimport \"testing\"\nfunc TestA(t *testing.T) {}\nfunc TestB(t *testing.T) {}\n"}
	for i := 0; i < 201; i++ {
		files[fmt.Sprintf("source/f%03d.go", i)] = "package source\n"
	}
	root, base := preflightRepository(t, files)
	code, _, stderr, calls := preflightRun(t, root, ChangeOptions{Base: base, Citations: writeInput(t, "1\tpre.go\t1:1\tspecification\n"), VerifyFile: writeInput(t, "\ngo test -run 'TestA|TestB' .\n"), AggregateOverLimit: true})
	if code != 2 || len(calls) != 0 || !strings.Contains(stderr, "REFUSE aggregate-verify-argv-unsupported") || !strings.Contains(stderr, "typed argv") || !strings.Contains(stderr, " line 2\n") || !strings.Contains(stderr, "text: go test -run 'TestA|TestB' .") || strings.Contains(stderr, "readable verify file") {
		t.Fatalf("code=%d calls=%v stderr=%s", code, calls, stderr)
	}
}

// DCW-V0-7403: fallback permission does not select an aggregate for a small change.
func TestDCWV07403SmallChangeAdmitsArgv(t *testing.T) {
	root, base := preflightRepository(t, map[string]string{"pre_test.go": "package pre\nimport \"testing\"\nfunc TestA(t *testing.T) {}\nfunc TestB(t *testing.T) {}\n"})
	code, _, stderr, calls := preflightRun(t, root, ChangeOptions{Base: base, Citations: writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n3\tunknown\tinsufficient-evidence\tnew test fixture\n"), Verify: "go test -run 'TestA|TestB' .", AggregateOverLimit: true})
	if code != 0 || len(calls) == 0 {
		t.Fatalf("code=%d calls=%v stderr=%s", code, calls, stderr)
	}
}

// DCW-V0-7401: both input forms name the actual line and text of a refusal.
func TestDCWV07401VerifyRefusalText(t *testing.T) {
	for _, file := range []bool{false, true} {
		root, base := preflightRepository(t, nil)
		options := ChangeOptions{Base: base, Citations: writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n"), Verify: "go version\necho bad; exit 1"}
		if file {
			options.VerifyFile = writeInput(t, options.Verify)
			options.Verify = ""
		}
		code, _, stderr, calls := preflightRun(t, root, options)
		if code != 2 || len(calls) != 0 || !strings.Contains(stderr, "line 2\n") || !strings.Contains(stderr, "text: echo bad; exit 1") {
			t.Fatalf("file=%t code=%d calls=%v stderr=%s", file, code, calls, stderr)
		}
	}
}
