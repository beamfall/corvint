package mutation

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ValueScreenDetail prefixes the refusal detail of a write that matches the
// adopter value screen (KHN-V0-030). The detail names the field and the
// class, never the value or any part of it.
const ValueScreenDetail = "VALUE_SCREEN_MATCHED"

// MinDenyValueBytes is the shortest declared value the screen compares; a
// shorter line is ignored so a stray token cannot refuse ordinary prose.
const MinDenyValueBytes = 8

// ValueScreen is the in-memory KHN-V0-028 screen: the adopter's declared
// values, read from the values file for one write and never stored, plus
// the opt-in personal-data classes. A nil screen matches nothing.
type ValueScreen struct {
	values  []string
	classes []string
}

// NewValueScreen builds a screen from the values-file bytes (nil when the
// policy names no file) and the declared classes.
func NewValueScreen(denyFile []byte, classes []string) *ValueScreen {
	return &ValueScreen{values: ParseDenyValues(denyFile), classes: append([]string{}, classes...)}
}

// ParseDenyValues reads an adopter values file: one value per line, or an
// env-style KEY=VALUE line (optionally `export`-prefixed and quoted), whose
// value is compared. A line whose KEY=VALUE value would be shorter than
// MinDenyValueBytes, such as a padded base64 value read as KEY "abc" and
// VALUE "=", is compared whole instead. Blank lines, `#` comments and values
// shorter than MinDenyValueBytes are ignored.
func ParseDenyValues(raw []byte) []string {
	var out []string
	for _, line := range bytes.Split(raw, []byte("\n")) {
		s := strings.TrimSpace(strings.TrimSuffix(string(line), "\r"))
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = strings.TrimSpace(strings.TrimPrefix(s, "export "))
		if i := strings.IndexByte(s, '='); i > 0 && envKey.MatchString(s[:i]) {
			if v := unquote(strings.TrimSpace(s[i+1:])); len(v) >= MinDenyValueBytes {
				s = v
			}
		}
		s = unquote(s)
		if len(s) >= MinDenyValueBytes {
			out = append(out, s)
		}
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

var (
	envKey      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	emailShape  = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`)
	phoneShapes = []*regexp.Regexp{
		regexp.MustCompile(`\+[0-9](?:[ .-]?[0-9]){9,14}`),
		regexp.MustCompile(`\(?[0-9]{3}\)?[ .-][0-9]{3}[ .-][0-9]{4}`),
	}
	cardShape = regexp.MustCompile(`[0-9](?:[ -]?[0-9]){12,18}`)
)

// Match returns the class s matches (DENY_VALUE first, then the declared
// classes in order), or "".
func (v *ValueScreen) Match(s string) string {
	if v == nil || s == "" {
		return ""
	}
	for _, d := range v.values {
		if strings.Contains(s, d) {
			return intent.ScreenClassDenyValue
		}
	}
	for _, c := range v.classes {
		switch c {
		case intent.ScreenClassEmail:
			if emailShape.MatchString(s) {
				return c
			}
		case intent.ScreenClassPhone:
			for _, re := range phoneShapes {
				if digitBounded(s, re, nil) {
					return c
				}
			}
		case intent.ScreenClassCard:
			if digitBounded(s, cardShape, luhn) {
				return c
			}
		}
	}
	return ""
}

// digitBounded reports a match of re in s that no digit adjoins and that
// passes check, when given.
func digitBounded(s string, re *regexp.Regexp, check func(string) bool) bool {
	for _, m := range re.FindAllStringIndex(s, -1) {
		if m[0] > 0 && isDigit(s[m[0]-1]) || m[1] < len(s) && isDigit(s[m[1]]) {
			continue
		}
		if check == nil || check(s[m[0]:m[1]]) {
			return true
		}
	}
	return false
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// luhn reports whether the digits of s pass the Luhn check.
func luhn(s string) bool {
	sum, double := 0, false
	for i := len(s) - 1; i >= 0; i-- {
		if !isDigit(s[i]) {
			continue
		}
		d := int(s[i] - '0')
		if double {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// screenValues refuses the first named field the screen matches
// (KHN-V0-030). The detail names the field and class only.
func (v *ValueScreen) screenValues(fields []screenField) *refusal {
	if v == nil {
		return nil
	}
	for _, f := range fields {
		if class := v.Match(f.text); class != "" {
			return refuse(OutcomeValidationFailed, wire.CodeSecretDetected, "%s: %s matches the declared screen class %s; remove it and retry with a new request ID", ValueScreenDetail, f.name, class)
		}
	}
	return nil
}

// screenField is one named stored text of a write.
type screenField struct{ name, text string }

// noteScreen refuses NOTE_SET text that matches the shared credential
// screen or the declared value screen (KHN-V0-031). It is consulted only
// while the policy declares knowHow.screen.
func (v *ValueScreen) noteScreen(text string) *refusal {
	if v == nil {
		return nil
	}
	if matchesSecretScreen(text) {
		return refuse(OutcomeValidationFailed, wire.CodeSecretDetected, "%s: /payload/text matches the secret screen; remove the secret and retry with a new request ID", ValueScreenDetail)
	}
	return v.screenValues([]screenField{{"/payload/text", text}})
}
