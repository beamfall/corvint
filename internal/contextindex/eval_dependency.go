package contextindex

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

// The dependency-link stage of confident-symbol selection, ported from the
// oracle's `_web_dependency_links` and its helpers (`src/context_corvint.py`).
//
// The frontier stage picks the strongest declaration per path by vocabulary
// alone. It has no way to say that the second result is the function the first
// one calls, so on a web repository it fills the packet with same-vocabulary
// neighbours instead of the call chain the task is actually about. This stage
// walks the root result's direct calls two hops deep and promotes the two
// strongest linked declarations to ranks 2 and 3, each carrying the import and
// call sites that bind it to the root.
//
// Resolution reuses the reverse-import rules' specifier admission and target
// resolution (`reverseimports.go`), so the `@/` alias comes from the project
// profile rather than the one repository root the oracle hardcodes (DR-0012).

const (
	webDependencyDepth   = 2
	webDependencyMaximum = 32
)

// webRelation is one resolved call edge: how the callee is reached, the symbol
// selector it is reached from, where the call is written, and -- for an
// imported callee -- where the binding import is written.
type webRelation struct {
	kind, from      string
	callPath        string
	callLine        int
	importPath      string
	importLine      int
	importedBinding bool
	depth           int
}

type webSymbolKey struct{ path, name string }

// webImportStatement is the oracle's static named-import pattern. RE2 has no
// lookahead, so the oracle's `(?!['"])` guard on the clause's first character
// is spelled as consuming one non-quote character: the two agree except on an
// import whose clause is empty, which is not a valid statement.
var webImportStatement = regexp.MustCompile(`(?ms)^[ \t]*import[ \t]+([^'"].*?)[ \t]+from[ \t]*['"]([^'"]+)['"]`)

var (
	webNamedClause = regexp.MustCompile(`(?s)\{(.*?)\}`)
	webAliasSplit  = regexp.MustCompile(`\s+as\s+`)
	webIdentifier  = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
	// The oracle's call-site pattern. Go's `\w` and `\b` are ASCII where
	// Python's are Unicode-aware, which can only differ on a non-ASCII
	// identifier; the symbol table this resolves against is built from the same
	// ASCII-anchored pattern, so a name that differs here resolves to nothing
	// either way.
	webCallSite = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\s*(?:\?\.)?\(`)
)

// webDependencyLinks walks the call graph out of root, breadth first, and
// returns the relation each reachable declaration was reached by.
func webDependencyLinks(index *Index, root Symbol) map[webSymbolKey]webRelation {
	links := make(map[webSymbolKey]webRelation)
	visited := map[webSymbolKey]struct{}{{root.Path, root.Name}: {}}
	queue := []struct {
		symbol Symbol
		depth  int
	}{{root, 0}}
	for len(queue) != 0 && len(links) < webDependencyMaximum {
		caller := queue[0]
		queue = queue[1:]
		if caller.depth >= webDependencyDepth {
			continue
		}
		for _, call := range webDirectCalls(index, caller.symbol) {
			key := webSymbolKey{call.callee.Path, call.callee.Name}
			if _, seen := visited[key]; seen {
				continue
			}
			visited[key] = struct{}{}
			call.relation.depth = caller.depth + 1
			links[key] = call.relation
			queue = append(queue, struct {
				symbol Symbol
				depth  int
			}{call.callee, caller.depth + 1})
			if len(links) >= webDependencyMaximum {
				break
			}
		}
	}
	return links
}

// evalLinkedSymbols promotes the two strongest declarations the frontier's
// root result depends on to ranks 2 and 3. Where the task names the root
// explicitly and something links to it, the rest of the frontier is dropped:
// the caller asked about that declaration, so its call chain answers the task
// and its same-vocabulary neighbours do not. Only a multi-part identifier
// counts as named (GPK-V0-047, decision 0157): a one-part name such as `kill`
// is indistinguishable from an ordinary task word.
func evalLinkedSymbols(index *Index, ranked, confident []evalSymbolCandidate, queryText string, limit int) []evalSymbolCandidate {
	root := confident[0].symbol.symbol
	links := webDependencyLinks(index, root)
	linked := make([]evalSymbolCandidate, 0, len(links))
	for _, candidate := range ranked {
		relation, isLinked := links[webSymbolKey{candidate.symbol.symbol.Path, candidate.symbol.symbol.Name}]
		if !isLinked {
			continue
		}
		candidate.result = linkedSymbolResult(index, candidate.symbol.symbol, candidate.score, relation)
		linked = append(linked, candidate)
	}
	sort.SliceStable(linked, func(left, right int) bool {
		if linked[left].score != linked[right].score {
			return linked[left].score > linked[right].score
		}
		leftDepth := links[webSymbolKey{linked[left].symbol.symbol.Path, linked[left].symbol.symbol.Name}].depth
		rightDepth := links[webSymbolKey{linked[right].symbol.symbol.Path, linked[right].symbol.symbol.Name}].depth
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return linked[left].id < linked[right].id
	})
	promoted := linked[:min(len(linked), 2)]
	promotedIDs := make(map[string]struct{}, len(promoted))
	for _, candidate := range promoted {
		promotedIDs[candidate.id] = struct{}{}
	}
	result := append(make([]evalSymbolCandidate, 0, min(limit, 8)), confident[0])
	result = append(result, promoted...)
	rootParts := strings.FieldsFunc(camelSplit(root.Name), func(character rune) bool {
		return !asciiLetter(character) && !asciiDigit(character)
	})
	if !(len(rootParts) > 1 && strings.Contains(compactText(queryText), compactText(root.Name)) && len(linked) != 0) {
		for _, candidate := range confident[1:] {
			if _, promoted := promotedIDs[candidate.id]; promoted || candidate.id == confident[0].id {
				continue
			}
			result = append(result, candidate)
		}
	}
	return result[:min(len(result), min(limit, 8))]
}

type webCall struct {
	callee   Symbol
	relation webRelation
}

// webDirectCalls resolves the calls written inside one declaration's span, to
// declarations in the same file and to statically imported declarations. Only
// an unambiguous target is kept: a name declared twice in the file, or an
// import that resolves to several sources, resolves to nothing.
func webDirectCalls(index *Index, caller Symbol) []webCall {
	if !webSuffixes[strings.ToLower(pythonPathSuffix(caller.Path))] {
		return nil
	}
	source, tracked := index.Sources[caller.Path]
	if !tracked {
		return nil
	}
	text, valid, loaded := source.Text()
	if !loaded || !valid {
		return nil
	}
	lines := pythonSplitLines(text)
	startLine, span := webSymbolSpan(index, caller, lines)
	code := stripWebNonCode(span)
	callLines := webCallLines(code, startLine)

	localByName := make(map[string][]Symbol)
	for _, symbol := range index.Symbols {
		if symbol.Path != caller.Path || symbol.Name == caller.Name || symbol.Line-1 >= len(lines) {
			continue
		}
		if declaration := lines[symbol.Line-1]; declaration != pythonTrimLeftSpace(declaration) {
			continue
		}
		localByName[symbol.Name] = append(localByName[symbol.Name], symbol)
	}

	calls := make([]webCall, 0)
	for _, name := range webCallOrder(callLines) {
		local := localByName[name]
		if len(local) != 1 {
			continue
		}
		calls = append(calls, webCall{local[0], webRelation{
			kind: "calls-local", from: "symbol:" + caller.Path + ":" + caller.Name,
			callPath: caller.Path, callLine: callLines[name],
		}})
	}
	for _, binding := range webImportBindings(text) {
		callLine, called := callLines[binding.local]
		if !called {
			continue
		}
		targetPath := resolvedWebSource(index, caller.Path, binding.module)
		if targetPath == "" {
			continue
		}
		targets := make([]Symbol, 0, 1)
		for _, symbol := range index.Symbols {
			if symbol.Path == targetPath && symbol.Name == binding.exported {
				targets = append(targets, symbol)
			}
		}
		if len(targets) != 1 {
			continue
		}
		calls = append(calls, webCall{targets[0], webRelation{
			kind: "calls-imported", from: "symbol:" + caller.Path + ":" + caller.Name,
			callPath: caller.Path, callLine: callLine,
			importPath: caller.Path, importLine: binding.line, importedBinding: true,
		}})
	}
	return calls
}

// webCallLines maps each called name to the line of its last call site, which
// is what the oracle's dict comprehension over `finditer` leaves behind.
func webCallLines(code string, startLine int) map[string]int {
	result := make(map[string]int)
	for _, match := range webCallSite.FindAllStringSubmatchIndex(code, -1) {
		result[code[match[2]:match[3]]] = startLine + strings.Count(code[:match[0]], "\n")
	}
	return result
}

// webCallOrder is the oracle's `sorted(names.items(), key=(line, name))`.
func webCallOrder(callLines map[string]int) []string {
	names := make([]string, 0, len(callLines))
	for name := range callLines {
		names = append(names, name)
	}
	sort.Slice(names, func(left, right int) bool {
		if callLines[names[left]] != callLines[names[right]] {
			return callLines[names[left]] < callLines[names[right]]
		}
		return names[left] < names[right]
	})
	return names
}

// webSymbolSpan is the text of one declaration: from its own line to the line
// before the next unindented declaration in the file.
func webSymbolSpan(index *Index, symbol Symbol, lines []string) (int, string) {
	start := symbol.Line - 1
	end := len(lines)
	for _, candidate := range index.Symbols {
		if candidate.Path != symbol.Path || candidate.Line <= symbol.Line || candidate.Line-1 >= len(lines) {
			continue
		}
		if declaration := lines[candidate.Line-1]; declaration == pythonTrimLeftSpace(declaration) {
			end = min(end, candidate.Line-1)
		}
	}
	if start < 0 || start > len(lines) || end < start {
		return symbol.Line, ""
	}
	return symbol.Line, strings.Join(lines[start:end], "\n")
}

type webBinding struct {
	local, module, exported string
	line                    int
}

// webImportBindings returns the local names a file binds through static named
// imports, in the oracle's `sorted(bindings.items())` order. A type-only clause
// or a type-only item binds no value and is skipped.
func webImportBindings(text string) []webBinding {
	bindings := make(map[string]webBinding)
	for _, match := range webImportStatement.FindAllStringSubmatchIndex(text, -1) {
		clause := TrimPythonSpace(text[match[2]:match[3]])
		module := text[match[4]:match[5]]
		if strings.HasPrefix(clause, "type ") {
			continue
		}
		named := webNamedClause.FindStringSubmatch(clause)
		if named == nil {
			continue
		}
		line := strings.Count(text[:match[0]], "\n") + 1
		for _, raw := range strings.Split(named[1], ",") {
			item := TrimPythonSpace(raw)
			if item == "" || strings.HasPrefix(item, "type ") {
				continue
			}
			parts := webAliasSplit.Split(item, -1)
			exported, local := "", ""
			switch len(parts) {
			case 1:
				exported, local = parts[0], parts[0]
			case 2:
				exported, local = parts[0], parts[1]
			default:
				continue
			}
			if webIdentifier.MatchString(exported) && webIdentifier.MatchString(local) {
				bindings[local] = webBinding{local, module, exported, line}
			}
		}
	}
	ordered := make([]webBinding, 0, len(bindings))
	for _, binding := range bindings {
		ordered = append(ordered, binding)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].local < ordered[right].local })
	return ordered
}

// resolvedWebSource resolves one specifier to the single indexed source it
// names, or to the empty string when it names none or several.
func resolvedWebSource(index *Index, importerPath, imported string) string {
	profile := projectprofile.ByID(index.ProfileID)
	if !webSpecifierAdmitted(imported, profile) {
		return ""
	}
	target := webImportTarget(importerPath, imported, profile)
	candidates := []string{target}
	for _, suffix := range webSuffixOrder {
		candidates = append(candidates, target+suffix)
	}
	for _, suffix := range webSuffixOrder {
		candidates = append(candidates, strings.TrimRight(target, "/")+"/index"+suffix)
	}
	resolved := ""
	for _, candidate := range candidates {
		if _, tracked := index.Sources[candidate]; !tracked {
			continue
		}
		if resolved != "" && resolved != candidate {
			return ""
		}
		resolved = candidate
	}
	return resolved
}

// webSuffixOrder is the oracle's `_WEB_SUFFIXES` tuple. Order is observable:
// the candidate list is built from it and a target matching two of them
// resolves to nothing at all.
var webSuffixOrder = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

// linkedSymbolResult is the result body for a dependency-linked declaration:
// the ordinary symbol result, the relation that reached it, and the import and
// call sites that prove the binding.
func linkedSymbolResult(index *Index, symbol Symbol, score int, relation webRelation) map[string]any {
	result := featureSymbolResult(symbol, score, relation.kind+" dependency is semantically relevant to the query")
	result["relation"] = map[string]any{"kind": relation.kind, "from": relation.from, "depth": relation.depth}
	items := anySlice(result["evidence"])
	if relation.importedBinding {
		items = append(items, evidence(relation.importPath, relation.importLine, index.Sources[relation.importPath].BlobHash,
			"static first-party import binds "+symbol.Name, "high", "syntax-import"))
	}
	items = append(items, evidence(relation.callPath, relation.callLine, index.Sources[relation.callPath].BlobHash,
		relation.from+" calls "+symbol.Name, "high", "syntax-call"))
	result["evidence"] = items
	return result
}

// stripWebNonCode blanks comments and string, character and template literals
// while preserving every line position, so a call site's line number survives
// and a name written inside a comment or a literal is not read as one.
func stripWebNonCode(text string) string {
	var output strings.Builder
	output.Grow(len(text))
	const (
		code = iota
		lineComment
		blockComment
		literal
	)
	state, quote := code, byte(0)
	for index := 0; index < len(text); {
		character := text[index]
		following := byte(0)
		if index+1 < len(text) {
			following = text[index+1]
		}
		switch state {
		case lineComment:
			if character == '\n' {
				output.WriteByte('\n')
				state = code
			} else {
				output.WriteByte(' ')
			}
			index++
		case blockComment:
			if character == '*' && following == '/' {
				output.WriteString("  ")
				state, index = code, index+2
				continue
			}
			output.WriteByte(blankOrNewline(character))
			index++
		case literal:
			if character == '\\' {
				output.WriteByte(' ')
				if index+1 < len(text) {
					output.WriteByte(blankOrNewline(following))
					index += 2
				} else {
					index++
				}
				continue
			}
			if character == quote {
				output.WriteByte(' ')
				state = code
			} else {
				output.WriteByte(blankOrNewline(character))
			}
			index++
		default:
			switch {
			case character == '/' && following == '/':
				output.WriteString("  ")
				state, index = lineComment, index+2
			case character == '/' && following == '*':
				output.WriteString("  ")
				state, index = blockComment, index+2
			case character == '\'' || character == '"' || character == '`':
				output.WriteByte(' ')
				quote, state, index = character, literal, index+1
			default:
				output.WriteByte(character)
				index++
			}
		}
	}
	return output.String()
}

func blankOrNewline(character byte) byte {
	if character == '\n' {
		return '\n'
	}
	return ' '
}

// pythonSplitLines is str.splitlines() over LF-separated text: strings.Split
// keeps the empty tail a terminating newline leaves behind, and splitlines
// drops it. The rest of the package already treats "\n" as the only separator.
func pythonSplitLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) != 0 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

// pythonTrimLeftSpace matches str.lstrip() for Python's Unicode whitespace set,
// which TrimPythonSpace already fixes for str.strip().
func pythonTrimLeftSpace(value string) string {
	return strings.TrimLeftFunc(value, func(character rune) bool {
		return unicode.IsSpace(character) || character >= 0x1c && character <= 0x1f
	})
}
