package doccorpus

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// The CI publish and environment rules of DCI-V1-021 to DCI-V1-023: a release or publish job is
// tied to the package manifest one of its executed publish commands publishes, and a deploy job
// without an environment key takes one from its name, as weaker, labelled evidence.
//
// A CI file is read as a tree of the block YAML subset these files use, so no result depends on
// the order of a mapping's keys. A node the subset cannot read (an alias, merge key, tag, flow
// collection, multi-line flow scalar, duplicate key or malformed indentation) is opaque: what it
// would hold is unknown, never guessed.

// ciNode is one mapping entry or sequence item of a CI file.
type ciNode struct {
	key      string
	item     bool
	col      int
	line     int
	value    string // an inline scalar, unquoted
	scalar   bool   // value holds an inline scalar
	block    []ciScriptLine
	isBlock  bool // a literal (|) block scalar
	folded   bool // a folded (>) block scalar
	children []*ciNode
	opaque   bool
}

// ciScriptLine is one line of a script with its 1-based line in the CI file.
type ciScriptLine struct {
	text string
	line int
}

var (
	ciKey         = regexp.MustCompile(`^(<<|[A-Za-z0-9_$][A-Za-z0-9_./$-]*|"[^"\\]*"|'[^']*')[ \t]*:(?:[ \t]+(.*))?$`)
	ciBlockHeader = regexp.MustCompile(`^([|>])[-+0-9]*[ \t]*(?:#.*)?$`)
)

// parseCI reads lines as the block YAML subset. The root is opaque when the file uses tabs for
// indentation, several documents or directives.
func parseCI(lines []string) *ciNode {
	root := &ciNode{col: -1}
	stack := []*ciNode{root}
	place := func(n *ciNode) *ciNode {
		for {
			top := stack[len(stack)-1]
			if top.col < n.col || top.col == n.col && n.item && !top.item && top.key != "" && !top.scalar && !top.isBlock && !top.folded && (len(top.children) == 0 || top.children[0].item) {
				break
			}
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		if parent.scalar || parent.isBlock || parent.folded || len(parent.children) > 0 && parent.children[0].item != n.item {
			parent.opaque = true
		}
		for _, c := range parent.children {
			if !n.item && c.key == n.key {
				parent.opaque = true
			}
		}
		parent.children = append(parent.children, n)
		stack = append(stack, n)
		return parent
	}
	seen := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t\r")
		content := strings.TrimLeft(line, " ")
		if content == "" || content[0] == '#' {
			continue
		}
		if content == "---" && !seen {
			// One leading document marker opens the only document.
			seen = true
			continue
		}
		seen = true
		if content[0] == '\t' || content[0] == '%' || content == "---" || strings.HasPrefix(content, "--- ") || content == "..." {
			root.opaque = true
			return root
		}
		col := len(line) - len(content)
		var item *ciNode
		for content == "-" || strings.HasPrefix(content, "- ") {
			item = &ciNode{item: true, col: col, line: i + 1}
			place(item)
			rest := strings.TrimLeft(content[1:], " ")
			col += len(content) - len(rest)
			content = rest
		}
		if content == "" || content[0] == '#' {
			continue
		}
		if m := ciKey.FindStringSubmatch(content); m != nil {
			key := m[1]
			if len(key) >= 2 && (key[0] == '"' || key[0] == '\'') {
				key = key[1 : len(key)-1]
			}
			n := &ciNode{key: key, col: col, line: i + 1}
			if parent := place(n); key == "<<" {
				parent.opaque = true
			}
			i = n.setValue(m[2], lines, i)
			continue
		}
		if item != nil {
			// A scalar sequence item, such as "- checkout".
			item.setValue(content, nil, i)
			continue
		}
		// A continuation line of a multi-line plain scalar, or anything else the subset does not read.
		stack[len(stack)-1].opaque = true
	}
	return root
}

// setValue reads the inline value raw of n at 0-based line i: a scalar, a block scalar whose
// lines follow (returning the last line it consumed), or nothing when children follow.
func (n *ciNode) setValue(raw string, lines []string, i int) int {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "&") {
		// An anchor names the node; its content is read as written.
		_, raw, _ = strings.Cut(raw, " ")
		raw = strings.TrimSpace(raw)
	}
	switch {
	case raw == "" || raw[0] == '#':
		return i
	case strings.ContainsRune("*!{[%@`", rune(raw[0])):
		n.opaque, n.scalar = true, true
		return i
	case raw[0] == '|' || raw[0] == '>':
		m := ciBlockHeader.FindStringSubmatch(raw)
		if m == nil || lines == nil {
			n.opaque, n.scalar = true, true
			return i
		}
		n.isBlock, n.folded = m[1] == "|", m[1] == ">"
		indent := -1
		j := i + 1
		for ; j < len(lines); j++ {
			text := strings.TrimRight(lines[j], " \t\r")
			content := strings.TrimLeft(text, " ")
			at := len(text) - len(content)
			if content == "" {
				n.block = append(n.block, ciScriptLine{"", j + 1})
				continue
			}
			if at <= n.col {
				break
			}
			if indent < 0 {
				indent = at
			}
			if at < indent || strings.HasPrefix(content, "\t") {
				n.opaque = true
			}
			n.block = append(n.block, ciScriptLine{text[min(indent, at):], j + 1})
		}
		return j - 1
	}
	value, ok := ciScalar(raw)
	n.value, n.scalar, n.opaque = value, true, !ok
	return i
}

// ciScalar unquotes one inline scalar with an optional trailing comment; ok is false for an
// unterminated quote, an escape other than \\ and \", text after a closing quote, or a plain
// scalar holding ": ".
func ciScalar(raw string) (string, bool) {
	rest, value := "", ""
	switch raw[0] {
	case '"':
		var b strings.Builder
		i := 1
		for ; i < len(raw) && raw[i] != '"'; i++ {
			if raw[i] == '\\' {
				if i+1 >= len(raw) || raw[i+1] != '\\' && raw[i+1] != '"' {
					return "", false
				}
				i++
			}
			b.WriteByte(raw[i])
		}
		if i >= len(raw) {
			return "", false
		}
		value, rest = b.String(), raw[i+1:]
	case '\'':
		var b strings.Builder
		i := 1
		for ; i < len(raw); i++ {
			if raw[i] == '\'' {
				if i+1 < len(raw) && raw[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				break
			}
			b.WriteByte(raw[i])
		}
		if i >= len(raw) {
			return "", false
		}
		value, rest = b.String(), raw[i+1:]
	default:
		value = raw
		if at := strings.Index(value, " #"); at >= 0 {
			value = value[:at]
		}
		value = strings.TrimSpace(value)
		return value, !strings.Contains(value, ": ")
	}
	rest = strings.TrimSpace(rest)
	return value, rest == "" || rest[0] == '#'
}

// field is the child of mapping n with key; known is false when n is opaque, so whether it holds
// key is unknown.
func (n *ciNode) field(key string) (child *ciNode, known bool) {
	if n == nil {
		return nil, true
	}
	if n.opaque {
		return nil, false
	}
	for _, c := range n.children {
		if !c.item && c.key == key {
			return c, true
		}
	}
	return nil, true
}

// text is the inline scalar of n; ok is false when n is absent, opaque, a block or holds children.
func (n *ciNode) text() (string, bool) {
	if n == nil || n.opaque || !n.scalar || len(n.children) > 0 {
		return "", false
	}
	return n.value, true
}

// script is the shell text of a run node: its inline scalar or literal block lines. ok is false
// for an opaque node, a mapping or a folded block, whose commands the subset cannot split.
func (n *ciNode) script() ([]ciScriptLine, bool) {
	if v, ok := n.text(); ok {
		return []ciScriptLine{{v, n.line}}, true
	}
	if n == nil || n.opaque || !n.isBlock || len(n.children) > 0 {
		return nil, false
	}
	return n.block, true
}

// publishTie is one publish command of a job with the repository directory it publishes.
type publishTie struct {
	dir  string
	line int
}

// ciDir is a working directory relative to the repository root; known is false when it could not
// be read as a literal relative directory inside the repository.
type ciDir struct {
	dir   string
	known bool
}

// literalDirectory reads a working directory value relative to the repository root.
func literalDirectory(n *ciNode) ciDir {
	v, ok := n.text()
	if !ok {
		return ciDir{}
	}
	dir, ok := ciDirectory(".", v)
	return ciDir{dir, ok}
}

// shellOK is true when a shell key names bash or sh, with their options.
func shellOK(n *ciNode) bool {
	v, ok := n.text()
	first, _, _ := strings.Cut(strings.TrimSpace(v), " ")
	return ok && (first == "bash" || first == "sh")
}

// posixRunner is true when a job with no shell key runs its scripts in a POSIX shell: no runner
// or executor value of it names Windows, and, on GitHub, runs-on is no expression.
func posixRunner(j *ciNode, keys ...string) bool {
	for _, key := range keys {
		n, ok := j.field(key)
		if !ok {
			return false
		}
		text := strings.ToLower(scriptText(n))
		if n != nil && (n.opaque || strings.Contains(text, "win") || strings.Contains(text, "${{")) {
			return false
		}
	}
	return true
}

// publishTies reads the executed npm, pnpm and yarn publish commands of job in a parsed CI file,
// each with the repository directory it publishes (DCI-V1-021, DCI-V1-023). opaque is true when a
// publish command's directory or the repository's place could not be read, so the job is only a
// candidate for the manifests it does not tie.
func publishTies(root *ciNode, job, rule string) (ties []publishTie, opaque bool) {
	jobs, ok := root.field("jobs")
	if !ok || jobs == nil {
		return nil, true
	}
	j, ok := jobs.field(job)
	if !ok || j == nil || j.opaque {
		return nil, true
	}
	base, shellJob, shellSet := ciDir{".", true}, true, false
	if rule == githubJobRule {
		// A step's working-directory replaces the job's defaults.run.working-directory, which
		// replaces the workflow's; each is relative to the repository root.
		for _, scope := range []*ciNode{root, j} {
			defaults, ok := scope.field("defaults")
			if !ok {
				return nil, true
			}
			run, ok := defaults.field("run")
			if !ok {
				return nil, true
			}
			wd, ok := run.field("working-directory")
			if !ok {
				return nil, true
			}
			if wd != nil {
				base = literalDirectory(wd)
			}
			shell, ok := run.field("shell")
			if !ok {
				return nil, true
			}
			if shell != nil {
				shellJob, shellSet = shellOK(shell), true
			}
		}
		if !shellSet {
			shellJob = posixRunner(j, "runs-on")
		}
	} else {
		// A CircleCI job's working_directory is where checkout places the repository, so it is
		// the root every relative step directory is read from.
		shell, ok := j.field("shell")
		if !ok {
			return nil, true
		}
		if shellJob = posixRunner(j, "executor", "machine"); shell != nil {
			shellJob = shellOK(shell)
		}
	}
	steps, ok := j.field("steps")
	if !ok {
		return nil, true
	}
	if steps == nil {
		return nil, false
	}
	if steps.scalar || steps.isBlock || steps.folded || len(steps.children) > 0 && !steps.children[0].item {
		return nil, true
	}
	checkedOut, moved := false, false
	for _, step := range steps.children {
		if step.opaque {
			opaque = true
			continue
		}
		run, dir, stepShell := (*ciNode)(nil), base, shellJob
		if rule == githubJobRule {
			if uses, ok := step.field("uses"); ok && uses != nil {
				v, _ := uses.text()
				if strings.HasPrefix(v, "actions/checkout@") {
					with, ok := step.field("with")
					if !ok || with != nil && (with.opaque || hasField(with, "path") || hasField(with, "repository")) {
						moved = true
					}
					checkedOut = true
				}
			}
			var ok bool
			if run, ok = step.field("run"); !ok {
				opaque = true
				continue
			}
			wd, ok := step.field("working-directory")
			if !ok {
				opaque = true
				continue
			}
			if wd != nil {
				dir = literalDirectory(wd)
			}
			shell, ok := step.field("shell")
			if !ok {
				opaque = true
				continue
			}
			if shell != nil {
				stepShell = shellOK(shell)
			}
		} else {
			if v, ok := step.text(); ok {
				checkedOut = checkedOut || v == "checkout"
				continue
			}
			if checkout, ok := step.field("checkout"); !ok {
				opaque = true
				continue
			} else if checkout != nil {
				checkedOut = true
				moved = moved || checkout.opaque || hasField(checkout, "path")
			}
			var ok bool
			if run, ok = step.field("run"); !ok {
				opaque = true
				continue
			}
			if run != nil && len(run.children) > 0 {
				command, ok1 := run.field("command")
				wd, ok2 := run.field("working_directory")
				shell, ok3 := run.field("shell")
				if !ok1 || !ok2 || !ok3 {
					opaque = true
					continue
				}
				if wd != nil {
					dir = literalDirectory(wd)
				}
				if shell != nil {
					stepShell = shellOK(shell)
				}
				run = command
			}
		}
		if run == nil {
			continue
		}
		lines, ok := run.script()
		if !ok {
			// A folded or unreadable script may publish: it is opaque when it names publish.
			if run.opaque || strings.Contains(scriptText(run), "publish") {
				opaque = true
			}
			continue
		}
		found, unread := scriptPublishes(lines, dir)
		switch {
		case !stepShell && (len(found) > 0 || unread):
			// Another shell may read the script differently.
			opaque = true
		case len(found) > 0 && (!checkedOut || moved):
			// What the step publishes is not this repository checked out at its root.
			opaque = true
		default:
			ties = append(ties, found...)
			opaque = opaque || unread
		}
	}
	return ties, opaque
}

func hasField(n *ciNode, key string) bool {
	c, ok := n.field(key)
	return !ok || c != nil
}

// scriptText is every line a node holds, for the publish mention check of an unreadable script.
func scriptText(n *ciNode) string {
	if n == nil {
		return ""
	}
	parts := []string{n.value}
	for _, l := range n.block {
		parts = append(parts, l.text)
	}
	for _, c := range n.children {
		parts = append(parts, scriptText(c))
	}
	return strings.Join(parts, "\n")
}

// shellWord is one word of a shell command: its literal text, whether quoting or expansion made
// any of it, and the 1-based CI file line it starts on.
type shellWord struct {
	text             string
	quoted, expanded bool
	line             int
}

// shellCommand is one simple command with the operators before and after it.
type shellCommand struct {
	words         []shellWord
	before, after string
}

// shellCommands splits a script into simple commands. structured is true when the script uses a
// compound command, a subshell or a function, and heredoc when it feeds a here-document, so its
// lines are not all commands; ok is false for an unterminated quote or substitution.
func shellCommands(lines []ciScriptLine) (cmds []shellCommand, structured, heredoc, ok bool) {
	var text strings.Builder
	var lineAt []int
	for _, l := range lines {
		text.WriteString(l.text + "\n")
		for range len(l.text) + 1 {
			lineAt = append(lineAt, l.line)
		}
	}
	s := text.String()
	var word strings.Builder
	cur := shellWord{}
	inWord, redirect := false, false
	cmd := shellCommand{}
	endWord := func() {
		if inWord {
			cur.text = word.String()
			if redirect {
				redirect = false
			} else {
				cmd.words = append(cmd.words, cur)
			}
		}
		word.Reset()
		cur, inWord = shellWord{}, false
	}
	start := func(i int) {
		if !inWord {
			inWord, cur.line = true, lineAt[i]
		}
	}
	endCommand := func(op string) {
		endWord()
		cmd.after = op
		if len(cmd.words) > 0 {
			cmds = append(cmds, cmd)
		}
		cmd = shellCommand{before: op}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			endWord()
		case c == '\n':
			endCommand("\n")
		case c == '#' && !inWord:
			for i+1 < len(s) && s[i+1] != '\n' {
				i++
			}
		case c == '\\':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
				continue
			}
			start(i)
			if i+1 < len(s) {
				i++
				word.WriteByte(s[i])
			}
			cur.quoted = true
		case c == '\'':
			start(i)
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return nil, structured, heredoc, false
			}
			word.WriteString(s[i+1 : i+1+end])
			cur.quoted, i = true, i+1+end
		case c == '"':
			start(i)
			cur.quoted = true
			i++
			for ; i < len(s) && s[i] != '"'; i++ {
				switch {
				case s[i] == '\\' && i+1 < len(s) && strings.IndexByte("\\\"$`\n", s[i+1]) >= 0:
					i++
				case s[i] == '$' || s[i] == '`':
					cur.expanded = true
				}
				word.WriteByte(s[i])
			}
			if i >= len(s) {
				return nil, structured, heredoc, false
			}
		case c == '$' && i+1 < len(s) && s[i+1] == '(' || c == '`':
			// A command substitution is one expanded word, read to its closing delimiter.
			start(i)
			cur.expanded = true
			if c == '`' {
				end := strings.IndexByte(s[i+1:], '`')
				if end < 0 {
					return nil, structured, heredoc, false
				}
				word.WriteString(s[i : i+2+end])
				i += 1 + end
				continue
			}
			depth, j := 0, i+1
			for ; j < len(s); j++ {
				if s[j] == '(' {
					depth++
				} else if s[j] == ')' {
					if depth--; depth == 0 {
						break
					}
				}
			}
			if j >= len(s) {
				return nil, structured, heredoc, false
			}
			word.WriteString(s[i : j+1])
			i = j
		case c == '$':
			start(i)
			cur.expanded = true
			word.WriteByte(c)
		case c == ';' || c == '&' || c == '|':
			op := string(c)
			if i+1 < len(s) && (s[i+1] == c || c == ';' && s[i+1] == '&' || c == '|' && s[i+1] == '&') {
				op += string(s[i+1])
				i++
			}
			if op == ";;" || op == ";&" {
				structured = true
			}
			endCommand(op)
		case c == '(' || c == ')':
			// A subshell, function definition or case pattern.
			structured = true
			endCommand(string(c))
		case c == '<' || c == '>':
			if inWord && strings.Trim(word.String(), "0123456789") == "" && !cur.quoted {
				word.Reset()
				inWord = false
			} else {
				endWord()
			}
			j := i
			for j+1 < len(s) && (s[j+1] == '<' || s[j+1] == '>' || s[j+1] == '|') {
				j++
			}
			if strings.Contains(s[i:j+1], "<<") {
				heredoc = true
			}
			if j+1 < len(s) && s[j+1] == '&' {
				// >&2 duplicates a descriptor; its target is the word that follows.
				j++
			}
			i = j
			redirect = true
		default:
			start(i)
			word.WriteByte(c)
		}
	}
	endCommand("")
	for _, c := range cmds {
		if w := c.words[0]; !w.quoted && shellReserved[w.text] {
			structured = true
		}
	}
	return cmds, structured, heredoc, true
}

var (
	shellReserved = map[string]bool{"if": true, "then": true, "else": true, "elif": true, "fi": true, "for": true, "while": true, "until": true, "do": true, "done": true, "case": true, "esac": true, "function": true, "select": true, "{": true, "}": true}
	shellPrefix   = map[string]bool{"if": true, "elif": true, "while": true, "until": true, "then": true, "do": true, "else": true, "!": true, "{": true}
	shellAssign   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// The publish options of npm, pnpm and yarn that take a value, that take none, and that choose
	// the package by a workspace or directory selector.
	publishValueOptions    = map[string]bool{"--tag": true, "--access": true, "--otp": true, "--registry": true, "--new-version": true, "--message": true, "--publish-branch": true, "--userconfig": true, "--cache": true}
	publishFlagOptions     = map[string]bool{"--provenance": true, "--no-git-checks": true, "--ignore-scripts": true, "--json": true, "--non-interactive": true, "--force": true, "--no-git-tag-version": true, "--no-commit-hooks": true, "--frozen-lockfile": true}
	publishSelectorOptions = map[string]bool{"--workspace": true, "--workspaces": true, "-w": true, "--filter": true, "-F": true, "-r": true, "--recursive": true, "--include-workspace-root": true, "--prefix": true, "-C": true, "--dir": true, "--cwd": true}
)

// scriptPublishes reads the publish commands of a POSIX shell script run in dir, following a
// literal cd (DCI-V1-021). unread is true when a publish command's directory cannot be read: an
// expression, a selector, an absolute, home or tarball argument, a cd the script may not run, a
// compound command or a here-document.
func scriptPublishes(lines []ciScriptLine, dir ciDir) (ties []publishTie, unread bool) {
	cmds, structured, heredoc, ok := shellCommands(lines)
	if !ok {
		for _, l := range lines {
			if strings.Contains(l.text, "publish") {
				return nil, true
			}
		}
		return nil, false
	}
	cwd := dir
	for _, c := range cmds {
		words := c.words
		for len(words) > 0 && !words[0].quoted && shellPrefix[words[0].text] {
			words = words[1:]
		}
		for len(words) > 0 && !words[0].quoted && shellAssign.MatchString(words[0].text) {
			words = words[1:]
		}
		if len(words) == 0 {
			continue
		}
		tool := words[0]
		if tool.expanded {
			// An expanded command word may be cd.
			cwd = ciDir{}
		} else if tool.text == "cd" || tool.text == "pushd" {
			plain := (c.before == "" || c.before == ";" || c.before == "\n" || c.before == "&&") && c.after != "|" && c.after != "&" && c.after != "|&"
			if !plain || structured || len(words) != 2 || words[1].expanded || words[1].text == "-" || !cwd.known {
				cwd = ciDir{}
				continue
			}
			next, ok := ciDirectory(cwd.dir, words[1].text)
			cwd = ciDir{next, ok}
			continue
		}
		if tool.text == "popd" {
			cwd = ciDir{}
			continue
		}
		publish, opaque, folder := publishCommand(words)
		switch {
		case opaque:
			unread = true
		case !publish:
		case heredoc || !cwd.known:
			unread = true
		default:
			target, ok := ciDirectory(cwd.dir, folder)
			if !ok {
				unread = true
				continue
			}
			ties = append(ties, publishTie{target, tool.line})
		}
	}
	return ties, unread
}

// publishCommand reads one simple command after quote removal: publish is true for an npm, pnpm
// or yarn publish whose package is the folder it returns ("." for the working directory); opaque
// is true for a publish whose package cannot be read, and for any other command that names
// publish, such as a wrapped, expanded or script-run one, except a print command.
func publishCommand(words []shellWord) (publish, opaque bool, folder string) {
	named := false
	for _, w := range words {
		named = named || strings.Contains(w.text, "publish")
	}
	other := func() (bool, bool, string) { return false, named, "" }
	tool := words[0]
	switch {
	case !tool.expanded && (tool.text == "echo" || tool.text == "printf"):
		return false, false, ""
	case tool.expanded || tool.text != "npm" && tool.text != "pnpm" && tool.text != "yarn":
		return other()
	}
	args := words[1:]
	i := 0
	for i < len(args) && strings.HasPrefix(args[i].text, "-") {
		i++
	}
	if i >= len(args) {
		return other()
	}
	berry := false
	switch {
	case args[i].expanded:
		return other()
	case args[i].text == "publish":
	case tool.text == "yarn" && args[i].text == "npm" && i+1 < len(args) && args[i+1].text == "publish":
		berry, i = true, i+1
	default:
		// npm run publish, yarn workspace x publish and every other subcommand.
		return other()
	}
	if i > 0 && !berry || berry && i > 1 {
		// An option before the subcommand, such as --prefix or -w, may choose the package.
		return false, true, ""
	}
	var positional []string
	unknown, options, dry := false, true, false
	for j := i + 1; j < len(args); j++ {
		w := args[j]
		if w.expanded {
			return false, true, ""
		}
		if options && w.text == "--" {
			options = false
			continue
		}
		if options && strings.HasPrefix(w.text, "-") {
			opt, _, eq := strings.Cut(w.text, "=")
			switch {
			case publishSelectorOptions[opt], opt == "--dry-run" && eq:
				return false, true, ""
			case opt == "--dry-run":
				dry = true
			case publishValueOptions[opt]:
				if !eq {
					j++
				}
			case !publishFlagOptions[opt]:
				unknown = true
			}
			continue
		}
		positional = append(positional, w.text)
	}
	switch {
	case dry:
		// A dry run publishes nothing.
		return false, false, ""
	case len(positional) == 0:
		return true, false, "."
	case len(positional) > 1 || berry || unknown:
		// berry's npm publish takes no folder; an unknown option may take the next word.
		return false, true, ""
	}
	p := positional[0]
	local := p == "." || p == ".." || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../")
	if !local || strings.HasSuffix(p, ".tgz") || strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tar") {
		// A bare word is a package spec, not a folder; an archive is no directory's manifest.
		return false, true, ""
	}
	return true, false, p
}

// ciDirectory joins a literal relative directory onto base, refusing an expression, an absolute
// or home path, whitespace or one that leaves the repository.
func ciDirectory(base, dir string) (string, bool) {
	if base == "" {
		base = "."
	}
	if dir == "" {
		dir = "."
	}
	for _, d := range []string{base, dir} {
		if strings.ContainsAny(d, "$`{}~*?[]\\ \t\n") || path.IsAbs(d) {
			return "", false
		}
	}
	clean := path.Clean(path.Join(base, dir))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

// environmentWords is the closed vocabulary a deploy job's name can name an environment by
// (DCI-V1-022, DCI-V1-027); the manifest can extend it (DCI-V1-028). A word is matched in any
// case and kept as written; none is mapped to another.
var (
	environmentWords = map[string]bool{
		"production": true, "prod": true, "prd": true, "preproduction": true, "preprod": true, "preprd": true,
		"staging": true, "stg": true, "uat": true, "qa": true, "sandbox": true, "sbx": true, "eph": true,
		"development": true, "dev": true,
	}
	environmentToken = regexp.MustCompile(`[A-Za-z0-9]+`)
)

// deployJobName reports whether a CI job key names a deploy job (DCI-V1-027): one of its words,
// split at "-", "_" and a lower-to-upper case change, is "deploy" in any case, unless the words
// before it end with "can" or "can i", as in a can-i-deploy contract check, which deploys
// nothing. "deployment", "redeploy" and "pact_can_deploy" name no deploy job.
func deployJobName(job string) bool {
	words := strings.FieldsFunc(strings.ToLower(caseBoundary.ReplaceAllString(job, "${1}-${2}")), func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		if w != "deploy" {
			continue
		}
		if canAsk := i > 0 && words[i-1] == "can" || i > 1 && words[i-1] == "i" && words[i-2] == "can"; !canAsk {
			return true
		}
	}
	return false
}

var caseBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// namedEnvironment is the one environment word text names, as written; ambiguous is true when it
// names two different words. "pre-prod", "pre_prod" and "pre prod" read as one word. declared
// holds the manifest's further words (DCI-V1-028).
func namedEnvironment(text string, declared map[string]bool) (word string, ambiguous bool) {
	spans := environmentToken.FindAllStringIndex(text, -1)
	found := ""
	for i := 0; i < len(spans); i++ {
		start, end := spans[i][0], spans[i][1]
		key := strings.ToLower(text[start:end])
		if key == "pre" && i+1 < len(spans) && spans[i+1][0] == end+1 && strings.ContainsRune("-_ ", rune(text[end])) {
			if next := strings.ToLower(text[spans[i+1][0]:spans[i+1][1]]); next == "prod" || next == "production" {
				key, end, i = key+next, spans[i+1][1], i+1
			}
		}
		if !environmentWords[key] && !declared[key] || strings.EqualFold(normalEnvironment(found), key) {
			continue
		}
		if found != "" {
			return "", true
		}
		found = text[start:end]
	}
	return found, false
}

// normalEnvironment is a written environment word as its vocabulary key.
func normalEnvironment(word string) string {
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(word))
}

// inferredEnvironment is the environment a deploy job without an environment key is named for:
// from its job key, else its workflow's top-level name, else its file name, with the source and
// the 1-based line to anchor (0 when the job's own anchor suffices). The first level that names
// two words, or whose name cannot be read, stops the inference: "" then, as when none names one.
func inferredEnvironment(root *ciNode, f inventoryFile, job string, declared map[string]bool) (environment, source string, line int) {
	if e, ambiguous := namedEnvironment(job, declared); ambiguous || e != "" {
		return e, "the job key", 0
	}
	name, ok := root.field("name")
	if !ok {
		return "", "", 0
	}
	if name != nil {
		v, ok := name.text()
		if !ok {
			return "", "", 0
		}
		if e, ambiguous := namedEnvironment(v, declared); ambiguous || e != "" {
			return e, fmt.Sprintf("the workflow name at line %d", name.line), name.line
		}
	}
	e, _ := namedEnvironment(strings.TrimSuffix(path.Base(f.path), path.Ext(f.path)), declared)
	return e, "the workflow file name", 0
}
