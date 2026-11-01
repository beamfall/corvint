package contextindex

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/rubyautoload"
)

// Ruby autoload-convention reverse references (NGI-V0-016 to NGI-V0-021,
// NGI-V0-033 to NGI-V0-040).
//
// A Rails/Zeitwerk application never requires the file that defines a model or
// service: the constant's name is derived from the file's path below an
// autoload root, and any code naming the constant loads it. The rubyautoload
// package maps each Ruby source below a root to the constant its path names;
// this indexes constant references (fully qualified and namespace-relative,
// resolved through the file's lexical `module`/`class` nesting) and reports a
// file naming the changed path's constant as a named-constant reverse
// reference, and a file naming such a referrer's own constant as a second-hop
// reference. Every such row is an advisory syntax hint. It is syntax only: it
// evaluates nothing, so reflective lookups and constant names spelled as
// strings or association symbols stay counted unknowns, and Ruby's
// ancestor-chain constant lookup is not modelled.

const (
	// RubyAutoloadDeclarationPath is the optional committed declaration of a
	// repository's autoload roots and inflections (NGI-V0-017).
	RubyAutoloadDeclarationPath = rubyautoload.DeclarationPath

	// Named-constant reverse references rank below every direct import
	// (700 code, 650 test) and every type-only import (600, 550): they are
	// a convention, not a load statement the file carries (NGI-V0-018). A
	// second-hop reference ranks below every first hop (NGI-V0-034).
	rubyConstantReferenceScore   = 525
	rubyConstantTestScore        = 500
	rubyTransitiveReferenceScore = 475
	rubyTransitiveTestScore      = 450
)

// The reference index is bounded (NGI-V0-040, TSA-V0-460): an unbalanced
// file resolves a reference against at most rubyResolutionBound of the
// namespaces it opens, and the index retains at most rubySiteBound sites in
// total. A reference either bound cuts counts in the
// ruby-reference-index-bounded unknown rather than being dropped silently.

// rubyAutoloadAdvisoryUncertainty is the coverage line a receipt carrying
// any autoload-convention reference adds (NGI-V0-035).
const rubyAutoloadAdvisoryUncertainty = "Ruby autoload-convention references are advisory syntax hints: a named constant does not establish load order, runtime dispatch or test closure"

// advisoryUncertainty names each advisory row family in results, in a fixed
// order: learned paths, then Ruby autoload-convention references
// (NGI-V0-035). The learned-path line is present only with a learned-path row.
func advisoryUncertainty(results []map[string]any) []any {
	lines := rubyAdvisoryUncertainty(results)
	for _, item := range results {
		if item["worktree_overlay"] == true { // DIRTY-CACHE-942
			lines = append(lines, worktreeOverlayUncertainty)
			break
		}
	}
	for _, item := range results {
		if item["kind"] == "learned-path" {
			return append([]any{"learned path relationships are advisory"}, lines...)
		}
	}
	return lines
}

// rubyAdvisoryUncertainty is the Ruby advisory line when results carry an
// autoload-convention row, which path and range receipts both add.
func rubyAdvisoryUncertainty(results []map[string]any) []any {
	for _, item := range results {
		if item["kind"] != "learned-path" && item["advisory"] == true && item["worktree_overlay"] != true {
			return []any{rubyAutoloadAdvisoryUncertainty}
		}
	}
	return nil
}

var (
	rubyDynamicLookup  = regexp.MustCompile(`\b(?:constantize|safe_constantize|const_get|send|public_send|__send__)\b`)
	rubyStringConstant = regexp.MustCompile(`(['"])(?:::)?([A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*)['"]`)
	rubyAssociation    = regexp.MustCompile(`^\s*(?:belongs_to|has_one|has_many|has_and_belongs_to_many)\s*\(?\s*:([a-z_][a-z0-9_]*)`)
)

type rubyReference struct {
	line     int
	name     string   // as written, without a leading `::`
	stack    []string // lexical namespaces, outermost first
	rooted   bool     // written with a leading `::`
	defined  bool     // the name of a `class`/`module` keyword
	constant string   // the constant a defined name opens, resolved when scanned
}

type rubyConstantSource struct {
	references []rubyReference
	namespaces []string        // every namespace the file opens
	local      map[string]bool // every constant the file defines, with its prefixes
	balanced   bool
	overflowed bool // a block opened past rubyNestingBound, so its namespaces are unknown
	lookups    int
	literals   []rubyLiteralName
	readable   bool
}

type rubyLiteralName struct {
	line int
	name string
}

// rubyReferenceSite is one reference naming a constant.
type rubyReferenceSite struct {
	path string
	line int
}

// rubyCut is one reference a bound cut (NGI-V0-040): the file holding it,
// the candidates it resolved, whether that resolution was complete (the
// namespace bound did not truncate it), and whether every site was recorded
// (the site bound did not stop it).
type rubyCut struct {
	path      string
	source    *rubyConstantSource
	reference rubyReference
	resolved  []string
	complete  bool
	indexed   bool
}

type rubyAutoload struct {
	mapper  *rubyautoload.Mapper
	sources map[string]*rubyConstantSource
	paths   []string
	// sites indexes every constant a reference may name, and each namespace
	// prefix of it, to its references in path order then line order, one
	// site per reference, so each hop is one lookup rather than a rescan.
	sites map[string][]rubyReferenceSite
	// retained counts the sites in sites; bounded counts the references
	// whose resolutions or sites a bound cut (NGI-V0-040).
	retained, bounded int
	// cuts holds each reference counted in bounded, so a changed path's
	// unknown counts only the cuts that could name its constants (NGI-V0-064).
	cuts []rubyCut
	// lookups counts referrers calls, so a test can observe that each
	// constant is traversed once.
	lookups int
}

// loadRubyAutoload maps every Ruby source to its constant and indexes every
// Ruby source's constant references once.
func loadRubyAutoload(index *Index) *rubyAutoload {
	autoload := &rubyAutoload{sources: map[string]*rubyConstantSource{}, sites: map[string][]rubyReferenceSite{}}
	for name := range index.Sources {
		if NonGoImpactLanguage(name) == "ruby" {
			autoload.paths = append(autoload.paths, name)
		}
	}
	sort.Strings(autoload.paths)
	repository := rubyautoload.Repository{Paths: autoload.paths, IsTest: isRubyTestPath,
		Source: func(name string) ([]string, []string, bool) {
			text, valid, loaded := index.Sources[name].Text()
			if !valid || !loaded {
				return nil, nil, false
			}
			return strings.Split(text, "\n"), FlowCodeLines("ruby", text), true
		}}
	repository.DeclarationPresent, repository.DeclarationReadable, repository.DeclarationText = rubyAutoloadDeclaration(index)
	autoload.mapper = rubyautoload.New(repository)
	if autoload.mapper.DeclarationInvalid() {
		return autoload
	}
	for _, name := range autoload.paths {
		source := scanRubyConstantSource(index.Sources[name], autoload.mapper.Known)
		autoload.sources[name] = source
		for _, reference := range source.references {
			resolved, complete := autoload.resolutions(source, reference)
			// A file nested past the nesting bound opened namespaces that were
			// never tracked, so no fallback is complete for it (TSA-V0-461).
			if indexed := autoload.indexSites(name, reference.line, resolved); !indexed || !complete || source.overflowed {
				autoload.bounded++
				autoload.cuts = append(autoload.cuts, rubyCut{name, source, reference, resolved, complete, indexed})
			}
		}
	}
	return autoload
}

// indexSites records one site for each constant in resolved and each of its
// namespace prefixes, once per reference. It reports false when the
// retained-site bound stopped it before every site was recorded.
func (autoload *rubyAutoload) indexSites(name string, line int, resolved []string) bool {
	named := map[string]bool{}
	for _, constant := range resolved {
		for prefix := constant; prefix != "" && !named[prefix]; {
			if !rubySiteBound.admits(autoload.retained + 1) {
				return false
			}
			named[prefix] = true
			autoload.retained++
			autoload.sites[prefix] = append(autoload.sites[prefix], rubyReferenceSite{name, line})
			cut := strings.LastIndex(prefix, "::")
			if cut < 0 {
				break
			}
			prefix = prefix[:cut]
		}
	}
	return true
}

// rubyAutoloadDeclaration reads the committed declaration. A present one
// that is excluded, undecodable or not strict JSON is passed as unreadable,
// which the mapper reports as invalid.
func rubyAutoloadDeclaration(index *Index) (present, readable bool, text string) {
	source, exists := index.Sources[RubyAutoloadDeclarationPath]
	if !exists {
		for _, exclusion := range index.Exclusions {
			if exclusion.Path == RubyAutoloadDeclarationPath {
				return true, false, ""
			}
		}
		return false, false, ""
	}
	text, valid, loaded := source.Text()
	return true, valid && loaded && len(text) <= rubyautoload.MaxDeclarationBytes && strictJSON(text), text
}

func sortedRubySet(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

// scanRubyConstantSource indexes one source's constant references with the
// lexical namespaces enclosing each. Blocks opened by keywords that need an
// `end` are tracked so a namespace closes with its `end`; a file whose
// openers and `end`s do not balance, or that nests deeper than
// rubyNestingBound blocks, is marked, and its references are then resolved
// against every namespace it opens, never fewer. known holds the mapped
// constants and their namespace prefixes; together with the constants this
// file defines it resolves the owner of a qualified `class A::B` name.
func scanRubyConstantSource(source Source, known func(string) bool) *rubyConstantSource {
	result := &rubyConstantSource{balanced: true, local: map[string]bool{}}
	text, valid, loaded := source.Text()
	if !valid || !loaded {
		return result
	}
	result.readable = true
	// frames holds one entry per open block, true when it opened a namespace;
	// lexical holds the open namespaces, outermost first. Every push and pop
	// is O(1): the innermost namespace is the top of lexical, and a block past
	// the nesting bound is only counted in overflow.
	frames := make([]bool, 0, 16)
	lexical := []string{}
	overflow := 0
	snapshot, fresh := []string{}, true
	namespaces := func() []string {
		if !fresh {
			snapshot, fresh = append([]string{}, lexical...), true
		}
		return snapshot
	}
	push := func(namespace string) bool {
		if !rubyNestingBound.admits(len(frames) + 1) {
			overflow++
			result.balanced, result.overflowed = false, true
			return false
		}
		frames = append(frames, namespace != "")
		if namespace != "" {
			lexical, fresh = append(lexical, namespace), false
		}
		return true
	}
	opened := map[string]bool{}
	raw := strings.Split(text, "\n")
	masked := FlowCodeLines("ruby", text)
	for lineIndex, code := range masked {
		line := lineIndex + 1
		if strings.HasPrefix(raw[lineIndex], "__END__") && strings.TrimSpace(raw[lineIndex]) == "__END__" {
			break
		}
		result.lookups += len(rubyDynamicLookup.FindAllStringIndex(code, -1))
		// A quoted constant-shaped name is data the mask blanked (a string,
		// or a comment); counting a comment too only widens the frontier.
		for _, match := range rubyStringConstant.FindAllStringSubmatchIndex(raw[lineIndex], -1) {
			result.literals = append(result.literals, rubyLiteralName{line, raw[lineIndex][match[4]:match[5]]})
		}
		if association := rubyAssociation.FindStringSubmatch(code); association != nil {
			result.literals = append(result.literals, rubyLiteralName{line, ":" + association[1]})
		}
		loopHeader := false
		for i := 0; i < len(code); i++ {
			c := code[i]
			if rubyKeywordStart(code, i) {
				word, after := rubyWord(code, i)
				switch word {
				case "module", "class":
					cursor, _ := rubySkipSpaces(code, after)
					rooted := strings.HasPrefix(code[cursor:], "::")
					if rooted {
						cursor += 2
					}
					name, end := rubyReferenceAt(code, cursor)
					if name == "" {
						// `class << self` opens a block with no namespace.
						push("")
						i = after - 1
						continue
					}
					full := name
					if !rooted {
						full = rubyDefinedConstant(name, lexical, known, result.local)
					}
					result.references = append(result.references, rubyReference{line: line, name: name, stack: namespaces(),
						rooted: rooted, defined: true, constant: full})
					// A superclass is resolved in the enclosing scope.
					next, _ := rubySkipSpaces(code, end)
					if word == "class" && next < len(code) && code[next] == '<' && !strings.HasPrefix(code[next:], "<<") {
						superStart, _ := rubySkipSpaces(code, next+1)
						superRooted := strings.HasPrefix(code[superStart:], "::")
						if superRooted {
							superStart += 2
						}
						if superName, superEnd := rubyReferenceAt(code, superStart); superName != "" {
							result.references = append(result.references, rubyReference{line: line, name: superName, stack: namespaces(), rooted: superRooted})
							end = superEnd
						}
					}
					push(full)
					opened[full] = true
					result.define(full)
					i = end - 1
					continue
				case "def":
					if !rubyEndlessDef(code, after) {
						push("")
					}
				case "begin", "case":
					push("")
				case "if", "unless", "while", "until", "for":
					if rubyStatementStart(code, i) {
						push("")
						loopHeader = loopHeader || word == "while" || word == "until" || word == "for"
					}
				case "do":
					if loopHeader {
						loopHeader = false
					} else {
						push("")
					}
				case "end":
					switch {
					case overflow != 0:
						overflow--
					case len(frames) == 0:
						result.balanced = false
					default:
						if frames[len(frames)-1] {
							lexical, fresh = lexical[:len(lexical)-1], false
						}
						frames = frames[:len(frames)-1]
					}
				}
				i = after - 1
				continue
			}
			if rubyUpperByte(c) && rubyConstantStart(code, i) {
				rooted := i >= 2 && code[i-1] == ':' && code[i-2] == ':'
				name, end := rubyReferenceAt(code, i)
				switch {
				case name == "" || rubyLabelOrCall(code, end):
				case rubyAssignsConstant(code, end):
					// `Thing = ...` defines a constant in the innermost
					// namespace, `::Thing = ...` a top-level one and
					// `A::B = ...` B inside the A lexical lookup finds; none is
					// a reference to an autoloaded constant, but the namespace
					// a qualified target names is (V1-1714).
					full := name
					if !rooted {
						full = rubyDefinedConstant(name, lexical, known, result.local)
					}
					if cut := strings.LastIndex(name, "::"); cut >= 0 {
						result.references = append(result.references, rubyReference{line: line, name: name[:cut], stack: namespaces(), rooted: rooted})
					}
					result.define(full)
				default:
					result.references = append(result.references, rubyReference{line: line, name: name, stack: namespaces(), rooted: rooted})
				}
				i = end - 1
			}
		}
	}
	if len(frames) != 0 || overflow != 0 {
		result.balanced = false
	}
	result.namespaces = sortedRubySet(opened)
	return result
}

func rubyInnermost(lexical []string) string {
	if len(lexical) == 0 {
		return ""
	}
	return lexical[len(lexical)-1]
}

// define records a constant this file defines, with its namespace prefixes.
func (source *rubyConstantSource) define(constant string) {
	for prefix := constant; prefix != ""; {
		source.local[prefix] = true
		cut := strings.LastIndex(prefix, "::")
		if cut < 0 {
			break
		}
		prefix = prefix[:cut]
	}
}

// rubyDefinedConstant is the constant a relative `class`/`module` name
// defines: a simple name is defined in the innermost namespace; a qualified
// `A::B` reopens B inside the constant A names by lexical lookup, the
// innermost enclosing namespace N for which N::A exists, else the top-level A.
func rubyDefinedConstant(name string, lexical []string, known func(string) bool, local map[string]bool) string {
	first, _, qualified := strings.Cut(name, "::")
	if !qualified {
		if outer := rubyInnermost(lexical); outer != "" {
			return outer + "::" + name
		}
		return name
	}
	for i := len(lexical) - 1; i >= 0; i-- {
		if candidate := lexical[i] + "::" + first; known(candidate) || local[candidate] {
			return lexical[i] + "::" + name
		}
	}
	return name
}

// rubyAssignsConstant reports `Name = value`, never `==`, `=~` or `=>`.
// `Name ||= value` is not a definition: it reads Name through lexical scope
// and ancestors and keeps an existing (autoloaded) constant, so it stays a
// reference (V1-1714).
func rubyAssignsConstant(code string, end int) bool {
	cursor, _ := rubySkipSpaces(code, end)
	return cursor < len(code) && code[cursor] == '=' &&
		(cursor+1 >= len(code) || (code[cursor+1] != '=' && code[cursor+1] != '~' && code[cursor+1] != '>'))
}

// rubyConstantStart excludes a constant that is the tail of a word, a method
// called on a receiver (`obj.Thing`), a symbol (`:Thing`) or a sigil variable.
func rubyConstantStart(code string, index int) bool {
	if index == 0 {
		return true
	}
	previous := code[index-1]
	if previous == ':' {
		return index >= 2 && code[index-2] == ':' && (index < 3 || !rubyWordByte(code[index-3]) && code[index-3] != ')' && code[index-3] != ']')
	}
	return !rubyWordByte(previous) && previous != '.' && previous != '@' && previous != '$'
}

// rubyReferenceAt reads `Foo` or `Foo::Bar` (each segment upper-case first).
func rubyReferenceAt(code string, index int) (string, int) {
	if index >= len(code) || !rubyUpperByte(code[index]) {
		return "", index
	}
	cursor := index
	for {
		_, after := rubyWord(code, cursor)
		cursor = after
		if cursor+2 < len(code) && code[cursor] == ':' && code[cursor+1] == ':' && rubyUpperByte(code[cursor+2]) {
			cursor += 2
			continue
		}
		return code[index:cursor], cursor
	}
}

// rubyLabelOrCall excludes `Thing: 1` (a hash label) and `Integer(x)` (a
// method named like a constant).
func rubyLabelOrCall(code string, end int) bool {
	if end >= len(code) {
		return false
	}
	if code[end] == '(' {
		return true
	}
	return code[end] == ':' && (end+1 >= len(code) || code[end+1] != ':')
}

// rubyStatementStart tells an `if`/`unless`/`while`/`until`/`for` that opens a
// block from a modifier: only the former follows nothing, or an operator or
// opening bracket, on its line.
func rubyStatementStart(code string, index int) bool {
	for cursor := index - 1; cursor >= 0; cursor-- {
		switch code[cursor] {
		case ' ', '\t':
			continue
		case ';', '=', '(', '[', '{', ',', '|', '&', '!':
			return true
		}
		return false
	}
	return true
}

// rubyEndlessDef reports Ruby 3's `def name(args) = expression`, which needs
// no `end`.
func rubyEndlessDef(code string, index int) bool {
	cursor, _ := rubySkipSpaces(code, index)
	receiver := rubySkipSigil(code, cursor)
	if word, after := rubyWord(code, receiver); word != "" && after < len(code) && code[after] == '.' {
		cursor = after + 1
	}
	matched := false
	for _, operator := range rubyOperatorMethods {
		if strings.HasPrefix(code[cursor:], operator) {
			cursor += len(operator)
			matched = true
			break
		}
	}
	if !matched {
		word, after := rubyWord(code, cursor)
		if word == "" {
			return false
		}
		cursor = after
		if cursor < len(code) && (code[cursor] == '?' || code[cursor] == '!') {
			cursor++
		} else if cursor+1 < len(code) && code[cursor] == '=' && code[cursor+1] == '(' {
			cursor++ // a setter `name=(value)`
		}
	}
	if cursor < len(code) && code[cursor] == '(' {
		depth := 0
		for ; cursor < len(code); cursor++ {
			if code[cursor] == '(' {
				depth++
			} else if code[cursor] == ')' {
				depth--
				if depth == 0 {
					cursor++
					break
				}
			}
		}
	}
	cursor, _ = rubySkipSpaces(code, cursor)
	return cursor < len(code) && code[cursor] == '=' &&
		(cursor+1 >= len(code) || (code[cursor+1] != '=' && code[cursor+1] != '~' && code[cursor+1] != '>'))
}

// resolutions are the constants a reference may name: Ruby's lexical lookup
// tries each enclosing namespace from the innermost, then the top level, and
// takes the first that exists, whether mapped by path or defined in the file
// itself (a local definition shadows an autoloaded constant). An unbalanced
// file is resolved against every namespace it opens. A `class`/`module` name
// resolves to the constant it opens, which reopening an autoloaded constant
// first loads. The unbalanced fallback tries at most rubyResolutionBound
// namespaces and reports false when the file opens more (NGI-V0-040).
func (autoload *rubyAutoload) resolutions(source *rubyConstantSource, reference rubyReference) ([]string, bool) {
	if reference.rooted {
		return []string{reference.name}, true
	}
	if reference.defined && source.balanced {
		return []string{reference.constant}, true
	}
	first, _, _ := strings.Cut(reference.name, "::")
	if !source.balanced {
		out := []string{reference.name}
		if reference.defined {
			out = append(out, reference.constant)
		}
		namespaces := source.namespaces
		complete := rubyResolutionBound.admits(len(namespaces))
		if !complete {
			namespaces = namespaces[:rubyResolutionBound.limit]
		}
		for _, namespace := range namespaces {
			if autoload.mapper.Known(namespace + "::" + first) {
				out = append(out, namespace+"::"+reference.name)
			}
		}
		return out, complete
	}
	stack := reference.stack
	for i := len(stack) - 1; i >= 0; i-- {
		if candidate := stack[i] + "::" + first; autoload.mapper.Known(candidate) || source.local[candidate] {
			return []string{stack[i] + "::" + reference.name}, true
		}
	}
	return []string{reference.name}, true
}

func rubyNamesConstant(resolved, constant string) bool {
	return resolved == constant || strings.HasPrefix(resolved, constant+"::")
}

// impact returns the named-constant reverse references to changedPath, their
// second hop, and its structured unknown rows (NGI-V0-018 to NGI-V0-020,
// NGI-V0-033 to NGI-V0-035).
func (autoload *rubyAutoload) impact(index *Index, changedPath string) ([]rankedResult, []any) {
	row := func(code string, count int) any {
		return map[string]any{"path": changedPath, "language": "ruby", "code": code, "line": 0, "count": count}
	}
	unknowns := []any{}
	if autoload.mapper.DeclarationInvalid() {
		unknowns = append(unknowns, row("ruby-autoload-declaration-invalid", 1))
	}
	if unresolved := autoload.mapper.ConfigUnresolved(); unresolved != 0 {
		unknowns = append(unknowns, row("ruby-autoload-config-unresolved", unresolved))
	}
	constant, mapped := autoload.mapper.Constant(changedPath)
	if !mapped {
		return nil, append(unknowns, row("ruby-constant-unmapped", 1))
	}
	last := constant[strings.LastIndex(constant, ":")+1:]
	lookups, literals, unreadable := 0, 0, 0
	for _, name := range autoload.paths {
		if name == changedPath {
			continue
		}
		source := autoload.sources[name]
		if !source.readable {
			unreadable++
			continue
		}
		lookups += source.lookups
		for _, literal := range source.literals {
			if strings.HasPrefix(literal.name, ":") {
				symbol := strings.TrimPrefix(literal.name, ":")
				singular, _ := autoload.mapper.Camelize(strings.TrimSuffix(symbol, "s"))
				plain, _ := autoload.mapper.Camelize(symbol)
				if plain == last || singular == last {
					literals++
				}
				continue
			}
			if literal.name == constant || literal.name == last || strings.HasSuffix(literal.name, "::"+last) {
				literals++
			}
		}
	}
	ranked := []rankedResult{}
	reached := map[string]bool{changedPath: true}
	first, firstLines, firstDropped := autoload.referrers(constant, reached)
	for _, name := range first {
		reached[name] = true
		item := autoload.result(index, name, firstLines[name], constant, changedPath, "")
		noteEvidenceTruncated(item.result, firstDropped[name])
		ranked = append(ranked, item)
	}
	// The second hop follows each first-hop referrer the mapper maps to its
	// own constant, once: a path a first hop or an earlier referrer already
	// reached is not reported again, and nothing goes past two hops.
	// Each constant is traversed once, however many first-hop paths map to it.
	traversed := map[string]bool{constant: true}
	for _, via := range first {
		viaConstant, ok := autoload.mapper.Constant(via)
		if !ok || traversed[viaConstant] {
			continue
		}
		traversed[viaConstant] = true
		second, secondLines, secondDropped := autoload.referrers(viaConstant, reached)
		for _, name := range second {
			reached[name] = true
			item := autoload.result(index, name, secondLines[name], viaConstant, via, constant)
			noteEvidenceTruncated(item.result, secondDropped[name])
			ranked = append(ranked, item)
		}
	}
	if lookups != 0 {
		unknowns = append(unknowns, row("ruby-constant-lookup-dynamic", lookups))
	}
	if literals != 0 {
		unknowns = append(unknowns, row("ruby-constant-name-literal", literals))
	}
	if unreadable != 0 {
		unknowns = append(unknowns, row("ruby-constant-source-unavailable", unreadable))
	}
	if bounded := autoload.cutReferences(changedPath, traversed); bounded != 0 {
		unknowns = append(unknowns, row("ruby-reference-index-bounded", bounded))
	}
	return ranked, unknowns
}

// cutReferences counts the cut references outside changedPath that could
// name one of the traversed constants, so the bounded unknown appears only on
// a changed path whose referrers a bound may have hidden (NGI-V0-064).
func (autoload *rubyAutoload) cutReferences(changedPath string, traversed map[string]bool) int {
	count := 0
	for _, cut := range autoload.cuts {
		if cut.path == changedPath {
			continue
		}
		for constant := range traversed {
			if autoload.cutNames(cut, constant) {
				count++
				break
			}
		}
	}
	return count
}

// cutNames reports whether a cut reference could name constant: a candidate
// whose sites the site bound left unrecorded names it or a namespace below it;
// a non-rooted reference in a file nested past the nesting bound could name
// anything, since its namespaces are unknown; and a candidate past the
// namespace bound, which resolutions never formed, names it.
func (autoload *rubyAutoload) cutNames(cut rubyCut, constant string) bool {
	if !cut.indexed {
		for _, candidate := range cut.resolved {
			if rubyNamesConstant(candidate, constant) {
				return true
			}
		}
	}
	if cut.reference.rooted {
		return false
	}
	if cut.source.overflowed {
		return true
	}
	if !cut.complete {
		first, _, _ := strings.Cut(cut.reference.name, "::")
		namespaces := cut.source.namespaces[min(rubyResolutionBound.limit, len(cut.source.namespaces)):]
		for _, namespace := range namespaces {
			if autoload.mapper.Known(namespace+"::"+first) && rubyNamesConstant(namespace+"::"+cut.reference.name, constant) {
				return true
			}
		}
	}
	return false
}

// referrers returns the paths, outside skip, with a reference naming
// constant, in path order, and each one's first maxEvidence reference lines,
// with the count of lines past that ceiling (MCPV0-112).
func (autoload *rubyAutoload) referrers(constant string, skip map[string]bool) ([]string, map[string][]int, map[string]int) {
	autoload.lookups++
	paths := []string{}
	lines := map[string][]int{}
	dropped := map[string]int{}
	for _, site := range autoload.sites[constant] {
		if skip[site.path] {
			continue
		}
		if _, seen := lines[site.path]; !seen {
			paths = append(paths, site.path)
		}
		if len(lines[site.path]) < maxEvidence {
			lines[site.path] = append(lines[site.path], site.line)
		} else {
			dropped[site.path]++
		}
	}
	return paths, lines, dropped
}

// result is one advisory named-constant reverse reference (NGI-V0-035). An
// empty origin marks a first hop to the changed path's constant; otherwise
// name references constant, the constant of via, which itself references
// origin, and the row ranks below every first hop (NGI-V0-034).
func (autoload *rubyAutoload) result(index *Index, name string, lines []int, constant, via, origin string) rankedResult {
	blob := index.Sources[name].BlobHash
	reason := "references constant " + constant + " autoloaded by path convention from " + via
	confidence := "medium"
	kind, score := "reference", rubyConstantReferenceScore
	summary := "named-constant reverse reference to " + constant + " (autoload convention; ranks below a direct import)"
	if origin != "" {
		reason += ", which references " + origin
		confidence = "low"
		score = rubyTransitiveReferenceScore
		summary = "second-hop named-constant reference to " + constant + ", which references " + origin +
			" (autoload convention; ranks below a first hop)"
	}
	if isRubyTestPath(name) {
		kind, score = "test", rubyConstantTestScore
		if origin != "" {
			score = rubyTransitiveTestScore
		}
	}
	rows := make([]any, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, evidence(name, line, blob, reason, confidence, "syntax"))
	}
	result := map[string]any{"kind": kind, "id": name, "score": score, "summary": summary, "evidence": rows, "advisory": true}
	if origin != "" {
		result["via"] = via
	}
	return rankedResult{score: score, order: 1, key: name, result: result}
}

// mapped reports whether the mapper resolves path to a constant (NGI-V0-036).
func (autoload *rubyAutoload) mapped(path string) bool {
	if autoload == nil {
		return false
	}
	_, ok := autoload.mapper.Constant(path)
	return ok
}

// report names the roots and inflections applied (NGI-V0-021).
func (autoload *rubyAutoload) report() map[string]any {
	applied := autoload.mapper.Report()
	roots := make([]any, 0, len(applied.Roots))
	for _, root := range applied.Roots {
		roots = append(roots, root)
	}
	inflections := make([]any, 0, len(applied.Inflections))
	for _, inflection := range applied.Inflections {
		inflections = append(inflections, map[string]any{"word": inflection.Word, "constant": inflection.Constant,
			"kind": inflection.Kind, "path": inflection.Path, "line": inflection.Line})
	}
	ignored := make([]any, 0, len(applied.Ignored))
	for _, directory := range applied.Ignored {
		ignored = append(ignored, directory)
	}
	report := map[string]any{"roots": roots, "ignored": ignored, "roots_source": applied.RootsSource, "inflections": inflections}
	if applied.Declaration != "" {
		report["declaration"] = applied.Declaration
	}
	return report
}

// isRubyTestPath is the Ruby autoload reading's test-path rule: the shared
// rule, plus RSpec's layout, a `_spec.rb` file or any Ruby file below a
// `spec/` directory, which holds its helpers, support files and shared
// examples (NGI-V0-038). Only the Ruby reading applies it, so the shared
// isTestPath, and every receipt for a request naming no Ruby path, keep
// their bytes.
func isRubyTestPath(value string) bool {
	name := strings.ToLower(path.Base(value))
	return isTestPath(value) || (strings.HasSuffix(name, ".rb") &&
		(strings.HasSuffix(name, "_spec.rb") || strings.Contains("/"+strings.ToLower(value)+"/", "/spec/")))
}

// withRubyTests returns a view of index whose rows classify paths with the
// Ruby reading's test-path rule, so an RSpec path is a test in every row of a
// request naming a Ruby path, not only in the autoload rows (NGI-V0-063). The
// view is a shallow copy; a request naming no Ruby path never takes it, so its
// bytes stay the base's.
func (index *Index) withRubyTests() *Index {
	view := *index
	view.rubyTests = true
	return &view
}

// isTest is the test-path rule of the request this index view serves.
func (index *Index) isTest(value string) bool {
	if index.rubyTests {
		return isRubyTestPath(value)
	}
	return isTestPath(value)
}

// rubyImpactRequested reports a request naming a Ruby path (NGI-V0-033).
// withRubyMapped returns a view of index naming the paths autoload maps, or
// index itself when it maps none. The view is a shallow copy, so the shared
// index is never written and a concurrent request never sees another's set.
func (index *Index) withRubyMapped(paths []string, autoload *rubyAutoload) *Index {
	mapped := map[string]struct{}{}
	for _, name := range paths {
		if autoload.mapped(name) {
			mapped[name] = struct{}{}
		}
	}
	if len(mapped) == 0 {
		return index
	}
	view := *index
	view.rubyMapped = mapped
	return &view
}

func rubyImpactRequested(paths []string) bool {
	for _, name := range paths {
		if NonGoImpactLanguage(name) == "ruby" {
			return true
		}
	}
	return false
}
