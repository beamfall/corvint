// Package mdreport renders values into CEM and OCM review-report Markdown
// safely. Report bodies interpolate repository-derived strings (paths,
// evidence identifiers) into backtick code spans; those strings are only
// byte-validated (wire.ValidatePath rejects bytes below 0x20 but allows
// backticks, "|", and U+2028/U+2029), so a raw interpolation can break out of
// its code span or, via the Unicode line/paragraph separators, out of its
// report line.
package mdreport

import "strings"

// lineSeparators neutralizes U+2028 LINE SEPARATOR and U+2029 PARAGRAPH
// SEPARATOR, which most Markdown renderers treat as line breaks even though
// they are not ASCII control bytes and so pass path validation unchanged.
var lineSeparators = strings.NewReplacer(
	"\r\n", " ",
	"\r", " ",
	"\n", " ",
	" ", " ",
	" ", " ",
)

// CodeSpan renders value as a single CommonMark inline code span that cannot
// be broken out of: the backtick fence is one longer than the longest run of
// consecutive backticks found in value (the CommonMark rule for delimiting a
// code span around arbitrary content), the fence is padded with a space on
// each side when value starts or ends with a backtick, and any embedded
// newline or Unicode line/paragraph separator is replaced with a space so
// the span cannot spill across report lines.
func CodeSpan(value string) string {
	value = lineSeparators.Replace(value)
	longest, run := 0, 0
	for i := 0; i < len(value); i++ {
		if value[i] == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") {
		return fence + " " + value + " " + fence
	}
	return fence + value + fence
}
