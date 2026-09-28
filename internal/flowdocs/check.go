package flowdocs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

type AnchorCheck struct {
	State  string            `json:"state"`
	Before doccorpus.Anchor  `json:"before"`
	After  *doccorpus.Anchor `json:"after,omitempty"`
	Reason string            `json:"reason"`
}
type BindingCheck struct {
	ID       string            `json:"id"`
	State    string            `json:"state"`
	Before   doccorpus.Anchor  `json:"before"`
	After    *doccorpus.Anchor `json:"after,omitempty"`
	Reason   string            `json:"reason"`
	Evidence []AnchorCheck     `json:"evidence"`
}
type Check struct {
	Source            doccorpus.Repository `json:"source"`
	Target            doccorpus.Repository `json:"target"`
	Bindings          []BindingCheck       `json:"bindings"`
	Added             []string             `json:"added"`
	Retired           []string             `json:"retired"`
	OutputDifferences []string             `json:"output_differences"`
	Clean             bool                 `json:"clean"`
}
type checkItem struct {
	anchors  []doccorpus.Anchor
	imported bool
}

func collectBindings(m Manifest) map[string]checkItem {
	r := map[string]checkItem{}
	for _, f := range m.Flows {
		r[f.ID] = checkItem{anchors: []doccorpus.Anchor{f.Paragraphs[0].Anchor}}
		for _, p := range f.Paragraphs {
			anchors := []doccorpus.Anchor{p.Anchor}
			if p.Imported {
				anchors = p.Bindings
			}
			r[p.ID] = checkItem{anchors: anchors, imported: p.Imported}
		}
	}
	for _, c := range m.Provider.Claims {
		r[c.ID] = checkItem{anchors: c.Evidence.Anchors}
	}
	return r
}
func CheckRevision(ctx context.Context, root string, previous []byte, revision, output string) (Check, error) {
	old, err := Open(ctx, root, previous)
	if err != nil {
		return Check{}, err
	}
	current, err := Generate(ctx, root, Options{Revision: revision, Scope: old.Manifest.Scope})
	if err != nil {
		return Check{}, err
	}
	result := Check{Source: old.Manifest.Source, Target: current.Manifest.Source, Bindings: []BindingCheck{}, Added: []string{}, Retired: []string{}, OutputDifferences: []string{}, Clean: true}
	before, after := collectBindings(old.Manifest), collectBindings(current.Manifest)
	keys := []string{}
	for id := range before {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	memo := map[string]AnchorCheck{}
	scanBytes := 0
	var auth *gitauth.Repository
	var index *contextindex.Index
	for _, id := range keys {
		item := before[id]
		row := BindingCheck{ID: id, State: "fresh", Reason: "all original bindings remain fresh", Evidence: []AnchorCheck{}}
		if len(item.anchors) > 0 {
			row.Before = item.anchors[0]
		}
		if item.imported {
			if auth == nil {
				auth, err = gitauth.Open(root, gitrun.NewDefaultBudget())
				if err != nil {
					return result, err
				}
				release := auth.BeginObjectSession()
				defer release()
				index, err = contextindex.BuildRevisionContext(ctx, root, revision)
				if err != nil {
					return result, err
				}
			}
			hasSource := false
			for _, a := range item.anchors {
				checked := AnchorCheck{State: "fresh", Before: a, After: &a, Reason: "original historical witness remains pinned; no claim of target execution"}
				if a.Revision == old.Manifest.Source.Revision {
					hasSource = true
					key := a.Path + ":" + a.SpanSHA256 + ":" + fmt.Sprint(a.End-a.Start+1)
					if cached, ok := memo[key]; ok {
						checked = cached
						checked.Before = a
					} else {
						checked, err = relocateAnchor(ctx, auth, index, current.Manifest.Source, a, &scanBytes)
						memo[key] = checked
					}
					if err != nil {
						return result, err
					}
				}
				row.Evidence = append(row.Evidence, checked)
			}
			if !hasSource {
				row.State = "unresolved"
				row.Reason = "imported paragraph has no binding to target source; historical evidence alone cannot establish current behavior"
			}
		} else if target, ok := after[id]; ok {
			if len(item.anchors) != len(target.anchors) {
				row.State = "stale"
				row.Reason = "evidence anchor inventory changed"
			}
			for i, a := range item.anchors {
				checked := AnchorCheck{State: "unresolved", Before: a, Reason: "anchor missing"}
				if i < len(target.anchors) {
					b := target.anchors[i]
					checked.After = &b
					checked.State = "fresh"
					checked.Reason = "same logical identity and source span; relocation allowed"
					if a.Path != b.Path || a.SpanSHA256 != b.SpanSHA256 {
						checked.State = "stale"
						checked.Reason = "anchored source span changed"
					}
				}
				row.Evidence = append(row.Evidence, checked)
			}
		} else {
			row.State = "unresolved"
			row.Reason = "logical identity absent/ambiguous; retired without inferred rename"
			result.Retired = append(result.Retired, id)
		}
		for i, e := range row.Evidence {
			if i == 0 {
				row.After = e.After
			}
			if e.State == "unresolved" {
				row.State = "unresolved"
				row.Reason = e.Reason
			} else if e.State == "stale" && row.State == "fresh" {
				row.State = "stale"
				row.Reason = e.Reason
			}
		}
		if row.State != "fresh" {
			result.Clean = false
		}
		result.Bindings = append(result.Bindings, row)
	}
	for id := range after {
		if _, ok := before[id]; !ok {
			result.Added = append(result.Added, id)
		}
	}
	sort.Strings(result.Added)
	if len(result.Added) > 0 {
		result.Clean = false
	}
	if output != "" {
		result.OutputDifferences, err = CompareOutput(output, old.Files)
		if err != nil {
			return result, err
		}
		if len(result.OutputDifferences) > 0 {
			result.Clean = false
		}
	}
	return result, nil
}
func relocateAnchor(ctx context.Context, auth *gitauth.Repository, index *contextindex.Index, repository doccorpus.Repository, a doccorpus.Anchor, scanBytes *int) (AnchorCheck, error) {
	row := AnchorCheck{State: "unresolved", Before: a, Reason: "original source path missing or unsupported"}
	if _, ok := index.Tracked[a.Path]; !ok {
		return row, nil
	}
	entry, ok, err := auth.LookupTreeEntry(ctx, repository.Revision, a.Path)
	if err != nil {
		return row, err
	}
	if !ok || entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" {
		return row, nil
	}
	raw, err := auth.BlobBytes(ctx, entry.OID)
	if err != nil {
		return row, err
	}
	if len(raw) > 4<<20 {
		return row, fail("imported source exceeds revalidation bound")
	}
	lines := strings.SplitAfter(string(raw), "\n")
	width := a.End - a.Start + 1
	if width < 1 || width > 4096 {
		return row, fail("imported anchor span exceeds revalidation bound")
	}
	// An unchanged immutable blob already fixes the original line binding.
	// Repeated equal spans elsewhere in that blob cannot make it ambiguous.
	if entry.OID == a.Blob && doccorpus.Digest(raw) == a.SHA256 && a.Start >= 1 && a.End <= len(lines) && doccorpus.Digest([]byte(strings.Join(lines[a.Start-1:a.End], ""))) == a.SpanSHA256 {
		b := a
		b.Revision = repository.Revision
		row.After = &b
		row.State = "fresh"
		row.Reason = "unchanged immutable blob preserves original exact span"
		return row, nil
	}
	matches := 0
	for i := 0; i+width <= len(lines); i++ {
		span := strings.Join(lines[i:i+width], "")
		*scanBytes += len(span)
		if *scanBytes > MaxBytes {
			return row, fail("imported span scan exceeds 64 MiB; narrow corpus context")
		}
		if doccorpus.Digest([]byte(span)) != a.SpanSHA256 {
			continue
		}
		matches++
		b := a
		b.Revision = repository.Revision
		b.Blob = entry.OID
		b.SHA256 = doccorpus.Digest(raw)
		b.Start = i + 1
		b.End = i + width
		row.After = &b
	}
	switch matches {
	case 0:
		row.State = "stale"
		row.Reason = "original imported span changed"
	case 1:
		row.State = "fresh"
		row.Reason = "original imported span uniquely revalidated at target source"
	default:
		row.After = nil
		row.Reason = "original imported span matches multiple target locations"
	}
	return row, nil
}
