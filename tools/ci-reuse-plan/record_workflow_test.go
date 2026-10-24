package main

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

// AFP-V0-170 (V1-2271): the ci-tested-tree-record job runs no repository code. Its only action
// is a checkout that persists no credentials, it sets no working directory, and every line of
// its one run script is a shell built-in, git or printf, so nothing the tested tree contains
// can write to its log.
func TestAFPV0170RecordJobRunsNoRepositoryCode(t *testing.T) {
	b := jobBlock(t, readWorkflow(t, "ci.yml"), "ci-tested-tree-record")
	wantLine(t, "ci-tested-tree-record", b, "    needs: [docs-plan, narrowing-verdict, go-product-shard]")
	wantLine(t, "ci-tested-tree-record", b, "          persist-credentials: false")
	if !strings.Contains(b, "needs.go-product-shard.result == 'success' && needs.narrowing-verdict.outputs.verdict != 'MET'") {
		t.Error("the record job must need every shard to succeed with no narrowing verdict")
	}
	uses := regexp.MustCompile(`(?m)^\s+(?:- )?uses: (\S+)`).FindAllStringSubmatch(b, -1)
	if len(uses) != 1 || !strings.HasPrefix(uses[0][1], "actions/checkout@") {
		t.Errorf("the record job may use only actions/checkout, got %v", uses)
	}
	if strings.Contains(b, "working-directory:") || strings.Contains(b, "shell:") {
		t.Error("the record job must run in the checkout root with the default shell")
	}
	allowed := regexp.MustCompile(`^(?:set -eu|test "\$\(git rev-parse HEAD\)" = "\$GITHUB_SHA"|if .*; then|fi|exit 0|echo '[^'$` + "`" + `\\]*'|tree=\$\(git rev-parse [^;|&` + "`" + `]*\)|printf '[^'%]*(?:%s[^'%]*)*\\n'(?: "\$[a-zA-Z_]+")*)$`)
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
			case "set", "test", "git", "if", "then", "fi", "echo", "exit", "printf", "tree", "head":
			default:
				t.Errorf("record job line %q runs %q", line, m[1])
			}
		}
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
		if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(stub), 0o700); err != nil {
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
