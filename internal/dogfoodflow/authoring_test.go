// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"bytes"
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
 root, base := preflightRepository(t, nil)
 code, _, stderr, calls := preflightRun(t, root, ChangeOptions{Base:base, Citations:writeInput(t,"1\tpre.go\t1:1\tspecification\n"), Verify:"go test -run 'TestA|TestB' .", AggregateOverLimit:true})
 if code != 2 || len(calls) != 0 || !strings.Contains(stderr,"REFUSE aggregate-verify-argv-unsupported") || !strings.Contains(stderr,"typed argv") { t.Fatalf("code=%d calls=%v stderr=%s",code,calls,stderr) }
}
