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

// jestRoot stands for Jest's <rootDir> in the absolute path Jest matches testMatch and
// testPathIgnorePatterns against.
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
	for k := range declared {
		if _, ok := configs[k]; !ok {
			configs[k] = nil
		}
	}
	for k, list := range configs {
		set.runners = append(set.runners, readRunner(k.runner, k.dir, list, files))
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
	keys := selectionKeys[runner]
	for _, k := range keys[1] {
		if _, found := configKeysIn(s, k); found {
			return nil, "runner-config-key-unsupported"
		}
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
	r.include, r.exclude = nil, nil
	for _, pattern := range r.Include {
		re, ok := r.includeMatcher(pattern)
		if !ok {
			return "runner-pattern-unsupported"
		}
		r.include = append(r.include, re)
	}
	for _, pattern := range r.Exclude {
		re, ok := r.excludeMatcher(pattern)
		if !ok {
			return "runner-pattern-unsupported"
		}
		r.exclude = append(r.exclude, re)
	}
	return ""
}

func (r *SelectionRunner) includeMatcher(pattern string) (*regexp.Regexp, bool) {
	switch r.Runner {
	case RunnerJest:
		return globRegexp(strings.ReplaceAll(pattern, "<rootDir>", jestRoot))
	case RunnerPlaywright:
		if !strings.HasPrefix(pattern, "**/") && !strings.HasPrefix(pattern, "/") {
			pattern = "**/" + pattern
		}
		return globRegexp(pattern)
	case RunnerMocha:
		pattern = strings.TrimPrefix(pattern, "./")
		if !strings.ContainsAny(pattern, "*?[{") && !slices.Contains(r.extensions, strings.TrimPrefix(path.Ext(pattern), ".")) {
			// A directory spec selects its files with a configured extension, recursively when set.
			files := "/*"
			if r.recursive {
				files = "/**/*"
			}
			pattern = strings.TrimSuffix(pattern, "/") + files + ".{" + strings.Join(r.extensions, ",") + "}"
		}
		return globRegexp(pattern)
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
		if !strings.HasPrefix(pattern, "**/") && !strings.HasPrefix(pattern, "/") {
			pattern = "**/" + pattern
		}
	}
	return globRegexp(strings.TrimPrefix(pattern, "./"))
}

// selectionSubject is the string a runner's patterns match for a path relative to its directory.
func (r *SelectionRunner) selectionSubject(relative string) string {
	if r.Runner == RunnerJest {
		return jestRoot + "/" + relative
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
	subject := r.selectionSubject(relative)
	for i, re := range r.exclude {
		if re.MatchString(subject) {
			return false, ReasonExcluded, r.Exclude[i]
		}
	}
	if r.included(subject) {
		return true, "", ""
	}
	// The directory is in scope when a conventionally named sibling would be selected.
	dir, extension := path.Dir(relative), path.Ext(relative)
	for _, ext := range []string{extension, ".js", ".ts"} {
		for _, stem := range []string{"corvintprobe.test", "corvintprobe.spec", "corvintprobe"} {
			probe := path.Join(dir, stem+ext)
			if r.included(r.selectionSubject(probe)) && !r.excluded(r.selectionSubject(probe)) {
				return false, ReasonSuffixMismatch, ""
			}
		}
	}
	return false, ReasonOutsideInclude, ""
}

func (r *SelectionRunner) included(subject string) bool {
	for _, re := range r.include {
		if re.MatchString(subject) {
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

// relativeTo returns name relative to dir ("." is the repository root).
func relativeTo(dir, name string) (string, bool) {
	if dir == "." || dir == "" {
		return name, true
	}
	relative, ok := strings.CutPrefix(name, dir+"/")
	return relative, ok
}
