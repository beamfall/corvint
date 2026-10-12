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

// TestCheckDCWV0047DeclaredOutputChange reproduces V1-1279: a change that adds
// one member to the canonical `cem status` output can never agree with its
// base verifier. Undeclared, it still fails verifier-disagreement; declared by
// a requirement whose spec the change modifies, the tree verifier fills both
// roles and the check passes recording base-independence=NOT_PROVED with both
// stdout digests, which the report and the seal keep; a declaration that
// names an absent requirement, or one whose spec is not in the diff, refuses
// before any verifier runs, including on an empty BASE..HEAD range.
func TestCheckDCWV0047DeclaredOutputChange(t *testing.T) {
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(name, "t")
	}
	for _, name := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "t@example.invalid")
	}
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	index := "id\tfile\tline\ttitle\nA-V0-001\tdocs/specs/a-v0.md\t3\tone\nB-V0-001\tdocs/specs/b-v0.md\t3\tother\n"
	testGit(t, root, "init", "-q")
	write("docs/specs/a-v0.md", "# A\n\n- `A-V0-001`: one\n")
	write("docs/specs/b-v0.md", "# B\n\n- `B-V0-001`: other\n")
	write("docs/specs/REQUIREMENTS.tsv", index)
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	write("docs/specs/a-v0.md", "# A\n\n- `A-V0-001`: one\n- `A-V0-002`: the status output gains a member\n")
	write("docs/specs/REQUIREMENTS.tsv", index+"A-V0-002\tdocs/specs/a-v0.md\t4\tthe status output gains a member\n")
	write(".corvint/change.cem.json", "{}\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "bind")
	head := testGit(t, root, "rev-parse", "HEAD")
	write(".corvint/change.ocm-intents", string(noIntentManifest))
	write(".corvint/change.ocm-status.json", "{}\n")

	baseStatus := `{"state":"ready-for-ci","counts":{"total":1,"supported":1,"unknown":0,"mechanical":0}}` + "\n"
	treeStatus := `{"state":"ready-for-ci","counts":{"total":1,"supported":1,"unknown":0,"mechanical":0},"citation":{"profile":"added"}}` + "\n"
	runs := map[string]int{}
	verifier := func(role, status string) Runner {
		path := filepath.Join(t.TempDir(), role)
		if err := os.WriteFile(path, []byte(role), 0o755); err != nil {
			t.Fatal(err)
		}
		return Runner{Path: path, Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
			if len(args) < 2 || args[0] != "cem" || args[1] != "status" {
				t.Fatalf("%s verifier ran %q", role, args)
			}
			runs[role]++
			_, _ = io.WriteString(stdout, status)
			return 0
		}}
	}
	baseVerifier, treeVerifier := verifier("base", baseStatus), verifier("tree", treeStatus)
	reportPath := filepath.Join(root, ".corvint/dogfood-report.json")
	report := []byte("{\n" + noIntentOCMStatus + "\n" +
		`  ,"dogfoodPolicy": {"bootstrapUnknown": 0, "maximumUnknownAfterBootstrap": 0}` + "\n" +
		`  ,"dogfoodCheck": null` + "\n}\n")
	check := func(declared string, tree Runner) (*check, int, string, string) {
		t.Helper()
		if err := os.WriteFile(reportPath, report, 0o666); err != nil {
			t.Fatal(err)
		}
		clear(runs)
		var stdout, stderr bytes.Buffer
		c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root, BaseVerifier: baseVerifier, TreeVerifier: tree, OutputChange: declared}, &stdout, &stderr)
		c.base, c.target, c.gitDir = base, head, filepath.Join(root, ".git")
		c.report, c.evidence = reportPath, t.TempDir()
		code, err := boundary(c.ctx, func() int {
			c.resolveVerifiers()
			c.admitOutputChange()
			return c.verifyBinding(report)
		}, func() {})
		if err != nil {
			t.Fatal(err)
		}
		return c, code, stdout.String(), stderr.String()
	}
	readReport := func() string {
		t.Helper()
		data, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	// Undeclared, the additive member fails as before.
	if _, code, stdout, stderr := check("", treeVerifier); code != 1 || stdout != "" || stderr != "dogfood-check: FAIL verifier-disagreement\n" ||
		!strings.Contains(readReport(), `"outputsAgree": false}`) || strings.Contains(readReport(), "baseIndependence") {
		t.Fatalf("undeclared exit=%d stdout=%q stderr=%q report=%q", code, stdout, stderr, readReport())
	}

	// Declared by the requirement the change adds to a spec it modifies.
	c, code, stdout, stderr := check("A-V0-002", treeVerifier)
	independence := "base-independence=NOT_PROVED base=sha256:" + sha256Hex([]byte(baseStatus)) + " tree=sha256:" + sha256Hex([]byte(treeStatus)) + " declared=A-V0-002"
	if code != 0 || stderr != "" || !strings.Contains(stdout, "dogfood-check: SUMMARY cem=ready-for-ci ") ||
		!strings.Contains(stdout, " ocm=NOT_ASSESSED "+independence+" report=.corvint/dogfood-report.json ") ||
		!strings.HasSuffix(stdout, "dogfood-check: PASS\n") {
		t.Fatalf("declared exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if runs["base"] != 1 || runs["tree"] != 2 {
		t.Fatalf("declared verifier runs = %v; want base once, tree in both roles", runs)
	}
	recorded := `"outputsAgree": true, "baseIndependence": {"state": "NOT_PROVED", "declared": "A-V0-002", "baseStatusStdoutSha256": "sha256:` +
		sha256Hex([]byte(baseStatus)) + `", "treeStatusStdoutSha256": "sha256:` + sha256Hex([]byte(treeStatus)) + `"}}`
	if !strings.Contains(readReport(), recorded) {
		t.Fatalf("declared report=%q; want %q", readReport(), recorded)
	}

	// The seal reusing the check keeps the declaration on its PASS line.
	var sealOut bytes.Buffer
	c.stdout, c.prefix = &sealOut, "dogfood-seal"
	if code, err := boundary(c.ctx, c.seal, func() {}); err != nil || code != 0 ||
		sealOut.String() != "dogfood-seal: PASS sealed=.corvint/changes/"+head+".cem.json "+independence+"\n" {
		t.Fatalf("declared seal exit=%d err=%v stdout=%q", code, err, sealOut.String())
	}
	testGit(t, root, "reset", "-q", "--hard", head)

	// The tree verifier must still agree with itself.
	flips := 0
	unstable := treeVerifier
	unstable.Run = func(_ context.Context, _ string, _ []string, stdout, _ io.Writer) int {
		flips++
		_, _ = io.WriteString(stdout, strings.Repeat(" ", flips)+treeStatus)
		return 0
	}
	if _, code, _, stderr := check("A-V0-002", unstable); code != 1 || stderr != "dogfood-check: FAIL verifier-disagreement\n" {
		t.Fatalf("unstable tree exit=%d stderr=%q", code, stderr)
	}

	// A declaration the diff does not carry refuses before any verifier runs.
	for _, tc := range []struct{ declared, refusal string }{
		{"B-V0-001", "output-change-not-in-diff"},
		{"A-V0-009", "output-change-requirement-unknown"},
		{"A-V0-002\tdocs/specs/a-v0.md", "output-change-requirement-unknown"},
	} {
		_, code, stdout, stderr := check(tc.declared, treeVerifier)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "dogfood-check: REFUSE "+tc.refusal+"\n  ") || len(runs) != 0 {
			t.Fatalf("%q exit=%d stdout=%q stderr=%q runs=%v", tc.declared, code, stdout, stderr, runs)
		}
	}

	// An empty range never admits a declaration: before the empty-range PASS
	// an unknown ID refuses requirement-unknown and a known one not-in-diff.
	clean := filepath.Join(t.TempDir(), "clean")
	testGit(t, root, "clone", "-q", root, clean)
	for _, tc := range []struct{ declared, want string }{
		{"", "dogfood-check: PASS no-change-clean-worktree\n"},
		{"DOES-NOT-EXIST", "output-change-requirement-unknown"},
		{"A-V0-002", "output-change-not-in-diff"},
	} {
		var stdout, stderr bytes.Buffer
		c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: clean, OutputChange: tc.declared}, &stdout, &stderr)
		c.base, c.target, c.gitDir = head, head, filepath.Join(clean, ".git")
		code, err := boundary(c.ctx, func() int { c.requireCleanChange(); return 1 }, func() {})
		if err != nil {
			t.Fatal(err)
		}
		if tc.declared == "" {
			if code != 0 || stdout.String() != tc.want || stderr.Len() != 0 {
				t.Fatalf("empty range exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			continue
		}
		if code != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "dogfood-check: REFUSE "+tc.want+"\n  ") {
			t.Fatalf("empty range %q exit=%d stdout=%q stderr=%q", tc.declared, code, stdout.String(), stderr.String())
		}
	}

	// Identical outputs without a declaration keep the former summary.
	if _, code, stdout, _ := check("", verifier("same", baseStatus)); code != 0 || strings.Contains(stdout, "base-independence") {
		t.Fatalf("agreeing exit=%d stdout=%q", code, stdout)
	}
}
