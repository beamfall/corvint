package main

import (
	"strings"
	"testing"
)

// V1-3343: every plan form of `affected` that accepts --format text (the worktree, --path and
// --base forms, AFP-V0-928) says so in its usage line, as its own example does.
func TestAffectedHelpNamesFormatOnEveryPlanForm_V13343(t *testing.T) {
	usage, _, _ := strings.Cut(affectedHelp, "\n\nThe command reads")
	forms := strings.Split(usage, "\n  corvint ")
	for _, want := range []string{"--path PATH [PATH ...]", "[--require-clean]\n          --base FULL_COMMIT_ID"} {
		found := false
		for _, form := range forms {
			if strings.Contains(form, want) {
				found = true
				if !strings.Contains(form, "[--format json|text]") {
					t.Errorf("the usage form %q accepts --format text but does not name it", strings.SplitN(form, "\n", 2)[0])
				}
			}
		}
		if !found {
			t.Errorf("no usage form holds %q", want)
		}
	}
}
