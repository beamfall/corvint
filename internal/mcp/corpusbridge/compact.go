package corpusbridge

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// CompactView marks a compact docs_search receipt (DCP-V1-653).
const CompactView = "compact"

// CompactReceipt derives the compact docs_search receipt from a full one
// (DCP-V1-653..655). Every distinct anchor appears once in an anchors table,
// in first-seen order over citations, results and the trust envelope, with id
// a1, a2, ...; the citations, each result's evidence anchors and the trust
// envelope citations name it by id, and a trust envelope citation's
// anchor_state moves onto its table entry. A symbol anchor check names its
// anchor by id, and its live location becomes that id when it is exactly the
// anchor's pinned span. Every distinct limitation appears once in the
// top-level limitations, in first-seen order over the receipt, its trust
// envelope and its results; the envelope and each result's evidence name
// theirs by index. capabilities is kept only when withCapabilities is set.
// The full receipt is not modified; members it does not hold stay absent.
func CompactReceipt(full map[string]any, withCapabilities bool) (map[string]any, error) {
	raw, err := json.Marshal(full)
	if err != nil {
		return nil, err
	}
	var r map[string]any
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	c := compactor{ids: map[string]string{}, limits: map[string]int{}, anchors: []any{}, limitations: []any{}}
	if !withCapabilities {
		delete(r, "capabilities")
	}
	envelope, _ := r["trust_envelope"].(map[string]any)
	// The receipt's own limitations come first, so a receipt without
	// per-result or envelope limitations keeps its list unchanged.
	if own, ok := r["limitations"].([]any); ok {
		for _, l := range own {
			c.limitation(l)
		}
	}
	if citations, ok := r["citations"].([]any); ok {
		r["citations"] = c.anchorRefs(citations)
	}
	if results, ok := r["results"].([]any); ok {
		for _, v := range results {
			evidence, _ := v.(map[string]any)["evidence"].(map[string]any)
			if evidence == nil {
				continue
			}
			if anchors, ok := evidence["anchors"].([]any); ok {
				evidence["anchors"] = c.anchorRefs(anchors)
			}
			if limitations, ok := evidence["limitations"].([]any); ok {
				evidence["limitations"] = c.limitationRefs(limitations)
			}
		}
	}
	if envelope != nil {
		if citations, ok := envelope["citations"].([]any); ok {
			refs := make([]any, 0, len(citations))
			for _, v := range citations {
				citation, _ := v.(map[string]any)
				state, hasState := citation["anchor_state"]
				anchor := map[string]any{}
				for k, x := range citation {
					if k != "anchor_state" {
						anchor[k] = x
					}
				}
				id, err := c.anchor(anchor)
				if err != nil {
					return nil, err
				}
				if hasState {
					c.entry(id)["anchor_state"] = state
				}
				refs = append(refs, id)
			}
			envelope["citations"] = refs
		}
		if limitations, ok := envelope["limitations"].([]any); ok {
			envelope["limitations"] = c.limitationRefs(limitations)
		}
	}
	// A ranked result's matched span is an anchor too (DCS-V1-581), taken in
	// result id order so ids are deterministic.
	if ranks, ok := r["result_ranks"].(map[string]any); ok {
		for _, key := range slices.Sorted(maps.Keys(ranks)) {
			rank, _ := ranks[key].(map[string]any)
			if anchor, ok := rank["match_anchor"].(map[string]any); ok {
				id, err := c.anchor(anchor)
				if err != nil {
					return nil, err
				}
				rank["match_anchor"] = id
			}
		}
	}
	if checks, ok := r["symbol_anchors"].([]any); ok {
		for _, v := range checks {
			check, _ := v.(map[string]any)
			if check == nil {
				continue
			}
			formatted, _ := check["anchor"].(string)
			id := c.symbolAnchor(formatted)
			if id == "" {
				continue
			}
			check["anchor"] = id
			if live, ok := check["live"].(map[string]any); ok && c.samePin(id, live) {
				check["live"] = id
			}
		}
	}
	if c.err != nil {
		return nil, c.err
	}
	r["anchors"] = c.anchors
	r["limitations"] = c.limitations
	r["receipt_view"] = CompactView
	return r, nil
}

type compactor struct {
	ids         map[string]string
	anchors     []any
	limits      map[string]int
	limitations []any
	err         error
}

func (c *compactor) anchor(a map[string]any) (string, error) {
	key, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	if id, ok := c.ids[string(key)]; ok {
		return id, nil
	}
	id := fmt.Sprintf("a%d", len(c.anchors)+1)
	c.ids[string(key)] = id
	entry := map[string]any{"id": id}
	for k, v := range a {
		entry[k] = v
	}
	c.anchors = append(c.anchors, entry)
	return id, nil
}

func (c *compactor) entry(id string) map[string]any {
	var n int
	fmt.Sscanf(id, "a%d", &n)
	return c.anchors[n-1].(map[string]any)
}

func (c *compactor) anchorRefs(anchors []any) []any {
	refs := make([]any, 0, len(anchors))
	for _, v := range anchors {
		a, ok := v.(map[string]any)
		if !ok {
			refs = append(refs, v)
			continue
		}
		id, err := c.anchor(a)
		if err != nil && c.err == nil {
			c.err = err
		}
		refs = append(refs, id)
	}
	return refs
}

func (c *compactor) limitation(l any) int {
	text, _ := l.(string)
	if i, ok := c.limits[text]; ok {
		return i
	}
	c.limits[text] = len(c.limitations)
	c.limitations = append(c.limitations, text)
	return c.limits[text]
}

func (c *compactor) limitationRefs(limitations []any) []any {
	refs := make([]any, 0, len(limitations))
	for _, l := range limitations {
		refs = append(refs, c.limitation(l))
	}
	return refs
}

// symbolAnchor finds the table anchor a symbol anchor check names in the
// path:START-END#Symbol@REV form (DCP-V1-141).
func (c *compactor) symbolAnchor(formatted string) string {
	for _, v := range c.anchors {
		a := v.(map[string]any)
		start, _ := a["start_line"].(float64)
		end, _ := a["end_line"].(float64)
		if fmt.Sprintf("%v:%d-%d#%v@%v", a["path"], int(start), int(end), a["symbol"], a["revision"]) == formatted {
			return a["id"].(string)
		}
	}
	return ""
}

// samePin reports whether a live symbol location is exactly the anchor's
// pinned span, ignoring only the known optional scalar defaults emitted by
// doccorpus's json/v2 encoder. Nonzero or unknown metadata must stay visible.
func (c *compactor) samePin(id string, live map[string]any) bool {
	a := c.entry(id)
	pins := 0
	for k := range live {
		switch k {
		case "revision", "path", "blob", "sha256", "start_line", "end_line", "span_sha256":
			if live[k] != a[k] {
				return false
			}
			pins++
		case "declaration_line":
			if live[k] != float64(0) {
				return false
			}
		case "estimated":
			if live[k] != false {
				return false
			}
		default:
			return false
		}
	}
	return pins == 7
}
