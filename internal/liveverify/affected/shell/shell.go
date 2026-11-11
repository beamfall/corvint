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
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Reasons this plugin can raise.
const (
	// UnboundedDynamicSource marks the unbounded reads of a `source`, `.` or bats
	// `load` whose target is not a literal this plugin can resolve.
	UnboundedDynamicSource = "shell:dynamic-source"
	// FrontierUnreadableSource is the global frontier for a shell file that
	// could not be read or is not valid UTF-8.
	FrontierUnreadableSource = "shell:unreadable-source"
	// FrontierForeignInvocation is the unit frontier of every non-test shell
	// unit when the source universe holds source text of another planned
	// language, whose programs may run a shell script by a computed path. Such
	// a program is an unseen dependent, so the reason bears only on a plan
	// that follows the unit's dependents (AFP-V0-581).
	FrontierForeignInvocation = "shell:foreign-invocation-unmodelled"
	// UnboundedRunsToolchain is the UnboundedReads reason of a shell test that
	// runs a build or language toolchain as a command (NSA-V0-011).
	UnboundedRunsToolchain = "shell:runs-toolchain"
	// UnboundedTreeWalk is the UnboundedReads reason of a shell test that
	// lists, walks or searches the tree from its working directory: `find`
	// with no path or `.`, recursive `grep`, `rg`, `ag` or `ack` with no path
	// or `.`, `du`, `tree` or `ls -R` likewise. What it reads changes with
	// every added, deleted or edited file (AFP-V0-9134). A helper that walks
	// the tree keeps its bounded relation: repository contracts hold helper
	// sources bounded (V1-4283), and the test that runs it is selected by its
	// own reads.
	UnboundedTreeWalk = "shell:tree-walk"
)

// Suffixes are the file-name endings this plugin owns (NSA-V0-006).
var Suffixes = affected.ShellSuffixes

// HiddenRoots are the hidden top-level directories whose shell files the
// plugin indexes although the shared source walk skips every hidden
// directory: CI keeps its scripts there, such as .github/forward-rows/*.sh,
// and an owned file no unit declares is UNINDEXED_SOURCE_PATH, which forces a
// full run (NSA-V0-800).
var HiddenRoots = []string{".github"}

// gitTopLevel is the git rev-parse option that prints the repository root, split
// so that this package's own source does not read as root-locating to the Go
// plugin (golang.rootLocatorToken).
var gitTopLevel = "--show-" + "toplevel"

// ForeignSourceSuffixes are the file-name endings of source text owned by the
// other planned languages (NSA-V0-012). A file with one of them in the source
// universe raises FrontierForeignInvocation on every non-test shell unit.
var ForeignSourceSuffixes = []string{
	".go", ".py", ".rb", ".rs", ".swift",
	".kt", ".kts", ".java", ".cs",
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
func (Language) Owns(relative string) bool {
	return hasAnySuffix(relative, Suffixes) || ExtensionlessScript(relative)
}

// ExtensionlessRoots are the top-level directories whose extensionless files
// the plugin owns as executables (AFP-V0-2205): `script/release-checklist`
// and its kind are run by path like any `.sh` file.
var ExtensionlessRoots = []string{"bin", "script", "scripts"}

// ExtensionlessScript reports a file directly under an ExtensionlessRoots
// directory whose name has no dot. The plugin builds a unit for one whose
// first line is a shell shebang (shellShebang); any other such file stays
// owned with no unit, so an edit to it is UNINDEXED_SOURCE_PATH.
func ExtensionlessScript(relative string) bool {
	return slices.Contains(ExtensionlessRoots, path.Dir(relative)) && !strings.Contains(path.Base(relative), ".")
}

// shellShebang reports a body whose first line runs sh, bash, dash, zsh or
// ksh, directly or through env.
func shellShebang(body []byte) bool {
	line, _, _ := strings.Cut(string(body[:min(len(body), 256)]), "\n")
	fields := strings.Fields(strings.TrimPrefix(line, "#!"))
	if !strings.HasPrefix(line, "#!") || len(fields) == 0 {
		return false
	}
	program := path.Base(fields[0])
	if program == "env" {
		rest := fields[1:]
		for len(rest) != 0 && strings.HasPrefix(rest[0], "-") {
			rest = rest[1:]
		}
		if len(rest) == 0 {
			return false
		}
		program = path.Base(rest[0])
	}
	switch program {
	case "sh", "bash", "dash", "zsh", "ksh":
		return true
	}
	return false
}

// Units observes every shell file in the repository rooted at root.
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

// UnitsSource reads only the explicitly supplied source universe.
func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	files, err := root.Files(func(relative, name string) bool {
		return hasAnySuffix(name, Suffixes) || ExtensionlessScript(relative)
	})
	if err != nil {
		return affected.Result{}, err
	}
	files, err = shellExecutables(root, files)
	if err != nil {
		return affected.Result{}, err
	}
	hidden, err := hiddenFiles(root)
	if err != nil {
		return affected.Result{}, err
	}
	files = append(files, hidden...)
	foreign, err := root.Files(func(_, name string) bool { return hasAnySuffix(name, ForeignSourceSuffixes) })
	if err != nil {
		return affected.Result{}, err
	}
	known := make(map[string]bool, len(files))
	for _, relative := range files {
		known[relative] = true
	}
	frontier, built, toolchainOnly := map[string]bool{}, map[string]computedNames{}, map[string]bool{}
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
		unit, names, dynamic := observe(relative, string(body), known)
		toolchainOnly[relative] = !dynamic
		if !names.empty() {
			built[relative] = names
		}
		units = append(units, foreignFrontier(unit, len(foreign) > 0))
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	applyForeignCallers(root, units, built, frontier)
	applyShellReadScopes(root, units, toolchainOnly, frontier)
	applyShellTestInputs(root, units, frontier)
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

// shellExecutables drops each extensionless file whose first line is not a
// shell shebang (AFP-V0-2205); a file it cannot read is kept, so the loop
// below records it as unreadable as it does any shell file.
func shellExecutables(root *affected.Source, files []string) ([]string, error) {
	kept := files[:0]
	for _, relative := range files {
		if hasAnySuffix(relative, Suffixes) {
			kept = append(kept, relative)
			continue
		}
		body, err := root.Read(relative)
		if err == nil && !shellShebang(body) {
			continue
		}
		kept = append(kept, relative)
	}
	return kept, root.Err()
}

// hiddenFiles lists the shell files under each HiddenRoots directory through
// Source.FilesUnder, which skips hidden directories below it as the shared
// walk does and stamps each listed directory, so the graph stays cacheable
// (NSA-V0-800). An absent root contributes nothing.
func hiddenFiles(root *affected.Source) ([]string, error) {
	var files []string
	for _, directory := range HiddenRoots {
		found, err := root.FilesUnder(directory, func(name string) bool { return hasAnySuffix(name, Suffixes) })
		if err != nil {
			return nil, err
		}
		files = append(files, found...)
	}
	return files, nil
}

func foreignFrontier(unit affected.Unit, foreign bool) affected.Unit {
	if foreign && len(unit.Tests) == 0 {
		reasons := map[string]bool{FrontierForeignInvocation: true}
		for _, reason := range unit.Frontier {
			reasons[reason] = true
		}
		unit.Frontier = sortedKeys(reasons)
		if len(unit.Frontier) == 1 {
			unit.FrontierBears = affected.FrontierBearsOnDependents
		}
	}
	return unit
}

// observe builds one readable file's unit and returns the script names it
// builds without a resolvable literal (TSP-V0-960) and whether a source it
// runs is dynamic, which a toolchain test's UnboundedReads does not show.
func observe(relative, text string, known map[string]bool) (affected.Unit, computedNames, bool) {
	parsed := parse(text)
	resolve := newResolver(relative, parsed.commands)
	resolve.text = text
	values := variableValues(parsed.commands)
	var names computedNames
	builds := false
	unit := affected.Unit{ID: "shell:" + relative}
	test := isTestName(relative) || (inTestDirectory(relative) && executesAtTopLevel(parsed.commands))
	if test {
		unit.Tests = []string{relative}
	} else {
		unit.Sources = []string{relative}
	}
	edges := map[string]bool{}
	unitFrontier := map[string]bool{}
	toolchain, dynamic, treeWalk := false, false, false
	// contentWalk marks a tree walk that is no admitted name-listing read
	// (AFP-V0-9963).
	contentWalk := false
	loads := strings.HasSuffix(relative, ".bats") || strings.HasSuffix(relative, ".bash")
	// computed links a run target or expands it as a computed name
	// (TSP-V0-960). An expansion the value bound cut short may drop a script
	// it would link, so the unit is then an unbounded reader like a computed
	// source.
	computed := func(target string) {
		if resolve.link(target, nil, known, edges) {
			return
		}
		unresolved, cut := constructed(target, resolve, values, known, edges, &names)
		builds = builds || unresolved
		dynamic = dynamic || cut
	}
	// A here-document that no delimiter line ended is read differently by
	// different shells, so the file is held like a computed source
	// (AFP-V0-9971).
	if parsed.incomplete {
		dynamic = true
	}
	temp := tempRoots(resolve, parsed, text)
	privateRoots := rootsWith(resolve, parsed, text, exitGuardedAssignment)
	commands := parsed.commands
	for index := 0; index < len(commands); index++ {
		plain := plainWords(commands[index].words)
		word, args := commandWord(plain)
		// A wrapper's own option values can hide the command it runs
		// (`sudo -u root bash x.sh`, `nice -n 5 go test`), and a wrapper
		// commandPrefixes lacks hides it outright; the first known command
		// after the wrapper is classified too, in addition to the command
		// word, and a repository script it names is linked (AFP-V0-9975).
		// This only adds holds and edges.
		runs := []invocation{{word, args}}
		if inner, rest, ok := wrapped(plain); ok && inner != word {
			runs = append(runs, invocation{inner, rest})
		}
		for _, operand := range wrapperOperands(plain) {
			resolve.link(operand, nil, known, edges)
		}
		// An interpreter startup name makes the next interpreter read a
		// file no word names, like a computed source (AFP-V0-9976).
		if setsStartupName(commands[index].words) {
			dynamic = true
		}
		private := privateDirectory(commands, len(parsed.commands), index, privateRoots)
		for _, run := range runs {
			word, args := run.word, run.args
			// An awk or sed program read from a file reads what no word of this
			// file names (AFP-V0-9974); git over a repository not proven private
			// reads every path (AFP-V0-9973).
			if programFile(word, args) {
				toolchain = true
			}
			if gitWalksTree(plain, word, args, privateRoots, private) {
				treeWalk = true
			}
			switch {
			case word == "source" || word == ".":
				// A source no literal, fresh directory or checked host guard resolves is dynamic,
				// an environment-named one (`. "$ENVFILE"`) included: nothing in
				// the file bounds what the caller's value names (AFP-V0-2953).
				if len(args) == 0 || !resolve.link(unquote(args[0]), nil, known, edges) && !freshDirectorySource(resolve, parsed, text, index, unquote(args[0])) && !hostEnvironmentSource(resolve, parsed, text, index) {
					dynamic = true
				}
			case word == "load" && loads:
				if len(args) == 0 || !resolve.link(unquote(args[0]), []string{"", ".bash"}, known, edges) {
					dynamic = true
				}
			case shells[word]:
				// A command string's commands are scanned as the file's own.
				if script, ok := commandString(args); ok {
					inner := parse(script)
					commands = append(commands, inner.commands...)
					// A here-document no delimiter ends holds the file as
					// in its own text (AFP-V0-9971).
					dynamic = dynamic || inner.incomplete
					if index < len(parsed.commands) && strings.ContainsAny(script, "$`") {
						// A command string held in a variable is scanned for each
						// value the file gives it (AFP-V0-553).
						// A value bound that cut the list may have dropped a
						// command string that runs a script, so the unit is then
						// a dynamic source (AFP-V0-3147).
						scanned, cut := commandStrings(script, values)
						commands = append(commands, scanned...)
						dynamic = dynamic || cut
					}
				}
				for _, target := range invokedScripts(word, args) {
					computed(unquote(target))
				}
			case strings.Contains(word, "/") || strings.HasPrefix(word, "$"):
				computed(word)
			}
			if isToolchain(word) && (bypassesFunctions(commands[index].words) || !shadowed(parsed, word, index)) {
				toolchain = true
			}
			if walksTree(word, args) {
				treeWalk = true
				contentWalk = contentWalk || !nameListingCommand(parsed, index)
			}
		}
	}
	listings := nameListingAdmitted(parsed, test)
	delete(edges, unit.ID)
	unit.Imports = sortedKeys(edges)
	if builds && !test {
		// What runs the script it builds is unmodelled (TSP-V0-960), so the
		// caller keeps the frontier, bearing as the foreign one does
		// (AFP-V0-581). A test is a leaf: nothing runs a script through it.
		unitFrontier[FrontierForeignInvocation] = true
		unit.FrontierBears = affected.FrontierBearsOnDependents
	}
	unit.Frontier = sortedKeys(unitFrontier)
	switch {
	case test && toolchain:
		unit.UnboundedReads = UnboundedRunsToolchain
	case dynamic:
		// What a computed path sources is unknown, so the unit is selected on
		// any dirty path, and so is every unit that sources a helper doing it.
		unit.UnboundedReads = UnboundedDynamicSource
		unit.LocatesRoot = !test
	case test && treeWalk:
		// A walk from the working directory reads a set no token names: a
		// file added or deleted anywhere changes a listing, and an edit
		// anywhere changes a search (AFP-V0-9134). A walk that only lists
		// names cannot observe an edit, so the planner narrows it by change
		// status (AFP-V0-9963).
		unit.UnboundedReads = UnboundedTreeWalk
		unit.UnboundedNamesOnly = listings && !contentWalk
	}
	unit.PlacesFileNames = placesFileNames(commands, len(parsed.commands), resolve.placement(test && !strings.HasSuffix(relative, ".bats")))
	tokens, globs, listed := pathTokens(parsed, temp, test, listings, !redefinesUtilities(parsed, text))
	unit.PathGlobs = sortedKeys(globs)
	if len(tokens)+len(globs) > maxPathTokens {
		unit.PathGlobs = nil
		unit.PathTokensBounded = true
	} else {
		unit.PathTokens = sortedKeys(tokens)
		if len(listed) != 0 {
			unit.NameListings = sortedKeys(listed)
		}
	}
	return unit, names, dynamic
}

// walkerPaths returns the path operands of a listing or walking command, by
// name: for find the words before its first expression word, for grep the
// operands after its pattern unless -e, -f or their long forms supply it, for
// rg, ag and ack the operands after the pattern, and for ls, du and tree every
// non-option word. recursive reports a command that reads below its paths:
// every one but ls without -R. ok is false for any other command, and for
// grep without a recursive option, which reads its operands as files
// (AFP-V0-9133, AFP-V0-9134).
//
// consume selects the other reading of option values: every option that
// walkerTakesValue cannot prove takes none consumes the next word that is not
// itself an option, as `grep -rm 1 PATTERN` or `du -d 1` do. The reading
// without consume names every word that may be an operand; this one names the
// fewest (AFP-V0-9972).
func walkerPaths(word string, args []string, consume bool) (paths []string, recursive, ok bool) {
	var operands []string
	var options []string
	ended := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		value := unquote(arg)
		switch {
		case ended || !walkerOption(arg):
			operands = append(operands, value)
		case value == "--":
			ended = true
		default:
			options = append(options, value)
			if consume && walkerTakesValue(path.Base(word), value) && index+1 < len(args) && !walkerOption(args[index+1]) {
				index++
			}
		}
	}
	switch path.Base(word) {
	case "find":
		for _, arg := range args {
			value := unquote(arg)
			if strings.HasPrefix(value, "-") || value == "(" || value == "!" || value == "\\(" {
				break
			}
			paths = append(paths, value)
		}
		return paths, true, true
	case "grep", "egrep", "fgrep", "zgrep":
		if !slices.ContainsFunc(options, grepOptions.matches) {
			return nil, false, false
		}
		pattern := !slices.ContainsFunc(options, func(option string) bool {
			return strings.HasPrefix(option, "-e") || strings.HasPrefix(option, "-f") || strings.HasPrefix(option, "--regexp") || strings.HasPrefix(option, "--file")
		})
		if pattern && len(operands) != 0 {
			operands = operands[1:]
		}
		return operands, true, true
	case "rg", "ag", "ack":
		if len(operands) != 0 {
			operands = operands[1:]
		}
		return operands, true, true
	case "ls":
		return operands, slices.ContainsFunc(options, recursiveFlags["ls"].matches), true
	case "du", "tree":
		return operands, true, true
	}
	return nil, false, false
}

// walkerOption reports an unquoted word spelled as an option: `-` followed by
// option characters only. A quoted word, or one holding a space, is a pattern
// or path even when it starts with `-` (`grep -Fq -- "- N-1 ..."`).
func walkerOption(raw string) bool {
	if !strings.HasPrefix(raw, "-") || raw == "-" {
		return false
	}
	return strings.IndexFunc(raw[1:], func(c rune) bool {
		return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '=' || c == '_' || c == '.' || c == '/' || c == '*' || c == ',' || c == ':')
	}) < 0
}

// walksTree reports a recursive listing, walking or searching command rooted
// at the working directory: one with no path operand, or with `.` or `./`
// among them (AFP-V0-9134). A command whose operands are all computed is not
// reported: an unknown operand keeps the existing conservative relation, and
// a private temporary root is no repository path. A plain `ls` of the working
// directory reads one directory's names, not the tree, and is not reported.
//
// Both readings of option values are tried (AFP-V0-9972): the one that takes
// no option value names every word that may be a path, so a `.` operand is
// never missed, and the one that consumes a value after every option not
// proven to take none names the fewest, so a walk with no path operand is
// never missed either.
func walksTree(word string, args []string) bool {
	for _, consume := range []bool{false, true} {
		paths, recursive, ok := walkerPaths(word, args, consume)
		if !ok || !recursive {
			continue
		}
		if len(paths) == 0 || slices.Contains(paths, ".") || slices.Contains(paths, "./") {
			return true
		}
	}
	return false
}

// walkerGrammar names the options of a walking command known to take no
// value: short letters, which may be grouped, and long names. Every other
// option may take one (AFP-V0-9972).
type walkerGrammar struct {
	letters string
	long    []string
}

var walkerGrammars = map[string]walkerGrammar{
	"grep": {"0123456789EFGPiywxvcLloqsbHhnTZzrRaIUuV", []string{
		"count", "files-with-matches", "files-without-match", "line-number", "quiet", "silent",
		"recursive", "dereference-recursive", "ignore-case", "no-ignore-case", "invert-match",
		"word-regexp", "line-regexp", "extended-regexp", "fixed-strings", "basic-regexp",
		"perl-regexp", "only-matching", "no-messages", "byte-offset", "with-filename", "no-filename",
		"initial-tab", "null", "null-data", "text", "binary", "line-buffered", "color", "colour"}},
	"rg": {"iIsSwxvclnNHuLFPUzoqap.", []string{
		"hidden", "no-ignore", "files", "files-with-matches", "files-without-match", "count",
		"line-number", "no-line-number", "heading", "no-heading", "fixed-strings", "ignore-case",
		"smart-case", "case-sensitive", "word-regexp", "line-regexp", "invert-match", "follow",
		"multiline", "null", "only-matching", "quiet", "text", "json", "vimgrep", "no-messages",
		"stats", "trim", "column", "pretty", "with-filename", "no-filename", "binary",
		"search-zip", "one-file-system", "unrestricted", "no-config"}},
	"ag": {"iswvclnLQSUaztHf", []string{
		"hidden", "unrestricted", "count", "files-with-matches", "files-without-matches",
		"literal", "ignore-case", "smart-case", "case-sensitive", "word-regexp", "invert-match",
		"follow", "search-zip", "silent", "nocolor", "nogroup", "noheading", "numbers", "null",
		"all-text", "all-types", "skip-vcs-ignores"}},
	"ack": {"iIwvclLnHhfQsxk", []string{
		"ignore-case", "smart-case", "word-regexp", "invert-match", "count", "files-with-matches",
		"files-without-matches", "literal", "follow", "nofollow", "with-filename", "no-filename",
		"recurse", "no-recurse", "nocolor", "noheading", "nogroup", "noenv", "ignore-ack-defaults"}},
	"ls": {"aAbBcCdfFgGhHiklLmnNopqQrRsStuUvxXZ1", []string{
		"all", "almost-all", "escape", "ignore-backups", "directory", "classify", "no-group",
		"human-readable", "si", "dereference", "dereference-command-line", "inode", "numeric-uid-gid",
		"literal", "hide-control-chars", "show-control-chars", "quote-name", "reverse",
		"recursive", "size", "context", "group-directories-first", "full-time", "color"}},
	"du": {"0abcDhHklLmsSxP", []string{
		"null", "all", "apparent-size", "bytes", "total", "dereference-args", "human-readable",
		"si", "count-links", "dereference", "no-dereference", "separate-dirs", "summarize",
		"one-file-system", "inodes", "time"}},
	"tree": {"adlfixqNQpugshDFvtcUrCASJXn", []string{
		"noreport", "dirsfirst", "filesfirst", "du", "prune", "inodes", "device", "gitignore",
		"matchdirs", "ignore-case", "fromfile", "info", "fflinks", "noindent"}},
}

// walkerTakesValue reports an option word that may consume the next word: a
// long option not listed as taking no value and given no `=` value, or a
// short group whose last letter is not proven to take no value (a letter
// before the last that takes a value has its value attached). A command with
// no grammar takes none.
func walkerTakesValue(command, option string) bool {
	if command == "egrep" || command == "fgrep" || command == "zgrep" {
		command = "grep"
	}
	grammar, ok := walkerGrammars[command]
	if !ok {
		return false
	}
	if long, isLong := strings.CutPrefix(option, "--"); isLong {
		return !strings.Contains(long, "=") && !slices.Contains(grammar.long, long)
	}
	for index := 1; index < len(option); index++ {
		if !strings.ContainsRune(grammar.letters, rune(option[index])) {
			// A value-taking letter: the rest of the group is its value.
			return index == len(option)-1
		}
	}
	return false
}

// listedDirectories returns the bare directory names a listing or walking
// command reads: a path operand made only of name characters, which has no
// `/` or `.` to make it a path token of its own (AFP-V0-9133). `find dir`,
// `ls dir` and recursive searches of `dir` are reported alike. The caller
// names them for test units only.
func listedDirectories(word string, args []string) []string {
	paths, _, ok := walkerPaths(word, args, false)
	if !ok {
		return nil
	}
	var directories []string
	for _, value := range paths {
		if value == "" || strings.ContainsAny(value, "/.$`*?[{~\"'\\") {
			continue
		}
		if strings.IndexFunc(value, func(c rune) bool {
			return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-')
		}) >= 0 {
			continue
		}
		directories = append(directories, value)
	}
	return directories
}

// recursiveReaders are the commands that read below a directory they are
// given, or run commands they are handed, so a lone file name they match
// may lie at any depth (AFP-V0-400).
var recursiveReaders = map[string]bool{
	"find": true, "rg": true, "ag": true, "ack": true, "tree": true, "du": true, "tar": true,
	"zip": true, "rsync": true, "xargs": true, "eval": true, "pushd": true,
}

// optionSet names the options that make a command read recursively, change
// its directory or run a command it is handed: a long option whose name, or
// any abbreviation of it that getopt_long accepts, is listed, with or without
// a value, or a short option group holding one of the letters anywhere.
type optionSet struct {
	long    []string
	letters string
}

// grepOptions include every `-d`/`--directories` action, because grep
// accepts an abbreviated action (`-d rec`) as well as `-drecurse`.
var grepOptions = optionSet{[]string{"recursive", "dereference-recursive", "directories"}, "rRd"}

// recursiveFlags are the commands that read recursively given one of their
// options.
var recursiveFlags = map[string]optionSet{
	"grep": grepOptions, "egrep": grepOptions, "fgrep": grepOptions, "zgrep": grepOptions,
	"ls":    {[]string{"recursive"}, "R"},
	"cp":    {[]string{"recursive", "archive"}, "rRa"},
	"scp":   {nil, "r"},
	"diff":  {[]string{"recursive"}, "r"},
	"chmod": {[]string{"recursive"}, "R"},
	"chown": {[]string{"recursive"}, "R"},
}

// envOptions are env's options that change the directory (`-C`, `--chdir`)
// or split a string into the command to run (`-S`, `--split-string`).
var envOptions = optionSet{[]string{"chdir", "split-string"}, "CS"}

// matches reports an option word in the set. A bare `--` matches nothing.
func (set optionSet) matches(value string) bool {
	if name, ok := strings.CutPrefix(value, "--"); ok {
		name, _, _ = strings.Cut(name, "=")
		if name == "" {
			return false
		}
		return slices.ContainsFunc(set.long, func(long string) bool { return strings.HasPrefix(long, name) })
	}
	return strings.HasPrefix(value, "-") && strings.ContainsAny(value[1:], set.letters)
}

// treeBlindGit are the git subcommands a placed script may run in a computed
// directory: none matches a pathspec at any depth of a tree it reads, as
// grep, ls-files, ls-tree, archive or diff do (AFP-V0-400).
var treeBlindGit = map[string]bool{
	"init": true, "config": true, "add": true, "commit": true, "tag": true,
	"rev-parse": true, "checkout": true, "switch": true,
}

// placesFileNames reports a shell unit whose lone file-name tokens can be
// placed as a Go unit's are (AFP-V0-400): none of its commands, those of its
// command strings included, changes directory other than by a placedDirectory
// `cd` in the file's own commands (the first own of them), runs env with an
// option that changes directory or splits a command string, runs git other
// than a treeBlindGit subcommand in a computed directory
// (`git -C "$root" add -A`), reads recursively, or runs commands it is
// handed. A `cd` to a variable, a computed or literal directory, `cd -` or a
// bare `cd` keeps the wider relation: its target is not proven to be a base.
func placesFileNames(commands []command, own int, literal *resolver) bool {
	for index, cmd := range commands {
		words := plainWords(cmd.words)
		if runsEnvOption(words) {
			return false
		}
		word, args := commandWord(words)
		word = path.Base(word)
		switch {
		case recursiveReaders[word]:
			return false
		case word == "cd":
			if index >= own || !literal.placedDirectory(args) {
				return false
			}
		case word == "git":
			if len(args) < 3 || unquote(args[0]) != "-C" || !computed(args[1]) || !treeBlindGit[unquote(args[2])] {
				return false
			}
		}
		if flags, ok := recursiveFlags[word]; ok {
			for _, arg := range args {
				if flags.matches(unquote(arg)) {
					return false
				}
			}
		}
	}
	return true
}

// runsEnvOption reports a command that runs env, as its command or through a
// prefix, with an envOptions option.
func runsEnvOption(words []string) bool {
	for index, raw := range words {
		if path.Base(unquote(raw)) != "env" {
			continue
		}
		for _, option := range words[index+1:] {
			value := unquote(option)
			if !strings.HasPrefix(value, "-") {
				break
			}
			if envOptions.matches(value) {
				return true
			}
		}
	}
	return false
}

// placement returns the resolver a `cd` target is placed with: no variable
// resolves, since a value assigned in the file may be skipped by a branch
// and taken from the environment, and `$0` names the file only in an
// executed test (scriptZero), not in a sourced helper or a bats file.
func (r *resolver) placement(scriptZero bool) *resolver {
	return &resolver{directory: r.directory, values: map[string]string{}, counts: map[string]int{}, resolving: map[string]bool{}, noZero: !scriptZero}
}

// placedDirectory reports a `cd` operand list that changes to a directory
// whose lone file names the unit's placement already covers: exactly one
// operand after `-L`/`-P`/`-e`/`-@` options and `--`, a directory form of
// the file itself (`"$(dirname "$0")/.."`, `"${BASH_SOURCE%/*}"`) that
// resolves to the repository root or into the file's own directory.
func (r *resolver) placedDirectory(args []string) bool {
	var operands []string
	options := true
	for _, arg := range args {
		value := unquote(arg)
		switch {
		case options && value == "--":
			options = false
		case options && len(value) > 1 && strings.HasPrefix(value, "-") && strings.Trim(value[1:], "LPe@") == "":
		default:
			options = false
			operands = append(operands, value)
		}
	}
	if len(operands) != 1 {
		return false
	}
	directory, ok := r.pathOf(operands[0], 0)
	return ok && (directory == "" || r.directory == "" || directory == r.directory || strings.HasPrefix(directory, r.directory+"/"))
}

// computed reports a word whose value is known only at run time: one with an
// expansion or a command substitution.
func computed(word string) bool {
	return strings.ContainsAny(word, "$`")
}

// shadowed reports a call to a function the file defined earlier. A later
// definition does not shadow an earlier call, and a command string runs in a
// child shell that does not inherit the file's functions.
func shadowed(parsed script, word string, index int) bool {
	defined, ok := parsed.functions[word]
	return ok && index >= defined && index < len(parsed.commands)
}

// bypassesFunctions reports a command run through a prefix that looks the
// command up on PATH, past any function of the same name.
func bypassesFunctions(words []string) bool {
	for _, raw := range plainWords(words) {
		if assignment.MatchString(raw) {
			continue
		}
		word := unquote(raw)
		if functionBypassPrefixes[word] {
			return true
		}
		if !commandPrefixes[word] {
			return false
		}
	}
	return false
}

// functionBypassPrefixes are the commandPrefixes that run an external command.
var functionBypassPrefixes = map[string]bool{
	"exec": true, "command": true, "nohup": true, "sudo": true, "env": true, "xcrun": true, "timeout": true,
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

// commandStrings parses each value of a computed command string. A part the
// file does not fix stays a computed name, so it never reads as a literal.
// It reports whether a value bound cut the list (AFP-V0-3147).
func commandStrings(script string, values map[string][]string) ([]command, bool) {
	var cut bool
	var out []command
	for _, value := range expand(script, values, 0, &cut) {
		parsed := parse(strings.ReplaceAll(value, wildcard, "$_"))
		out = append(out, parsed.commands...)
		// An incomplete here-document in a value holds the file as a cut
		// bound does (AFP-V0-9971).
		cut = cut || parsed.incomplete
	}
	return out, cut
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
	// conflicting marks a name whose assignments do not all carry the same
	// text as a plain `NAME=VALUE` word, or that a declaration, array or
	// `+=` form also binds (AFP-V0-2954).
	conflicting map[string]bool
	// text is the file's text, for the whole-word checks of AFP-V0-2954.
	text string
	// plainAt holds, for each name, the index of a command that is that
	// name's assignment alone (TSP-V0-004).
	plainAt   map[string]int
	resolving map[string]bool
	// noZero refuses `$0` forms, which name the running script rather than
	// this file when it is sourced or run by bats.
	noZero bool
}

func newResolver(relative string, commands []command) *resolver {
	directory := path.Dir(relative)
	if directory == "." {
		directory = ""
	}
	r := &resolver{directory: directory, values: map[string]string{}, counts: map[string]int{}, conflicting: map[string]bool{}, plainAt: map[string]int{}, resolving: map[string]bool{}}
	for at, cmd := range commands {
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
					r.conflicting[value] = true
					continue
				}
				break
			}
			name := match[1]
			r.counts[name]++
			if match[2] != "" || match[3] != "" {
				r.counts[name]++
				r.conflicting[name] = true
			}
			assigned := unquote(raw[len(match[0]):])
			if previous, repeated := r.values[name]; repeated && previous != assigned || declaring {
				r.conflicting[name] = true
			}
			r.values[name] = assigned
			if index == 0 && len(words) == 1 {
				r.plainAt[name] = at
			}
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
			if r.noZero && strings.HasPrefix(form, "${0") {
				return "", "", false
			}
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
	// A name assigned more than once resolves only when every assignment is a
	// plain word with the same text and nothing else binds it, so whichever
	// ran, the value is the same (AFP-V0-2954).
	if r.counts[name] != 1 && (r.counts[name] == 0 || r.conflicting[name] || !onlyAssigned(r, name)) || r.resolving[name] {
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

// redirection matches a discarded redirection around shell blanks only: `\s`
// would also take a carriage return, which a shell keeps as word text
// (AFP-V0-9970).
var redirection = regexp.MustCompile(`[ \t]*[0-9]*(?:>>?|<|&>)[ \t]*(?:&[0-9]+|/dev/null)`)

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
		return !r.noZero || (text != "$0" && text != "${0}")
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

// tempExpansion is a plain or braced expansion of a name and the literal path
// that follows it.
var tempExpansion = regexp.MustCompile(`\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))(/[A-Za-z0-9._~@+/-]*)?`)

// tempRoots names the variables whose value is a directory the file itself
// creates with mktemp (AFP-V0-2204): the name is assigned exactly once in
// the file, every other occurrence is an expansion that cannot assign or
// part of a path (assignedOnce), and the value is one `$(mktemp -d ...)` substitution
// (makesDirectory) or another such name followed only by a literal path
// without `..` (`rp=$tmp/review`). The assignment is a command of its own at
// the file's top level, run unconditionally (not after `&&` or `||`, not in a
// pipeline or the background), and no expansion of the name comes before it
// in the text, so no value from the environment can stand in. A file that
// defines or aliases mktemp has none; one that assigns PATH before the name's
// assignment must call `command -p mktemp`, as makesDirectory requires when
// the path is overridden. A file that can fill a directory with repository
// files without naming each one (populates) has none either: a path below
// such a copy reads a repository file no token names.
func tempRoots(r *resolver, parsed script, text string) map[string]bool {
	return rootsWith(r, parsed, text, unconditionalAssignment)
}

// rootsWith is tempRoots with the check that a name's one assignment runs:
// unconditionalAssignment for path tokens, exitGuardedAssignment for the git
// location of AFP-V0-9973.
func rootsWith(r *resolver, parsed script, text string, runs func(*resolver, script, string, string) bool) map[string]bool {
	if strings.Contains(text, "mktemp()") || strings.Contains(text, "mktemp ()") || strings.Contains(text, "function mktemp") || strings.Contains(text, "alias mktemp") || populates.MatchString(text) {
		return nil
	}
	firstPath := firstAssignment(text, "PATH")
	roots := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for name, value := range r.values {
			if roots[name] || !assignedOnce(text, name) || !runs(r, parsed, text, name) {
				continue
			}
			pathOverridden := firstPath >= 0 && firstPath < firstAssignment(text, name)
			if makesDirectory(value, pathOverridden) || underTempRoot(value, roots) {
				roots[name] = true
				changed = true
			}
		}
	}
	return roots
}

// populates matches a command that can copy repository files into a
// directory without a path token naming each file: a recursive, archive or
// parents copy (a recursive letter anywhere in a short option group, as in
// cp -rf), rsync, an archive extractor, a link, a move, xargs, or a Git
// command that checks out, clones or archives a tree.
var populates = regexp.MustCompile(`(?m)(^|[^A-Za-z0-9_-])(cp[ \t]+[^\n;|&]*(-[A-Za-z]*[rRa][A-Za-z]*|--recursive|--archive|--parents)|rsync|tar|cpio|pax|unzip|ditto|ln|mv|xargs)([ \t]|$)|git[^\n;|&]*[ \t](clone|worktree|archive|checkout-index|--work-tree)([ \t=]|$)`)

// unconditionalAssignment reports that name's one assignment is a command of
// its own at the file's top level, not after `&&` or `||` (also after a
// compound command, `{ :; } || tmp=...`) and not in a pipeline or the
// background, and that no expansion of the name precedes it. An `&&` or `||`
// after it also keeps the name's tokens: whether the list it leads ends in
// the background is not proven (AFP-V0-2243, disabled for 1.0).
func unconditionalAssignment(r *resolver, parsed script, text, name string) bool {
	at, plain := r.plainAt[name]
	if !plain || at >= len(parsed.commands) {
		return false
	}
	command := parsed.commands[at]
	if command.substituted || command.inFunction || command.block != 0 {
		return false
	}
	// A redirection on the assignment (`tmp=$(mktemp -d) 3>&9`) can fail,
	// and a shell such as dash then performs no assignment, leaving an
	// inherited value in place; nothing here proves it succeeds.
	if len(command.words) != 1 {
		return false
	}
	switch command.lead {
	case "&&", "||", "|", "&":
		return false
	}
	switch command.trail {
	case "&&", "||", "|", "&":
		return false
	}
	assigned := firstAssignment(text, name)
	if assigned < 0 {
		return false
	}
	for _, expansion := range []string{"$" + name, "${" + name} {
		for from := 0; ; {
			offset := strings.Index(text[from:], expansion)
			if offset < 0 {
				break
			}
			at := from + offset
			end := at + len(expansion)
			from = end
			if expansion[1] != '{' && end < len(text) && nameByte(text[end]) {
				continue
			}
			if at < assigned {
				return false
			}
		}
	}
	return true
}

// firstAssignment is the text offset of the first whole-word `name=`, or -1.
func firstAssignment(text, name string) int {
	for from := 0; ; {
		offset := strings.Index(text[from:], name+"=")
		if offset < 0 {
			return -1
		}
		at := from + offset
		from = at + len(name)
		if at == 0 || !nameByte(text[at-1]) && text[at-1] != '$' && text[at-1] != '{' {
			return at
		}
	}
}

// assignedOnce is onlyExpanded with one more inert occurrence: the name
// joined to a `/`, `.` or `-` on either side is part of a word such as the
// `/tmp` of a `${TMPDIR:-/tmp}` template, which no assignment can spell.
// A bare word occurrence (an argument of `read`, `for`, `local`, or prose
// in a comment) still refuses.
func assignedOnce(text, name string) bool {
	assignments := 0
	for from := 0; ; {
		offset := strings.Index(text[from:], name)
		if offset < 0 {
			return assignments == 1
		}
		at := from + offset
		end := at + len(name)
		from = end
		if at > 0 && nameByte(text[at-1]) || end < len(text) && nameByte(text[end]) {
			continue
		}
		after := text[end:]
		switch {
		case at >= 2 && text[at-2:at] == "${":
			if strings.HasPrefix(after, "=") || strings.HasPrefix(after, ":=") {
				return false
			}
		case at >= 1 && text[at-1] == '$':
		case strings.HasPrefix(after, "=") && (at == 0 || strings.IndexByte(" \t\n;", text[at-1]) >= 0):
			assignments++
		case at > 0 && strings.IndexByte("/.-", text[at-1]) >= 0, end < len(text) && strings.IndexByte("/.-", text[end]) >= 0:
		default:
			return false
		}
	}
}

// underTempRoot reports a value that is a temp root's expansion alone or
// followed by a literal path without `..`.
func underTempRoot(value string, roots map[string]bool) bool {
	match := tempExpansion.FindStringSubmatchIndex(value)
	if match == nil || match[0] != 0 || match[1] != len(value) {
		return false
	}
	groups := tempExpansion.FindStringSubmatch(value)
	return roots[groups[1]+groups[2]] && !slices.Contains(strings.Split(groups[3], "/"), "..")
}

// pathTokens lists a file's AFP-V0-012 path tokens. A literal path that
// follows the expansion of a temp root (tempRoots) names a file below a
// directory the script created, never a repository path, so it adds no
// token (AFP-V0-2204); a path with `..` after the root keeps its tokens. test
// marks a test unit, whose glob directories (AFP-V0-9129) and bare listed
// directories (AFP-V0-9133) are named; a helper's are not, since a non-test
// shell unit bears the foreign-invocation frontier on every unit that reaches
// it, and a helper listing `internal` would turn every Go edit below it into
// an unknown. listings admits name-listing reads (nameListingAdmitted):
// the tokens of an admitted name-only command (nameListingCommand) that no
// other read holds are returned in names as well, so the planner narrows
// them by change status (AFP-V0-9961, AFP-V0-9962).
//
// utilities reports that cp, mv and mkdir name the standard utilities in
// this file; when a function, alias, hash, enable or PATH assignment may make
// them run something else, readWords drops none of their operands
// (AFP-V0-9977).
func pathTokens(parsed script, temp map[string]bool, test, listings, utilities bool) (map[string]bool, map[string]bool, map[string]bool) {
	tokens, globs := map[string]bool{}, map[string]bool{}
	content, names := map[string]bool{}, map[string]bool{}
	held := content
	add := func(text string) {
		if len(temp) != 0 {
			text = tempExpansion.ReplaceAllStringFunc(text, func(match string) string {
				groups := tempExpansion.FindStringSubmatch(match)
				if !temp[groups[1]+groups[2]] || slices.Contains(strings.Split(groups[3], "/"), "..") {
					return match
				}
				return " "
			})
		}
		for _, token := range pathToken.FindAllString(expansion.ReplaceAllString(text, " "), -1) {
			if pathShaped(token) {
				tokens[token] = true
				held[token] = true
			}
		}
	}
	for index, cmd := range parsed.commands {
		// An admitted name-only command with no glob among its words holds
		// name-listing reads; every other command holds content reads.
		held = content
		if listings && nameListingCommand(parsed, index) && !slices.ContainsFunc(readWords(cmd.words, utilities), func(word string) bool {
			_, glob := literalPathGlob(word)
			return glob
		}) {
			held = names
		}
		// A bare directory operand of a listing command names the files
		// below that directory, which an added or deleted file changes
		// (AFP-V0-9133); the `/dir/` form matches the component at any
		// depth, as the glob directory below does.
		if test {
			name, operands := commandWord(plainWords(cmd.words))
			for _, directory := range listedDirectories(name, operands) {
				tokens["/"+directory+"/"] = true
				held["/"+directory+"/"] = true
			}
		}
		for _, word := range readWords(cmd.words, utilities) {
			if glob, ok := literalPathGlob(word); ok && !tempRootedWord(word, temp) {
				globs[glob] = true
				// The matched names can feed a derived read of a sibling
				// (`${f%_test.sh}.tsv`), so in a test the glob's literal
				// directory names every file below it, at any depth, as the
				// `/dir/` fragment left by a stripped root variable does. A
				// bare `dir/` token would name only a file called dir
				// (V1-0290) and lose that read (AFP-V0-9129). A helper keeps
				// the glob alone, for the frontier reason given above.
				if directory := globDirectory(glob); test && directory != "" {
					tokens["/"+directory+"/"] = true
					content["/"+directory+"/"] = true
				}
			}
			// Matched names can feed derived reads (for example, replacing
			// _test.sh with .tsv), so the glob cannot replace broad tokens.
			add(word)
		}
	}
	held = content
	for _, body := range parsed.data {
		add(body)
	}
	for token := range content {
		delete(names, token)
	}
	return tokens, globs, names
}

// globDirectory returns the literal directory components of a literalPathGlob
// value before its first wildcard component, or "" when the first component
// holds the wildcard.
func globDirectory(glob string) string {
	parts := strings.Split(glob, "/")
	for index, part := range parts {
		if strings.Contains(part, "*") {
			return strings.Join(parts[:index], "/")
		}
	}
	return ""
}

// tempRootedWord reports a word that starts with an expansion of a private
// temporary root (for example "$tmp/script/*.tsv" after tmp=$(mktemp -d)).
// Its suffix names paths under that root, not repository paths, so it must
// not become a repository glob. A parent climb keeps the conservative glob.
func tempRootedWord(word string, temp map[string]bool) bool {
	if len(temp) == 0 {
		return false
	}
	value := unquote(word)
	match := tempExpansion.FindStringSubmatch(value)
	if match == nil || !strings.HasPrefix(value, "$") || strings.Index(value, match[0]) != 0 || !temp[match[1]+match[2]] {
		return false
	}
	return !slices.Contains(strings.Split(value[len(match[0])-len(match[3]):], "/"), "..")
}

// literalPathGlob records a complete, simple wildcard path alongside its
// conservative directory tokens (AFP-V0-3333).
// An unknown leading root is allowed, so the pattern is unanchored. Other
// substitutions, escapes, parent climbs, globstars and shell pattern extensions
// keep the conservative token relation. Heredocs are not narrowed here.
func literalPathGlob(word string) (string, bool) {
	if strings.Contains(word, "\\") {
		return "", false
	}
	value := unquote(word)
	if strings.HasPrefix(value, "$") {
		match := tempExpansion.FindStringSubmatchIndex(value)
		// Strip only the variable, not its literal suffix. Parameter operators
		// or a second expansion leave non-path bytes and are rejected below.
		if match == nil || match[0] != 0 {
			return "", false
		}
		end := match[1]
		if match[6] >= 0 {
			end = match[6]
		}
		value = value[end:]
		if !strings.HasPrefix(value, "/") {
			return "", false
		}
	}
	value = strings.TrimPrefix(value, "/")
	if !pathShaped(value) || !strings.Contains(value, "*") || strings.Contains(value, "**") {
		return "", false
	}
	for _, c := range value {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._~@+/-*", c) {
			return "", false
		}
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return "", false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	return value, true
}

// readWords lists the words of one command through which a dirty path can
// reach it: every word except the redirection markers and the words a command
// only creates or truncates (NSA-V0-016): the target of a `>` or `>|`
// redirection, every operand of `mkdir`, and the last of two or more operands
// of `cp` or `mv` run without a target-directory option. A word in any other
// role stays, so a path both copied from and copied to is still read, and an
// appending redirection, a `-t` or `--target-directory` option, or fewer than
// two operands keeps every word (fail closed).
//
// utilities false keeps every operand of cp, mv and mkdir: those names may
// not run the utilities whose written operands are dropped (AFP-V0-9977).
// The `>` and `>|` targets are dropped either way, since the shell itself
// truncates them before the command runs.
func readWords(words []string, utilities bool) []string {
	written := map[string]bool{}
	read := make([]string, 0, len(words))
	for index := 0; index < len(words); index++ {
		switch words[index] {
		case writeRedirect:
			if index+1 < len(words) {
				index++
				written[words[index]] = true
			}
		case redirect:
			if index+1 < len(words) {
				index++
				read = append(read, words[index])
			}
		default:
			read = append(read, words[index])
		}
	}
	name, operands := commandWord(plainWords(words))
	if !utilities {
		name = ""
	}
	// Qualified and computed commands may be wrappers that read every operand;
	// their basename alone does not establish standard utility semantics.
	switch name {
	case "mkdir":
		for _, operand := range dropFlags(operands) {
			written[operand] = true
		}
	case "cp", "mv":
		paths := dropFlags(operands)
		if len(paths) >= 2 && !targetDirectoryOption(operands) {
			written[paths[len(paths)-1]] = true
		}
	}
	if len(written) == 0 {
		return read
	}
	kept := read[:0]
	for _, word := range read {
		if !written[word] {
			kept = append(kept, word)
		}
	}
	return kept
}

// targetDirectoryOption reports a `cp` or `mv` option that names the
// destination itself (`-t DIR`, `--target-directory`, or a short cluster that
// may carry `t`), after which the last operand is a source.
func targetDirectoryOption(operands []string) bool {
	for _, operand := range operands {
		word := unquote(operand)
		if strings.HasPrefix(word, "--target-directory") {
			return true
		}
		if strings.HasPrefix(word, "-") && !strings.HasPrefix(word, "--") && strings.ContainsRune(word, 't') {
			return true
		}
	}
	return false
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
