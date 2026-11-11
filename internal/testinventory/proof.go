// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
)

// An executing runner's instance keeps the default test-file names only
// when the reader proves it from the tree (TIV-V1-005, TIV-V1-008). Every
// other instance is unproven: Declared.Pattern names why, and the verdict
// judges each file of the runner's languages under its root that the record
// does not bind as unassigned. The proof is deliberately narrow, so a config
// the reader cannot evaluate fails closed rather than open.

// packageJSON is the npm manifest a JavaScript instance may be declared by
// (a "jest" member, or a script that runs vitest) when its directory holds no
// config file of the runner.
const packageJSON = "package.json"

// patternMembers lists, per executing runner, the config members that set or
// may move a test-file pattern, or the root it applies under. A JavaScript
// config naming one as an identifier or string, anywhere, is unproven.
// Playwright testDir is not one: it only narrows where the default names
// apply, and a testDir outside the root fails the lister.
var patternMembers = map[string][]string{
	testid.Vitest:     {"include", "includeSource", "projects", "workspace", "typecheck", "dir", "root"},
	testid.Jest:       {"testMatch", "testRegex", "projects", "preset", "rootDir", "roots"},
	testid.Playwright: {"testMatch"},
}

// runnerModules lists, per JavaScript runner, the modules a provable config
// may import: the runner's own package. A type-only import is erased and is
// not checked.
var runnerModules = map[string][]string{
	testid.Vitest:     {"vitest", "vitest/config"},
	testid.Jest:       {"jest", "@jest/types"},
	testid.Playwright: {"@playwright/test"},
}

// identityHelpers lists, per JavaScript runner, the functions a provable
// config may call: the runner's identity helper, which returns its argument.
var identityHelpers = map[string][]string{
	testid.Vitest:     {"defineConfig"},
	testid.Playwright: {"defineConfig"},
}

// workspacePrefixes are the file name prefixes Vitest auto-discovers beside
// its config as a workspace or projects file, whose members set their own
// patterns.
var workspacePrefixes = []string{"vitest.workspace.", "vitest.projects."}

// maxCauses bounds the causes Pattern lists.
const maxCauses = 4

// proof collects why an instance is unproven, in the order found.
type proof struct {
	causes []string
	seen   map[string]bool
	docs   bool
	any    bool
}

func (p *proof) add(cause string) {
	if p.seen == nil {
		p.seen = map[string]bool{}
	}
	if !p.seen[cause] {
		p.seen[cause] = true
		p.causes = append(p.causes, cause)
	}
}

// pattern is Pattern's text: the first maxCauses causes joined by "; ".
func (p *proof) pattern() string {
	if len(p.causes) <= maxCauses {
		return strings.Join(p.causes, "; ")
	}
	return strings.Join(p.causes[:maxCauses], "; ") + fmt.Sprintf("; and %d more", len(p.causes)-maxCauses)
}

// extensions is the unproven instance's candidate extension list: nil for a
// proven or static instance, ["*"] when any file may be a test.
func (p *proof) extensions(runner string) []string {
	if len(p.causes) == 0 {
		return nil
	}
	if p.any {
		return []string{"*"}
	}
	if runner == testid.Pytest {
		if p.docs {
			return []string{".py", ".txt", ".rst", ".md"}
		}
		return []string{".py"}
	}
	return jsExtensions
}

// proveDefault judges whether instance keeps the runner's default test-file
// names. It returns the unproven causes (Pattern) and the candidate
// extensions, both empty when proven. Static runners and Go are not judged.
func proveDefault(runner, root, config string, files map[string]string, read func(name string, limit int) ([]byte, error)) (string, []string) {
	var p proof
	switch runner {
	case testid.Vitest, testid.Jest, testid.Playwright:
		proveJavaScript(&p, runner, root, config, files, read)
	case testid.Pytest:
		provePytest(&p, root, config, files, read)
	default:
		return "", nil
	}
	return p.pattern(), p.extensions(runner)
}

func proveJavaScript(p *proof, runner, root, config string, files map[string]string, read func(name string, limit int) ([]byte, error)) {
	if path.Base(config) == packageJSON {
		if runner == testid.Jest {
			p.add("declared by the package.json jest member, which the reader does not evaluate")
		} else {
			p.add("declared by a package.json script that runs vitest with no vitest config")
		}
		return
	}
	data, err := read(config, maxConfigProbe)
	switch {
	case err != nil:
		p.add("an unreadable config")
	case strings.HasSuffix(config, ".json"):
		proveJSONConfig(p, runner, data)
	default:
		proveJSConfig(p, runner, data)
	}
	if runner == testid.Vitest {
		for name := range files {
			if path.Dir(name) != root {
				continue
			}
			for _, prefix := range workspacePrefixes {
				if strings.HasPrefix(path.Base(name), prefix) {
					p.add("workspace file " + name)
				}
			}
		}
	}
	if runner == testid.Jest {
		manifest := path.Join(root, packageJSON)
		if _, ok := files[manifest]; ok {
			members, err := readManifest(manifest, read)
			switch {
			case err != nil:
				p.add("an unreadable " + manifest)
			case members.jest:
				p.add(manifest + " sets the jest member")
			}
		}
	}
}

// proveJSONConfig proves a jest.config.json: a JSON object naming no
// pattern member.
func proveJSONConfig(p *proof, runner string, data []byte) {
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		p.add("a config that is not a JSON object")
		return
	}
	for _, member := range patternMembers[runner] {
		if _, ok := object[member]; ok {
			p.add("sets " + member)
		}
	}
}

// proveJSConfig proves a JavaScript or TypeScript config from its tokens:
// no import but the runner's own package, no call but the identity helper,
// no spread, function, constructor, computed key or computed member, and no
// pattern member named as an identifier or string anywhere (comments
// aside). The one spread allowed is a Playwright device descriptor,
// `...devices['Name']` or `...devices.Name`.
func proveJSConfig(p *proof, runner string, data []byte) {
	tokens, cause := lexJS(data)
	if cause != "" {
		p.add(cause)
		return
	}
	members := map[string]bool{}
	for _, member := range patternMembers[runner] {
		members[member] = true
	}
	allowed := map[string]bool{}
	for _, module := range runnerModules[runner] {
		allowed[module] = true
	}
	helpers := map[string]bool{}
	for _, helper := range identityHelpers[runner] {
		helpers[helper] = true
	}
	at := func(index int) jsToken {
		if index < 0 || index >= len(tokens) {
			return jsToken{}
		}
		return tokens[index]
	}
	module := func(name string) {
		if !allowed[name] {
			p.add("imports " + name)
		}
	}
	typeOnly := false
	for index, token := range tokens {
		previous, next := at(index-1), at(index+1)
		if (token.kind == tokenIdent || token.kind == tokenString) && members[token.text] {
			p.add("sets " + token.text)
		}
		switch {
		case token.is(tokenIdent, "import"):
			switch {
			case next.is(tokenPunct, "("):
				if at(index+2).kind == tokenString && at(index+3).is(tokenPunct, ")") {
					module(at(index + 2).text)
				} else {
					p.add("a dynamic import")
				}
			case next.kind == tokenString:
				module(next.text)
			case next.is(tokenPunct, "."):
				p.add("import.meta")
			case next.is(tokenIdent, "type") && !at(index+2).is(tokenIdent, "from") && !at(index+2).is(tokenPunct, ","):
				typeOnly = true
			}
		case token.is(tokenIdent, "from") && next.kind == tokenString:
			if !typeOnly {
				module(next.text)
			}
			typeOnly = false
		case token.is(tokenIdent, "require") && next.is(tokenPunct, "("):
			if at(index+2).kind == tokenString && at(index+3).is(tokenPunct, ")") {
				module(at(index + 2).text)
			} else {
				p.add("a dynamic require")
			}
		case token.is(tokenIdent, "new"):
			p.add("a constructor call")
		case token.is(tokenIdent, "function"), token.is(tokenPunct, "=>"):
			p.add("a function")
		case token.is(tokenIdent, "class"):
			p.add("a class")
		case token.is(tokenPunct, "..."):
			device := runner == testid.Playwright && next.is(tokenIdent, "devices") &&
				((at(index+2).is(tokenPunct, "[") && at(index+3).kind == tokenString && at(index+4).is(tokenPunct, "]")) ||
					(at(index+2).is(tokenPunct, ".") && at(index+3).kind == tokenIdent))
			if !device {
				p.add("a spread")
			}
		case token.is(tokenPunct, "("):
			switch {
			case previous.kind == tokenIdent && notCallKeywords[previous.text]:
			case previous.is(tokenIdent, "import"), previous.is(tokenIdent, "require"):
			case previous.kind == tokenIdent:
				if !helpers[previous.text] || at(index-2).is(tokenPunct, ".") || at(index-2).is(tokenPunct, "?.") {
					p.add("calls " + previous.text)
				}
			case previous.is(tokenPunct, ")"), previous.is(tokenPunct, "]"), previous.is(tokenPunct, ">"), previous.is(tokenPunct, "?."), previous.kind == tokenTemplate:
				p.add("a call")
			}
		case token.kind == tokenTemplate && (previous.kind == tokenIdent || previous.is(tokenPunct, ")") || previous.is(tokenPunct, "]")) && !notCallKeywords[previous.text]:
			p.add("a tagged template")
		case token.is(tokenPunct, "["):
			literal := (next.kind == tokenString || next.kind == tokenNumber) && at(index+2).is(tokenPunct, "]")
			switch {
			case previous.is(tokenPunct, "{") || previous.is(tokenPunct, ","):
				// After "{" or "," a "[" opens an array element or, directly
				// inside an object literal, a computed key.
				if !literal && objectContext(tokens, index) {
					p.add("a computed key")
				}
			case previous.kind == tokenIdent && !notCallKeywords[previous.text], previous.is(tokenPunct, ")"), previous.is(tokenPunct, "]"), previous.is(tokenPunct, "?."):
				if !literal {
					p.add("a computed member")
				}
			}
		}
	}
}

// objectContext reports whether the "[" at index sits directly inside an
// object literal (a computed key) rather than an array.
func objectContext(tokens []jsToken, index int) bool {
	depth := 0
	for i := index - 1; i >= 0; i-- {
		switch {
		case tokens[i].is(tokenPunct, ")"), tokens[i].is(tokenPunct, "]"), tokens[i].is(tokenPunct, "}"):
			depth++
		case tokens[i].is(tokenPunct, "("), tokens[i].is(tokenPunct, "["), tokens[i].is(tokenPunct, "{"):
			if depth == 0 {
				return tokens[i].is(tokenPunct, "{")
			}
			depth--
		}
	}
	return false
}

// notCallKeywords are the identifiers a "(" or "[" may follow without
// making a call or a member access.
var notCallKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "typeof": true,
	"void": true, "delete": true, "await": true, "in": true, "of": true, "instanceof": true, "case": true,
	"do": true, "else": true, "yield": true, "satisfies": true, "as": true, "export": true, "default": true,
}

// The kinds of a JavaScript token.
const (
	tokenIdent = iota + 1
	tokenString
	tokenTemplate
	tokenNumber
	tokenRegexp
	tokenPunct
)

type jsToken struct {
	kind int
	text string
}

func (token jsToken) is(kind int, text string) bool { return token.kind == kind && token.text == text }

// lexJS splits a JavaScript or TypeScript config into tokens, dropping
// comments. It returns a cause instead when the source holds what the
// proof cannot read: an escape outside a regular expression (an escaped
// identifier or string could spell a member), a template expression, or an
// unterminated comment, string or template. A string token's text is its
// content.
func lexJS(src []byte) ([]jsToken, string) {
	var tokens []jsToken
	regexAllowed := func() bool {
		if len(tokens) == 0 {
			return true
		}
		last := tokens[len(tokens)-1]
		switch last.kind {
		case tokenPunct:
			return last.text != ")" && last.text != "]" && last.text != "}"
		case tokenIdent:
			return notCallKeywords[last.text]
		}
		return false
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := bytes.Index(src[i+2:], []byte("*/"))
			if end < 0 {
				return nil, "an unterminated comment"
			}
			i += end + 4
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < len(src) && src[j] != c {
				switch {
				case src[j] == '\\':
					return nil, "an escape in a string"
				case c != '`' && src[j] == '\n':
					return nil, "an unterminated string"
				case c == '`' && src[j] == '$' && j+1 < len(src) && src[j+1] == '{':
					return nil, "a template expression"
				}
				j++
			}
			if j >= len(src) {
				return nil, "an unterminated string"
			}
			kind := tokenString
			if c == '`' {
				kind = tokenTemplate
			}
			tokens = append(tokens, jsToken{kind: kind, text: string(src[i+1 : j])})
			i = j + 1
		case c == '/' && regexAllowed():
			j, class := i+1, false
			for j < len(src) && (class || src[j] != '/') {
				switch src[j] {
				case '\\':
					j++
				case '[':
					class = true
				case ']':
					class = false
				case '\n':
					return nil, "an unterminated regular expression"
				}
				j++
			}
			if j >= len(src) {
				return nil, "an unterminated regular expression"
			}
			j++
			for j < len(src) && identPart(src[j]) {
				j++
			}
			tokens = append(tokens, jsToken{kind: tokenRegexp, text: string(src[i:j])})
			i = j
		case c == '\\':
			return nil, "an escape outside a string"
		case identStart(c):
			j := i + 1
			for j < len(src) && identPart(src[j]) {
				j++
			}
			tokens = append(tokens, jsToken{kind: tokenIdent, text: string(src[i:j])})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(src) && (identPart(src[j]) || src[j] == '.') {
				j++
			}
			tokens = append(tokens, jsToken{kind: tokenNumber, text: string(src[i:j])})
			i = j
		case bytes.HasPrefix(src[i:], []byte("...")):
			tokens = append(tokens, jsToken{kind: tokenPunct, text: "..."})
			i += 3
		case bytes.HasPrefix(src[i:], []byte("=>")), bytes.HasPrefix(src[i:], []byte("?.")) && !(i+2 < len(src) && src[i+2] >= '0' && src[i+2] <= '9'):
			tokens = append(tokens, jsToken{kind: tokenPunct, text: string(src[i : i+2])})
			i += 2
		default:
			tokens = append(tokens, jsToken{kind: tokenPunct, text: string(c)})
			i++
		}
	}
	return tokens, ""
}

func identStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func identPart(c byte) bool { return identStart(c) || (c >= '0' && c <= '9') }

// manifestMembers is what a package.json says about the JavaScript
// runners: a "jest" member, and whether a script runs vitest.
type manifestMembers struct {
	jest   bool
	vitest bool
}

var vitestWord = regexp.MustCompile(`(^|[^A-Za-z0-9_@/-])vitest($|[^A-Za-z0-9_-])`)

func readManifest(name string, read func(name string, limit int) ([]byte, error)) (manifestMembers, error) {
	data, err := read(name, maxConfigProbe)
	if err != nil {
		return manifestMembers{}, err
	}
	var members map[string]any
	if err := json.Unmarshal(data, &members); err != nil {
		return manifestMembers{}, err
	}
	_, jest := members["jest"]
	result := manifestMembers{jest: jest}
	scripts, _ := members["scripts"].(map[string]any)
	for _, value := range scripts {
		if text, ok := value.(string); ok && vitestWord.MatchString(text) {
			result.vitest = true
		}
	}
	return result, nil
}

// pytestSafeOptions match the addopts options that cannot add a collected
// file: output, reporting, strictness, failure and import-mode options.
// Anything else, a positional path, -p, --doctest-modules or
// --doctest-glob included, leaves the instance unproven.
var pytestSafeOptions = regexp.MustCompile(`^(-[qvsxl]+|-r[A-Za-z]+|--(strict-markers|strict-config|strict|quiet|verbose|exitfirst|showlocals|no-header|no-summary|disable-warnings|disable-pytest-warnings|lf|ff|nf|sw|stepwise|last-failed|failed-first|new-first)|--(color|tb|durations|durations-min|maxfail|import-mode|capture|show-capture|log-level|log-cli-level|code-highlight|verbosity)=[A-Za-z0-9_.-]+)$`)

// pytestKey matches an unindented `key =` or `key:` line of a pytest
// section.
var pytestKey = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_.-]*)\s*[=:]\s*(.*)$`)

// conftestHooks match a conftest.py hook or variable that may collect a file
// under any name: a collection hook or a plugin load.
var conftestHooks = regexp.MustCompile(`\b(pytest_collect_file|pytest_pycollect_makemodule|pytest_collect_directory|pytest_ignore_collect|pytest_plugins)\b`)

// provePytest proves a pytest instance: its section sets no python_files
// and no testpaths, its addopts hold only pytestSafeOptions, and no
// conftest.py under the root defines a collection hook or loads a plugin.
// Doctest files (.txt, .rst, .md) are candidates when a doctest option
// appears or cannot be ruled out; any file is one when a conftest.py hook
// may collect it.
func provePytest(p *proof, root, config string, files map[string]string, read func(name string, limit int) ([]byte, error)) {
	data, err := read(config, maxConfigProbe)
	if err != nil {
		p.add("an unreadable config")
		p.docs = true
	} else {
		section := "[pytest]"
		if shared, ok := pytestSections[path.Base(config)]; ok {
			section = shared
		}
		var key string
		inside := path.Base(config) == "pytest.ini" && !hasSectionLine(data, section)
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimRight(raw, "\r")
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && line == trimmed {
				inside, key = trimmed == section, ""
				continue
			}
			if !inside || trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
				continue
			}
			value := trimmed
			if match := pytestKey.FindStringSubmatch(line); match != nil {
				key, value = match[1], match[2]
				switch key {
				case "python_files", "testpaths":
					p.add("sets " + key)
				}
			}
			if key == "addopts" {
				provePytestOptions(p, value)
			}
		}
	}
	for name := range files {
		if path.Base(name) != "conftest.py" || !under(root, name) {
			continue
		}
		source, err := read(name, maxConfigProbe)
		switch {
		case err != nil:
			p.add("an unreadable " + name)
			p.any = true
		case conftestHooks.Match(source):
			p.add(name + " defines " + string(conftestHooks.Find(source)))
			p.any = true
		}
	}
}

func provePytestOptions(p *proof, value string) {
	value = strings.NewReplacer("[", " ", "]", " ", ",", " ", `"`, " ", "'", " ").Replace(value)
	for _, option := range strings.Fields(value) {
		if pytestSafeOptions.MatchString(option) {
			continue
		}
		p.add("addopts " + option)
		p.docs = true
	}
}
