package dogfoodflow

import (
	"fmt"
	"strings"
	"testing"
)

// TestDWCV03726InputsCheckRefusesTypedVerifyOverTheTraceBound (V1-4169): a
// change admitting more changed paths than a learning trace holds records the
// aggregate receipt, which has no typed argv form, so inputs-check refuses a
// quoted -run alternation before any bind, naming the line; a plain command
// and a change under the bound are not refused for it.
func TestDWCV03726InputsCheckRefusesTypedVerifyOverTheTraceBound(t *testing.T) {
	extra := map[string]string{}
	for index := 0; index < 201; index++ {
		extra[fmt.Sprintf("many/f%03d.go", index)] = fmt.Sprintf("package many\n\nconst F%03d = %d\n", index, index)
	}
	root, base := preflightRepository(t, extra)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n")
	typed := writeInput(t, "go version\ngo test -count=1 -run 'TestA|TestB' ./pre\n")
	code, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: typed, AggregateOverLimit: true})
	output := stdout + stderr
	if code != 2 || len(labels) != 0 || !strings.Contains(output, "REFUSE aggregate-verify-argv") {
		t.Fatalf("code=%d steps=%q stdout=%s stderr=%s", code, labels, stdout, stderr)
	}
	for _, want := range []string{"  input: --verify " + typed + " line 2\n", "(DWC-V0-3726)", "nothing was prepared, bound or committed"} {
		if !strings.Contains(output, want) {
			t.Errorf("output lacks %q:\n%s", want, output)
		}
	}
	plain := writeInput(t, "go version\ngo test -count=1 -run TestA ./pre\n")
	_, stdout, stderr, _ = preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: plain, AggregateOverLimit: true})
	if strings.Contains(stdout+stderr, "aggregate-verify-argv") {
		t.Errorf("a plain verify command was refused:\n%s%s", stdout, stderr)
	}
	small, smallBase := preflightRepository(t, nil)
	_, stdout, stderr, _ = preflightRun(t, small, ChangeOptions{Base: smallBase, Citations: plan, VerifyFile: typed, AggregateOverLimit: true})
	if strings.Contains(stdout+stderr, "aggregate-verify-argv") {
		t.Errorf("a change under the bound was refused:\n%s%s", stdout, stderr)
	}
}
