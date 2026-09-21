package typescript

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// These inputs are already covered by ObservePlaywrightSources. The general
// affected-plan adapter deliberately retains its original alias frontier.
type typeScriptAliases struct {
	directory string
	base      string
	baseURL   bool
	paths     map[string][]string
	valid     bool
}

func readTypeScriptAliases(bodies map[string]string, frontier map[string]bool) []typeScriptAliases {
	var configs []typeScriptAliases
	for _, name := range sortedKeys(bodies) {
		if path.Base(name) != "tsconfig.json" {
			continue
		}
		config := parseTypeScriptAliases(name, bodies[name])
		if !config.valid {
			frontier[FrontierPathAlias] = true
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
	if jsonv2.Unmarshal([]byte(clean), &raw) != nil || len(raw.Extends) != 0 || len(raw.CompilerOptions.RootDirs) != 0 || len(raw.CompilerOptions.ModuleSuffixes) != 0 {
		return config
	}
	config.base = config.directory
	if raw.CompilerOptions.BaseURL != nil {
		base := *raw.CompilerOptions.BaseURL
		if !playwrightRelativeDirectory(base) {
			return config
		}
		config.base = path.Clean(path.Join(config.directory, base))
		config.baseURL = true
	}
	config.paths = raw.CompilerOptions.Paths
	for pattern, targets := range config.paths {
		if pattern == "" || strings.Count(pattern, "*") > 1 || len(targets) == 0 {
			return config
		}
		for _, target := range targets {
			if strings.Count(target, "*") > strings.Count(pattern, "*") || !playwrightRelativeDirectory(strings.ReplaceAll(target, "*", "wildcard")) {
				return config
			}
		}
	}
	config.valid = true
	return config
}

func resolveTypeScriptAliases(owner string, refs []string, configs []typeScriptAliases, bodies map[string]string, frontier map[string]bool) []string {
	var nearest *typeScriptAliases
	for index := range configs {
		config := &configs[index]
		if inside(owner, config.directory) && (nearest == nil || nearest.directory == "." || len(config.directory) > len(nearest.directory)) {
			nearest = config
		}
	}
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
				frontier[FrontierPathAlias] = true
			}
			continue
		}
		relative, err := filepath.Rel(filepath.FromSlash(path.Dir(owner)), filepath.FromSlash(target))
		if err != nil {
			frontier[FrontierPathAlias] = true
			continue
		}
		resolved[index] = "./" + filepath.ToSlash(relative)
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
