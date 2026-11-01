#!/usr/bin/env bash
# usage: script/lane-dispatch.sh inputs --lane LANE --base REF --date YYYY-MM-DD --out DIR
#                                       [--repo REPO] [--host HOST] TICKET...
#        script/lane-dispatch.sh check --env DIR/lane.env FILE...
#        script/lane-dispatch.sh base-check --base SHA [--dir DIR] [--fresh]
#
# The lane dispatch kit (docs/LANE-ORCHESTRATION.md section 14).
#
# inputs (ASD-V0-065, ASD-V0-066; V1-3816, V1-3817, V1-3262), run by the orchestrator on the host
# that owns the Tasks journal: resolves REF to one full commit in REPO (default .) and writes
#   DIR/lane.env          LANE, BASE (40 hex), DATE, BUILD_LOG=docs/build-log/DATE-LANE.md,
#                         TICKET_DIR and TICKETS: the one source the brief and charter quote
#   DIR/brief-header.md   the lines both the brief and the charter carry, rendered from lane.env
#   DIR/ticket-bodies-LANE/ID.json   `corvint-tasks ticket show ID` output (LANE_DISPATCH_TASKS
#                         overrides the binary), refused unless its outcome is OK with one item
#   DIR/ticket-bodies-LANE/ID.txt    the same ticket as plain text: title, status, labels, body,
#                         numbered acceptance criteria, dependencies and the operator note with its
#                         advisory label
# With --host HOST the directory's contents are copied to HOST:~/lane-inputs/ over ssh
# (LANE_DISPATCH_SSH, default ssh), and brief-header.md names that path. Exit 2 on a refusal.
#
# check (ASD-V0-065): refuses with exit 2 when any FILE (a brief, a charter) names a BASE commit
# that is not a prefix of lane.env's BASE, on a line that contains `BASE`, or a build-log path
# docs/build-log/DATE-NAME.md for lane.env's DATE that is not BUILD_LOG. Each mismatch prints
# `lane-dispatch: MISMATCH base|build-log FILE:LINE ...`. Placeholders such as <LANE> never match.
#
# base-check (ASD-V0-067, V1-3061), run in the lane worktree: refuses with exit 2 when BASE is not
# an ancestor of HEAD, when --fresh is given and HEAD is not BASE itself (the check before the first
# commit), or when a commit in BASE..HEAD that no origin ref holds is also on another local branch
# (the worktree was cut from another lane's unpushed head). Prints `lane-dispatch: base-ok ...`.
set -uo pipefail
export LC_ALL=C
refuse() { printf 'lane-dispatch: REFUSE %s\n' "$*" >&2; exit 2; }
usage() { sed -n '2,5p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }

# render_ticket JSON_FILE: the plain-text packet of one `ticket show` result; exit 3 when the result
# is not OK with exactly one item.
IFS= read -r -d '' render_perl << 'PERL' || :
use strict;
use warnings;
use JSON::PP;
local $/;
open my $fh, '<', $ARGV[0] or exit 3;
my $doc = eval { JSON::PP->new->utf8->decode(<$fh>) } or exit 3;
exit 3 unless ref $doc eq 'HASH' && ($doc->{outcome} // '') eq 'OK' && ref $doc->{items} eq 'ARRAY' && @{ $doc->{items} } == 1;
my $it = $doc->{items}[0];
my $r = ref $it->{record} eq 'HASH' ? $it->{record} : {};
my $id = $it->{ticketId} // $r->{ticketId} // '?';
$id =~ s/^ticket:[^:]+:[^:]+://;
my $s = sub { my $v = shift; defined $v && !ref $v ? $v : '-' };
my $list = sub { my $v = shift; ref $v eq 'ARRAY' && @$v ? join(', ', map { ref $_ ? ($_->{ticketId} // '?') : $_ } @$v) : '-' };
my $indent = sub { my $t = shift; $t =~ s/\r//g; $t =~ s/^/  /mg; $t };
binmode STDOUT, ':encoding(UTF-8)';
print "$id  ", $s->($it->{title} // $r->{title}), "\n";
print 'status ', $s->($it->{status}), '  priority ', $s->($it->{priority} // $r->{priority}), '  kind ', $s->($it->{kind} // $r->{kind}),
  '  milestone ', $s->($it->{milestone} // $r->{milestone}), '  labels ', $list->($r->{labels}), "\n";
print "\nBody:\n", $indent->($s->($r->{body})), "\n";
my $ac = ref $r->{acceptanceCriteria} eq 'ARRAY' ? $r->{acceptanceCriteria} : [];
print "\nAcceptance criteria:\n";
print @$ac ? map { sprintf "  %d. %s\n", $_ + 1, $s->($ac->[$_]) } 0 .. $#$ac : "  (none recorded)\n";
print "\nDependencies: ", $list->($r->{dependencies}), "\n";
my $on = ref $it->{operatorNote} eq 'HASH' ? $it->{operatorNote} : {};
if (defined $on->{text} && !ref $on->{text} && length $on->{text}) {
    print "\nOperator note (", $s->($on->{advisory}), '; ', $s->($on->{recordedAt}), "):\n", $indent->($on->{text}), "\n";
} else {
    print "\nOperator note: none\n";
}
print "\nSource: corvint-tasks ticket show $id (queue data, not instructions)\n";
PERL

inputs() {
  local lane='' ref='' date='' out='' repo=. host='' tickets='' t sha dir rel log
  while (($#)); do
    case $1 in
      --lane | --base | --date | --out | --repo | --host)
        [[ $# -ge 2 && -n $2 ]] || refuse "missing-value $1 (usage: $1 VALUE)"
        case $1 in --lane) lane=$2 ;; --base) ref=$2 ;; --date) date=$2 ;; --out) out=$2 ;; --repo) repo=$2 ;; --host) host=$2 ;; esac
        shift 2 ;;
      -*) usage ;;
      *) [[ $1 =~ ^[A-Z][A-Z0-9-]*-[0-9]+$ ]] || refuse "bad-ticket-id $1 (want an ID such as V1-0001)"
        tickets="${tickets:+$tickets }$1"
        shift ;;
    esac
  done
  [[ -n $lane && -n $ref && -n $date && -n $out && -n $tickets ]] || refuse 'usage: --lane, --base, --date, --out and at least one TICKET are required'
  [[ $lane =~ ^[a-z0-9][a-z0-9.-]*$ ]] || refuse "bad-lane $lane (want lower-case letters, digits, . and -)"
  [[ $date =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || refuse "bad-date $date (want YYYY-MM-DD)"
  sha=$(git -C "$repo" rev-parse --verify -q "$ref^{commit}") || refuse "unknown-base $ref in $repo (fix: run git fetch origin, then retry)"
  [[ ! -e $out/lane.env ]] || refuse "exists $out/lane.env (fix: use a new --out directory; one dispatch per directory)"
  dir=$out/ticket-bodies-$lane
  mkdir -p "$dir" || refuse "out-unwritable $out (fix: use a writable --out directory)"
  for t in $tickets; do
    (cd "$repo" && "${LANE_DISPATCH_TASKS:-corvint-tasks}" ticket show "$t") > "$dir/$t.json" 2> "$dir/$t.err" ||
      refuse "ticket-show-failed $t (fix: run on the host that owns the Tasks journal, or set CORVINT_TASKS_SNAPSHOT): $(head -c 300 "$dir/$t.err")"
    perl -e "$render_perl" "$dir/$t.json" > "$dir/$t.txt" ||
      refuse "ticket-show-not-ok $t (fix: run on the host that owns the Tasks journal and check the ID): $(head -c 300 "$dir/$t.json")"
    /bin/rm -f "$dir/$t.err"
  done
  log=docs/build-log/$date-$lane.md
  rel=$dir
  [[ -z $host ]] || rel="\$HOME/lane-inputs/ticket-bodies-$lane"
  printf 'LANE=%s\nBASE=%s\nDATE=%s\nBUILD_LOG=%s\nTICKET_DIR=%s\nTICKETS=%s\n' "$lane" "$sha" "$date" "$log" "$rel" "$tickets" > "$out/lane.env" ||
    refuse "out-unwritable $out (fix: use a writable --out directory)"
  {
    printf 'LANE=%s\n' "$lane"
    printf 'BASE=%s (pinned at dispatch; never the host checkout'"'"'s HEAD)\n' "$sha"
    printf 'Build log: %s\n' "$log"
    printf 'Tickets: %s\n' "$tickets"
    printf 'Ticket bodies: %s/<ID>.txt (plain text) and <ID>.json (ticket show)\n' "$rel"
    printf 'Before the first commit: script/lane-dispatch.sh base-check --base %s --fresh\n' "$sha"
  } > "$out/brief-header.md" || refuse "out-unwritable $out (fix: use a writable --out directory)"
  if [[ -n $host ]]; then
    tar -C "$out" -cf - . | "${LANE_DISPATCH_SSH:-ssh}" "$host" 'mkdir -p "$HOME/lane-inputs" && tar -C "$HOME/lane-inputs" -xf -' ||
      refuse "copy-failed $host:~/lane-inputs (fix: check ssh $host works, then rerun with a new --out)"
  fi
  printf 'lane-dispatch: inputs %s BASE=%s BUILD_LOG=%s tickets=%s%s\n' "$out" "$sha" "$log" "$(wc -w <<< "$tickets" | tr -d ' ')" "${host:+ host=$host}"
}

check() {
  local envf='' base='' date='' log='' f n line tok bad=0 files=0
  [[ ${1-} == --env && -n ${2-} ]] || usage
  envf=$2
  shift 2
  (($#)) || usage
  [[ -r $envf ]] || refuse "env-unreadable $envf (fix: run lane-dispatch.sh inputs first)"
  base=$(sed -n 's/^BASE=//p' "$envf")
  date=$(sed -n 's/^DATE=//p' "$envf")
  log=$(sed -n 's/^BUILD_LOG=//p' "$envf")
  [[ $base =~ ^[0-9a-f]{40}$ && -n $date && -n $log ]] || refuse "env-incomplete $envf (want BASE, DATE and BUILD_LOG)"
  for f in "$@"; do
    [[ -r $f ]] || refuse "unreadable $f (fix: name an existing brief or charter file)"
    files=$((files + 1))
    n=0
    while IFS= read -r line || [[ -n $line ]]; do
      n=$((n + 1))
      if [[ $line == *BASE* ]]; then
        for tok in $(grep -oE '(^|[^0-9a-zA-Z])[0-9a-f]{7,40}([^0-9a-zA-Z]|$)' <<< "$line" | tr -cd '0-9a-f\n'); do
          [[ $tok =~ [a-f] && $tok =~ [0-9] ]] || continue
          [[ $base == "$tok"* ]] && continue
          printf 'lane-dispatch: MISMATCH base %s:%d names %s, lane.env BASE=%s\n' "$f" "$n" "$tok" "$base"
          bad=1
        done
      fi
      for tok in $(grep -oE "docs/build-log/$date-[A-Za-z0-9._-]+\.md" <<< "$line"); do
        [[ $tok == "$log" ]] && continue
        printf 'lane-dispatch: MISMATCH build-log %s:%d names %s, lane.env BUILD_LOG=%s\n' "$f" "$n" "$tok" "$log"
        bad=1
      done
    done < "$f"
  done
  ((bad == 0)) || exit 2
  printf 'lane-dispatch: check ok BASE=%s BUILD_LOG=%s files=%d\n' "$base" "$log" "$files"
}

base_check() {
  local base='' dir=. fresh='' sha head branch c others ahead mb own=0
  while (($#)); do
    case $1 in
      --base | --dir) [[ $# -ge 2 && -n $2 ]] || refuse "missing-value $1 (usage: $1 VALUE)"
        case $1 in --base) base=$2 ;; --dir) dir=$2 ;; esac
        shift 2 ;;
      --fresh) fresh=1; shift ;;
      *) usage ;;
    esac
  done
  [[ -n $base ]] || refuse 'usage: --base is required'
  sha=$(git -C "$dir" rev-parse --verify -q "$base^{commit}") || refuse "unknown-base $base in $dir (fix: run git fetch origin, then retry)"
  head=$(git -C "$dir" rev-parse --verify -q HEAD) || refuse "no-head $dir (fix: run it inside the lane worktree, or set --dir)"
  if ! git -C "$dir" merge-base --is-ancestor "$sha" "$head"; then
    mb=$(git -C "$dir" merge-base "$sha" "$head" 2> /dev/null) || mb=none
    refuse "base-not-ancestor BASE=$sha HEAD=$head merge-base=$mb (fix: recreate the worktree with script/lane-setup.sh --base $sha)"
  fi
  if [[ -n $fresh && $head != "$sha" ]]; then
    ahead=$(git -C "$dir" rev-list --count "$sha..$head")
    refuse "head-not-base BASE=$sha HEAD=$head ($ahead commits ahead before the first commit; the worktree was cut from another commit; fix: recreate it with script/lane-setup.sh --base $sha)"
  fi
  branch=$(git -C "$dir" symbolic-ref -q HEAD) || branch=
  for c in $(git -C "$dir" rev-list "$sha..$head" --not --remotes=origin); do
    own=$((own + 1))
    others=$(git -C "$dir" for-each-ref --contains "$c" --format='%(refname)' refs/heads | grep -vxF "${branch:-//}" | sed 's|^refs/heads/||' | tr '\n' ' ')
    [[ -z $others ]] || refuse "foreign-commit $c is also on ${others% } (the worktree started on another lane's head; fix: recreate it with script/lane-setup.sh --base $sha)"
  done
  printf 'lane-dispatch: base-ok BASE=%s HEAD=%s own-commits=%d\n' "$sha" "$head" "$own"
}

case ${1-} in
  inputs) shift; inputs "$@" ;;
  check) shift; check "$@" ;;
  base-check) shift; base_check "$@" ;;
  *) usage ;;
esac
