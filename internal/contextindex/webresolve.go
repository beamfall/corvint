package contextindex

import (
	"cmp"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

// This file extends reverse-import rule (c) past the oracle's relative and
// profile-alias specifiers (GPK-V0-077 to GPK-V0-081, proposed; V1-0958, GitHub
// issue 659). A bare specifier such as `lib/pages/Login` is resolved through
// the nearest tsconfig.json or jsconfig.json -- `compilerOptions.baseUrl`,
// `paths` with one `*` wildcard, and `extends` chains -- using TypeScript's
// file, extension and directory-index rules. A bare specifier that neither
// resolves to a repository file nor names a builtin, a URL-scheme module or a
// declared package is unresolved, and the impact receipt says so instead of
// reporting a complete answer.
//
// Everything here reads the immutable index: configs and manifests are indexed
// sources and existence is a map lookup, so no query walks the filesystem.
// Each config is parsed once per resolver, each (config, specifier) pair is
// resolved once, and every bound below turns an oversized or unbounded input
// into an unknown, never a guess.

const (
	// maxWebConfigBytes bounds one tsconfig/jsconfig/package.json read.
	maxWebConfigBytes = 1 << 20
	// maxWebConfigDepth bounds one extends chain; a longer chain is unknown.
	maxWebConfigDepth = 16
	// maxWebConfigVisits bounds the configs one leaf's extends graph reads,
	// so array extends sharing ancestors cannot grow the walk exponentially.
	maxWebConfigVisits = 64
)

// WebImportState classifies one resolved specifier.
type WebImportState string

const (
	// WebImportRepository names a specifier that resolved to a tracked path.
	WebImportRepository WebImportState = "repository"
	// WebImportPackage names a builtin, URL-scheme or declared package module.
	WebImportPackage WebImportState = "package"
	// WebImportUnresolved is UNKNOWN: the specifier may name any repository
	// file, so an answer that depends on it is incomplete.
	WebImportUnresolved WebImportState = "unresolved"
)

// WebImportResolution is one specifier's resolution from one importer.
type WebImportResolution struct {
	// Target is the tracked path, set only for WebImportRepository.
	Target string
	State  WebImportState
}

// WebImportResolver resolves JavaScript and TypeScript module specifiers over
// one immutable index. It is not safe for concurrent use; build one per
// goroutine. It is the stable API the import graph is read through.
type WebImportResolver struct {
	index    *Index
	profile  projectprofile.Profile
	configs  map[string]*webConfig      // config path -> parsed file, nil when unreadable
	options  map[string]*webOptions     // leaf config path -> effective options
	governs  map[string]string          // directory -> nearest config path, "" for none
	aliases  map[string]webAliasOutcome // leaf config + NUL + specifier -> outcome
	manifest map[string]string          // directory -> nearest package.json directory, "" for none
	declared map[string]map[string]bool // package.json directory -> declared names with ancestors'
	names    map[string]bool            // nested workspace package names
	ownNames map[string]map[string]bool // package.json path -> own declared names, nil when unreadable
}

// NewWebImportResolver prepares a resolver; all work is lazy and memoized.
func NewWebImportResolver(index *Index) *WebImportResolver {
	return &WebImportResolver{
		index:    index,
		profile:  projectprofile.ByID(index.ProfileID),
		configs:  map[string]*webConfig{},
		options:  map[string]*webOptions{},
		governs:  map[string]string{},
		aliases:  map[string]webAliasOutcome{},
		manifest: map[string]string{},
		declared: map[string]map[string]bool{},
		ownNames: map[string]map[string]bool{},
	}
}

// Resolve classifies specifier as named by importer. Relative specifiers
// resolve against the importer's directory with the same file, extension and
// index rules as aliases, under the governing config's module resolution; a
// missing relative target is unresolved.
func (resolver *WebImportResolver) Resolve(importer, specifier string) WebImportResolution {
	if strings.HasPrefix(specifier, ".") {
		return resolvedOrUnknown(resolver.loadFrom(importer, webJoin(path.Dir(importer), specifier)).target)
	}
	if prefix := resolver.profile.WebAliasPrefix; prefix != "" && strings.HasPrefix(specifier, prefix) {
		return resolvedOrUnknown(resolver.loadFrom(importer, webJoin(resolver.profile.WebAliasRoot, specifier[len(prefix):])).target)
	}
	return resolver.resolveBare(importer, specifier)
}

// loadFrom loads one candidate path under the module resolution of the
// config governing importer, or TypeScript's default (node10) without one.
func (resolver *WebImportResolver) loadFrom(importer, candidate string) webAliasOutcome {
	mode := webModeNode10
	if config := resolver.governing(path.Dir(importer)); config != "" {
		mode = resolver.effective(config).mode
	}
	return inMode(mode, func(pass webPass) webAliasOutcome {
		target, inferred, stop := resolver.load(candidate, pass, mode != webModeClassic)
		return webAliasOutcome{target: target, inferred: inferred, unknown: stop}
	})
}

// webJoin joins a relative path onto a directory and keeps a trailing slash,
// which TypeScript reads as naming a directory only.
func webJoin(directory, relative string) string {
	joined := path.Join(directory, relative)
	if strings.HasSuffix(relative, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return joined
}

func resolvedOrUnknown(target string) WebImportResolution {
	if target == "" {
		return WebImportResolution{State: WebImportUnresolved}
	}
	return WebImportResolution{Target: target, State: WebImportRepository}
}

// resolveBare follows TypeScript's order for a non-relative name: `paths`,
// then `baseUrl`, then node_modules. The node_modules step is replaced by the
// declared-package test, because dependency trees are not indexed.
func (resolver *WebImportResolver) resolveBare(importer, specifier string) WebImportResolution {
	if webSchemeSpecifier(specifier) || strings.HasPrefix(specifier, "/") {
		if strings.HasPrefix(specifier, "/") {
			return WebImportResolution{State: WebImportUnresolved}
		}
		return WebImportResolution{State: WebImportPackage}
	}
	config := resolver.governing(path.Dir(importer))
	if config != "" {
		outcome, cached := resolver.aliases[config+"\x00"+specifier]
		if !cached {
			outcome = resolver.alias(config, specifier)
			resolver.aliases[config+"\x00"+specifier] = outcome
		}
		if outcome.target != "" && outcome.untyped && resolver.declaredPackage(importer, specifier) {
			// node10 tries node_modules with TypeScript and declaration
			// files before it tries JavaScript anywhere, so a declared
			// package's types (not indexed) may win over this file.
			return WebImportResolution{State: WebImportUnresolved}
		}
		if outcome.target != "" {
			return WebImportResolution{Target: outcome.target, State: WebImportRepository}
		}
		if outcome.unknown {
			return WebImportResolution{State: WebImportUnresolved}
		}
	}
	if resolver.declaredPackage(importer, specifier) {
		return WebImportResolution{State: WebImportPackage}
	}
	return WebImportResolution{State: WebImportUnresolved}
}

// webAliasOutcome is one (config, specifier) answer: a target, an explicit
// unknown (an unknown config, an ambiguous pattern, a substitution outside the
// repository or a package directory), or neither, when the config claims no
// file for the name and the package test decides.
type webAliasOutcome struct {
	target  string
	unknown bool
	// inferred marks a target reached by an added extension or a directory
	// index, which TypeScript's ESM mode does not do.
	inferred bool
	// untyped marks a target only node10's JavaScript pass reached.
	untyped bool
}

// webMode is the effective moduleResolution, read or derived from `module`
// and `target` as TypeScript does.
type webMode uint8

const (
	webModeUnknown webMode = iota
	webModeNode10
	webModeBundler
	webModeNode16
	webModeClassic
)

// webPass selects the file forms one resolution pass may pick. node10 and
// classic run the whole lookup with TypeScript and declaration files first
// and then again with JavaScript; bundler, node16 and nodenext run it once
// with every form.
type webPass uint8

const (
	webPassTyped webPass = 1 << iota
	webPassUntyped
	webPassAll = webPassTyped | webPassUntyped
)

func (pass webPass) admits(name string) bool {
	typed := false
	for _, extension := range []string{".ts", ".tsx", ".mts", ".cts"} {
		typed = typed || strings.HasSuffix(name, extension)
	}
	return typed && pass&webPassTyped != 0 || !typed && pass&webPassUntyped != 0
}

// inMode runs one lookup in mode's pass order. Under node16 and nodenext a
// file is in ESM or CommonJS mode by its own extension and package.json,
// which are not read here, and only CommonJS mode adds extensions or reads a
// directory index, so a target reached that way is unknown. An unknown mode
// is unknown.
func inMode(mode webMode, resolve func(webPass) webAliasOutcome) webAliasOutcome {
	switch mode {
	case webModeNode10, webModeClassic:
		if outcome := resolve(webPassTyped); outcome.target != "" || outcome.unknown {
			return outcome
		}
		outcome := resolve(webPassUntyped)
		outcome.untyped = outcome.target != ""
		return outcome
	case webModeBundler:
		return resolve(webPassAll)
	case webModeNode16:
		if outcome := resolve(webPassAll); !outcome.inferred {
			return outcome
		}
	}
	return webAliasOutcome{unknown: true}
}

func (resolver *WebImportResolver) alias(config, specifier string) webAliasOutcome {
	options := resolver.effective(config)
	if !options.pathsKnown || !options.baseKnown || options.mode == webModeClassic {
		// An unknown inherited baseUrl or paths may claim any name, even a
		// declared package's, so no answer is safe. Classic resolution also
		// looks for the name in every ancestor directory, which is not
		// modelled.
		return webAliasOutcome{unknown: true}
	}
	return inMode(options.mode, func(pass webPass) webAliasOutcome {
		return resolver.aliasIn(options, specifier, pass)
	})
}

func (resolver *WebImportResolver) aliasIn(options *webOptions, specifier string, pass webPass) webAliasOutcome {
	if options.paths != nil {
		pattern, capture, matched, ambiguous := options.match(specifier)
		if ambiguous {
			return webAliasOutcome{unknown: true}
		}
		if matched {
			// A matched key ends the paths/baseUrl stage even when no
			// substitution resolves: TypeScript then goes straight to
			// node_modules, so baseUrl is not tried.
			for _, substitution := range options.paths[pattern] {
				// An empty capture leaves the substitution as written, as in
				// TypeScript, so `alias/` never names the target directory.
				replaced := substitution
				if capture != "" {
					replaced = strings.Replace(substitution, "*", capture, 1)
				}
				candidate, inside := options.webSubstitutionPath(replaced)
				if !inside {
					// An absolute or escaping substitution may name a file
					// outside the repository that TypeScript would pick.
					return webAliasOutcome{unknown: true}
				}
				// A substitution naming a TypeScript extension is tried as
				// written before any replacement, in every pass.
				if webExtension(substitution) != "" && resolver.exists(candidate) {
					return webAliasOutcome{target: candidate}
				}
				target, inferred, stop := resolver.load(candidate, pass, true)
				if target != "" {
					return webAliasOutcome{target: target, inferred: inferred}
				}
				if stop {
					return webAliasOutcome{unknown: true}
				}
			}
			return webAliasOutcome{}
		}
	}
	if options.baseURL != "" {
		target, inferred, stop := resolver.load(webJoin(options.baseURL, specifier), pass, true)
		if target != "" {
			return webAliasOutcome{target: target, inferred: inferred}
		}
		if stop {
			return webAliasOutcome{unknown: true}
		}
	}
	return webAliasOutcome{}
}

// webExtensions are the extensions TypeScript strips from a candidate before
// replacing them, longest declaration forms first.
var webExtensions = []string{".d.ts", ".d.mts", ".d.cts", ".mjs", ".mts", ".cjs", ".cts", ".ts", ".js", ".tsx", ".jsx", ".json"}

func webExtension(name string) string {
	for _, extension := range webExtensions {
		if strings.HasSuffix(name, extension) {
			return extension
		}
	}
	return ""
}

// webReplacements is TypeScript's extension replacement (tryAddingExtensions):
// a `.js` specifier names the `.ts` source that compiles to it, and so on.
var webReplacements = map[string][]string{
	".ts": webImplicitExtensions, ".d.ts": webImplicitExtensions, ".js": webImplicitExtensions,
	".tsx": {".tsx", ".ts", ".d.ts", ".jsx", ".js"}, ".jsx": {".tsx", ".ts", ".d.ts", ".jsx", ".js"},
	".mjs": {".mts", ".d.mts", ".mjs"}, ".mts": {".mts", ".d.mts", ".mjs"}, ".d.mts": {".mts", ".d.mts", ".mjs"},
	".cjs": {".cts", ".d.cts", ".cjs"}, ".cts": {".cts", ".d.cts", ".cjs"}, ".d.cts": {".cts", ".d.cts", ".cjs"},
	".json": {".d.json.ts", ".json"},
}

// webImplicitExtensions is the order TypeScript tries for an extensionless
// candidate and for a directory index, with JavaScript allowed.
var webImplicitExtensions = []string{".ts", ".tsx", ".d.ts", ".js", ".jsx"}

// load resolves one candidate path to a tracked file: the file itself with an
// extension replaced or added, then the directory's index. Another named
// extension (a stylesheet, say) resolves exactly or through its `.d<ext>.ts`
// declaration, because a bundler resolves it. A candidate with a trailing
// slash names a directory only. index is false under classic resolution,
// which reads no directory. inferred reports an added extension or a
// directory index. stop reports a directory holding a package.json, whose
// entry point is not resolved here, so the caller reports unknown instead of
// trying a later candidate.
func (resolver *WebImportResolver) load(candidate string, pass webPass, index bool) (target string, inferred, stop bool) {
	directoryOnly := strings.HasSuffix(candidate, "/")
	candidate = path.Clean(candidate)
	if candidate == ".." || strings.HasPrefix(candidate, "../") || strings.HasPrefix(candidate, "/") {
		return "", false, false
	}
	directory := candidate + "/"
	if candidate == "." {
		// The repository root has no file form, only a directory index.
		directory = ""
	} else if !directoryOnly {
		if extension := webExtension(candidate); extension != "" {
			stem := strings.TrimSuffix(candidate, extension)
			for _, replacement := range webReplacements[extension] {
				if resolver.has(stem+replacement, pass) {
					return stem + replacement, false, false
				}
			}
		} else if extension := path.Ext(candidate); extension != "" {
			stem := strings.TrimSuffix(candidate, extension)
			for _, name := range []string{candidate, stem + ".d" + extension + ".ts"} {
				if resolver.has(name, pass) {
					return name, false, false
				}
			}
		}
		for _, added := range webImplicitExtensions {
			if resolver.has(candidate+added, pass) {
				return candidate + added, true, false
			}
		}
	}
	if !index {
		return "", false, false
	}
	if resolver.exists(directory + "package.json") {
		return "", false, true
	}
	for _, added := range webImplicitExtensions {
		if resolver.has(directory+"index"+added, pass) {
			return directory + "index" + added, true, false
		}
	}
	return "", false, false
}

// has reports a tracked file the pass may pick.
func (resolver *WebImportResolver) has(name string, pass webPass) bool {
	return pass.admits(name) && resolver.exists(name)
}

func (resolver *WebImportResolver) exists(name string) bool {
	if _, ok := resolver.index.Sources[name]; ok {
		return true
	}
	_, ok := resolver.index.Tracked[name]
	return ok
}

// governing finds the nearest directory at or above directory holding a
// tsconfig.json, else a jsconfig.json, which is how the TypeScript language
// service picks a file's project. `include`, `files` and project references
// are not evaluated.
func (resolver *WebImportResolver) governing(directory string) string {
	if config, ok := resolver.governs[directory]; ok {
		return config
	}
	config := ""
	for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
		candidate := path.Join(directory, name)
		if _, ok := resolver.index.Sources[candidate]; ok {
			config = candidate
			break
		}
		if _, ok := resolver.index.Tracked[candidate]; ok {
			config = candidate
			break
		}
	}
	if config == "" && directory != "." {
		config = resolver.governing(path.Dir(directory))
	}
	resolver.governs[directory] = config
	return config
}

// webConfig is one parsed config file. Each field records whether the file
// declares it (a JSON null declares it unset, which stops inheritance).
type webConfig struct {
	directory            string
	extends              []string
	extendsValid         bool
	baseURL              string
	baseURLSet           bool
	baseURLNull          bool
	paths                map[string][]string
	pathsSet, pathsValid bool
	moduleSuffixes       bool
	// settings holds moduleResolution, module and target, in that order.
	settings [3]webSetting
}

// webSetting is one string compiler option a config declares: set, with its
// lower-cased value ("" for JSON null), valid when the value is a string.
type webSetting struct {
	set, valid bool
	value      string
}

var webSettingNames = [3]string{"moduleResolution", "module", "target"}

// webOptions is a leaf config's effective resolution input after extends.
// A field is known only when a readable config declares it before any
// unreadable link in TypeScript's precedence order; an unknown field makes
// every name the config might claim unknown.
type webOptions struct {
	baseURL, pathsBase, leafDirectory string
	baseKnown, pathsKnown             bool
	paths                             map[string][]string
	mode                              webMode
}

func (resolver *WebImportResolver) parse(name string) *webConfig {
	if config, ok := resolver.configs[name]; ok {
		return config
	}
	config := parseWebConfig(resolver.index.Sources, name)
	resolver.configs[name] = config
	return config
}

func parseWebConfig(sources map[string]Source, name string) *webConfig {
	source, ok := sources[name]
	if !ok {
		return nil
	}
	text, valid, loaded := source.Text()
	if !valid || !loaded || len(text) > maxWebConfigBytes {
		return nil
	}
	clean, ok := jsoncToJSON(text)
	if !ok {
		return nil
	}
	var raw struct {
		Extends         json.RawMessage `json:"extends"`
		CompilerOptions *struct {
			BaseURL          json.RawMessage `json:"baseUrl"`
			Paths            json.RawMessage `json:"paths"`
			ModuleSuffixes   json.RawMessage `json:"moduleSuffixes"`
			ModuleResolution json.RawMessage `json:"moduleResolution"`
			Module           json.RawMessage `json:"module"`
			Target           json.RawMessage `json:"target"`
		} `json:"compilerOptions"`
	}
	if jsonv2.Unmarshal([]byte(clean), &raw) != nil {
		return nil
	}
	config := &webConfig{directory: path.Dir(name), extendsValid: true}
	switch extends := strings.TrimSpace(string(raw.Extends)); {
	case extends == "" || extends == "null":
	case strings.HasPrefix(extends, "["):
		config.extendsValid = jsonv2.Unmarshal(raw.Extends, &config.extends) == nil
	default:
		var single string
		config.extendsValid = jsonv2.Unmarshal(raw.Extends, &single) == nil
		config.extends = []string{single}
	}
	if raw.CompilerOptions == nil {
		return config
	}
	options := raw.CompilerOptions
	if len(options.BaseURL) != 0 {
		config.baseURLSet, config.baseURLNull = true, string(options.BaseURL) == "null"
		if !config.baseURLNull && jsonv2.Unmarshal(options.BaseURL, &config.baseURL) != nil {
			return nil
		}
	}
	if len(options.Paths) != 0 {
		config.pathsSet = true
		config.pathsValid = string(options.Paths) == "null" || jsonv2.Unmarshal(options.Paths, &config.paths) == nil && validWebPaths(config.paths)
	}
	if moduleSuffixes := strings.TrimSpace(string(options.ModuleSuffixes)); moduleSuffixes != "" && moduleSuffixes != "null" {
		var values []string
		config.moduleSuffixes = jsonv2.Unmarshal(options.ModuleSuffixes, &values) != nil || len(values) != 1 || values[0] != ""
	}
	for position, raw := range [3]json.RawMessage{options.ModuleResolution, options.Module, options.Target} {
		if len(raw) == 0 {
			continue
		}
		setting := webSetting{set: true, valid: true}
		if string(raw) != "null" {
			setting.valid = jsonv2.Unmarshal(raw, &setting.value) == nil && setting.value != ""
			setting.value = strings.ToLower(setting.value)
		}
		config.settings[position] = setting
	}
	return config
}

// webModeOf applies TypeScript 5.9's computed moduleResolution: the declared
// value, else one derived from `module`, else from `target` (CommonJS below
// ES2015, else ES2015, which resolves classically). An unknown or invalid
// input is an unknown mode.
func webModeOf(settings [3]webSetting, known [3]bool) webMode {
	for position := range settings {
		if !known[position] || settings[position].set && !settings[position].valid {
			return webModeUnknown
		}
		value := settings[position].value
		if value == "" {
			continue
		}
		switch position {
		case 0:
			switch value {
			case "node", "node10":
				return webModeNode10
			case "bundler":
				return webModeBundler
			case "node16", "nodenext":
				return webModeNode16
			case "classic":
				return webModeClassic
			}
		case 1:
			switch value {
			case "commonjs":
				return webModeNode10
			case "node16", "node18", "node20", "nodenext":
				return webModeNode16
			case "preserve":
				return webModeBundler
			case "none", "amd", "umd", "system", "es6", "es2015", "es2020", "es2022", "esnext":
				return webModeClassic
			}
		default:
			switch value {
			case "es3", "es5":
				return webModeNode10
			case "es6", "es2015", "es2016", "es2017", "es2018", "es2019", "es2020", "es2021", "es2022", "es2023", "es2024", "esnext":
				return webModeClassic
			}
		}
		return webModeUnknown
	}
	return webModeNode10
}

// validWebPaths applies TypeScript's own validity rule: at most one `*` in a
// pattern and in each substitution, and a non-empty substitution list.
func validWebPaths(paths map[string][]string) bool {
	for pattern, targets := range paths {
		if strings.Count(pattern, "*") > 1 || len(targets) == 0 {
			return false
		}
		for _, target := range targets {
			if strings.Count(target, "*") > 1 {
				return false
			}
		}
	}
	return true
}

// effective walks the extends graph child first, which is TypeScript's
// precedence: the leaf, then its last extends entry's whole chain, then the
// one before it, and the first declaration of a field wins.
//
// A link TypeScript itself rejects -- a cycle, an unreadable or missing
// relative config, more than maxWebConfigDepth links, or more than
// maxWebConfigVisits configs in all -- makes the whole config unknown, even a
// field the leaf declares, because the project does not compile as written. A
// package-named or absolute extends is legal but not in the index, so only the
// fields no config before it declares become unknown.
func (resolver *WebImportResolver) effective(leaf string) *webOptions {
	if options, ok := resolver.options[leaf]; ok {
		return options
	}
	options := &webOptions{baseKnown: true, pathsKnown: true, leafDirectory: path.Dir(leaf)}
	var (
		baseDecided, pathsDecided, broken bool
		pathsDirectory                    string
		visits                            int
		settings                          [3]webSetting
		settingDecided                    [3]bool
		settingKnown                      = [3]bool{true, true, true}
	)
	external := func() {
		if !baseDecided {
			baseDecided, options.baseKnown = true, false
		}
		if !pathsDecided {
			pathsDecided, options.pathsKnown = true, false
		}
		for position := range settingDecided {
			if !settingDecided[position] {
				settingDecided[position], settingKnown[position] = true, false
			}
		}
	}
	var walk func(name string, depth int, stack map[string]bool)
	walk = func(name string, depth int, stack map[string]bool) {
		visits++
		config := resolver.parse(name)
		if broken || config == nil || stack[name] || depth > maxWebConfigDepth || visits > maxWebConfigVisits || !config.extendsValid {
			broken = true
			return
		}
		if config.moduleSuffixes {
			// moduleSuffixes changes which file every candidate names.
			broken = true
			return
		}
		if config.baseURLSet && !baseDecided {
			baseDecided = true
			if !config.baseURLNull {
				// TypeScript reads an empty baseUrl as the declaring
				// config's directory.
				options.baseURL = webConfigPath(cmp.Or(config.baseURL, "."), config.directory, options.leafDirectory)
				options.baseKnown = options.baseURL != ""
			}
		}
		if config.pathsSet && !pathsDecided {
			pathsDecided = true
			options.pathsKnown = config.pathsValid
			options.paths, pathsDirectory = config.paths, config.directory
		}
		for position, setting := range config.settings {
			if setting.set && !settingDecided[position] {
				settingDecided[position], settings[position] = true, setting
			}
		}
		stack[name] = true
		for position := len(config.extends) - 1; position >= 0 && !broken; position-- {
			parent, state := resolver.extendsPath(config.extends[position], config.directory)
			switch state {
			case webExtendsIndexed:
				walk(parent, depth+1, stack)
			case webExtendsExternal:
				external()
			default:
				broken = true
			}
		}
		delete(stack, name)
	}
	walk(leaf, 0, map[string]bool{})
	options.mode = webModeOf(settings, settingKnown)
	if broken {
		options.baseKnown, options.pathsKnown, options.paths, options.mode = false, false, nil, webModeUnknown
	}
	options.pathsBase = options.baseURL
	if options.pathsBase == "" {
		options.pathsBase = pathsDirectory
	}
	resolver.options[leaf] = options
	return options
}

// webConfigPath resolves a config-relative directory value to a repository
// path, "" when it is absolute or leaves the repository.
func webConfigPath(value, directory, leafDirectory string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	if value == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(value, "${configDir}"); ok {
		directory, value = leafDirectory, "./"+strings.TrimPrefix(rest, "/")
	}
	if webRooted(value) {
		return ""
	}
	joined := path.Join(directory, value)
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return ""
	}
	return joined
}

// webSubstitutionPath places one `paths` substitution, with its `*` already
// replaced, in the repository: against the leaf config's directory for a
// `${configDir}` substitution, else against the paths base. inside is false
// for an absolute substitution or one that leaves the repository.
func (options *webOptions) webSubstitutionPath(substitution string) (candidate string, inside bool) {
	substitution = strings.ReplaceAll(substitution, "\\", "/")
	if rest, ok := strings.CutPrefix(substitution, "${configDir}"); ok {
		candidate = webJoin(options.leafDirectory, rest)
	} else if webRooted(substitution) {
		return "", false
	} else {
		candidate = webJoin(options.pathsBase, substitution)
	}
	return candidate, candidate != ".." && !strings.HasPrefix(candidate, "../")
}

type webExtendsState int

const (
	webExtendsIndexed webExtendsState = iota
	webExtendsExternal
	webExtendsMissing
)

// extendsPath resolves one extends entry to an indexed config. A relative
// entry must name an indexed config, as itself or with `.json` added, or it is
// missing. A package-named entry lives in node_modules, outside the index, and
// is external; an absolute or repository-escaping entry is treated as missing.
func (resolver *WebImportResolver) extendsPath(entry, directory string) (string, webExtendsState) {
	entry = strings.ReplaceAll(entry, "\\", "/")
	if !strings.HasPrefix(entry, "./") && !strings.HasPrefix(entry, "../") {
		if entry == "" || webRooted(entry) || strings.HasPrefix(entry, "${configDir}") {
			return "", webExtendsMissing
		}
		return "", webExtendsExternal
	}
	candidate := path.Join(directory, entry)
	if candidate == ".." || strings.HasPrefix(candidate, "../") {
		return "", webExtendsMissing
	}
	if _, ok := resolver.index.Sources[candidate]; ok {
		return candidate, webExtendsIndexed
	}
	if !strings.HasSuffix(candidate, ".json") {
		if _, ok := resolver.index.Sources[candidate+".json"]; ok {
			return candidate + ".json", webExtendsIndexed
		}
	}
	return "", webExtendsMissing
}

// match applies TypeScript's pattern choice: an exact key first, else the
// wildcard key with the longest prefix. Two matching wildcard keys with the
// same prefix length are ambiguous here, because TypeScript breaks that tie by
// object key order, which the parsed map does not keep.
func (options *webOptions) match(specifier string) (pattern, capture string, matched, ambiguous bool) {
	if _, ok := options.paths[specifier]; ok && !strings.Contains(specifier, "*") {
		return specifier, "", true, false
	}
	best := -1
	for candidate := range options.paths {
		star := strings.IndexByte(candidate, '*')
		if star < 0 {
			continue
		}
		prefix, suffix := candidate[:star], candidate[star+1:]
		if len(specifier) < len(prefix)+len(suffix) || !strings.HasPrefix(specifier, prefix) || !strings.HasSuffix(specifier, suffix) {
			continue
		}
		switch {
		case len(prefix) > best:
			best, pattern, ambiguous = len(prefix), candidate, false
			capture = specifier[len(prefix) : len(specifier)-len(suffix)]
		case len(prefix) == best:
			ambiguous = true
		}
	}
	return pattern, capture, best >= 0, ambiguous
}

// webSchemeSpecifier reports a URL-shaped specifier (`node:fs`, `bun:test`,
// `https://...`, `virtual:x`): a module the runtime or bundler supplies.
func webSchemeSpecifier(specifier string) bool {
	colon := strings.IndexByte(specifier, ':')
	if colon <= 0 {
		return false
	}
	for position, character := range specifier[:colon] {
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		if !letter && (position == 0 || !(character >= '0' && character <= '9' || character == '+' || character == '-' || character == '.')) {
			return false
		}
	}
	return true
}

// webPackageName is the package a bare specifier names: `@scope/name` or
// `name`, the first one or two segments.
func webPackageName(specifier string) string {
	parts := strings.SplitN(specifier, "/", 3)
	if strings.HasPrefix(specifier, "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// webNodeBuiltins are the Node.js core modules importable without `node:`.
var webNodeBuiltins = map[string]bool{
	"assert": true, "async_hooks": true, "buffer": true, "child_process": true, "cluster": true,
	"console": true, "constants": true, "crypto": true, "dgram": true, "diagnostics_channel": true,
	"dns": true, "domain": true, "events": true, "fs": true, "http": true, "http2": true, "https": true,
	"inspector": true, "module": true, "net": true, "os": true, "path": true, "perf_hooks": true,
	"process": true, "punycode": true, "querystring": true, "readline": true, "repl": true,
	"stream": true, "string_decoder": true, "sys": true, "timers": true, "tls": true,
	"trace_events": true, "tty": true, "url": true, "util": true, "v8": true, "vm": true,
	"wasi": true, "worker_threads": true, "zlib": true,
}

// declaredPackage reports whether the specifier's package is a Node builtin,
// a nested workspace package (GPK-V0-069 discloses those separately), or a
// dependency declared by a package.json at or above the importer, directly
// or through its `@types/` package.
func (resolver *WebImportResolver) declaredPackage(importer, specifier string) bool {
	name := webPackageName(specifier)
	if webNodeBuiltins[name] {
		return true
	}
	if resolver.workspaceNames()[name] {
		return true
	}
	declared := resolver.declaredAt(resolver.nearestManifest(path.Dir(importer)))
	if declared[name] {
		return true
	}
	types := "@types/" + name
	if scope, rest, scoped := strings.Cut(strings.TrimPrefix(name, "@"), "/"); scoped && strings.HasPrefix(name, "@") {
		types = "@types/" + scope + "__" + rest
	}
	return declared[types]
}

func (resolver *WebImportResolver) nearestManifest(directory string) string {
	if manifest, ok := resolver.manifest[directory]; ok {
		return manifest
	}
	manifest := ""
	if _, ok := resolver.index.Sources[path.Join(directory, "package.json")]; ok {
		manifest = directory
	} else if directory != "." {
		manifest = resolver.nearestManifest(path.Dir(directory))
	} else {
		manifest = "\x00"
	}
	resolver.manifest[directory] = manifest
	return manifest
}

// declaredAt unions a manifest's dependency names with every ancestor
// manifest's, which is what node_modules lookup walking upward can reach.
func (resolver *WebImportResolver) declaredAt(directory string) map[string]bool {
	if directory == "\x00" {
		return nil
	}
	if declared, ok := resolver.declared[directory]; ok {
		return declared
	}
	declared := map[string]bool{}
	for name := range resolver.manifestNames(path.Join(directory, "package.json")) {
		declared[name] = true
	}
	if directory != "." {
		for name := range resolver.declaredAt(resolver.nearestManifest(path.Dir(directory))) {
			declared[name] = true
		}
	}
	resolver.declared[directory] = declared
	return declared
}

func (resolver *WebImportResolver) manifestNames(name string) map[string]bool {
	if names, ok := resolver.ownNames[name]; ok {
		return names
	}
	names := map[string]bool{}
	source := resolver.index.Sources[name]
	if text, valid, loaded := source.Text(); valid && loaded && len(text) <= maxWebConfigBytes {
		var fields struct {
			Dependencies         map[string]json.RawMessage `json:"dependencies"`
			DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
			PeerDependencies     map[string]json.RawMessage `json:"peerDependencies"`
			OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
		}
		if json.Unmarshal([]byte(text), &fields) == nil {
			for _, group := range []map[string]json.RawMessage{fields.Dependencies, fields.DevDependencies, fields.PeerDependencies, fields.OptionalDependencies} {
				for dependency := range group {
					names[dependency] = true
				}
			}
		}
	}
	resolver.ownNames[name] = names
	return names
}

// workspaceNames is the set of nested package.json names, read once.
func (resolver *WebImportResolver) workspaceNames() map[string]bool {
	if resolver.names != nil {
		return resolver.names
	}
	resolver.names = map[string]bool{}
	for name, source := range resolver.index.Sources {
		if path.Base(name) != "package.json" || name == "package.json" {
			continue
		}
		if manifest, readable := packageManifestName(source); readable && manifest != "" {
			resolver.names[manifest] = true
		}
	}
	return resolver.names
}

// webManifestIndexed reports whether the repository indexes any manifest or
// config that declares a JavaScript dependency or resolution universe. Without
// one the default impact profile keeps the frozen GPK-V0-027 reading of a bare
// specifier as a dependency (GPK-V0-080).
func webManifestIndexed(index *Index) bool {
	for name := range index.Sources {
		switch path.Base(name) {
		case "package.json", "tsconfig.json", "jsconfig.json":
			return true
		}
	}
	return false
}

// jsoncToJSON removes `//` and `/* */` comments and trailing commas outside
// strings, the JSONC tsconfig dialect, in one bounded pass. An unterminated
// string or block comment is unreadable rather than guessed at.
func jsoncToJSON(text string) (string, bool) {
	var builder strings.Builder
	builder.Grow(len(text))
	pendingComma := -1
	for position := 0; position < len(text); position++ {
		character := text[position]
		switch {
		case character == '"':
			end := position + 1
			for end < len(text) && text[end] != '"' {
				if text[end] == '\\' {
					end++
				}
				end++
			}
			if end >= len(text) {
				return "", false
			}
			pendingComma = -1
			builder.WriteString(text[position : end+1])
			position = end
		case strings.HasPrefix(text[position:], "//"):
			end := strings.IndexAny(text[position:], "\r\n")
			if end < 0 {
				position = len(text)
				continue
			}
			position += end - 1
		case strings.HasPrefix(text[position:], "/*"):
			end := strings.Index(text[position+2:], "*/")
			if end < 0 {
				return "", false
			}
			builder.WriteByte(' ')
			position += end + 3
		case character == ',':
			pendingComma = builder.Len()
			builder.WriteByte(',')
		case character == '}' || character == ']':
			if pendingComma >= 0 {
				// Rewrite the trailing comma in place as a space.
				raw := builder.String()
				builder.Reset()
				builder.WriteString(raw[:pendingComma])
				builder.WriteByte(' ')
				builder.WriteString(raw[pendingComma+1:])
				pendingComma = -1
			}
			builder.WriteByte(character)
		case character == ' ' || character == '\t' || character == '\n' || character == '\r':
			builder.WriteByte(character)
		default:
			pendingComma = -1
			builder.WriteByte(character)
		}
	}
	return builder.String(), true
}

// webImportGraph is rule (c)'s alias extension over one index, built once
// per impact call: the importers each tracked target is reached by through a
// bare specifier, and the unresolved bare specifiers that leave every web
// reverse-import answer incomplete.
type webImportGraph struct {
	resolver   *WebImportResolver
	importers  map[string][]importer
	unresolved int
	sources    int
}

func buildWebImportGraph(index *Index) *webImportGraph {
	graph := &webImportGraph{resolver: NewWebImportResolver(index), importers: map[string][]importer{}}
	prefix := graph.resolver.profile.WebAliasPrefix
	for importerPath, imports := range index.Imports {
		if !webSuffixes[strings.ToLower(pythonPathSuffix(importerPath))] {
			continue
		}
		unresolved := 0
		for imported := range imports {
			// Relative and profile-alias specifiers stay rule (c)'s oracle arm.
			if strings.HasPrefix(imported, ".") || prefix != "" && strings.HasPrefix(imported, prefix) {
				continue
			}
			resolution := graph.resolver.resolveBare(importerPath, imported)
			switch resolution.State {
			case WebImportRepository:
				if resolution.Target != importerPath {
					graph.importers[resolution.Target] = append(graph.importers[resolution.Target], importer{importerPath, imported})
				}
			case WebImportUnresolved:
				unresolved++
			}
		}
		if unresolved != 0 {
			graph.unresolved += unresolved
			graph.sources++
		}
	}
	for target := range graph.importers {
		sortImporters(graph.importers[target])
	}
	return graph
}

// webTypeOnlyImport reports whether every statement by which source names
// imported is type-only.
func webTypeOnlyImport(source Source, imported string, cache map[string]map[string]bool) bool {
	if !webSuffixes[strings.ToLower(pythonPathSuffix(source.Path))] {
		return false
	}
	kinds, ok := cache[source.Path]
	if !ok {
		if text, valid, loaded := source.Text(); valid && loaded {
			kinds, _ = WebImportKinds(text)
		}
		cache[source.Path] = kinds
	}
	return kinds[imported]
}

// webRooted reports a path TypeScript treats as rooted (getEncodedRootLength):
// a leading slash, a drive letter or a URL. It cannot name a file by its
// repository-relative path.
func webRooted(value string) bool {
	if strings.HasPrefix(value, "/") || strings.Contains(value, "://") {
		return true
	}
	return len(value) >= 2 && value[1] == ':' && ('a' <= value[0]|0x20 && value[0]|0x20 <= 'z')
}
