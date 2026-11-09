package bridge

import "strings"

// PathRule is one repository-relative path rule a tool schema advertises
// (MCPV0-954). Every rule refuses a leading slash, an empty, `.` or `..`
// segment and a backslash; the options add to that.
type PathRule struct {
	// Excluded is the character-class body of the other refused characters,
	// such as `\x00-\x1f\x7f`.
	Excluded string
	// GitSegment refuses a `.git` segment: "" never, "lower" only `.git`,
	// "fold" every case of it.
	GitSegment string
	// TrailingSlash admits one trailing slash.
	TrailingSlash bool
	// Suffixes, when set, are the extensions the last segment must end with,
	// after at least one other character of the path.
	Suffixes []string
}

// Pattern is the rule as one regular expression without lookaround, so ECMA
// and RE2 validators read it the same way. A path is segments joined by single
// slashes; a segment is any run of allowed characters other than `.` and `..`
// (and the refused `.git` spellings), spelled by its first characters.
func (rule PathRule) Pattern() string {
	char := `[^/\\` + rule.Excluded + `]`
	not := func(extra string) string { return `[^/\\` + extra + rule.Excluded + `]` }
	dotted := `\.` + not(".") + char + `*`
	switch rule.GitSegment {
	case "lower":
		dotted = `\.(?:` + not(".g") + char + `*|g(?:` + not("i") + char + `*)?|gi(?:` + not("t") + char + `*)?|git` + char + `+)`
	case "fold":
		dotted = `\.(?:` + not(".Gg") + char + `*|[Gg](?:` + not("Ii") + char + `*)?|[Gg][Ii](?:` + not("Tt") + char + `*)?|[Gg][Ii][Tt]` + char + `+)`
	}
	segment := `(?:` + not(".") + char + `*|` + dotted + `|\.\.` + char + `+)`
	if len(rule.Suffixes) != 0 {
		return `^(?:(?:` + segment + `/)+` + char + `*|` + char + `+)[.](?:` + strings.Join(rule.Suffixes, "|") + `)$`
	}
	trailing := ""
	if rule.TrailingSlash {
		trailing = "/?"
	}
	return `^` + segment + `(?:/` + segment + `)*` + trailing + `$`
}

// SubjectPathRule is a repository-relative path that may end in a slash, as
// corvint.context's subject and the docs tools' paths take it; impactPathRule
// adds the impact path suffixes.
var (
	SubjectPathRule = PathRule{Excluded: `\n\r`, TrailingSlash: true}
	impactPathRule  = PathRule{Excluded: `\n\r`, Suffixes: []string{"go", "rb", "js", "jsx", "mjs", "cjs", "ts", "tsx"}}
)
