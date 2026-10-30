package testsource

import (
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// Runner discovery and configuration reading for `corvint tests unselected` (TSA-V0-141,
// TSA-V0-142). Only literal configuration is read; anything else makes the runner uncertain.

// Modelled runners and the framework each one runs.
const (
	RunnerJest       = "jest"
	RunnerVitest     = "vitest"
	RunnerPlaywright = "playwright"
	RunnerMocha      = "mocha"
)

// Runner configuration states.
const (
	RunnerRead      = "read"
	RunnerDefaults  = "defaults"
	RunnerUncertain = "uncertain"
)

// jestRoot stands for the repository root in the absolute path Jest (as <rootDir>) and Playwright
// match their patterns against.
const jestRoot = "/__corvint_root__"

// SelectionRunner is one configured runner: its configuration file ("" for package defaults), the
// directory whose files it may select, and the include and exclude patterns it applies.
type SelectionRunner struct {
	Runner    string `json:"runner"`
	Config    string `json:"config"`
	Directory string `json:"directory"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	// Line is the configuration line that triggered Reason, when the reading knows it (TSA-V0-648).
	Line    int      `json:"line,omitempty"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	// Projects are a Playwright config's projects, each selecting with its own directory and
	// patterns; a file is selected when any project selects it (TSA-V0-653).
	Projects []SelectionProject `json:"projects,omitempty"`

	units []SelectionRunner
	// includeRegex and excludeRegex mark, per pattern, a Playwright regular-expression literal
	// (TSA-V0-654); lines maps a pattern to its key's configuration line.
	includeRegex []bool
	excludeRegex []bool
	lines        map[string]int
	recursive    bool
	extensions   []string
	include      []*regexp.Regexp
	exclude      []*regexp.Regexp
	// hidden, per include and exclude pattern, matches a path with a hidden segment, or is nil
	// when the pattern's own matcher decides; only Mocha leaves hidden files out (glob dot:false
	// and its directory scan), segment by segment (TSA-V0-645).
	includeHidden []func(string) bool
	excludeHidden []func(string) bool
}

// SelectionProject is one Playwright project's selection: its directory and the include and
// exclude patterns it applies (TSA-V0-653).
type SelectionProject struct {
	Directory string   `json:"directory"`
	Include   []string `json:"include"`
	Exclude   []string `json:"exclude"`
}

var (
	runnerPackages = map[string]string{"jest": RunnerJest, "vitest": RunnerVitest, "@playwright/test": RunnerPlaywright, "mocha": RunnerMocha}
	// unmodelledRunnerPackages are runners whose selection this reading does not model.
	unmodelledRunnerPackages = map[string]bool{"cypress": true, "karma": true, "jasmine": true, "ava": true, "@web/test-runner": true,
		"@wdio/cli": true, "tap": true, "uvu": true, "@japa/runner": true, "@storybook/test-runner": true, "testcafe": true}
	unmodelledConfigStems = map[string]bool{"cypress.config": true, "karma.conf": true, "wdio.conf": true, "ava.config": true,
		"web-test-runner.config": true, "vitest.workspace": true, "vitest.projects": true, ".testcaferc": true}
	configExtensions = map[string]bool{".js": true, ".ts": true, ".mjs": true, ".cjs": true, ".mts": true, ".cts": true, ".json": true}
	runnerDefaults   = map[string][2][]string{
		RunnerJest:       {{"**/__tests__/**/*.?([mc])[jt]s?(x)", "**/?(*.)+(spec|test).?([mc])[jt]s?(x)"}, {"/node_modules/"}},
		RunnerVitest:     {{"**/*.{test,spec}.?(c|m)[jt]s?(x)"}, {"**/node_modules/**", "**/.git/**"}},
		RunnerPlaywright: {{"**/*.@(spec|test).?(c|m)[jt]s?(x)"}, {}},
		RunnerMocha:      {{"./test"}, {}},
	}
)

// isRunnerConfigPath reports a committed file the discovery reads: a package.json or a runner
// configuration file, outside node_modules.
func isRunnerConfigPath(name string) bool {
	if inNodeModules(name) {
		return false
	}
	base := path.Base(name)
	if base == "package.json" || strings.HasPrefix(base, ".mocharc.") {
		return true
	}
	stem, extension := strings.TrimSuffix(base, path.Ext(base)), path.Ext(base)
	if !configExtensions[extension] {
		return false
	}
	switch stem {
	case "jest.config", "vitest.config", "vite.config", "playwright.config":
		return true
	}
	return unmodelledConfigStems[stem]
}

func inNodeModules(name string) bool {
	return name == "node_modules" || strings.HasPrefix(name, "node_modules/") || strings.Contains(name, "/node_modules/")
}

// runnerSet is the discovery result: the modelled runners and the unmodelled-runner evidence.
type runnerSet struct {
	runners    []SelectionRunner
	unmodelled []string
}

// discoverRunners builds the runner set from the committed package.json and configuration files.
func discoverRunners(files map[string]string) runnerSet {
	set := runnerSet{}
	type key struct{ runner, dir string }
	configs := map[key][]string{}
	declared := map[key]bool{}
	workspaces := []string{}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		dir, base := path.Dir(name), path.Base(name)
		stem := strings.TrimSuffix(base, path.Ext(base))
		switch {
		case base == "package.json":
			var manifest map[string]json.RawMessage
			if json.Unmarshal([]byte(files[name]), &manifest) != nil {
				set.unmodelled = append(set.unmodelled, name)
				continue
			}
			for _, field := range []string{"dependencies", "devDependencies", "optionalDependencies"} {
				var deps map[string]json.RawMessage
				_ = json.Unmarshal(manifest[field], &deps)
				for dep := range deps {
					if runner, ok := runnerPackages[dep]; ok {
						declared[key{runner, dir}] = true
					}
					if unmodelledRunnerPackages[dep] {
						set.unmodelled = append(set.unmodelled, name+"#"+dep)
					}
				}
			}
			for field, runner := range map[string]string{"jest": RunnerJest, "mocha": RunnerMocha} {
				if _, ok := manifest[field]; ok {
					configs[key{runner, dir}] = append(configs[key{runner, dir}], name+"#"+field)
				}
			}
		case unmodelledConfigStems[stem]:
			set.unmodelled = append(set.unmodelled, name)
			if stem == "vitest.workspace" || stem == "vitest.projects" {
				workspaces = append(workspaces, dir)
			}
		case strings.HasPrefix(base, ".mocharc."):
			configs[key{RunnerMocha, dir}] = append(configs[key{RunnerMocha, dir}], name)
		case stem == "jest.config":
			configs[key{RunnerJest, dir}] = append(configs[key{RunnerJest, dir}], name)
		case stem == "vitest.config":
			configs[key{RunnerVitest, dir}] = append(configs[key{RunnerVitest, dir}], name)
		case stem == "playwright.config":
			configs[key{RunnerPlaywright, dir}] = append(configs[key{RunnerPlaywright, dir}], name)
		case stem == "vite.config":
			// A Vite config is Vitest's configuration only where no vitest.config exists beside it,
			// and only when it holds a test block or Vitest is declared there.
			if _, hasTest := configKeys(files[name], "test"); hasTest {
				configs[key{RunnerVitest, dir + "\x00vite"}] = append(configs[key{RunnerVitest, dir + "\x00vite"}], name)
			}
		}
	}
	for k, list := range configs {
		if dir, vite := strings.CutSuffix(k.dir, "\x00vite"); vite {
			if _, own := configs[key{RunnerVitest, dir}]; !own {
				configs[key{RunnerVitest, dir}] = list
			}
			delete(configs, k)
		}
	}
	for _, dir := range workspaces {
		declared[key{RunnerVitest, dir}] = true
	}
	for k := range declared {
		if _, ok := configs[k]; !ok {
			configs[k] = nil
		}
	}
	for k, list := range configs {
		r := readRunner(k.runner, k.dir, list, files)
		if r.Runner == RunnerVitest && len(workspaces) > 0 {
			// A Vitest workspace selects per project, which this reading does not model, so every
			// Vitest instance abstains, and one exists at each workspace file.
			r = r.uncertain("runner-config-project-selection")
		}
		set.runners = append(set.runners, r)
	}
	slices.SortFunc(set.runners, func(a, b SelectionRunner) int {
		return strings.Compare(a.Directory+"\x00"+a.Runner+"\x00"+a.Config, b.Directory+"\x00"+b.Runner+"\x00"+b.Config)
	})
	slices.Sort(set.unmodelled)
	return set
}

// readRunner reads one runner's configuration; no configuration file means the package defaults.
func readRunner(runner, dir string, configs []string, files map[string]string) SelectionRunner {
	r := SelectionRunner{Runner: runner, Directory: dir, Status: RunnerDefaults, Include: []string{}, Exclude: []string{}}
	defaults := runnerDefaults[runner]
	r.Include, r.Exclude = append(r.Include, defaults[0]...), append(r.Exclude, defaults[1]...)
	r.extensions = []string{"js", "cjs", "mjs"}
	if len(configs) > 1 {
		r.Config = strings.Join(configs, ",")
		return r.uncertain("runner-config-multiple")
	}
	if len(configs) == 1 {
		r.Config, r.Status = configs[0], RunnerRead
		values, reason, line := readConfigValues(runner, configs[0], files)
		if reason != "" {
			r = r.uncertain(reason)
			r.Line = line
			return r
		}
		if reason := r.apply(values); reason != "" {
			return r.uncertain(reason)
		}
	}
	if reason := r.compile(); reason != "" {
		return r.uncertain(reason)
	}
	return r
}

func (r SelectionRunner) uncertain(reason string) SelectionRunner {
	r.Status, r.Reason, r.include, r.exclude = RunnerUncertain, reason, nil, nil
	return r
}

// configValue is one literal configuration value: strings (a string is a one-element list) or a
// boolean. regex marks, per string, a regular-expression literal kept as its source with slashes
// and flags (TSA-V0-654); line is the key's line; projects holds each Playwright projects entry's
// own selection keys (TSA-V0-653).
type configValue struct {
	strings  []string
	regex    []bool
	boolean  *bool
	line     int
	projects []map[string]configValue
}

// selectionKeys names, per runner, the keys that change selection and are read, and the keys
// whose presence makes the runner uncertain.
var selectionKeys = map[string][2][]string{
	RunnerJest:       {{"testMatch", "testPathIgnorePatterns"}, {"testRegex", "roots", "rootDir", "projects", "testPathPattern"}},
	RunnerVitest:     {{"include", "exclude"}, {"projects", "workspace", "root", "dir"}},
	RunnerPlaywright: {{"testDir", "testMatch", "testIgnore"}, {}},
	RunnerMocha:      {{"spec", "ignore", "exclude", "extension", "recursive"}, {"config", "package"}},
}

// readConfigValues reads the selection keys of one configuration: a JSON file, a package.json
// member or a JS/TS module.
func readConfigValues(runner, config string, files map[string]string) (map[string]configValue, string, int) {
	name, member, inPackage := strings.Cut(config, "#")
	text := files[name]
	keys := selectionKeys[runner]
	if inPackage || path.Ext(name) == ".json" || strings.HasSuffix(name, ".jsonc") {
		var object map[string]json.RawMessage
		if json.Unmarshal([]byte(text), &object) != nil {
			return nil, "runner-config-unparsed", 0
		}
		if inPackage {
			var nested map[string]json.RawMessage
			if json.Unmarshal(object[member], &nested) != nil {
				return nil, "runner-config-unparsed", 0
			}
			object = nested
		}
		if runner == RunnerVitest {
			return nil, "runner-config-unparsed", 0
		}
		values := map[string]configValue{}
		for _, k := range keys[1] {
			if _, ok := object[k]; ok {
				return nil, "runner-config-key-unsupported", 0
			}
		}
		for _, k := range keys[0] {
			raw, ok := object[k]
			if !ok {
				continue
			}
			var one string
			var list []string
			var flag bool
			switch {
			case json.Unmarshal(raw, &one) == nil:
				values[k] = configValue{strings: []string{one}}
			case json.Unmarshal(raw, &list) == nil:
				values[k] = configValue{strings: list}
			case json.Unmarshal(raw, &flag) == nil:
				values[k] = configValue{boolean: &flag}
			default:
				return nil, "runner-config-value-dynamic", 0
			}
		}
		return values, "", 0
	}
	if !webExtensions[path.Ext(name)] {
		return nil, "runner-config-unparsed", 0
	}
	return readModuleConfig(runner, text)
}

// readModuleConfig reads the selection keys of a JS/TS configuration module with the TSA-V0-010
// lexer. A spread, a repeated or non-literal selection key, an unsupported key, a Vitest selection
// key outside one `test` object or a Playwright selection key inside `projects` makes it uncertain.
func readModuleConfig(runner, text string) (map[string]configValue, string, int) {
	s := parseSource("config", text)
	if s.uncertain {
		return nil, "runner-config-unparsed", s.uncertainLine
	}
	nested := s.projectNested(runner)
	for i := range s.tokens {
		if s.punct(i, ".") && s.punct(i+1, ".") && s.punct(i+2, ".") && !nested(i) {
			return nil, "runner-config-spread", s.tokens[i].Line
		}
		if s.ident(i, "mergeConfig") || s.ident(i, "defineWorkspace") {
			return nil, "runner-config-merged", s.tokens[i].Line
		}
	}
	if ok, line := s.literalExport(); !ok {
		return nil, "runner-config-export-dynamic", line
	}
	keys := selectionKeys[runner]
	for _, k := range keys[1] {
		if at, found := configKeysIn(s, k); found {
			return nil, "runner-config-key-unsupported", s.tokens[at[0]].Line
		}
	}
	if line, dynamic := s.selectionKeyDynamic(runner); dynamic {
		return nil, "runner-config-key-dynamic", line
	}
	var testObject [2]int
	if runner == RunnerVitest {
		at, found := configKeysIn(s, "test")
		if !found {
			return map[string]configValue{}, "", 0
		}
		if len(at) != 1 || !s.punct(at[0]+2, "{") || s.match[at[0]+2] < 0 {
			return nil, "runner-config-value-dynamic", s.tokens[at[0]].Line
		}
		testObject = [2]int{at[0] + 2, s.match[at[0]+2]}
	}
	var projects [2]int
	var entries [][2]int
	if runner == RunnerPlaywright {
		if at, found := configKeysIn(s, "projects"); found {
			if len(at) != 1 || !s.punct(at[0]+2, "[") || s.match[at[0]+2] < 0 {
				return nil, "runner-config-value-dynamic", s.tokens[at[0]].Line
			}
			projects = [2]int{at[0] + 2, s.match[at[0]+2]}
			var line int
			if entries, line = s.projectEntries(projects); entries == nil {
				return nil, "runner-config-project-selection", line
			}
		}
	}
	// read reads the selection keys that are direct members of object, or, for the whole module
	// (object[1] < 0), those outside the projects array.
	read := func(object [2]int) (map[string]configValue, string, int) {
		values := map[string]configValue{}
		for _, k := range keys[0] {
			at, _ := configKeysIn(s, k)
			kept := []int{}
			for _, i := range at {
				switch {
				case runner == RunnerVitest && !s.directMember(i, testObject):
					continue // include/exclude of another block, such as coverage
				case object[1] >= 0 && !s.directMember(i, object):
					continue
				case object[1] < 0 && runner == RunnerPlaywright && within(i, projects):
					if !slices.ContainsFunc(entries, func(entry [2]int) bool { return s.directMember(i, entry) }) {
						// A selection key nested inside a project entry, not one of its own keys.
						return nil, "runner-config-project-selection", s.tokens[i].Line
					}
					continue
				}
				kept = append(kept, i)
			}
			if len(kept) > 1 {
				return nil, "runner-config-key-repeated", s.tokens[kept[1]].Line
			}
			if len(kept) == 1 {
				value, ok := s.literalValue(kept[0] + 2)
				if !ok {
					return nil, "runner-config-value-dynamic", s.tokens[kept[0]].Line
				}
				value.line = s.tokens[kept[0]].Line
				values[k] = value
			}
		}
		return values, "", 0
	}
	values, reason, line := read([2]int{0, -1})
	if reason != "" || len(entries) == 0 {
		return values, reason, line
	}
	all := configValue{line: s.tokens[projects[0]].Line}
	for _, entry := range entries {
		project, reason, line := read(entry)
		if reason != "" {
			return nil, reason, line
		}
		all.projects = append(all.projects, project)
	}
	values["projects"] = all
	return values, "", 0
}

// projectNested reports a token nested inside a value of a Playwright project entry, such as the
// `...devices['Desktop Chrome']` spread of its `use` object, which cannot add a selection key to
// the entry (TSA-V0-653). A spread directly in an entry or anywhere else still abstains.
func (s *source) projectNested(runner string) func(int) bool {
	if runner != RunnerPlaywright {
		return func(int) bool { return false }
	}
	at, found := configKeysIn(s, "projects")
	if !found || len(at) != 1 || !s.punct(at[0]+2, "[") || s.match[at[0]+2] < 0 {
		return func(int) bool { return false }
	}
	entries, _ := s.projectEntries([2]int{at[0] + 2, s.match[at[0]+2]})
	return func(i int) bool {
		return slices.ContainsFunc(entries, func(entry [2]int) bool {
			return i > entry[0] && i < entry[1] && !s.directMember(i, entry)
		})
	}
}

// projectEntries returns the object literals of a Playwright projects array spanning projects
// (TSA-V0-653). An empty array or an entry that is not an object literal, such as a variable, a
// call or a conditional, returns nil and the line to name.
func (s *source) projectEntries(projects [2]int) ([][2]int, int) {
	entries := [][2]int{}
	for j := projects[0] + 1; j < projects[1]; j++ {
		if !s.punct(j, "{") || s.match[j] <= j || s.match[j] >= projects[1] {
			return nil, s.tokens[j].Line
		}
		entries = append(entries, [2]int{j, s.match[j]})
		j = s.match[j] + 1
		if j < projects[1] && !s.punct(j, ",") {
			return nil, s.tokens[j].Line
		}
	}
	if len(entries) == 0 {
		return nil, s.tokens[projects[0]].Line
	}
	return entries, 0
}

// literalExport reports that every whole export of a configuration module (`module.exports =`,
// `export default`) is an object literal, a `defineConfig({…})` call, or a name the module
// declares as one. Any other export, such as `require('./base')`, a factory call or a function,
// is not runner defaults, and a module with no whole export is not read either (TSA-V0-645). The
// line is the first export that is not literal (TSA-V0-655), or 0 when there is no export.
func (s *source) literalExport() (bool, int) {
	found := false
	for i := range s.tokens {
		value := -1
		switch {
		case s.ident(i, "module") && !s.memberAccess(i) && s.punct(i+1, ".") && s.ident(i+2, "exports") && s.punct(i+3, "=") && !s.punct(i+4, "=") && !s.punct(i+4, ">"):
			value = i + 4
		case s.ident(i, "export") && s.ident(i+1, "default"):
			value = i + 2
		default:
			continue
		}
		found = true
		if !s.literalConfigValue(value, true) {
			return false, s.tokens[i].Line
		}
	}
	return found, 0
}

// literalConfigValue reports an object literal or a `defineConfig({…})` call at token i, or, when
// named is true, a bare name the module declares with one (`const config: Config = {…}`).
func (s *source) literalConfigValue(i int, named bool) bool {
	switch {
	case s.punct(i, "{"):
		return true
	case s.ident(i, "defineConfig") && s.punct(i+1, "(") && s.punct(i+2, "{"):
		return true
	case !named || !s.ident(i, "") || s.punct(i+1, "(") || s.punct(i+1, "."):
		return false
	}
	name := s.tokens[i].Text
	for j := range s.tokens {
		if !(s.ident(j, "const") || s.ident(j, "let") || s.ident(j, "var")) || !s.ident(j+1, name) {
			continue
		}
		// Skip a type annotation up to the declaration's `=`.
		for k := j + 2; k < len(s.tokens) && k < j+64; k++ {
			if s.punct(k, ";") {
				break
			}
			if s.punct(k, "=") && !s.punct(k+1, "=") && !s.punct(k+1, ">") && !s.punct(k-1, "=") {
				return s.literalConfigValue(k+1, false)
			}
		}
	}
	return false
}

// configKeys lexes text and returns the token indexes of every `key:` property.
func configKeys(text, key string) ([]int, bool) {
	return configKeysIn(parseSource("config", text), key)
}

// configKeysIn returns the token indexes of every `key:` or `"key":` object property in s.
func configKeysIn(s *source, key string) ([]int, bool) {
	out := []int{}
	for i, t := range s.tokens {
		if t.Text != key || !(t.Kind == appmap.JSIdent || t.Kind == appmap.JSString && t.Literal) || s.memberAccess(i) || !s.punct(i+1, ":") {
			continue
		}
		if s.punct(i-1, "{") || s.punct(i-1, ",") {
			out = append(out, i)
		}
	}
	return out, len(out) > 0
}

// selectionKeyDynamic reports a selection key the literal reading cannot place: a computed
// property, or a selection key name (or Vitest `test`, Playwright `projects`) that is not a
// `key:` property, such as a shorthand `{ testMatch }`, `module.exports.testMatch = …` or a
// variable (TSA-V0-142), and the line of the first such token (TSA-V0-655).
func (s *source) selectionKeyDynamic(runner string) (int, bool) {
	keys := selectionKeys[runner]
	watched := append(append([]string{}, keys[0]...), keys[1]...)
	switch runner {
	case RunnerVitest:
		watched = append(watched, "test")
	case RunnerPlaywright:
		watched = append(watched, "projects")
	}
	for i, t := range s.tokens {
		if s.punct(i, "]") && s.punct(i+1, ":") && s.match[i] >= 0 && (s.punct(s.match[i]-1, "{") || s.punct(s.match[i]-1, ",")) {
			return s.tokens[s.match[i]].Line, true
		}
		if !slices.Contains(watched, t.Text) || !(t.Kind == appmap.JSIdent || t.Kind == appmap.JSString && t.Literal) {
			continue
		}
		property := (s.punct(i-1, "{") || s.punct(i-1, ",")) && s.punct(i+1, ":") && !s.memberAccess(i)
		if !property {
			return t.Line, true
		}
	}
	return 0, false
}

// directMember reports a property token directly inside the object literal spanning object.
func (s *source) directMember(i int, object [2]int) bool {
	if !(i > object[0] && i < object[1]) {
		return false
	}
	for j := object[0] + 1; j < i; j++ {
		if s.match[j] > j {
			if s.match[j] > i {
				return false
			}
			j = s.match[j]
		}
	}
	return true
}

// literalValue reads the value starting at token i: a literal string, an array of literal
// strings, or true/false, ending at a `,` or the closing `}`.
func (s *source) literalValue(i int) (configValue, bool) {
	end := i + 1
	value := configValue{}
	switch {
	case s.literalText(i):
		value.strings, value.regex = []string{s.tokens[i].Text}, []bool{false}
	case s.regexLiteral(i):
		value.strings, value.regex = []string{s.tokens[i].Source}, []bool{true}
	case s.ident(i, "true") || s.ident(i, "false"):
		flag := s.tokens[i].Text == "true"
		value.boolean = &flag
	case s.punct(i, "[") && s.match[i] > i:
		value.strings, value.regex = []string{}, []bool{}
		for j := i + 1; j < s.match[i]; j++ {
			switch {
			case s.literalText(j):
				value.strings, value.regex = append(value.strings, s.tokens[j].Text), append(value.regex, false)
			case s.regexLiteral(j):
				value.strings, value.regex = append(value.strings, s.tokens[j].Source), append(value.regex, true)
			default:
				return configValue{}, false
			}
			if j+1 < s.match[i] && !s.punct(j+1, ",") {
				return configValue{}, false
			}
			j++
		}
		end = s.match[i] + 1
	default:
		return configValue{}, false
	}
	return value, s.punct(end, ",") || s.punct(end, "}")
}

// regexLiteral reports a regular-expression literal token, whose Source keeps its slashes and flags.
func (s *source) regexLiteral(i int) bool {
	return i >= 0 && i < len(s.tokens) && s.tokens[i].Kind == appmap.JSString && !s.tokens[i].Literal && strings.HasPrefix(s.tokens[i].Source, "/")
}

func (s *source) literalText(i int) bool {
	return i >= 0 && i < len(s.tokens) && (s.tokens[i].Kind == appmap.JSString || s.tokens[i].Kind == appmap.JSTemplate) && s.tokens[i].Literal
}

// apply sets the runner's patterns from its literal configuration values.
func (r *SelectionRunner) apply(values map[string]configValue) string {
	list := func(k string) ([]string, bool, string) {
		v, ok := values[k]
		if !ok {
			return nil, false, ""
		}
		if v.boolean != nil || slices.Contains(v.regex, true) && r.Runner != RunnerPlaywright {
			r.Line = v.line
			return nil, true, "runner-config-value-dynamic"
		}
		return v.strings, true, ""
	}
	switch r.Runner {
	case RunnerJest:
		if v, ok, bad := list("testMatch"); bad != "" {
			return bad
		} else if ok {
			r.Include = v
		}
		if v, ok, bad := list("testPathIgnorePatterns"); bad != "" {
			return bad
		} else if ok {
			r.Exclude = v
		}
	case RunnerVitest:
		if v, ok, bad := list("include"); bad != "" {
			return bad
		} else if ok {
			r.Include = v
		}
		if v, ok, bad := list("exclude"); bad != "" {
			return bad
		} else if ok {
			r.Exclude = v
		}
	case RunnerPlaywright:
		base := r.Directory
		if reason := r.applyPlaywright(base, values); reason != "" {
			return reason
		}
		for _, project := range values["projects"].projects {
			unit := SelectionRunner{Runner: r.Runner, Config: r.Config, Directory: r.Directory, Status: r.Status, Include: r.Include, Exclude: r.Exclude,
				includeRegex: r.includeRegex, excludeRegex: r.excludeRegex, lines: r.lines}
			if reason := unit.applyPlaywright(base, project); reason != "" {
				r.Line = unit.Line
				return reason
			}
			r.units = append(r.units, unit)
			r.Projects = append(r.Projects, SelectionProject{Directory: unit.Directory, Include: unit.Include, Exclude: unit.Exclude})
		}
	case RunnerMocha:
		if v, ok, bad := list("spec"); bad != "" {
			return bad
		} else if ok {
			r.Include = v
		}
		ignore, ok, bad := list("ignore")
		if bad != "" {
			return bad
		}
		exclude, okExclude, bad := list("exclude")
		if bad != "" {
			return bad
		}
		if ok || okExclude {
			r.Exclude = append(append([]string{}, ignore...), exclude...)
		}
		if v, ok, bad := list("extension"); bad != "" {
			return bad
		} else if ok {
			r.extensions = v
		}
		if v, ok := values["recursive"]; ok {
			if v.boolean == nil {
				r.Line = v.line
				return "runner-config-value-dynamic"
			}
			r.recursive = *v.boolean
		}
	}
	return ""
}

// applyPlaywright sets a Playwright selection from the testDir, testMatch and testIgnore in values,
// a testDir resolved against base, the configuration's directory. A key values lacks keeps what r
// has, so a project inherits the configuration's top-level selection (TSA-V0-653).
func (r *SelectionRunner) applyPlaywright(base string, values map[string]configValue) string {
	if v, ok := values["testDir"]; ok {
		dir, valid := "", v.boolean == nil && len(v.strings) == 1 && !v.regex[0]
		if valid {
			dir, valid = joinRelative(base, v.strings[0])
		}
		if !valid {
			r.Line = v.line
			return "runner-config-value-dynamic"
		}
		r.Directory = dir
	}
	for _, key := range []string{"testMatch", "testIgnore"} {
		v, ok := values[key]
		if !ok {
			continue
		}
		if v.boolean != nil {
			r.Line = v.line
			return "runner-config-value-dynamic"
		}
		lines := map[string]int{}
		for pattern, line := range r.lines {
			lines[pattern] = line
		}
		for _, pattern := range v.strings {
			lines[pattern] = v.line
		}
		r.lines = lines
		if key == "testMatch" {
			r.Include, r.includeRegex = v.strings, v.regex
		} else {
			r.Exclude, r.excludeRegex = v.strings, v.regex
		}
	}
	return ""
}

// joinRelative joins a configuration-relative directory to dir, refusing absolute paths and
// paths that leave the repository.
func joinRelative(dir, relative string) (string, bool) {
	if relative == "" || strings.HasPrefix(relative, "/") || strings.ContainsAny(relative, "\\\x00*?[{") {
		return "", false
	}
	joined := path.Join(dir, relative)
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return "", false
	}
	return joined, true
}

// compile turns the runner's patterns into matchers over the path relative to its directory.
func (r *SelectionRunner) compile() string {
	for i := range r.units {
		if reason := r.units[i].compile(); reason != "" {
			return reason
		}
	}
	r.include, r.exclude, r.includeHidden, r.excludeHidden = nil, nil, nil, nil
	for i, pattern := range r.Include {
		if marked(r.includeRegex, i) {
			re, ok := typescript.PlaywrightRegexMatcher(pattern)
			if !ok {
				return "runner-pattern-unsupported"
			}
			r.include, r.includeHidden = append(r.include, re), append(r.includeHidden, nil)
			continue
		}
		re, ok := r.includeMatcher(pattern)
		if !ok {
			return "runner-pattern-unsupported"
		}
		hidden, ok := r.hiddenMatcher(pattern, true)
		if !ok {
			return "runner-pattern-unsupported"
		}
		r.include, r.includeHidden = append(r.include, re), append(r.includeHidden, hidden)
	}
	for i, pattern := range r.Exclude {
		if marked(r.excludeRegex, i) {
			re, ok := typescript.PlaywrightRegexMatcher(pattern)
			if !ok {
				return "runner-pattern-unsupported"
			}
			r.exclude, r.excludeHidden = append(r.exclude, re), append(r.excludeHidden, nil)
			continue
		}
		re, ok := r.excludeMatcher(pattern)
		if !ok {
			return "runner-pattern-unsupported"
		}
		hidden, ok := r.hiddenMatcher(pattern, false)
		if !ok {
			return "runner-pattern-unsupported"
		}
		r.exclude, r.excludeHidden = append(r.exclude, re), append(r.excludeHidden, hidden)
	}
	return ""
}

// hiddenMatcher returns how pattern treats a path with a hidden segment: nil for every runner
// but Mocha, whose glob (dot:false) matches a hidden segment only with a pattern segment that
// itself starts with a literal `.`, at that segment, so neither `*` nor `**` reaches into a
// hidden directory or file (TSA-V0-645).
func (r *SelectionRunner) hiddenMatcher(pattern string, include bool) (func(string) bool, bool) {
	if r.Runner != RunnerMocha {
		return nil, true
	}
	globs := []string{strings.TrimPrefix(pattern, "./")}
	if include {
		globs = r.mochaGlobs(pattern)
	}
	return dotFalseMatcher(globs)
}

// mochaGlobs are the globs one Mocha spec stands for. A spec without glob characters is an
// existing file, that path plus a configured extension, or a directory whose files with a
// configured extension it selects (recursively when set). Mocha checks the file first; the
// committed tree alone does not tell them apart here, so any of them selects.
func (r *SelectionRunner) mochaGlobs(pattern string) []string {
	pattern = strings.TrimPrefix(pattern, "./")
	if strings.ContainsAny(pattern, "*?[{") {
		return []string{pattern}
	}
	exts := ".{" + strings.Join(r.extensions, ",") + "}"
	if len(r.extensions) == 1 {
		exts = "." + r.extensions[0]
	}
	files := "/*"
	if r.recursive {
		files = "/**/*"
	}
	dir := strings.TrimSuffix(pattern, "/")
	return []string{pattern, pattern + exts, dir + files + exts}
}

// dotFalseMatcher compiles globs segment by segment under minimatch dot:false: `**` matches zero
// or more segments that do not start with `.`, and any other pattern segment matches a segment
// starting with `.` only when the pattern segment starts with a literal `.` (TSA-V0-645).
func dotFalseMatcher(globs []string) (func(string) bool, bool) {
	type segment struct {
		any    bool // a whole-segment `**`
		dot    bool // starts with a literal `.`
		regexp *regexp.Regexp
	}
	patterns := [][]segment{}
	for _, glob := range globs {
		expanded, ok := expandBraces(glob)
		if !ok {
			return nil, false
		}
		for _, one := range expanded {
			segments := []segment{}
			for _, part := range strings.Split(one, "/") {
				if part == "**" {
					segments = append(segments, segment{any: true})
					continue
				}
				re, ok := globRegexp(part)
				if !ok {
					return nil, false
				}
				segments = append(segments, segment{dot: strings.HasPrefix(part, "."), regexp: re})
			}
			patterns = append(patterns, segments)
		}
	}
	var match func(p []segment, names []string) bool
	match = func(p []segment, names []string) bool {
		if len(p) == 0 {
			return len(names) == 0
		}
		if p[0].any {
			for k := 0; ; k++ {
				if match(p[1:], names[k:]) {
					return true
				}
				if k == len(names) || strings.HasPrefix(names[k], ".") {
					return false
				}
			}
		}
		if len(names) == 0 || strings.HasPrefix(names[0], ".") && !p[0].dot || !p[0].regexp.MatchString(names[0]) {
			return false
		}
		return match(p[1:], names[1:])
	}
	return func(name string) bool {
		names := strings.Split(name, "/")
		for _, p := range patterns {
			if match(p, names) {
				return true
			}
		}
		return false
	}, true
}

func (r *SelectionRunner) includeMatcher(pattern string) (*regexp.Regexp, bool) {
	switch r.Runner {
	case RunnerJest:
		return globRegexp(strings.ReplaceAll(pattern, "<rootDir>", jestRoot))
	case RunnerPlaywright:
		return playwrightMatcher(pattern)
	case RunnerMocha:
		return globsRegexp(r.mochaGlobs(pattern), false)
	}
	return globRegexp(strings.TrimPrefix(pattern, "./"))
}

func (r *SelectionRunner) excludeMatcher(pattern string) (*regexp.Regexp, bool) {
	switch r.Runner {
	case RunnerJest:
		// testPathIgnorePatterns are regular expressions searched in the absolute path.
		re, err := regexp.Compile(strings.ReplaceAll(pattern, "<rootDir>", regexp.QuoteMeta(jestRoot)))
		return re, err == nil
	case RunnerPlaywright:
		return playwrightMatcher(pattern)
	}
	return globRegexp(strings.TrimPrefix(pattern, "./"))
}

// playwrightMatcher compiles a Playwright testMatch or testIgnore glob: matched case-insensitively
// against the absolute file path, with `**/` prepended unless it starts with `**/`. An absolute
// pattern is outside the reading.
func playwrightMatcher(pattern string) (*regexp.Regexp, bool) {
	if strings.HasPrefix(pattern, "/") {
		return nil, false
	}
	if !strings.HasPrefix(pattern, "**/") {
		pattern = "**/" + pattern
	}
	return globsRegexp([]string{pattern}, true)
}

// selectionSubject is the string a runner's patterns match for a path relative to its directory:
// Jest and Playwright match the absolute path, the others the relative one.
func (r *SelectionRunner) selectionSubject(relative string) string {
	switch r.Runner {
	case RunnerJest:
		return jestRoot + "/" + relative
	case RunnerPlaywright:
		return jestRoot + "/" + path.Join(r.Directory, relative)
	}
	return relative
}

// verdict is how one runner treats one path: selected, or the reason it is not with the
// excluding pattern. A Playwright runner with projects selects a path any project selects; when
// none does, a project whose pattern depends on the checkout directory makes the path
// ReasonPatternRootDependent, and otherwise the most specific reason is kept (TSA-V0-653).
func (r *SelectionRunner) verdict(name string) (bool, string, string) {
	if len(r.units) == 0 {
		return r.unitVerdict(name)
	}
	rank := map[string]int{ReasonPatternRootDependent: -1, ReasonExcluded: 0, ReasonSuffixMismatch: 1, ReasonOutsideInclude: 2}
	reason, pattern := "", ""
	for i := range r.units {
		selected, why, at := r.units[i].unitVerdict(name)
		if selected {
			return true, "", ""
		}
		if reason == "" || rank[why] < rank[reason] {
			reason, pattern = why, at
		}
	}
	return false, reason, pattern
}

// patternLine is the configuration line of a pattern of r or of one of its projects.
func (r *SelectionRunner) patternLine(pattern string) int {
	if line, ok := r.lines[pattern]; ok {
		return line
	}
	for _, unit := range r.units {
		if line, ok := unit.lines[pattern]; ok {
			return line
		}
	}
	return 0
}

func (r *SelectionRunner) unitVerdict(name string) (bool, string, string) {
	relative, inside := relativeTo(r.Directory, name)
	if !inside {
		return false, ReasonOutsideInclude, ""
	}
	hidden := hiddenPath(relative)
	excluded, dependent := r.excludedBy(name, hidden)
	if excluded != "" {
		return false, ReasonExcluded, excluded
	}
	// An exclude pattern the checkout decides matters only for a path an include selects.
	included, includeDependent := r.included(name, hidden)
	switch {
	case included && dependent == "":
		return true, "", ""
	case included:
		return false, ReasonPatternRootDependent, dependent
	case includeDependent != "":
		return false, ReasonPatternRootDependent, includeDependent
	}
	if hidden && r.Runner == RunnerMocha {
		return false, ReasonOutsideInclude, "" // Mocha leaves hidden files out
	}
	// The directory is in scope when a conventionally named sibling would be selected.
	dir, extension := path.Dir(relative), path.Ext(relative)
	for _, ext := range []string{extension, ".js", ".ts"} {
		for _, stem := range []string{"corvintprobe.test", "corvintprobe.spec", "corvintprobe"} {
			probe := path.Join(r.Directory, dir, stem+ext)
			if selected, _ := r.included(probe, false); selected {
				if excluded, _ := r.excludedBy(probe, false); excluded == "" {
					return false, ReasonSuffixMismatch, ""
				}
			}
		}
	}
	return false, ReasonOutsideInclude, ""
}

// matches reports whether pattern i of patterns, compiled as re, matches the repository path
// name. A Playwright regular expression is decided on `/<name>` only when no checkout directory
// can change its verdict; otherwise dependent is true (TSA-V0-654).
func (r *SelectionRunner) matches(re *regexp.Regexp, raw string, regex bool, hiddenMatch func(string) bool, name string, hidden bool) (match, dependent bool) {
	if regex {
		if typescript.PlaywrightRegexReachesRoot(raw, name) {
			return false, true
		}
		return typescript.PlaywrightRegexMatches(raw, name), false
	}
	relative, _ := relativeTo(r.Directory, name)
	subject := r.selectionSubject(relative)
	return re.MatchString(subject) && (!hidden || hiddenMatch == nil || hiddenMatch(subject)), false
}

// included reports whether an include pattern selects name, or else the first include pattern
// whose verdict depends on the checkout directory.
func (r *SelectionRunner) included(name string, hidden bool) (bool, string) {
	dependent := ""
	for i, re := range r.include {
		match, unknown := r.matches(re, r.Include[i], marked(r.includeRegex, i), r.includeHidden[i], name, hidden)
		if match {
			return true, ""
		}
		if unknown && dependent == "" {
			dependent = r.Include[i]
		}
	}
	return false, dependent
}

// excludedBy returns the exclude pattern that excludes name, or else the first exclude pattern
// whose verdict depends on the checkout directory.
func (r *SelectionRunner) excludedBy(name string, hidden bool) (string, string) {
	dependent := ""
	for i, re := range r.exclude {
		match, unknown := r.matches(re, r.Exclude[i], marked(r.excludeRegex, i), r.excludeHidden[i], name, hidden)
		if match {
			return r.Exclude[i], ""
		}
		if unknown && dependent == "" {
			dependent = r.Exclude[i]
		}
	}
	return "", dependent
}

// marked reports flags[i], false past its end.
func marked(flags []bool, i int) bool {
	return i < len(flags) && flags[i]
}

// hiddenPath reports a path with a segment starting with `.`.
func hiddenPath(relative string) bool {
	return strings.HasPrefix(relative, ".") || strings.Contains(relative, "/.")
}

// relativeTo returns name relative to dir ("." is the repository root).
func relativeTo(dir, name string) (string, bool) {
	if dir == "." || dir == "" {
		return name, true
	}
	relative, ok := strings.CutPrefix(name, dir+"/")
	return relative, ok
}
