package analyzerrust

import "strings"

// The Cargo grammars below are a closed TOML subset, not a TOML parser. Only
// the exact rows named here are accepted; every other TOML feature — inline
// tables, arrays, multi-line or literal strings, dotted keys, datetimes, and
// unknown sections or keys — rejects the whole request. Cargo's own resolution,
// feature unification, and range semantics are never simulated.

var manifestSections = map[string]bool{
	"package": true, "dependencies": true, "dev-dependencies": true,
	"build-dependencies": true,
}

var packageKeys = map[string]bool{
	"name": true, "version": true, "edition": true, "rust-version": true,
}

var editions = map[string]bool{"2015": true, "2018": true, "2021": true, "2024": true}

var namedChannels = map[string]bool{"stable": true, "beta": true, "nightly": true}

// dependencyPredicate maps a manifest section to the predicate its rows emit,
// so a dev-dependency is never conflated with a build or normal dependency.
var dependencyPredicate = map[string]string{
	"dependencies":       "requires",
	"dev-dependencies":   "requires-for-development",
	"build-dependencies": "requires-for-build",
}

// tomlLine is one classified row of the closed subset.
type tomlLine struct {
	section string
	key     string
	value   string
}

// scanTOML splits a closed-subset document into classified rows. It rejects
// any construct outside the subset rather than skipping it.
func scanTOML(content []byte) ([]tomlLine, string) {
	if len(content) > MaxManifestBytes {
		return nil, "LIMIT_EXCEEDED"
	}
	if indexAnyByte(content, "\r\x00") >= 0 {
		return nil, "MALFORMED_INPUT"
	}
	text := string(content)
	if text != "" && !strings.HasSuffix(text, "\n") {
		return nil, "MALFORMED_INPUT"
	}
	rows := make([]tomlLine, 0, 32)
	section := ""
	for _, raw := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")
		if line == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		if line != strings.TrimLeft(line, " \t") {
			return nil, "MALFORMED_INPUT"
		}
		if strings.HasPrefix(line, "[") {
			name, why := sectionName(line)
			if why != "" {
				return nil, why
			}
			section = name
			rows = append(rows, tomlLine{section: name})
			continue
		}
		if section == "" {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		key, value, why := keyValue(line)
		if why != "" {
			return nil, why
		}
		rows = append(rows, tomlLine{section: section, key: key, value: value})
		if len(rows) > MaxManifestRows {
			return nil, "LIMIT_EXCEEDED"
		}
	}
	return rows, ""
}

// sectionName reads a plain table header. An array-of-tables header is not part
// of any accepted record, so it is unsupported rather than silently flattened.
func sectionName(line string) (string, string) {
	if strings.HasPrefix(line, "[[") {
		return "", "UNSUPPORTED_SCHEMA"
	}
	if !strings.HasSuffix(line, "]") {
		return "", "MALFORMED_INPUT"
	}
	return line[1 : len(line)-1], ""
}

// keyValue accepts exactly `key = value` with single spaces around the equals.
// Noncanonical spacing rejects, matching the closed manifest grammars of the
// other families.
func keyValue(line string) (string, string, string) {
	equals := strings.Index(line, " = ")
	if equals < 0 {
		return "", "", "MALFORMED_INPUT"
	}
	key, value := line[:equals], line[equals+3:]
	if key == "" || strings.ContainsAny(key, " \t\"'.[]{}") {
		return "", "", "MALFORMED_INPUT"
	}
	return key, value, ""
}

// quoted unwraps a basic TOML string. Escapes are not part of the subset, so a
// backslash anywhere rejects rather than being interpreted.
func quoted(value string) (string, bool) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", false
	}
	inner := value[1 : len(value)-1]
	if strings.ContainsAny(inner, "\"\\") {
		return "", false
	}
	for i := 0; i < len(inner); i++ {
		if inner[i] < 0x20 || inner[i] > 0x7e {
			return "", false
		}
	}
	return inner, true
}

// parseManifest reads the closed Cargo.toml subset.
func parseManifest(in Input, content []byte, c *factCollector) string {
	rows, why := scanTOML(content)
	if why != "" {
		return why
	}
	seenSections := map[string]bool{}
	seenKeys := map[string]bool{}
	packageName := ""
	for _, row := range rows {
		if row.key == "" {
			if !manifestSections[row.section] {
				return "UNSUPPORTED_SCHEMA"
			}
			if seenSections[row.section] {
				return "DUPLICATE_VALUE"
			}
			seenSections[row.section] = true
			continue
		}
		if seenKeys[row.section+"."+row.key] {
			return "DUPLICATE_VALUE"
		}
		seenKeys[row.section+"."+row.key] = true
		if row.section == "package" {
			name, why := packageRow(in, row, c)
			if why != "" {
				return why
			}
			if name != "" {
				packageName = name
			}
			continue
		}
		if why := dependencyRow(in, row, c); why != "" {
			return why
		}
	}
	if !seenSections["package"] || packageName == "" {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}

func packageRow(in Input, row tomlLine, c *factCollector) (string, string) {
	if !packageKeys[row.key] {
		return "", "UNSUPPORTED_SCHEMA"
	}
	value, ok := quoted(row.value)
	if !ok {
		return "", "UNSUPPORTED_SCHEMA"
	}
	switch row.key {
	case "name":
		if !crateName(value) {
			return "", "INVALID_IDENTIFIER"
		}
		if why := c.declare("package.name", in.Handle, value); why != "" {
			return "", why
		}
		return value, c.add(in.Handle, "rust.package", c.request.ScopeID,
			"declares-package", value, c.request.ScopeID)
	case "version":
		if !exactVersion(value) {
			return "", "UNSUPPORTED_SCHEMA"
		}
		if why := c.declare("package.version", in.Handle, value); why != "" {
			return "", why
		}
		return "", c.add(in.Handle, "rust.package.version", c.request.ScopeID,
			"declares-version", value, c.request.ScopeID)
	case "edition":
		if !editions[value] {
			return "", "UNSUPPORTED_SCHEMA"
		}
		if why := c.declare("package.edition", in.Handle, value); why != "" {
			return "", why
		}
		return "", c.add(in.Handle, "rust.edition", "rust", "declares-edition",
			value, c.request.ScopeID)
	}
	if !exactVersion(value) {
		return "", "UNSUPPORTED_SCHEMA"
	}
	if why := c.declare("package.rust-version", in.Handle, value); why != "" {
		return "", why
	}
	return "", c.add(in.Handle, "rust.language.declaration", "rust",
		"declares-language", value, c.request.ScopeID)
}

// dependencyRow accepts only `name = "X.Y.Z"`. A caret, tilde, wildcard, range,
// inline table, array, or git/path/workspace source is not an exact version and
// rejects, matching the Ruby rule that ranges and pessimistic operators reject.
func dependencyRow(in Input, row tomlLine, c *factCollector) string {
	predicate, ok := dependencyPredicate[row.section]
	if !ok {
		return "UNSUPPORTED_SCHEMA"
	}
	if !crateName(row.key) {
		return "INVALID_IDENTIFIER"
	}
	value, quotedOK := quoted(row.value)
	if !quotedOK || !exactVersion(value) {
		return "UNSUPPORTED_SCHEMA"
	}
	return c.add(in.Handle, "rust.dependency", row.key, predicate, value,
		row.key+"@"+value)
}

// parseToolchain reads the closed rust-toolchain.toml subset: exactly one
// [toolchain] section with a single channel row.
func parseToolchain(in Input, content []byte, c *factCollector) string {
	rows, why := scanTOML(content)
	if why != "" {
		return why
	}
	seen := false
	channel := ""
	for _, row := range rows {
		if row.key == "" {
			if row.section != "toolchain" || seen {
				return "UNSUPPORTED_SCHEMA"
			}
			seen = true
			continue
		}
		if row.key != "channel" || channel != "" {
			return "UNSUPPORTED_SCHEMA"
		}
		value, ok := quoted(row.value)
		if !ok || !(exactVersion(value) || namedChannels[value]) {
			return "UNSUPPORTED_SCHEMA"
		}
		channel = value
	}
	if !seen || channel == "" {
		return "UNSUPPORTED_SCHEMA"
	}
	if why := c.declare("toolchain.channel", in.Handle, channel); why != "" {
		return why
	}
	return c.add(in.Handle, "rust.toolchain.declaration", "rust",
		"declares-toolchain", channel, c.request.ScopeID)
}

// crateName is RUST_NAME: a Cargo package name.
func crateName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			continue
		}
		if i > 0 && (c == '_' || c == '-') {
			continue
		}
		return false
	}
	return true
}

// exactVersion is CORE_VERSION: NUMBER.NUMBER.NUMBER with no leading zero,
// prefix, prerelease, build metadata, or range operator.
func exactVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 9 {
			return false
		}
		if part[0] == '0' && len(part) > 1 {
			return false
		}
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return false
			}
		}
	}
	return true
}
