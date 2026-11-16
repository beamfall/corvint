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
		// N1: the Python tests read the cost and split tables; the Python
		// plugin carries no path literal, so the owner names them.
		{"N1", ".github/cishards/package-costs.json", []string{"python:.github.macos.shard_test", "python:tools.ci-reuse-replay.replay_test"}},
		{"N1", ".github/cishards/test-split-allow.json", []string{"python:tools.ci-reuse-replay.replay_test"}},
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

// wholeTreeMarker matches a text that enumerates the tracked tree or walks
// a directory: git ls-files or git grep in any spelling, find, rg, or a
// Python or Go directory walk (AZU-V0-8923).
var wholeTreeMarker = regexp.MustCompile(`ls-files|\bgit\s+(?:-C\s+\S+\s+)?grep\b|['"]git['"]\s*,\s*['"]grep['"]|\bfind\s+(?:\.|"?\$)|\brg\s|os\.walk\(|\.rglob\(|filepath\.Walk`)

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
	".github/forward-rows/collect.sh":     {nil, "find runs over the downloaded artifact directory, never the repository"},
	"script/acceptance-log-audit.sh":      {[]string{"docs/specs/openspec/changes/**"}, "find lists the change folders under docs/specs/openspec/changes"},
	"script/batch-regen.sh":               {nil, "git ls-files lists only untracked files, to report what the regeneration wrote"},
	"script/check-ci-least-privilege.sh":  {[]string{".github/workflows/*.yml", ".github/workflows/*.yaml"}, "git ls-files lists the workflow files; its exception table is a named path"},
	"script/check-decision-numbers.sh":    {[]string{"docs/decisions/**"}, "git ls-files lists docs/decisions"},
	"script/check-spec-claims.sh":         {[]string{"docs/specs/**"}, "git ls-files lists docs/specs"},
	"script/check-test-script-refs.sh":    {[]string{"**/*_test.sh"}, "git ls-files lists the shell tests"},
	"script/gate-receipt":                 {nil, "git ls-files -v reads the index flags, which no path edit changes, and lists ignored Go files"},
	"script/gen-documentation-paths.sh":   {[]string{"docs/*.md", "integrations/README.md", "integrations/*/README.md", "AGENTS.md", "CLAUDE.md", "internal/nativelinecov/README.md", "internal/runnerprotocol/README.md", "tools/runner-plugin/README.md", "tools/runner-reference/README.md"}, "git ls-files lists the documentation paths it generates the declaration from"},
	"script/gen-spec-requirements.sh":     {[]string{"docs/specs/**"}, "git ls-files lists docs/specs; its one other input is a path literal, which selects it"},
	"script/go-archive-gate":              {nil, "find counts the files of its temporary release directory"},
	"script/go-cache-trim.sh":             {nil, "find walks the Go build cache outside the repository"},
	"script/heavy.sh":                     {nil, "a command runner: it lists untracked files and walks its own output and copied worktree to ship them; the reads are its command's, which the command's own driver models"},
	"script/release-checklist":            {nil, "git ls-files tests three named legacy paths"},
	"script/requirement-id-collisions.sh": {nil, "git grep appears only in a printed note"},
	"script/scratch-sweep.sh":             {nil, "find and git ls-files walk scratch worktrees outside the repository"},
	"script/spec-requirements-resolve.sh": {[]string{"docs/specs/REQUIREMENTS.tsv", "docs/specs/INDEX.json", "docs/specs/README.md"}, "git grep reads the three generated spec files for conflict markers"},
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
	declared := map[string]bool{}
	for _, runs := range marked {
		declared[runs] = true
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
