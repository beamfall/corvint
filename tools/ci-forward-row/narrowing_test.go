// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// narrowingRecord is one shard's corvint-pr-narrowing/0 record (narrow.jq) of run RUN.
func narrowingRecord(shard int, mode, run string, executed, skipped []string) doc {
	n, _ := strconv.Atoi(run)
	if executed == nil {
		executed = []string{}
	}
	if skipped == nil {
		skipped = []string{}
	}
	return doc{"profile": "corvint-pr-narrowing/0", "mode": mode, "reasons": []string{}, "shard": shard, "shards": shards,
		"runId": n, "executed": executed, "skipped": skipped}
}

func TestAFPV0099NarrowedRows(t *testing.T) {
	// p is a package the real shadow plan of shard 0 selected; the queue fails it once and
	// passes it on its rerun.
	failQueue := func(t *testing.T) (*fixture, string) {
		f := queueFixture(t)
		p := strs(f.sel[0]["selected"])[0]
		if _, ok := sub(f.out[0], "outcomes")[p]; !ok {
			t.Fatalf("shard 0 outcomes lack %s", p)
		}
		fail(f.out[0], p, "fail", "RAN", "pass")
		return f, p
	}
	t.Run("a queue failure of a selected package is not a miss", func(t *testing.T) {
		f, _ := failQueue(t)
		row, _ := f.classify(t, nil)
		if row["class"] != "CLEAN" || num(sub(row, "counts")["selectedFailedInQueue"]) != 1 || sub(row, "selection")["narrowed"] != nil {
			t.Fatalf("row = %v", row)
		}
	})
	t.Run("a queue failure of a package the pull request narrowed out is a MISS, even when its rerun passes", func(t *testing.T) {
		f, p := failQueue(t)
		f.nar = map[int]doc{0: narrowingRecord(0, "NARROWED", prRun, nil, []string{p}), 1: narrowingRecord(1, "FULL", prRun, nil, nil)}
		row, _ := f.classify(t, nil)
		if row["class"] != "MISS" || strs(row["missed"])[0] != p || len(row["flaky"].([]any)) != 0 {
			t.Fatalf("row = %v", row)
		}
		n := sub(sub(row, "selection"), "narrowed")
		if fmt.Sprint(n["shards"]) != "[0]" || num(n["narrowedOut"]) != 1 {
			t.Fatalf("selection.narrowed = %v", n)
		}
	})
	t.Run("FULL records narrow nothing", func(t *testing.T) {
		f, p := failQueue(t)
		f.nar = map[int]doc{0: narrowingRecord(0, "FULL", prRun, []string{p}, nil)}
		if row, _ := f.classify(t, nil); row["class"] != "CLEAN" {
			t.Fatalf("row = %v", row)
		}
	})
	t.Run("a record of another run, a malformed record or an unreadable one leaves the queue row INCOMPLETE", func(t *testing.T) {
		f, p := failQueue(t)
		f.nar = map[int]doc{0: narrowingRecord(0, "NARROWED", "1", nil, []string{p})}
		row, _ := f.classify(t, nil)
		wantIncomplete(t, row, "narrowing shard 0 is run 1, want "+prRun)
		f, _ = failQueue(t)
		bad := narrowingRecord(0, "NARROWED", prRun, nil, nil)
		delete(bad, "skipped")
		f.nar = map[int]doc{0: bad}
		row, _ = f.classify(t, nil)
		wantIncomplete(t, row, "fail the corvint-pr-narrowing/0 shape")
		f, _ = failQueue(t)
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "ci-narrowing-0", "narrowing.json"), []byte("{"))
		row, _ = f.classify(t, map[string]string{"ROW_NARROWING_DIR": dir})
		wantIncomplete(t, row, "narrowing record ci-narrowing-0 is unreadable")
	})
	t.Run("a pull-request row of a narrowed run is NARROWED", func(t *testing.T) {
		pr := map[string]string{"ROW_KIND": "pr", "ROW_SHA": "", "ROW_RUN": prRun, "ROW_OUTCOMES_RUN": prRun}
		f := prFixture(t)
		if row, _ := f.classify(t, pr); row["class"] != "CLEAN" {
			t.Fatalf("unnarrowed pull-request row = %v", row["class"])
		}
		f.nar = map[int]doc{3: narrowingRecord(3, "NARROWED", prRun, nil, []string{"github.com/Beamfall/corvint/x"})}
		if row, _ := f.classify(t, pr); row["class"] != "NARROWED" {
			t.Fatalf("narrowed pull-request row = %v", row["class"])
		}
		// A record that cannot be read could have narrowed: never a shadow row.
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "ci-narrowing-3", "narrowing.json"), []byte("not json"))
		if row, _ := prFixture(t).classify(t, map[string]string{"ROW_KIND": "pr", "ROW_SHA": "", "ROW_RUN": prRun,
			"ROW_OUTCOMES_RUN": prRun, "ROW_NARROWING_DIR": dir}); row["class"] != "NARROWED" {
			t.Fatalf("pull-request row with an unreadable record = %v", row["class"])
		}
	})
}

// The collector downloads the selection run's records, so the merge-queue row of a narrowed
// pull request names the narrowed-out failure a MISS.
func TestAFPV0099CollectorFetchesNarrowing(t *testing.T) {
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
	writeFile(t, filepath.Join(dir, "names-"+queueRun), lines(move(outDir, queueRun)...))
	prNames := append(move(selDir, prRun), move(filepath.Join(dir, "src", "narrowing"), prRun)...)
	writeFile(t, filepath.Join(dir, "names-"+prRun), lines(prNames...))
	output, summary := filepath.Join(dir, "output"), filepath.Join(dir, "summary")
	writeFile(t, output, nil)
	cmd := exec.Command("sh", script(t, "collect.sh"))
	cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
		"GH_TOKEN=x", "REPO=beamfall/corvint", "RUN_ID=" + queueRun, "EVENT=merge_group", "HEAD_SHA=" + queueSHA,
		"HEAD_BRANCH=gh-readonly-queue/main/pr-868-" + mainBase, "PR_NUMBER=", "PR_BASE_SHA=" + mainBase,
		"FORWARD_SHARDS=8", "FORWARD_EPOCH=1", "RUNNER_TEMP=" + filepath.Join(dir, "temp"),
		"GITHUB_OUTPUT=" + output, "GITHUB_STEP_SUMMARY=" + summary}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("collect.sh: %v\n%s", err, out)
	}
	o, _ := os.ReadFile(output)
	s, _ := os.ReadFile(summary)
	// AFP-V0-483: the MISS row also names its cost, where S carries the miss (flag 1).
	got := strings.Split(strings.TrimSpace(string(o)), "\n")
	if len(got) != 2 || got[0] != "name=ci-forward-MISS-pr868-"+queueSHA+"-e1-f1" ||
		!regexp.MustCompile(`^cost=ci-forward-cost-pr868-`+queueSHA+`-e1-MISS-f1-s[0-9]{4}-bna-cna-dna-m1nnn$`).MatchString(got[1]) {
		t.Fatalf("queue row = %q\n%s", got, s)
	}
}

func barNames(class string, pr int, epoch, f string) string {
	return fmt.Sprintf("ci-forward-%s-pr%d-%040x-e%s-f%s", class, pr, pr, epoch, f)
}

// metNames are 22 complete rows of epoch 1, five of them failing, and one row of epoch 0, so
// the listing reaches the epoch's start: the bar is MET.
func metNames() []string {
	var names []string
	for pr := 1; pr <= 22; pr++ {
		f := "0"
		if pr <= 5 {
			f = "1"
		}
		names = append(names, barNames("CLEAN", pr, "1", f))
	}
	return append(names, barNames("CLEAN", 90, "0", "0"))
}

func TestAFPV0099BarExcludesNarrowedRows(t *testing.T) {
	bar := func(t *testing.T, names []string, args ...string) string {
		cmd := exec.Command("jq", append([]string{"-R", "-s", "-r", "--arg", "epoch", "1", "-f", script(t, "bar.jq")}, args...)...)
		cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("bar.jq: %v", err)
		}
		return string(out)
	}
	// Newer NARROWED rows, including one of a pull request and commit the bar already counts,
	// neither replace a counted row nor take a place in the kill-switch window.
	var narrowed []string
	for pr := 100; pr < 125; pr++ {
		narrowed = append(narrowed, barNames("NARROWED", pr, "1", "0"))
	}
	names := append(append(narrowed, barNames("NARROWED", 3, "1", "1")), metNames()...)
	md := bar(t, names)
	for _, want := range []string{"22 complete, 5 complete with failing packages, 0 FLAKY, 0 MISS, 0 INCOMPLETE, 0 INFRA.",
		"Latest 20 row(s) of the epoch (AFP-V0-030 window): 20 complete", "(AFP-V0-099): 26 NARROWED row(s)", ": MET (22/20 complete, 5/5 failing)."} {
		if !strings.Contains(md, want) {
			t.Fatalf("summary lacks %q:\n%s", want, md)
		}
	}
	var v doc
	if err := jsonUnmarshal(bar(t, names, "--arg", "format", "json"), &v); err != nil {
		t.Fatal(err)
	}
	if v["status"] != "MET" || v["killSwitch"] != "clear" || num(v["narrowed"]) != 26 || num(sub(v, "epochRows")["complete"]) != 22 ||
		num(sub(v, "window")["rows"]) != 20 || v["covered"] != true {
		t.Fatalf("json = %v", v)
	}
	// Without the NARROWED exclusion the 26 rows alone would fill the window and the bar.
	if err := jsonUnmarshal(bar(t, narrowed, "--arg", "format", "json"), &v); err != nil || v["status"] != "NOT MET" ||
		num(sub(v, "epochRows")["rows"]) != 0 || num(v["narrowed"]) != 25 {
		t.Fatalf("NARROWED rows alone = %v (%v)", v, err)
	}
}

// TestAFPV0463ReclassifiedRowsAreInfra pins AFP-V0-463: only a listed INCOMPLETE row counts as
// INFRA, every bar.jq caller passes the list, and every listed key is well formed.
func TestAFPV0463ReclassifiedRowsAreInfra(t *testing.T) {
	key := func(pr int) string { return fmt.Sprintf("pr%d-%040x", pr, pr) }
	list := filepath.Join(t.TempDir(), "infra-reclassified.txt")
	if err := os.WriteFile(list, []byte("# comment\n"+key(200)+" artifact=1\n"+key(201)+"\n"+key(202)+"\n"+key(1)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Newest first: three INCOMPLETE rows (two listed) and one listed MISS row, which stays MISS;
	// the listed CLEAN row pr1 of metNames stays complete (22 of metNames plus the MISS row).
	names := append([]string{barNames("INCOMPLETE", 200, "1", "0"), barNames("INCOMPLETE", 201, "1", "0"),
		barNames("INCOMPLETE", 203, "1", "0"), barNames("MISS", 202, "1", "0")}, metNames()...)
	verdict := func(args ...string) doc {
		cmd := exec.Command("jq", append([]string{"-R", "-s", "-r", "--arg", "epoch", "1", "--arg", "format", "json", "-f", script(t, "bar.jq")}, args...)...)
		cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("bar.jq: %v", err)
		}
		var v doc
		if err := jsonUnmarshal(string(out), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	without := verdict()
	if w := sub(without, "window"); num(w["incomplete"]) != 3 || num(w["miss"]) != 1 || without["killSwitch"] != "TRIPPED" {
		t.Fatalf("without the list = %v", without)
	}
	with := verdict("--rawfile", "reclassified", list)
	if w := sub(with, "window"); num(w["incomplete"]) != 1 || num(w["miss"]) != 1 || num(with["windowInfra"]) != 2 ||
		num(sub(with, "epochRows")["infra"]) != 2 || num(sub(with, "epochRows")["complete"]) != 23 || with["killSwitch"] != "clear" {
		t.Fatalf("with the list = %v", with)
	}
	for _, caller := range []string{"bar.sh", "verdict.sh", "accept.sh"} {
		b, err := os.ReadFile(script(t, caller))
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(b), `-f "$here/bar.jq"`); n == 0 || n != strings.Count(string(b), `--rawfile reclassified "$here/infra-reclassified.txt" -f "$here/bar.jq"`) {
			t.Fatalf("%s runs bar.jq without the AFP-V0-463 list", caller)
		}
	}
	b, err := os.ReadFile(script(t, "infra-reclassified.txt"))
	if err != nil {
		t.Fatal(err)
	}
	keyLine := regexp.MustCompile(`^pr[0-9]+-[0-9a-f]{40} artifact=[0-9]+$`)
	keys := 0
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if !keyLine.MatchString(line) {
			t.Fatalf("malformed AFP-V0-463 key line %q", line)
		}
		keys++
	}
	if keys != 92 {
		t.Fatalf("AFP-V0-463 lists %d keys, want the 92 reviewed head-null rows", keys)
	}
}

func TestAFPV0097VerdictWrittenOnMain(t *testing.T) {
	verdict := func(t *testing.T, ref string, names []string) (doc, error) {
		dir := t.TempDir()
		gh := fakeGH(t, dir)
		var all []any
		start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		for i, n := range names {
			all = append(all, doc{"name": n, "expired": false, "created_at": start.Add(time.Duration(len(names)-i) * time.Hour).Format(time.RFC3339),
				"workflow_run": doc{"head_branch": "main", "repository_id": 1, "head_repository_id": 1}})
		}
		writeDoc(t, filepath.Join(dir, "all-artifacts.json"), doc{"artifacts": all})
		cmd := exec.Command("sh", script(t, "verdict.sh"))
		cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
			"GH_TOKEN=x", "REPO=beamfall/corvint", "FORWARD_EPOCH=1", "RUNNER_TEMP=" + dir, "GITHUB_REF=" + ref,
			"GITHUB_SHA=" + queueSHA, "GITHUB_RUN_ID=4242", "GITHUB_RUN_ATTEMPT=1",
			"GITHUB_WORKFLOW_REF=beamfall/corvint/.github/workflows/ci-forward-bar.yml@refs/heads/main",
			"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary")}
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("%v: %s", err, out)
		}
		return readDoc(t, filepath.Join(dir, "afp-shadow-bar", "verdict.json")), nil
	}
	if _, err := verdict(t, "refs/heads/claude/x", metNames()); err == nil {
		t.Fatal("verdict.sh wrote a verdict off main")
	}
	v, err := verdict(t, "refs/heads/main", append([]string{barNames("NARROWED", 200, "1", "0")}, metNames()...))
	if err != nil {
		t.Fatal(err)
	}
	// The oldest listed row of epoch 1 is pr22's, listed last before the epoch-0 row.
	if v["profile"] != "afp-shadow-bar-verdict/0" || v["status"] != "MET" || num(v["epoch"]) != 1 || v["mainSha"] != queueSHA ||
		num(v["runId"]) != 4242 || num(v["runAttempt"]) != 1 || v["repository"] != "beamfall/corvint" || v["killSwitch"] != "clear" ||
		num(v["narrowed"]) != 1 || num(sub(v, "counts")["complete"]) != 22 || v["epochStart"] != "2026-10-01T02:00:00Z" || v["exhausted"] != true {
		t.Fatalf("verdict = %v", v)
	}
	v, err = verdict(t, "refs/heads/main", append(metNames(), barNames("MISS", 300, "1", "1")))
	if err != nil {
		t.Fatal(err)
	}
	if v["status"] != "NOT MET" {
		t.Fatalf("verdict with a MISS in the epoch = %v", v["status"])
	}
}

func TestAFPV0097VerdictAcceptance(t *testing.T) {
	const run, repo, workflow = "4242", "beamfall/corvint", ".github/workflows/ci-forward-bar.yml"
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	type setup struct {
		artifacts []any    // the afp-shadow-bar-epoch listing
		run       doc      // the verdict run's metadata
		verdict   any      // verdict.json of the verdict run, or a raw string
		rows      []string // the newest forward-row names
		epoch     string
		pages     [][]any  // when set, the full artifact listing page by page (page N+1 onward is empty)
		killPages string   // KILL_PAGES, the page budget of the scan since the verdict
		fail      []string // AFP-V0-760: unreadable listing pages, as "SIZE-PAGE"
		reads     *int     // CIN-V0-004: when set, gets the number of artifact listing pages read
	}
	artifact := func(run string, age time.Duration) doc {
		id, _ := strconv.Atoi(run)
		return doc{"id": 1, "name": "afp-shadow-bar-epoch-1", "expired": false, "created_at": now.Add(-age).Format(time.RFC3339),
			"workflow_run": doc{"id": id, "head_branch": "main", "repository_id": 1, "head_repository_id": 1}}
	}
	good := func() setup {
		return setup{
			artifacts: []any{artifact(run, 24*time.Hour)},
			run: doc{"path": workflow + "@refs/heads/main", "event": "schedule", "head_branch": "main",
				"head_repository": doc{"full_name": repo}, "repository": doc{"full_name": repo}},
			verdict: doc{"profile": "afp-shadow-bar-verdict/0", "status": "MET", "epoch": 1, "killSwitch": "clear",
				"runId": 4242, "repository": repo, "mainSha": queueSHA},
			rows:  metNames()[:5],
			epoch: "1",
		}
	}
	accept := func(t *testing.T, s setup) (string, string) {
		dir := t.TempDir()
		gh := fakeGH(t, dir)
		writeDoc(t, filepath.Join(dir, "verdict-artifacts.json"), doc{"artifacts": s.artifacts})
		for _, a := range s.artifacts {
			id := fmt.Sprint(sub(a.(doc), "workflow_run")["id"])
			writeDoc(t, filepath.Join(dir, "run-"+id+".json"), s.run)
			path := filepath.Join(dir, "runs", id, "afp-shadow-bar-epoch-1", "verdict.json")
			if raw, ok := s.verdict.(string); ok {
				writeFile(t, path, []byte(raw))
			} else if s.verdict != nil {
				writeDoc(t, path, s.verdict)
			}
		}
		var all []any
		for _, n := range s.rows {
			all = append(all, doc{"name": n, "expired": false, "workflow_run": doc{"head_branch": "main", "repository_id": 1, "head_repository_id": 1}})
		}
		writeDoc(t, filepath.Join(dir, "all-artifacts.json"), doc{"artifacts": all})
		if s.pages != nil {
			writeDoc(t, filepath.Join(dir, "all-artifacts.json"), doc{"artifacts": []any{}})
			for i, page := range s.pages {
				writeDoc(t, filepath.Join(dir, fmt.Sprintf("all-artifacts-%d.json", i+1)), doc{"artifacts": page})
			}
		}
		for _, f := range s.fail {
			writeFile(t, filepath.Join(dir, "fail-"+f), nil)
		}
		output := filepath.Join(dir, "output")
		writeFile(t, output, nil)
		cmd := exec.Command("sh", script(t, "accept.sh"))
		cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
			"GH_TOKEN=x", "REPO=" + repo, "FORWARD_EPOCH=" + s.epoch, "RUNNER_TEMP=" + dir, "GITHUB_OUTPUT=" + output,
			"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary"), "VERDICT_WORKFLOW=" + workflow,
			"NOW=" + strconv.FormatInt(now.Unix(), 10)}
		if s.killPages != "" {
			cmd.Env = append(cmd.Env, "KILL_PAGES="+s.killPages)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("accept.sh: %v\n%s", err, out)
		}
		if s.reads != nil {
			r, _ := os.ReadFile(filepath.Join(dir, "requests"))
			*s.reads = len(regexp.MustCompile(`(?m)/actions/artifacts\?per_page=`).FindAllIndex(r, -1))
		}
		o, _ := os.ReadFile(output)
		verdict := regexp.MustCompile(`(?m)^verdict=(.*)$`).FindStringSubmatch(string(o))
		reason := regexp.MustCompile(`(?m)^reason=(.*)$`).FindStringSubmatch(string(o))
		if verdict == nil || reason == nil {
			t.Fatalf("accept.sh wrote no verdict:\n%s", o)
		}
		// AFP-V0-461: every verdict and its reason reach the job log as a line and a notice,
		// and the step summary as a line.
		line := "AFP-V0-097 narrowing verdict: " + verdict[1] + " (" + reason[1] + ")"
		summary, _ := os.ReadFile(filepath.Join(dir, "summary"))
		if !strings.Contains(string(out), line+"\n") || !strings.Contains(string(out), "::notice title=AFP-V0-097 narrowing verdict::"+verdict[1]+" ("+reason[1]+")\n") ||
			!strings.Contains(string(summary), line+"\n") {
			t.Fatalf("accept.sh did not print %q to the log, a notice and the summary:\nlog:\n%s\nsummary:\n%s", line, out, summary)
		}
		return verdict[1], reason[1]
	}
	// listed builds a listing page: the named main rows created `age` ago, then the verdict
	// artifact when `verdict` is set, then artifacts of another branch up to `size`.
	listed := func(names []string, age time.Duration, verdict bool, size int) []any {
		var page []any
		for _, n := range names {
			page = append(page, doc{"name": n, "expired": false, "created_at": now.Add(-age).Format(time.RFC3339),
				"workflow_run": doc{"head_branch": "main", "repository_id": 1, "head_repository_id": 1}})
		}
		if verdict {
			page = append(page, artifact(run, 24*time.Hour))
		}
		for i := len(page); i < size; i++ {
			page = append(page, doc{"name": fmt.Sprintf("other-%d", i), "expired": false, "created_at": now.Format(time.RFC3339),
				"workflow_run": doc{"head_branch": "claude/x", "repository_id": 1, "head_repository_id": 1}})
		}
		return page
	}
	epochRows := func(n int) []string {
		var names []string
		for pr := 1; pr <= n; pr++ {
			names = append(names, barNames("CLEAN", 1000+pr, "1", "0"))
		}
		return names
	}
	cases := []struct {
		name, want string
		edit       func(*setup)
	}{
		{"MET from a schedule run", "MET", func(*setup) {}},
		{"MET from a workflow_dispatch run", "MET", func(s *setup) { s.run["event"] = "workflow_dispatch" }},
		{"no verdict artifact", "no afp-shadow-bar-epoch-1 artifact exists", func(s *setup) { s.artifacts = []any{} }},
		// (a) the trusted workflow path on main of this repository.
		{"(a) a verdict artifact of another branch", "is not from main of this repository", func(s *setup) {
			sub(s.artifacts[0].(doc), "workflow_run")["head_branch"] = "claude/x"
		}},
		{"(a) a verdict artifact of a fork", "is not from main of this repository", func(s *setup) {
			sub(s.artifacts[0].(doc), "workflow_run")["head_repository_id"] = 2
		}},
		{"(a) a pull-request run", "is not a schedule or workflow_dispatch run", func(s *setup) { s.run["event"] = "pull_request" }},
		{"(a) a main push run", "is not a schedule or workflow_dispatch run", func(s *setup) { s.run["event"] = "push" }},
		{"(a) another workflow", "is not a schedule or workflow_dispatch run", func(s *setup) { s.run["path"] = ".github/workflows/ci.yml" }},
		{"(a) another repository", "is not a schedule or workflow_dispatch run", func(s *setup) {
			s.run["head_repository"] = doc{"full_name": "someone/corvint"}
		}},
		// (b) status MET with a clear kill switch.
		{"(b) NOT MET", "is NOT MET", func(s *setup) { s.verdict.(doc)["status"] = "NOT MET" }},
		{"(b) NOT ESTABLISHED", "is NOT ESTABLISHED", func(s *setup) { s.verdict.(doc)["status"] = "NOT ESTABLISHED" }},
		{"(b) a tripped kill switch", "tripped or unknown kill switch", func(s *setup) { s.verdict.(doc)["killSwitch"] = "TRIPPED" }},
		// (c) at most 7 days old.
		{"(c) eight days old", "not within 7 days", func(s *setup) { s.artifacts = []any{artifact(run, 8*24*time.Hour)} }},
		{"(c) created in the future", "not within 7 days", func(s *setup) { s.artifacts = []any{artifact(run, -time.Hour)} }},
		{"(c) expired", "has expired", func(s *setup) { s.artifacts[0].(doc)["expired"] = true }},
		// (d) the current epoch.
		{"(d) a verdict of another epoch", "names epoch 0, not 1", func(s *setup) { s.verdict.(doc)["epoch"] = 0 }},
		{"(d) the forward epoch moved on", "no afp-shadow-bar-epoch-2 artifact exists", func(s *setup) { s.epoch = "2" }},
		// The newest artifact by created_at decides, even when an older one said MET.
		{"a newer verdict that is not MET", "is NOT MET", func(s *setup) {
			newer := artifact("4343", time.Hour)
			s.artifacts = append(s.artifacts, newer)
			s.verdict.(doc)["status"] = "NOT MET"
			s.verdict.(doc)["runId"] = 4343
		}},
		{"an unparseable verdict", "unparseable", func(s *setup) { s.verdict = "{" }},
		{"a verdict naming another run", "unparseable or names another run", func(s *setup) { s.verdict.(doc)["runId"] = 1 }},
		{"a verdict that cannot be downloaded", "could not be downloaded", func(s *setup) { s.verdict = nil }},
		// AFP-V0-760: a truncated page of 100 is read again as pages of 25; an unreadable page of
		// 25 still names its limit and runs full CI.
		{"a truncated page of 100 read as pages of 25", "MET", func(s *setup) { s.fail = []string{"100-1"} }},
		{"a truncated page of 100 whose page of 25 is unreadable",
			"the newest forward rows are unreadable: artifact page 1 is unreadable: its page 1 of 25 failed after 3 tries (unexpected end of JSON input",
			func(s *setup) { s.fail = []string{"100-1", "25-1"} }},
		{"a MISS row since the verdict", "show a MISS", func(s *setup) { s.rows = append([]string{barNames("MISS", 400, "1", "1")}, s.rows...) }},
		// The scan since the verdict must prove its coverage when it stops at the page budget.
		{"the default page budget of 25 runs out with no main row", "do not reach the verdict", func(s *setup) {
			for i := 0; i < 25; i++ {
				s.pages = append(s.pages, listed(nil, 0, false, 100))
			}
		}},
		// 2026-10-28: the 20th epoch row sat on page 9, past the former default of 5 pages.
		{"MET when the 20 rows and the verdict only appear on page 9 with KILL_PAGES unset", "MET", func(s *setup) {
			for i := 0; i < 8; i++ {
				s.pages = append(s.pages, listed(nil, 0, false, 100))
			}
			s.pages = append(s.pages, listed(epochRows(25), time.Hour, true, 100))
		}},
		{"a MISS beyond the scanned pages", "do not reach the verdict", func(s *setup) {
			s.killPages = "1"
			s.pages = [][]any{listed(epochRows(100), time.Hour, false, 100),
				listed([]string{barNames("MISS", 400, "1", "1")}, 2*time.Hour, true, 2)}
		}},
		{"the scanned pages reach the verdict but hold under 20 rows", "fewer than 20 rows", func(s *setup) {
			s.killPages = "1"
			s.pages = [][]any{listed(epochRows(5), time.Hour, true, 100)}
		}},
		{"MET when the scanned pages reach the verdict and hold 20 rows", "MET", func(s *setup) {
			s.killPages = "1"
			s.pages = [][]any{listed(epochRows(25), time.Hour, true, 100)}
		}},
		{"a MISS within the scanned pages", "show a MISS", func(s *setup) {
			s.killPages = "1"
			s.pages = [][]any{listed(append([]string{barNames("MISS", 400, "1", "1")}, epochRows(25)...), time.Hour, true, 100)}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := good()
			c.edit(&s)
			verdict, reason := accept(t, s)
			if c.want == "MET" {
				if verdict != "MET" {
					t.Fatalf("verdict %s (%s), want MET", verdict, reason)
				}
				return
			}
			if verdict != "FULL" || !strings.Contains(reason, c.want) {
				t.Fatalf("verdict %s (%s), want FULL with %q", verdict, reason, c.want)
			}
		})
	}
	// CIN-V0-004 (V1-3091): a verdict that names its listing time and window rows is re-checked
	// over the rows since that listing only. The replayed listing is 30 pages of 100, past the
	// old 25-page budget: the verdict artifact sits on page 30, two new rows of the epoch on
	// page 1, artifacts of other branches created four hours ago (before the verdict's listing)
	// on page 2, and the window's 20 rows on page 3, which the incremental listing never reads.
	window := epochRows(20)
	incremental := func(s *setup) {
		s.verdict.(doc)["listedAt"] = now.Add(-2 * time.Hour).Format(time.RFC3339)
		s.verdict.(doc)["windowNames"] = window
	}
	replay := func(fresh ...string) func(*setup) {
		return func(s *setup) {
			s.pages = [][]any{listed(append(fresh, barNames("CLEAN", 2001, "1", "0"), barNames("CLEAN", 2002, "1", "1")), time.Hour, false, 100),
				listed(nil, 0, false, 100), listed(window, 5*time.Hour, false, 100)}
			for _, a := range s.pages[1] {
				a.(doc)["created_at"] = now.Add(-4 * time.Hour).Format(time.RFC3339)
			}
			for len(s.pages) < 29 {
				s.pages = append(s.pages, listed(nil, 0, false, 100))
			}
			s.pages = append(s.pages, listed(nil, 0, true, 100))
		}
	}
	big := []struct {
		name, want string
		reads      int
		edit       func(*setup)
	}{
		// The same rows give the same verdict: the whole listing (31 reads, the 31st empty) and
		// the incremental one (2 reads) are both MET, where the old 25-page budget ran out.
		{"the whole listing of 30 pages", "MET", 31, func(s *setup) { replay()(s); s.killPages = "40" }},
		{"the old 25-page budget", "do not reach the verdict", 25, replay()},
		{"the incremental listing", "MET", 2, func(s *setup) { replay()(s); incremental(s) }},
		{"a MISS since the verdict", "show a MISS", 2, func(s *setup) { replay(barNames("MISS", 2003, "1", "1"))(s); incremental(s) }},
		{"a MISS in the verdict's window", "show a MISS", 2, func(s *setup) {
			replay()(s)
			incremental(s)
			s.verdict.(doc)["windowNames"] = append([]string{barNames("MISS", 2004, "1", "0")}, window[1:]...)
		}},
		{"three INCOMPLETE rows trip the switch", "tripped kill switch", 2, func(s *setup) {
			replay(barNames("INCOMPLETE", 2005, "1", "0"), barNames("INCOMPLETE", 2006, "1", "0"))(s)
			incremental(s)
			s.verdict.(doc)["windowNames"] = append([]string{barNames("INCOMPLETE", 2007, "1", "0")}, window[1:]...)
		}},
		{"a re-run row replaces its window row", "show a MISS", 2, func(s *setup) {
			replay(barNames("MISS", 1003, "1", "0"))(s)
			incremental(s)
		}},
		{"an unreadable page since the verdict", "artifact page 2 is unreadable", 5, func(s *setup) {
			replay()(s)
			incremental(s)
			s.fail = []string{"100-2", "25-5"}
		}},
		{"a listing that does not reach the verdict's listing", "do not reach " + now.Add(-3*time.Hour).Format(time.RFC3339), 1, func(s *setup) {
			replay()(s)
			incremental(s)
			s.killPages = "1"
		}},
		{"a window of under 20 rows", "fewer than 20 rows", 2, func(s *setup) {
			replay()(s)
			incremental(s)
			s.verdict.(doc)["windowNames"] = window[:5]
		}},
		// A malformed listing time or window falls back to the whole listing and its budget.
		{"a window row of another epoch", "do not reach the verdict", 25, func(s *setup) {
			replay()(s)
			incremental(s)
			s.verdict.(doc)["windowNames"] = append([]string{barNames("CLEAN", 2008, "0", "0")}, window[1:]...)
		}},
		{"a malformed listing time", "do not reach the verdict", 25, func(s *setup) {
			replay()(s)
			incremental(s)
			s.verdict.(doc)["listedAt"] = "yesterday"
		}},
	}
	for _, c := range big {
		t.Run("CIN-V0-004 "+c.name, func(t *testing.T) {
			s := good()
			c.edit(&s)
			reads := -1
			s.reads = &reads
			verdict, reason := accept(t, s)
			if (c.want == "MET") != (verdict == "MET") || (c.want != "MET" && !strings.Contains(reason, c.want)) || reads != c.reads {
				t.Fatalf("verdict %s (%s) after %d listing reads, want %q after %d", verdict, reason, reads, c.want, c.reads)
			}
		})
	}
}

func TestAFPV0098SupersetGuard(t *testing.T) {
	const (
		base   = "142f0e3e7bfb153f803fb6c4bad7989f87ee324a"
		head   = "460fc31ae6d0134fa27f7a66336b99db88fc91c6"
		target = "5163d1ed4b61084fc32cf79355848899c3e8a5bb"
		a, b   = "example.org/m/a", "example.org/m/b"
		c, d   = "example.org/m/c", "example.org/m/d"
	)
	type in struct {
		driver, shadow     any
		shadowExit         string
		github, mergeQueue string
		baseRef            string
		packages, slices   string
		// AFP-V0-590: the selection within the universe and this shard's part of its placement.
		selected, placedPackages, placedSlices, balanced string
		universe, selectedOk                             string
	}
	units := func(pkgs ...string) []any {
		var u []any
		for _, p := range pkgs {
			u = append(u, doc{"adapter": "go", "unit": "go:" + p})
		}
		return u
	}
	good := func() in {
		return in{
			driver:     doc{"Base": base, "Head": head, "Target": target, "Packages": []string{a, b, c}},
			shadow:     doc{"scope": "NARROWED", "incomplete": []string{"shell: no qualified adapter and none declared"}, "invocations": []any{doc{"units": units(a, b)}}},
			shadowExit: "0", github: "0", mergeQueue: "1", baseRef: "main",
			packages: a + "\n" + d + "\n", slices: "-run ^(TestX)$ " + c + "\n-run ^(TestY)$ " + c + "\n",
			selected: a + "\n" + b + "\n" + c + "\n", placedPackages: b + "\n", placedSlices: "-run ^(TestY)$ " + c + "\n", balanced: "1",
			universe: a + "\n" + b + "\n" + c + "\n" + d + "\n", selectedOk: "1",
		}
	}
	decide := func(t *testing.T, x in) doc {
		dir := t.TempDir()
		write := func(name string, v any) string {
			p := filepath.Join(dir, name)
			if s, ok := v.(string); ok {
				writeFile(t, p, []byte(s))
			} else {
				writeDoc(t, p, v)
			}
			return p
		}
		args := []string{"-n", "-c", "--slurpfile", "driver", write("driver.json", x.driver), "--slurpfile", "shadow", write("shadow.json", x.shadow),
			"--arg", "shadowExit", x.shadowExit, "--rawfile", "packages", write("packages", x.packages), "--rawfile", "slices", write("slices", x.slices),
			"--rawfile", "selected", write("selected", x.selected), "--rawfile", "balancedPackages", write("placed-packages", x.placedPackages),
			"--rawfile", "balancedSlices", write("placed-slices", x.placedSlices), "--arg", "balanced", x.balanced,
			"--rawfile", "universe", write("universe", x.universe), "--arg", "selectedOk", x.selectedOk,
			"--arg", "github", x.github, "--arg", "mergeQueue", x.mergeQueue, "--arg", "baseRef", x.baseRef, "--arg", "base", base,
			"--arg", "head", head, "--arg", "target", target, "--arg", "pr", "868", "--arg", "runId", prRun, "--arg", "runAttempt", "1",
			"--arg", "shard", "3", "--arg", "shards", "8", "--arg", "toolSource", target, "--arg", "verdictRun", "4242", "-f", script(t, "narrow.jq")}
		out, err := exec.Command("jq", args...).Output()
		if err != nil {
			t.Fatalf("narrow.jq: %v", err)
		}
		var v doc
		if err := jsonUnmarshal(string(out), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	// AFP-V0-590: a NARROWED shard runs its part of the balanced placement (b whole, c's slice),
	// not its partition's intersection (a, c); a selected package of its partition placed on
	// another shard (a) is not skipped, and only its unselected package (d) is.
	v := decide(t, good())
	if v["mode"] != "NARROWED" || len(v["reasons"].([]any)) != 0 || fmt.Sprint(v["executed"]) != fmt.Sprint([]string{b, c}) ||
		fmt.Sprint(v["skipped"]) != fmt.Sprint([]string{d}) || v["placement"] != "BALANCED" || fmt.Sprint(v["selected"]) != fmt.Sprint([]string{a, b, c}) || v["profile"] != "corvint-pr-narrowing/0" || num(v["runId"]) != 38482995697 ||
		num(v["shard"]) != 3 || num(v["shards"]) != 8 || num(v["verdictRunId"]) != 4242 {
		t.Fatalf("superset decision = %v", v)
	}
	// A driver package outside the universe (another module) is not part of the selection.
	x := good()
	x.driver.(doc)["Packages"] = []string{a, b, c, "example.org/other/x"}
	if v := decide(t, x); v["mode"] != "NARROWED" || fmt.Sprint(v["selected"]) != fmt.Sprint([]string{a, b, c}) {
		t.Fatalf("decision with a driver package outside the universe = %v", v)
	}
	cases := []struct {
		name, reason string
		edit         func(*in)
	}{
		{"the driver omits a shadow-selected package", "is not a superset of the shadow selection: it omits 1 package(s) (" + b + ")", func(x *in) {
			x.driver.(doc)["Packages"] = []string{a, c}
		}},
		{"the driver fell back", "driver: planner failed", func(x *in) { x.driver.(doc)["Reason"] = "planner failed" }},
		// Split so that no literal of this package names every package (AFP-V0-025).
		{"the driver selected everything", "the whole universe", func(x *in) {
			x.driver.(doc)["Packages"] = []string{"./" + "..."}
		}},
		{"the driver planned another head", "another base, head or target", func(x *in) { x.driver.(doc)["Head"] = target }},
		{"the driver wrote nothing", "the driver wrote no selection", func(x *in) { x.driver = "" }},
		{"the shadow failed", "corvint test --dry-run exited 2", func(x *in) { x.shadowExit = "2" }},
		{"the shadow wrote nothing", "the shadow wrote no plan", func(x *in) { x.shadow = "" }},
		{"the shadow scope is unknown", "the shadow scope is UNKNOWN", func(x *in) { x.shadow.(doc)["scope"] = "UNKNOWN" }},
		{"the shadow is incomplete for Go", "the shadow plan is incomplete", func(x *in) {
			x.shadow.(doc)["incomplete"] = []string{"go: no qualified adapter and x"}
		}},
		{"the change touches .github/", "touches .github/", func(x *in) { x.github = "1" }},
		{"the change set is unreadable", "change set is unreadable", func(x *in) { x.github = "unreadable" }},
		{"main has no merge queue", "no merge-queue rule", func(x *in) { x.mergeQueue = "0" }},
		{"the rules are unreadable", "rules of main are unreadable", func(x *in) { x.mergeQueue = "unreadable" }},
		{"the base is not main", "the base branch is release, not main", func(x *in) { x.baseRef = "release" }},
		{"the balanced placement failed", "the balanced placement of the selection failed", func(x *in) { x.balanced = "0" }},
		{"the placement names an unselected package", "names an unselected package", func(x *in) { x.placedPackages = d + "\n" }},
		// AFP-V0-590 (2): the recorded selection is exactly the driver selection within the universe.
		{"a selection stage failed", "the selection within the universe could not be computed", func(x *in) { x.selectedOk = "0" }},
		{"the selection is truncated", "the recorded selection is not the driver selection within the universe", func(x *in) {
			x.selected = a + "\n" + b + "\n"
			x.placedSlices = ""
		}},
		{"the selection names an unselected package", "the recorded selection is not the driver selection within the universe", func(x *in) {
			x.selected = a + "\n" + b + "\n" + c + "\n" + d + "\n"
		}},
		{"every shard agrees on an empty selection", "the recorded selection is not the driver selection within the universe", func(x *in) {
			x.selected, x.placedPackages, x.placedSlices = "", "", ""
		}},
		{"the driver names no package of the universe", "the driver selection names no package of the universe", func(x *in) {
			x.driver.(doc)["Packages"] = []string{"example.org/other/x"}
			x.shadow.(doc)["invocations"] = []any{}
			x.selected, x.placedPackages, x.placedSlices = "", "", ""
		}},
		{"the universe is empty", "the driver selection names no package of the universe", func(x *in) {
			x.universe, x.selected, x.placedPackages, x.placedSlices = "", "", "", ""
			x.shadow.(doc)["invocations"] = []any{}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			x := good()
			c.edit(&x)
			v := decide(t, x)
			if v["mode"] != "FULL" || !strings.Contains(fmt.Sprint(v["reasons"]), c.reason) || len(v["skipped"].([]any)) != 0 ||
				len(v["executed"].([]any)) != 3 || v["placement"] != "PARTITION" || len(v["selected"].([]any)) != 0 {
				t.Fatalf("decision = %v, want FULL with %q and the whole shard", v, c.reason)
			}
		})
	}
	// AFP-V0-814 (V1-3100): a merge-queue selection's Reused packages count toward the shadow
	// superset, run nowhere and are recorded as `reused`, not `skipped`, only beside an applied
	// Reuse record that counts them; a reused package may not also be selected.
	reusing := func() in {
		x := good()
		x.driver = doc{"Base": base, "Head": head, "Target": target, "Packages": []string{b, c}, "Reused": []string{a},
			"Reuse": doc{"applied": true, "reason": "", "recordRunId": 40803231280, "recordJobId": 7, "recordBase": head,
				"queueBase": base, "reused": 1}}
		x.selected = b + "\n" + c + "\n"
		return x
	}
	if v := decide(t, reusing()); v["mode"] != "NARROWED" || fmt.Sprint(v["reused"]) != fmt.Sprint([]string{a}) ||
		fmt.Sprint(v["selected"]) != fmt.Sprint([]string{b, c}) || fmt.Sprint(v["skipped"]) != fmt.Sprint([]string{d}) ||
		v["reuse"].(doc)["recordRunId"] != float64(40803231280) {
		t.Fatalf("reusing decision = %v", v)
	}
	for _, c := range []struct {
		name, reason string
		edit         func(doc)
	}{
		{"reuse not applied", "name no applied reuse record", func(d doc) { d["Reuse"].(doc)["applied"] = false }},
		{"no reuse record", "name no applied reuse record", func(d doc) { delete(d, "Reuse") }},
		{"a miscounted reuse record", "name no applied reuse record", func(d doc) { d["Reuse"].(doc)["reused"] = 2 }},
		{"a reuse record of no base", "name no applied reuse record", func(d doc) { d["Reuse"].(doc)["recordBase"] = "main" }},
		{"a reused package also selected", "a reused package is also selected", func(d doc) {
			d["Reused"], d["Reuse"].(doc)["reused"] = []string{a, b}, 2
		}},
		{"a reused list that is no list", "the reused list is not a list of packages", func(d doc) { d["Reused"] = a }},
		{"no reused list", "is not a superset of the shadow selection", func(d doc) { delete(d, "Reused") }},
	} {
		t.Run("reuse: "+c.name, func(t *testing.T) {
			x := reusing()
			c.edit(x.driver.(doc))
			v := decide(t, x)
			if v["mode"] != "FULL" || !strings.Contains(fmt.Sprint(v["reasons"]), c.reason) || len(v["reused"].([]any)) != 0 ||
				len(v["skipped"].([]any)) != 0 {
				t.Fatalf("decision = %v, want FULL with %q", v, c.reason)
			}
		})
	}
}

// TestAFPV0098SupersetGuardFilter runs the tests step's narrowing filter from ci.yml: only a
// retained NARROWED record whose `executed` list equals its sidecar narrows the shard; every
// other gate outcome, including a skipped or refused gate (empty NARROW_MODE), runs it whole.
// AFP-V0-590/591: a BALANCED record runs the recomputed part of the balanced placement only
// when it equals the record, and the coverage marker names the placement the shard covered.
func TestAFPV0098SupersetGuardFilter(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ci.yml narrowing filter runs under bash")
	const a, b, c, d = "example.org/m/a", "example.org/m/b", "example.org/m/c", "example.org/m/d"
	workflow, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?ms)^( {10}narrowed=""\n.*?)^ {10}if \[ -z "\$packages" \] && \[ -z "\$slices" \]; then$`).FindAllStringSubmatch(string(workflow), -1)
	if len(m) != 1 {
		t.Fatalf("%d narrowing filters in ci.yml, want 1", len(m))
	}
	filter := regexp.MustCompile(`(?m)^ {10}`).ReplaceAllString(m[0][1], "")
	const packages, slices = a + "\n" + d, "-run ^(TestX)$ " + c + "\n-run ^(TestY)$ " + c
	whole := "narrowed=\npackages=" + packages + "\nslices=" + slices + "\n"
	type in struct {
		event, mode, recorded string
		record, sidecar       any // a doc, a raw string, or nil for no file
		sidecarDir            bool
		ciShards              string // the fake ci-shards script, or "" for none
	}
	good := func() in {
		return in{event: "pull_request", mode: "NARROWED", recorded: "success",
			record:  narrowingRecord(3, "NARROWED", prRun, []string{a, c}, []string{d}),
			sidecar: a + "\n" + c + "\n"}
	}
	run := func(t *testing.T, x in) string {
		dir := t.TempDir()
		out := filepath.Join(dir, "corvint-narrowing")
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, v := range map[string]any{"narrowing.json": x.record, "executed": x.sidecar} {
			switch v := v.(type) {
			case string:
				writeFile(t, filepath.Join(out, name), []byte(v))
			case doc:
				writeDoc(t, filepath.Join(out, name), v)
			}
		}
		if x.sidecarDir {
			if err := os.MkdirAll(filepath.Join(out, "executed"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		writeFile(t, filepath.Join(dir, "ci-package-universe"), []byte(a+"\n"+b+"\n"+c+"\n"+d+"\n"))
		if x.ciShards != "" {
			writeFile(t, filepath.Join(dir, "ci-shards"), []byte(x.ciShards))
			if err := os.Chmod(filepath.Join(dir, "ci-shards"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		state := filepath.Join(dir, "state")
		script := "packages=$PACKAGES\nslices=$SLICES\n" + filter +
			"printf 'narrowed=%s\\npackages=%s\\nslices=%s\\n' \"$narrowed\" \"$packages\" \"$slices\" > \"$STATE\"\n"
		cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", script)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GITHUB_EVENT_NAME=" + x.event, "NARROW_MODE=" + x.mode,
			"NARROW_RECORDED=" + x.recorded, "RUNNER_TEMP=" + dir, "SHARD=3", "SHARDS=8", "GITHUB_RUN_ID=" + prRun,
			"PACKAGES=" + packages, "SLICES=" + slices, "STATE=" + state}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("filter: %v\n%s", err, out)
		}
		got, err := os.ReadFile(state)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "ci-coverage", "coverage.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cov doc
		if err := json.Unmarshal(raw, &cov); err != nil {
			t.Fatal(err)
		}
		if cov["profile"] != "corvint-pr-coverage/0" || cov["runId"] != prRun || cov["shard"] != float64(3) || cov["shards"] != float64(8) {
			t.Fatalf("coverage marker %s", raw)
		}
		return string(got) + fmt.Sprintf("placement=%v selected=%v\n", cov["placement"], cov["selected"])
	}
	partition := "placement=PARTITION selected=[]\n"
	balancedRecord := func(executed []string) doc {
		r := narrowingRecord(3, "NARROWED", prRun, executed, []string{d})
		r["placement"], r["selected"] = "BALANCED", []string{a, b, c}
		return r
	}
	// The fake ci-shards places b whole and one slice of c on shard 3.
	const placer = "#!/bin/sh\ncase \"$*\" in *--select*--slices*) echo '-run ^(TestY)$ " + c + "' ;; *--select*) echo " + b + " ;; *) exit 9 ;; esac\n"
	balancedSelected := "placement=BALANCED selected=[" + a + " " + b + " " + c + "]\n"
	cases := []struct {
		name, want string
		edit       func(*in)
	}{
		{"a retained NARROWED record that agrees with its sidecar", "narrowed=1\npackages=" + a + "\nslices=" + slices + "\n" + partition, func(*in) {}},
		{"an empty intersection runs no Go test (AFP-V0-028)", "narrowed=1\npackages=\nslices=\n" + partition, func(x *in) {
			x.record, x.sidecar = narrowingRecord(3, "NARROWED", prRun, []string{}, []string{a, c, d}), ""
		}},
		{"a skipped or refused gate (empty mode)", whole, func(x *in) { x.mode = "" }},
		{"a FULL gate", whole, func(x *in) { x.mode = "FULL" }},
		{"an unretained record", whole, func(x *in) { x.recorded = "failure" }},
		{"a skipped record upload", whole, func(x *in) { x.recorded = "" }},
		// AFP-V0-732: a merge-queue entry's retained NARROWED record narrows it the same way.
		{"a merge-queue run", "narrowed=1\npackages=" + a + "\nslices=" + slices + "\n" + partition, func(x *in) { x.event = "merge_group" }},
		{"a push run", whole, func(x *in) { x.event = "push" }},
		{"a missing sidecar", whole, func(x *in) { x.sidecar = nil }},
		{"an unreadable sidecar", whole, func(x *in) { x.sidecar, x.sidecarDir = nil, true }},
		{"an empty sidecar for a non-empty record", whole, func(x *in) { x.sidecar = "" }},
		{"a sidecar that names more than the record", whole, func(x *in) { x.sidecar = a + "\n" + c + "\n" + d + "\n" }},
		{"a sidecar in another order", whole, func(x *in) { x.sidecar = c + "\n" + a + "\n" }},
		{"a missing record", whole, func(x *in) { x.record = nil }},
		{"an unreadable record", whole, func(x *in) { x.record = "{" }},
		{"a FULL record", whole, func(x *in) { x.record = narrowingRecord(3, "FULL", prRun, []string{a, c, d}, []string{}) }},
		{"a record naming a package outside the shard", whole, func(x *in) {
			x.record = narrowingRecord(3, "NARROWED", prRun, []string{a, "example.org/m/z"}, []string{d})
			x.sidecar = a + "\nexample.org/m/z\n"
		}},
		{"a BALANCED record that its recomputed placement equals (AFP-V0-590)", "narrowed=1\npackages=" + b + "\nslices=-run ^(TestY)$ " + c + "\n" + balancedSelected, func(x *in) {
			x.record, x.sidecar, x.ciShards = balancedRecord([]string{b, c}), b+"\n"+c+"\n", placer
		}},
		{"a BALANCED record its placement disagrees with runs the shard and that part", "narrowed=\npackages=" + a + "\n" + d + "\n" + b + "\nslices=" + slices + "\n" + balancedSelected, func(x *in) {
			x.record, x.sidecar, x.ciShards = balancedRecord([]string{a, c}), a+"\n"+c+"\n", placer
		}},
		{"a BALANCED record whose placement cannot be recomputed runs whole", whole + partition, func(x *in) {
			x.record, x.sidecar, x.ciShards = balancedRecord([]string{b, c}), b+"\n"+c+"\n", "#!/bin/sh\nexit 9\n"
		}},
		{"a BALANCED record without a selection runs whole", whole + partition, func(x *in) {
			r := balancedRecord([]string{b, c})
			delete(r, "selected")
			x.record, x.sidecar, x.ciShards = r, b+"\n"+c+"\n", placer
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x := good()
			tc.edit(&x)
			if !strings.HasSuffix(tc.want, "]\n") {
				tc.want += partition
			}
			if got := run(t, x); got != tc.want {
				t.Fatalf("filter output:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

// TestAFPV0098SupersetGuardSidecar runs narrow.sh with fake tools: a NARROWED decision writes
// the `executed` sidecar equal to its record, and a sidecar that cannot be written drops the
// record and reports FULL, so the tests step runs the whole shard.
func TestAFPV0098SupersetGuardSidecar(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the git, gh, go and driver spies are #!/bin/sh scripts run by narrow.sh under sh")
	const (
		base   = "142f0e3e7bfb153f803fb6c4bad7989f87ee324a"
		head   = "460fc31ae6d0134fa27f7a66336b99db88fc91c6"
		target = "5163d1ed4b61084fc32cf79355848899c3e8a5bb"
		a, b   = "example.org/m/a", "example.org/m/b"
		c, d   = "example.org/m/c", "example.org/m/d"
	)
	narrow := func(t *testing.T, blockSidecar bool, override map[string]string) (string, string) {
		dir := t.TempDir()
		fake := filepath.Join(dir, "fake")
		bin := filepath.Join(dir, "bin")
		writeDoc(t, filepath.Join(fake, "selection.json"), doc{"EventBase": base, "Reason": "", "Base": base, "Head": head, "Target": target, "Packages": []string{a, b, c}})
		writeDoc(t, filepath.Join(fake, "shadow.json"), doc{"scope": "NARROWED", "incomplete": []string{},
			"invocations": []any{doc{"units": []any{doc{"adapter": "go", "unit": "go:" + a}, doc{"adapter": "go", "unit": "go:" + b}}}}})
		tools := map[string]string{
			"git":       "#!/bin/sh\necho internal/x.go\n",
			"gh":        "#!/bin/sh\necho true\n",
			"go":        "#!/bin/sh\nprintf '%s\\n' " + a + " " + b + " " + c + " " + d + "\n",
			"driver":    "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" != --out ] || o=$2; shift; done\ncat \"$FAKE/selection.json\" > \"$o/selection.json\"\n",
			"shadow":    "#!/bin/sh\ncat \"$FAKE/shadow.json\"\n",
			"ci-shards": "#!/bin/sh\ncase \"$*\" in *--select*--slices*) echo '-run ^(TestX)$ " + c + "' ;; *--select*) echo " + b + " ;; *--slices*) echo '-run ^(TestX)$ " + c + "' ;; *) printf '%s\\n' " + a + " " + d + " ;; esac\n",
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
		if blockSidecar {
			if err := os.MkdirAll(filepath.Join(dir, "corvint-narrowing", "executed"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		output := filepath.Join(dir, "output")
		writeFile(t, output, nil)
		cmd := exec.Command("sh", script(t, "narrow.sh"))
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir, "FAKE=" + fake,
			"GH_TOKEN=x", "REPO=beamfall/corvint", "BASE_REF=main", "PR_BASE=" + base, "PR_HEAD=" + head, "PR_NUMBER=868",
			"TESTED_MERGE=" + target, "SHARD=3", "SHARDS=8", "TOOL_SOURCE=" + target, "VERDICT_RUN=4242", "PR_BIN=" + bin,
			"SHADOW=" + filepath.Join(bin, "shadow"), "CI_SHARDS=" + filepath.Join(bin, "ci-shards"), "CONFINE=" + filepath.Join(bin, "confine"),
			"GITHUB_RUN_ID=" + prRun, "GITHUB_RUN_ATTEMPT=1", "RUNNER_TEMP=" + dir, "GITHUB_OUTPUT=" + output,
			"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary")}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("narrow.sh: %v\n%s", err, out)
		}
		o, _ := os.ReadFile(output)
		mode := regexp.MustCompile(`(?m)^mode=(.*)$`).FindStringSubmatch(string(o))
		if mode == nil {
			t.Fatalf("narrow.sh wrote no mode:\n%s", o)
		}
		return mode[1], filepath.Join(dir, "corvint-narrowing")
	}
	t.Run("a NARROWED record and its sidecar agree", func(t *testing.T) {
		mode, out := narrow(t, false, nil)
		record, err := os.ReadFile(filepath.Join(out, "narrowing.json"))
		if err != nil {
			t.Fatal(err)
		}
		var v doc
		if err := jsonUnmarshal(string(record), &v); err != nil {
			t.Fatal(err)
		}
		sidecar, err := os.ReadFile(filepath.Join(out, "executed"))
		if err != nil {
			t.Fatal(err)
		}
		if mode != "NARROWED" || v["mode"] != "NARROWED" || fmt.Sprint(v["executed"]) != fmt.Sprint([]string{b, c}) ||
			string(sidecar) != b+"\n"+c+"\n" || v["placement"] != "BALANCED" || fmt.Sprint(v["selected"]) != fmt.Sprint([]string{a, b, c}) {
			t.Fatalf("mode %s, record %v, sidecar %q", mode, v, sidecar)
		}
	})
	// AFP-V0-801: all consumers use the proven parent, retaining the original event base.
	t.Run("stale event base", func(t *testing.T) {
		effective := strings.Repeat("a", 40)
		mode, out := narrow(t, false, map[string]string{
			"driver": "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" != --out ] || o=$2; shift; done\njq --arg base " + effective + " '.Base=$base' \"$FAKE/selection.json\" > \"$o/selection.json\"\n",
			"git":    "#!/bin/sh\n[ \"$*\" = 'diff --no-renames --name-only " + effective + " " + target + "' ] || { echo .github/main-only.yml; exit 0; }\necho internal/x.go\n",
			"shadow": "#!/bin/sh\ncase \"$*\" in *'--base " + effective + " '*) cat \"$FAKE/shadow.json\" ;; *) exit 1 ;; esac\n",
		})
		v := readDoc(t, filepath.Join(out, "narrowing.json"))
		if mode != "NARROWED" || v["base"] != effective || v["effectiveBase"] != effective || v["eventBase"] != base {
			t.Fatalf("range: %s %v", mode, v)
		}
	})
	t.Run("PR-owned github edit stays full", func(t *testing.T) {
		mode, out := narrow(t, false, map[string]string{"git": "#!/bin/sh\necho .github/pr-owned.yml\n"})
		v := readDoc(t, filepath.Join(out, "narrowing.json"))
		if mode != "FULL" || !strings.Contains(fmt.Sprint(v["reasons"]), "the change touches .github/") {
			t.Fatalf("github: %s %v", mode, v)
		}
	})
	// AFP-V0-800: each driver topology refusal survives in the FULL record.
	for _, reason := range []string{"pr-base-invalid-oid", "pr-base-parents-unreadable", "pr-base-parent-count", "pr-base-head-mismatch", "pr-base-not-ancestor"} {
		t.Run(reason, func(t *testing.T) {
			mode, out := narrow(t, false, map[string]string{"driver": "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" != --out ] || o=$2; shift; done\njq --arg reason " + reason + " '.Reason=$reason' \"$FAKE/selection.json\" > \"$o/selection.json\"\n"})
			v := readDoc(t, filepath.Join(out, "narrowing.json"))
			if mode != "FULL" || !strings.Contains(fmt.Sprint(v["reasons"]), reason) {
				t.Fatalf("refusal: %s %v", mode, v)
			}
		})
	}
	// AFP-V0-801: reject each identity field independently, even with valid packages.
	for _, tc := range []struct{ name, edit string }{
		{"missing-event-base", "del(.EventBase)"},
		{"event-base-mismatch", ".EventBase = .Head"},
		{"head-mismatch", ".Head = .Base"},
		{"target-mismatch", ".Target = .Base"},
		{"nonempty-reason", `.Reason = "identity refused"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, out := narrow(t, false, map[string]string{"driver": "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" != --out ] || o=$2; shift; done\njq '" + tc.edit + "' \"$FAKE/selection.json\" > \"$o/selection.json\"\n"})
			selection := readDoc(t, filepath.Join(out, "driver", "selection.json"))
			if fmt.Sprint(selection["Packages"]) != fmt.Sprint([]string{a, b, c}) {
				t.Fatalf("driver fixture lost its valid packages: %v", selection)
			}
			v := readDoc(t, filepath.Join(out, "narrowing.json"))
			if mode != "FULL" || v["mode"] != "FULL" || v["effectiveBase"] != "" {
				t.Fatalf("identity guard: mode=%s record=%v", mode, v)
			}
		})
	}
	t.Run("a sidecar that cannot be written is FULL without a record", func(t *testing.T) {
		mode, out := narrow(t, true, nil)
		if _, err := os.Stat(filepath.Join(out, "narrowing.json")); mode != "FULL" || !os.IsNotExist(err) {
			t.Fatalf("mode %s, record stat %v; want FULL and no record", mode, err)
		}
	})
	// AFP-V0-590 (2): a selection stage that fails after a partial write is not masked by the
	// stages after it (dash has no pipefail), so its truncated selection never narrows.
	t.Run("a failing selection stage is FULL", func(t *testing.T) {
		mode, out := narrow(t, false, map[string]string{"grep": "#!/bin/sh\necho " + a + "\nexit 2\n"})
		summary, _ := os.ReadFile(filepath.Join(filepath.Dir(out), "summary"))
		if mode != "FULL" || !strings.Contains(string(summary), "the selection within the universe could not be computed") {
			t.Fatalf("mode %s, summary:\n%s", mode, summary)
		}
	})
}

func TestAFPV0097WorkflowBindings(t *testing.T) {
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	ci, bar := read("../../.github/workflows/ci.yml"), read("../../.github/workflows/ci-forward-bar.yml")
	one := func(text, pattern string) string {
		t.Helper()
		m := regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1)
		if len(m) != 1 {
			t.Fatalf("%d matches of %q, want 1", len(m), pattern)
		}
		return m[0][1]
	}
	epoch := one(ci, `(?m)^      CORVINT_PR_FORWARD_EPOCH: "([0-9]+)"$`)
	if e := one(bar, `(?m)^      FORWARD_EPOCH: "([0-9]+)"$`); e != epoch {
		t.Fatalf("ci-forward-bar.yml computes epoch %s, ci.yml rows are epoch %s", e, epoch)
	}
	if n := one(bar, `(?m)^          name: afp-shadow-bar-epoch-([0-9]+)$`); n != epoch {
		t.Fatalf("ci-forward-bar.yml uploads epoch %s, ci.yml rows are epoch %s", n, epoch)
	}
	verdictJob := one(ci, `(?ms)^  narrowing-verdict:\n(.*?)\n  go-product-shard:`)
	if e := one(verdictJob, `(?m)^          FORWARD_EPOCH: "([0-9]+)"$`); e != epoch {
		t.Fatalf("narrowing-verdict accepts epoch %s, ci.yml rows are epoch %s", e, epoch)
	}
	// The trusted verdict runs only from main on a schedule or dispatch, with read-only scopes.
	on := one(bar, `(?ms)^on:\n(.*?)\n\S`)
	if !strings.Contains(on, "schedule:") || !strings.Contains(on, "workflow_dispatch:") || strings.Contains(on, "pull_request") ||
		strings.Contains(on, "push") || strings.Contains(on, "workflow_run") {
		t.Fatalf("ci-forward-bar.yml triggers:\n%s", on)
	}
	for _, want := range []string{"if: github.ref == 'refs/heads/main'", "run: sh trusted/.github/forward-rows/verdict.sh",
		"sparse-checkout: .github/forward-rows", "ref: ${{ github.sha }}"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("ci-forward-bar.yml lacks %q", want)
		}
	}
	// Exactly two permission blocks, the workflow's and the job's, each read-only.
	if strings.Count(bar, "permissions:") != 2 || !strings.Contains(bar, "\npermissions:\n  actions: read\n  contents: read\n\n") ||
		!strings.Contains(bar, "\n    permissions:\n      actions: read\n      contents: read\n    env:") {
		t.Fatal("ci-forward-bar.yml permissions are not exactly actions: read and contents: read")
	}
	// The pull-request side: off unless the variable is on, never for a fork, accepts only
	// that workflow's verdict from the base commit's scripts.
	for _, want := range []string{"vars.CORVINT_PR_NARROWING == 'on'", "github.event.pull_request.head.repo.full_name == github.repository",
		"github.event.pull_request.base.ref == 'main'", "ref: ${{ github.event.pull_request.base.sha || github.event.merge_group.base_sha }}",
		// AFP-V0-730: a merge-queue entry only on main and only while its own variable is on.
		"(github.event_name == 'merge_group' && vars.CORVINT_MQ_NARROWING == 'on' && github.event.merge_group.base_ref == 'refs/heads/main')",
		"VERDICT_WORKFLOW: .github/workflows/ci-forward-bar.yml", "run: sh trusted/.github/forward-rows/accept.sh"} {
		if !strings.Contains(verdictJob, want) {
			t.Fatalf("narrowing-verdict lacks %q", want)
		}
	}
	gate := one(ci, `(?ms)^      - name: Narrowing gate .*?\n(.*?)\n      - name: Retain the narrowing record`)
	for _, want := range []string{"needs.narrowing-verdict.outputs.verdict == 'MET'", "vars.CORVINT_PR_NARROWING == 'on'",
		"github.event_name == 'pull_request'", "github.event.pull_request.head.repo.full_name == github.repository",
		`sh "$GITHUB_WORKSPACE/trusted-tools/.github/forward-rows/narrow.sh"`,
		// AFP-V0-731: a merge-queue entry runs mq-narrow.sh from its queue base, never the entry.
		"vars.CORVINT_MQ_NARROWING == 'on'", `git archive "$QUEUE_BASE" .github/forward-rows`, `sh "$trusted/.github/forward-rows/mq-narrow.sh"`,
		"QUEUE_BASE: ${{ github.event.merge_group.base_sha }}"} {
		if !strings.Contains(gate, want) {
			t.Fatalf("the narrowing gate lacks %q", want)
		}
	}
	// AFP-V0-461: the tests step that writes every shard's outcomes (outcomes.sh) names the pull
	// request head, or each forward row of a pull request is INCOMPLETE and trips the kill switch.
	testsEnv := one(ci, `(?ms)^      - name: Go tests \(full until matching shadow qualification\)\n(.*?)\n        run: \|`)
	if !strings.Contains(testsEnv, "\n          PR_HEAD: ${{ github.event.pull_request.head.sha }}\n") {
		t.Fatalf("the Go tests step does not name PR_HEAD:\n%s", testsEnv)
	}
	// A narrowed shard narrows only after its record was retained and names no tested tree.
	for _, want := range []string{`{ [ "$GITHUB_EVENT_NAME" = pull_request ] || [ "$GITHUB_EVENT_NAME" = merge_group ]; } && [ "$NARROW_MODE" = NARROWED ] && [ "$NARROW_RECORDED" = success ]`,
		"name: ci-narrowing-${{ strategy.job-index }}", `if [ -n "$narrowed" ]; then`, `[ -n "$narrowed" ] || echo "costs=1"`} {
		if !strings.Contains(ci, want) {
			t.Fatalf("ci.yml lacks %q", want)
		}
	}
	// The gate is the only route to a narrowed shard: the tests step never runs the deciding
	// driver, so a skipped or refused verdict or gate (empty NARROW_MODE) runs the whole shard,
	// and no historical qualification pin can admit a narrowed run.
	tests := one(ci, `(?ms)^      - name: Go tests \(full until.*?\n(.*?)\n      - name: `)
	for _, banned := range []string{"corvint-pr-bin", "--qualification", "--profile", `-z "$NARROW_MODE"`, "/driver"} {
		if strings.Contains(tests, banned) {
			t.Fatalf("the Go tests step contains %q", banned)
		}
	}
	if strings.Contains(ci, "CORVINT_PR_QUALIFICATION") || strings.Contains(ci, "trusted-qualification") {
		t.Fatal("ci.yml still pins a qualification for the driver path")
	}
}

func jsonUnmarshal(s string, v *doc) error { return json.Unmarshal([]byte(s), v) }

// TestAFPV0591CoverageGuard runs go-product's coverage guard from ci.yml over the shards'
// coverage markers. It fails closed: a pull request passes only when each shard 0..N-1 left one
// marker of this run and either all ran their partition or all ran their part of one balanced
// placement; no marker, a missing or extra shard, a marker of another run or a later attempt,
// or a mix fails, since a shard that ran only its partition may leave another shard's selected
// package unrun. DOCS and REUSE runs, whose shards run no tests step, need no marker.
func TestAFPV0591CoverageGuard(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ci.yml coverage guard runs under bash")
	workflow, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^ {6}- name: Every go-product shard and the static job succeeded\n(?: {8,}.*\n)*? {8}run: \|\n((?: {10}.*\n)+)`).FindAllStringSubmatch(string(workflow), -1)
	if len(m) != 1 || !strings.Contains(m[0][1], "AFP-V0-591") {
		t.Fatalf("%d go-product checks with the coverage guard in ci.yml, want 1", len(m))
	}
	if !strings.Contains(m[0][0], "          SHARDS: \"8\"\n") || !strings.Contains(m[0][0], "          DOCS_MODE: ${{ needs.docs-plan.outputs.mode }}\n") {
		t.Fatal("the go-product check does not read the shard count and the docs-plan mode")
	}
	if !strings.Contains(string(workflow), "      - name: Fetch the shard coverage markers (AFP-V0-591)\n"+
		"        if: (github.event_name == 'pull_request' || github.event_name == 'merge_group') && needs.go-product-shard.result == 'success'\n") {
		t.Fatal("go-product does not fetch the coverage markers of a pull request or merge-queue entry whose shards passed (AFP-V0-732)")
	}
	guard := regexp.MustCompile(`(?m)^ {10}`).ReplaceAllString(m[0][1], "")
	selected := []string{"example.org/m/a", "example.org/m/b"}
	marker := func(shard int, placement string) doc {
		sel := []string{}
		if placement == "BALANCED" {
			sel = selected
		}
		return doc{"profile": "corvint-pr-coverage/0", "runId": prRun, "runAttempt": "1", "shard": shard, "shards": shards,
			"placement": placement, "selected": sel}
	}
	all := func(placement string) []doc {
		out := []doc{}
		for i := 0; i < shards; i++ {
			out = append(out, marker(i, placement))
		}
		return out
	}
	run := func(t *testing.T, event, docsMode string, markers []doc) (string, error) {
		dir := t.TempDir()
		for i, mk := range markers {
			path := filepath.Join(dir, "ci-coverage", "ci-coverage-"+strconv.Itoa(i), "coverage.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(mk)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, raw)
		}
		cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", guard)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RUNNER_TEMP=" + dir, "GITHUB_RUN_ID=" + prRun, "GITHUB_RUN_ATTEMPT=2",
			"GITHUB_EVENT_NAME=" + event, "BATCHED=false", "SHARDS_RESULT=success", "STATIC_RESULT=success", "DOCS_RESULT=success",
			"DOCS_MODE=" + docsMode, "SHARDS=" + strconv.Itoa(shards), "GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary")}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	edit := func(placement string, f func([]doc) []doc) []doc { return f(all(placement)) }
	cases := []struct {
		name, want string
		markers    []doc
		pass       bool
	}{
		{"every shard ran its partition", "all 8 shards ran their partition", all("PARTITION"), true},
		{"every shard ran one balanced placement", "all 8 shards ran their part of one balanced placement of 2 selected package(s)", all("BALANCED"), true},
		// Each shard keeps the latest marker it uploaded, so a re-run of some shards mixes attempts.
		{"a re-run shard agrees with an earlier attempt", "one balanced placement", edit("BALANCED", func(ms []doc) []doc {
			ms[4]["runAttempt"] = "2"
			return ms
		}), true},
		{"no marker at all", "no shard coverage marker was found", nil, false},
		{"a shard ran only its partition", "1 shard(s) covered only their partition", edit("BALANCED", func(ms []doc) []doc {
			ms[5] = marker(5, "PARTITION")
			return ms
		}), false},
		{"a balanced shard left no marker", "the markers name shards 0,1,2,3,4,5,6, want 0 to 7", edit("BALANCED", func(ms []doc) []doc { return ms[:7] }), false},
		{"a partition shard left no marker", "want 0 to 7", edit("PARTITION", func(ms []doc) []doc { return ms[1:] }), false},
		{"an extra marker", "want 0 to 7", append(all("PARTITION"), marker(8, "PARTITION")), false},
		{"a shard is named twice", "want 0 to 7", edit("BALANCED", func(ms []doc) []doc { ms[7]["shard"] = 6; return ms }), false},
		{"the shard counts disagree", "a marker names another shard count than 8", edit("BALANCED", func(ms []doc) []doc { ms[2]["shards"] = 4; return ms }), false},
		{"every marker names another shard count", "a marker names another shard count than 8", edit("PARTITION", func(ms []doc) []doc {
			for _, m := range ms {
				m["shards"] = 4
			}
			return ms
		}), false},
		{"a marker of another run", "a marker is from another run", edit("PARTITION", func(ms []doc) []doc { ms[0]["runId"] = "1"; return ms }), false},
		{"a marker of a later attempt", "a marker names no attempt of this run up to this one", edit("PARTITION", func(ms []doc) []doc {
			ms[0]["runAttempt"] = "3"
			return ms
		}), false},
		{"a marker of another profile", "a marker is not corvint-pr-coverage/0", edit("PARTITION", func(ms []doc) []doc {
			ms[6]["profile"] = "x"
			return ms
		}), false},
		{"an unknown placement", "a marker names an unknown placement", edit("PARTITION", func(ms []doc) []doc {
			ms[1]["placement"] = "INTERSECT"
			return ms
		}), false},
		{"the selections differ", "the balanced shards placed different selections", edit("BALANCED", func(ms []doc) []doc {
			ms[3]["selected"] = []string{"example.org/m/a"}
			return ms
		}), false},
		{"a docs-only run needs no marker", "docs-plan DOCS", nil, true},
		{"a reused run needs no marker", "docs-plan REUSE", nil, true},
		// AFP-V0-732: a merge-queue entry may narrow, so it is checked as a pull request is.
		{"a merge-queue run with a mixed placement fails", "covered only their partition", edit("BALANCED", func(ms []doc) []doc {
			ms[2]["placement"], ms[2]["selected"] = "PARTITION", []string{}
			return ms
		}), false},
		{"a merge-queue run of one balanced placement passes", "AFP-V0-591: covered; all 8 shards ran their part", all("BALANCED"), true},
		{"a push run reads no marker", "", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			event, docsMode := "pull_request", "FULL"
			switch {
			case strings.HasPrefix(c.name, "a merge-queue run"):
				event = "merge_group"
			case strings.HasPrefix(c.name, "a push run"):
				event = "push"
			case strings.HasPrefix(c.name, "a docs-only run"):
				docsMode = "DOCS"
			case strings.HasPrefix(c.name, "a reused run"):
				docsMode = "REUSE"
			}
			out, err := run(t, event, docsMode, c.markers)
			if (err == nil) != c.pass || !strings.Contains(out, c.want) {
				t.Fatalf("guard err=%v output:\n%s\nwant pass=%v containing %q", err, out, c.pass, c.want)
			}
			if !c.pass && (!strings.Contains(out, "::error::AFP-V0-591: the shard coverage is not proven") || !strings.Contains(out, "Re-run all jobs.")) {
				t.Fatalf("guard failed without the AFP-V0-591 error:\n%s", out)
			}
		})
	}
	// Unreadable markers fail closed with the same instruction.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ci-coverage", "ci-coverage-0", "coverage.json"), []byte("{not json"))
	cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", guard)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RUNNER_TEMP=" + dir, "GITHUB_RUN_ID=" + prRun, "GITHUB_RUN_ATTEMPT=1",
		"GITHUB_EVENT_NAME=pull_request", "BATCHED=false", "SHARDS_RESULT=success", "STATIC_RESULT=success", "DOCS_RESULT=success",
		"DOCS_MODE=FULL", "SHARDS=" + strconv.Itoa(shards), "GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary")}
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "::error::AFP-V0-591: the shard coverage markers are unreadable. Re-run all jobs.") {
		t.Fatalf("unreadable markers: err=%v output:\n%s", err, out)
	}
}
