// SPDX-License-Identifier: Apache-2.0
package shell

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
	for index, cmd := range parsed.commands {
		word, args := commandWord(plainWords(cmd.words))
		switch word {
		case "alias", "hash", "enable", "eval":
			return false
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
