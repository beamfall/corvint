// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/testsupport"
)

var ownershipGraph struct {
	once    sync.Once
	graph   *affected.Graph
	tracked []string
	err     error
}

// repositoryOwnershipGraph builds this repository's full graph once for the
// ownership reader checks, with the tracked listing.
func repositoryOwnershipGraph(t *testing.T) (*affected.Graph, []string) {
	t.Helper()
	testsupport.RequireLiveTier(t)
	ownershipGraph.once.Do(func() {
		root := repositoryRoot(t)
		listing, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
		if err != nil {
			ownershipGraph.err = err
			return
		}
		ownershipGraph.tracked = strings.Split(strings.TrimSuffix(string(listing), "\x00"), "\x00")
		ownershipGraph.graph, ownershipGraph.err = affected.Build(context.Background(), root, languages.All()...)
	})
	if ownershipGraph.err != nil {
		t.Fatal(ownershipGraph.err)
	}
	return ownershipGraph.graph, ownershipGraph.tracked
}

func selectedSet(plan affected.Plan) map[string]affected.Selection {
	selected := make(map[string]affected.Selection, len(plan.Selected))
	for _, selection := range plan.Selected {
		selected[selection.UnitID] = selection
	}
	return selected
}

// ownershipRuleRemoved reports whether the plan of path alone holds it in a
// non-source group of the ownership rule (AZU-V0-002, AFP-V0-2482): a
// declared non-source path or a declared test owner path.
func ownershipRuleRemoved(plan affected.Plan, path string) bool {
	for _, group := range plan.NonSource {
		if (group.Class == affected.ClassDeclaredTestOwner || group.Class == affected.ClassDeclaredNonSource) && slices.Contains(group.Paths, path) {
			return true
		}
	}
	return false
}

// AZU-V0-8926: one repository regression per miss class of the V1-4685
// reader audit. Each path is owned by the ownership rule, and each unit
// reads, runs or compiles it; the plan of the path alone selects the unit.
func TestOwnershipReaderMissClasses_AZU_V0_8926(t *testing.T) {
	graph, _ := repositoryOwnershipGraph(t)
	for _, tc := range []struct {
		class, path string
		units       []string
	}{
		// N1: the Python shard test reads the cost table; the Python plugin
		// carries no path literal, so the owner names it. The Go shard package
		// embeds both tables and survives the replay tool's retirement.
		{"N1", ".github/cishards/package-costs.json", []string{"python:.github.macos.shard_test", "go:github.com/Beamfall/corvint/.github/cishards"}},
		{"N1", ".github/cishards/test-split-allow.json", []string{"go:github.com/Beamfall/corvint/.github/cishards"}},
		// N2: dr-numbers-check runs a script that reads the register as
		// the default word of ${1:-...}.
		{"N2", "conformance/divergence-register.md", []string{"driver:make:Makefile#dr-numbers-check"}},
		// W: the whole-tree content checks read every tracked path.
		{"W", "internal/mcp/cli/testdata/tools-list-default.golden.jsonl", []string{"driver:make:Makefile#eol-policy-check", "driver:make:Makefile#license-headers-check", "driver:make:Makefile#long-lines-check"}},
		{"W", "benchmarks/README.md", []string{"driver:make:Makefile#eol-policy-check", "driver:make:Makefile#license-headers-check", "driver:make:Makefile#long-lines-check"}},
	} {
		plan := affected.Select(graph, []string{tc.path})
		if !ownershipRuleRemoved(plan, tc.path) {
			t.Errorf("%s %s: the ownership rule no longer owns the path; the regression exercises nothing (non-source %+v)", tc.class, tc.path, plan.NonSource)
		}
		selected := selectedSet(plan)
		for _, unit := range tc.units {
			if _, hit := selected[unit]; !hit {
				t.Errorf("%s %s: %s is not selected", tc.class, tc.path, unit)
			}
		}
	}
}

// AZU-V0-8925 (class G): go.mod sets the module path, the language version
// and every dependency version, so each Go unit with tests compiles from it.
// The plan of go.mod alone selects every one of them; a new top-level
// directory of Go tests needs its own entry in the go.mod owner.
func TestGoModSelectsEveryGoTestUnit_AZU_V0_8925(t *testing.T) {
	graph, _ := repositoryOwnershipGraph(t)
	plan := affected.Select(graph, []string{"go.mod"})
	if !ownershipRuleRemoved(plan, "go.mod") {
		t.Fatalf("the ownership rule no longer owns go.mod (non-source %+v)", plan.NonSource)
	}
	selected := selectedSet(plan)
	count := 0
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		if !strings.HasPrefix(id, "go:") || !slices.ContainsFunc(unit.Tests, func(test string) bool { return strings.HasSuffix(test, "_test.go") }) {
			continue
		}
		count++
		if _, hit := selected[id]; !hit {
			t.Errorf("go.mod alone leaves out %s", id)
		}
	}
	if count < 200 {
		t.Fatalf("only %d Go test units: the check exercised too little", count)
	}
}

// AZU-V0-8922 (the independent review of round 2): a new change-evidence
// file is a tracked file like any other, so its plan selects every declared
// whole-tree reader driver: the end-of-line, binary, stray-backup and
// tracked-ignored checks read it. The project's unbounded-reader exclusion
// (AFP-V0-772) keeps it from the tests that locate the root, not from a
// script that lists every tracked path.
func TestNewChangeEvidenceSelectsTreeReaders_AZU_V0_8922(t *testing.T) {
	graph, _ := repositoryOwnershipGraph(t)
	marked := graph.TreeReaderDrivers()
	if len(marked) < 20 {
		t.Fatalf("only %d declared tree reader drivers: the check exercises too little", len(marked))
	}
	for _, dirty := range []string{".corvint/changes/zz.cem.json", ".corvint/changes/zz.cem.json.bak", ".corvint/change.cem.json"} {
		plan := affected.Select(graph, []string{dirty})
		selected := selectedSet(plan)
		for _, target := range []string{"eol-policy-check", "binary-blobs-check", "stray-backups-check", "tracked-ignored-check"} {
			if _, hit := selected["driver:make:Makefile#"+target]; !hit {
				t.Errorf("%s: %s is not selected (scope %s, %d unknown)", dirty, target, plan.Scope, len(plan.Unknown))
			}
		}
		var missed []string
		for id := range marked {
			_, target, _ := strings.Cut(id, "#")
			if _, hit := selected[id]; !hit && target != "*" && !strings.Contains(target, "%") {
				missed = append(missed, id)
			}
		}
		slices.Sort(missed)
		if len(missed) != 0 {
			t.Errorf("%s: %d declared tree reader drivers are left out: %v", dirty, len(missed), missed)
		}
	}
}

// wholeTreeForms are the textual forms the AZU-V0-8923 guard reads as an
// enumeration of the tracked tree or a directory walk, each with a line that
// spells it. A file that matches one is a declared tree reader or a reasoned
// exception. TestWholeTreeMarkerForms_AZU_V0_8923 holds the list to its
// examples and names the forms the guard cannot see.
var wholeTreeForms = []struct{ name, pattern, example string }{
	{"git ls-files, any spelling", `ls-files`, `git -c core.quotePath=false ls-files -z`},
	{"git ls-tree", `ls-tree`, `git ls-tree -r --name-only HEAD`},
	{"git grep", `\bgit\s+(?:-C\s+\S+\s+)?grep\b`, `git -C "$root" grep -l TODO`},
	{"git grep as an argument list", `['"]git['"]\s*,\s*['"]grep['"]`, `subprocess.run(["git", "grep", "-l", "TODO"])`},
	{"a git listing of changed names", `--name-only|--name-status`, `git diff --name-only "$base" --`},
	{"find over the root or a variable", `\bfind\s+(?:\.|"?\$)`, `find . -name '*.orig'`},
	{"ripgrep", `\brg\s`, `rg -l TODO`},
	{"recursive grep", `\bgrep\s+(?:-\S+\s+)*(?:-[A-Za-z]*[rR]|--recursive|--dereference-recursive)`, `grep -n -rl TODO .`},
	{"recursive ls", `\bls\s+(?:-\S+\s+)*-[A-Za-z]*R`, `ls -laR`},
	{"shell globstar", `\*\*/`, `for f in **/*.md; do :; done`},
	{"Python os.walk", `os\.walk\(`, `for d, _, names in os.walk(root):`},
	{"Python os.scandir", `os\.scandir\(`, `with os.scandir(root) as it:`},
	{"Python os.listdir", `os\.listdir\(`, `for name in os.listdir(root):`},
	{"Python glob and pathlib glob", `\bi?glob\(`, `glob.glob("**/*.md", recursive=True)`},
	{"Python pathlib rglob", `\.rglob\(`, `root.rglob("*")`},
	{"Python pathlib iterdir", `\.iterdir\(`, `for child in root.iterdir():`},
	{"Go filepath.Walk and WalkDir", `filepath\.Walk`, `filepath.WalkDir(root, visit)`},
	{"Go fs.WalkDir", `fs\.WalkDir`, `fs.WalkDir(os.DirFS(root), ".", visit)`},
	{"Go os.ReadDir", `os\.ReadDir`, `entries, err := os.ReadDir(root)`},
	{"Go Glob", `(?:filepath|fs)\.Glob`, `filepath.Glob(filepath.Join(root, "*"))`},
}

// wholeTreeMarker matches a text in any of wholeTreeForms.
var wholeTreeMarker = func() *regexp.Regexp {
	patterns := make([]string, len(wholeTreeForms))
	for i, form := range wholeTreeForms {
		patterns[i] = form.pattern
	}
	return regexp.MustCompile(strings.Join(patterns, "|"))
}()

// wholeTreeUnseen are enumerations the textual guard cannot see, each with
// the rule that covers it instead. They are listed so that the bound of the
// guard is a tested statement, not an assumption.
var wholeTreeUnseen = []struct{ name, example, covered string }{
	{"a computed command", `"$lister" -z | while read -r -d '' f; do :; done`, "a computed site is selected on every dirty path (AZU-V0-001)"},
	{"a compiled tool run by go run", `go run ./tools/tree-audit`, "go run makes its target a whole-tree reader (AFP-V0-9401)"},
	{"a tool on PATH that walks by itself", `shellcheck-all`, "the V1-4685 reader audit; a new one needs a treeReaders entry by hand"},
	{"a listing read from a file", `while read -r f; do cat "$f"; done < "$manifest"`, "the manifest is a path literal, which selects the script when it changes"},
}

// AZU-V0-8923: the guard's marker covers exactly wholeTreeForms. Each form's
// example matches through its own pattern, so a pattern edit that stops
// seeing a form fails here; each wholeTreeUnseen example does not match, so
// the guard's stated bound is true.
func TestWholeTreeMarkerForms_AZU_V0_8923(t *testing.T) {
	for _, form := range wholeTreeForms {
		if !regexp.MustCompile(form.pattern).MatchString(form.example) {
			t.Errorf("%s: pattern %q does not match its example %q", form.name, form.pattern, form.example)
		}
		if !wholeTreeMarker.MatchString(form.example) {
			t.Errorf("%s: the marker does not match %q", form.name, form.example)
		}
	}
	// The forms the round 2 review found missing.
	for _, text := range []string{`grep -r TODO .`, `git ls-tree -r HEAD`, `git diff --name-only main`, `fs.WalkDir(tree, ".", visit)`, `glob.glob("*")`, `os.scandir(".")`} {
		if !wholeTreeMarker.MatchString(text) {
			t.Errorf("the marker does not match %q", text)
		}
	}
	for _, unseen := range wholeTreeUnseen {
		if wholeTreeMarker.MatchString(unseen.example) {
			t.Errorf("%s: %q matches the marker; move it to wholeTreeForms", unseen.name, unseen.example)
		}
		if unseen.covered == "" {
			t.Errorf("%s: no covering rule is named", unseen.name)
		}
	}
	// No path is read by naming a file: a plain read stays unmarked.
	for _, text := range []string{`cat docs/README.md`, `grep -n TODO docs/README.md`, `ls -la script`, `git diff --exit-code -- docs/specs/REQUIREMENTS.tsv`} {
		if wholeTreeMarker.MatchString(text) {
			t.Errorf("the marker matches the plain read %q", text)
		}
	}
}

// wholeTreeException is one driver-run file that matches wholeTreeMarker but
// reads no path the ownership rule owns. scope, when set, is every owner
// pattern (AFP-V0-2481) its enumeration reads within the tracked tree, and the
// guard checks that no ownership-rule path falls in it; without scope the
// reason says why the enumeration reads no tracked path at all.
type wholeTreeException struct {
	scope  []string
	reason string
}

// wholeTreeExceptions are the reasoned exceptions of AZU-V0-8923. A file that
// reads the content or the name of every tracked path is a treeReaders entry
// instead.
var wholeTreeExceptions = map[string]wholeTreeException{
	".github/consumers/gate.py":          {nil, "git diff --name-only lists the paths the pull request merge commit changed, only to decide whether the VS Code extension suite runs; any doubt runs it, and it reads no ownership-rule path"},
	".github/forward-rows/collect.sh":    {nil, "find runs over the downloaded artifact directory, never the repository"},
	"script/acceptance-log-audit.sh":     {[]string{"docs/specs/openspec/changes/**"}, "find lists the change folders under docs/specs/openspec/changes"},
	"script/analyzer-inputs.sh":          {nil, "git diff --name-only lists go.mod, go.sum and the audited internal Go files that differ from a revision; its one caller in a recipe, the analyzer schema check, prints the list as a note that never changes the exit status"},
	"script/batch-assemble.sh":           {nil, "git merge-tree --name-only prints the conflicted paths of the lane merges it assembles: the result of merging other branches, never a read of this tree"},
	"script/batch-regen.sh":              {nil, "git ls-files lists only untracked files, to report what the regeneration wrote"},
	"script/batch-review-packet.sh":      {nil, "git diff --name-only lists the paths each first-parent merge of the reviewed range changed, to print the overlap table of a review packet: it reads the range's history, judges nothing, and no path changes its exit status"},
	"script/check-ci-least-privilege.sh": {[]string{".github/workflows/*.yml", ".github/workflows/*.yaml"}, "git ls-files lists the workflow files; its exception table is a named path"},
	"script/check-decision-numbers.sh":   {[]string{"docs/decisions/**"}, "git ls-files lists docs/decisions"},
	"script/check-spec-claims.sh":        {[]string{"docs/specs/**"}, "git ls-files lists docs/specs"},
	"script/check-test-script-refs.sh":   {[]string{"**/*_test.sh"}, "git ls-files lists the shell tests"},
	"script/doc-gates-summary.sh":        {nil, "git diff --name-only decides only whether a hint about changed help sources is printed; the exit status is the wrapped command's"},
	"script/gate-receipt":                {nil, "git ls-files -v reads the index flags, which no path edit changes, and lists ignored Go files"},
	"script/gen-documentation-paths.sh": {
		[]string{"docs/*.md", "integrations/README.md", "integrations/*/README.md",
			"AGENTS.md", "CLAUDE.md", "internal/nativelinecov/README.md"},
		"git ls-files lists the documentation paths it generates the declaration from",
	},
	"script/gen-spec-requirements.sh":            {[]string{"docs/specs/**"}, "git ls-files lists docs/specs; its one other input is a path literal, which selects it"},
	"script/go-archive-gate":                     {nil, "find counts the files of its temporary release directory"},
	"script/go-cache-trim.sh":                    {nil, "find walks the Go build cache outside the repository"},
	"script/heavy.sh":                            {nil, "a command runner: it lists untracked files and walks its own output and copied worktree to ship them; the reads are its command's, which the command's own driver models"},
	"script/lane-dispatch-check.sh":              {nil, "git ls-tree -d tests whether one directory a ticket names exists at a commit; it lists no tree"},
	"script/release-checklist":                   {nil, "git ls-files tests three named legacy paths"},
	"script/repin-use-case-receipts.sh":          {nil, "git diff --name-only feeds the informational list of changed paths no receipt pins (UCV0-018), which never changes the exit status or what is repinned"},
	"script/requirement-id-collisions.sh":        {nil, "git grep appears only in a printed note"},
	"script/review-head-check.sh":                {nil, "git diff --name-only lists what changed between a reviewed head and HEAD, both taken from the saved review the caller passes; it is a step of the Codex review flow that refuses without a review, not a check a make target or workflow job runs on the tree"},
	"script/scratch-sweep.sh":                    {nil, "find and git ls-files walk scratch worktrees outside the repository"},
	"script/spec-requirements-resolve.sh":        {[]string{"docs/specs/REQUIREMENTS.tsv", "docs/specs/INDEX.json", "docs/specs/README.md"}, "git grep reads the three generated spec files for conflict markers"},
	"script/spec-requirements-worktree-check.sh": {[]string{"docs/specs/**"}, "git diff --name-only lists the unstaged docs/specs files"},
}

// ownershipRulePaths returns the paths the ownership rule takes out of
// unknown, by one plan of all of them: a path classified as a declared test
// owner path or a declared non-source path (AZU-V0-002, AFP-V0-2482). Each
// path's classification is its own, whatever the other dirty paths.
func ownershipRulePaths(graph *affected.Graph, paths []string) map[string]bool {
	removed := map[string]bool{}
	if len(paths) == 0 {
		return removed
	}
	for _, group := range affected.Select(graph, paths).NonSource {
		if group.Class == affected.ClassDeclaredTestOwner || group.Class == affected.ClassDeclaredNonSource {
			for _, path := range group.Paths {
				removed[path] = true
			}
		}
	}
	return removed
}

// AZU-V0-8923: every testless file a make target or workflow job runs that
// enumerates the tree is either a declared whole-tree reader, so the drivers
// that run it are selected on every dirty path, or a reasoned exception. A
// new driver-run whole-tree script fails here until it is classified. The
// marker is textual, so an enumeration it cannot see (a computed command, a
// compiled tool) stays outside this guard: `go run` already makes its target
// a tree reader (AFP-V0-9401), a computed site is selected on every dirty
// path, and the reader audit of V1-4685 covers the rest by hand.
func TestDriverRunWholeTreeScriptsAreDeclared_AZU_V0_8923(t *testing.T) {
	graph, tracked := repositoryOwnershipGraph(t)
	root := repositoryRoot(t)
	marked := graph.TreeReaderDrivers()
	// The declaration itself, not the marked drivers: a declared file whose
	// drivers an earlier entry already marked is still declared.
	declared := map[string]bool{}
	for _, entry := range graph.DeclaredTreeReaders() {
		declared[entry.Runs] = true
	}
	// covered reports a driver selected on every dirty path whatever it
	// runs: a computed site, or a recipe that enumerates the tree itself.
	covered := func(id string) bool {
		unit, _ := graph.Unit(id)
		_, byDeclaration := marked[id]
		return slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation) || !byDeclaration && slices.Contains(unit.PathGlobs, "/*")
	}
	flagged := map[string]bool{}
	scoped := map[string][]string{}
	for _, file := range tracked {
		owner, owned := graph.OwnerOf(file)
		if !owned || strings.HasPrefix(owner, "go:") || strings.HasPrefix(owner, affected.DriverNamespace+":") {
			continue
		}
		if unit, _ := graph.Unit(owner); len(unit.Tests) != 0 {
			continue
		}
		runners := slices.DeleteFunc(graph.DriversRunning(file), covered)
		if len(runners) == 0 && !declared[file] {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !wholeTreeMarker.Match(body) {
			continue
		}
		flagged[file] = true
		if exception, excepted := wholeTreeExceptions[file]; excepted {
			if declared[file] {
				t.Errorf("%s is both a declared tree reader and an exception", file)
			}
			for _, path := range tracked {
				if slices.ContainsFunc(exception.scope, func(pattern string) bool { return affected.OwnerPatternMatches(pattern, path) }) {
					scoped[path] = append(scoped[path], file)
				}
			}
			continue
		}
		if !declared[file] {
			t.Errorf("%s enumerates the tree and %v run it: declare it in treeReaders of %s, or add it to wholeTreeExceptions with the reason it reads no ownership-rule path (AZU-V0-8923)", file, runners, affected.DeclaredSelectionPath)
		}
	}
	// One plan of every scoped path names those the ownership rule owns.
	paths := make([]string, 0, len(scoped))
	for path := range scoped {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	owned := ownershipRulePaths(graph, paths)
	for _, path := range paths {
		if owned[path] {
			t.Errorf("exceptions %v read %s, which the ownership rule owns: declare the file in treeReaders instead (AZU-V0-8923)", scoped[path], path)
		}
	}
	for file := range wholeTreeExceptions {
		if !flagged[file] {
			t.Errorf("stale exception %s: no driver runs it or it no longer enumerates the tree", file)
		}
	}
	for file := range declared {
		if !flagged[file] {
			t.Logf("declared tree reader %s carries no textual marker", file)
		}
	}
}

// namedPathToken is the AFP-V0-012 path lexicon the completeness guard reads
// a file with.
var namedPathToken = regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)

// ownedPathNamerExceptions are the tracked tests and scripts that spell an
// ownership-rule path without reading it, each with the reason
// (AZU-V0-8924).
var ownedPathNamerExceptions = map[string]string{
	"cmd/corvint-js-test-provider/runner_control_test.go":    "lists .corvint/.gitignore among the files the provider writes in its temporary root",
	"internal/gateselect/select_test.go":                     "writes the .taskman paths as fixtures of a temporary repository",
	"internal/liveverify/affected/documentation_test.go":     "plans the .taskman paths as dirty inputs over a temporary fixture",
	"internal/liveverify/affected/readscope_literal_test.go": "plans .taskman paths as dirty inputs and scope entries over a temporary fixture",
	"internal/liveverify/affected/replay_v14446_test.go":     "writes the Ruby fixture paths into a temporary repository and plans them",
	"internal/liveverify/affected/shell/hidden_test.go":      "writes .github/forward-rows/bar.jq as a fixture of a temporary root",
	"internal/liveverify/affected/taskstate_test.go":         "plans the .taskman paths as dirty inputs over a temporary fixture",
	"internal/noncodemap/decide_test.go":                     "uses .github/cishards/package-costs.json as a classification input string; no file is read",
	"internal/testevidence/gitignore_test.go":                "writes and reads .corvint/.gitignore in its temporary directory",
	"tools/str-parity/main_test.go":                          "expects .github/cishards/test-commands.json in a message; the test reads no repository file",
}

// AZU-V0-8924: every tracked test or driver-run script whose text spells a
// path the ownership rule takes out of unknown is reached when that path
// changes alone: the test's unit is selected, and every driver that runs the
// script is. A reader the planner's rules miss fails here until a declared
// owner, a tree reader or a reasoned exception covers it. The guard reads
// exact repository paths of two or more components outside `#` and `//`
// comment lines; a lone file name (README.md, VERSION) names a file in any
// directory and is mostly a fixture name, and a read assembled from parts
// (os.path.join) or of a whole directory stays with the reader rules and the
// V1-4685 reader audit.
func TestOwnedPathNamersAreReached_AZU_V0_8924(t *testing.T) {
	graph, tracked := repositoryOwnershipGraph(t)
	root := repositoryRoot(t)
	isTracked := make(map[string]bool, len(tracked))
	for _, path := range tracked {
		isTracked[path] = true
	}
	marked := graph.TreeReaderDrivers()
	covered := func(id string) bool {
		unit, _ := graph.Unit(id)
		_, byDeclaration := marked[id]
		return slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation) || !byDeclaration && slices.Contains(unit.PathGlobs, "/*")
	}
	// A pattern rule or a whole-file unit is reached but never run as a
	// selection (driverSelectable).
	runnable := func(id string) bool {
		_, target, _ := strings.Cut(id, "#")
		return target != "*" && !strings.Contains(target, "%")
	}
	type namer struct {
		file, unit string
		drivers    []string
	}
	names := map[string][]namer{}
	for _, file := range tracked {
		if !repositoryTestFile(file) && (strings.Contains("/"+file, "/testdata/") || strings.Contains("/"+file, "/fixture/") || strings.Contains("/"+file, "/fixtures/")) {
			continue
		}
		owner, owned := graph.OwnerOf(file)
		if !owned || strings.HasPrefix(owner, affected.DriverNamespace+":") {
			continue
		}
		unit, _ := graph.Unit(owner)
		current := namer{file: file, unit: owner}
		switch {
		case slices.Contains(unit.Tests, file):
		case len(unit.Tests) == 0 && !strings.HasPrefix(owner, "go:"):
			current.drivers = slices.DeleteFunc(graph.DriversRunning(file), covered)
			if len(current.drivers) == 0 {
				continue
			}
		default:
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, line := range strings.Split(string(body), "\n") {
			// A comment names, and a //corvint:fixture-path line spells a
			// fixture of a temporary tree; neither reads the path.
			if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") || strings.Contains(line, "//corvint:fixture-path") {
				continue
			}
			for _, token := range namedPathToken.FindAllString(line, -1) {
				// `${1:-PATH}` lexes as `-PATH`; an option is no path.
				token = strings.TrimPrefix(strings.TrimLeft(token, "-"), "./")
				if strings.Contains(token, "/") && token != file && isTracked[token] && !seen[token] {
					seen[token] = true
					names[token] = append(names[token], current)
				}
			}
		}
	}
	paths := make([]string, 0, len(names))
	for path := range names {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	owned := ownershipRulePaths(graph, paths)
	checked, excepted := 0, map[string]bool{}
	for _, path := range paths {
		if !owned[path] {
			continue
		}
		selected := selectedSet(affected.Select(graph, []string{path}))
		reached := map[string]bool{}
		for _, id := range graph.ReachedUnitIDs([]string{path}) {
			reached[id] = true
		}
		for _, current := range names[path] {
			checked++
			missed := []string{}
			if current.drivers == nil {
				if _, hit := selected[current.unit]; !hit && !reached[current.unit] {
					missed = append(missed, current.unit)
				}
			}
			for _, id := range current.drivers {
				if _, hit := selected[id]; !hit && runnable(id) {
					missed = append(missed, id)
				}
			}
			if len(missed) == 0 {
				continue
			}
			if _, ok := ownedPathNamerExceptions[current.file]; ok {
				excepted[current.file] = true
				continue
			}
			t.Errorf("%s names %s, which the ownership rule owns, but %v is not reached from it alone: declare an owner or a tree reader, or add a reasoned exception (AZU-V0-8924)", current.file, path, missed)
		}
	}
	for file := range ownedPathNamerExceptions {
		if !excepted[file] {
			t.Errorf("stale exception %s: every ownership-rule path it names reaches it", file)
		}
	}
	if checked < 50 {
		t.Fatalf("only %d (file, path) pairs checked: the guard exercised too little", checked)
	}
	t.Logf("%d (file, path) pairs over %d ownership-rule paths checked", checked, len(owned))
}
