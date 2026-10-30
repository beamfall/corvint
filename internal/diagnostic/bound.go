package diagnostic

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// MaxValueBytes and MaxEvidence are DRC-V0-008's per-value and per-diagnostic bounds.
const (
	MaxValueBytes = 512
	MaxEvidence   = 16
	// EmptySubject discloses a refused operand spelled as the empty string, since an emitted
	// subject value is never empty (DRC-V0-001).
	EmptySubject = "[empty]"
)

// Bounded is the emitted form of refusal (DRC-V0-008): every subject and evidence string is
// secret-screened, control characters are escaped visibly, and the result is cut to
// MaxValueBytes with the cut disclosed; evidence beyond MaxEvidence pairs keeps the first
// MaxEvidence-1 and discloses the dropped count in a final evidence-truncated pair.
func (refusal Refusal) Bounded() Refusal {
	emitted := refusal
	emitted.Subject = Subject{Kind: refusal.Subject.Kind, Value: boundValue(refusal.Subject.Value)}
	if emitted.Subject.Value == "" {
		emitted.Subject.Value = EmptySubject
	}
	emitted.Evidence = boundEvidence(refusal.Evidence)
	emitted.SupportedFixes = nonNil(append([]string(nil), refusal.SupportedFixes...))
	return emitted
}

func boundEvidence(evidence []Evidence) []Evidence {
	kept := evidence
	if len(evidence) > MaxEvidence {
		kept = evidence[:MaxEvidence-1]
	}
	bounded := make([]Evidence, 0, len(kept)+1)
	for _, pair := range kept {
		bounded = append(bounded, Evidence{Name: boundValue(pair.Name), Value: boundValue(pair.Value)})
	}
	if dropped := len(evidence) - len(kept); dropped > 0 {
		bounded = append(bounded, Evidence{Name: "evidence-truncated", Value: strconv.Itoa(dropped)})
	}
	return bounded
}

// boundValue screens before cutting, so a cut can never split a secret the screen matches.
func boundValue(value string) string {
	screened, _ := secretscreen.Screen(strings.ToValidUTF8(value, string(utf8.RuneError)))
	visible := escapeControls(screened)
	if len(visible) <= MaxValueBytes {
		return visible
	}
	cut := MaxValueBytes - len(truncation(len(visible)))
	for !utf8.RuneStart(visible[cut]) {
		cut--
	}
	cut = escapeStart(visible, cut)
	return visible[:cut] + truncation(len(visible)-cut)
}

// escapeStart moves a cut that falls inside a `\uXXXX` control escape back to that escape's
// backslash, so the kept prefix never ends in a partial escape. An escape's hex digits hold no
// backslash, so the last `\u` starting within an escape's width before the cut is the only
// candidate; one that is literal input rather than an escape only cuts a few bytes earlier.
func escapeStart(visible string, cut int) int {
	low := max(0, cut-len(`\u0000`)+1)
	offset := strings.LastIndex(visible[low:cut+1], `\u`)
	if offset < 0 {
		return cut
	}
	return low + offset
}

func truncation(dropped int) string {
	return fmt.Sprintf("...[truncated %d bytes]", dropped)
}

func escapeControls(value string) string {
	if !strings.ContainsFunc(value, unicode.IsControl) {
		return value
	}
	var escaped strings.Builder
	for _, character := range value {
		if unicode.IsControl(character) {
			fmt.Fprintf(&escaped, `\u%04x`, character)
			continue
		}
		escaped.WriteRune(character)
	}
	return escaped.String()
}
