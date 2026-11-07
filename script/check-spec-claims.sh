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
 my ($digest,$line)=(0,0);
 while(<$fh>) {
  $line++;
  if(/^##\s+Agent digest\s*$/) {$digest=1;next}
  if(/^##\s/) {$digest=0}
  next unless $digest && /^- Claim:\s*(.*?)\s*$/;
  my $length=length($1);
  if($length>160) {printf STDERR "%s:%d: Agent digest Claim is %d characters; limit 160; shorten it before staging (SRG-V0-763, LVT-V0-010)\n",$path,$line,$length;$failed=1}
 }
 close $fh or die "$path: $!\n";
}
exit($failed ? 1 : 0);
PERL
