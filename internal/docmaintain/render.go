package docmaintain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/doccompiler"
)

const insertionPoint = "<!-- corvint:docmaintain:insertion-point -->"
const beginMarkerPrefix = "<!-- corvint:docmaintain begin "
const endMarker = "<!-- corvint:docmaintain end -->"

// beginLine is one complete begin marker line. Marker discovery is line based
// and fence aware (scanPage); a marker-looking line that does not match it
// exactly is malformed.
var beginLine = regexp.MustCompile(
	`^<!-- corvint:docmaintain begin source="([^"]*)" package="([^"]*)" commit="[^"]*" source_digest="([0-9a-f]*)" -->$`,
)

// sourceDigest identifies exactly the cited source content of a draft — each
// entry's path, blob and content sha256, in the draft's own deterministic
// order — independent of the commit/tree stamps the rendered Markdown
// embeds. A repository-wide commit that touches unrelated files changes
// every draft's commit/tree stamp (the tree hash covers the whole
// repository) without changing what any given selector actually cites; only
// this digest, not a hash of the rendered Markdown, can tell eligible
// changes apart from unrelated churn.
//
// A paged draft (SDD-V0-632) carries only one page of entries, so the digest
// also covers each TRUNCATED limitation: an entry added or removed off this
// page changes the page's span and counts, and so the block. An unpaged draft
// has no such limitation and keeps its earlier digest.
func sourceDigest(draft doccompiler.SourceDraft) string {
	hasher := sha256.New()
	for _, entry := range draft.Entries {
		fmt.Fprintf(hasher, "%s|%s|%s;", entry.Path, entry.Blob, entry.SHA256)
	}
	for _, limitation := range draft.Limitations {
		if strings.HasPrefix(limitation, "TRUNCATED:") {
			fmt.Fprintf(hasher, "limitation|%s;", limitation)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// validSelector reports whether selector's Source and Package can be safely
// embedded in a begin marker. %q escapes quotes and control bytes but leaves
// a literal "-->" untouched, which would close the marker's own HTML comment
// early and turn the rest of the page into live content; any value %q
// changes (a `"`, a `\`, or a non-printable rune) is also unsafe because
// findBlock compares the raw selector with the escaped marker value, so it
// could never re-find the block and would insert it again. Such a value is
// refused rather than mangled.
func validSelector(selector Selector) bool {
	return markerValueExact(selector.Source) && markerValueExact(selector.Package)
}

func markerValueExact(value string) bool {
	return strconv.Quote(value) == `"`+value+`"` && !strings.Contains(value, "-->")
}

// renderBlock renders one generated block: a begin marker recording the
// selector identity, the commit the draft was built at (informational only),
// and the draft's source digest; the verbatim draft Markdown; and an end
// marker. The digest is what a later session compares against to decide
// eligibility — an unchanged source digest never rewrites the block.
func renderBlock(selector Selector, commit, tree string, digest string, markdown []byte) []byte {
	body := bytes.TrimRight(markdown, "\n")
	return []byte(fmt.Sprintf(
		"<!-- corvint:docmaintain begin source=%q package=%q commit=%q source_digest=%q -->\n%s\n<!-- corvint:docmaintain end -->\n",
		selector.Source, selector.Package, commit+"|"+tree, digest, body,
	))
}

// pageBlock is one generated block found by scanPage. start..end spans the
// begin marker line through the end marker line and its newline; bodyStart..
// bodyEnd is the managed body without its final newline (empty for a seeded
// begin/end pair).
type pageBlock struct {
	source, pkg, digest string
	start, end          int
	bodyStart, bodyEnd  int
}

// pageLayout is the structure of a page as docs maintain sees it: its
// generated blocks and the byte offset just after the first insertion-point
// line (-1 when there is none).
type pageLayout struct {
	blocks    []pageBlock
	insertion int
}

// scanPage finds generated blocks and the insertion point line by line,
// outside CommonMark fenced code (``` or ~~~, at most three spaces of
// indentation, closed by the same character at least as long with nothing
// after it), so documentation that shows marker examples in a fence is
// ordinary prose. A block body is opaque: fences inside it are not tracked
// and only its end marker closes it. It returns a refusal code instead of a
// partial layout when any marker line outside a fence is not part of exactly
// one well-formed block ("malformed-block": a begin without its end, an end
// without a begin, a nested begin or a marker line that does not parse), or
// when two blocks name the same selector ("duplicate-block"). Replacing one
// of two blocks would leave the other stale while later sessions report
// "unchanged", so the whole page is validated before anything is spliced.
func scanPage(page []byte) (pageLayout, string) {
	layout := pageLayout{insertion: -1}
	seen := map[[2]string]bool{}
	var fenceChar byte
	fenceLen := 0
	var open *pageBlock
	for offset := 0; offset < len(page); {
		lineEnd := bytes.IndexByte(page[offset:], '\n')
		next := len(page)
		if lineEnd < 0 {
			lineEnd = len(page)
		} else {
			lineEnd += offset
			next = lineEnd + 1
		}
		line := string(page[offset:lineEnd])
		switch {
		case open != nil:
			if markerKind(line) == "end" {
				if line != endMarker {
					return pageLayout{}, "malformed-block"
				}
				open.bodyEnd = offset - 1
				if open.bodyEnd < open.bodyStart {
					open.bodyEnd = open.bodyStart
				}
				open.end = next
				layout.blocks = append(layout.blocks, *open)
				open = nil
			} else if markerKind(line) == "begin" {
				return pageLayout{}, "malformed-block"
			}
		case fenceLen > 0:
			if closesFence(line, fenceChar, fenceLen) {
				fenceLen = 0
			}
		default:
			if char, length, ok := opensFence(line); ok {
				fenceChar, fenceLen = char, length
				break
			}
			switch markerKind(line) {
			case "begin":
				match := beginLine.FindStringSubmatch(line)
				if match == nil {
					return pageLayout{}, "malformed-block"
				}
				key := [2]string{match[1], match[2]}
				if seen[key] {
					return pageLayout{}, "duplicate-block"
				}
				seen[key] = true
				open = &pageBlock{source: match[1], pkg: match[2], digest: match[3], start: offset, bodyStart: next}
			case "end":
				return pageLayout{}, "malformed-block"
			default:
				if layout.insertion < 0 && strings.TrimRight(line, " \t") == insertionPoint {
					layout.insertion = lineEnd
				}
			}
		}
		offset = next
	}
	if open != nil {
		return pageLayout{}, "malformed-block"
	}
	return layout, ""
}

// markerKind classifies a line that looks like a marker after at most three
// spaces of indentation (a CommonMark HTML block); deeper indentation is
// indented code, not a marker.
func markerKind(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return ""
	}
	switch {
	case strings.HasPrefix(trimmed, strings.TrimRight(beginMarkerPrefix, " ")):
		return "begin"
	case strings.HasPrefix(trimmed, strings.TrimSuffix(endMarker, " -->")):
		return "end"
	}
	return ""
}

func opensFence(line string) (byte, int, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, 0, false
	}
	char := trimmed[0]
	length := len(trimmed) - len(strings.TrimLeft(trimmed, string(char)))
	if length < 3 || (char == '`' && strings.ContainsRune(trimmed[length:], '`')) {
		return 0, 0, false
	}
	return char, length, true
}

func closesFence(line string, char byte, length int) bool {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return false
	}
	rest := strings.TrimLeft(trimmed, string(char))
	return len(trimmed)-len(rest) >= length && strings.Trim(rest, " \t") == ""
}

// findBlock locates the existing generated block for selector inside page, if
// any, returning its recorded source digest and its exact byte span (start
// inclusive of the begin marker, end exclusive, i.e. the byte right after the
// block's trailing newline) so the caller can splice a replacement in without
// disturbing any other byte of the page. A page scanPage refuses has no
// blocks; callers check scanPage first.
func findBlock(page []byte, selector Selector) (body []byte, digest string, start, end int, found bool) {
	layout, _ := scanPage(page)
	for _, block := range layout.blocks {
		if block.source == selector.Source && block.pkg == selector.Package {
			if block.bodyEnd == block.bodyStart {
				return nil, block.digest, block.start, block.end, true
			}
			return page[block.bodyStart:block.bodyEnd], block.digest, block.start, block.end, true
		}
	}
	return nil, "", 0, 0, false
}

// containsMarker reports whether body carries either literal marker
// substring. blockPattern anchors markers at line start with a non-greedy
// body match; a rendered draft or a page's existing managed body that
// happens to contain either marker text (anywhere, not only at line start)
// could otherwise truncate the block span or orphan the real tail, so this
// is checked and refused before any splice.
func containsMarker(body []byte) bool {
	return bytes.Contains(body, []byte(beginMarkerPrefix)) || bytes.Contains(body, []byte(endMarker))
}

// insertBlock appends block after the page's insertion-point line (right
// after any generated blocks already anchored there), or at the end of the
// page if no insertion point exists outside fenced code. It never edits bytes
// before that point. page must have passed scanPage.
func insertBlock(page, block []byte) []byte {
	layout, _ := scanPage(page)
	if layout.insertion < 0 {
		result := make([]byte, 0, len(page)+len(block)+1)
		result = append(result, page...)
		if len(page) > 0 && page[len(page)-1] != '\n' {
			result = append(result, '\n')
		}
		result = append(result, block...)
		return result
	}
	insertAt := layout.insertion
	for insertAt < len(page) && page[insertAt] == '\n' {
		insertAt++
	}
	// Skip past any generated blocks already anchored here so new blocks are
	// appended after the last one, keeping selector order stable.
	for _, existing := range layout.blocks {
		if existing.start == insertAt {
			insertAt = existing.end
		}
	}
	result := make([]byte, 0, len(page)+len(block)+2)
	result = append(result, page[:insertAt]...)
	result = append(result, block...)
	result = append(result, page[insertAt:]...)
	return result
}
