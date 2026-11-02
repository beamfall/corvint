#!/bin/sh
# script/line-citations-resolve.sh (DCG-V0-745, V1-3746): after a merge, resolve every conflict
# block in an unmerged scanned document whose two sides differ only in pinned citations' line spans,
# by keeping the current branch's side; stage each document it fully resolves, then run
# `script/check-line-citations.sh --repin`, because the merged code decides the numbers anyway. A
# block that differs in anything else is refused, naming its line, and its document is left
# unresolved and unstaged. Exit 1 when any unmerged document is refused; otherwise exit with
# --repin's status, so a citation that still fails after the repin fails the run. Only the gate's
# exact document set is scanned (scanned_doc in script/check-line-citations.sh), so a nested path it
# excludes, such as docs/build-log/*.md, is skipped. A document is refused unless both sides are in
# the index (a modify/delete or other conflict without a stage 2 and 3 has no markers to read), its
# marker size is Git's default 7, and its markers form at least one fully validated block.
set -eu

root=$(git rev-parse --show-toplevel)
cd "$root"
status=0
for doc in $(git diff --name-only --diff-filter=U); do
  printf '%s\n' "$doc" | grep -Eq '^(docs/(specs|agent-memory|decisions)/[^/]+\.md|README\.md|docs/(DOGFOOD|AGENT-ROUTES|GLOSSARY|AUTOMATION|SELF-DEVELOPMENT|SPEC-TOOLCHAIN-INTEGRATION|RELEASE-NOTES-[^/]*)\.md|conformance/[^/]+/README\.md)$' ||
    continue
  stages=$(git ls-files -u -- "$doc" | awk '{print $3}' | sort -u | tr -d '\n')
  case $stages in
  123 | 23) ;;
  *)
    printf 'line citations: %s  conflict is not a two-sided content conflict (index stages %s); resolve it by hand\n' "$doc" "$stages" >&2
    status=1
    continue
    ;;
  esac
  size=$(git check-attr conflict-marker-size -- "$doc" | sed 's/.*: conflict-marker-size: //')
  case $size in
  unspecified | unset | 7) ;;
  *)
    printf 'line citations: %s  sets conflict-marker-size %s, which this resolver does not parse; resolve it by hand\n' "$doc" "$size" >&2
    status=1
    continue
    ;;
  esac
  if perl -e '
    my ($doc) = @ARGV;
    open my $in, "<", $doc or die "$doc: $!\n";
    my @lines = <$in>;
    close $in;
    sub mask { my $t = join "", @_; $t =~ s/:\d[\d,-]*(?=\@[0-9a-f]{4,64}`)/:N/g; $t }
    my (@out, $refused, $state, $start, @ours, @theirs);
    my $blocks = 0;
    sub bad { print STDERR "line citations: $_[0]:$_[1]  $_[2]; resolve it by hand\n"; exit 1 }
    for my $i (0 .. $#lines) {
        local $_ = $lines[$i];
        my $marker = /^(?:<{7,}|>{7,}|\|{7,})(?: |\r?\n?\z)|^={7,}\r?\n?\z/;
        if (!defined $state && /^<<<<<<< /) { ($state, $start, @ours, @theirs) = ("ours", $i + 1); next }
        if (!defined $state) {
            bad($doc, $i + 1, "conflict marker outside a <<<<<<< block") if $marker;
            push @out, $_;
            next;
        }
        if (/^\|\|\|\|\|\|\| / && $state eq "ours") { $state = "base"; next }
        if (/^=======\r?\n?\z/ && ($state eq "ours" || $state eq "base")) { $state = "theirs"; next }
        if (/^>>>>>>> / && $state eq "theirs") {
            ++$blocks;
            if (mask(@ours) eq mask(@theirs)) { push @out, @ours }
            else { print STDERR "line citations: $doc:$start  conflict differs in more than citation line numbers; resolve it by hand\n"; $refused = 1 }
            undef $state;
            next;
        }
        bad($doc, $i + 1, "conflict marker out of order in the block at line $start") if $marker;
        push @ours, $_ if $state eq "ours";
        push @theirs, $_ if $state eq "theirs";
    }
    bad($doc, $start, "conflict block is not closed") if defined $state;
    bad($doc, 1, "no conflict block found") unless $blocks;
    exit 1 if $refused;
    open my $out, ">", $doc or die "$doc: $!\n";
    print $out @out;
    close $out;
  ' "$doc"; then
    git add -- "$doc"
    printf 'line citations: resolved %s to the current side'"'"'s text; --repin sets its numbers\n' "$doc"
  else
    status=1
  fi
done
# A refused block leaves the merge unfinished, so nothing is repinned until it is resolved.
[ "$status" = 0 ] || exit 1
rc=0
script/check-line-citations.sh --repin || rc=$?
exit "$rc"
