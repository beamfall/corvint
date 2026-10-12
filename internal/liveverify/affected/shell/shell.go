// Package shell is the shell-script implementation of the affected-selection
// language seam (NSA-V0-006..012).
//
// A unit is one shell file. Edges come from literal `source`, `.` and bats
// `load` targets and from literal invocations of repository shell files, read
// from source text with comments and heredoc bodies set aside. No shell is
// started and nothing is expanded, so the graph is rebuilt from text alone.
//
// The plugin deliberately does not implement affected.PathReader. A path
// reader takes part in every plan with a dirty path, so the frontier of a
// plugin present in nearly every polyglot repository would widen plans that no
// shell file bears on. Literal-reader selection through PathTokens works
// without it, and a unit selected that way, or by its unbounded reads, is
// reached, so the plugin takes part exactly when one of its units does.
package shell

import (
	"context"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Reasons this plugin can raise.
const (
	// FrontierDynamicSource is the unit frontier of a `source`, `.` or bats
	// `load` whose target is not a literal this plugin can resolve.
	FrontierDynamicSource = "shell:dynamic-source"
	// FrontierUnreadableSource is the global frontier for a shell file that
	// could not be read or is not valid UTF-8.
	FrontierUnreadableSource = "shell:unreadable-source"
	// FrontierForeignInvocation is the unit frontier of every non-test shell
	// unit when the source universe holds source text of another planned
	// language, whose programs may run a shell script by a computed path.
	FrontierForeignInvocation = "shell:foreign-invocation-unmodelled"
	// UnboundedRunsToolchain is the UnboundedReads reason of a shell test that
	// runs a build or language toolchain as a command (NSA-V0-011).
	UnboundedRunsToolchain = "shell:runs-toolchain"
)

// Suffixes are the file-name endings this plugin owns (NSA-V0-006).
var Suffixes = []string{".sh", ".bash", ".zsh", ".ksh", ".bats"}

// gitTopLevel is the git rev-parse option that prints the repository root, split
// so that this package's own source does not read as root-locating to the Go
// plugin (golang.rootLocatorToken).
var gitTopLevel = "--show-" + "toplevel"

// ForeignSourceSuffixes are the file-name endings of source text owned by the
// other planned languages (NSA-V0-012). A file with one of them in the source
// universe raises FrontierForeignInvocation on every non-test shell unit.
var ForeignSourceSuffixes = []string{
	".go", ".py", ".rb", ".rs", ".swift",
	".kt", ".kts", ".cs",
	".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx",
	".c", ".h", ".cc", ".cpp", ".cxx", ".c++", ".hh", ".hpp", ".hxx", ".h++",
	".inl", ".ipp", ".tpp", ".m", ".mm",
}

// Toolchains are the command words that make a shell test's reads unbounded
// (NSA-V0-011). A command word with a directory part matches by base name.
var Toolchains = []string{
	"go", "make", "cargo", "npm", "npx", "pnpm", "yarn", "node", "python",
	"python3", "ruby", "bundle", "swift", "xcodebuild", "gradle", "./gradlew",
	"mvn", "dotnet", "cmake", "ctest", "bazel",
}

// maxPathTokens bounds one unit's distinct path tokens. A unit over it keeps
// none and is marked PathTokensBounded (NSA-V0-010); tests lower it.
var maxPathTokens = affected.MaxPathsPerUnit

// Language observes shell files under one repository root.
type Language struct{}

// New returns the shell language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "shell" }

// Owns reports whether a path is shell source text by its suffix.
func (Language) Owns(relative string) bool { return hasAnySuffix(relative, Suffixes) }

// Units observes every shell file in the repository rooted at root.
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

// UnitsSource reads only the explicitly supplied source universe.
func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	files, err := root.Files(func(name string) bool { return hasAnySuffix(name, Suffixes) })
	if err != nil {
		return affected.Result{}, err
	}
	foreign, err := root.Files(func(name string) bool { return hasAnySuffix(name, ForeignSourceSuffixes) })
	if err != nil {
		return affected.Result{}, err
	}
	known := make(map[string]bool, len(files))
	for _, relative := range files {
		known[relative] = true
	}
	frontier := map[string]bool{}
	units := make([]affected.Unit, 0, len(files))
	for _, relative := range files {
		unit := affected.Unit{ID: "shell:" + relative}
		body, readErr := root.Read(relative)
		if readErr != nil || !utf8.Valid(body) {
			frontier[FrontierUnreadableSource] = true
			// The unit is kept, so its path stays owned and edges into it
			// resolve; a test-directory file whose body is unknown is a test.
			if isTestName(relative) || inTestDirectory(relative) {
				unit.Tests = []string{relative}
			} else {
				unit.Sources = []string{relative}
			}
			units = append(units, foreignFrontier(unit, len(foreign) > 0))
			continue
		}
		units = append(units, foreignFrontier(observe(relative, string(body), known), len(foreign) > 0))
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

func foreignFrontier(unit affected.Unit, foreign bool) affected.Unit {
	if foreign && len(unit.Tests) == 0 {
		reasons := map[string]bool{FrontierForeignInvocation: true}
		for _, reason := range unit.Frontier {
			reasons[reason] = true
		}
		unit.Frontier = sortedKeys(reasons)
	}
	return unit
}

// observe builds one readable file's unit.
func observe(relative, text string, known map[string]bool) affected.Unit {
	parsed := parse(text)
	resolve := newResolver(relative, parsed.commands)
	unit := affected.Unit{ID: "shell:" + relative}
	test := isTestName(relative) || (inTestDirectory(relative) && executesAtTopLevel(parsed.commands))
	if test {
		unit.Tests = []string{relative}
	} else {
		unit.Sources = []string{relative}
	}
	edges := map[string]bool{}
	unitFrontier := map[string]bool{}
	toolchain := false
	loads := strings.HasSuffix(relative, ".bats") || strings.HasSuffix(relative, ".bash")
	commands := parsed.commands
	for index := 0; index < len(commands); index++ {
		word, args := commandWord(plainWords(commands[index].words))
		switch {
		case word == "source" || word == ".":
			if len(args) == 0 || !resolve.link(unquote(args[0]), nil, known, edges) {
				unitFrontier[FrontierDynamicSource] = true
			}
		case word == "load" && loads:
			if len(args) == 0 || !resolve.link(unquote(args[0]), []string{"", ".bash"}, known, edges) {
				unitFrontier[FrontierDynamicSource] = true
			}
		case shells[word]:
			// A command string's commands are scanned as the file's own.
			if script, ok := commandString(args); ok {
				commands = append(commands, parse(script).commands...)
			}
			for _, target := range invokedScripts(word, args) {
				resolve.link(unquote(target), nil, known, edges)
			}
		case strings.Contains(word, "/") || strings.HasPrefix(word, "$"):
			resolve.link(word, nil, known, edges)
		}
		// A function the file defines shadows a toolchain of the same name.
		if isToolchain(word) && !parsed.functions[word] {
			toolchain = true
		}
	}
	delete(edges, unit.ID)
	unit.Imports = sortedKeys(edges)
	unit.Frontier = sortedKeys(unitFrontier)
	if test && toolchain {
		unit.UnboundedReads = UnboundedRunsToolchain
	}
	tokens := pathTokens(parsed)
	if len(tokens) > maxPathTokens {
		unit.PathTokensBounded = true
	} else {
		unit.PathTokens = sortedKeys(tokens)
	}
	return unit
}

// shells are the interpreters whose script argument is an invocation edge.
var shells = map[string]bool{"bash": true, "sh": true, "zsh": true, "ksh": true, "dash": true, "bats": true}

// commandString returns the operand of an interpreter's `-c` option.
func commandString(args []string) (string, bool) {
	for index, arg := range args {
		if unquote(arg) == "-c" && index+1 < len(args) {
			return unquote(args[index+1]), true
		}
	}
	return "", false
}

// invokedScripts lists an interpreter command's script operands: the first
// operand of a shell, or every operand of bats. `-c` names a command string,
// not a script.
func invokedScripts(word string, args []string) []string {
	var targets []string
	for index := 0; index < len(args); index++ {
		value := unquote(args[index])
		if value == "-c" {
			return nil
		}
		if value == "-o" || value == "-O" || value == "+o" || value == "+O" {
			index++
			continue
		}
		if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
			continue
		}
		targets = append(targets, args[index])
		if word != "bats" {
			break
		}
	}
	return targets
}

func isToolchain(word string) bool {
	for _, name := range Toolchains {
		if word == name || (strings.Contains(word, "/") && path.Base(word) == path.Base(name)) {
			return true
		}
	}
	return false
}

// commandPrefixes run their operand as the command. Their own options are
// skipped; timeout's duration is its first operand.
var commandPrefixes = map[string]bool{
	"exec": true, "command": true, "builtin": true, "nohup": true, "time": true,
	"sudo": true, "env": true, "run": true, "xcrun": true, "timeout": true,
	"!": true, "if": true, "then": true, "else": true, "elif": true, "do": true,
	"while": true, "until": true,
}

// commandWord returns the word a simple command runs, unquoted, and its
// operands, past leading assignments and command prefixes.
func commandWord(words []string) (string, []string) {
	for index := 0; index < len(words); index++ {
		raw := words[index]
		if assignment.MatchString(raw) {
			continue
		}
		word := unquote(raw)
		if !commandPrefixes[word] {
			return word, words[index+1:]
		}
		// `command -v NAME` looks a command up without running it.
		if word == "command" && index+1 < len(words) {
			if option := unquote(words[index+1]); option == "-v" || option == "-V" {
				return word, words[index+1:]
			}
		}
		for index+1 < len(words) && strings.HasPrefix(unquote(words[index+1]), "-") {
			index++
		}
		if word == "timeout" && index+1 < len(words) {
			index++
		}
	}
	return "", nil
}

// helperCommands may appear at the top level of a sourced helper without
// making it executable test code (NSA-V0-007).
var helperCommands = map[string]bool{
	"": true, "source": true, ".": true, "load": true, "bats_load_library": true,
	"export": true, "readonly": true, "declare": true, "typeset": true,
	"local": true, "set": true, "shopt": true, "unset": true, ":": true,
	"true": true, "return": true, "alias": true, "trap": true,
	"fi": true, "done": true, "esac": true,
}

// executesAtTopLevel reports a file that runs a command outside its function
// bodies, so it is executable test code rather than a helper of functions.
func executesAtTopLevel(commands []command) bool {
	for _, cmd := range commands {
		if cmd.inFunction || cmd.substituted {
			continue
		}
		word, _ := commandWord(plainWords(cmd.words))
		if !helperCommands[word] {
			return true
		}
	}
	return false
}

// isTestName applies the file-name test conventions of NSA-V0-007.
func isTestName(relative string) bool {
	name := path.Base(relative)
	if strings.HasSuffix(name, ".bats") {
		return true
	}
	stem := strings.TrimSuffix(name, path.Ext(name))
	return strings.HasSuffix(stem, "_test") || strings.HasSuffix(stem, "-test") ||
		strings.HasPrefix(stem, "test_") || strings.HasPrefix(stem, "test-")
}

func inTestDirectory(relative string) bool {
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "test" || component == "tests" {
			return true
		}
	}
	return false
}

// assignment matches a shell variable assignment word.
var assignment = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(\[[^]]*\])?(\+?)=`)

// declarers take assignments as operands.
var declarers = map[string]bool{"local": true, "export": true, "readonly": true, "declare": true, "typeset": true}

// resolver turns a target word into repository paths for one file.
type resolver struct {
	directory string
	values    map[string]string
	counts    map[string]int
	resolving map[string]bool
}

func newResolver(relative string, commands []command) *resolver {
	directory := path.Dir(relative)
	if directory == "." {
		directory = ""
	}
	r := &resolver{directory: directory, values: map[string]string{}, counts: map[string]int{}, resolving: map[string]bool{}}
	for _, cmd := range commands {
		words := plainWords(cmd.words)
		declaring := false
		for index, raw := range words {
			value := unquote(raw)
			if index == 0 && declarers[value] {
				declaring = true
				continue
			}
			match := assignment.FindStringSubmatch(raw)
			if match == nil {
				if declaring && strings.HasPrefix(value, "-") {
					continue
				}
				if declaring {
					// A bare declared name (`local dir`) is an assignment
					// of its own, so a later value is not the only one.
					r.counts[value]++
					continue
				}
				break
			}
			name := match[1]
			r.counts[name]++
			if match[2] != "" || match[3] != "" {
				r.counts[name]++
			}
			r.values[name] = unquote(raw[len(match[0]):])
		}
	}
	return r
}

// link adds an edge for each owned shell file that target names, trying each
// suffix in turn, and reports whether target was a literal: a literal outside
// the repository or naming no file is resolved, with no edge.
func (r *resolver) link(target string, suffixes []string, known map[string]bool, edges map[string]bool) bool {
	candidates, literal := r.candidates(target)
	if !literal {
		return false
	}
	if suffixes == nil {
		suffixes = []string{""}
	}
	for _, candidate := range candidates {
		for _, suffix := range suffixes {
			if known[candidate+suffix] {
				edges["shell:"+candidate+suffix] = true
			}
		}
	}
	return true
}

// candidates lists the repository paths a literal target may name: the one a
// directory form fixes, or the target against the file's directory and the
// repository root.
func (r *resolver) candidates(target string) ([]string, bool) {
	if base, rest, ok := r.base(target, 0); ok {
		resolved, within := join(base, rest)
		if !within {
			return nil, !dynamic(rest)
		}
		return []string{resolved}, true
	}
	if target == "" || dynamic(target) {
		return nil, false
	}
	if strings.HasPrefix(target, "/") {
		return nil, true
	}
	var candidates []string
	for _, base := range []string{r.directory, ""} {
		if resolved, within := join(base, "/"+target); within && resolved != "" {
			candidates = append(candidates, resolved)
		}
	}
	return candidates, true
}

// dynamic reports text whose value is decided at run time.
func dynamic(text string) bool {
	return strings.ContainsAny(text, "$`*?[~")
}

// join appends rest, empty or slash-led and literal, to a repository
// directory, and reports whether the result stays inside the repository.
func join(base, rest string) (string, bool) {
	if rest != "" && (!strings.HasPrefix(rest, "/") || dynamic(rest)) {
		return "", false
	}
	joined := path.Join(base, strings.TrimPrefix(rest, "/"))
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return "", false
	}
	if joined == "." {
		joined = ""
	}
	return joined, true
}

// scriptPaths are the expansions that name the running script's own file.
var scriptPaths = map[string]bool{
	"$0": true, "${0}": true, "$BASH_SOURCE": true, "${BASH_SOURCE}": true,
	"${BASH_SOURCE[0]}": true, "$BATS_TEST_FILENAME": true, "${BATS_TEST_FILENAME}": true,
}

// directoryForms are the parameter expansions that name the script's own
// directory.
var directoryForms = []string{
	"${0%/*}", "${BASH_SOURCE%/*}", "${BASH_SOURCE[0]%/*}", "${BATS_TEST_FILENAME%/*}",
	"${BATS_TEST_DIRNAME}", "$BATS_TEST_DIRNAME",
}

var variable = regexp.MustCompile(`^\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))`)

// base matches a leading directory form of an unquoted target: the script's
// directory, the repository root, `cd FORM && pwd`, or a variable assigned
// once from one of those. It returns the repository directory and the rest.
func (r *resolver) base(target string, depth int) (string, string, bool) {
	if depth > 8 {
		return "", "", false
	}
	for _, form := range directoryForms {
		if strings.HasPrefix(target, form) && !nameContinues(target[len(form):], form) {
			return r.directory, target[len(form):], true
		}
	}
	if inner, rest, ok := substitution(target); ok {
		directory, ok := r.substituted(inner, depth)
		return directory, rest, ok
	}
	match := variable.FindStringSubmatch(target)
	if match == nil {
		return "", "", false
	}
	name := match[1] + match[2]
	if r.counts[name] != 1 || r.resolving[name] {
		return "", "", false
	}
	r.resolving[name] = true
	defer delete(r.resolving, name)
	directory, ok := r.pathOf(r.values[name], depth+1)
	return directory, target[len(match[0]):], ok
}

// nameContinues reports a bare `$NAME` form followed by more name characters,
// which is a different variable.
func nameContinues(rest, form string) bool {
	if strings.HasSuffix(form, "}") || rest == "" {
		return false
	}
	c := rest[0]
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// pathOf evaluates a whole unquoted value as a repository path, usually a
// directory; a variable holding a script's path names that file.
func (r *resolver) pathOf(value string, depth int) (string, bool) {
	base, rest, ok := r.base(value, depth)
	if !ok {
		return "", false
	}
	return join(base, strings.TrimSuffix(rest, "/"))
}

// substitution splits a leading `$(...)` or backquoted command from the rest.
func substitution(text string) (string, string, bool) {
	if strings.HasPrefix(text, "`") {
		end := strings.IndexByte(text[1:], '`')
		if end < 0 {
			return "", "", false
		}
		return text[1 : end+1], text[end+2:], true
	}
	if !strings.HasPrefix(text, "$(") || strings.HasPrefix(text, "$((") {
		return "", "", false
	}
	depth := 0
	for index := 1; index < len(text); index++ {
		switch text[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return text[2:index], text[index+1:], true
			}
		}
	}
	return "", "", false
}

var redirection = regexp.MustCompile(`\s*[0-9]*(?:>>?|<|&>)\s*(?:&[0-9]+|/dev/null)`)

// substituted evaluates the commands a directory form substitutes:
// `dirname SCRIPT`, `git rev-parse` with its top-level option, `realpath DIR`,
// `readlink -f DIR`, and `cd DIR && pwd`.
func (r *resolver) substituted(inner string, depth int) (string, bool) {
	parts := splitTop(redirection.ReplaceAllString(inner, ""))
	switch len(parts) {
	case 1:
		words := fieldsTop(parts[0])
		words = dropFlags(words)
		if len(words) == 2 && words[0] == "dirname" && r.scriptPath(words[1], depth) {
			return r.directory, true
		}
		if len(words) == 3 && words[0] == "git" && words[1] == "rev-parse" && words[2] == gitTopLevel {
			return "", true
		}
		if len(words) == 2 && (words[0] == "realpath" || words[0] == "readlink" || words[0] == "greadlink") {
			return r.pathOf(words[1], depth+1)
		}
	case 2:
		cd := dropFlags(dropAssignments(fieldsTop(parts[0])))
		pwd := dropFlags(fieldsTop(parts[1]))
		if len(cd) == 2 && cd[0] == "cd" && len(pwd) == 1 && pwd[0] == "pwd" {
			return r.pathOf(cd[1], depth+1)
		}
	}
	return "", false
}

// scriptPath reports an expansion naming the running script's file, directly
// or through realpath or readlink.
func (r *resolver) scriptPath(text string, depth int) bool {
	if scriptPaths[text] {
		return true
	}
	inner, rest, ok := substitution(text)
	if !ok || rest != "" || depth > 8 {
		return false
	}
	words := dropFlags(fieldsTop(inner))
	return len(words) == 2 && (words[0] == "realpath" || words[0] == "readlink" || words[0] == "greadlink") && r.scriptPath(words[1], depth+1)
}

// dropAssignments drops a command's leading environment assignments.
func dropAssignments(words []string) []string {
	for len(words) > 0 && assignment.MatchString(words[0]) {
		words = words[1:]
	}
	return words
}

func dropFlags(words []string) []string {
	kept := words[:0:0]
	for _, word := range words {
		if !strings.HasPrefix(word, "-") || word == gitTopLevel {
			kept = append(kept, word)
		}
	}
	return kept
}

// splitTop splits an unquoted command at `&&` and `;` outside parentheses.
func splitTop(text string) []string {
	var parts []string
	depth, start := 0, 0
	for index := 0; index < len(text); index++ {
		switch {
		case text[index] == '(':
			depth++
		case text[index] == ')':
			depth--
		case depth == 0 && text[index] == ';':
			parts = append(parts, text[start:index])
			start = index + 1
		case depth == 0 && strings.HasPrefix(text[index:], "&&"):
			parts = append(parts, text[start:index])
			start = index + 2
			index++
		}
	}
	parts = append(parts, text[start:])
	kept := parts[:0]
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, part)
		}
	}
	return kept
}

// fieldsTop splits at blanks outside parentheses and braces.
func fieldsTop(text string) []string {
	var fields []string
	depth, start := 0, -1
	for index := 0; index <= len(text); index++ {
		if index < len(text) {
			switch text[index] {
			case '(', '{':
				depth++
			case ')', '}':
				depth--
			}
		}
		blank := index == len(text) || (depth == 0 && (text[index] == ' ' || text[index] == '\t' || text[index] == '\n'))
		if blank {
			if start >= 0 {
				fields = append(fields, text[start:index])
				start = -1
			}
		} else if start < 0 {
			start = index
		}
	}
	return fields
}

// unquote removes quoting from a word: quote characters and backslash escapes.
// Expansions are left in place.
func unquote(word string) string {
	if !strings.ContainsAny(word, `'"\`) {
		return word
	}
	var out strings.Builder
	for index := 0; index < len(word); index++ {
		switch word[index] {
		case '\'', '"':
		case '\\':
			if index+1 < len(word) {
				index++
				out.WriteByte(word[index])
			}
		default:
			out.WriteByte(word[index])
		}
	}
	return out.String()
}

// Path tokens use the AFP-V0-012 lexicon after expansions are blanked; a token
// is path-shaped when it carries a slash or a dot and is not a bare number.
var (
	expansion = regexp.MustCompile(`\$\{[^}]*\}|\$\([^)]*\)|\$[A-Za-z_][A-Za-z0-9_]*|\$[0-9@*#?$!-]`)
	pathToken = regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)
)

func pathTokens(parsed script) map[string]bool {
	tokens := map[string]bool{}
	add := func(text string) {
		for _, token := range pathToken.FindAllString(expansion.ReplaceAllString(text, " "), -1) {
			if pathShaped(token) {
				tokens[token] = true
			}
		}
	}
	for _, cmd := range parsed.commands {
		for _, word := range cmd.words {
			if word != redirect {
				add(word)
			}
		}
	}
	for _, body := range parsed.data {
		add(body)
	}
	return tokens
}

func pathShaped(token string) bool {
	if !strings.ContainsAny(token, "/.") || strings.Trim(token, "0123456789.") == "" {
		return false
	}
	return strings.IndexFunc(token, func(c rune) bool {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
	}) >= 0
}

func hasAnySuffix(name string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
