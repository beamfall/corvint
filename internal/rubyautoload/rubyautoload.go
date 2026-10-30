// Package rubyautoload maps Ruby source paths to the constants Rails and
// Zeitwerk autoload from them, and back (NGI-V0-016, NGI-V0-017, NGI-V0-039).
//
// A Rails application never requires the file that defines a model or
// service: the constant's name is derived from the file's path below an
// autoload root. A Mapper applies the documented default roots, or the
// committed `ruby-autoload/0` declaration at DeclarationPath, and the literal
// inflections the repository configures. It is syntax only: it evaluates no
// Ruby and loads no Rails configuration, so configuration it cannot read is
// counted, never guessed. The package reads nothing itself; the caller passes
// the repository's Ruby paths and a way to read a source, so the mapping is a
// pure function of those inputs.
package rubyautoload

import (
	"bytes"
	"encoding/json"
	"io"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	// DeclarationPath is the optional committed declaration of a repository's
	// autoload roots and inflections.
	DeclarationPath = ".corvint/ruby-autoload.json"
	// Schema is the declaration's closed schema name.
	Schema = "ruby-autoload/0"
	// MaxDeclarationBytes bounds the declaration's size.
	MaxDeclarationBytes = 64 << 10

	maxRoots       = 64
	maxInflections = 256
)

// Roots sources named by Report.RootsSource.
const (
	RootsDefault            = "default"
	RootsDeclared           = "declared"
	RootsDeclarationInvalid = "declaration-invalid"
)

var (
	inflectionWord     = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
	inflectionConstant = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]{0,63}$`)
	acronymCall        = regexp.MustCompile(`\.acronym\b`)
	acronymLiteral     = regexp.MustCompile(`^\.acronym\s*(?:\(\s*(['"])([A-Za-z0-9]{1,64})['"]\s*\)|\s(['"])([A-Za-z0-9]{1,64})['"])\s*(?:#.*)?$`)
	inflectCall        = regexp.MustCompile(`\.inflect\s*\(`)
	inflectPair        = regexp.MustCompile(`^\s*(['"])([a-z0-9_]{1,64})['"]\s*=>\s*(['"])([A-Z][A-Za-z0-9_]{0,63})['"]\s*(?:,|$)`)
	autoloadRootConfig = regexp.MustCompile(`\b(?:autoload_paths|eager_load_paths|autoload_once_paths|autoload_lib|autoload_lib_once|push_dir)\b`)
	collapseConfig     = regexp.MustCompile(`\.collapse\b`)
	// stockAutoloadLib is the generated Rails 7.1+ line, alone on its line
	// except for a trailing comment, with a literal %w ignore list.
	stockAutoloadLib = regexp.MustCompile(`^\s*config\.autoload_lib\s*\(\s*ignore:\s*%w(?:\[([a-z0-9_ ]*)\]|\(([a-z0-9_ ]*)\))\s*\)\s*(?:#.*)?$`)
)

// defaultIgnored are the default `lib` subdirectories Rails' `autoload_lib`
// ignores: they hold no autoloaded constants.
var defaultIgnored = []string{"lib/assets", "lib/tasks"}

// Repository is everything a Mapper reads.
type Repository struct {
	// Paths are the repository's Ruby source paths, slash-separated and
	// relative to its root. Order does not matter.
	Paths []string
	// DeclarationPresent reports that DeclarationPath is tracked;
	// DeclarationText is its content when DeclarationReadable. A present
	// declaration that is unreadable, or not one strict JSON document (no
	// duplicate members), must be passed with DeclarationReadable false: it
	// then maps no path.
	DeclarationPresent  bool
	DeclarationReadable bool
	DeclarationText     string
	// Source returns a non-test Ruby source's raw lines and its lexically
	// masked code lines (comments, strings and heredocs blanked, columns
	// preserved), or false when it cannot be read. It is called only to
	// read inflection and autoload configuration.
	Source func(path string) (raw, code []string, ok bool)
	// IsTest reports a test path, whose configuration is not read.
	IsTest func(path string) bool
}

// Inflection is one applied inflection: a Rails acronym (a word inside a
// basename), a Zeitwerk `inflect` override or a declared override (a whole
// basename). Line is 0 for a declared entry.
type Inflection struct {
	Word, Constant, Kind, Path string
	Line                       int
}

// Report names what a Mapper applied (NGI-V0-021).
type Report struct {
	Roots       []string
	Ignored     []string
	RootsSource string
	// Declaration is DeclarationPath when a declaration is present, valid or not.
	Declaration string
	Inflections []Inflection
}

// Mapper is an immutable path-to-constant map over one repository snapshot.
type Mapper struct {
	roots              []string
	ignored            []string
	rootsSource        string
	inflections        []Inflection
	acronyms           map[string]string
	overrides          map[string]string
	declarationInvalid bool
	declared           bool
	configUnresolved   int
	configSites        []string
	autoloadLib        bool
	constants          map[string]string
	paths              map[string]string
	known              map[string]bool
}

type declaration struct {
	Schema      string            `json:"schema"`
	Roots       []string          `json:"roots"`
	Inflections map[string]string `json:"inflections"`
}

// New reads the declaration or the default roots and the literal inflections
// the repository configures, then maps every Ruby path to its constant.
func New(repository Repository) *Mapper {
	mapper := &Mapper{acronyms: map[string]string{}, overrides: map[string]string{},
		constants: map[string]string{}, paths: map[string]string{}, known: map[string]bool{}}
	paths := append([]string(nil), repository.Paths...)
	sort.Strings(paths)
	decoded, valid := readDeclaration(repository)
	mapper.declared = repository.DeclarationPresent && valid
	mapper.declarationInvalid = repository.DeclarationPresent && !valid
	if mapper.declarationInvalid {
		// A declaration that cannot be read maps nothing: falling back to
		// the defaults would replace the adopter's stated roots silently.
		mapper.rootsSource = RootsDeclarationInvalid
		mapper.roots = []string{}
		return mapper
	}
	if mapper.declared {
		mapper.rootsSource = RootsDeclared
		mapper.roots = append([]string(nil), decoded.Roots...)
		words := make([]string, 0, len(decoded.Inflections))
		for word := range decoded.Inflections {
			words = append(words, word)
		}
		sort.Strings(words)
		for _, word := range words {
			mapper.addInflection(Inflection{word, decoded.Inflections[word], "declared", DeclarationPath, 0})
		}
	} else {
		mapper.rootsSource = RootsDefault
		mapper.roots = DefaultRoots(paths)
		mapper.ignored = append([]string(nil), defaultIgnored...)
	}
	sort.Strings(mapper.roots)
	mapper.readConfiguredInflections(repository, paths)
	for _, name := range paths {
		constant, ok := mapper.constantFor(name)
		if !ok {
			continue
		}
		mapper.constants[name] = constant
		if _, taken := mapper.paths[constant]; !taken {
			mapper.paths[constant] = name
		}
		for prefix := constant; prefix != ""; {
			mapper.known[prefix] = true
			cut := strings.LastIndex(prefix, "::")
			if cut < 0 {
				break
			}
			prefix = prefix[:cut]
		}
	}
	return mapper
}

func readDeclaration(repository Repository) (declaration, bool) {
	var decoded declaration
	if !repository.DeclarationPresent || !repository.DeclarationReadable || len(repository.DeclarationText) > MaxDeclarationBytes {
		return decoded, false
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(repository.DeclarationText)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&decoded) != nil || decoder.Decode(new(any)) != io.EOF {
		return decoded, false
	}
	return decoded, validDeclaration(decoded)
}

func validDeclaration(decoded declaration) bool {
	if decoded.Schema != Schema || len(decoded.Roots) == 0 || len(decoded.Roots) > maxRoots ||
		len(decoded.Inflections) > maxInflections {
		return false
	}
	seen := map[string]bool{}
	for _, root := range decoded.Roots {
		first, _, _ := strings.Cut(root, "/")
		if root == "" || root == "." || root == ".." || path.Clean(root) != root || path.IsAbs(root) || strings.HasPrefix(root, "../") ||
			strings.Contains(root, "\\") || strings.EqualFold(first, ".git") || seen[root] {
			return false
		}
		seen[root] = true
	}
	for word, constant := range decoded.Inflections {
		if !inflectionWord.MatchString(word) || !inflectionConstant.MatchString(constant) {
			return false
		}
	}
	return true
}

// DefaultRoots is the documented default root set over a repository's Ruby
// paths: each `app/<dir>` holding Ruby source except Rails' non-autoloaded
// assets, javascript and views, each `app/<dir>/concerns`, and `lib`.
func DefaultRoots(paths []string) []string {
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
	values := make([]string, 0, len(roots))
	for value := range roots {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func (mapper *Mapper) addInflection(inflection Inflection) {
	table := mapper.overrides
	if inflection.Kind == "acronym" {
		table = mapper.acronyms
	}
	if _, ok := table[inflection.Word]; ok {
		return
	}
	table[inflection.Word] = inflection.Constant
	mapper.inflections = append(mapper.inflections, inflection)
}

// readConfiguredInflections reads literal `inflect.acronym "API"` and Zeitwerk
// `inflector.inflect("api" => "API")` calls in non-test Ruby sources. Any other
// form, and any autoload-root or collapse configuration the declaration does
// not stand in for, is counted as unresolved configuration.
func (mapper *Mapper) readConfiguredInflections(repository Repository, paths []string) {
	for _, name := range paths {
		if repository.Source == nil || (repository.IsTest != nil && repository.IsTest(name)) {
			continue
		}
		raw, masked, ok := repository.Source(name)
		if !ok {
			continue
		}
		for i, code := range masked {
			if i >= len(raw) {
				break
			}
			if !mapper.declared && autoloadRootConfig.MatchString(code) && !mapper.readStockAutoloadLib(raw[i]) {
				mapper.unresolvedAt(name, i+1)
			}
			if collapseConfig.MatchString(code) {
				mapper.unresolvedAt(name, i+1)
			}
			for _, loc := range acronymCall.FindAllStringIndex(code, -1) {
				match := acronymLiteral.FindStringSubmatch(raw[i][loc[0]:])
				if match == nil {
					mapper.unresolvedAt(name, i+1)
					continue
				}
				acronym := match[2] + match[4]
				mapper.addInflection(Inflection{strings.ToLower(acronym), acronym, "acronym", name, i + 1})
			}
			for _, loc := range inflectCall.FindAllStringIndex(code, -1) {
				pairs, ok := inflectPairs(raw, i, loc[1])
				if !ok {
					mapper.unresolvedAt(name, i+1)
					continue
				}
				for _, pair := range pairs {
					mapper.addInflection(Inflection{pair[0], pair[1], "inflect", name, i + 1})
				}
			}
		}
	}
	sort.SliceStable(mapper.inflections, func(left, right int) bool {
		a, b := mapper.inflections[left], mapper.inflections[right]
		if a.Word != b.Word {
			return a.Word < b.Word
		}
		return a.Kind < b.Kind
	})
}

// readStockAutoloadLib applies the generated Rails 7.1+
// `config.autoload_lib(ignore: %w[assets tasks])` line: `lib` becomes an
// autoload root and exactly the listed `lib` subdirectories are ignored, as
// Rails does. Any other spelling, including a non-literal ignore list, a second
// such line or autoload_lib_once, reports false and stays unresolved
// (NGI-V0-021, V1-1850).
func (mapper *Mapper) readStockAutoloadLib(line string) bool {
	match := stockAutoloadLib.FindStringSubmatch(line)
	if match == nil || mapper.autoloadLib {
		return false
	}
	words := strings.Fields(match[1] + match[2])
	ignored := make([]string, 0, len(words))
	for _, word := range words {
		ignored = append(ignored, "lib/"+word)
	}
	sort.Strings(ignored)
	mapper.autoloadLib = true
	mapper.ignored = slices.Compact(ignored)
	if !slices.Contains(mapper.roots, "lib") {
		mapper.roots = append(mapper.roots, "lib")
		sort.Strings(mapper.roots)
	}
	return true
}

// inflectPairs reads the literal `"word" => "Constant"` pairs of one
// `.inflect(` call, which may span up to 64 lines, up to its closing `)`.
func inflectPairs(raw []string, line, column int) ([][2]string, bool) {
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
		match := inflectPair.FindStringSubmatch(body)
		if match == nil {
			return nil, false
		}
		pairs = append(pairs, [2]string{match[2], match[4]})
		body = strings.TrimSpace(body[len(match[0]):])
	}
	return pairs, len(pairs) != 0
}

// constantFor maps a Ruby path below its longest autoload root to a constant.
func (mapper *Mapper) constantFor(name string) (string, bool) {
	root := ""
	for _, candidate := range mapper.roots {
		if strings.HasPrefix(name, candidate+"/") && len(candidate) > len(root) {
			root = candidate
		}
	}
	if root == "" || !strings.HasSuffix(name, ".rb") {
		return "", false
	}
	for _, ignored := range mapper.ignored {
		if strings.HasPrefix(name, ignored+"/") {
			return "", false
		}
	}
	segments := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, root+"/"), ".rb"), "/")
	for i, segment := range segments {
		constant, ok := mapper.Camelize(segment)
		if !ok {
			return "", false
		}
		segments[i] = constant
	}
	return strings.Join(segments, "::"), true
}

// Camelize is ActiveSupport's camelize over one file or directory basename,
// after a whole-basename override and with acronyms applied per word. A
// segment that is not lower snake case, or that does not camelize to a
// constant name, reports false.
func (mapper *Mapper) Camelize(segment string) (string, bool) {
	if override, ok := mapper.overrides[segment]; ok {
		return override, true
	}
	if !inflectionWord.MatchString(segment) || segment[0] < 'a' || segment[0] > 'z' {
		return "", false
	}
	var out strings.Builder
	for _, word := range strings.Split(segment, "_") {
		if word == "" {
			return "", false
		}
		if acronym, ok := mapper.acronyms[word]; ok {
			out.WriteString(acronym)
			continue
		}
		out.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	return out.String(), inflectionConstant.MatchString(out.String())
}

// Constant returns the constant a mapped Ruby path names, such as
// `Bar::BazService` for `app/services/bar/baz_service.rb`.
func (mapper *Mapper) Constant(path string) (string, bool) {
	constant, ok := mapper.constants[path]
	return constant, ok
}

// Path returns the mapped path that defines constant. When several roots map
// paths to the same constant, the first path in byte order is returned.
func (mapper *Mapper) Path(constant string) (string, bool) {
	name, ok := mapper.paths[constant]
	return name, ok
}

// Known reports whether name is a mapped constant or a namespace prefix of
// one (`Bar` for `Bar::BazService`), the names lexical lookup can resolve to.
func (mapper *Mapper) Known(name string) bool {
	return mapper.known[name]
}

// DeclarationInvalid reports a present declaration that does not validate;
// such a repository maps no path.
func (mapper *Mapper) DeclarationInvalid() bool {
	return mapper.declarationInvalid
}

// unresolvedAt counts one autoload configuration line the mapper does not
// model and records where it is (RUBY-AFFECTED-V0-046).
func (mapper *Mapper) unresolvedAt(name string, line int) {
	mapper.configUnresolved++
	mapper.configSites = append(mapper.configSites, name+":"+strconv.Itoa(line))
}

// UnresolvedConfigSites names, as "path:line" in the order read, every
// configuration line ConfigUnresolved counts (RUBY-AFFECTED-V0-046).
func (mapper *Mapper) UnresolvedConfigSites() []string {
	return append([]string(nil), mapper.configSites...)
}

// ConfigUnresolved counts autoload-root, collapse and inflection
// configuration sites that were not literal or are not modelled.
func (mapper *Mapper) ConfigUnresolved() int {
	return mapper.configUnresolved
}

// Report names the roots, ignored directories and inflections applied.
func (mapper *Mapper) Report() Report {
	report := Report{Roots: append([]string{}, mapper.roots...), Ignored: append([]string{}, mapper.ignored...),
		RootsSource: mapper.rootsSource, Inflections: append([]Inflection{}, mapper.inflections...)}
	if mapper.declared || mapper.declarationInvalid {
		report.Declaration = DeclarationPath
	}
	return report
}
