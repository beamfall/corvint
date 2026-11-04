#!/usr/bin/env bash
# usage: dogfood-seal.sh BASE
# Checks the bound change, then moves its CEM out of the one shared tracked path
# in a rename-only commit, so concurrent branches never edit the same file
# (docs/DOGFOOD.md §4, DOGFOOD-013, decision 0319).
set -uo pipefail
export LC_ALL=C

base_arg=${1:?usage: dogfood-seal.sh BASE}
repo=$(cd "$(dirname "$0")/.." && pwd)
# The check's stdout streams through unchanged; a copy keeps the DCW-V0-047
# base-independence fields of a declared output change for the PASS line.
check_out=$(mktemp "${TMPDIR:-/tmp}/corvint-dogfood-seal.XXXXXX") || exit 2
review_snapshot=
trap 'rm -f "$check_out" ${review_snapshot:+"$review_snapshot"}' EXIT
# ASD-V0-1024 (V1-3668): review before seal. DOGFOOD_REVIEW names the saved review of the
# bind-ready head; the seal refuses before any check when HEAD changed more than .corvint/ since it.
review_evidence=
review_target=
if [[ -n ${DOGFOOD_REVIEW-} ]]; then
  # Validate, hash and record one bounded snapshot of the supplied bytes.
  [[ -f $DOGFOOD_REVIEW && -r $DOGFOOD_REVIEW ]] || {
    printf 'dogfood-seal: REFUSE review-not-current unreadable report\n' >&2; exit 2;
  }
  review_snapshot=$(mktemp "${TMPDIR:-/tmp}/corvint-seal-review.XXXXXX") || exit 2
  head -c 1048577 "$DOGFOOD_REVIEW" > "$review_snapshot" || exit 2
  [[ $(wc -c < "$review_snapshot") -le 1048576 ]] || {
    printf 'dogfood-seal: REFUSE review-not-current report exceeds 1 MiB\n' >&2; exit 2;
  }
  review_target=$(git -C "$repo" rev-parse 'HEAD^{commit}') || exit 2
  if ! (cd "$repo" && "$repo/script/review-head-check.sh" "$review_snapshot"); then
    printf 'dogfood-seal: REFUSE review-not-current DOGFOOD_REVIEW=%s\n' "$DOGFOOD_REVIEW" >&2
    printf '  fix: save a report for exact HEAD with header dogfood-review: reviewed-head=FULL_SHA verdict=PASS; rerun with DOGFOOD_REVIEW set to its path\n' >&2
    exit 2
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    digest=$(sha256sum < "$review_snapshot") || exit 2
  else
    digest=$(shasum -a 256 < "$review_snapshot") || exit 2
  fi
  digest=${digest%% *}
  reviewed=$(sed -n -E 's/^(dogfood-review|codex-review): reviewed-head=([0-9a-f]{40}).*/\2/p' "$review_snapshot")
  verdict=NOT_OBSERVED
  grep -q '^dogfood-review:' "$review_snapshot" && verdict=PASS
  printf -v review_evidence 'review-before-seal OBSERVED path=%q reviewed-head=%s sha256=sha256:%s verdict=%s' "$DOGFOOD_REVIEW" "$reviewed" "$digest" "$verdict"
  printf 'dogfood-seal: %s\n' "$review_evidence"
else
  printf 'dogfood-seal: NOTE review-before-seal NOT_OBSERVED (set DOGFOOD_REVIEW to a saved head-bound review report)\n' >&2
fi
# DCW-V0-347: a passing check of this BASE, HEAD and inputs is replayed from its
# receipt instead of rerunning the verifiers; any other state reruns them.
DOGFOOD_CHECK_REUSE=1 "$repo/script/dogfood-check.sh" "$base_arg" | tee "$check_out"
check_status=${PIPESTATUS[0]}
(( check_status == 0 )) || exit "$check_status"
independence=$(grep -E '^dogfood-check: (SUMMARY|NOTE) ' "$check_out" |
  grep -oE 'base-independence=NOT_PROVED base=(sha256:[0-9a-f]{64}|NOT_PRODUCED base-refusal=[A-Za-z0-9_-]+) tree=sha256:[0-9a-f]{64} declared=[A-Za-z0-9-]+' |
  head -n 1)
bind=$(git -C "$repo" rev-parse 'HEAD^{commit}') || exit 2
if [[ -n $review_target && $bind != "$review_target" ]]; then
  printf 'dogfood-seal: REFUSE review-not-current HEAD changed during check\n' >&2
  exit 2
fi
if ! git -C "$repo" cat-file -e "$bind:.corvint/change.cem.json" 2>/dev/null; then
  printf 'dogfood-seal: REFUSE nothing-to-seal\n' >&2
  exit 2
fi
# V1-0137: never drop a CEM BASE tracks that this change replaced and no
# .corvint/changes/ file keeps.
base_cem=$(git -C "$repo" rev-parse --verify -q "$base_arg^{commit}:.corvint/change.cem.json" 2>/dev/null)
# A failing ls-tree keeps nothing, so the seal refuses rather than trusting partial output.
archived=$(git -C "$repo" ls-tree -r "$bind" -- .corvint/changes) || archived=
if [[ -n $base_cem && $base_cem != "$(git -C "$repo" rev-parse "$bind:.corvint/change.cem.json")" ]] &&
  ! grep -Fq " $base_cem"$'\t' <<< "$archived"; then
  replaced=$(git -C "$repo" log -1 --format=%H "$base_arg^{commit}" -- .corvint/change.cem.json)
  printf 'dogfood-seal: REFUSE unarchived-base-cem\n' >&2
  printf '  BASE tracks .corvint/change.cem.json (bound at %s) that this change replaced and no .corvint/changes/ file keeps\n' "$replaced" >&2
  # DCW-V0-043: name the exact command that archives BASE's blob under its writer.
  if [[ $replaced =~ ^[0-9a-f]{40}$ ]]; then
    archive=".corvint/changes/$replaced.cem.json"
    printf '  fix: mkdir -p .corvint/changes && git cat-file blob %s > %s && git add %s && git commit -q -m "chore: archive inherited change evidence" -- %s\n' "$base_cem" "$archive" "$archive" "$archive" >&2
    printf '  then rerun dogfood-change, bind, dogfood-check and dogfood-seal with the same BASE (DCW-V0-043)\n' >&2
  fi
  exit 2
fi
sealed=".corvint/changes/$bind.cem.json"
mkdir -p "$repo/.corvint/changes"
git -C "$repo" mv .corvint/change.cem.json "$sealed" || exit 2
commit_args=(-q -m "chore: seal change evidence")
[[ -z $review_evidence ]] || commit_args+=(-m "$review_evidence")
git -C "$repo" commit "${commit_args[@]}" || exit 2
if [[ -n $independence ]]; then
  printf 'dogfood-seal: PASS sealed=%s %s\n' "$sealed" "$independence"
else
  printf 'dogfood-seal: PASS sealed=%s\n' "$sealed"
fi
