// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	"path"
	"slices"
	"strings"
)

// pytest reads one config file per run: the first of configNames[pytest]
// present that declares it, in pytest's documented search order (pytest.toml
// and .pytest.toml since 9.0, then pytest.ini and .pytest.ini, which hold
// even when empty; then pyproject.toml, tox.ini and setup.cfg only with a
// pytest table or section). Which file an older pytest reads differs (it
// ignores the names it predates), so provePytest proves every declaring
// config in the root, not only the first.

// pytestConfig is how a pytest config file is read.
type pytestConfig struct {
	toml    bool   // TOML rather than INI
	section string // the INI section pytest reads
	always  bool   // declares pytest with no pytest section or table
}

var pytestConfigs = map[string]pytestConfig{
	"pytest.toml":    {toml: true, always: true},
	".pytest.toml":   {toml: true, always: true},
	"pytest.ini":     {section: "pytest", always: true},
	".pytest.ini":    {section: "pytest", always: true},
	"pyproject.toml": {toml: true},
	"tox.ini":        {section: "pytest"},
	"setup.cfg":      {section: "tool:pytest"},
}

// declaresPytest reports whether the Python config file name declares
// pytest: always for pytest.toml, .pytest.toml, pytest.ini and .pytest.ini;
// for pyproject.toml a table or key under tool.pytest (tool.pytest.ini_options
// or the pytest 9 native tool.pytest), or any "pytest" text or an escaped key
// when the reader cannot scan the file; for tox.ini and setup.cfg the pytest section, or an
// indented line holding its header, which provePytest then refuses. A read
// that fails declares nothing.
func declaresPytest(name string, read func(name string, limit int) ([]byte, error)) bool {
	config, ok := pytestConfigs[path.Base(name)]
	if !ok {
		return false
	}
	if config.always {
		return true
	}
	data, err := read(name, maxConfigProbe)
	if err != nil {
		return false
	}
	if config.toml {
		doc, cause := scanTOML(data)
		if cause != "" {
			// An escaped key may spell tool."pytest" without the text.
			return bytes.Contains(data, []byte("pytest")) || cause == tomlKeyEscape
		}
		atOrUnder := func(full []string) bool {
			return len(full) >= len(pyprojectNative) && slices.Equal(full[:len(pyprojectNative)], pyprojectNative)
		}
		for _, table := range doc.tables {
			if atOrUnder(table) {
				return true
			}
		}
		for _, entry := range doc.entries {
			if atOrUnder(entry.path) || (slices.Equal(entry.path, []string{"tool"}) && strings.Contains(entry.raw, "pytest")) {
				return true
			}
		}
		return false
	}
	lines := parseINI(data)
	return iniHasSection(lines, config.section) || iniIndentedHeader(lines, config.section)
}

// The pyproject.toml tables pytest reads: INI-style options, and the
// native TOML table of pytest 9.
var (
	pyprojectINI    = []string{"tool", "pytest", "ini_options"}
	pyprojectNative = []string{"tool", "pytest"}
)

// provePytestConfig proves one pytest config file: it sets no python_files
// and no testpaths, and its addopts hold only pytestSafeOptions. prefix
// starts each cause (the file's name for a config other than the
// instance's).
func provePytestConfig(p *proof, prefix, name string, read func(name string, limit int) ([]byte, error)) {
	data, err := read(name, maxConfigProbe)
	if err != nil {
		p.add(prefix + "an unreadable config")
		p.docs = true
		return
	}
	config := pytestConfigs[path.Base(name)]
	if config.toml {
		provePytestTOML(p, prefix, path.Base(name), data)
		return
	}
	lines := parseINI(data)
	if iniIndentedHeader(lines, config.section) {
		p.add(prefix + "an indented [" + config.section + "] line, which the INI reader takes as a continuation")
		p.docs = true
	}
	// A pytest.ini with no [pytest] section is read whole (fail closed).
	inside := config.always && !iniHasSection(lines, config.section)
	var key string
	for _, line := range lines {
		switch {
		case line.header:
			inside, key = line.section == config.section, ""
			continue
		case !inside:
			continue
		case line.key != "":
			key = line.key
			provePytestKey(p, prefix, key)
		}
		if key == "addopts" {
			provePytestOptions(p, prefix, line.value)
		}
	}
}

func provePytestKey(p *proof, prefix, key string) {
	switch key {
	case "python_files", "testpaths":
		p.add(prefix + "sets " + key)
	}
}

// provePytestTOML proves a TOML pytest config from its scanned keys: every
// key under the pytest table, at any indentation, bare, quoted or dotted.
// A file the scanner cannot read (one starting with a UTF-8 byte order mark
// among them), an escaped key, or a pytest table set inline is unproven; a
// table below the pytest table counts as setting the key it names. In pytest.toml a top-level key counts as a pytest key
// too (fail closed).
func provePytestTOML(p *proof, prefix, base string, data []byte) {
	doc, cause := scanTOML(data)
	if cause != "" {
		p.add(prefix + cause)
		p.docs = true
		return
	}
	// A table below the pytest table sets the option it names to a table
	// (pytest takes its text), even with no key under it.
	for _, table := range doc.tables {
		if key, whole := pytestTOMLKey(base, tomlEntry{path: table}); !whole {
			provePytestKey(p, prefix, key)
		}
	}
	for _, entry := range doc.entries {
		key, whole := pytestTOMLKey(base, entry)
		switch {
		case whole:
			p.add(prefix + "sets the pytest table inline")
			p.docs = true
		case key == "addopts":
			provePytestOptions(p, prefix, strings.Join(entry.strings, " "))
		default:
			provePytestKey(p, prefix, key)
		}
	}
}

// pytestTOMLKey is the pytest option a TOML entry sets, or whole when the
// entry assigns the pytest table itself (or a table holding it) as a value.
func pytestTOMLKey(base string, entry tomlEntry) (string, bool) {
	full := entry.path
	if base != "pyproject.toml" {
		switch {
		case slices.Equal(full, []string{"pytest"}):
			return "", true
		case hasPathPrefix(full, []string{"pytest"}):
			return full[1], false
		case len(full) == 1:
			return full[0], false
		}
		return "", false
	}
	switch {
	case len(full) <= len(pyprojectINI) && slices.Equal(full, pyprojectINI[:len(full)]):
		return "", len(full) > 1 || strings.Contains(entry.raw, "pytest")
	case hasPathPrefix(full, pyprojectINI):
		return full[len(pyprojectINI)], false
	case hasPathPrefix(full, pyprojectNative):
		return full[len(pyprojectNative)], false
	}
	return "", false
}

func hasPathPrefix(full, prefix []string) bool {
	return len(full) > len(prefix) && slices.Equal(full[:len(prefix)], prefix)
}

// iniLine is one meaningful line of an INI file, read as pytest's iniconfig
// reads it: a section header starts at column 0 with "[" and ends with "]"
// before any "#" or ";" comment; an unindented line is a key, split at its
// first "=" or ":"; an indented line, or a "[" line with no closing "]",
// continues the value above. Blank and comment lines are dropped.
type iniLine struct {
	header  bool
	section string
	key     string
	value   string
	raw     string
}

func parseINI(data []byte) []iniLine {
	var lines []iniLine
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, " \t\r\f\v")
		trimmed := strings.TrimLeft(line, " \t\f\v")
		if trimmed == "" || trimmed[0] == '#' || trimmed[0] == ';' {
			continue
		}
		switch {
		case line[0] == '[':
			head := line
			if cut := strings.IndexAny(head, "#;"); cut >= 0 {
				head = strings.TrimRight(head[:cut], " \t")
			}
			if strings.HasSuffix(head, "]") {
				lines = append(lines, iniLine{header: true, section: head[1 : len(head)-1], raw: line})
			} else {
				lines = append(lines, iniLine{value: trimmed, raw: line})
			}
		case line != trimmed:
			lines = append(lines, iniLine{value: trimmed, raw: line})
		default:
			cut := strings.IndexAny(line, "=:")
			if cut < 0 {
				lines = append(lines, iniLine{value: trimmed, raw: line})
				continue
			}
			lines = append(lines, iniLine{key: strings.TrimSpace(line[:cut]), value: strings.TrimSpace(line[cut+1:]), raw: line})
		}
	}
	return lines
}

func iniHasSection(lines []iniLine, section string) bool {
	return slices.ContainsFunc(lines, func(line iniLine) bool { return line.header && line.section == section })
}

// iniIndentedHeader reports an indented line that reads as the section's
// header once trimmed: iniconfig takes it as a value continuation, but a
// reader that trims lines would not, so it is unproven.
func iniIndentedHeader(lines []iniLine, section string) bool {
	return slices.ContainsFunc(lines, func(line iniLine) bool {
		return !line.header && line.key == "" && strings.HasPrefix(line.value, "["+section+"]")
	})
}

// tomlDoc is what scanTOML reads from a TOML document: every table header's
// key path and every key/value entry with its full key path (table path plus
// dotted key). Keys inside an inline table are not listed; the entry that
// holds the table is.
type tomlDoc struct {
	tables  [][]string
	entries []tomlEntry
}

// tomlEntry is one key/value line. strings holds the content of every string
// in the value, escapes left as written; raw is the value's source text.
type tomlEntry struct {
	path    []string
	strings []string
	raw     string
}

// tomlKeyEscape is scanTOML's cause for an escape in a quoted key.
const tomlKeyEscape = "an escape in a TOML key"

// maxTOMLDepth bounds array and inline table nesting.
const maxTOMLDepth = 64

type tomlScanner struct {
	src   string
	i     int
	cause string
	depth int
}

// scanTOML reads data as TOML well enough to name each key's path: any
// indentation, bare, quoted and dotted keys, table and array-of-table
// headers, and values of every kind spanning lines (strings, multi-line
// strings, arrays, inline tables, scalars). It returns a cause instead when
// the document is not one it can read, and for an escape in a quoted key
// (an escape could spell a pytest key).
func scanTOML(data []byte) (tomlDoc, string) {
	s := &tomlScanner{src: string(data)}
	var doc tomlDoc
	var table []string
	for s.cause == "" && s.i < len(s.src) {
		s.space()
		switch {
		case s.i >= len(s.src):
		case s.src[s.i] == '\n', strings.HasPrefix(s.src[s.i:], "\r\n"), s.src[s.i] == '#':
			s.lineEnd()
		case s.src[s.i] == '[':
			closing := "]"
			s.i++
			if s.i < len(s.src) && s.src[s.i] == '[' {
				closing = "]]"
				s.i++
			}
			key := s.key()
			s.space()
			if s.cause == "" && !strings.HasPrefix(s.src[s.i:], closing) {
				s.fail()
			}
			s.i += len(closing)
			table = key
			doc.tables = append(doc.tables, key)
			s.lineEnd()
		default:
			key := s.key()
			s.space()
			if s.cause != "" || s.i >= len(s.src) || s.src[s.i] != '=' {
				s.fail()
				break
			}
			s.i++
			s.space()
			entry := tomlEntry{path: append(slices.Clone(table), key...)}
			start := s.i
			s.value(&entry)
			if s.i <= len(s.src) && start <= s.i {
				entry.raw = s.src[start:min(s.i, len(s.src))]
			}
			doc.entries = append(doc.entries, entry)
			s.lineEnd()
		}
	}
	if s.cause != "" {
		return tomlDoc{}, s.cause
	}
	return doc, ""
}

func (s *tomlScanner) fail() {
	if s.cause == "" {
		s.cause = "a TOML document the reader cannot scan"
	}
}

func (s *tomlScanner) space() {
	for s.i < len(s.src) && (s.src[s.i] == ' ' || s.src[s.i] == '\t') {
		s.i++
	}
}

// blank skips whitespace, newlines and comments inside an array or inline
// table.
func (s *tomlScanner) blank() {
	for s.i < len(s.src) {
		switch c := s.src[s.i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			s.i++
		case c == '#':
			for s.i < len(s.src) && s.src[s.i] != '\n' {
				s.i++
			}
		default:
			return
		}
	}
}

// lineEnd accepts the rest of a line: whitespace, an optional comment, then
// a newline or the end of the document.
func (s *tomlScanner) lineEnd() {
	if s.cause != "" {
		return
	}
	s.space()
	if s.i < len(s.src) && s.src[s.i] == '#' {
		for s.i < len(s.src) && s.src[s.i] != '\n' {
			s.i++
		}
	}
	switch {
	case s.i >= len(s.src):
	case s.src[s.i] == '\n':
		s.i++
	case strings.HasPrefix(s.src[s.i:], "\r\n"):
		s.i += 2
	default:
		s.fail()
	}
}

// key reads a dotted key: bare, basic-quoted or literal-quoted parts.
func (s *tomlScanner) key() []string {
	var parts []string
	for s.cause == "" {
		s.space()
		if s.i >= len(s.src) {
			s.fail()
			return nil
		}
		switch c := s.src[s.i]; {
		case c == '"' || c == '\'':
			end := strings.IndexAny(s.src[s.i+1:], string(c)+"\n")
			if end < 0 || s.src[s.i+1+end] != c {
				s.fail()
				return nil
			}
			part := s.src[s.i+1 : s.i+1+end]
			if c == '"' && strings.Contains(part, `\`) {
				s.cause = tomlKeyEscape
				return nil
			}
			parts = append(parts, part)
			s.i += end + 2
		default:
			start := s.i
			for s.i < len(s.src) && bareKeyByte(s.src[s.i]) {
				s.i++
			}
			if s.i == start {
				s.fail()
				return nil
			}
			parts = append(parts, s.src[start:s.i])
		}
		s.space()
		if s.i < len(s.src) && s.src[s.i] == '.' {
			s.i++
			continue
		}
		return parts
	}
	return nil
}

func bareKeyByte(c byte) bool {
	return c == '_' || c == '-' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// value reads one value, recording the content of each string in it.
func (s *tomlScanner) value(entry *tomlEntry) {
	if s.cause != "" {
		return
	}
	if s.i >= len(s.src) {
		s.fail()
		return
	}
	s.depth++
	defer func() { s.depth-- }()
	if s.depth > maxTOMLDepth {
		s.cause = "a TOML value nested too deeply"
		return
	}
	rest := s.src[s.i:]
	switch {
	case strings.HasPrefix(rest, `"""`), strings.HasPrefix(rest, "'''"):
		delimiter := rest[:3]
		j := 3
		for {
			if j >= len(rest) {
				s.fail()
				return
			}
			if delimiter == `"""` && rest[j] == '\\' {
				j += 2
				continue
			}
			if strings.HasPrefix(rest[j:], delimiter) {
				break
			}
			j++
		}
		end := j + 3
		// Up to two quotes may sit just inside the closing delimiter.
		for extra := 0; extra < 2 && end < len(rest) && rest[end] == delimiter[0]; extra++ {
			end++
		}
		entry.strings = append(entry.strings, rest[3:end-3])
		s.i += end
	case rest[0] == '"' || rest[0] == '\'':
		j := 1
		for ; j < len(rest) && rest[j] != rest[0]; j++ {
			if rest[j] == '\n' {
				s.fail()
				return
			}
			if rest[0] == '"' && rest[j] == '\\' {
				j++
			}
		}
		if j >= len(rest) {
			s.fail()
			return
		}
		entry.strings = append(entry.strings, rest[1:j])
		s.i += j + 1
	case rest[0] == '[':
		s.i++
		for s.cause == "" {
			s.blank()
			if s.i < len(s.src) && s.src[s.i] == ']' {
				s.i++
				return
			}
			s.value(entry)
			s.blank()
			switch {
			case s.i < len(s.src) && s.src[s.i] == ',':
				s.i++
			case s.i < len(s.src) && s.src[s.i] == ']':
				s.i++
				return
			default:
				s.fail()
			}
		}
	case rest[0] == '{':
		s.i++
		for s.cause == "" {
			s.blank()
			if s.i < len(s.src) && s.src[s.i] == '}' {
				s.i++
				return
			}
			s.key()
			s.space()
			if s.cause != "" || s.i >= len(s.src) || s.src[s.i] != '=' {
				s.fail()
				return
			}
			s.i++
			s.space()
			s.value(entry)
			s.blank()
			switch {
			case s.i < len(s.src) && s.src[s.i] == ',':
				s.i++
			case s.i < len(s.src) && s.src[s.i] == '}':
				s.i++
				return
			default:
				s.fail()
			}
		}
	default:
		j := strings.IndexAny(rest, "\n#,]}")
		if j < 0 {
			j = len(rest)
		}
		if strings.TrimSpace(strings.TrimRight(rest[:j], "\r")) == "" {
			s.fail()
			return
		}
		s.i += j
	}
}
