// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"regexp"
	"strings"
)

// hostEnvironmentOperand is the checked AFP-V0-3341 form. The substitution
// prints only the canonical path of a regular file outside Git checkouts and
// the supplied checkout/clone/worktree roots. On refusal it prints nothing,
// so source receives an empty operand and fails. Keeping the validation inside
// the operand prevents a skipped guard or a reassigned result from being admitted.
// This is deliberately byte-exact, including quotes: unquote would erase Perl
// strings and shell quoting that matter to the proof. It is not a filename rule.
const hostEnvironmentOperand = `"$(command -p perl -MCwd=realpath -MFile::Basename=dirname -e '
    my $file = realpath(shift @ARGV);
    die "host-only ENVFILE: unreadable path\n" unless defined($file) && -f $file && $file !~ /[\r\n]/;
    die "host-only ENVFILE: hard-linked $file\n" if (stat $file)[3] != 1;
    for my $root (@ARGV) {
      $root =~ s{^~(?=/|$)}{$ENV{HOME}};
      next unless length($root) && -e $root;
      $root = realpath($root);
      die "host-only ENVFILE: unresolved root\n" unless defined($root);
      die "host-only ENVFILE: repository path $file\n" if $file eq $root || index($file, "$root/") == 0 || $root eq "/";
    }
    for (my $dir = dirname($file); ; $dir = dirname($dir)) {
      die "host-only ENVFILE: Git checkout $file\n" if -e "$dir/.git" || -l "$dir/.git";
      last if $dir eq dirname($dir);
    }
    print $file;
  ' -- "${ENVFILE/#\~/$HOME}" "${CHECKOUT-}" "${CLONE:-$HOME/corvint}" "${ROOT:-$root}/wt")"`

func hostEnvironmentSource(r *resolver, parsed script, text string, at int) bool {
	// A command scanned from a command string (`bash -c`, appended past the
	// file's own commands) or inside a substitution is outside the one form.
	if at >= len(parsed.commands) || parsed.commands[at].substituted {
		return false
	}
	words := parsed.commands[at].words
	// A prefix assignment could redirect the validator; extra operands or
	// redirections are outside this one checked form.
	if len(words) != 2 || (words[0] != "." && words[0] != "source") || words[1] != hostEnvironmentOperand {
		return false
	}
	// command -p bypasses PATH and functions named perl, but a function named
	// command or shell command rewriting would invalidate that guarantee.
	if parsed.definitions["command"] != 0 || parsed.definitions["."] != 0 || parsed.definitions["source"] != 0 {
		return false
	}
	// command -p still runs command_not_found_handle when perl is missing, and
	// perl itself loads modules named by PERL5LIB, PERL5OPT or PERLLIB (and the
	// loader preloads by LD_ or DYLD_ names): any mention anywhere, strings and
	// prose included, is refused because assignments can be built dynamically.
	normalized := strings.ReplaceAll(text, "\\\n", "")
	normalized = strings.NewReplacer("'", "", `"`, "", `\`, "").Replace(normalized)
	if strings.Contains(text, "command_not_found_handle") || hostEnvironmentHijack.MatchString(text) ||
		strings.Contains(normalized, "command_not_found_handle") || hostEnvironmentHijack.MatchString(normalized) {
		return false
	}
	for index, cmd := range parsed.commands {
		word, args := commandWord(plainWords(cmd.words))
		switch word {
		case "alias", "hash", "enable", "eval":
			return false
		case "trap":
			// A trap string is not parsed as commands: only a reset, a listing or
			// an exit/signal handler built from commands proven inert is admitted.
			if !hostTrapAllowed(parsed, args) {
				return false
			}
		case ".", "source", "load":
			// Another source could replace command, and a function body runs
			// in call order rather than text order, so a source anywhere else
			// is admitted only in the proven fresh runtime metadata form
			// (TSP-V0-004).
			if index != at && (len(args) == 0 || !freshDirectorySource(r, parsed, text, index, unquote(args[0]))) {
				return false
			}
		}
	}
	return true
}

// hostEnvironmentHijack matches a variable name that makes perl or the
// dynamic loader read code chosen outside the validator: PERL-prefixed names
// longer than PERL itself (PERL5LIB, PERL5OPT, PERLLIB, PERLIO, PERL_...) and
// LD_ or DYLD_ loader names.
var hostEnvironmentHijack = regexp.MustCompile(`(^|[^A-Za-z0-9_])(PERL[A-Za-z0-9_]+|LD_[A-Za-z0-9_]+|DYLD_[A-Za-z0-9_]+)`)

// hostTrapAllowed reports a trap that cannot change the shell state the
// validator relies on before it runs: no operands or a listing option, a
// reset (`-`), or a handler for EXIT or the INT, TERM, HUP and QUIT signals
// whose text lexes to commands, without definitions or assignments, that each
// run a file-defined function (whose body the caller scans), an inert builtin
// or an explicitly allowed cleanup command. DEBUG, RETURN and ERR handlers run
// between ordinary commands, so they and any other signal are refused.
func hostTrapAllowed(parsed script, args []string) bool {
	if len(args) > 0 && unquote(args[0]) == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return true
	}
	action, signals := unquote(args[0]), args[1:]
	switch {
	case action == "-p" || action == "-l":
		return true
	case action == "-":
	case strings.HasPrefix(action, "-") || len(signals) == 0:
		return false
	}
	for _, raw := range signals {
		switch strings.TrimPrefix(unquote(raw), "SIG") {
		case "EXIT", "0", "INT", "TERM", "HUP", "QUIT", "1", "2", "3", "15":
		default:
			return false
		}
	}
	if action == "-" {
		return true
	}
	// Preserve the handler's inner quoting: unquote removes every quote,
	// including the double quotes that keep git's directory a single word.
	raw := args[0]
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' && strings.Count(raw, "'") == 2 {
		action = raw[1 : len(raw)-1]
	} else if strings.ContainsAny(raw, "$`\\") {
		// Other forms may expand while installing the handler.
		return false
	}
	// Inspect the entire action before parsing: parameter expansions and
	// redirections can hide executable words, even as arguments to ':' or
	// inside a single-quoted action that expands only when the trap runs.
	action = strings.ReplaceAll(action, "\\\n", "")
	if strings.Contains(action, "$(") || strings.Contains(action, "`") ||
		// An input redirection is a read the reader index never records.
		strings.Contains(action, "<") || strings.Contains(action, ">(") ||
		hostTrapExpandedParameterWord.MatchString(action) {
		return false
	}
	handler := parse(action)
	if len(handler.definitions) != 0 {
		return false
	}
	for _, cmd := range handler.commands {
		plain := plainWords(cmd.words)
		for _, word := range plain {
			if assignment.MatchString(word) {
				return false
			}
		}
		if len(plain) == 0 {
			continue
		}
		// Inspect the actual verb: commandWord would unwrap external runners
		// such as env, which are outside this exact allowlist.
		word := unquote(plain[0])
		if parsed.definitions[word] != 0 || hostTrapInert[word] {
			continue
		}
		switch word {
		case "rm", "/bin/rm", "rmdir", "/bin/rmdir":
		case "git":
			args := plain[1:]
			for len(args) >= 2 && unquote(args[0]) == "-C" {
				// Unquoted expansion (and even quoted $@) can inject global
				// options. Admit literal paths or the host runner's single-word form.
				if args[1] != `"${clone:-/nonexistent}"` && !hostTrapGitDirectory.MatchString(unquote(args[1])) {
					return false
				}
				args = args[2:]
			}
			if len(args) == 0 || unquote(args[0]) != "update-ref" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// hostTrapInert are the builtins a trap handler may run: none defines,
// sources, assigns or rewrites a command.
var hostTrapInert = map[string]bool{
	"exit": true, "return": true, ":": true, "true": true, "false": true,
	"[[": true, "[": true, "test": true, "echo": true, "kill": true, "wait": true,
}

// hostTrapGitDirectory excludes expansions, globbing and shell metacharacters.
var hostTrapGitDirectory = regexp.MustCompile(`^[A-Za-z0-9_./+-]+$`)

// hostTrapExpandedParameterWord conservatively refuses an expansion in an
// operator's word, including nested parameter expansions. Literal defaults
// remain allowed; command and arithmetic substitutions are refused separately.
var hostTrapExpandedParameterWord = regexp.MustCompile(`\$\{[^}]*[-+=?][^}]*\$`)
