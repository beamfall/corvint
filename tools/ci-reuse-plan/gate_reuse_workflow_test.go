// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// gateReuse names one AFP-V0-670 gate: its workflow, decision job, record job and gated jobs.
type gateReuse struct {
	workflow, decide, record, label string
	gated                           []string
}

var gateReuses = []gateReuse{
	{"ci.yml", "doc-gates-reuse", "doc-gates-record", "doc-gates", []string{"doc-gates"}},
	{"cem-release-gate.yml", "cem-release-reuse", "cem-release-record", "cem-release", []string{"cem-release-gate", "cem-release-readiness"}},
}

const gateReuseScript = "../../.github/gate-reuse/find.sh"

// decisionEnv reads the GATE, WORKFLOW, GATE_JOBS and RECORD_JOB values of decision job block B.
func decisionEnv(t *testing.T, b string) map[string]string {
	t.Helper()
	env := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^          (GATE|WORKFLOW|GATE_JOBS|RECORD_JOB): (.*)$`).FindAllStringSubmatch(b, -1) {
		v := m[2]
		if strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") {
			v = v[1 : len(v)-1]
		}
		if _, dup := env[m[1]]; dup {
			t.Fatalf("decision job sets %s twice", m[1])
		}
		env[m[1]] = v
	}
	if len(env) != 4 {
		t.Fatalf("decision job env %v, want GATE, WORKFLOW, GATE_JOBS and RECORD_JOB", env)
	}
	return env
}

// AFP-V0-670 to AFP-V0-672 (V1-2987): each decision job runs only for a merge-queue entry or main
// push, reads listings and one log with actions: read, and decides through the shared script,
// whose error means FULL. Every step of every gated job, and nothing else, waits on its REUSE
// decision; the gated jobs keep their ci:batched condition and still conclude success when skipped.
func TestAFPV0670GatesWireExactTreeReuse(t *testing.T) {
	for _, g := range gateReuses {
		w := readWorkflow(t, g.workflow)
		d := jobBlock(t, w, g.decide)
		for _, line := range []string{
			"    if: github.event_name == 'merge_group' || github.event_name == 'push'",
			"      actions: read",
			"      contents: read",
			"      mode: ${{ steps.find.outputs.mode }}",
			"          persist-credentials: false",
			"          fetch-depth: 2",
			"        id: find",
			"        continue-on-error: true",
			"          GH_TOKEN: ${{ github.token }}",
			"        run: bash .github/gate-reuse/find.sh",
		} {
			wantLine(t, g.decide, d, line)
		}
		env := decisionEnv(t, d)
		if env["GATE"] != g.label || env["WORKFLOW"] != g.workflow || env["RECORD_JOB"] != g.record {
			t.Errorf("%s env %v, want gate %s, workflow %s, record job %s", g.decide, env, g.label, g.workflow, g.record)
		}
		var names []string
		if err := json.Unmarshal([]byte(env["GATE_JOBS"]), &names); err != nil || len(names) == 0 {
			t.Fatalf("%s GATE_JOBS %q is not a JSON name list: %v", g.decide, env["GATE_JOBS"], err)
		}
		// Every gated job (each matrix leg by its listed name) is required to have passed.
		var want []string
		for _, job := range g.gated {
			if job == "cem-release-gate" {
				want = append(want, "cem-release-gate (ubuntu-24.04)", "cem-release-gate (macos-15)")
				continue
			}
			want = append(want, job)
		}
		if strings.Join(names, "|") != strings.Join(want, "|") {
			t.Errorf("%s GATE_JOBS %v, want %v", g.decide, names, want)
		}
		cond := "        if: needs." + g.decide + ".outputs.mode != 'REUSE'"
		always := "        if: ${{ always() && needs." + g.decide + ".outputs.mode != 'REUSE' }}"
		for _, job := range g.gated {
			b := jobBlock(t, w, job)
			if !regexp.MustCompile(`(?m)^    needs: (` + regexp.QuoteMeta(g.decide) + `|\[` + regexp.QuoteMeta(g.decide) + `, [a-z-]+\])$`).MatchString(b) {
				t.Errorf("%s must need %s", job, g.decide)
			}
			if !regexp.MustCompile(`(?m)^    if: \$\{\{ !cancelled\(\)( && .*)? \}\}$`).MatchString(b) {
				t.Errorf("%s must still run when %s is skipped (a pull request)", job, g.decide)
			}
			steps := strings.Count(b, "\n      - ")
			if n := strings.Count(b, "\n"+cond+"\n") + strings.Count(b, "\n"+always+"\n"); n != steps || steps == 0 {
				t.Errorf("%s: %d of %d steps wait on the REUSE decision", job, n, steps)
			}
		}
		for _, job := range []string{g.decide, g.record} {
			if strings.Contains(jobBlock(t, w, job), ".outputs.mode != 'REUSE'") {
				t.Errorf("%s must not be skipped by its own decision", job)
			}
		}
	}
	// AFP-V0-671: the host package version gate reads committed history; the record names the
	// tested merge's first parent and the decision demands it, so equal history is required too.
	b, err := os.ReadFile(gateReuseScript)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`test "$(printf '%s\n' "$line" | sed -E 's/.* base=([0-9a-f]{40}) .*/\1/')" = "$base" || continue`,
		`if ! git diff --quiet "$base" "$GITHUB_SHA" -- .github; then`,
		`test "$GITHUB_REF" = refs/heads/main || full 'the push is not to main'`,
	} {
		if strings.Count(string(b), s) != 1 {
			t.Errorf("the decision script must hold %q once", s)
		}
	}
	for _, banned := range []string{"/artifacts", "download", "eval ", "source "} {
		if strings.Contains(string(b), banned) {
			t.Errorf("the decision script must not use %q", banned)
		}
	}
}

// AFP-V0-670: each record job is the only evidence its decision accepts. It runs in a pull-request
// run only after every gated job passed there, holds no token, and runs no repository code.
func TestAFPV0670RecordJobsRunNoRepositoryCode(t *testing.T) {
	for _, g := range gateReuses {
		w := readWorkflow(t, g.workflow)
		rec := jobBlock(t, w, g.record)
		var needs, results []string
		for _, job := range g.gated {
			needs = append(needs, job)
			results = append(results, "needs."+job+".result == 'success'")
		}
		needLine := "    needs: " + needs[0]
		if len(needs) > 1 {
			needLine = "    needs: [" + strings.Join(needs, ", ") + "]"
		}
		for _, line := range []string{
			needLine,
			"    if: ${{ !cancelled() && github.event_name == 'pull_request' && " + strings.Join(results, " && ") + " }}",
			"          persist-credentials: false",
			"          fetch-depth: 2",
			"          PR_HEAD: ${{ github.event.pull_request.head.sha }}",
		} {
			wantLine(t, g.record, rec, line)
		}
		if n := strings.Count(rec, "\n      - "); n != 2 || !strings.Contains(rec, "\n      - uses: actions/checkout@") || strings.Count(rec, "uses:") != 1 {
			t.Errorf("%s must be exactly a checkout and one run step: %d steps", g.record, n)
		}
		for _, banned := range []string{"github.token", "secrets.", "GH_TOKEN", "permissions:", "services:", "container:", "shell:", "working-directory:", "continue-on-error"} {
			if strings.Contains(rec, banned) {
				t.Errorf("%s must not use %q", g.record, banned)
			}
		}
		allowed := regexp.MustCompile(`^(set -eu|test "\$\(git rev-parse [^;&|<>]*\)" = "\$(GITHUB_SHA|PR_HEAD)"|base=\$\(git rev-parse --verify -q "\$GITHUB_SHA\^1"\)|tree=\$\(git rev-parse --verify -q "\$GITHUB_SHA\^\{tree\}"\)|printf 'corvint-ci-gate-tree/0 gate=` + regexp.QuoteMeta(g.label) + ` tree=%s base=%s head=%s target=%s\\n' "\$tree" "\$base" "\$PR_HEAD" "\$GITHUB_SHA")$`)
		script := stepScript(t, rec, "record")
		lines := strings.Split(strings.TrimSpace(script), "\n")
		for _, line := range lines {
			if !allowed.MatchString(line) {
				t.Errorf("%s may run only builtins, git and printf: %q", g.record, line)
			}
		}
		if len(lines) != 6 {
			t.Errorf("%s script has %d lines, want 6", g.record, len(lines))
		}
	}
}

// AFP-V0-670 fail-closed: the shared decision script, run with each gate's own workflow inputs
// against a stub curl and a real two-parent merge. A merge-queue entry decides REUSE only for a
// successful pull-request run of its second parent whose latest gated and record jobs all passed
// on that head, with exactly one record line naming this tree, first parent and head; a main push
// decides REUSE only for a successful merge-queue run of this exact commit whose latest gated jobs
// all passed on it. A commit that changes .github, a single-parent commit, a pull request, a push
// to another branch, and every other listing, log or unreadable answer leave mode empty.
func TestAFPV0670ReuseDecidesOnlyForAPassedRunOfThisTree(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the decision and record steps run under bash with a #!/bin/sh curl stub")
	for _, tool := range []string{"bash", "jq", "git", "grep", "sed", "tr", "head"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the gate reuse step needs %s on PATH: %v", tool, err)
		}
	}
	findScript, err := filepath.Abs(gateReuseScript)
	if err != nil {
		t.Fatal(err)
	}
	const repo = "beamfall/corvint"
	const runID, jobID = 39472511845, 112233445566
	work := t.TempDir()
	gitEnv := []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test"}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		cmd.Dir, cmd.Env = work, append(append([]string{}, gitEnv...), "PATH="+os.Getenv("PATH"), "HOME="+work)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(work, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// main: base; pr changes f; main moves; the queue merge of pr is the tested commit. A second
	// pull request changes .github; its queue merge must never reuse.
	git("init", "-q", "-b", "main")
	write(".github/workflows/ci.yml", "on: push\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "pr")
	write("f", "x\n")
	git("add", "f")
	git("commit", "-q", "-m", "pr")
	head := git("rev-parse", "HEAD")
	git("checkout", "-q", "-b", "prg", "main")
	write(".github/workflows/ci.yml", "on: pull_request\n")
	git("add", ".")
	git("commit", "-q", "-m", "prg")
	headG := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	git("commit", "-q", "--allow-empty", "-m", "main moved")
	base := git("rev-parse", "HEAD")
	git("merge", "-q", "--no-ff", "-m", "queue", "pr")
	merge, tree := git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")
	git("checkout", "-q", "-b", "q2", base)
	git("merge", "-q", "--no-ff", "-m", "queue .github", "prg")
	mergeG := git("rev-parse", "HEAD")
	other := func(c string) string { return strings.Repeat(c, 40) }
	ts := "2026-10-28T10:00:00.1234567Z "

	for _, g := range gateReuses {
		w := readWorkflow(t, g.workflow)
		env := decisionEnv(t, jobBlock(t, w, g.decide))
		var gated []string
		if err := json.Unmarshal([]byte(env["GATE_JOBS"]), &gated); err != nil {
			t.Fatal(err)
		}
		recordScript := filepath.Join(t.TempDir(), "record.sh")
		if err := os.WriteFile(recordScript, []byte(stepScript(t, jobBlock(t, w, g.record), "record")), 0o600); err != nil {
			t.Fatal(err)
		}
		record := func(target, prHead string) string {
			t.Helper()
			git("checkout", "-q", target)
			cmd := exec.Command("bash", "--noprofile", "--norc", "-e", recordScript)
			cmd.Dir = work
			cmd.Env = append(append([]string{}, gitEnv...), "PATH="+os.Getenv("PATH"), "HOME="+work, "GITHUB_SHA="+target, "PR_HEAD="+prHead)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("%s record script: %v", g.record, err)
			}
			return strings.TrimSuffix(string(out), "\n")
		}
		line := record(merge, head)
		if line != "corvint-ci-gate-tree/0 gate="+g.label+" tree="+tree+" base="+base+" head="+head+" target="+merge {
			t.Fatalf("%s printed %q", g.record, line)
		}
		lineG := record(mergeG, headG)
		logOf := func(lines ...string) string {
			var b strings.Builder
			b.WriteString(ts + "##[group]Run set -eu\r\n" + ts + "printf 'corvint-ci-gate-tree/0 gate=" + g.label + " tree=%s base=%s head=%s target=%s\\n'\r\n" + ts + "##[endgroup]\r\n")
			for _, l := range lines {
				b.WriteString(ts + l + "\r\n")
			}
			return b.String()
		}

		// state is one fake GitHub answer set: the candidate run, its jobs and the record log.
		type state struct {
			run  map[string]any
			jobs []map[string]any
			log  string
		}
		decide := func(event, target, listed string, st state, extra ...string) (string, string) {
			t.Helper()
			git("checkout", "-q", target)
			dir := t.TempDir()
			writeJSON(t, filepath.Join(dir, "runs.json"), map[string]any{"workflow_runs": []any{st.run}})
			if st.jobs != nil {
				var jobs []any
				for _, j := range st.jobs {
					jobs = append(jobs, j)
				}
				writeJSON(t, filepath.Join(dir, "jobs.json"), map[string]any{"jobs": jobs})
			}
			if st.log != "unreadable" {
				if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte(st.log), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			bin := filepath.Join(dir, "bin")
			stub := `out= url= auth=
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift ;;
    -H) case "$2" in Authorization:*) auth=1 ;; esac; shift ;;
    -*) ;;
    *) url=$1 ;;
  esac
  shift
done
[ -n "$auth" ] && [ -n "${STUB_DENY_AUTH:-}" ] && exit 22
case "$url" in
  "$STUB_RUNS") f=runs.json ;;
  "$STUB_API/runs/$STUB_RUN/jobs?filter=latest&per_page=100") f=jobs.json ;;
  "$STUB_API/jobs/$STUB_JOB/logs") f=log.txt ;;
  *) exit 22 ;;
esac
[ -f "$STUB_DIR/$f" ] || exit 22
cat "$STUB_DIR/$f" > "$out"
`
			writeExecutable(t, filepath.Join(bin, "curl"), stub)
			api := "https://api.github.test/repos/" + repo + "/actions"
			runsEvent := "pull_request"
			if event == "push" {
				runsEvent = "merge_group"
			}
			output, summary := filepath.Join(dir, "output"), filepath.Join(dir, "summary")
			cmd := exec.Command("bash", "--noprofile", "--norc", findScript)
			cmd.Dir = work
			cmd.Env = append(append([]string{}, gitEnv...),
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "HOME="+dir,
				"GITHUB_API_URL=https://api.github.test", "GITHUB_REPOSITORY="+repo, "GITHUB_SHA="+target,
				"GITHUB_EVENT_NAME="+event, "GITHUB_REF=refs/heads/main",
				"RUNNER_TEMP="+dir, "GITHUB_OUTPUT="+output, "GITHUB_STEP_SUMMARY="+summary, "GH_TOKEN=t",
				"GATE="+env["GATE"], "WORKFLOW="+env["WORKFLOW"], "GATE_JOBS="+env["GATE_JOBS"], "RECORD_JOB="+env["RECORD_JOB"],
				"STUB_DIR="+dir, "STUB_API="+api, "STUB_RUN="+strconv.Itoa(runID), "STUB_JOB="+strconv.Itoa(jobID),
				"STUB_RUNS="+api+"/workflows/"+env["WORKFLOW"]+"/runs?head_sha="+listed+"&event="+runsEvent+"&status=success&per_page=20")
			cmd.Env = append(cmd.Env, extra...)
			_ = cmd.Run() // a nonzero exit is continue-on-error in the workflow; only the output binds
			o, _ := os.ReadFile(output)
			s, _ := os.ReadFile(summary)
			return string(o), string(s)
		}

		// merge_group: a passed pull-request run of the second parent.
		prState := func(sha, log string) state {
			st := state{run: map[string]any{"id": runID, "head_sha": sha, "event": "pull_request", "conclusion": "success",
				"path": ".github/workflows/" + g.workflow, "repository": map[string]any{"full_name": repo}}, log: log}
			st.jobs = append(st.jobs, map[string]any{"id": 7, "run_id": runID, "name": "unrelated", "conclusion": "failure", "head_sha": sha})
			for i, name := range gated {
				st.jobs = append(st.jobs, map[string]any{"id": 10 + i, "run_id": runID, "name": name, "conclusion": "success", "head_sha": sha})
			}
			st.jobs = append(st.jobs, map[string]any{"id": jobID, "run_id": runID, "name": g.record, "conclusion": "success", "head_sha": sha})
			return st
		}
		out, summary := decide("merge_group", merge, head, prState(head, logOf(line)))
		if out != "mode=REUSE\n" || !strings.Contains(summary, `{"profile":"corvint-ci-gate-reuse/0","gate":"`+g.label+`","mode":"REUSE","event":"merge_group","tree":"`+tree+`","base":"`+base+`","head":"`+head+`","target":"`+merge+`","runId":`+strconv.Itoa(runID)+`,"recordJobId":`+strconv.Itoa(jobID)+`}`) {
			t.Fatalf("%s: a passed pull-request run of this tree: output %q summary %q, want REUSE", g.label, out, summary)
		}
		if out, _ := decide("merge_group", merge, head, prState(head, logOf(line)), "STUB_DENY_AUTH=1"); out != "mode=REUSE\n" {
			t.Errorf("%s: a refused token must retry the public listing anonymously: output %q", g.label, out)
		}
		if out, summary := decide("merge_group", mergeG, headG, prState(headG, logOf(lineG))); out != "" || !strings.Contains(summary, "changes .github; the gate runs in full") {
			t.Errorf("%s: a queue commit that changes .github: output %q summary %q, want FULL", g.label, out, summary)
		}
		if out, _ := decide("merge_group", head, head, prState(head, logOf(line))); out != "" {
			t.Errorf("%s: a single-parent commit: output %q, want no mode", g.label, out)
		}
		if out, _ := decide("pull_request", merge, head, prState(head, logOf(line))); out != "" {
			t.Errorf("%s: a pull request never reuses: output %q", g.label, out)
		}
		job := func(st state, name string) map[string]any {
			for _, j := range st.jobs {
				if j["name"] == name {
					return j
				}
			}
			t.Fatalf("no job %s", name)
			return nil
		}
		drop := func(st *state, name string) {
			var keep []map[string]any
			for _, j := range st.jobs {
				if j["name"] != name {
					keep = append(keep, j)
				}
			}
			st.jobs = keep
		}
		last := gated[len(gated)-1]
		for name, edit := range map[string]func(st *state){
			"other head":                func(st *state) { st.run["head_sha"] = other("0") },
			"merge_group run":           func(st *state) { st.run["event"] = "merge_group" },
			"failed run":                func(st *state) { st.run["conclusion"] = "failure" },
			"other workflow":            func(st *state) { st.run["path"] = ".github/workflows/other.yml" },
			"other repository":          func(st *state) { st.run["repository"] = map[string]any{"full_name": "fork/corvint"} },
			"string run id":             func(st *state) { st.run["id"] = strconv.Itoa(runID) },
			"first gate job failed":     func(st *state) { job(*st, gated[0])["conclusion"] = "failure" },
			"last gate job skipped":     func(st *state) { job(*st, last)["conclusion"] = "skipped" },
			"last gate job missing":     func(st *state) { drop(st, last) },
			"gate job duplicated":       func(st *state) { st.jobs = append(st.jobs, job(*st, gated[0])) },
			"gate job on other head":    func(st *state) { job(*st, gated[0])["head_sha"] = other("4") },
			"record job failed":         func(st *state) { job(*st, g.record)["conclusion"] = "failure" },
			"record job missing":        func(st *state) { drop(st, g.record) },
			"record job duplicated":     func(st *state) { st.jobs = append(st.jobs, job(*st, g.record)) },
			"record job on other head":  func(st *state) { job(*st, g.record)["head_sha"] = other("1") },
			"record job of another run": func(st *state) { job(*st, g.record)["run_id"] = 1 },
			"record job string id":      func(st *state) { job(*st, g.record)["id"] = strconv.Itoa(jobID) },
			"jobs unreadable":           func(st *state) { st.jobs = nil },
			"log unreadable":            func(st *state) { st.log = "unreadable" },
			"log without a record":      func(st *state) { st.log = logOf() },
			"log with two records":      func(st *state) { st.log = logOf(line, line) },
			"record of another gate": func(st *state) {
				st.log = logOf(strings.Replace(line, " gate="+g.label+" ", " gate=other-gate ", 1))
			},
			"record of another tree": func(st *state) {
				st.log = logOf(strings.Replace(line, " tree="+tree+" ", " tree="+other("2")+" ", 1))
			},
			"record of another base": func(st *state) {
				st.log = logOf(strings.Replace(line, " base="+base+" ", " base="+other("5")+" ", 1))
			},
			"record of another head": func(st *state) {
				st.log = logOf(strings.Replace(line, " head="+head+" ", " head="+other("3")+" ", 1))
			},
			"record with trailing text": func(st *state) { st.log = logOf(line + " x") },
			"record inside other text":  func(st *state) { st.log = logOf("echo " + line) },
		} {
			st := prState(head, logOf(line))
			edit(&st)
			if out, summary := decide("merge_group", merge, head, st); out != "" || !strings.Contains(summary, "the gate runs in full") {
				t.Errorf("%s merge_group %s: output %q summary %q, want FULL", g.label, name, out, summary)
			}
		}

		// push: a passed merge-queue run of this exact commit.
		queueState := func(sha string) state {
			st := state{run: map[string]any{"id": runID, "head_sha": sha, "event": "merge_group", "conclusion": "success",
				"path": ".github/workflows/" + g.workflow, "head_branch": "gh-readonly-queue/main/pr-1-" + base,
				"head_repository": map[string]any{"full_name": repo}}}
			for i, name := range gated {
				st.jobs = append(st.jobs, map[string]any{"id": 10 + i, "run_id": runID, "name": name, "conclusion": "success", "head_sha": sha})
			}
			return st
		}
		out, summary = decide("push", merge, merge, queueState(merge))
		if out != "mode=REUSE\n" || !strings.Contains(summary, `"gate":"`+g.label+`","mode":"REUSE","event":"push","tree":"`+tree+`"`) || !strings.Contains(summary, `"runId":`+strconv.Itoa(runID)+"}") {
			t.Fatalf("%s: a passed merge-queue run of this commit: output %q summary %q, want REUSE", g.label, out, summary)
		}
		if out, _ := decide("push", merge, merge, queueState(merge), "GITHUB_REF=refs/heads/other"); out != "" {
			t.Errorf("%s: a push to another branch: output %q, want no mode", g.label, out)
		}
		if out, _ := decide("push", mergeG, mergeG, queueState(mergeG)); out != "" {
			t.Errorf("%s: a main push that changes .github: output %q, want no mode", g.label, out)
		}
		for name, edit := range map[string]func(st *state){
			"other commit":          func(st *state) { st.run["head_sha"] = other("6") },
			"pull_request run":      func(st *state) { st.run["event"] = "pull_request" },
			"failed run":            func(st *state) { st.run["conclusion"] = "failure" },
			"other workflow":        func(st *state) { st.run["path"] = ".github/workflows/other.yml" },
			"other branch":          func(st *state) { st.run["head_branch"] = "main" },
			"no branch":             func(st *state) { delete(st.run, "head_branch") },
			"fork repository":       func(st *state) { st.run["head_repository"] = map[string]any{"full_name": "fork/corvint"} },
			"string run id":         func(st *state) { st.run["id"] = strconv.Itoa(runID) },
			"first gate job failed": func(st *state) { job(*st, gated[0])["conclusion"] = "failure" },
			"last gate job missing": func(st *state) { drop(st, last) },
			"gate job duplicated":   func(st *state) { st.jobs = append(st.jobs, job(*st, gated[0])) },
			"gate job other commit": func(st *state) { job(*st, last)["head_sha"] = other("7") },
			"gate job other run":    func(st *state) { job(*st, last)["run_id"] = 1 },
			"jobs unreadable":       func(st *state) { st.jobs = nil },
		} {
			st := queueState(merge)
			edit(&st)
			if out, summary := decide("push", merge, merge, st); out != "" || !strings.Contains(summary, "the gate runs in full") {
				t.Errorf("%s push %s: output %q summary %q, want FULL", g.label, name, out, summary)
			}
		}
		// A malformed job list never reuses.
		for _, bad := range []string{`[]`, `"doc-gates"`, `["a","a"]`, `[1]`} {
			if out, _ := decide("push", merge, merge, queueState(merge), "GATE_JOBS="+bad); out != "" {
				t.Errorf("%s: GATE_JOBS %s: output %q, want no mode", g.label, bad, out)
			}
		}
	}
}
