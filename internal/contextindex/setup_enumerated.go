package contextindex

import (
	"sort"
	"strconv"
	"strings"
)

// sharedEntry is one `[C.key]: ...` entry of an enumerated lookup table (TSA-V0-854): a computed
// key that names one member of a shared constant.
type sharedEntry struct {
	value sharedConst
	key   string
	line  int
}

// sharedTable is a top-level `const|let|var T [: Type] = { [C.key]: ..., ... }` object literal in a
// module that is not a setup file. Its entries are not reads of their members: a member's key
// joins the closure only when a lookup of the table can select it (TSA-V0-855).
type sharedTable struct {
	module, local string
	line          int
	start, end    int
	entries       []sharedEntry
	// open is the first lookup of the table the selector cannot reduce to literal values,
	// recomputed after every traversal round; nil when every lookup is reduced (TSA-V0-856).
	open *sharedSite
}

// enumeratedEntries finds every top-level `const|let|var T [: Type] = { ... }` whose object literal
// is the whole initializer (the declarator ends at the closing brace, or after `as`/`satisfies`),
// and returns, per token that starts a `[C.key]:` or `[C['key']]:` entry at the literal's own
// level, the table that holds it. An immediate `.x`, `[x]` or `(...)` after the literal is a
// lookup, not a table, and is left to the ordinary read rules.
func enumeratedEntries(module string, tokens []registryToken) map[int]*sharedTable {
	entries := map[int]*sharedTable{}
	depth := 0
	for at := 0; at+1 < len(tokens); at++ {
		token := tokens[at]
		if token.kind == webPunct {
			if strings.Contains("([{", token.text) {
				depth++
			} else if strings.Contains(")]}", token.text) {
				depth--
			}
			continue
		}
		if depth != 0 || registryMember(tokens, at) || tokens[at+1].kind != webWord ||
			!(isRegistryWord(token, "const") || isRegistryWord(token, "let") || isRegistryWord(token, "var")) {
			continue
		}
		equals := registryAnnotationEnd(tokens, at+2)
		if equals+1 >= len(tokens) || !isRegistryPunct(tokens[equals], "=") || !isRegistryPunct(tokens[equals+1], "{") {
			continue
		}
		open := equals + 1
		close := registryClose(tokens, open)
		if close <= open || close >= len(tokens) || !isRegistryPunct(tokens[close], "}") || !sharedTableEnds(tokens, close) {
			continue
		}
		table := &sharedTable{module: module, local: tokens[at+1].text, line: tokens[at+1].line, start: open, end: close}
		inner := 0
		for scan := open + 1; scan < close; scan++ {
			current := tokens[scan]
			if current.kind != webPunct {
				continue
			}
			if inner == 0 && current.text == "[" && (scan == open+1 || isRegistryPunct(tokens[scan-1], ",")) {
				if end := registryClose(tokens, scan); end+1 < close && isRegistryPunct(tokens[end+1], ":") && sharedEntryShape(tokens[scan+1:end]) {
					entries[scan+1] = table
				}
			}
			if strings.Contains("([{", current.text) {
				inner++
			} else if strings.Contains(")]}", current.text) {
				inner--
			}
		}
		at = close
	}
	return entries
}

// sharedTableEnds reports whether the declarator ends at the object literal closing at tokens[close].
func sharedTableEnds(tokens []registryToken, close int) bool {
	if close+1 >= len(tokens) {
		return true
	}
	next := tokens[close+1]
	switch {
	case isRegistryPunct(next, ";"), isRegistryPunct(next, ","), isRegistryWord(next, "as"), isRegistryWord(next, "satisfies"):
		return true
	}
	return next.line > tokens[close].line && !sharedContinues(tokens, close+1)
}

// sharedEntryShape admits the inside of a computed key that is one member of one name: `C.key` or
// `C['key']`.
func sharedEntryShape(inner []registryToken) bool {
	switch len(inner) {
	case 3:
		return inner[0].kind == webWord && isRegistryPunct(inner[1], ".") && inner[2].kind == webWord
	case 4:
		return inner[0].kind == webWord && isRegistryPunct(inner[1], "[") && inner[2].kind == webString && isRegistryPunct(inner[3], "]")
	}
	return false
}

// tableLookups gathers the lookups of one table: the literal values looked up, each with the first
// closure file that looked it up, and the first lookup that is not reduced.
type tableLookups struct {
	values map[string]string
	open   *sharedSite
}

func (l *tableLookups) literal(value, path string) {
	if _, seen := l.values[value]; !seen {
		l.values[value] = path
	}
}

func (l *tableLookups) fail(path string, line int, detail string) {
	if l.open == nil {
		l.open = &sharedSite{path: path, line: line, detail: detail}
	}
}

// resolveEnumerations reduces the lookups of every reached enumerated table (TSA-V0-855): a
// literal lookup adds the member key of each entry whose constant value equals it to the closure's
// reads; a lookup whose key is itself a shared-constant member is covered by that member's own
// read; anything else is recorded as the table's open lookup (TSA-V0-856).
func (s *setupSelector) resolveEnumerations() {
	keys := make([]sharedConst, 0, len(s.shared.tables))
	for key := range s.shared.tables {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(a, b int) bool {
		return keys[a].module+"\x00"+keys[a].local < keys[b].module+"\x00"+keys[b].local
	})
	for _, key := range keys {
		table := s.shared.tables[key]
		look := &tableLookups{values: map[string]string{}}
		s.lookupsOf(table, look)
		if len(look.values) != 0 {
			for _, entry := range table.entries {
				value, literal := s.constValue(entry.value, entry.key)
				if !literal {
					look.fail(table.module, entry.line, "entry "+sharedLabel(entry)+" of "+table.local+" has no literal value")
					continue
				}
				if reader, hit := look.values[value]; hit {
					s.addShared(reader, map[sharedConst]*sharedUse{entry.value: {keys: map[string]bool{entry.key: true}}}, nil)
				}
			}
		}
		table.open = look.open
	}
}

// lookupsOf classifies every lookup of table: its module's own mentions, the closure's reads of the
// table when the module exports it, and the calls of each accessor function that reads the table
// at one of its parameters.
func (s *setupSelector) lookupsOf(table *sharedTable, look *tableLookups) {
	exports := s.shared.exports[table.module]
	tokens := exports.tokens
	accessors := map[string]int{}
	var declarations *sharedGate
	for at, token := range tokens {
		if !isRegistryWord(token, table.local) || registryMember(tokens, at) || sharedNonRead(tokens, at) || sharedDeclaration(tokens, at) {
			continue
		}
		if at > table.start && at < table.end {
			look.fail(table.module, token.line, table.local+" is read inside its own initializer")
			continue
		}
		if values, closed := sharedAccessKeys(tokens, at); closed {
			for _, value := range values {
				look.literal(value, table.module)
			}
			continue
		}
		if at+2 < len(tokens) && isRegistryPunct(tokens[at+1], "[") {
			close := registryClose(tokens, at+1)
			inner := tokens[at+2 : max(close, at+2)]
			if len(inner) == 3 && sharedEntryShape(inner) && s.sharedMember(table.module, exports, inner[0].text) {
				continue
			}
			if len(inner) == 1 && inner[0].kind == webWord {
				if declarations == nil {
					declarations = sharedDeclarations(tokens)
				}
				if function := declarations.owners[at]; function != "" {
					if index, ok := sharedParameter(tokens, declarations.owners, function, table.local, inner[0].text); ok {
						accessors[function] = index
						continue
					}
				}
			}
		}
		look.fail(table.module, token.line, "a lookup of "+table.local+" the selector cannot reduce to literal keys")
	}
	if names := exportedNames(exports, table.local); len(names) != 0 {
		value := sharedConst{table.module, table.local}
		if reached := s.shared.reached[value]; reached != nil {
			for key := range reached.keys {
				look.literal(key, s.shared.readers[value][key])
			}
		}
		for _, site := range s.shared.sites {
			if site.value == value && site.path != table.module && !site.any && !site.keyed {
				look.fail(site.path, site.line, site.detail)
			}
		}
	}
	functions := make([]string, 0, len(accessors))
	for function := range accessors {
		functions = append(functions, function)
	}
	sort.Strings(functions)
	for _, function := range functions {
		s.accessorCalls(table, function, accessors[function], declarations.exported, look)
	}
}

// accessorCalls classifies every reference of an accessor function: in its own module, and in each
// closure file that imports it by name directly from that module. A call with a literal argument at
// the parameter index is a literal lookup; any other reference, and a reach through a namespace,
// re-export, require() or import(), is open.
func (s *setupSelector) accessorCalls(table *sharedTable, function string, index int, declared map[string]string, look *tableLookups) {
	exports := s.shared.exports[table.module]
	s.callsIn(table.module, exports.tokens, function, index, look)
	names := exportedNames(exports, function)
	for name, local := range declared {
		if local == function {
			names[name] = true
		}
	}
	if len(names) == 0 {
		return
	}
	direct := func(from string, binding registryBinding) bool {
		if !names[binding.imported] {
			return false
		}
		resolution := s.resolver.Resolve(from, binding.specifier)
		return resolution.State == WebImportRepository && resolution.Target == table.module
	}
	visited := make([]string, 0, len(s.visited))
	for name := range s.visited {
		visited = append(visited, name)
	}
	sort.Strings(visited)
	for _, name := range visited {
		if name == table.module {
			continue
		}
		file, ok := s.exportsOf(name)
		if !ok {
			continue
		}
		locals := make([]string, 0)
		for local, binding := range file.bindings {
			if direct(name, binding) {
				locals = append(locals, local)
			}
		}
		sort.Strings(locals)
		for _, local := range locals {
			s.callsIn(name, file.tokens, local, index, look)
		}
	}
	demand := s.sharedDemandExcept(direct)[table.module]
	for name := range names {
		if demand[name] || demand["*"] {
			look.fail(table.module, table.line, "accessor "+function+" of "+table.local+" is reached other than by a direct named import")
			return
		}
	}
}

// callsIn classifies the mentions of local in one file's tokens as calls of an accessor.
func (s *setupSelector) callsIn(path string, tokens []registryToken, local string, index int, look *tableLookups) {
	for at, token := range tokens {
		if !isRegistryWord(token, local) || registryMember(tokens, at) || sharedNonRead(tokens, at) || sharedDeclaration(tokens, at) {
			continue
		}
		if at+1 < len(tokens) && isRegistryPunct(tokens[at+1], "(") {
			close := registryClose(tokens, at+1)
			if close > at+1 && close < len(tokens) {
				if arguments := sharedListParts(tokens[at+2 : close]); index < len(arguments) && len(arguments[index]) == 1 {
					if value, literal := registryLiteral(arguments[index][0]); literal {
						look.literal(value, path)
						continue
					}
				}
			}
			look.fail(path, token.line, "a call of "+local+" whose argument "+strconv.Itoa(index+1)+" is not a literal")
			continue
		}
		look.fail(path, token.line, local+" used other than as a call")
	}
}

// sharedParameter reports the index of parameter in function's own parameter list when function is
// a top-level `function` or arrow whose body mentions parameter only as `table[parameter]`.
func sharedParameter(tokens []registryToken, owners []string, function, table, parameter string) (int, bool) {
	first := -1
	for at := range tokens {
		if owners[at] == function {
			first = at
			break
		}
	}
	if first < 0 {
		return 0, false
	}
	open, keyword := -1, false
	for at := first; at < len(tokens) && owners[at] == function; at++ {
		token := tokens[at]
		if isRegistryPunct(token, "(") {
			open = at
			break
		}
		switch {
		case isRegistryWord(token, "function"):
			keyword = true
		case token.kind == webWord && (token.text == function || token.text == "export" || token.text == "default" ||
			token.text == "declare" || token.text == "async" || token.text == "const" || token.text == "let" || token.text == "var"):
		case isRegistryPunct(token, "*"), isRegistryPunct(token, "="):
		default:
			return 0, false
		}
	}
	if open < 0 {
		return 0, false
	}
	close := registryClose(tokens, open)
	if close <= open || close+2 >= len(tokens) {
		return 0, false
	}
	if !keyword && !(isRegistryPunct(tokens[close+1], "=") && isRegistryPunct(tokens[close+2], ">")) {
		return 0, false
	}
	index := -1
	for position, part := range sharedListParts(tokens[open+1 : close]) {
		if len(part) != 0 && isRegistryWord(part[0], parameter) && (len(part) == 1 || isRegistryPunct(part[1], ":")) {
			index = position
		}
	}
	if index < 0 {
		return 0, false
	}
	for at := close + 1; at < len(tokens) && owners[at] == function; at++ {
		if !isRegistryWord(tokens[at], parameter) || registryMember(tokens, at) {
			continue
		}
		if !(at >= 2 && isRegistryPunct(tokens[at-1], "[") && isRegistryWord(tokens[at-2], table) && at+1 < len(tokens) && isRegistryPunct(tokens[at+1], "]")) {
			return 0, false
		}
	}
	return index, true
}

// sharedMember reports whether local names a shared constant in module: an exported object
// constant it declares, or an import that resolves to one.
func (s *setupSelector) sharedMember(module string, exports *moduleExports, local string) bool {
	if imported, ok := exports.bindings[local]; ok {
		resolution := s.resolver.Resolve(module, imported.specifier)
		return resolution.State == WebImportRepository && imported.imported != "*" && s.resolveConst(resolution.Target, imported.imported).kind == sharedFound
	}
	return len(exportedNames(exports, local)) != 0 && constObject(exports.tokens, local)
}

// exportedNames returns the names under which a module exports its own local declaration.
func exportedNames(exports *moduleExports, local string) map[string]bool {
	names := map[string]bool{}
	if _, imported := exports.bindings[local]; imported {
		return names
	}
	for name, exported := range exports.local {
		if exported == local {
			names[name] = true
		}
	}
	return names
}

// constValue returns the literal value of key in the object literal that declares value, or false
// when that value is not a literal or the literal has a part it cannot read (TSA-V0-856).
func (s *setupSelector) constValue(value sharedConst, key string) (string, bool) {
	exports, ok := s.exportsOf(value.module)
	if !ok {
		return "", false
	}
	tokens := exports.tokens
	for at := 0; at+1 < len(tokens); at++ {
		if word := tokens[at]; registryMember(tokens, at) || !(isRegistryWord(word, "const") || isRegistryWord(word, "let") || isRegistryWord(word, "var")) ||
			!isRegistryWord(tokens[at+1], value.local) {
			continue
		}
		equals := registryAnnotationEnd(tokens, at+2)
		if equals+1 >= len(tokens) || !isRegistryPunct(tokens[equals], "=") || !isRegistryPunct(tokens[equals+1], "{") {
			return "", false
		}
		close := registryClose(tokens, equals+1)
		if close <= equals+1 || close >= len(tokens) {
			return "", false
		}
		// The last part naming key wins, as in JavaScript. A part whose key cannot be read (a
		// spread, a computed key, a method or accessor) may set key, so it fails closed.
		value, found := "", false
		for _, part := range sharedListParts(tokens[equals+2 : close]) {
			if len(part) == 0 {
				continue
			}
			name := part[0].text
			if part[0].kind == webString {
				if name, ok = registryLiteral(part[0]); !ok {
					return "", false
				}
			} else if part[0].kind != webWord {
				return "", false
			}
			if len(part) > 1 && !isRegistryPunct(part[1], ":") {
				return "", false
			}
			if name == key {
				value, found = "", false
				if len(part) == 3 {
					value, found = registryLiteral(part[2])
				}
			}
		}
		return value, found
	}
	return "", false
}

func sharedLabel(entry sharedEntry) string {
	return entry.value.local + "." + entry.key
}

// enumeratedReasons fails closed for every table with an open lookup while a setup file that is
// not selected reads one of its entries' members, or reads that constant non-literally
// (TSA-V0-856).
func (s *setupSelector) enumeratedReasons(setups []string) {
	keys := make([]sharedConst, 0, len(s.shared.tables))
	for key, table := range s.shared.tables {
		if table.open != nil {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(a, b int) bool {
		return keys[a].module+"\x00"+keys[a].local < keys[b].module+"\x00"+keys[b].local
	})
	for _, key := range keys {
		table := s.shared.tables[key]
	next:
		for _, setup := range setups {
			for _, entry := range table.entries {
				if use := s.shared.setupUses[setup][entry.value]; use != nil && (use.keys[entry.key] || use.open != 0) {
					s.reason("shared-constant-enumerated", table.open.path, table.open.line, table.open.detail+"; "+table.local+" ("+
						table.module+":"+strconv.Itoa(table.line)+") enumerates "+sharedLabel(entry)+", which "+setup+" reads")
					break next
				}
			}
		}
	}
}
