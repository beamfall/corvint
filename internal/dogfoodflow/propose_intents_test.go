package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0711IntentsDraftPassesInputsCheck (V1-3956):
// --intents-draft writes the change's Markdown files that derive as intents at
// HEAD, sorted one per line, leaving out a citation-repin-only spec edit and a
// Markdown file with no Requirements section, and the draft is accepted by the
// inputs pre-flight unchanged; a change with no such file drafts the no-intent
// manifest.
func TestProposeCitationsDCWV0711IntentsDraftPassesInputsCheck(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	spec := func(prefix, note string) string {
		return "# " + prefix + "\n\n## Requirements\n\n- `" + prefix + "-V0-001`: " + note + "\n"
	}
	code := "package pre\n\n// First is one.\nfunc First() int {\n" + numbered("\t// first", 12) + "\treturn 1\n}\n"
	writeTree(t, root, map[string]string{
		".gitignore":         ".corvint/\n",
		"pre.go":             code,
		"README.md":          "# Readme\n\nplain text\n",
		"docs/specs/c-v0.md": spec("CPIN", "see pre.go:3 for the rule."), //corvint:fixture-path
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"pre.go":                 strings.Replace(code, "\t// first line 6\n", "\t// first line 6 changed\n", 1),
		"README.md":              "# Readme\n\nplain text, edited\n",
		"docs/specs/b-v0.md":     spec("BSPEC", "keep b."),                   //corvint:fixture-path
		"docs/specs/a-v0.md":     spec("ASPEC", "keep a."),                   //corvint:fixture-path
		"docs/specs/c-v0.md":     spec("CPIN", "see pre.go:4 for the rule."), //corvint:fixture-path
		"docs/build-log/x-v0.md": "# Build log\n\nno requirements here\n",    //corvint:fixture-path
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")

	draft := filepath.Join(t.TempDir(), "intents.txt")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, IntentsDraft: draft}, &stdout, &stderr); err != nil || code != 0 {
		t.Fatalf("propose: code=%d err=%v\n%s", code, err, &stderr)
	}
	got, err := os.ReadFile(draft)
	if err != nil {
		t.Fatal(err)
	}
	if want := "docs/specs/a-v0.md\ndocs/specs/b-v0.md\n"; string(got) != want { //corvint:fixture-path
		t.Fatalf("intents draft = %q, want %q\nstderr:\n%s", got, want, &stderr)
	}
	if !strings.Contains(stderr.String(), "NOTE intents-draft wrote 2 intent scopes to "+draft) {
		t.Errorf("stderr does not note the draft:\n%s", &stderr)
	}

	// The draft is used unchanged as the pre-flight's intents manifest,
	// with every hunk given a reviewed unknown row.
	plan := new(strings.Builder)
	for _, line := range strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n") {
		hunk, _, _ := strings.Cut(line, "\t")
		plan.WriteString(hunk + "\tunknown\tinsufficient-evidence\treviewed\n")
	}
	pass, out, errOut, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: writeInput(t, plan.String()), IntentsFile: draft})
	if pass != 0 || !strings.Contains(out, "inputs-check PASS") {
		t.Fatalf("inputs-check with the draft: code=%d\nstdout=%s\nstderr=%s", pass, out, errOut)
	}

	// A change with no intent drafts the no-intent manifest.
	head := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"pre.go": code})
	testGit(t, root, "commit", "-q", "-am", "code only")
	stdout.Reset()
	stderr.Reset()
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: head, IntentsDraft: draft}, &stdout, &stderr); err != nil || code != 0 {
		t.Fatalf("propose code only: code=%d err=%v\n%s", code, err, &stderr)
	}
	if got, _ := os.ReadFile(draft); string(got) != string(noIntentManifest) {
		t.Fatalf("code-only intents draft = %q, want the no-intent manifest", got)
	}
}
