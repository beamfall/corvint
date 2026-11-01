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
func aliasFreeTests(observations []observation, bodies map[string]string, scopes []packageScope, opaque map[string]bool, excluded func(string) bool) map[string]bool {
	var aliases []typeScriptAliases
	for _, name := range sortedKeys(bodies) {
		if path.Base(name) == "tsconfig.json" {
			aliases = append(aliases, parseTypeScriptAliasesIn(name, bodies))
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
		if aliasFreeClosure(start.path, byPath, paths, aliases, bodies, scopes, opaque, excluded) {
			free[start.path] = true
		}
	}
	return free
}

func aliasFreeClosure(start string, byPath map[string]*observation, paths map[string]string, aliases []typeScriptAliases, bodies map[string]string, scopes []packageScope, opaque map[string]bool, excluded func(string) bool) bool {
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
				target, _, _ := resolveImport(current, ref, paths, excluded)
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
// the closed static profile resolves (testDir and projects, and a project testMatch or testIgnore
// that is only literal root-free string globs, TJAA-V0-094), and every file the profile assigns to a
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
	// matcherSite names the first key (testMatch or testIgnore) whose value refused reports, or
	// the first key when none does, so a refused second project names its own line (TJAA-V0-092,
	// TJAA-V0-094).
	matcherSite := func(key string, refused func(items []string) bool) (string, string, bool) {
		first := strings.Index(config.body, key)
		if first < 0 {
			// An escaped key such as "test\u004datch" parses to a key the raw body never names,
			// so the site is the escaped spelling's line (TJAA-V0-104).
			if at := playwrightKeyAt(config.body, key); at >= 0 {
				return lineSite(config.path, config.body, at), key, false
			}
			return config.path, key + " (no line: the config does not spell the key)", false
		}
		at := playwrightMatcherAt(config.body, key, refused)
		if at < 0 {
			at = first
		}
		return lineSite(config.path, config.body, at), sourceToken(config.body[at:]), false
	}
	notRootFree := func(items []string) bool {
		for _, item := range items {
			if !playwrightDecidableMatcher(item) {
				return true
			}
		}
		return len(items) == 0
	}
	parsedTestMatch, parsedTestIgnore := false, false
	for _, project := range projects {
		parsedTestMatch = parsedTestMatch || len(project.TestMatch) != 0
		parsedTestIgnore = parsedTestIgnore || len(project.TestIgnore) != 0 || len(project.ignores) != 0
	}
	if (testMatchAt >= 0 || parsedTestMatch) && !playwrightLiteralMatchers(config.body, "testMatch", projects, unknown) {
		return matcherSite("testMatch", notRootFree)
	}
	if unmodelled >= 0 {
		return lineSite(config.path, config.body, unmodelled), sourceToken(config.body[unmodelled:]), false
	}
	// A testIgnore, global or per project, that is not literal root-free string globs names its
	// own line before the membership unknown it leaves (TJAA-V0-094).
	if (strings.Contains(config.body, "testIgnore") || parsedTestIgnore) && !playwrightLiteralMatchers(config.body, "testIgnore", projects, unknown) {
		return matcherSite("testIgnore", notRootFree)
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
		at, why := playwrightUnknownAt(config.body, membership[0])
		if at < 0 {
			return config.path, membership[0].Reason + " (no line: " + why + ")", false
		}
		return lineSite(config.path, config.body, at), membership[0].Reason, false
	}
	if len(projects) == 0 {
		if at, _, _ := playwrightProjectEntries(config.body); at >= 0 {
			return lineSite(config.path, config.body, at), "projects", false
		}
		return config.path, "projects (no line: the config does not spell projects)", false
	}
	byPath := make(map[string]observation, len(observations))
	for _, observation := range observations {
		byPath[observation.path] = observation
	}
	for _, name := range files {
		owned, owner := false, PlaywrightProject{}
		for _, project := range projects {
			// Playwright matches the absolute path, so a glob longer than a path whose own
			// segments match the glob's tail is decided by the checkout's directories, and a path
			// outside the matcher model (a non-BMP character or a line break) can match in minimatch
			// where the Go matcher does not (TJAA-V0-091).
			for _, glob := range project.TestMatch {
				if playwrightInsideTestDir(project, globalTestDir, name) && (playwrightPathOutsideMatcherModel(name) || playwrightGlobReachesRoot(glob, name)) {
					return matcherSite("testMatch", func(items []string) bool { return slices.Contains(items, glob) })
				}
			}
			// A testIgnore glob on a path the project's testMatch holds stops only for a path outside
			// the matcher model (TJAA-V0-094). A match that needs directories above the repository
			// can only drop the path from the project, so the static membership, which keeps it, is
			// a superset of Playwright's and the glob stays decided (TJAA-V0-105).
			matched := project
			matched.ignores = nil
			for _, glob := range project.TestIgnore {
				if playwrightProjectOwns("/", matched, globalTestDir, name) && playwrightPathOutsideMatcherModel(name) {
					return matcherSite("testIgnore", func(items []string) bool { return slices.Contains(items, glob) })
				}
			}
			if playwrightProjectOwns("/", project, globalTestDir, name) {
				owned, owner = true, project
				break
			}
		}
		if !owned {
			continue
		}
		observation, observed := byPath[name]
		if !observed || !observation.test || observation.runner != runnerPlaywright || !containsString(observation.configs, config.path) {
			// The site is the owning project's testMatch, or its entry when it uses the
			// default matcher (TJAA-V0-104).
			key := ""
			if len(owner.TestMatch) != 0 {
				key = "testMatch"
			}
			if at := playwrightProjectKeyAt(config.body, owner.Name, key); at >= 0 {
				return lineSite(config.path, config.body, at), "owns " + name, false
			}
			return config.path, "owns " + name + " (no line: the owning project entry is not located)", false
		}
	}
	return "", "", true
}

// playwrightUnknownAt returns the offset in a config body of the construct a membership unknown
// names, so its executable-config site names a line (TJAA-V0-104): the key of a global or
// per-project detail (also when an escape spells it), the project entry a project-set detail
// refuses, the later entry of a duplicate project name, the `projects` key for a projects array
// the profile cannot read, and the `export default` of a dynamic config. When the body spells no
// such construct it returns -1 and why.
func playwrightUnknownAt(body string, entry PlaywrightUnknown) (int, string) {
	clean, err := stripComments(body, false)
	detail := entry.Detail
	if err != nil || detail == "config tokenization failed" {
		return -1, "the config does not tokenize, so no construct has a line"
	}
	switch detail {
	case "projects is absent":
		return -1, "projects is absent, so no key has a line"
	case "exported config is not one static object", "top-level config object is dynamic":
		if export := playwrightExportDefault.FindStringIndex(clean); export != nil {
			return export[0] + strings.Index(clean[export[0]:], "export"), ""
		}
		return -1, "the config has no export default"
	case "projects is not one nonempty static array", "no static project identity was recovered":
		if at, _, _ := playwrightProjectEntries(clean); at >= 0 {
			return at, ""
		}
		return -1, "the config does not spell projects"
	}
	projectsAt, items, offsets := playwrightProjectEntries(clean)
	nameOf := func(item string) (string, bool, bool) {
		properties, static := playwrightObjectProperties(item)
		if !static {
			return "", false, false
		}
		name, literal := playwrightString(properties["name"])
		return name, true, literal && name != ""
	}
	switch {
	case detail == "project entry is not a static object" || detail == "project name is not one nonempty string":
		for index, item := range items {
			_, static, named := nameOf(item)
			if detail == "project entry is not a static object" && !static || detail == "project name is not one nonempty string" && static && !named {
				return offsets[index], ""
			}
		}
	case strings.HasPrefix(detail, "duplicate project name "):
		seen := false
		for index, item := range items {
			if name, _, named := nameOf(item); named && name == strings.TrimPrefix(detail, "duplicate project name ") {
				if seen {
					return offsets[index], ""
				}
				seen = true
			}
		}
	case detail == "global testDir is not one static relative path" || strings.HasPrefix(detail, "global ") && strings.HasSuffix(detail, " is outside the static project subset"):
		// Only these two phrasings are global; a project named global reports its own keys.
		key := playwrightLeadingWord(strings.TrimPrefix(detail, "global "))
		end := projectsAt
		for index, item := range items {
			end = max(end, offsets[index]+len(item))
		}
		for _, at := range playwrightKeyOffsets(clean, key) {
			if projectsAt < 0 || at < projectsAt || at >= end {
				return at, ""
			}
		}
		return -1, "the config does not spell the global " + key + " key the unknown names"
	default:
		// A per-project detail starts with the longest project name it can name.
		owner, best := -1, ""
		for index, item := range items {
			if name, _, named := nameOf(item); named && len(name) > len(best) && strings.HasPrefix(detail, name+" ") {
				owner, best = index, name
			}
		}
		if owner >= 0 {
			key := playwrightLeadingWord(detail[len(best)+1:])
			if found := playwrightKeyOffsets(items[owner], key); len(found) != 0 {
				return offsets[owner] + found[0], ""
			}
			return -1, "the project " + best + " does not spell the " + key + " key the unknown names"
		}
		if key := playwrightLeadingWord(detail); key != "" {
			if found := playwrightKeyOffsets(clean, key); len(found) != 0 {
				return found[0], ""
			}
			return -1, "the config does not spell the " + key + " key the unknown names"
		}
	}
	return -1, "the config does not spell the construct the unknown names"
}

// playwrightExportDefault locates the `export default` of a config.
var playwrightExportDefault = regexp.MustCompile(`(?m)^[\t ]*export[\t ]+default\b`)

// playwrightLeadingWord returns the identifier that starts text, or "".
func playwrightLeadingWord(text string) string {
	end := 0
	for end < len(text) && isIdentifier(text[end]) {
		end++
	}
	return text[:end]
}

// playwrightKeyOffsets returns the offsets in text of key spelled as a token, or, when no raw
// token spells it, of the escaped spellings that decode to it.
func playwrightKeyOffsets(text, key string) []int {
	if key == "" {
		return nil
	}
	if found := playwrightTokenOffsets(text, key); len(found) != 0 {
		return found
	}
	decoded, raw := decodeStringEscapes(text)
	found := playwrightTokenOffsets(decoded, key)
	for index, at := range found {
		found[index] = raw(at)
	}
	return found
}

// playwrightKeyAt returns the offset of the first spelling of key in a comment-blanked config
// body, raw or escaped, or -1 (TJAA-V0-104).
func playwrightKeyAt(body, key string) int {
	if clean, err := stripComments(body, false); err == nil {
		body = clean
	}
	if found := playwrightKeyOffsets(body, key); len(found) != 0 {
		return found[0]
	}
	return -1
}

// playwrightProjectEntries returns, for a config body, the offset of its top-level `projects`
// key, and the items of its projects array with each item's offset, located in order inside the
// array; the offset is -1 and the items nil when the body has no static projects key.
func playwrightProjectEntries(body string) (int, []string, []int) {
	if clean, err := stripComments(body, false); err == nil {
		body = clean
	}
	object, ok := playwrightConfigObject(body)
	if !ok {
		return -1, nil, nil
	}
	properties, ok := playwrightObjectProperties(object)
	raw, exists := properties["projects"]
	if !ok || !exists {
		return -1, nil, nil
	}
	keyAt := -1
	for _, at := range playwrightKeyOffsets(body, "projects") {
		rest, index := body[at+len("projects"):], 0
		if index < len(rest) && (rest[index] == '\'' || rest[index] == '"') {
			index++
		}
		if index = playwrightNextCode(rest, index); index < len(rest) && rest[index] == ':' {
			keyAt = at
			break
		}
	}
	if keyAt < 0 {
		return -1, nil, nil
	}
	valueAt := strings.Index(body[keyAt:], raw)
	items, ok := playwrightArrayItems(raw)
	if valueAt < 0 || !ok {
		return keyAt, nil, nil
	}
	cursor, offsets, located := keyAt+valueAt+1, []int{}, []string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		found := strings.Index(body[cursor:], trimmed)
		if trimmed == "" || found < 0 {
			continue
		}
		offsets, located = append(offsets, cursor+found), append(located, trimmed)
		cursor += found + len(trimmed)
	}
	return keyAt, located, offsets
}

// playwrightProjectKeyAt returns the offset of key in the projects entry named name, or of the
// entry itself when key is empty or the entry does not spell it, or -1 (TJAA-V0-104).
func playwrightProjectKeyAt(body, name, key string) int {
	_, items, offsets := playwrightProjectEntries(body)
	for index, item := range items {
		properties, ok := playwrightObjectProperties(item)
		if found, literal := playwrightString(properties["name"]); !ok || !literal || found != name {
			continue
		}
		if found := playwrightKeyOffsets(item, key); key != "" && len(found) != 0 {
			return offsets[index] + found[0]
		}
		return offsets[index]
	}
	return -1
}

// playwrightLiteralMatchers reports whether every key (`testMatch` or `testIgnore`) in a config
// body is one parsed project's key whose items are all playwrightDecidableMatcher items
// (escape-free root-free string globs or static regular-expression literals): the parse left no
// unknown about the key, and the body names the key exactly once for each project that has one,
// so no global, nested, computed, commented or spread key is left unaccounted for. A regular
// expression that is not static, a computed value or a glob with a brace, a class, an extglob or
// a globstar that is not a whole segment is not literal (TJAA-V0-091, TJAA-V0-092, TJAA-V0-094,
// TJAA-V0-127).
func playwrightLiteralMatchers(body, key string, projects []PlaywrightProject, unknown []PlaywrightUnknown) bool {
	for _, entry := range unknown {
		if strings.Contains(entry.Detail, key) {
			return false
		}
	}
	matching, global := 0, 0
	for _, project := range projects {
		items := project.TestMatch
		if key == "testIgnore" {
			items = project.TestIgnore
		}
		if len(items) == 0 {
			continue
		}
		// An inherited global testIgnore is the body's one global key (TJAA-V0-770).
		if key == "testIgnore" && project.ignoreInherited {
			global = 1
		} else {
			matching++
		}
		for _, raw := range items {
			if !playwrightDecidableMatcher(raw) {
				return false
			}
		}
	}
	return matching+global != 0 && matching+global == strings.Count(body, key)
}

// playwrightLiteralProjects parses a Playwright config and returns its projects and global
// testDir when every testMatch in it is a literal root-free string glob
// (playwrightLiteralMatchers), or nil (TJAA-V0-092).
func playwrightLiteralProjects(path, body string) ([]PlaywrightProject, string) {
	projects, globalTestDir, unknown := parsePlaywrightConfig(path, body)
	if !playwrightLiteralMatchers(body, "testMatch", projects, unknown) {
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
			if len(project.TestMatch) != 0 && playwrightProjectOwns("/", project, config.literalTestDir, relative) {
				return true
			}
		}
	}
	return false
}

// playwrightMatcherAt returns the offset of the first key (`testMatch` or `testIgnore`) in body
// whose value refused reports, given its items (one item for a value that is not an array
// literal, none for a value that does not parse), or -1 when none does (TJAA-V0-092, TJAA-V0-094).
func playwrightMatcherAt(body, key string, refused func(items []string) bool) int {
	// Blanking comments keeps offsets, so `testMatch: /* note */ 'glob'` yields the parsed glob.
	if clean, err := stripComments(body, false); err == nil {
		body = clean
	}
	for _, at := range playwrightTokenOffsets(body, key) {
		rest := body[at+len(key):]
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

// playwrightDecidableMatcher reports a project testMatch or testIgnore item the static profile
// decides per path: a playwrightRootFreeGlob glob, or a static regular-expression literal with no
// start anchor or word boundary, whose match on `/<path>` is a match on the absolute path
// (TJAA-V0-127). A testMatch regex is then decided per path by PlaywrightRegexReachesRoot; a
// testIgnore regex that matches only through a checkout directory can only drop the path, so the
// static membership stays a superset (TJAA-V0-105). Ownership is evaluated on `/<path>`, which a
// glob's leading `**/` matches as it matches `<path>`.
func playwrightDecidableMatcher(raw string) bool {
	if _, ok := playwrightRootFreeGlob(raw); ok {
		return true
	}
	return PlaywrightRegexContextFree(raw)
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
	if strings.HasPrefix(strings.TrimSpace(raw), "/") {
		return PlaywrightRegexReachesRoot(raw, name)
	}
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
