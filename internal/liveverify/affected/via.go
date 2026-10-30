package affected

// viaLink is one step of a witness chain held by reference: the unit at this
// step, the step before it, and the chain's length up to here. The selector
// keeps one link per reached unit and shares every prefix, so a chain of depth
// n costs n links however many units extend it, not the sum of their chain
// lengths (V1-2979, AFP-V0-641).
type viaLink struct {
	unit   string
	prev   *viaLink
	length int
}

// chainLink returns the witness chain as a link, building links from Via for
// a witness that carries its chain as a copy.
func (witness Witness) chainLink() *viaLink {
	if witness.link != nil {
		return witness.link
	}
	var link *viaLink
	for index, unit := range witness.Via {
		link = &viaLink{unit: unit, prev: link, length: index + 1}
	}
	return link
}

// extended returns the chain of a unit one edge past this witness: this chain
// followed by unit, sharing this chain instead of copying it.
func (witness Witness) extended(unit string) *viaLink {
	prev := witness.chainLink()
	length := 1
	if prev != nil {
		length = prev.length + 1
	}
	return &viaLink{unit: unit, prev: prev, length: length}
}

// ViaLength is the number of units on the witness chain, whether the chain is
// carried in Via or held by reference (AFP-V0-641).
func (witness Witness) ViaLength() int {
	if witness.link != nil {
		return witness.link.length
	}
	return len(witness.Via)
}

// Chain returns the witness chain in order: Via when the witness carries it,
// else a fresh copy built from the shared reference (AFP-V0-641).
func (witness Witness) Chain() []string {
	if witness.link == nil {
		return witness.Via
	}
	chain := make([]string, witness.link.length)
	for link := witness.link; link != nil; link = link.prev {
		chain[link.length-1] = link.unit
	}
	return chain
}

// viaHead is the first unit of the witness chain, or "" for an empty chain.
func (witness Witness) viaHead() string {
	if witness.link == nil {
		if len(witness.Via) == 0 {
			return ""
		}
		return witness.Via[0]
	}
	link := witness.link
	for link.prev != nil {
		link = link.prev
	}
	return link.unit
}

// ViaStep is one position on a witness chain, read from the selected unit
// back toward the unit that holds the dirty path. It lets a caller share
// chain prefixes without copying the chain (AFP-V0-640, AFP-V0-641).
type ViaStep struct {
	link  *viaLink
	via   []string
	index int
}

// LastStep is the step at the end of the witness chain; false for an empty
// chain.
func (witness Witness) LastStep() (ViaStep, bool) {
	if witness.link != nil {
		return ViaStep{link: witness.link}, true
	}
	if len(witness.Via) == 0 {
		return ViaStep{}, false
	}
	return ViaStep{via: witness.Via, index: len(witness.Via) - 1}, true
}

// Unit is the unit at this step.
func (step ViaStep) Unit() string {
	if step.link != nil {
		return step.link.unit
	}
	return step.via[step.index]
}

// Index is this step's zero-based position on the chain.
func (step ViaStep) Index() int {
	if step.link != nil {
		return step.link.length - 1
	}
	return step.index
}

// Previous is the step before this one; false at the chain's first unit.
func (step ViaStep) Previous() (ViaStep, bool) {
	if step.link != nil {
		if step.link.prev == nil {
			return ViaStep{}, false
		}
		return ViaStep{link: step.link.prev}, true
	}
	if step.index == 0 {
		return ViaStep{}, false
	}
	return ViaStep{via: step.via, index: step.index - 1}, true
}

// SamePrefix reports whether the chain prefixes ending at the two steps hold
// the same units in the same order. Two steps on one shared link agree at
// once, so comparing chains that share their history costs no walk.
func (step ViaStep) SamePrefix(other ViaStep) bool {
	for {
		if step.link != nil && step.link == other.link {
			return true
		}
		if step.Index() != other.Index() || step.Unit() != other.Unit() {
			return false
		}
		previous, more := step.Previous()
		otherPrevious, otherMore := other.Previous()
		if !more || !otherMore {
			return more == otherMore
		}
		step, other = previous, otherPrevious
	}
}

// SelectLinked is Select with each selected witness chain held by reference
// instead of copied into Via: memory stays linear in the reached units however
// deep the chains are (V1-2979, AFP-V0-641). Read a chain with Witness.Chain,
// ViaLength or LastStep; Via is nil for a chain held by reference until
// MaterializeVia copies it. The plan is otherwise Select's, and
// MaterializeVia makes it equal to Select's plan.
func SelectLinked(graph *Graph, dirty []string) Plan {
	return selectPlan(graph, dirty)
}

// MaterializeVia copies every selected witness chain held by reference into
// Via, so the plan encodes exactly as Select's plan does (affected-plan/0).
func (plan *Plan) MaterializeVia() {
	for index := range plan.Selected {
		witness := &plan.Selected[index].Witness
		if witness.link != nil {
			witness.Via, witness.link = witness.Chain(), nil
		}
	}
}
