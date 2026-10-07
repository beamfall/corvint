package appmap

import "context"

// AuthorityLearned is the only authority an overlay fact carries (AMAP-V0-014).
const AuthorityLearned = "learned"

// Fact is one learned statement about a map element, keyed by the element's stable ID
// (`screen:`, `flow:`, `step:`, `file:`, `method:` or `selector:`). A fact is advisory: a
// projection prints it in its `learned` section and never lets it change a node, an edge, a
// selector's strength or a freshness verdict (AMAP-V0-014).
type Fact struct {
	ElementID string `json:"element_id"`
	// Source names the overlay provider, for example a run-evidence or know-how store.
	Source string `json:"source"`
	// Kind is the provider's own fact class, for example `run-verified` or `note`.
	Kind string `json:"kind"`
	// Revision is the commit the fact was observed or written at, when the provider knows it.
	Revision string `json:"revision,omitempty"`
	Text     string `json:"text"`
	// Authority is always AuthorityLearned on output, whatever the provider set.
	Authority string `json:"authority"`
}

// Overlay supplies learned facts for map elements. Facts receives the sorted, unique element IDs a
// projection is about to print and returns facts for any of them; facts naming other IDs are
// ignored. It must be read-only and bounded: a projection calls it once per invocation.
type Overlay interface {
	Facts(ctx context.Context, elementIDs []string) ([]Fact, error)
}
