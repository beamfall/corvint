// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// reasonRun runs the real narrow.sh in a scratch checkout with the fake tools of
// TestAFPV0098SupersetGuardSidecar (the driver selects a, b and c of the universe a..d, the shadow
// a and b, git names no narrowing input) and returns the mode output, the corvint-narrowing
// directory, the job log (stdout and stderr) and the step summary.
func reasonRun(t *testing.T, blockSidecar bool, override map[string]string) (mode, out, log, summary string) {
	t.Helper()
	const (
		base   = "142f0e3e7bfb153f803fb6c4bad7989f87ee324a"
		head   = "460fc31ae6d0134fa27f7a66336b99db88fc91c6"
		target = "5163d1ed4b61084fc32cf79355848899c3e8a5bb"
		a, b   = "example.org/m/a", "example.org/m/b"
		c, d   = "example.org/m/c", "example.org/m/d"
	)
	dir := t.TempDir()
	fake, bin := filepath.Join(dir, "fake"), filepath.Join(dir, "bin")
	writeDoc(t, filepath.Join(fake, "selection.json"), doc{"EventBase": base, "Reason": "", "Base": base, "Head": head, "Target": target, "Packages": []string{a, b, c}})
	writeDoc(t, filepath.Join(fake, "shadow.json"), doc{"scope": "NARROWED", "incomplete": []string{},
		"invocations": []any{doc{"units": []any{doc{"adapter": "go", "unit": "go:" + a}, doc{"adapter": "go", "unit": "go:" + b}}}}})
	tools := fakeNarrowTools(a, b, c, d)
	tools["git"] = guardGit
	for name, body := range override {
		tools[name] = body
	}
	installTools(t, bin, tools)
	if blockSidecar {
		if err := os.MkdirAll(filepath.Join(dir, "corvint-narrowing", "executed"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dir, "output"), nil)
	cmd := exec.Command("sh", script(t, "narrow.sh"))
	cmd.Dir = dir
	cmd.Env = narrowEnv(dir, bin, fake, base, head, target, "3", "8")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("narrow.sh: %v\n%s", err, raw)
	}
	s, _ := os.ReadFile(filepath.Join(dir, "summary"))
	return outputValue(t, dir, "mode"), filepath.Join(dir, "corvint-narrowing"), string(raw), string(s)
}

// TestCINV08421FullReasonIsLoggedAndRecorded_CINV08421: every FULL decision of narrow.sh prints
// `AFP-V0-098 shard N: FULL (REASON); reason codes: CODE` to the job log as well as the step
// summary, the record carries `reasonCodes` parallel to `reasons`, and decision.json beside the
// record keeps the mode and codes also when a fallback drops the record (V1-5196).
func TestCINV08421FullReasonIsLoggedAndRecorded_CINV08421(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/narrow.sh", "CIN-V0-8421")
	testplatform.SkipPOSIXShell(t, "the git, gh, go and driver spies are #!/bin/sh scripts run by narrow.sh under sh")
	realJQ, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("jq not found")
	}
	t.Run("a decided FULL names its code in the log, the summary, the record and the sidecar", func(t *testing.T) {
		mode, out, log, summary := reasonRun(t, false, map[string]string{"git": "#!/bin/sh\ncase \"$*\" in 'diff --name-only -z'*) printf '.github/forward-rows/narrow.sh\\0' ;; 'diff --quiet'*) exit 0 ;; *) exit 1 ;; esac\n"})
		want := "AFP-V0-098 shard 3: FULL (the change touches .github/ narrowing inputs (AFP-V0-2720): .github/forward-rows/narrow.sh); reason codes: github-input\n"
		if mode != "FULL" || !strings.Contains(log, want) || !strings.Contains(summary, want) {
			t.Fatalf("mode %s\nlog:\n%s\nsummary:\n%s", mode, log, summary)
		}
		record := readDoc(t, filepath.Join(out, "narrowing.json"))
		if fmt.Sprint(record["reasonCodes"]) != fmt.Sprint([]string{"github-input"}) || len(record["reasons"].([]any)) != 1 {
			t.Fatalf("record codes %v reasons %v", record["reasonCodes"], record["reasons"])
		}
		decision := readDoc(t, filepath.Join(out, "decision.json"))
		if decision["profile"] != "corvint-pr-narrowing-decision/0" || decision["mode"] != "FULL" || fmt.Sprint(decision["reasonCodes"]) != fmt.Sprint([]string{"github-input"}) ||
			fmt.Sprint(decision["shard"]) != "3" || fmt.Sprint(decision["shards"]) != "8" || decision["runId"] != float64(38482995697) || fmt.Sprint(decision["pr"]) != "868" ||
			fmt.Sprint(decision["reasons"]) != fmt.Sprint(record["reasons"]) {
			t.Fatalf("decision %v", decision)
		}
	})
	t.Run("several reasons list every code in order", func(t *testing.T) {
		_, out, log, _ := reasonRun(t, false, map[string]string{
			"git":    "#!/bin/sh\ncase \"$*\" in 'diff --name-only -z'*) printf '.github/forward-rows/narrow.sh\\0' ;; 'diff --quiet'*) exit 0 ;; *) exit 1 ;; esac\n",
			"shadow": "#!/bin/sh\nexit 7\n",
		})
		record := readDoc(t, filepath.Join(out, "narrowing.json"))
		if fmt.Sprint(record["reasonCodes"]) != fmt.Sprint([]string{"github-input", "shadow-no-plan"}) {
			t.Fatalf("codes %v", record["reasonCodes"])
		}
		if !strings.Contains(log, "); reason codes: github-input, shadow-no-plan\n") || !strings.Contains(log, "narrow.sh; the shadow wrote no plan); reason codes") {
			t.Fatalf("log:\n%s", log)
		}
	})
	t.Run("a dropped record still leaves the decision and the log line", func(t *testing.T) {
		mode, out, log, summary := reasonRun(t, true, nil)
		want := "AFP-V0-098 shard 3: FULL (the executed list could not be written beside the record); reason codes: sidecar-unwritable\n"
		if mode != "FULL" || !strings.Contains(log, want) || !strings.Contains(summary, want) {
			t.Fatalf("mode %s\nlog:\n%s\nsummary:\n%s", mode, log, summary)
		}
		if _, err := os.Stat(filepath.Join(out, "narrowing.json")); err == nil {
			t.Fatal("the fallback kept the record")
		}
		decision := readDoc(t, filepath.Join(out, "decision.json"))
		if decision["mode"] != "FULL" || fmt.Sprint(decision["reasonCodes"]) != fmt.Sprint([]string{"sidecar-unwritable"}) ||
			fmt.Sprint(decision["reasons"]) != fmt.Sprint([]string{"the executed list could not be written beside the record"}) {
			t.Fatalf("decision %v", decision)
		}
	})
	t.Run("a failed decision is logged with its code", func(t *testing.T) {
		mode, out, log, _ := reasonRun(t, false, map[string]string{
			"jq": "#!/bin/sh\ncase \"$*\" in *narrow.jq*) echo boom >&2; exit 5 ;; *) exec '" + realJQ + "' \"$@\" ;; esac\n",
		})
		if mode != "FULL" || !strings.Contains(log, "AFP-V0-098 shard 3: FULL (the narrowing decision failed: boom); reason codes: decision-failed\n") {
			t.Fatalf("mode %s\nlog:\n%s", mode, log)
		}
		if decision := readDoc(t, filepath.Join(out, "decision.json")); fmt.Sprint(decision["reasonCodes"]) != fmt.Sprint([]string{"decision-failed"}) {
			t.Fatalf("decision %v", decision)
		}
	})
	t.Run("a NARROWED decision is logged and recorded with no code", func(t *testing.T) {
		mode, out, log, summary := reasonRun(t, false, nil)
		if mode != "NARROWED" || !strings.Contains(log, "AFP-V0-098 shard 3: NARROWED (BALANCED), 2 package(s)") || strings.Contains(log, "reason codes") ||
			!strings.Contains(summary, "AFP-V0-098 shard 3: NARROWED (BALANCED)") {
			t.Fatalf("mode %s\nlog:\n%s\nsummary:\n%s", mode, log, summary)
		}
		decision := readDoc(t, filepath.Join(out, "decision.json"))
		if decision["mode"] != "NARROWED" || fmt.Sprint(decision["reasonCodes"]) != "[]" || fmt.Sprint(readDoc(t, filepath.Join(out, "narrowing.json"))["reasonCodes"]) != "[]" {
			t.Fatalf("decision %v", decision)
		}
	})
	// The merge-queue path reaches the same narrow.sh, so a selection refusal of mq-narrow.sh is
	// logged as the driver reason with its code.
	t.Run("mq-narrow.sh logs its FULL reason through narrow.sh", func(t *testing.T) {
		repo, base, entry1, entry2 := stackedGroup(t)
		const a, b, c = "example.org/m/a", "example.org/m/b", "example.org/m/c"
		dir := t.TempDir()
		bin, fake := filepath.Join(dir, "bin"), filepath.Join(dir, "fake")
		installTools(t, bin, map[string]string{
			"gh":        "#!/bin/sh\necho true\n",
			"go":        "#!/bin/sh\nprintf '%s\\n' " + a + " " + b + " " + c + "\n",
			"shadow":    "#!/bin/sh\nprintf '%s\\n' '{\"scope\":\"NARROWED\",\"incomplete\":[],\"invocations\":[{\"units\":[{\"adapter\":\"go\",\"unit\":\"go:" + a + "\"}]}]}'\n",
			"ci-shards": "#!/bin/sh\ncase \"$*\" in *--slices*) ;; *--select*) echo " + b + " ;; *) printf '%s\\n' " + a + " " + c + " ;; esac\n",
		})
		if err := os.MkdirAll(fake, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "output"), nil)
		cmd := exec.Command("sh", script(t, "mq-narrow.sh"))
		cmd.Dir = repo
		cmd.Env = append(narrowEnv(dir, bin, fake, base, entry2, entry2, "3", "8"),
			"QUEUE_BASE="+base, "QUEUE_BASE_REF=refs/heads/release", "QUEUE_HEAD="+entry2, "QUEUE_HEAD_REF=refs/heads/gh-readonly-queue/main/pr-77-"+entry1)
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("mq-narrow.sh: %v\n%s", err, raw)
		}
		// The refused selection also fails the superset and placement checks; the driver reason leads.
		if outputValue(t, dir, "mode") != "FULL" || !strings.Contains(string(raw), "AFP-V0-098 shard 3: FULL (driver: the queue base ref is refs/heads/release, not refs/heads/main; ") ||
			!strings.Contains(string(raw), "); reason codes: driver-reason, ") {
			t.Fatalf("log:\n%s", raw)
		}
		if decision := readDoc(t, filepath.Join(dir, "corvint-narrowing", "decision.json")); decision["mode"] != "FULL" || fmt.Sprint(decision["reasonCodes"].([]any)[0]) != "driver-reason" {
			t.Fatalf("decision %v", decision)
		}
	})
}

// TestCINV08422FullReasonsTally_CINV08422: full-reasons.sh lists the last N completed runs with
// the mode and reason codes of their ci-narrowing-* decisions and tallies FULL by code: a run
// whose shards all narrowed, a FULL run with two shards' codes, an older run with a record but
// no decision (`legacy`), a run with no record at all and a fallback decision without a record.
func TestCINV08422FullReasonsTally_CINV08422(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/full-reasons.sh", "")
	testplatform.SkipPOSIXShell(t, "full-reasons.sh and its gh spy run under sh")
	dir := t.TempDir()
	bin, fake := filepath.Join(dir, "bin"), filepath.Join(dir, "fake")
	writeFile(t, filepath.Join(fake, "runs"), lines("101 pull_request success", "102 pull_request success", "103 merge_group failure", "104 pull_request success", "105 push success"))
	writeDoc(t, filepath.Join(fake, "101", "ci-narrowing-0", "decision.json"), doc{"profile": "corvint-pr-narrowing-decision/0", "mode": "NARROWED", "reasonCodes": []string{}})
	writeDoc(t, filepath.Join(fake, "101", "ci-narrowing-0", "narrowing.json"), doc{"profile": "corvint-pr-narrowing/0", "mode": "NARROWED"})
	writeDoc(t, filepath.Join(fake, "101", "ci-narrowing-1", "decision.json"), doc{"profile": "corvint-pr-narrowing-decision/0", "mode": "NARROWED", "reasonCodes": []string{}})
	writeDoc(t, filepath.Join(fake, "102", "ci-narrowing-0", "decision.json"), doc{"profile": "corvint-pr-narrowing-decision/0", "mode": "FULL", "reasonCodes": []string{"github-input"}})
	writeDoc(t, filepath.Join(fake, "102", "ci-narrowing-1", "decision.json"), doc{"profile": "corvint-pr-narrowing-decision/0", "mode": "FULL", "reasonCodes": []string{"github-input", "shadow-no-plan"}})
	writeDoc(t, filepath.Join(fake, "103", "ci-narrowing-0", "narrowing.json"), doc{"profile": "corvint-pr-narrowing/0", "mode": "FULL", "reasons": []string{"the shadow wrote no plan"}})
	writeDoc(t, filepath.Join(fake, "105", "ci-narrowing-4", "decision.json"), doc{"profile": "corvint-pr-narrowing-decision/0", "mode": "FULL", "reasonCodes": []string{"sidecar-unwritable"}})
	// The gh spy records its argv, serves the run listing and copies a run's fixture artifacts.
	installTools(t, bin, map[string]string{"gh": "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$FAKE/argv\"\ncase \"$1 $2\" in\n" +
		"'api repos/beamfall/corvint/actions/workflows/ci.yml/runs?per_page=5&status=completed') cat \"$FAKE/runs\" ;;\n" +
		"'api repos/beamfall/corvint/actions/workflows/ci.yml/runs?per_page=2&status=completed&event=pull_request') head -n 2 \"$FAKE/runs\" ;;\n" +
		"'run download') [ -d \"$FAKE/$3\" ] && cp -R \"$FAKE/$3/.\" \"$9\" || exit 1 ;;\n" +
		"*) echo \"unexpected gh $*\" >&2; exit 9 ;;\nesac\n"})
	env := []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir, "FAKE=" + fake, "GH_TOKEN=x", "REPO=beamfall/corvint", "TMPDIR=" + dir}
	tally := func(args ...string) (string, int) {
		cmd := exec.Command("sh", append([]string{script(t, "full-reasons.sh")}, args...)...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			code = cmd.ProcessState.ExitCode()
		}
		return string(out), code
	}
	out, code := tally("5")
	want := lines(
		"run 101 pull_request success NARROWED -",
		"run 102 pull_request success FULL github-input,shadow-no-plan",
		"run 103 merge_group failure FULL legacy:the-shadow-wrote-no",
		"run 104 pull_request success none -",
		"run 105 push success FULL sidecar-unwritable",
		"code github-input 1 102",
		"code legacy:the-shadow-wrote-no 1 103",
		"code shadow-no-plan 1 102",
		"code sidecar-unwritable 1 105",
		"runs 5 full 3 narrowed 1 none 1")
	if code != 0 || out != string(want) {
		t.Fatalf("code %d output:\n%s\nwant:\n%s", code, out, want)
	}
	argv, _ := os.ReadFile(filepath.Join(fake, "argv"))
	if !strings.Contains(string(argv), "run download 102 --repo beamfall/corvint --pattern ci-narrowing-* --dir ") {
		t.Fatalf("gh argv:\n%s", argv)
	}
	if out, code := tally("2", "pull_request"); code != 0 || !strings.HasSuffix(out, "runs 2 full 1 narrowed 1 none 0\n") || strings.Contains(out, "run 103") {
		t.Fatalf("filtered: code %d output:\n%s", code, out)
	}
	for _, args := range [][]string{{"0"}, {"101"}, {"5", "schedule"}, {}} {
		if _, code := tally(args...); code != 2 {
			t.Fatalf("args %q: exit %d, want 2", args, code)
		}
	}
	cmd := exec.Command("sh", script(t, "full-reasons.sh"), "5")
	cmd.Env = append(env, "WORKFLOW=other.yml")
	if out, err := cmd.CombinedOutput(); err == nil || cmd.ProcessState.ExitCode() != 1 || !strings.Contains(string(out), "could not list the runs of other.yml") {
		t.Fatalf("unlistable: %v\n%s", err, out)
	}
}

// guardMapped runs github-guard.sh over base..target and returns the first line and the
// consumer map of a `mapped` answer.
func (r *guardRepo) guardMapped(target string) (string, map[string][]string) {
	r.t.Helper()
	first, rest := r.guard(target)
	mapped := map[string][]string{}
	if first == "mapped" {
		for _, line := range rest {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				r.t.Fatalf("malformed mapped line %q", line)
			}
			mapped[fields[0]] = fields[1:]
		}
	}
	return first, mapped
}

// guardGlobalInputs are the trusted narrowing inputs under .github/forward-rows that decide or
// execute what a shard runs or admit narrowing (CIN-V0-8423); every other file of the directory
// must be mapped to its consumers, and a file in neither list is unmapped, so FULL.
var guardGlobalInputs = []string{"accept.sh", "bar.jq", "github-guard.sh", "mq-narrow.sh", "names.sh", "narrow.jq", "narrow.sh", "pin-lag.sh", "pr-base.sh", "shard-plan.sh", "verdict.sh"}

// TestCINV08423GuardClassifiesEveryInput_CINV08423: every file of this repository's
// .github/forward-rows and both guarded workflows is either a global input (FULL, named) or mapped
// to consumer packages that include every package whose .corvint/test-read-scopes.json entry
// covers the path; an unmapped new file, a renamed file and a mix of a mapped and a global input
// stay FULL (V1-5202).
func TestCINV08423GuardClassifiesEveryInput_CINV08423(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/github-guard.sh", "CIN-V0-8423")
	testplatform.SkipPOSIXShell(t, "github-guard.sh runs under sh against a real Git repository")
	entries, err := os.ReadDir("../../.github/forward-rows")
	if err != nil {
		t.Fatal(err)
	}
	scopes, err := os.ReadFile("../../.corvint/test-read-scopes.json")
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(scopes, &declared); err != nil {
		t.Fatal(err)
	}
	// A consumer with no read-scope declaration is selected only as an unbounded reader, which
	// runs on every change.
	raw, err := os.ReadFile("../../.corvint/unbounded-readers.json")
	if err != nil {
		t.Fatal(err)
	}
	var unbounded struct {
		Units []string `json:"units"`
	}
	if err := json.Unmarshal(raw, &unbounded); err != nil {
		t.Fatal(err)
	}
	readers := func(path string) []string {
		var out []string
		for pkg, scope := range declared.Packages {
			for _, entry := range scope {
				if entry == path || (strings.HasSuffix(entry, "/") && strings.HasPrefix(path, entry)) {
					out = append(out, pkg)
					break
				}
			}
		}
		sort.Strings(out)
		return out
	}
	r := newGuardRepo(t)
	var inputs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			inputs = append(inputs, ".github/forward-rows/"+entry.Name())
		}
	}
	inputs = append(inputs, ".github/workflows/ci-control-plane.yml", ".github/workflows/ci-forward-bar.yml")
	mappedCount := 0
	for i, input := range inputs {
		t.Run(input, func(t *testing.T) {
			target := r.change(fmt.Sprintf("input-%d", i), func() { r.write(input, "# edited\n") })
			first, mapped := r.guardMapped(target)
			if slices.Contains(guardGlobalInputs, strings.TrimPrefix(input, ".github/forward-rows/")) {
				if first != "1" {
					t.Fatalf("global input %s: guard = %s, want 1", input, first)
				}
				return
			}
			consumers := mapped[input]
			if first != "mapped" || len(mapped) != 1 || len(consumers) == 0 {
				t.Fatalf("guard = %s %v, want mapped with the consumers of %s", first, mapped, input)
			}
			mappedCount++
			for _, reader := range readers(input) {
				if !slices.Contains(consumers, reader) {
					t.Fatalf("%s is read by %s under %s, which the guard does not name among %v", input, reader, "test-read-scopes.json", consumers)
				}
			}
			for _, consumer := range consumers {
				if _, ok := declared.Packages[consumer]; !ok && !slices.Contains(unbounded.Units, consumer) {
					t.Fatalf("consumer %s of %s is neither a declared read-scope package nor an unbounded reader, so the planner never selects it by name", consumer, input)
				}
			}
		})
	}
	if mappedCount == 0 {
		t.Fatal("no input is mapped")
	}
	t.Run("an unmapped new file is FULL", func(t *testing.T) {
		target := r.change("new-file", func() { r.write(".github/forward-rows/new-report.sh", "#!/bin/sh\n") })
		if first, _ := r.guardMapped(target); first != "1" {
			t.Fatalf("guard = %s, want 1", first)
		}
	})
	t.Run("a mapped input renamed is FULL", func(t *testing.T) {
		r.write(".github/forward-rows/collect.sh", "#!/bin/sh\n")
		r.git("add", "-A")
		r.git("commit", "-q", "-m", "collect")
		r.base = r.git("rev-parse", "HEAD")
		target := r.change("rename", func() {
			r.remove(".github/forward-rows/collect.sh")
			r.write(".github/forward-rows/collect2.sh", "#!/bin/sh\n")
		})
		first, rest := r.guard(target)
		if first != "1" || len(rest) != 2 {
			t.Fatalf("guard = %s %q, want 1 naming both sides", first, rest)
		}
	})
	t.Run("a mapped and a global input together are FULL", func(t *testing.T) {
		target := r.change("mixed", func() {
			r.write(".github/forward-rows/collect.sh", "#!/bin/sh\necho rows\n")
			r.write(".github/forward-rows/narrow.sh", "#!/bin/sh\necho mode=FULL\n")
		})
		first, rest := r.guard(target)
		if first != "1" || fmt.Sprint(rest) != fmt.Sprint([]string{".github/forward-rows/collect.sh", ".github/forward-rows/narrow.sh"}) {
			t.Fatalf("guard = %s %q", first, rest)
		}
	})
	t.Run("two mapped inputs list both", func(t *testing.T) {
		target := r.change("two-mapped", func() {
			r.write(".github/forward-rows/collect.sh", "#!/bin/sh\necho rows2\n")
			r.write(".github/forward-rows/tiers.sh", "#!/bin/sh\n")
		})
		first, mapped := r.guardMapped(target)
		if first != "mapped" || len(mapped) != 2 || fmt.Sprint(mapped[".github/forward-rows/tiers.sh"]) != fmt.Sprint([]string{"tools/ci-forward-row", "tools/tiers-replay"}) {
			t.Fatalf("guard = %s %v", first, mapped)
		}
	})
}

// TestCINV08424NarrowShRequiresMappedConsumers_CINV08424 runs the real narrow.sh, guard and git
// over pull requests that edit only a mapped input: the shard narrows when the driver selection
// holds every named consumer and records the mapping; it is FULL naming the omitted consumer when
// the selection lacks one; a checkout without a module path is unreadable, so FULL (V1-5202).
func TestCINV08424NarrowShRequiresMappedConsumers_CINV08424(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/narrow.sh", "CIN-V0-8424")
	testplatform.SkipPOSIXShell(t, "narrow.sh and github-guard.sh run under sh against a real Git repository")
	const (
		a, b, c, d = "example.org/m/a", "example.org/m/b", "example.org/m/c", "example.org/m/d"
		rows       = "example.org/m/tools/ci-forward-row"
		tiers      = "example.org/m/tools/tiers-replay"
	)
	r := newGuardRepo(t)
	r.write("go.mod", "module example.org/m\n\ngo 1.27.3\n")
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "module")
	r.base = r.git("rev-parse", "HEAD")
	narrow := func(t *testing.T, target string, selected []string, override map[string]string) (string, doc, string) {
		t.Helper()
		dir := t.TempDir()
		fake, bin := filepath.Join(dir, "fake"), filepath.Join(dir, "bin")
		base, head := r.git("rev-parse", target+"^1"), r.git("rev-parse", target+"^2")
		writeDoc(t, filepath.Join(fake, "selection.json"), doc{"EventBase": base, "Reason": "", "Base": base, "Head": head, "Target": target, "Packages": selected})
		writeDoc(t, filepath.Join(fake, "shadow.json"), doc{"scope": "NARROWED", "incomplete": []string{},
			"invocations": []any{doc{"units": []any{doc{"adapter": "go", "unit": "go:" + a}, doc{"adapter": "go", "unit": "go:" + b}}}}})
		tools := fakeNarrowTools(a, b, c, d)
		tools["go"] = "#!/bin/sh\nprintf '%s\\n' " + a + " " + b + " " + c + " " + d + " " + rows + " " + tiers + "\n"
		for name, body := range override {
			tools[name] = body
		}
		installTools(t, bin, tools)
		writeFile(t, filepath.Join(dir, "output"), nil)
		cmd := exec.Command("sh", script(t, "narrow.sh"))
		cmd.Dir = r.dir
		cmd.Env = narrowEnv(dir, bin, fake, base, head, target, "3", "8")
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("narrow.sh: %v\n%s", err, raw)
		}
		return outputValue(t, dir, "mode"), readDoc(t, filepath.Join(dir, "corvint-narrowing", "narrowing.json")), string(raw)
	}
	edit := r.change("edit-collect", func() { r.write(".github/forward-rows/collect.sh", "#!/bin/sh\necho rows\n") })
	t.Run("a selection holding the consumer narrows and records the mapping", func(t *testing.T) {
		mode, v, log := narrow(t, edit, []string{a, b, c, rows}, nil)
		if mode != "NARROWED" || v["mode"] != "NARROWED" || fmt.Sprint(v["reasons"]) != "[]" || fmt.Sprint(v["githubInputs"]) != "[]" ||
			fmt.Sprint(v["githubMapped"]) != fmt.Sprint([]string{".github/forward-rows/collect.sh: tools/ci-forward-row"}) ||
			fmt.Sprint(v["selected"]) != fmt.Sprint([]string{a, b, c, rows}) {
			t.Fatalf("mode %s, record %v", mode, v)
		}
		if !strings.Contains(log, "; mapped .github/ inputs (CIN-V0-8423): .github/forward-rows/collect.sh: tools/ci-forward-row") {
			t.Fatalf("log:\n%s", log)
		}
	})
	t.Run("a selection omitting the consumer is FULL naming it", func(t *testing.T) {
		mode, v, log := narrow(t, edit, []string{a, b, c}, nil)
		want := "the selection omits a consumer of a mapped .github/ narrowing input (CIN-V0-8424): .github/forward-rows/collect.sh: tools/ci-forward-row"
		if mode != "FULL" || v["mode"] != "FULL" || fmt.Sprint(v["reasons"]) != "["+want+"]" || fmt.Sprint(v["reasonCodes"]) != "[github-consumer-unselected]" ||
			fmt.Sprint(v["githubMapped"]) != fmt.Sprint([]string{".github/forward-rows/collect.sh: tools/ci-forward-row"}) || v["placement"] != "PARTITION" {
			t.Fatalf("mode %s, record %v", mode, v)
		}
		if !strings.Contains(log, "AFP-V0-098 shard 3: FULL ("+want+"); reason codes: github-consumer-unselected\n") {
			t.Fatalf("log:\n%s", log)
		}
	})
	t.Run("an input with two consumers needs both", func(t *testing.T) {
		two := r.change("edit-tiers", func() { r.write(".github/forward-rows/tiers.sh", "#!/bin/sh\necho tiers\n") })
		if mode, v, _ := narrow(t, two, []string{a, b, c, rows}, nil); mode != "FULL" || !strings.Contains(fmt.Sprint(v["reasons"]), ".github/forward-rows/tiers.sh: tools/tiers-replay") ||
			strings.Contains(fmt.Sprint(v["reasons"]), "tiers.sh: tools/ci-forward-row") {
			t.Fatalf("mode %s, record %v", mode, v)
		}
		if mode, v, _ := narrow(t, two, []string{a, b, c, rows, tiers}, nil); mode != "NARROWED" || fmt.Sprint(v["githubMapped"]) != fmt.Sprint([]string{".github/forward-rows/tiers.sh: tools/ci-forward-row tools/tiers-replay"}) {
			t.Fatalf("mode %s, record %v", mode, v)
		}
	})
	t.Run("a checkout without a module path is unreadable", func(t *testing.T) {
		gone := r.change("no-module", func() {
			r.write(".github/forward-rows/collect.sh", "#!/bin/sh\necho rows3\n")
			r.write("go.mod", "// no module line\n")
		})
		if mode, v, _ := narrow(t, gone, []string{a, b, c, rows}, nil); mode != "FULL" || fmt.Sprint(v["reasonCodes"]) != "[github-unreadable]" {
			t.Fatalf("mode %s, record %v", mode, v)
		}
	})
	t.Run("a global input beside a mapped one stays FULL naming both", func(t *testing.T) {
		both := r.change("edit-both", func() {
			r.write(".github/forward-rows/collect.sh", "#!/bin/sh\necho rows4\n")
			r.write(".github/forward-rows/narrow.jq", "{mode: \"NARROWED\"}\n")
		})
		if mode, v, _ := narrow(t, both, []string{a, b, c, rows}, nil); mode != "FULL" || fmt.Sprint(v["reasonCodes"]) != "[github-input]" ||
			fmt.Sprint(v["githubInputs"]) != fmt.Sprint([]string{".github/forward-rows/collect.sh", ".github/forward-rows/narrow.jq"}) || fmt.Sprint(v["githubMapped"]) != "[]" {
			t.Fatalf("mode %s, record %v", mode, v)
		}
	})
}
