package contextindex

import (
	"regexp"
	"sort"
	"strings"
)

// contextTicketAnchor is an id token with four or more trailing digits, such
// as a Tasks ticket `V1-0523`. contextRequirementID can never match one, so no
// pass resolves it.
var contextTicketAnchor = regexp.MustCompile(`[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[0-9]{4,}`)

const (
	undefinedRequirementReason = "no tracked spec has a defining clause for this requirement id"
	ticketAnchorReason         = "a ticket-shaped id: the index holds no Tasks ticket state (TCP-V0-121), so nothing resolves it to the specs or paths its ticket cites"
	undeclaredAnchorReason     = "no indexed declaration defines this identifier (a member the extractor does not record, such as an interface method or a local type's field, is not a definition); only lexical rows can carry it"
)

// undeclaredAnchor holds for a code-shaped name: at least four bytes, a
// lower-case letter, and an inner upper-case letter or underscore, so a
// version token such as `V0` or a plain word never qualifies.
func undeclaredAnchor(name string) bool {
	if len(name) < 4 || strings.ToUpper(name) == name {
		return false
	}
	return strings.Contains(name, "_") || strings.ContainsAny(name[1:], "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
}

// unresolvedAnchors is TCP-V0-152: the id anchors the task names that no
// evidence resolves are disclosed as uncertainty, never silently dropped. It
// adds a bounded member only when there is one, and changes no ranking.
func (compiler *taskContextCompiler) unresolvedAnchors(coverage map[string]any) {
	reasons := map[string]string{}
	for _, name := range compiler.undeclared {
		reasons[name] = undeclaredAnchorReason
	}
	for _, id := range compiler.undefinedIDs {
		reasons[id] = undefinedRequirementReason
	}
	for _, span := range contextTicketAnchor.FindAllStringIndex(compiler.task, -1) {
		if idBoundary(compiler.task, span[0]-1) && idBoundary(compiler.task, span[1]) {
			reasons[compiler.task[span[0]:span[1]]] = ticketAnchorReason
		}
	}
	if len(reasons) == 0 {
		return
	}
	anchors := make([]string, 0, len(reasons))
	for anchor := range reasons {
		anchors = append(anchors, anchor)
	}
	sort.Strings(anchors)
	samples := make([]any, 0, min(len(anchors), maxExclusionSamples))
	for _, anchor := range anchors[:min(len(anchors), maxExclusionSamples)] {
		samples = append(samples, map[string]any{"anchor": anchor, "reason": reasons[anchor]})
	}
	coverage["unresolved_anchors"] = map[string]any{"count": len(anchors), "samples": samples}
}
