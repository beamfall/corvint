package typescript

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// playwrightTagSources decides whether the tags a listing reports for a test file are exactly
// the tags of its grep titles, from the config, the test file and their repository-relative static
// import closure. Playwright reports a config or describe tag only as its `@\S+` runs, so the
// listing alone cannot show a tag that contains whitespace or is `@` alone; when every tag in the
// closure is a literal `@\S+` string the listing hides nothing. The literal tags are returned so the
// caller can also require each reported tag to be one of them (AFP-V0-409, AFP-V0-410). Every
// file of the proof must be in the source set the discovery receipt binds, so a change to any of
// them invalidates the receipt (AFP-V0-409).
type playwrightTagSources struct {
	root, configPath string
	bound            map[string]bool
	files            map[string]playwrightTagFile
	// names holds every bound file name (with no body) for alias resolution, and aliases the
	// parsed alias configs among them: each tsconfig.json, and each jsconfig.json with no
	// tsconfig.json beside it, which Playwright reads first (AFP-V0-418).
	names   map[string]string
	aliases []typeScriptAliases
}

// playwrightTagFile is one checked file. stop, when the file is not exact, says why: the first
// import it cannot follow, or that its text is not literal (AFP-V0-419).
type playwrightTagFile struct {
	exact   bool
	imports []string
	tags    []string
	stop    string
	// fresh records that the file indexes a split or map result by a computed member, and
	// freshStop, when not empty, names where the file defines or reads `split` or `map` other than
	// as a direct method call; the closure stops on freshStop once any file is fresh (TJAA-V0-093).
	fresh     bool
	freshStop string
	// objectWrite records that the file writes or reads a computed member of a fresh object it
	// keeps (playwrightFreshObject, TJAA-V0-095 to TJAA-V0-099), and objectSink that such an
	// object reaches a `.fulfill(...)` argument. selfStop names the file's first `this`, `super` or
	// accessor definition, and sinkStop its first `fulfill` other than a direct method call; the
	// closure stops on selfStop once any file has an object write, and on sinkStop once any file
	// has an object sink (TJAA-V0-095, TJAA-V0-096).
	objectWrite, objectSink bool
	selfStop, sinkStop      string
}

func newPlaywrightTagSources(root, configPath string) *playwrightTagSources {
	return &playwrightTagSources{root: root, configPath: configPath, files: map[string]playwrightTagFile{}}
}

// boundFiles returns the files ObservePlaywrightSources digests into the receipt's source binding,
// or nil when they cannot be listed.
func (sources *playwrightTagSources) boundFiles() map[string]bool {
	if sources.bound == nil {
		files, _, err := playwrightBoundSourceFiles(sources.root)
		if err != nil {
			return nil
		}
		sources.bound = map[string]bool{sources.configPath: true}
		sources.names = map[string]string{sources.configPath: ""}
		for _, file := range files {
			sources.bound[file] = true
			sources.names[file] = ""
		}
		// Playwright reads the nearest folder holding tsconfig.json or jsconfig.json, and
		// tsconfig.json first within one folder (AFP-V0-418).
		// The other tsconfig*.json and jsconfig*.json files are bodies an `extends` chain may
		// name, never alias configs of their own (TJAA-V0-114).
		bodies := map[string]string{}
		var configs []string
		for _, file := range files {
			base := path.Base(file)
			nearest := base == "tsconfig.json" || base == "jsconfig.json" && !sources.bound[path.Join(path.Dir(file), "tsconfig.json")]
			if !typeScriptConfigName(base) && !javaScriptConfigName(base) {
				continue
			}
			if body, err := affected.ReadSource(sources.root, file); err == nil && utf8.Valid(body) {
				bodies[file] = string(body)
			} else {
				bodies[file] = "" // an unreadable config parses as invalid and blocks its aliases
			}
			if nearest {
				configs = append(configs, file)
			}
		}
		// A config naming `tsconfig` can point Playwright at another file than the nearest one, so
		// no alias is followed, as TJAA-V0-037 refuses for the dependency graph.
		config, err := affected.ReadSource(sources.root, sources.configPath)
		if err == nil && len(playwrightTokenOffsets(string(config), "tsconfig")) == 0 {
			slices.Sort(configs)
			for _, name := range configs {
				sources.aliases = append(sources.aliases, parseTypeScriptAliasesIn(name, bodies))
			}
		}
	}
	return sources.bound
}

// exact returns the literal tags of every file reachable from entries when each is in the bound
// source set and passes playwrightTagFileExact, or nil when one does not.
func (sources *playwrightTagSources) exact(entries ...string) map[string]bool {
	tags, _ := sources.exactOrStop(entries...)
	return tags
}

// exactOrStop is exact that also returns, when the tags are nil, where the closure stopped
// (AFP-V0-419).
func (sources *playwrightTagSources) exactOrStop(entries ...string) (map[string]bool, string) {
	bound := sources.boundFiles()
	if bound == nil {
		return nil, "the bound source set cannot be listed"
	}
	tags := map[string]bool{}
	seen := map[string]bool{}
	fresh, freshStop := false, ""
	objectWrite, objectSink, selfStop, sinkStop := false, false, "", ""
	queue := slices.Clone(entries)
	for len(queue) != 0 {
		file := queue[0]
		queue = queue[1:]
		if seen[file] {
			continue
		}
		seen[file] = true
		if !bound[file] {
			return nil, file + " is outside the bound sources"
		}
		checked, ok := sources.files[file]
		if !ok {
			checked = playwrightTagFileExact(sources.root, file, sources.resolveAlias)
			sources.files[file] = checked
		}
		if !checked.exact {
			return nil, checked.stop
		}
		for _, tag := range checked.tags {
			tags[tag] = true
		}
		fresh = fresh || checked.fresh
		if freshStop == "" {
			freshStop = checked.freshStop
		}
		objectWrite, objectSink = objectWrite || checked.objectWrite, objectSink || checked.objectSink
		if selfStop == "" {
			selfStop = checked.selfStop
		}
		if sinkStop == "" {
			sinkStop = checked.sinkStop
		}
		queue = append(queue, checked.imports...)
	}
	if fresh && freshStop != "" {
		return nil, freshStop
	}
	if objectWrite && selfStop != "" {
		return nil, selfStop
	}
	if objectSink && sinkStop != "" {
		return nil, sinkStop
	}
	return tags, ""
}

// resolveAlias resolves a bare specifier of owner through the nearest tsconfig.json or
// jsconfig.json paths or baseUrl to the one bound repository file it names, as the dependency
// graph does (TJAA-V0-037). A specifier the nearest config does not claim, a config that is not
// valid, an owner that is not .ts or .tsx under a config without allowJs (Playwright then maps
// nothing), a Playwright config that names `tsconfig`, and an ambiguous or missing target all
// leave it unresolved (AFP-V0-418).
func (sources *playwrightTagSources) resolveAlias(owner, ref string) (string, bool) {
	nearest := nearestTypeScriptAliases(owner, sources.aliases)
	typeScript := strings.HasSuffix(owner, ".ts") || strings.HasSuffix(owner, ".tsx")
	if nearest == nil || !nearest.valid || !typeScript && !nearest.allowJS || externalScheme(ref) {
		return "", false
	}
	target, _ := nearest.resolve(ref, sources.names)
	return target, target != "" && sources.bound[target]
}

// playwrightTagNeutralModules are bare modules whose import gives a case no tag.
var playwrightTagNeutralModules = map[string]bool{"@playwright/test": true, "dotenv": true, "dotenv/config": true, "path": true, "os": true, "url": true, "fs": true}

// playwrightTagIndirection are names through which a module can build, rename or enumerate a
// property at run time, so a tag could exist without the text `tag` in source: the reflective
// globals, the routes that reach them without naming them (`constructor`, `prototype`,
// `__proto__`, the global object aliases), and the enumerating and defining helpers. Any such name
// in code keeps the file inexact (AFP-V0-409). The in-place array mutators (`push`, `fill` and the
// like) are not among them: every literal tag array is the value of a literal `tag` key, so the
// only routes to it name `tag` outside a key, index it by a computed member, or pass one of these
// names, and each of those is already inexact (AFP-V0-425). The one exempt use is a plain
// `JSON.stringify` call (playwrightPlainStringify, TJAA-V0-090).
var playwrightTagIndirection = []string{
	"Object", "Reflect", "JSON", "Proxy", "eval", "Function", "globalThis", "global", "window", "self", "structuredClone",
	"constructor", "prototype", "__proto__", "__defineGetter__", "__defineSetter__",
	"values", "entries", "assign", "defineProperty", "defineProperties", "fromEntries", "getOwnPropertyNames", "getOwnPropertyDescriptor", "getOwnPropertyDescriptors", "setPrototypeOf",
}

// playwrightTagFileExact checks one file. Its static loads must all resolve: a relative import to
// one repository file, or a bare specifier that alias resolves to one through tsconfig paths or
// baseUrl (AFP-V0-418), is followed; any other bare module than @playwright/test, dotenv or a
// neutral Node built-in, an unresolved import, or a dynamic load is not exact, and stop names the
// first such specifier (AFP-V0-419). Every `tag` in its code must be an object-literal property key whose
// value is a literal string or an array of literal strings, each `@` followed by one or more
// non-whitespace characters without an escape. A string literal "tag", an escaped identifier or
// string property key, a computed object key, a computed member assignment or access or a name in
// playwrightTagIndirection is not exact (AFP-V0-409).
func playwrightTagFileExact(root, relative string, alias func(owner, ref string) (string, bool)) playwrightTagFile {
	body, err := affected.ReadSource(root, relative)
	if err != nil || !utf8.Valid(body) {
		return playwrightTagFile{stop: relative + " is not readable as bounded UTF-8 source"}
	}
	scan := scanImportSites(relative, string(body))
	if scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 || scan.syntax >= 0 {
		return playwrightTagFile{stop: relative + " has a module load that is not static"}
	}
	result := playwrightTagFile{exact: true}
	for _, ref := range scan.refs {
		if playwrightTagNeutralModules[ref] || strings.HasPrefix(ref, "node:") {
			continue
		}
		resolved, ok := playwrightResolveRelative(root, relative, ref)
		if !ok && !strings.HasPrefix(ref, ".") && alias != nil {
			resolved, ok = alias(relative, ref)
		}
		if !ok {
			return playwrightTagFile{stop: relative + " imports " + strconv.Quote(ref) + ", which resolves to no single repository file"}
		}
		result.imports = append(result.imports, resolved)
	}
	clean, err := stripComments(string(body), jsxSource(relative))
	if err != nil {
		return playwrightTagFile{stop: relative + " does not tokenize"}
	}
	stop := func(at int, why string) string {
		return relative + " holds a tag or construct that is not one literal @\\S+ string (line " + strconv.Itoa(lineAt(clean, at)) + ": " + why + ")"
	}
	literal := playwrightTagTextLiteral(clean)
	if literal.why != "" {
		// The stop names the first construct and its line, so an owner can see which one keeps the
		// closure inexact (AFP-V0-425).
		return playwrightTagFile{stop: stop(literal.at, literal.why)}
	}
	result.tags, result.fresh = literal.tags, literal.fresh
	if literal.defines >= 0 {
		result.freshStop = stop(literal.defines, "`"+literal.defined+"` is defined or read other than as a direct method call, so a computed index on a split or map result can reach a tag array")
	}
	result.objectWrite, result.objectSink = literal.objectWrite, literal.objectSink
	switch {
	case literal.self >= 0 && (literal.selfName == "get" || literal.selfName == "set"):
		result.selfStop = stop(literal.self, "`"+literal.selfName+"` defines an accessor that a computed member access on a fresh object can run, so a value it reads or writes can reach a test without its name")
	case literal.self >= 0:
		result.selfStop = stop(literal.self, "`"+literal.selfName+"` can bind the fresh object a computed member write sets, so its value can reach a test without its name")
	}
	if literal.sinks >= 0 {
		result.sinkStop = stop(literal.sinks, "`fulfill` is defined or read other than as a direct method call, so a fresh object a computed member write sets can reach a test through it")
	}
	return result
}

// playwrightResolveRelative resolves a relative static import of owner to the one existing source
// file inside root among the candidates the import graph tries. When two candidates exist, such as
// tags.cjs and tags.js, the module Playwright loads depends on its resolver's order, so the import
// is not resolved (AFP-V0-409).
func playwrightResolveRelative(root, owner, ref string) (string, bool) {
	clean := strings.SplitN(strings.SplitN(ref, "?", 2)[0], "#", 2)[0]
	if !strings.HasPrefix(clean, ".") {
		return "", false
	}
	base := path.Clean(path.Join(path.Dir(owner), clean))
	if base == "." || base == ".." || strings.HasPrefix(base, "../") {
		return "", false
	}
	found := ""
	for _, candidate := range moduleCandidates(base) {
		if !affected.ValidRelativePath(candidate) || candidate == found {
			continue
		}
		_, err := affected.ReadSource(root, candidate)
		switch {
		case err == nil && found != "":
			return "", false
		case err == nil:
			found = candidate
		case !errors.Is(err, fs.ErrNotExist):
			return "", false
		}
	}
	return found, found != ""
}

// playwrightTagLiteral is the per-file verdict of playwrightTagTextLiteral. When the module is
// not literal, why names the first rule it breaks and at is that construct's offset (AFP-V0-425).
// fresh records a computed index on a split or map result, and defines is the offset of the
// first `split` or `map` (named by defined) that is not a direct method call, or -1 (TJAA-V0-093).
// objectWrite and objectSink record a computed write or read on a fresh object and such an
// object reaching a `.fulfill(...)` argument (TJAA-V0-095 to TJAA-V0-099); self is the offset of
// the first `this`, `super` or accessor definition (named by selfName) and sinks of the first
// `fulfill` that is not a direct method call, or -1 (TJAA-V0-095, TJAA-V0-096).
type playwrightTagLiteral struct {
	tags                    []string
	at                      int
	why                     string
	fresh                   bool
	defines                 int
	defined                 string
	objectWrite, objectSink bool
	self, sinks             int
	selfName                string
}

// playwrightFreshMethods are the methods whose direct call returns a new array that no tag key
// holds: String.prototype.split, whose items are strings, and Array.prototype.map, whose items
// are the callback's values (TJAA-V0-093).
var playwrightFreshMethods = []string{"split", "map"}

// playwrightTagTextLiteral applies the per-file text rules of playwrightTagFileExact to a
// comment-stripped module and returns its literal tags.
func playwrightTagTextLiteral(clean string) playwrightTagLiteral {
	fresh, objectWrite, objectSink := false, false, false
	objects := map[int][2]bool{}
	written := map[int]bool{}
	lexed, ok := playwrightLex(clean)
	if !ok {
		return playwrightTagLiteral{at: 0, why: "the module does not lex", defines: -1}
	}
	first, name := -1, ""
	var opens map[int]int
	// `Object.entries(NAME)` over a fresh object playwrightFreshObject admits enumerates only the
	// values the file wrote into it (TJAA-V0-117).
	enumerated := map[int]bool{}
	for _, offset := range playwrightTokenOffsets(clean, "Object") {
		if !lexed.code[offset] {
			continue
		}
		member, argument := playwrightEntriesCall(clean, lexed, offset)
		if argument < 0 {
			continue
		}
		declaration := playwrightFreshDeclarationFor(clean, lexed, argument)
		if declaration < 0 {
			continue
		}
		if _, checked := objects[declaration]; !checked {
			admitted, sink := playwrightFreshObject(clean, lexed, declaration)
			objects[declaration] = [2]bool{admitted, sink}
		}
		if objects[declaration][0] {
			enumerated[offset], enumerated[member] = true, true
			objectWrite, objectSink = true, objectSink || objects[declaration][1]
		}
	}
	for _, candidate := range playwrightTagIndirection {
		for _, offset := range playwrightTokenOffsets(clean, candidate) {
			if lexed.code[offset] && !enumerated[offset] && (first < 0 || offset < first) && (candidate != "JSON" || !playwrightPlainStringify(clean, lexed, offset)) {
				first, name = offset, candidate
			}
		}
	}
	if first >= 0 {
		return playwrightTagLiteral{at: first, why: "`" + name + "` can build or enumerate a property at run time", defines: -1}
	}
	for _, quoted := range []string{"'tag'", `"tag"`, "`tag`"} {
		if offset := strings.Index(clean, quoted); offset >= 0 {
			return playwrightTagLiteral{at: offset, why: "the string " + quoted + " can name a tag key", defines: -1}
		}
	}
	for index := 0; index < len(clean); index++ {
		if !lexed.code[index] {
			if quote := clean[index]; (quote == '\'' || quote == '"') && (index == 0 || lexed.code[index-1]) {
				end, ok := playwrightQuotedEnd(clean, index)
				if !ok {
					return playwrightTagLiteral{at: index, why: "a string does not close", defines: -1}
				}
				if next := playwrightNextCode(clean, end); strings.ContainsRune(clean[index:end], '\\') && next < len(clean) && clean[next] == ':' {
					return playwrightTagLiteral{at: index, why: "an escaped string property key can spell tag", defines: -1}
				} else if strings.ContainsRune(clean[index:end], '\\') && next < len(clean) && clean[next] == '(' {
					// A method `'\x66ulfill'(o) {}` defines a method whose name the closure's
					// split, map and fulfill guards cannot read (TJAA-V0-093, TJAA-V0-095).
					return playwrightTagLiteral{at: index, why: "an escaped string method name can spell a method the closure trusts", defines: -1}
				}
				index = end - 1
			}
			continue
		}
		switch clean[index] {
		case '\\':
			return playwrightTagLiteral{at: index, why: "an identifier escape can spell tag", defines: -1}
		case '[':
			previous := playwrightPreviousCode(clean, index)
			closeAt, closed := lexed.closes[index]
			if !closed {
				return playwrightTagLiteral{at: index, why: "a bracket does not close", defines: -1}
			}
			next := playwrightNextCode(clean, closeAt+1)
			if previous >= 0 && (clean[previous] == '{' || clean[previous] == ',') && next < len(clean) && (clean[next] == ':' || clean[next] == '(') {
				return playwrightTagLiteral{at: index, why: "a computed object key or method can name a tag key", defines: -1}
			}
			// `}` ends an object literal as well as a block, so `{ tag: [...] }[k]` indexes the
			// literal; a block followed by an array literal stops too, conservatively. A literal
			// index, including the empty `[]` of an array type, stays exact (TJAA-V0-103).
			if previous >= 0 && clean[previous] == '}' && !playwrightLiteralIndex(clean[index+1:closeAt]) {
				return playwrightTagLiteral{at: index, why: "a `[` after `}` can index an object literal and reach a tag array without naming it", defines: -1}
			}
			if playwrightMemberAccessAt(clean, previous) && !playwrightLiteralIndex(clean[index+1:closeAt]) {
				if opens == nil {
					opens = make(map[int]int, len(lexed.closes))
					for open, close := range lexed.closes {
						opens[close] = open
					}
				}
				literal, method := playwrightFreshArrayAt(clean, opens, previous)
				if !literal && !method {
					declaration := -1
					if receiver := playwrightComputedReceiverAt(clean, previous); receiver >= 0 {
						declaration = playwrightFreshDeclarationFor(clean, lexed, receiver)
					}
					if declaration >= 0 {
						if _, checked := objects[declaration]; !checked {
							admitted, sink := playwrightFreshObject(clean, lexed, declaration)
							objects[declaration] = [2]bool{admitted, sink}
						}
					}
					if declaration < 0 || !objects[declaration][0] {
						return playwrightTagLiteral{at: index, why: "a computed member access can reach a tag array without naming it", defines: -1}
					}
					objectWrite, objectSink = true, objectSink || objects[declaration][1]
					written[closeAt] = true
				}
				fresh = fresh || method
			}
		case ']':
			// `T[] =` is a type annotation; any other `] =` writes a computed member, unless it
			// writes a fresh object playwrightFreshObject admits (TJAA-V0-095, TJAA-V0-097).
			if previous := playwrightPreviousCode(clean, index); !written[index] && (previous < 0 || clean[previous] != '[') && playwrightAssignmentAt(clean, playwrightNextCode(clean, index+1)) {
				return playwrightTagLiteral{at: index, why: "a computed member write can set a tag", defines: -1}
			}
		}
	}
	tags := []string{}
	for _, offset := range playwrightTokenOffsets(clean, "tag") {
		if !lexed.code[offset] {
			continue
		}
		literal, ok := playwrightLiteralTagProperty(clean, lexed, offset)
		if !ok {
			return playwrightTagLiteral{at: offset, why: "`tag` is not an object-literal key whose value is literal @\\S+ strings", defines: -1}
		}
		tags = append(tags, literal...)
	}
	defines, defined := playwrightMethodDefinition(clean, lexed, playwrightFreshMethods)
	sinks, _ := playwrightMethodDefinition(clean, lexed, []string{"fulfill"})
	self, selfName := playwrightSelfReference(clean, lexed)
	if accessor, accessorName := playwrightAccessorDefinition(clean, lexed); accessor >= 0 && (self < 0 || accessor < self) {
		self, selfName = accessor, accessorName
	}
	return playwrightTagLiteral{tags: tags, fresh: fresh, defines: defines, defined: defined, objectWrite: objectWrite, objectSink: objectSink, self: self, selfName: selfName, sinks: sinks}
}

// playwrightComputedReceiverAt returns the start of the plain identifier that ends at previous,
// the code byte before the `[` of a computed member access, or -1 when the receiver is not one: a
// property name after `.`, a number, or no identifier at all (TJAA-V0-095, TJAA-V0-096).
func playwrightComputedReceiverAt(clean string, previous int) int {
	if previous < 0 || !playwrightIdentifierByte(clean[previous]) {
		return -1
	}
	start := previous
	for start > 0 && playwrightIdentifierByte(clean[start-1]) {
		start--
	}
	if before := playwrightPreviousCode(clean, start); before >= 0 && clean[before] == '.' || clean[start] >= '0' && clean[start] <= '9' {
		return -1
	}
	return start
}

// playwrightPlainAssignmentAt reports whether a plain `=`, not `==`, `===` or `=>`, starts at index.
func playwrightPlainAssignmentAt(raw string, index int) bool {
	rest := raw[min(index, len(raw)):]
	return strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") && !strings.HasPrefix(rest, "=>")
}

// playwrightIdentifierAt returns the identifier that starts at offset.
func playwrightIdentifierAt(clean string, offset int) string {
	end := offset
	for end < len(clean) && playwrightIdentifierByte(clean[end]) {
		end++
	}
	return clean[offset:end]
}

// playwrightBlockDeclaration reports whether the code occurrence of a name at offset is the name
// of a `const` or `let` declaration placed directly in a block, a function body or the module,
// and returns that scope's range: the open and close of its `{`, or -1 and len(clean) for the
// module. A declaration in a `for (...)` header, a destructuring pattern, a declaration list after
// its first name, and every `var` are not block declarations (TJAA-V0-096).
func playwrightBlockDeclaration(clean string, lexed playwrightLexed, offset int) (open, close int, ok bool) {
	if !lexed.code[offset] {
		return 0, 0, false
	}
	if word := playwrightWordBefore(clean, playwrightPreviousCode(clean, offset)); word != "const" && word != "let" {
		return 0, 0, false
	}
	open = lexed.parent[offset]
	if open < 0 {
		return -1, len(clean), true
	}
	close, closed := lexed.closes[open]
	return open, close, clean[open] == '{' && closed
}

// playwrightFreshDeclarationFor returns the offset of the declaration that binds the receiver name
// starting at receiver: the innermost `const` or `let` block declaration of that name whose scope
// holds the receiver, or -1 when there is none. Lexical scope makes it the binding the receiver
// names unless an inner parameter, `var`, function, class, catch or destructuring binding shadows
// it, and playwrightFreshObject refuses every such binding inside the scope (TJAA-V0-096).
func playwrightFreshDeclarationFor(clean string, lexed playwrightLexed, receiver int) int {
	name := playwrightIdentifierAt(clean, receiver)
	found, innermost := -1, -2
	for _, at := range playwrightTokenOffsets(clean, name) {
		if open, close, ok := playwrightBlockDeclaration(clean, lexed, at); ok && open < receiver && receiver < close && open > innermost {
			found, innermost = at, open
		}
	}
	return found
}

// playwrightFreshObject reports whether the declaration at declaration binds a fresh object whose
// value the file provably keeps from every test and describe title, tag and details argument, so
// a computed member access on it cannot set or reach a tag that reaches one, and whether one of
// its uses reaches a `.fulfill(...)` argument. This is a small escape analysis over the binding's
// lexical scope (TJAA-V0-096): the declaration is `const NAME = { ... }` or `let NAME = { ... }`
// (with at most a simple type annotation, not exported), so the value is a new object no other
// binding holds, and every code occurrence of NAME in the declaration's scope, outside a nested
// block that declares its own `const` or `let` NAME, is a proven non-flowing use:
//   - a property name after `.`, or an object-literal key, which names no binding;
//   - a plain computed write `NAME[...] = value` (TJAA-V0-095);
//   - a compound computed write `NAME[...] op= value` in statement position, whose value the
//     statement discards (playwrightStatementStart, TJAA-V0-097);
//   - a computed read `NAME[...]` that only an equality operator, `!`, `typeof`, or the test of an
//     `if` or `while` consumes, so only a boolean or a type string leaves it
//     (playwrightConsumedRead, TJAA-V0-098);
//   - the right operand of `in` (a for-in subject or a key test, whose value is a key or a
//     boolean);
//   - the first argument of a plain `JSON.stringify(...)` call, whose value is a string
//     (playwrightPlainStringify, TJAA-V0-099);
//   - a shorthand property, property value or spread of object literals nested directly in the
//     first argument of a direct `.fulfill(...)` call, which Playwright's Route.fulfill serializes
//     into a response, and which no assignment destructures (TJAA-V0-095).
//
// Any other use fails closed: a read, a call argument, an alias, a return, a template substitution,
// a parameter, a reassignment or another declaration of the name. A `with` statement anywhere in
// the file can change which binding a name resolves to, so it refuses every fresh object. The
// written value, a setter or a toJSON method can still reach the object through `this` or `super`,
// and a `fulfill` the closure defines can be called in place of Route.fulfill, so the closure keeps
// the rule only while it holds neither (playwrightSelfReference, playwrightMethodDefinition,
// TJAA-V0-095).
func playwrightFreshObject(clean string, lexed playwrightLexed, declaration int) (admitted, sink bool) {
	open, close, ok := playwrightBlockDeclaration(clean, lexed, declaration)
	if !ok || playwrightWithStatement(clean, lexed) {
		return false, false
	}
	name := playwrightIdentifierAt(clean, declaration)
	keyword := playwrightPreviousCode(clean, declaration)
	if !playwrightFreshObjectDeclaration(clean, lexed, declaration+len(name), keyword) || playwrightNonASCIINeighbour(clean, name) {
		return false, false
	}
	// A nested block that declares its own NAME binds every occurrence of NAME inside it, so those
	// occurrences cannot name this object.
	shadows := [][2]int{}
	offsets := playwrightTokenOffsets(clean, name)
	for _, at := range offsets {
		if inner, innerClose, ok := playwrightBlockDeclaration(clean, lexed, at); ok && at != declaration && inner > open && innerClose < close {
			shadows = append(shadows, [2]int{inner, innerClose})
		}
	}
	for _, at := range offsets {
		if !lexed.code[at] || at <= open || at >= close || at == declaration || slices.ContainsFunc(shadows, func(scope [2]int) bool { return scope[0] < at && at < scope[1] }) {
			continue
		}
		previous := playwrightPreviousCode(clean, at)
		next := playwrightNextCode(clean, at+len(name))
		before := ""
		if previous >= 0 {
			before = string(clean[previous])
		}
		after := ""
		if next < len(clean) {
			after = string(clean[next])
		}
		parentObject := lexed.parent[at] >= 0 && clean[lexed.parent[at]] == '{'
		switch {
		case before == "." && (previous == 0 || clean[previous-1] != '.'):
			// A property name such as `body.settings` names no binding.
		case (before == "{" || before == ",") && after == ":" && parentObject:
			// An object-literal key names no binding.
		case parentObject && (before == "{" || before == "," || before == ";") && playwrightTypeMemberKey(clean, next):
			// A type-literal or interface member key `NAME?:`, or `NAME:` after `;`, names no
			// binding; in a block, `NAME:` after `;` is a label (TJAA-V0-116).
		case before == "(" && after == ")" && playwrightEntriesArgument(clean, lexed, previous):
			// `Object.entries(NAME)` returns new arrays of keys and the values the file wrote,
			// none of which can be a tag array (TJAA-V0-117).
			if !playwrightFreshObjectWithoutSpread(clean, lexed, declaration+len(name)) {
				return false, false
			}
		case after == "[":
			closeAt, closed := lexed.closes[next]
			if !closed {
				return false, false
			}
			operator := playwrightNextCode(clean, closeAt+1)
			switch {
			case playwrightPlainAssignmentAt(clean, operator):
			case playwrightAssignmentAt(clean, operator):
				if !playwrightStatementStart(clean, lexed, at) {
					return false, false
				}
			case !playwrightConsumedRead(clean, lexed, at, operator):
				return false, false
			}
		case playwrightWordBefore(clean, previous) == "in":
			if !playwrightInOperandEnd(clean, next) {
				return false, false
			}
		case before == "(" && playwrightStringifyArgument(clean, lexed, previous, next):
			// `JSON.stringify(NAME)` returns a string that only a `.fulfill(...)` argument
			// receives (TJAA-V0-099).
			sink = true
		case (after == "," || after == "}") && parentObject && (before == "{" || before == "," || before == ":" || before == "." && previous >= 2 && clean[previous-2:previous+1] == "..."):
			if !playwrightFulfillArgument(clean, lexed, lexed.parent[at]) {
				return false, false
			}
			sink = true
		default:
			return false, false
		}
	}
	return true, sink
}

// playwrightTypeMemberKey reports whether the code at next, after a name in an object-literal
// or type-literal position, makes the name a type member key: `?` followed by `:`, which no
// expression can follow a name with, or `:` (the caller admits it only after `;`, where it is a
// type member key or a label) (TJAA-V0-116).
func playwrightTypeMemberKey(clean string, next int) bool {
	if next >= len(clean) {
		return false
	}
	if clean[next] == ':' {
		return true
	}
	colon := playwrightNextCode(clean, next+1)
	return clean[next] == '?' && colon < len(clean) && clean[colon] == ':'
}

// playwrightEntriesCall returns, for the `Object` token at offset, the offsets of `entries` and
// of the identifier NAME when the token begins a direct call `Object.entries(NAME)` whose only
// argument is that plain identifier, or -1, -1 (TJAA-V0-117).
func playwrightEntriesCall(clean string, lexed playwrightLexed, offset int) (member, argument int) {
	if previous := playwrightPreviousCode(clean, offset); previous >= 0 && clean[previous] == '.' {
		return -1, -1
	}
	dot := playwrightNextCode(clean, offset+len("Object"))
	if dot >= len(clean) || clean[dot] != '.' {
		return -1, -1
	}
	member = playwrightNextCode(clean, dot+1)
	if playwrightIdentifierAt(clean, member) != "entries" {
		return -1, -1
	}
	open := playwrightNextCode(clean, member+len("entries"))
	if open >= len(clean) || clean[open] != '(' || !lexed.code[open] {
		return -1, -1
	}
	argument = playwrightNextCode(clean, open+1)
	if argument >= len(clean) || !playwrightIdentifierByte(clean[argument]) || clean[argument] >= '0' && clean[argument] <= '9' {
		return -1, -1
	}
	end := playwrightNextCode(clean, argument+len(playwrightIdentifierAt(clean, argument)))
	if closeAt, closed := lexed.closes[open]; !closed || end != closeAt {
		return -1, -1
	}
	return member, argument
}

// playwrightEntriesArgument reports whether the `(` at open is the call of a direct
// `Object.entries(...)` (TJAA-V0-117).
func playwrightEntriesArgument(clean string, lexed playwrightLexed, open int) bool {
	end := playwrightPreviousCode(clean, open)
	if end < len("Object.entries")-1 || !playwrightIdentifierByte(clean[end]) {
		return false
	}
	start := end
	for start > 0 && playwrightIdentifierByte(clean[start-1]) {
		start--
	}
	dot := playwrightPreviousCode(clean, start)
	if clean[start:end+1] != "entries" || dot < 0 || clean[dot] != '.' {
		return false
	}
	receiver := playwrightPreviousCode(clean, dot)
	if receiver < 0 || !playwrightIdentifierByte(clean[receiver]) {
		return false
	}
	begin := receiver
	for begin > 0 && playwrightIdentifierByte(clean[begin-1]) {
		begin--
	}
	member, argument := playwrightEntriesCall(clean, lexed, begin)
	return clean[begin:receiver+1] == "Object" && member == start && argument == playwrightNextCode(clean, open+1)
}

// playwrightFreshObjectWithoutSpread reports whether the fresh object literal declared by the
// name ending at end holds no spread. A spread copies another object's own members, a `tag`
// array among them, so enumerating the copy could reach that array without naming it
// (TJAA-V0-117).
func playwrightFreshObjectWithoutSpread(clean string, lexed playwrightLexed, end int) bool {
	assignment := strings.IndexByte(clean[end:], '=')
	if assignment < 0 {
		return false
	}
	open := playwrightNextCode(clean, end+assignment+1)
	closeAt, closed := lexed.closes[open]
	if open >= len(clean) || clean[open] != '{' || !closed {
		return false
	}
	for index := open; index+3 <= closeAt; index++ {
		if lexed.code[index] && clean[index:index+3] == "..." {
			return false
		}
	}
	return true
}

// playwrightNonASCIINeighbour reports whether name holds a non-ASCII byte or occurs anywhere in
// clean next to one. playwrightTokenOffsets ends an identifier at a combining mark, a connector or
// a zero-width joiner, which JavaScript reads as part of a longer identifier, so `box\u0301` could
// otherwise pass for a declaration or a use of `box` (TJAA-V0-096, PR review).
func playwrightNonASCIINeighbour(clean, name string) bool {
	if strings.IndexFunc(name, func(character rune) bool { return character >= 0x80 }) >= 0 {
		return true
	}
	for from := 0; ; {
		index := strings.Index(clean[from:], name)
		if index < 0 {
			return false
		}
		at := from + index
		if at > 0 && clean[at-1] >= 0x80 || at+len(name) < len(clean) && clean[at+len(name)] >= 0x80 {
			return true
		}
		from = at + len(name)
	}
}

// playwrightWithStatement reports whether code holds a `with (` statement, which resolves the
// names in its body against an object first (TJAA-V0-096).
func playwrightWithStatement(clean string, lexed playwrightLexed) bool {
	for _, at := range playwrightTokenOffsets(clean, "with") {
		if !lexed.code[at] {
			continue
		}
		if dot := playwrightPreviousCode(clean, at); dot >= 0 && clean[dot] == '.' {
			continue
		}
		if open := playwrightNextCode(clean, at+len("with")); open < len(clean) && clean[open] == '(' {
			return true
		}
	}
	return false
}

// playwrightStatementStart reports whether the expression starting at offset begins a statement,
// so the value of an assignment that starts there is discarded: assignment binds looser than
// every operator but the comma, whose left value is also discarded. It holds after the start of
// the module, `;`, a block's `{` or `}`, `else`, `do`, or the `)` of an `if`, `for` or `while`
// header. Any other predecessor, such as `=>`, `(`, `,`, `=`, `return` or a template
// substitution, can use the value (TJAA-V0-097).
func playwrightStatementStart(clean string, lexed playwrightLexed, offset int) bool {
	previous := playwrightPreviousCode(clean, offset)
	if previous < 0 {
		return true
	}
	if !lexed.code[previous] {
		return false
	}
	switch clean[previous] {
	case ';', '{', '}':
		return true
	case ')':
		open := lexed.parent[previous]
		return open >= 0 && slices.Contains([]string{"if", "for", "while"}, playwrightWordBefore(clean, playwrightPreviousCode(clean, open)))
	}
	word := playwrightWordBefore(clean, previous)
	return word == "else" || word == "do"
}

// playwrightConsumedRead reports whether the computed read `NAME[...]` starting at offset, whose
// next code after `]` is at after, is consumed by an operator whose value is a boolean or a type
// string: the left operand of `===`, `!==`, `==` or `!=` (every operator that binds tighter on its
// left yields a primitive), or a right operand of one of them, the operand of `!` or `typeof`, or
// the whole test of an `if (...)` or `while (...)`, each followed by an explicit end that binds
// looser than equality (playwrightReadEnds). A member, call, template, `?.`, assignment, `||` or
// `??` operand, a `new` or `await` operand, or any other position can hand the read value on
// (TJAA-V0-098).
func playwrightConsumedRead(clean string, lexed playwrightLexed, offset, after int) bool {
	previous := playwrightPreviousCode(clean, offset)
	if word := playwrightWordBefore(clean, previous); word == "new" || word == "await" {
		// `new NAME[k]` and `await NAME[k]` run the read value's constructor or `then` first.
		return false
	}
	rest := clean[min(after, len(clean)):]
	if strings.HasPrefix(rest, "==") || strings.HasPrefix(rest, "!=") {
		return true
	}
	if !playwrightReadEnds(clean, after) {
		return false
	}
	if previous < 0 || !lexed.code[previous] {
		return false
	}
	switch {
	case clean[previous] == '=' && previous > 0 && (clean[previous-1] == '=' || clean[previous-1] == '!'):
		return true
	case clean[previous] == '!':
		return true
	case clean[previous] == '(':
		return clean[after] == ')' && lexed.parent[after] == previous && slices.Contains([]string{"if", "while"}, playwrightWordBefore(clean, playwrightPreviousCode(clean, previous)))
	}
	return playwrightWordBefore(clean, previous) == "typeof"
}

// playwrightReadEnds reports whether the code at index explicitly ends an operand of equality:
// a closing bracket, `;`, `,`, `:`, a conditional `?`, `&&`, `||`, `??` or an equality operator.
// The end of the module and a line break are not ends, since the next line can continue the
// expression with a call or a member (TJAA-V0-098).
func playwrightReadEnds(clean string, index int) bool {
	rest := clean[min(index, len(clean)):]
	if rest == "" {
		return false
	}
	for _, operator := range []string{"&&=", "||=", "??=", "?."} {
		if strings.HasPrefix(rest, operator) {
			return false
		}
	}
	for _, operator := range []string{"&&", "||", "??", "==", "!="} {
		if strings.HasPrefix(rest, operator) {
			return true
		}
	}
	return strings.IndexByte(")]};,:?", rest[0]) >= 0
}

// playwrightStringifyArgument reports whether the name between open, a `(`, and next is the whole
// first argument of a plain `JSON.stringify(...)` call (playwrightPlainStringify), and the call is
// itself a whole property value of object literals nested directly in a `.fulfill(...)` argument
// (playwrightFulfillArgument). The string the call returns could carry the object's members into a
// title, so a call anywhere else, such as an initializer or a test argument, stops (TJAA-V0-099,
// PR review).
func playwrightStringifyArgument(clean string, lexed playwrightLexed, open, next int) bool {
	if !lexed.code[open] || next >= len(clean) || clean[next] != ',' && clean[next] != ')' || lexed.parent[next] != open {
		return false
	}
	end := playwrightPreviousCode(clean, open)
	if end < 0 || !playwrightIdentifierByte(clean[end]) {
		return false
	}
	start := end
	for start > 0 && playwrightIdentifierByte(clean[start-1]) {
		start--
	}
	dot := playwrightPreviousCode(clean, start)
	if clean[start:end+1] != "stringify" || dot < 1 || clean[dot] != '.' || clean[dot-1] == '.' || clean[dot-1] == '?' {
		return false
	}
	receiverEnd := playwrightPreviousCode(clean, dot)
	if receiverEnd < 3 || clean[receiverEnd-3:receiverEnd+1] != "JSON" || receiverEnd >= 4 && playwrightIdentifierByte(clean[receiverEnd-4]) {
		return false
	}
	json := receiverEnd - 3
	if !playwrightPlainStringify(clean, lexed, json) {
		return false
	}
	value, parent := playwrightPreviousCode(clean, json), lexed.parent[json]
	after := playwrightNextCode(clean, lexed.closes[open]+1)
	if value < 0 || clean[value] != ':' || parent < 0 || clean[parent] != '{' || after >= len(clean) || clean[after] != ',' && clean[after] != '}' || lexed.parent[after] != parent {
		return false
	}
	return playwrightFulfillArgument(clean, lexed, parent)
}

// playwrightWordBefore returns the identifier ending at previous when it is not itself a
// property name after `.`, or "".
func playwrightWordBefore(clean string, previous int) string {
	if previous < 0 || !playwrightIdentifierByte(clean[previous]) {
		return ""
	}
	start := previous
	for start > 0 && playwrightIdentifierByte(clean[start-1]) {
		start--
	}
	if before := playwrightPreviousCode(clean, start); before >= 0 && clean[before] == '.' {
		return ""
	}
	return clean[start : previous+1]
}

// playwrightFreshObjectDeclaration reports whether the `const NAME` or `let NAME` whose name
// ends at end and whose keyword ends at previous declares an object literal: it is not exported,
// carries at most a simple type annotation, is a plain `=` of one `{ ... }` and the literal ends
// its expression.
func playwrightFreshObjectDeclaration(clean string, lexed playwrightLexed, end, previous int) bool {
	keyword := previous
	for keyword > 0 && playwrightIdentifierByte(clean[keyword-1]) {
		keyword--
	}
	if playwrightWordBefore(clean, playwrightPreviousCode(clean, keyword)) == "export" {
		return false
	}
	index := playwrightNextCode(clean, end)
	if index < len(clean) && clean[index] == ':' {
		for index++; index < len(clean) && (playwrightIdentifierByte(clean[index]) || strings.IndexByte(" \t<>,.|&[]", clean[index]) >= 0); index++ {
		}
	}
	if !playwrightPlainAssignmentAt(clean, index) {
		return false
	}
	open := playwrightNextCode(clean, index+1)
	if open >= len(clean) || clean[open] != '{' || !lexed.code[open] {
		return false
	}
	closeAt, closed := lexed.closes[open]
	if !closed {
		return false
	}
	next := playwrightNextCode(clean, closeAt+1)
	if next >= len(clean) || strings.IndexByte(";,}", clean[next]) >= 0 {
		return true
	}
	// With no semicolon, a following line that starts a statement ends the declaration; a word
	// that continues the expression (`in`, `instanceof`, `as`, `satisfies`) does not.
	if !strings.ContainsAny(clean[closeAt+1:next], "\n\r") || !playwrightIdentifierByte(clean[next]) {
		return false
	}
	word := next
	for word < len(clean) && playwrightIdentifierByte(clean[word]) {
		word++
	}
	return !slices.Contains([]string{"in", "instanceof", "as", "satisfies"}, clean[next:word])
}

// playwrightInOperandEnd reports whether the right operand of `in` that ends before index ends
// its expression there, so the operator reads only whether a key is present.
func playwrightInOperandEnd(clean string, index int) bool {
	rest := clean[min(index, len(clean)):]
	if rest == "" || strings.IndexByte(");,}]:", rest[0]) >= 0 {
		return true
	}
	for _, operator := range []string{"&&", "||", "??", "==", "!="} {
		if strings.HasPrefix(rest, operator) {
			return true
		}
	}
	return rest[0] == '?' && !strings.HasPrefix(rest, "?.")
}

// playwrightFulfillArgument reports whether the object literal opening at open is nested only
// through property values in the first argument of a direct `.fulfill(...)` call, and no literal
// of that chain is an assignment pattern.
func playwrightFulfillArgument(clean string, lexed playwrightLexed, open int) bool {
	for open >= 0 && clean[open] == '{' {
		previous := playwrightPreviousCode(clean, open)
		closeAt, closed := lexed.closes[open]
		if previous < 0 || !closed || playwrightAssignmentAt(clean, playwrightNextCode(clean, closeAt+1)) {
			// `({ NAME } = value)` destructures into NAME rather than reading it (TJAA-V0-096).
			return false
		}
		switch clean[previous] {
		case ':':
			open = lexed.parent[open]
		case '(':
			if lexed.parent[open] != previous {
				return false
			}
			end := playwrightPreviousCode(clean, previous)
			if end < 0 || !playwrightIdentifierByte(clean[end]) {
				return false
			}
			start := end
			for start > 0 && playwrightIdentifierByte(clean[start-1]) {
				start--
			}
			dot := playwrightPreviousCode(clean, start)
			return clean[start:end+1] == "fulfill" && dot > 0 && clean[dot] == '.' && clean[dot-1] != '.'
		default:
			return false
		}
	}
	return false
}

// playwrightSelfReference returns the offset and name of the first `this` or `super` in code
// that is not a property name after `.`, or -1. A setter, a toJSON method or another method a
// computed write or a serializer calls on a fresh object binds it there, so the object can reach
// a test without its name (TJAA-V0-095).
func playwrightSelfReference(clean string, lexed playwrightLexed) (int, string) {
	first, name := -1, ""
	for _, keyword := range []string{"this", "super"} {
		for _, offset := range playwrightTokenOffsets(clean, keyword) {
			if first >= 0 && offset >= first {
				break
			}
			if !lexed.code[offset] {
				continue
			}
			if dot := playwrightPreviousCode(clean, offset); dot > 0 && clean[dot] == '.' && clean[dot-1] != '.' {
				continue
			}
			first, name = offset, keyword
			break
		}
	}
	return first, name
}

// playwrightAccessorDefinition returns the offset and keyword of the first `get` or `set` in code
// that is not a property name after `.` and is followed by a property name rather than `(`, `:`,
// `,`, `=` or another punctuator, or -1. Such a getter or setter, on a fresh object literal or on
// a prototype a computed `__proto__` write installs, runs on a computed member access and can hand
// the value it reads or is written to a test without `this` or the object's name (TJAA-V0-096).
// A `get` or `set` followed by a name in any other position, such as `import { get as g }`, also
// stops: the rule over-stops rather than parse it.
func playwrightAccessorDefinition(clean string, lexed playwrightLexed) (int, string) {
	first, name := -1, ""
	for _, keyword := range []string{"get", "set"} {
		for _, offset := range playwrightTokenOffsets(clean, keyword) {
			if first >= 0 && offset >= first {
				break
			}
			if !lexed.code[offset] {
				continue
			}
			if dot := playwrightPreviousCode(clean, offset); dot >= 0 && clean[dot] == '.' {
				continue
			}
			next := playwrightNextCode(clean, offset+len(keyword))
			if next >= len(clean) || !playwrightIdentifierByte(clean[next]) && strings.IndexByte("['\"#", clean[next]) < 0 {
				continue
			}
			first, name = offset, keyword
			break
		}
	}
	return first, name
}

// playwrightPlainStringify reports whether the `JSON` token at offset begins a direct call
// `JSON.stringify(value)`, `JSON.stringify(value, null)` or `JSON.stringify(value, null, space)`.
// Its value is a string, and a string reaches a test only as a title, which the listing shows
// exactly, or as a tag value, which the tag rule already refuses; building a `tag` key from it
// needs JSON.parse, eval, a computed key or another name this rule keeps. With no replacer
// function the call runs only the toJSON methods and getters of its argument, user code under the
// same rules, which cannot reach a tag array without naming `tag`, a computed member or a
// reflective name. A replacer, a spread argument, `JSON?.`, a member `x.JSON`, `JSON.stringify`
// read without a direct call, and any other JSON member stay inexact (TJAA-V0-090).
func playwrightPlainStringify(clean string, lexed playwrightLexed, at int) bool {
	if previous := playwrightPreviousCode(clean, at); previous >= 0 && clean[previous] == '.' {
		return false
	}
	dot := playwrightNextCode(clean, at+len("JSON"))
	if dot >= len(clean) || clean[dot] != '.' {
		return false
	}
	member := playwrightNextCode(clean, dot+1)
	if !strings.HasPrefix(clean[member:], "stringify") || member+len("stringify") < len(clean) && playwrightIdentifierByte(clean[member+len("stringify")]) {
		return false
	}
	open := playwrightNextCode(clean, member+len("stringify"))
	if open >= len(clean) || clean[open] != '(' || !lexed.code[open] {
		return false
	}
	closeAt, closed := lexed.closes[open]
	if !closed {
		return false
	}
	arguments, start := []string{}, open+1
	for index := open + 1; index <= closeAt; index++ {
		if index == closeAt || lexed.code[index] && lexed.parent[index] == open && clean[index] == ',' {
			arguments = append(arguments, strings.TrimSpace(clean[start:index]))
			start = index + 1
		}
	}
	if len(arguments) == 0 || len(arguments) > 3 || len(arguments) >= 2 && arguments[1] != "null" {
		return false
	}
	for _, argument := range arguments {
		if argument == "" || strings.HasPrefix(argument, "...") {
			return false
		}
	}
	return true
}

// playwrightMemberAccessAt reports whether a `[` whose previous code byte is at previous opens a
// member access rather than an array literal: it follows an identifier other than a keyword that
// precedes an expression, a `)`, a `]` or the `.` of `?.`.
func playwrightMemberAccessAt(clean string, previous int) bool {
	if previous < 0 {
		return false
	}
	switch character := clean[previous]; {
	case character == ')' || character == ']' || character == '.':
		return true
	case !playwrightIdentifierByte(character):
		return false
	}
	start := previous
	for start > 0 && playwrightIdentifierByte(clean[start-1]) {
		start--
	}
	return !slices.Contains(playwrightExpressionKeywords, clean[start:previous+1])
}

// playwrightFreshArrayAt reports whether the receiver ending at previous, the code byte before a
// computed member access, is a fresh array: an array literal, or the result of a direct
// `.split(...)` or `.map(...)` method call. Such an array is no tag key's value and its items were
// produced by code the closure checks, so indexing it cannot reach a tag array without naming it,
// as long as no closure file defines a split or map of its own (playwrightMethodDefinition).
// A parenthesized receiver, an indexed array, a spread call and any other call are not fresh.
// literal reports an array literal and method a split or map result (TJAA-V0-093).
func playwrightFreshArrayAt(clean string, opens map[int]int, previous int) (literal, method bool) {
	open, closed := opens[previous]
	if !closed {
		return false, false
	}
	switch clean[previous] {
	case ']':
		return playwrightArrayLiteralAt(clean, playwrightPreviousCode(clean, open)), false
	case ')':
		end := playwrightPreviousCode(clean, open)
		start := end
		for start > 0 && playwrightIdentifierByte(clean[start-1]) {
			start--
		}
		if end < 0 || !slices.Contains(playwrightFreshMethods, clean[start:end+1]) {
			return false, false
		}
		dot := playwrightPreviousCode(clean, start)
		return false, dot > 0 && clean[dot] == '.' && clean[dot-1] != '.'
	}
	return false, false
}

// playwrightArrayLiteralAt reports whether a `[` whose previous code byte is at previous
// positively opens an array literal: it starts the text, follows an operator or opening
// punctuator, or follows an expression keyword that is not itself a property name. A `]`, `)`,
// `}`, `.`, `/`, `!` or any identifier, such as `default` in `box.default[0]`, is not
// positively an operand start, so the bracket may index a value (TJAA-V0-093).
func playwrightArrayLiteralAt(clean string, previous int) bool {
	if previous < 0 {
		return true
	}
	if strings.IndexByte("([{,;:=?&|+-*%<>^~", clean[previous]) >= 0 {
		return true
	}
	if !playwrightIdentifierByte(clean[previous]) {
		return false
	}
	start := previous
	for start > 0 && playwrightIdentifierByte(clean[start-1]) {
		start--
	}
	if !slices.Contains([]string{"return", "yield", "await", "case", "typeof", "void", "delete", "in", "of", "throw"}, clean[start:previous+1]) {
		return false
	}
	before := playwrightPreviousCode(clean, start)
	return before < 0 || clean[before] != '.'
}

// playwrightMethodDefinition returns the offset and name of the first of methods in code that is
// not the name of a direct method call such as `.split(` or `.map(`, or of the first string
// literal spelling one, or -1. Such a use can define an object's own split or map that returns a
// details object, so a computed index on its result could reach a tag array (TJAA-V0-093), or an
// object's own fulfill that hands its argument to a test (TJAA-V0-095).
func playwrightMethodDefinition(clean string, lexed playwrightLexed, methods []string) (int, string) {
	first, name := -1, ""
	for _, method := range methods {
		for _, offset := range playwrightTokenOffsets(clean, method) {
			if first >= 0 && offset >= first {
				break
			}
			if !lexed.code[offset] {
				continue
			}
			dot := playwrightPreviousCode(clean, offset)
			call := playwrightNextCode(clean, offset+len(method))
			if dot > 0 && clean[dot] == '.' && clean[dot-1] != '.' && call < len(clean) && clean[call] == '(' {
				continue
			}
			first, name = offset, method
		}
		for _, quoted := range []string{"'" + method + "'", `"` + method + `"`, "`" + method + "`"} {
			if offset := strings.Index(clean, quoted); offset >= 0 && (first < 0 || offset < first) {
				first, name = offset, method
			}
		}
	}
	return first, name
}

// playwrightExpressionKeywords are the keywords after which `[` starts an array literal.
var playwrightExpressionKeywords = []string{"return", "of", "in", "typeof", "instanceof", "case", "yield", "await", "else", "do", "void", "delete", "throw", "new", "default", "extends"}

func playwrightIdentifierByte(character byte) bool {
	return character == '_' || character == '$' || character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= 0x80
}

// playwrightLiteralIndex reports whether a member access index is empty (a type such as
// `string[]`), a decimal integer or one escape-free string literal.
func playwrightLiteralIndex(raw string) bool {
	raw = strings.Trim(raw, " \t\r\n")
	if raw == "" || strings.Trim(raw, "0123456789") == "" {
		return true
	}
	_, literal := playwrightString(raw)
	return literal && !strings.ContainsRune(raw, '\\')
}

// playwrightNextCode returns the index of the first non-space byte at or after index.
func playwrightNextCode(raw string, index int) int {
	for ; index < len(raw) && strings.IndexByte(" \t\r\n", raw[index]) >= 0; index++ {
	}
	return index
}

// playwrightAssignmentAt reports whether an assignment operator (=, +=, ||=, ??=, &&= and the
// other compound forms) starts at index.
func playwrightAssignmentAt(raw string, index int) bool {
	rest := raw[min(index, len(raw)):]
	for _, operator := range []string{"**=", "<<=", ">>>=", ">>=", "||=", "&&=", "??=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^="} {
		if strings.HasPrefix(rest, operator) {
			return true
		}
	}
	return strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") && !strings.HasPrefix(rest, "=>")
}

// playwrightLiteralTagProperty reports whether the `tag` token at offset is a key of its enclosing
// object literal whose value is one or more literal `@\S+` strings, and returns them.
func playwrightLiteralTagProperty(clean string, lexed playwrightLexed, at int) ([]string, bool) {
	previous := playwrightPreviousCode(clean, at)
	next := playwrightNextCode(clean, at+len("tag"))
	if previous < 0 || clean[previous] != '{' && clean[previous] != ',' || next >= len(clean) || clean[next] != ':' {
		return nil, false
	}
	open := lexed.parent[at]
	if open < 0 || clean[open] != '{' {
		return nil, false
	}
	closeAt, closed := lexed.closes[open]
	if !closed {
		return nil, false
	}
	properties, ok := playwrightObjectProperties(clean[open : closeAt+1])
	value, found := properties["tag"]
	if !ok || !found {
		return nil, false
	}
	items, array := playwrightArrayItems(value)
	if !array {
		items = []string{value}
	}
	tags := make([]string, 0, len(items))
	for _, item := range items {
		tag, literal := playwrightString(item)
		if !literal || strings.ContainsRune(item, '\\') || len(tag) < 2 || tag[0] != '@' || strings.ContainsFunc(tag, playwrightJSSpace) {
			return nil, false
		}
		tags = append(tags, tag)
	}
	return tags, len(tags) != 0
}
