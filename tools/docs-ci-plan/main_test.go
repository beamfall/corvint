// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	r                  repo
	base, head, target string
}

const fixtureREADME = "# Corvint\n\nA context compiler.\n\n## Evidence\n\nStable citation.\n\n```console\ncorvint --version\n```\n\n[Guide](docs/guide.md#usage)\n"

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func put(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func commit(t *testing.T, root string) string {
	t.Helper()
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "fixture")
	return gitTest(t, root, "rev-parse", "HEAD")
}
func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q", "-b", "main")
	put(t, root, readme, fixtureREADME)
	put(t, root, "docs/guide.md", "# Guide\n\n## Usage\n")
	put(t, root, citationFile, fmt.Sprintf("See README.md:7-7@%s.\n", digest([]byte("Stable citation.\n"))[:8]))
	put(t, root, "tools/docs-ci-plan/main.go", "package main\n")
	put(t, root, "docs/specs/documentation-ci-v0.md", "# Policy\n")
	rev := commit(t, root)
	r := repo{root}
	tree, tool, e := r.identities(rev)
	if e != nil {
		t.Fatal(e)
	}
	q := qualification{Profile: "corvint-docs-ci-qualification/0", Approved: true, Tree: tree, Classifier: tool, Checks: checks, Inventory: "docs/specs/documentation-ci-v0.md#consumer-inventory", Review: "independent-fixture-review"}
	b, _ := json.Marshal(q)
	put(t, root, qualificationFile, string(b))
	return fixture{r: r, base: commit(t, root)}
}
func (f *fixture) change(t *testing.T, edit func(string)) {
	t.Helper()
	gitTest(t, f.r.root, "switch", "-qc", "pr")
	edit(f.r.root)
	f.head = commit(t, f.r.root)
	gitTest(t, f.r.root, "switch", "-q", "main")
	gitTest(t, f.r.root, "merge", "--no-ff", "-qm", "test merge", f.head)
	f.target = gitTest(t, f.r.root, "rev-parse", "HEAD")
}
func (f fixture) plan() receipt { return f.r.plan(f.base, f.head, f.target) }
func prose(t *testing.T, root string) {
	put(t, root, readme, strings.Replace(fixtureREADME, "A context compiler.", "A Beamfall project. Context your agents can cite.", 1))
}

// DCI-V0-001, DCI-V0-002, DCI-V0-003, DCI-V0-004: the observed README + moved
// citation shape selects docs, while the corresponding stale citation fails.
func TestREADMEAndCitationRegression(t *testing.T) {
	f := newFixture(t)
	f.change(t, func(root string) {
		put(t, root, readme, "A Beamfall project.\n\n"+fixtureREADME)
		put(t, root, citationFile, fmt.Sprintf("See README.md:9-9@%s.\n", digest([]byte("Stable citation.\n"))[:8]))
	})
	p := f.plan()
	if p.Mode != "DOCS" {
		t.Fatalf("%+v", p)
	}
	if e := f.r.verify(&p); e != nil {
		t.Fatal(e)
	}
	if !p.Verified || p.Qualification == "" || p.Tree == "" || p.Target != f.target || p.Unknown == "" {
		t.Fatalf("incomplete receipt: %+v", p)
	}
}

// DCI-V0-004: failed document verification is an error, never a successful skip.
func TestBrokenDocumentation(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"citation", "Inserted line.\n" + fixtureREADME, "broken-readme-citation"},
		{"link", strings.Replace(fixtureREADME, "docs/guide.md#usage", "docs/missing.md", 1), "missing-link-target"},
		{"anchor", strings.Replace(fixtureREADME, "#usage", "#missing", 1), "missing-heading-anchor"},
		{"unsafe-scheme", strings.Replace(fixtureREADME, "docs/guide.md#usage", "javascript:evil", 1), "unsupported-link"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.change(t, func(root string) { put(t, root, readme, tc.body) })
			p := f.plan()
			if p.Mode != "DOCS" {
				t.Fatalf("%+v", p)
			}
			e := f.r.verify(&p)
			if e == nil || e.Error() != tc.want || p.Verified {
				t.Fatalf("error=%v receipt=%+v", e, p)
			}
		})
	}
}

// DCI-V0-002, DCI-V0-003: only the closed presentation surface is admissible.
func TestFullFallback(t *testing.T) {
	cases := []struct {
		name string
		edit func(*testing.T, string)
	}{
		{"code", func(t *testing.T, r string) { prose(t, r); put(t, r, "lib.go", "package example\n") }},
		{"workflow", func(t *testing.T, r string) { prose(t, r); put(t, r, ".github/workflows/ci.yml", "changed\n") }},
		{"spec", func(t *testing.T, r string) {
			prose(t, r)
			put(t, r, "docs/specs/documentation-ci-v0.md", "new intent\n")
		}},
		{"qualification-self-admission", func(t *testing.T, r string) { prose(t, r); put(t, r, qualificationFile, "{}\n") }},
		{"command", func(t *testing.T, r string) {
			put(t, r, readme, strings.Replace(fixtureREADME, "corvint --version", "corvint record", 1))
		}},
		{"new-code-block", func(t *testing.T, r string) { put(t, r, readme, fixtureREADME+"\n~~~sh\nrun\n~~~\n") }},
		{"indented-code", func(t *testing.T, r string) { put(t, r, readme, fixtureREADME+"\n    run\n") }},
		{"script", func(t *testing.T, r string) { put(t, r, readme, fixtureREADME+"<script>run()</script>\n") }},
		{"html-event", func(t *testing.T, r string) { put(t, r, readme, fixtureREADME+"<img src=\"x\" onerror=\"run()\">\n") }},
		{"unclosed-fence", func(t *testing.T, r string) { put(t, r, readme, fixtureREADME+"```sh\nrun\n") }},
		{"citation-prose", func(t *testing.T, r string) { prose(t, r); put(t, r, citationFile, "Changed governing meaning\n") }},
		{"rename", func(t *testing.T, r string) {
			if e := os.Rename(filepath.Join(r, readme), filepath.Join(r, "RENAMED.md")); e != nil {
				t.Fatal(e)
			}
		}},
		{"delete", func(t *testing.T, r string) {
			if e := os.Remove(filepath.Join(r, readme)); e != nil {
				t.Fatal(e)
			}
		}},
		{"symlink", func(t *testing.T, r string) {
			if e := os.Remove(filepath.Join(r, readme)); e != nil {
				t.Fatal(e)
			}
			if e := os.Symlink("docs/guide.md", filepath.Join(r, readme)); e != nil {
				t.Fatal(e)
			}
		}},
		{"non-UTF8", func(t *testing.T, r string) { put(t, r, readme, fixtureREADME+"\xff") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.change(t, func(r string) { tc.edit(t, r) })
			p := f.plan()
			if p.Mode != "FULL" || p.Verified {
				t.Fatalf("%+v", p)
			}
		})
	}
}

// DCI-V0-001: all exact event and local-state bindings must hold.
func TestMergeBindings(t *testing.T) {
	for _, name := range []string{"swapped-parents", "missing-object", "head-not-merge", "dirty", "grafts", "replace"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.change(t, func(r string) { prose(t, r) })
			switch name {
			case "swapped-parents":
				f.base, f.head = f.head, f.base
			case "missing-object":
				f.target = strings.Repeat("f", 40)
			case "head-not-merge":
				f.target = f.head
			case "dirty":
				put(t, f.r.root, "untracked", "x")
			case "grafts":
				put(t, f.r.root, ".git/info/grafts", f.target+"\n")
			case "replace":
				gitTest(t, f.r.root, "replace", f.base, f.head)
			}
			if p := f.plan(); p.Mode != "FULL" {
				t.Fatalf("%+v", p)
			}
		})
	}
}

// DCI-V0-005: the qualification binds the full source/control-plane closure,
// including names and modes. PR-proposed approval cannot admit that PR.
func TestQualificationInvalidation(t *testing.T) {
	for _, name := range []string{"missing", "unapproved", "bad-checks", "malformed", "stale-source", "mode-drift"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			b, e := os.ReadFile(filepath.Join(f.r.root, qualificationFile))
			if e != nil {
				t.Fatal(e)
			}
			var q qualification
			if e = json.Unmarshal(b, &q); e != nil {
				t.Fatal(e)
			}
			switch name {
			case "missing":
				if e = os.Remove(filepath.Join(f.r.root, qualificationFile)); e != nil {
					t.Fatal(e)
				}
			case "unapproved":
				q.Approved = false
				b, _ = json.Marshal(q)
				put(t, f.r.root, qualificationFile, string(b))
			case "bad-checks":
				q.Checks = []string{"none"}
				b, _ = json.Marshal(q)
				put(t, f.r.root, qualificationFile, string(b))
			case "malformed":
				put(t, f.r.root, qualificationFile, "{} {}")
			case "stale-source":
				put(t, f.r.root, "new.go", "package changed\n")
			case "mode-drift":
				gitTest(t, f.r.root, "update-index", "--chmod=+x", "docs/guide.md")
				if e = os.Chmod(filepath.Join(f.r.root, "docs/guide.md"), 0700); e != nil {
					t.Fatal(e)
				}
			}
			f.base = commit(t, f.r.root)
			f.change(t, func(r string) { prose(t, r) })
			if p := f.plan(); p.Mode != "FULL" {
				t.Fatalf("%+v", p)
			}
		})
	}
}

func TestCodeFenceAndCitationBounds(t *testing.T) {
	for _, body := range []string{"```go\nx\n```\n", "~~~go\nx\n~~~\n", "  ````go\n```\nx\n  ````\n"} {
		b, e := executableBlocks([]byte(body))
		if e != nil || string(b) != body {
			t.Fatalf("%q %q %v", body, b, e)
		}
	}
	if citationOnly([]byte("README.md:0-1@00000000"), []byte("README.md:1-2@00000000"), []byte("x\n"), []byte("x\n")) {
		t.Fatal("invalid citation admitted")
	}
}

// DCI-V0-003/004: adversarial Markdown and quoted HTML must not bypass admission.
func TestAdversarialPresentation(t *testing.T) {
	for name, extra := range map[string]string{
		"blockquote-fence":        "> ```sh\n> run\n> ```\n",
		"container-indented-code": ">     run-something\n",
		"mixed-indent":            " \trun-something\n",
		"list-indented-code":      "-     run-something\n",
		"unsupported-link-title":  "[Guide](docs/missing.md 'title')\n",
		"list-fence":              "- ```sh\n  run\n  ```\n",
		"quoted-close":            `<img alt=">" onerror="evil()" src="https://example.com/x">`,
		"unquoted-url":            `<a href=javascript:evil()>x</a>`,
		"reference-link":          "[Guide][g]\n\n[g]: docs/missing.md\n",
		"duplicate-attribute":     `<a href="https://example.com" href="javascript:evil()">x</a>`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.change(t, func(r string) { put(t, r, readme, fixtureREADME+extra) })
			if p := f.plan(); p.Mode != "FULL" {
				t.Fatalf("%+v", p)
			}
		})
	}
	f := newFixture(t)
	f.change(t, func(r string) { put(t, r, readme, strings.Replace(fixtureREADME, "#usage", "#example", 1)) })
	// The unchanged guide has no such heading; separate direct check ensures a code
	// example does not create an anchor for a subsequently admitted README link.
	if headingIDs([]byte("# Guide\n```md\n# Example\n```\n"))["example"] {
		t.Fatal("code example became a heading")
	}
	p := f.plan()
	if p.Mode != "DOCS" {
		t.Fatalf("%+v", p)
	}
	if e := f.r.verify(&p); e == nil {
		t.Fatal("missing heading accepted")
	}
}
func TestCitationSentinelCollision(t *testing.T) {
	body := []byte("stable\n")
	c := fmt.Sprintf("README.md:1-1@%s", digest(body)[:8])
	if citationOnly([]byte(c+"CITATION"), []byte("CITATION"+c), body, body) {
		t.Fatal("citation moved across ordinary prose")
	}
}

// DCI-V0-006, DCI-V0-008: exercise the real process interface and exit statuses,
// with the binary outside the tested tree, not only in-process planner helpers.
func TestNativeCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "docs-ci-plan")
	build := exec.Command("go", "build", "-o", binary, ".")
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	for _, tc := range []struct {
		name     string
		edit     func(string, *testing.T)
		mode     string
		wantExit int
	}{
		{"docs", func(r string, t *testing.T) { prose(t, r) }, "DOCS", 0},
		{"broken-citation", func(r string, t *testing.T) { put(t, r, readme, "New line.\n"+fixtureREADME) }, "DOCS", 1},
		{"full", func(r string, t *testing.T) { prose(t, r); put(t, r, "new.go", "package x\n") }, "FULL", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.change(t, func(r string) { tc.edit(r, t) })
			cmd := exec.Command(binary, "--root", f.r.root, "--base", f.base, "--head", f.head, "--target", f.target)
			b, e := cmd.Output()
			exit := 0
			if e != nil {
				ee, ok := e.(*exec.ExitError)
				if !ok {
					t.Fatal(e)
				}
				exit = ee.ExitCode()
			}
			var p receipt
			if e = json.Unmarshal(b, &p); e != nil {
				t.Fatal(e)
			}
			if exit != tc.wantExit || p.Mode != tc.mode || p.Verified != (tc.mode == "DOCS" && exit == 0) || p.Target != f.target {
				t.Fatalf("exit=%d receipt=%+v", exit, p)
			}
		})
	}
	f := newFixture(t)
	cmd := exec.Command(binary, "--root", f.r.root, "--base", f.base, "--qualification-proposal")
	b, e := cmd.Output()
	if e != nil {
		t.Fatal(e)
	}
	var q qualification
	if e = json.Unmarshal(b, &q); e != nil {
		t.Fatal(e)
	}
	if q.Approved || q.Review != "" || q.Tree == "" {
		t.Fatalf("proposal self-approved: %+v", q)
	}
}

func TestWhitespaceLink(t *testing.T) {
	f := newFixture(t)
	f.change(t, func(r string) { put(t, r, readme, fixtureREADME+"[broken]( docs/missing.md )\n") })
	p := f.plan()
	if p.Mode != "DOCS" {
		t.Fatalf("%+v", p)
	}
	if e := f.r.verify(&p); e == nil || e.Error() != "missing-link-target" {
		t.Fatalf("%v", e)
	}
}
