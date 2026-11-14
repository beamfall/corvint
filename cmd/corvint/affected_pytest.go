package main

import (
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

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/python"
)

// The advisory pytest check (AFP-V0-472 to AFP-V0-477).
const (
	adviceAdvisoryPytest       = "python -m pytest"
	advicePythonUnitPrefix     = "python:"
	adviceMaxPytestFiles       = 1024
	adviceMaxPytestConfigBytes = 256 << 10
	// The dependency discovery bound (AFP-V0-473): root entries listed,
	// requirements*.txt files read, and manifest bytes read in all.
	adviceMaxPytestRootEntries      = 4096
	adviceMaxPytestRequirements     = 16
	adviceMaxPytestManifestBytes    = 1 << 20
	adviceNoPytestDeclared          = "NO_ADVISORY_PYTEST_COMMAND: the plan selected Python test units but the repository declares no pytest configuration, conftest.py or pytest dependency"
	adviceNoPytestModule            = "NO_ADVISORY_PYTEST_COMMAND: the plan selected %d Python test unit(s) but no selected file matches python_files %s"
	advicePytestBoundUnknown        = "NO_ADVISORY_PYTEST_COMMAND: %d selected test files exceed the %d-file bound"
	advicePytestConfigUnknown       = "NO_ADVISORY_PYTEST_COMMAND: %s may hold the pytest configuration but is not a readable regular file of at most 256 KiB"
	advicePytestSplitUnknown        = "NO_ADVISORY_PYTEST_COMMAND: %s sets %s to a value pytest cannot split"
	advicePytestUnsettled           = "NO_ADVISORY_PYTEST_COMMAND: the pytest configuration pytest would read for the selected test files does not settle (%s, then %s)"
	advicePytestDependencyUnbounded = "NO_ADVISORY_PYTEST_COMMAND: no pytest configuration or conftest.py is declared and the root dependency manifests exceed the discovery bound (%s)"
)

// advicePathListSeparator joins PYTHONPATH roots with the separator of the
// platform corvint runs on, where the agent runs the command (AFP-V0-475).
var advicePathListSeparator = string(os.PathListSeparator)

// adviceDefaultPythonFiles is pytest's default python_files value.
var adviceDefaultPythonFiles = []string{"test_*.py", "*_test.py"}

// advicePytestConfigNames is pytest's per-directory configuration search order.
var advicePytestConfigNames = []string{"pytest.ini", ".pytest.ini", "pyproject.toml", "tox.ini", "setup.cfg"}

// advicePytestDependencyManifests are the root files whose text naming pytest
// declares it as a dependency (AFP-V0-473), besides requirements*.txt.
var advicePytestDependencyManifests = []string{"pyproject.toml", "setup.cfg", "setup.py", "tox.ini", "Pipfile"}

var (
	adviceIniSection   = regexp.MustCompile(`^\[([^\]]+)\]`)
	adviceIniKey       = regexp.MustCompile(`^([^=:\s][^=:]*?)[ \t]*[=:][ \t]*(.*)$`)
	adviceTomlTable    = regexp.MustCompile(`^\[\s*([^\]]+?)\s*\][ \t]*(#.*)?$`)
	adviceTomlKey      = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)[ \t]*=[ \t]*(.*)$`)
	advicePytestNamed  = regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_-])pytest([^A-Za-z0-9_-]|$)`)
	adviceRequirements = regexp.MustCompile(`^requirements[A-Za-z0-9_.-]*\.txt$`)
)

// pytestConfig is what the advice reads from the one configuration file
// pytest would use: its repository-relative path ("" when pytest finds
// none), whether it holds a pytest section (a pyproject.toml without one is
// still pytest's configuration file, but declares nothing), its python_files
// patterns, and its pythonpath entries as repository-relative paths.
type pytestConfig struct {
	file        string
	declared    bool
	pythonFiles []string
	pythonPath  []string
}

// advisoryPytestCheck proposes the pytest test modules the plan selected,
// run from the repository root (AFP-V0-472). It adds nothing when the plan
// selected no Python test file; otherwise every reason it cannot name a
// runnable command is one NO_ADVISORY_PYTEST_COMMAND line.
func advisoryPytestCheck(root string, graph *affected.Graph, plan affected.Plan) ([]affectedCheck, []string) {
	units := map[string][]string{}
	seen := map[string]bool{}
	candidates := []string{}
	for _, selection := range plan.Selected {
		if !strings.HasPrefix(selection.UnitID, advicePythonUnitPrefix) {
			continue
		}
		for _, file := range selection.Tests {
			if strings.HasSuffix(file, ".py") {
				units[selection.UnitID] = append(units[selection.UnitID], file)
				if !seen[file] {
					seen[file] = true
					candidates = append(candidates, file)
				}
			}
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Strings(candidates)
	config, files, unknown := settlePytestConfig(root, candidates)
	if unknown != "" {
		return nil, []string{unknown}
	}
	if !config.declared && !hasConftest(graph, candidates) {
		declared, bound := adviceDeclaresPytestDependency(root)
		if bound != "" {
			return nil, []string{fmt.Sprintf(advicePytestDependencyUnbounded, bound)}
		}
		if !declared {
			return nil, []string{adviceNoPytestDeclared}
		}
	}
	if len(files) == 0 {
		patterns := strings.Join(config.pythonFiles, " ")
		if patterns == "" {
			patterns = "(empty)"
		}
		return nil, []string{fmt.Sprintf(adviceNoPytestModule, len(units), patterns)}
	}
	if len(files) > adviceMaxPytestFiles {
		return nil, []string{fmt.Sprintf(advicePytestBoundUnknown, len(files), adviceMaxPytestFiles)}
	}
	// A leading `./` keeps a dash-leading file from being read as a pytest
	// option, as AFP-V0-066 does for RSpec.
	operands := make([]string, len(files))
	for index, file := range files {
		operands[index] = "./" + file
	}
	command := adviceAdvisoryPytest + " " + shellQuoteJoin(operands)
	reason := fmt.Sprintf("the plan selected %d Python test unit(s); run from the repository root, where ", len(units))
	if config.file != "" {
		reason += "pytest reads its configuration from " + config.file
	} else {
		reason += "no pytest configuration lies between these files and the repository root, so pytest's defaults apply"
	}
	if roots := pytestSourceRoots(graph, units, files, config.pythonPath); len(roots) > 0 {
		command = "PYTHONPATH=" + shellQuoteJoin([]string{strings.Join(roots, advicePathListSeparator)}) + " " + command
		reason += fmt.Sprintf(", and PYTHONPATH adds the source root(s) %s that these tests import from", strings.Join(roots, ", "))
	}
	reason += ", so these test modules are the plausible first pass"
	return []affectedCheck{{Command: command, Kind: adviceKindAdvisory, Reason: reason, Source: adviceSourcePlan}}, nil
}

// settlePytestConfig finds the configuration pytest would read for the test
// modules it is given and filters the candidates by that configuration's
// python_files (AFP-V0-474). Filtering changes pytest's arguments, and so can
// change the configuration it finds, so the search repeats until the
// configuration is the one the filtered files find, at most three times; a
// configuration that does not settle is an unknown.
func settlePytestConfig(root string, candidates []string) (pytestConfig, []string, string) {
	config, unknown := determinePytestConfig(root, candidates)
	if unknown != "" {
		return pytestConfig{}, nil, unknown
	}
	for attempt := 0; ; attempt++ {
		files := matchPythonFiles(candidates, config.pythonFiles)
		if len(files) == 0 {
			return config, nil, ""
		}
		next, unknown := determinePytestConfig(root, files)
		if unknown != "" {
			return pytestConfig{}, nil, unknown
		}
		if next.file == config.file {
			return config, files, ""
		}
		if attempt == 2 {
			return pytestConfig{}, nil, fmt.Sprintf(advicePytestUnsettled, pytestConfigName(config), pytestConfigName(next))
		}
		config = next
	}
}

func pytestConfigName(config pytestConfig) string {
	if config.file == "" {
		return "no configuration"
	}
	return config.file
}

// commonDirectory is the deepest repository-relative directory holding every
// file, "." for the repository root, as pytest's common ancestor of its
// arguments' directories.
func commonDirectory(files []string) string {
	common := strings.Split(path.Dir(files[0]), "/")
	for _, file := range files[1:] {
		parts := strings.Split(path.Dir(file), "/")
		length := 0
		for length < len(common) && length < len(parts) && common[length] == parts[length] {
			length++
		}
		common = common[:length]
	}
	if len(common) == 0 {
		return "."
	}
	return strings.Join(common, "/")
}

// determinePytestConfig follows pytest 8's determine_setup for the files as
// its arguments, run from the repository root with no -c or --rootdir
// (AFP-V0-474): locate a configuration from their common directory; when none
// is found and no setup.py lies between that directory and the repository
// root, locate one from each argument's own directory in argument order.
func determinePytestConfig(root string, files []string) (pytestConfig, string) {
	ancestor := commonDirectory(files)
	config, found, unknown := locatePytestConfig(root, []string{ancestor})
	if found || unknown != "" {
		return config, unknown
	}
	for directory := ancestor; ; directory = path.Dir(directory) {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path.Join(directory, "setup.py")))); err == nil && info.Mode().IsRegular() {
			return config, ""
		}
		if directory == "." {
			break
		}
	}
	directories := make([]string, len(files))
	for index, file := range files {
		directories[index] = path.Dir(file)
	}
	config, _, unknown = locatePytestConfig(root, directories)
	return config, unknown
}

// locatePytestConfig applies pytest's locate_config to the directories in
// order, each searched up to the repository root: in each directory,
// pytest.ini, .pytest.ini, pyproject.toml with a [tool.pytest.ini_options] or
// [tool.pytest] table, tox.ini with a [pytest] section and setup.cfg with a
// [tool:pytest] section, in that order; the first that holds a configuration
// wins. A pytest.ini always does, even when empty. When none does, the first
// pyproject.toml seen is the configuration file with pytest's defaults. A
// candidate that exists but cannot be read as a bounded regular file, or that
// sets a value pytest cannot split, is an unknown.
func locatePytestConfig(root string, directories []string) (pytestConfig, bool, string) {
	fallback := ""
	searched := map[string]bool{}
	for _, directory := range directories {
		for ; !searched[directory]; directory = path.Dir(directory) {
			searched[directory] = true
			for _, name := range advicePytestConfigNames {
				relative := path.Join(directory, name)
				full := filepath.Join(root, filepath.FromSlash(relative))
				if _, err := os.Lstat(full); errors.Is(err, fs.ErrNotExist) {
					continue
				}
				body, err := readBoundedFile(full, adviceMaxPytestConfigBytes)
				if err != nil {
					return pytestConfig{}, false, fmt.Sprintf(advicePytestConfigUnknown, relative)
				}
				values, ok := pytestConfigValues(name, string(body))
				if !ok {
					if name == "pyproject.toml" && fallback == "" {
						fallback = relative
					}
					continue
				}
				return pytestConfigFrom(relative, values)
			}
			if directory == "." {
				break
			}
		}
	}
	if fallback != "" {
		return pytestConfig{file: fallback, pythonFiles: adviceDefaultPythonFiles}, true, ""
	}
	return pytestConfig{pythonFiles: adviceDefaultPythonFiles}, false, ""
}

// pytestConfigFrom reads python_files and pythonpath from one configuration
// file's values. An ini string is split as pytest splits it (shlex); a TOML
// array is taken as is. A key that is present but empty is kept empty: an
// empty python_files collects nothing.
func pytestConfigFrom(file string, values map[string]pytestValue) (pytestConfig, bool, string) {
	config := pytestConfig{file: file, declared: true, pythonFiles: adviceDefaultPythonFiles}
	for _, key := range []string{"python_files", "pythonpath"} {
		value, ok := values[key]
		if !ok {
			continue
		}
		items := value.items
		if !value.array {
			split, ok := shlexSplit(value.text)
			if !ok {
				return pytestConfig{}, false, fmt.Sprintf(advicePytestSplitUnknown, file, key)
			}
			items = split
		}
		if key == "python_files" {
			config.pythonFiles = items
			continue
		}
		for _, entry := range items {
			if !strings.HasPrefix(entry, "/") {
				config.pythonPath = append(config.pythonPath, path.Join(path.Dir(file), entry))
			}
		}
	}
	return config, true, ""
}

// pytestValue is one configuration value: an ini or TOML string, or a TOML
// array of strings.
type pytestValue struct {
	text  string
	items []string
	array bool
}

// pytestConfigValues returns the values of one configuration file's pytest
// section and whether the file holds a pytest configuration at all.
func pytestConfigValues(name, body string) (map[string]pytestValue, bool) {
	switch name {
	case "pyproject.toml":
		return tomlPytestValues(body)
	case "setup.cfg":
		return iniPytestValues(body, "tool:pytest", false)
	default:
		return iniPytestValues(body, "pytest", name == "pytest.ini")
	}
}

// iniPytestValues reads one section of an ini file as pytest's iniconfig
// does: `key = value` or `key: value` lines, indented continuation lines
// joined to the value with newlines, whole-line `#` or `;` comments skipped,
// and no inline comment removed.
func iniPytestValues(body, section string, always bool) (map[string]pytestValue, bool) {
	values := map[string]pytestValue{}
	found, inside := false, false
	key := ""
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if match := adviceIniSection.FindStringSubmatch(line); match != nil {
			inside = strings.TrimSpace(match[1]) == section
			found = found || inside
			key = ""
			continue
		}
		if !inside {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			if key != "" {
				value := values[key]
				if value.text != "" {
					value.text += "\n"
				}
				value.text += trimmed
				values[key] = value
			}
			continue
		}
		key = ""
		if match := adviceIniKey.FindStringSubmatch(line); match != nil {
			key = strings.TrimSpace(match[1])
			values[key] = pytestValue{text: strings.TrimSpace(match[2])}
		}
	}
	return values, found || always
}

// tomlPytestValues reads the [tool.pytest.ini_options] (or pytest 9
// [tool.pytest]) table of a pyproject.toml: each key's string, or its array
// of strings joined across lines until its brackets close, with comments
// outside strings removed. A table declared another way (dotted keys, an
// inline table) is not read.
func tomlPytestValues(body string) (map[string]pytestValue, bool) {
	values := map[string]pytestValue{}
	found, inside := false, false
	lines := strings.Split(body, "\n")
	for index := 0; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if match := adviceTomlTable.FindStringSubmatch(trimmed); match != nil {
			inside = match[1] == "tool.pytest.ini_options" || match[1] == "tool.pytest"
			found = found || inside
			continue
		}
		if !inside {
			continue
		}
		match := adviceTomlKey.FindStringSubmatch(trimmed)
		if match == nil {
			continue
		}
		var value pytestValue
		value, index = tomlValue(lines, index, match[2])
		values[match[1]] = value
	}
	return values, found
}

// tomlValue scans one TOML value that begins with text on lines[index]: its
// basic ("...") and literal ('...') strings, an array continuing over the
// following lines until its brackets close, and `#` comments outside strings,
// which are dropped. It returns the value and the last line it consumed.
func tomlValue(lines []string, index int, text string) (pytestValue, int) {
	value := pytestValue{}
	scalar := strings.Builder{}
	depth := 0
	for position := 0; ; position++ {
		if position >= len(text) {
			if depth > 0 && index+1 < len(lines) {
				index++
				text += "\n" + lines[index]
				position--
				continue
			}
			break
		}
		switch character := text[position]; character {
		case '#':
			for position < len(text) && text[position] != '\n' {
				position++
			}
			if depth == 0 {
				position = len(text)
			}
		case '[':
			depth++
			value.array = true
		case ']':
			depth--
			if depth <= 0 {
				position = len(text)
			}
		case '"', '\'':
			literal, end := tomlString(text, position)
			if depth > 0 {
				value.items = append(value.items, literal)
			} else {
				scalar.WriteString(literal)
			}
			position = end
		default:
			if depth == 0 && character != ' ' && character != '\t' && character != '\r' {
				scalar.WriteByte(character)
			}
		}
		if position >= len(text) && depth <= 0 {
			break
		}
	}
	value.text = scalar.String()
	return value, index
}

// tomlString reads the basic or literal string opening at text[start] and
// returns its value and the index of its closing quote (or the text's end).
// A basic string's `\"` and `\\` escapes are honoured.
func tomlString(text string, start int) (string, int) {
	quote := text[start]
	var literal strings.Builder
	position := start + 1
	for ; position < len(text) && text[position] != quote && text[position] != '\n'; position++ {
		if quote == '"' && text[position] == '\\' && position+1 < len(text) {
			position++
		}
		literal.WriteByte(text[position])
	}
	return literal.String(), position
}

// shlexSplit splits a value as Python's shlex.split does in POSIX mode:
// whitespace separates words, single quotes are literal, double quotes honour
// `\"` and `\\`, and a backslash outside quotes escapes the next character.
// An unclosed quote or a trailing backslash cannot be split.
func shlexSplit(text string) ([]string, bool) {
	words := []string{}
	var word strings.Builder
	inWord := false
	for position := 0; position < len(text); position++ {
		switch character := text[position]; character {
		case ' ', '\t', '\n', '\r':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		case '\\':
			if position+1 >= len(text) {
				return nil, false
			}
			position++
			word.WriteByte(text[position])
			inWord = true
		case '\'':
			end := strings.IndexByte(text[position+1:], '\'')
			if end < 0 {
				return nil, false
			}
			word.WriteString(text[position+1 : position+1+end])
			position += end + 1
			inWord = true
		case '"':
			inWord = true
			for position++; ; position++ {
				if position >= len(text) {
					return nil, false
				}
				if text[position] == '"' {
					break
				}
				if text[position] == '\\' && position+1 < len(text) && (text[position+1] == '"' || text[position+1] == '\\') {
					position++
				}
				word.WriteByte(text[position])
			}
		default:
			word.WriteByte(character)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, true
}

// matchPythonFiles keeps the candidates pytest's python_files patterns
// collect, sorted. A pattern with no `/` matches the file name; one with a
// `/` matches the end of the path, as pytest's fnmatch_ex does. Helper
// modules, conftest.py and __init__.py under a test directory are test units
// of the Python plugin but not test modules, so they drop out here.
func matchPythonFiles(candidates, patterns []string) []string {
	type matcher struct {
		expression *regexp.Regexp
		wholePath  bool
	}
	matchers := make([]matcher, 0, len(patterns))
	for _, pattern := range patterns {
		if strings.Contains(pattern, "/") {
			matchers = append(matchers, matcher{fnmatchExpression("*/" + pattern), true})
		} else {
			matchers = append(matchers, matcher{fnmatchExpression(pattern), false})
		}
	}
	files := []string{}
	for _, file := range candidates {
		for _, match := range matchers {
			subject := path.Base(file)
			if match.wholePath {
				subject = "/" + file
			}
			if match.expression.MatchString(subject) {
				files = append(files, file)
				break
			}
		}
	}
	return files
}

// fnmatchExpression compiles Python's fnmatch.fnmatchcase pattern: `*`
// matches any run of characters, a path separator included, `?` one
// character, and `[...]` or `[!...]` a character class.
func fnmatchExpression(pattern string) *regexp.Regexp {
	var expression strings.Builder
	expression.WriteString("^")
	for index := 0; index < len(pattern); index++ {
		switch character := pattern[index]; character {
		case '*':
			expression.WriteString(".*")
		case '?':
			expression.WriteString(".")
		case '[':
			end := strings.IndexByte(pattern[index+1:], ']')
			if end < 0 {
				expression.WriteString(`\[`)
				continue
			}
			class := pattern[index+1 : index+1+end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			expression.WriteString("[" + strings.ReplaceAll(class, `\`, `\\`) + "]")
			index += end + 1
		default:
			expression.WriteString(regexp.QuoteMeta(string(character)))
		}
	}
	expression.WriteString("$")
	if compiled, err := regexp.Compile(expression.String()); err == nil {
		return compiled
	}
	return regexp.MustCompile(`^\b$`) // never matches
}

// hasConftest reports whether the graph, or the candidate files when there is
// no graph, holds a conftest.py: a repository that has one uses pytest.
func hasConftest(graph *affected.Graph, candidates []string) bool {
	for _, file := range candidates {
		if path.Base(file) == "conftest.py" {
			return true
		}
	}
	if graph == nil {
		return false
	}
	for _, id := range graph.UnitIDs() {
		if id == advicePythonUnitPrefix+"conftest" || (strings.HasPrefix(id, advicePythonUnitPrefix) && strings.HasSuffix(id, ".conftest")) {
			return true
		}
	}
	return false
}

// adviceDeclaresPytestDependency reports whether a root dependency manifest
// (pyproject.toml, setup.cfg, setup.py, tox.ini, Pipfile or a
// requirements*.txt) names pytest, each read as a regular file of at most
// 256 KiB. The search is bounded: at most 4096 root entries listed, 16
// requirements*.txt files and 1 MiB read in all; reaching a bound before
// pytest is found returns the bound that was hit (AFP-V0-473).
func adviceDeclaresPytestDependency(root string) (bool, string) {
	names := append([]string{}, advicePytestDependencyManifests...)
	if directory, err := os.Open(root); err == nil {
		entries, err := directory.ReadDir(adviceMaxPytestRootEntries + 1)
		directory.Close()
		if len(entries) > adviceMaxPytestRootEntries {
			return false, fmt.Sprintf("more than %d repository root entries", adviceMaxPytestRootEntries)
		}
		if err == nil || errors.Is(err, io.EOF) {
			requirements := []string{}
			for _, entry := range entries {
				if adviceRequirements.MatchString(entry.Name()) {
					requirements = append(requirements, entry.Name())
				}
			}
			sort.Strings(requirements)
			names = append(names, requirements...)
		}
	}
	read, requirements := 0, 0
	for _, name := range names {
		full := filepath.Join(root, name)
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if adviceRequirements.MatchString(name) {
			if requirements++; requirements > adviceMaxPytestRequirements {
				return false, fmt.Sprintf("more than %d requirements*.txt files", adviceMaxPytestRequirements)
			}
		}
		if read+int(min(info.Size(), adviceMaxPytestConfigBytes)) > adviceMaxPytestManifestBytes {
			return false, fmt.Sprintf("more than %d bytes of manifests", adviceMaxPytestManifestBytes)
		}
		body, err := readBoundedFile(full, adviceMaxPytestConfigBytes)
		read += len(body)
		if err == nil && advicePytestNamed.Match(body) {
			return true, ""
		}
	}
	return false, ""
}

// pytestSourceRoots names each Python source root (python.SourceRoots) that
// the emitted test modules reach through the graph's import edges, so the
// tests import its modules by their root-relative names and the root must be
// on the module path (AFP-V0-475). A test module under the root is no
// exception: pytest's rootdir-relative import puts the test's own directory
// on the path, not the root. A root that is itself a regular package (it
// holds __init__.py) is importable by its own name, and a root the
// configuration's pythonpath already names is on the path, so neither is
// added.
func pytestSourceRoots(graph *affected.Graph, units map[string][]string, files, declared []string) []string {
	if graph == nil {
		return nil
	}
	emitted := make(map[string]bool, len(files))
	for _, file := range files {
		emitted[file] = true
	}
	roots := []string{}
	for _, sourceRoot := range python.SourceRoots {
		if slices.Contains(declared, sourceRoot) {
			continue
		}
		if unit, ok := graph.Unit(advicePythonUnitPrefix + sourceRoot); ok && slices.Contains(unit.Sources, sourceRoot+"/__init__.py") {
			continue
		}
		queue := []string{}
		visited := map[string]bool{}
		for id, testFiles := range units {
			if !visited[id] && slices.ContainsFunc(testFiles, func(file string) bool { return emitted[file] }) {
				visited[id] = true
				queue = append(queue, id)
			}
		}
		sort.Strings(queue)
		prefix := advicePythonUnitPrefix + sourceRoot + "."
	search:
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			unit, ok := graph.Unit(id)
			if !ok {
				continue
			}
			for _, target := range unit.Imports {
				if strings.HasPrefix(target, prefix) {
					roots = append(roots, sourceRoot)
					break search
				}
				if !visited[target] {
					visited[target] = true
					queue = append(queue, target)
				}
			}
		}
	}
	return roots
}
