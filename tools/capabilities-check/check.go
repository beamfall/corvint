// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Inventory is what the code says exists (CAP-V0-002).
type Inventory struct {
	Commands   []string        // root help Commands: block, in help order
	Core       map[string]bool // verbs the root help lists as Core
	Dispatched []string        // cmd/corvint/main.go topLevelCommands: the verbs runContext dispatches
	Binaries   []string        // cmd/ directories holding a package main Go file
	Tools      []string        // corvint.* MCP tool name literals
	// Experimental holds the verbs the grouped root help lists under the Experimental header (CAP-V0-008).
	Experimental map[string]bool
	// Subcommands maps a verb to the subcommands its code-declared table names (CAP-V0-007) and,
	// after MergeSubcommands, its two-word command-tree nodes (CAP-V0-960).
	Subcommands map[string][]string
	// TreeVerbs holds the verbs with a node in the corvint command tree; only their rows are
	// checked for stale subcommands (CAP-V0-961).
	TreeVerbs map[string]bool
	// Literals holds the whole value of every string literal in non-test Go source under cmd/
	// and internal/; a listed refusal code must equal one of them (CAP-V0-009, CAP-V0-962). Nil
	// skips that check.
	Literals map[string]bool
}

// Finding is one disagreement between the catalog or README and the code (CAP-V0-003, CAP-V0-004).
type Finding struct {
	Kind    string
	Section string
	Name    string
	Detail  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s %s %q: %s", f.Kind, f.Section, f.Name, f.Detail)
}

const (
	sectionCommands = "commands"
	sectionBinaries = "binaries"
	sectionTools    = "mcp-tools"
	sectionReadme   = "readme"
)

var (
	packageMain = regexp.MustCompile(`(?m)^package main\s*$`)
	toolLiteral = regexp.MustCompile(`"(corvint\.[A-Za-z0-9_.-]+)"`)
	dispatchSet = regexp.MustCompile(`(?s)\btopLevelCommands\s*=\s*\[\]string\{(.*?)\}`)
	quotedWord  = regexp.MustCompile(`"([^"]+)"`)
	backticked  = regexp.MustCompile("`([^`]+)`")
	markdownRef = regexp.MustCompile(`\]\(([^)\s]+)\)`)
)

// Collect reads the inventory from the root help source, the dispatch list in the corvint
// main package, the cmd/ tree and the MCP bridge sources. It reads only those paths and
// writes nothing (CAP-V0-006).
func Collect(helpGo, mainGo, cmdDir, mcpDir string) (Inventory, error) {
	src, err := os.ReadFile(helpGo)
	if err != nil {
		return Inventory{}, err
	}
	commands, core, experimental, err := parseCommands(src)
	if err != nil {
		return Inventory{}, err
	}
	mainSrc, err := os.ReadFile(mainGo)
	if err != nil {
		return Inventory{}, err
	}
	dispatched, err := parseDispatched(mainSrc)
	if err != nil {
		return Inventory{}, err
	}
	binaries, err := scanBinaries(cmdDir)
	if err != nil {
		return Inventory{}, err
	}
	toolDirs := []string{mcpDir}
	for _, b := range binaries {
		serves, err := servesMCP(filepath.Join(cmdDir, b))
		if err != nil {
			return Inventory{}, err
		}
		if serves || strings.HasPrefix(b, "corvint-") && strings.HasSuffix(b, "mcp") {
			toolDirs = append(toolDirs, filepath.Join(cmdDir, b))
		}
	}
	tools, err := scanTools(toolDirs)
	if err != nil {
		return Inventory{}, err
	}
	subcommands, err := parseSubcommands(filepath.Dir(helpGo))
	if err != nil {
		return Inventory{}, err
	}
	return Inventory{Commands: commands, Core: core, Dispatched: dispatched, Binaries: binaries, Tools: tools,
		Experimental: experimental, Subcommands: subcommands}, nil
}

// subcommandTables names the code-declared subcommand tables in cmd/corvint (CAP-V0-007). An
// empty command marks a nested table whose keys are verbs and whose values are subcommand sets.
var subcommandTables = []struct{ variable, command string }{
	{"helpSubcommands", ""},
	{"docsSubcommands", "docs"},
	{"flowSubcommands", "flows"},
	{"leaseActions", "lease"},
}

// parseSubcommands reads each declared table from the non-test Go files of the corvint main
// package: a top-level []string or map literal, a nested map whose values are literals or
// top-level identifiers, and any NAME["sub"] = ... registration. A missing table is an error,
// so renaming one cannot silently disable the check.
func parseSubcommands(dir string) (map[string][]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	vars := map[string]ast.Expr{}
	registered := map[string][]string{}
	tracked := map[string]bool{}
	for _, t := range subcommandTables {
		tracked[t.variable] = true
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, ident := range vs.Names {
					if i < len(vs.Values) {
						vars[ident.Name] = vs.Values[i]
					}
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range assign.Lhs {
				index, ok := lhs.(*ast.IndexExpr)
				if !ok {
					continue
				}
				target, ok := index.X.(*ast.Ident)
				if !ok || !tracked[target.Name] {
					continue
				}
				if key, ok := stringLit(index.Index); ok {
					registered[target.Name] = append(registered[target.Name], key)
				}
			}
			return true
		})
	}
	out := map[string][]string{}
	for _, t := range subcommandTables {
		expr, ok := vars[t.variable]
		if !ok {
			return nil, fmt.Errorf("subcommand table %s not found in %s", t.variable, dir)
		}
		if t.command != "" {
			out[t.command] = append(out[t.command], literalKeys(expr, vars)...)
			out[t.command] = append(out[t.command], registered[t.variable]...)
			continue
		}
		lit, ok := expr.(*ast.CompositeLit)
		if !ok {
			return nil, fmt.Errorf("subcommand table %s is not a composite literal", t.variable)
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if verb, ok := stringLit(kv.Key); ok {
				out[verb] = append(out[verb], literalKeys(kv.Value, vars)...)
			}
		}
	}
	for verb, subs := range out {
		slices.Sort(subs)
		out[verb] = slices.Compact(subs)
	}
	return out, nil
}

// literalKeys returns the string keys of a map literal or the string elements of a slice
// literal, following one top-level identifier.
func literalKeys(expr ast.Expr, vars map[string]ast.Expr) []string {
	if ident, ok := expr.(*ast.Ident); ok {
		if resolved, ok := vars[ident.Name]; ok {
			expr = resolved
		}
	}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var keys []string
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			elt = kv.Key
		}
		if key, ok := stringLit(elt); ok {
			keys = append(keys, key)
		}
	}
	return keys
}

func stringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

// literalWord splits backticked catalog text into the words a subcommand is matched against.
var literalWord = regexp.MustCompile(`[A-Za-z0-9_-]+`)

// ScanLiterals returns the unquoted value of every string literal in non-test Go source under
// dirs, skipping testdata trees (CAP-V0-009, CAP-V0-962).
func ScanLiterals(dirs []string) (map[string]bool, error) {
	literals := map[string]bool{}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			fset := token.NewFileSet()
			var s scanner.Scanner
			s.Init(fset.AddFile(p, -1, len(src)), src, nil, 0)
			for {
				_, tok, lit := s.Scan()
				if tok == token.EOF {
					break
				}
				if tok == token.STRING {
					if value, err := strconv.Unquote(lit); err == nil {
						literals[value] = true
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return literals, nil
}

// parseDispatched reads the topLevelCommands literal, which names every top-level verb
// runContext dispatches (cmd/corvint core_freeze_test pins that correspondence).
func parseDispatched(src []byte) ([]string, error) {
	m := dispatchSet.FindSubmatch(src)
	if m == nil {
		return nil, errors.New("topLevelCommands literal not found")
	}
	var out []string
	for _, w := range quotedWord.FindAllSubmatch(m[1], -1) {
		out = append(out, string(w[1]))
	}
	if len(out) == 0 {
		return nil, errors.New("topLevelCommands literal is empty")
	}
	return out, nil
}

// Root help maturity group headers (CCF-V1-008, V1-0945). The Commands: block may group its
// verbs under these headers, each indented less than the verbs it heads. One Core header leads,
// and one or more Experimental headers group the other verbs by job (V1-3730).
const (
	coreGroupPrefix         = "Core ("
	experimentalGroupPrefix = "Experimental"
)

// hiddenSet reads the hiddenCommands literal: dispatched verbs that root help leaves unlisted
// (CCF-V1-008, V1-3730), so they are documented, experimental and not dispatch-undocumented.
var hiddenSet = regexp.MustCompile(`(?s)hiddenCommands = map\[string\]string\{(.*?)\n\}`)

var hiddenKey = regexp.MustCompile(`(?m)^\s*"([a-z][a-z-]*)":`)

func parseHidden(src []byte) []string {
	m := hiddenSet.FindSubmatch(src)
	if m == nil {
		return nil
	}
	var out []string
	for _, w := range hiddenKey.FindAllSubmatch(m[1], -1) {
		out = append(out, string(w[1]))
	}
	return out
}

// parseCommands reads the root help's Commands: block and its Core maturity list. The block is
// either flat, with the Core list in a separate section, or grouped under exactly the Core and
// Experimental headers, in which case the Core group is the Core list and the Experimental group
// is returned as the experimental set (nil for a flat block).
func parseCommands(src []byte) ([]string, map[string]bool, map[string]bool, error) {
	text := string(src)
	start := strings.Index(text, "const rootHelp = `")
	if start < 0 {
		return nil, nil, nil, errors.New("root help constant not found")
	}
	text = text[start:]
	block := strings.Index(text, "\nCommands:\n")
	if block < 0 {
		return nil, nil, nil, errors.New("root help Commands: block not found")
	}
	var commands []string
	core := map[string]bool{}
	experimental := map[string]bool{}
	seen := map[string]bool{}
	group, groupIndent := "", -1
	for _, line := range strings.Split(text[block+len("\nCommands:\n"):], "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		name, _, ok := strings.Cut(trimmed, "  ")
		if !ok && strings.HasSuffix(trimmed, ":") {
			switch {
			case strings.HasPrefix(trimmed, coreGroupPrefix):
				group = "core"
			case strings.HasPrefix(trimmed, experimentalGroupPrefix):
				group = "experimental"
			default:
				return nil, nil, nil, fmt.Errorf("root help group header is neither Core nor Experimental: %q", line)
			}
			switch {
			case group == "core" && seen[group], group == "core" && seen["experimental"]:
				return nil, nil, nil, fmt.Errorf("root help repeats or misplaces the Core group header: %q", line)
			case groupIndent < 0 && len(commands) > 0:
				return nil, nil, nil, fmt.Errorf("root help group header follows ungrouped commands: %q", line)
			case groupIndent >= 0 && indent != groupIndent:
				return nil, nil, nil, fmt.Errorf("root help group header is not aligned with the first header: %q", line)
			}
			seen[group], groupIndent = true, indent
			continue
		}
		if !ok {
			return nil, nil, nil, fmt.Errorf("root help command line has no description: %q", line)
		}
		if groupIndent >= 0 && indent <= groupIndent {
			return nil, nil, nil, fmt.Errorf("root help command line is not indented under its group header: %q", line)
		}
		for _, family := range strings.Split(name, " | ") {
			switch group {
			case "core":
				core[strings.Fields(family)[0]] = true
			case "experimental":
				experimental[strings.Fields(family)[0]] = true
			}
			commands = append(commands, family)
		}
	}
	for _, verb := range parseHidden(src) {
		if groupIndent >= 0 {
			experimental[verb] = true
		}
		commands = append(commands, verb)
	}
	if len(commands) == 0 {
		return nil, nil, nil, errors.New("root help Commands: block is empty")
	}
	if groupIndent >= 0 {
		if len(core) == 0 {
			return nil, nil, nil, errors.New("root help Core group is empty")
		}
		return commands, core, experimental, nil
	}
	whole := string(src)
	at := strings.Index(whole, coreGroupPrefix)
	end := strings.Index(whole, experimentalGroupPrefix)
	if at < 0 || end < at {
		return nil, nil, nil, errors.New("command maturity Core list not found")
	}
	coreText := whole[at:end]
	coreText = coreText[strings.Index(coreText, ":\n")+2:]
	for _, word := range strings.FieldsFunc(coreText, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' }) {
		core[word] = true
	}
	return commands, core, nil, nil
}

func scanBinaries(cmdDir string) ([]string, error) {
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(cmdDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			name := file.Name()
			if file.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			src, err := os.ReadFile(filepath.Join(cmdDir, entry.Name(), name))
			if err != nil {
				return nil, err
			}
			if packageMain.Match(src) {
				out = append(out, entry.Name())
				break
			}
		}
	}
	return out, nil
}

// servesMCP reports whether a binary's non-test sources name the MCP "tools/list" method in an
// interpreted or raw string literal, so an MCP server is found by what it serves rather than by
// its name (CAP-V0-047).
func servesMCP(dir string) (bool, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, file := range files {
		name := file.Name()
		if file.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return false, err
		}
		if bytes.Contains(src, []byte(`"tools/list"`)) || bytes.Contains(src, []byte("`tools/list`")) {
			return true, nil
		}
	}
	return false, nil
}

func scanTools(dirs []string) ([]string, error) {
	seen := map[string]bool{}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if name == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, name := range toolNames(src) {
				seen[name] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	slices.Sort(out)
	return out, nil
}

// toolNames returns the corvint.* string literals in one Go source file.
func toolNames(src []byte) []string {
	var out []string
	for _, m := range toolLiteral.FindAllSubmatch(src, -1) {
		out = append(out, strings.TrimRight(string(m[1]), ".-"))
	}
	return out
}

// row is one gated table row: its name, its maturity cell and every relative link in it.
type row struct {
	Name     string
	Maturity string
	Links    []string
	Line     int
	Spans    []string // every backticked span in the row
	Refusals []string // backticked spans in the "Abstains or refuses on" cell, when the table has one
	Cells    []string // the row's cells, split as splitCells does
}

// parseRows returns the rows of every region between <!-- capability-gate:SECTION --> and
// <!-- /capability-gate:SECTION -->. A row's name is the first backticked span of its first cell.
func parseRows(doc []byte, section string) ([]row, error) {
	open := "<!-- capability-gate:" + section + " -->"
	closing := "<!-- /capability-gate:" + section + " -->"
	var rows []row
	inside := false
	maturityCol, refusalCol := -1, -1
	regions := 0
	scanner := bufio.NewScanner(bytes.NewReader(doc))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == open:
			if inside {
				return nil, fmt.Errorf("line %d: nested %s", n, open)
			}
			inside, maturityCol, refusalCol = true, -1, -1
			regions++
			continue
		case line == closing:
			if !inside {
				return nil, fmt.Errorf("line %d: %s without an opener", n, closing)
			}
			inside = false
			continue
		}
		if !inside || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitCells(line)
		if maturityCol < 0 {
			for i, c := range cells {
				if strings.EqualFold(c, "Maturity") {
					maturityCol = i
				}
				if strings.Contains(strings.ToLower(c), "refuses") {
					refusalCol = i
				}
			}
			if maturityCol < 0 {
				return nil, fmt.Errorf("line %d: %s table header has no Maturity column", n, section)
			}
			continue
		}
		if strings.Trim(line, "|-: ") == "" {
			continue
		}
		m := backticked.FindStringSubmatch(cells[0])
		if section == sectionReadme {
			m = backticked.FindStringSubmatch(line)
		}
		if m == nil {
			return nil, fmt.Errorf("line %d: %s row has no backticked name", n, section)
		}
		r := row{Name: m[1], Line: n, Cells: cells}
		if maturityCol < len(cells) {
			r.Maturity = cells[maturityCol]
		}
		for _, span := range backticked.FindAllStringSubmatch(line, -1) {
			r.Spans = append(r.Spans, span[1])
		}
		if refusalCol >= 0 && refusalCol < len(cells) {
			for _, span := range backticked.FindAllStringSubmatch(cells[refusalCol], -1) {
				r.Refusals = append(r.Refusals, span[1])
			}
		}
		for _, link := range markdownRef.FindAllStringSubmatch(line, -1) {
			target := link[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			r.Links = append(r.Links, target)
		}
		rows = append(rows, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if inside {
		return nil, fmt.Errorf("%s region is not closed", open)
	}
	if regions == 0 {
		return nil, fmt.Errorf("no %s region", open)
	}
	return rows, nil
}

// splitCells splits a Markdown table row, keeping a pipe that sits inside a code span.
func splitCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var cells []string
	var cur strings.Builder
	code := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '`':
			code = !code
		case c == '\\' && i+1 < len(line) && line[i+1] == '|':
			cur.WriteByte('|')
			i++
			continue
		case c == '|' && !code:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

func firstWord(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if i := strings.IndexAny(s, " ;,("); i >= 0 {
		return s[:i]
	}
	return s
}

// commandName strips the leading "corvint " from a command row's name.
func commandName(name string) string {
	return strings.TrimPrefix(name, "corvint ")
}

// Check compares the catalog (and, when readme is non-nil, the README capability table) with
// the inventory. linkBase is the directory relative catalog links resolve against.
func Check(catalog, readme []byte, inv Inventory, linkBase string) []Finding {
	findings, commands := checkDispatch(inv)
	catalogRows := map[string]map[string]row{}
	sections := []struct {
		name  string
		items []string
		key   func(string) string
	}{
		{sectionCommands, commands, commandName},
		{sectionBinaries, inv.Binaries, func(s string) string { return s }},
		{sectionTools, inv.Tools, func(s string) string { return s }},
	}
	for _, sec := range sections {
		rows, err := parseRows(catalog, sec.name)
		if err != nil {
			findings = append(findings, Finding{"malformed", sec.name, "", err.Error()})
			continue
		}
		byName := map[string]row{}
		for _, r := range rows {
			name := sec.key(r.Name)
			if _, dup := byName[name]; dup {
				findings = append(findings, Finding{"duplicate", sec.name, name, fmt.Sprintf("second row at line %d", r.Line)})
				continue
			}
			byName[name] = r
			for _, link := range r.Links {
				if _, err := os.Stat(filepath.Join(linkBase, filepath.FromSlash(path.Clean(link)))); err != nil {
					findings = append(findings, Finding{"broken-link", sec.name, name, link})
				}
			}
		}
		want := map[string]bool{}
		for _, item := range sec.items {
			want[item] = true
			if _, ok := byName[item]; !ok {
				findings = append(findings, Finding{"missing", sec.name, item, "no catalog row"})
			}
		}
		for name, r := range byName {
			if !want[name] {
				findings = append(findings, Finding{"stale", sec.name, name, fmt.Sprintf("row at line %d names nothing in the code", r.Line)})
			}
		}
		if sec.name == sectionCommands {
			for name, r := range byName {
				if !want[name] {
					continue
				}
				verb := strings.Fields(name)[0]
				isCore := inv.Core[verb]
				got := firstWord(r.Maturity)
				switch {
				case isCore != (got == "core"):
					findings = append(findings, Finding{"maturity", sec.name, name, fmt.Sprintf("maturity %q but root help Core list says core=%t", r.Maturity, isCore)})
				case inv.Experimental[verb] && got != "experimental":
					findings = append(findings, Finding{"maturity", sec.name, name, fmt.Sprintf("maturity %q but the root help lists it under Experimental", r.Maturity)})
				}
			}
		}
		catalogRows[sec.name] = byName
		findings = append(findings, checkRefusals(sec.name, rows, inv.Literals)...)
	}
	findings = append(findings, checkSubcommands(catalogRows[sectionCommands], inv.Subcommands)...)
	findings = append(findings, checkStaleSubcommands(catalogRows[sectionCommands], inv.Subcommands, inv.TreeVerbs)...)
	if readme != nil {
		findings = append(findings, checkReadme(readme, inv, catalogRows)...)
	}
	slices.SortFunc(findings, func(a, b Finding) int {
		return strings.Compare(a.String(), b.String())
	})
	return findings
}

// checkSubcommands requires each code-declared subcommand to appear as a word inside a backticked
// span of a catalog row for its verb (CAP-V0-007). A verb without a row is left to the missing check.
func checkSubcommands(rows map[string]row, subcommands map[string][]string) []Finding {
	var findings []Finding
	for verb, subs := range subcommands {
		words := map[string]bool{}
		found := false
		for name, r := range rows {
			if strings.Fields(name)[0] != verb {
				continue
			}
			found = true
			for _, span := range r.Spans {
				for _, w := range literalWord.FindAllString(span, -1) {
					words[w] = true
				}
			}
		}
		if !found {
			continue
		}
		for _, sub := range subs {
			if !words[sub] {
				findings = append(findings, Finding{"subcommand-undocumented", sectionCommands, verb + " " + sub, "the code declares this subcommand but no catalog row for the verb names it in a backticked span"})
			}
		}
	}
	return findings
}

// checkStaleSubcommands reports each subcommand a row's Subcommands sentence names that neither
// the command tree nor a declared table holds for the verb (CAP-V0-961). Only verbs with a tree
// node are checked, so the tree stays the authority.
func checkStaleSubcommands(rows map[string]row, subcommands map[string][]string, treeVerbs map[string]bool) []Finding {
	var findings []Finding
	for name, r := range rows {
		verb := strings.Fields(name)[0]
		if !treeVerbs[verb] {
			continue
		}
		for _, sub := range documentedSubcommands(r.Cells) {
			if !slices.Contains(subcommands[verb], sub) {
				findings = append(findings, Finding{"subcommand-stale", sectionCommands, verb + " " + sub,
					fmt.Sprintf("row at line %d lists it as a subcommand but neither the corvint command tree nor a declared table has it", r.Line)})
			}
		}
	}
	return findings
}

// documentedSubcommands returns the first word of each backticked span at parenthesis depth zero
// in a cell's Subcommands sentence, which runs from the word "Subcommands" to the first ". " at
// depth zero or the cell end. A span starting with "-" is an option, not a subcommand.
func documentedSubcommands(cells []string) []string {
	var out []string
	for _, cell := range cells {
		at := strings.Index(cell, "Subcommands")
		if at < 0 {
			continue
		}
		depth, code := 0, false
		var span strings.Builder
		text := cell[at:]
	scan:
		for i := 0; i < len(text); i++ {
			c := text[i]
			switch {
			case c == '`':
				if code && depth == 0 {
					if word := literalWord.FindString(span.String()); word != "" && !strings.HasPrefix(span.String(), "-") {
						out = append(out, word)
					}
				}
				code = !code
				span.Reset()
			case code:
				span.WriteByte(c)
			case c == '(':
				depth++
			case c == ')' && depth > 0:
				depth--
			case c == '.' && depth == 0 && (i+1 == len(text) || text[i+1] == ' '):
				break scan
			}
		}
	}
	return out
}

// refusalCode matches a backticked refusal or abstention code: kebab-case or SCREAMING_SNAKE_CASE
// with at least two parts.
var refusalCode = regexp.MustCompile(`^(?:[a-z][a-z0-9]*(?:-[a-z0-9]+)+|[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+)$`)

// checkRefusals requires each refusal code a row lists to equal the whole value of some string
// literal in the scanned source (CAP-V0-009, CAP-V0-962). Nil literals skips the check.
func checkRefusals(section string, rows []row, literals map[string]bool) []Finding {
	if literals == nil {
		return nil
	}
	var findings []Finding
	for _, r := range rows {
		for _, code := range r.Refusals {
			if refusalCode.MatchString(code) && !literals[code] {
				findings = append(findings, Finding{"refusal-unknown", section, r.Name, fmt.Sprintf("row at line %d lists %q, which no non-test Go string literal under cmd/ or internal/ equals", r.Line, code)})
			}
		}
	}
	return findings
}

// checkDispatch compares the root help's verbs with the dispatched verbs. A dispatched verb
// the help omits is reported and still needs a catalog row; a help verb nothing dispatches is
// reported too. It returns the findings and the command names the catalog must cover.
func checkDispatch(inv Inventory) ([]Finding, []string) {
	var findings []Finding
	commands := slices.Clone(inv.Commands)
	documented := map[string]bool{}
	for _, c := range inv.Commands {
		documented[strings.Fields(c)[0]] = true
	}
	dispatched := map[string]bool{}
	for _, verb := range inv.Dispatched {
		dispatched[verb] = true
		if !documented[verb] {
			findings = append(findings, Finding{"dispatch-undocumented", sectionCommands, verb, "runContext dispatches it but the root help Commands: block does not list it"})
			commands = append(commands, verb)
		}
	}
	for verb := range documented {
		if !dispatched[verb] {
			findings = append(findings, Finding{"dispatch-absent", sectionCommands, verb, "the root help lists it but topLevelCommands does not dispatch it"})
		}
	}
	return findings, commands
}

// checkReadme maps each README capability row to a catalog row (CAP-V0-004).
func checkReadme(readme []byte, inv Inventory, catalogRows map[string]map[string]row) []Finding {
	rows, err := parseRows(readme, sectionReadme)
	if err != nil {
		return []Finding{{"malformed", sectionReadme, "", err.Error()}}
	}
	var findings []Finding
	for _, r := range rows {
		section, name := resolveReadme(r.Name, inv)
		target, ok := catalogRows[section][name]
		if section == "" || !ok {
			findings = append(findings, Finding{"readme-unmapped", sectionReadme, r.Name, "no catalog row for this command"})
			continue
		}
		if firstWord(r.Maturity) != firstWord(target.Maturity) {
			findings = append(findings, Finding{"readme-maturity", sectionReadme, r.Name, fmt.Sprintf("README says %q, catalog says %q", r.Maturity, target.Maturity)})
		}
	}
	return findings
}

// resolveReadme names the catalog section and row a README command invokes.
func resolveReadme(command string, inv Inventory) (string, string) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", ""
	}
	switch {
	case strings.HasPrefix(fields[0], "corvint."):
		return sectionTools, fields[0]
	case fields[0] != "corvint":
		return sectionBinaries, fields[0]
	}
	words := fields[1:]
	best := ""
	for _, c := range inv.Commands {
		parts := strings.Fields(c)
		if len(parts) <= len(words) && slices.Equal(parts, words[:len(parts)]) && len(c) > len(best) {
			best = c
		}
	}
	if best == "" {
		return "", ""
	}
	return sectionCommands, best
}
