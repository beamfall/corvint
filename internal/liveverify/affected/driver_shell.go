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
	// driverRuntimeComputed marks a workflow expression whose value another
	// step computes at run time, such as `steps.x.outputs.y` written from a
	// command's output: two markers never form the root-like `$X/` prefix
	// driverComputedWord admits, so a command word built from it stays
	// computed (AFP-V0-9132).
	driverRuntimeComputed = driverComputed + driverComputed
)

// driverScan is what the shell scanner found in recipe or step text: the
// literal path tokens, the paths whose owners the text runs (PathImports),
// the paths whose owners only `go vet` or `go list` reads and nothing runs
// (BuildPathImports, AFP-V0-2951), the make invocations, whether a command word
// is computed, and whether its reads are outside the graph (uncertified).
type driverScan struct {
	source   *Source
	computed bool
	// runOpen marks text whose run set did not resolve: every computed
	// reason but envPrefix's sets it (NCM-V1-017).
	runOpen     bool
	uncertified bool
	// uncertifiedBy names why uncertified was set (AZU-V0-611): the
	// command, operand or form the scan could not bound. Diagnostic only.
	uncertifiedBy map[string]bool
	readsTree     bool
	tokens        map[string]bool
	paths         map[string]bool
	buildPaths    map[string]bool
	makes         []driverMakeCall
	// reasons names why computed was set (AFP-V0-9620). It is diagnostic
	// only: no unit field, wire contract or selection reads it.
	reasons map[string]bool
	// envKnown reports that envNames, lower-cased, holds every name the
	// caller's own configuration exports to the script (AFP-V0-9623). Without
	// it any name may already be exported, so no assignment is narrowed.
	envKnown bool
	envNames map[string]bool
	// Narrowing is provisional until the entire script has been parsed.
	// locals and commandRefs retain the parsed-reference soundness floor,
	// even in opaque scripts where no form can be narrowed. references also
	// includes operands and redirects: a local value may escape through them.
	narrowed    map[string]bool
	locals      map[string]bool
	commandRefs map[string]bool
	references  map[string]bool
	revoked     string
	// functions holds the names the script defines functions for. A function
	// named as a command the scanner models shadows it (AFP-V0-9130); an
	// unknown command word is already trusted for nothing.
	functions map[string]bool
	// strict marks the AFP-V0-3600 scan of a recipe expanded with every make
	// value unknown, and leaks each word outside a GOCACHE value that holds
	// one: a non-empty leaks refuses that proof.
	strict bool
	leaks  map[string]bool
}

// markComputed marks the scanned text a computed invocation for reason.
func (scan *driverScan) markComputed(reason string) {
	scan.runOpen = true
	scan.envPrefix(reason)
}

// unfollowed leaves the scanned text's run set open without making it a
// computed site: it runs a command whose own execution the scan does not
// follow, so what that command runs in turn is unknown (NCM-V1-019). Only a
// make edit's reach reads it; the planner's graph is unchanged.
func (scan *driverScan) unfollowed() { scan.runOpen = true }

// driverFollowed reports a simple command whose execution the scan follows
// completely (NCM-V1-019): a pure or bounded command, which runs no other
// program; make, whose goals are the unit's edges; a go command other than
// generate or tool, with no -exec, -toolexec or -vettool program, which runs
// only the packages it names; `sh -c` or `bash -c` and the like, whose
// inline script is scanned as its own commands; and find with no -exec,
// -execdir, -ok, -okdir or -delete, which runs nothing. A path, an interpreter given
// a file, and every other program are not followed: a script or binary may
// run tests it does not name.
func driverFollowed(words []driverWord) bool {
	head := words[0].text
	if strings.Contains(head, "/") {
		return false
	}
	base := path.Base(head)
	switch {
	case driverPureCommands[base], driverBoundedCommands[base], base == "make", base == "gmake":
		return true
	case base == "go":
		subcommand := ""
		for _, word := range words[1:] {
			text := word.text
			if strings.HasPrefix(text, "-exec") || strings.HasPrefix(text, "-toolexec") || strings.HasPrefix(text, "-vettool") {
				return false
			}
			if subcommand == "" && !strings.HasPrefix(text, "-") {
				subcommand = text
			}
		}
		return subcommand != "generate" && subcommand != "tool"
	case driverShells[base]:
		return len(words) > 2 && words[1].text == "-c"
	case base == "find":
		return !slices.ContainsFunc(words[1:], func(word driverWord) bool {
			return strings.HasPrefix(word.text, "-exec") || strings.HasPrefix(word.text, "-ok") || word.text == "-delete" || driverDynamic(word.text)
		})
	}
	return false
}

// envPrefix marks the scanned text a computed invocation for reason without
// leaving its run set open: the reason is a literal environment assignment
// in front of a command word the scan resolves (NCM-V1-017). The planner
// still reads the text as a computed site (AFP-V0-3128); only a make edit's
// reach, which bounds what a target runs, tells the two apart.
func (scan *driverScan) envPrefix(reason string) {
	scan.computed = true
	if scan.reasons == nil {
		scan.reasons = map[string]bool{}
	}
	scan.reasons[reason] = true
}

// uncertify marks the scanned text's reads outside the graph for reason,
// which names the blocking command word, operand or form (AZU-V0-611).
func (scan *driverScan) uncertify(reason string) {
	scan.uncertified = true
	if scan.uncertifiedBy == nil {
		scan.uncertifiedBy = map[string]bool{}
	}
	scan.uncertifiedBy[reason] = true
}

// driverReasonWord spells a shell word for a diagnostic reason: each
// expansion the scan replaced is written `$?`.
func driverReasonWord(word string) string {
	return strings.ReplaceAll(word, driverExpansion, "$?")
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
// expansion replaced by driverExpansion, quoting/escaping, and parsed
// parameter references ("*" for an indirect or arithmetic reference).
type driverWord struct {
	text    string
	quoted  bool
	escaped bool
	refs    []string
}

// driverShellScanner splits shell text into simple-command segments.
type driverShellScanner struct {
	scan     *driverScan
	module   string
	depth    int
	heredocs []driverHeredoc
	// feeds counts the enclosing contexts whose standard output another
	// command consumes: a pipe, a command or process substitution, or make's
	// `$(shell ...)` (AFP-V0-2956).
	feeds int
	// fed counts the enclosing xargs runs, whose command takes arguments
	// read from standard input, and replaces holds their replacement strings
	// (AFP-V0-2957).
	fed      int
	replaces []string
	// opaque counts the enclosing inline scripts of a shell other than sh or
	// bash, whose special parameters differ: no assignment is narrowed there.
	opaque int
}

type driverHeredoc struct {
	delimiter string
	stripTabs bool // Only <<- permits leading tabs on the terminator.
}

func (doc driverHeredoc) terminates(line string) bool {
	if doc.stripTabs {
		line = strings.TrimLeft(line, "\t")
	}
	return line == doc.delimiter
}

// scanShell scans script, run with working directory cwd ("" when unknown),
// into scan. module is the main Go module path, "" when there is none.
func (scan *driverScan) scanShell(script, cwd, module string) {
	scanner := &driverShellScanner{scan: scan, module: module}
	scanner.script(script, cwd)
	scan.settleNarrowing()
}

// scanShellOutput is scanShell for a script whose standard output the caller
// substitutes into other text, such as make's `$(shell ...)` (AFP-V0-2956).
func (scan *driverScan) scanShellOutput(script, cwd, module string) {
	scanner := &driverShellScanner{scan: scan, module: module, feeds: 1}
	scanner.script(script, cwd)
	scan.settleNarrowing()
}

// narrow records a form left uncomputed that would have marked reason.
func (scan *driverScan) narrow(reason string) {
	if scan.narrowed == nil {
		scan.narrowed = map[string]bool{}
	}
	scan.narrowed[reason] = true
}

// local records a parsed declaration independently of narrowing eligibility.
func (scan *driverScan) local(name string) {
	if scan.locals == nil {
		scan.locals = map[string]bool{}
	}
	scan.locals[name] = true
}

// commandWord retains references in code-bearing words, including words in
// opaque scripts. A locally assigned command prefix is never a root locator.
func (scan *driverScan) commandWord(word driverWord) {
	if scan.commandRefs == nil {
		scan.commandRefs = map[string]bool{}
	}
	for _, name := range word.refs {
		scan.commandRefs[name] = true
	}
}

func (scan *driverScan) referenceNames(names []string) {
	if scan.references == nil {
		scan.references = map[string]bool{}
	}
	for _, name := range names {
		scan.references[name] = true
		switch strings.ToLower(name) {
		case "github_env", "github_path":
			scan.markComputed("shell:github-env-file")
		case "github_output":
			scan.revoke("shell:github-output")
		}
	}
}

// revoke records why no narrowing in the script holds.
func (scan *driverScan) revoke(reason string) {
	if scan.revoked == "" {
		scan.revoked = reason
	}
}

// defineFunction records a function definition of name.
func (scan *driverScan) defineFunction(name string) {
	if scan.functions == nil {
		scan.functions = map[string]bool{}
	}
	scan.functions[name] = true
}

// driverModelledCommand reports a command name whose semantics the scanner
// relies on: a function of that name may run anything instead
// (AFP-V0-9130).
func driverModelledCommand(name string) bool {
	switch name {
	case "go", "make", "gmake", "git", "cd", "pushd", "eval", "source", ".", "test", "[", "[[",
		"find", "rg", "xargs", "parallel", "env", "nice", "timeout", "ionice", "taskset",
		"read", "set", "export", "local", "declare", "unset", "shift", "trap", "alias", "exec",
		"command", "builtin", "nohup", "sudo", "stdbuf", "echo", "printf", "true", "false", ":":
		return true
	}
	return driverPureCommands[name] || driverBoundedCommands[name] || driverInterpreters[name]
}

// settleNarrowing preserves parsed local/command tracking before considering
// narrowing. Only a wholly understood script with unused locals can discard
// a computed reason; raw-text occurrence counts provide no authority here.
func (scan *driverScan) settleNarrowing() {
	for name := range scan.functions {
		if driverModelledCommand(name) {
			scan.markComputed("shell:function-shadow")
		}
	}
	for name := range scan.locals {
		if scan.commandRefs[name] || scan.commandRefs["*"] {
			scan.markComputed("shell:local-command-word")
		}
		if scan.references[name] || scan.references["*"] {
			scan.revoke("shell:local-referenced")
			scan.markComputed("shell:local-referenced")
		}
	}
	if scan.revoked != "" && len(scan.narrowed) != 0 {
		scan.markComputed(scan.revoked)
		for reason := range scan.narrowed {
			scan.markComputed(reason)
		}
	}
}

func (s *driverShellScanner) script(script, cwd string) {
	// exact reports that every backslash-newline the shell removes was
	// joined and no other one was (AFP-V0-9130); a line that still ends in a
	// backslash is then complete.
	exact := true
	if strings.Contains(script, "\\\n") {
		joined, ok := driverJoinContinuations(script)
		if !ok {
			// Join before recognizing keywords and names, but do not claim
			// this line-oriented scanner understands every continued shell
			// construct.
			s.scan.revoke("shell:line-continuation")
			joined = strings.ReplaceAll(script, "\\\n", "")
		}
		exact = ok
		script = joined
	}
	if s.depth > 8 {
		s.scan.markComputed("shell:depth")
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
				s.scan.revoke("shell:unsupported-syntax")
				// A pipe, `|` or `|&`, not either character of `||`.
				flush(!strings.HasPrefix(line[position:], "||") && (position == 0 || line[position-1] != '|'))
				position++
			case char == '(':
				if len(words) != 0 && strings.HasPrefix(strings.TrimSpace(line[position+1:]), ")") {
					s.scan.revoke("shell:function-definition")
					s.scan.defineFunction(words[len(words)-1].text)
				} else {
					s.scan.revoke("shell:unsupported-syntax")
				}
				flush(false)
				// After a redirection operator, `(` opens a process
				// substitution: no redirect target word follows.
				substitutions = append(substitutions, redirect)
				redirect = false
				position++
			case char == ')':
				s.scan.revoke("shell:unsupported-syntax")
				flush(false)
				if len(substitutions) != 0 {
					substitutions = substitutions[:len(substitutions)-1]
				}
				position++
			case char == '`':
				s.scan.revoke("shell:unsupported-syntax")
				flush(false)
				backtick = !backtick
				position++
			case char == ';':
				if strings.HasPrefix(line[position:], ";;") || strings.HasPrefix(line[position:], ";&") {
					s.scan.revoke("shell:unsupported-syntax")
				}
				flush(false)
				position++
			case char == '&':
				s.scan.revoke("shell:unsupported-syntax")
				if position+1 < len(line) && line[position+1] == '>' {
					redirect = true
					position += 2
					continue
				}
				flush(false)
				position++
			case char == '<' && strings.HasPrefix(line[position:], "<<") && !strings.HasPrefix(line[position:], "<<<"):
				// AFP-V0-9400: a wrapper may hand this body to an
				// interpreter, or save its output for later execution. The
				// scanner skips bodies, so it cannot certify their effects,
				// even when the first command word is not an interpreter.
				s.scan.markComputed("shell:heredoc-body")
				position += 2
				stripTabs := position < len(line) && line[position] == '-'
				if stripTabs {
					position++
				}
				for position < len(line) && (line[position] == ' ' || line[position] == '\t') {
					position++
				}
				word, next := s.word(line, position, cwd)
				position = next
				if word.escaped {
					// A backslash in a delimiter decides which line ends the
					// body; fail closed rather than trust that one decoding.
					s.scan.revoke("shell:escaped-heredoc-delimiter")
				}
				if word.text != "" {
					s.heredocs = append(s.heredocs, driverHeredoc{delimiter: word.text, stripTabs: stripTabs})
				} else {
					s.scan.revoke("shell:incomplete-heredoc")
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
					if s.scan.strict && strings.Contains(word.text, driverComputed) {
						// AFP-V0-3600: a redirection target is never a cache value.
						s.scan.leak(word.text)
					}
					// AFP-V0-3128: a file read through a redirection whose
					// name is unknown is outside every bounded read set.
					// AFP-V0-9051: so is one whose name no edge records, such
					// as an absolute, `~` or escaping path.
					if input && !s.driverOperandRecorded(word.text, cwd) {
						s.scan.uncertify("shell:redirect-input:" + driverReasonWord(word.text))
					}
					if input {
						s.pathOperand(word.text, cwd)
					}
					redirect, input = false, false
					continue
				}
				if !word.quoted && (word.text == "{" || word.text == "}") {
					flush(false)
					s.scan.revoke("shell:unsupported-syntax")
					continue
				}
				if word.text != "" || word.quoted {
					words = append(words, word)
				}
			}
		}
		if !exact && strings.HasSuffix(line, "\\") {
			continue
		}
		flush(false)
		redirect, input = false, false
		for len(s.heredocs) != 0 && index+1 < len(lines) {
			doc := s.heredocs[0]
			s.heredocs = s.heredocs[1:]
			for index+1 < len(lines) {
				index++
				if doc.terminates(lines[index]) {
					break
				}
				body := lines[index]
				if driverSubstitutes(body) || strings.Contains(body, "${") || strings.Contains(body, "$[") {
					// A skipped body can execute code, evaluate arithmetic or
					// assign a parameter.
					// Quoted delimiters deliberately get the same fallback.
					s.scan.markComputed("shell:heredoc-substitution")
					s.scan.revoke("shell:heredoc-substitution")
					s.scan.commandWord(driverWord{refs: []string{"*"}})
				}
				s.scan.referenceNames(driverReferencedNames(body))
			}
			if !doc.terminates(lines[index]) {
				s.scan.revoke("shell:incomplete-heredoc")
			}
		}
		if len(s.heredocs) != 0 {
			s.scan.revoke("shell:incomplete-heredoc")
		}
		s.heredocs = nil
	}
	flush(false)
	if backtick || len(substitutions) != 0 {
		s.scan.revoke("shell:unsupported-syntax")
	}
}

// driverJoinContinuations removes each backslash-newline the shell removes,
// so a `$(...)` or a command continued over several lines is scanned as one
// line (AFP-V0-9130): outside single quotes and comments, inside double quotes
// or not, and not itself escaped. It reports false, with the text unchanged,
// for text whose quoting it does not follow exactly: a backtick, `$'` or `$"`
// outside single quotes, a here-document operator, or a quote or `$(` left
// open at the end.
func driverJoinContinuations(script string) (string, bool) {
	var out strings.Builder
	// quote is the enclosing quote, 0 outside quotes; nesting holds, for each
	// enclosing `(`, `$(` or `${`, the quote state outside it with its closer.
	quote := byte(0)
	type level struct {
		quote  byte
		closer byte
	}
	var nesting []level
	comment := false
	for index := 0; index < len(script); index++ {
		char := script[index]
		switch {
		case comment:
			if char == '\n' {
				comment = false
			}
		case quote != '\'' && (char == '`' || char == '$' && index+1 < len(script) && (script[index+1] == '\'' || script[index+1] == '"')):
			// A backtick substitution, or `$'` and `$"` quoting, is not
			// followed; inside double quotes `$'` is literal but `$"` closes
			// the quote, which the lexer does not model either.
			return script, false
		case quote == 0 && char == '<' && strings.HasPrefix(script[index:], "<<") && !strings.HasPrefix(script[index:], "<<<"):
			// A here-document body keeps or removes backslash-newlines by
			// its delimiter's quoting, which this pass does not read.
			return script, false
		case char == '\\' && quote != '\'':
			if index+1 < len(script) && script[index+1] == '\n' {
				index++
				continue
			}
			if index+1 < len(script) {
				out.WriteByte(char)
				index++
				char = script[index]
			}
		case char == '\'' && quote == 0:
			quote = '\''
		case char == '\'' && quote == '\'':
			quote = 0
		case char == '"' && quote == 0:
			quote = '"'
		case char == '"' && quote == '"':
			quote = 0
		case quote == '\'':
		case char == '#' && quote == 0 && (index == 0 || strings.IndexByte(" \t\n;&|()<>", script[index-1]) >= 0):
			comment = true
		case char == '$' && index+1 < len(script) && (script[index+1] == '(' || script[index+1] == '{'):
			closer := byte(')')
			if script[index+1] == '{' {
				closer = '}'
			}
			nesting = append(nesting, level{quote: quote, closer: closer})
			quote = 0
			out.WriteByte(char)
			index++
			char = script[index]
		case char == '(' && quote == 0:
			nesting = append(nesting, level{closer: ')'})
		case (char == ')' || char == '}') && quote == 0 && len(nesting) != 0 && nesting[len(nesting)-1].closer == char:
			quote = nesting[len(nesting)-1].quote
			nesting = nesting[:len(nesting)-1]
		}
		out.WriteByte(char)
	}
	if quote != 0 || len(nesting) != 0 {
		return script, false
	}
	return out.String(), true
}

// word reads one shell word starting at position. A command substitution
// inside it is scanned as its own script and leaves an expansion marker.
func (s *driverShellScanner) word(line string, position int, cwd string) (driverWord, int) {
	var text strings.Builder
	quoted := false
	escaped := false
	inDouble := false
	var refs []string
	for position < len(line) {
		char := line[position]
		if !inDouble && strings.IndexByte(" \t\r;&|()<>`", char) >= 0 {
			break
		}
		switch {
		case char == '\\' && inDouble && position+1 < len(line) && strings.IndexByte("$`\"\\", line[position+1]) < 0:
			// Inside double quotes Bash keeps a backslash that does not
			// precede $, `, ", or \ (so <<"\true" ends at \true, not true).
			escaped = true
			text.WriteByte('\\')
			position++
		case char == '\\' && position+1 < len(line):
			escaped = true
			text.WriteByte(line[position+1])
			position += 2
		case char == '\\':
			escaped = true
			s.scan.revoke("shell:incomplete-word")
			position++
		case char == '\'' && !inDouble:
			// A quote after a literal `NAME=` leaves the word an assignment.
			quoted = quoted || !driverAssignment(text.String())
			end := strings.IndexByte(line[position+1:], '\'')
			if end < 0 {
				s.scan.revoke("shell:incomplete-word")
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
				s.scan.revoke("shell:incomplete-word")
				end = len(line) - position - 1
			}
			s.feeds++
			s.script(line[position+1:position+1+end], cwd)
			s.feeds--
			refs = append(refs, driverReferencedNames(line[position+1:position+1+end])...)
			text.WriteString(driverExpansion)
			position += end + 2
		case char == '$':
			next := position + 1
			switch {
			case next < len(line) && line[next] == '(':
				end := driverMatchingClose(line, next)
				if end == len(line) {
					s.scan.revoke("shell:incomplete-word")
				}
				inner := line[next+1 : max(next+1, end)]
				if !strings.HasPrefix(inner, "(") {
					s.feeds++
					s.script(inner, cwd)
					s.feeds--
				} else {
					s.scan.revoke("shell:arithmetic-context")
					// Arithmetic reads, and may assign, bare names.
					refs = append(refs, "*")
					if driverArithmeticAssigns(inner) {
						// AFP-V0-9625: it may assign any name, PATH included.
						s.scan.markComputed("shell:arithmetic-assignment")
					}
					if driverSubstitutes(inner) {
						// AFP-V0-9625: the scanner does not run its command
						// substitution as shell code.
						s.scan.markComputed("shell:arithmetic-substitution")
					}
				}
				refs = append(refs, driverReferencedNames(inner)...)
				text.WriteString(driverExpansion)
				position = min(end+1, len(line))
			case next < len(line) && line[next] == '{':
				end := driverMatchingClose(line, next)
				if end == len(line) {
					s.scan.revoke("shell:incomplete-word")
				}
				inner := line[next+1 : max(next+1, min(end, len(line)))]
				// Only the direct ${name} spelling is fully understood for
				// narrowing. Operators may hide assignments or bare-name reads.
				if strings.HasPrefix(inner, "!") {
					s.scan.revoke("shell:indirect-expansion")
				} else if strings.ContainsAny(inner, "[:") {
					s.scan.revoke("shell:arithmetic-context")
				} else if inner == "" || strings.IndexFunc(inner, func(r rune) bool { return r > 127 || !isDriverNameByte(byte(r)) }) >= 0 {
					s.scan.revoke("shell:unsupported-expansion")
				}
				names, assigns := driverBraceRefs(inner)
				refs = append(refs, names...)
				if assigns {
					// AFP-V0-9625: `${NAME=value}` and `${NAME:=value}` assign
					// NAME, which may be exported or special.
					s.scan.markComputed("shell:assigning-expansion")
				}
				if driverSubstitutes(inner) {
					// AFP-V0-9625: the scanner does not run a command
					// substitution in `${NAME:-$(...)}` as shell code.
					s.scan.markComputed("shell:brace-substitution")
				}
				text.WriteString(driverExpansion)
				position = min(end+1, len(line))
			case next < len(line) && (isDriverNameByte(line[next]) || strings.IndexByte("@*#?$!-0123456789", line[next]) >= 0):
				if isDriverNameByte(line[next]) {
					for next < len(line) && isDriverNameByte(line[next]) {
						next++
					}
					refs = append(refs, line[position+1:next])
				} else {
					next++
				}
				text.WriteString(driverExpansion)
				position = next
			default:
				if next < len(line) && strings.ContainsAny(line[next:next+1], "['\"") {
					s.scan.revoke("shell:unsupported-expansion")
				}
				text.WriteByte('$')
				position++
			}
		default:
			// Unquoted brace/glob/tilde expansion is not a literal word.
			// Standalone braces and `[` are handled as shell syntax after
			// the preceding command is flushed (including `function name`).
			syntaxToken := strings.ContainsRune("{}[", rune(char)) && text.Len() == 0 &&
				(position+1 == len(line) || strings.ContainsRune(" \t\r;&|()<>`", rune(line[position+1])))
			if !inDouble && strings.ContainsRune("{}[*?~", rune(char)) && !syntaxToken {
				s.scan.revoke("shell:unsupported-expansion")
			}
			text.WriteByte(char)
			position++
		}
	}
	if inDouble {
		s.scan.revoke("shell:incomplete-word")
	}
	s.scan.referenceNames(refs)
	return driverWord{text: text.String(), quoted: quoted, escaped: escaped, refs: refs}, position
}

// driverSubstitutes reports text holding a command substitution.
func driverSubstitutes(text string) bool {
	return strings.Contains(text, "$(") || strings.Contains(text, "`")
}

// driverArithmeticAssigns reports arithmetic text with an assignment or an
// increment operator: any `=` other than in `==`, `!=`, `<=` or `>=`.
func driverArithmeticAssigns(text string) bool {
	for _, comparison := range []string{"==", "!=", "<=", ">="} {
		text = strings.ReplaceAll(text, comparison, "")
	}
	return strings.Contains(text, "=") || strings.Contains(text, "++") || strings.Contains(text, "--")
}

// driverBraceRefs returns the names a `${...}` expansion with body inner
// reads, "*" for an indirect `${!...}`, and whether it assigns its name.
func driverBraceRefs(inner string) (names []string, assigns bool) {
	switch {
	case strings.HasPrefix(inner, "!"):
		names = append(names, "*")
		inner = inner[1:]
	case strings.HasPrefix(inner, "#") && len(inner) > 1:
		inner = inner[1:]
	}
	length := 0
	for length < len(inner) && isDriverNameByte(inner[length]) {
		length++
	}
	rest := inner[length:]
	if length > 0 {
		names = append(names, inner[:length])
		if strings.HasPrefix(rest, "[") {
			if end := driverMatchingBracket(rest); end > 0 {
				rest = rest[end+1:]
			}
		}
		assigns = strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, ":=")
	}
	return append(names, driverReferencedNames(inner[length:])...), assigns
}

// driverMatchingBracket returns the index of the `]` closing the `[` that
// text starts with, or -1.
func driverMatchingBracket(text string) int {
	depth := 0
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// driverReferencedNames returns every parameter name a `$` in text expands,
// quoted or not, and "*" for an indirect or arithmetic expansion: an
// over-approximation for text the word scanner does not split.
func driverReferencedNames(text string) []string {
	var names []string
	for index := 0; index < len(text); index++ {
		if text[index] != '$' || index+1 >= len(text) {
			continue
		}
		next := index + 1
		switch {
		case text[next] == '{':
			end := driverMatchingClose(text, next)
			inner, _ := driverBraceRefs(text[next+1 : max(next+1, min(end, len(text)))])
			names = append(names, inner...)
		case strings.HasPrefix(text[next:], "(("):
			names = append(names, "*")
		case isDriverNameByte(text[next]):
			end := next
			for end < len(text) && isDriverNameByte(text[end]) {
				end++
			}
			names = append(names, text[next:end])
			index = end - 1
		}
	}
	return names
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
	if s.scan.strict {
		s.strictWords(words)
	}
	// prefixed marks an assignment in front of the command word itself,
	// with no wrapper between them (NCM-V1-017).
	assignment := func(word string, prefixed bool) {
		name, value, _ := strings.Cut(word, "=")
		s.scan.local(strings.TrimSuffix(name, "+"))
		// AFP-V0-3128: only the cache location, empty Go flags and the local
		// toolchain have modeled effects here. Unknown environment names can
		// inject startup hooks or arbitrary tool reads, including through
		// shell wrappers.
		switch name {
		case "GOCACHE":
			return
		case "GOFLAGS":
			if value == "" {
				return
			}
		case "GOTOOLCHAIN":
			// AZU-V0-612: `local` forbids the toolchain switch that any
			// other value may perform, which downloads and runs a toolchain
			// this graph does not model; it adds no read of its own.
			if value == "local" {
				return
			}
		case "GOOS", "GOARCH", "CGO_ENABLED", "GOENV", "GOWORK", "GOPROXY":
			// AFP-V0-3601: these values only choose a build variant the Go
			// claimant already unions or stop the go command reading a
			// configuration file or the network.
			if driverGoSelectionValues[name][value] {
				return
			}
		}
		if prefixed && driverInertPrefix(strings.TrimSuffix(name, "+"), value) {
			s.scan.envPrefix("shell:assignment")
		} else {
			s.scan.markComputed("shell:assignment")
		}
		s.scan.uncertify("shell:assignment:" + name)
	}
	// pending holds the leading assignments until the scan knows whether a
	// command word follows them: only a standalone assignment, not one in a
	// command's own environment, can be a plain shell variable (AFP-V0-9622).
	// wrapped marks a wrapper word, after which an assignment is the wrapped
	// command's environment.
	var pending []driverWord
	wrapped := false
	settle := func(standalone, prefixed bool) {
		for _, word := range pending {
			if !standalone || !s.localAssignment(word.text) {
				assignment(word.text, prefixed)
			}
		}
		pending = nil
	}
	for len(words) != 0 {
		head := words[0].text
		if words[0].quoted || words[0].escaped {
			s.scan.revoke("shell:nonliteral-name")
		}
		switch {
		case !words[0].quoted && driverAssignment(head):
			if wrapped {
				assignment(head, false)
			} else {
				pending = append(pending, words[0])
			}
			words = words[1:]
		case !words[0].quoted && head == "command" && len(words) > 1 && (words[1].text == "-v" || words[1].text == "-V"):
			// A lookup: it names a command and runs none.
			settle(false, false)
			return cwd
		case !words[0].quoted && driverPrefixKeywords[head]:
			// Control flow and wrappers retain their existing path scan, but
			// its permissive token stripping is not a complete grammar proof.
			s.scan.revoke("shell:unsupported-syntax")
			if len(pending) != 0 || !driverReservedWords[head] {
				settle(false, false)
				wrapped = true
			}
			words = words[1:]
		case head == "env":
			s.scan.revoke("shell:unsupported-wrapper")
			settle(false, false)
			wrapped = true
			words = words[1:]
			for len(words) != 0 && (strings.HasPrefix(words[0].text, "-") || driverAssignment(words[0].text)) {
				option := words[0].text
				if driverAssignment(option) {
					assignment(option, false)
				}
				// AZU-V0-9851: env changes only the wrapped command's cwd.
				// Until that scoped directory is modelled, keep the site computed.
				if strings.HasPrefix(option, "-C") || option == "--chdir" || strings.HasPrefix(option, "--chdir=") {
					s.scan.markComputed("shell:env-chdir")
				}
				if option == "-u" || option == "-C" || option == "--chdir" || option == "-S" {
					if len(words) < 2 {
						s.scan.markComputed("shell:env-missing-value")
						return cwd
					}
					if option == "-S" {
						s.scan.commandWord(words[1])
					}
					words = words[1:]
				}
				words = words[1:]
			}
		case head == "xargs":
			s.scan.unfollowed() // The command's operands come from its input.
			s.scan.revoke("shell:unsupported-wrapper")
			settle(false, false)
			wrapped = true
			s.scan.uncertify("shell:xargs-operands") // Operands come from its input (AFP-V0-3128).
			// The command xargs runs takes arguments read from standard
			// input, which the scan cannot see (AFP-V0-2957).
			rest, replace, ok := driverXargs(words[1:])
			if !ok {
				s.scan.markComputed("shell:xargs-options")
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
			s.scan.revoke("shell:unsupported-wrapper")
			settle(false, false)
			wrapped = true
			words = words[1:]
			for len(words) != 0 && strings.HasPrefix(words[0].text, "-") {
				if !strings.Contains(words[0].text, "=") && len(words[0].text) == 2 && strings.IndexByte("nksIcpLPdEaR", words[0].text[1]) >= 0 {
					if len(words) < 2 {
						s.scan.markComputed("shell:wrapper-missing-value")
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
			// `IFS= read -r name...` sets IFS for read alone (AFP-V0-9624).
			if !(len(pending) == 1 && pending[0].text == "IFS=" && s.localRead(words) != nil) {
				settle(false, true)
			}
			pending = nil
			return s.simple(words, cwd)
		}
	}
	settle(true, false)
	return cwd
}

// driverGoSelectionValues are the exact Go toolchain assignments AFP-V0-3601
// admits beside AFP-V0-3128's: a GOOS or GOARCH whose files the Go claimant's
// package units already union (AFP-V0-580) and whose programs no wrapper in
// the Go distribution runs, as js and wasip1 programs are, and the values that
// stop cgo, the user's go env file, go.work and module downloads.
var driverGoSelectionValues = map[string]map[string]bool{
	"GOOS":        {"linux": true, "darwin": true, "windows": true, "freebsd": true, "netbsd": true, "openbsd": true},
	"GOARCH":      {"amd64": true, "arm64": true, "386": true, "arm": true},
	"CGO_ENABLED": {"0": true},
	"GOENV":       {"off": true},
	"GOWORK":      {"off": true},
	"GOPROXY":     {"off": true},
}

// driverReservedWords are the prefix keywords that are shell syntax, not a
// command that runs the rest of the words.
var driverReservedWords = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "fi": true, "do": true, "done": true,
	"while": true, "until": true, "!": true,
}

// driverSpecialNames are lower-case names that a shell (bash, POSIX sh, or
// zsh's tied arrays) or common tools read from the shell's own variables or
// from an inherited environment. They are compared after lower-casing, so
// a narrowed name can never alias one in any case (AFP-V0-9622).
var driverSpecialNames = map[string]bool{
	"ifs": true, "path": true, "cdpath": true, "env": true, "home": true, "pwd": true, "oldpwd": true,
	"globignore": true, "shellopts": true, "bashopts": true, "ps1": true, "ps2": true, "ps3": true,
	"ps4": true, "prompt_command": true, "posixly_correct": true, "shell": true, "tmpdir": true,
	"optind": true, "optarg": true, "opterr": true, "reply": true, "funcnest": true, "execignore": true,
	"histfile": true, "hostfile": true, "inputrc": true, "mail": true, "mailpath": true, "mailcheck": true,
	"timeformat": true, "tmout": true, "lang": true, "tz": true, "term": true,
	"http_proxy": true, "https_proxy": true, "no_proxy": true, "all_proxy": true, "ftp_proxy": true,
	"rsync_proxy": true, "fpath": true, "manpath": true, "module_path": true, "fignore": true,
	"psvar": true, "watch": true,
}

// driverLocalName reports a name a standalone assignment may set as a plain
// shell variable (AFP-V0-9622): lower-case letters, digits and underscores
// only, and none a shell or tool gives a meaning.
func driverLocalName(name string) bool {
	if name == "" || name == "_" || name[0] >= '0' && name[0] <= '9' {
		return false
	}
	for index := 0; index < len(name); index++ {
		char := name[index]
		if char != '_' && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
	}
	for _, prefix := range []string{"bash_", "lc_", "npm_", "yarn_", "pnpm_"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return !driverSpecialNames[name] && !strings.Contains(name, "proxy")
}

// driverStartupNames are lower-cased environment names that make a shell
// source a file, set options or trace commands at startup (AFP-V0-9623).
var driverStartupNames = map[string]bool{"env": true, "shellopts": true, "bashopts": true, "ps4": true, "posixly_correct": true}

// narrowable reports a bash or sh script, run as `-c` with no other
// option when inline, whose environment is known and sets nothing a shell
// reads at startup (AFP-V0-9623). Nothing else is narrowed.
func (s *driverShellScanner) narrowable() bool {
	if s.opaque != 0 || !s.scan.envKnown {
		return false
	}
	for name := range s.scan.envNames {
		if driverStartupNames[name] || strings.HasPrefix(name, "bash_") {
			return false
		}
	}
	return true
}

// localName reports a name this script may assign or read into as a plain
// shell variable: a driverLocalName that the caller's configuration does not
// export to the script, in a bash or sh script (AFP-V0-9622, AFP-V0-9623).
func (s *driverShellScanner) localName(name string) bool {
	return s.narrowable() && driverLocalName(name) && !s.scan.envNames[strings.ToLower(name)]
}

// localAssignment reports, and records, a standalone assignment word that
// sets a plain shell variable (AFP-V0-9622). Its value is not modelled:
// settleNarrowing keeps the script computed if a parsed reference uses it.
func (s *driverShellScanner) localAssignment(word string) bool {
	name, _, _ := strings.Cut(word, "=")
	name = strings.TrimSuffix(name, "+")
	if !s.localName(name) {
		return false
	}
	s.scan.local(name)
	s.scan.narrow("shell:assignment")
	return true
}

// localRead returns the names `read -r name...` reads into when every one is
// a plain shell variable (AFP-V0-9624), or nil for any other read.
func (s *driverShellScanner) localRead(words []driverWord) []string {
	if len(words) < 3 || words[0].text != "read" || words[0].quoted || words[1].text != "-r" {
		return nil
	}
	names := make([]string, 0, len(words)-2)
	for _, word := range words[2:] {
		if word.quoted || word.escaped || !s.localName(word.text) {
			return nil
		}
		names = append(names, word.text)
	}
	return names
}

// driverSetOptions are the `set -o` names that change only how the shell
// reports or stops on errors (AFP-V0-9621).
var driverSetOptions = map[string]bool{"pipefail": true, "errexit": true, "nounset": true, "xtrace": true}

// setNarrowed reports a `set` whose every argument is an allowlisted option
// (AFP-V0-9621): -e, -u, -x, -v and -E, alone or combined, with - or +, and
// -o or +o naming pipefail, errexit, nounset or xtrace, in a narrowable
// script. Tracing expands PS4, which narrowable excludes from the
// environment and settleNarrowing from sourced files.
func (s *driverShellScanner) setNarrowed(args []driverWord) bool {
	if len(args) == 0 || !s.narrowable() {
		return false
	}
	for index := 0; index < len(args); index++ {
		arg := args[index].text
		if len(arg) < 2 || arg[0] != '-' && arg[0] != '+' {
			return false
		}
		for offset := 1; offset < len(arg); offset++ {
			switch letter := arg[offset]; {
			case letter == 'o' && offset == len(arg)-1:
				index++
				if index >= len(args) || !driverSetOptions[args[index].text] {
					return false
				}
			case strings.IndexByte("euxvE", letter) >= 0:
			default:
				return false
			}
		}
	}
	return true
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

// driverRunNames are the environment names that change which program a
// command word runs or load code into it beside that program: the command
// search path, a shell's startup file or options, the home and
// configuration directories tools read their hooks from, the dynamic loader,
// an interpreter's or make's own options and include paths, and the
// toolchain controls of Go and Git. driverRunPrefixes are the families of them.
var driverRunNames = map[string]bool{
	"PATH": true, "ENV": true, "BASH_ENV": true, "SHELLOPTS": true, "BASHOPTS": true, "IFS": true,
	"CDPATH": true, "PS4": true, "SHELL": true, "GLOBIGNORE": true, "HOME": true,
	"MAKEFILES": true, "MAKEFLAGS": true, "MFLAGS": true, "GNUMAKEFLAGS": true, "MAKELEVEL": true,
	"JAVA_TOOL_OPTIONS": true, "_JAVA_OPTIONS": true, "JDK_JAVA_OPTIONS": true, "CLASSPATH": true,
	"PERL5OPT": true, "PERL5LIB": true, "PERLLIB": true, "RUBYOPT": true, "RUBYLIB": true,
	"GEM_HOME": true, "GEM_PATH": true, "CC": true, "CXX": true, "LUA_INIT": true, "LUA_PATH": true,
}

var driverRunPrefixes = []string{
	"LD_", "DYLD_", "GO", "CGO_", "GIT_", "NODE_", "NPM_", "PYTHON", "BASH_FUNC_", "XDG_",
}

// driverInertPrefix reports an environment assignment in front of a command
// word that leaves what the command runs as the scan resolves it
// (NCM-V1-017): a literal value, with no command substitution, parameter
// expansion or make value the graph does not know (a `$(shell ...)` result
// among them), of a name outside driverRunNames and driverRunPrefixes,
// compared without case. Such a value still reaches what the command reads,
// so the site stays uncertified and computed for the planner.
func driverInertPrefix(name, value string) bool {
	if name == "" || driverDynamic(value) || driverSubstitutes(value) {
		return false
	}
	upper := strings.ToUpper(name)
	if driverRunNames[upper] {
		return false
	}
	for _, prefix := range driverRunPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return false
		}
	}
	return true
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
	if relative, ok := s.repositoryExecutable(words[0].text); ok {
		// An absolute path below the repository root names the repository
		// program at that relative path, never a command of the same base
		// name (AFP-V0-9131).
		words = append([]driverWord{{text: relative, quoted: words[0].quoted, escaped: words[0].escaped, refs: words[0].refs}}, words[1:]...)
	}
	head := words[0].text
	s.scan.commandWord(words[0])
	for _, word := range words {
		switch word.text {
		case "-eq", "-ne", "-lt", "-le", "-gt", "-ge":
			s.scan.revoke("shell:arithmetic-context")
		}
	}
	switch {
	case head == "set" && s.setNarrowed(words[1:]):
		s.scan.narrow("shell:builtin-set") // AFP-V0-9621.
		return cwd
	case head == "read":
		// Retain declarations even when read's spelling or environment makes
		// it ineligible for narrowing; escaped names have already been decoded.
		for _, word := range words[1:] {
			if !strings.HasPrefix(word.text, "-") {
				s.scan.local(word.text)
			}
			if word.quoted || word.escaped {
				s.scan.revoke("shell:nonliteral-name")
			}
		}
		if names := s.localRead(words); names != nil {
			for _, name := range names {
				s.scan.local(name)
			}
			s.scan.narrow("shell:builtin-read") // AFP-V0-9624.
			return cwd
		}
	case head == "shopt" && s.narrowable() && driverShoptNarrowed(words[1:]):
		s.scan.narrow("shell:builtin-shopt") // AFP-V0-9627.
		return cwd
	case head == "printf" && slices.ContainsFunc(words[1:], func(word driverWord) bool { return strings.HasPrefix(word.text, "-v") }):
		head = "printf-v"
	case (head == "for" || head == "select") && len(words) > 1:
		// Loop execution and variable lifetime are not modelled. This floor
		// also applies to make recipes and opaque inline shells (AFP-V0-9628).
		s.scan.local(words[1].text)
		s.scan.markComputed("shell:loop-variable")
		s.scan.revoke("shell:loop-variable")
	case head == "function":
		s.scan.revoke("shell:function-definition")
		if len(words) > 1 {
			s.scan.defineFunction(words[1].text)
		}
	case head == "case" || head == "in" || head == "esac" || head == "[[" || head == "]]":
		s.scan.revoke("shell:unsupported-syntax")
	}
	switch head {
	case "export", "local", "readonly", "declare", "set", "unset", "read",
		"typeset", "let", "mapfile", "readarray", "getopts", "hash", "enable", "shopt", "printf-v", "alias":
		// Environment mutations persist beyond this simple command. No
		// forms of these builtins have a modeled environment transition.
		// Retain the site even when the recipe has no make expansion.
		s.scan.markComputed("shell:builtin-" + head)
		s.scan.revoke("shell:builtin-" + head)
		s.scan.uncertify("shell:builtin-" + head)
		s.embedded(words[1:], cwd)
		return cwd
	case "trap":
		// AFP-V0-3128: the body runs later as shell code, potentially
		// with a different environment or directory.
		s.scan.unfollowed()
		s.scan.uncertify("shell:trap")
		s.scan.revoke("shell:unsupported-syntax")
		for _, word := range words[1:] {
			s.scan.commandWord(word)
		}
		s.embedded(words[1:], cwd)
		return cwd
	case "for", "case", "select", "function", "in", "esac", "[[", "]]", "[", "test", "echo", "printf", "true", "false", ":", "shift", "return", "exit", "wait":
		if (head == "[[" || head == "[" || head == "test") && !s.driverTestCertified(words, cwd) {
			s.scan.uncertify("shell:test-operand:" + driverReasonWord(s.driverTestUnrecordedOperand(words, cwd)))
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
		// Preserve uncertainty before lexical cleaning can erase a link
		// followed by parent traversal (AFP-V0-9051).
		if _, ok := s.driverPlainDirectory(target, cwd); !ok {
			s.scan.uncertify("shell:cd-unproven:" + driverReasonWord(target))
		}
		return driverJoin(cwd, target)
	case "eval":
		s.scan.markComputed("shell:eval")
		s.scan.revoke("shell:eval")
		s.scan.uncertify("shell:eval")
		return cwd
	case ".", "source":
		s.scan.unfollowed()
		s.scan.uncertify("shell:sourced")
		// AFP-V0-9628: the file may define functions or set PS4.
		s.scan.revoke("shell:sourced")
		if len(words) > 1 {
			s.scan.commandWord(words[1])
			if driverComputedWord(words[1].text) {
				s.scan.markComputed("shell:source-computed")
			}
			s.pathWord(words[1].text, cwd)
		}
		return cwd
	}
	if driverComputedWord(head) {
		s.scan.markComputed("shell:computed-command-word")
	}
	if !driverFollowed(words) {
		s.scan.unfollowed()
	}
	if !s.driverCertified(words, cwd) {
		s.scan.uncertify(s.driverUncertifiedReason(words, cwd))
	}
	for _, replace := range s.replaces {
		// The command xargs runs is named by what it reads (AFP-V0-2957).
		if strings.Contains(head, replace) {
			s.scan.markComputed("shell:xargs-replace-command")
		}
	}
	base := path.Base(head)
	// AFP-V0-9401: enumeration depends on the file set, including paths
	// that have no owner or do not exist yet. Do not infer a narrower root
	// from operands: flags, symlinks and runtime directories can change it.
	if base == "find" || base == "rg" || base == "git" && slices.ContainsFunc(words[1:], func(word driverWord) bool { return word.text == "ls-files" || word.text == "grep" }) {
		s.scan.readsTree = true
	}
	if driverInterpreters[base] {
		for _, word := range words[1:] {
			s.scan.commandWord(word)
		}
		// operand reports a script operand or an inline program; without
		// one the interpreter reads its program from standard input: a
		// pipe, a heredoc whose body this scanner skips, or a redirect
		// (AFP-V0-2958).
		operand := false
		// AFP-V0-3602: `node --test` over literal existing files runs those
		// files, as `node FILE` runs its script operand.
		nodeTest := base == "node" && len(words) > 2 && words[1].text == "--test" && s.driverNodeTestFiles(words[2:], cwd)
		if nodeTest {
			for _, word := range words[2:] {
				s.pathWord(word.text, cwd)
			}
			operand = true
		}
		for index := 1; index < len(words) && !nodeTest; index++ {
			arg := words[index].text
			if driverShells[base] && strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "c") && !strings.HasPrefix(arg, "--") {
				if index+1 < len(words) {
					// Another shell's special parameters differ, and an option
					// such as -a exports every assignment: nothing in the inline
					// script is narrowed unless it is exactly `sh|bash -c`
					// (AFP-V0-9623).
					opaque := base != "sh" && base != "bash" || index != 1 || arg != "-c"
					if opaque {
						s.scan.revoke("shell:opaque-script")
						s.opaque++
					}
					s.script(words[index+1].text, cwd)
					if opaque {
						s.opaque--
					}
				}
				operand = true
				break
			}
			if driverShells[base] && (strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "+")) {
				// Even an option not recognized as an inline program must
				// not let an outer candidate claim the whole script is known.
				s.scan.revoke("shell:opaque-script")
			}
			if arg == "-c" || arg == "-e" || (!driverShells[base] && strings.HasPrefix(arg, "-")) {
				// Inline programs in other languages are outside the shell
				// scanner. Unsupported options may contain a program or
				// consume the next word; retain the site rather than guess
				// which word names a script.
				s.scan.markComputed("shell:interpreter-inline")
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
				s.scan.markComputed("shell:interpreter-computed-script")
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
			s.scan.markComputed("shell:interpreter-stdin")
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

// driverNodeTestFiles reports `node --test` operands that are each a literal
// relative path to an existing regular `.js`, `.mjs` or `.cjs` file recorded
// as a repository edge (AFP-V0-3602): Node runs exactly those files as tests.
// No operand, an option, a glob, a directory or a missing file refuses.
func (s *driverShellScanner) driverNodeTestFiles(words []driverWord, cwd string) bool {
	for _, word := range words {
		text := word.text
		if driverDynamic(text) || strings.HasPrefix(text, "-") || strings.ContainsAny(text, "*?[]{}") ||
			!(strings.HasSuffix(text, ".js") || strings.HasSuffix(text, ".mjs") || strings.HasSuffix(text, ".cjs")) ||
			!s.driverOperandRecorded(text, cwd) {
			return false
		}
		info, err := s.scan.source.Lstat(driverJoin(cwd, text))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return len(words) != 0
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
		directory := driverGlobDirectory(word)
		if directory == "" {
			return false
		}
		relative := driverJoin(cwd, directory)
		if !ValidRelativePath(relative) || !driverPlainPath(s.scan.source, relative) {
			return false
		}
		entries, err := s.scan.source.ReadDir(relative)
		if err != nil || len(entries) > 4096 {
			return false
		}
		// Only regular matched files are covered by the directory edge.
		// Links, directories and unfamiliar file types may read elsewhere.
		suffix := word[strings.LastIndexByte(word, '/')+2:]
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(entry.Name(), suffix) {
				continue
			}
			info, err := s.scan.source.Lstat(path.Join(relative, entry.Name()))
			if err != nil || !info.Mode().IsRegular() {
				return false
			}
		}
		return true
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

// repositoryExecutable maps an absolute command word below the source's disk
// root to its repository-relative path (AFP-V0-9131). Any other absolute word
// names no repository edge: the caller leaves it uncertified and records no
// path for it, whatever its base name.
func (s *driverShellScanner) repositoryExecutable(head string) (string, bool) {
	if !strings.HasPrefix(head, "/") || driverDynamic(head) || s.scan.source == nil || s.scan.source.root == "" {
		return "", false
	}
	root := strings.TrimSuffix(strings.ReplaceAll(s.scan.source.root, "\\", "/"), "/")
	if !strings.HasPrefix(root, "/") {
		return "", false
	}
	relative, found := strings.CutPrefix(head, root+"/")
	if !found || relative == "" || !ValidRelativePath(relative) {
		return "", false
	}
	return relative, true
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
	if strings.Contains(head, "/") {
		return s.driverOperandRecorded(head, cwd) && driverShellTest(head) && s.driverTestArgumentsRecorded(words[1:], cwd)
	}
	switch {
	case driverPureCommands[base]:
		return true
	case base == "make" || base == "gmake":
		return driverMakeCommandCertified(words[1:])
	case base == "go":
		return s.driverGoCommandCertified(words[1:], cwd)
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

// driverUncertifiedReason names why driverCertified refused words
// (AZU-V0-611): the command word, and for a command kind the rules can
// certify, the first operand they could not bound.
func (s *driverShellScanner) driverUncertifiedReason(words []driverWord, cwd string) string {
	head := words[0].text
	reason := "shell:uncertified-command:" + driverReasonWord(head)
	base := path.Base(head)
	switch {
	case driverDynamic(head), words[0].quoted && head == "":
		return reason
	case strings.Contains(head, "/"), base == "make", base == "gmake", base == "go", driverShells[base], driverBoundedCommands[base]:
		for _, word := range words[1:] {
			text := word.text
			if driverDynamic(text) || strings.ContainsAny(text, "*?[") || driverRootedPath(text) || driverAssignment(text) || !strings.HasPrefix(text, "-") && !s.driverOperandRecorded(text, cwd) {
				return reason + ":" + driverReasonWord(text)
			}
		}
	}
	return reason
}

// driverTestUnrecordedOperand returns the first path operand of a test
// command that driverTestCertified refused, for its diagnostic reason.
func (s *driverShellScanner) driverTestUnrecordedOperand(words []driverWord, cwd string) string {
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
			if !s.driverOperandRecorded(operand, cwd) {
				return operand
			}
		}
	}
	return ""
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
func (s *driverShellScanner) driverGoCommandCertified(words []driverWord, cwd string) bool {
	if !driverLiteral(words) || slices.ContainsFunc(words, func(word driverWord) bool {
		return strings.ContainsAny(word.text, "*?[")
	}) {
		return false
	}
	for len(words) > 0 && (words[0].text == "-C" || strings.HasPrefix(words[0].text, "-C=")) {
		directory := strings.TrimPrefix(words[0].text, "-C=")
		if words[0].text == "-C" {
			if len(words) < 2 {
				return false
			}
			directory = words[1].text
			words = words[1:]
		}
		var ok bool
		cwd, ok = s.driverPlainDirectory(directory, cwd)
		if !ok {
			return false
		}
		words = words[1:]
	}
	if len(words) == 0 || !driverGoCertified[words[0].text] {
		return false
	}
	packageCommand := driverGoPackageCommands[words[0].text]
	if packageCommand {
		if _, ok := s.driverPlainDirectory(".", cwd); !ok {
			return false
		}
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
			// AFP-V0-9051: certify only package forms whose source paths
			// agree with the lexical paths recorded by goPattern.
			if driverRootedPath(arg) || packageCommand && !s.driverGoPackageRecorded(arg, cwd) {
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

// driverPlainDirectory admits only relative ASCII paths without parent traversal,
// and proves every source component is a directory without a link. Cleaning
// before this proof could erase a symlink followed by ".." (AFP-V0-9051).
func (s *driverShellScanner) driverPlainDirectory(word, cwd string) (string, bool) {
	if s.scan.source == nil || word == "" || cwd == "" || driverRootedPath(word) || strings.HasPrefix(word, "-") {
		return "", false
	}
	for _, component := range strings.Split(word, "/") {
		if component == ".." {
			return "", false
		}
	}
	for _, char := range word {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("/._-+", char)) {
			return "", false
		}
	}
	relative := driverJoin(cwd, word)
	if relative != "." && !ValidRelativePath(relative) || !driverPlainPath(s.scan.source, relative) {
		return "", false
	}
	info, err := s.scan.source.Lstat(relative)
	return relative, err == nil && info.IsDir()
}

// driverGoPackageRecorded is a closed allowlist: all/std/cmd, relative
// directories (optionally recursive), and main-module directories. Unknown
// import forms and linked directories retain the frontier (AFP-V0-9051).
func (s *driverShellScanner) driverGoPackageRecorded(word, cwd string) bool {
	if word == "all" || word == "std" || word == "cmd" {
		return true
	}
	if s.module != "" && (word == s.module || strings.HasPrefix(word, s.module+"/")) {
		word = "./" + strings.TrimPrefix(strings.TrimPrefix(word, s.module), "/")
		cwd = "."
	}
	if word != "." && !strings.HasPrefix(word, "./") {
		return false
	}
	word = strings.TrimSuffix(word, "/...")
	if strings.Contains(word, "...") {
		return false
	}
	_, ok := s.driverPlainDirectory(word, cwd)
	return ok
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
			s.scan.markComputed("shell:fed-interpreter")
		}
		if fed {
			// A feeder's later word may name the command it executes.
			s.scan.commandWord(word)
		}
		switch {
		case base == "parallel":
			fed, parallel = true, true
		case base == "xargs" || word.text == "-exec" || word.text == "-execdir" || word.text == "-ok" || word.text == "-okdir":
			fed = true
			if index+1 < len(words) && strings.Contains(words[index+1].text, "{}") {
				s.scan.markComputed("shell:feeder-replace")
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
	for _, component := range strings.Split(word[:slash], "/") {
		if component == ".." {
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
			s.scan.uncertify("shell:glob-operand:" + driverReasonWord(word))
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
			s.scan.markComputed("go:chdir-missing")
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
	if command == "run" {
		// A tool's runtime reads are not its Go package imports. Until
		// those reads are bounded, retain every repository path, whatever
		// the tool's command directory or literal program arguments.
		s.scan.readsTree = true
	}
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
		s.scan.markComputed("go:list-unproven-output")
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
						s.scan.markComputed("go:chdir-missing")
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
		s.scan.markComputed("go:glob-pattern")
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

// driverShoptSafe are the shopt options that change only how a glob matches
// (AFP-V0-9627); every other option, such as expand_aliases, stays computed.
var driverShoptSafe = map[string]bool{"nullglob": true, "failglob": true, "dotglob": true, "globstar": true, "nocaseglob": true}

// driverShoptNarrowed reports `shopt -s|-u` naming only driverShoptSafe options.
func driverShoptNarrowed(args []driverWord) bool {
	if len(args) < 2 || args[0].text != "-s" && args[0].text != "-u" {
		return false
	}
	for _, arg := range args[1:] {
		if !driverShoptSafe[arg.text] {
			return false
		}
	}
	return true
}
