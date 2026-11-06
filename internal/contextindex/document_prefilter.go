// SPDX-License-Identifier: Apache-2.0

package contextindex

import "unicode/utf8"

// These backend seams let the regression count actual regex executions, not
// elapsed time on a shared host. Production always uses the original regexes.
var (
	findDocumentTitle   = documentTitle.FindStringSubmatch
	findDocumentHeading = documentHeading.FindStringSubmatch
	findDocumentStatus  = documentStatus.FindStringSubmatch
)

// GPK-V0-8001: necessary conditions only. The original regex still owns each
// match, including Unicode whitespace and case folding; these gates avoid
// allocating regex state on the ordinary prose lines that cannot match it.
func matchDocumentTitle(line string, found bool) []string {
	if found || len(line) == 0 || line[0] != '#' {
		return nil
	}
	return findDocumentTitle(line)
}

func matchDocumentHeading(line string) []string {
	if len(line) == 0 || line[0] != '#' {
		return nil
	}
	return findDocumentHeading(line)
}

func isDocumentHeading(line string) bool {
	return len(line) != 0 && line[0] == '#' && documentHeading.MatchString(line)
}

func matchDocumentStatus(line string) []string {
	if !mayHoldDocumentStatus(line) {
		return nil
	}
	return findDocumentStatus(line)
}

// The literal "status" in the case-insensitive regex can also spell s as
// U+017F (long s). Its other letters have only ASCII case-fold equivalents.
// Finding this literal anywhere is necessary, not sufficient, for the anchored
// labelled-field expression to match. Invalid UTF-8 cannot add a match.
func mayHoldDocumentStatus(line string) bool {
	for at, first := range line {
		if first != 's' && first != 'S' && first != 'ſ' {
			continue
		}
		rest, match := line[at:], true
		for _, want := range "status" {
			got, n := utf8.DecodeRuneInString(rest)
			if got != want && got != want-('a'-'A') && !(want == 's' && got == 'ſ') {
				match = false
				break
			}
			rest = rest[n:]
		}
		if match {
			return true
		}
	}
	return false
}
