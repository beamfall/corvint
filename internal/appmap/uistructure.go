package appmap

import (
	"encoding/json"
	"sort"
	"strings"
)

// ARIA structure digests (docs/specs/observed-ui-evidence-v0.md, OUE-V0-040..044). An action's
// ARIA snapshot YAML is scrubbed to its structure (indentation, role and allowlisted state
// attributes) before it is hashed; accessible names, text content, link targets and other
// property values never reach the digest or the output. A digest difference between two
// revisions is a candidate structural-change signal, never a pass/fail verdict.
const (
	UIStructureDiffSchema = "observed-ui-structure-diff/0"
	SignalCandidate       = "candidate"

	StructureChanged = "structure-changed"
	StructureSame    = "same-structure"

	maxAriaLines = 65536
)

// ariaStateAttrs are the bracket attributes kept, because they describe structure or state and
// carry no free text.
var ariaStateAttrs = map[string]bool{"level": true, "checked": true, "disabled": true, "expanded": true, "pressed": true, "selected": true, "active": true}

// ariaRoles is the closed role vocabulary a scrubbed node keeps (WAI-ARIA 1.2 roles plus the
// Playwright snapshot's `text`, `img` and `image`). A role token outside it is the one free token
// that could otherwise reach the digest, so it is replaced by `#other` and counted dropped
// (OUE-V0-075).
var ariaRoles = func() map[string]bool {
	out := map[string]bool{}
	for _, r := range strings.Fields(`alert alertdialog application article banner blockquote button caption cell
		checkbox code columnheader combobox complementary contentinfo definition deletion dialog directory
		document emphasis feed figure form generic grid gridcell group heading img image insertion link list
		listbox listitem log main mark marquee math menu menubar menuitem menuitemcheckbox menuitemradio meter
		navigation none note option paragraph presentation progressbar radio radiogroup region row rowgroup
		rowheader scrollbar search searchbox separator slider spinbutton status strong subscript superscript
		switch tab table tablist tabpanel term text textbox time timer toolbar tooltip tree treegrid treeitem`) {
		out[r] = true
	}
	return out
}()

// ScrubAriaDigest scrubs one ARIA snapshot YAML and returns the sha256 digest of the scrubbed
// structure, its node count and how many names, texts or property values it dropped. A snapshot
// over maxAriaLines lines returns ok false: a digest of a truncated prefix would read as a
// complete structure.
func ScrubAriaDigest(yaml string) (string, int, int, bool) {
	if strings.Count(yaml, "\n") >= maxAriaLines {
		return "", 0, 0, false
	}
	var lines []string
	nodes, dropped := 0, 0
	for _, raw := range strings.Split(yaml, "\n") {
		raw = strings.TrimRight(raw, " \t\r")
		body := strings.TrimLeft(raw, " ")
		if body == "" {
			continue
		}
		indent := len(raw) - len(body)
		if !strings.HasPrefix(body, "- ") && body != "-" {
			dropped++ // a continuation or block-scalar line: free text.
			continue
		}
		body = strings.TrimSpace(strings.TrimPrefix(body, "-"))
		if strings.HasPrefix(body, "/") {
			dropped++ // a property line such as `/url: ...`.
			continue
		}
		line, d := scrubAriaNode(body)
		dropped += d
		nodes++
		lines = append(lines, strings.Repeat(" ", indent)+"- "+line)
	}
	return digest([]byte(strings.Join(lines, "\n"))), nodes, dropped, true
}

// scrubAriaNode keeps a node's role and allowlisted attributes.
func scrubAriaNode(body string) (string, int) {
	dropped := 0
	n := 0
	for n < len(body) && (body[n] >= 'a' && body[n] <= 'z' || body[n] >= 'A' && body[n] <= 'Z' || body[n] == '-') {
		n++
	}
	role := body[:n]
	// The role token runs to the first separator; a token with any other character (`button123`)
	// is outside the vocabulary as a whole.
	whole := n
	for whole < len(body) && !strings.ContainsRune(" \t:[\"'/", rune(body[whole])) {
		whole++
	}
	switch {
	case whole == 0:
		role = "#text"
	case whole != n || !ariaRoles[role]:
		n = whole
		role = "#other"
		dropped++
	}
	rest := body[n:]
	var attrs []string
	for i := 0; i < len(rest); i++ {
		switch c := rest[i]; c {
		case '"', '/', '\'':
			j := i + 1
			for j < len(rest) && rest[j] != c {
				if rest[j] == '\\' {
					j++
				}
				j++
			}
			dropped++
			i = j
		case '[':
			j := strings.IndexByte(rest[i:], ']')
			if j < 0 {
				dropped++
				i = len(rest)
				continue
			}
			name, value, hasValue := strings.Cut(rest[i+1:i+j], "=")
			switch {
			case ariaStateAttrs[name] && !hasValue:
				attrs = append(attrs, name)
			case ariaStateAttrs[name] && plainAttrValue(value):
				attrs = append(attrs, name+"="+value)
			default:
				dropped++
			}
			i += j
		case ':':
			tail := strings.TrimSpace(rest[i+1:])
			if tail != "" {
				dropped++
			}
			i = len(rest)
		}
	}
	sort.Strings(attrs)
	out := role
	for _, a := range attrs {
		out += " [" + a + "]"
	}
	return out, dropped
}

func plainAttrValue(v string) bool {
	if v == "" || len(v) > 16 {
		return false
	}
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// UIStructureDiff is the observed-ui-structure-diff/0 document.
type UIStructureDiff struct {
	Schema       string                  `json:"schema"`
	Signal       string                  `json:"signal"`
	Authority    string                  `json:"authority"`
	BaseRevision string                  `json:"base_revision"`
	HeadRevision string                  `json:"head_revision"`
	BaseDigest   string                  `json:"base_records_digest"`
	HeadDigest   string                  `json:"head_records_digest"`
	Counts       UIStructureCounts       `json:"counts"`
	Screens      []UIStructureScreenDiff `json:"screens"`
}

// UIStructureCounts is the denominator: every compared screen lands in exactly one state.
type UIStructureCounts struct {
	Screens    int `json:"screens"`
	Changed    int `json:"structure_changed"`
	Same       int `json:"same_structure"`
	Unknown    int `json:"unknown"`
	BaseNoAria int `json:"base_traces_without_aria"`
	HeadNoAria int `json:"head_traces_without_aria"`
}

// UIStructureScreenDiff is one screen's comparison.
type UIStructureScreenDiff struct {
	Screen   string              `json:"screen"`
	State    string              `json:"state"`
	Reason   string              `json:"reason,omitempty"`
	Paths    []string            `json:"paths"`
	BaseOnly []UIStructureDigest `json:"base_only"`
	HeadOnly []UIStructureDigest `json:"head_only"`
	Shared   int                 `json:"shared"`
	// Tests are the tests that saw a base-only or head-only digest of a structure-changed screen
	// (OUE-V0-076); empty for every other state.
	Tests []string `json:"tests"`
}

// DiffUIStructures compares the scrubbed ARIA digests two record documents observed per screen
// (OUE-V0-042). A screen observed on one side only, or not resolved to a screen, reads UNKNOWN.
func DiffUIStructures(base, head *UIRecords) ([]byte, error) {
	doc := UIStructureDiff{Schema: UIStructureDiffSchema, Signal: SignalCandidate, Authority: AuthorityLearned,
		BaseRevision: base.Revision, HeadRevision: head.Revision, BaseDigest: base.Digest, HeadDigest: head.Digest, Screens: []UIStructureScreenDiff{}}
	for _, t := range base.Traces {
		if t.AriaSnapshots == 0 {
			doc.Counts.BaseNoAria++
		}
	}
	for _, t := range head.Traces {
		if t.AriaSnapshots == 0 {
			doc.Counts.HeadNoAria++
		}
	}
	index := func(r *UIRecords) map[string]UIStructure {
		out := map[string]UIStructure{}
		for _, s := range r.Structures {
			out[s.Screen] = s
		}
		return out
	}
	b, h := index(base), index(head)
	ids := map[string]bool{}
	for id := range b {
		ids[id] = true
	}
	for id := range h {
		ids[id] = true
	}
	for _, id := range sortedKeys(ids) {
		bs, inB := b[id]
		hs, inH := h[id]
		d := UIStructureScreenDiff{Screen: id, BaseOnly: []UIStructureDigest{}, HeadOnly: []UIStructureDigest{}, Tests: []string{}}
		set := map[string]bool{}
		for _, p := range append(append([]string{}, bs.Paths...), hs.Paths...) {
			set[p] = true
		}
		d.Paths = sortedKeys(set)
		switch {
		case strings.HasPrefix(id, "unknown:"):
			d.State, d.Reason = StatusUnknown, "screen-unresolved"
		case !inB || !inH:
			d.State, d.Reason = StatusUnknown, "observed-one-side"
		default:
			inBase := map[string]bool{}
			for _, x := range bs.Digests {
				inBase[x.Digest] = true
			}
			inHead := map[string]bool{}
			for _, x := range hs.Digests {
				inHead[x.Digest] = true
				if !inBase[x.Digest] {
					d.HeadOnly = append(d.HeadOnly, x)
				} else {
					d.Shared++
				}
			}
			for _, x := range bs.Digests {
				if !inHead[x.Digest] {
					d.BaseOnly = append(d.BaseOnly, x)
				}
			}
			d.State = StructureSame
			if len(d.BaseOnly)+len(d.HeadOnly) > 0 {
				d.State = StructureChanged
				saw := map[string]bool{}
				for _, x := range append(append([]UIStructureDigest{}, d.BaseOnly...), d.HeadOnly...) {
					for _, k := range x.Tests {
						saw[k] = true
					}
				}
				d.Tests = sortedKeys(saw)
			}
		}
		switch d.State {
		case StructureChanged:
			doc.Counts.Changed++
		case StructureSame:
			doc.Counts.Same++
		default:
			doc.Counts.Unknown++
		}
		doc.Screens = append(doc.Screens, d)
	}
	doc.Counts.Screens = len(doc.Screens)
	out, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
