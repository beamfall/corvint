package affected

import (
	"path"
	"slices"
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
// the paths whose owners only `go vet` or `go list` reads and nothing runs
// (BuildPathImports, AFP-V0-2951), the make invocations, whether a command word
// is computed, and whether its reads are outside the graph (uncertified).
type driverScan struct {
	source      *Source
	computed    bool
	uncertified bool
	tokens      map[string]bool
	paths       map[string]bool
	buildPaths  map[string]bool
	makes       []driverMakeCall
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
	return &driverScan{tokens: map[string]bool{}, paths: map[string]bool{}, buildPaths: map[string]bool{}}
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
	// feeds counts the enclosing contexts whose standard output another
	// command consumes: a pipe, a command or process substitution, or make's
	// `$(shell ...)` (AFP-V0-2956).
	feeds int
	// fed counts the enclosing xargs runs, whose command takes arguments
	// read from standard input, and replaces holds their replacement strings
	// (AFP-V0-2957).
	fed      int
	replaces []string
}

// scanShell scans script, run with working directory cwd ("" when unknown),
// into scan. module is the main Go module path, "" when there is none.
func (scan *driverScan) scanShell(script, cwd, module string) {
	scanner := &driverShellScanner{scan: scan, module: module}
	scanner.script(script, cwd)
}

// scanShellOutput is scanShell for a script whose standard output the caller
// substitutes into other text, such as make's `$(shell ...)` (AFP-V0-2956).
func (scan *driverScan) scanShellOutput(script, cwd, module string) {
	scanner := &driverShellScanner{scan: scan, module: module, feeds: 1}
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
	var redirect, input, backtick bool
	// substitutions holds, per open parenthesis, whether it opened a
	// process substitution (`<(`, `>(`).
	var substitutions []bool
	// flush runs the pending command. Its standard output feeds another
	// command when a pipe follows it or a substitution encloses it
	// (AFP-V0-2956).
	flush := func(piped bool) {
		if len(words) != 0 {
			feeding := piped || backtick || slices.Contains(substitutions, true)
			if feeding {
				s.feeds++
			}
			cwd = s.command(words, cwd)
			if feeding {
				s.feeds--
			}
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
			case char == '|':
				// A pipe, `|` or `|&`, not either character of `||`.
				flush(!strings.HasPrefix(line[position:], "||") && (position == 0 || line[position-1] != '|'))
				position++
			case char == '(':
				flush(false)
				// After a redirection operator, `(` opens a process
				// substitution: no redirect target word follows.
				substitutions = append(substitutions, redirect)
				redirect = false
				position++
			case char == ')':
				flush(false)
				if len(substitutions) != 0 {
					substitutions = substitutions[:len(substitutions)-1]
				}
				position++
			case char == '`':
				flush(false)
				backtick = !backtick
				position++
			case char == ';':
				flush(false)
				position++
			case char == '&':
				if position+1 < len(line) && line[position+1] == '>' {
					redirect = true
					position += 2
					continue
				}
				flush(false)
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
				// `<` and `<>` open a file for reading; `<&` duplicates a
				// descriptor and `<<<` passes a string.
				input = char == '<' && !strings.HasPrefix(line[position:], "<<") && !strings.HasPrefix(line[position:], "<&")
				position++
				for position < len(line) && (line[position] == '>' || line[position] == '&' || line[position] == '|') {
					position++
				}
			default:
				word, next := s.word(line, position, cwd)
				position = next
				if redirect {
					// AFP-V0-3128: a file read through a redirection whose
					// name is unknown is outside every bounded read set.
					// AFP-V0-9051: so is one whose name no edge records, such
					// as an absolute, `~` or escaping path.
					if input && !s.driverOperandRecorded(word.text, cwd) {
						s.scan.uncertified = true
					}
					if input {
						s.pathOperand(word.text, cwd)
					}
					redirect, input = false, false
					continue
				}
				if !word.quoted && (word.text == "{" || word.text == "}") {
					flush(false)
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
		flush(false)
		redirect, input = false, false
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
	flush(false)
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
			s.feeds++
			s.script(line[position+1:position+1+end], cwd)
			s.feeds--
			text.WriteString(driverExpansion)
			position += end + 2
		case char == '$':
			next := position + 1
			switch {
			case next < len(line) && line[next] == '(':
				end := driverMatchingClose(line, next)
				inner := line[next+1 : max(next+1, end)]
				if !strings.HasPrefix(inner, "(") {
					s.feeds++
					s.script(inner, cwd)
					s.feeds--
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
	assignment := func(word string) {
		name, value, _ := strings.Cut(word, "=")
		// AFP-V0-3128: only the cache location and empty Go flags have
		// modeled effects here. Unknown environment names can inject startup
		// hooks or arbitrary tool reads, including through shell wrappers.
		switch name {
		case "GOCACHE":
			return
		case "GOFLAGS":
			if value == "" {
				return
			}
		}
		s.scan.computed = true
		s.scan.uncertified = true
	}
	for len(words) != 0 {
		head := words[0].text
		switch {
		case !words[0].quoted && driverAssignment(head):
			assignment(head)
			words = words[1:]
		case !words[0].quoted && head == "command" && len(words) > 1 && (words[1].text == "-v" || words[1].text == "-V"):
			// A lookup: it names a command and runs none.
			return cwd
		case !words[0].quoted && driverPrefixKeywords[head]:
			words = words[1:]
		case head == "env":
			words = words[1:]
			for len(words) != 0 && (strings.HasPrefix(words[0].text, "-") || driverAssignment(words[0].text)) {
				option := words[0].text
				if driverAssignment(option) {
					assignment(option)
				}
				// AZU-V0-9851: env changes only the wrapped command's cwd.
				// Until that scoped directory is modelled, keep the site computed.
				if strings.HasPrefix(option, "-C") || option == "--chdir" || strings.HasPrefix(option, "--chdir=") {
					s.scan.computed = true
				}
				if option == "-u" || option == "-C" || option == "--chdir" || option == "-S" {
					if len(words) < 2 {
						s.scan.computed = true
						return cwd
					}
					words = words[1:]
				}
				words = words[1:]
			}
		case head == "xargs":
			s.scan.uncertified = true // Operands come from its input (AFP-V0-3128).
			// The command xargs runs takes arguments read from standard
			// input, which the scan cannot see (AFP-V0-2957).
			rest, replace, ok := driverXargs(words[1:])
			if !ok {
				s.scan.computed = true
				return cwd
			}
			s.fed++
			defer func() { s.fed-- }()
			if replace != "" {
				s.replaces = append(s.replaces, replace)
				defer func() { s.replaces = s.replaces[:len(s.replaces)-1] }()
			}
			words = rest
		case head == "nice" || head == "timeout" || head == "ionice" || head == "taskset":
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

// driverXargsValues are the short xargs options (GNU and BSD) whose value is
// the rest of the word or, when that is empty, the next word, and
// driverXargsLongValues the long options that take a required value.
var (
	driverXargsValues     = map[byte]bool{'a': true, 'd': true, 'E': true, 'I': true, 'J': true, 'L': true, 'n': true, 'P': true, 's': true, 'R': true, 'S': true}
	driverXargsLongValues = map[string]bool{"--arg-file": true, "--delimiter": true, "--max-args": true, "--max-procs": true, "--max-chars": true, "--process-slot-var": true}
)

// driverXargs splits the options of an xargs invocation from the command it
// runs. replace is the replacement string `-I`, `-J`, `-i` or `--replace`
// names ("" without one); ok is false when an option lacks its value.
func driverXargs(words []driverWord) (rest []driverWord, replace string, ok bool) {
	for len(words) != 0 {
		option := words[0].text
		if option == "--" {
			return words[1:], replace, true
		}
		if !strings.HasPrefix(option, "-") || option == "-" {
			break
		}
		words = words[1:]
		if strings.HasPrefix(option, "--") {
			name, value, valued := strings.Cut(option, "=")
			if name == "--replace" {
				replace = "{}"
				if valued {
					replace = value
				}
			}
			if !valued && driverXargsLongValues[name] {
				if len(words) == 0 {
					return nil, "", false
				}
				words = words[1:]
			}
			continue
		}
		for index := 1; index < len(option); index++ {
			letter := option[index]
			if letter == 'i' {
				// -i[R]: R, by default {}, is the replacement.
				replace = "{}"
				if index+1 < len(option) {
					replace = option[index+1:]
				}
				break
			}
			if letter == 'e' || letter == 'l' {
				// -e[EOF] and -l[MAX] take only an attached value.
				break
			}
			if driverXargsValues[letter] {
				value := option[index+1:]
				if value == "" {
					if len(words) == 0 {
						return nil, "", false
					}
					value = words[0].text
					words = words[1:]
				}
				if letter == 'I' || letter == 'J' {
					replace = value
				}
				break
			}
		}
	}
	return words, replace, true
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
	case "export", "local", "readonly", "declare", "set", "unset", "read":
		// Environment mutations persist beyond this simple command. No
		// forms of these builtins have a modeled environment transition.
		// Retain the site even when the recipe has no make expansion.
		s.scan.computed = true
		s.scan.uncertified = true
		s.embedded(words[1:], cwd)
		return cwd
	case "trap":
		// AFP-V0-3128: the body runs later as shell code, potentially
		// with a different environment or directory.
		s.scan.uncertified = true
		s.embedded(words[1:], cwd)
		return cwd
	case "for", "case", "select", "function", "in", "esac", "[[", "]]", "[", "test", "echo", "printf", "true", "false", ":", "shift", "return", "exit", "wait":
		if (head == "[[" || head == "[" || head == "test") && !s.driverTestCertified(words, cwd) {
			s.scan.uncertified = true
		}
		if head == "[[" || head == "[" || head == "test" {
			for _, word := range words[1:] {
				s.pathOperand(word.text, cwd)
			}
		}
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
		s.scan.uncertified = true
		return cwd
	case ".", "source":
		s.scan.uncertified = true
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
	if !s.driverCertified(words, cwd) {
		s.scan.uncertified = true
	}
	for _, replace := range s.replaces {
		// The command xargs runs is named by what it reads (AFP-V0-2957).
		if strings.Contains(head, replace) {
			s.scan.computed = true
		}
	}
	base := path.Base(head)
	if driverInterpreters[base] {
		// operand reports a script operand or an inline program; without
		// one the interpreter reads its program from standard input: a
		// pipe, a heredoc whose body this scanner skips, or a redirect
		// (AFP-V0-2958).
		operand := false
		for index := 1; index < len(words); index++ {
			arg := words[index].text
			if driverShells[base] && strings.HasPrefix(arg, "-") && strings.HasSuffix(arg, "c") && !strings.HasPrefix(arg, "--") {
				if index+1 < len(words) {
					s.script(words[index+1].text, cwd)
				}
				operand = true
				break
			}
			if arg == "-c" || arg == "-e" || (!driverShells[base] && strings.HasPrefix(arg, "-")) {
				// Inline programs in other languages are outside the shell
				// scanner. Unsupported options may contain a program or
				// consume the next word; retain the site rather than guess
				// which word names a script.
				s.scan.computed = true
				operand = true
				break
			}
			if driverShells[base] && (arg == "-" || !strings.HasPrefix(arg, "--") && strings.HasPrefix(arg, "-") && strings.Contains(arg, "s")) {
				// `-s` and `-` read the program from standard input.
				break
			}
			if driverShells[base] && (arg == "-o" || arg == "+o") && index+1 < len(words) {
				index++
				continue
			}
			if strings.HasPrefix(arg, "-") || driverShells[base] && strings.HasPrefix(arg, "+") {
				if arg == "-m" && index+1 < len(words) {
					index++
				}
				continue
			}
			if driverComputedWord(arg) {
				s.scan.computed = true
			}
			s.pathWord(arg, cwd)
			if driverShells[base] && driverShellTest(arg) {
				// A certified test script's arguments are recorded as
				// file operands (AFP-V0-9051).
				for _, word := range words[index+1:] {
					s.pathOperand(word.text, cwd)
				}
			}
			operand = true
			break
		}
		// An interpreter xargs runs may take its script from what xargs
		// reads (AFP-V0-2957).
		if !operand || s.fed > 0 {
			s.scan.computed = true
		}
	}
	if driverBoundedCommands[base] || strings.Contains(head, "/") && driverShellTest(head) {
		// Every literal operand can name a file, including LICENSE, and
		// so can a test script's argument (AFP-V0-9051).
		for _, word := range words[1:] {
			s.pathOperand(word.text, cwd)
		}
	}
	s.pathWord(head, cwd)
	s.embedded(words, cwd)
	return cwd
}

// driverPureCommands read no file, whatever their operands (AFP-V0-3128).
var driverPureCommands = map[string]bool{
	"echo": true, "printf": true, "true": true, "false": true, "basename": true, "dirname": true,
	"sleep": true, "seq": true, "expr": true, "pwd": true, "uname": true, "id": true, "mktemp": true,
}

// driverBoundedCommands read only the operands they are given, which the
// unit's own path edges and tokens already name, once the option allowlist
// admits an exact option (AFP-V0-3122, AFP-V0-3128). awk, sed
// and the like are absent: their programs can read any file or run one.
var driverBoundedCommands = map[string]bool{
	"cat": true, "head": true, "tail": true, "wc": true, "sort": true, "uniq": true, "cut": true,
	"tr": true, "grep": true, "egrep": true, "fgrep": true, "cmp": true, "diff": true, "comm": true,
	"paste": true, "sha256sum": true, "sha1sum": true, "shasum": true, "md5sum": true, "cksum": true,
	"realpath": true, "readlink": true, "date": true, "mkdir": true, "rm": true, "rmdir": true,
	"cp": true, "mv": true, "ln": true, "touch": true, "chmod": true, "tee": true, "jq": true,
	"stat": true,
}

// driverCommandOptions admits exact options whose modes read only literal
// operands (AFP-V0-3128). Missing commands admit no options. In particular,
// long option abbreviations and checksum checking modes are never inferred.
type driverCommandOptions struct {
	flags, values string
}

var driverCommandOptionAllowlist = map[string]driverCommandOptions{
	"cat":       {flags: "-A -b -e -E -n -s -t -T -u -v --show-all --number --squeeze-blank"},
	"head":      {flags: "-q -v --quiet --verbose", values: "-c -n --bytes --lines"},
	"tail":      {flags: "-q -v --quiet --verbose", values: "-c -n --bytes --lines"},
	"wc":        {flags: "-c -m -l -L -w --bytes --chars --lines --max-line-length --words"},
	"sort":      {flags: "-b -d -f -g -h -i -M -n -r -s -u -V -z --numeric-sort --reverse --unique", values: "-k -o -S -t -T --key --output --buffer-size --field-separator --temporary-directory"},
	"uniq":      {flags: "-c -d -D -i -u -z --count --repeated --unique", values: "-f -s -w --skip-fields --skip-chars --check-chars"},
	"cut":       {flags: "-n -s -z --complement --only-delimited", values: "-b -c -d -f --bytes --characters --delimiter --fields --output-delimiter"},
	"tr":        {flags: "-c -C -d -s -t --complement --delete --squeeze-repeats --truncate-set1"},
	"grep":      {flags: "-E -F -G -P -i -v -w -x -z -c -l -L -n -H -h -o -q -s -a -I --count --files-with-matches --files-without-match --line-number --quiet", values: "-m -e -f -A -B -C --regexp --file --after-context --before-context --context --max-count"},
	"cmp":       {flags: "-b -l -s --print-bytes --verbose --silent --quiet", values: "-i -n --ignore-initial --bytes"},
	"diff":      {flags: "-q -s -u -c -b -w -B -i -a --brief --unified --context --ignore-space-change --ignore-all-space"},
	"comm":      {flags: "-1 -2 -3 -z --check-order --nocheck-order --zero-terminated", values: "--output-delimiter"},
	"paste":     {flags: "-s -z --serial --zero-terminated", values: "-d --delimiters"},
	"sha256sum": {flags: "-b -t -z --binary --text --zero"},
	"sha1sum":   {flags: "-b -t -z --binary --text --zero"},
	"md5sum":    {flags: "-b -t -z --binary --text --zero"},
	"shasum":    {flags: "-b -t -U --binary --text --UNIVERSAL", values: "-a --algorithm"},
	"cksum":     {flags: "-b -t -z --binary --text --zero", values: "-a --algorithm"},
	"stat":      {flags: "-f -L --file-system --dereference", values: "-c --format --printf"},
	"jq":        {flags: "-r -R -s -c -n -e -S -a -j -M -C --raw-output --raw-input --slurp --compact-output --null-input --exit-status --sort-keys --ascii-output --join-output --monochrome-output --color-output"},
}

// driverOperandRecorded reports a file operand, read in cwd, that pathOperand
// records as a repository edge: a relative literal path that stays inside the
// repository, or the bounded glob form. An absolute, `~` or escaping path, an
// empty word or any operand in an unknown directory has no edge, so a command
// that reads it must not certify (AFP-V0-9051). Dash-prefixed operands are
// not recorded, and linked paths do not cover their targets. Certification
// requires a source proving every path component is plain. /dev/null reads nothing.
func (s *driverShellScanner) driverOperandRecorded(word, cwd string) bool {
	switch {
	case word == "/dev/null":
		return true
	case s.scan.source == nil || cwd == "" || word == "" || driverDynamic(word) || driverRootedPath(word):
		return false
	case strings.HasPrefix(word, "-") && word != "-":
		return false
	case strings.ContainsAny(word, "*?["):
		word = driverGlobDirectory(word)
		if word == "" {
			return false
		}
	}
	relative := driverJoin(cwd, strings.TrimSuffix(word, "/"))
	return ValidRelativePath(relative) && driverPlainPath(s.scan.source, relative)
}

// driverRootedPath reports a word that names a path outside every repository
// edge whatever the working directory: absolute or `~`-prefixed.
func driverRootedPath(word string) bool {
	return strings.HasPrefix(word, "/") || strings.HasPrefix(word, "~")
}

// driverCommandOptionPaths are the option values that name a file the command
// reads or writes. Other values are counts, delimiters or formats.
var driverCommandOptionPaths = map[string]string{
	"grep": "-f --file",
	"sort": "-o -T --output --temporary-directory",
}

func (s *driverShellScanner) driverCommandOptionsCertified(base string, words []driverWord, cwd string) bool {
	// Check every word, including operands after --. The sole modeled glob
	// form has an explicit containing-directory read edge in pathOperand.
	if slices.ContainsFunc(words, func(word driverWord) bool {
		return strings.ContainsAny(word.text, "*?[") && driverGlobDirectory(word.text) == ""
	}) {
		return false
	}
	if base == "egrep" || base == "fgrep" {
		base = "grep"
	}
	options := driverCommandOptionAllowlist[base]
	has := func(list, option string) bool { return slices.Contains(strings.Fields(list), option) }
	// AFP-V0-9051: every file operand and file-naming option value must be
	// recorded. tr operands are sets, jq's first operand is its program, and
	// grep's first is its pattern unless -e or -f supplies one.
	var operands []string
	pattern := base == "grep" || base == "jq"
	value := func(option string, word string) bool {
		if base == "grep" && (option == "-e" || option == "--regexp" || option == "-f" || option == "--file") {
			pattern = false
		}
		return !has(driverCommandOptionPaths[base], option) || s.driverOperandRecorded(word, cwd)
	}
	for index := 0; index < len(words); index++ {
		arg := words[index].text
		if arg == "--" {
			for _, word := range words[index+1:] {
				operands = append(operands, word.text)
			}
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			operands = append(operands, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			name, _, attached := strings.Cut(arg, "=")
			if has(options.flags, name) && !attached {
				continue
			}
			if !has(options.values, name) {
				return false
			}
			// Attached values can name extensionless input files, but the
			// operand scanner skips the option word. Retain the frontier
			// claim until those inputs have explicit edges (AFP-V0-3128).
			if attached {
				return false
			}
			if !attached {
				index++
				if index >= len(words) || (strings.HasPrefix(words[index].text, "-") || strings.ContainsAny(words[index].text, "*?[")) || !value(name, words[index].text) {
					return false
				}
			}
			continue
		}
		for offset := 1; offset < len(arg); offset++ {
			option := "-" + arg[offset:offset+1]
			if has(options.values, option) {
				// Joined values hide paths from the operand scanner.
				if offset+1 != len(arg) {
					return false
				}
				index++
				if index >= len(words) || (strings.HasPrefix(words[index].text, "-") || strings.ContainsAny(words[index].text, "*?[")) || !value(option, words[index].text) {
					return false
				}
				break
			}
			if !has(options.flags, option) {
				return false
			}
		}
	}
	if base == "tr" {
		return true
	}
	if pattern && len(operands) > 0 {
		operands = operands[1:]
	}
	for _, operand := range operands {
		if operand != "-" && !s.driverOperandRecorded(operand, cwd) {
			return false
		}
	}
	return true
}

// driverGoCertified are the go subcommands whose reads the Go plugin models:
// the named packages' files and, for a test, the tests' own reads, which
// select their package (AFP-V0-3122). `go run` runs a program from the
// recipe's directory, whose reads no package model bounds.
var driverGoCertified = map[string]bool{"test": true, "vet": true, "build": true, "list": true, "install": true, "env": true, "version": true}

// driverGoValueFlags are the go flags whose value names no input: an output
// path, a test selection or a count. Every value must be bounded and literal
// before certification (AFP-V0-3128).
var driverGoValueFlags = map[string]bool{
	"o": true, "run": true, "skip": true, "count": true, "timeout": true, "parallel": true, "p": true,
	"bench": true, "benchtime": true, "cpu": true, "shuffle": true, "coverprofile": true, "outputdir": true,
	"cpuprofile": true, "memprofile": true, "blockprofile": true, "mutexprofile": true, "trace": true,
	"covermode": true, "fuzztime": true, "tags": true,
}

// driverCertified reports a simple command, run in cwd ("" when unknown),
// whose reads the graph already bounds (AFP-V0-3122, AFP-V0-3128): a command
// that reads no file; a bounded command in a known directory with only
// literal operands and recognized options; a go package command
// with recognized flags and literal values that hands the build no program; a make invocation of literal goals that overrides no
// variable, whose targets are units of their own; or a shell test script run
// with literal arguments, which its plugin selects for itself. Any other
// repository program, interpreter, git or unlisted command may read any
// path, such as every tracked file.
func (s *driverShellScanner) driverCertified(words []driverWord, cwd string) bool {
	head := words[0].text
	if driverDynamic(head) || words[0].quoted && head == "" {
		return false
	}
	base := path.Base(head)
	if strings.Contains(head, "/") && !strings.HasPrefix(head, "/") {
		return s.driverOperandRecorded(head, cwd) && driverShellTest(head) && s.driverTestArgumentsRecorded(words[1:], cwd)
	}
	switch {
	case driverPureCommands[base]:
		return true
	case base == "make" || base == "gmake":
		return driverMakeCommandCertified(words[1:])
	case base == "go":
		return driverGoCommandCertified(words[1:])
	case driverShells[base]:
		for index := 1; index < len(words); index++ {
			arg := words[index].text
			if strings.HasPrefix(arg, "-") && strings.HasSuffix(arg, "c") && !strings.HasPrefix(arg, "--") {
				// The command string is scanned as its own commands.
				return true
			}
			if !strings.HasPrefix(arg, "-") {
				return s.driverOperandRecorded(arg, cwd) && driverShellTest(arg) && s.driverTestArgumentsRecorded(words[index+1:], cwd)
			}
		}
		return false
	case driverBoundedCommands[base]:
		if cwd == "" || !driverLiteral(words[1:]) {
			return false
		}
		for _, word := range words[1:] {
			if base == "jq" && (strings.Contains(word.text, "import") || strings.Contains(word.text, "include")) {
				return false
			}
		}
		return s.driverCommandOptionsCertified(base, words[1:], cwd)
	}
	return false
}

// driverTestArgumentsRecorded reports shell test script arguments that are
// literal and, unless they are bare options, recorded file operands: the
// script may read any argument it is given (AFP-V0-9051).
func (s *driverShellScanner) driverTestArgumentsRecorded(words []driverWord, cwd string) bool {
	for _, word := range words {
		text := word.text
		if driverDynamic(text) {
			return false
		}
		if strings.HasPrefix(text, "-") && !strings.Contains(text, "=") || s.driverOperandRecorded(text, cwd) {
			continue
		}
		return false
	}
	return true
}

// driverMakeCommandCertified admits only options whose effects the make
// invocation scanner models. Evaluation, loading, search paths and unknown
// options may add arbitrary reads and must retain the site (AFP-V0-3128).
func driverMakeCommandCertified(words []driverWord) bool {
	if !driverLiteral(words) || slices.ContainsFunc(words, func(word driverWord) bool {
		return driverAssignment(word.text) || strings.ContainsAny(word.text, "*?[")
	}) {
		return false
	}
	flags := strings.Fields("-s --silent --quiet -n --just-print --dry-run --recon -k --keep-going -S --no-keep-going --stop -r --no-builtin-rules -R --no-builtin-variables -B --always-make -i --ignore-errors -w --print-directory --no-print-directory")
	for index := 0; index < len(words); index++ {
		arg := words[index].text
		switch {
		case !strings.HasPrefix(arg, "-"):
			// AFP-V0-9051: a rooted goal names a file no edge records.
			if driverRootedPath(arg) {
				return false
			}
		case slices.Contains(flags, arg):
		case arg == "-C" || arg == "--directory":
			index++
			if index >= len(words) || words[index].text == "" || strings.HasPrefix(words[index].text, "-") || driverRootedPath(words[index].text) {
				return false
			}
		case strings.HasPrefix(arg, "--directory="):
			if value := strings.TrimPrefix(arg, "--directory="); value == "" || driverRootedPath(value) {
				return false
			}
		case strings.HasPrefix(arg, "-C") && !strings.HasPrefix(arg, "--"):
			if driverRootedPath(arg[2:]) {
				return false
			}
		case arg == "-j" || arg == "--jobs":
			if index+1 < len(words) && driverNumeric(words[index+1].text) {
				index++
			}
		case strings.HasPrefix(arg, "--jobs="):
			if !driverNumeric(strings.TrimPrefix(arg, "--jobs=")) {
				return false
			}
		case strings.HasPrefix(arg, "-j"):
			if !driverNumeric(arg[2:]) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// driverLiteral reports words of which none holds a value only the
// environment or the shell supplies at run time.
func driverLiteral(words []driverWord) bool {
	return !slices.ContainsFunc(words, func(word driverWord) bool { return driverDynamic(word.text) })
}

// driverTestFileOperators are the test operators whose operand is a path.
var driverTestFileOperators = map[string]bool{
	"-a": true, "-b": true, "-c": true, "-d": true, "-e": true, "-f": true, "-g": true, "-h": true,
	"-k": true, "-p": true, "-r": true, "-s": true, "-u": true, "-w": true, "-x": true, "-G": true,
	"-L": true, "-N": true, "-O": true, "-S": true,
}

// driverTestCertified reports a test command that inspects only literal
// paths in a known directory (AFP-V0-3128): a file operator, or -nt, -ot or
// -ef, with an unknown operand inspects a path no edge names.
func (s *driverShellScanner) driverTestCertified(words []driverWord, cwd string) bool {
	for index := 1; index < len(words); index++ {
		word := words[index].text
		var operands []string
		switch {
		case driverTestFileOperators[word] && index+1 < len(words):
			operands = []string{words[index+1].text}
		case (word == "-nt" || word == "-ot" || word == "-ef") && index+1 < len(words):
			operands = []string{words[index-1].text, words[index+1].text}
		}
		for _, operand := range operands {
			// AFP-V0-9051: an absolute, `~` or escaping operand has no edge.
			if !s.driverOperandRecorded(operand, cwd) {
				return false
			}
		}
	}
	return true
}

// driverGoCommandCertified admits only recognized flags with literal values.
// An unbounded value can split into executable flags, including -toolexec,
// even when the flag that precedes it only names an output (AFP-V0-3128).
func driverGoCommandCertified(words []driverWord) bool {
	if !driverLiteral(words) || slices.ContainsFunc(words, func(word driverWord) bool {
		return strings.ContainsAny(word.text, "*?[")
	}) {
		return false
	}
	for len(words) > 0 && (words[0].text == "-C" || strings.HasPrefix(words[0].text, "-C=")) {
		if words[0].text == "-C" {
			if len(words) < 2 || strings.HasPrefix(words[1].text, "-") || driverRootedPath(words[1].text) {
				return false
			}
			words = words[1:]
		} else if driverRootedPath(strings.TrimPrefix(words[0].text, "-C=")) {
			return false
		}
		words = words[1:]
	}
	if len(words) == 0 || !driverGoCertified[words[0].text] {
		return false
	}
	for index := 1; index < len(words); index++ {
		arg := words[index].text
		if arg == "-args" && words[0].text == "test" {
			// All remaining test arguments were checked literal above. The
			// test binary may read any path they name from its package
			// directory; only names without a directory stay there
			// (AFP-V0-9051).
			return !slices.ContainsFunc(words[index+1:], func(word driverWord) bool {
				return strings.Contains(word.text, "/") || strings.Contains(word.text, "~")
			})
		}
		if !strings.HasPrefix(arg, "-") {
			// AFP-V0-9051: a rooted package pattern records no edge.
			if driverRootedPath(arg) {
				return false
			}
			continue
		}
		flag, value, attached := strings.Cut(arg, "=")
		if driverGoBoolFlags[flag] {
			if attached && value != "true" && value != "false" && !(flag == "-buildvcs" && value == "auto") {
				return false
			}
			continue
		}
		if !driverGoValueFlags[strings.TrimPrefix(flag, "-")] || strings.HasPrefix(flag, "--") {
			return false
		}
		if !attached {
			index++
			if index >= len(words) || strings.HasPrefix(words[index].text, "-") {
				return false
			}
		}
	}
	return true
}

// driverShellTest reports a repository shell test script by the shell
// plugin's own naming rule (a `_test`, `-test`, `test_` or `test-` stem, or
// `.bats`) and suffix: the plugin selects that test for itself, so a target
// that runs it reads no more than the test does.
func driverShellTest(word string) bool {
	if driverDynamic(word) || strings.HasPrefix(word, "/") {
		return false
	}
	name := path.Base(word)
	if !slices.ContainsFunc(ShellSuffixes, func(suffix string) bool { return strings.HasSuffix(name, suffix) }) {
		return false
	}
	if strings.HasSuffix(name, ".bats") {
		return true
	}
	stem := strings.TrimSuffix(name, path.Ext(name))
	return strings.HasSuffix(stem, "_test") || strings.HasSuffix(stem, "-test") || strings.HasPrefix(stem, "test_") || strings.HasPrefix(stem, "test-")
}

// embedded finds a go or make invocation anywhere in a command, such as one
// a wrapper script runs, and records every path-like argument, except a
// package pattern a build-only go command already recorded as a build path
// import (AFP-V0-2951). After a feeder (xargs, parallel, or find's -exec,
// -execdir, -ok or -okdir) the command takes arguments the scan cannot see:
// a go or make invocation is fed, an interpreter is a computed invocation,
// and so is a command word holding the replacement string `{}` (or, after
// parallel, any `{`) (AFP-V0-2957).
func (s *driverShellScanner) embedded(words []driverWord, cwd string) {
	built := map[int]bool{}
	fed := s.fed > 0
	parallel := false
	for index, word := range words {
		base := path.Base(word.text)
		switch base {
		case "go":
			if index+1 < len(words) && !driverDynamic(word.text) {
				for pattern := range s.goInvocation(words[index+1:], cwd, fed) {
					built[index+1+pattern] = true
				}
			}
		case "make", "gmake":
			s.makeInvocation(words[index+1:], cwd, fed)
		}
		if fed && driverInterpreters[base] || parallel && strings.Contains(word.text, "{") {
			s.scan.computed = true
		}
		switch {
		case base == "parallel":
			fed, parallel = true, true
		case base == "xargs" || word.text == "-exec" || word.text == "-execdir" || word.text == "-ok" || word.text == "-okdir":
			fed = true
			if index+1 < len(words) && strings.Contains(words[index+1].text, "{}") {
				s.scan.computed = true
			}
		}
		if index > 0 && !built[index] {
			s.pathWord(word.text, cwd)
		}
	}
}

// pathWord records a path-like word as a path import: the owner of the file,
// or of the files directly in the directory, that it names.
func (s *driverShellScanner) pathWord(word, cwd string) {
	if !strings.Contains(word, "/") && !strings.Contains(word, ".") {
		return
	}
	s.pathOperand(word, cwd)
}

// driverGlobDirectory recognizes only literal-dir/*literal-suffix. Its entire
// read set is covered by a directory edge; every other glob remains unproven.
func driverGlobDirectory(word string) string {
	slash := strings.LastIndexByte(word, '/')
	if slash <= 0 || slash+1 >= len(word) || word[slash+1] != '*' {
		return ""
	}
	for index, char := range word {
		if index == slash+1 {
			continue
		}
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("/._-+", char)) {
			return ""
		}
	}
	directory := path.Clean(word[:slash])
	if directory == "." || strings.HasPrefix(directory, "-") || !ValidRelativePath(directory) {
		return ""
	}
	return directory
}

// pathOperand records literal file operands and the sole bounded glob form.
func (s *driverShellScanner) pathOperand(word, cwd string) {
	if strings.ContainsAny(word, "*?[") {
		if directory := driverGlobDirectory(word); directory != "" && cwd != "" {
			s.pathOperand(directory, cwd)
			for _, candidate := range driverVariants(cwd, directory) {
				if ValidRelativePath(candidate) {
					// A lone unanchored token matches filenames only. Mark
					// the directory explicitly so extensionless dirs cover
					// existing and newly added descendants alike.
					s.scan.tokens["/"+candidate+"/"] = true
				}
			}
		} else {
			s.scan.uncertified = true
		}
		return
	}
	if driverDynamic(word) || strings.HasPrefix(word, "-") || strings.HasPrefix(word, "/") {
		return
	}
	word = strings.TrimSuffix(word, "/")
	for _, candidate := range driverVariants(cwd, word) {
		if ValidRelativePath(candidate) {
			s.scan.paths[candidate] = true
			// PathImports reach owners; tokens also reach unowned files.
			s.scan.tokens[candidate] = true
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

// driverGoBuildOnlyCommands are the package commands that read the packages
// they name without running any of their code or leaving a binary another
// step could run (AFP-V0-2951). `go build` and `go install` are not among
// them: a later step, or a target that depends on this one, may run what they
// build.
var driverGoBuildOnlyCommands = map[string]bool{"vet": true, "list": true}

// goInvocation records the package patterns of `go <command> ...` as path
// imports, and those of a build-only command as build path imports. A
// computed pattern, or a relative one under an unknown working directory,
// imports every owner, and so does a fed invocation, whose further arguments
// a feeder supplies (AFP-V0-2957). A build-only command whose standard output
// feeds another command is no build-only command: the consumer may run what
// it lists (AFP-V0-2956). It returns the indexes, into words, of the patterns
// a build-only command recorded, so the caller does not also record them as
// paths the command runs; every other word stays the caller's.
func (s *driverShellScanner) goInvocation(words []driverWord, cwd string, fed bool) map[int]bool {
	offset := 0
	for len(words) > 0 && (words[0].text == "-C" || strings.HasPrefix(words[0].text, "-C=")) {
		var directory string
		switch {
		case words[0].text == "-C" && len(words) >= 2:
			directory = words[1].text
			words = words[2:]
			offset += 2
		case strings.HasPrefix(words[0].text, "-C="):
			directory = strings.TrimPrefix(words[0].text, "-C=")
			words = words[1:]
			offset++
		default:
			s.scan.computed = true
			return nil
		}
		if driverDynamic(directory) || cwd == "" {
			cwd = ""
		} else {
			cwd = driverJoin(cwd, directory)
		}
	}
	if len(words) == 0 || !driverGoPackageCommands[words[0].text] {
		return nil
	}
	command := words[0].text
	words = words[1:]
	offset++
	into := s.scan.paths
	pathOutput := command != "list" || driverListPathOutput(words)
	if !pathOutput {
		// Uncertified output can name any execution target, not just the
		// packages being listed, even when buffered to a named script.
		// AFP-V0-2956: retain every owner unless the output is proven path data.
		into["..."] = true
		// Its targets may also carry runtime reads that do not propagate
		// through package edges, or live outside the scanned owner graph.
		s.scan.computed = true
	}
	buildOnly := driverGoBuildOnlyCommands[command] && s.feeds == 0 && pathOutput
	if buildOnly {
		into = s.scan.buildPaths
	}
	if fed {
		into["..."] = true
	}
	built := map[int]bool{}
	hasPackage := false
	for index := 0; index < len(words); index++ {
		arg := words[index].text
		if arg == "-args" || arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			if arg == "-C" || strings.HasPrefix(arg, "-C=") {
				directory := strings.TrimPrefix(arg, "-C=")
				if arg == "-C" {
					if index+1 == len(words) {
						s.scan.computed = true
						return built
					}
					index++
					directory = words[index].text
				}
				if driverDynamic(directory) || cwd == "" {
					cwd = ""
				} else {
					cwd = driverJoin(cwd, directory)
				}
				continue
			}
			if !strings.Contains(arg, "=") && !driverGoBoolFlags[strings.TrimPrefix(arg, "-")] && !driverGoBoolFlags[arg] {
				index++
			}
			continue
		}
		hasPackage = true
		if s.goPattern(arg, cwd, into) && buildOnly {
			built[offset+index] = true
		}
		if command == "run" && !strings.HasSuffix(arg, ".go") {
			return built
		}
	}
	if !hasPackage && command != "run" {
		// Package commands other than run default to the current package.
		// goPattern keeps every owner when cd or -C left cwd unresolved.
		s.goPattern(".", cwd, into)
	}
	return built
}

// goPattern records one package pattern in into: the scan's paths, or its
// build paths for a build-only command. It reports whether it recorded one; a
// glob, a standard-library or external pattern records none.
func (s *driverShellScanner) goPattern(pattern, cwd string, into map[string]bool) bool {
	switch {
	case strings.ContainsAny(pattern, "*?["):
		// Shell globs are not literal package paths. Retain the caller
		// rather than silently importing a path that has no owner.
		s.scan.computed = true
		return false
	case driverDynamic(pattern):
		into["..."] = true
		return true
	case pattern == "all":
		into["..."] = true
		return true
	case pattern == "std" || pattern == "cmd":
		return false
	}
	pattern, _, _ = strings.Cut(pattern, "@")
	relative := ""
	switch {
	case pattern == "." || pattern == ".." || strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, "../"):
		if cwd == "" {
			into["..."] = true
			return true
		}
		relative = driverGoRelative(cwd, pattern)
	case s.module != "" && (pattern == s.module || pattern == s.module+"/..."):
		into["..."] = true
		return true
	case s.module != "" && strings.HasPrefix(pattern, s.module+"/"):
		relative = strings.TrimPrefix(pattern, s.module+"/")
	case strings.HasSuffix(pattern, ".go"):
		if cwd == "" {
			into["..."] = true
			return true
		}
		relative = driverJoin(cwd, pattern)
	default:
		return false
	}
	if relative == "" {
		into["..."] = true
		return true
	}
	into[relative] = true
	return true
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

// makeInvocation records a `make` call: its directory, file and goals. A fed
// call, whose further goals a feeder supplies, is computed (AFP-V0-2957).
func (s *driverShellScanner) makeInvocation(words []driverWord, cwd string, fed bool) {
	call := driverMakeCall{Dir: cwd, Computed: fed}
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

// driverListPathOutput certifies the closed set of listing formats this
// scanner understands as path data. Arbitrary templates can print programs
// consumed later from a file without any pipe or expansion (AFP-V0-2956).
func driverListPathOutput(words []driverWord) bool {
	for index := 0; index < len(words); index++ {
		arg := words[index].text
		if arg == "-args" || arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		flag, value, inline := strings.Cut(strings.TrimPrefix(arg, "-"), "=")
		flag = strings.TrimPrefix(flag, "-")
		switch flag {
		case "f", "C", "tags", "mod", "modfile", "overlay", "compiler", "gcflags", "asmflags", "ldflags", "pkgdir":
			if !inline {
				index++
				if index == len(words) {
					return false
				}
				value = words[index].text
			}
			if flag == "f" && value != "{{.Dir}}" && value != "{{.ImportPath}}" {
				return false
			}
		case "e", "deps", "test", "find", "race", "msan", "asan", "trimpath", "modcacherw", "buildvcs":
			// These known switches do not replace the path output format.
		default:
			// New options and output encodings are not certified implicitly.
			return false
		}
	}
	return true
}
