// SPDX-License-Identifier: AGPL-3.0-or-later
package forwardrow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The fixtures come from real runs (docs/build-log/2026-10-13-ci-forward-rows.md): merge-queue
// run 38485555207 tested queue commit queueSHA (PR #868, head prHead, tree queueTree) and passed
// on every shard;
// pull-request run 38482995697 is the latest successful run of that head, and the selection
// documents are the shadow planner's dry-run of that PR at the shadow pin. Run 38476173623
// failed shard 7 once on the internal/contextindex flake (V1-1362) and passed on its rerun.
const (
	queueSHA  = "5163d1ed4b61084fc32cf79355848899c3e8a5bb"
	prHead    = "460fc31ae6d0134fa27f7a66336b99db88fc91c6"
	mainBase  = "142f0e3e7bfb153f803fb6c4bad7989f87ee324a"
	queueTree = "728e6e3f9907b941ceef18f92d3cb7117dbe45dd"
	queueRun  = "38485555207"
	prRun     = "38482995697"
	flakyPkg  = "github.com/Beamfall/corvint/internal/contextindex"
	shards    = 8
)

type doc = map[string]any

func script(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("../../.github/forward-rows", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// run executes NAME with only PATH, HOME and ENV, so a CI runner's GITHUB_* variables never
// reach a fixture run.
func run(t *testing.T, env []string, name string, args ...string) string {
	t.Helper()
	for _, tool := range []string{"sh", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the forward-row scripts need %s on PATH: %v", tool, err)
		}
	}
	cmd := exec.Command(name, args...)
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}, env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, stderr.String())
	}
	return string(out)
}

func readDoc(t *testing.T, path string) doc {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d doc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return d
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDoc(t *testing.T, path string, d any) {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, append(b, '\n'))
}

func lines(s ...string) []byte { return []byte(strings.Join(s, "\n") + "\n") }

func strs(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func num(v any) int { return int(v.(float64)) }

func sub(d doc, key string) doc { return d[key].(doc) }

// fixture is one run's forward-row inputs, written as the gh run download trees the
// forward-rows job classifies.
type fixture struct {
	sel    map[int]doc
	out    map[int]doc
	rawOut map[int]string
	reuse  doc
	event  string
}

func queueFixture(t *testing.T) *fixture {
	f := &fixture{sel: map[int]doc{}, out: map[int]doc{}, rawOut: map[int]string{}, event: "merge_group"}
	for k := 0; k < shards; k++ {
		f.sel[k] = readDoc(t, filepath.Join("testdata", "selection", fmt.Sprintf("shard-%d.json", k)))
		f.out[k] = readDoc(t, filepath.Join("testdata", "outcomes", fmt.Sprintf("shard-%d.json", k)))
	}
	return f
}

// prFixture is the same evidence as the pull-request run's own outcomes: a shadow row.
func prFixture(t *testing.T) *fixture {
	f := queueFixture(t)
	f.event = "pull_request"
	for _, d := range f.out {
		d["event"], d["head"], d["runId"] = "pull_request", prHead, json.Number(prRun)
	}
	return f
}

func (f *fixture) write(t *testing.T, dir string) (selDir, outDir string) {
	selDir, outDir = filepath.Join(dir, "selection"), filepath.Join(dir, "outcomes")
	for k, d := range f.sel {
		writeDoc(t, filepath.Join(selDir, fmt.Sprintf("corvint-test-shadow-%d", k), "selection.json"), d)
	}
	for k, d := range f.out {
		writeDoc(t, filepath.Join(outDir, fmt.Sprintf("ci-outcomes-%s-%d", f.event, k), "outcomes.json"), d)
	}
	for k, raw := range f.rawOut {
		writeFile(t, filepath.Join(outDir, fmt.Sprintf("ci-outcomes-%s-%d", f.event, k), "outcomes.json"), []byte(raw))
	}
	return selDir, outDir
}

func (f *fixture) classify(t *testing.T, override map[string]string) (doc, []byte) {
	t.Helper()
	dir := t.TempDir()
	selDir, outDir := f.write(t, dir)
	env := map[string]string{
		"ROW_KIND": "queue", "ROW_PR": "868", "ROW_SHA": queueSHA, "ROW_HEAD": prHead,
		"ROW_RUN": queueRun, "ROW_SELECTION_RUN": prRun, "ROW_OUTCOMES_RUN": queueRun,
		"ROW_SHARDS": strconv.Itoa(shards), "ROW_EPOCH": "1",
		"ROW_SELECTION_DIR": selDir, "ROW_OUTCOMES_DIR": outDir,
	}
	if f.reuse != nil {
		env["ROW_REUSE"] = filepath.Join(dir, "decision.json")
		writeDoc(t, env["ROW_REUSE"], f.reuse)
	}
	for k, v := range override {
		env[k] = v
	}
	var list []string
	for k, v := range env {
		list = append(list, k+"="+v)
	}
	out := filepath.Join(dir, "row.json")
	class := strings.TrimSpace(run(t, list, "sh", script(t, "classify.sh"), out))
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	row := readDoc(t, out)
	if row["class"] != class {
		t.Fatalf("classify.sh printed %s, row class %v", class, row["class"])
	}
	return row, raw
}

// fail marks PKG failed in outcomes D and records its single rerun.
func fail(d doc, pkg, outcome, state, rerun string) {
	sub(d, "outcomes")[pkg] = outcome
	d["status"] = 1
	r := sub(d, "rerun")
	r["state"], r["invocations"] = state, 1
	if rerun != "" {
		r["outcomes"] = doc{pkg: rerun}
	}
}

func unselect(d doc, pkg string) {
	var kept []any
	for _, p := range d["selected"].([]any) {
		if p != pkg {
			kept = append(kept, p)
		}
	}
	d["selected"] = kept
}

func wantIncomplete(t *testing.T, row doc, fragment string) {
	t.Helper()
	if row["class"] != "INCOMPLETE" {
		t.Fatalf("class = %v, want INCOMPLETE (%q)", row["class"], fragment)
	}
	for _, r := range strs(row["incomplete"]) {
		if strings.Contains(r, fragment) {
			return
		}
	}
	t.Fatalf("incomplete = %q, want a reason containing %q", row["incomplete"], fragment)
}

var rowKeys = []string{"class", "counts", "epoch", "failed", "flaky", "head", "incomplete", "infrastructure", "kind", "missed",
	"outcomesRunId", "outcomesSource", "pr", "profile", "reruns", "runId", "selectedFailedInQueue", "selection",
	"selectionRunId", "sha", "shards", "truncated"}

func TestAFPV0029ForwardRowsAndShadowBar(t *testing.T) {
	t.Run("CLEAN on the real queue run, with the shell gap as a selection limit", func(t *testing.T) {
		row, _ := queueFixture(t).classify(t, nil)
		if row["class"] != "CLEAN" || len(strs(row["incomplete"])) != 0 {
			t.Fatalf("row = %v, want CLEAN", row)
		}
		var keys []string
		for k := range row {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if fmt.Sprint(keys) != fmt.Sprint(rowKeys) {
			t.Fatalf("row keys = %v, want the closed corvint-pr-forward-row/0 set %v", keys, rowKeys)
		}
		sel := sub(row, "selection")
		if num(sel["selected"]) != 192 || sel["scope"] != "NARROWED" || row["outcomesSource"] != "merge_group" ||
			row["sha"] != queueSHA || row["profile"] != "corvint-pr-forward-row/0" {
			t.Fatalf("row = %v", row)
		}
		if l := strs(sel["limits"]); len(l) != 1 || !strings.HasPrefix(l[0], "shell: no qualified adapter and ") {
			t.Fatalf("limits = %q, want the shell adapter gap only", l)
		}
		if np := strs(sel["notProduced"]); len(np) != 1 || !strings.HasPrefix(np[0], "selectionDigest: ") {
			t.Fatalf("notProduced = %q", np)
		}
		if !reflect.DeepEqual(row["reruns"], map[string]any{"NOT_NEEDED": float64(shards)}) {
			t.Fatalf("reruns = %v", row["reruns"])
		}
	})
	t.Run("a selected package that fails in the queue is not a miss", func(t *testing.T) {
		f := queueFixture(t)
		fail(f.out[7], flakyPkg, "fail", "RAN", "fail")
		row, _ := f.classify(t, nil)
		c := sub(row, "counts")
		if row["class"] != "CLEAN" || num(c["failed"]) != 1 || num(c["missed"]) != 0 || num(c["selectedFailedInQueue"]) != 1 {
			t.Fatalf("row = %v, want CLEAN with one SELECTED_FAILED_IN_QUEUE", row)
		}
		if got := strs(row["selectedFailedInQueue"]); len(got) != 1 || got[0] != flakyPkg {
			t.Fatalf("selectedFailedInQueue = %q", got)
		}
	})
	t.Run("FLAKY: a missed package passes its rerun", func(t *testing.T) {
		f := queueFixture(t)
		unselect(f.sel[7], flakyPkg)
		fail(f.out[7], flakyPkg, "fail", "RAN", "pass")
		row, _ := f.classify(t, nil)
		if row["class"] != "FLAKY" || fmt.Sprint(strs(row["missed"])) != fmt.Sprint([]string{flakyPkg}) ||
			fmt.Sprint(strs(row["flaky"])) != fmt.Sprint([]string{flakyPkg}) {
			t.Fatalf("row = %v, want FLAKY", row)
		}
	})
	for _, tc := range []struct{ name, outcome, state, rerun string }{
		{"MISS: a missed package fails its rerun", "fail", "RAN", "fail"},
		{"MISS: a missed package's rerun was bounded", "fail", "SKIPPED_BOUND", ""},
		{"MISS: a missed package's rerun was interrupted", "fail", "INTERRUPTED", ""},
		{"MISS: a missed package reported no terminal event", "missing", "RAN", "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := queueFixture(t)
			unselect(f.sel[7], flakyPkg)
			fail(f.out[7], flakyPkg, tc.outcome, tc.state, tc.rerun)
			row, _ := f.classify(t, nil)
			if row["class"] != "MISS" || len(strs(row["flaky"])) != 0 || num(sub(row, "counts")["missed"]) != 1 {
				t.Fatalf("row = %v, want MISS", row)
			}
		})
	}
	t.Run("MISS outranks FLAKY", func(t *testing.T) {
		f := queueFixture(t)
		unselect(f.sel[7], flakyPkg)
		fail(f.out[7], flakyPkg, "fail", "RAN", "pass")
		other := "github.com/Beamfall/corvint/benchmarks/dogfood-measure"
		unselect(f.sel[0], other)
		fail(f.out[0], other, "fail", "RAN", "fail")
		if row, _ := f.classify(t, nil); row["class"] != "MISS" {
			t.Fatalf("row = %v, want MISS", row)
		}
	})

	incomplete := []struct {
		name, fragment string
		mutate         func(*fixture)
		env            map[string]string
	}{
		{"missing selection artifact", "selection documents cover shards 0,1,2,4,5,6,7 of 8",
			func(f *fixture) { delete(f.sel, 3) }, nil},
		{"no selection at all", "selection documents cover shards  of 8",
			func(f *fixture) { f.sel = map[int]doc{} }, nil},
		{"missing shard outcomes", "outcomes documents cover shards 0,1,2,3,4,6,7 of 8",
			func(f *fixture) { delete(f.out, 5) }, nil},
		{"shard-count mismatch", "fail the corvint-ci-outcomes/0 shape or shard count",
			func(f *fixture) { f.out[2]["shards"] = 4 }, nil},
		{"row shard count differs from the documents", "fail the corvint-pr-selection/0 shape or shard count",
			nil, map[string]string{"ROW_SHARDS": "7"}},
		{"unreadable outcomes artifact", "outcomes artifact ci-outcomes-merge_group-1 is unreadable",
			func(f *fixture) { delete(f.out, 1); f.rawOut[1] = "{\"profile\": " }, nil},
		{"an unknown outcome value", "fail the corvint-ci-outcomes/0 shape",
			func(f *fixture) { sub(f.out[6], "outcomes")[flakyPkg] = "flaky" }, nil},
		{"a Go gap in the selection", "selection incomplete: go: ",
			func(f *fixture) {
				f.sel[0]["incomplete"] = append(f.sel[0]["incomplete"].([]any), "go: package list unreadable")
			}, nil},
		{"a non-Go gap that is not an adapter limit", "selection incomplete: the dry-run printed no plan",
			func(f *fixture) { f.sel[4]["incomplete"] = []any{"the dry-run printed no plan"} }, nil},
		{"a failed shard with no failed package", "outcomes shard 4 exited 1 with no failed or missing package",
			func(f *fixture) { f.out[4]["status"] = 1 }, nil},
		{"selection of another head", "selection shard 2 planned head " + queueSHA,
			func(f *fixture) { f.sel[2]["head"] = queueSHA }, nil},
		{"selection of another run", "selection shard 2 is run 1, want " + prRun,
			func(f *fixture) { f.sel[2]["runId"] = 1 }, nil},
		{"dry-run failure", "selection shard 5: corvint test --dry-run exited 2",
			func(f *fixture) { f.sel[5]["exit"] = 2 }, nil},
		{"manifest digests disagree", "selection shards disagree on the manifest digest",
			func(f *fixture) { f.sel[6]["manifestDigest"] = "sha256:" + strings.Repeat("0", 64) }, nil},
		{"selections without digests", "8 selection document(s) fail the corvint-pr-selection/0 shape",
			func(f *fixture) {
				for k := range f.sel {
					delete(f.sel[k], "manifestDigest")
					delete(f.sel[k], "planDigest")
				}
			}, nil},
		{"selection without tool source", "1 selection document(s) fail the corvint-pr-selection/0 shape",
			func(f *fixture) { f.sel[2]["toolSource"] = nil }, nil},
		{"outcomes without status", "1 outcomes document(s) fail the corvint-ci-outcomes/0 shape",
			func(f *fixture) { delete(f.out[4], "status") }, nil},
		{"outcomes of another commit", "outcomes shard 6 is merge_group of " + prHead,
			func(f *fixture) { f.out[6]["target"] = prHead }, nil},
		{"collector reason", "no successful pull_request run", nil,
			map[string]string{"ROW_COLLECT": "no successful pull_request run of head " + prHead + " is readable, so no selection\n"}},
		{"pull-request row of queue outcomes", "want pull_request of " + prHead, nil,
			map[string]string{"ROW_KIND": "pr", "ROW_SHA": "", "ROW_RUN": prRun, "ROW_OUTCOMES_RUN": queueRun}},
		{"a non-numeric pull request", "pull request number x is not a number", nil, map[string]string{"ROW_PR": "x"}},
	}
	for _, tc := range incomplete {
		t.Run("INCOMPLETE: "+tc.name, func(t *testing.T) {
			f := queueFixture(t)
			if tc.mutate != nil {
				tc.mutate(f)
			}
			row, _ := f.classify(t, tc.env)
			wantIncomplete(t, row, tc.fragment)
		})
	}
	t.Run("INCOMPLETE outranks a failure", func(t *testing.T) {
		f := queueFixture(t)
		unselect(f.sel[7], flakyPkg)
		fail(f.out[7], flakyPkg, "fail", "RAN", "fail")
		delete(f.sel, 0)
		row, _ := f.classify(t, nil)
		wantIncomplete(t, row, "selection documents cover")
		if num(sub(row, "counts")["missed"]) != 1 {
			t.Fatalf("counts = %v, want the miss still counted", row["counts"])
		}
	})

	// AFP-V0-052 (V1-1514): merge-queue run 38686348902 lost go-product-shard (5) at the hosted
	// "Set up job" step, so the shard retained no outcomes. The hosted failure is INFRA only when
	// it explains every gap of the row.
	infraReason := "infrastructure: go-product-shard (5) of run " + queueRun + " failed at Set up job"
	t.Run("INFRA: a shard lost at Set up job (run 38686348902)", func(t *testing.T) {
		f := queueFixture(t)
		delete(f.out, 5)
		row, _ := f.classify(t, map[string]string{"ROW_INFRA": "5\n"})
		if row["class"] != "INFRA" || fmt.Sprint(strs(row["infrastructure"])) != fmt.Sprint([]string{infraReason}) {
			t.Fatalf("row = %v, want INFRA naming shard 5", row)
		}
		if got := strs(row["incomplete"]); len(got) != 2 || got[0] != "outcomes documents cover shards 0,1,2,3,4,6,7 of 8, want each of 0..7 once" ||
			got[1] != infraReason {
			t.Fatalf("incomplete = %q, want the coverage gap and its hosted cause", got)
		}
	})
	t.Run("INFRA: a pull-request row loses the shard's selection too", func(t *testing.T) {
		f := prFixture(t)
		delete(f.out, 5)
		delete(f.sel, 5)
		row, _ := f.classify(t, map[string]string{
			"ROW_KIND": "pr", "ROW_SHA": "", "ROW_RUN": prRun, "ROW_OUTCOMES_RUN": prRun, "ROW_INFRA": "5"})
		if row["class"] != "INFRA" || len(strs(row["incomplete"])) != 3 {
			t.Fatalf("row = %v, want INFRA", row)
		}
	})
	t.Run("no hosted failure named: the lost shard stays INCOMPLETE", func(t *testing.T) {
		f := queueFixture(t)
		delete(f.out, 5)
		row, _ := f.classify(t, nil)
		wantIncomplete(t, row, "outcomes documents cover shards 0,1,2,3,4,6,7 of 8")
		if len(strs(row["infrastructure"])) != 0 {
			t.Fatalf("infrastructure = %v, want none", row["infrastructure"])
		}
	})
	for _, tc := range []struct {
		name, fragment, infra string
		mutate                func(*fixture)
		env                   map[string]string
	}{
		{"another shard is also missing", "outcomes documents cover shards 0,1,2,4,6,7 of 8", "5",
			func(f *fixture) { delete(f.out, 5); delete(f.out, 3) }, nil},
		{"the named shard retained outcomes", infraReason, "5", nil, nil},
		{"a queue row's selection misses the shard", "selection documents cover shards 0,1,2,3,4,6,7 of 8", "5",
			func(f *fixture) { delete(f.out, 5); delete(f.sel, 5) }, nil},
		{"a collector reason beside it", "no successful pull_request run", "5",
			func(f *fixture) { delete(f.out, 5) },
			map[string]string{"ROW_COLLECT": "no successful pull_request run of head " + prHead + " is readable, so no selection\n"}},
		{"a shard beyond the count", "go-product-shard (9) of run", "9", nil, nil},
		{"a non-FLAKY miss on the shards that ran", infraReason, "5",
			func(f *fixture) {
				delete(f.out, 5)
				unselect(f.sel[7], flakyPkg)
				fail(f.out[7], flakyPkg, "fail", "RAN", "fail")
			}, nil},
	} {
		t.Run("INCOMPLETE beside a hosted failure: "+tc.name, func(t *testing.T) {
			f := queueFixture(t)
			if tc.mutate != nil {
				tc.mutate(f)
			}
			env := map[string]string{"ROW_INFRA": tc.infra}
			for k, v := range tc.env {
				env[k] = v
			}
			row, _ := f.classify(t, env)
			wantIncomplete(t, row, tc.fragment)
		})
	}
	t.Run("INFRA keeps a FLAKY package on the shards that ran", func(t *testing.T) {
		f := queueFixture(t)
		delete(f.out, 5)
		unselect(f.sel[7], flakyPkg)
		fail(f.out[7], flakyPkg, "fail", "RAN", "pass")
		row, _ := f.classify(t, map[string]string{"ROW_INFRA": "5"})
		if row["class"] != "INFRA" || num(sub(row, "counts")["flaky"]) != 1 {
			t.Fatalf("row = %v, want INFRA with the flaky package counted", row)
		}
	})

	t.Run("REUSE takes the outcomes of the reused run", func(t *testing.T) {
		f := queueFixture(t)
		f.reuse = readDoc(t, filepath.Join("testdata", "reuse", "decision-push38487301206.json"))
		reuseQueue := strings.Repeat("e", 40)
		row, _ := f.classify(t, map[string]string{"ROW_SHA": reuseQueue, "ROW_RUN": "38487301206"})
		if row["class"] != "CLEAN" || row["outcomesSource"] != "reuse" || row["sha"] != reuseQueue ||
			num(row["outcomesRunId"]) != 38485555207 {
			t.Fatalf("row = %v, want a CLEAN reuse row", row)
		}
		f.reuse["tree"] = strings.Repeat("0", 40)
		row, _ = f.classify(t, map[string]string{"ROW_SHA": reuseQueue, "ROW_RUN": "38487301206"})
		wantIncomplete(t, row, "is not run 38485555207 of tree "+strings.Repeat("0", 40))
	})
	t.Run("a pull-request shadow row names its tested merge", func(t *testing.T) {
		row, _ := prFixture(t).classify(t, map[string]string{
			"ROW_KIND": "pr", "ROW_SHA": "", "ROW_RUN": prRun, "ROW_OUTCOMES_RUN": prRun})
		if row["class"] != "CLEAN" || row["kind"] != "pr" || row["sha"] != queueSHA || row["outcomesSource"] != "pull_request" {
			t.Fatalf("row = %v, want a CLEAN pull-request row of %s", row, queueSHA)
		}
	})
	t.Run("the row stays within 64 KiB", func(t *testing.T) {
		f := queueFixture(t)
		o := f.out[0]
		for i := 0; i < 3000; i++ {
			sub(o, "outcomes")[fmt.Sprintf("example.com/a/rather/long/module/path/never/selected/by/the/plan/pkg%04d", i)] = "fail"
		}
		o["status"] = 1
		sub(o, "rerun")["state"], sub(o, "rerun")["invocations"] = "SKIPPED_BOUND", 3000
		f.sel[3]["incomplete"] = []any{strings.Repeat("go: an overlong gap ", 100)}
		row, raw := f.classify(t, nil)
		if len(raw) > 65536 || row["truncated"] != true || num(sub(row, "counts")["failed"]) != 3000 {
			t.Fatalf("row is %d bytes, truncated %v, counts %v", len(raw), row["truncated"], row["counts"])
		}
		if len(strs(row["failed"])) >= 3000 || row["class"] != "INCOMPLETE" {
			t.Fatalf("row lists %d failures, class %v", len(strs(row["failed"])), row["class"])
		}
		for _, r := range strs(row["incomplete"]) {
			if len(r) > 303 {
				t.Fatalf("incomplete reason of %d bytes is not shortened", len(r))
			}
		}
	})
}

func outcomesEnv(event, run, target, head, tree, shard string) []string {
	return []string{"GITHUB_EVENT_NAME=" + event, "GITHUB_RUN_ID=" + run, "GITHUB_RUN_ATTEMPT=1",
		"TESTED_MERGE=" + target, "PR_HEAD=" + head, "OUTCOMES_TREE=" + tree, "SHARD=" + shard, "SHARDS=8"}
}

func TestAFPV0047ShardOutcomes(t *testing.T) {
	t.Run("the real flaky shard reruns its failed package once", func(t *testing.T) {
		dir := t.TempDir()
		stream, err := filepath.Abs(filepath.Join("testdata", "streams", "pr38476173623-shard7-attempt1.json"))
		if err != nil {
			t.Fatal(err)
		}
		// expected: each package-level start of the stream (one per go test invocation).
		starts := run(t, nil, "jq", "-R", "-r", `fromjson? | select(.Action == "start" and .Test == null) | .Package`, stream)
		writeFile(t, filepath.Join(dir, "expected"), []byte(starts))
		writeFile(t, filepath.Join(dir, "whole"), []byte(starts))
		writeFile(t, filepath.Join(dir, "failed-slices"), nil)
		env := outcomesEnv("pull_request", "38476173623", "406dce90998aa2f4fb52613ef099267dfd5c2a60",
			"302f258fadd17989eebb12bdbb8aaeff34e15901", "b696b9721fad342027e126ae0047162c2895f9c7", "7")
		run(t, env, "sh", script(t, "outcomes.sh"), "first", dir, "1", stream)
		d := readDoc(t, filepath.Join(dir, "outcomes.json"))
		o := sub(d, "outcomes")
		if len(o) != 40 || o[flakyPkg] != "fail" || d["profile"] != "corvint-ci-outcomes/0" || num(d["status"]) != 1 {
			t.Fatalf("outcomes = %v", d)
		}
		for p, v := range o {
			if p != flakyPkg && v != "pass" && v != "skip" {
				t.Fatalf("%s = %v, want only %s failing", p, v, flakyPkg)
			}
		}
		if r := sub(d, "rerun"); r["state"] != "INTERRUPTED" || num(r["invocations"]) != 1 || num(r["bound"]) != 8 {
			t.Fatalf("rerun = %v, want INTERRUPTED before the rerun", r)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, "rerun-lines")); string(b) != flakyPkg+"\n" {
			t.Fatalf("rerun-lines = %q", b)
		}
		rerun, err := filepath.Abs(filepath.Join("testdata", "streams", "pr38476173623-shard7-attempt2-contextindex.json"))
		if err != nil {
			t.Fatal(err)
		}
		run(t, env, "sh", script(t, "outcomes.sh"), "rerun", dir, rerun)
		d = readDoc(t, filepath.Join(dir, "outcomes.json"))
		if r := sub(d, "rerun"); r["state"] != "RAN" || !reflect.DeepEqual(r["outcomes"], map[string]any{flakyPkg: "pass"}) {
			t.Fatalf("rerun = %v, want RAN with %s passing", r, flakyPkg)
		}
		if sub(d, "outcomes")[flakyPkg] != "fail" || num(d["status"]) != 1 {
			t.Fatalf("the rerun changed the first outcome or status: %v", d)
		}
	})

	event := func(pkg, action string) string {
		return fmt.Sprintf(`{"Action":%q,"Package":%q}`, action, pkg)
	}
	first := func(t *testing.T, status string, expected, whole, slices []string, stream ...string) (doc, string) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "expected"), lines(expected...))
		writeFile(t, filepath.Join(dir, "whole"), lines(whole...))
		writeFile(t, filepath.Join(dir, "failed-slices"), []byte(strings.Join(append(slices, ""), "\n")))
		if len(slices) == 0 {
			writeFile(t, filepath.Join(dir, "failed-slices"), nil)
		}
		writeFile(t, filepath.Join(dir, "stream"), lines(stream...))
		run(t, outcomesEnv("merge_group", "1", queueSHA, "", queueTree, "0"), "sh", script(t, "outcomes.sh"),
			"first", dir, status, filepath.Join(dir, "stream"))
		b, _ := os.ReadFile(filepath.Join(dir, "rerun-lines"))
		return readDoc(t, filepath.Join(dir, "outcomes.json")), string(b)
	}
	t.Run("a passing shard needs no rerun", func(t *testing.T) {
		d, rl := first(t, "0", []string{"a"}, []string{"a"}, nil, event("a", "start"), event("a", "pass"))
		if sub(d, "rerun")["state"] != "NOT_NEEDED" || rl != "" || d["head"] != nil || d["tree"] != queueTree {
			t.Fatalf("doc = %v, rerun-lines %q", d, rl)
		}
	})
	t.Run("missing packages and failed slices are rerun, passing slices are not", func(t *testing.T) {
		d, rl := first(t, "1", []string{"a", "b", "s", "s"}, []string{"a", "b"}, []string{"-run ^TestTwo$ s"},
			event("a", "pass"), event("s", "pass"), event("s", "fail"), `{"Action":"fail","Package":"a","Test":"TestX"}`)
		o := sub(d, "outcomes")
		if o["a"] != "pass" || o["b"] != "missing" || o["s"] != "fail" {
			t.Fatalf("outcomes = %v", o)
		}
		if rl != "b\n-run ^TestTwo$ s\n" || sub(d, "rerun")["state"] != "INTERRUPTED" || num(sub(d, "rerun")["invocations"]) != 2 {
			t.Fatalf("rerun-lines = %q, rerun %v", rl, d["rerun"])
		}
	})
	t.Run("a slice reporting fewer terminal events than invocations is missing", func(t *testing.T) {
		d, _ := first(t, "1", []string{"s", "s"}, nil, []string{"-run ^TestTwo$ s"}, event("s", "pass"))
		if sub(d, "outcomes")["s"] != "missing" {
			t.Fatalf("outcomes = %v", d["outcomes"])
		}
	})
	t.Run("more than eight failed invocations skip the rerun", func(t *testing.T) {
		var pkgs, stream []string
		for i := 0; i < 9; i++ {
			p := fmt.Sprintf("p%d", i)
			pkgs = append(pkgs, p)
			stream = append(stream, event(p, "fail"))
		}
		d, rl := first(t, "1", pkgs, pkgs, nil, stream...)
		if r := sub(d, "rerun"); r["state"] != "SKIPPED_BOUND" || num(r["invocations"]) != 9 || rl != "" {
			t.Fatalf("rerun = %v, rerun-lines %q", r, rl)
		}
	})
}

func TestAFPV0047SelectionDocument(t *testing.T) {
	args := func(dry, manifest string, want doc) []string {
		a := []string{"-n", "-c", "--slurpfile", "dry", dry, "--slurpfile", "manifest", manifest}
		for _, k := range []string{"toolSource", "toolDigest", "head", "base", "target", "tree"} {
			a = append(a, "--arg", k, want[k].(string))
		}
		for _, k := range []string{"pr", "runId", "runAttempt", "shard", "shards", "exit"} {
			a = append(a, "--arg", k, strconv.Itoa(num(want[k])))
		}
		return append(a, "-f", script(t, "selection.jq"))
	}
	want := readDoc(t, filepath.Join("testdata", "selection", "shard-3.json"))
	got := run(t, nil, "jq", args(filepath.Join("testdata", "shadow", "dry-run-3.json"),
		filepath.Join("testdata", "shadow", "manifest-3.json"), want)...)
	var d doc
	if err := json.Unmarshal([]byte(got), &d); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("selection.jq of the shard 3 dry-run = %v, want the fixture %v", d, want)
	}
	if num(d["manifestUnits"]) != 193 || len(d["selected"].([]any)) != 18 || d["toolSource"] != queueSHA {
		t.Fatalf("selection = %v", d)
	}
	// A dry-run that printed nothing names that gap; it is never a clean selection.
	empty := filepath.Join(t.TempDir(), "empty.json")
	writeFile(t, empty, nil)
	got = run(t, nil, "jq", args(empty, empty, want)...)
	d = nil
	if err := json.Unmarshal([]byte(got), &d); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(d["incomplete"]) != "[the dry-run printed no plan]" || len(d["selected"].([]any)) != 0 ||
		d["manifestDigest"] != nil || d["manifestUnits"] != nil {
		t.Fatalf("empty dry-run selection = %v", d)
	}
}

// fakeGH writes a gh stand-in that answers collect.sh and bar.sh from DIR: the commit,
// compare, run-jobs and repository-artifact `api` paths run the script's own --jq filter over
// API-shaped JSON, the other paths map to canned (already filtered) outputs, and
// `run download` copies the matching artifact directories of DIR/runs/<id>.
func fakeGH(t *testing.T, dir string) string {
	bin := filepath.Join(dir, "bin")
	writeFile(t, filepath.Join(bin, "gh"), []byte(`#!/bin/sh
set -eu
d=`+dir+`
if [ "$1" = api ]; then
  case "$2" in
  */commits/*) [ "$3" = --jq ]; jq -r "$4" "$d/commit.json" ;;
  */compare/*) [ "$3" = --jq ]; jq -r "$4" "$d/compare.json" ;;
  */actions/runs/*/jobs*) id=${2#*/actions/runs/}; id=${id%%/*}; [ "$3" = --jq ]; jq -r "$4" "$d/jobs-$id.json" ;;
  */actions/runs/*/artifacts*) id=${2#*/actions/runs/}; id=${id%%/*}; cat "$d/names-$id" ;;
  */workflows/ci.yml/runs*) cat "$d/selection-run" ;;
  */actions/artifacts*) [ "$3" = --jq ]; case "$2" in *page=1) jq -r "$4" "$d/all-artifacts.json" ;; esac ;;
  *) exit 1 ;;
  esac
  exit 0
fi
[ "$1 $2" = "run download" ] || exit 1
id=$3 pattern="" out=""
shift 3
while [ $# -gt 0 ]; do
  case "$1" in
  --pattern) pattern=$2; shift ;;
  --dir) out=$2; shift ;;
  esac
  shift
done
found=0
for a in "$d/runs/$id"/*; do
  [ -d "$a" ] || continue
  case "${a##*/}" in
  $pattern) mkdir -p "$out"; cp -R "$a" "$out/"; found=1 ;;
  esac
done
[ "$found" = 1 ]
`))
	if err := os.Chmod(filepath.Join(bin, "gh"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestAFPV0047CollectorNamesRows(t *testing.T) {
	type result struct{ name, summary string }
	// changed is the compare API's file list of the run's change set.
	changed := []any{doc{"filename": "internal/contextindex/index.go"}}
	// jobs is the run-jobs API response served for the outcomes run, or nil for an unreadable list.
	var jobs []byte
	collect := func(t *testing.T, f *fixture, runID, event, branch, pr string, reuse doc) result {
		dir := t.TempDir()
		gh := fakeGH(t, dir)
		writeDoc(t, filepath.Join(dir, "commit.json"), doc{"parents": []any{doc{"sha": mainBase}, doc{"sha": prHead}}})
		writeDoc(t, filepath.Join(dir, "compare.json"), doc{"files": changed})
		writeFile(t, filepath.Join(dir, "selection-run"), lines(prRun))
		selDir, outDir := f.write(t, filepath.Join(dir, "src"))
		names := []string{}
		outRun := runID
		if reuse != nil {
			writeDoc(t, filepath.Join(dir, "runs", runID, "ci-reuse", "decision.json"), reuse)
			names = append(names, "ci-reuse")
			if reuse["mode"] == "REUSE" {
				outRun = strconv.Itoa(num(reuse["runId"]))
			}
		}
		entries, _ := os.ReadDir(outDir)
		for _, e := range entries {
			names = append(names, e.Name())
			if err := os.MkdirAll(filepath.Join(dir, "runs", outRun), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(outDir, e.Name()), filepath.Join(dir, "runs", outRun, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
		entries, _ = os.ReadDir(selDir)
		for _, e := range entries {
			if event == "pull_request" {
				names = append(names, e.Name())
			}
			if err := os.MkdirAll(filepath.Join(dir, "runs", prRun), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(selDir, e.Name()), filepath.Join(dir, "runs", prRun, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
		writeFile(t, filepath.Join(dir, "names-"+runID), lines(names...))
		if jobs != nil {
			writeFile(t, filepath.Join(dir, "jobs-"+outRun+".json"), jobs)
		}
		temp, output, summary := filepath.Join(dir, "temp"), filepath.Join(dir, "output"), filepath.Join(dir, "summary")
		writeFile(t, output, nil)
		head := queueSHA
		if event == "pull_request" {
			head = prHead
		}
		cmd := exec.Command("sh", script(t, "collect.sh"))
		cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
			"GH_TOKEN=x", "REPO=beamfall/corvint", "RUN_ID=" + runID, "EVENT=" + event, "HEAD_SHA=" + head,
			"HEAD_BRANCH=" + branch, "PR_NUMBER=" + pr, "PR_BASE_SHA=" + mainBase, "FORWARD_SHARDS=8", "FORWARD_EPOCH=1",
			"RUNNER_TEMP=" + temp, "GITHUB_OUTPUT=" + output, "GITHUB_STEP_SUMMARY=" + summary}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("collect.sh: %v\n%s", err, out)
		}
		o, _ := os.ReadFile(output)
		s, _ := os.ReadFile(summary)
		return result{strings.TrimPrefix(strings.TrimSpace(string(o)), "name="), string(s)}
	}
	queueBranch := "gh-readonly-queue/main/pr-868-142f0e3e7bfb153f803fb6c4bad7989f87ee324a"
	full := readDoc(t, filepath.Join("testdata", "reuse", "decision-push38487301206.json"))
	full["mode"], full["runId"], full["records"] = "FULL", nil, []any{}

	if r := collect(t, queueFixture(t), queueRun, "merge_group", queueBranch, "", full); r.name !=
		"ci-forward-CLEAN-pr868-"+queueSHA+"-e1-f0" {
		t.Fatalf("queue row = %q\n%s", r.name, r.summary)
	}
	f := queueFixture(t)
	unselect(f.sel[7], flakyPkg)
	fail(f.out[7], flakyPkg, "fail", "RAN", "pass")
	if r := collect(t, f, queueRun, "merge_group", queueBranch, "", full); r.name !=
		"ci-forward-FLAKY-pr868-"+queueSHA+"-e1-f1" {
		t.Fatalf("flaky queue row = %q\n%s", r.name, r.summary)
	}
	reuse := readDoc(t, filepath.Join("testdata", "reuse", "decision-push38487301206.json"))
	if r := collect(t, queueFixture(t), "99", "merge_group", queueBranch, "", reuse); r.name !=
		"ci-forward-CLEAN-pr868-"+queueSHA+"-e1-f0" || !strings.Contains(r.summary, "outcomes from run 38485555207") {
		t.Fatalf("reuse queue row = %q\n%s", r.name, r.summary)
	}
	f = queueFixture(t)
	f.out = map[int]doc{}
	if r := collect(t, f, queueRun, "merge_group", queueBranch, "", nil); r.name !=
		"ci-forward-INCOMPLETE-pr868-"+queueSHA+"-e1-f0" || !strings.Contains(r.summary, "retained no ci-outcomes artifact") {
		t.Fatalf("queue row without outcomes = %q\n%s", r.name, r.summary)
	}
	if r := collect(t, prFixture(t), prRun, "pull_request", "claude/x", "868", nil); r.name !=
		"ci-forward-CLEAN-pr868-"+queueSHA+"-e1-f0" {
		t.Fatalf("pull-request row = %q\n%s", r.name, r.summary)
	}
	// AFP-V0-052: the real job list of run 38686348902, whose go-product-shard (5) failed at
	// "Set up job" while every other shard succeeded, names shard 5 of the outcomes run.
	realJobs, err := os.ReadFile(filepath.Join("testdata", "jobs", "merge_group38686348902.json"))
	if err != nil {
		t.Fatal(err)
	}
	f = queueFixture(t)
	delete(f.out, 5)
	if r := collect(t, f, queueRun, "merge_group", queueBranch, "", full); r.name !=
		"ci-forward-INCOMPLETE-pr868-"+queueSHA+"-e1-f0" || !strings.Contains(r.summary, "jobs of run "+queueRun+" are unreadable") {
		t.Fatalf("queue row of a lost shard without a job list = %q\n%s", r.name, r.summary)
	}
	jobs = realJobs
	f = queueFixture(t)
	delete(f.out, 5)
	if r := collect(t, f, queueRun, "merge_group", queueBranch, "", full); r.name !=
		"ci-forward-INFRA-pr868-"+queueSHA+"-e1-f0" || !strings.Contains(r.summary, "go-product-shard (5) of run "+queueRun) {
		t.Fatalf("queue row of a shard lost at Set up job = %q\n%s", r.name, r.summary)
	}
	if r := collect(t, queueFixture(t), queueRun, "merge_group", queueBranch, "", full); r.name !=
		"ci-forward-INCOMPLETE-pr868-"+queueSHA+"-e1-f0" {
		t.Fatalf("queue row whose named shard retained outcomes = %q\n%s", r.name, r.summary)
	}
	jobs = nil
	empty := &fixture{sel: map[int]doc{}, out: map[int]doc{}, rawOut: map[int]string{}, event: "pull_request"}
	if r := collect(t, empty, prRun, "pull_request", "claude/x", "868", nil); r.name != "" ||
		!strings.Contains(r.summary, "executed no full shard and no shadow plan") {
		t.Fatalf("a DOCS pull-request run produced row %q\n%s", r.name, r.summary)
	}
	if r := collect(t, queueFixture(t), queueRun, "merge_group", "gh-readonly-queue/release/pr-1-x", "", nil); r.name != "" {
		t.Fatalf("a queue run of another branch produced row %q", r.name)
	}

	// A change that touches .github/ ran its own workflow and scripts, and a change set of 300
	// files or more cannot be proved not to: neither run is a row. An unreadable change set
	// is INCOMPLETE.
	for _, files := range [][]any{
		{doc{"filename": "internal/x.go"}, doc{"filename": ".github/workflows/ci.yml"}},
		{doc{"filename": "docs/x.md", "previous_filename": ".github/forward-rows/row.jq"}},
		func() []any {
			many := make([]any, 300)
			for i := range many {
				many[i] = doc{"filename": fmt.Sprintf("docs/f%d.md", i)}
			}
			return many
		}(),
	} {
		changed = files
		if r := collect(t, prFixture(t), prRun, "pull_request", "claude/x", "868", nil); r.name != "" ||
			!strings.Contains(r.summary, "touches .github/ or is too large") {
			t.Fatalf("pull-request row of a change touching .github/ = %q\n%s", r.name, r.summary)
		}
		if r := collect(t, queueFixture(t), queueRun, "merge_group", queueBranch, "", full); r.name != "" {
			t.Fatalf("queue row of a change touching .github/ = %q\n%s", r.name, r.summary)
		}
	}
	changed = nil
	if r := collect(t, prFixture(t), prRun, "pull_request", "claude/x", "868", nil); r.name !=
		"ci-forward-INCOMPLETE-pr868-"+queueSHA+"-e1-f0" || !strings.Contains(r.summary, "is unreadable") {
		t.Fatalf("pull-request row with an unreadable change set = %q\n%s", r.name, r.summary)
	}
}

func TestAFPV0047BarCountsArtifactNames(t *testing.T) {
	name := func(class string, pr int, epoch, f string) string {
		return fmt.Sprintf("ci-forward-%s-pr%d-%040x-e%s-f%s", class, pr, pr, epoch, f)
	}
	bar := func(t *testing.T, names []string) string {
		dir := t.TempDir()
		gh := fakeGH(t, dir)
		artifact := func(name string, expired bool, branch string, headRepo int) doc {
			return doc{"name": name, "expired": expired,
				"workflow_run": doc{"head_branch": branch, "repository_id": 1, "head_repository_id": headRepo}}
		}
		var all []any
		for _, n := range names {
			all = append(all, artifact(n, false, "main", 1))
		}
		// An expired row, a row a pull-request or queue run uploaded itself, a fork's
		// `main` and an unrelated artifact never count.
		all = append(all, artifact(name("MISS", 999, "1", "1"), true, "main", 1),
			artifact(name("MISS", 998, "1", "1"), false, "claude/x", 1),
			artifact(name("MISS", 997, "1", "1"), false, "gh-readonly-queue/main/pr-997-"+prHead, 1),
			artifact(name("MISS", 996, "1", "1"), false, "main", 2),
			artifact("ci-shard-costs-0", false, "main", 1))
		writeDoc(t, filepath.Join(dir, "all-artifacts.json"), doc{"artifacts": all})
		cmd := exec.Command("sh", script(t, "bar.sh"))
		cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
			"GH_TOKEN=x", "REPO=beamfall/corvint", "FORWARD_EPOCH=1", "RUNNER_TEMP=" + dir,
			"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary")}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("bar.sh: %v\n%s", err, out)
		}
		s, _ := os.ReadFile(filepath.Join(dir, "summary"))
		return string(s)
	}
	var names []string
	for pr := 1; pr <= 22; pr++ {
		f := "0"
		if pr <= 5 {
			f = "1"
		}
		names = append(names, name("CLEAN", pr, "1", f))
	}
	names = append([]string{name("INCOMPLETE", 1, "1", "0"), name("CLEAN", 50, "0", "1")}, names...)
	s := bar(t, names)
	// pr1's newer INCOMPLETE row replaces its CLEAN row; the epoch-0 and expired rows never count.
	for _, want := range []string{"Latest 20 row(s) of the epoch: 19 complete, 4 complete with failing packages, 0 FLAKY, 0 MISS, 1 INCOMPLETE.",
		"Shadow bar (20 complete rows, at least 5 with failing packages, no non-FLAKY miss): NOT MET (19/20 complete, 4/5 failing).",
		"Kill switch (AFP-V0-030: 2 MISS or 3 INCOMPLETE in the latest 20): clear.", "Window: 1 page(s)"} {
		if !strings.Contains(s, want) {
			t.Fatalf("summary lacks %q:\n%s", want, s)
		}
	}
	if s := bar(t, names[1:]); !strings.Contains(s, ": MET (20/20 complete, 5/5 failing)") {
		t.Fatalf("summary:\n%s", s)
	}
	// AFP-V0-052: INFRA rows are counted on their own, never trip the INCOMPLETE switch and
	// take no place in the window, so the 20 complete rows behind them still meet the bar.
	infra := append([]string{name("INFRA", 40, "1", "0"), name("INFRA", 41, "1", "0"), name("INFRA", 42, "1", "1")}, names[1:]...)
	s = bar(t, infra)
	for _, want := range []string{"Latest 20 row(s) of the epoch: 20 complete, 5 complete with failing packages, 0 FLAKY, 0 MISS, 0 INCOMPLETE.",
		"Hosted infrastructure failures (AFP-V0-052): 3 INFRA row(s)", ": MET (20/20 complete, 5/5 failing)",
		"3 INCOMPLETE in the latest 20): clear."} {
		if !strings.Contains(s, want) {
			t.Fatalf("summary lacks %q:\n%s", want, s)
		}
	}
	tripped := append([]string{name("MISS", 30, "1", "1"), name("MISS", 31, "1", "1")}, names[1:]...)
	if s := bar(t, tripped); !strings.Contains(s, "NOT MET") || !strings.Contains(s, "TRIPPED") || !strings.Contains(s, "2 MISS") {
		t.Fatalf("summary:\n%s", s)
	}
}

func TestAFPV0047WorkflowBindings(t *testing.T) {
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	ci, cp := read("../../.github/workflows/ci.yml"), read("../../.github/workflows/ci-control-plane.yml")
	one := func(text, pattern string) string {
		t.Helper()
		m := regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1)
		if len(m) != 1 {
			t.Fatalf("%d matches of %q, want 1", len(m), pattern)
		}
		return m[0][1]
	}
	one(ci, `(?m)^      CORVINT_TEST_SHADOW_SOURCE: "([0-9a-f]{40})"$`)
	if e, f := one(ci, `(?m)^      CORVINT_PR_FORWARD_EPOCH: "([0-9]+)"$`), one(cp, `(?m)^      FORWARD_EPOCH: "([0-9]+)"$`); e != f {
		t.Fatalf("FORWARD_EPOCH %s names rows of another epoch than CORVINT_PR_FORWARD_EPOCH %s", f, e)
	}
	job := one(cp, `(?ms)^  forward-rows:\n(.*?)\z`)
	if one(job, `(?m)^    permissions:\n((?:      .*\n)+)`) != "      actions: read\n      contents: read\n" {
		t.Fatalf("forward-rows permissions are not exactly actions: read and contents: read")
	}
	for _, want := range []string{"sparse-checkout: .github/forward-rows", "ref: ${{ github.sha }}",
		"run: sh trusted/.github/forward-rows/collect.sh", "run: sh trusted/.github/forward-rows/bar.sh",
		"PR_BASE_SHA: ${{ github.event.workflow_run.pull_requests[0].base.sha }}"} {
		if !strings.Contains(job, want) {
			t.Fatalf("forward-rows job lacks %q", want)
		}
	}
	shadow := one(ci, `(?ms)^      - name: Shadow corvint test plan .*?\n(.*?)\n      - name: Retain corvint test shadow plan`)
	if !strings.Contains(shadow, `shadow="$RUNNER_TEMP/corvint-shadow-bin/corvint"`) || strings.Contains(shadow, "corvint-pr-bin") ||
		!strings.Contains(shadow, "-f .github/forward-rows/selection.jq") {
		t.Fatalf("the shadow step does not plan with the shadow-pinned binary into selection.json")
	}
	for _, want := range []string{"name: ci-outcomes-${{ github.event_name }}-${{ strategy.job-index }}",
		"sh .github/forward-rows/outcomes.sh first", "sh .github/forward-rows/outcomes.sh rerun",
		`-o "$RUNNER_TEMP/corvint-shadow-bin/corvint" ./cmd/corvint`} {
		if !strings.Contains(ci, want) {
			t.Fatalf("ci.yml lacks %q", want)
		}
	}
	// The rerun bound named in the spec is the one the script enforces.
	if b := read("../../.github/forward-rows/outcomes.sh"); !strings.Contains(b, "\nRERUN_BOUND=8\n") {
		t.Fatal("outcomes.sh no longer bounds the rerun at 8 invocations")
	}
}
