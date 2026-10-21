package testsource

import (
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
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
	Runner    string   `json:"runner"`
	Config    string   `json:"config"`
	Directory string   `json:"directory"`
	Status    string   `json:"status"`
	Reason    string   `json:"reason,omitempty"`
	Include   []string `json:"include"`
	Exclude   []string `json:"exclude"`

	recursive  bool
	extensions []string
	include    []*regexp.Regexp
	exclude    []*regexp.Regexp
	// hidden, per include and exclude pattern, matches a path with a hidden segment, or is nil
	// when the pattern's own matcher decides; only Mocha leaves hidden files out (glob dot:false
	// and its directory scan), segment by segment (TSA-V0-645).
	includeHidden []func(string) bool
	excludeHidden []func(string) bool
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
		values, reason := readConfigValues(runner, configs[0], files)
		if reason != "" {
			return r.uncertain(reason)
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
// boolean.
type configValue struct {
	strings []string
	boolean *bool
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
func readConfigValues(runner, config string, files map[string]string) (map[string]configValue, string) {
	name, member, inPackage := strings.Cut(config, "#")
	text := files[name]
	keys := selectionKeys[runner]
	if inPackage || path.Ext(name) == ".json" || strings.HasSuffix(name, ".jsonc") {
		var object map[string]json.RawMessage
		if json.Unmarshal([]byte(text), &object) != nil {
			return nil, "runner-config-unparsed"
		}
		if inPackage {
			var nested map[string]json.RawMessage
			if json.Unmarshal(object[member], &nested) != nil {
				return nil, "runner-config-unparsed"
			}
			object = nested
		}
		if runner == RunnerVitest {
			return nil, "runner-config-unparsed"
		}
		values := map[string]configValue{}
		for _, k := range keys[1] {
			if _, ok := object[k]; ok {
				return nil, "runner-config-key-unsupported"
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
				return nil, "runner-config-value-dynamic"
			}
		}
		return values, ""
	}
	if !webExtensions[path.Ext(name)] {
		return nil, "runner-config-unparsed"
	}
	return readModuleConfig(runner, text)
}

// readModuleConfig reads the selection keys of a JS/TS configuration module with the TSA-V0-010
// lexer. A spread, a repeated or non-literal selection key, an unsupported key, a Vitest selection
// key outside one `test` object or a Playwright selection key inside `projects` makes it uncertain.
func readModuleConfig(runner, text string) (map[string]configValue, string) {
	s := parseSource("config", text)
	if s.uncertain {
		return nil, "runner-config-unparsed"
	}
	for i := range s.tokens {
		if s.punct(i, ".") && s.punct(i+1, ".") && s.punct(i+2, ".") {
			return nil, "runner-config-spread"
		}
		if s.ident(i, "mergeConfig") || s.ident(i, "defineWorkspace") {
			return nil, "runner-config-merged"
		}
	}
	if !s.literalExport() {
		return nil, "runner-config-export-dynamic"
	}
	keys := selectionKeys[runner]
	for _, k := range keys[1] {
		if _, found := configKeysIn(s, k); found {
			return nil, "runner-config-key-unsupported"
		}
	}
	if s.selectionKeyDynamic(runner) {
		return nil, "runner-config-key-dynamic"
	}
	var testObject [2]int
	if runner == RunnerVitest {
		at, found := configKeysIn(s, "test")
		if !found {
			return map[string]configValue{}, ""
		}
		if len(at) != 1 || !s.punct(at[0]+2, "{") || s.match[at[0]+2] < 0 {
			return nil, "runner-config-value-dynamic"
		}
		testObject = [2]int{at[0] + 2, s.match[at[0]+2]}
	}
	var projects [2]int
	if runner == RunnerPlaywright {
		if at, found := configKeysIn(s, "projects"); found {
			if len(at) != 1 || !s.punct(at[0]+2, "[") || s.match[at[0]+2] < 0 {
				return nil, "runner-config-value-dynamic"
			}
			projects = [2]int{at[0] + 2, s.match[at[0]+2]}
		}
	}
	values := map[string]configValue{}
	for _, k := range keys[0] {
		at, _ := configKeysIn(s, k)
		kept := []int{}
		for _, i := range at {
			if runner == RunnerVitest && !s.directMember(i, testObject) {
				continue // include/exclude of another block, such as coverage
			}
			if runner == RunnerPlaywright && within(i, projects) {
				return nil, "runner-config-project-selection"
			}
			kept = append(kept, i)
		}
		if len(kept) > 1 {
			return nil, "runner-config-key-repeated"
		}
		if len(kept) == 1 {
			value, ok := s.literalValue(kept[0] + 2)
			if !ok {
				return nil, "runner-config-value-dynamic"
			}
			values[k] = value
		}
	}
	return values, ""
}

// literalExport reports that every whole export of a configuration module (`module.exports =`,
// `export default`) is an object literal, a `defineConfig({…})` call, or a name the module
// declares as one. Any other export, such as `require('./base')`, a factory call or a function,
// is not runner defaults, and a module with no whole export is not read either (TSA-V0-645).
func (s *source) literalExport() bool {
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
			return false
		}
	}
	return found
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
// variable (TSA-V0-142).
func (s *source) selectionKeyDynamic(runner string) bool {
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
			return true
		}
		if !slices.Contains(watched, t.Text) || !(t.Kind == appmap.JSIdent || t.Kind == appmap.JSString && t.Literal) {
			continue
		}
		property := (s.punct(i-1, "{") || s.punct(i-1, ",")) && s.punct(i+1, ":") && !s.memberAccess(i)
		if !property {
			return true
		}
	}
	return false
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
		value.strings = []string{s.tokens[i].Text}
	case s.ident(i, "true") || s.ident(i, "false"):
		flag := s.tokens[i].Text == "true"
		value.boolean = &flag
	case s.punct(i, "[") && s.match[i] > i:
		value.strings = []string{}
		for j := i + 1; j < s.match[i]; j++ {
			if !s.literalText(j) {
				return configValue{}, false
			}
			value.strings = append(value.strings, s.tokens[j].Text)
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
		if v.boolean != nil {
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
		if v, ok, bad := list("testDir"); bad != "" {
			return bad
		} else if ok {
			if len(v) != 1 {
				return "runner-config-value-dynamic"
			}
			dir, valid := joinRelative(r.Directory, v[0])
			if !valid {
				return "runner-config-value-dynamic"
			}
			r.Directory = dir
		}
		if v, ok, bad := list("testMatch"); bad != "" {
			return bad
		} else if ok {
			r.Include = v
		}
		if v, ok, bad := list("testIgnore"); bad != "" {
			return bad
		} else if ok {
			r.Exclude = v
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
				return "runner-config-value-dynamic"
			}
			r.recursive = *v.boolean
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
	r.include, r.exclude, r.includeHidden, r.excludeHidden = nil, nil, nil, nil
	for _, pattern := range r.Include {
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
	for _, pattern := range r.Exclude {
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
// excluding pattern.
func (r *SelectionRunner) verdict(name string) (bool, string, string) {
	relative, inside := relativeTo(r.Directory, name)
	if !inside {
		return false, ReasonOutsideInclude, ""
	}
	subject, hidden := r.selectionSubject(relative), hiddenPath(relative)
	for i, re := range r.exclude {
		if re.MatchString(subject) && (!hidden || r.excludeHidden[i] == nil || r.excludeHidden[i](subject)) {
			return false, ReasonExcluded, r.Exclude[i]
		}
	}
	if r.included(subject, hidden) {
		return true, "", ""
	}
	if hidden && r.Runner == RunnerMocha {
		return false, ReasonOutsideInclude, "" // Mocha leaves hidden files out
	}
	// The directory is in scope when a conventionally named sibling would be selected.
	dir, extension := path.Dir(relative), path.Ext(relative)
	for _, ext := range []string{extension, ".js", ".ts"} {
		for _, stem := range []string{"corvintprobe.test", "corvintprobe.spec", "corvintprobe"} {
			probe := path.Join(dir, stem+ext)
			if r.included(r.selectionSubject(probe), false) && !r.excluded(r.selectionSubject(probe)) {
				return false, ReasonSuffixMismatch, ""
			}
		}
	}
	return false, ReasonOutsideInclude, ""
}

func (r *SelectionRunner) included(subject string, hidden bool) bool {
	for i, re := range r.include {
		if re.MatchString(subject) && (!hidden || r.includeHidden[i] == nil || r.includeHidden[i](subject)) {
			return true
		}
	}
	return false
}

func (r *SelectionRunner) excluded(subject string) bool {
	for _, re := range r.exclude {
		if re.MatchString(subject) {
			return true
		}
	}
	return false
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
