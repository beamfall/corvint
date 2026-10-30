// SPDX-License-Identifier: Apache-2.0
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

// stackedGroup builds a repository holding a two-entry stacked merge-queue group: main at base,
// entry one merging pull request one onto base, and entry two merging pull request two onto
// entry one, as GitHub builds a grouped queue. It returns the repository and the three commits.
func stackedGroup(t *testing.T) (repo, base, entry1, entry2 string) {
	t.Helper()
	repo = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, body string) { writeFile(t, filepath.Join(repo, path), []byte(body)) }
	git("init", "-q", "-b", "main")
	write("go.mod", "module example.org/m\n\ngo 1.27.3\n")
	write("a/a.go", "package a\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base = git("rev-parse", "HEAD")
	git("update-ref", "refs/remotes/origin/main", base)
	git("checkout", "-q", "-b", "pr1", base)
	write("a/one.go", "package a\n")
	git("add", "-A")
	git("commit", "-q", "-m", "one")
	git("checkout", "-q", "-b", "pr2", base)
	write("b/b.go", "package b\n")
	git("add", "-A")
	git("commit", "-q", "-m", "two")
	git("checkout", "-q", "--detach", base)
	git("merge", "-q", "--no-ff", "-m", "entry one", "pr1")
	entry1 = git("rev-parse", "HEAD")
	git("merge", "-q", "--no-ff", "-m", "entry two", "pr2")
	entry2 = git("rev-parse", "HEAD")
	if p := git("show", "-s", "--format=%P", entry2); !strings.HasPrefix(p, entry1+" ") {
		t.Fatalf("entry two's parents are %s, want entry one first", p)
	}
	return repo, base, entry1, entry2
}

// TestAFPV0731MergeQueueSelection runs mq-narrow.sh, and through it narrow.sh, in the second entry
// of a two-entry stacked group with fake pinned tools. The planner must be asked for the whole
// queue range (the queue base, not the entry's first parent), an admitted `run` verdict narrows
// the shard to its part of the selection, and an UNKNOWN plan (selector FALLBACK), a planner that
// fails or names another range, a head other than the tested commit, a base that is not main's,
// or a queue range touching .github/ records FULL.
func TestAFPV0731MergeQueueSelection(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the planner, selector, gh, go and shadow spies are #!/bin/sh scripts run by mq-narrow.sh under sh")
	repo, base, entry1, entry2 := stackedGroup(t)
	const a, b, c = "example.org/m/a", "example.org/m/b", "example.org/m/c"
	type result struct {
		mode, summary, planned string
		record                 doc
	}
	narrow := func(t *testing.T, env map[string]string, override map[string]string) result {
		t.Helper()
		dir := t.TempDir()
		bin := filepath.Join(dir, "bin")
		fake := filepath.Join(dir, "fake")
		// The planner echoes its --base and HEAD into a receipt, as `corvint affected` does.
		tools := map[string]string{
			"corvint": "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$FAKE/planned\"\nwhile [ $# -gt 0 ]; do [ \"$1\" != --base ] || b=$2; shift; done\n" +
				"printf '{\"profile\":\"affected-plan/1\",\"tool\":\"affected\",\"ok\":true,\"mutates\":false,\"revision\":\"%s\",\"range\":{\"base\":\"%s\"}}\\n' \"$(git rev-parse HEAD)\" \"$b\"\n",
			"selector":  "#!/bin/sh\necho 'selected " + a + "'\necho 'run " + a + " " + b + "'\n",
			"gh":        "#!/bin/sh\necho true\n",
			"go":        "#!/bin/sh\nprintf '%s\\n' " + a + " " + b + " " + c + "\n",
			"shadow":    "#!/bin/sh\nprintf '%s\\n' '{\"scope\":\"NARROWED\",\"incomplete\":[],\"invocations\":[{\"units\":[{\"adapter\":\"go\",\"unit\":\"go:" + a + "\"}]}]}'\n",
			"ci-shards": "#!/bin/sh\ncase \"$*\" in *--slices*) ;; *--select*) echo " + b + " ;; *) printf '%s\\n' " + a + " " + c + " ;; esac\n",
		}
		for name, body := range override {
			tools[name] = body
		}
		for name, body := range tools {
			writeFile(t, filepath.Join(bin, name), []byte(body))
			if err := os.Chmod(filepath.Join(bin, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.MkdirAll(fake, 0o755); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(dir, "output")
		writeFile(t, output, nil)
		vars := map[string]string{"PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME": dir, "FAKE": fake,
			"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
			"GH_TOKEN": "x", "REPO": "beamfall/corvint", "QUEUE_BASE": base, "QUEUE_BASE_REF": "refs/heads/main", "QUEUE_HEAD": entry2,
			"QUEUE_HEAD_REF": "refs/heads/gh-readonly-queue/main/pr-77-" + entry1, "TESTED_MERGE": entry2, "SHARD": "3", "SHARDS": "8",
			"TOOL_SOURCE": base, "VERDICT_RUN": "4242", "PR_BIN": bin, "SHADOW": filepath.Join(bin, "shadow"),
			"CI_SHARDS": filepath.Join(bin, "ci-shards"), "CONFINE": filepath.Join(bin, "confine"), "GITHUB_RUN_ID": prRun,
			"GITHUB_RUN_ATTEMPT": "1", "RUNNER_TEMP": dir, "GITHUB_OUTPUT": output, "GITHUB_STEP_SUMMARY": filepath.Join(dir, "summary")}
		for k, v := range env {
			vars[k] = v
		}
		cmd := exec.Command("sh", script(t, "mq-narrow.sh"))
		cmd.Dir = repo
		for k, v := range vars {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mq-narrow.sh: %v\n%s", err, out)
		}
		o, _ := os.ReadFile(output)
		m := regexp.MustCompile(`(?m)^mode=(.*)$`).FindStringSubmatch(string(o))
		if m == nil {
			t.Fatalf("mq-narrow.sh wrote no mode:\n%s", o)
		}
		r := result{mode: m[1]}
		s, _ := os.ReadFile(filepath.Join(dir, "summary"))
		r.summary = string(s)
		p, _ := os.ReadFile(filepath.Join(fake, "planned"))
		r.planned = strings.TrimSpace(string(p))
		if raw, err := os.ReadFile(filepath.Join(dir, "corvint-narrowing", "narrowing.json")); err == nil {
			if err := jsonUnmarshal(string(raw), &r.record); err != nil {
				t.Fatal(err)
			}
		}
		return r
	}
	t.Run("an admitted selection of the stacked range narrows the second entry", func(t *testing.T) {
		r := narrow(t, nil, nil)
		if r.mode != "NARROWED" || r.record["mode"] != "NARROWED" || r.record["base"] != base || r.record["target"] != entry2 ||
			fmt.Sprint(r.record["selected"]) != fmt.Sprint([]string{a, b}) || fmt.Sprint(r.record["executed"]) != fmt.Sprint([]string{b}) ||
			fmt.Sprint(r.record["pr"]) != "77" {
			t.Fatalf("mode %s, record %v\n%s", r.mode, r.record, r.summary)
		}
		// The range is the queue base .. the entry, not the entry's first parent (entry one).
		if !strings.Contains(r.planned, "affected --base "+base) || strings.Contains(r.planned, entry1) {
			t.Fatalf("planner argv %q, want affected --base %s", r.planned, base)
		}
	})
	for _, tc := range []struct {
		name     string
		env      map[string]string
		override map[string]string
		reason   string
	}{
		{"an UNKNOWN plan (selector FALLBACK) is FULL", nil,
			map[string]string{"selector": "#!/bin/sh\necho 'FALLBACK the plan is UNKNOWN'\n"}, "selector: FALLBACK the plan is UNKNOWN"},
		{"a selection of nothing is FULL", nil,
			map[string]string{"selector": "#!/bin/sh\necho 'NOTHING no package'\n"}, "selector: NOTHING no package"},
		{"a failing planner is FULL", nil,
			map[string]string{"corvint": "#!/bin/sh\necho boom >&2\nexit 1\n"}, "planner failed: boom"},
		{"a receipt of another range is FULL", nil,
			map[string]string{"corvint": "#!/bin/sh\nprintf '{\"profile\":\"affected-plan/1\",\"tool\":\"affected\",\"ok\":true,\"mutates\":false,\"revision\":\"%s\",\"range\":{\"base\":\"" + entry1 + "\"}}\\n' \"$(git rev-parse HEAD)\"\n"},
			"malformed or mismatched planner receipt"},
		{"a package outside the module is FULL", nil,
			map[string]string{"selector": "#!/bin/sh\necho 'run example.org/other'\n"}, "invalid executable package argument"},
		{"a queue head other than the tested commit is FULL", map[string]string{"QUEUE_HEAD": entry1}, nil,
			"the head is not the tested commit"},
		{"a queue base that is not main is FULL", map[string]string{"QUEUE_BASE_REF": "refs/heads/release"}, nil,
			"not refs/heads/main"},
		{"a queue base outside the entry's history is FULL", map[string]string{"QUEUE_BASE": strings.Repeat("a", 40)}, nil,
			"not an ancestor of both the queue head and main"},
		{"an empty run verdict is FULL", nil,
			map[string]string{"selector": "#!/bin/sh\necho 'run '\n"}, "run names no package"},
		{"a glob in the verdict is not expanded", nil,
			map[string]string{"selector": "#!/bin/sh\necho 'run example.org/m/*'\n"}, "invalid executable package argument"},
		{"a queue base that main does not contain is FULL", map[string]string{"QUEUE_BASE": entry1}, nil,
			"not an ancestor of both the queue head and main"},
		{"a range touching .github/ is FULL", nil,
			map[string]string{"git": "#!/bin/sh\ncase \"$1\" in diff) echo .github/workflows/ci.yml ;; *) exec REALGIT \"$@\" ;; esac\n"},
			"the change touches .github/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.override["git"] != "" {
				real, err := exec.LookPath("git")
				if err != nil {
					t.Skip("git not found")
				}
				tc.override = map[string]string{"git": strings.Replace(tc.override["git"], "REALGIT", real, 1)}
			}
			r := narrow(t, tc.env, tc.override)
			if r.mode != "FULL" || (r.record != nil && r.record["mode"] != "FULL") || !strings.Contains(r.summary, tc.reason) {
				t.Fatalf("mode %s, record %v, summary:\n%s\nwant FULL, naming %q", r.mode, r.record, r.summary, tc.reason)
			}
		})
	}
}

// TestAFPV0733NarrowedQueueRunIsNoRow runs collect.sh over a queue run of a narrowed pull request.
// A queue run that narrowed its own shards (a NARROWED ci-narrowing record of its own) or reused a
// narrowed pull-request run (a REUSE decision of scope NARROWED) holds no full outcomes, so it is
// no row at all, rather than an INCOMPLETE row that would trip the kill switch or a CLEAN row that
// would vouch for packages nobody ran. A queue run whose own gate recorded FULL is still a row.
func TestAFPV0733NarrowedQueueRunIsNoRow(t *testing.T) {
	collect := func(t *testing.T, queueArtifacts map[string]doc) (output, summary string) {
		dir := t.TempDir()
		gh := fakeGH(t, dir)
		writeDoc(t, filepath.Join(dir, "commit.json"), doc{"parents": []any{doc{"sha": mainBase}, doc{"sha": prHead}}})
		writeDoc(t, filepath.Join(dir, "compare.json"), doc{"files": []any{doc{"filename": "internal/contextindex/index.go"}}})
		writeFile(t, filepath.Join(dir, "selection-run"), lines(prRun))
		f := queueFixture(t)
		p := strs(f.sel[0]["selected"])[0]
		fail(f.out[0], p, "fail", "RAN", "pass")
		f.nar = map[int]doc{0: narrowingRecord(0, "NARROWED", prRun, nil, []string{p})}
		f.stampEpoch(1) // CIN-V0-001: the collector runs at FORWARD_EPOCH=1
		selDir, outDir := f.write(t, filepath.Join(dir, "src"))
		move := func(src, run string) []string {
			var names []string
			entries, _ := os.ReadDir(src)
			for _, e := range entries {
				names = append(names, e.Name())
				if err := os.MkdirAll(filepath.Join(dir, "runs", run), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(src, e.Name()), filepath.Join(dir, "runs", run, e.Name())); err != nil {
					t.Fatal(err)
				}
			}
			return names
		}
		queueNames := move(outDir, queueRun)
		for name, d := range queueArtifacts {
			file := "narrowing.json"
			if name == "ci-reuse" {
				file = "decision.json"
			}
			writeDoc(t, filepath.Join(dir, "runs", queueRun, name, file), d)
			queueNames = append(queueNames, name)
		}
		writeFile(t, filepath.Join(dir, "names-"+queueRun), lines(queueNames...))
		prNames := append(move(selDir, prRun), move(filepath.Join(dir, "src", "narrowing"), prRun)...)
		writeFile(t, filepath.Join(dir, "names-"+prRun), lines(prNames...))
		out, sum := filepath.Join(dir, "output"), filepath.Join(dir, "summary")
		writeFile(t, out, nil)
		cmd := exec.Command("sh", script(t, "collect.sh"))
		cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
			"GH_TOKEN=x", "REPO=beamfall/corvint", "RUN_ID=" + queueRun, "EVENT=merge_group", "HEAD_SHA=" + queueSHA,
			"HEAD_BRANCH=gh-readonly-queue/main/pr-868-" + mainBase, "PR_NUMBER=", "PR_BASE_SHA=" + mainBase,
			"FORWARD_SHARDS=8", "FORWARD_EPOCH=1", "RUNNER_TEMP=" + filepath.Join(dir, "temp"),
			"GITHUB_OUTPUT=" + out, "GITHUB_STEP_SUMMARY=" + sum}
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("collect.sh: %v\n%s", err, o)
		}
		o, _ := os.ReadFile(out)
		s, _ := os.ReadFile(sum)
		return strings.TrimSpace(string(o)), string(s)
	}
	t.Run("a queue run that narrowed its own shards", func(t *testing.T) {
		o, s := collect(t, map[string]doc{"ci-narrowing-3": narrowingRecord(3, "NARROWED", queueRun, []string{"example.org/m/a"}, nil),
			"ci-narrowing-4": narrowingRecord(4, "FULL", queueRun, nil, nil)})
		if o != "" || !strings.Contains(s, "narrowed its own shards (AFP-V0-731)") {
			t.Fatalf("output %q, summary:\n%s", o, s)
		}
	})
	t.Run("a queue run that reused a narrowed pull-request run", func(t *testing.T) {
		o, s := collect(t, map[string]doc{"ci-reuse": {"profile": "corvint-ci-reuse/0", "mode": "REUSE", "scope": "NARROWED", "runId": 1}})
		if o != "" || !strings.Contains(s, "reused a narrowed pull-request run (AFP-V0-734)") {
			t.Fatalf("output %q, summary:\n%s", o, s)
		}
	})
	t.Run("a queue run whose own gate recorded FULL is still a row", func(t *testing.T) {
		o, s := collect(t, map[string]doc{"ci-narrowing-3": narrowingRecord(3, "FULL", queueRun, nil, nil)})
		if !strings.HasPrefix(o, "name=ci-forward-MISS-pr868-"+queueSHA+"-e1-f1") {
			t.Fatalf("output %q, summary:\n%s", o, s)
		}
	})
}

// AFP-V0-800: the shadow resolves exactly the same PR range as the driver.
func TestAFPV0800ShadowBase(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "pr-base.sh is a POSIX shell script")
	repo, base, first, target := stackedGroup(t)
	cmd := exec.Command("git", "rev-parse", target+"^2")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(out))
	for _, tc := range []struct{ name, event, head, target, checkout, want, reason string }{
		{"stale", base, head, target, target, first, ""},
		{"current", first, head, target, target, first, ""},
		{"head", base, base, target, target, "", "pr-base-head-mismatch"},
		{"ancestor", head, head, target, target, "", "pr-base-not-ancestor"},
		{"count", base, head, head, head, "", "pr-base-parent-count"},
		{"head-drift", base, head, target, first, "", "pr-base-head-drift"},
		{"invalid-base", "invalid", head, target, target, "", "pr-base-invalid-oid"},
		{"invalid-head", base, "invalid", target, target, "", "pr-base-invalid-oid"},
		{"invalid-target", base, head, "invalid", target, "", "pr-base-invalid-oid"},
		{"missing-base", "", head, target, target, "", "pr-base-invalid-oid"},
		{"missing-head", base, "", target, target, "", "pr-base-invalid-oid"},
		{"missing-target", base, head, "", target, "", "pr-base-invalid-oid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkout := exec.Command("git", "checkout", "--detach", tc.checkout)
			checkout.Dir = repo
			if out, err := checkout.CombinedOutput(); err != nil {
				t.Fatalf("checkout: %s %v", out, err)
			}
			cmd := exec.Command("sh", script(t, "pr-base.sh"), tc.event, tc.head, tc.target)
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			if tc.reason == "" {
				if err != nil || strings.TrimSpace(string(out)) != tc.want {
					t.Fatalf("base: %s %v", out, err)
				}
			} else if err == nil || !strings.Contains(string(out), tc.reason) {
				t.Fatalf("refusal: %s %v", out, err)
			}
		})
	}
}

// movedBase builds a pull request whose narrowed run tested prTree, the merge of recordBase and
// head, and the merge-queue entry of the same head on queueBase, a later commit of main: main
// moved by c/c.go (and, with github, a .github/ file) after the pull-request run.
func movedBase(t *testing.T, github string) (repo, recordBase, head, queueBase, entry, prTree string) {
	t.Helper()
	repo = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, body string) { writeFile(t, filepath.Join(repo, path), []byte(body)) }
	git("init", "-q", "-b", "main")
	write("go.mod", "module example.org/m\n\ngo 1.27.3\n")
	write("a/a.go", "package a\n")
	git("add", "-A")
	git("commit", "-q", "-m", "record base")
	recordBase = git("rev-parse", "HEAD")
	git("checkout", "-q", "-b", "pr", recordBase)
	write("b/b.go", "package b\n")
	git("add", "-A")
	git("commit", "-q", "-m", "pull request")
	head = git("rev-parse", "HEAD")
	git("checkout", "-q", "--detach", recordBase)
	git("merge", "-q", "--no-ff", "-m", "pull-request run", head)
	prTree = git("rev-parse", "HEAD^{tree}")
	git("checkout", "-q", "main")
	write("c/c.go", "package c\n")
	if github != "" {
		write(github, "on: push\n")
	}
	git("add", "-A")
	git("commit", "-q", "-m", "main moved")
	queueBase = git("rev-parse", "HEAD")
	git("update-ref", "refs/remotes/origin/main", queueBase)
	git("checkout", "-q", "--detach", queueBase)
	git("merge", "-q", "--no-ff", "-m", "queue entry", head)
	entry = git("rev-parse", "HEAD")
	return repo, recordBase, head, queueBase, entry, prTree
}

// TestAFPV0813MergeQueueReusesTheNarrowedRecordOfAMovedBase runs mq-narrow.sh, and through it
// narrow.sh, on a queue entry whose pull request's narrowed run tested an older base of main. The
// fake selector selects a, b and c over the queue range, a and b over the record's range and b
// over the main delta (from the record tree), so the entry runs b (changed by the delta) and c
// (outside the record's range) and reuses a, recording the reuse. Every refusal keeps the whole
// queue-range selection and names why (AFP-V0-816).
func TestAFPV0813MergeQueueReusesTheNarrowedRecordOfAMovedBase(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the planner, selector, gh, go and shadow spies are #!/bin/sh scripts run by mq-narrow.sh under sh")
	const a, b, c = "example.org/m/a", "example.org/m/b", "example.org/m/c"
	const recordRun, recordJob = 40803231280, 119900000001
	type result struct {
		mode, summary, planned string
		record                 doc
	}
	selector := func(queueBase, recordBase, delta string) string {
		return "#!/bin/sh\ncase $(jq -r .range.base \"$1\") in\n" + queueBase + ") echo 'run " + a + " " + b + " " + c + "' ;;\n" +
			recordBase + ") echo 'run " + a + " " + b + "' ;;\n*) echo '" + delta + "' ;;\nesac\n"
	}
	narrow := func(t *testing.T, github string, edit func(env map[string]string, tools map[string]string, recordBase, head, queueBase, entry, prTree string)) (result, string, string) {
		t.Helper()
		repo, recordBase, head, queueBase, entry, prTree := movedBase(t, github)
		dir := t.TempDir()
		bin := filepath.Join(dir, "bin")
		fake := filepath.Join(dir, "fake")
		delta := fmt.Sprintf(`{"runId":%d,"recordJobId":%d,"tree":"%s","target":"%s","base":"%s","head":"%s"}`,
			recordRun, recordJob, prTree, strings.Repeat("e", 40), recordBase, head)
		tools := map[string]string{
			// The planner appends its --base to a log and echoes it into a receipt whose plan scope is
			// BOUNDED, except for a base in UNKNOWN_BASES or, when BOUNDED_BASES is set, one outside it.
			"corvint": "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" != --base ] || b=$2; shift; done\nprintf '%s\\n' \"$b\" >> \"$FAKE/planned\"\n" +
				"s=BOUNDED\ncase \" ${UNKNOWN_BASES-} \" in *\" $b \"*) s=UNKNOWN ;; esac\n" +
				"if [ -n \"${BOUNDED_BASES-}\" ]; then case \" $BOUNDED_BASES \" in *\" $b \"*) ;; *) s=UNKNOWN ;; esac; fi\n" +
				"printf '{\"profile\":\"affected-plan/1\",\"tool\":\"affected\",\"ok\":true,\"mutates\":false,\"revision\":\"%s\",\"range\":{\"base\":\"%s\"},\"plan\":{\"scope\":\"%s\"}}\\n' \"$(git rev-parse HEAD)\" \"$b\" \"$s\"\n",
			"selector":  selector(queueBase, recordBase, "run "+b),
			"gh":        "#!/bin/sh\necho true\n",
			"go":        "#!/bin/sh\nprintf '%s\\n' " + a + " " + b + " " + c + "\n",
			"shadow":    "#!/bin/sh\nprintf '%s\\n' '{\"scope\":\"NARROWED\",\"incomplete\":[],\"invocations\":[{\"units\":[{\"adapter\":\"go\",\"unit\":\"go:" + a + "\"}]}]}'\n",
			"ci-shards": "#!/bin/sh\ncase \"$*\" in *--slices*) ;; *--select*) echo " + b + " ;; *) printf '%s\\n' " + a + " " + c + " ;; esac\n",
		}
		output := filepath.Join(dir, "output")
		vars := map[string]string{"PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME": dir, "FAKE": fake,
			"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
			"GH_TOKEN": "x", "REPO": "beamfall/corvint", "QUEUE_BASE": queueBase, "QUEUE_BASE_REF": "refs/heads/main", "QUEUE_HEAD": entry,
			"QUEUE_HEAD_REF": "refs/heads/gh-readonly-queue/main/pr-1991-" + queueBase, "TESTED_MERGE": entry, "SHARD": "3", "SHARDS": "8",
			"TOOL_SOURCE": queueBase, "VERDICT_RUN": "4242", "PR_BIN": bin, "SHADOW": filepath.Join(bin, "shadow"),
			"CI_SHARDS": filepath.Join(bin, "ci-shards"), "CONFINE": filepath.Join(bin, "confine"), "GITHUB_RUN_ID": prRun,
			"GITHUB_RUN_ATTEMPT": "1", "RUNNER_TEMP": dir, "GITHUB_OUTPUT": output, "GITHUB_STEP_SUMMARY": filepath.Join(dir, "summary"),
			"MQ_DELTA": delta}
		if edit != nil {
			edit(vars, tools, recordBase, head, queueBase, entry, prTree)
		}
		for name, body := range tools {
			writeFile(t, filepath.Join(bin, name), []byte(body))
			if err := os.Chmod(filepath.Join(bin, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.MkdirAll(fake, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, output, nil)
		cmd := exec.Command("sh", script(t, "mq-narrow.sh"))
		cmd.Dir = repo
		for k, v := range vars {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mq-narrow.sh: %v\n%s", err, out)
		}
		o, _ := os.ReadFile(output)
		m := regexp.MustCompile(`(?m)^mode=(.*)$`).FindStringSubmatch(string(o))
		if m == nil {
			t.Fatalf("mq-narrow.sh wrote no mode:\n%s", o)
		}
		r := result{mode: m[1]}
		s, _ := os.ReadFile(filepath.Join(dir, "summary"))
		r.summary = string(s)
		p, _ := os.ReadFile(filepath.Join(fake, "planned"))
		r.planned = strings.TrimSpace(string(p))
		if raw, err := os.ReadFile(filepath.Join(dir, "corvint-narrowing", "narrowing.json")); err == nil {
			if err := jsonUnmarshal(string(raw), &r.record); err != nil {
				t.Fatal(err)
			}
		}
		if st, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output(); err != nil || len(st) != 0 {
			t.Fatalf("mq-narrow.sh left the checkout dirty: %q %v", st, err)
		}
		return r, recordBase, queueBase
	}
	t.Run("only the main delta and the packages outside the record's range run", func(t *testing.T) {
		r, recordBase, queueBase := narrow(t, "", nil)
		reuse, _ := r.record["reuse"].(doc)
		if r.mode != "NARROWED" || fmt.Sprint(r.record["selected"]) != fmt.Sprint([]string{b, c}) ||
			fmt.Sprint(r.record["reused"]) != fmt.Sprint([]string{a}) || fmt.Sprint(r.record["executed"]) != fmt.Sprint([]string{b}) ||
			fmt.Sprint(r.record["skipped"]) != "[]" || reuse == nil || reuse["applied"] != true ||
			reuse["recordBase"] != recordBase || reuse["queueBase"] != queueBase || reuse["recordRunId"] != float64(recordRun) ||
			reuse["recordJobId"] != float64(recordJob) || reuse["selected"] != float64(3) || reuse["intersection"] != float64(2) ||
			reuse["reused"] != float64(1) || !strings.Contains(r.summary, "reused 1 from run 40803231280") {
			t.Fatalf("mode %s, record %v\n%s", r.mode, r.record, r.summary)
		}
		// The planner covered the queue range, the record tree's synthetic commit and the record range.
		planned := strings.Split(r.planned, "\n")
		if len(planned) != 3 || planned[0] != queueBase || planned[1] != reuse["deltaBase"] || planned[2] != recordBase {
			t.Fatalf("planner bases %q, want %s, the delta base %v and %s", planned, queueBase, reuse["deltaBase"], recordBase)
		}
	})
	t.Run("a main delta that selects nothing reuses the whole record range", func(t *testing.T) {
		r, _, _ := narrow(t, "", func(_ map[string]string, tools map[string]string, recordBase, _, queueBase, _, _ string) {
			tools["selector"] = selector(queueBase, recordBase, "NOTHING no dirty or committed path; no Go package to test")
			tools["ci-shards"] = strings.Replace(tools["ci-shards"], "*--select*) echo "+b, "*--select*) echo "+c, 1)
		})
		if r.mode != "NARROWED" || fmt.Sprint(r.record["selected"]) != fmt.Sprint([]string{c}) ||
			fmt.Sprint(r.record["reused"]) != fmt.Sprint([]string{a, b}) {
			t.Fatalf("mode %s, record %v\n%s", r.mode, r.record, r.summary)
		}
	})
	for _, tc := range []struct {
		name   string
		github string
		edit   func(env map[string]string, tools map[string]string, recordBase, head, queueBase, entry, prTree string)
		reason string
	}{
		{"no offered record", "", func(env map[string]string, _ map[string]string, _, _, _, _, _ string) { delete(env, "MQ_DELTA") },
			"no narrowed pull-request record of this head was offered"},
		{"a malformed record", "", func(env map[string]string, _ map[string]string, _, _, _, _, _ string) {
			env["MQ_DELTA"] = `{"runId":"1"}`
		},
			"the offered record is malformed"},
		{"a record of another head", "", func(env map[string]string, _ map[string]string, recordBase, head, _, _, _ string) {
			env["MQ_DELTA"] = strings.Replace(env["MQ_DELTA"], `"head":"`+head, `"head":"`+recordBase, 1)
		}, "the entry is not the record head merged onto the queue base"},
		{"an entry not built directly on the queue base", "", func(env map[string]string, _ map[string]string, recordBase, _, _, _, _ string) {
			// The queue range is then the record's own range, which selects a and b.
			env["QUEUE_BASE"] = recordBase
		}, "the entry is not the record head merged onto the queue base"},
		{"a record base off the queue base's history", "", func(env map[string]string, _ map[string]string, _, head, _, _, _ string) {
			env["MQ_DELTA"] = regexp.MustCompile(`"base":"[0-9a-f]{40}"`).ReplaceAllString(env["MQ_DELTA"], `"base":"`+head+`"`)
		}, "the record base is not an ancestor of the queue base"},
		{"a record tree that is not the merge of its base and head", "", func(env map[string]string, _ map[string]string, _, _, _, _, prTree string) {
			env["MQ_DELTA"] = strings.Replace(env["MQ_DELTA"], prTree, strings.Repeat("0", 40), 1)
		}, "the record tree is not the merge of the record base and head"},
		{"a main delta touching .github/", ".github/workflows/x.yml", nil, "the record base or tree differs from the entry under .github/"},
		// git quotes a non-ASCII name in --name-only output, so only the exit status sees this path.
		{"a main delta touching a non-ASCII .github/ path", ".github/\u00e9.yml", nil,
			"the record base or tree differs from the entry under .github/"},
		{"an UNKNOWN main-delta receipt under a run verdict", "", func(env map[string]string, _ map[string]string, recordBase, _, queueBase, _, _ string) {
			env["BOUNDED_BASES"] = queueBase + " " + recordBase
		}, "main delta: plan scope is UNKNOWN, not BOUNDED"},
		{"an UNKNOWN record-range receipt under a run verdict", "", func(env map[string]string, _ map[string]string, recordBase, _, _, _, _ string) {
			env["UNKNOWN_BASES"] = recordBase
		}, "record range: plan scope is UNKNOWN, not BOUNDED"},
		{"an UNKNOWN main-delta plan", "", func(_ map[string]string, tools map[string]string, recordBase, _, queueBase, _, _ string) {
			tools["selector"] = selector(queueBase, recordBase, "FALLBACK the plan is UNKNOWN")
		}, "main delta: selector: FALLBACK the plan is UNKNOWN"},
		{"an UNKNOWN record-range plan", "", func(_ map[string]string, tools map[string]string, recordBase, _, queueBase, _, _ string) {
			tools["selector"] = strings.Replace(selector(queueBase, recordBase, "run "+b), recordBase+") echo 'run "+a+" "+b+"'",
				recordBase+") echo 'FALLBACK the plan is UNKNOWN'", 1)
		}, "record range: selector: FALLBACK the plan is UNKNOWN"},
		{"a failing main-delta planner", "", func(_ map[string]string, tools map[string]string, recordBase, _, queueBase, _, _ string) {
			tools["corvint"] = strings.Replace(tools["corvint"], "printf '%s\\n' \"$b\" >> \"$FAKE/planned\"\n",
				"printf '%s\\n' \"$b\" >> \"$FAKE/planned\"\ncase $b in "+queueBase+"|"+recordBase+") ;; *) echo boom >&2; exit 1 ;; esac\n", 1)
		}, "main delta: planner failed: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, _ := narrow(t, tc.github, tc.edit)
			reuse, _ := r.record["reuse"].(doc)
			want := []string{a, b, c}
			if tc.name == "an entry not built directly on the queue base" {
				want = []string{a, b}
			}
			if r.mode != "NARROWED" || fmt.Sprint(r.record["selected"]) != fmt.Sprint(want) || fmt.Sprint(r.record["reused"]) != "[]" ||
				reuse == nil || reuse["applied"] != false || !strings.Contains(fmt.Sprint(reuse["reason"]), tc.reason) ||
				!strings.Contains(r.summary, "no reuse (AFP-V0-816): "+tc.reason) {
				t.Fatalf("mode %s, record %v, summary:\n%s\nwant the whole queue-range selection, naming %q", r.mode, r.record, r.summary, tc.reason)
			}
		})
	}
}
