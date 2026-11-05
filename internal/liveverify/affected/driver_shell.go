package affected

import (
	"path"
	"strings"
)

// driverComputed marks text a driver scanner could not resolve to a literal:
// a make expansion it does not evaluate or a workflow `${{ }}` expression
// (AZU-V0-001). driverExpansion marks a shell parameter expansion. Both are
// word characters to the shell scanner and separators to path tokens.
const (
	driverComputed  = "\x01"
	driverExpansion = "\x02"
)

// driverScan is what the shell scanner found in recipe or step text: the
// literal path tokens, the paths whose owners the text runs (PathImports),
// the make invocations, and whether a command word is computed.
type driverScan struct {
	computed bool
	tokens   map[string]bool
	paths    map[string]bool
	makes    []driverMakeCall
}

// driverMakeCall is one `make` invocation. Dir is the repository-relative
// directory make runs in, File the make file relative to it ("" for the
// default lookup), Targets the named goals (none for the default goal).
// Unbounded marks a computed directory or file, Computed a computed goal.
type driverMakeCall struct {
	Dir       string
	File      string
	Targets   []string
	Unbounded bool
	Computed  bool
}

func newDriverScan() *driverScan {
	return &driverScan{tokens: map[string]bool{}, paths: map[string]bool{}}
}

// driverWord is one shell word: its text with quotes removed and each
// expansion replaced by driverExpansion, and whether any part was quoted.
type driverWord struct {
	text   string
	quoted bool
}

// driverShellScanner splits shell text into simple-command segments.
type driverShellScanner struct {
	scan     *driverScan
	module   string
	depth    int
	heredocs []string
}

// scanShell scans script, run with working directory cwd ("" when unknown),
// into scan. module is the main Go module path, "" when there is none.
func (scan *driverScan) scanShell(script, cwd, module string) {
	scanner := &driverShellScanner{scan: scan, module: module}
	scanner.script(script, cwd)
}

func (s *driverShellScanner) script(script, cwd string) {
	if s.depth > 8 {
		s.scan.computed = true
		return
	}
	s.depth++
	defer func() { s.depth-- }()
	for _, token := range driverPathTokens(script) {
		s.scan.tokens[token] = true
	}
	var words []driverWord
	var redirect bool
	flush := func() {
		if len(words) != 0 {
			cwd = s.command(words, cwd)
		}
		words = nil
	}
	lines := strings.Split(script, "\n")
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		position := 0
		for position < len(line) {
			char := line[position]
			switch {
			case char == ' ' || char == '\t' || char == '\r':
				position++
			case char == '#':
				position = len(line)
			case char == ';' || char == '|' || char == '(' || char == ')' || char == '`':
				flush()
				position++
			case char == '&':
				if position+1 < len(line) && line[position+1] == '>' {
					redirect = true
					position += 2
					continue
				}
				flush()
				position++
			case char == '<' && strings.HasPrefix(line[position:], "<<") && !strings.HasPrefix(line[position:], "<<<"):
				position += 2
				if position < len(line) && line[position] == '-' {
					position++
				}
				for position < len(line) && (line[position] == ' ' || line[position] == '\t') {
					position++
				}
				word, next := s.word(line, position, cwd)
				position = next
				if word.text != "" {
					s.heredocs = append(s.heredocs, word.text)
				}
			case char == '>' || char == '<':
				redirect = true
				position++
				for position < len(line) && (line[position] == '>' || line[position] == '&' || line[position] == '|') {
					position++
				}
			default:
				word, next := s.word(line, position, cwd)
				position = next
				if redirect {
					redirect = false
					continue
				}
				if !word.quoted && (word.text == "{" || word.text == "}") {
					flush()
					continue
				}
				if word.text != "" || word.quoted {
					words = append(words, word)
				}
			}
		}
		if strings.HasSuffix(line, "\\") {
			continue
		}
		flush()
		redirect = false
		for len(s.heredocs) != 0 && index+1 < len(lines) {
			terminator := s.heredocs[0]
			s.heredocs = s.heredocs[1:]
			for index+1 < len(lines) {
				index++
				if strings.TrimLeft(lines[index], "\t") == terminator {
					break
				}
			}
		}
		s.heredocs = nil
	}
	flush()
}

// word reads one shell word starting at position. A command substitution
// inside it is scanned as its own script and leaves an expansion marker.
func (s *driverShellScanner) word(line string, position int, cwd string) (driverWord, int) {
	var text strings.Builder
	quoted := false
	inDouble := false
	for position < len(line) {
		char := line[position]
		if !inDouble && strings.IndexByte(" \t\r;&|()<>`", char) >= 0 {
			break
		}
		switch {
		case char == '\\' && position+1 < len(line):
			text.WriteByte(line[position+1])
			position += 2
		case char == '\\':
			position++
		case char == '\'' && !inDouble:
			// A quote after a literal `NAME=` leaves the word an assignment.
			quoted = quoted || !driverAssignment(text.String())
			end := strings.IndexByte(line[position+1:], '\'')
			if end < 0 {
				text.WriteString(line[position+1:])
				position = len(line)
				continue
			}
			text.WriteString(line[position+1 : position+1+end])
			position += end + 2
		case char == '"':
			quoted = quoted || !driverAssignment(text.String())
			inDouble = !inDouble
			position++
		case char == '`' && inDouble:
			end := strings.IndexByte(line[position+1:], '`')
			if end < 0 {
				end = len(line) - position - 1
			}
			s.script(line[position+1:position+1+end], cwd)
			text.WriteString(driverExpansion)
			position += end + 2
		case char == '$':
			next := position + 1
			switch {
			case next < len(line) && line[next] == '(':
				end := driverMatchingClose(line, next)
				inner := line[next+1 : max(next+1, end)]
				if !strings.HasPrefix(inner, "(") {
					s.script(inner, cwd)
				}
				text.WriteString(driverExpansion)
				position = min(end+1, len(line))
			case next < len(line) && line[next] == '{':
				end := driverMatchingClose(line, next)
				text.WriteString(driverExpansion)
				position = min(end+1, len(line))
			case next < len(line) && (isDriverNameByte(line[next]) || strings.IndexByte("@*#?$!-0123456789", line[next]) >= 0):
				if isDriverNameByte(line[next]) {
					for next < len(line) && isDriverNameByte(line[next]) {
						next++
					}
				} else {
					next++
				}
				text.WriteString(driverExpansion)
				position = next
			default:
				text.WriteByte('$')
				position++
			}
		default:
			text.WriteByte(char)
			position++
		}
	}
	return driverWord{text: text.String(), quoted: quoted}, position
}

// driverMatchingClose returns the index of the bracket closing the one at
// open, honouring nesting and quotes, or len(text) when it is unclosed.
func driverMatchingClose(text string, open int) int {
	closing := byte(')')
	if text[open] == '{' {
		closing = '}'
	}
	depth := 0
	var quote byte
	for index := open; index < len(text); index++ {
		char := text[index]
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '\'' || char == '"':
			quote = char
		case char == text[open]:
			depth++
		case char == closing:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return len(text)
}

func isDriverNameByte(char byte) bool {
	return char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
}

var driverPrefixKeywords = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "fi": true, "do": true, "done": true,
	"while": true, "until": true, "!": true, "time": true, "exec": true, "command": true,
	"builtin": true, "nohup": true, "sudo": true, "stdbuf": true,
}

var driverShells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true}

var driverInterpreters = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "python": true, "python3": true,
	"node": true, "bun": true, "deno": true, "ruby": true, "perl": true, "pwsh": true,
}

// command handles one simple command and returns the working directory the
// next command of the same script runs in.
func (s *driverShellScanner) command(words []driverWord, cwd string) string {
	for len(words) != 0 {
		head := words[0].text
		switch {
		case !words[0].quoted && driverAssignment(head):
			words = words[1:]
		case !words[0].quoted && driverPrefixKeywords[head]:
			words = words[1:]
		case head == "env":
			words = words[1:]
			for len(words) != 0 && (strings.HasPrefix(words[0].text, "-") || driverAssignment(words[0].text)) {
				if words[0].text == "-u" || words[0].text == "-C" || words[0].text == "-S" {
					if len(words) < 2 {
						s.scan.computed = true
						return cwd
					}
					words = words[1:]
				}
				words = words[1:]
			}
		case head == "nice" || head == "timeout" || head == "xargs" || head == "ionice" || head == "taskset":
			words = words[1:]
			for len(words) != 0 && strings.HasPrefix(words[0].text, "-") {
				if !strings.Contains(words[0].text, "=") && len(words[0].text) == 2 && strings.IndexByte("nksIcpLPdEaR", words[0].text[1]) >= 0 {
					if len(words) < 2 {
						s.scan.computed = true
						return cwd
					}
					words = words[1:]
				}
				words = words[1:]
			}
			if head == "timeout" && len(words) != 0 {
				words = words[1:]
			}
			if head == "taskset" && len(words) != 0 {
				words = words[1:]
			}
		default:
			return s.simple(words, cwd)
		}
	}
	return cwd
}

func driverAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for index := 0; index < len(name); index++ {
		if !isDriverNameByte(name[index]) && !(index == len(name)-1 && name[index] == '+') {
			return false
		}
	}
	return name[0] < '0' || name[0] > '9'
}

func driverDynamic(text string) bool {
	return strings.Contains(text, driverComputed) || strings.Contains(text, driverExpansion)
}

// driverComputedWord reports a command word whose name the scanner cannot
// read: an expansion other than a leading `$VAR/` before a literal path.
func driverComputedWord(text string) bool {
	if !driverDynamic(text) {
		return false
	}
	for _, marker := range []string{driverExpansion, driverComputed} {
		if rest, found := strings.CutPrefix(text, marker+"/"); found && rest != "" && !driverDynamic(rest) {
			return false
		}
	}
	return true
}

func (s *driverShellScanner) simple(words []driverWord, cwd string) string {
	head := words[0].text
	switch head {
	case "for", "case", "select", "function", "in", "esac", "[[", "]]", "[", "test", "echo", "printf", "true", "false", ":", "export", "local", "readonly", "declare", "set", "unset", "shift", "return", "exit", "trap", "wait", "read":
		s.embedded(words[1:], cwd)
		return cwd
	case "cd", "pushd":
		if len(words) < 2 {
			return cwd
		}
		target := words[1].text
		if driverDynamic(target) || cwd == "" {
			return ""
		}
		return driverJoin(cwd, target)
	case "eval":
		s.scan.computed = true
		return cwd
	case ".", "source":
		if len(words) > 1 {
			if driverComputedWord(words[1].text) {
				s.scan.computed = true
			}
			s.pathWord(words[1].text, cwd)
		}
		return cwd
	}
	if driverComputedWord(head) {
		s.scan.computed = true
	}
	base := path.Base(head)
	if driverInterpreters[base] {
		for index := 1; index < len(words); index++ {
			arg := words[index].text
			if driverShells[base] && strings.HasPrefix(arg, "-") && strings.HasSuffix(arg, "c") && !strings.HasPrefix(arg, "--") {
				if index+1 < len(words) {
					s.script(words[index+1].text, cwd)
				}
				break
			}
			if arg == "-c" || arg == "-e" {
				break
			}
			if strings.HasPrefix(arg, "-") {
				if arg == "-m" && index+1 < len(words) {
					index++
				}
				continue
			}
			if driverComputedWord(arg) {
				s.scan.computed = true
			}
			s.pathWord(arg, cwd)
			break
		}
	}
	s.pathWord(head, cwd)
	s.embedded(words, cwd)
	return cwd
}

// embedded finds a go or make invocation anywhere in a command, such as one
// a wrapper script runs, and records every path-like argument.
func (s *driverShellScanner) embedded(words []driverWord, cwd string) {
	for index, word := range words {
		switch path.Base(word.text) {
		case "go":
			if index+1 < len(words) && !driverDynamic(word.text) {
				s.goInvocation(words[index+1:], cwd)
			}
		case "make", "gmake":
			s.makeInvocation(words[index+1:], cwd)
		}
		if index > 0 {
			s.pathWord(word.text, cwd)
		}
	}
}

// pathWord records a path-like word as a path import: the owner of the file,
// or of the files directly in the directory, that it names.
func (s *driverShellScanner) pathWord(word, cwd string) {
	if driverDynamic(word) || strings.HasPrefix(word, "-") || strings.HasPrefix(word, "/") {
		return
	}
	word = strings.TrimSuffix(word, "/")
	if !strings.Contains(word, "/") && !strings.Contains(word, ".") {
		return
	}
	for _, candidate := range driverVariants(cwd, word) {
		if ValidRelativePath(candidate) {
			s.scan.paths[candidate] = true
		}
	}
}

// driverVariants is the word joined to cwd and, when that differs, the word
// as written from the repository root.
func driverVariants(cwd, word string) []string {
	variants := make([]string, 0, 2)
	if cwd != "" {
		variants = append(variants, driverJoin(cwd, word))
	}
	if raw := path.Clean(word); cwd != "." && raw != "." && !strings.HasPrefix(raw, "../") {
		variants = append(variants, raw)
	}
	return variants
}

func driverJoin(cwd, relative string) string {
	if strings.HasPrefix(relative, "/") {
		return ""
	}
	joined := path.Clean(path.Join(cwd, relative))
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return ""
	}
	return joined
}

var driverGoPackageCommands = map[string]bool{
	"test": true, "run": true, "build": true, "vet": true, "install": true, "generate": true, "list": true, "fix": true,
}

var driverGoBoolFlags = map[string]bool{
	"-v": true, "-race": true, "-short": true, "-cover": true, "-json": true, "-x": true, "-n": true, "-a": true,
	"-failfast": true, "-trimpath": true, "-benchmem": true, "-msan": true, "-asan": true, "-linkshared": true,
	"-work": true, "-modcacherw": true, "-i": true, "-c": true, "-m": true, "-e": true, "-deps": true,
	"-test": true, "-find": true, "-compiled": true, "-export": true, "-buildvcs": true, "-fullpath": true,
}

// goInvocation records the package patterns of `go <command> ...` as path
// imports. A computed pattern, or a relative one under an unknown working
// directory, imports every owner.
func (s *driverShellScanner) goInvocation(words []driverWord, cwd string) {
	for len(words) >= 2 && words[0].text == "-C" {
		if driverDynamic(words[1].text) || cwd == "" {
			cwd = ""
		} else {
			cwd = driverJoin(cwd, words[1].text)
		}
		words = words[2:]
	}
	if len(words) == 0 || !driverGoPackageCommands[words[0].text] {
		return
	}
	command := words[0].text
	words = words[1:]
	for index := 0; index < len(words); index++ {
		arg := words[index].text
		if arg == "-args" || arg == "--" {
			return
		}
		if strings.HasPrefix(arg, "-") {
			if arg == "-C" && index+1 < len(words) {
				index++
				if driverDynamic(words[index].text) || cwd == "" {
					cwd = ""
				} else {
					cwd = driverJoin(cwd, words[index].text)
				}
				continue
			}
			if !strings.Contains(arg, "=") && !driverGoBoolFlags[strings.TrimPrefix(arg, "-")] && !driverGoBoolFlags[arg] {
				index++
			}
			continue
		}
		s.goPattern(arg, cwd)
		if command == "run" && !strings.HasSuffix(arg, ".go") {
			return
		}
	}
}

func (s *driverShellScanner) goPattern(pattern, cwd string) {
	switch {
	case driverDynamic(pattern):
		s.scan.paths["..."] = true
		return
	case pattern == "all":
		s.scan.paths["..."] = true
		return
	case pattern == "std" || pattern == "cmd":
		return
	}
	pattern, _, _ = strings.Cut(pattern, "@")
	relative := ""
	switch {
	case pattern == "." || pattern == ".." || strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, "../"):
		if cwd == "" {
			s.scan.paths["..."] = true
			return
		}
		relative = driverGoRelative(cwd, pattern)
	case s.module != "" && (pattern == s.module || pattern == s.module+"/..."):
		s.scan.paths["..."] = true
		return
	case s.module != "" && strings.HasPrefix(pattern, s.module+"/"):
		relative = strings.TrimPrefix(pattern, s.module+"/")
	case strings.HasSuffix(pattern, ".go"):
		if cwd == "" {
			s.scan.paths["..."] = true
			return
		}
		relative = driverJoin(cwd, pattern)
	default:
		return
	}
	if relative == "" {
		s.scan.paths["..."] = true
		return
	}
	s.scan.paths[relative] = true
}

// driverGoRelative resolves a relative package pattern against cwd, keeping a
// trailing "/..." and naming the repository root's tree as "...".
func driverGoRelative(cwd, pattern string) string {
	recursive := pattern == "..." || strings.HasSuffix(pattern, "/...")
	base := strings.TrimSuffix(strings.TrimSuffix(pattern, "..."), "/")
	if base == "" {
		base = "."
	}
	joined := path.Clean(path.Join(cwd, base))
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return ""
	}
	if !recursive {
		if joined == "." {
			return ""
		}
		return joined
	}
	if joined == "." {
		return "..."
	}
	return joined + "/..."
}

var driverMakeValueFlags = map[string]bool{"-o": true, "-W": true, "-I": true, "-l": true, "--old-file": true, "--what-if": true, "--include-dir": true, "--load-average": true}

// makeInvocation records a `make` call: its directory, file and goals.
func (s *driverShellScanner) makeInvocation(words []driverWord, cwd string) {
	call := driverMakeCall{Dir: cwd}
	if cwd == "" {
		call.Unbounded = true
	}
	setDir := func(value string) {
		if driverDynamic(value) || call.Dir == "" {
			call.Unbounded = true
			call.Dir = ""
			return
		}
		call.Dir = driverJoin(call.Dir, value)
		if call.Dir == "" {
			call.Unbounded = true
		}
	}
	setFile := func(value string) {
		if driverDynamic(value) {
			call.Unbounded = true
			return
		}
		call.File = value
	}
	for index := 0; index < len(words); index++ {
		arg := words[index].text
		switch {
		case arg == "-C" || arg == "--directory":
			if index+1 < len(words) {
				index++
				setDir(words[index].text)
			}
		case strings.HasPrefix(arg, "--directory="):
			setDir(strings.TrimPrefix(arg, "--directory="))
		case strings.HasPrefix(arg, "-C") && !strings.HasPrefix(arg, "--"):
			setDir(arg[2:])
		case arg == "-f" || arg == "--file" || arg == "--makefile":
			if index+1 < len(words) {
				index++
				setFile(words[index].text)
			}
		case strings.HasPrefix(arg, "--file=") || strings.HasPrefix(arg, "--makefile="):
			_, value, _ := strings.Cut(arg, "=")
			setFile(value)
		case strings.HasPrefix(arg, "-f") && !strings.HasPrefix(arg, "--"):
			setFile(arg[2:])
		case arg == "-j" || arg == "--jobs":
			if index+1 < len(words) && driverNumeric(words[index+1].text) {
				index++
			}
		case driverMakeValueFlags[arg]:
			index++
		case strings.HasPrefix(arg, "-"):
		case driverAssignment(arg):
		case driverDynamic(arg):
			call.Computed = true
		default:
			call.Targets = append(call.Targets, arg)
		}
	}
	s.scan.makes = append(s.scan.makes, call)
}

func driverNumeric(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

// driverPathTokens is the path-shaped tokens of text, as the other plugins
// record string literals: a run of path characters with a slash or an
// extension, with any expansion treated as a separator.
func driverPathTokens(text string) []string {
	seen := map[string]bool{}
	tokens := make([]string, 0)
	isPathByte := func(char byte) bool {
		return isDriverNameByte(char) || strings.IndexByte("._/-+@~", char) >= 0
	}
	start := -1
	emit := func(end int) {
		if start < 0 {
			return
		}
		token := text[start:end]
		start = -1
		token = strings.TrimSuffix(token, "/...")
		for {
			trimmed := strings.TrimPrefix(token, "./")
			trimmed = strings.TrimRight(trimmed, "/.,:")
			if trimmed == token {
				break
			}
			token = trimmed
		}
		for strings.HasPrefix(token, "../") {
			token = strings.TrimPrefix(token, "../")
		}
		if token == "" || strings.HasPrefix(token, "-") || strings.HasPrefix(token, "/") || strings.HasPrefix(token, "@") || strings.HasPrefix(token, "~") {
			return
		}
		if !strings.Contains(token, "/") && !strings.Contains(token, ".") {
			return
		}
		if !ValidRelativePath(token) || seen[token] {
			return
		}
		seen[token] = true
		tokens = append(tokens, token)
	}
	for index := 0; index < len(text); index++ {
		char := text[index]
		if char == '$' && index+1 < len(text) {
			// $VAR/ and ${VAR}/ name a directory the token does not include.
			end := index + 1
			if text[end] == '{' {
				close := strings.IndexByte(text[end:], '}')
				if close > 0 {
					end += close + 1
				}
			} else {
				for end < len(text) && isDriverNameByte(text[end]) {
					end++
				}
			}
			emit(index)
			if end < len(text) && text[end] == '/' {
				index = end
			} else {
				index = end - 1
			}
			continue
		}
		if isPathByte(char) {
			if start < 0 {
				start = index
			}
			continue
		}
		emit(index)
	}
	emit(len(text))
	return tokens
}
