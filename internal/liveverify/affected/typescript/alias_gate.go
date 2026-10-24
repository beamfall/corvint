package typescript

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// aliasFreeTests returns the test observations of a runner other than Playwright whose loader
// never meets a tsconfig path alias: no module in the test's relative-import closure names a bare
// reference the nearest tsconfig.json can claim. Such a runner's resolution of aliases decides no
// edge, so it does not keep the aliases unresolved (AFP-V0-389). A closure module whose imports
// are not all statically observed, a relative import that resolves to no observed module, a bare
// reference under an invalid nearest tsconfig, or one that names a workspace package (whose
// sources the closure does not follow) keeps the test blocking.
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
				target, _, _ := resolveImport(current, ref, paths)
				if target == "" {
					// An import the closure cannot follow may reach an alias.
					return false
				}
				if !seen[target] {
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
// the closed static profile resolves (testDir and projects, and a project testMatch that is only
// literal root-free string globs, with no testIgnore), and every file the profile assigns to a
// project is a Playwright test of that config in the general adapter (TJAA-V0-091). The general adapter's membership is then the config's, so the
// markers raise no executable-config frontier (AFP-V0-404). files is every observed file name.
// When it is not static, site and token name what blocks it: the first marker the profile does not
// model, the membership unknown the parse leaves, or a profile-owned file that is not a Playwright
// test of the config, never a modelled testDir or projects marker (AFP-V0-423).
func playwrightStaticMembership(config config, files []string, observations []observation) (site, token string, static bool) {
	unmodelled := -1
	for _, marker := range dynamicConfigMarkers {
		if found := strings.Index(config.body, marker); marker != "testDir" && marker != "projects:" && marker != "testMatch" && found >= 0 && (unmodelled < 0 || found < unmodelled) {
			unmodelled = found
		}
	}
	testMatchAt := strings.Index(config.body, "testMatch")
	if unmodelled >= 0 && (testMatchAt < 0 || unmodelled < testMatchAt) {
		return lineSite(config.path, config.body, unmodelled), sourceToken(config.body[unmodelled:]), false
	}
	projects, globalTestDir, unknown := parsePlaywrightConfig(config.path, config.body)
	// testMatchSite names the first testMatch whose value refused reports, or the first testMatch
	// when none does, so a refused second project names its own line (TJAA-V0-092).
	testMatchSite := func(refused func(items []string) bool) (string, string, bool) {
		if testMatchAt < 0 {
			// An escaped key such as "test\u004datch" parses to a testMatch the raw body never names.
			return config.path, "testMatch", false
		}
		at := playwrightTestMatchAt(config.body, refused)
		if at < 0 {
			at = testMatchAt
		}
		return lineSite(config.path, config.body, at), sourceToken(config.body[at:]), false
	}
	parsedTestMatch := false
	for _, project := range projects {
		parsedTestMatch = parsedTestMatch || len(project.TestMatch) != 0
	}
	if (testMatchAt >= 0 || parsedTestMatch) && !playwrightLiteralTestMatches(config.body, projects, unknown) {
		return testMatchSite(func(items []string) bool {
			for _, item := range items {
				if _, ok := playwrightRootFreeGlob(item); !ok {
					return true
				}
			}
			return len(items) == 0
		})
	}
	if unmodelled >= 0 {
		return lineSite(config.path, config.body, unmodelled), sourceToken(config.body[unmodelled:]), false
	}
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
	if len(membership) != 0 {
		return config.path, membership[0].Reason, false
	}
	if len(projects) == 0 {
		return config.path, "projects", false
	}
	for _, project := range projects {
		if len(project.ignores) != 0 || len(project.TestIgnore) != 0 {
			return config.path, "testMatch", false
		}
	}
	byPath := make(map[string]observation, len(observations))
	for _, observation := range observations {
		byPath[observation.path] = observation
	}
	for _, name := range files {
		owned := false
		for _, project := range projects {
			// Playwright matches the absolute path, so a glob longer than a path whose own
			// segments match the glob's tail is decided by the checkout's directories, and a path
			// outside the matcher model (a non-BMP character or a line break) can match in minimatch
			// where the Go matcher does not (TJAA-V0-091).
			for _, glob := range project.TestMatch {
				if playwrightInsideTestDir(project, globalTestDir, name) && (playwrightPathOutsideMatcherModel(name) || playwrightGlobReachesRoot(glob, name)) {
					return testMatchSite(func(items []string) bool { return slices.Contains(items, glob) })
				}
			}
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
			return config.path, "owns " + name, false
		}
	}
	return "", "", true
}

// playwrightLiteralTestMatches reports whether every `testMatch` in a config body is one parsed
// project's testMatch whose items are all escape-free literal string globs that
// playwrightRootFreeGlob accepts: the parse left no unknown about a testMatch, and the body names
// testMatch exactly once for each project that has one, so no global, nested, computed, commented
// or spread testMatch is left unaccounted for. A regular expression, a computed value or a glob
// with a brace, a class, an extglob or a globstar that is not a whole segment is not literal
// (TJAA-V0-091, TJAA-V0-092).
func playwrightLiteralTestMatches(body string, projects []PlaywrightProject, unknown []PlaywrightUnknown) bool {
	for _, entry := range unknown {
		if strings.Contains(entry.Detail, "testMatch") {
			return false
		}
	}
	matching := 0
	for _, project := range projects {
		if len(project.TestMatch) == 0 {
			continue
		}
		matching++
		for _, raw := range project.TestMatch {
			if _, ok := playwrightRootFreeGlob(raw); !ok {
				return false
			}
		}
	}
	return matching != 0 && matching == strings.Count(body, "testMatch")
}

// playwrightLiteralProjects parses a Playwright config and returns its projects and global
// testDir when every testMatch in it is a literal root-free string glob
// (playwrightLiteralTestMatches), or nil (TJAA-V0-092).
func playwrightLiteralProjects(path, body string) ([]PlaywrightProject, string) {
	projects, globalTestDir, unknown := parsePlaywrightConfig(path, body)
	if !playwrightLiteralTestMatches(body, projects, unknown) {
		return nil, ""
	}
	return projects, globalTestDir
}

// playwrightLiteralOwned reports whether a project with a literal testMatch in one of the
// Playwright configs named by paths owns relative, so a setup file such as
// setup/global-setup.ts under testDir './' is a test of that project rather than support code
// (TJAA-V0-092).
func playwrightLiteralOwned(relative string, paths []string, configs []config) bool {
	for _, config := range configs {
		if !containsString(paths, config.path) {
			continue
		}
		for _, project := range config.literalProjects {
			if len(project.TestMatch) != 0 && playwrightProjectOwns("", project, config.literalTestDir, relative) {
				return true
			}
		}
	}
	return false
}

// playwrightTestMatchAt returns the offset of the first `testMatch` key in body whose value
// refused reports, given its items (one item for a value that is not an array literal, none for
// a value that does not parse), or -1 when none does (TJAA-V0-092).
func playwrightTestMatchAt(body string, refused func(items []string) bool) int {
	for _, at := range playwrightTokenOffsets(body, "testMatch") {
		rest := body[at+len("testMatch"):]
		index := 0
		if index < len(rest) && (rest[index] == '\'' || rest[index] == '"') {
			index++
		}
		index = playwrightNextCode(rest, index)
		items := []string{}
		if index < len(rest) && rest[index] == ':' {
			if value, _, ok := playwrightBalancedValue(rest, index+1, '}'); ok {
				if parts, ok := playwrightSplitTopLevel(value); ok && len(parts) != 0 && parts[0] != "" {
					items = []string{parts[0]}
					if array, ok := playwrightArrayItems(parts[0]); ok {
						items = array
					}
				}
			}
		}
		if refused(items) {
			return at
		}
	}
	return -1
}

// playwrightRootFreeGlob reports whether raw is an escape-free literal string glob that
// playwrightGlobReachesRoot can decide for a test path, and returns its segments without
// Playwright's own leading `**/`. A segment is a whole globstar `**` or holds only segment-local
// `*` and `?`; the last segment is not a globstar, and no segment is empty, `.` or `..`
// (TJAA-V0-091, TJAA-V0-092).
func playwrightRootFreeGlob(raw string) ([]string, bool) {
	value, literal := playwrightString(raw)
	if !literal || strings.ContainsRune(raw, '\\') {
		return nil, false
	}
	value = strings.TrimPrefix(value, "**/")
	if value == "" || strings.ContainsAny(value, "{}[]()!+@\\") {
		return nil, false
	}
	segments := strings.Split(value, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || segment != "**" && strings.Contains(segment, "**") {
			return nil, false
		}
	}
	if segments[len(segments)-1] == "**" {
		return nil, false
	}
	return segments, true
}

// playwrightGlobReachesRoot reports whether Playwright's match of a playwrightRootFreeGlob glob
// against the absolute path of the repository path name depends on directories above the
// repository. Playwright matches `**/` plus the glob, so the absolute path matches whenever name
// alone does; otherwise some directory prefix makes it match exactly when name matches the glob
// with one or more of its leading segments dropped, since those segments, or a globstar's share
// of them, can be matched by the prefix. A glob that is not root-free reaches the root.
func playwrightGlobReachesRoot(raw, name string) bool {
	segments, ok := playwrightRootFreeGlob(raw)
	if !ok {
		return true
	}
	matches := func(glob string) (bool, bool) {
		pattern, valid := playwrightGlobPattern(glob)
		if !valid {
			return false, false
		}
		re, err := regexp.Compile("(?i)^(?:" + pattern + ")$")
		return err == nil && re.MatchString(name), err == nil
	}
	if full, valid := matches("**/" + strings.Join(segments, "/")); !valid || full {
		return !valid
	}
	for drop := 1; drop < len(segments); drop++ {
		if hit, valid := matches(strings.Join(segments[drop:], "/")); !valid || hit {
			return true
		}
	}
	return false
}

// playwrightInsideTestDir reports whether test is inside the project's testDir, or the config's
// when the project names none, as playwrightProjectOwns first requires.
func playwrightInsideTestDir(project PlaywrightProject, globalTestDir, test string) bool {
	directory := project.TestDir
	if directory == "" {
		directory = globalTestDir
	}
	return directory == "." || inside(test, directory)
}
