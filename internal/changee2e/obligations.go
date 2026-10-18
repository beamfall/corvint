package changee2e

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// Obligation states (CCE-V0-004).
const (
	Reached = "REACHED"
	Unknown = "UNKNOWN"
	None    = "NONE"
)

// Obligation is one outcome a diff puts at risk, with the changed span and the chain reaching it.
type Obligation struct {
	ID      string   `json:"id"`
	State   string   `json:"state"`
	Outcome string   `json:"outcome"`
	Screen  string   `json:"screen"`
	Spans   []Span   `json:"spans"`
	Chain   []string `json:"chain"`
}

// UnknownSpan is a changed span whose reach could not be bounded.
type UnknownSpan struct {
	Span   Span   `json:"span"`
	Reason string `json:"reason"`
}

// Obligations is the e2e-obligations/0 document.
type Obligations struct {
	Schema        string        `json:"schema"`
	Base          string        `json:"base"`
	Head          string        `json:"head"`
	CatalogDigest string        `json:"catalog_digest"`
	ImpactDigest  string        `json:"impact_digest"`
	State         string        `json:"state"`
	Obligations   []Obligation  `json:"obligations"`
	Unknown       []UnknownSpan `json:"unknown"`
	Digest        string        `json:"digest"`

	catalog *Catalog
	encoded []byte
}

// Encoded returns the sealed bytes of the report.
func (o *Obligations) Encoded() []byte { return o.encoded }

// Catalog returns the head catalog the report was joined against.
func (o *Obligations) Catalog() *Catalog { return o.catalog }

// DiffOptions name a base..head change.
type DiffOptions struct {
	Root, Base, Head string
	Manifest         string
}

// BuildObligations joins the base..head diff to outcomes through the AMAP/ANL impact relation and
// the head map's flows (CCE-V0-004). A changed screen reaches each declared outcome that a flow
// step on that screen expects; the chain is the changed paths, the screen and its basis, the step
// and the outcome. Any changed path impact cannot place reads UNKNOWN overall with its head-side
// spans, and never NONE. A diff that reaches no screen and leaves nothing unknown reads NONE.
func BuildObligations(ctx context.Context, o DiffOptions) (*Obligations, error) {
	cat, err := BuildCatalog(ctx, CatalogOptions{Root: o.Root, Rev: o.Head, Manifest: o.Manifest})
	if err != nil {
		return nil, err
	}
	base, err := resolveRev(ctx, o.Root, o.Base)
	if err != nil {
		return nil, err
	}
	raw, err := appmap.Impact(ctx, cat.m, nil, base, appmap.Options{Root: o.Root, Revision: cat.Revision, Full: true})
	if err != nil {
		return nil, refuse("e2e-impact-unavailable", "%v", err)
	}
	var impact appmap.ImpactReport
	if err := json.Unmarshal(raw, &impact); err != nil {
		return nil, refuse("e2e-impact-unavailable", "%v", err)
	}
	r := &Obligations{Schema: ObligationsSchema, Base: base, Head: cat.Revision, CatalogDigest: cat.Digest, ImpactDigest: impact.Digest,
		Obligations: []Obligation{}, Unknown: []UnknownSpan{}, catalog: cat}
	for _, u := range impact.Unknown {
		spans, err := headSpans(ctx, o.Root, base, cat.Revision, u.Path)
		if err != nil {
			return nil, err
		}
		for _, s := range spans {
			r.Unknown = append(r.Unknown, UnknownSpan{Span: s, Reason: u.Reason})
		}
	}
	byID := map[string]int{}
	for _, hit := range impact.Screens {
		var spans []Span
		for _, p := range hit.Paths {
			ss, err := headSpans(ctx, o.Root, base, cat.Revision, p)
			if err != nil {
				return nil, err
			}
			spans = append(spans, ss...)
		}
		for _, f := range cat.m.Flows {
			for _, st := range f.Steps {
				if st.Screen != hit.ID {
					continue
				}
				for _, oc := range cat.expects[st.ID] {
					id := "obligation:" + f.FlowID + "/" + oc
					if i, ok := byID[id]; ok {
						r.Obligations[i].Spans = append(r.Obligations[i].Spans, spans...)
						continue
					}
					byID[id] = len(r.Obligations)
					r.Obligations = append(r.Obligations, Obligation{ID: id, State: Reached, Outcome: "outcome:" + f.FlowID + "/" + oc,
						Screen: hit.ID, Spans: append([]Span(nil), spans...),
						Chain: []string{"paths:" + strings.Join(hit.Paths, ","), hit.ID + " (" + hit.Basis + ")", st.ID,
							"outcome:" + f.FlowID + "/" + oc}})
				}
			}
		}
	}
	for i := range r.Obligations {
		r.Obligations[i].Spans = uniqueSpans(r.Obligations[i].Spans)
	}
	sort.Slice(r.Obligations, func(i, j int) bool { return r.Obligations[i].ID < r.Obligations[j].ID })
	switch {
	case len(r.Unknown) != 0:
		r.State = Unknown
	case len(r.Obligations) != 0:
		r.State = Reached
	default:
		r.State = None
	}
	if len(r.Obligations) > MaxObligations {
		return nil, refuse("e2e-obligations-over-limit", "%d obligations exceed %d", len(r.Obligations), MaxObligations)
	}
	r.encoded, err = seal(r, &r.Digest)
	return r, err
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// headSpans returns the head-side line spans of path's change; a deletion-only hunk or a removed
// file is a zero-width span at its head position.
func headSpans(ctx context.Context, root, base, head, path string) ([]Span, error) {
	out, err := git(ctx, root, "diff", "--no-ext-diff", "--no-color", "-U0", base, head, "--", path)
	if err != nil {
		return nil, err
	}
	spans := []Span{}
	for _, line := range strings.Split(string(out), "\n") {
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		n := 1
		if m[2] != "" {
			n, _ = strconv.Atoi(m[2])
		}
		end := start + n - 1
		if n == 0 {
			end = start
		}
		spans = append(spans, Span{Path: path, Start: start, End: end})
	}
	if len(spans) == 0 {
		spans = append(spans, Span{Path: path})
	}
	return spans, nil
}

func uniqueSpans(in []Span) []Span {
	sort.Slice(in, func(i, j int) bool {
		a, b := in[i], in[j]
		return a.Path < b.Path || (a.Path == b.Path && (a.Start < b.Start || (a.Start == b.Start && a.End < b.End)))
	})
	out := []Span{}
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}
