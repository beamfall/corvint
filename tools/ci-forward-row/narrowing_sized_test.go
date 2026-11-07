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
