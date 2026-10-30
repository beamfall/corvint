package main

import (
	"bytes"
	"fmt"
	"sort"
)

// nonRegularProfile names the additive cem-non-regular/1 profile
// (CAX-V0-020). It is opt-in: without it a non-regular changed path refuses
// unsupported-tree-mode exactly as cem/1.0 requires.
const nonRegularProfile = "cem-non-regular/1"

// nonRegularEntry is one changed path the profile leaves out of the patch; an
// absent side is "absent".
type nonRegularEntry struct {
	Path       string `json:"path"`
	BaseMode   string `json:"baseMode"`
	TargetMode string `json:"targetMode"`
	// Disposition is always "unknown": the entry is never followed or mapped.
	Disposition string `json:"disposition"`
}

func nonRegularSide(entry *treeEntry) string {
	if entry == nil {
		return "absent"
	}
	return entry.mode
}

// filterNonRegular removes from the proved canonical patch every section of a
// changed path with a non-regular side, and that path from the inventory, so
// the remaining bytes are the canonical patch of the regular paths alone. The
// excluded entry is never read as content or followed. Sections are cut
// before each "diff --git " line, which inside a proved section only a
// section header can begin.
func filterNonRegular(canon *stableCanonical) []nonRegularEntry {
	headers := map[string]bool{}
	var excluded []nonRegularEntry
	for path, change := range canon.changed {
		if (change.a == nil || regularFile(change.a)) && (change.b == nil || regularFile(change.b)) {
			continue
		}
		headers["diff --git "+gitQuoteLabel("a/", path)+" "+gitQuoteLabel("b/", path)+"\n"] = true
		excluded = append(excluded, nonRegularEntry{Path: path, BaseMode: nonRegularSide(change.a), TargetMode: nonRegularSide(change.b), Disposition: "unknown"})
		delete(canon.changed, path)
	}
	sort.Slice(excluded, func(i, j int) bool { return excluded[i].Path < excluded[j].Path })
	if len(excluded) == 0 {
		return excluded
	}
	var kept bytes.Buffer
	for _, section := range splitGitSections(canon.patch) {
		line, _, _ := bytes.Cut(section, []byte("\n"))
		if !headers[string(line)+"\n"] {
			kept.Write(section)
		}
	}
	canon.patch = kept.Bytes()
	return excluded
}

func splitGitSections(patch []byte) [][]byte {
	var sections [][]byte
	start := -1
	for offset := 0; offset < len(patch); {
		next := len(patch)
		if end := bytes.IndexByte(patch[offset:], '\n'); end >= 0 {
			next = offset + end + 1
		}
		if bytes.HasPrefix(patch[offset:], []byte("diff --git ")) {
			if start >= 0 {
				sections = append(sections, patch[start:offset])
			}
			start = offset
		}
		offset = next
	}
	if start >= 0 {
		sections = append(sections, patch[start:])
	}
	return sections
}

// gitQuoteLabel renders a diff header label as Git does under
// core.quotePath=false: quoted, with C escapes, only when it holds a control
// byte, a double quote or a backslash.
func gitQuoteLabel(prefix, path string) string {
	label := prefix + path
	quoted := false
	var out bytes.Buffer
	out.WriteByte('"')
	for _, c := range []byte(label) {
		switch c {
		case '\a':
			out.WriteString(`\a`)
		case '\b':
			out.WriteString(`\b`)
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\v':
			out.WriteString(`\v`)
		case '\f':
			out.WriteString(`\f`)
		case '\r':
			out.WriteString(`\r`)
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		default:
			if c < 0x20 || c == 0x7f {
				out.WriteString(fmt.Sprintf("\\%03o", c))
			} else {
				out.WriteByte(c)
			}
		}
		quoted = quoted || c < 0x20 || c == 0x7f || c == '"' || c == '\\'
	}
	out.WriteByte('"')
	if !quoted {
		return label
	}
	return out.String()
}
