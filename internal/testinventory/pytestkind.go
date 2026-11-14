// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"path"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
)

// The test kinds (PYR-V1-005, G13) and where a kind came from. A kind is a
// label only: it never changes selection in 1.0.
const (
	KindUnit        = "unit"
	KindIntegration = "integration"
	KindE2E         = "e2e"
	KindUnknown     = "UNKNOWN"
	// KindSourceRunner: the test's own pytest constructs, a pytest-playwright
	// fixture or the `integration` marker, gave the kind.
	KindSourceRunner = "runner"
	// KindSourceDefault: the test carries neither, so it is a unit test.
	KindSourceDefault = "default"
	// KindSourceConflict: the test carries both, so its kind is UNKNOWN.
	KindSourceConflict = "conflict"
	// KindSourceUnresolved: the test's function is not found in its file
	// (a generated or inherited test), so its kind is UNKNOWN.
	KindSourceUnresolved = "unresolved"
)

// pytestPlaywrightFixtures are the fixtures the pytest-playwright plugin
// provides (https://playwright.dev/python/docs/test-runners#fixtures, read
// 2026-11-12, NOT_VERIFIED against an installed plugin). A test taking one
// is a browser end-to-end test when the plugin is in use.
var pytestPlaywrightFixtures = map[string]bool{
	"page": true, "context": true, "browser": true, "browser_type": true, "browser_name": true, "browser_channel": true,
	"browser_context_args": true, "browser_type_launch_args": true, "is_chromium": true, "is_firefox": true, "is_webkit": true,
	"playwright": true, "new_context": true, "launch_browser": true,
}

var (
	pytestDef          = regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	pytestClass        = regexp.MustCompile(`^class\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	pytestMarkAssign   = regexp.MustCompile(`^pytestmark\s*(?::[^=]*)?=`)
	pytestIntegration  = regexp.MustCompile(`\bmark\s*\.\s*integration\b`)
	pytestUseFixtures  = regexp.MustCompile(`\busefixtures\s*\(([^)]*)\)`)
	pytestQuotedName   = regexp.MustCompile(`["']([A-Za-z_][A-Za-z0-9_]*)["']`)
	pytestPlaywrightRe = regexp.MustCompile(`(?i)pytest[-_]playwright`)
)

// pytestFunction is one test function's signature and the marker text that
// applies to it: its own decorators, its classes' decorators and
// pytestmark assignments, and the module's pytestmark.
type pytestFunction struct {
	params []string
	marks  string
}

// pytestFunctions scans Python source by indentation for the functions a
// pytest node ID can name, keyed by their `::`-joined class path and name.
// It reads decorators, class bodies and pytestmark assignments; it does not
// evaluate code, so a fixture applied by a conftest, autouse or an
// inherited method is not seen.
func pytestFunctions(source string) map[string]pytestFunction {
	type class struct {
		indent int
		name   string
		marks  string
	}
	lines := strings.Split(source, "\n")
	functions := map[string]pytestFunction{}
	var classes []class
	var pending []string
	module := ""
	// statement joins line index and the lines after it until the
	// brackets it opens are closed, at most 64 lines.
	statement := func(index int) (string, int) {
		text, depth := "", 0
		for at := index; at < len(lines) && at < index+64; at++ {
			text += " " + strings.TrimSpace(lines[at])
			depth += bracketDepth(lines[at])
			if depth <= 0 {
				return text, at
			}
		}
		return text, index
	}
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		for len(classes) != 0 && classes[len(classes)-1].indent >= indent {
			classes = classes[:len(classes)-1]
		}
		switch {
		case strings.HasPrefix(trimmed, "@"):
			text, end := statement(index)
			pending, index = append(pending, text), end
		case pytestClass.MatchString(trimmed):
			name := pytestClass.FindStringSubmatch(trimmed)[1]
			marks := strings.Join(pending, " ")
			if len(classes) != 0 {
				marks += " " + classes[len(classes)-1].marks
			}
			classes, pending = append(classes, class{indent: indent, name: name, marks: marks}), nil
		case pytestDef.MatchString(trimmed):
			name := pytestDef.FindStringSubmatch(trimmed)[1]
			text, end := statement(index)
			var names []string
			marks := strings.Join(pending, " ") + " " + module
			if len(classes) != 0 {
				marks += " " + classes[len(classes)-1].marks
			}
			for _, owner := range classes {
				names = append(names, owner.name)
			}
			key := strings.Join(append(names, name), "::")
			if _, seen := functions[key]; !seen {
				functions[key] = pytestFunction{params: pytestParams(text), marks: marks}
			}
			pending, index = nil, end
		case pytestMarkAssign.MatchString(trimmed):
			text, end := statement(index)
			if len(classes) != 0 {
				classes[len(classes)-1].marks += " " + text
			} else if indent == 0 {
				module += " " + text
			}
			pending, index = nil, end
		default:
			pending = nil
		}
	}
	return functions
}

// bracketDepth is the bracket depth change of one line, outside string
// literals and comments.
func bracketDepth(line string) int {
	depth := 0
	var quote rune
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			escaped = false
		case quote != 0 && r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '#':
			return depth
		case r == '(' || r == '[' || r == '{':
			depth++
		case r == ')' || r == ']' || r == '}':
			depth--
		}
	}
	return depth
}

// pytestParams is the parameter names of a def statement: the words before
// any annotation or default at bracket depth one, without `*` and `/`.
func pytestParams(def string) []string {
	open := strings.Index(def, "(")
	if open < 0 {
		return nil
	}
	var params []string
	depth, start := 0, open+1
	for at := open; at < len(def); at++ {
		switch def[at] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				params = appendParam(params, def[start:at])
				return params
			}
		case ',':
			if depth == 1 {
				params = appendParam(params, def[start:at])
				start = at + 1
			}
		}
	}
	return params
}

func appendParam(params []string, text string) []string {
	name := strings.TrimSpace(text)
	if cut := strings.IndexAny(name, ":="); cut >= 0 {
		name = strings.TrimSpace(name[:cut])
	}
	name = strings.TrimLeft(name, "*")
	if name == "" || name == "/" {
		return params
	}
	return append(params, name)
}

// pytestKind labels one listed test (PYR-V1-005): e2e when its function
// takes, or its markers apply through usefixtures, a pytest-playwright
// fixture and the plugin is in use; integration when the `integration`
// marker applies; UNKNOWN on both, or when its function is not in the file;
// unit otherwise.
func pytestKind(id testid.ID, functions map[string]pytestFunction, playwright bool) (string, string) {
	if len(id.Segments) == 0 {
		return KindUnknown, KindSourceUnresolved
	}
	segments := append([]string{}, id.Segments...)
	last := segments[len(segments)-1]
	if open := strings.Index(last, "["); open >= 0 {
		segments[len(segments)-1] = last[:open]
	}
	function, ok := functions[strings.Join(segments, "::")]
	if !ok {
		return KindUnknown, KindSourceUnresolved
	}
	e2e := false
	if playwright {
		fixtures := append([]string{}, function.params...)
		for _, use := range pytestUseFixtures.FindAllStringSubmatch(function.marks, -1) {
			for _, name := range pytestQuotedName.FindAllStringSubmatch(use[1], -1) {
				fixtures = append(fixtures, name[1])
			}
		}
		for _, name := range fixtures {
			e2e = e2e || pytestPlaywrightFixtures[name]
		}
	}
	integration := pytestIntegration.MatchString(function.marks)
	switch {
	case e2e && integration:
		return KindUnknown, KindSourceConflict
	case e2e:
		return KindE2E, KindSourceRunner
	case integration:
		return KindIntegration, KindSourceRunner
	}
	return KindUnit, KindSourceDefault
}

// pytestPlaywrightManifests are the dependency manifests, in an instance
// root or the repository root, whose naming pytest-playwright shows the
// plugin is in use.
var pytestPlaywrightManifests = []string{"pyproject.toml", "requirements.txt", "requirements-dev.txt", "requirements-test.txt", "setup.cfg", "tox.ini", "Pipfile", "uv.lock", "poetry.lock", "Pipfile.lock", "pdm.lock"}

// pytestPlaywrightDeclared reports whether a dependency manifest of root or
// of the repository root names pytest-playwright.
func pytestPlaywrightDeclared(input listInput, root string) bool {
	for _, dir := range []string{root, "."} {
		for _, name := range pytestPlaywrightManifests {
			candidate := path.Join(dir, name)
			if _, ok := input.Files[candidate]; !ok {
				continue
			}
			if data, err := input.Tree.Read(candidate, 4<<20); err == nil && pytestPlaywrightRe.Match(data) {
				return true
			}
		}
	}
	return false
}

// labelPytest sets the kind of every test result names (PYR-V1-005),
// reading each test file once from the listed tree. A file that cannot be
// read leaves its tests UNKNOWN.
func labelPytest(input listInput, instance Declared, result *listing) {
	declared := pytestPlaywrightDeclared(input, instance.Root)
	scanned := map[string]map[string]pytestFunction{}
	mentions := map[string]bool{}
	result.kinds = make([][2]string, len(result.named))
	for index, named := range result.named {
		file := result.sources[index]
		functions, seen := scanned[file]
		if !seen {
			data, err := input.Tree.Read(file, maxPytestSource)
			if err == nil {
				functions = pytestFunctions(string(data))
				mentions[file] = strings.Contains(string(data), "playwright")
			}
			scanned[file] = functions
		}
		if functions == nil {
			result.kinds[index] = [2]string{KindUnknown, KindSourceUnresolved}
			continue
		}
		kind, source := pytestKind(named.ID, functions, declared || mentions[file])
		result.kinds[index] = [2]string{kind, source}
	}
}

// maxPytestSource bounds the bytes of one test file the kind scan reads.
const maxPytestSource = 4 << 20
