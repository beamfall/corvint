package main

import (
	"fmt"
	"strings"
	"testing"
)

// DCW-V0-446 (V1-3167): finish-lane binds a 300-hunk commit from one 300-row
// plan end to end, staging it through the DCW-V0-037 route without manual
// plans; at BASE the plan is refused citation-plan-row-limit-exceeded.
func TestDogfoodFinishLaneDCWV0446StagesOversizedPlan(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, _ := portableDogfoodRepo(t)
	const hunks = 300
	var before, after, plan strings.Builder
	for i := 0; i < hunks; i++ {
		fmt.Fprintf(&before, "// item %d old\n", i)
		fmt.Fprintf(&after, "// item %d new\n", i)
		for j := 0; j < 8; j++ {
			line := fmt.Sprintf("// unchanged %d %d\n", i, j)
			before.WriteString(line)
			after.WriteString(line)
		}
		// Two spans, so the whole plan is not uniform (DCW-V0-048).
		span := "1:5"
		if i%2 == 1 {
			span = "5:5"
		}
		fmt.Fprintf(&plan, "%d\tintent.md\t%s\tspecification\n", i+1, span)
	}
	cemWrite(t, root, "fixture/many.go", "package fixture\n"+before.String())
	cemGit(t, root, "add", "fixture/many.go")
	cemGit(t, root, "commit", "-qm", "base for oversized patch")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/many.go", "package fixture\n"+after.String())
	cemGit(t, root, "commit", "-qam", "one oversized commit")
	env, args := finishLaneInputs(t, plan.String(), "intent.md\n")
	code, stdout, stderr := run.exec(t, root, append(env[:3:3], finishLaneIdentity...), append([]string{"dogfood", "finish-lane", "--base", base}, args...)...)
	if code != 0 {
		t.Fatalf("finish-lane exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, want := range []string{
		"dogfood-finish-lane: prepare PASS hunks=300 stage=1/2\n",
		"dogfood-finish-lane: prepare PASS hunks=300 stage=2/2\n",
		"dogfood-finish-lane: bind PASS stage=1/2\n",
		"dogfood-finish-lane: bind PASS stage=2/2\n",
		"dogfood-finish-lane: rebind PASS\n",
		"hunks=300 supported=300 unknown=0 mechanical=0",
		"dogfood-check: PASS\n",
		"dogfood-seal: PASS sealed=.corvint/changes/",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("finish-lane stdout lacks %q:\n%s\nstderr:\n%s", want, stdout, stderr)
		}
	}
	if subjects := finishLaneSubjects(t, root, base); subjects != "chore: seal change evidence\nchore: bind change evidence\none oversized commit" {
		t.Fatalf("subjects %q", subjects)
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("status %q", status)
	}
}
