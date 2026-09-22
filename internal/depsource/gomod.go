package depsource

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

type requirement struct {
	Path    string
	Version string
}

type replacement struct {
	OldPath    string
	OldVersion string // Empty when the directive replaces every version.
	NewPath    string
	NewVersion string // Empty when the target is a local directory.
}

type goModule struct {
	Requires           []requirement
	Replaces           []replacement
	Excludes           []requirement
	replacementTargets map[[2]string]replacement
}

// parseGoMod reads the require, replace, and exclude directives this verb
// needs. It is a deliberately small reader: golang.org/x/mod/modfile is not in
// the dependency graph and the local product stays one binary with no added
// module (invariant 7). A line it cannot tokenize is refused rather than
// skipped: a dropped require reads downstream as "no commit requires this
// module", which is a claim the reader never established (invariant 2).
func parseGoMod(content []byte) (goModule, error) {
	if !utf8.Valid(content) {
		return goModule{}, &Error{Message: "go.mod is not valid UTF-8"}
	}
	parsed := goModule{replacementTargets: map[[2]string]replacement{}}
	block := ""
	for number, raw := range strings.Split(string(content), "\n") {
		fields, err := tokenizeGoModLine(raw)
		if err != nil {
			return goModule{}, goModLineError(number+1, err)
		}
		if len(fields) == 0 {
			continue
		}
		if block != "" {
			if len(fields) == 1 && fields[0] == ")" {
				block = ""
				continue
			}
			if isGoModSyntaxToken(fields[0]) {
				return goModule{}, goModLineError(number+1, errors.New(block+" block contains a nested directive"))
			}
			if err := parsed.absorb(block, fields); err != nil {
				return goModule{}, goModLineError(number+1, err)
			}
			continue
		}
		directive := fields[0]
		if isGoModDirective(directive) {
			if len(fields) == 2 && fields[1] == "(" {
				block = directive
				continue
			}
			if err := parsed.absorb(directive, fields[1:]); err != nil {
				return goModule{}, goModLineError(number+1, err)
			}
			continue
		}
		if err := validateGoModDeclaration(fields); err != nil {
			return goModule{}, goModLineError(number+1, err)
		}
	}
	if block != "" {
		return goModule{}, &Error{Message: "go.mod: unterminated " + block + " block"}
	}
	parsed.replacementTargets = nil
	return parsed, nil
}

func goModLineError(number int, err error) error {
	return &Error{Message: "go.mod line " + strconv.Itoa(number) + ": " + err.Error()}
}

func isGoModDirective(token string) bool {
	return token == "require" || token == "replace" || token == "exclude"
}

// validateGoModDeclaration accepts the top-level declarations that cannot
// affect dependency resolution. Every other directive is outside this small
// parser's grammar and is refused rather than risk interpreting one of its
// block lines as a require, replace, or exclude.
func validateGoModDeclaration(fields []string) error {
	if fields[0] != "module" && fields[0] != "go" && fields[0] != "toolchain" {
		return errors.New("unsupported directive " + fields[0])
	}
	if len(fields) != 2 || fields[1] == "(" || fields[1] == ")" {
		return errors.New(fields[0] + " directive expects exactly one argument")
	}
	return nil
}

func (parsed *goModule) absorb(directive string, fields []string) error {
	for _, field := range fields {
		if field == "(" || field == ")" {
			return errors.New(directive + " directive contains an unexpected parenthesis")
		}
	}
	switch directive {
	case "require":
		if len(fields) != 2 {
			return errors.New("require directive expects a module path and version")
		}
		parsed.Requires = append(parsed.Requires, requirement{fields[0], fields[1]})
	case "exclude":
		if len(fields) != 2 {
			return errors.New("exclude directive expects a module path and version")
		}
		parsed.Excludes = append(parsed.Excludes, requirement{fields[0], fields[1]})
	case "replace":
		arrow := indexOf(fields, "=>")
		if arrow < 1 || arrow > 2 || len(fields)-arrow-1 < 1 || len(fields)-arrow-1 > 2 ||
			indexOf(fields[arrow+1:], "=>") >= 0 {
			return errors.New("replace directive has unsupported shape")
		}
		left, right := fields[:arrow], fields[arrow+1:]
		entry := replacement{OldPath: left[0], NewPath: right[0]}
		if len(left) > 1 {
			entry.OldVersion = left[1]
		}
		if len(right) > 1 {
			entry.NewVersion = right[1]
		}
		key := [2]string{entry.OldPath, entry.OldVersion}
		if existing, found := parsed.replacementTargets[key]; found && existing != entry {
			return errors.New("replace directives disagree on the target")
		}
		parsed.replacementTargets[key] = entry
		parsed.Replaces = append(parsed.Replaces, entry)
	}
	return nil
}

func indexOf(fields []string, target string) int {
	for position, value := range fields {
		if value == target {
			return position
		}
	}
	return -1
}

// tokenizeGoModLine splits one committed go.mod line into directive fields.
// A field is a bare run of non-space bytes or a Go string literal, quoted or
// backquoted, which strconv.Unquote decodes; the go.mod grammar allows a
// literal wherever a bare token would be ambiguous, such as a path holding a
// space. An unquoted "//" opens a line comment and ends the line.
func tokenizeGoModLine(line string) ([]string, error) {
	fields := []string{}
	rest := line
	for {
		rest = strings.TrimLeft(rest, " \t\r")
		if rest == "" || strings.HasPrefix(rest, "//") {
			return fields, nil
		}
		if strings.HasPrefix(rest, "/*") {
			return nil, errors.New("block comments are unsupported")
		}
		field, remainder, err := nextGoModField(rest)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
		rest = remainder
	}
}

// nextGoModField consumes one field from the head of a left-trimmed go.mod
// line. A malformed literal is refused rather than decoded into bytes that
// would read as a module path no committed line carries.
func nextGoModField(rest string) (string, string, error) {
	if rest[0] == '(' || rest[0] == ')' {
		return rest[:1], rest[1:], nil
	}
	if rest[0] != '"' && rest[0] != '`' {
		return bareGoModField(rest)
	}
	end := goModLiteralEnd(rest)
	if end < 0 {
		return "", "", errors.New("unterminated string literal")
	}
	value, err := strconv.Unquote(rest[:end])
	if err != nil {
		return "", "", errors.New("malformed string literal " + rest[:end])
	}
	if isGoModSyntaxToken(value) {
		return "", "", errors.New("string literal decodes to syntax token " + value)
	}
	if !startsGoModGap(rest[end:]) {
		return "", "", errors.New("string literal is not followed by a separator")
	}
	return value, rest[end:], nil
}

// isGoModSyntaxToken identifies values whose role depends on being unquoted.
// Refusing a literal that decodes to one keeps token kind from being erased:
// for example, `")"` must not close a require block.
func isGoModSyntaxToken(value string) bool {
	switch value {
	case "(", ")", "=>", "module", "go", "toolchain", "require", "replace", "exclude":
		return true
	default:
		return false
	}
}

// bareGoModField consumes an unquoted field, which ends at the next separator
// or at a "//" comment opened partway through the token.
func bareGoModField(rest string) (string, string, error) {
	end := len(rest)
	for _, separator := range []string{" ", "\t", "\r", "(", ")", "//", "/*"} {
		if offset := strings.Index(rest, separator); offset >= 0 && offset < end {
			end = offset
		}
	}
	field := rest[:end]
	if strings.ContainsAny(field, "\"`") {
		return "", "", errors.New("unquoted field contains a quote")
	}
	if strings.HasPrefix(rest[end:], "/*") {
		return "", "", errors.New("block comments are unsupported")
	}
	return field, rest[end:], nil
}

// startsGoModGap reports whether a remainder begins where a field may end:
// end of line, a separator, or a comment.
func startsGoModGap(remainder string) bool {
	if remainder == "" || strings.HasPrefix(remainder, "//") || remainder[0] == '(' || remainder[0] == ')' {
		return true
	}
	return strings.IndexByte(" \t\r", remainder[0]) >= 0
}

// goModLiteralEnd returns the offset just past a literal's closing delimiter,
// or -1 when the line ends before it.
func goModLiteralEnd(rest string) int {
	if rest[0] == '`' {
		closing := strings.IndexByte(rest[1:], '`')
		if closing < 0 {
			return -1
		}
		return closing + 2
	}
	for offset := 1; offset < len(rest); offset++ {
		switch rest[offset] {
		case '\\':
			offset++
		case '"':
			return offset + 1
		}
	}
	return -1
}

// requiredVersion reports the single go.mod version for a module path. Two
// differing versions for one path are ambiguous, never silently first-wins.
// An excluded pin is still reported here; the caller abstains on it by name
// (excluded-version) rather than letting it read as a module nothing requires.
func (parsed goModule) requiredVersion(path string) (string, error) {
	found := ""
	for _, entry := range parsed.Requires {
		if entry.Path != path {
			continue
		}
		if found != "" && found != entry.Version {
			return "", &Error{Message: "ambiguous version"}
		}
		found = entry.Version
	}
	return found, nil
}

// excluded reports whether the committed go.mod excludes this exact
// module@version pin. An excluded version must never be reported as resolved,
// pinned evidence (AGENTS.md invariant 2).
func (parsed goModule) excluded(path, version string) bool {
	for _, entry := range parsed.Excludes {
		if entry.Path == path && entry.Version == version {
			return true
		}
	}
	return false
}

func (parsed goModule) replacementFor(path, version string) (replacement, bool) {
	for _, entry := range parsed.Replaces {
		if entry.OldPath == path && entry.OldVersion == version {
			return entry, true
		}
	}
	for _, entry := range parsed.Replaces {
		if entry.OldPath == path && entry.OldVersion == "" {
			return entry, true
		}
	}
	return replacement{}, false
}

// sumHash returns the "h1:" line for module@version in go.sum. The "/go.mod"
// line hashes only the module file and is never the source hash.
func sumHash(content []byte, path, version string) string {
	prefix := path + " " + version + " "
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, prefix))
	}
	return ""
}
