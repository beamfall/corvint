// Package console is the read-through local admin console of
// `docs/specs/local-admin-console-v0.md` (decision 0081). It renders what the
// owning tools report and performs a change only by invoking the owning
// tool's own verb; it holds no database, opens no outbound connection, and
// carries no authority of its own.
package console

// Axis is one of the six independent evidence axes of
// `docs/specs/local-observability-dashboard-v0.md`. Their closed values are
// reproduced here so the console can reject a value outside them rather than
// pass an unrecognized string through as if it were an axis.
type Axis string

const (
	AxisValidity       Axis = "validity"
	AxisEpistemicClass Axis = "epistemicClass"
	AxisAuthorityClass Axis = "authorityClass"
	AxisCompleteness   Axis = "completeness"
	AxisCurrency       Axis = "currency"
	AxisDeliveryStage  Axis = "deliveryStage"
)

// Unstated is what the console renders for an axis its source did not state.
// It is deliberately not one of the closed values: LAC-V0-007 forbids the
// console from deriving or defaulting an axis, and every closed value would
// be a claim the source never made. `taskman-command-result/0` states none of
// the six, so most values a board renders carry this.
const Unstated = "NOT_STATED"

// axisValues are the closed values of each axis, strongest first where the
// axis is ordered. `currency` has no CURRENT: a dashboard value is at best
// VALIDATED_AT, because the observation boundary has already passed.
var axisValues = map[Axis][]string{
	AxisValidity:       {"VALID", "INVALID", "NOT_PRESENT", "INACCESSIBLE", "UNSUPPORTED", "DISABLED", "EXPIRED"},
	AxisEpistemicClass: {"OBSERVED", "DECLARED", "ADVISORY", "NOT_OBSERVED"},
	AxisAuthorityClass: {"REPOSITORY_ACCEPTED", "OWNING_VERIFIER", "PROVIDER_QUALIFIED", "ADAPTER_QUALIFIED", "CALLER_REPORTED", "ADVISORY", "NONE"},
	AxisCompleteness:   {"COMPLETE", "PARTIAL", "UNKNOWN"},
	AxisCurrency:       {"VALIDATED_AT", "HISTORICAL", "STALE", "MIXED", "UNKNOWN"},
	AxisDeliveryStage:  {"ACCEPTED", "VALIDATED", "IMPLEMENTED", "EXPERIMENTAL", "NOT_STARTED", "FAILED", "UNSUPPORTED"},
}

// Axes is one value's six axes as its source stated them. An axis the source
// did not state stays Unstated; nothing in this package fills one in.
type Axes struct {
	Validity       string
	EpistemicClass string
	AuthorityClass string
	Completeness   string
	Currency       string
	DeliveryStage  string
}

// UnstatedAxes is the starting point for a source that states no axis at all,
// which is every `taskman-command-result/0` envelope today.
func UnstatedAxes() Axes {
	return Axes{Unstated, Unstated, Unstated, Unstated, Unstated, Unstated}
}

// Set records one axis as its source stated it. A value outside the axis's
// closed set is refused rather than stored, so an unrecognized string can
// never reach the page dressed as an axis.
func (a *Axes) Set(axis Axis, value string) bool {
	for _, allowed := range axisValues[axis] {
		if allowed != value {
			continue
		}
		switch axis {
		case AxisValidity:
			a.Validity = value
		case AxisEpistemicClass:
			a.EpistemicClass = value
		case AxisAuthorityClass:
			a.AuthorityClass = value
		case AxisCompleteness:
			a.Completeness = value
		case AxisCurrency:
			a.Currency = value
		case AxisDeliveryStage:
			a.DeliveryStage = value
		}
		return true
	}
	return false
}

// Pairs renders the axes in the table order of the owning spec, so a reader
// meets them in the same order everywhere.
func (a Axes) Pairs() []AxisPair {
	return []AxisPair{
		{AxisValidity, a.Validity},
		{AxisEpistemicClass, a.EpistemicClass},
		{AxisAuthorityClass, a.AuthorityClass},
		{AxisCompleteness, a.Completeness},
		{AxisCurrency, a.Currency},
		{AxisDeliveryStage, a.DeliveryStage},
	}
}

// AxisPair is one axis and the value its source stated.
type AxisPair struct {
	Axis  Axis
	Value string
}

// Stated reports whether the source stated this axis at all.
func (p AxisPair) Stated() bool { return p.Value != Unstated }

// Weakest combines the axes of several contributing sources into the axes of
// a value derived from all of them (LAC-V0-010). Every axis takes the weakest
// contribution: for an ordered axis that is the last value in its closed
// list, and an unstated contribution makes the result unstated, because a
// value can be no better supported than the source that says least about it.
func Weakest(contributions ...Axes) Axes {
	if len(contributions) == 0 {
		return UnstatedAxes()
	}
	out := contributions[0]
	for _, next := range contributions[1:] {
		out = Axes{
			Validity:       weakestOf(AxisValidity, out.Validity, next.Validity),
			EpistemicClass: weakestOf(AxisEpistemicClass, out.EpistemicClass, next.EpistemicClass),
			AuthorityClass: weakestOf(AxisAuthorityClass, out.AuthorityClass, next.AuthorityClass),
			Completeness:   weakestOf(AxisCompleteness, out.Completeness, next.Completeness),
			Currency:       weakestOf(AxisCurrency, out.Currency, next.Currency),
			DeliveryStage:  weakestOf(AxisDeliveryStage, out.DeliveryStage, next.DeliveryStage),
		}
	}
	return out
}

// weakestOf picks the later of two values in the axis's closed order. An
// unstated side wins outright: nothing supports the combination better than
// the contribution that stated nothing.
func weakestOf(axis Axis, left, right string) string {
	if left == Unstated || right == Unstated {
		return Unstated
	}
	if left == right {
		return left
	}
	for _, value := range axisValues[axis] {
		if value == left {
			return right
		}
		if value == right {
			return left
		}
	}
	return Unstated
}
