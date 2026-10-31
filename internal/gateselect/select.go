// Package gateselect is the AFP-V0-012 attribution behind
// tools/gate-affected-select (AFP-V0-011, AFP-V0-012) and `corvint test`
// (STR-V0-002). Select reads one affected-plan receipt, indexes the repository
// under test (readers.go), and returns one audit line per decision
// (`selected <pkg>` from the plan; `frontier <pkg> <- <path>` for a package or
// importer a dirty Go source reaches; `data <path>` for any other dirty path;
// `documentation <path>` for a declared documentation path whose rule (c)
// readers SelectStructural takes from the plan's selections, ADN-V0-010;
// `evidence <path>` for a sealed CEM under .corvint/changes/ whose rule (c)
// readers SelectStructural takes from the plan likewise, ADN-V0-011;
// `reader <pkg> <- <path>` for a package that encloses or names a dirty path;
// `unresolved <pkg>: <reason>` for a package whose reads no literal bounds,
// selected whenever any path is dirty; `declared <pkg> <- <path>` for a package
// whose declared read scope holds a dirty path, AFP-V0-023, where an edit of the
// declaration itself is measured against the plan's base, AFP-V0-050), then one verdict
// as the last line:
// `run <pkgs>`, `FALLBACK <reason>`, or `NOTHING <reason>`. A dirty path carrying
// a control character falls back before any line echoes it, so the verdict
// stays the last line. Package paths are validated before they reach a shell
// word.
//
// Unresolved returns only the `unresolved <pkg>: <reason>` lines for a root;
// Bounds adds one `bound <pkg> <rule> <path>` line per path rules (a) to (c)
// attribute to each resolved package (tools/gate-ledger, GL-V0-004/009).
package gateselect

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/taskprojection"
)

// Receipt is the part of an affected-plan receipt the selection reads.
type Receipt struct {
	Plan struct {
		Dirty   []string `json:"dirty"`
		Scope   string   `json:"scope"`
		Unknown []struct {
			Reason string `json:"reason"`
			Detail string `json:"detail"`
		} `json:"unknown"`
		// NonSource is the planner's task-state classification
		// (AFP-V0-088); taskStateOnly reads it (AFP-V0-153).
		NonSource []struct {
			Class string   `json:"class"`
			Paths []string `json:"paths"`
		} `json:"nonSource"`
		// GoAdditive is the planner's additive-change records
		// (ADN-V0-001); a NARROWED one limits its package's dependents
		// to the referencing importers (ADN-V0-006).
		GoAdditive []struct {
			Unit         string   `json:"unit"`
			State        string   `json:"state"`
			Paths        []string `json:"paths"`
			Unreferenced []string `json:"unreferenced"`
		} `json:"goAdditive"`
	} `json:"plan"`
	Provider struct {
		Go struct {
			Packages []string `json:"packages"`
			State    string   `json:"state"`
		} `json:"go"`
	} `json:"provider"`
	// Range.Base, else Revision, is the commit the plan was measured from; a
	// read-scope declaration edit is narrowed against its declaration
	// (AFP-V0-050) unless the plan is a --snapshot plan, which keeps the
	// AFP-V0-023 rule as the planner's --snapshot form does.
	Snapshot map[string]any `json:"snapshot"`
	Range    struct {
		Base string `json:"base"`
	} `json:"range"`
	Revision string `json:"revision"`
}

var moduleLevelFrontiers = map[string]bool{
	"go:module-path-unresolved":          true,
	"go:included-directory-walk-bounded": true,
	"go:unparsed-source":                 true,
	"go:cgo-frontier":                    true,
	"go:test-read-scopes-invalid":        true,
	"go:test-binary-execs-invalid":       true,
	// TSP-V0-940: the planner leaves an invalid or dirty gated root-locator
	// declaration UNKNOWN, so the selector does not narrow under it.
	"go:gated-root-locators-invalid": true,
	"go:gated-root-locators-changed": true,
}

var rootModuleDefinitions = map[string]bool{"go.mod": true, "go.sum": true, "go.work": true, "go.work.sum": true}

// changeEvidence is the CEM sidecar (internal/frontier.ExcludedPath) that every
// dogfooded change commits. It is derived from the rest of the diff, so a
// package's tests read it only through a literal that resolves to it from the
// package's directory; a `.corvint` or `change.cem.json` token joined to a
// fixture root does not select its holder (AFP-V0-012 (c)).
const changeEvidence = ".corvint/change.cem.json"

// sealedChanges holds the sealed CEMs a dogfood seal moves the sidecar to.
const sealedChanges = ".corvint/changes/"

// resolvedEvidence reports whether p is the sidecar or a sealed CEM directly
// under sealedChanges; both narrow rule (c) to resolvingReaders (AFP-V0-152).
func resolvedEvidence(p string) bool {
	if p == changeEvidence {
		return true
	}
	name, sealed := strings.CutPrefix(p, sealedChanges)
	return sealed && strings.HasSuffix(name, ".cem.json") && len(name) > len(".cem.json") && !strings.Contains(name, "/")
}

var plainImportPath = regexp.MustCompile(`^[A-Za-z0-9._~/-]+$`)

// MaxPlanBytes bounds a plan, and a path list, read from a file or stream.
const MaxPlanBytes = 8 << 20

// Select is selectPackages: the audit lines and the verdict as the last line.
func Select(plan Receipt, module, root string) []string {
	return selectPackages(plan, module, root, true)
}

// SelectStructural is Select without this index's own rule (d) closure
// (STR-V0-061): rules (a) to (c) and the AFP-V0-023 declarations attribute each
// dirty path, while the packages whose reads no literal bounds come from the
// plan's own `UNBOUNDED_READER` selections, which the reviewed
// `.corvint/unbounded-readers.json` set binds (AFP-V0-025, AFP-V0-033). The
// closure this index computes follows test-only imports and path tokens past
// the test binaries that link a root locator, so it over-selects; `corvint
// test` reads the plan's selections instead. tools/gate-affected-select
// (script/gate-affected.sh and the CI PR tier's frozen selector) keeps Select;
// `make gate-affected` runs `corvint test` and so follows this.
func SelectStructural(plan Receipt, module, root string) []string {
	return selectPackages(plan, module, root, false)
}

// Unresolved is unresolvedPackages.
func Unresolved(module, root string) ([]string, error) { return unresolvedPackages(module, root) }

// Bounds is packageBounds.
func Bounds(module, root string, paths []string) ([]string, error) {
	return packageBounds(module, root, paths)
}

// ReadPaths reads worktree paths, one per line, bounded by MaxPlanBytes; an
// oversized or unreadable stream yields no paths.
func ReadPaths(r io.Reader) []string {
	data, err := io.ReadAll(io.LimitReader(r, MaxPlanBytes+1))
	if err != nil || len(data) > MaxPlanBytes {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// ReadPlan reads and decodes one plan file, refusing one over MaxPlanBytes
// before decoding.
func ReadPlan(name string) (Receipt, error) {
	var plan Receipt
	file, err := os.Open(name)
	if err != nil {
		return plan, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxPlanBytes+1))
	if err != nil {
		return plan, err
	}
	if len(data) > MaxPlanBytes {
		return plan, fmt.Errorf("plan exceeds %d bytes", MaxPlanBytes)
	}
	err = json.Unmarshal(data, &plan)
	return plan, err
}

// selectPackages attributes every dirty path of plan to Go packages. withUnresolved
// adds rule (d) from this index's own root-locator closure; without it the
// plan's selected units carry rule (d) (STR-V0-061).
func selectPackages(plan Receipt, module, root string, withUnresolved bool) []string {
	var lines []string
	fallback := func(reason string) []string { return append(lines, "FALLBACK "+reason) }
	state := plan.Provider.Go.State
	if state != "RUNNABLE" && state != "EMPTY_SELECTION" {
		return fallback("provider.go.state is " + state)
	}
	for _, entry := range plan.Plan.Unknown {
		if entry.Reason == "LANGUAGE_FRONTIER" && moduleLevelFrontiers[entry.Detail] {
			return fallback("Go frontier is unknown at module level: " + entry.Detail)
		}
	}
	for _, dirty := range plan.Plan.Dirty {
		if rootModuleDefinitions[dirty] {
			return fallback("root module definition is dirty: " + dirty)
		}
	}
	for _, dirty := range plan.Plan.Dirty {
		if strings.ContainsFunc(dirty, unicode.IsControl) {
			return fallback("a dirty path contains a control character")
		}
	}
	for _, entry := range plan.Plan.Unknown {
		if strings.ContainsFunc(entry.Detail, unicode.IsControl) {
			return fallback("an unattributed path contains a control character")
		}
	}
	index, err := indexRepository(root, module)
	if err != nil {
		return fallback(fmt.Sprintf("the repository could not be indexed for attribution: %q", err.Error()))
	}
	if withUnresolved {
		if reason := unknownNarrowing(plan, index, root); reason != "" {
			return fallback(reason)
		}
	}
	packages := map[string]bool{}
	for _, pkg := range sortedUnique(plan.Provider.Go.Packages) {
		packages[pkg] = true
		lines = append(lines, "selected "+pkg)
	}
	add := func(directories []string, kind, cause string) {
		for _, directory := range directories {
			if pkg := importPath(module, directory); !packages[pkg] {
				packages[pkg] = true
				lines = append(lines, kind+" "+pkg+cause)
			}
		}
	}
	dirties := sortedUnique(plan.Plan.Dirty)
	readers := index.pathReaders(dirties)
	// The planner's narrowings apply to SelectStructural (`corvint test`)
	// only; Select, the CI PR tier's frozen selector, keeps the whole
	// attribution as it keeps rule (d) (STR-V0-061).
	var narrowed map[string]map[string]bool
	var taken map[string]string
	if !withUnresolved {
		narrowed = additiveNarrowing(plan, module)
		taken = planReaders(plan, index, module, root)
	}
	for i, dirty := range dirties {
		rules := index.attribute(dirty, readers[i])
		if unreferenced, ok := narrowed[dirty]; ok && !strings.HasSuffix(dirty, "_test.go") && index.packages[parentDirectory(dirty)] != nil {
			rules = index.attributeNarrowed(dirty, readers[i], unreferenced)
		}
		if kind := taken[dirty]; kind != "" {
			rules = append(index.attributeStructure(dirty), attribution{kind: kind})
		}
		for _, rule := range rules {
			if rule.kind == "documentation" || rule.kind == "evidence" {
				lines = append(lines, rule.kind+" "+dirty)
				continue
			}
			if rule.kind == "data" {
				lines = append(lines, "data "+dirty)
				continue
			}
			add(rule.directories, rule.kind, " <- "+dirty)
		}
	}
	scopes, err := index.readScopes(root)
	if err != nil {
		return fallback(fmt.Sprintf("the read-scope declaration is invalid: %q", err.Error()))
	}
	if withUnresolved {
		unresolved := index.unresolved()
		for _, directory := range sortedKeys(unresolved) {
			if _, declared := scopes[directory]; !declared && len(plan.Plan.Dirty) > 0 {
				add([]string{directory}, "unresolved", fmt.Sprintf(": %q", unresolved[directory]))
			}
		}
	}
	var prior map[string][]string
	if slices.Contains(plan.Plan.Dirty, readScopesPath) {
		prior = baseReadScopes(plan, root)
	}
	declared := map[string]bool{}
	for directory := range scopes {
		declared[directory] = true
	}
	for directory := range prior {
		declared[directory] = index.packages[directory] != nil
	}
	for _, directory := range sortedKeys(declared) {
		for _, dirty := range sortedUnique(plan.Plan.Dirty) {
			if declared[directory] && declaredBears(directory, scopes, prior, dirty) {
				add([]string{directory}, "declared", " <- "+dirty)
				break
			}
		}
	}
	selected := sortedKeys(packages)
	for _, pkg := range selected {
		if !plainImportPath.MatchString(pkg) || !underModule(pkg, module) {
			return fallback(fmt.Sprintf("package path is not a plain import path under the module: %q", pkg))
		}
	}
	if len(selected) == 0 && len(plan.Plan.Dirty) > 0 {
		if taskStateOnly(plan) {
			return append(lines, fmt.Sprintf("NOTHING %d dirty path(s) are Tasks store projection task state; no Go package names or reads them", len(plan.Plan.Dirty)))
		}
		return fallback(fmt.Sprintf("the selection is empty while %d path(s) are dirty", len(plan.Plan.Dirty)))
	}
	if len(selected) == 0 {
		return append(lines, "NOTHING no dirty or committed path; no Go package to test")
	}
	return append(lines, "run "+strings.Join(selected, " "))
}

// unknownNarrowing is CIN-V0-860's rule for Select, the CI PR tier: a plan
// may still run narrowed while it names unknowns only when each is a
// LANGUAGE_FRONTIER (a module-level one falls back above), an
// UNOWNED_DIRTY_PATH, a NO_SELECTABLE_TEST, or an UNINDEXED_SOURCE_PATH that
// deletedGoSource admits. CI narrows only the Go shards; rules (b) and (c)
// select every package that encloses or names a dirty path and rule (d) every
// package whose reads no literal bounds, so every Go reader of an unowned path
// is selected; a frontier names what a language model cannot see inside units
// the dirty paths already reach; and a unit with no selectable test gets no
// test from a FULL run either. Any other unknown, a malformed reason, and an
// UNKNOWN plan that names no unknown return the fallback reason.
func unknownNarrowing(plan Receipt, index *repositoryIndex, root string) string {
	if plan.Plan.Scope == "UNKNOWN" && len(plan.Plan.Unknown) == 0 {
		return "the plan is UNKNOWN but names no unknown"
	}
	for _, entry := range plan.Plan.Unknown {
		switch {
		case entry.Reason == "LANGUAGE_FRONTIER", entry.Reason == "UNOWNED_DIRTY_PATH", entry.Reason == "NO_SELECTABLE_TEST":
			continue
		case entry.Reason == "UNINDEXED_SOURCE_PATH" && deletedGoSource(plan, index, root, entry.Detail):
			continue
		}
		reason := entry.Reason
		if !unknownReason.MatchString(reason) {
			reason = "a malformed reason"
		}
		return "the plan names the unknown " + reason + ", which the selector cannot attribute"
	}
	return ""
}

// deletedGoSource reports an unindexed Go source that this change deleted from
// a directory still holding an indexed package: rule (a) attributes the
// deletion to that package and its importers (CIN-V0-860 (4)). The path must be
// a clean dirty path, absent from the tested tree.
func deletedGoSource(plan Receipt, index *repositoryIndex, root, p string) bool {
	if !fs.ValidPath(p) || !packageSource(p) || !slices.Contains(plan.Plan.Dirty, p) || index.packages[parentDirectory(p)] == nil {
		return false
	}
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(p)))
	return errors.Is(err, fs.ErrNotExist)
}

var unknownReason = regexp.MustCompile(`^[A-Z][A-Z_]{0,63}$`)

// taskStateOnly reports whether an empty selection over a non-empty diff is
// the AFP-V0-088 task-state case (AFP-V0-153): the plan is BOUNDED and every
// dirty path is in the recognised Tasks store projection layout and listed in
// the plan's TASK_STORE_PROJECTION group. The caller has already applied rules
// (a) to (d) and the declared scopes to those paths and selected nothing, so
// the gate's own attribution agrees with the planner; any other empty
// selection keeps the full-suite fallback.
func taskStateOnly(plan Receipt) bool {
	if plan.Plan.Scope != "BOUNDED" {
		return false
	}
	classified := map[string]bool{}
	for _, group := range plan.Plan.NonSource {
		if group.Class != taskprojection.Class {
			continue
		}
		for _, p := range group.Paths {
			classified[p] = true
		}
	}
	for _, dirty := range plan.Plan.Dirty {
		if !classified[dirty] || !taskprojection.IsPath(dirty) {
			return false
		}
	}
	return len(plan.Plan.Dirty) > 0
}

// attribution is one AFP-V0-012 rule applied to one path: the kind
// selectPackages prints (`frontier`, `data`, `reader`) and the package
// directories it selects; a `data` line selects none itself.
type attribution struct {
	kind        string
	directories []string
}

// attribute applies rules (a) to (c) to one path, in the order selectPackages
// prints them: (a) a package source selects its package and that directory's
// dependents; (b) any other path selects its enclosing packages, the nearest
// one's dependents when the path sits directly in it, and the dependents of
// every embedding ancestor (`//go:embed` reaches into subdirectories that are
// packages of their own); (c) every path selects the packages whose files name
// it, given as readers (index.readers(dirty), or pathReaders for many paths).
// Rule (d), the unresolved packages, is index.unresolved().
func (index *repositoryIndex) attribute(dirty string, readers []string) []attribution {
	if resolvedEvidence(dirty) {
		readers = index.resolvingReaders(dirty)
	}
	return append(index.attributeStructure(dirty), attribution{"reader", readers})
}

// planReaders returns the dirty paths whose rule (c) readers SelectStructural
// takes from the plan, each with the line kind it prints: `documentation` for
// a path the planner classified DECLARED_DOCUMENTATION (AFP-V0-160,
// ADN-V0-010), and `evidence` for a sealed CEM directly under
// .corvint/changes/ (ADN-V0-011), whose readers the planner resolves from
// each unit's directory (AFP-V0-152). In both cases the planner's reader rules
// already selected, with a witness, every unit whose path literal, glob or
// declared read scope reaches the path. It returns nil, so every path keeps
// this index's rule (c), when the plan states a unit whose path tokens its
// plugin dropped at the bound (a path-token-bound frontier, AFP-V0-021): that
// unit's reads are not in the plan's reader decision.
//
// The plan is not trusted to prove either classification. A path is taken
// only when it is dirty in the plan, a documentation path is not Go source and
// is declared by root's own documentation declaration, an evidence path is
// named <commit>.cem.json with a 40- or 64-digit lowercase hexadecimal commit,
// and every package whose literal this index resolves, from the holder's own
// directory, to the path or one of its ancestor directories is among the
// plan's selected packages, unless root's own read-scope declaration, unchanged
// in the plan, confines that package's reads outside the path (AFP-V0-161, as
// the planner). A stale or forged plan that fails any check keeps this index's
// whole rule (c) for that path.
func planReaders(plan Receipt, index *repositoryIndex, module, root string) map[string]string {
	for _, entry := range plan.Plan.Unknown {
		if entry.Reason == "LANGUAGE_FRONTIER" && strings.Contains(entry.Detail, ":path-token-bound:") {
			return nil
		}
	}
	scopes, err := index.readScopes(root)
	if err != nil {
		return nil
	}
	if slices.Contains(plan.Plan.Dirty, readScopesPath) {
		scopes = nil
	}
	planned := func(p string) bool {
		for _, directory := range index.holderReaders(p) {
			if scope, declared := scopes[directory]; declared && !inReadScope(directory, scope, p) {
				continue
			}
			if !slices.Contains(plan.Provider.Go.Packages, importPath(module, directory)) {
				return false
			}
		}
		return true
	}
	declared := documentationDeclaration(root)
	taken := map[string]string{}
	for _, group := range plan.Plan.NonSource {
		if group.Class != "DECLARED_DOCUMENTATION" {
			continue
		}
		for _, p := range group.Paths {
			if slices.Contains(plan.Plan.Dirty, p) && !packageSource(p) && declared(p) && planned(p) {
				taken[p] = "documentation"
			}
		}
	}
	for _, p := range plan.Plan.Dirty {
		if sealedEvidence(p) && planned(p) {
			taken[p] = "evidence"
		}
	}
	return taken
}

// sealedEvidence reports a sealed CEM name directly under .corvint/changes/:
// a 40- or 64-digit lowercase hexadecimal commit and `.cem.json`.
func sealedEvidence(p string) bool {
	name, sealed := strings.CutPrefix(p, sealedChanges)
	commit, named := strings.CutSuffix(name, ".cem.json")
	if !sealed || !named || (len(commit) != 40 && len(commit) != 64) {
		return false
	}
	return !strings.ContainsFunc(commit, func(r rune) bool { return (r < '0' || r > '9') && (r < 'a' || r > 'f') })
}

// documentationPathsPath is the repository's documentation declaration
// (AFP-V0-160).
const documentationPathsPath = ".corvint/documentation-paths.json"

// documentationDeclaration reads root's documentation declaration and reports
// whether a path is one of its entries or lies under a directory entry. An
// absent, oversized, unreadable or malformed declaration declares nothing,
// and an entry at or under .corvint/ declares nothing.
func documentationDeclaration(root string) func(string) bool {
	none := func(string) bool { return false }
	file, err := os.Open(path.Join(root, documentationPathsPath))
	if err != nil {
		return none
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
	if err != nil || len(body) > 64<<10 {
		return none
	}
	var declaration struct {
		Profile string   `json:"profile"`
		Paths   []string `json:"paths"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&declaration) != nil || declaration.Profile != "corvint-documentation-paths/0" {
		return none
	}
	return func(p string) bool {
		for _, entry := range declaration.Paths {
			if entry == ".corvint" || strings.HasPrefix(entry, ".corvint/") {
				continue
			}
			if p == entry || (strings.HasSuffix(entry, "/") && strings.HasPrefix(p, entry)) {
				return true
			}
		}
		return false
	}
}

// additiveNarrowing maps each dirty path of a NARROWED additive-change record
// to the package directories of the importers the planner examined and found
// to reference none of its new declarations (ADN-V0-006). An importer this
// index sees but the record does not name stays followed. A record naming a
// unit outside the module is left out, so its paths keep the whole
// dependents rule.
func additiveNarrowing(plan Receipt, module string) map[string]map[string]bool {
	directory := func(unit string) (string, bool) {
		pkg, ok := strings.CutPrefix(unit, "go:")
		if !ok {
			return "", false
		}
		if pkg == module {
			return "", true
		}
		return strings.CutPrefix(pkg, module+"/")
	}
	narrowed := map[string]map[string]bool{}
	for _, record := range plan.Plan.GoAdditive {
		if record.State != "NARROWED" {
			continue
		}
		own, ok := directory(record.Unit)
		unreferenced := map[string]bool{}
		for _, unit := range record.Unreferenced {
			importer, mapped := directory(unit)
			ok = ok && mapped
			unreferenced[importer] = true
		}
		if !ok {
			continue
		}
		for _, dirty := range record.Paths {
			if parentDirectory(dirty) == own {
				narrowed[dirty] = unreferenced
			}
		}
	}
	return narrowed
}

// attributeNarrowed is attribute for a non-test source of a package the plan
// narrowed as an additive change (ADN-V0-006): the package itself, and the
// dependents reached through its importers and namers other than the
// unreferenced ones, whose test importers are dropped likewise; rule (c) is
// unchanged.
func (index *repositoryIndex) attributeNarrowed(dirty string, readers []string, unreferenced map[string]bool) []attribution {
	if resolvedEvidence(dirty) {
		readers = index.resolvingReaders(dirty)
	}
	directory := parentDirectory(dirty)
	var start []string
	for _, importer := range index.importers[directory] {
		if !unreferenced[importer] {
			start = append(start, importer)
		}
	}
	start = append(start, index.namers[directory]...)
	reached := map[string]bool{}
	for _, reachedDirectory := range append(append([]string(nil), start...), index.reach(true, start)...) {
		reached[reachedDirectory] = true
	}
	for _, user := range index.testImporters[directory] {
		if !unreferenced[user] {
			reached[user] = true
		}
	}
	return []attribution{{"frontier", []string{directory}}, {"frontier", sortedKeys(reached)}, {"reader", readers}}
}

// attributeStructure is attribute without rule (c).
func (index *repositoryIndex) attributeStructure(dirty string) []attribution {
	var rules []attribution
	if packageSource(dirty) && !index.nestedModule(dirty) {
		directory := parentDirectory(dirty)
		if index.packages[directory] != nil {
			rules = append(rules, attribution{"frontier", []string{directory}})
			// Go compiles a _test.go file only into its own package's test
			// binary, so no importer can see it, added, changed or deleted
			// (AFP-V0-154, the AFP-V0-036 rule; V1-0996). A package whose
			// non-test files embed anything may embed the test file as
			// production data its importers' tests consume, so it keeps the
			// widening (fail closed: any //go:embed, not a pattern match).
			if strings.HasSuffix(dirty, "_test.go") && !index.packages[directory].embeds {
				return rules
			}
		}
		rules = append(rules, attribution{"frontier", index.dependents(directory)})
	} else {
		rules = append(rules, attribution{"data", nil})
		enclosing := index.enclosing(dirty)
		rules = append(rules, attribution{"reader", enclosing})
		if len(enclosing) > 0 && parentDirectory(dirty) == enclosing[0] {
			rules = append(rules, attribution{"frontier", index.dependents(enclosing[0])})
		}
		for _, directory := range enclosing {
			if index.packages[directory].embeds {
				rules = append(rules, attribution{"frontier", index.dependents(directory)})
			}
		}
	}
	return rules
}

// packageBounds indexes ROOT and, for every package, prints either the
// `unresolved <pkg>: "<reason>"` line of unresolvedPackages or one
// `bound <pkg> <rule> <path>` line per path of paths that rules (a) to (c)
// attribute to the package, in import-path then path order, tagged with the
// first rule that selects it; a package the read-scope declaration bounds is
// resolved, and its declaration, subtree and declared entries are `declared`
// paths (GL-V0-050). The bound of a resolved package is therefore every
// worktree path whose change would select it: the complete set of paths its
// tests can read (GL-V0-009). A path carrying a control character cannot
// be printed on one line and is an error.
func packageBounds(module, root string, paths []string) ([]string, error) {
	index, err := indexRepository(root, module)
	if err != nil {
		return nil, fmt.Errorf("the repository could not be indexed: %w", err)
	}
	reasons, scopes, err := index.undeclaredUnresolved(root)
	if err != nil {
		return nil, err
	}
	bounds := map[string]map[string]string{}
	bind := func(directories []string, kind, p string) {
		for _, directory := range directories {
			if bounds[directory] == nil {
				bounds[directory] = map[string]string{}
			}
			if _, seen := bounds[directory][p]; !seen {
				bounds[directory][p] = kind
			}
		}
	}
	for _, p := range paths {
		if strings.ContainsFunc(p, unicode.IsControl) {
			return nil, fmt.Errorf("a path contains a control character")
		}
		for _, rule := range index.attributeStructure(p) {
			bind(rule.directories, rule.kind, p)
		}
		if resolvedEvidence(p) {
			bind(index.resolvingReaders(p), "reader", p)
		}
	}
	// A declared package (AFP-V0-023, GL-V0-050) is bound by what its
	// declaration admits: the declaration, its subtree and its declared
	// entries, exactly the paths that select it in selectPackages.
	for _, directory := range sortedKeys(scopes) {
		for _, p := range paths {
			if inReadScope(directory, scopes[directory], p) {
				bind([]string{directory}, "declared", p)
			}
		}
	}
	// Rule (c) over every path at once: the same namesPath relation readers()
	// applies per dirty path, evaluated token by token.
	matcher := newPathMatcher(paths)
	for value, directories := range index.holders {
		for _, i := range matcher.named(value) {
			if !resolvedEvidence(paths[i]) {
				bind(directories, "reader", paths[i])
			}
		}
	}
	var lines []string
	for _, directory := range sortedKeys(index.packages) {
		pkg := importPath(module, directory)
		if reasons[directory] != "" {
			lines = append(lines, fmt.Sprintf("unresolved %s: %q\n", pkg, reasons[directory]))
			continue
		}
		for _, p := range sortedKeys(bounds[directory]) {
			lines = append(lines, fmt.Sprintf("bound %s %s %s\n", pkg, bounds[directory][p], p))
		}
	}
	return lines, nil
}

// packageSource reports whether dirtyPath is a `.go` file outside testdata, `.`,
// and `_` directories, the only path a frontier may attribute to one package.
// unresolvedPackages indexes ROOT and returns one `unresolved <pkg>: "<reason>"`
// line per package whose reads no literal and no read-scope declaration
// bounds, in import-path order, in the same shape selectPackages prints them.
func unresolvedPackages(module, root string) ([]string, error) {
	index, err := indexRepository(root, module)
	if err != nil {
		return nil, fmt.Errorf("the repository could not be indexed: %w", err)
	}
	reasons, _, err := index.undeclaredUnresolved(root)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, directory := range sortedKeys(reasons) {
		lines = append(lines, fmt.Sprintf("unresolved %s: %q\n", importPath(module, directory), reasons[directory]))
	}
	return lines, nil
}

// undeclaredUnresolved is index.unresolved() less the packages the read-scope
// declaration bounds (AFP-V0-023), as selectPackages applies rule (d), with
// the declared scopes. An invalid declaration is an error, so the caller keys
// nothing on it (GL-V0-050).
func (index *repositoryIndex) undeclaredUnresolved(root string) (map[string]string, map[string][]string, error) {
	scopes, err := index.readScopes(root)
	if err != nil {
		return nil, nil, fmt.Errorf("the read-scope declaration is invalid: %w", err)
	}
	reasons := index.unresolved()
	for directory := range scopes {
		delete(reasons, directory)
	}
	return reasons, scopes, nil
}

func packageSource(dirtyPath string) bool {
	return strings.HasSuffix(dirtyPath, ".go") && !hiddenDirectory(parentDirectory(dirtyPath))
}

func parentDirectory(p string) string {
	parent := path.Dir(p)
	if parent == "." {
		return ""
	}
	return parent
}

func hiddenDirectory(directory string) bool {
	if directory == "" {
		return false
	}
	for _, part := range strings.Split(directory, "/") {
		if part == "testdata" || strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") {
			return true
		}
	}
	return false
}

// underModule reports whether pkg is the module's own root package or lies
// strictly beneath it, rather than merely sharing its string as a prefix: a
// bare strings.HasPrefix would also accept a sibling module whose path
// happens to extend the same characters, such as "<module>2/x".
func underModule(pkg, module string) bool {
	return pkg == module || strings.HasPrefix(pkg, module+"/")
}

func importPath(module, directory string) string {
	if directory == "" {
		return module
	}
	return module + "/" + directory
}

func exists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func sortedUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return sortedKeys(set)
}

func sortedKeys[V any](set map[string]V) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
