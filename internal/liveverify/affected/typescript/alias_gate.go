package typescript

import (
	"path"
	"strings"
)

// aliasFreeTests returns the test observations of a runner other than Playwright whose loader
// never meets a tsconfig path alias: no module in the test's relative-import closure names a bare
// reference the nearest tsconfig.json can claim. Such a runner's resolution of aliases decides no
// edge, so it does not keep the aliases unresolved (AFP-V0-389). A closure module whose imports
// are not all statically observed, a bare reference under an invalid nearest tsconfig, or one that
// names a workspace package (whose sources the closure does not follow) keeps the test blocking.
func aliasFreeTests(observations []observation, bodies map[string]string, scopes []packageScope, opaque map[string]bool) map[string]bool {
	var aliases []typeScriptAliases
	for _, name := range sortedKeys(bodies) {
		if path.Base(name) == "tsconfig.json" {
			aliases = append(aliases, parseTypeScriptAliases(name, bodies[name]))
		}
	}
	byPath := make(map[string]*observation, len(observations))
	paths := make(map[string]string, len(observations))
	for index := range observations {
		byPath[observations[index].path] = &observations[index]
		paths[observations[index].path] = observations[index].path
	}
	free := map[string]bool{}
	for _, start := range observations {
		if !start.test || start.runner == runnerPlaywright || start.runner == "" || start.runner == runnerUnknown {
			continue
		}
		if aliasFreeClosure(start.path, byPath, paths, aliases, bodies, scopes, opaque) {
			free[start.path] = true
		}
	}
	return free
}

func aliasFreeClosure(start string, byPath map[string]*observation, paths map[string]string, aliases []typeScriptAliases, bodies map[string]string, scopes []packageScope, opaque map[string]bool) bool {
	seen := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		if opaque[current] {
			return false
		}
		nearest := nearestTypeScriptAliases(current, aliases)
		for _, ref := range byPath[current].refs {
			if strings.HasPrefix(ref, ".") {
				if target, _, _ := resolveImport(current, ref, paths); target != "" && !seen[target] {
					seen[target] = true
					queue = append(queue, target)
				}
				continue
			}
			if externalScheme(ref) {
				continue
			}
			if localPackage(importedPackage(ref), scopes) {
				return false
			}
			if nearest == nil {
				continue
			}
			if !nearest.valid {
				return false
			}
			if target, claimed := nearest.resolve(ref, bodies); claimed || target != "" {
				return false
			}
		}
	}
	return true
}

// nearestTypeScriptAliases returns the tsconfig.json alias config nearest to owner, or nil.
func nearestTypeScriptAliases(owner string, configs []typeScriptAliases) *typeScriptAliases {
	var nearest *typeScriptAliases
	for index := range configs {
		config := &configs[index]
		if inside(owner, config.directory) && (nearest == nil || nearest.directory == "." || len(config.directory) > len(nearest.directory)) {
			nearest = config
		}
	}
	return nearest
}

// playwrightStaticMembership reports whether a Playwright config's membership markers are ones
// the closed static profile resolves (testDir and projects only, with no testMatch or
// testIgnore), and every file the profile assigns to a project is a Playwright test of that
// config in the general adapter. The general adapter's membership is then the config's, so the
// markers raise no executable-config frontier (AFP-V0-390). files is every observed file name.
func playwrightStaticMembership(config config, files []string, observations []observation) bool {
	for _, marker := range dynamicConfigMarkers {
		if marker != "testDir" && marker != "projects:" && strings.Contains(config.body, marker) {
			return false
		}
	}
	projects, globalTestDir, unknown := parsePlaywrightConfig(config.path, config.body)
	membership := []PlaywrightUnknown{}
	for _, entry := range unknown {
		switch entry.Reason {
		case PlaywrightUnknownBrowserIdentity, PlaywrightUnknownUseValueUnverified, PlaywrightUnknownProjectDependency:
			// These decide which browser runs or in which order, not which files belong.
		default:
			membership = append(membership, entry)
		}
	}
	if len(projects) == 0 && len(membership) == 1 && membership[0].Reason == PlaywrightUnknownProjectSet && membership[0].Detail == "projects is absent" {
		projects, membership = []PlaywrightProject{{}}, nil
	}
	if len(membership) != 0 || len(projects) == 0 {
		return false
	}
	for _, project := range projects {
		if len(project.matchers) != 0 || len(project.ignores) != 0 || len(project.TestMatch) != 0 || len(project.TestIgnore) != 0 {
			return false
		}
	}
	byPath := make(map[string]observation, len(observations))
	for _, observation := range observations {
		byPath[observation.path] = observation
	}
	for _, name := range files {
		owned := false
		for _, project := range projects {
			if playwrightProjectOwns("", project, globalTestDir, name) {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}
		observation, observed := byPath[name]
		if !observed || !observation.test || observation.runner != runnerPlaywright || !containsString(observation.configs, config.path) {
			return false
		}
	}
	return true
}
