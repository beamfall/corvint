package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyArgumentsAFPV07922AdmitsEnvWordBeforeQuotedAssignment covers
// V1-5015: env NAME='value' cmd is recorded as the typed argv it runs, the env
// word first and the value without its quotes, while unquoted shell syntax
// after or around it stays refused.
func TestVerifyArgumentsAFPV07922AdmitsEnvWordBeforeQuotedAssignment(t *testing.T) {
	for line, want := range map[string]string{
		"env LANE_SCRIPTS_GROUPS='core policy' bash script/lane-scripts_test.sh": `["env","LANE_SCRIPTS_GROUPS=core policy","bash","script/lane-scripts_test.sh"]`,
		`env A="two words" GOTOOLCHAIN=local go test ./a`:                        `["env","A=two words","GOTOOLCHAIN=local","go","test","./a"]`,
		"env A='r' go test -run 'TestA|TestB' ./a":                               `["env","A=r","go","test","-run","TestA|TestB","./a"]`,
	} {
		got := verifyArguments([]byte(line + "\n"))
		if len(got) != 2 || got[0] != "--verify-argv-json" || got[1] != want {
			t.Errorf("%q became %q, want argv %s", line, got, want)
			continue
		}
		if err := ValidateVerifyLine(line); err != nil {
			t.Errorf("%q refused: %v", line, err)
		}
	}
	// An unquoted env line keeps the unchanged v1 command form.
	if got := verifyArguments([]byte("env GOTOOLCHAIN=local go test ./a\n")); len(got) != 2 || got[0] != "--verify" || got[1] != "env GOTOOLCHAIN=local go test ./a" {
		t.Errorf("unquoted env line became %q, want the v1 command", got)
	}
	for _, line := range []string{
		"env A='x' bash script/a.sh | tee log",
		"env A='x' bash script/a.sh; rm -rf b",
		"env A='x' bash script/*.sh",
		"env A=\"$HOME\" bash script/a.sh",
		"env -i A='x' bash script/a.sh",
		"bash script/a.sh env A='x'",
	} {
		args := verifyArguments([]byte(line + "\n"))
		if err := validateVerify(args); args[0] != "--verify" || err == nil || !strings.Contains(err.Error(), "unsupported shell syntax") {
			t.Errorf("%q became %q (%v), want a refused v1 command", line, args, err)
		}
	}
}

// TestFinishLaneAFPV07923NamesFixForCommandSubstitution covers V1-5015: a
// command substitution stays refused, and the refusal names the line and the
// plain command to list instead: make go-version for the toolchain check, a
// committed script otherwise.
func TestFinishLaneAFPV07923NamesFixForCommandSubstitution(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	scratch := t.TempDir()
	plan := filepath.Join(scratch, "cites.tsv")
	if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, line, fix string }{
		{"go-version", `test "$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.3"`, "list make go-version"},
		{"backtick", "test `cat VERSION` = 1", "commit the check as a script and list it as sh path/to/script.sh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verify := filepath.Join(scratch, tc.name+".txt")
			if err := os.WriteFile(verify, []byte("make go-test\n"+tc.line+"\n"), 0o666); err != nil {
				t.Fatal(err)
			}
			steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int { return 1 }}
			var stdout, stderr bytes.Buffer
			code, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, VerifyFile: verify, Outcome: "passed"}}, &stdout, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			out := stderr.String()
			if code != 2 || !strings.HasPrefix(out, "dogfood-finish-lane: REFUSE unsupported-verify-syntax\n") {
				t.Fatalf("exit=%d stderr=%q, want REFUSE unsupported-verify-syntax", code, out)
			}
			for _, want := range []string{
				"  refused: line 2: a command substitution, $(...) or `...`, needs a shell to run it",
				"  fix: line 2: " + tc.fix,
			} {
				if !strings.Contains(out, want) {
					t.Fatalf("stderr lacks %q:\n%s", want, out)
				}
			}
		})
	}
	if why, fix := commandSubstitutionFix("make go-version"); why != "" || fix != "" {
		t.Fatalf("a line without a substitution got %q / %q", why, fix)
	}
	if err := ValidateVerifyLine("make go-version"); err != nil {
		t.Fatalf("the named fix is itself refused: %v", err)
	}
}
