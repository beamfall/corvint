package breakagemap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/extevidence"
)

type Anchor struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Tree       string `json:"tree"`
	Path       string `json:"path"`
	Blob       string `json:"blob"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	SpanSHA256 string `json:"span_sha256"`
}
type Unknown struct {
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
}
type Edge struct {
	Kind           string                 `json:"kind"`
	Confidence     string                 `json:"confidence"`
	Reason         string                 `json:"reason"`
	From           *Anchor                `json:"from,omitempty"`
	To             *Anchor                `json:"to,omitempty"`
	ProviderSHA256 string                 `json:"provider_sha256,omitempty"`
	Relation       *extevidence.Relation1 `json:"relation,omitempty"`
	ByteState      string                 `json:"byte_state"`
}
type RepoState struct {
	Repository
	Head  string `json:"head,omitempty"`
	State string `json:"state"`
}

// SpanChange is one declared line span of the selected source compared between the base and the
// target (BKM-V0-099).
type SpanChange struct {
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Change string `json:"change"`
}

type Report struct {
	Schema         string `json:"schema"`
	Mutates        bool   `json:"mutates"`
	Experimental   bool   `json:"experimental"`
	ManifestSHA256 string `json:"manifest_sha256"`
	API            string `json:"api"`
	Base           string `json:"base,omitempty"`
	Change         string `json:"change"`
	// DeclaredSpanChange compares the declared source span bytes at the base and the target,
	// separately from declaration analysis (BKM-V0-092).
	DeclaredSpanChange string `json:"declared_span_change,omitempty"`
	// EndpointSpanChanges compares, the same way, each distinct span an EEP /3 relation endpoint
	// declares on the selected source (BKM-V0-099).
	EndpointSpanChanges []SpanChange `json:"endpoint_span_changes,omitempty"`
	Repositories        []RepoState  `json:"repositories"`
	Edges               []Edge       `json:"edges"`
	Unknowns            []Unknown    `json:"unknowns"`
	Scope               string       `json:"scope"`
	Limits              string       `json:"limits"`
	Truncated           bool         `json:"truncated"`
	unknownSeen         map[string]bool
}

func (r *Report) unknown(subject, reason string) {
	if r.unknownSeen == nil {
		r.unknownSeen = map[string]bool{}
	}
	k := subject + "\x00" + reason
	if r.unknownSeen[k] {
		return
	}
	if len(r.Unknowns) == MaxEdges {
		r.Truncated = true
		return
	}
	r.unknownSeen[k] = true
	r.Unknowns = append(r.Unknowns, Unknown{subject, reason})
}
func (r *Report) add(e Edge) {
	if len(r.Edges) == MaxEdges {
		r.Truncated = true
		return
	}
	r.Edges = append(r.Edges, e)
}

// Compile reads only enumerated immutable blobs. Bindings are operator input and
// are deliberately absent from the portable manifest and resulting report.
func Compile(parent context.Context, raw []byte, bindings map[string]string, api, base string) (Report, error) {
	r := Report{Schema: Schema, Experimental: true, ManifestSHA256: digest(raw), API: api, Base: base, Change: "explicit-selection", Edges: []Edge{}, Unknowns: []Unknown{}, Scope: "INCOMPLETE", Limits: "8 repositories; 256 files; 1 MiB/blob; 16 MiB source total; 2000 edges and unknowns each; 30 seconds; explicit paths only"}
	m, err := Decode(raw)
	if err != nil {
		return r, err
	}
	parts := strings.Split(api, ":")
	if len(parts) != 3 || !identifier.MatchString(parts[0]) || !safePath(parts[1]) || parts[2] == "" {
		return r, errors.New("API must be REPOSITORY:PATH:SYMBOL")
	}
	if base != "" && !oid.MatchString(base) {
		return r, errors.New("base must be a full commit id in the API repository")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	repos := map[string]Repository{}
	ready := map[string]bool{}
	for _, repo := range m.Repositories {
		repos[repo.ID] = repo
	}
	for id := range bindings {
		if _, ok := repos[id]; !ok {
			return r, errors.New("undeclared repository binding")
		}
	}
	if _, ok := repos[parts[0]]; !ok {
		return r, errors.New("API repository undeclared")
	}
	for _, repo := range m.Repositories {
		state := RepoState{Repository: repo, State: "unavailable"}
		root := bindings[repo.ID]
		if root == "" {
			r.unknown(repo.ID, "checkout-unbound")
		} else if err := bind(ctx, repo, root); err != nil {
			r.unknown(repo.ID, err.Error())
		} else {
			ready[repo.ID] = true
			head, e := git(ctx, root, "rev-parse", "HEAD")
			if e != nil {
				return r, e
			}
			state.Head = strings.TrimSpace(string(head))
			state.State = "pinned"
			if state.Head != repo.Commit {
				state.State = "pinned-historical"
				r.unknown(repo.ID, "checkout HEAD differs from pinned commit; map describes historical bytes")
			}
		}
		r.Repositories = append(r.Repositories, state)
	}
	sources := map[string]captured{}
	total := 0
	for _, s := range m.Sources {
		key := sourceKey(s.Repository, s.Path)
		if !ready[s.Repository] {
			r.unknown(key, "repository-unavailable")
			continue
		}
		b, e := readBlob(ctx, bindings[s.Repository], repos[s.Repository].Commit, s.Path, s.Blob)
		if e != nil {
			r.unknown(key, e.Error())
			continue
		}
		total += len(b)
		if total > 16*MaxBytes {
			return r, errors.New("aggregate source bytes exceed bound")
		}
		if s.End > len(strings.Split(string(b), "\n")) {
			return r, errors.New("source span outside blob")
		}
		sources[key] = captured{s, repos[s.Repository], b}
	}
	apiKey := sourceKey(parts[0], parts[1])
	selected, ok := sources[apiKey]
	if !ok {
		r.unknown(apiKey, "selected API source unavailable")
	} else if strings.HasSuffix(selected.source.Path, ".go") && !moduleBoundary(ctx, bindings[selected.repo.ID], selected, sources) {
		r.unknown(apiKey, "module boundary has an undeclared or unavailable go.mod; syntax mapping withheld")
	} else {
		analyzeGo(&r, sources, selected, parts[2])
	}
	if base != "" && ready[parts[0]] {
		repo := repos[parts[0]]
		root := bindings[parts[0]]
		// ready means bind admitted this checkout, which refuses rewritten ancestry (EEP-V0-770).
		ancestry, e := git(ctx, root, "merge-base", "--all", base, repo.Commit)
		if e != nil || strings.TrimSpace(string(ancestry)) != base {
			return r, errors.New("base must be an available ancestor in the API repository")
		}
		before, e := readBlob(ctx, root, base, parts[1], "")
		r.DeclaredSpanChange = "span-unavailable"
		if e == nil && ok {
			r.DeclaredSpanChange = spanChange(before, selected.text, selected.source.Start, selected.source.End)
		}
		for _, span := range endpointSpans(m, parts[0], parts[1]) {
			change := SpanChange{Start: span[0], End: span[1], Change: "span-unavailable"}
			if e == nil && ok {
				change.Change = spanChange(before, selected.text, span[0], span[1])
			}
			r.EndpointSpanChanges = append(r.EndpointSpanChanges, change)
		}
		switch {
		case e != nil:
			r.Change = "base-path-unavailable"
			r.unknown(apiKey, "base path unavailable; added/renamed paths are not inferred")
		case !ok:
			r.Change = "target-path-unavailable"
			r.unknown(apiKey, "deleted/renamed/missing target remains unresolved")
		default:
			// BKM-V0-011: Go and JS/TS declarations resolve; other
			// languages stay file-level below.
			old, oldOK := declarationText(parts[1], before, parts[2])
			now, newOK := declarationText(parts[1], selected.text, parts[2])
			switch {
			case !oldOK || !newOK:
				// BKM-V0-009: whole-blob byte identity is language independent.
				// It bounds, but never resolves, the declaration.
				r.Change = "declaration-unresolved-file-changed"
				if string(before) == string(selected.text) {
					r.Change = "declaration-unresolved-file-unchanged"
				}
				r.unknown(apiKey, "deleted/added/renamed/unsupported declaration is not inferred")
			case old == now:
				r.Change = "selected-declaration-unchanged"
			default:
				r.Change = "selected-declaration-changed"
			}
		}
	}
	composeProviders(&r, m, sources, apiKey)
	r.unknown("scope", "only explicitly enumerated paths examined; no complete caller, behavior, test-coverage or breakage claim")
	if r.Truncated {
		r.unknown("output", "edge bound reached; relationship frontier remains open")
	}
	if ctx.Err() != nil {
		return r, errors.New("breakage map deadline exceeded")
	}
	sort.Slice(r.Unknowns, func(i, j int) bool {
		a, b := r.Unknowns[i], r.Unknowns[j]
		return a.Subject+"\x00"+a.Reason < b.Subject+"\x00"+b.Reason
	})
	sort.Slice(r.Repositories, func(i, j int) bool { return r.Repositories[i].ID < r.Repositories[j].ID })
	return r, nil
}

// spanChange compares the declared lines start..end of the target blob with the same line numbers
// of the base blob. It is positional: a shift of the span's lines reads as changed, and a base or
// target shorter than the span is unavailable (BKM-V0-092, BKM-V0-099).
func spanChange(before, now []byte, start, end int) string {
	baseLines := strings.Split(string(before), "\n")
	nowLines := strings.Split(string(now), "\n")
	if start < 1 || end < start || end > len(baseLines) || end > len(nowLines) {
		return "span-unavailable"
	}
	if strings.Join(baseLines[start-1:end], "\n") == strings.Join(nowLines[start-1:end], "\n") {
		return "declared-span-unchanged"
	}
	return "declared-span-changed"
}

func endpointKey(e extevidence.Endpoint1) string {
	if e.Repository != "" {
		return sourceKey(e.Repository, e.Path)
	}
	return "entity:" + e.Provider + ":" + e.Entity
}

// decodeProvider admits an EEP /1, /2 or /3 record (BKM-V0-012); only /3 path
// endpoints may narrow to a span or symbol.
func decodeProvider(raw []byte) (extevidence.Record1, error) {
	record, err := extevidence.Decode1(raw)
	if err == nil {
		return record, nil
	}
	if v3, err3 := extevidence.Decode3(raw); err3 == nil {
		return v3, nil
	}
	return record, err
}

// narrowEndpoint resolves an EEP /3 endpoint span or symbol against the
// pinned source bytes only (BKM-V0-012). An endpoint without either keeps the
// whole supplied span; a symbol must resolve to exactly one declaration and,
// with a span, lie within it. The returned reason is non-empty on refusal.
func narrowEndpoint(c captured, e extevidence.Endpoint1) (int, int, string) {
	start, end := c.source.Start, c.source.End
	if e.Span != nil {
		start, end = e.Span.Start, e.Span.End
	}
	if e.Symbol == "" {
		return start, end, ""
	}
	first, last, ok := declarationSpan(e.Path, c.text, e.Symbol)
	if !ok {
		return 0, 0, "EEP endpoint symbol not resolved to one declaration in the pinned blob"
	}
	if e.Span != nil && (first < start || last > end) {
		return 0, 0, "EEP endpoint symbol declaration outside its declared span"
	}
	return first, last, ""
}

func composeProviders(r *Report, m Manifest, sources map[string]captured, apiKey string) {
	if len(m.Providers) > 0 {
		reason := "provider associations bind files/entities, not the selected symbol; shared file membership does not demonstrate API dependency"
		if providersNarrowed(m.Providers) {
			reason = "provider associations narrow some endpoints to a span or declared symbol, but traversal stays file-level and the narrowing is not the selected symbol's own dependency; shared file membership does not demonstrate API dependency"
		}
		r.unknown(apiKey, reason)
	}
	for _, raw := range m.Providers {
		record, _ := decodeProvider(raw)
		hash := digest(raw)
		if !providerPinned(record, m) {
			r.unknown(record.Provider.ID, "provider repository pin mismatch or undeclared repository")
			continue
		}
		declared := map[string]bool{}
		for _, pr := range record.Repositories {
			declared[pr.ID] = true
		}
		entities := map[string]bool{}
		for _, e := range record.Entities {
			entities["entity:"+record.Provider.ID+":"+e.ID] = true
		}
		reached := map[string]bool{apiKey: true}
		used := make([]bool, len(record.Relations))
		for progress := true; progress; {
			progress = false
			for i, rel := range record.Relations {
				a, b := endpointKey(rel.From), endpointKey(rel.To)
				if used[i] || (!reached[a] && !reached[b]) {
					continue
				}
				used[i] = true
				progress = true
				edge := Edge{Kind: "provider-relationship", Confidence: "external-provider", Reason: "explicit file/entity EEP association, not symbol-specific coupling; byte verification does not establish assertion truth", ProviderSHA256: hash, Relation: &record.Relations[i], ByteState: "verified"}
				good, narrowed := true, false
				if rel.Evidence != "declared" && rel.Evidence != "observed" && rel.Evidence != "inferred" && rel.Evidence != "generated" {
					good = false
					r.unknown(record.Provider.ID, "unsupported EEP evidence kind")
				}
				if record.Schema == extevidence.Schema1 && rel.From.Repository != "" && rel.To.Repository != "" {
					good = false
					r.unknown(record.Provider.ID, "path-to-path relations require EEP V2")
				}
				for side, e := range []extevidence.Endpoint1{rel.From, rel.To} {
					key := endpointKey(e)
					if e.Repository == "" {
						if !entities[key] {
							r.unknown(key, "provider entity undeclared")
							good = false
						}
						edge.ByteState = "entity-declaration"
						continue
					}
					c, ok := sources[key]
					if !declared[e.Repository] || !safePath(e.Path) || !ok || (e.Blob != "" && c.source.Blob != e.Blob) {
						r.unknown(key, "EEP endpoint unbound, missing, out of scope or blob mismatch")
						good = false
						continue
					}
					start, end, why := narrowEndpoint(c, e)
					if why != "" {
						r.unknown(key, why)
						good = false
						continue
					}
					narrowed = narrowed || e.Span != nil || e.Symbol != ""
					anchor, valid := c.anchor(start, end)
					if !valid {
						r.unknown(key, "EEP endpoint outside supplied physical span")
						good = false
						continue
					}
					if side == 0 {
						edge.From = &anchor
					} else {
						edge.To = &anchor
					}
				}
				if narrowed {
					edge.Reason = "explicit EEP association narrowed to a pinned span or declared symbol; traversal stays file-level and byte verification does not establish assertion truth"
				}
				if !good {
					edge.ByteState = "unresolved"
				} else {
					reached[a] = true
					reached[b] = true
				}
				r.add(edge)
			}
		}
		for i, rel := range record.Relations {
			if !used[i] {
				r.unknown(fmt.Sprintf("%s:relation:%d", record.Provider.ID, i), "not reached from selected API: "+rel.Type)
			}
		}
	}
}

// endpointSpans returns the distinct [start, end] spans, in order, that the relation endpoints of
// the pin-matching EEP /3 records declare on repository:path (BKM-V0-099). A symbol-only
// endpoint declares no span and is not compared.
func endpointSpans(m Manifest, repository, path string) [][2]int {
	seen := map[[2]int]bool{}
	spans := [][2]int{}
	for _, raw := range m.Providers {
		record, err := decodeProvider(raw)
		if err != nil || !providerPinned(record, m) {
			continue
		}
		for _, rel := range record.Relations {
			for _, e := range []extevidence.Endpoint1{rel.From, rel.To} {
				if e.Span == nil || e.Repository != repository || e.Path != path {
					continue
				}
				span := [2]int{e.Span.Start, e.Span.End}
				if !seen[span] {
					seen[span] = true
					spans = append(spans, span)
				}
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		return spans[i][0] < spans[j][0] || spans[i][0] == spans[j][0] && spans[i][1] < spans[j][1]
	})
	return spans
}

// providerPinned reports whether every repository a provider record names matches a manifest pin.
func providerPinned(record extevidence.Record1, m Manifest) bool {
	for _, pr := range record.Repositories {
		found := false
		for _, mr := range m.Repositories {
			if pr.ID == mr.ID && pr.Origin == mr.Origin && pr.Revision == mr.Commit && (pr.Tree == "" || pr.Tree == mr.Tree) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// providersNarrowed reports whether any decoded provider relation endpoint carries a span or
// symbol (BKM-V0-093).
func providersNarrowed(providers []json.RawMessage) bool {
	for _, raw := range providers {
		record, err := decodeProvider(raw)
		if err != nil {
			continue
		}
		for _, rel := range record.Relations {
			for _, e := range []extevidence.Endpoint1{rel.From, rel.To} {
				if e.Span != nil || e.Symbol != "" {
					return true
				}
			}
		}
	}
	return false
}
