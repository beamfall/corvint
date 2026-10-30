package wire

// Closed facet key sets (CAL-V0-206, amendment A26). They mirror the ticket
// enums the summary counts; the ticket and cli tests keep them equal.
var (
	FacetStatuses         = []string{"DRAFT", "OPEN", "HELD", "COMPLETED", "ARCHIVED"}
	FacetPriorities       = []string{"P0", "P1", "P2", "P3"}
	FacetKinds            = []string{"FEATURE", "BUG", "CHORE", "SPIKE", "DOC", "MANUAL", "EXTERNAL"}
	FacetExecutionClasses = []string{"AUTONOMOUS", "APPROVAL_REQUIRED", "MANUAL", "EXTERNAL", "NEVER"}
	FacetEligibilities    = []string{"BLOCKED", "UNKNOWN"}
	FacetNextActions      = []string{"admit", "answer", "complete-manual", "cutover", "grant-approval", "refine", "release-hold", "reopen", "restore", "set-dependencies", "set-effects", "wait-attempt", "wait-dependency"}
)

// MaxFacetOpenKeys bounds byMilestone and byLabel (CAL-V0-206).
const MaxFacetOpenKeys = 64

// decodeFacets validates the closed CAL-V0-206 summary (V1-1052): exactly the
// specified members, wire Counts throughout, every closed enum value present
// in the five closed maps and summing to total, only known non-zero keys in
// the open maps, and the 64-key cap with its omitted count.
func decodeFacets(r *Reader) {
	r.Closed("total", "byStatus", "byPriority", "byKind", "byExecutionClass", "byEligibility",
		"byNextAction", "byBlockerCode", "byMilestone", "milestonesOmitted", "withoutMilestone",
		"byLabel", "labelsOmitted")
	if r.st.err != nil {
		return
	}
	total := r.Field("total").Count().Int()
	for _, closed := range []struct {
		key  string
		enum []string
	}{{"byStatus", FacetStatuses}, {"byPriority", FacetPriorities}, {"byKind", FacetKinds}, {"byExecutionClass", FacetExecutionClasses}, {"byEligibility", FacetEligibilities}} {
		f := r.Field(closed.key)
		f.Closed(closed.enum...)
		if sum := facetCounts(f, total, false, nil); r.st.err == nil && sum != total {
			f.Fail(CodeMalformed, "counts sum to %d, not total %d", sum, total)
		}
	}
	next := r.Field("byNextAction")
	known := map[string]bool{}
	for _, a := range FacetNextActions {
		known[a] = true
	}
	if sum := facetCounts(next, total, true, func(k string) bool { return known[k] }); r.st.err == nil && sum != total {
		next.Fail(CodeMalformed, "counts sum to %d, not total %d", sum, total)
	}
	facetCounts(r.Field("byBlockerCode"), total, true, IsCode)
	isLabel := func(k string) bool { _, err := ParseLabel("", k); return err == nil }
	without := r.Field("withoutMilestone").Count().Int()
	milestones := r.Field("byMilestone")
	sum := facetCounts(milestones, total, true, isLabel)
	facetCap(r, milestones, "milestonesOmitted")
	if r.st.err == nil && without+sum > total {
		milestones.Fail(CodeMalformed, "milestone counts and withoutMilestone exceed total %d", total)
	}
	labels := r.Field("byLabel")
	facetCounts(labels, total, true, isLabel)
	facetCap(r, labels, "labelsOmitted")
}

// facetCounts requires an object of Counts, each at most total, with keys
// admitted by key when key is non-nil and non-zero counts when open, and
// returns their sum.
func facetCounts(f *Reader, total int64, open bool, key func(string) bool) int64 {
	if f.st.err != nil {
		return 0
	}
	if f.v.Kind != KindObject || f.v.Obj == nil {
		f.Fail(CodeMalformed, "expected object, got %s", f.v.Kind)
		return 0
	}
	sum := int64(0)
	for _, k := range f.v.Obj.Keys {
		c := f.Field(k)
		if key != nil && !key(k) {
			c.Fail(CodeMalformed, "unknown facet key %q", k)
			return 0
		}
		n := c.Count().Int()
		if f.st.err != nil {
			return 0
		}
		if n > total || (open && n == 0) {
			c.Fail(CodeMalformed, "count %d outside 1..total %d", n, total)
			return 0
		}
		sum += n
	}
	return sum
}

// facetCap enforces the open-map cap: at most MaxFacetOpenKeys keys, and
// exactly that many whenever the omitted count is non-zero.
func facetCap(r, f *Reader, omittedKey string) {
	omitted := r.Field(omittedKey).Count().Int()
	if r.st.err != nil {
		return
	}
	n := len(f.v.Obj.Keys)
	if n > MaxFacetOpenKeys || (omitted > 0 && n != MaxFacetOpenKeys) {
		f.Fail(CodeMalformed, "%d keys with %s %d (cap %d)", n, omittedKey, omitted, MaxFacetOpenKeys)
	}
}
