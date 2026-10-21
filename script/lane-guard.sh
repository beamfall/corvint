#!/usr/bin/env bash
# usage: script/lane-guard.sh < hook-input.json
#
# Claude Code PreToolUse(Bash) hook for Corvint agent lanes (ASD-V0-024,
# docs/LANE-ORCHESTRATION.md section 4). It reads the hook input on stdin and prints a deny
# decision when the command
#   - runs a foreground polling loop: until/while ... sleep, for v in $(seq|jot ...) ... sleep,
#     or tail -f ... | grep -m (ASD-V0-016); or
#   - runs go test without the host test semaphore heavy.sh (ASD-V0-014).
# Heredoc bodies are file content, not commands, so they are never matched.
#
#   LANE_GUARD_ROOTS  colon-separated directory prefixes; when set, a command whose cwd is under
#                     none of them is allowed unchecked. Unset: every command is checked.
#   LANE_GUARD_HEAVY  the wrapper path the deny message names; default script/heavy.sh.
# Prints nothing (allow) or one JSON decision line; always exits 0.
set -uo pipefail

if [[ ${1-} == -h || ${1-} == --help ]]; then
  sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
fi

exec perl -MJSON::PP -e '
use strict;
use warnings;
local $/;
my $raw = <STDIN>;
my $in = eval { JSON::PP->new->decode($raw // "") };
exit 0 unless ref $in eq "HASH";
my $cwd = defined $in->{cwd} ? "$in->{cwd}" : "";
my $cmd = ref $in->{tool_input} eq "HASH" && defined $in->{tool_input}{command} ? "$in->{tool_input}{command}" : "";
if (defined $ENV{LANE_GUARD_ROOTS} && $ENV{LANE_GUARD_ROOTS} ne "") {
    my $inside = 0;
    for my $root (grep { $_ ne "" } split /:/, $ENV{LANE_GUARD_ROOTS}) {
        $root =~ s{/+$}{};
        $inside = 1 if $cwd eq $root || index($cwd, "$root/") == 0;
    }
    exit 0 unless $inside;
}
# Drop heredoc bodies: every line after a <<WORD (or <<-WORD, quoted or not) up to its
# terminator line. The line carrying the operator stays a command.
my (@kept, @pending);
for my $line (split /\n/, $cmd, -1) {
    if (@pending) {
        my ($word, $strip) = @{ $pending[0] };
        my $probe = $line;
        $probe =~ s/^\t+// if $strip;
        shift @pending if $probe eq $word;
        next;
    }
    push @kept, $line;
    while ($line =~ /(?<!<)<<(-?)[ \t]*(?:"([^"\n]+)"|\x27([^\x27\n]+)\x27|\\?([A-Za-z_][A-Za-z0-9_.-]*))/g) {
        push @pending, [defined $2 ? $2 : defined $3 ? $3 : $4, $1 eq "-"];
    }
}
my $flat = join " ", @kept;
my $heavy = $ENV{LANE_GUARD_HEAVY} // "script/heavy.sh";
sub deny {
    print JSON::PP->new->canonical->encode({hookSpecificOutput => {hookEventName => "PreToolUse",
        permissionDecision => "deny", permissionDecisionReason => $_[0]}}), "\n";
    exit 0;
}
if ($flat =~ /(?:^|[;&|({\s])(?:until|while)\s.*sleep/ ||
    $flat =~ /for\s+[A-Za-z_]+\s+in\s+\$\((?:seq|jot)\s.*sleep/ ||
    $flat =~ /tail\s+(?:-[^|]*\s)?-[A-Za-z]*[fF][^|]*\|[^;&]*grep\s.*-m/) {
    deny("Lane rule: no polling loops (until/while/for-seq with sleep, tail -f | grep -m). Run the command in the foreground under 600s, or end your turn and let the orchestrator wake you.");
}
if ($flat =~ /(?:^|[;&|(]|&&|\|\|)\s*(?:(?:[A-Za-z_]+=\S*|env|time|nice|command)\s+)*go\s+test(?:\s|$)/ &&
    $flat !~ /(?:^|[\s;&|(])\S*heavy\.sh(?:\s|$)/) {
    deny("Lane rule: run go test through $heavy (e.g. $heavy env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run <narrow> ./pkg/), one package per command.");
}
exit 0;
'
