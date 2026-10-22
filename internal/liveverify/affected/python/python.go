// Package python is the Python implementation of the affected-selection
// language seam.
//
// A unit is one module — one .py file — because that is Python's compilation
// and import unit. Import edges are read from source text; no interpreter is
// started and nothing is imported, so the graph can be rebuilt on a dirty
// worktree with no virtual environment present.
package python

import (
	"context"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	// FrontierDynamicImport reports importlib or __import__ usage, whose target
	// is not visible in source text.
	FrontierDynamicImport = "python:dynamic-import"
	// FrontierRelativeImport reports a relative import that resolved to no
	// observed module.
	FrontierRelativeImport = "python:relative-import-unresolved"
	// FrontierUnreadableSource reports a file that could not be read or decoded.
	FrontierUnreadableSource = "python:unreadable-source"
	// FrontierConditionalImport reports an import nested inside a block, whose
	// execution is conditional. The edge is kept; the condition is the frontier.
	FrontierConditionalImport = "python:conditional-import"
	// FrontierPytestPlugins reports a pytest_plugins assignment whose value is
	// not only string literals, so the plugin modules it loads are not all
	// visible in source text (AFP-V0-188).
	FrontierPytestPlugins = "python:pytest-plugins-unresolved"
)

// SourceRoots are the directory prefixes stripped when deriving a dotted module
// name, in addition to the repository root itself. They cover the layouts a
// Python project uses to put importable modules somewhere other than the root.
var SourceRoots = []string{"src", "lib", "python"}

// Language observes Python modules under one repository root.
type Language struct{}

// New returns the Python language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "python" }

// Owns reports whether a path is Python source text.
func (Language) Owns(relative string) bool { return strings.HasSuffix(relative, ".py") }

// Units observes every Python module in the repository rooted at root.
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

// UnitsSource reads only the explicitly supplied source universe.
func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	files, err := root.Files(func(name string) bool {
		return strings.HasSuffix(name, ".py")
	})
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{}
	// moduleNames maps each dotted name a file is importable under to its unit
	// identity. One file can be importable under several names because a source
	// root may or may not be on the interpreter's path.
	moduleNames := make(map[string]string, len(files)*2)
	seenUnit := make(map[string]bool, len(files))
	units := make([]affected.Unit, 0, len(files))
	rawImports := make(map[string][]importRef, len(files))
	for _, relative := range files {
		id := "python:" + primaryModuleName(relative)
		if seenUnit[id] {
			continue
		}
		seenUnit[id] = true
		body, readErr := root.Read(relative)
		if readErr != nil {
			frontier[FrontierUnreadableSource] = true
			continue
		}
		text := string(body)
		refs, flags := scanImports(relative, text)
		for flag := range flags {
			frontier[flag] = true
		}
		unit := affected.Unit{ID: id}
		if isTestFile(relative) {
			unit.Tests = []string{relative}
		} else {
			unit.Sources = []string{relative}
		}
		units = append(units, unit)
		rawImports[id] = refs
		for _, name := range moduleAliases(relative) {
			if _, taken := moduleNames[name]; !taken {
				moduleNames[name] = id
			}
		}
	}
	resolve(units, rawImports, moduleNames, frontier)
	linkConftests(units)
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

// linkConftests adds, to every unit declaring a test, an import edge to each
// conftest.py in the test's directory or an ancestor of it (AFP-V0-189).
// pytest loads those conftest files, and the fixtures and hooks they define,
// before it collects the test, although no import statement names them.
func linkConftests(units []affected.Unit) {
	conftests := make(map[string]string, 4)
	for _, unit := range units {
		for _, file := range append(append([]string(nil), unit.Sources...), unit.Tests...) {
			if path.Base(file) == "conftest.py" {
				conftests[path.Dir(file)] = unit.ID
			}
		}
	}
	if len(conftests) == 0 {
		return
	}
	for index := range units {
		unit := &units[index]
		edges := make(map[string]bool, len(unit.Imports)+len(conftests))
		for _, target := range unit.Imports {
			edges[target] = true
		}
		for _, test := range unit.Tests {
			for directory := path.Dir(test); ; directory = path.Dir(directory) {
				if id, ok := conftests[directory]; ok && id != unit.ID {
					edges[id] = true
				}
				if directory == "." {
					break
				}
			}
		}
		if len(edges) != len(unit.Imports) {
			unit.Imports = sortedKeys(edges)
		}
	}
}

type importRef struct {
	module string
	// dots is the leading-dot count of a relative import; zero for absolute.
	dots int
	// owner is the repository-relative path of the importing file, needed to
	// anchor a relative import.
	owner string
}

// resolve rewrites raw import references into unit identities.
//
// An absolute import is matched against the longest observed module name that
// prefixes it, so `from context_corvint.index import x` reaches
// `context_corvint.index` when that module exists and `context_corvint` when it is
// a package importing the name through its __init__.
func resolve(units []affected.Unit, rawImports map[string][]importRef, moduleNames map[string]string, frontier map[string]bool) {
	for index := range units {
		unit := &units[index]
		edges := make(map[string]bool, len(rawImports[unit.ID]))
		for _, ref := range rawImports[unit.ID] {
			target, ok := resolveRef(ref, moduleNames)
			if !ok {
				if ref.dots > 0 {
					frontier[FrontierRelativeImport] = true
				}
				continue
			}
			if target == unit.ID {
				continue
			}
			edges[target] = true
		}
		unit.Imports = sortedKeys(edges)
	}
}

func resolveRef(ref importRef, moduleNames map[string]string) (string, bool) {
	name := ref.module
	if ref.dots > 0 {
		base := path.Dir(ref.owner)
		for step := 1; step < ref.dots; step++ {
			base = path.Dir(base)
		}
		if base == "." || base == "/" {
			return "", false
		}
		prefix := strings.ReplaceAll(base, "/", ".")
		if name == "" {
			name = prefix
		} else {
			name = prefix + "." + name
		}
	}
	for name != "" {
		if id, ok := moduleNames[name]; ok {
			return id, true
		}
		cut := strings.LastIndex(name, ".")
		if cut < 0 {
			return "", false
		}
		name = name[:cut]
	}
	return "", false
}

// scanImports reads import statements line by line. A statement whose
// parentheses are still open at the end of its first line, or whose line ends
// in a backslash, continues on the following lines; those lines are joined to
// it, comments removed, before it is parsed, so every name of a multi-line
// `from X import (...)` is read (AFP-V0-187). A `pytest_plugins` assignment at
// module level is read the same way: each plugin module it names is an import
// (AFP-V0-188).
//
// A name that appears inside a string or comment can produce an edge that does
// not exist. That is conservative in the safe direction: a spurious edge widens
// selection, never narrows it.
func scanImports(owner, text string) ([]importRef, map[string]bool) {
	refs := make([]importRef, 0, 16)
	flags := make(map[string]bool, 2)
	lines := strings.Split(text, "\n")
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "importlib") || strings.Contains(trimmed, "__import__") {
			flags[FrontierDynamicImport] = true
		}
		if plugins, ok := strings.CutPrefix(line, "pytest_plugins"); ok && pluginsAssignment(plugins) {
			statement, next := joinContinuation(lines, index)
			index = next
			names, literal := pluginModules(statement)
			if !literal {
				flags[FrontierPytestPlugins] = true
			}
			for _, name := range names {
				refs = append(refs, importRef{module: name, owner: owner})
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "import ") && !strings.HasPrefix(trimmed, "from ") {
			continue
		}
		if len(line) != len(trimmed) {
			flags[FrontierConditionalImport] = true
		}
		statement, next := joinContinuation(lines, index)
		index = next
		for _, ref := range parseImportLine(statement) {
			ref.owner = owner
			refs = append(refs, ref)
		}
	}
	return refs, flags
}

// joinContinuation returns the statement starting at lines[start], comments
// removed and continuation lines joined with single spaces, and the index of
// its last line. Parentheses and brackets are counted outside comments only;
// a statement still open at the end of the file ends there.
func joinContinuation(lines []string, start int) (string, int) {
	parts := make([]string, 0, 4)
	depth := 0
	index := start
	for ; index < len(lines); index++ {
		part := strings.TrimSpace(lines[index])
		if cut := strings.IndexByte(part, '#'); cut >= 0 {
			part = strings.TrimSpace(part[:cut])
		}
		depth += strings.Count(part, "(") + strings.Count(part, "[") - strings.Count(part, ")") - strings.Count(part, "]")
		backslash := strings.HasSuffix(part, "\\")
		part = strings.TrimSpace(strings.TrimSuffix(part, "\\"))
		if part != "" {
			parts = append(parts, part)
		}
		if depth <= 0 && !backslash {
			break
		}
	}
	if index == len(lines) {
		index--
	}
	return strings.Join(parts, " "), index
}

// pluginsAssignment reports whether the text after a column-zero
// `pytest_plugins` is an assignment to it.
func pluginsAssignment(rest string) bool {
	rest = strings.TrimLeft(rest, " \t")
	return (strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==")) || strings.HasPrefix(rest, "+=")
}

// pluginLiteral is one quoted dotted module name in a pytest_plugins value.
var pluginLiteral = regexp.MustCompile(`"([A-Za-z_][A-Za-z0-9_.]*)"|'([A-Za-z_][A-Za-z0-9_.]*)'`)

// pluginModules lists the module names a pytest_plugins assignment names, and
// whether its value is only string literals in a list, tuple or bare string.
// Any other value (a variable, a call, a computed string) is not literal: the
// names it does show are still edges, and the caller raises a frontier.
func pluginModules(statement string) ([]string, bool) {
	_, value, _ := strings.Cut(statement, "=")
	names := make([]string, 0, 4)
	for _, match := range pluginLiteral.FindAllStringSubmatch(value, -1) {
		names = append(names, match[1]+match[2])
	}
	rest := pluginLiteral.ReplaceAllString(value, "")
	return names, strings.TrimSpace(value) != "" && strings.Trim(rest, " \t()[],") == ""
}

func parseImportLine(line string) []importRef {
	if strings.HasPrefix(line, "from ") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "from "))
		cut := strings.Index(rest, " import")
		if cut < 0 {
			return nil
		}
		target := strings.TrimSpace(rest[:cut])
		dots := 0
		for dots < len(target) && target[dots] == '.' {
			dots++
		}
		module := target[dots:]
		refs := []importRef{{module: module, dots: dots}}
		// Each imported name may itself be a submodule (`from pkg import
		// sub`), so it is tried as one; a name that is not resolves to the
		// same longest observed prefix as the statement's module. A
		// multi-line form arrives joined by scanImports (AFP-V0-187).
		names := rest[cut+len(" import"):]
		if stop := strings.IndexAny(names, "#;"); stop >= 0 {
			names = names[:stop]
		}
		for _, clause := range strings.Split(strings.Trim(strings.TrimSpace(names), "()"), ",") {
			name := strings.TrimSpace(strings.Trim(strings.TrimSpace(clause), "()"))
			if as := strings.Index(name, " as "); as >= 0 {
				name = strings.TrimSpace(name[:as])
			}
			if name == "" || name == "*" || strings.ContainsAny(name, " \t\\().") {
				continue
			}
			if module != "" {
				name = module + "." + name
			}
			refs = append(refs, importRef{module: name, dots: dots})
		}
		return refs
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "import "))
	refs := make([]importRef, 0, 2)
	for _, clause := range strings.Split(rest, ",") {
		name := strings.TrimSpace(clause)
		if cut := strings.Index(name, " as "); cut >= 0 {
			name = strings.TrimSpace(name[:cut])
		}
		if cut := strings.IndexAny(name, " \t#;"); cut >= 0 {
			name = name[:cut]
		}
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		refs = append(refs, importRef{module: name})
	}
	return refs
}

// primaryModuleName is the dotted name derived from the repository root. It is
// the unit identity, so it is unique per file regardless of source-root layout.
func primaryModuleName(relative string) string {
	trimmed := strings.TrimSuffix(relative, ".py")
	trimmed = strings.TrimSuffix(trimmed, "/__init__")
	return strings.ReplaceAll(trimmed, "/", ".")
}

// moduleAliases lists every dotted name this file is importable under: the
// root-relative name, and the name relative to each recognized source root.
func moduleAliases(relative string) []string {
	aliases := []string{primaryModuleName(relative)}
	for _, sourceRoot := range SourceRoots {
		prefix := sourceRoot + "/"
		if !strings.HasPrefix(relative, prefix) {
			continue
		}
		aliases = append(aliases, primaryModuleName(strings.TrimPrefix(relative, prefix)))
	}
	return aliases
}

// isTestFile applies the discovery conventions pytest and unittest share.
func isTestFile(relative string) bool {
	name := path.Base(relative)
	if strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.py") {
		return true
	}
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "tests" || component == "test" {
			return true
		}
	}
	return false
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
