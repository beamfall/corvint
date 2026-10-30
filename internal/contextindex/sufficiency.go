package contextindex

import (
	"fmt"
	"strings"
)

// TCP-V0-028: the evidence-set sufficiency check. The task's anchors are the
// tracked paths it names (the `mentioned` rule) and its specific names
// (TCP-V0-016's terms). Each anchor is `satisfied` only when a selected span
// row's own lines, read back from the pinned source, carry it; `insufficient`
// when the index shows evidence for it that no selected span carries; and
// `unknown` when the index cannot tell. The set is `satisfied` only when every
// anchor is, so missing evidence never reads as sufficient (invariant 2).
const contextSufficiencyListCap = 16

const (
	sufficiencySatisfied    = "satisfied"
	sufficiencyInsufficient = "insufficient"
	sufficiencyUnknown      = "unknown"
)

type anchorVerdict struct {
	anchor, kind, state, detail string
}

type sufficiencyVerdict struct {
	verdict, reason string
	anchors         []anchorVerdict
}

// sufficiency checks every anchor against the selected spans; lines reads a
// span's source the way the ranker did.
func (compiler *taskContextCompiler) sufficiency(spans []contextSpan, lines func(string) ([]string, bool)) sufficiencyVerdict {
	anchors := make([]anchorVerdict, 0)
	for _, anchor := range compiler.mentionedPaths() {
		anchors = append(anchors, compiler.pathAnchor(anchor, spans))
	}
	for _, term := range compiler.answerability.terms {
		anchors = append(anchors, nameAnchor(term, spans, lines))
	}
	return sufficiencyVerdict{verdict: setVerdict(anchors), reason: setReason(anchors), anchors: anchors}
}

func (compiler *taskContextCompiler) pathAnchor(anchor string, spans []contextSpan) anchorVerdict {
	for _, span := range spans {
		if span.path == anchor {
			return anchorVerdict{anchor, "path", sufficiencySatisfied, "span " + spanLabel(span)}
		}
	}
	if _, pinned := compiler.index.Sources[anchor]; pinned {
		return anchorVerdict{anchor, "path", sufficiencyInsufficient, "the path is indexed but no selected span reads it"}
	}
	return anchorVerdict{anchor, "path", sufficiencyUnknown, "the path is not indexed: " + evidenceGapReason(compiler.index, anchor)}
}

// nameAnchor is satisfied by a span whose lines carry the name as a whole
// word, or whose path carries it (TCP-V0-016 counts a path naming `x_test`).
func nameAnchor(term specificTerm, spans []contextSpan, lines func(string) ([]string, bool)) anchorVerdict {
	for _, span := range spans {
		if spanCarries(span, term.term, lines) {
			return anchorVerdict{term.term, "name", sufficiencySatisfied, "carried by " + spanLabel(span)}
		}
	}
	if term.known() {
		return anchorVerdict{term.term, "name", sufficiencyInsufficient,
			fmt.Sprintf("%d indexed sources and %d tracked paths name it; no selected span carries it", len(term.sources), term.tracked)}
	}
	return anchorVerdict{term.term, "name", sufficiencyUnknown,
		"no indexed source or tracked path names it; it may sit in an unread file or be added by the change"}
}

func spanCarries(span contextSpan, name string, lines func(string) ([]string, bool)) bool {
	if strings.Contains(strings.ToLower(span.path), strings.ToLower(name)) {
		return true
	}
	text, ok := lines(span.path)
	if !ok || span.start < 1 || span.end > len(text) || span.start > span.end {
		return false
	}
	count, _ := countWholeWord(strings.Join(text[span.start-1:span.end], "\n"), name)
	return count > 0
}

func spanLabel(span contextSpan) string {
	return fmt.Sprintf("%s:%d-%d", span.path, span.start, span.end)
}

// setVerdict: no anchors makes the set unknown; otherwise one insufficient
// anchor makes it insufficient, even beside an unknown anchor; else one unknown
// anchor makes it unknown; only all satisfied is satisfied (TCP-V0-028).
func setVerdict(anchors []anchorVerdict) string {
	if len(anchors) == 0 {
		return sufficiencyUnknown
	}
	states := map[string]int{}
	for _, anchor := range anchors {
		states[anchor.state]++
	}
	if states[sufficiencyInsufficient] > 0 {
		return sufficiencyInsufficient
	}
	if states[sufficiencyUnknown] > 0 {
		return sufficiencyUnknown
	}
	return sufficiencySatisfied
}

func setReason(anchors []anchorVerdict) string {
	if len(anchors) == 0 {
		return "the task names no tracked path and no specific name to check"
	}
	return fmt.Sprintf("%d of %d task anchors carried by the selected lines; not evidence the task is answered",
		len(anchors)-len(missingAnchors(anchors)), len(anchors))
}

func missingAnchors(anchors []anchorVerdict) []string {
	missing := make([]string, 0)
	for _, anchor := range anchors {
		if anchor.state != sufficiencySatisfied {
			missing = append(missing, anchor.anchor)
		}
	}
	return missing
}

// packet lists at most contextSufficiencyListCap anchors and missing names;
// the verdict and anchors_total always count every anchor.
func (verdict sufficiencyVerdict) packet() map[string]any {
	rows := make([]any, 0, min(len(verdict.anchors), contextSufficiencyListCap))
	for _, anchor := range verdict.anchors[:min(len(verdict.anchors), contextSufficiencyListCap)] {
		rows = append(rows, map[string]any{"anchor": anchor.anchor, "kind": anchor.kind, "state": anchor.state, "detail": anchor.detail})
	}
	missing := missingAnchors(verdict.anchors)
	listed := make([]any, 0, min(len(missing), contextSufficiencyListCap))
	for _, name := range missing[:min(len(missing), contextSufficiencyListCap)] {
		listed = append(listed, name)
	}
	return map[string]any{
		"verdict": verdict.verdict, "scope": "task-anchors", "reason": verdict.reason, "anchors_total": len(verdict.anchors),
		"missing_total": len(missing), "anchors": rows, "missing": listed,
	}
}
