package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// AFP-V0-170 (V1-2271): the ci-tested-tree-record job runs no repository code. Its only action
// is a checkout that persists no credentials, it sets no working directory, and every line of
// its one run script is a shell built-in, git or printf, so nothing the tested tree contains
// can write to its log.
func TestAFPV0170RecordJobRunsNoRepositoryCode(t *testing.T) {
	b := jobBlock(t, readWorkflow(t, "ci.yml"), "ci-tested-tree-record")
	wantLine(t, "ci-tested-tree-record", b, "    needs: [docs-plan, narrowing-verdict, go-product-shard]")
	if !strings.Contains(b, "needs.go-product-shard.result == 'success' && (needs.narrowing-verdict.outputs.verdict != 'MET' || (") {
		t.Error("the record job must need every shard to succeed with no narrowing verdict or every shard FULL")
	}
	runsNoRepositoryCode(t, "ci-tested-tree-record", b)
}

// AFP-V0-734 (V1-3064): the ci-narrowed-tree-record job runs no repository code either, runs only
// in a pull-request run after go-product (and so the AFP-V0-591 coverage proof) and every shard
// passed under a MET verdict with some shard narrowed, and its script prints exactly one line the
// reuse decision parses, naming the tested merge's first parent as the selection base.
func TestAFPV0734NarrowedRecordJob(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the record script runs under sh against a real Git repository")
	b := jobBlock(t, readWorkflow(t, "ci.yml"), NarrowedRecordJob)
	wantLine(t, NarrowedRecordJob, b, "    needs: [docs-plan, narrowing-verdict, go-product-shard, go-product]")
	for _, want := range []string{"github.event_name == 'pull_request'", "needs.go-product.result == 'success'",
		"needs.go-product-shard.result == 'success'", "needs.narrowing-verdict.outputs.verdict == 'MET'",
		"!(needs.go-product-shard.outputs.full0 == 'FULL' &&", "needs.docs-plan.outputs.mode != 'REUSE'"} {
		if !strings.Contains(b, want) {
			t.Errorf("the narrowed record job condition lacks %q", want)
		}
	}
	if strings.Contains(b, "merge_group") {
		t.Error("only a pull-request run may print a narrowed record")
	}
	runsNoRepositoryCode(t, NarrowedRecordJob, b)
	// Run the script in a two-parent merge and parse what it prints.
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t",
			"GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	base := git("rev-parse", "HEAD")
	git("checkout", "-q", "-b", "pr")
	git("commit", "-q", "--allow-empty", "-m", "pr")
	head := git("rev-parse", "HEAD")
	git("checkout", "-q", "--detach", base)
	git("merge", "-q", "--no-ff", "-m", "merge", head)
	target, tree := git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")
	run := func(prHead string, prBase ...string) string {
		if len(prBase) == 0 {
			prBase = []string{base}
		}
		cmd := exec.Command("sh", "-c", runScript(t, b))
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GITHUB_SHA="+target, "PR_HEAD="+prHead, "PR_BASE="+prBase[0], "SHARDS=8")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("record script: %v\n%s", err, out)
		}
		return string(out)
	}
	m := narrowedLine.FindStringSubmatch(strings.TrimSpace(run(head)))
	if m == nil || m[1] != tree || m[2] != target || m[3] != base || m[4] != head || m[5] != "8" {
		t.Fatalf("narrowed record %q, want tree %s target %s base %s head %s shards 8", m, tree, target, base, head)
	}
	if out := run(base); narrowedLine.MatchString(strings.TrimSpace(out)) || !strings.Contains(out, "does not name the pull-request head") {
		t.Fatalf("a merge of another head printed %q", out)
	}
	// A merge ref recomputed on another first parent than the event's base covers another range.
	if out := run(head, head); narrowedLine.MatchString(strings.TrimSpace(out)) || !strings.Contains(out, "not built on the base its selection covered") {
		t.Fatalf("a merge on another base printed %q", out)
	}
}

// runsNoRepositoryCode checks a record job: its only action is a checkout that persists no
// credentials, it sets no working directory, and every line of its one run script is a shell
// built-in, git or printf.
func runsNoRepositoryCode(t *testing.T, name, b string) {
	t.Helper()
	wantLine(t, name, b, "          persist-credentials: false")
	uses := regexp.MustCompile(`(?m)^\s+(?:- )?uses: (\S+)`).FindAllStringSubmatch(b, -1)
	if len(uses) != 1 || !strings.HasPrefix(uses[0][1], "actions/checkout@") {
		t.Errorf("the record job may use only actions/checkout, got %v", uses)
	}
	if strings.Contains(b, "working-directory:") || strings.Contains(b, "shell:") {
		t.Error("the record job must run in the checkout root with the default shell")
	}
	allowed := regexp.MustCompile(`^(?:set -eu|test "\$\(git rev-parse HEAD\)" = "\$GITHUB_SHA"|if .*; then|fi|exit 0|echo '[^'$` + "`" + `\\]*'|(?:tree|base)=\$\(git rev-parse [^;|&` + "`" + `]*\)|printf '[^'%]*(?:%s[^'%]*)*\\n'(?: "\$[a-zA-Z_]+")*)$`)
	command := regexp.MustCompile(`(?:^|[;|&(!] *|\$\()([a-zA-Z_./][a-zA-Z0-9_./-]*)`)
	for _, line := range strings.Split(runScript(t, b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !allowed.MatchString(line) {
			t.Errorf("record job line %q is outside the no-repository-code allowlist", line)
		}
		unquoted := regexp.MustCompile(`'[^']*'`).ReplaceAllString(line, "''")
		for _, m := range command.FindAllStringSubmatch(unquoted, -1) {
			switch m[1] {
			case "set", "test", "git", "if", "then", "fi", "echo", "exit", "printf", "tree", "head", "base":
			default:
				t.Errorf("record job line %q runs %q", line, m[1])
			}
		}
	}
}

// AFP-V0-170 (V1-3051): under a MET narrowing verdict the record job still runs when every shard
// says, through its own job output, that it ran its complete package set. Each full<index> output
// is non-empty only in the shard of that index (matrix outputs are last-writer-wins), says FULL
// only when that shard's tests passed and its narrowing gate, the only route to a narrowed shard,
// did not say NARROWED, and reads no output of the tests step, which runs repository code. The
// record job needs all of them, so one narrowed, missing or overwritten shard records no tree.
func TestAFPV0170RecordNeedsEveryShardFullUnderAMetVerdict(t *testing.T) {
	w := readWorkflow(t, "ci.yml")
	shard := jobBlock(t, w, "go-product-shard")
	m := regexp.MustCompile(`(?m)^        shard: \[([0-9, ]+)\]$`).FindStringSubmatch(shard)
	if m == nil {
		t.Fatal("go-product-shard has no shard matrix")
	}
	n := len(strings.Split(m[1], ","))
	outs := regexp.MustCompile(`(?m)^      (full[0-9]+): (.*)$`).FindAllStringSubmatch(shard, -1)
	if len(outs) != n {
		t.Fatalf("go-product-shard has %d full<index> outputs, want one per shard (%d)", len(outs), n)
	}
	var all []string
	for i, o := range outs {
		want := fmt.Sprintf("${{ strategy.job-index == %d && steps.tests.outcome == 'success' && steps.narrow.outputs.mode != 'NARROWED' && 'FULL' || '' }}", i)
		if o[1] != fmt.Sprintf("full%d", i) || o[2] != want {
			t.Errorf("output %s: %s, want full%d: %s", o[1], o[2], i, want)
		}
		all = append(all, fmt.Sprintf("needs.go-product-shard.outputs.full%d == 'FULL'", i))
	}
	if regexp.MustCompile(`(?m)^      full[0-9]+: .*steps\.tests\.outputs\.`).MatchString(shard) {
		t.Error("a full<index> output must not read an output the tests step can write")
	}
	record := jobBlock(t, w, "ci-tested-tree-record")
	if !strings.Contains(record, "(needs.narrowing-verdict.outputs.verdict != 'MET' || ("+strings.Join(all, " && ")+"))") {
		t.Errorf("the record job must need every one of the %d full<index> outputs under a MET verdict", n)
	}
	// The narrowing gate is the only route to a narrowed shard: the tests step narrows only on
	// its NARROWED output, and the gate runs before the tests step.
	wantLine(t, "go-product-shard", shard, "          NARROW_MODE: ${{ steps.narrow.outputs.mode }}")
	gate, tests := strings.Index(shard, "        id: narrow\n"), strings.Index(shard, "        id: tests\n")
	if gate < 0 || tests < 0 || gate > tests {
		t.Error("the narrowing gate must run before the tests step")
	}
	if strings.Count(shard, `"$NARROW_MODE"`) != 1 || !strings.Contains(shard, `[ "$NARROW_MODE" = NARROWED ]`) {
		t.Error("the tests step must narrow only on a NARROWED gate output")
	}
}

// AFP-V0-170 (V1-2271): the reuse step of docs-plan, run end to end against a stub API, never
// reuses a tree on the strength of artifacts. A run that uploaded every
// ci-tested-tree-<tree>-full-<shard>-of-<shards> artifact but has no ci-tested-tree-record job,
// or whose record names another tree, stays FULL; only the record job's log of this exact tree
// decides REUSE. Before AFP-V0-170 the forged artifacts alone decided REUSE.
func TestAFPV0170ReuseStepRefusesForgedArtifacts(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the reuse step runs under bash with a #!/bin/sh gh spy")
	for _, tool := range []string{"bash", "jq", "git", "go", "tar"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the reuse step needs %s on PATH: %v", tool, err)
		}
	}
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "step.sh")
	if err := os.WriteFile(script, []byte(stepScript(t, jobBlock(t, readWorkflow(t, "ci.yml"), "docs-plan"), "reuse")), 0o600); err != nil {
		t.Fatal(err)
	}
	goEnv := map[string]string{}
	for _, k := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		out, err := exec.Command("go", "env", k).Output()
		if err != nil {
			t.Fatal(err)
		}
		goEnv[k] = strings.TrimSpace(string(out))
	}

	ws := t.TempDir()
	repo := filepath.Join(ws, "source")
	gitEnv := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "HOME=" + ws}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), gitEnv...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	put := func(rel, body string) {
		t.Helper()
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	git("init", "-q", "-b", "main")
	put("go.mod", "module github.com/Beamfall/corvint\n\ngo 1.27\n")
	put("tools/ci-reuse-plan/main.go", string(src))
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")
	git("checkout", "-q", "-b", "topic")
	put("change.txt", "pull request change\n")
	git("add", "-A")
	git("commit", "-q", "-m", "head")
	head := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "merge", "topic")
	target := git("rev-parse", "HEAD")
	tree := git("rev-parse", target+"^{tree}")
	git("update-ref", "refs/remotes/origin/main", base)

	const repoName = "beamfall/corvint"
	const runID, recordID = 39635500953, 119345281299
	shards := []map[string]any{}
	artifacts := []map[string]any{}
	for i := 0; i < 8; i++ {
		shards = append(shards, map[string]any{"id": 119345281200 + i, "run_id": runID, "name": fmt.Sprintf("go-product-shard (%d)", i),
			"head_sha": head, "status": "completed", "conclusion": "success"})
		artifacts = append(artifacts, map[string]any{"name": RecordName(tree, i, 8),
			"workflow_run": map[string]any{"id": runID, "head_sha": head}})
	}
	recordJob := map[string]any{"id": recordID, "run_id": runID, "name": RecordJob, "head_sha": head,
		"status": "completed", "conclusion": "success"}
	line := func(tree string) string {
		return fmt.Sprintf("2026-10-21T10:00:00.0000000Z corvint-ci-tested-tree/1 tree=%s target=%s head=%s shards=8\n",
			tree, strings.Repeat("e", 40), head)
	}
	decide := func(event string, jobs []map[string]any, log string) (string, string) {
		t.Helper()
		dir := t.TempDir()
		writeJSON(t, filepath.Join(dir, "runs.json"), map[string]any{"workflow_runs": []any{map[string]any{
			"id": runID, "head_sha": head, "event": "pull_request", "status": "completed", "conclusion": "success",
			"path": ".github/workflows/ci.yml", "repository": map[string]any{"full_name": repoName}}}})
		writeJSON(t, filepath.Join(dir, fmt.Sprintf("artifacts-%d.json", runID)), map[string]any{"artifacts": artifacts})
		writeJSON(t, filepath.Join(dir, fmt.Sprintf("jobs-%d.json", runID)), map[string]any{"jobs": jobs})
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("log-%d.txt", recordID)), []byte(log), 0o600); err != nil {
			t.Fatal(err)
		}
		// Shard 0 prints a well-formed record line to its own log, as any job running
		// repository code can.
		if err := os.WriteFile(filepath.Join(dir, "log-119345281200.txt"), []byte(line(tree)), 0o600); err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		stub := `#!/bin/sh
out= url=
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift ;;
    -H|--max-time|--max-filesize) shift ;;
    -*) ;;
    *) url=$1 ;;
  esac
  shift
done
case "$url" in
  "$STUB_API/workflows/ci.yml/runs?head_sha=$STUB_HEAD&event=pull_request&status=success&per_page=20") f=runs.json ;;
  "$STUB_API/runs/"*"/artifacts?per_page=100") id=${url#"$STUB_API/runs/"}; f=artifacts-${id%%/*}.json ;;
  "$STUB_API/runs/"*"/jobs?filter=latest&per_page=100") id=${url#"$STUB_API/runs/"}; f=jobs-${id%%/*}.json ;;
  "$STUB_API/jobs/"*"/logs") id=${url#"$STUB_API/jobs/"}; f=log-${id%%/*}.txt ;;
  *) exit 22 ;;
esac
[ -f "$STUB_DIR/$f" ] || exit 22
cat "$STUB_DIR/$f" > "$out"
`
		if err := execfile.WriteFile(filepath.Join(bin, "curl"), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
		output, summary := filepath.Join(dir, "output"), filepath.Join(dir, "summary")
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Dir = repo
		cmd.Env = append(gitEnv,
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GITHUB_EVENT_NAME="+event,
			"GITHUB_API_URL=https://api.github.test", "GITHUB_REPOSITORY="+repoName, "GITHUB_SHA="+target,
			"GITHUB_WORKSPACE="+ws, "RUNNER_TEMP="+dir, "GITHUB_OUTPUT="+output, "GITHUB_STEP_SUMMARY="+summary,
			"GH_TOKEN=t", "REUSE_SHARDS=8", "STUB_DIR="+dir, "STUB_HEAD="+head,
			"STUB_API=https://api.github.test/repos/"+repoName+"/actions",
			"GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off", "GOFLAGS=", "GOCACHE="+goEnv["GOCACHE"],
			"GOMODCACHE="+goEnv["GOMODCACHE"], "GOPATH="+goEnv["GOPATH"])
		combined, _ := cmd.CombinedOutput() // continue-on-error in the workflow; only the output binds
		o, _ := os.ReadFile(output)
		s, _ := os.ReadFile(summary)
		return string(o), string(s) + string(combined)
	}
	for _, event := range []string{"push", "merge_group"} {
		if out, log := decide(event, shards, ""); strings.Contains(out, "mode=REUSE") {
			t.Errorf("%s: forged artifacts with no record job decided REUSE\n%s", event, log)
		}
		if out, log := decide(event, append(shards[:8:8], recordJob), line(strings.Repeat("0", 40))); strings.Contains(out, "mode=REUSE") {
			t.Errorf("%s: a record of another tree decided REUSE\n%s", event, log)
		}
		if out, log := decide(event, append(shards[:8:8], recordJob), "##[group]Run set -eu\n"+line(tree)); out != "mode=REUSE\n" {
			t.Errorf("%s: the record job's log of this exact tree: output %q, want REUSE\n%s", event, out, log)
		}
	}
	// AFP-V0-734: a narrowed pull-request run's ci-narrowed-tree-record job vouches only for a
	// merge-queue entry whose first parent is the base it names; never for a main push.
	narrowedJob := map[string]any{"id": recordID, "run_id": runID, "name": NarrowedRecordJob, "head_sha": head,
		"status": "completed", "conclusion": "success"}
	narrowed := func(base string) string {
		return fmt.Sprintf("2026-10-28T10:00:00.0000000Z corvint-ci-tested-tree/2 scope=NARROWED tree=%s target=%s base=%s head=%s shards=8\n",
			tree, strings.Repeat("e", 40), base, head)
	}
	if out, log := decide("merge_group", append(shards[:8:8], narrowedJob), narrowed(base)); out != "mode=REUSE\n" ||
		!strings.Contains(log, `"scope":"NARROWED"`) {
		t.Errorf("merge_group: a narrowed record of this tree and base: output %q, want REUSE\n%s", out, log)
	}
	// AFP-V0-812: a narrowed record of another base is FULL and hands the record on as the delta
	// candidate, one JSON line naming the record's run, job, tree, target, base and head.
	out, log := decide("merge_group", append(shards[:8:8], narrowedJob), narrowed(head))
	if strings.Contains(out, "mode=REUSE") || !strings.HasPrefix(out, "delta=") || strings.Count(out, "\n") != 1 {
		t.Errorf("merge_group: a narrowed record of another base: output %q, want only a delta\n%s", out, log)
	} else {
		var got delta
		want := delta{RunID: runID, RecordJobID: recordID, Tree: tree, Target: strings.Repeat("e", 40), Base: head, Head: head}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(out), "delta=")), &got); err != nil || got != want {
			t.Errorf("merge_group: delta = %+v (%v), want %+v", got, err, want)
		}
	}
	if out, log := decide("merge_group", append(shards[:8:8], narrowedJob), narrowed(base)); strings.Contains(out, "delta=") {
		t.Errorf("merge_group: an exact narrowed reuse also named a delta %q\n%s", out, log)
	}
	if out, log := decide("push", append(shards[:8:8], narrowedJob), narrowed(base)); out != "" {
		t.Errorf("push: a narrowed record decided %q, want nothing\n%s", out, log)
	}
}

// stepScript extracts the run script of the step with id ID in job block B.
func stepScript(t *testing.T, b, id string) string {
	t.Helper()
	marker := "        id: " + id + "\n"
	if strings.Count(b, marker) != 1 {
		t.Fatalf("want exactly one step %q", id)
	}
	_, rest, _ := strings.Cut(b, marker)
	if next := strings.Index(rest, "\n      - "); next >= 0 {
		rest = rest[:next+1]
	}
	return runScript(t, rest)
}
