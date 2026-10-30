package rubyprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
)

// fixtureRoot is the conformance/ruby-profile-v0 fixture (RUBY-AFFECTED-V0-014).
var fixtureRoot = filepath.Join("..", "..", "conformance", "ruby-profile-v0")

// The fixture classes the row table must cover: the positive classes the
// #1130 profile names and the dynamic lookups that must stay unknown.
var (
	positiveClasses = []string{"autoloaded-constant", "described-class", "shared-examples", "support-helper", "request-spec", "controller-spec", "worker-spec", "rails-7-1-autoload-lib"}
	negativeClasses = []string{"dynamic-constantize", "dynamic-const-get"}
)

const (
	dynamicLookupUnknown = "ruby-constant-lookup-dynamic"
	autoloadFrontier     = affected.UnknownLanguageFrontier + " " + ruby.FrontierRailsAutoload
)

type rowTable struct {
	Profile     string         `json:"profile"`
	Application string         `json:"application"`
	Report      string         `json:"report"`
	Classes     []fixtureClass `json:"classes"`
	Impact      []impactRow    `json:"impact"`
	Affected    []affectedRow  `json:"affected"`
	Provider    []providerRow  `json:"provider"`
}

type fixtureClass struct {
	Class    string `json:"class"`
	Changed  string `json:"changed"`
	Overlay  string `json:"overlay,omitempty"`
	Negative bool   `json:"negative,omitempty"`
}

// impactRow is the expected context impact of the class's changed path:
// every result as "kind id" and every unknown code, both sorted.
type impactRow struct {
	Class    string   `json:"class"`
	Results  []string `json:"results"`
	Unknowns []string `json:"unknowns"`
}

// affectedRow is the expected selection: scope, each selected spec with its
// witness kind, and every widening as "REASON detail".
type affectedRow struct {
	Class    string            `json:"class"`
	Scope    string            `json:"scope"`
	Selected map[string]string `json:"selected"`
	Unknown  []string          `json:"unknown"`
}

// providerRow is the expected corvint-ruby-test-provider/0 example and its
// testvalidity projection from the recorded report; freshnessAfterChange is
// the document's freshness once the class's changed path is edited.
type providerRow struct {
	Class                string `json:"class"`
	Example              string `json:"example"`
	RSpecID              string `json:"rspecId"`
	Status               string `json:"status"`
	Association          string `json:"association"`
	Execution            string `json:"execution"`
	Hygiene              string `json:"hygiene"`
	Strength             string `json:"strength"`
	Freshness            string `json:"freshness"`
	FreshnessAfterChange string `json:"freshnessAfterChange"`
}

func loadTable(t *testing.T) rowTable {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureRoot, "rows.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var table rowTable
	if err := decoder.Decode(&table); err != nil {
		t.Fatalf("rows.json: %v", err)
	}
	if table.Profile != "corvint-ruby-profile-rows/0" {
		t.Fatalf("rows.json profile = %q", table.Profile)
	}
	return table
}

func (table rowTable) class(t *testing.T, name string) fixtureClass {
	t.Helper()
	for _, class := range table.Classes {
		if class.Class == name {
			return class
		}
	}
	t.Fatalf("row names undeclared class %q", name)
	return fixtureClass{}
}

// materialize copies the application, plus the class's overlay, into a fresh
// directory and, when commit is set, commits it to a new Git repository.
func materialize(t *testing.T, table rowTable, class fixtureClass, commit bool) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sources := []string{filepath.Join(fixtureRoot, table.Application)}
	if class.Overlay != "" {
		sources = append(sources, filepath.Join(fixtureRoot, filepath.FromSlash(class.Overlay)))
	}
	for _, source := range sources {
		copyTree(t, source, root)
	}
	if commit {
		git(t, root, "init", "-q", "-b", "main")
		git(t, root, "config", "user.email", "conformance@example.invalid")
		git(t, root, "config", "user.name", "conformance")
		git(t, root, "add", ".")
		git(t, root, "commit", "-q", "-m", "ruby profile fixture")
	}
	return root
}

func copyTree(t *testing.T, source, target string) {
	t.Helper()
	err := filepath.WalkDir(source, func(name string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	var environment []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			environment = append(environment, entry)
		}
	}
	command.Env = append(environment, "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func sameStrings(got, want []string) bool {
	return slices.Equal(got, want) || len(got) == 0 && len(want) == 0
}

// TestRubyProfileImpactRows_RUBYAFFECTEDV0014 replays each class's changed
// path through context impact and compares results and unknowns exactly.
func TestRubyProfileImpactRows_RUBYAFFECTEDV0014(t *testing.T) {
	requireGit(t)
	table := loadTable(t)
	for _, row := range table.Impact {
		t.Run(row.Class, func(t *testing.T) {
			class := table.class(t, row.Class)
			index, err := contextindex.Build(context.Background(), materialize(t, table, class, true))
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := contextindex.Impact(index, []string{class.Changed}, 50)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Results []struct {
					ID   string `json:"id"`
					Kind string `json:"kind"`
				} `json:"results"`
				Unknowns []struct {
					Code string `json:"code"`
				} `json:"unknowns"`
			}
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			var results, unknowns []string
			for _, result := range decoded.Results {
				results = append(results, result.Kind+" "+result.ID)
			}
			for _, unknown := range decoded.Unknowns {
				unknowns = append(unknowns, unknown.Code)
			}
			slices.Sort(results)
			slices.Sort(unknowns)
			if !sameStrings(results, row.Results) || !sameStrings(unknowns, row.Unknowns) {
				t.Fatalf("impact of %s:\n results  %q\n want     %q\n unknowns %q\n want     %q", class.Changed, results, row.Results, unknowns, row.Unknowns)
			}
		})
	}
}

// TestRubyProfileAffectedRows_RUBYAFFECTEDV0014 replays each class's changed
// path through the Ruby affected plugin and compares scope, selection with
// witnesses, and widenings exactly.
func TestRubyProfileAffectedRows_RUBYAFFECTEDV0014(t *testing.T) {
	table := loadTable(t)
	for _, row := range table.Affected {
		t.Run(row.Class, func(t *testing.T) {
			class := table.class(t, row.Class)
			graph, err := affected.Build(context.Background(), materialize(t, table, class, false), ruby.New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{class.Changed})
			selected := map[string]string{}
			for _, selection := range plan.Selected {
				for _, test := range selection.Tests {
					selected[test] = selection.Witness.Kind
				}
			}
			var unknown []string
			for _, widening := range plan.Unknown {
				unknown = append(unknown, widening.Reason+" "+widening.Detail)
			}
			slices.Sort(unknown)
			unknown = slices.Compact(unknown)
			wantSelected := row.Selected
			if wantSelected == nil {
				wantSelected = map[string]string{}
			}
			if plan.Scope != row.Scope || !equalMaps(selected, wantSelected) || !sameStrings(unknown, row.Unknown) {
				t.Fatalf("selection for %s:\n scope    %s want %s\n selected %v\n want     %v\n unknown  %q\n want     %q", class.Changed, plan.Scope, row.Scope, selected, wantSelected, unknown, row.Unknown)
			}
		})
	}
}

func equalMaps(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
}

// TestRubyProfileProviderRows_RUBYAFFECTEDV0014 runs the Ruby test provider
// over the committed fixture with a stand-in rspec that emits the recorded
// RSpec JSON report, then compares each class's example and projection.
func TestRubyProfileProviderRows_RUBYAFFECTEDV0014(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in rspec needs a POSIX shell")
	}
	requireGit(t)
	table := loadTable(t)
	root := materialize(t, table, fixtureClass{}, true)
	document, err := rubytestprovider.Run(context.Background(), rubytestprovider.Config{Dir: root, RSpec: []string{standInRSpec(t, table)}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := rubytestprovider.Validate(document); err != nil {
		t.Fatal(err)
	}
	if document.Infrastructure != nil {
		t.Fatalf("infrastructure = %+v", document.Infrastructure)
	}
	freshness := func() string {
		tree, err := os.OpenRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		defer tree.Close()
		read := func(name string) ([]byte, error) { return tree.ReadFile(name) }
		helpers := func(project string) ([]string, error) { return rubytestprovider.CurrentHelpers(tree, project) }
		return rubytestprovider.Freshness(document, read, helpers).State
	}
	current := freshness()
	examples := map[string]rubytestprovider.Example{}
	for _, example := range document.Examples {
		examples[example.ID] = example
	}
	for _, row := range table.Provider {
		t.Run(row.Class, func(t *testing.T) {
			class := table.class(t, row.Class)
			example, found := examples[row.Example]
			if !found {
				t.Fatalf("example %s not observed; have %v", row.Example, slices.Sorted(mapKeys(examples)))
			}
			projection := rubytestprovider.ProjectExample(document, example)
			got := providerRow{
				Class: row.Class, Example: example.ID, RSpecID: example.RSpecID, Status: example.Status,
				Association: projection.Association.State, Execution: projection.Execution.State,
				Hygiene: projection.Hygiene.State, Strength: projection.Strength.State, Freshness: current,
			}
			changed := filepath.Join(root, filepath.FromSlash(class.Changed))
			original, err := os.ReadFile(changed)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(changed, append(slices.Clone(original), "# edited\n"...), 0o644); err != nil {
				t.Fatal(err)
			}
			got.FreshnessAfterChange = freshness()
			if err := os.WriteFile(changed, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if got != row {
				t.Fatalf("provider row:\n got  %+v\n want %+v", got, row)
			}
		})
	}
}

func mapKeys(examples map[string]rubytestprovider.Example) func(func(string) bool) {
	return func(yield func(string) bool) {
		for key := range examples {
			if !yield(key) {
				return
			}
		}
	}
}

// standInRSpec writes a shell rspec that copies the recorded report to the
// JSON --out path the provider names; it executes no Ruby.
func standInRSpec(t *testing.T, table rowTable) string {
	t.Helper()
	report, err := filepath.Abs(filepath.Join(fixtureRoot, table.Report))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nformat=\nprevious=\nfor argument in \"$@\"; do\n" +
		"  case \"$previous\" in\n" +
		"    --format) format=\"$argument\" ;;\n" +
		"    --out) if [ \"$format\" = json ]; then cat '" + report + "' > \"$argument\"; fi ;;\n" +
		"  esac\n  previous=\"$argument\"\ndone\nexit 0\n"
	path := filepath.Join(t.TempDir(), "rspec")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRubyProfileRowsCoverEveryClass_RUBYAFFECTEDV0014 keeps the table
// complete: every positive class has one impact, affected, and provider row
// and stays BOUNDED; every negative class has impact and affected rows that
// keep the dynamic lookup unknown.
func TestRubyProfileRowsCoverEveryClass_RUBYAFFECTEDV0014(t *testing.T) {
	table := loadTable(t)
	declared := map[string]fixtureClass{}
	for _, class := range table.Classes {
		if _, duplicate := declared[class.Class]; duplicate {
			t.Fatalf("class %s declared twice", class.Class)
		}
		if class.Negative && class.Overlay == "" {
			t.Fatalf("class %s: a negative class must carry an overlay", class.Class)
		}
		declared[class.Class] = class
	}
	if len(declared) != len(positiveClasses)+len(negativeClasses) {
		t.Fatalf("classes = %v", slices.Sorted(func(yield func(string) bool) {
			for name := range declared {
				if !yield(name) {
					return
				}
			}
		}))
	}
	count := func(rows int, match func(int) bool) int {
		total := 0
		for index := range rows {
			if match(index) {
				total++
			}
		}
		return total
	}
	for _, name := range append(slices.Clone(positiveClasses), negativeClasses...) {
		class, found := declared[name]
		if !found || class.Negative != slices.Contains(negativeClasses, name) {
			t.Fatalf("class %s missing or misclassified", name)
		}
		impacts := count(len(table.Impact), func(i int) bool { return table.Impact[i].Class == name })
		selections := count(len(table.Affected), func(i int) bool { return table.Affected[i].Class == name })
		providers := count(len(table.Provider), func(i int) bool { return table.Provider[i].Class == name })
		wantProviders := 1
		if class.Negative {
			wantProviders = 0
		}
		if impacts != 1 || selections != 1 || providers != wantProviders {
			t.Fatalf("class %s: impact=%d affected=%d provider=%d rows", name, impacts, selections, providers)
		}
	}
	for _, row := range table.Impact {
		if declared[row.Class].Negative != slices.Contains(row.Unknowns, dynamicLookupUnknown) {
			t.Fatalf("impact row %s: %s only and always for negative classes", row.Class, dynamicLookupUnknown)
		}
		if !declared[row.Class].Negative && len(row.Results) == 0 {
			t.Fatalf("impact row %s has no expected results", row.Class)
		}
	}
	for _, row := range table.Affected {
		negative := declared[row.Class].Negative
		switch {
		case negative && (row.Scope != affected.ScopeUnknown || !slices.ContainsFunc(row.Unknown, func(unknown string) bool {
			// The site is located (RUBY-AFFECTED-V0-046): reason:path:line.
			return strings.HasPrefix(unknown, autoloadFrontier+":")
		})):
			t.Fatalf("affected row %s must stay UNKNOWN with a located %s", row.Class, autoloadFrontier)
		case !negative && (row.Scope != affected.ScopeBounded || len(row.Unknown) != 0 || len(row.Selected) == 0):
			t.Fatalf("affected row %s must be BOUNDED with a selection", row.Class)
		}
	}
	if _, err := os.Stat(filepath.Join(fixtureRoot, table.Report)); errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recorded report %s missing", table.Report)
	}
}
