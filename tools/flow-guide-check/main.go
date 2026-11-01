// Command flow-guide-check is the lane flow guide drift gate and the refusal-hint
// ratchet (docs/specs/corvint-self-development-v0.md, ASD-V0-996 to ASD-V0-998). Run from
// the repository root, it fails unless every command the guide (section 13 of
// docs/LANE-ORCHESTRATION.md) and the lane-flow skill name resolves against the command-tree
// snapshots, the Makefile and script/, every refusal code the guide's table maps occurs in
// source, and every refusal site on the gated surfaces carries a recovery hint or is listed in
// unhinted-refusals.tsv. With --audit it prints the refusal audit counts instead. It reads
// files only; it runs nothing and writes nothing.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/clitree"
)

const (
	guidePath     = "docs/LANE-ORCHESTRATION.md"
	guideHeading  = "## 13. "
	skillPath     = ".claude/skills/corvint-lane-flow/SKILL.md"
	treesDir      = "internal/capcatalog/trees"
	baselinePath  = "tools/flow-guide-check/unhinted-refusals.tsv"
	dogfoodDir    = "internal/dogfoodflow"
	recoveriesVar = "failRecoveries"
)

// hintMarkers is the closed set of message fragments that count as a recovery hint (ASD-V0-997).
var hintMarkers = []string{"fix:", "want ", "run ", "rerun", "set ", "use ", "usage:"}

var (
	backticked  = regexp.MustCompile("`([^`]+)`")
	kebabCode   = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)+$`)
	upperCode   = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+$`)
	shellRefuse = regexp.MustCompile(`\brefuse "((?:[^"\\]|\\.)*)"`)
	makeRule    = regexp.MustCompile(`^([A-Za-z0-9_./%-]+(?:[ \t]+[A-Za-z0-9_./%-]+)*)[ \t]*::?(?:[^=]|$)`)
)

func main() {
	os.Exit(run(".", os.Args[1:], os.Stdout, os.Stderr))
}

func run(root string, arguments []string, stdout, stderr io.Writer) int {
	audit := false
	switch {
	case len(arguments) == 1 && arguments[0] == "--audit":
		audit = true
	case len(arguments) != 0:
		fmt.Fprintln(stderr, "usage: go run ./tools/flow-guide-check [--audit]")
		return 2
	}
	repository, err := load(root)
	if err != nil {
		fmt.Fprintln(stderr, "flow-guide-check:", err)
		return 2
	}
	sites, err := refusalSites(root, repository)
	if err != nil {
		fmt.Fprintln(stderr, "flow-guide-check:", err)
		return 2
	}
	if audit {
		unclassified, err := unclassifiedCounts(root)
		if err != nil {
			fmt.Fprintln(stderr, "flow-guide-check:", err)
			return 2
		}
		writeAudit(stdout, sites, unclassified)
		return 0
	}
	findings := checkGuide(root, repository)
	ratchet, err := checkRatchet(root, sites)
	if err != nil {
		fmt.Fprintln(stderr, "flow-guide-check:", err)
		return 2
	}
	findings = append(findings, ratchet...)
	for _, finding := range findings {
		fmt.Fprintln(stderr, "flow-guide-check:", finding)
	}
	if len(findings) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "flow-guide-check: ok (%d guide spans, %d mapped refusal codes, %d refusal sites)\n", repository.spanCount, len(repository.guideCodes), len(sites))
	return 0
}

// repository holds what the checks read once.
type repository struct {
	documents  map[string]string // path -> checked text (the guide section, the skill)
	offsets    map[string]int    // path -> line number of the text's first line
	trees      map[string]clitree.Tree
	targets    map[string]bool
	guideCodes []string
	spanCount  int
}

func load(root string) (*repository, error) {
	repo := &repository{documents: map[string]string{}, offsets: map[string]int{}, trees: map[string]clitree.Tree{}}
	guide, err := os.ReadFile(filepath.Join(root, guidePath))
	if err != nil {
		return nil, err
	}
	section, line, found := sectionOf(string(guide), guideHeading)
	if !found {
		return nil, fmt.Errorf("%s has no %q section", guidePath, strings.TrimSpace(guideHeading))
	}
	repo.documents[guidePath], repo.offsets[guidePath] = section, line
	skill, err := os.ReadFile(filepath.Join(root, skillPath))
	if err != nil {
		return nil, err
	}
	repo.documents[skillPath], repo.offsets[skillPath] = string(skill), 1
	for _, binary := range []string{"corvint", "corvint-tasks"} {
		raw, err := os.ReadFile(filepath.Join(root, treesDir, binary+".json"))
		if err != nil {
			return nil, err
		}
		var tree clitree.Tree
		if err := json.Unmarshal(raw, &tree); err != nil {
			return nil, fmt.Errorf("%s/%s.json: %w", treesDir, binary, err)
		}
		repo.trees[binary] = tree
	}
	if repo.targets, err = makeTargets(filepath.Join(root, "Makefile")); err != nil {
		return nil, err
	}
	repo.guideCodes = tableCodes(section)
	return repo, nil
}

// sectionOf returns the text from the heading line to the next level-2 heading and the
// heading's line number.
func sectionOf(document, heading string) (string, int, bool) {
	lines := strings.SplitAfter(document, "\n")
	for index, line := range lines {
		if !strings.HasPrefix(line, heading) {
			continue
		}
		end := index + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], "## ") {
			end++
		}
		return strings.Join(lines[index:end], ""), index + 1, true
	}
	return "", 0, false
}

func makeTargets(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	targets := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if match := makeRule.FindStringSubmatch(line); match != nil {
			for _, target := range strings.Fields(match[1]) {
				targets[target] = true
			}
		}
	}
	return targets, nil
}

// tableCodes returns the codes in the first column of the guide's refusal table.
func tableCodes(section string) []string {
	var codes []string
	inTable := false
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "| Refusal |") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		for _, span := range backticked.FindAllStringSubmatch(cells[1], -1) {
			for _, token := range strings.FieldsFunc(span[1], func(r rune) bool { return r == ' ' || r == ',' || r == '=' }) {
				if (kebabCode.MatchString(token) || upperCode.MatchString(token)) && !slices.Contains(codes, token) {
					codes = append(codes, token)
				}
			}
		}
	}
	return codes
}

func checkGuide(root string, repo *repository) []string {
	var findings []string
	for _, path := range []string{guidePath, skillPath} {
		text := repo.documents[path]
		for _, match := range backticked.FindAllStringSubmatchIndex(text, -1) {
			span := strings.Join(strings.Fields(text[match[2]:match[3]]), " ")
			line := repo.offsets[path] + strings.Count(text[:match[0]], "\n")
			problem, checked := checkSpan(root, repo, span)
			if checked {
				repo.spanCount++
			}
			if problem != "" {
				findings = append(findings, fmt.Sprintf("%s:%d: `%s`: %s", path, line, span, problem))
			}
		}
	}
	sources, err := sourceText(root)
	if err != nil {
		return append(findings, err.Error())
	}
	for _, code := range repo.guideCodes {
		if !bytes.Contains(sources, []byte(code)) {
			findings = append(findings, fmt.Sprintf("%s: refusal code %s in the recovery table occurs in no non-test source under cmd/, internal/, script/ or tools/", guidePath, code))
		}
	}
	return findings
}

// checkSpan resolves one backticked span; the second result reports whether it named a
// command the check covers.
func checkSpan(root string, repo *repository, span string) (string, bool) {
	tokens := strings.Fields(span)
	for index, token := range tokens {
		if token == ">" || token == "|" || token == ";" || token == "&&" || strings.HasPrefix(token, ">") {
			tokens = tokens[:index]
			break
		}
	}
	// A bare binary name, a glob or a directory names no command to resolve.
	if len(tokens) == 0 || (len(tokens) == 1 && (tokens[0] == "corvint" || tokens[0] == "corvint-tasks" || tokens[0] == "make")) ||
		strings.ContainsAny(tokens[0], "*?[") || strings.HasSuffix(tokens[0], "/") {
		return "", false
	}
	switch {
	case tokens[0] == "corvint" || tokens[0] == "corvint-tasks":
		return checkCommand(repo.trees[tokens[0]], tokens[0], tokens[1:]), true
	case tokens[0] == "make":
		return checkMake(repo, tokens[1:]), true
	case strings.HasPrefix(tokens[0], "script/"):
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(tokens[0]))); err != nil || !info.Mode().IsRegular() {
			return "no such script " + tokens[0], true
		}
		if tokens[0] == "script/heavy.sh" && len(tokens) > 1 && tokens[1] == "make" {
			return checkMake(repo, tokens[2:]), true
		}
		return "", true
	}
	return "", false
}

func checkCommand(tree clitree.Tree, binary string, arguments []string) string {
	var words []string
	for _, argument := range arguments {
		if !clitree.OperationWord(argument) {
			break
		}
		words = append(words, argument)
	}
	var node *clitree.Node
	for length := len(words); length > 0 && node == nil; length-- {
		for index := range tree.Nodes {
			if slices.Equal(tree.Nodes[index].Path, words[:length]) {
				node = &tree.Nodes[index]
				break
			}
		}
	}
	if node == nil {
		if len(words) == 0 {
			return "names no " + binary + " subcommand"
		}
		return "no " + binary + " subcommand " + strings.Join(words, " ")
	}
	for _, argument := range arguments {
		if !strings.HasPrefix(argument, "-") {
			continue
		}
		flag, _, _ := strings.Cut(argument, "=")
		if flag != "--help" && !slices.Contains(node.Flags, flag) && !slices.Contains(tree.Global, flag) {
			return node.Command + " has no flag " + flag
		}
	}
	return ""
}

func checkMake(repo *repository, arguments []string) string {
	named := false
	for _, argument := range arguments {
		if strings.HasPrefix(argument, "-") || strings.Contains(argument, "=") {
			continue
		}
		named = true
		if !repo.targets[argument] {
			return "no Makefile rule " + argument
		}
	}
	if !named {
		return "names no Makefile rule"
	}
	return ""
}

// sourceText concatenates the non-test sources the refusal codes must occur in.
func sourceText(root string) ([]byte, error) {
	var all bytes.Buffer
	for _, dir := range []string{"cmd", "internal", "script", "tools"} {
		if _, err := os.Stat(filepath.Join(root, dir)); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" || entry.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			name := entry.Name()
			if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.sh") || !(strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".sh")) {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			all.Write(raw)
			all.WriteByte('\n')
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return all.Bytes(), nil
}

// site is one refusal call on a gated surface.
type site struct {
	surface, path, code string
	line                int
	hinted              bool
}

func hinted(message string) bool {
	for _, marker := range hintMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func refusalSites(root string, repo *repository) ([]site, error) {
	var sites []site
	scripts, err := filepath.Glob(filepath.Join(root, "script", "*.sh"))
	if err != nil {
		return nil, err
	}
	sort.Strings(scripts)
	for _, path := range scripts {
		if strings.HasSuffix(path, "_test.sh") {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		relative := "script/" + filepath.Base(path)
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for number := 1; scanner.Scan(); number++ {
			for _, match := range shellRefuse.FindAllStringSubmatch(scanner.Text(), -1) {
				code, _, _ := strings.Cut(match[1], " ")
				if code == "" {
					continue
				}
				code = strings.TrimSuffix(code, ":")
				sites = append(sites, site{"script", relative, code, number, hinted(match[1]) || slices.Contains(repo.guideCodes, code)})
			}
		}
		err = scanner.Err()
		file.Close()
		if err != nil {
			return nil, err
		}
	}
	goSites, err := dogfoodSites(root, repo)
	if err != nil {
		return nil, err
	}
	return append(sites, goSites...), nil
}

func dogfoodSites(root string, repo *repository) ([]site, error) {
	fileSet := token.NewFileSet()
	dir := filepath.Join(root, filepath.FromSlash(dogfoodDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		files, paths = append(files, parsed), append(paths, dogfoodDir+"/"+name)
	}
	recoveries := recoveryKeys(files)
	var sites []site
	for index, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "fail" && selector.Sel.Name != "refuse") {
				return true
			}
			first, ok := call.Args[0].(*ast.BasicLit)
			if !ok || first.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(first.Value)
			if err != nil {
				return true
			}
			code, _, _ := strings.Cut(value, " ")
			code = strings.TrimSuffix(code, ":")
			var message strings.Builder
			for _, argument := range call.Args {
				ast.Inspect(argument, func(inner ast.Node) bool {
					if literal, ok := inner.(*ast.BasicLit); ok && literal.Kind == token.STRING {
						if text, err := strconv.Unquote(literal.Value); err == nil {
							message.WriteString(text)
							message.WriteByte(' ')
						}
					}
					return true
				})
			}
			isHinted := hinted(message.String()) || slices.Contains(repo.guideCodes, code) ||
				(selector.Sel.Name == "fail" && recoveries[code])
			sites = append(sites, site{"dogfoodflow", paths[index], code, fileSet.Position(call.Pos()).Line, isHinted})
			return true
		})
	}
	return sites, nil
}

// recoveryKeys returns the literal keys of the package's failRecoveries map.
func recoveryKeys(files []*ast.File) map[string]bool {
	keys := map[string]bool{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != 1 || value.Names[0].Name != recoveriesVar || len(value.Values) != 1 {
					continue
				}
				literal, ok := value.Values[0].(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, element := range literal.Elts {
					if pair, ok := element.(*ast.KeyValueExpr); ok {
						if key, ok := pair.Key.(*ast.BasicLit); ok && key.Kind == token.STRING {
							if text, err := strconv.Unquote(key.Value); err == nil {
								keys[text] = true
							}
						}
					}
				}
			}
		}
	}
	return keys
}

// checkRatchet compares the unhinted file and code pairs with the baseline.
func checkRatchet(root string, sites []site) ([]string, error) {
	unhinted := map[string]site{}
	for _, current := range sites {
		if !current.hinted {
			key := current.path + "\t" + current.code
			if _, found := unhinted[key]; !found {
				unhinted[key] = current
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, baselinePath))
	if err != nil {
		return nil, err
	}
	listed := map[string]bool{}
	var findings []string
	for number, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Count(line, "\t") != 1 {
			findings = append(findings, fmt.Sprintf("%s:%d: want PATH<TAB>CODE", baselinePath, number+1))
			continue
		}
		listed[line] = true
		if _, found := unhinted[line]; !found {
			findings = append(findings, fmt.Sprintf("%s:%d: stale row %q: the code now carries a recovery hint everywhere or is gone; delete the row", baselinePath, number+1, line))
		}
	}
	keys := make([]string, 0, len(unhinted))
	for key := range unhinted {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !listed[key] {
			current := unhinted[key]
			findings = append(findings, fmt.Sprintf("%s:%d: refusal %s names no recovery: add a fix: line (or a failRecoveries entry), name the valid form (want, run, use, set, usage:), or map it in the %s refusal table", current.path, current.line, current.code, guidePath))
		}
	}
	return findings, nil
}

func writeAudit(stdout io.Writer, sites []site, unclassified map[string]int) {
	fmt.Fprintln(stdout, "flow-guide-check: refusal audit (ASD-V0-998)")
	for _, surface := range []string{"script", "dogfoodflow"} {
		total, hintedSites := 0, 0
		codes := map[string]int{}
		for _, current := range sites {
			if current.surface != surface {
				continue
			}
			total++
			if current.hinted {
				hintedSites++
			} else {
				codes[current.code]++
			}
		}
		fmt.Fprintf(stdout, "surface=%s sites=%d hinted=%d unhinted=%d unhinted_codes=%d\n", surface, total, hintedSites, total-hintedSites, len(codes))
		names := make([]string, 0, len(codes))
		for code := range codes {
			names = append(names, code)
		}
		sort.Slice(names, func(i, j int) bool {
			if codes[names[i]] != codes[names[j]] {
				return codes[names[i]] > codes[names[j]]
			}
			return names[i] < names[j]
		})
		for _, code := range names {
			fmt.Fprintf(stdout, "  unhinted %s sites=%d\n", code, codes[code])
		}
	}
	for _, area := range []string{"corvint", "corvint-tasks"} {
		fmt.Fprintf(stdout, "surface=%s NOT_CLASSIFIED refusal_call_sites=%d\n", area, unclassified[area])
	}
}

// unclassifiedCounts counts Go calls whose callee name starts with refuse or Refuse outside
// the gated dogfoodflow surface: internal/tasks and cmd/corvint-tasks for corvint-tasks,
// the rest of cmd/ and internal/ for corvint.
func unclassifiedCounts(root string) (map[string]int, error) {
	counts := map[string]int{}
	for _, dir := range []string{"cmd", "internal"} {
		if _, err := os.Stat(filepath.Join(root, dir)); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(root, path)
			relative = filepath.ToSlash(relative)
			if entry.IsDir() {
				if entry.Name() == "testdata" || relative == dogfoodDir {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(relative, ".go") || strings.HasSuffix(relative, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return nil
			}
			area := "corvint"
			if strings.HasPrefix(relative, "internal/tasks/") || strings.HasPrefix(relative, "cmd/corvint-tasks/") {
				area = "corvint-tasks"
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					name := ""
					switch callee := call.Fun.(type) {
					case *ast.Ident:
						name = callee.Name
					case *ast.SelectorExpr:
						name = callee.Sel.Name
					}
					if strings.HasPrefix(name, "refuse") || strings.HasPrefix(name, "Refuse") {
						counts[area]++
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return counts, nil
}
