package appmap

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Setup writer index (AMAP-V0-114): which setup endpoints, FactoryBot factories and traits, and
// YAML fixtures write which model attributes. It is a write-name join over committed source,
// built once per application root and queried by (model, attribute); the branch query
// (AMAP-V0-115) is its first consumer and V1-1900's data-setup model is the intended second.

// setupWriter is one source that can write model attributes.
type setupWriter struct {
	Kind       string   `json:"kind"` // endpoint, factory, trait, fixture
	Name       string   `json:"name"`
	At         string   `json:"at"`
	Model      string   `json:"model"`
	Attributes []string `json:"attributes,omitempty"`
	All        bool     `json:"all_attributes,omitempty"`
	Basis      string   `json:"basis"`
}

type setupWriterIndex struct {
	writers   []setupWriter
	factories map[string]*setupWriter // factory name -> its writer
	traits    map[string]*setupWriter // factory:trait -> its writer
	endpoints int
}

// setupEndpointSegments are the first path segments that mark a test-setup route.
var setupEndpointSegments = map[string]bool{"test": true, "__test__": true, "e2e": true, "cypress": true, "__cypress__": true,
	"testing": true, "test_support": true, "test_setup": true}

var setupWriterKinds = map[string]int{"endpoint": 0, "factory": 1, "trait": 2, "fixture": 3}

// writersOf lists the writers of model.attribute in AMAP-V0-115 order: endpoints, factories and
// traits, then fixtures.
func (x *setupWriterIndex) writersOf(model, attribute string) []setupWriter {
	out := []setupWriter{}
	for _, w := range x.writers {
		if w.Model != model {
			continue
		}
		hit := w.All
		for _, a := range w.Attributes {
			hit = hit || a == attribute
		}
		if hit {
			out = append(out, w)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return setupWriterKinds[out[i].Kind] < setupWriterKinds[out[j].Kind] })
	return out
}

// newSetupWriterIndex derives the index from an application's factories, fixtures and the setup
// endpoints among its compiled routes.
func newSetupWriterIndex(app *railsApp, routes []railsRoute) *setupWriterIndex {
	x := &setupWriterIndex{factories: map[string]*setupWriter{}, traits: map[string]*setupWriter{}}
	paths := make([]string, 0, len(app.data))
	for p := range app.data {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		rel := strings.TrimPrefix(p, appJoin(app.root, ""))
		if app.root == "." || app.root == "" {
			rel = p
		}
		switch {
		case strings.HasSuffix(p, ".rb") && (strings.HasPrefix(rel, "spec/factories/") || strings.HasPrefix(rel, "test/factories/") || strings.HasPrefix(rel, "factories/")):
			x.readFactories(p, app.data[p])
		case strings.HasSuffix(p, ".yml") && (strings.HasPrefix(rel, "spec/fixtures/") || strings.HasPrefix(rel, "test/fixtures/")):
			x.readFixture(p, rel, app.data[p])
		}
	}
	// Factories are indexed before endpoints so FactoryBot calls in an endpoint can expand them.
	for _, rt := range routes {
		first, _, _ := strings.Cut(strings.TrimPrefix(rt.path, "/"), "/")
		if !setupEndpointSegments[first] {
			continue
		}
		x.endpoints++
		x.readEndpoint(app, rt)
	}
	return x
}

var (
	factoryOpen  = regexp.MustCompile(`^factory\s*\(?\s*:([A-Za-z_]\w*)(.*)$`)
	factoryClass = regexp.MustCompile(`class:\s*(?:["']?(?:::)?([A-Z][\w:]*)["']?(?:[\s,)]|$)|(?::|["'])([a-z_]\w*)["']?(?:[\s,)]|$))`)
	traitOpen    = regexp.MustCompile(`^trait\s*\(?\s*:([A-Za-z_]\w*)`)
	sequenceAttr = regexp.MustCompile(`^sequence\s*\(?\s*:([A-Za-z_]\w*)`)
	factoryWords = map[string]bool{"factory": true, "trait": true, "transient": true, "after": true, "before": true, "callback": true,
		"to_create": true, "initialize_with": true, "skip_create": true, "association": true, "end": true, "do": true,
		"traits_for_enum": true, "sequence": true, "if": true, "unless": true, "add_attribute": true}
)

// readFactories reads `factory :x [, class: X] do`, nested factories (which inherit their
// parent's model and attributes), `trait :t do` and attribute lines of FactoryBot definition
// files. The class option is read before masking, which blanks a string class name.
func (x *setupWriterIndex) readFactories(p string, data []byte) {
	raw := strings.Split(string(data), "\n")
	code := parseRubyStructure(p, data).code
	type frame struct {
		owner   int // index of the factory or trait writer whose attributes this block lists; -1 none
		factory string
		model   string
	}
	stack := []frame{}
	parents := map[int]int{}
	for n, line := range code {
		stmt := strings.TrimSpace(line)
		if stmt == "" {
			continue
		}
		at := p + ":" + strconv.Itoa(n+1)
		opens := strings.HasSuffix(stmt, " do") || strings.Contains(stmt, " do |") || stmt == "do"
		top := frame{owner: -1}
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		if stmt == "end" || strings.HasPrefix(stmt, "end ") || strings.HasPrefix(stmt, "end;") || strings.HasPrefix(stmt, "end.") {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if m := factoryOpen.FindStringSubmatch(stmt); m != nil {
			model := top.model
			if c := factoryClass.FindStringSubmatch(raw[n]); c != nil {
				model = c[1]
				if model == "" {
					model = rubyCamelize(c[2])
				}
			} else if strings.Contains(raw[n], "class:") {
				// An unread class option never binds to a guessed model.
				model = "?"
			}
			if model == "" {
				model = rubyCamelize(m[1])
			}
			x.writers = append(x.writers, setupWriter{Kind: "factory", Name: "factory :" + m[1], At: at, Model: model, Attributes: []string{}, Basis: "factory-attributes"})
			if top.owner >= 0 && top.factory != "" && !strings.Contains(top.factory, ":") {
				parents[len(x.writers)-1] = top.owner
			}
			if opens {
				stack = append(stack, frame{owner: len(x.writers) - 1, factory: m[1], model: model})
			}
			continue
		}
		if m := traitOpen.FindStringSubmatch(stmt); m != nil && top.factory != "" {
			x.writers = append(x.writers, setupWriter{Kind: "trait", Name: "trait :" + m[1] + " (factory :" + top.factory + ")", At: at, Model: top.model, Attributes: []string{}, Basis: "factory-attributes"})
			if opens {
				stack = append(stack, frame{owner: len(x.writers) - 1, factory: top.factory + ":" + m[1], model: top.model})
			}
			continue
		}
		if opens {
			stack = append(stack, frame{owner: -1, factory: top.factory, model: top.model})
			continue
		}
		if top.owner < 0 {
			continue
		}
		attr := ""
		if m := sequenceAttr.FindStringSubmatch(stmt); m != nil {
			attr = m[1]
		} else if words := rubyWordsOf(stmt); len(words) > 0 && words[0].at == 0 && words[0].keyword && !factoryWords[words[0].text] {
			attr = strings.TrimRight(words[0].text, "?!")
		}
		if attr != "" {
			x.writers[top.owner].Attributes = appendUnique(x.writers[top.owner].Attributes, attr)
		}
	}
	// A parent always precedes its children, so ascending order completes each parent's inherited
	// attributes before a descendant copies them.
	children := make([]int, 0, len(parents))
	for child := range parents {
		children = append(children, child)
	}
	sort.Ints(children)
	for _, child := range children {
		parent := parents[child]
		for _, a := range x.writers[parent].Attributes {
			x.writers[child].Attributes = appendUnique(x.writers[child].Attributes, a)
		}
	}
	for i := range x.writers {
		w := &x.writers[i]
		switch w.Kind {
		case "factory":
			x.factories[strings.TrimPrefix(w.Name, "factory :")] = w
		case "trait":
			name, fac, _ := strings.Cut(strings.TrimPrefix(w.Name, "trait :"), " (factory :")
			x.traits[strings.TrimSuffix(fac, ")")+":"+name] = w
		}
	}
}

func appendUnique(list []string, v string) []string {
	for _, s := range list {
		if s == v {
			return list
		}
	}
	return append(list, v)
}

var fixtureKey = regexp.MustCompile(`^(\s+)([A-Za-z_]\w*)\s*:`)

// readFixture reads one YAML fixture: its path under the fixtures directory names the table and
// the keys one level under each record are the attributes written.
func (x *setupWriterIndex) readFixture(p, rel string, data []byte) {
	table := strings.TrimSuffix(rel[strings.Index(rel, "fixtures/")+len("fixtures/"):], ".yml")
	dir, base := "", table
	if i := strings.LastIndex(table, "/"); i >= 0 {
		dir, base = table[:i+1], table[i+1:]
	}
	singular := railsSingular(base)
	if singular == "" {
		singular = base
	}
	w := setupWriter{Kind: "fixture", Name: "fixtures " + table, At: p + ":1", Model: rubyCamelize(dir + singular), Attributes: []string{}, Basis: "fixture-keys"}
	indent := ""
	for _, line := range strings.Split(string(data), "\n") {
		m := fixtureKey.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if indent == "" {
			indent = m[1]
		}
		if m[1] == indent {
			w.Attributes = appendUnique(w.Attributes, m[2])
		}
	}
	if len(w.Attributes) > 0 {
		x.writers = append(x.writers, w)
	}
}

var (
	modelWrite    = regexp.MustCompile(`\b([A-Z]\w*(?:::[A-Z]\w*)*)\.(create!|create|new|update!|update|find_or_create_by!|find_or_create_by|insert|upsert)`)
	factoryCall   = regexp.MustCompile(`(?:\bFactoryBot\.|(?:^|[^.\w]))create(?:_list)?\s*\(?\s*:([A-Za-z_]\w*)((?:\s*,\s*:[A-Za-z_]\w*)*)`)
	recordUpdate  = regexp.MustCompile(`(?:^|[^.\w])@?([a-z_]\w*)\.(update!|update_columns|update_attribute|update_column|update)`)
	recordAssign  = regexp.MustCompile(`(?:^|[^.\w])@?([a-z_]\w*)\.([a-z_]\w*)\s*=[^=~>]`)
	hashKey       = regexp.MustCompile(`(?:^|[\s(,{])([a-z_]\w*):\s`)
	symbolWord    = regexp.MustCompile(`:([A-Za-z_]\w*)`)
	symbolKey     = regexp.MustCompile(`^\s*\(?\s*:([a-z_]\w*)`)
	maxSetupCalls = 32
)

// readEndpoint reads the writes of one setup endpoint's action and the same-class methods it
// calls (AMAP-V0-114).
func (x *setupWriterIndex) readEndpoint(app *railsApp, rt railsRoute) {
	f, why := app.ruby("app/controllers/" + rt.ctrl + "_controller.rb")
	if why != "" || f.broken != 0 {
		return
	}
	scope := f.scope(controllerConstant(rt.ctrl))
	if scope == nil {
		return
	}
	defs := map[string]*rubyDef{}
	for _, d := range scope.defs {
		if d.instance {
			defs[d.name] = d
		}
	}
	add, flush := writeSet(setupWriter{Kind: "endpoint", Name: rt.verb + " " + rt.path, At: rt.declaration()})
	seen := map[string]bool{}
	queue := []string{rt.action}
	for len(queue) > 0 && len(seen) < maxSetupCalls {
		name := queue[0]
		queue = queue[1:]
		d := defs[name]
		if seen[name] || d == nil {
			continue
		}
		seen[name] = true
		for ln := d.line; ln <= d.end && ln <= len(f.code); ln++ {
			code := f.code[ln-1]
			if ln == d.line {
				code = strings.Repeat(" ", len(code)) // the def line's parameters are not calls
			}
			for _, word := range rubyWordsOf(code) {
				if word.keyword && defs[word.text] != nil && !seen[word.text] {
					queue = append(queue, word.text)
				}
			}
			x.lineWrites(app, code, add)
		}
	}
	x.writers = append(x.writers, flush()...)
}

// writeSet accumulates one writer per model written by a source w names, and flush returns those
// that write any attribute, ordered by model.
func writeSet(w setupWriter) (add func(model, attr string, all bool, basis string), flush func() []setupWriter) {
	writes := map[string]*setupWriter{}
	add = func(model, attr string, all bool, basis string) {
		k := model
		if writes[k] == nil {
			c := w
			c.Model, c.Attributes, c.Basis = model, []string{}, basis
			writes[k] = &c
		}
		if all {
			writes[k].All, writes[k].Basis = true, "attributes-from-request"
		}
		if attr != "" {
			writes[k].Attributes = appendUnique(writes[k].Attributes, attr)
		}
	}
	flush = func() []setupWriter {
		models := make([]string, 0, len(writes))
		for k := range writes {
			models = append(models, k)
		}
		sort.Strings(models)
		out := []setupWriter{}
		for _, k := range models {
			if wr := writes[k]; wr.All || len(wr.Attributes) > 0 {
				sort.Strings(wr.Attributes)
				out = append(out, *wr)
			}
		}
		return out
	}
	return add, flush
}

// lineWrites reads the model writes of one masked Ruby line: model class writes, FactoryBot calls
// expanded through the indexed factories and traits, record updates and attribute assignments
// (AMAP-V0-114). Setup endpoints and the site-knowledge seed reader share it (SKH-V0-032).
func (x *setupWriterIndex) lineWrites(app *railsApp, code string, add func(model, attr string, all bool, basis string)) {
	for _, m := range modelWrite.FindAllStringSubmatchIndex(code, -1) {
		if m[1] < len(code) && rubyIdentByte(code[m[1]]) {
			continue
		}
		model := code[m[2]:m[3]]
		if model == "FactoryBot" {
			continue // a FactoryBot call, read below through its factory
		}
		keys, all := writeArgs(code[m[1]:])
		for _, k := range keys {
			add(model, k, false, "literal-hash")
		}
		if all {
			add(model, "", true, "attributes-from-request")
		}
	}
	for _, m := range factoryCall.FindAllStringSubmatchIndex(code, -1) {
		name := code[m[2]:m[3]]
		model := rubyCamelize(name)
		attrs := []string{}
		if fw := x.factories[name]; fw != nil {
			model, attrs = fw.Model, append(attrs, fw.Attributes...)
		}
		for _, t := range symbolWord.FindAllStringSubmatch(code[m[4]:m[5]], -1) {
			if tw := x.traits[name+":"+t[1]]; tw != nil {
				attrs = append(attrs, tw.Attributes...)
			}
		}
		keys, all := writeArgs("(" + strings.TrimLeft(code[m[1]:], " ,"))
		attrs = append(attrs, keys...)
		for _, a := range attrs {
			add(model, a, false, "factory-call")
		}
		if all {
			add(model, "", true, "attributes-from-request")
		}
	}
	for _, m := range recordUpdate.FindAllStringSubmatchIndex(code, -1) {
		model, ok := app.modelOf(code[m[2]:m[3]])
		if !ok || m[1] < len(code) && rubyIdentByte(code[m[1]]) {
			continue
		}
		rest := code[m[1]:]
		if method := code[m[4]:m[5]]; method == "update_attribute" || method == "update_column" {
			if k := symbolKey.FindStringSubmatch(rest); k != nil {
				add(model, k[1], false, "literal-hash")
			}
			continue
		}
		keys, all := writeArgs(rest)
		for _, k := range keys {
			add(model, k, false, "literal-hash")
		}
		if all {
			add(model, "", true, "attributes-from-request")
		}
	}
	for _, m := range recordAssign.FindAllStringSubmatch(code, -1) {
		if model, ok := app.modelOf(m[1]); ok {
			add(model, m[2], false, "assignment")
		}
	}
}

// writeArgs reads the argument list after a write call: the literal hash keys it names, and
// whether any argument is not a literal key (so every attribute may be written).
func writeArgs(rest string) ([]string, bool) {
	rest = strings.TrimLeft(rest, " ")
	args := rest
	if strings.HasPrefix(rest, "(") {
		depth, end := 0, len(rest)
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				end = i
				break
			}
		}
		args = rest[1:end]
	} else if strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "do") {
		return nil, false
	}
	if strings.TrimSpace(args) == "" {
		return nil, false
	}
	keys := []string{}
	for _, m := range hashKey.FindAllStringSubmatch(" "+args, -1) {
		keys = appendUnique(keys, m[1])
	}
	all := len(keys) == 0 || strings.Contains(args, "**")
	if len(keys) > 0 {
		// A positional argument before the first key is a whole attribute hash.
		first := strings.TrimSpace(args)
		if loc := hashKey.FindStringIndex(" " + args); loc != nil && strings.TrimSpace((" " + args)[:loc[0]+1]) != "" && !strings.HasPrefix(first, "{") {
			all = true
		}
	}
	return keys, all
}

// modelOf binds a receiver (`x`, `@x` or `current_x`) to the model X when app/models/x.rb
// exists (AMAP-V0-113).
func (a *railsApp) modelOf(recv string) (string, bool) {
	name := strings.TrimPrefix(strings.TrimPrefix(recv, "@"), "current_")
	if name == "" || name == "params" || name == "self" {
		return "", false
	}
	if _, ok := a.entries[appJoin(a.root, "app/models/"+name+".rb")]; !ok {
		return "", false
	}
	return rubyCamelize(name), true
}
