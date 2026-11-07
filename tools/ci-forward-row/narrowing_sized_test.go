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

// TestAFPV02724ShardPlanSizesTheMatrix runs shard-plan.sh with the narrow.sh spies: a NARROWED
// decision sizes the recorded selection with ci-shards at the shards' parallelism and prints the
// count and matrix; a FULL decision, a sizing failure and a count outside 1..FULL_SHARDS keep the
// full matrix. The job never fails.
func TestAFPV02724ShardPlanSizesTheMatrix_AFPV02724(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "shard-plan.sh runs narrow.sh and the tool spies under sh")
	const (
		base       = "142f0e3e7bfb153f803fb6c4bad7989f87ee324a"
		head       = "460fc31ae6d0134fa27f7a66336b99db88fc91c6"
		target     = "5163d1ed4b61084fc32cf79355848899c3e8a5bb"
		a, b, c, d = "example.org/m/a", "example.org/m/b", "example.org/m/c", "example.org/m/d"
	)
	full16 := "[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]"
	plan := func(t *testing.T, override map[string]string, extra ...string) (count, matrix, summary, fake string) {
		t.Helper()
		dir := t.TempDir()
		fake, bin := filepath.Join(dir, "fake"), filepath.Join(dir, "bin")
		writeDoc(t, filepath.Join(fake, "selection.json"), doc{"EventBase": base, "Reason": "", "Base": base, "Head": head, "Target": target, "Packages": []string{a, b, c}})
		writeDoc(t, filepath.Join(fake, "shadow.json"), doc{"scope": "NARROWED", "incomplete": []string{},
			"invocations": []any{doc{"units": []any{doc{"adapter": "go", "unit": "go:" + a}, doc{"adapter": "go", "unit": "go:" + b}}}}})
		tools := fakeNarrowTools(a, b, c, d)
		tools["git"] = guardGit
		for k, v := range override {
			tools[k] = v
		}
		installTools(t, bin, tools)
		writeFile(t, filepath.Join(dir, "output"), nil)
		cmd := exec.Command("sh", script(t, "shard-plan.sh"))
		cmd.Dir = dir
		cmd.Env = append(narrowEnv(dir, bin, fake, base, head, target, "9", "9"), "SHARD_TEST_P=4")
		cmd.Env = append(cmd.Env, extra...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("shard-plan.sh: %v\n%s", err, out)
		}
		s, _ := os.ReadFile(filepath.Join(dir, "summary"))
		return outputValue(t, dir, "count"), outputValue(t, dir, "matrix"), string(s), fake
	}
	t.Run("a NARROWED decision sizes the recorded selection", func(t *testing.T) {
		count, matrix, summary, fake := plan(t, nil)
		if count != "3" || matrix != "[0,1,2]" || !strings.Contains(summary, "AFP-V0-2724 shard plan: 3 of 16 shard(s)") || !strings.Contains(summary, "3 selected package(s)") {
			t.Fatalf("count %s matrix %s\n%s", count, matrix, summary)
		}
		argv, _ := os.ReadFile(filepath.Join(fake, "size-argv"))
		for _, want := range []string{"--shards 16", "--min-shards 2", "--size-budget-ms 400000", "--parallel 4", "--select "} {
			if !strings.Contains(string(argv), want) {
				t.Fatalf("ci-shards argv %q lacks %q", argv, want)
			}
		}
		selected := regexp.MustCompile(`--select (\S+)`).FindStringSubmatch(string(argv))
		sel, _ := os.ReadFile(selected[1])
		universe, _ := os.ReadFile(filepath.Join(fake, "size-universe"))
		if string(sel) != a+"\n"+b+"\n"+c+"\n" || string(universe) != a+"\n"+b+"\n"+c+"\n"+d+"\n" {
			t.Fatalf("sized selection %q over universe %q", sel, universe)
		}
		// narrow.sh ran as shard 0 of the full matrix, and its record is the plan's evidence.
		if !strings.Contains(summary, "AFP-V0-098 shard 0: NARROWED") {
			t.Fatalf("summary lacks the shard 0 decision:\n%s", summary)
		}
	})
	t.Run("the floor, the budget and the full matrix are the job's", func(t *testing.T) {
		count, matrix, _, fake := plan(t, nil, "FULL_SHARDS=4", "MIN_SHARDS=1", "WALL_BUDGET_MS=120000")
		argv, _ := os.ReadFile(filepath.Join(fake, "size-argv"))
		if count != "3" || matrix != "[0,1,2]" || !strings.Contains(string(argv), "--shards 4 --min-shards 1 --size-budget-ms 120000") {
			t.Fatalf("count %s matrix %s argv %q", count, matrix, argv)
		}
	})
	t.Run("a FULL decision keeps the full matrix", func(t *testing.T) {
		count, matrix, summary, _ := plan(t, map[string]string{"git": "#!/bin/sh\ncase \"$*\" in 'diff --name-only -z'*) printf '.github/forward-rows/narrow.sh\\0' ;; 'diff --quiet'*) exit 0 ;; *) exit 1 ;; esac\n"})
		if count != "16" || matrix != full16 || !strings.Contains(summary, "the trusted decision is FULL") {
			t.Fatalf("count %s matrix %s\n%s", count, matrix, summary)
		}
	})
	sizing := func(body string) map[string]string {
		return map[string]string{"ci-shards": "#!/bin/sh\ncase \"$*\" in *--size-budget-ms*) " + body + " ;; *--select*--slices*) echo '-run ^(TestX)$ " + c + "' ;; *--select*) echo " + b + " ;; *--slices*) echo '-run ^(TestX)$ " + c + "' ;; *) printf '%s\\n' " + a + " " + d + " ;; esac\n"}
	}
	for _, tc := range []struct{ name, body, want string }{
		{"a sizing failure", "echo refused >&2; exit 2", "ci-shards could not size the selection: refused"},
		{"a count above the full matrix", "echo 17", "ci-shards sized '17'"},
		{"a zero count", "echo 0", "ci-shards sized '0'"},
		{"a count that is not a number", "echo three", "ci-shards sized 'three'"},
	} {
		t.Run(tc.name+" keeps the full matrix", func(t *testing.T) {
			count, matrix, summary, _ := plan(t, sizing(tc.body))
			if count != "16" || matrix != full16 || !strings.Contains(summary, tc.want) {
				t.Fatalf("count %s matrix %s\n%s", count, matrix, summary)
			}
		})
	}
	t.Run("a floor above the full matrix keeps the full matrix", func(t *testing.T) {
		if count, matrix, _, _ := plan(t, nil, "MIN_SHARDS=20"); count != "16" || matrix != full16 {
			t.Fatalf("count %s matrix %s", count, matrix)
		}
	})
	t.Run("a narrow.sh that writes no record keeps the full matrix", func(t *testing.T) {
		count, matrix, summary, _ := plan(t, map[string]string{"jq": "#!/bin/sh\nexit 1\n"})
		if count != "16" || matrix != full16 || !strings.Contains(summary, "the trusted decision is missing") {
			t.Fatalf("count %s matrix %s\n%s", count, matrix, summary)
		}
	})
}

// TestAFPV02724ShardPlanOverlapsDocsPlan: the shard-plan job needs only narrowing-verdict and reads
// no docs-plan output, so its checkout, builds and narrowing run beside docs-plan instead of after
// it. That is safe only while every job that reads its outputs skips or ignores them on a DOCS or
// REUSE decision: go-product-shard does not start, ci-narrowed-tree-record does not run, and
// go-product reads SHARDS only in the coverage branch that DOCS and REUSE never reach.
func TestAFPV02724ShardPlanOverlapsDocsPlan_AFPV02724(t *testing.T) {
	plan := workflowJob(t, "shard-plan")
	needs := ""
	if m := regexp.MustCompile(`(?m)^    needs: (.*)$`).FindStringSubmatch(plan); m != nil {
		needs = m[1]
	}
	if needs != "[narrowing-verdict]" {
		t.Fatalf("shard-plan needs %q, want [narrowing-verdict] alone so it runs beside docs-plan", needs)
	}
	if strings.Contains(plan, "needs.docs-plan") {
		t.Fatal("shard-plan reads a docs-plan output, which it cannot without waiting for docs-plan")
	}
	data, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, jobs, ok := strings.Cut(string(data), "\njobs:\n")
	if !ok {
		t.Fatal("ci.yml has no jobs: section")
	}
	var readers []string
	for _, m := range regexp.MustCompile(`(?m)^  ([a-zA-Z][a-zA-Z0-9_-]*):\n`).FindAllStringSubmatch(jobs, -1) {
		if m[1] != "shard-plan" && strings.Contains(workflowJob(t, m[1]), "needs.shard-plan.outputs") {
			readers = append(readers, m[1])
		}
	}
	if got := strings.Join(readers, " "); got != "go-product-shard ci-narrowed-tree-record go-product" {
		t.Fatalf("the jobs that read shard-plan's outputs are %q; a new reader must ignore them on DOCS and REUSE, and this test must say how", got)
	}
	ifLine := func(job string) string {
		m := regexp.MustCompile(`(?m)^    if: (.*)$`).FindStringSubmatch(workflowJob(t, job))
		if m == nil {
			t.Fatalf("%s has no job-level if:", job)
		}
		return m[1]
	}
	if s := ifLine("go-product-shard"); !strings.Contains(s, "!(needs.docs-plan.result == 'success' && (needs.docs-plan.outputs.mode == 'DOCS' || needs.docs-plan.outputs.mode == 'REUSE'))") {
		t.Fatalf("go-product-shard must not start on a passed DOCS or REUSE decision: %s", s)
	}
	if s := ifLine("ci-narrowed-tree-record"); !strings.Contains(s, "needs.docs-plan.result == 'success' && needs.docs-plan.outputs.mode != 'DOCS' && needs.docs-plan.outputs.mode != 'REUSE'") {
		t.Fatalf("ci-narrowed-tree-record must not run on a DOCS or REUSE decision: %s", s)
	}
	product := workflowJob(t, "go-product")
	branch := strings.Index(product, "if [ \"$DOCS_MODE\" = DOCS ] || [ \"$DOCS_MODE\" = REUSE ]; then\n              echo \"AFP-V0-591: no shard ran a tests step")
	uses := regexp.MustCompile(`\$\{?SHARDS\}?\b`).FindAllStringIndex(product, -1)
	if branch < 0 || len(uses) == 0 {
		t.Fatalf("go-product's DOCS/REUSE coverage branch (%d) or its SHARDS use (%d) is missing", branch, len(uses))
	}
	for _, u := range uses {
		if u[0] < branch {
			t.Fatalf("go-product reads SHARDS before its DOCS/REUSE coverage branch: %q", product[u[0]:u[1]+40])
		}
	}
}

// TestAFPV02725RowsAcceptTheSelectionRunsShardCount: a queue row whose pull-request selection
// run sized its matrix to four shards is a complete row (its four narrowing records and four
// selection documents cover 0..3 and its outcomes come from the eight-shard queue run), a
// narrowed-out package still counts as a MISS, and the row records the selection run's count. A
// selection run of another count that did not narrow stays INCOMPLETE, as before.
func TestAFPV02725RowsAcceptTheSelectionRunsShardCount_AFPV02725(t *testing.T) {
	const sized = 4
	f := queueFixture(t)
	p := strs(f.sel[0]["selected"])[0]
	fail(f.out[0], p, "fail", "RAN", "pass")
	resize := func(f *fixture, narrowed bool) {
		for k := range f.sel {
			if k >= sized {
				delete(f.sel, k)
				continue
			}
			f.sel[k]["shards"], f.sel[k]["shardCount"] = sized, sized
		}
		f.nar = map[int]doc{}
		for k := 0; k < sized; k++ {
			mode := "FULL"
			var skipped []string
			if narrowed && k == 1 {
				mode, skipped = "NARROWED", []string{p}
			}
			rec := narrowingRecord(k, mode, prRun, nil, skipped)
			rec["shards"] = sized
			f.nar[k] = rec
		}
	}
	resize(f, true)
	row, _ := f.classify(t, nil)
	if row["class"] != "MISS" || num(row["shards"]) != shards || num(row["selectionShards"]) != sized || len(strs(row["incomplete"])) != 0 ||
		fmt.Sprint(strs(row["missed"])) != fmt.Sprint([]string{p}) {
		t.Fatalf("row = %v", row)
	}
	n := sub(sub(row, "selection"), "narrowed")
	if fmt.Sprint(n["shards"]) != "[1]" || num(n["narrowedOut"]) != 1 {
		t.Fatalf("selection.narrowed = %v", n)
	}
	// Coverage is over the sized count: a missing sized record is a gap of 0..3.
	f2 := queueFixture(t)
	resize(f2, true)
	delete(f2.nar, 2)
	delete(f2.sel, 2)
	row, _ = f2.classify(t, nil)
	wantIncomplete(t, row, "selection documents cover shards 0,1,3 of 4")
	// Without a NARROWED record the sized count is not admitted: the documents fail the shape.
	f3 := queueFixture(t)
	resize(f3, false)
	row, _ = f3.classify(t, nil)
	wantIncomplete(t, row, "selection document(s) fail the corvint-pr-selection/0 shape or shard count")
	// A pull-request row of the sized run is NARROWED, with its own four-shard outcomes.
	f4 := prFixture(t)
	resize(f4, true)
	for k := range f4.out {
		if k >= sized {
			delete(f4.out, k)
			continue
		}
		f4.out[k]["shards"] = sized
	}
	row, _ = f4.classify(t, map[string]string{"ROW_KIND": "pr", "ROW_SHA": "", "ROW_RUN": prRun, "ROW_OUTCOMES_RUN": prRun})
	if row["class"] != "NARROWED" || num(row["selectionShards"]) != sized {
		t.Fatalf("pull-request row = %v", row)
	}
}

// TestAFPV03260ShardPlanSizesMergeQueueEntries: a merge-queue entry on main sizes its matrix too.
// shard-plan.sh runs mq-narrow.sh, the queue-range gate every shard runs, when QUEUE_BASE is set and
// narrow.sh otherwise, always as shard 0 of the full matrix and without MQ_DELTA; the job runs for
// a merge-queue entry only under CORVINT_MQ_NARROWING with a MET verdict, takes shard-plan.sh and
// pin-lag.sh from the commit of main the shards take mq-narrow.sh from, and sizes at the queue
// shards' go test -p.
func TestAFPV03260ShardPlanSizesMergeQueueEntries_AFPV03260(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "shard-plan.sh runs its gate under sh")
	gate := func(t *testing.T, env ...string) (ran, count, summary string) {
		t.Helper()
		dir := t.TempDir()
		rows := filepath.Join(dir, "rows")
		body, err := os.ReadFile(script(t, "shard-plan.sh"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(rows, "shard-plan.sh"), body)
		for _, name := range []string{"narrow.sh", "mq-narrow.sh"} {
			writeFile(t, filepath.Join(rows, name), []byte("#!/bin/sh\nprintf '%s shard=%s of %s delta=[%s]\\n' "+name+" \"$SHARD\" \"$SHARDS\" \"${MQ_DELTA-unset}\" >> \"$RUNNER_TEMP/ran\"\n"+
				"mkdir -p \"$RUNNER_TEMP/corvint-narrowing\"\nprintf '{\"mode\":\"FULL\"}' > \"$RUNNER_TEMP/corvint-narrowing/narrowing.json\"\n"))
		}
		writeFile(t, filepath.Join(dir, "output"), nil)
		cmd := exec.Command("sh", filepath.Join(rows, "shard-plan.sh"))
		cmd.Dir = dir
		cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "RUNNER_TEMP=" + dir, "GITHUB_OUTPUT=" + filepath.Join(dir, "output"),
			"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary"), "MQ_DELTA=candidate"}, env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("shard-plan.sh: %v\n%s", err, out)
		}
		r, _ := os.ReadFile(filepath.Join(dir, "ran"))
		s, _ := os.ReadFile(filepath.Join(dir, "summary"))
		return string(r), outputValue(t, dir, "count"), string(s)
	}
	if ran, count, summary := gate(t, "QUEUE_BASE=142f0e3e7bfb153f803fb6c4bad7989f87ee324a"); ran != "mq-narrow.sh shard=0 of 16 delta=[]\n" || count != "16" || !strings.Contains(summary, "the trusted decision is FULL") {
		t.Fatalf("a merge-queue entry ran %q, count %s\n%s", ran, count, summary)
	}
	if ran, _, _ := gate(t); ran != "narrow.sh shard=0 of 16 delta=[]\n" {
		t.Fatalf("a pull request ran %q", ran)
	}

	plan := workflowJob(t, "shard-plan")
	for _, want := range []string{
		"    if: needs.narrowing-verdict.outputs.verdict == 'MET' && ((github.event_name == 'pull_request' && vars.CORVINT_PR_NARROWING == 'on' && github.event.pull_request.head.repo.full_name == github.repository) || (github.event_name == 'merge_group' && vars.CORVINT_MQ_NARROWING == 'on' && github.event.merge_group.base_ref == 'refs/heads/main'))\n",
		"          SHARD_TEST_P: ${{ github.event_name == 'pull_request' && '4' || '1' }}\n",
		"          QUEUE_BASE: ${{ github.event.merge_group.base_sha }}\n",
		"          QUEUE_HEAD_REF: ${{ github.event.merge_group.head_ref }}\n",
		"            git archive \"$scripts\" .github/forward-rows | tar -x -C \"$trusted\"\n",
		"            grep -q 'mq-narrow.sh' \"$trusted/.github/forward-rows/shard-plan.sh\"\n",
		"            sh \"$trusted/.github/forward-rows/pin-lag.sh\" \"$scripts\" \"$CORVINT_PR_TOOL_SOURCE\" \"$CORVINT_TEST_SHADOW_SOURCE\"\n",
		"              sh \"$trusted/.github/forward-rows/shard-plan.sh\"\n",
	} {
		if !strings.Contains(plan, want) {
			t.Fatalf("the shard-plan job lacks %q", want)
		}
	}
	// The scripts commit is the one the shards' gate derives, by the same text.
	shard := workflowJob(t, "go-product-shard")
	for _, line := range []string{
		"            scripts=$QUEUE_BASE\n",
		"            if ! git merge-base --is-ancestor \"$QUEUE_BASE\" refs/remotes/origin/main 2> /dev/null; then\n",
	} {
		if !strings.Contains(plan, line) || !strings.Contains(shard, line) {
			t.Fatalf("shard-plan and go-product-shard disagree on the scripts commit: %q", line)
		}
	}
	if !strings.Contains(shard, "          SHARD_TEST_P: ${{ github.event_name == 'pull_request' && '4' || '1' }}\n") {
		t.Fatal("the shards' go test -p changed; the shard-plan sizing must price the same placement")
	}
}
