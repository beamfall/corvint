package ruby

import (
	json "encoding/json/v2"
	"errors"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/rubyautoload"
)

var (
	// constantReference is one constant path as written, with an optional
	// leading `::` naming the top level (RUBY-AFFECTED-V0-009).
	constantReference = regexp.MustCompile(`(?:::)?[A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*`)
	namespaceOpening  = regexp.MustCompile(`(?m)^[\t ]*(?:class|module)[\t ]+(?:::)?([A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*)`)
	// dynamicConstantLookup names a constant computed at run time, which no
	// edge can bound (RUBY-AFFECTED-V0-010).
	dynamicConstantLookup = regexp.MustCompile(`\b(?:constantize|safe_constantize|const_get)\b`)
	association           = regexp.MustCompile(`^[\t ]*(?:belongs_to|has_one|has_many|has_and_belongs_to_many)[\t ]*\(?[\t ]*:([a-z_][a-z0-9_]*)`)
	associationClassName  = regexp.MustCompile(`(?:\bclass_name:|:class_name[\t ]*=>)[\t ]*["'](?:::)?([A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*)["']`)
	associationDynamic    = regexp.MustCompile(`\bpolymorphic:|:polymorphic[\t ]*=>|\bclass_name:|:class_name[\t ]*=>`)
	routeDispatchedType   = regexp.MustCompile(`\btype:[\t ]*:(?:request|system|feature|routing|controller|integration)\b`)
)

// routeDispatchedDirectories are the test directories whose tests reach the
// application through routing rather than a constant (RUBY-AFFECTED-V0-011).
var routeDispatchedDirectories = map[string]bool{
	"requests": true, "system": true, "features": true, "integration": true,
	"routing": true, "controllers": true,
}

// railsAutoload is the constant-edge view of one Rails application rooted at
// the repository root (RUBY-AFFECTED-V0-009 to RUBY-AFFECTED-V0-011).
type railsAutoload struct {
	mapper     *rubyautoload.Mapper
	byConstant map[string][]string
	ids        map[string]string
	// unresolved locates every cause that keeps the mapping from standing in
	// for Rails, one ruby:rails-autoload entry each (RUBY-AFFECTED-V0-046).
	unresolved []string
	// boot and routes hold the bootEdges candidates, listed once per
	// observation rather than once per test (RUBY-AFFECTED-V0-050).
	boot, routes []supportCandidate
	indexed      bool
}

// newRailsAutoload maps every observed Ruby file through the documented
// autoload roots or the committed declaration. It reports unresolved when the
// mapping cannot stand in for Rails: a nested or absent root application, an
// invalid declaration, or autoload configuration the mapper does not model.
func newRailsAutoload(root *affected.Source, observed []observation, files []string) *railsAutoload {
	rootApplication := false
	var nested []string
	for _, relative := range files {
		if relative == "config/application.rb" {
			rootApplication = true
		} else if strings.HasSuffix(relative, "/config/application.rb") {
			nested = append(nested, located(FrontierRailsAutoload, relative, 0))
		}
	}
	if !rootApplication {
		if len(nested) == 0 {
			nested = []string{FrontierRailsAutoload}
		}
		return &railsAutoload{byConstant: map[string][]string{}, ids: map[string]string{}, unresolved: nested}
	}
	autoload := mapAutoload(root, observed, files)
	autoload.unresolved = append(autoload.unresolved, nested...)
	return autoload
}

// newConventionAutoload is the constant-edge view of a repository with no
// Rails application whose Ruby sources still follow the autoload convention
// impact maps: a committed declaration, or a default `app/<dir>` root
// (RUBY-AFFECTED-V0-045). It returns nil when neither is present, so a plain
// library keeps its require edges alone.
func newConventionAutoload(root *affected.Source, observed []observation, files []string) *railsAutoload {
	autoload := mapAutoload(root, observed, files)
	report := autoload.mapper.Report()
	if report.Declaration != "" {
		return autoload
	}
	for _, mapped := range report.Roots {
		if strings.HasPrefix(mapped, "app/") {
			return autoload
		}
	}
	return nil
}

// mapAutoload maps every observed Ruby file through the declaration or the
// default roots, the same mapper impact reports under ruby_autoload, and
// reports unresolved for an invalid declaration or autoload configuration the
// mapper does not model.
func mapAutoload(root *affected.Source, observed []observation, files []string) *railsAutoload {
	autoload := &railsAutoload{byConstant: map[string][]string{}, ids: map[string]string{}}
	sources := make(map[string]observation, len(observed))
	for _, item := range observed {
		sources[item.relative] = item
		autoload.ids[item.relative] = item.id
	}
	repository := rubyautoload.Repository{
		Paths: append([]string(nil), files...),
		IsTest: func(relative string) bool {
			item, ok := sources[relative]
			return ok && item.kind != frameworkSource || insideTestTree(relative, "spec") || insideTestTree(relative, "test")
		},
		Source: func(relative string) ([]string, []string, bool) {
			item, ok := sources[relative]
			if !ok {
				return nil, nil, false
			}
			return strings.Split(item.text, "\n"), strings.Split(maskQuotedLiteralsAndInlineComments(item.text), "\n"), true
		},
	}
	body, err := root.Read(rubyautoload.DeclarationPath)
	switch {
	case err == nil:
		repository.DeclarationPresent = true
		repository.DeclarationReadable = len(body) <= rubyautoload.MaxDeclarationBytes && strictJSON(body)
		repository.DeclarationText = string(body)
	case !errors.Is(err, fs.ErrNotExist):
		repository.DeclarationPresent = true
	}
	autoload.mapper = rubyautoload.New(repository)
	if autoload.mapper.DeclarationInvalid() {
		autoload.unresolved = append(autoload.unresolved, located(FrontierRailsAutoload, rubyautoload.DeclarationPath, 0))
	}
	for _, site := range autoload.mapper.UnresolvedConfigSites() {
		autoload.unresolved = append(autoload.unresolved, FrontierRailsAutoload+":"+site)
	}
	for _, relative := range files {
		if _, observed := autoload.ids[relative]; !observed {
			continue
		}
		if constant, ok := autoload.mapper.Constant(relative); ok {
			autoload.byConstant[constant] = append(autoload.byConstant[constant], relative)
		}
	}
	return autoload
}

// strictJSON reports one JSON document with no duplicate member names, the
// form the declaration must take to be read.
func strictJSON(body []byte) bool {
	var decoded any
	return json.Unmarshal(body, &decoded) == nil
}

// edges returns the units item's constant references resolve to, and the
// lines of every reference the mapping cannot bound: a dynamic constant lookup
// or an association whose class is not a mapped constant
// (RUBY-AFFECTED-V0-010, RUBY-AFFECTED-V0-046).
func (autoload *railsAutoload) edges(item observation) (map[string]bool, []int) {
	text := maskHeredocBodies(item.text)
	masked := maskQuotedLiteralsAndInlineComments(text)
	targets := map[string]bool{}
	var unbounded []int
	for _, at := range dynamicConstantLookup.FindAllStringIndex(masked, -1) {
		unbounded = append(unbounded, strings.Count(masked[:at[0]], "\n")+1)
	}
	scopes := scanLexicalScopes(text, masked)
	for _, at := range constantReference.FindAllStringIndex(masked, -1) {
		autoload.resolve(masked[at[0]:at[1]], scopes.at(at[0]), targets)
	}
	raw := strings.Split(item.text, "\n")
	offset := 0
	for index, line := range strings.Split(masked, "\n") {
		lineStart := offset
		offset += len(line) + 1
		match := association.FindStringSubmatch(line)
		if match == nil || index >= len(raw) {
			continue
		}
		if !autoload.association(match[1], raw[index], scopes.at(lineStart), targets) {
			unbounded = append(unbounded, index+1)
		}
	}
	edges := map[string]bool{}
	for relative := range targets {
		if relative != item.relative {
			edges[autoload.ids[relative]] = true
		}
	}
	return edges, unbounded
}

// association resolves one Active Record association to the paths defining
// its class: the literal class_name, or the camelized name and its naive
// singular. It reports false when no candidate is a mapped constant.
func (autoload *railsAutoload) association(name, raw string, namespaces []string, targets map[string]bool) bool {
	if literal := associationClassName.FindStringSubmatch(raw); literal != nil {
		return autoload.resolve(literal[1], namespaces, targets)
	}
	if associationDynamic.MatchString(raw) {
		return false
	}
	resolved := false
	for _, word := range []string{name, singular(name)} {
		if constant, ok := autoload.mapper.Camelize(word); ok && autoload.resolve(constant, namespaces, targets) {
			resolved = true
		}
	}
	return resolved
}

// resolve adds the paths defining reference under Ruby's lexical lookup: the
// name itself with each namespace prefix it loads on the way, and the name
// below every namespace lexically enclosing it. It over-approximates the
// nesting (each enclosing name also stands alone, with its prefixes), so
// it can add an edge Ruby would not take but never drops one the mapping
// knows. It reports whether any candidate is itself a mapped constant.
func (autoload *railsAutoload) resolve(reference string, namespaces []string, targets map[string]bool) bool {
	type candidate struct{ name, within string }
	name := strings.TrimPrefix(reference, "::")
	candidates := []candidate{{name: name}}
	if !strings.HasPrefix(reference, "::") {
		for _, namespace := range namespaces {
			candidates = append(candidates, candidate{namespace + "::" + name, namespace})
		}
	}
	resolved := false
	for _, current := range candidates {
		resolved = resolved || len(autoload.byConstant[current.name]) != 0
		// A prefix the file's own namespace already names is loaded, so
		// only the reference's own segments add an edge.
		for prefix := current.name; prefix != current.within; {
			for _, relative := range autoload.byConstant[prefix] {
				targets[relative] = true
			}
			cut := strings.LastIndex(prefix, "::")
			if cut < 0 {
				break
			}
			prefix = prefix[:cut]
		}
	}
	return resolved
}

// openedNamespaces names every namespace a `class` or `module` line opens,
// each alone and nested below every earlier one, with their prefixes. It is
// the whole-file fallback when block nesting cannot be scanned
// (RUBY-AFFECTED-V0-043).
func openedNamespaces(masked string) []string {
	set := map[string]bool{}
	nested := ""
	for _, match := range namespaceOpening.FindAllStringSubmatch(masked, -1) {
		if nested == "" {
			nested = match[1]
		} else {
			nested += "::" + match[1]
		}
		for _, name := range []string{match[1], nested} {
			for prefix := name; prefix != ""; {
				set[prefix] = true
				cut := strings.LastIndex(prefix, "::")
				if cut < 0 {
					break
				}
				prefix = prefix[:cut]
			}
		}
	}
	return sortedKeys(set)
}

func singular(word string) string {
	switch {
	case strings.HasSuffix(word, "ies"):
		return strings.TrimSuffix(word, "ies") + "y"
	case strings.HasSuffix(word, "sses"), strings.HasSuffix(word, "xes"), strings.HasSuffix(word, "ches"), strings.HasSuffix(word, "shes"):
		return strings.TrimSuffix(word, "es")
	case strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss"):
		return strings.TrimSuffix(word, "s")
	}
	return word
}

// bootEdges are the units every test of a Rails application loads when it
// boots the application: every source under config/ (RUBY-AFFECTED-V0-011).
// routes are the controllers and view helpers a route-dispatched test also
// reaches, which no constant names, so they are route-dispatch edges rather
// than advisory ones (RUBY-AFFECTED-V0-049).
func (autoload *railsAutoload) bootEdges(item observation) (boot, routes map[string]bool) {
	if !autoload.indexed {
		autoload.indexed = true
		for relative, id := range autoload.ids {
			if !strings.HasPrefix(id, "ruby:source:") {
				continue
			}
			switch {
			case strings.HasPrefix(relative, "config/"):
				autoload.boot = append(autoload.boot, supportCandidate{relative, id})
			case strings.HasPrefix(relative, "app/controllers/") || strings.HasPrefix(relative, "app/helpers/"):
				autoload.routes = append(autoload.routes, supportCandidate{relative, id})
			}
		}
	}
	boot, routes = candidateEdges(autoload.boot, item.relative), map[string]bool{}
	if routeDispatched(item) {
		routes = candidateEdges(autoload.routes, item.relative)
	}
	return boot, routes
}

func routeDispatched(item observation) bool {
	if item.kind == frameworkSource {
		return false
	}
	if _, relative, ok := testRelative(item.relative); ok {
		if first, _, nested := strings.Cut(relative, "/"); nested && routeDispatchedDirectories[first] {
			return true
		}
	}
	return routeDispatchedType.MatchString(item.text)
}

// supportEdges are the shared helpers every test of a suite loads: the
// suite's spec_helper, rails_helper or test_helper and every file below its
// support directory (RUBY-AFFECTED-V0-012). They are explicit import edges, so
// a change to any of them selects every test of that suite.
//
// The candidates of each suite are listed once per observation, not once per
// test, so the scan is linear in the units rather than in tests times units;
// each test still receives exactly the edges a per-test scan gives
// (RUBY-AFFECTED-V0-050).
func (index supportIndex) supportEdges(item observation, units map[string]string) map[string]bool {
	if item.kind == frameworkSource {
		return map[string]bool{}
	}
	prefix, _, ok := testRelative(item.relative)
	if !ok {
		return map[string]bool{}
	}
	suite := strings.TrimPrefix(item.relative, prefix)
	suite = prefix + suite[:strings.Index(suite, "/")+1]
	candidates, listed := index[suite]
	if !listed {
		helpers := map[string]bool{suite + "spec_helper.rb": true, suite + "rails_helper.rb": true, suite + "test_helper.rb": true}
		candidates = []supportCandidate{}
		for relative, id := range units {
			if strings.HasPrefix(id, "ruby:source:") && (helpers[relative] || strings.HasPrefix(relative, suite+"support/")) {
				candidates = append(candidates, supportCandidate{relative, id})
			}
		}
		index[suite] = candidates
	}
	return candidateEdges(candidates, item.relative)
}

// supportIndex memoizes, per suite directory, the support candidates of one
// observation's units map.
type supportIndex map[string][]supportCandidate

// supportCandidate is one source unit a test may receive as a shared edge.
type supportCandidate struct{ relative, id string }

// candidateEdges is the edge set of candidates other than the test's own file.
func candidateEdges(candidates []supportCandidate, self string) map[string]bool {
	edges := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if candidate.relative != self {
			edges[candidate.id] = true
		}
	}
	return edges
}

// supportDirectory reports a source file below a suite's support directory.
func supportDirectory(relative string) bool {
	_, rest, ok := testRelative(relative)
	return ok && strings.HasPrefix(rest, "support/")
}

func sortedEdges(edges map[string]bool, without map[string]bool) []string {
	values := make([]string, 0, len(edges))
	for id := range edges {
		if !without[id] {
			values = append(values, id)
		}
	}
	sort.Strings(values)
	return values
}

// supportGlobLoad matches the conventional helper line that requires every
// file below the suite's support directory, which supportEdges already
// expresses: `Dir[Rails.root.join("spec/support/**/*.rb")].sort.each { |f|
// require f }` and its `__dir__` and `File.dirname(__FILE__)` forms.
var supportGlobLoad = regexp.MustCompile(`^[\t ]*Dir(?:\[|\.glob\()[\t ]*` +
	`(?:Rails\.root\.join\([\t ]*(?:["'](spec|test)/support/\*\*/\*\.rb["']|["'](spec|test)["'][\t ]*,[\t ]*["']support["'][\t ]*,[\t ]*["']\*\*["'][\t ]*,[\t ]*["']\*\.rb["'])[\t ]*\)` +
	`|File\.join\([\t ]*(?:__dir__|File\.dirname\([\t ]*__FILE__[\t ]*\))[\t ]*,[\t ]*(?:["']support/\*\*/\*\.rb["']|["']support["'][\t ]*,[\t ]*["']\*\*["'][\t ]*,[\t ]*["']\*\.rb["'])[\t ]*\))` +
	`[\t ]*(?:\]|\))(?:\.sort)?\.each[\t ]*(?:\{[\t ]*\|([a-z_][a-z0-9_]*)\|[\t ]*require[\t (]+([a-z_][a-z0-9_]*)[\t )]*\}|\([\t ]*&method\([\t ]*:require[\t ]*\)[\t ]*\))[\t ]*(?:#.*)?$`)

// coveredSupportGlob reports a helper line that requires its own suite's
// support files (RUBY-AFFECTED-V0-012): the helper sits directly in a spec
// or test directory, and a Rails.root form names that directory at the root.
func coveredSupportGlob(owner, line string) bool {
	match := supportGlobLoad.FindStringSubmatch(line)
	if match == nil || match[3] != match[4] {
		return false
	}
	directory := path.Dir(owner)
	suite := path.Base(directory)
	if suite != "spec" && suite != "test" {
		return false
	}
	named := match[1] + match[2]
	return named == "" || directory == named
}
