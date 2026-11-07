package forwardrow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// guardRepo is a Git repository whose base commit holds a reduced .github tree: the trusted
// narrowing inputs (forward-rows, cishards, testconfine, the verdict workflows, ci.yml with the
// narrowing jobs) beside paths that are not inputs (another workflow, ci-cadence, macos, a doc).
type guardRepo struct {
	t    *testing.T
	dir  string
	base string
}

const guardCI = `name: CI
on:
  pull_request:
permissions:
  contents: read
env:
  CORVINT_PR_TOOL_SOURCE: "8daec98ce4349c1583186692726d2c3a782ac647"
jobs:
  cadence-plan:
    runs-on: ubuntu-24.04
    steps:
      - run: echo cadence
  docs-plan:
    runs-on: ubuntu-24.04
    steps:
      - run: echo docs
  narrowing-verdict:
    runs-on: ubuntu-24.04
    steps:
      - run: sh trusted/.github/forward-rows/accept.sh
  shard-plan:
    runs-on: ubuntu-24.04
    steps:
      - run: sh trusted-tools/.github/forward-rows/shard-plan.sh
  go-product-shard:
    needs: [docs-plan, narrowing-verdict, shard-plan]
    strategy:
      matrix:
        shard: [0, 1]
    steps:
      - name: Narrowing gate (AFP-V0-098, off unless CORVINT_PR_NARROWING is on)
        run: sh "$GITHUB_WORKSPACE/trusted-tools/.github/forward-rows/narrow.sh"
      - name: Go tests (full until matching shadow qualification)
        run: |
          go list ./... > universe
          packages=$(ci-shards --shard "$SHARD" --shards "$SHARDS" < universe)
          go test $packages
  go-static:
    runs-on: ubuntu-24.04
    steps:
      - run: go vet ./...
  ci-tested-tree-record:
    needs: [go-product-shard]
    steps:
      - run: printf 'corvint-ci-tested-tree/1\n'
  ci-narrowed-tree-record:
    needs: [go-product-shard, go-product]
    steps:
      - run: printf 'corvint-ci-tested-tree/2\n'
  go-product:
    needs: [go-product-shard, go-static]
    steps:
      - run: test "$SHARDS_RESULT" = success
  artifact-integrity:
    runs-on: ubuntu-24.04
    steps:
      - run: echo archive
`

func newGuardRepo(t *testing.T) *guardRepo {
	t.Helper()
	r := &guardRepo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	for path, body := range map[string]string{
		".github/forward-rows/narrow.sh":         "#!/bin/sh\necho narrow\n",
		".github/forward-rows/narrow.jq":         "{}\n",
		".github/forward-rows/github-guard.sh":   "#!/bin/sh\necho 0\n",
		".github/cishards/partition.go":          "package cishards\n",
		".github/cishards/package-costs.json":    "{}\n",
		".github/testconfine/cmd/main.go":        "package main\n",
		".github/workflows/ci.yml":               guardCI,
		".github/workflows/ci-forward-bar.yml":   "name: bar\n",
		".github/workflows/ci-control-plane.yml": "name: control\n",
		".github/workflows/go-macos.yml":         "name: macos\n",
		".github/ci-cadence/classify.sh":         "#!/bin/sh\necho DEFER\n",
		".github/macos/shard.py":                 "SHARDS = 4\n",
		".github/qualifications/README.md":       "# qualifications\n",
		"internal/x/x.go":                        "package x\n",
	} {
		r.write(path, body)
	}
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "base")
	r.base = r.git("rev-parse", "HEAD")
	return r
}

func (r *guardRepo) write(path, body string) {
	r.t.Helper()
	full := filepath.Join(r.dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *guardRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t",
		"GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// change commits edit on a branch off the base and returns the merge of base and that branch,
// the tested merge a pull-request run narrows.
func (r *guardRepo) change(name string, edit func()) string {
	r.t.Helper()
	r.git("checkout", "-q", "-b", name, r.base)
	edit()
	r.git("add", "-A")
	r.git("commit", "-q", "--allow-empty", "-m", name)
	head := r.git("rev-parse", "HEAD")
	r.git("checkout", "-q", "--detach", r.base)
	r.git("merge", "-q", "--no-ff", "-m", "merge "+name, head)
	return r.git("rev-parse", "HEAD")
}

func (r *guardRepo) remove(path string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.dir, path)); err != nil {
		r.t.Fatal(err)
	}
}

// guard runs the real github-guard.sh over base..target in the repository.
func (r *guardRepo) guard(target string) (string, []string) {
	r.t.Helper()
	cmd := exec.Command("sh", script(r.t, "github-guard.sh"), r.base, target)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("github-guard.sh: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	return lines[0], lines[1:]
}

// TestAFPV02720GuardNamesOnlyTheNarrowingInputs: a change to a trusted narrowing input under
// .github/ is reported with its path, and any other .github/ change is not.
func TestAFPV02720GuardNamesOnlyTheNarrowingInputs_AFPV02720(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "github-guard.sh runs under sh against a real Git repository")
	r := newGuardRepo(t)
	for _, tc := range []struct {
		name string
		edit func()
		want []string
	}{
		{"narrow.sh", func() { r.write(".github/forward-rows/narrow.sh", "#!/bin/sh\necho mode=NARROWED\n") }, []string{".github/forward-rows/narrow.sh"}},
		{"the guard itself", func() { r.write(".github/forward-rows/github-guard.sh", "#!/bin/sh\necho 0 # always\n") }, []string{".github/forward-rows/github-guard.sh"}},
		{"a new forward-rows script", func() { r.write(".github/forward-rows/new.jq", ".\n") }, []string{".github/forward-rows/new.jq"}},
		{"the confine wrapper", func() { r.write(".github/testconfine/cmd/main.go", "package main\n\nfunc main() {}\n") }, []string{".github/testconfine/cmd/main.go"}},
		{"the placement helper", func() { r.write(".github/cishards/partition.go", "package cishards\n\nvar X = 1\n") }, []string{".github/cishards/partition.go"}},
		{"the cost table", func() { r.write(".github/cishards/package-costs.json", "{\"a\":1}\n") }, []string{".github/cishards/package-costs.json"}},
		{"a renamed cost table", func() {
			r.remove(".github/cishards/package-costs.json")
			r.write(".github/cishards/costs.json", "{}\n")
		}, []string{".github/cishards/costs.json", ".github/cishards/package-costs.json"}},
		{"the forward bar workflow", func() { r.write(".github/workflows/ci-forward-bar.yml", "name: bar2\n") }, []string{".github/workflows/ci-forward-bar.yml"}},
		{"the control plane workflow", func() { r.write(".github/workflows/ci-control-plane.yml", "name: c2\n") }, []string{".github/workflows/ci-control-plane.yml"}},
		{"a non-ASCII path under forward-rows (AFP-V0-933)", func() { r.write(".github/forward-rows/é.jq", ".\n") }, []string{".github/forward-rows/é.jq"}},
	} {
		t.Run("FULL: "+tc.name, func(t *testing.T) {
			target := r.change(strings.ReplaceAll(tc.name, " ", "-"), tc.edit)
			mode, inputs := r.guard(target)
			if mode != "1" || fmt.Sprint(inputs) != fmt.Sprint(tc.want) {
				t.Fatalf("guard = %s %q, want 1 %q", mode, inputs, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name string
		edit func()
	}{
		{"a new unrelated workflow", func() { r.write(".github/workflows/greptile-gate.yml", "name: greptile\n") }},
		{"an edited unrelated workflow", func() { r.write(".github/workflows/go-macos.yml", "name: macos2\n") }},
		{"ci-cadence data", func() { r.write(".github/ci-cadence/classify.sh", "#!/bin/sh\necho FULL\n") }},
		{"the macOS shard script", func() { r.write(".github/macos/shard.py", "SHARDS = 8\n") }},
		{"a document", func() { r.write(".github/qualifications/README.md", "# qualifications\n\nmore\n") }},
		{"a deleted unrelated workflow", func() { r.remove(".github/workflows/go-macos.yml") }},
		{"code outside .github", func() { r.write("internal/x/x.go", "package x\n\nvar Y = 2\n") }},
		{"nothing", func() {}},
	} {
		t.Run("narrows: "+tc.name, func(t *testing.T) {
			target := r.change(strings.ReplaceAll(tc.name, " ", "-"), tc.edit)
			if mode, inputs := r.guard(target); mode != "0" || len(inputs) != 0 {
				t.Fatalf("guard = %s %q, want 0", mode, inputs)
			}
		})
	}
}

// TestAFPV02721GuardComparesTheNarrowingJobsOfCIYML: ci.yml forces FULL only when a narrowing
// job, or a line outside the job blocks, changes; another job may change. Anything the section
// compare cannot attribute (a YAML anchor, a deleted file, a job key moved out of shape) is FULL.
func TestAFPV02721GuardComparesTheNarrowingJobsOfCIYML_AFPV02721(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "github-guard.sh runs under sh against a real Git repository")
	r := newGuardRepo(t)
	edit := func(old, new string) func() {
		return func() {
			if !strings.Contains(guardCI, old) {
				t.Fatalf("fixture lacks %q", old)
			}
			r.write(".github/workflows/ci.yml", strings.Replace(guardCI, old, new, 1))
		}
	}
	for _, tc := range []struct {
		name string
		edit func()
		want string
	}{
		{"the selection line of the tests step", edit(`          packages=$(ci-shards --shard "$SHARD" --shards "$SHARDS" < universe)`, `          packages=""`), ".github/workflows/ci.yml (go-product-shard)"},
		{"the narrowing gate step", edit(`        run: sh "$GITHUB_WORKSPACE/trusted-tools/.github/forward-rows/narrow.sh"`, `        run: echo mode=NARROWED >> "$GITHUB_OUTPUT"`), ".github/workflows/ci.yml (go-product-shard)"},
		{"the shard matrix", edit("        shard: [0, 1]", "        shard: [0]"), ".github/workflows/ci.yml (go-product-shard)"},
		{"the verdict job", edit("      - run: sh trusted/.github/forward-rows/accept.sh", "      - run: echo verdict=MET"), ".github/workflows/ci.yml (narrowing-verdict)"},
		{"the shard-plan job", edit("      - run: sh trusted-tools/.github/forward-rows/shard-plan.sh", "      - run: echo count=1"), ".github/workflows/ci.yml (shard-plan)"},
		{"docs-plan", edit("      - run: echo docs", "      - run: echo mode=REUSE"), ".github/workflows/ci.yml (docs-plan)"},
		{"the full record job", edit("      - run: printf 'corvint-ci-tested-tree/1\\n'", "      - run: printf 'corvint-ci-tested-tree/1 tree=x\\n'"), ".github/workflows/ci.yml (ci-tested-tree-record)"},
		{"the narrowed record job", edit("      - run: printf 'corvint-ci-tested-tree/2\\n'", "      - run: printf 'corvint-ci-tested-tree/2 tree=x\\n'"), ".github/workflows/ci.yml (ci-narrowed-tree-record)"},
		{"go-product", edit(`      - run: test "$SHARDS_RESULT" = success`, "      - run: true"), ".github/workflows/ci.yml (go-product)"},
		{"the workflow env", edit(`  CORVINT_PR_TOOL_SOURCE: "8daec98ce4349c1583186692726d2c3a782ac647"`, `  CORVINT_PR_TOOL_SOURCE: ""`), ".github/workflows/ci.yml (-)"},
		{"the trigger", edit("on:\n  pull_request:\n", "on:\n  pull_request:\n  push:\n"), ".github/workflows/ci.yml (-)"},
		{"a job key moved out of shape", edit("  go-product-shard:\n", "  go-product-shard: # moved\n"), ".github/workflows/ci.yml (go-product-shard)"},
		{"a deleted ci.yml", func() { r.remove(".github/workflows/ci.yml") }, ".github/workflows/ci.yml (-)"},
	} {
		t.Run("FULL: "+tc.name, func(t *testing.T) {
			target := r.change(strings.ReplaceAll(tc.name, " ", "-"), tc.edit)
			mode, inputs := r.guard(target)
			// A key moved out of shape also lands its block in the neighbouring guarded section.
			if mode != "1" || len(inputs) == 0 || !strings.Contains(fmt.Sprint(inputs), tc.want) {
				t.Fatalf("guard = %s %q, want 1 naming %q", mode, inputs, tc.want)
			}
		})
	}
	t.Run("FULL: a YAML anchor", func(t *testing.T) {
		target := r.change("anchor", edit("  cadence-plan:\n    runs-on: ubuntu-24.04\n", "  cadence-plan:\n    runs-on: &runner ubuntu-24.04\n"))
		mode, inputs := r.guard(target)
		if mode != "1" || fmt.Sprint(inputs) != fmt.Sprint([]string{".github/workflows/ci.yml (a YAML anchor)"}) {
			t.Fatalf("guard = %s %q", mode, inputs)
		}
	})
	for _, tc := range []struct {
		name string
		edit func()
	}{
		{"another job's step", edit("      - run: echo cadence", "      - run: echo cadence2")},
		{"a new job", edit("  artifact-integrity:\n", "  new-job:\n    runs-on: ubuntu-24.04\n    steps:\n      - run: echo new\n  artifact-integrity:\n")},
		{"go-static", edit("      - run: go vet ./...", "      - run: go vet -tags x ./...")},
		{"a deleted job", edit("  artifact-integrity:\n    runs-on: ubuntu-24.04\n    steps:\n      - run: echo archive\n", "")},
	} {
		t.Run("narrows: "+tc.name, func(t *testing.T) {
			target := r.change(strings.ReplaceAll(tc.name, " ", "-"), tc.edit)
			if mode, inputs := r.guard(target); mode != "0" || len(inputs) != 0 {
				t.Fatalf("guard = %s %q, want 0", mode, inputs)
			}
		})
	}
	t.Run("unreadable: a short commit id or an unknown commit", func(t *testing.T) {
		for _, args := range [][]string{{r.base[:12], r.base}, {r.base, strings.Repeat("f", 40)}} {
			cmd := exec.Command("sh", script(t, "github-guard.sh"), args[0], args[1])
			cmd.Dir = r.dir
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.HasPrefix(string(out), "unreadable\n") {
				t.Fatalf("guard %v = %v\n%s", args, err, out)
			}
		}
	})
}

// fakeNarrowTools are the narrow.sh spies of a range whose driver selects a, b and c of the
// universe a..d and whose shadow selects a and b: gh (a merge-queue rule), go (the universe),
// the driver, the shadow and ci-shards. git is not among them: the caller adds guardGit or runs
// in a real repository.
func fakeNarrowTools(a, b, c, d string) map[string]string {
	return map[string]string{
		"gh":        "#!/bin/sh\necho true\n",
		"go":        "#!/bin/sh\nprintf '%s\\n' " + a + " " + b + " " + c + " " + d + "\n",
		"driver":    "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" != --out ] || o=$2; shift; done\ncat \"$FAKE/selection.json\" > \"$o/selection.json\"\n",
		"shadow":    "#!/bin/sh\ncat \"$FAKE/shadow.json\"\n",
		"ci-shards": "#!/bin/sh\ncase \"$*\" in *--size-budget-ms*) printf '%s\\n' \"$*\" > \"$FAKE/size-argv\"; cat > \"$FAKE/size-universe\"; echo 3 ;; *--select*--slices*) echo '-run ^(TestX)$ " + c + "' ;; *--select*) echo " + b + " ;; *--slices*) echo '-run ^(TestX)$ " + c + "' ;; *) printf '%s\\n' " + a + " " + d + " ;; esac\n",
	}
}

func installTools(t *testing.T, bin string, tools map[string]string) {
	t.Helper()
	for name, body := range tools {
		writeFile(t, filepath.Join(bin, name), []byte(body))
		if err := os.Chmod(filepath.Join(bin, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func narrowEnv(dir, bin, fake, base, head, target, shard, shards string) []string {
	return []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir, "FAKE=" + fake,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GH_TOKEN=x", "REPO=beamfall/corvint", "BASE_REF=main", "PR_BASE=" + base, "PR_HEAD=" + head, "PR_NUMBER=868",
		"TESTED_MERGE=" + target, "SHARD=" + shard, "SHARDS=" + shards, "TOOL_SOURCE=" + base, "VERDICT_RUN=4242", "PR_BIN=" + bin,
		"SHADOW=" + filepath.Join(bin, "shadow"), "CI_SHARDS=" + filepath.Join(bin, "ci-shards"), "CONFINE=" + filepath.Join(bin, "confine"),
		"GITHUB_RUN_ID=" + prRun, "GITHUB_RUN_ATTEMPT=1", "RUNNER_TEMP=" + dir, "GITHUB_OUTPUT=" + filepath.Join(dir, "output"),
		"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary")}
}

func outputValue(t *testing.T, dir, key string) string {
	t.Helper()
	o, _ := os.ReadFile(filepath.Join(dir, "output"))
	m := regexp.MustCompile(`(?m)^`+key+`=(.*)$`).FindAllStringSubmatch(string(o), -1)
	if len(m) == 0 {
		t.Fatalf("no %s output:\n%s", key, o)
	}
	return m[len(m)-1][1]
}

// TestAFPV02720NarrowShFaultInjection runs the real narrow.sh, with the real github-guard.sh
// beside it and real git, over pull requests of a repository: one that edits narrow.sh, one that
// edits the confine wrapper and one that edits the shard job's selection step each stay FULL and
// name the input, while one that adds an unrelated workflow narrows (V1-4930).
func TestAFPV02720NarrowShFaultInjection_AFPV02720(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "narrow.sh and github-guard.sh run under sh against a real Git repository")
	const a, b, c, d = "example.org/m/a", "example.org/m/b", "example.org/m/c", "example.org/m/d"
	r := newGuardRepo(t)
	narrow := func(t *testing.T, target string) (string, doc) {
		t.Helper()
		dir := t.TempDir()
		fake, bin := filepath.Join(dir, "fake"), filepath.Join(dir, "bin")
		base, head := r.git("rev-parse", target+"^1"), r.git("rev-parse", target+"^2")
		writeDoc(t, filepath.Join(fake, "selection.json"), doc{"EventBase": base, "Reason": "", "Base": base, "Head": head, "Target": target, "Packages": []string{a, b, c}})
		writeDoc(t, filepath.Join(fake, "shadow.json"), doc{"scope": "NARROWED", "incomplete": []string{},
			"invocations": []any{doc{"units": []any{doc{"adapter": "go", "unit": "go:" + a}, doc{"adapter": "go", "unit": "go:" + b}}}}})
		installTools(t, bin, fakeNarrowTools(a, b, c, d))
		writeFile(t, filepath.Join(dir, "output"), nil)
		cmd := exec.Command("sh", script(t, "narrow.sh"))
		cmd.Dir = r.dir
		cmd.Env = narrowEnv(dir, bin, fake, base, head, target, "3", "8")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("narrow.sh: %v\n%s", err, out)
		}
		return outputValue(t, dir, "mode"), readDoc(t, filepath.Join(dir, "corvint-narrowing", "narrowing.json"))
	}
	for _, tc := range []struct {
		name string
		edit func()
		want string
	}{
		{"narrow.sh", func() {
			r.write(".github/forward-rows/narrow.sh", "#!/bin/sh\necho mode=NARROWED >> \"$GITHUB_OUTPUT\"\n")
		}, ".github/forward-rows/narrow.sh"},
		{"the confine wrapper", func() {
			r.write(".github/testconfine/cmd/main.go", "package main\n\nfunc main() { /* run nothing */ }\n")
		}, ".github/testconfine/cmd/main.go"},
		{"the selection step", func() {
			r.write(".github/workflows/ci.yml", strings.Replace(guardCI, `          packages=$(ci-shards --shard "$SHARD" --shards "$SHARDS" < universe)`, `          packages=""`, 1))
		}, ".github/workflows/ci.yml (go-product-shard)"},
	} {
		t.Run("FULL: the pull request edits "+tc.name, func(t *testing.T) {
			mode, v := narrow(t, r.change("edit-"+strings.ReplaceAll(tc.name, " ", "-"), tc.edit))
			reasons := fmt.Sprint(v["reasons"])
			if mode != "FULL" || v["mode"] != "FULL" || !strings.Contains(reasons, "the change touches .github/ narrowing inputs (AFP-V0-2720): "+tc.want) ||
				fmt.Sprint(v["githubInputs"]) != fmt.Sprint([]string{tc.want}) || v["placement"] != "PARTITION" {
				t.Fatalf("mode %s, record %v", mode, v)
			}
			if strings.Count(reasons, "the change") != 1 {
				t.Fatalf("the guard is the one reason: %s", reasons)
			}
		})
	}
	for _, tc := range []struct {
		name string
		edit func()
	}{
		{"adds an unrelated workflow", func() { r.write(".github/workflows/greptile-gate.yml", "name: greptile\non: pull_request\n") }},
		{"edits ci-cadence data and a document", func() {
			r.write(".github/ci-cadence/classify.sh", "#!/bin/sh\necho FULL\n")
			r.write(".github/qualifications/README.md", "# more\n")
		}},
		{"edits another job of ci.yml", func() {
			r.write(".github/workflows/ci.yml", strings.Replace(guardCI, "      - run: echo cadence", "      - run: echo cadence2", 1))
		}},
		{"changes only code", func() { r.write("internal/x/x.go", "package x\n\nvar Z = 3\n") }},
	} {
		t.Run("NARROWED: the pull request "+tc.name, func(t *testing.T) {
			mode, v := narrow(t, r.change("ok-"+strings.ReplaceAll(tc.name, " ", "-"), tc.edit))
			if mode != "NARROWED" || v["mode"] != "NARROWED" || fmt.Sprint(v["reasons"]) != "[]" || fmt.Sprint(v["githubInputs"]) != "[]" ||
				fmt.Sprint(v["selected"]) != fmt.Sprint([]string{a, b, c}) || v["placement"] != "BALANCED" {
				t.Fatalf("mode %s, record %v", mode, v)
			}
		})
	}
}
