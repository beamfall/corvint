#!/usr/bin/env bash
# usage: review-head-check.sh [--reviewed-head REV] REVIEW_FILE [HEAD]
# ASD-V0-1021..1023 (V1-3959, V1-3668): compare a saved review with the lane head before a fix
# round or a seal. The reviewed head is the `codex-review: reviewed-head=SHA` pin that
# script/codex-review.sh prints first (ASD-V0-1020), or --reviewed-head for a review saved without
# one. HEAD defaults to the checked-out commit.
#   CURRENT  HEAD changes nothing outside .corvint/ since the reviewed head: the review covers HEAD
#            and the change may bind and seal (exit 0).
#   MOVED    HEAD changed paths since the review (exit 1). Each finding (a line carrying a [P0]..[P3]
#            marker, through the next such line) prints one line:
#              finding N POSSIBLY_FIXED|OPEN|UNKNOWN cited=PATH[:L[-M]],... evidence=... :: TITLE
#            POSSIBLY_FIXED: a cited path changed, at the cited lines when the finding names lines;
#            OPEN: no cited path changed, or only lines the finding does not cite; UNKNOWN: the
#            finding cites no path of the reviewed tree. Send only OPEN and UNKNOWN findings back,
#            and review HEAD again before the seal.
# Refusals exit 2 with `review-head-check: REFUSE CODE` and a fix line. Reads Git only; writes nothing.
set -uo pipefail
export LC_ALL=C

# refuse "CODE DETAIL; fix: RECOVERY" prints the REFUSE line and the fix line, then exits 2.
refuse() {
  printf 'review-head-check: REFUSE %s\n' "${1%%; fix: *}" >&2
  [[ $1 == *'; fix: '* ]] && printf '  fix: %s\n' "${1#*; fix: }" >&2
  exit 2
}
if [[ ${1-} == -h || ${1-} == --help ]]; then
  sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
fi
pinned=
if [[ ${1-} == --reviewed-head ]]; then
  [[ $# -ge 2 ]] || refuse "usage: review-head-check.sh [--reviewed-head REV] REVIEW_FILE [HEAD]"
  pinned=$2
  shift 2
fi
[[ $# == 1 || $# == 2 ]] || refuse "usage: review-head-check.sh [--reviewed-head REV] REVIEW_FILE [HEAD]"
review=$1
[[ -f $review && -r $review ]] || refuse "unreadable-review $review; fix: pass the file the review output was saved to"
git rev-parse --git-dir > /dev/null 2>&1 || refuse "not-a-git-checkout $PWD; fix: run from the lane checkout"

pin=$(grep -m 1 -E '^codex-review: reviewed-head=' "$review")
if [[ -z $pinned ]]; then
  [[ -n $pin ]] || refuse "unpinned-review $review (no codex-review: reviewed-head= line); fix: run the review through script/codex-review.sh, which pins the head, or pass --reviewed-head with the commit the reviewer read"
  pinned=${pin#codex-review: reviewed-head=}
  pinned=${pinned%% *}
  [[ $pinned =~ ^[0-9a-f]{40}$ ]] || refuse "pin-not-produced $pin; fix: the review ran outside a Git checkout; rerun it from the lane checkout, or pass --reviewed-head"
fi
reviewed=$(git rev-parse --verify -q "$pinned^{commit}") || refuse "unknown-reviewed-head $pinned; fix: git fetch the lane branch that holds it"
head=$(git rev-parse --verify -q "${2:-HEAD}^{commit}") || refuse "unknown-head ${2:-HEAD}; fix: git fetch the lane branch"
git merge-base --is-ancestor "$reviewed" "$head" ||
  refuse "reviewed-head-not-ancestor reviewed=$reviewed head=$head; fix: the findings are about a commit HEAD does not contain; review HEAD itself, or pass the lane head that descends from the reviewed one"
[[ $pin == *' dirty=yes'* ]] &&
  printf 'review-head-check: NOTE the review read uncommitted changes on top of %s\n' "$reviewed"

changed=$(git diff --no-renames --name-only "$reviewed" "$head" -- . ':(exclude).corvint') || refuse "diff-failed reviewed=$reviewed head=$head; fix: rerun after git fsck shows both commits complete"
if [[ -z $changed ]]; then
  printf 'review-head-check: CURRENT reviewed=%s head=%s\n' "$reviewed" "$head"
  exit 0
fi

# Old-side line ranges each changed path's hunks touch: "PATH<TAB>FIRST<TAB>LAST" (an insertion
# after line A touches A..A+1).
ranges=$(git diff --no-renames -U0 "$reviewed" "$head" -- . ':(exclude).corvint' | perl -ne '
  if (m{^diff --git a/(.*) b/}) { $p = $1; next }
  next unless defined $p && /^@@ -(\d+)(?:,(\d+))? /;
  my ($a, $n) = ($1, defined $2 ? $2 : 1);
  printf "%s\t%d\t%d\n", $p, $a, ($n ? $a + $n - 1 : $a + 1);
')
# Findings: "N<TAB>TITLE<TAB>REF REF ...", one per [P0]..[P3] line and the lines that follow it.
findings=$(perl -ne '
  BEGIN { $n = 0 }
  chomp;
  if (/\[P[0-3]\]/) {
    flush();
    $n++; ($t = $_) =~ s/^\s+|\s+$//g; $t = substr($t, 0, 120); %r = (); @o = ();
  }
  next unless $n;
  for my $w (/([A-Za-z0-9_.\/@+-]+(?::\d+(?:-\d+)?)?)/g) {
    $w =~ s/[.,;]+$//;
    next unless $w =~ m{[./]} && $w !~ m{^\.+$} && $w !~ m{://};
    push @o, $w unless $r{$w}++;
  }
  sub flush { printf "%d\t%s\t%s\n", $n, $t, join(" ", @o) if $n }
  END { flush() }
' "$review")

total=0 fixed=0 open=0 unknown=0
while IFS=$'\t' read -r n title refs; do
  [[ -n $n ]] || continue
  total=$((total + 1))
  cited= evidence= status=
  for ref in $refs; do
    path=${ref%%:*} lines=
    [[ $ref == *:* ]] && lines=${ref#*:}
    git cat-file -e "$reviewed:$path" 2> /dev/null || continue
    [[ $(git cat-file -t "$reviewed:$path") == blob ]] || continue
    cited="${cited:+$cited,}$ref"
    if ! grep -Fxq -- "$path" <<< "$changed"; then
      evidence="${evidence:+$evidence,}unchanged"
      continue
    fi
    if [[ -z $lines ]]; then
      evidence="${evidence:+$evidence,}path-changed"
      status=POSSIBLY_FIXED
      continue
    fi
    lo=${lines%-*} hi=${lines#*-}
    if awk -F '\t' -v p="$path" -v lo="$lo" -v hi="$hi" \
      '$1 == p && $2 <= hi + 0 && $3 >= lo + 0 { hit = 1 } END { exit !hit }' <<< "$ranges"; then
      evidence="${evidence:+$evidence,}lines-changed"
      status=POSSIBLY_FIXED
    else
      evidence="${evidence:+$evidence,}other-lines-changed"
    fi
  done
  if [[ -z $cited ]]; then
    status=UNKNOWN evidence=no-cited-path cited=none
    unknown=$((unknown + 1))
  elif [[ $status == POSSIBLY_FIXED ]]; then
    fixed=$((fixed + 1))
  else
    status=OPEN
    open=$((open + 1))
  fi
  printf 'finding %s %s cited=%s evidence=%s :: %s\n' "$n" "$status" "$cited" "$evidence" "$title"
done <<< "$findings"
count=$(grep -c . <<< "$changed")
printf 'review-head-check: MOVED reviewed=%s head=%s changed=%s findings=%s possibly-fixed=%s open=%s unknown=%s\n' \
  "$reviewed" "$head" "$count" "$total" "$fixed" "$open" "$unknown"
sed -n '1,50s/^/  changed: /p' <<< "$changed"
((count > 50)) && printf '  ... %d more changed paths\n' $((count - 50))
printf '  fix: send only OPEN and UNKNOWN findings to the lane, then review HEAD (script/codex-review.sh) before the seal\n'
exit 1
