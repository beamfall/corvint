package contextindex

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"sort"
	"strings"
)

// SetupSelectSchema names the `corvint tests setup-select` result (TSA-V0-001).
const SetupSelectSchema = "corvint-tests-setup-select/0"

const (
	maxSetupClosure = 10000
	maxSetupReasons = 100
)

// SetupSelectReason is one typed cause of an ALL result (TSA-V0-003).
type SetupSelectReason struct {
	Code   string `json:"code"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// SetupNotFollowed is a reached file whose imports were deliberately not followed (TSA-V0-004).
type SetupNotFollowed struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// SetupSelection is the setup files a spec list depends on at one revision (TSA-V0-001..006).
// Mode is "selected" when every dependency closed, or "all" with at least one reason, in which
// case SetupFiles is the whole setup universe.
type SetupSelection struct {
	Schema      string              `json:"schema"`
	Revision    string              `json:"revision"`
	Mode        string              `json:"mode"`
	Specs       []string            `json:"specs"`
	SetupFiles  []string            `json:"setupFiles"`
	Universe    int                 `json:"universe"`
	Reasons     []SetupSelectReason `json:"reasons"`
	NotFollowed []SetupNotFollowed  `json:"notFollowed"`
	InputDigest string              `json:"inputDigest"`
}

// IsSetupFile reports a JavaScript or TypeScript setup or seed file: a base name whose stem ends
// in `.setup` or `.seed` (TSA-V0-002).
func IsSetupFile(name string) bool {
	extension := path.Ext(name)
	if language := NonGoImpactLanguage(name); language != "javascript" && language != "typescript" {
		return false
	}
	stem := strings.TrimSuffix(path.Base(name), extension)
	return strings.HasSuffix(stem, ".setup") || strings.HasSuffix(stem, ".seed")
}

type setupSelector struct {
	index       *Index
	resolver    *WebImportResolver
	dialect     *registryDialect
	universe    map[string]bool
	registries  map[string]bool
	read        map[string]string
	reasons     []SetupSelectReason
	notFollowed map[string]string
	visited     map[string]bool
	queue       []string
	// keyTargets are the modules registry keys resolved to: the data contexts the specs read.
	keyTargets map[string]bool
	seeds      map[string]bool
	// shared is the shared-constant key bookkeeping (TSA-V0-006, TSA-V0-007).
	shared *sharedState
}

// SelectSetup returns the setup files specs transitively depend on in index: through resolved
// value imports, literal require() and import() calls, and literal keys of declared
// string-key-registry/0 accessors (TSA-V0-001..006). It never invents a narrower answer: any
// dependency it cannot close returns the whole setup universe with typed reasons.
func SelectSetup(index *Index, specs []string) SetupSelection {
	s := &setupSelector{index: index, resolver: NewWebImportResolver(index), dialect: loadRegistryDialect(index),
		universe: map[string]bool{}, registries: map[string]bool{}, read: map[string]string{},
		notFollowed: map[string]string{}, visited: map[string]bool{}, keyTargets: map[string]bool{}, seeds: map[string]bool{},
		shared: newSharedState()}
	// The universe is every tracked setup path, including generated or oversized files the index
	// does not read, so ALL is always the whole safe superset (TSA-V0-003).
	for name := range index.Tracked {
		if _, skipped := index.Skipped[name]; !skipped && IsSetupFile(name) {
			s.universe[name] = true
		}
	}
	for _, registry := range s.dialect.registries {
		s.registries[registry.declaration.Path] = true
		if registry.blob != "" {
			s.read[registry.declaration.Path] = registry.blob
		}
	}
	for name := range index.Sources {
		if path.Ext(name) == ".json" {
			if text, valid, loaded := index.Sources[name].Text(); valid && loaded && strings.Contains(text, "application-map-manifest") {
				s.read[name] = index.Sources[name].BlobHash
			}
		}
	}
	s.readSetupShared()
	if s.dialect.invalid != 0 {
		s.reason("registry-unresolved", "", 0, "an application-map manifest declares invalid registries")
	}
	cleaned := []string{}
	for _, spec := range specs {
		if spec = strings.TrimSpace(spec); spec != "" {
			cleaned = append(cleaned, spec)
		}
	}
	sort.Strings(cleaned)
	if len(cleaned) == 0 {
		s.reason("empty-spec-list", "", 0, "no spec paths were given")
	}
	for _, spec := range cleaned {
		name := path.Clean(spec)
		_, indexed := index.Sources[name]
		if language := NonGoImpactLanguage(name); !indexed || name != spec || (language != "javascript" && language != "typescript") {
			s.reason("missing-path", spec, 0, "not a tracked JavaScript or TypeScript source at the revision")
			continue
		}
		s.enqueue(name, true)
	}
	// A seeder a registry key or a shared constant key selects is itself traversed, and the seeders
	// of any key it reaches are selected in turn, until the closure stops growing (TSA-V0-001,
	// TSA-V0-006). Non-literal shared reads are judged only against the final closure (TSA-V0-007).
	for s.drain() {
		registry := s.seedersOfKeyTargets()
		if shared := s.seedersOfSharedKeys(); !registry && !shared {
			s.sharedReasons()
			break
		}
	}
	return s.result(index, cleaned)
}

// drain visits the queued closure, reporting false when the bound stopped it.
func (s *setupSelector) drain() bool {
	for len(s.queue) > 0 {
		name := s.queue[0]
		s.queue = s.queue[1:]
		if len(s.visited) > maxSetupClosure {
			s.reason("bound-exceeded", "", 0, "the dependency closure exceeds the bound")
			return false
		}
		s.visit(name)
	}
	return true
}

func (s *setupSelector) reason(code, name string, line int, detail string) {
	s.reasons = append(s.reasons, SetupSelectReason{Code: code, Path: name, Line: line, Detail: detail})
}

// enqueue follows a reached file unless it is a declared registry or a hub (TSA-V0-004).
func (s *setupSelector) enqueue(name string, spec bool) {
	if s.visited[name] || s.notFollowed[name] != "" {
		return
	}
	if !spec && s.registries[name] {
		s.notFollowed[name] = "registry"
		return
	}
	if !spec && s.hub(name) {
		s.notFollowed[name] = "hub"
		return
	}
	s.visited[name] = true
	s.queue = append(s.queue, name)
}

// hub reports a file whose resolved imports name every setup file of a universe of two or more:
// following it would select everything, which is the over-selection the command exists to avoid.
func (s *setupSelector) hub(name string) bool {
	if len(s.universe) < 2 || s.universe[name] {
		return false
	}
	text, ok := s.text(name)
	if !ok {
		return false
	}
	kinds, _ := WebImportKinds(text)
	named := map[string]bool{}
	for specifier := range kinds {
		if resolution := s.resolver.Resolve(name, specifier); resolution.State == WebImportRepository && s.universe[resolution.Target] {
			named[resolution.Target] = true
		}
	}
	return len(named) == len(s.universe)
}

func (s *setupSelector) text(name string) (string, bool) {
	source, indexed := s.index.Sources[name]
	if !indexed {
		return "", false
	}
	text, valid, loaded := source.Text()
	if !valid || !loaded {
		return "", false
	}
	s.read[name] = source.BlobHash
	return text, true
}

func (s *setupSelector) follow(from, specifier string, line int, via string) {
	resolution := s.resolver.Resolve(from, specifier)
	switch resolution.State {
	case WebImportRepository:
		if via != "" {
			s.keyTargets[resolution.Target] = true
		}
		s.enqueue(resolution.Target, false)
	case WebImportUnresolved:
		code := "import-unresolved"
		if via != "" {
			code = "registry-unresolved"
		}
		s.reason(code, from, line, strings.TrimSpace(via+" "+specifier))
	}
}

// visit follows one closure file's imports, require() and import() calls and registry keys.
func (s *setupSelector) visit(name string) {
	text, ok := s.text(name)
	if !ok {
		s.reason("source-unreadable", name, 0, "the source cannot be read")
		return
	}
	kinds, unterminated := WebImportKinds(text)
	tokens, complete := registryTokens(text)
	if unterminated != "" || !complete {
		s.reason("source-unreadable", name, 0, "the source ends inside an unterminated construct")
	}
	specifiers := make([]string, 0, len(kinds))
	for specifier, typeOnly := range kinds {
		if !typeOnly {
			specifiers = append(specifiers, specifier)
		}
	}
	sort.Strings(specifiers)
	for _, specifier := range specifiers {
		s.follow(name, specifier, 0, "")
	}
	for at, token := range tokens {
		if token.kind != webWord || (token.text != "require" && token.text != "import") || registryMember(tokens, at) ||
			at+1 >= len(tokens) || !isRegistryPunct(tokens[at+1], "(") {
			continue
		}
		close := registryClose(tokens, at+1)
		literal := at+3 == close && tokens[at+2].kind == webString && tokens[at+2].quoted
		switch {
		case !literal:
			s.reason("computed-lookup", name, token.line, "a "+token.text+"() whose module is not a string literal")
		case token.text == "require":
			s.follow(name, tokens[at+2].text, token.line, "")
		}
	}
	for _, site := range s.dialect.sites {
		if site.path == name {
			s.lookup(site)
		}
	}
	s.collectShared(name, text)
}

// lookup follows a registry accessor call by its literal key (TSA-V0-003).
func (s *setupSelector) lookup(site registrySite) {
	if !site.literal {
		s.reason("computed-lookup", site.path, site.line, "a "+site.accessor+" call whose key is not a literal")
		return
	}
	found, unclosed := false, false
	for _, registry := range s.dialect.registries {
		declares := false
		for _, accessor := range registry.declaration.Accessors {
			declares = declares || accessor == site.accessor
		}
		if !declares {
			continue
		}
		if registry.missing || registry.unresolved != 0 {
			unclosed = true
		}
		for _, entry := range registry.entries {
			if entry.key == site.key {
				found = true
				s.follow(registry.declaration.Path, entry.binding.specifier, entry.line, "key "+site.key+":")
			}
		}
	}
	switch {
	case unclosed:
		s.reason("registry-unresolved", site.path, site.line, "a registry "+site.accessor+" reads has entries that do not close")
	case !found:
		s.reason("unknown-key", site.path, site.line, site.accessor+"("+site.key+") names no registry key")
	}
}

// seedersOfKeyTargets selects every setup file that directly imports a module a registry key
// resolved to: the setup that seeds a data context the specs read (TSA-V0-001). Each new seeder
// is enqueued so its own dependencies are traversed; it reports whether any was.
func (s *setupSelector) seedersOfKeyTargets() bool {
	if len(s.keyTargets) == 0 {
		return false
	}
	enqueued := false
	names := make([]string, 0, len(s.universe))
	for name := range s.universe {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if s.visited[name] || s.seeds[name] {
			continue
		}
		text, ok := s.text(name)
		if !ok {
			s.reason("source-unreadable", name, 0, "the source cannot be read")
			continue
		}
		kinds, _ := WebImportKinds(text)
		for specifier, typeOnly := range kinds {
			if resolution := s.resolver.Resolve(name, specifier); !typeOnly && resolution.State == WebImportRepository && s.keyTargets[resolution.Target] {
				s.seeds[name] = true
			}
		}
		if s.seeds[name] {
			s.enqueue(name, false)
			enqueued = true
		}
	}
	return enqueued
}

func (s *setupSelector) result(index *Index, specs []string) SetupSelection {
	out := SetupSelection{Schema: SetupSelectSchema, Revision: index.CommitRevision, Mode: "selected", Specs: specs,
		SetupFiles: []string{}, Universe: len(s.universe), Reasons: []SetupSelectReason{}, NotFollowed: []SetupNotFollowed{}}
	if out.Specs == nil {
		out.Specs = []string{}
	}
	for name := range s.universe {
		if s.visited[name] || s.seeds[name] || len(s.reasons) != 0 {
			out.SetupFiles = append(out.SetupFiles, name)
		}
	}
	sort.Strings(out.SetupFiles)
	if len(s.reasons) != 0 {
		out.Mode = "all"
		sort.Slice(s.reasons, func(a, b int) bool {
			x, y := s.reasons[a], s.reasons[b]
			if x.Code != y.Code {
				return x.Code < y.Code
			}
			if x.Path != y.Path {
				return x.Path < y.Path
			}
			if x.Line != y.Line {
				return x.Line < y.Line
			}
			return x.Detail < y.Detail
		})
		for i, reason := range s.reasons {
			if i > 0 && reason == s.reasons[i-1] {
				continue
			}
			if len(out.Reasons) == maxSetupReasons {
				break
			}
			out.Reasons = append(out.Reasons, reason)
		}
	}
	for name, why := range s.notFollowed {
		out.NotFollowed = append(out.NotFollowed, SetupNotFollowed{Path: name, Reason: why})
	}
	sort.Slice(out.NotFollowed, func(a, b int) bool { return out.NotFollowed[a].Path < out.NotFollowed[b].Path })
	names := make([]string, 0, len(s.read))
	for name := range s.read {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		digest.Write([]byte(name + "\t" + s.read[name] + "\n"))
	}
	out.InputDigest = "sha256:" + hex.EncodeToString(digest.Sum(nil))
	return out
}
