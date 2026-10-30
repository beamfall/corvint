package ticket

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MaxExternalReviewGates bounds a ticket's externalReviews map (ERG-V0-004).
const MaxExternalReviewGates = 16

// MaxExternalReviewEvents bounds one gate's event history (ERG-V0-004).
const MaxExternalReviewEvents = 4096

// ExternalReviewRef is one gate's pointer to its head review event
// (ERG-V0-004). It mirrors snapshot.ExternalReviewRef, which this package
// cannot import; history lives in the content-addressed events, not here.
type ExternalReviewRef struct {
	Generation, Revision wire.Count
	Head                 wire.Digest
}

func (n ExternalReviewRef) Value() wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("generation", wire.String(string(n.Generation))).Set("revision", wire.String(string(n.Revision))).Set("head", wire.String(string(n.Head))))
}

// ExternalReviewsValue encodes the optional map; nil or empty is never encoded
// by Record.Value, so a ticket without reviews keeps its legacy bytes.
func ExternalReviewsValue(m map[string]ExternalReviewRef) wire.Value {
	o := wire.NewObject()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, gate := range keys {
		o.Set(gate, m[gate].Value())
	}
	return wire.ObjectValue(o)
}

// ExternalReviewsFromValue validates a present externalReviews field: 1..16
// label keys, each a closed {generation,revision,head} with
// 1 <= generation <= revision <= 4096. An empty map is refused so that the
// field stays omitted until first use.
func ExternalReviewsFromValue(v wire.Value) (map[string]ExternalReviewRef, error) {
	r := wire.NewReader(v, "/externalReviews")
	if v.Kind != wire.KindObject {
		return nil, wire.Errorf(wire.CodeMalformed, "/externalReviews", "externalReviews must be an object")
	}
	if len(v.Obj.Keys) < 1 || len(v.Obj.Keys) > MaxExternalReviewGates {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/externalReviews", "externalReviews must name 1..%d gates", MaxExternalReviewGates)
	}
	out := map[string]ExternalReviewRef{}
	for _, gate := range v.Obj.Keys {
		if _, err := wire.ParseLabel("/externalReviews", gate); err != nil {
			return nil, err
		}
		g := r.Field(gate)
		g.Closed("generation", "revision", "head")
		ref := ExternalReviewRef{Generation: g.Field("generation").Count(), Revision: g.Field("revision").Count(), Head: g.Field("head").Digest()}
		if err := r.Err(); err != nil {
			return nil, err
		}
		if ref.Generation.Int() < 1 || ref.Generation.Int() > ref.Revision.Int() || ref.Revision.Int() > MaxExternalReviewEvents {
			return nil, wire.Errorf(wire.CodeMalformed, "/externalReviews/"+gate, "invalid reference counters")
		}
		out[gate] = ref
	}
	return out, nil
}
