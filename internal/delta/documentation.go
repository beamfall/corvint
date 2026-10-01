package delta

import (
	"context"
	json "encoding/json/v2"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/flowdocs"
	"io/fs"
	"path"
	"sort"
	"strings"
)

func (r *Record) documentation(ctx context.Context, root string, o Options) {
	previous, previousValid := r.previous(ctx, root, o)
	if len(r.ChangedPaths) == 0 {
		return
	}
	if hasCode(r, "immutable-graph-unavailable") {
		r.unknown("documentation-incomplete", 1, nil)
		return
	}
	r.unknown("runtime-behaviour-unclassified", 1, nil)
	scopes := map[string]bool{}
	for _, p := range r.ChangedPaths {
		scope, _, _ := strings.Cut(p, "/")
		scopes[scope] = true
	}
	names := make([]string, 0, len(scopes))
	for s := range scopes {
		names = append(names, s)
	}
	sort.Strings(names)
	if len(names) > 32 {
		r.unknown("documentation-incomplete", len(names)-32, nil)
		r.RunFullSuite = true
		names = names[:32]
	}
	touched := map[string]bool{}
	for _, scope := range names {
		if !r.documentationScope(ctx, root, o.Base, scope) || !r.documentationScope(ctx, root, o.Head, scope) {
			r.RunFullSuite = true
			continue
		}
		before, e1 := flowdocs.Generate(ctx, root, flowdocs.Options{Revision: o.Base, Scope: scope})
		after, e2 := flowdocs.Generate(ctx, root, flowdocs.Options{Revision: o.Head, Scope: scope})
		if e1 != nil || e2 != nil {
			r.unknown("documentation-incomplete", 1, nil)
			r.RunFullSuite = true
			continue
		}
		if additionalGaps(before.Manifest.Gaps) || additionalGaps(after.Manifest.Gaps) {
			r.unknown("documentation-incomplete", 1, nil)
			r.RunFullSuite = true
		}
		old := map[string]flowdocs.Flow{}
		current := map[string]flowdocs.Flow{}
		for _, f := range before.Manifest.Flows {
			if contains(r.ChangedPaths, f.Path) {
				old[f.ID] = f
			}
		}
		for _, f := range after.Manifest.Flows {
			if contains(r.ChangedPaths, f.Path) {
				current[f.ID] = f
			}
		}
		ids := map[string]bool{}
		for id := range old {
			ids[id] = true
		}
		for id := range current {
			ids[id] = true
		}
		ordered := make([]string, 0, len(ids))
		for id := range ids {
			ordered = append(ordered, id)
		}
		sort.Strings(ordered)
		for _, id := range ordered {
			a, aok := old[id]
			b, bok := current[id]
			state := "unchanged"
			f := b
			switch {
			case !aok:
				state = "added"
			case !bok:
				state = "retired"
				f = a
			case flowDigest(a) != flowDigest(b):
				state = "stale"
			}
			opaque := digest("flow", []byte(id))
			touched[opaque] = true
			r.addSpan(Span{opaque, f.Path, state, flowDigest(f)})
		}
	}
	r.Denominators.LexicalFlows = len(touched)
	ids := make([]string, 0, len(touched))
	for id := range touched {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r.Gaps = append(r.Gaps, Gap{id, "missing-flow-assertion", "lexical-flows"})
	}
	if len(ids) > 0 {
		r.unknown("flow-assertion-witness-unavailable", len(ids), nil)
	}
	if !previousValid {
		return
	}
	raw := previous
	check, err := flowdocs.CheckRevision(ctx, root, raw, o.Head, "")
	if err != nil {
		r.unknown("documentation-incomplete", 1, nil)
		return
	}
	if !check.Clean {
		r.unknown("documentation-stale", 1, nil)
	}
	for _, b := range check.Bindings {
		state := "unknown"
		switch b.State {
		case "fresh":
			state = "unchanged"
		case "stale":
			state = "stale"
		}
		data, _ := json.Marshal(b.Evidence, json.Deterministic(true))
		r.addSpan(Span{digest("binding", []byte(b.ID)), b.Before.Path, state, digest("binding-evidence", data)})
	}
	for _, id := range check.Added {
		r.addSpan(Span{digest("binding", []byte(id)), "", "added", digest("binding", []byte(id))})
	}
	for _, id := range check.Retired {
		r.addSpan(Span{digest("binding", []byte(id)), "", "retired", digest("binding", []byte(id))})
	}
}
func flowDigest(f flowdocs.Flow) string {
	// Identity-neutral span digest: commit/paragraph prose never enters the wire.
	spans := []string{}
	for _, p := range f.Paragraphs {
		spans = append(spans, p.Anchor.SpanSHA256)
	}
	raw, _ := json.Marshal(spans)
	return digest("flow-spans", raw)
}
func (r *Record) addSpan(s Span) {
	if len(r.Documentation) >= MaxObservations {
		r.unknown("observation-bound-exceeded", 1, nil)
		r.RunFullSuite = true
		return
	}
	r.Documentation = append(r.Documentation, s)
}

func hasCode(r *Record, code string) bool {
	for _, u := range r.Unknowns {
		if u.Code == code {
			return true
		}
	}
	return false
}
func additionalGaps(gaps []string) bool {
	for _, g := range gaps {
		switch g {
		case "Source-only lexical candidates: dynamic dispatch, metaprogramming, regex literals, interpolation, unsupported templates and runtime order are unresolved.", "Accepted variation denominator and runtime coverage are unknown unless separately supplied by revalidated corpus evidence.", "Gitlinks are opaque superproject identities; their contents are not inspected.":
		default:
			return true
		}
	}
	return false
}

// Native flowdocs HTML uses a post-allocation bound. This source slice does
// not broaden that reader: any scoped HTML refuses before Generate/Open/check.
// Native lexical exclusions are separately retained; manifest prose is not an
// inventory of excluded inputs.
func (r *Record) documentationScope(ctx context.Context, root, revision, scope string) bool {
	index, err := contextindex.BuildRevisionContext(ctx, root, revision)
	if err != nil {
		r.unknown("documentation-incomplete", 1, nil)
		return false
	}
	scoped := func(p string) bool { return p == scope || strings.HasPrefix(p, scope+"/") }
	names := make([]string, 0, len(index.Tracked))
	for p := range index.Tracked {
		names = append(names, p)
	}
	sort.Strings(names)
	safe := true
	for _, p := range names {
		if !scoped(p) {
			continue
		}
		ext := path.Ext(p)
		if ext == ".html" {
			r.unknown("documentation-incomplete", 1, []byte(p))
			safe = false
			continue
		}
		if isDocumentation(p) {
			continue
		}
		source, found := index.Sources[p]
		_, valid, loaded := source.Text()
		if !found || !valid || !loaded || !oneOf(ext, ".rb", ".js", ".jsx", ".ts", ".tsx") {
			r.unknown("documentation-incomplete", 1, []byte(p))
		}
	}
	for _, p := range index.Unparsed {
		if scoped(p.Path) {
			r.unknown("documentation-incomplete", 1, []byte(p.Path))
		}
	}
	for _, p := range index.ExtractionNotes {
		if scoped(p.Path) {
			r.unknown("documentation-incomplete", 1, []byte(p.Path))
		}
	}
	return safe
}
func (r *Record) previous(ctx context.Context, root string, o Options) ([]byte, bool) {
	if o.PreviousGeneration == "" {
		if len(r.ChangedPaths) > 0 {
			r.unknown("documentation-baseline-unavailable", 1, nil)
		}
		return nil, false
	}
	raw, err := capture(root, o.PreviousGeneration, MaxPreviousBytes)
	if err != nil {
		r.unknown("documentation-baseline-invalid", 1, nil)
		return nil, false
	}
	r.InputDigests = append(r.InputDigests, digest("previous-generation", raw))
	var header struct {
		Source doccorpus.Repository `json:"source"`
		Scope  string               `json:"scope"`
	}
	if json.Unmarshal(raw, &header) != nil || header.Source.Revision != o.Base || !fs.ValidPath(header.Scope) || header.Scope == "." || strings.ContainsAny(header.Scope, "\\\x00\r\n") {
		r.unknown("documentation-baseline-invalid", 1, nil)
		return nil, false
	}
	if !r.documentationScope(ctx, root, o.Base, header.Scope) || !r.documentationScope(ctx, root, o.Head, header.Scope) {
		r.unknown("documentation-baseline-invalid", 1, nil)
		r.RunFullSuite = true
		return nil, false
	}
	old, err := flowdocs.Open(ctx, root, raw)
	if err != nil || old.Manifest.Source.Revision != o.Base {
		r.unknown("documentation-baseline-invalid", 1, nil)
		return nil, false
	}
	repo, err := contextindex.CorpusRepositoryID(ctx, root, o.Base)
	if err != nil || repo != old.Manifest.Source.ID {
		r.unknown("documentation-baseline-invalid", 1, nil)
		return nil, false
	}
	return raw, true
}
