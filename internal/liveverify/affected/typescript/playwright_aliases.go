package typescript

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// These inputs are already covered by ObservePlaywrightSources. The general
// affected-plan adapter resolves the same aliases only when Playwright is the sole
// runner identity it observes (TJAA-V0-037); otherwise it retains its alias frontier.
type typeScriptAliases struct {
	directory string
	base      string
	baseURL   bool
	paths     map[string][]string
	valid     bool
	// allowJS is a literal `allowJs: true`, or its absence in a jsconfig.json, which Playwright
	// requires before it maps a specifier of a module that is not .ts or .tsx (AFP-V0-418).
	allowJS bool
	// refusal names the key or pattern that left an invalid config unresolved, for its frontier
	// site (TJAA-V0-052).
	refusal string
	// refusalKey is the JSON key that locates refusal's site when refusal is not itself one, such
	// as "extends" for a named extends depth bound (TJAA-V0-114).
	refusalKey string
}

// aliasConfigFile reports whether name is the alias config of its folder that Playwright reads:
// tsconfig.json, or jsconfig.json when no tsconfig.json is beside it (AFP-V0-418, TJAA-V0-960).
func aliasConfigFile(name string, bodies map[string]string) bool {
	switch path.Base(name) {
	case "tsconfig.json":
		return true
	case "jsconfig.json":
		_, sibling := bodies[path.Join(path.Dir(name), "tsconfig.json")]
		return !sibling
	}
	return false
}

func readTypeScriptAliases(bodies map[string]string, frontier frontierSet) []typeScriptAliases {
	var configs []typeScriptAliases
	for _, name := range sortedKeys(bodies) {
		if !aliasConfigFile(name, bodies) {
			continue
		}
		config := parseTypeScriptAliasesIn(name, bodies)
		if !config.valid {
			body := bodies[name]
			offset := -1
			key := config.refusal
			if config.refusalKey != "" {
				key = config.refusalKey
			}
			if key != "" {
				if found := regexp.MustCompile(regexp.QuoteMeta(strconv.Quote(key)) + `\s*:`).FindStringIndex(body); found != nil {
					offset = found[0]
				}
			}
			// TJAA-V0-058: a refused tsconfig explains every import it leaves unresolved.
			site := lineSite(name, body, offset)
			frontier.raiseToken(FrontierPathAlias, site, config.refusal)
			frontier.pin(FrontierPathAlias, site)
		}
		configs = append(configs, config)
	}
	return configs
}

// typeScriptExtendsBound caps the `extends` hops one config chain may follow; a longer chain is
// refused and the refusal names the bound (TJAA-V0-114).
const typeScriptExtendsBound = 8

// typeScriptConfigLayer is the alias-relevant part of one config in an `extends` chain.
type typeScriptConfigLayer struct {
	name    string
	extends json.RawMessage
	baseURL *string
	paths   map[string][]string
	allowJS string
	refusal string
}

func parseTypeScriptConfigLayer(name, body string) (typeScriptConfigLayer, bool) {
	layer := typeScriptConfigLayer{name: name}
	clean, err := stripComments(body, false)
	if err != nil || !strings.HasPrefix(strings.TrimSpace(clean), "{") {
		return layer, false
	}
	clean = typeScriptJSONTrailingCommas(clean)
	var raw struct {
		Extends         json.RawMessage `json:"extends"`
		CompilerOptions struct {
			BaseURL        *string             `json:"baseUrl"`
			Paths          map[string][]string `json:"paths"`
			AllowJS        json.RawMessage     `json:"allowJs"`
			RootDirs       json.RawMessage     `json:"rootDirs"`
			ModuleSuffixes json.RawMessage     `json:"moduleSuffixes"`
		} `json:"compilerOptions"`
	}
	if jsonv2.Unmarshal([]byte(clean), &raw) != nil {
		return layer, false
	}
	// Alternative module-resolution configurations are not approximated.
	for key, value := range map[string]json.RawMessage{"rootDirs": raw.CompilerOptions.RootDirs, "moduleSuffixes": raw.CompilerOptions.ModuleSuffixes} {
		if len(value) != 0 && (layer.refusal == "" || key < layer.refusal) {
			layer.refusal = key
		}
	}
	layer.extends = raw.Extends
	layer.baseURL = raw.CompilerOptions.BaseURL
	layer.paths = raw.CompilerOptions.Paths
	layer.allowJS = strings.TrimSpace(string(raw.CompilerOptions.AllowJS))
	return layer, true
}

// typeScriptExtendsTarget resolves one `extends` value of the config at name to an observed config
// inside the repository root: a single string that starts with `./` or `../`, with `.json`
// appended when the value lacks it and only that spelling is observed. A package name, an array,
// an absolute path, an escape from the root or an unobserved file is refused (TJAA-V0-114).
func typeScriptExtendsTarget(name string, raw json.RawMessage, bodies map[string]string) (string, bool) {
	var value string
	if jsonv2.Unmarshal(raw, &value) != nil || !(strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../")) {
		return "", false
	}
	target, inside := playwrightJoinInsideRoot(path.Dir(name), value)
	if !inside {
		return "", false
	}
	if _, observed := bodies[target]; observed {
		return target, true
	}
	if !strings.HasSuffix(target, ".json") {
		if _, observed := bodies[target+".json"]; observed {
			return target + ".json", true
		}
	}
	return "", false
}

// typeScriptAliasDeclaringConfig returns the first config in the observed relative `extends`
// chain of name, name included, whose body holds a paths or baseUrl key, or "" when none does.
// A chain longer than typeScriptExtendsBound returns name, so an alias it may inherit is never
// assumed absent (TJAA-V0-114).
func typeScriptAliasDeclaringConfig(name string, bodies map[string]string) string {
	seen := map[string]bool{}
	for current := name; !seen[current]; {
		if len(seen) > typeScriptExtendsBound {
			return name
		}
		seen[current] = true
		body := bodies[current]
		if pathAliasMarker(body) >= 0 {
			return current
		}
		layer, parsed := parseTypeScriptConfigLayer(current, body)
		if !parsed || len(layer.extends) == 0 {
			return ""
		}
		next, resolved := typeScriptExtendsTarget(current, layer.extends, bodies)
		if !resolved {
			return ""
		}
		current = next
	}
	return ""
}

// parseTypeScriptAliases parses one config with no other config observed, so an `extends` it
// declares is refused.
func parseTypeScriptAliases(name, body string) typeScriptAliases {
	return parseTypeScriptAliasesIn(name, map[string]string{name: body})
}

// parseTypeScriptAliasesIn parses the config at name, following its relative `extends` chain
// through the observed configs in bodies up to typeScriptExtendsBound hops, with TypeScript's
// inheritance: the nearest config that sets baseUrl, paths or allowJs wins, a baseUrl resolves
// against the config that declares it, and paths resolve against the effective baseUrl or, with
// none, against the config that declares them. Every directory and target must stay inside the
// repository root; a `../` that does is accepted (TJAA-V0-113, TJAA-V0-114). A refusal that an
// inherited config causes is named "extends", at the child's extends key.
func parseTypeScriptAliasesIn(name string, bodies map[string]string) typeScriptAliases {
	config := typeScriptAliases{directory: path.Dir(name)}
	var chain []typeScriptConfigLayer
	seen := map[string]bool{}
	for current := name; ; {
		layer, ok := parseTypeScriptConfigLayer(current, bodies[current])
		switch {
		case !ok && current == name:
			return config
		case !ok:
			config.refusal = "extends"
			return config
		case layer.refusal != "" && current == name:
			config.refusal = layer.refusal
			return config
		case layer.refusal != "":
			config.refusal = "extends"
			return config
		}
		chain = append(chain, layer)
		seen[current] = true
		if len(layer.extends) == 0 {
			break
		}
		if len(chain) > typeScriptExtendsBound {
			config.refusal, config.refusalKey = "extends depth bound "+strconv.Itoa(typeScriptExtendsBound), "extends"
			return config
		}
		next, ok := typeScriptExtendsTarget(current, layer.extends, bodies)
		if !ok || seen[next] {
			config.refusal = "extends"
			return config
		}
		current = next
	}
	baseLayer, pathsLayer := -1, -1
	allowJS := ""
	for index, layer := range chain {
		if baseLayer < 0 && layer.baseURL != nil {
			baseLayer = index
		}
		if pathsLayer < 0 && layer.paths != nil {
			pathsLayer = index
		}
		if allowJS == "" {
			allowJS = layer.allowJS
		}
	}
	config.allowJS = allowJS == "true" || allowJS == "" && path.Base(name) == "jsconfig.json"
	config.base = config.directory
	if pathsLayer >= 0 {
		config.base = path.Dir(chain[pathsLayer].name)
		config.paths = chain[pathsLayer].paths
	}
	if baseLayer >= 0 {
		base, inside := playwrightJoinInsideRoot(path.Dir(chain[baseLayer].name), *chain[baseLayer].baseURL)
		if !inside {
			config.refusal = "baseUrl"
			if baseLayer != 0 {
				config.refusal = "extends"
			}
			return config
		}
		config.base = base
		config.baseURL = true
	}
	for _, pattern := range sortedKeys(config.paths) {
		targets := config.paths[pattern]
		config.refusal = pattern
		if pathsLayer != 0 {
			config.refusal = "extends"
		}
		if pattern == "" || strings.Count(pattern, "*") > 1 || len(targets) == 0 {
			return config
		}
		for _, target := range targets {
			_, inside := playwrightJoinInsideRoot(config.base, strings.ReplaceAll(target, "*", "wildcard"))
			if strings.Count(target, "*") > strings.Count(pattern, "*") || !inside {
				return config
			}
		}
	}
	config.refusal = ""
	config.valid = true
	return config
}

// resolveTypeScriptAliases rewrites each bare reference the nearest alias config resolves to the
// relative path of its target. When keep reports true for the original reference, the target is
// added beside it instead, so the edges the original names are kept (TJAA-V0-059).
func resolveTypeScriptAliases(owner string, refs []string, configs []typeScriptAliases, bodies map[string]string, sites sourceSites, frontier frontierSet, keep func(string) bool) []string {
	nearest := nearestTypeScriptAliases(owner, configs)
	typeScript := strings.HasSuffix(owner, ".ts") || strings.HasSuffix(owner, ".tsx")
	// Playwright maps no specifier of a module that is not .ts or .tsx without allowJs
	// (AFP-V0-418, TJAA-V0-960). Under a blocked gate (keep set) every edge is speculative and
	// kept, so the selection stays a superset of what any runner resolves (TJAA-V0-059).
	if nearest == nil || !nearest.valid || !typeScript && !nearest.allowJS && keep == nil {
		return refs
	}
	resolved := append([]string(nil), refs...)
	for index, ref := range refs {
		if strings.HasPrefix(ref, ".") || externalScheme(ref) {
			continue
		}
		target, claimed := nearest.resolve(ref, bodies)
		if target == "" {
			if claimed {
				sites.raiseRef(frontier, FrontierPathAlias, owner, ref)
			}
			continue
		}
		relative, err := filepath.Rel(filepath.FromSlash(path.Dir(owner)), filepath.FromSlash(target))
		if err != nil {
			sites.raiseRef(frontier, FrontierPathAlias, owner, ref)
			continue
		}
		rewritten := "./" + filepath.ToSlash(relative)
		if keep != nil && keep(ref) {
			resolved = append(resolved, rewritten)
		} else {
			resolved[index] = rewritten
		}
		sites.alias(owner, ref, rewritten)
	}
	return resolved
}

func (config typeScriptAliases) resolve(ref string, bodies map[string]string) (string, bool) {
	patterns := sortedKeys(config.paths)
	// Exact keys win; wildcard keys use the longest matching prefix.
	sort.SliceStable(patterns, func(i, j int) bool {
		left, right := strings.IndexByte(patterns[i], '*'), strings.IndexByte(patterns[j], '*')
		if left == -1 {
			return right != -1
		}
		if right == -1 {
			return false
		}
		return left > right
	})
	for _, pattern := range patterns {
		capture, matches := typeScriptAliasMatch(pattern, ref)
		if !matches {
			continue
		}
		if prefix := strings.IndexByte(pattern, '*'); prefix >= 0 {
			for _, other := range patterns {
				if other == pattern || strings.IndexByte(other, '*') != prefix {
					continue
				}
				if _, matches := typeScriptAliasMatch(other, ref); matches {
					return "", true
				}
			}
		}
		chosen := ""
		for _, target := range config.paths[pattern] {
			base := path.Clean(path.Join(config.base, strings.ReplaceAll(target, "*", capture)))
			resolved, ambiguous := typeScriptAliasTarget(base, bodies)
			if ambiguous {
				return "", true
			}
			if resolved != "" {
				if chosen != "" {
					return "", true
				}
				chosen = resolved
			}
		}
		return chosen, true
	}
	if config.baseURL {
		return typeScriptAliasTarget(path.Clean(path.Join(config.base, ref)), bodies)
	}
	return "", false
}

func typeScriptAliasMatch(pattern, ref string) (string, bool) {
	star := strings.IndexByte(pattern, '*')
	if star == -1 {
		return "", pattern == ref
	}
	prefix, suffix := pattern[:star], pattern[star+1:]
	if !strings.HasPrefix(ref, prefix) || !strings.HasSuffix(ref, suffix) || len(ref) < len(prefix)+len(suffix) {
		return "", false
	}
	return ref[len(prefix) : len(ref)-len(suffix)], true
}

func typeScriptAliasTarget(base string, bodies map[string]string) (string, bool) {
	if !playwrightRelativeDirectory(base) {
		return "", true
	}
	if _, packageDirectory := bodies[path.Join(base, "package.json")]; packageDirectory {
		return "", true
	}
	var target string
	for _, candidate := range moduleCandidates(base) {
		if _, exists := bodies[candidate]; !exists || !hasSourceExtension(candidate) {
			continue
		}
		if target != "" && target != candidate {
			return "", true
		}
		target = candidate
	}
	return target, false
}

func typeScriptJSONTrailingCommas(body string) string {
	clean := []byte(body)
	for index := 0; index < len(body); index++ {
		if body[index] == '"' {
			end := quotedEnd(body, index)
			if end <= index {
				return body
			}
			index = end - 1
			continue
		}
		if body[index] != ',' {
			continue
		}
		next := skipSpace(body, index+1)
		if next < len(body) && (body[next] == '}' || body[next] == ']') {
			clean[index] = ' '
		}
	}
	return string(clean)
}
