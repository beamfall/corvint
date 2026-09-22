package wire

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Canonical identity encoding per interop/cem-0.1/ALGORITHMS.md: UTF-8, no BOM
// or trailing LF, members sorted by name, no insignificant spaces, shortest
// unsigned decimal integers, and the exact string escape profile.

// CanonicalString encodes one JSON string with the frozen escape profile.
func CanonicalString(text string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 {
				out.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

func canonicalOptionalString(text *string) string {
	if text == nil {
		return "null"
	}
	return CanonicalString(*text)
}

// EvidenceIdentity derives the frozen evidence ID for one evidence record.
func EvidenceIdentity(blobOid, path string, span Span, spanSha256 string) string {
	canonical := "{" +
		`"blobOid":` + CanonicalString(blobOid) + "," +
		`"path":` + CanonicalString(path) + "," +
		`"span":{"end":` + strconv.FormatInt(span.End, 10) +
		`,"start":` + strconv.FormatInt(span.Start, 10) + "}," +
		`"spanSha256":` + CanonicalString(spanSha256) + "}"
	digest := sha256.Sum256([]byte(canonical))
	return EvidencePrefix + hex.EncodeToString(digest[:])
}

// HunkIdentity derives the frozen hunk ID. oldPath and newPath are nil for the
// created-from and deleted-to sides respectively.
func HunkIdentity(contentSha256 string, oldPath, newPath *string, oldRange, newRange Range) string {
	canonical := "{" +
		`"contentSha256":` + CanonicalString(contentSha256) + "," +
		`"newPath":` + canonicalOptionalString(newPath) + "," +
		`"newRange":{"count":` + strconv.FormatInt(newRange.Count, 10) +
		`,"start":` + strconv.FormatInt(newRange.Start, 10) + "}," +
		`"oldPath":` + canonicalOptionalString(oldPath) + "," +
		`"oldRange":{"count":` + strconv.FormatInt(oldRange.Count, 10) +
		`,"start":` + strconv.FormatInt(oldRange.Start, 10) + "}}"
	digest := sha256.Sum256([]byte(canonical))
	return HunkPrefix + hex.EncodeToString(digest[:])
}
