package appmap

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Rails setup sources (SKH-V0-030, SKH-V0-032, SKH-V0-100..103): FactoryBot factories and traits,
// YAML fixture files and test-setup endpoints with their handlers' model writes come from the
// AMAP-V0-114 setup-writer index the branch query (V1-1891) builds, over the same application
// roots; seeds reuse its per-line write scan. Nothing is re-parsed by a second reader.

// Rails setup record kinds.
const (
	SetupFactory     = "factory"
	SetupTrait       = "trait"
	SetupFixtureFile = "fixture-file"
	SetupEndpoint    = "setup-endpoint"
	SetupSeed        = "seed"
	SetupProject     = "setup-project"
)

// SetupNotRead names one SKH-V0-030 source the report did not read, and why (SKH-V0-101).
type SetupNotRead struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// skRailsSources are the sources the Rails reader supplies; with no Rails router each is not read.
var skRailsSources = []string{"factorybot", "fixture-files", "seeds", "setup-endpoints"}

var skWriterKinds = map[string]string{"factory": SetupFactory, "trait": SetupTrait, "fixture": SetupFixtureFile, "endpoint": SetupEndpoint, "seed": SetupSeed}

// skSnake is the TSA-V0-102 name rule: lower snake_case, `::` as `/`.
func skSnake(name string) string {
	var b strings.Builder
	name = strings.ReplaceAll(name, "::", "/")
	for i, c := range name {
		if c >= 'A' && c <= 'Z' {
			if i > 0 && name[i-1] != '/' && name[i-1] != '_' {
				b.WriteByte('_')
			}
			c += 'a' - 'A'
		}
		b.WriteRune(c)
	}
	return b.String()
}

// readRailsSetup reads the Rails setup records of every application root of the map's Rails
// routers. A map without one reads no Rails source and names each as not read.
func readRailsSetup(r repo, m *Map, repos map[string]string) ([]SKSetup, []SetupNotRead, error) {
	man, err := readManifestAt(r, m, repos)
	if err != nil {
		return nil, nil, err
	}
	compiled, err := compileRailsRoutes(r, m.Revision, man)
	if err != nil {
		return nil, nil, err
	}
	if len(compiled.roots) == 0 {
		out := []SetupNotRead{}
		for _, s := range skRailsSources {
			out = append(out, SetupNotRead{Source: s, Reason: "no-rails-router"})
		}
		return nil, out, nil
	}
	roots := make([]string, 0, len(compiled.roots))
	for root := range compiled.roots {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	reads, records := newAppReadBudget(), []SKSetup{}
	routerFiles := map[string][]byte{}
	routerEntries := map[string]blobEntry{}
	for _, root := range roots {
		app, err := loadRailsApp(r, m.Revision, root, []string{"app", "spec", "test", "factories", "db"}, func(p string) bool {
			rel := strings.TrimPrefix(p, appJoin(root, ""))
			if root == "." {
				rel = p
			}
			return strings.HasSuffix(p, ".rb") || strings.HasSuffix(p, ".yml") && (strings.HasPrefix(rel, "spec/fixtures/") || strings.HasPrefix(rel, "test/fixtures/"))
		}, reads)
		if err != nil {
			return nil, nil, err
		}
		routes := []railsRoute{}
		for _, i := range compiled.roots[root] {
			routes = append(routes, compiled.routes[i])
		}
		x := newSetupWriterIndex(app, routes)
		writers := append(append([]setupWriter{}, x.writers...), x.seedWriters(app)...)
		for _, w := range writers {
			p, line := splitAt(w.At)
			e, data := app.entries[p], app.data[p]
			if w.Kind == "endpoint" {
				if routerFiles[p] == nil {
					re, rd, err := r.readOne(m.Revision, p, maxRouterBytes)
					if err != nil {
						return nil, nil, err
					}
					routerFiles[p], routerEntries[p] = rd, re
				}
				e, data = routerEntries[p], routerFiles[p]
			}
			if data == nil {
				continue
			}
			records = append(records, skWriterRecord(w, e, data, line))
		}
	}
	return mergeWriterRecords(records), []SetupNotRead{}, nil
}

func splitAt(at string) (string, int) {
	i := strings.LastIndex(at, ":")
	n, _ := strconv.Atoi(at[i+1:])
	return at[:i], n
}

// skWriterRecord converts one AMAP-V0-114 writer into a setup record. Produces lists the entity
// and attribute names it writes as `entity.attribute`, or `entity.*` when the request supplies them.
func skWriterRecord(w setupWriter, e blobEntry, data []byte, line int) SKSetup {
	s := SKSetup{Kind: skWriterKinds[w.Kind], Name: w.Name, Path: e.path, Status: StatusResolved, Basis: w.Basis, Authority: skAuthority}
	entity := skSnake(w.Model)
	if w.Model == "?" {
		s.Status, s.Reason, entity = StatusUnknown, "unread-factory-class", ""
	}
	switch w.Kind {
	case "factory":
		s.Target = strings.TrimPrefix(w.Name, "factory :")
		s.Params = append([]string{}, w.Attributes...)
	case "trait":
		name, fac, _ := strings.Cut(strings.TrimPrefix(w.Name, "trait :"), " (factory :")
		s.Target = strings.TrimSuffix(fac, ")") + ":" + name
	case "fixture":
		s.Target = strings.TrimPrefix(w.Name, "fixtures ")
	case "endpoint":
		s.Method, s.Target, _ = strings.Cut(w.Name, " ")
		if w.All {
			s.Params = []string{"*"}
		}
	case "seed":
		s.Target = e.path
	}
	if entity != "" {
		s.Produces = []string{entity}
		for _, a := range w.Attributes {
			s.Produces = append(s.Produces, entity+"."+a)
		}
		if w.All {
			s.Produces = append(s.Produces, entity+".*")
		}
	}
	s.Anchor = spanOf(e, data, line, line)
	s.ID = "setup:" + e.path + "#" + skDigest(s.Kind, s.Name, s.Target, strconv.Itoa(line))
	return s
}

// mergeWriterRecords joins the per-model writers of one endpoint or seed file into one record.
func mergeWriterRecords(in []SKSetup) []SKSetup {
	out, at := []SKSetup{}, map[string]int{}
	for _, s := range in {
		if i, ok := at[s.ID]; ok {
			out[i].Produces = append(out[i].Produces, s.Produces...)
			out[i].Params = append(out[i].Params, s.Params...)
			continue
		}
		at[s.ID] = len(out)
		out = append(out, s)
	}
	for i := range out {
		out[i].Produces, out[i].Params = nilIfEmpty(uniqueSorted(out[i].Produces)), nilIfEmpty(uniqueSorted(out[i].Params))
	}
	return out
}

// seedWriters reads db/seeds.rb and db/seeds/**/*.rb top to bottom with the endpoint write scan.
func (x *setupWriterIndex) seedWriters(app *railsApp) []setupWriter {
	paths := []string{}
	for p := range app.data {
		rel := strings.TrimPrefix(p, appJoin(app.root, ""))
		if app.root == "." || app.root == "" {
			rel = p
		}
		if rel == "db/seeds.rb" || strings.HasPrefix(rel, "db/seeds/") && strings.HasSuffix(rel, ".rb") {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	out := []setupWriter{}
	for _, p := range paths {
		add, flush := writeSet(setupWriter{Kind: "seed", Name: "seeds " + path.Base(p), At: p + ":1"})
		for _, code := range parseRubyStructure(p, app.data[p]).code {
			x.lineWrites(app, code, add)
		}
		out = append(out, flush()...)
	}
	return out
}

var (
	skFactoryUse = regexp.MustCompile(`(?:\bFactoryBot\.|(?:^|[^.\w]))(?:create|build)(?:_list)?\s*\(?\s*:([A-Za-z_]\w*)((?:\s*,\s*(?::[A-Za-z_]\w*|\d+))*)`)
	skFixtureUse = regexp.MustCompile(`^\s*fixtures\s*\(?\s*(:[A-Za-z_]\w*(?:\s*,\s*:[A-Za-z_]\w*)*)`)
)

// linkRuby links each Ruby test to the factory, trait and fixture-file records its body and its
// file's `fixtures` declarations name (SKH-V0-031, SKH-V0-102). A factory with no record is named
// unresolved, never guessed.
func (m *SKModel) linkRuby(records []SKSetup) []SKSetupUse {
	byTarget := map[string]string{}
	for _, s := range records {
		switch s.Kind {
		case SetupFactory, SetupTrait, SetupFixtureFile:
			byTarget[s.Kind+":"+s.Target] = s.ID
		}
	}
	paths := []string{}
	for p, f := range m.files {
		if f.ruby {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	uses := []SKSetupUse{}
	for _, p := range paths {
		f := m.files[p]
		fileUses, fileUnresolved := []string{}, []string{}
		for _, code := range f.rb.code {
			if mm := skFixtureUse.FindStringSubmatch(code); mm != nil {
				for _, t := range symbolWord.FindAllStringSubmatch(mm[1], -1) {
					if id := byTarget[SetupFixtureFile+":"+t[1]]; id != "" {
						fileUses = append(fileUses, id)
					} else {
						fileUnresolved = append(fileUnresolved, "fixtures:"+t[1])
					}
				}
			}
		}
		for _, u := range f.tests {
			ids, unresolved := append([]string{}, fileUses...), append([]string{}, fileUnresolved...)
			for l := u.from; l <= u.to && l <= len(f.rb.code); l++ {
				for _, mm := range skFactoryUse.FindAllStringSubmatch(f.rb.code[l-1], -1) {
					id := byTarget[SetupFactory+":"+mm[1]]
					if id == "" {
						unresolved = append(unresolved, "factory:"+mm[1])
						continue
					}
					ids = append(ids, id)
					for _, t := range symbolWord.FindAllStringSubmatch(mm[2], -1) {
						if tid := byTarget[SetupTrait+":"+mm[1]+":"+t[1]]; tid != "" {
							ids = append(ids, tid)
						} else {
							unresolved = append(unresolved, "trait:"+mm[1]+":"+t[1])
						}
					}
				}
			}
			uses = append(uses, SKSetupUse{Test: skTestID(p, u), Title: u.title, Uses: uniqueSorted(ids), Unresolved: nilIfEmpty(uniqueSorted(unresolved))})
		}
	}
	return uses
}

// SetupCandidate is one record that can produce a condition, with the lexical basis of the match.
type SetupCandidate struct {
	Setup  string `json:"setup"`
	Kind   string `json:"kind"`
	Basis  string `json:"basis"`
	Mocked bool   `json:"mocked,omitempty"`
}

// SetupCondition answers "what can produce condition C" (SKH-V0-031, SKH-V0-103): candidates with
// evidence, or unresolved.
type SetupCondition struct {
	Condition  string           `json:"condition"`
	Flow       string           `json:"flow,omitempty"`
	Status     string           `json:"status"`
	Reason     string           `json:"reason,omitempty"`
	Candidates []SetupCandidate `json:"candidates"`
}

const (
	ConditionCandidates = "candidates"
	ConditionUnresolved = "unresolved"
)

// skConditionWords are the lower-case words of a condition: identifiers, with `Model.attr` kept.
var skConditionWords = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:(?:::|\.)[A-Za-z_][A-Za-z0-9_]*)*`)

// skSingular is a lexical singular for matching only (orders -> order, companies -> company).
func skSingular(w string) string {
	switch {
	case strings.HasSuffix(w, "ies") && len(w) > 3:
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "ses") || strings.HasSuffix(w, "xes"):
		return w[:len(w)-2]
	case strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") && len(w) > 1:
		return w[:len(w)-1]
	}
	return w
}

// conditionQuery matches condition text against what each record produces or targets. It is
// lexical and deterministic (SKH-V0-004): a literal target (a storageState path, an endpoint, a
// task or route) matches exactly; otherwise each word matches an entity, `entity.attribute`, or a
// fixture name that a record produces. A route mock stays labelled mocked (SKH-V0-034).
func conditionQuery(c string, records []SKSetup) SetupCondition {
	out := SetupCondition{Condition: c, Status: ConditionUnresolved, Reason: "no-producing-record", Candidates: []SetupCandidate{}}
	seen := map[string]bool{}
	add := func(s SKSetup, basis string) {
		if !seen[s.ID] {
			seen[s.ID] = true
			out.Candidates = append(out.Candidates, SetupCandidate{Setup: s.ID, Kind: s.Kind, Basis: basis, Mocked: s.Mocked})
		}
	}
	text := strings.TrimSpace(c)
	for _, s := range records {
		if s.Target != "" && s.Target == text && s.Mode != "reads" {
			add(s, "target")
		}
	}
	for _, raw := range skConditionWords.FindAllString(text, -1) {
		entity, attr, dotted := strings.Cut(raw, ".")
		word := skSingular(skSnake(entity))
		for _, s := range records {
			for _, p := range s.Produces {
				pe, pa, _ := strings.Cut(p, ".")
				switch {
				case dotted && pe == word && (pa == skSnake(attr) || pa == "*"):
					add(s, "produces-attribute")
				case !dotted && pa == "" && (pe == word || skSingular(pe) == word):
					add(s, "produces-entity")
				}
			}
		}
	}
	sort.SliceStable(out.Candidates, func(i, j int) bool { return out.Candidates[i].Setup < out.Candidates[j].Setup })
	if len(out.Candidates) > 0 {
		out.Status, out.Reason = ConditionCandidates, ""
	}
	return out
}
