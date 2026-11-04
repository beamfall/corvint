package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TestDWCV0753VerifySyntaxCheckMatchesThePreflight (V1-3615 friction from
// rdp-w33): the up-front VERIFY check script/dogfood-finish.sh runs before any
// build refuses each malformed line with exactly the lines the inputs-check
// pre-flight prints for it, in well under a second and without a repository,
// and admits a valid file silently. Under a wrapper's --entry its fix line
// names that make target with the run's inputs (DCW-V0-730).
func TestDWCV0753VerifySyntaxCheckMatchesThePreflight(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n")
	intents := writeInput(t, "#no-intent-declared\n")
	for _, line := range []string{
		"GOTOOLCHAIN=local go test -count=1 -run '^TestA|^TestB' ./pre\n",
		"go test -run TestA|TestB ./pre\n",
		"go test -run '^Test.*' ./pre\n",
		"go test ./pre && echo done\n",
	} {
		verify := writeInput(t, "go test -run TestA ./pre\n"+line)
		change := ChangeOptions{Base: base, Citations: plan, IntentsFile: intents, VerifyFile: verify, Outcome: "passed"}
		_, _, preflight, _ := preflightRun(t, root, change)
		var early bytes.Buffer
		start := time.Now()
		code, err := CheckVerifySyntax(context.Background(), FinishLaneOptions{Change: change}, "dogfood-inputs-check", &early)
		elapsed := time.Since(start)
		if code != 2 || err != nil || elapsed > time.Second {
			t.Fatalf("%q: CheckVerifySyntax = %d, %v after %s\n%s", line, code, err, elapsed, early.String())
		}
		at := strings.Index(preflight, "dogfood-inputs-check: REFUSE ")
		if at < 0 || !strings.HasPrefix(preflight[at:], early.String()) {
			t.Fatalf("%q: the early refusal differs from the pre-flight's:\nearly:\n%s\npre-flight:\n%s", line, early.String(), preflight)
		}
		var entry bytes.Buffer
		if code, _ := CheckVerifySyntax(context.Background(), FinishLaneOptions{Change: change, Entry: "dogfood-finish"}, "dogfood-finish-lane", &entry); code != 2 ||
			!strings.Contains(entry.String(), "rerun make dogfood-finish BASE="+base+" CITATIONS="+plan) || strings.Contains(entry.String(), "rerun corvint dogfood") {
			t.Fatalf("%q: the early refusal under --entry does not name the make target:\n%s", line, entry.String())
		}
	}
	var quiet bytes.Buffer
	if code, err := CheckVerifySyntax(context.Background(), FinishLaneOptions{Change: ChangeOptions{Base: base, VerifyFile: writeInput(t, "go test -run 'TestA|TestB' ./pre\n")}}, "dogfood-inputs-check", &quiet); code != 0 || err != nil || quiet.Len() != 0 {
		t.Fatalf("a valid file: %d, %v, %q", code, err, quiet.String())
	}
}
