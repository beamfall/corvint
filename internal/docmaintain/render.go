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

var blockPattern = regexp.MustCompile(
	`(?ms)^<!-- corvint:docmaintain begin source="([^"]*)" package="([^"]*)" commit="[^"]*" source_digest="([0-9a-f]*)" -->\n(.*?)\n^<!-- corvint:docmaintain end -->\n?`,
)

// sourceDigest identifies exactly the cited source content of a draft — each
// entry's path, blob and content sha256, in the draft's own deterministic
// order — independent of the commit/tree stamps the rendered Markdown
// embeds. A repository-wide commit that touches unrelated files changes
// every draft's commit/tree stamp (the tree hash covers the whole
// repository) without changing what any given selector actually cites; only
// this digest, not a hash of the rendered Markdown, can tell eligible
// changes apart from unrelated churn.
func sourceDigest(entries []doccompiler.DraftEntry) string {
	hasher := sha256.New()
	for _, entry := range entries {
		fmt.Fprintf(hasher, "%s|%s|%s;", entry.Path, entry.Blob, entry.SHA256)
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

// findBlock locates the existing generated block for selector inside page, if
// any, returning its recorded markdown_sha256 and its exact byte span
// (start inclusive of the begin marker, end exclusive, i.e. the byte right
// after the block's trailing newline) so the caller can splice a replacement
// in without disturbing any other byte of the page.
func findBlock(page []byte, selector Selector) (body []byte, digest string, start, end int, found bool) {
	for _, match := range blockPattern.FindAllSubmatchIndex(page, -1) {
		source := string(page[match[2]:match[3]])
		pkg := string(page[match[4]:match[5]])
		if source != selector.Source || pkg != selector.Package {
			continue
		}
		recorded := string(page[match[6]:match[7]])
		return page[match[8]:match[9]], recorded, match[0], match[1], true
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

// insertBlock appends block after the page's insertion-point marker (right
// after any generated blocks already anchored there), or at the end of the
// page if no insertion point exists. It never edits bytes before that point.
func insertBlock(page, block []byte) []byte {
	anchor := []byte(insertionPoint)
	index := bytes.Index(page, anchor)
	if index < 0 {
		result := make([]byte, 0, len(page)+len(block)+1)
		result = append(result, page...)
		if len(page) > 0 && page[len(page)-1] != '\n' {
			result = append(result, '\n')
		}
		result = append(result, block...)
		return result
	}
	insertAt := index + len(anchor)
	for insertAt < len(page) && page[insertAt] == '\n' {
		insertAt++
	}
	// Skip past any generated blocks already anchored here so new blocks are
	// appended after the last one, keeping selector order stable.
	for {
		match := blockPattern.FindIndex(page[insertAt:])
		if match == nil || match[0] != 0 {
			break
		}
		insertAt += match[1]
	}
	result := make([]byte, 0, len(page)+len(block)+2)
	result = append(result, page[:insertAt]...)
	result = append(result, block...)
	result = append(result, page[insertAt:]...)
	return result
}
