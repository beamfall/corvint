package contextindex

import (
	"bytes"
	"encoding/json"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Ruby autoload-convention reverse references (NGI-V0-016 to NGI-V0-021).
//
// A Rails/Zeitwerk application never requires the file that defines a model or
// service: the constant's name is derived from the file's path below an
// autoload root, and any code naming the constant loads it. This maps each
// Ruby source below a root to the constant its path names, indexes constant
// references (fully qualified and namespace-relative, resolved through the
// file's lexical `module`/`class` nesting) and reports a file naming the
// changed path's constant as a named-constant reverse reference. It is syntax
// only: it evaluates nothing, so reflective lookups and constant names spelled
// as strings or association symbols stay counted unknowns, and Ruby's
// ancestor-chain constant lookup is not modelled.

const (
	// RubyAutoloadDeclarationPath is the optional committed declaration of a
	// repository's autoload roots and inflections (NGI-V0-017).
	RubyAutoloadDeclarationPath = ".corvint/ruby-autoload.json"
	rubyAutoloadSchema          = "ruby-autoload/0"
	maxRubyAutoloadBytes        = 64 << 10
	maxRubyAutoloadRoots        = 64
	maxRubyInflections          = 256
	maxRubyNestingDepth         = 256

	// Named-constant reverse references rank below every direct import
	// (700 code, 650 test) and every type-only import (600, 550): they are
	// a convention, not a load statement the file carries (NGI-V0-018).
	rubyConstantReferenceScore = 525
	rubyConstantTestScore      = 500
)

var (
	rubyInflectionWord     = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
	rubyInflectionConstant = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]{0,63}$`)
	rubyAcronymCall        = regexp.MustCompile(`\.acronym\b`)
	rubyAcronymLiteral     = regexp.MustCompile(`^\.acronym\s*(?:\(\s*(['"])([A-Za-z0-9]{1,64})['"]\s*\)|\s(['"])([A-Za-z0-9]{1,64})['"])\s*(?:#.*)?$`)
	rubyInflectCall        = regexp.MustCompile(`\.inflect\s*\(`)
	rubyInflectPair        = regexp.MustCompile(`^\s*(['"])([a-z0-9_]{1,64})['"]\s*=>\s*(['"])([A-Z][A-Za-z0-9_]{0,63})['"]\s*(?:,|$)`)
	rubyAutoloadRootConfig = regexp.MustCompile(`\b(?:autoload_paths|eager_load_paths|autoload_once_paths|autoload_lib|autoload_lib_once|push_dir)\b`)
	rubyCollapseConfig     = regexp.MustCompile(`\.collapse\b`)
	rubyDynamicLookup      = regexp.MustCompile(`\b(?:constantize|safe_constantize|const_get|send|public_send|__send__)\b`)
	rubyStringConstant     = regexp.MustCompile(`(['"])(?:::)?([A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*)['"]`)
	rubyAssociation        = regexp.MustCompile(`^\s*(?:belongs_to|has_one|has_many|has_and_belongs_to_many)\s*\(?\s*:([a-z_][a-z0-9_]*)`)
)

// rubyAutoloadDeclaration is the closed ruby-autoload/0 document.
type rubyAutoloadDeclaration struct {
	Schema      string            `json:"schema"`
	Roots       []string          `json:"roots"`
	Inflections map[string]string `json:"inflections"`
}

// rubyInflection is one applied inflection: a Rails acronym (a word inside a
// basename) or a Zeitwerk/declared override (a whole basename).
type rubyInflection struct {
	word, constant, kind, path string
	line                       int
}

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
	lookups    int
	literals   []rubyLiteralName
	readable   bool
}

type rubyLiteralName struct {
	line int
	name string
}

type rubyAutoload struct {
	roots              []string
	ignored            []string
	rootsSource        string
	inflections        []rubyInflection
	acronyms           map[string]string
	overrides          map[string]string
	declarationInvalid bool
	declared           bool
	configUnresolved   int
	constants          map[string]string
	known              map[string]bool
	sources            map[string]*rubyConstantSource
	paths              []string
}

// loadRubyAutoload reads the declaration or the default roots, the literal
// inflections the repository configures, maps every Ruby source to its
// constant and indexes every Ruby source's constant references once.
func loadRubyAutoload(index *Index) *rubyAutoload {
	autoload := &rubyAutoload{acronyms: map[string]string{}, overrides: map[string]string{},
		constants: map[string]string{}, known: map[string]bool{}, sources: map[string]*rubyConstantSource{}}
	for name := range index.Sources {
		if NonGoImpactLanguage(name) == "ruby" {
			autoload.paths = append(autoload.paths, name)
		}
	}
	sort.Strings(autoload.paths)
	declaration, present, valid := readRubyAutoloadDeclaration(index)
	autoload.declared = present && valid
	autoload.declarationInvalid = present && !valid
	if autoload.declarationInvalid {
		// A declaration that cannot be read maps nothing: falling back to
		// the defaults would replace the adopter's stated roots silently.
		autoload.rootsSource = "declaration-invalid"
		autoload.roots = []string{}
		return autoload
	}
	if autoload.declared {
		autoload.rootsSource = "declared"
		autoload.roots = append([]string(nil), declaration.Roots...)
		words := make([]string, 0, len(declaration.Inflections))
		for word := range declaration.Inflections {
			words = append(words, word)
		}
		sort.Strings(words)
		for _, word := range words {
			autoload.addInflection(rubyInflection{word, declaration.Inflections[word], "declared", RubyAutoloadDeclarationPath, 0})
		}
	} else {
		autoload.rootsSource = "default"
		autoload.roots = defaultRubyAutoloadRoots(autoload.paths)
		autoload.ignored = append([]string(nil), defaultRubyAutoloadIgnored...)
	}
	sort.Strings(autoload.roots)
	autoload.readConfiguredInflections(index)
	for _, name := range autoload.paths {
		if constant, ok := autoload.constantFor(name); ok {
			autoload.constants[name] = constant
			for prefix := constant; prefix != ""; {
				autoload.known[prefix] = true
				cut := strings.LastIndex(prefix, "::")
				if cut < 0 {
					break
				}
				prefix = prefix[:cut]
			}
		}
	}
	for _, name := range autoload.paths {
		autoload.sources[name] = scanRubyConstantSource(index.Sources[name], autoload.known)
	}
	return autoload
}

func readRubyAutoloadDeclaration(index *Index) (rubyAutoloadDeclaration, bool, bool) {
	var declaration rubyAutoloadDeclaration
	source, exists := index.Sources[RubyAutoloadDeclarationPath]
	if !exists {
		for _, exclusion := range index.Exclusions {
			if exclusion.Path == RubyAutoloadDeclarationPath {
				return declaration, true, false
			}
		}
		return declaration, false, false
	}
	text, valid, loaded := source.Text()
	if !valid || !loaded || len(text) > maxRubyAutoloadBytes || !strictJSON(text) {
		return declaration, true, false
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(text)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&declaration) != nil || decoder.Decode(new(any)) != io.EOF {
		return declaration, true, false
	}
	return declaration, true, validRubyAutoloadDeclaration(declaration)
}

func validRubyAutoloadDeclaration(declaration rubyAutoloadDeclaration) bool {
	if declaration.Schema != rubyAutoloadSchema || len(declaration.Roots) == 0 || len(declaration.Roots) > maxRubyAutoloadRoots ||
		len(declaration.Inflections) > maxRubyInflections {
		return false
	}
	seen := map[string]bool{}
	for _, root := range declaration.Roots {
		first, _, _ := strings.Cut(root, "/")
		if root == "" || root == "." || root == ".." || path.Clean(root) != root || path.IsAbs(root) || strings.HasPrefix(root, "../") ||
			strings.Contains(root, "\\") || strings.EqualFold(first, ".git") || seen[root] {
			return false
		}
		seen[root] = true
	}
	for word, constant := range declaration.Inflections {
		if !rubyInflectionWord.MatchString(word) || !rubyInflectionConstant.MatchString(constant) {
			return false
		}
	}
	return true
}

// defaultRubyAutoloadIgnored are the default `lib` subdirectories Rails'
// `autoload_lib` ignores: they hold no autoloaded constants.
var defaultRubyAutoloadIgnored = []string{"lib/assets", "lib/tasks"}

// defaultRubyAutoloadRoots is the documented default set: each `app/<dir>`
// holding Ruby source except Rails' non-autoloaded assets, javascript and
// views, each `app/<dir>/concerns`, and `lib` (NGI-V0-017).
func defaultRubyAutoloadRoots(paths []string) []string {
	roots := map[string]bool{}
	for _, name := range paths {
		parts := strings.Split(name, "/")
		switch {
		case len(parts) >= 3 && parts[0] == "app" && parts[1] != "assets" && parts[1] != "javascript" && parts[1] != "views":
			roots["app/"+parts[1]] = true
			if len(parts) >= 4 && parts[2] == "concerns" {
				roots["app/"+parts[1]+"/concerns"] = true
			}
		case len(parts) >= 2 && parts[0] == "lib" && parts[1] != "assets" && parts[1] != "tasks":
			roots["lib"] = true
		}
	}
	return sortedRubySet(roots)
}

func sortedRubySet(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func (autoload *rubyAutoload) addInflection(inflection rubyInflection) {
	if inflection.kind == "acronym" {
		if _, ok := autoload.acronyms[inflection.word]; ok {
			return
		}
		autoload.acronyms[inflection.word] = inflection.constant
	} else {
		if _, ok := autoload.overrides[inflection.word]; ok {
			return
		}
		autoload.overrides[inflection.word] = inflection.constant
	}
	autoload.inflections = append(autoload.inflections, inflection)
}

// readConfiguredInflections reads literal `inflect.acronym "API"` and Zeitwerk
// `inflector.inflect("api" => "API")` calls in non-test Ruby sources. Any other
// form, and any autoload-root or collapse configuration the declaration does
// not stand in for, is counted as unresolved configuration.
func (autoload *rubyAutoload) readConfiguredInflections(index *Index) {
	for _, name := range autoload.paths {
		if rubyTestPath(name) {
			continue
		}
		text, valid, loaded := index.Sources[name].Text()
		if !valid || !loaded {
			continue
		}
		masked := FlowCodeLines("ruby", text)
		raw := strings.Split(text, "\n")
		for i, code := range masked {
			if !autoload.declared && rubyAutoloadRootConfig.MatchString(code) {
				autoload.configUnresolved++
			}
			if rubyCollapseConfig.MatchString(code) {
				autoload.configUnresolved++
			}
			for _, loc := range rubyAcronymCall.FindAllStringIndex(code, -1) {
				match := rubyAcronymLiteral.FindStringSubmatch(raw[i][loc[0]:])
				if match == nil {
					autoload.configUnresolved++
					continue
				}
				acronym := match[2] + match[4]
				autoload.addInflection(rubyInflection{strings.ToLower(acronym), acronym, "acronym", name, i + 1})
			}
			for _, loc := range rubyInflectCall.FindAllStringIndex(code, -1) {
				pairs, ok := rubyInflectPairs(raw, i, loc[1])
				if !ok {
					autoload.configUnresolved++
					continue
				}
				for _, pair := range pairs {
					autoload.addInflection(rubyInflection{pair[0], pair[1], "inflect", name, i + 1})
				}
			}
		}
	}
	sort.SliceStable(autoload.inflections, func(left, right int) bool {
		a, b := autoload.inflections[left], autoload.inflections[right]
		if a.word != b.word {
			return a.word < b.word
		}
		return a.kind < b.kind
	})
}

// rubyInflectPairs reads the literal `"word" => "Constant"` pairs of one
// `.inflect(` call, which may span up to 64 lines, up to its closing `)`.
func rubyInflectPairs(raw []string, line, column int) ([][2]string, bool) {
	body := raw[line][column:]
	for next := line + 1; !strings.Contains(body, ")") && next < len(raw) && next <= line+64; next++ {
		body += "\n" + raw[next]
	}
	end := strings.Index(body, ")")
	if end < 0 {
		return nil, false
	}
	body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body[:end]), ","))
	pairs := [][2]string{}
	for body != "" {
		match := rubyInflectPair.FindStringSubmatch(body)
		if match == nil {
			return nil, false
		}
		pairs = append(pairs, [2]string{match[2], match[4]})
		body = strings.TrimSpace(body[len(match[0]):])
	}
	return pairs, len(pairs) != 0
}

// constantFor maps a Ruby path below its longest autoload root to a constant.
func (autoload *rubyAutoload) constantFor(name string) (string, bool) {
	root := ""
	for _, candidate := range autoload.roots {
		if strings.HasPrefix(name, candidate+"/") && len(candidate) > len(root) {
			root = candidate
		}
	}
	if root == "" || !strings.HasSuffix(name, ".rb") {
		return "", false
	}
	for _, ignored := range autoload.ignored {
		if strings.HasPrefix(name, ignored+"/") {
			return "", false
		}
	}
	segments := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, root+"/"), ".rb"), "/")
	for i, segment := range segments {
		constant, ok := autoload.camelize(segment)
		if !ok {
			return "", false
		}
		segments[i] = constant
	}
	return strings.Join(segments, "::"), true
}

// camelize is ActiveSupport's camelize over a file or directory basename,
// after a whole-basename override and with acronyms applied per word.
func (autoload *rubyAutoload) camelize(segment string) (string, bool) {
	if override, ok := autoload.overrides[segment]; ok {
		return override, true
	}
	if !rubyInflectionWord.MatchString(segment) || segment[0] < 'a' || segment[0] > 'z' {
		return "", false
	}
	var out strings.Builder
	for _, word := range strings.Split(segment, "_") {
		if word == "" {
			return "", false
		}
		if acronym, ok := autoload.acronyms[word]; ok {
			out.WriteString(acronym)
			continue
		}
		out.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	return out.String(), rubyInflectionConstant.MatchString(out.String())
}

func rubyTestPath(name string) bool {
	wrapped := "/" + name
	return isTestPath(name) || strings.Contains(wrapped, "/spec/") || strings.HasSuffix(name, "_spec.rb")
}

// scanRubyConstantSource indexes one source's constant references with the
// lexical namespaces enclosing each. Blocks opened by keywords that need an
// `end` are tracked so a namespace closes with its `end`; a file whose
// openers and `end`s do not balance, or that nests deeper than
// maxRubyNestingDepth blocks, is marked, and its references are then resolved
// against every namespace it opens, never fewer. known holds the mapped
// constants and their namespace prefixes; together with the constants this
// file defines it resolves the owner of a qualified `class A::B` name.
func scanRubyConstantSource(source Source, known map[string]bool) *rubyConstantSource {
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
		if len(frames) >= maxRubyNestingDepth {
			overflow++
			result.balanced = false
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
				case !rooted && !strings.Contains(name, "::") && rubyAssignsConstant(code, end):
					// `Thing = ...` defines a constant in the innermost
					// namespace; it is not a reference to an autoloaded one.
					if outer := rubyInnermost(lexical); outer != "" {
						result.define(outer + "::" + name)
					} else {
						result.define(name)
					}
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
func rubyDefinedConstant(name string, lexical []string, known, local map[string]bool) string {
	first, _, qualified := strings.Cut(name, "::")
	if !qualified {
		if outer := rubyInnermost(lexical); outer != "" {
			return outer + "::" + name
		}
		return name
	}
	for i := len(lexical) - 1; i >= 0; i-- {
		if candidate := lexical[i] + "::" + first; known[candidate] || local[candidate] {
			return lexical[i] + "::" + name
		}
	}
	return name
}

// rubyAssignsConstant reports `Name = value`, never `==`, `=~` or `=>`.
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
// first loads.
func (autoload *rubyAutoload) resolutions(source *rubyConstantSource, reference rubyReference) []string {
	if reference.rooted {
		return []string{reference.name}
	}
	if reference.defined && source.balanced {
		return []string{reference.constant}
	}
	first, _, _ := strings.Cut(reference.name, "::")
	if !source.balanced {
		out := []string{reference.name}
		if reference.defined {
			out = append(out, reference.constant)
		}
		for _, namespace := range source.namespaces {
			if autoload.known[namespace+"::"+first] {
				out = append(out, namespace+"::"+reference.name)
			}
		}
		return out
	}
	stack := reference.stack
	for i := len(stack) - 1; i >= 0; i-- {
		if candidate := stack[i] + "::" + first; autoload.known[candidate] || source.local[candidate] {
			return []string{stack[i] + "::" + reference.name}
		}
	}
	return []string{reference.name}
}

func rubyNamesConstant(resolved, constant string) bool {
	return resolved == constant || strings.HasPrefix(resolved, constant+"::")
}

// impact returns the named-constant reverse references to changedPath and its
// structured unknown rows (NGI-V0-018 to NGI-V0-020).
func (autoload *rubyAutoload) impact(index *Index, changedPath string) ([]rankedResult, []any) {
	row := func(code string, count int) any {
		return map[string]any{"path": changedPath, "language": "ruby", "code": code, "line": 0, "count": count}
	}
	unknowns := []any{}
	if autoload.declarationInvalid {
		unknowns = append(unknowns, row("ruby-autoload-declaration-invalid", 1))
	}
	if autoload.configUnresolved != 0 {
		unknowns = append(unknowns, row("ruby-autoload-config-unresolved", autoload.configUnresolved))
	}
	constant, mapped := autoload.constants[changedPath]
	if !mapped {
		return nil, append(unknowns, row("ruby-constant-unmapped", 1))
	}
	last := constant[strings.LastIndex(constant, ":")+1:]
	ranked := []rankedResult{}
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
				singular, _ := autoload.camelize(strings.TrimSuffix(symbol, "s"))
				plain, _ := autoload.camelize(symbol)
				if plain == last || singular == last {
					literals++
				}
				continue
			}
			if literal.name == constant || literal.name == last || strings.HasSuffix(literal.name, "::"+last) {
				literals++
			}
		}
		rows := []any{}
		blob := index.Sources[name].BlobHash
		for _, reference := range source.references {
			for _, resolved := range autoload.resolutions(source, reference) {
				if rubyNamesConstant(resolved, constant) {
					if len(rows) < maxEvidence {
						rows = append(rows, evidence(name, reference.line, blob,
							"references constant "+constant+" autoloaded by path convention from "+changedPath, "medium", "syntax"))
					}
					break
				}
			}
		}
		if len(rows) == 0 {
			continue
		}
		kind, score := "reference", rubyConstantReferenceScore
		if rubyTestPath(name) {
			kind, score = "test", rubyConstantTestScore
		}
		ranked = append(ranked, rankedResult{score: score, order: 1, key: name, result: map[string]any{
			"kind": kind, "id": name, "score": score,
			"summary":  "named-constant reverse reference to " + constant + " (autoload convention; ranks below a direct import)",
			"evidence": rows,
		}})
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
	return ranked, unknowns
}

// report names the roots and inflections applied (NGI-V0-021).
func (autoload *rubyAutoload) report() map[string]any {
	roots := make([]any, 0, len(autoload.roots))
	for _, root := range autoload.roots {
		roots = append(roots, root)
	}
	inflections := make([]any, 0, len(autoload.inflections))
	for _, inflection := range autoload.inflections {
		inflections = append(inflections, map[string]any{"word": inflection.word, "constant": inflection.constant,
			"kind": inflection.kind, "path": inflection.path, "line": inflection.line})
	}
	ignored := make([]any, 0, len(autoload.ignored))
	for _, directory := range autoload.ignored {
		ignored = append(ignored, directory)
	}
	report := map[string]any{"roots": roots, "ignored": ignored, "roots_source": autoload.rootsSource, "inflections": inflections}
	if autoload.declared || autoload.declarationInvalid {
		report["declaration"] = RubyAutoloadDeclarationPath
	}
	return report
}
