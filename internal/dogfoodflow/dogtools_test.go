// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// DTL-V0-005: split spec-first commits, new files, competing IDs, and missing historical support.
func TestDTLV0005SameChangeRefinementNewFiles(t *testing.T) {
	t.Run("DTL-V0-005", func(t *testing.T) {
		root := t.TempDir()
		testGit(t, root, "init", "-q")
		spec := "# X\n\n## Requirements\n\n- `XYZ-V0-001`: the widget answers.\n" + numbered("padding", 10)
		writeTree(t, root, map[string]string{"docs/specs/x.md": spec})
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "base")
		base := testGit(t, root, "rev-parse", "HEAD")
		writeTree(t, root, map[string]string{"docs/specs/x.md": spec + "\n- `XYZ-V0-002`: a new unrelated contract.\n- `XYZ-V0-003`: (refines XYZ-V0-001) the widget answers twice.\n"})
		testGit(t, root, "commit", "-qam", "spec first")
		writeTree(t, root, map[string]string{"new.go": "package x\n// XYZ-V0-002 and XYZ-V0-003.\nfunc New() {}\n", "unsupported.go": "package x\n// XYZ-V0-002.\nfunc Unsupported() {}\n"})
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "code")
		var out, notes bytes.Buffer
		if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &out, &notes); code != 0 || err != nil {
			t.Fatalf("%d %v %s", code, err, &notes)
		}
		if !strings.Contains(out.String(), "\tdocs/specs/x.md\t5:5\tspecification") || !strings.Contains(notes.String(), "same-change requirement XYZ-V0-003 refines") {
			t.Fatalf("out=%s notes=%s", &out, &notes)
		}
		if !strings.Contains(out.String(), "\tunknown\tinsufficient-evidence\t") {
			t.Fatalf("unrelated requirement gained historical support: %s", &out)
		}
	})
}

// TestDTLV0004RetryKeepsStopBeforeSeal: a refused run that opted out of the seal (make SEAL=0 or
// --seal no) prints a retry that keeps the opt-out, so following the fix line never seals; a
// sealing run's retry is unchanged, and the input pre-flight's own entry takes no seal word.
func TestDTLV0004RetryKeepsStopBeforeSeal(t *testing.T) {
	t.Run("DTL-V0-004", func(t *testing.T) {
		root, base := preflightRepository(t, nil)
		plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
		unsorted := writeInput(t, "docs/b.md\ndocs/a.md\n")
		verify := writeInput(t, "go version\n")
		vars := " CITATIONS=" + plan + " INTENTS=" + unsorted + " VERIFY=" + verify + " OUTCOME=passed\n"
		flags := " --citations " + plan + " --intents " + unsorted + " --verify " + verify + " --outcome passed\n"
		for _, tc := range []struct {
			name, entry      string
			preflight, stop  bool
			want, mustNotSay string
		}{
			{"make dogfood-finish SEAL=0", "dogfood-finish", false, true, "rerun make dogfood-finish BASE=" + base + " SEAL=0" + vars, ""},
			{"make dogfood-finish SEAL=0 pre-flight", "dogfood-finish", true, true, "rerun make dogfood-finish BASE=" + base + " SEAL=0" + vars, ""},
			{"make dogfood-closeout SEAL=0", "dogfood-closeout", true, true, "rerun make dogfood-closeout BASE=" + base + " SEAL=0" + vars, ""},
			{"finish-lane --seal no", "", false, true, "rerun corvint dogfood finish-lane --base " + base + flags[:len(flags)-1] + " --seal no\n", ""},
			{"inputs-check --seal no", "", true, true, "rerun corvint dogfood inputs-check --base " + base + flags, "--seal"},
			{"make dogfood-inputs-check", "dogfood-inputs-check", true, true, "rerun make dogfood-inputs-check BASE=" + base + vars, "SEAL="},
			{"sealing make dogfood-finish", "dogfood-finish", false, false, "rerun make dogfood-finish BASE=" + base + vars, "SEAL="},
			{"sealing finish-lane", "", false, false, "rerun corvint dogfood finish-lane --base " + base + "\n", "--seal"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				code, stdout, stderr, calls := entryRun(t, FinishLaneOptions{Preflight: tc.preflight, Entry: tc.entry, StopBeforeSeal: tc.stop, Change: ChangeOptions{
					Root: root, Base: base, Citations: plan, IntentsFile: unsorted, VerifyFile: verify, Outcome: "passed"}})
				if code != 2 || calls != 0 || stdout != "" || !strings.Contains(stderr, tc.want) {
					t.Fatalf("code=%d steps=%d stdout=%q stderr=\n%s\nwant %q", code, calls, stdout, stderr, tc.want)
				}
				if tc.mustNotSay != "" && strings.Contains(stderr, tc.mustNotSay) {
					t.Fatalf("retry names %q:\n%s", tc.mustNotSay, stderr)
				}
			})
		}
	})
}
