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
	// refusal names the key or pattern that left an invalid config unresolved, for its frontier
	// site (TJAA-V0-052).
	refusal string
}

func readTypeScriptAliases(bodies map[string]string, frontier frontierSet) []typeScriptAliases {
	var configs []typeScriptAliases
	for _, name := range sortedKeys(bodies) {
		if path.Base(name) != "tsconfig.json" {
			continue
		}
		config := parseTypeScriptAliases(name, bodies[name])
		if !config.valid {
			body := bodies[name]
			offset := -1
			if config.refusal != "" {
				if found := regexp.MustCompile(regexp.QuoteMeta(strconv.Quote(config.refusal)) + `\s*:`).FindStringIndex(body); found != nil {
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

func parseTypeScriptAliases(name, body string) typeScriptAliases {
	config := typeScriptAliases{directory: path.Dir(name)}
	clean, err := stripComments(body, false)
	if err != nil || !strings.HasPrefix(strings.TrimSpace(clean), "{") {
		return config
	}
	clean = typeScriptJSONTrailingCommas(clean)
	var raw struct {
		Extends         json.RawMessage `json:"extends"`
		CompilerOptions struct {
			BaseURL        *string             `json:"baseUrl"`
			Paths          map[string][]string `json:"paths"`
			RootDirs       json.RawMessage     `json:"rootDirs"`
			ModuleSuffixes json.RawMessage     `json:"moduleSuffixes"`
		} `json:"compilerOptions"`
	}
	// Executable, inherited, or alternative module-resolution configurations are
	// not approximated. A nearer unsupported config also blocks ancestor aliases.
	if jsonv2.Unmarshal([]byte(clean), &raw) != nil {
		return config
	}
	for key, value := range map[string]json.RawMessage{"extends": raw.Extends, "rootDirs": raw.CompilerOptions.RootDirs, "moduleSuffixes": raw.CompilerOptions.ModuleSuffixes} {
		if len(value) != 0 && (config.refusal == "" || key < config.refusal) {
			config.refusal = key
		}
	}
	if config.refusal != "" {
		return config
	}
	config.base = config.directory
	if raw.CompilerOptions.BaseURL != nil {
		base := *raw.CompilerOptions.BaseURL
		if !playwrightRelativeDirectory(base) {
			config.refusal = "baseUrl"
			return config
		}
		config.base = path.Clean(path.Join(config.directory, base))
		config.baseURL = true
	}
	config.paths = raw.CompilerOptions.Paths
	for _, pattern := range sortedKeys(config.paths) {
		targets := config.paths[pattern]
		config.refusal = pattern
		if pattern == "" || strings.Count(pattern, "*") > 1 || len(targets) == 0 {
			return config
		}
		for _, target := range targets {
			if strings.Count(target, "*") > strings.Count(pattern, "*") || !playwrightRelativeDirectory(strings.ReplaceAll(target, "*", "wildcard")) {
				return config
			}
		}
	}
	config.refusal = ""
	config.valid = true
	return config
}

// resolveTypeScriptAliases rewrites each bare reference the nearest tsconfig resolves to the
// relative path of its target. When keep reports true for the original reference, the target is
// added beside it instead, so the edges the original names are kept (TJAA-V0-059).
func resolveTypeScriptAliases(owner string, refs []string, configs []typeScriptAliases, bodies map[string]string, sites sourceSites, frontier frontierSet, keep func(string) bool) []string {
	nearest := nearestTypeScriptAliases(owner, configs)
	if nearest == nil || !nearest.valid {
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
