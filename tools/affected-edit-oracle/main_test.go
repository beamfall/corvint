// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompare_AEO_V0_003_005(t *testing.T) {
	c := testCase{MustSelect: []string{"go:needed", "go:missing"}, MaySelect: []string{"go:allowed", "witness:PERMITTED"}}
	selections := []selection{
		{UnitID: "go:extra", Witness: witness{Kind: "FRONTIER", DirtyPath: "leaf.go", ViaLength: 3}},
		{UnitID: "go:needed", Witness: witness{Kind: "OWN"}},
		{UnitID: "go:allowed", Witness: witness{Kind: "REVERSE"}},
		{UnitID: "go:witness-allowed", Witness: witness{Kind: "PERMITTED"}},
	}
	got := compare(c, selections)
	if got.Selected != 4 || got.MustHit != 1 || !reflect.DeepEqual(got.Missing, []string{"go:missing"}) || len(got.Over) != 1 || got.Over[0] != selections[0] || got.ByWitness["FRONTIER"] != 1 {
		t.Fatalf("incorrect comparison: %+v", got)
	}
	reversed := []selection{selections[3], selections[2], selections[1], selections[0], selections[0]}
	if again := compare(c, reversed); !reflect.DeepEqual(got, again) {
		t.Fatalf("not stable under ordering/duplicates: %+v", again)
	}
	for _, r := range []row{{Status: "FAIL", Safety: safety{Status: "NOT_RUN"}}, {Status: "ERROR", Safety: safety{Status: "NOT_RUN"}}, {Status: "PASS", Safety: safety{Status: "SAFETY_FAIL"}}, {Status: "PASS", Safety: safety{Status: "SURVIVED"}}} {
		if !rejected([]row{r}, true) {
			t.Fatalf("allow-over suppressed %+v", r)
		}
	}
	r := row{Status: "OVER", Must: 2, Comparison: got, Safety: safety{Status: "PASS"}}
	if !rejected([]row{r}, false) || rejected([]row{r}, true) {
		t.Fatal("incorrect OVER exit policy")
	}
	totals, witnesses := summarize([]row{r})
	if totals["mustHit"] != 1 || totals["must"] != 2 || totals["missing"] != 1 || totals["over"] != 1 || totals["SAFETY_PASS"] != 1 || witnesses["FRONTIER"] != 1 {
		t.Fatalf("bad totals: %v %v", totals, witnesses)
	}
}

func TestSyntheticClone_AEO_V0_001_002_003_004(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local process-group harness supports Darwin and Linux")
	}
	ctx := context.Background()
	source := t.TempDir()
	clones := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	// These files belong only to a tiny synthetic repository, never the checkout.
	write(".gitignore", "ignored.go\n") //corvint:fixture-path
	write("leaf.go", "package leaf\nfunc Value() int { return 1 }\n")
	write("go.mod", "module oracle.invalid/leaf\n\ngo 1.27.3\n") //corvint:fixture-path
	write("leaf_test.go", "package leaf\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value()!=1 { t.Fatal(\"value\") } }\n")
	for _, args := range [][]string{{"init", "--quiet"}, {"add", "--all"}, {"commit", "--quiet", "-m", "fixture"}} {
		if _, err := git(ctx, source, args...); err != nil {
			t.Fatal(err)
		}
	}
	pin, err := git(ctx, source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	// A protocol stub asserts the actual command and committed diff. It does not
	// implement dependency selection; the comparator's expected sets are separate.
	fake := filepath.Join(t.TempDir(), "planner")
	body := `#!/bin/sh
set -eu
[ "$*" = 'affected --base HEAD~1 --format json' ]
[ -z "$(git status --porcelain)" ]
changed=$(git diff --name-only HEAD~1 HEAD)
selected='[]'
if [ -n "$changed" ]; then
 selected='[{"unitId":"go:oracle.invalid/leaf","witness":{"kind":"OWN","dirtyPath":"leaf.go","viaLength":0}}]'
fi
printf '{"ok":true,"plan":{"selected":%s,"unknown":[]},"summary":{"line":"fixture"}}\n' "$selected"
`
	if err = os.WriteFile(fake, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	tc := testCase{ID: "tiny", Repo: "corvint", BaseSHA: pin, EditClass: "body", Edit: edit{Path: "leaf.go", Operation: "replace-literal", Before: "return 1", Content: "return (1)"}, MustSelect: []string{"go:oracle.invalid/leaf"}, MaySelect: []string{}, Why: "Value is asserted by TestValue."}
	tc.Mutation = &mutation{Edit: edit{Path: "leaf.go", Operation: "replace-literal", Before: "return 1", Content: "return 2"}, Commands: []testCommand{{Argv: []string{"go", "test", "-json", "-count=1", "-run", "^TestValue$", "."}, Format: "go-json"}}}
	if err = validate(corpus{Profile: profile, Cases: []testCase{tc}}); err != nil {
		t.Fatal(err)
	}
	cfg := config{Root: source, Clones: clones, Binary: fake, Mutate: true}
	r := executeCase(ctx, cfg, tc)
	if r.Status != "PASS" || r.Safety.Status != "PASS" || !reflect.DeepEqual(r.Safety.FailingUnits, tc.MustSelect) {
		t.Fatalf("controlled edit/mutation: %+v", r)
	}
	empty := tc
	empty.ID = "empty"
	empty.EditClass = "empty"
	empty.Edit = edit{Operation: "none"}
	empty.MustSelect = []string{}
	empty.Mutation = nil
	if r = executeCase(ctx, cfg, empty); r.Status != "PASS" || r.Comparison.Selected != 0 {
		t.Fatalf("empty: %+v", r)
	}
	// The same real killed mutation must be unsafe when the planner drops its unit.
	if err = os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s\\n' '{\"ok\":true,\"plan\":{\"selected\":[],\"unknown\":[]},\"summary\":{\"line\":\"dropped\"}}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	r = executeCase(ctx, cfg, tc)
	if r.Status != "FAIL" || r.Safety.Status != "SAFETY_FAIL" || !reflect.DeepEqual(r.Safety.Missing, tc.MustSelect) {
		t.Fatalf("missed killer accepted: %+v", r)
	}
	ignored := empty
	ignored.ID = "ignored"
	ignored.EditClass = "new-test"
	ignored.Edit = edit{Path: "ignored.go", Operation: "add-file", Content: "package leaf\n"}
	if r = executeCase(ctx, cfg, ignored); r.Status != "ERROR" || !strings.Contains(r.Error, "committed paths") {
		t.Fatalf("ignored addition became vacuous pass: %+v", r)
	}
	// Refuse caller mistakes before creating an untracked directory in a source.
	bad := cfg
	bad.Clones = source
	if r = executeCase(ctx, bad, empty); r.Status != "ERROR" || !strings.Contains(r.Error, "outside source") {
		t.Fatalf("source clone parent accepted: %+v", r)
	}
	if err = os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s\\n' '{\"ok\":true,\"plan\":{\"unknown\":[]},\"summary\":{\"line\":\"missing selected\"}}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if r = executeCase(ctx, cfg, empty); r.Status != "ERROR" {
		t.Fatalf("missing selected treated as empty success: %+v", r)
	}
	after, err := git(ctx, source, "rev-parse", "HEAD")
	if err != nil || after != pin {
		t.Fatalf("source moved %s %v", after, err)
	}
	status, err := git(ctx, source, "status", "--porcelain")
	if err != nil || status != "" {
		t.Fatalf("source dirty %q %v", status, err)
	}
	entries, err := os.ReadDir(clones)
	if err != nil || len(entries) != 0 {
		t.Fatalf("clones leaked: %v %v", entries, err)
	}
}

func TestEditValidation_AEO_V0_001_002(t *testing.T) {
	root := t.TempDir()
	for _, e := range []edit{{Path: "../outside", Operation: "add-file", Content: "x"}, {Path: ".git/config", Operation: "delete-file"}, {Path: "a", Operation: "replace-literal", Before: "same", Content: "same"}} {
		if err := apply(root, e); err == nil {
			t.Fatalf("unsafe edit accepted: %+v", e)
		}
	}
	if err := apply(root, edit{Path: "a", Operation: "add-file", Content: "x x"}); err != nil {
		t.Fatal(err)
	}
	if err := apply(root, edit{Path: "a", Operation: "replace-literal", Before: "x", Content: "y"}); err == nil {
		t.Fatal("ambiguous replacement accepted")
	}
	if err := apply(root, edit{Path: "a", Operation: "add-file", Content: "overwrite"}); err == nil {
		t.Fatal("add overwrote file")
	}
	if err := apply(root, edit{Path: "a", Operation: "delete-file"}); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(t.TempDir(), filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		if err := apply(root, edit{Path: "link/x", Operation: "add-file", Content: "x"}); err == nil {
			t.Fatal("symlink escaped")
		}
	}
}

func TestObservations_AEO_V0_004(t *testing.T) {
	cmd := testCommand{Format: "go-json"}
	_, units, _, err := testObservations(cmd, []byte("{\"Action\":\"fail\",\"Package\":\"bad-build\"}\n"))
	if err != nil || len(units) != 0 {
		t.Fatalf("build failure counted as kill %v %v", units, err)
	}
	passes, units, tests, err := testObservations(cmd, []byte("admission log\n{\"Action\":\"pass\",\"Package\":\"p\",\"Test\":\"TestA\"}\n{\"Action\":\"fail\",\"Package\":\"p\",\"Test\":\"TestB\"}\n"))
	if err != nil || passes != 1 || !reflect.DeepEqual(units, []string{"go:p"}) || !reflect.DeepEqual(tests, []string{"go:p#TestB"}) {
		t.Fatalf("events: %d %v %v %v", passes, units, tests, err)
	}
	// A subtest failure followed by a binary timeout is not a qualified kill.
	if _, _, _, err := testObservations(cmd, []byte("{\"Action\":\"fail\",\"Package\":\"p\",\"Test\":\"TestA/sub\"}\n{\"Action\":\"output\",\"Package\":\"p\",\"Test\":\"TestA\",\"Output\":\"panic: test timed out after 1m30s\\n\"}\n")); err == nil {
		t.Fatal("timed-out run counted as a kill")
	}
	_, units, _, err = testObservations(testCommand{Format: "node-assert", UnitID: "typescript:test"}, []byte("Error [ERR_ASSERTION]: mismatch"))
	if err != nil || !reflect.DeepEqual(units, []string{"typescript:test"}) {
		t.Fatal("node assertion missing")
	}
}

func TestCorpus_AEO_V0_001_006(t *testing.T) {
	body, err := os.ReadFile("testdata/corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var c corpus
	if err = json.Unmarshal(body, &c); err != nil {
		t.Fatal(err)
	}
	if err = validate(c); err != nil {
		t.Fatal(err)
	}
	classes := map[string]map[string]bool{}
	mutations := map[string]int{}
	for _, tc := range c.Cases {
		if classes[tc.Repo] == nil {
			classes[tc.Repo] = map[string]bool{}
		}
		classes[tc.Repo][tc.EditClass] = true
		if tc.Mutation != nil {
			mutations[tc.Repo]++
			for _, cmd := range tc.Mutation.Commands {
				if tc.Repo == "corvint" && cmd.Format == "go-json" && (len(cmd.Argv) < 5 || strings.Join(cmd.Argv[:5], " ") != "script/heavy.sh env GOTOOLCHAIN=local go test") {
					t.Fatal("Corvint mutation bypasses heavy wrapper")
				}
			}
		}
	}
	for _, repo := range []string{"corvint", "beamfall-workspace/beamfall-web", "beamfall-workspace/beamfall-cloud", "beamfall-workspace/beamfall-plugin-sdk", "beamfall-workspace/beamfall"} {
		if len(classes[repo]) < 12 || mutations[repo] < 1 {
			t.Fatalf("corpus incomplete %s classes=%d mutations=%d", repo, len(classes[repo]), mutations[repo])
		}
	}
	if mutations["corvint"] < 4 {
		t.Fatal("need four Corvint killers")
	}
}

func TestCommandCancellation_AEO_V0_002(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires process groups")
	}
	// The shell waits for its descendant: completion after cancellation exercises
	// retirement of both, not merely killing/reaping the wrapper.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := command(ctx, t.TempDir(), time.Minute, "sh", "-c", "sleep 60 & wait")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 6*time.Second {
		t.Fatalf("cancellation failed: %v", err)
	}
}
