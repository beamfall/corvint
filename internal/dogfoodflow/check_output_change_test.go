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
	if _, code, stdout, stderr := check("", treeVerifier); code != 1 || stdout != "" || !strings.HasPrefix(stderr, "dogfood-check: FAIL verifier-disagreement\n  fix: the base and tree verifiers disagree") ||
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
	if _, code, _, stderr := check("A-V0-002", unstable); code != 1 || !strings.HasPrefix(stderr, "dogfood-check: FAIL verifier-disagreement\n  fix: the base and tree verifiers disagree") {
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

// TestCheckDCWV0160ComputesOCMStatusOnce covers V1-2274 (DCW-V0-160, which
// supersedes the V1-1276 OCM rule of DCW-V0-047): the OCM aggregate is a
// function of the target tree and its scope set, so one check runs
// `dogfood-ocm status` once, by the tree verifier, whether or not an output
// change is declared, where it ran once per verifier role (three runs with an
// override). A base verifier that cannot read the map no longer matters to it;
// the tree verifier's refusal and a drifted published aggregate still fail.
func TestCheckDCWV0160ComputesOCMStatusOnce(t *testing.T) {
	root := t.TempDir()
	aggregate := filepath.Join(root, "ocm-aggregate.json")
	treeOCM := `{"aggregate":{"state":"ready-for-review"},"ok":true}` + "\n"
	reportPath := filepath.Join(root, "dogfood-report.json")
	runs := map[string]int{}
	verifier := func(role string, run func(stdout, stderr io.Writer) int) Runner {
		path := filepath.Join(t.TempDir(), role)
		if err := os.WriteFile(path, []byte(role), 0o755); err != nil {
			t.Fatal(err)
		}
		return Runner{Path: path, Run: func(_ context.Context, _ string, args []string, stdout, stderr io.Writer) int {
			if len(args) < 2 || args[0] != "dogfood-ocm" || args[1] != "status" {
				t.Fatalf("%s verifier ran %q", role, args)
			}
			runs[role]++
			return run(stdout, stderr)
		}}
	}
	// With baseRefuses the base verifier refuses the read (unsupported-spec),
	// so any comparison with it would disagree.
	baseRefuses := false
	baseVerifier := verifier("base", func(stdout, stderr io.Writer) int {
		if !baseRefuses {
			_, _ = io.WriteString(stdout, treeOCM)
			return 0
		}
		_, _ = io.WriteString(stderr, `{"code": "unsupported-spec", "error": "OCM read verification accepts cem/0.1 or cem/0.2", "ok": false}`+"\n")
		return 2
	})
	treeVerifier := verifier("tree", func(stdout, _ io.Writer) int {
		_, _ = io.WriteString(stdout, treeOCM)
		return 0
	})
	override := verifier("override", func(stdout, _ io.Writer) int {
		_, _ = io.WriteString(stdout, treeOCM)
		return 0
	})
	check := func(declared string, tree Runner, published string) (int, []byte, string) {
		t.Helper()
		report := "{\n" + `  ,"dogfoodCheck": null` + "\n}\n"
		if err := os.WriteFile(reportPath, []byte(report), 0o666); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(aggregate, []byte(published), 0o666); err != nil {
			t.Fatal(err)
		}
		clear(runs)
		var stdout, stderr bytes.Buffer
		c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root, BaseVerifier: baseVerifier, TreeVerifier: tree, Override: &override}, &stdout, &stderr)
		c.base, c.target, c.outputChange = strings.Repeat("a", 40), strings.Repeat("b", 40), declared
		// The report is staged in the Git directory; an unset one staged it in the package
		// directory and left an untracked file in the source tree.
		c.report, c.evidence, c.gitDir = reportPath, t.TempDir(), root
		var line []byte
		code, err := boundary(c.ctx, func() int {
			c.resolveVerifiers()
			line = c.verifyOCM(aggregate)
			return 0
		}, func() {})
		if err != nil {
			t.Fatal(err)
		}
		return code, line, stderr.String()
	}

	for _, refuses := range []bool{false, true} {
		baseRefuses = refuses
		for _, declared := range []string{"", "DCW-V0-047"} {
			code, line, stderr := check(declared, treeVerifier, treeOCM)
			if code != 0 || stderr != "" || string(line) != treeOCM {
				t.Fatalf("base refuses=%v declared=%q exit=%d line=%q stderr=%q", refuses, declared, code, line, stderr)
			}
			if runs["tree"] != 1 || runs["base"] != 0 || runs["override"] != 0 {
				t.Fatalf("base refuses=%v declared=%q dogfood-ocm status runs = %v; want the tree verifier once", refuses, declared, runs)
			}
		}
	}
	if code, _, stderr := check("", baseVerifier, treeOCM); code != 1 || stderr != "dogfood-check: FAIL unsupported-spec\n" {
		t.Fatalf("refusing tree verifier exit=%d stderr=%q; want its refusal code", code, stderr)
	}
	if code, _, stderr := check("", treeVerifier, " "+treeOCM); code != 1 || !strings.HasPrefix(stderr, "dogfood-check: FAIL intent-scope-drift\n") {
		t.Fatalf("drifted aggregate exit=%d stderr=%q; want intent-scope-drift", code, stderr)
	}
}

// TestCheckDCWV0047RefusedBaseNotProduced covers V1-1307 (decision 0535,
// superseding DCW-V0-072): under an admitted declaration a base verifier that
// refuses the `cem status` read (exit 2, for example `unsupported-spec` for a
// cem/1.0 map its release cannot read) records NOT_PRODUCED and its refusal
// code instead of a digest of its absent stdout. A base verifier that reads
// the map, even one that exits 1, keeps its stdout digest.
func TestCheckDCWV0047RefusedBaseNotProduced(t *testing.T) {
	treeStatus := `{"state":"ready-for-ci"}` + "\n"
	runner := func(role string, run func(stdout, stderr io.Writer) int) Runner {
		path := filepath.Join(t.TempDir(), role)
		if err := os.WriteFile(path, []byte(role), 0o755); err != nil {
			t.Fatal(err)
		}
		return Runner{Path: path, Run: func(_ context.Context, _ string, _ []string, stdout, stderr io.Writer) int {
			return run(stdout, stderr)
		}}
	}
	tree := runner("tree", func(stdout, _ io.Writer) int {
		_, _ = io.WriteString(stdout, treeStatus)
		return 0
	})
	treeSHA := sha256Hex([]byte(treeStatus))
	for _, tc := range []struct {
		name               string
		base               func(stdout, stderr io.Writer) int
		fields, recordWant string
	}{
		{"refused", func(_, stderr io.Writer) int {
			_, _ = io.WriteString(stderr, `{"code": "unsupported-spec", "ok": false}`+"\n")
			return 2
		}, "base=NOT_PRODUCED base-refusal=unsupported-spec", `"baseStatus": "NOT_PRODUCED", "baseRefusal": "unsupported-spec"`},
		{"refused without a code", func(_, _ io.Writer) int { return 3 },
			"base=NOT_PRODUCED base-refusal=exit-3", `"baseStatus": "NOT_PRODUCED", "baseRefusal": "exit-3"`},
		{"rejected", func(stdout, _ io.Writer) int {
			_, _ = io.WriteString(stdout, `{"state":"rejected"}`+"\n")
			return 1
		}, "base=sha256:" + sha256Hex([]byte(`{"state":"rejected"}`+"\n")),
			`"baseStatusStdoutSha256": "sha256:` + sha256Hex([]byte(`{"state":"rejected"}`+"\n")) + `"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			reportPath := filepath.Join(root, "report.json")
			if err := os.WriteFile(reportPath, []byte("{\n  ,\"dogfoodCheck\": null\n}\n"), 0o666); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root, BaseVerifier: runner("base", tc.base), TreeVerifier: tree, OutputChange: "DCW-V0-047"}, &stdout, &stderr)
			c.outputChange, c.report, c.gitDir = "DCW-V0-047", reportPath, filepath.Join(root, ".git")
			var agreed bool
			code, err := boundary(c.ctx, func() int {
				c.resolveVerifiers()
				_, agreed = c.verifyStatus("cem", "status")
				if err := c.record(agreed, 0); err != nil {
					t.Fatal(err)
				}
				return 0
			}, func() {})
			if err != nil || code != 0 || !agreed {
				t.Fatalf("exit=%d err=%v agreed=%v stderr=%q", code, err, agreed, stderr.String())
			}
			want := "base-independence=NOT_PROVED " + tc.fields + " tree=sha256:" + treeSHA + " declared=DCW-V0-047"
			if got := c.baseIndependence(); got != want {
				t.Fatalf("baseIndependence = %q; want %q", got, want)
			}
			data, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			recorded := `"baseIndependence": {"state": "NOT_PROVED", "declared": "DCW-V0-047", ` + tc.recordWant + `, "treeStatusStdoutSha256": "sha256:` + treeSHA + `"}}`
			if !strings.Contains(string(data), recorded) {
				t.Fatalf("report=%q; want %q", data, recorded)
			}
		})
	}
}
