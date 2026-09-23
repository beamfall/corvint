// Command gate-affected-select is the selection step of script/gate-affected.sh
// (AFP-V0-011, AFP-V0-012). It reads one affected-plan/0 receipt, indexes the
// repository under test (readers.go), and prints one audit line per decision
// (`selected <pkg>` from the plan; `frontier <pkg> <- <path>` for a package or
// importer a dirty Go source reaches; `data <path>` for any other dirty path;
// `reader <pkg> <- <path>` for a package that encloses or names a dirty path;
// `unresolved <pkg>: <reason>` for a package whose reads no literal bounds,
// selected whenever any path is dirty), then one verdict as the last line:
// `run <pkgs>`, `FALLBACK <reason>`, or `NOTHING <reason>`. A dirty path carrying
// a control character falls back before any line echoes it, so the verdict
// stays the last line. Package paths are validated before they reach a shell
// word.
//
// Usage: gate-affected-select PLAN MODULE ROOT, where ROOT is the repository
// under test that dirty paths are resolved against.
//
// Usage: gate-affected-select -unresolved MODULE ROOT prints only the
// `unresolved <pkg>: <reason>` lines for ROOT, one per package, with no verdict.
// tools/gate-ledger (GL-V0-004) reads it to decide which packages `go test` may
// answer from its cache and which the ledger keys on the whole tree.
//
// Usage: gate-affected-select -bounds MODULE ROOT reads worktree paths, one per
// line, on stdin and prints the same `unresolved` lines plus one
// `bound <pkg> <rule> <path>` line per path rules (a) to (c) attribute to each
// resolved package. tools/gate-ledger (GL-V0-009) keys a resolved package on
// the content of its bound.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type receipt struct {
	Plan struct {
		Dirty   []string `json:"dirty"`
		Unknown []struct {
			Reason string `json:"reason"`
			Detail string `json:"detail"`
		} `json:"unknown"`
	} `json:"plan"`
	Provider struct {
		Go struct {
			Packages []string `json:"packages"`
			State    string   `json:"state"`
		} `json:"go"`
	} `json:"provider"`
}

var moduleLevelFrontiers = map[string]bool{
	"go:module-path-unresolved":          true,
	"go:included-directory-walk-bounded": true,
	"go:unparsed-source":                 true,
	"go:cgo-frontier":                    true,
}

var rootModuleDefinitions = map[string]bool{"go.mod": true, "go.sum": true, "go.work": true, "go.work.sum": true}

// changeEvidence is the CEM sidecar (internal/frontier.ExcludedPath) that every
// dogfooded change commits. It is derived from the rest of the diff, so a
// package's tests read it only through a literal that resolves to it from the
// package's directory; a `.corvint` or `change.cem.json` token joined to a
// fixture root does not select its holder (AFP-V0-012 (c)).
const changeEvidence = ".corvint/change.cem.json"

var plainImportPath = regexp.MustCompile(`^[A-Za-z0-9._~/-]+$`)

const maxPlanBytes = 8 << 20

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: gate-affected-select PLAN MODULE ROOT | -unresolved MODULE ROOT | -bounds MODULE ROOT < PATHS")
		os.Exit(2)
	}
	if os.Args[1] == "-unresolved" || os.Args[1] == "-bounds" {
		lines, err := unresolvedPackages(os.Args[2], os.Args[3])
		if os.Args[1] == "-bounds" {
			lines, err = packageBounds(os.Args[2], os.Args[3], stdinPaths())
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(strings.Join(lines, ""))
		return
	}
	data, err := readPlan(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var plan receipt
	if err := json.Unmarshal(data, &plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(strings.Join(selectPackages(plan, os.Args[2], os.Args[3]), "\n"))
}

// stdinPaths reads the worktree paths `-bounds` attributes, one per line.
func stdinPaths() []string {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxPlanBytes+1))
	if err != nil || len(data) > maxPlanBytes {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func readPlan(name string) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPlanBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPlanBytes {
		return nil, fmt.Errorf("plan exceeds %d bytes", maxPlanBytes)
	}
	return data, nil
}

func selectPackages(plan receipt, module, root string) []string {
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
	for _, dirty := range sortedUnique(plan.Plan.Dirty) {
		for _, rule := range index.attribute(dirty) {
			if rule.kind == "data" {
				lines = append(lines, "data "+dirty)
				continue
			}
			add(rule.directories, rule.kind, " <- "+dirty)
		}
	}
	unresolved := index.unresolved()
	for _, directory := range sortedKeys(unresolved) {
		if len(plan.Plan.Dirty) > 0 {
			add([]string{directory}, "unresolved", fmt.Sprintf(": %q", unresolved[directory]))
		}
	}
	selected := sortedKeys(packages)
	for _, pkg := range selected {
		if !plainImportPath.MatchString(pkg) || !underModule(pkg, module) {
			return fallback(fmt.Sprintf("package path is not a plain import path under the module: %q", pkg))
		}
	}
	if len(selected) == 0 && len(plan.Plan.Dirty) > 0 {
		return fallback(fmt.Sprintf("the selection is empty while %d path(s) are dirty", len(plan.Plan.Dirty)))
	}
	if len(selected) == 0 {
		return append(lines, "NOTHING no dirty or committed path; no Go package to test")
	}
	return append(lines, "run "+strings.Join(selected, " "))
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
// it. Rule (d), the unresolved packages, is index.unresolved().
func (index *repositoryIndex) attribute(dirty string) []attribution {
	readers := index.readers
	if dirty == changeEvidence {
		readers = index.resolvingReaders
	}
	return append(index.attributeStructure(dirty), attribution{"reader", readers(dirty)})
}

// attributeStructure is attribute without rule (c).
func (index *repositoryIndex) attributeStructure(dirty string) []attribution {
	var rules []attribution
	if packageSource(dirty) && !index.nestedModule(dirty) {
		directory := parentDirectory(dirty)
		if index.packages[directory] != nil {
			rules = append(rules, attribution{"frontier", []string{directory}})
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
// first rule that selects it. The bound of a resolved package is therefore
// every worktree path whose change would select it: the complete set of paths
// its tests can read (GL-V0-009). A path carrying a control character cannot
// be printed on one line and is an error.
func packageBounds(module, root string, paths []string) ([]string, error) {
	index, err := indexRepository(root, module)
	if err != nil {
		return nil, fmt.Errorf("the repository could not be indexed: %w", err)
	}
	reasons := index.unresolved()
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
		if p == changeEvidence {
			bind(index.resolvingReaders(p), "reader", p)
		}
	}
	// Rule (c) over every path at once: the same namesPath relation readers()
	// applies per dirty path, evaluated token by token.
	matcher := newPathMatcher(paths)
	for value, directories := range index.holders {
		for _, i := range matcher.named(value) {
			if paths[i] != changeEvidence {
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
// line per package whose reads no literal bounds, in import-path order, in the
// same shape selectPackages prints them.
func unresolvedPackages(module, root string) ([]string, error) {
	index, err := indexRepository(root, module)
	if err != nil {
		return nil, fmt.Errorf("the repository could not be indexed: %w", err)
	}
	reasons := index.unresolved()
	var lines []string
	for _, directory := range sortedKeys(reasons) {
		lines = append(lines, fmt.Sprintf("unresolved %s: %q\n", importPath(module, directory), reasons[directory]))
	}
	return lines, nil
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
