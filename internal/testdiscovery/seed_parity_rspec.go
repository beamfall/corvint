package testdiscovery

import (
	"regexp"
)

var (
	rspecFixturesLine = regexp.MustCompile(`^fixtures\b`)
	RSpecSeedCall     = regexp.MustCompile(`(?:\bFactoryBot\.)?\b(create_list|build_list|create|build)\b`)
	RubyWord          = regexp.MustCompile(`[A-Za-z_]\w*[!?]?`)
	RSpecAssertLine   = regexp.MustCompile(`\b(?:expect|is_expected|should|assert\w*|refute\w*)\b`)
	RSpecSubjectUse   = regexp.MustCompile(`\bis_expected\b|(?:^|[\s{])should\b`)
	RubyAssignTail    = regexp.MustCompile(`(?:^|[\s;{(])@?([A-Za-z_]\w*)\s*=\s*$`)
	RubySeedSymbol    = regexp.MustCompile(`^:([A-Za-z_]\w*[?!]?)$`)
	RubyQuotedName    = regexp.MustCompile(`^(?:"([A-Za-z_]\w*)"|'([A-Za-z_]\w*)')$`)
	RubyKeyValue      = regexp.MustCompile(`^([A-Za-z_]\w*[?!]?):(\s+\S.*|[^:\s].*)$`)
	RubyRocket        = regexp.MustCompile(`^(?::([A-Za-z_]\w*[?!]?)|"([A-Za-z_]\w*)"|'([A-Za-z_]\w*)')\s*=>\s*(\S.*)$`)
	SeedNumber        = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	// RubyConditional matches a line whose calls may not all run.
	RubyConditional = regexp.MustCompile(`\b(?:if|unless|while|until|rescue|and|or|then|else|elsif|when|case|return|next|break)\b|&&|\|\||\s\?\s`)
	YAMLKey         = regexp.MustCompile(`^([A-Za-z_][\w.-]*|"[^"]*"|'[^']*'):(?:\s+(.*))?$`)
)

// RSpecLet is one let, let!, subject or subject! declaration; name is "subject" for an anonymous
// subject.
type RSpecLet struct {
	Name string
	Lazy bool
	Body []RubyLine
}

// MaskRubyStrings blanks the contents of quoted strings, keeping every byte offset.
func MaskRubyStrings(text string) string {
	b := []byte(text)
	quote := byte(0)
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case quote != 0 && c == '\\':
			b[i] = ' '
			if i+1 < len(b) {
				b[i+1] = ' '
				i++
			}
		case quote != 0 && c == quote:
			quote = 0
		case quote != 0:
			b[i] = ' '
		case c == '\'' || c == '"':
			quote = c
		}
	}
	return string(b)
}
