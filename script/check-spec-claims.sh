#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# LVT-V0-010: cheap worktree check before staging/generation.
# Usage: script/check-spec-claims.sh (or make spec-claims-check) from a checkout.
set -eu
case ${1:-} in
  --help) printf 'usage: script/check-spec-claims.sh\nCheck working-tree Agent digest Claims (at most 160 characters).\nExample: make spec-claims-check\n'; exit 0 ;;
  '') ;;
  *) printf 'usage: script/check-spec-claims.sh [--help]\n' >&2; exit 2 ;;
esac
top=$(git rev-parse --show-toplevel) || exit 2
cd "$top"
paths=$(mktemp "${TMPDIR:-/tmp}/spec-claims.XXXXXX")
trap 'rm -f "$paths"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
git ls-files -z --cached --others --exclude-standard -- docs/specs > "$paths"
perl - "$paths" <<'PERL'
use strict;
use warnings;
open my $paths, '<:raw', $ARGV[0] or die "$ARGV[0]: $!\n";
my @paths=do {local $/="\0"; <$paths>};
my (%seen,$failed);
for my $path (@paths) {
 chomp($path) if substr($path,-1) eq "\0";
 $path =~ s/\0$//;
 next unless $path =~ /\.md$/ && !$seen{$path}++;
 next unless -e $path; # A worktree deletion has no Claim to validate.
 open my $fh, '<:encoding(UTF-8)', $path or die "$path: $!\n";
 # LVT-V0-010: read the digest as script/gen-spec-requirements.sh does: the first
 # "## Agent digest" heading, horizontal whitespace allowed around it, runs to the
 # first blank line; each trimmed "- KEY: VALUE" line sets KEY, the last one wins.
 my ($state,$line,$claim,$claimline)=(0,0);
 while(my $text=<$fh>) {
  $line++;
  $text =~ s/\r?\n\z//;
  if(!$state) {$state=1 if $text =~ /^[^\S\n]*## Agent digest[^\S\n]*$/; next}
  $text =~ s/\A\s+|\s+\z//g;
  last if $text eq '';
  next unless $text =~ s/\A- //;
  my ($key,$value)=split /: /,$text,2;
  ($claim,$claimline)=($value,$line) if $key eq 'Claim' && defined $value;
 }
 if(defined $claim && length($claim)>160) {printf STDERR "%s:%d: Agent digest Claim is %d characters; limit 160; shorten it before staging (SRG-V0-763, LVT-V0-010)\n",$path,$claimline,length($claim);$failed=1}
 close $fh or die "$path: $!\n";
}
exit($failed ? 1 : 0);
PERL
