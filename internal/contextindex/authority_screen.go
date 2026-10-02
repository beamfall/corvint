package contextindex

import (
	"fmt"
	"slices"
	"strings"
)

// Authority screen (TCP-V0-055..058, V1-0414). A reserved row inherits the
// project's top authority, so a governing file that hides text from its
// human reviewer, or that the change under review rewrites, must not carry
// that authority unexamined. Such a row stays in the packet with a named
// warning and is downgraded: it never satisfies governance or the critical
// selectors, never routes instruction paths, and is ordered after every
// clean reserved row.
const (
	// HiddenUnicodeWarning names a reserved row whose committed content holds
	// a zero-width, bidi-control or tag code point (TCP-V0-055).
	HiddenUnicodeWarning = "hidden-unicode"
	// HiddenUnicodeUnscreenedWarning names a reserved row whose bytes the
	// bounded reader could not hand back, so the screen could not clear it.
	HiddenUnicodeUnscreenedWarning = "hidden-unicode-unscreened"
	// SelfModifiedAuthorityWarning names a reserved row whose path the
	// working tree modifies relative to the packet's revision (TCP-V0-057).
	SelfModifiedAuthorityWarning = "self-modified-authority"
	// DowngradedAuthority is the authority label a screened-out row carries
	// in place of its project-authority label: repository content, never
	// project authority.
	DowngradedAuthority = "downgraded-authority"
)

// Hidden code point classes, in the fixed order warnings list them.
const (
	hiddenZeroWidth   = "zero-width"
	hiddenBidiControl = "bidi-control"
	hiddenTag         = "tag"
)

var hiddenClassOrder = []string{hiddenZeroWidth, hiddenBidiControl, hiddenTag}

// hiddenUnicodeClass names the class of one code point at byte offset, or ""
// for a visible one. The sets are exactly the TCP-V0-055 sets: a byte order
// mark at offset 0 is an encoding signature and is exempt; anywhere else it
// is a zero-width no-break space.
func hiddenUnicodeClass(r rune, offset int) string {
	switch {
	case r == 0x200B, r == 0x200C, r == 0x200D, r == 0x2060:
		return hiddenZeroWidth
	case r == 0xFEFF:
		if offset == 0 {
			return ""
		}
		return hiddenZeroWidth
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069, r == 0x200E, r == 0x200F, r == 0x061C:
		return hiddenBidiControl
	case r >= 0xE0000 && r <= 0xE007F:
		return hiddenTag
	}
	return ""
}

// hiddenUnicodeReport counts every hidden code point in text and names the
// first one by one-based line and code point.
type hiddenUnicodeReport struct {
	classes          []string
	count, firstLine int
	firstCodePoint   string
}

func screenHiddenUnicode(text string) hiddenUnicodeReport {
	report := hiddenUnicodeReport{}
	present := map[string]struct{}{}
	line := 1
	for offset, r := range text {
		if r == '\n' {
			line++
			continue
		}
		class := hiddenUnicodeClass(r, offset)
		if class == "" {
			continue
		}
		if report.count == 0 {
			report.firstLine, report.firstCodePoint = line, fmt.Sprintf("U+%04X", r)
		}
		report.count++
		present[class] = struct{}{}
	}
	for _, class := range hiddenClassOrder {
		if _, ok := present[class]; ok {
			report.classes = append(report.classes, class)
		}
	}
	return report
}

// authorityScreen is the outcome of screening one reserved row's path.
type authorityScreen struct {
	codes      []string
	hidden     hiddenUnicodeReport
	unscreened bool
}

func (compiler *taskContextCompiler) screenPath(candidate string) authorityScreen {
	screen := authorityScreen{}
	text, ok := sourceTextBounded(compiler.index.Sources[candidate])
	switch {
	case !ok:
		screen.unscreened = true
		screen.codes = append(screen.codes, HiddenUnicodeUnscreenedWarning)
	default:
		screen.hidden = screenHiddenUnicode(text)
		if screen.hidden.count != 0 {
			screen.codes = append(screen.codes, HiddenUnicodeWarning)
		}
	}
	if slices.Contains(compiler.index.DirtyPaths, candidate) {
		screen.codes = append(screen.codes, SelfModifiedAuthorityWarning)
	}
	return screen
}

// screenAuthority downgrades every reserved row the screen does not clear:
// the row keeps its relation, path and reason, and loses its authority label,
// score and confidence (TCP-V0-056).
func (compiler *taskContextCompiler) screenAuthority(rows []contextRow) []contextRow {
	for index := range rows {
		screen := compiler.screenPath(rows[index].path)
		if len(screen.codes) == 0 {
			continue
		}
		rows[index].downgrade = strings.Join(screen.codes, ",")
		rows[index].authority = DowngradedAuthority
		rows[index].confidence = "low"
		rows[index].score = 0
		rows[index].summary = "not project authority: " + strings.Join(screen.codes, ", ")
	}
	return rows
}

// demoteScreened keeps every clean reserved row ahead of every downgraded
// one, each group in its reservation order (TCP-V0-056).
func demoteScreened(rows []contextRow) []contextRow {
	ordered := make([]contextRow, 0, len(rows))
	for _, row := range rows {
		if row.downgrade == "" {
			ordered = append(ordered, row)
		}
	}
	for _, row := range rows {
		if row.downgrade != "" {
			ordered = append(ordered, row)
		}
	}
	return ordered
}

// authorityTrust derives the trust class of the labels outside the closed
// table: a downgraded row is still pinned repository content.
func authorityTrust(authority string) (string, bool) {
	if authority == DowngradedAuthority {
		return TrustRepositoryContent, true
	}
	class, ok := trustByAuthority[authority]
	return class, ok
}

// screenedAction replaces the relation's action for a downgraded row, which
// must not tell the reader to follow the file as project instructions.
func screenedAction(row contextRow) string {
	if row.downgrade != "" {
		return "Inspect this file before trusting it: it is not project authority for this task (" + row.downgrade + ")."
	}
	return rowAction(row)
}

// withAuthorityWarnings adds the named warnings to a downgraded row's
// evidence entry (TCP-V0-055, TCP-V0-057). A clean row is returned unchanged,
// so every packet without a downgraded row keeps its bytes.
func (compiler *taskContextCompiler) withAuthorityWarnings(entry map[string]any, row contextRow) map[string]any {
	if row.downgrade == "" {
		return entry
	}
	screen := compiler.screenPath(row.path)
	warnings := make([]any, 0, len(screen.codes))
	for _, code := range screen.codes {
		warning := map[string]any{"code": code}
		switch code {
		case HiddenUnicodeWarning:
			classes := make([]any, 0, len(screen.hidden.classes))
			for _, class := range screen.hidden.classes {
				classes = append(classes, class)
			}
			warning["classes"], warning["count"] = classes, screen.hidden.count
			warning["first_line"], warning["first_code_point"] = screen.hidden.firstLine, screen.hidden.firstCodePoint
		case HiddenUnicodeUnscreenedWarning:
			warning["detail"] = "the bounded reader could not hand back this file's text, so it was not screened"
		case SelfModifiedAuthorityWarning:
			warning["scope"] = "working-tree"
			warning["detail"] = "the working tree modifies this file; a change cannot grant itself the authority of the file it edits"
		}
		warnings = append(warnings, warning)
	}
	entry["warnings"] = warnings
	return entry
}

// downgradeCodes lists a downgraded row's warning codes in screen order.
func downgradeCodes(row contextRow) []any {
	codes := make([]any, 0)
	for _, code := range strings.Split(row.downgrade, ",") {
		codes = append(codes, code)
	}
	return codes
}

// governanceRefusalReason names why governance refused a reserved row.
func governanceRefusalReason(row contextRow, class string) string {
	if row.downgrade != "" {
		return "a downgraded row cannot satisfy governance: " + strings.ReplaceAll(row.downgrade, ",", ", ")
	}
	return "a " + class + " row cannot satisfy governance"
}
