package analyzerruby

import "strings"

// Closed lexical atoms for the Ruby family.
//
// CORE_VERSION is NUMBER.NUMBER.NUMBER where NUMBER is `0` or a nonzero digit
// followed by digits: no prefix, leading zero, prerelease, build, whitespace,
// or alternate spelling. RUBY_NAME and RUBY_PLATFORM share
// `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`, which admits `x86_64-linux` but not a
// leading separator or any other punctuation.
// RUBY_RUNTIME_VERSION is CORE_VERSION optionally followed by `p` and a
// nonzero-leading patch integer.

const maxAtomBytes = 128

func coreVersion(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !versionNumber(part) {
			return false
		}
	}
	return true
}

// versionNumber accepts `0` or a nonzero-leading digit run.
func versionNumber(s string) bool {
	if s == "" || len(s) > maxAtomBytes {
		return false
	}
	if s == "0" {
		return true
	}
	if s[0] < '1' || s[0] > '9' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func rubyName(s string) bool {
	if s == "" || len(s) > maxAtomBytes {
		return false
	}
	if !alnum(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !alnum(s[i]) && s[i] != '.' && s[i] != '_' && s[i] != '-' {
			return false
		}
	}
	return true
}

func alnum(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9'
}

// runtimeVersion accepts CORE_VERSION with an optional `p` patch suffix whose
// integer is nonzero-leading (`0` itself is a valid nonzero-leading integer).
func runtimeVersion(s string) (string, bool) {
	if at := strings.IndexByte(s, 'p'); at >= 0 {
		if !versionNumber(s[at+1:]) {
			return "", false
		}
		s = s[:at]
	}
	if !coreVersion(s) {
		return "", false
	}
	return s, true
}

// lines splits an LF-terminated document. A body with CR, NUL, or a missing
// final LF is not the closed record.
func lines(body []byte) ([]string, string) {
	for _, b := range body {
		if b == '\r' || b == 0 {
			return nil, "MALFORMED_INPUT"
		}
	}
	if len(body) == 0 || body[len(body)-1] != '\n' {
		return nil, "MALFORMED_INPUT"
	}
	text := string(body[:len(body)-1])
	if text == "" {
		return []string{}, ""
	}
	return strings.Split(text, "\n"), ""
}

// gemDecl is one `NAME`/`VERSION` pair from a Gemfile or a lock DEPENDENCIES row.
type gemDecl struct{ name, version string }

// gemfileDoc is the parsed closed Gemfile: exactly one rubygems source, an
// optional `ruby` declaration, and zero or more exact gem pins.
type gemfileDoc struct {
	rubyVersion string
	gems        []gemDecl
}

// parseGemfile accepts only `source "https://rubygems.org"`, `ruby "X.Y.Z"`,
// and `gem "NAME", "X.Y.Z"` at column zero, plus whole-line `#` comments and
// empty lines. Blocks, groups, method calls, interpolation, variables,
// alternate URLs, and Git/path sources are outside the closed grammar.
func parseGemfile(body []byte) (*gemfileDoc, string) {
	rows, why := lines(body)
	if why != "" {
		return nil, why
	}
	doc := &gemfileDoc{}
	seenSource := false
	seen := map[string]bool{}
	for _, line := range rows {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case line == `source "`+gemfileRemote+`"`:
			if seenSource {
				return nil, "DUPLICATE_VALUE"
			}
			seenSource = true
		case strings.HasPrefix(line, `ruby "`):
			version, ok := quotedTail(line, `ruby "`)
			if !ok || !coreVersion(version) {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			if doc.rubyVersion != "" {
				return nil, "DUPLICATE_VALUE"
			}
			doc.rubyVersion = version
		case strings.HasPrefix(line, `gem "`):
			gem, ok := parseGemLine(line)
			if !ok {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			if seen[gem.name] {
				return nil, "DUPLICATE_VALUE"
			}
			seen[gem.name] = true
			doc.gems = append(doc.gems, gem)
		default:
			return nil, "UNSUPPORTED_SCHEMA"
		}
	}
	if !seenSource {
		return nil, "UNSUPPORTED_SCHEMA"
	}
	return doc, ""
}

// quotedTail returns the body of a trailing `"..."` after prefix.
func quotedTail(line, prefix string) (string, bool) {
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, `"`) || len(line) < len(prefix)+1 {
		return "", false
	}
	value := line[len(prefix) : len(line)-1]
	if strings.ContainsAny(value, `"\#`) {
		return "", false
	}
	return value, true
}

// parseGemLine accepts exactly `gem "NAME", "X.Y.Z"`. Any option, group, git or
// path source, or version range leaves the closed grammar.
func parseGemLine(line string) (gemDecl, bool) {
	rest, ok := strings.CutPrefix(line, `gem "`)
	if !ok {
		return gemDecl{}, false
	}
	name, rest, ok := strings.Cut(rest, `", "`)
	if !ok || !rubyName(name) {
		return gemDecl{}, false
	}
	version, ok := strings.CutSuffix(rest, `"`)
	if !ok || !coreVersion(version) {
		return gemDecl{}, false
	}
	return gemDecl{name, version}, true
}

// lockSpec is one resolved gem plus its resolved dependency edges.
type lockSpec struct {
	name, version string
	deps          []gemDecl
}

// lockDoc is the parsed closed Gemfile.lock.
type lockDoc struct {
	specs        []lockSpec
	platforms    []string
	dependencies []gemDecl
	rubyRuntime  string
	bundler      string
}

// lockCursor walks the lock's exact line grammar.
type lockCursor struct {
	rows []string
	at   int
}

func (c *lockCursor) done() bool { return c.at >= len(c.rows) }

func (c *lockCursor) peek() string {
	if c.done() {
		return "\x00"
	}
	return c.rows[c.at]
}

func (c *lockCursor) take(literal string) bool {
	if c.peek() != literal {
		return false
	}
	c.at++
	return true
}

// parseGemfileLock enforces the lock's exact LF grammar and indentation:
// five sections in order, each exactly once, separated by one empty line, with
// typed-sorted unique rows.
func parseGemfileLock(body []byte) (*lockDoc, string) {
	rows, why := lines(body)
	if why != "" {
		return nil, why
	}
	c := &lockCursor{rows: rows}
	doc := &lockDoc{}
	if why := parseGemSection(c, doc); why != "" {
		return nil, why
	}
	if why := parseListSection(c, "PLATFORMS", &doc.platforms); why != "" {
		return nil, why
	}
	if why := parseDependenciesSection(c, doc); why != "" {
		return nil, why
	}
	if why := parseRubyVersionSection(c, doc); why != "" {
		return nil, why
	}
	if why := parseBundledWithSection(c, doc); why != "" {
		return nil, why
	}
	if !c.done() {
		return nil, "UNSUPPORTED_SCHEMA"
	}
	return doc, ""
}

// separator consumes the single empty line that precedes every section after
// the first.
func separator(c *lockCursor) bool { return c.take("") }

func parseGemSection(c *lockCursor, doc *lockDoc) string {
	if !c.take("GEM") {
		return "UNSUPPORTED_SCHEMA"
	}
	if !c.take("  remote: " + lockRemote) {
		return "UNSUPPORTED_SCHEMA"
	}
	if !c.take("  specs:") {
		return "UNSUPPORTED_SCHEMA"
	}
	seen := map[string]bool{}
	for strings.HasPrefix(c.peek(), "    ") && !strings.HasPrefix(c.peek(), "      ") {
		spec, ok := parseVersionedRow(strings.TrimPrefix(c.peek(), "    "))
		if !ok {
			return "UNSUPPORTED_SCHEMA"
		}
		c.at++
		if seen[spec.name] {
			return "DUPLICATE_VALUE"
		}
		if len(doc.specs) != 0 && doc.specs[len(doc.specs)-1].name >= spec.name {
			return "MALFORMED_INPUT"
		}
		seen[spec.name] = true
		deps, why := parseDependencyRows(c)
		if why != "" {
			return why
		}
		doc.specs = append(doc.specs, lockSpec{spec.name, spec.version, deps})
	}
	if len(doc.specs) == 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	if !separator(c) {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}

// parseDependencyRows reads the six-space resolved dependency edges under one
// spec row.
func parseDependencyRows(c *lockCursor) ([]gemDecl, string) {
	var deps []gemDecl
	seen := map[string]bool{}
	for strings.HasPrefix(c.peek(), "      ") {
		dep, ok := parseVersionedRow(strings.TrimPrefix(c.peek(), "      "))
		if !ok {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		c.at++
		if seen[dep.name] {
			return nil, "DUPLICATE_VALUE"
		}
		if len(deps) != 0 && deps[len(deps)-1].name >= dep.name {
			return nil, "MALFORMED_INPUT"
		}
		seen[dep.name] = true
		deps = append(deps, dep)
	}
	return deps, ""
}

// parseVersionedRow accepts exactly `NAME (X.Y.Z)`.
func parseVersionedRow(row string) (gemDecl, bool) {
	name, rest, ok := strings.Cut(row, " (")
	if !ok || !rubyName(name) {
		return gemDecl{}, false
	}
	version, ok := strings.CutSuffix(rest, ")")
	if !ok || !coreVersion(version) {
		return gemDecl{}, false
	}
	return gemDecl{name, version}, true
}

func parseListSection(c *lockCursor, header string, out *[]string) string {
	if !c.take(header) {
		return "UNSUPPORTED_SCHEMA"
	}
	seen := map[string]bool{}
	for strings.HasPrefix(c.peek(), "  ") {
		value := strings.TrimPrefix(c.peek(), "  ")
		if !rubyName(value) {
			return "UNSUPPORTED_SCHEMA"
		}
		c.at++
		if seen[value] {
			return "DUPLICATE_VALUE"
		}
		if len(*out) != 0 && (*out)[len(*out)-1] >= value {
			return "MALFORMED_INPUT"
		}
		seen[value] = true
		*out = append(*out, value)
	}
	if len(*out) == 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	if !separator(c) {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}

func parseDependenciesSection(c *lockCursor, doc *lockDoc) string {
	if !c.take("DEPENDENCIES") {
		return "UNSUPPORTED_SCHEMA"
	}
	seen := map[string]bool{}
	for strings.HasPrefix(c.peek(), "  ") {
		row := strings.TrimPrefix(c.peek(), "  ")
		name, rest, ok := strings.Cut(row, " (= ")
		if !ok || !rubyName(name) {
			return "UNSUPPORTED_SCHEMA"
		}
		version, ok := strings.CutSuffix(rest, ")")
		if !ok || !coreVersion(version) {
			return "UNSUPPORTED_SCHEMA"
		}
		c.at++
		if seen[name] {
			return "DUPLICATE_VALUE"
		}
		if len(doc.dependencies) != 0 && doc.dependencies[len(doc.dependencies)-1].name >= name {
			return "MALFORMED_INPUT"
		}
		seen[name] = true
		doc.dependencies = append(doc.dependencies, gemDecl{name, version})
	}
	if len(doc.dependencies) == 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	if !separator(c) {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}

func parseRubyVersionSection(c *lockCursor, doc *lockDoc) string {
	if !c.take("RUBY VERSION") {
		return "UNSUPPORTED_SCHEMA"
	}
	declared, ok := strings.CutPrefix(c.peek(), "   ruby ")
	if !ok {
		return "UNSUPPORTED_SCHEMA"
	}
	core, ok := runtimeVersion(declared)
	if !ok {
		return "UNSUPPORTED_SCHEMA"
	}
	c.at++
	doc.rubyRuntime = core
	if !separator(c) {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}

func parseBundledWithSection(c *lockCursor, doc *lockDoc) string {
	if !c.take("BUNDLED WITH") {
		return "UNSUPPORTED_SCHEMA"
	}
	version, ok := strings.CutPrefix(c.peek(), "   ")
	if !ok || !coreVersion(version) {
		return "UNSUPPORTED_SCHEMA"
	}
	c.at++
	doc.bundler = version
	return ""
}

// parseRubyVersionFile accepts exactly one LF-terminated CORE_VERSION line.
func parseRubyVersionFile(body []byte) (string, string) {
	rows, why := lines(body)
	if why != "" {
		return "", why
	}
	if len(rows) != 1 || !coreVersion(rows[0]) {
		return "", "UNSUPPORTED_SCHEMA"
	}
	return rows[0], ""
}
