// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// This file holds the fail-closed classifier rules of lane shellwalk
// (AFP-V0-9972 to AFP-V0-9977): each only adds a hold, an edge or a token
// the plugin did not have before; none removes one.

// wrapperRunners are commands that run another command given among their
// operands, after options whose values the plugin does not parse. Together
// with commandPrefixes, whose own option values commandWord does not parse
// either (`sudo -u root`, `env -u NAME`, `timeout -s KILL 5`), they can hide
// the command that actually runs (AFP-V0-9975).
var wrapperRunners = map[string]bool{
	"nice": true, "ionice": true, "stdbuf": true, "setsid": true, "taskset": true, "chrt": true,
	"flock": true, "chronic": true, "unbuffer": true, "doas": true, "strace": true, "ltrace": true,
	"valgrind": true, "xargs": true, "parallel": true, "watch": true, "caffeinate": true,
	"systemd-run": true, "firejail": true, "bwrap": true, "unshare": true, "nsenter": true,
	"faketime": true, "fakeroot": true, "proot": true, "catchsegv": true, "runuser": true,
	"setpriv": true, "prlimit": true, "numactl": true, "cgexec": true, "eatmydata": true,
	"script": true, "su": true, "sg": true,
}

// invocation is one command word and its operands that observe classifies.
type invocation struct {
	word string
	args []string
}

// runnerPrefixes are the commandPrefixes that run their operand as an
// external command: not the control words (`if`, `then`, `!`, ...) or
// `builtin`, whose next word commandWord already reads as the command.
var runnerPrefixes = map[string]bool{
	"exec": true, "command": true, "nohup": true, "time": true, "sudo": true, "env": true,
	"run": true, "xcrun": true, "timeout": true,
}

// wrapperLead returns the first word past assignments when it is a
// runnerPrefixes or wrapperRunners word.
func wrapperLead(words []string) ([]string, bool) {
	words = dropAssignments(words)
	if len(words) == 0 {
		return nil, false
	}
	lead := path.Base(unquote(words[0]))
	return words, runnerPrefixes[lead] || wrapperRunners[lead]
}

// classifiedCommand reports a command word one of this plugin's hold rules
// classifies: an interpreter whose script operand is an edge, a toolchain, a
// tree walker, git or awk.
func classifiedCommand(word string) bool {
	base := path.Base(word)
	return shells[base] || isToolchain(word) || base == "git" || awkCommands[base] || base == "sed" || walkerCommands[base]
}

// walkerCommands are the commands walkerPaths reads, by name, whatever
// their options.
var walkerCommands = map[string]bool{
	"find": true, "grep": true, "egrep": true, "fgrep": true, "zgrep": true, "rg": true,
	"ag": true, "ack": true, "ls": true, "du": true, "tree": true,
}

// wrapped returns the command a wrapper runs and its operands: the first
// later word that classifiedCommand knows, when the command's first word
// (past assignments) is a runnerPrefixes or wrapperRunners word. ok is false
// when no wrapper leads or no later word is known (AFP-V0-9975).
func wrapped(words []string) (string, []string, bool) {
	words, ok := wrapperLead(words)
	if !ok {
		return "", nil, false
	}
	lead := path.Base(unquote(words[0]))
	// `command -v NAME` and `command -V NAME` look a command up and run none.
	if lead == "command" && len(words) > 1 {
		if option := unquote(words[1]); option == "-v" || option == "-V" {
			return "", nil, false
		}
	}
	for index := 1; index < len(words); index++ {
		if word := unquote(words[index]); classifiedCommand(word) {
			return word, words[index+1:], true
		}
	}
	return "", nil, false
}

// wrapperOperands returns the words after a leading wrapper word whose
// unquoted text holds a `/`: a repository script it may run (`nice
// script/x.sh`, `sudo -u u script/x.sh`), which links like a command word
// does, unless commandWord already reads a path command word, which links
// on the base (AFP-V0-9975).
func wrapperOperands(words []string) []string {
	words, ok := wrapperLead(words)
	if !ok {
		return nil
	}
	if word, _ := commandWord(words); strings.Contains(word, "/") || strings.HasPrefix(word, "$") {
		// The command word itself runs the script, linked as on the base.
		return nil
	}
	var operands []string
	for _, raw := range words[1:] {
		if word := unquote(raw); strings.Contains(word, "/") && !strings.HasPrefix(word, "-") {
			operands = append(operands, word)
		}
	}
	return operands
}

// startupNames are environment names that make an interpreter, the dynamic
// loader or a shell read a file or code no operand names: a shell's startup
// file, a language's module path or preload options (AFP-V0-9976).
var startupNames = map[string]bool{
	"BASH_ENV": true, "ENV": true, "ZDOTDIR": true, "BASH_LOADABLES_PATH": true,
	"AWKPATH": true, "AWKLIBPATH": true, "GCONV_PATH": true,
	"RUBYOPT": true, "RUBYLIB": true, "NODE_OPTIONS": true, "NODE_PATH": true,
	"PYTHONPATH": true, "PYTHONSTARTUP": true, "PYTHONHOME": true, "PYTHONUSERBASE": true,
	"JAVA_TOOL_OPTIONS": true, "_JAVA_OPTIONS": true, "JDK_JAVA_OPTIONS": true,
	"PHPRC": true, "PHP_INI_SCAN_DIR": true, "TCLLIBPATH": true,
	// git runs the commands and hooks of these directories (AFP-V0-9710).
	"GIT_EXEC_PATH": true, "GIT_TEMPLATE_DIR": true,
}

// commandNames are environment names whose value is a command line that a
// program runs on its own: git's and other programs' pager, editor, ssh,
// askpass, external diff and proxy commands, and less's input filters. Such
// a value runs a file no operand names, so it is a startup name unless it
// names no repository file (commandOutsideValue, AFP-V0-9710).
var commandNames = map[string]bool{
	"PAGER": true, "GIT_PAGER": true, "MANPAGER": true, "EDITOR": true, "VISUAL": true,
	"GIT_EDITOR": true, "GIT_SEQUENCE_EDITOR": true, "GIT_SSH": true, "GIT_SSH_COMMAND": true,
	"GIT_ASKPASS": true, "SSH_ASKPASS": true, "GIT_EXTERNAL_DIFF": true, "GIT_PROXY_COMMAND": true,
	"LESSOPEN": true, "LESSCLOSE": true,
}

// commandOutsideValue reports an unquoted commandNames value that names no
// file of the repository: an outsideValue, or one bare command word with no
// `/`, blank or expansion, which the shell finds on PATH and never in the
// working directory (`GIT_EDITOR=true`, `PAGER=cat`). Any other value, such
// as `./tools/pager.sh` or `ssh -F cfg`, may run a repository file.
func commandOutsideValue(value string) bool {
	return outsideValue(value) || !strings.ContainsAny(value, "/ \t\n$`\\;|&<>(){}*?[~")
}

// startupName reports a startupNames name or one of the families that load
// code: PERL-prefixed names longer than PERL, LD_ and DYLD_ loader names and
// exported bash functions (BASH_FUNC_), and lua's module path and init
// names in every versioned form.
func startupName(name string) bool {
	if startupNames[name] {
		return true
	}
	if strings.HasPrefix(name, "PERL") && len(name) > len("PERL") {
		return true
	}
	// LUA_PATH, LUA_CPATH and LUA_INIT, each also in its versioned form
	// (LUA_PATH_5_4, LUA_INIT_5_1, ...), choose the modules or code lua loads.
	for _, prefix := range []string{"LUA_PATH", "LUA_CPATH", "LUA_INIT"} {
		if name == prefix || strings.HasPrefix(name, prefix+"_") {
			return true
		}
	}
	return strings.HasPrefix(name, "LD_") || strings.HasPrefix(name, "DYLD_") || strings.HasPrefix(name, "BASH_FUNC_")
}

// namingCommands set or export the names given as their operands.
var namingCommands = map[string]bool{
	"export": true, "declare": true, "typeset": true, "readonly": true, "local": true,
	"read": true, "printf": true, "getopts": true, "for": true, "select": true,
	"mapfile": true, "readarray": true,
}

// defaultAssignment matches a `${NAME=...}` or `${NAME:=...}` expansion,
// which assigns NAME when it is unset or empty.
var defaultAssignment = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*):?=`)

// setsStartupName reports a command that assigns or exports a startupName
// or a commandNames name (AFP-V0-9710):
// an assignment word anywhere among its words (a prefix, a standalone
// assignment, which also changes an inherited exported value, or one after
// a wrapper such as env or sudo), a bare name given to a naming command, or a
// default-assigning expansion. Such a value makes the next interpreter read a
// file the plugin does not link (AFP-V0-9976). An assignment of an
// outsideValue, or of a commandNames name to a commandOutsideValue, names no
// repository file and is not counted.
func setsStartupName(words []string) bool {
	plain := plainWords(words)
	naming := false
	for index, raw := range plain {
		word := unquote(raw)
		for _, match := range defaultAssignment.FindAllStringSubmatch(raw, -1) {
			if startupName(match[1]) || commandNames[match[1]] {
				return true
			}
		}
		if match := assignment.FindStringSubmatch(word); match != nil {
			value := word[len(match[0]):]
			if startupName(match[1]) && !outsideValue(value) || commandNames[match[1]] && !commandOutsideValue(value) {
				return true
			}
			continue
		}
		if index == 0 || !naming {
			naming = naming || namingCommands[word]
			continue
		}
		if startupName(word) || commandNames[word] {
			return true
		}
	}
	return false
}

// outsideValue reports an unquoted assignment value that names no file of
// the repository: empty, which clears the name, or a list of absolute
// literal paths, which the plugin resolves outside every repository edge as
// it does a `source` of an absolute literal (AFP-V0-9976).
func outsideValue(value string) bool {
	if value == "" {
		return true
	}
	for _, part := range strings.Split(value, ":") {
		if !strings.HasPrefix(part, "/") || dynamic(part) {
			return false
		}
	}
	return true
}

// redefinesUtilities reports a file in which the name of a utility whose
// written operands readWords drops may run something else: a function of
// that name, any alias, hash or enable command, or an assignment of PATH
// (AFP-V0-9977).
func redefinesUtilities(parsed script, text string) bool {
	for _, name := range []string{"cp", "mv", "mkdir", "command", "builtin", "exec"} {
		if parsed.definitions[name] != 0 {
			return true
		}
	}
	for _, cmd := range parsed.commands {
		word, _ := commandWord(plainWords(cmd.words))
		switch path.Base(word) {
		case "alias", "hash", "enable":
			return true
		}
	}
	return firstAssignment(text, "PATH") >= 0
}

// awkCommands are the awk implementations.
var awkCommands = map[string]bool{"awk": true, "gawk": true, "mawk": true, "nawk": true}

// programFile reports an awk or sed command that takes its program, or part
// of it, from a file (`awk -f prog`, gawk's `-i`/`--include`, `-E`/`--exec`
// and `-l`/`--load`, `sed -f script`): what that program reads, by getline,
// a redirection or an `r` command, no word of this file names (AFP-V0-9974).
// Every word before `--` is read as a possible option, past option values
// such as `-F ,` and the program operand alike, so an option after a value
// is never missed; a data operand spelled like an option only adds a hold.
func programFile(word string, args []string) bool {
	base := path.Base(word)
	if !awkCommands[base] && base != "sed" {
		return false
	}
	for _, raw := range args {
		value := unquote(raw)
		if value == "--" {
			return false
		}
		if !strings.HasPrefix(value, "-") || value == "-" {
			continue
		}
		if long, ok := strings.CutPrefix(value, "--"); ok {
			name, _, _ := strings.Cut(long, "=")
			if name == "" {
				return false
			}
			for _, option := range []string{"file", "include", "exec", "load"} {
				if base == "sed" && option != "file" {
					continue
				}
				if strings.HasPrefix(option, name) {
					return true
				}
			}
			continue
		}
		letters := "fiEl"
		if base == "sed" {
			letters = "f"
		}
		if strings.ContainsAny(value[1:2], letters) || base == "sed" && strings.ContainsAny(value[1:], letters) {
			return true
		}
	}
	return false
}

// gitTreeReaders are the git subcommands that print paths, or text about
// paths, chosen by the working tree, the index or the history of every path:
// listings, searches, status, diffs and history with its path output
// (AFP-V0-9973). A history command prints the paths of every commit that
// touched them, so an edit anywhere changes its output once committed.
var gitTreeReaders = map[string]bool{
	"ls-files": true, "ls-tree": true, "grep": true, "status": true, "diff": true,
	"diff-files": true, "diff-index": true, "diff-tree": true, "clean": true, "log": true,
	"reflog": true, "show": true, "whatchanged": true,
}

// gitValueOptions are git's global options that take the next word as their
// value.
var gitValueOptions = map[string]bool{
	"-C": true, "-c": true, "--git-dir": true, "--work-tree": true, "--namespace": true,
	"--config-env": true, "--exec-path": true, "--super-prefix": true, "--attr-source": true,
}

// gitWalksTree reports a git command, after any wrapper, that runs a
// gitTreeReaders subcommand in a repository not proven private: with no -C,
// --git-dir or --work-tree value and no GIT_DIR or GIT_WORK_TREE assignment
// it runs in the working directory, which the plugin does not track, and any
// such location that is not a private temporary root (tempRootedWord) may be
// the repository (AFP-V0-9973).
//
// private reports a working directory privateDirectory proved to be a
// private temporary root, which stands for the working directory when no
// location is given.
func gitWalksTree(plain []string, word string, args []string, temp map[string]bool, private bool) bool {
	if path.Base(word) != "git" {
		return false
	}
	var locations []string
	for _, raw := range plain {
		match := assignment.FindStringSubmatch(unquote(raw))
		if match == nil {
			continue
		}
		if match[1] == "GIT_DIR" || match[1] == "GIT_WORK_TREE" {
			_, value, _ := strings.Cut(raw, "=")
			locations = append(locations, value)
		}
	}
	subcommand := ""
	for index := 0; index < len(args); index++ {
		value := unquote(args[index])
		if !strings.HasPrefix(value, "-") {
			subcommand = value
			break
		}
		name, _, hasValue := strings.Cut(value, "=")
		location := name == "-C" || name == "--git-dir" || name == "--work-tree"
		switch {
		case hasValue && strings.HasPrefix(value, "--"):
			if location {
				_, raw, _ := strings.Cut(args[index], "=")
				locations = append(locations, raw)
			}
		case gitValueOptions[name]:
			if index+1 < len(args) {
				index++
				if location {
					locations = append(locations, args[index])
				}
			}
		}
	}
	if !gitTreeReaders[subcommand] {
		return false
	}
	if len(locations) == 0 {
		return !private
	}
	return slices.ContainsFunc(locations, func(location string) bool { return !tempRootedWord(location, temp) })
}

// privateDirectory reports that the command at index at runs in a private
// temporary directory: an earlier `cd` of the file's own commands, in the
// same block (so the two run in order in one straight-line list) or at the
// file's top level when the command is outside every function body, changes
// to one tempRootedWord operand, runs unconditionally (no `&&`, `||`, `|` or
// `&` before it, unless an `|| exit` follows it, which leaves the script
// either in the directory or ended) and is not piped or backgrounded; and no
// other directory change of the file's own non-substituted commands comes
// between them. A command scanned from a command string (index past the
// file's own commands) has none (AFP-V0-9973).
func privateDirectory(commands []command, own, at int, temp map[string]bool) bool {
	if at >= own || len(temp) == 0 {
		return false
	}
	target := commands[at]
	for index := at - 1; index >= 0; index-- {
		cmd := commands[index]
		if cmd.substituted {
			continue
		}
		word, args := commandWord(plainWords(cmd.words))
		switch path.Base(word) {
		case "cd", "pushd", "popd":
		default:
			continue
		}
		sameList := cmd.block == target.block || cmd.block == 0 && !cmd.inFunction && !target.inFunction && !target.substituted
		operands := dropFlags(args)
		if !sameList || path.Base(word) != "cd" || len(operands) != 1 || !tempRootedWord(operands[0], temp) {
			return false
		}
		switch cmd.trail {
		case "|", "&":
			return false
		}
		switch cmd.lead {
		case "&&", "||", "|", "&":
			if cmd.trail != "||" || index+1 >= own {
				return false
			}
			if next, _ := commandWord(plainWords(commands[index+1].words)); next != "exit" {
				return false
			}
		}
		return true
	}
	return false
}

// exitGuardedAssignment is unconditionalAssignment that also admits the one
// form `NAME=VALUE || exit [N]` whose exit ends the list (no `&`, `|`, `&&`
// or `||` after it): the shell then either performed the assignment or
// ended, so no inherited value stands in. It decides only whether a git
// location is private (AFP-V0-9973), never a path token: AFP-V0-2243 keeps
// its trail refusal for tokens.
func exitGuardedAssignment(r *resolver, parsed script, text, name string) bool {
	if unconditionalAssignment(r, parsed, text, name) {
		return true
	}
	at, plain := r.plainAt[name]
	if !plain || at+1 >= len(parsed.commands) {
		return false
	}
	assigned, guard := parsed.commands[at], parsed.commands[at+1]
	if assigned.trail != "||" || guard.lead != "||" || guard.block != assigned.block || guard.substituted {
		return false
	}
	switch guard.trail {
	case "", "\n", ";":
	default:
		return false
	}
	words := plainWords(guard.words)
	if len(words) == 0 || len(words) > 2 || unquote(words[0]) != "exit" || len(words) == 2 && strings.Trim(unquote(words[1]), "0123456789") != "" {
		return false
	}
	// The rest of unconditionalAssignment's conditions, with the trail
	// replaced by the guard's: a lone word at the top level, unconditional
	// lead and no expansion of the name before it.
	if assigned.substituted || assigned.inFunction || assigned.block != 0 || len(assigned.words) != 1 {
		return false
	}
	switch assigned.lead {
	case "&&", "||", "|", "&":
		return false
	}
	saved := assigned.trail
	parsed.commands[at].trail = ""
	ok := unconditionalAssignment(r, parsed, text, name)
	parsed.commands[at].trail = saved
	return ok
}
