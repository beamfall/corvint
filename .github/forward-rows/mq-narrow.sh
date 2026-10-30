#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-731: the selection step of a merge-queue entry. The go-product-shard job runs this copy
# from the queue base (merge_group.base_sha) when main contains it, else from the newest commit of
# main it descends from (AFP-V0-934), never from the entry under test,
# in the tested checkout, only after the narrowing-verdict job said MET for the entry (AFP-V0-730).
# It selects over the whole queue range SELECTION_BASE..TESTED_MERGE, so an entry stacked on earlier
# entries of its group selects for their changes too, with the pinned planner (`corvint affected
# --base SELECTION_BASE`) and the pinned selector (tools/gate-affected-select), and writes their
# verdict as a driver-shaped selection. narrow.sh beside it then applies every AFP-V0-098 guard
# (no .github/ change in the range, the shadow superset, the AFP-V0-590 balanced placement) and
# writes the corvint-pr-narrowing/0 record, the executed sidecar and the step output `mode`.
# Environment: as narrow.sh except PR_BASE, PR_HEAD, BASE_REF and PR_NUMBER, which this script
# derives, plus QUEUE_BASE, QUEUE_BASE_REF and QUEUE_HEAD (the merge_group event's base_sha,
# base_ref and head_sha) and QUEUE_HEAD_REF (its head_ref). PR_BIN holds the pinned corvint and
# selector. Anything missing or disagreeing, a planner or selector failure, and a selector verdict
# other than `run` (FALLBACK for an UNKNOWN plan, NOTHING) leave the selection with a reason, so
# narrow.sh records FULL and the shard runs whole; this script never fails the shard.
# AFP-V0-812 to AFP-V0-816 (V1-3100): MQ_DELTA, docs-plan's optional candidate, names a narrowed
# pull-request record of this entry's head on an older base of main, read only from that run's
# ci-narrowed-tree-record log. When it verifies here the selection keeps only the packages the main
# delta can change (or the record's range did not select) and names the rest as Reused, with the
# record in Reuse; any refusal keeps the whole selection and names why in Reuse.
set -u
here=$(cd "$(dirname "$0")" && pwd)
sel="$RUNNER_TEMP/corvint-mq-selection"
mkdir -p "$sel"
hex40() { printf '%s' "$1" | grep -Eqx '[0-9a-f]{40}'; }
# SELECTION_BASE: the base the queue-range selection is measured from, the queue base unless it is a
# stacked entry's (AFP-V0-934).
SELECTION_BASE=${QUEUE_BASE:-}
# select REASON [PACKAGE...]: write the driver-shaped selection; any REASON makes narrow.sh FULL.
# Without a REASON the AFP-V0-813 reuse files beside it add Reused and Reuse.
select_() {
  reason=$1
  shift
  printf '%s\n' "$@" | sed '/^$/d' > "$sel/packages"
  [ -f "$sel/reused" ] || : > "$sel/reused"
  [ -s "$sel/reuse.json" ] || printf '%s\n' '{"applied":false,"reason":"no reuse was considered"}' > "$sel/reuse.json"
  jq -n -c --arg base "$SELECTION_BASE" --arg target "$TESTED_MERGE" --arg reason "$reason" --rawfile packages "$sel/packages" \
    --rawfile reused "$sel/reused" --slurpfile reuse "$sel/reuse.json" \
    '{Schema: "corvint-mq-selection/0", Base: $base, Head: $target, Target: $target, Reason: $reason,
      Packages: (if $reason == "" then [$packages | split("\n")[] | select(length > 0)] else ["./..."] end),
      Reused: (if $reason == "" then [$reused | split("\n")[] | select(length > 0)] else [] end),
      Reuse: (if $reason == "" then $reuse[0] else {applied: false, reason: "the selection failed"} end)}' \
    > "$sel/selection.json" 2> /dev/null || : > "$sel/selection.json"
}
# tree: the tested checkout's tree when it is clean and at TESTED_MERGE; empty otherwise.
tree() {
  [ "$(git rev-parse HEAD 2> /dev/null)" = "$TESTED_MERGE" ] || return 0
  st=$(git status --porcelain=v1 --untracked-files=all 2> /dev/null) || return 0
  [ -z "$st" ] || return 0
  git rev-parse --verify -q "HEAD^{tree}" 2> /dev/null || true
}
# plan_ BASE PREFIX REUSE: the pinned planner's receipt for BASE..TESTED_MERGE and the pinned
# selector's verdict, in $sel/PREFIX{plan.json,selector.txt}, and its packages, sorted, in
# $sel/PREFIXselected. When REUSE is 1 (an AFP-V0-813 plan) the receipt's .plan.scope must be
# BOUNDED whatever the selector prints, and a NOTHING verdict is an empty selection. Anything
# else returns 1 with the reason in why.
plan_() {
  p_base=$1
  p_out="$sel/$2"
  if ! "$PR_BIN/corvint" --root "$PWD" affected --base "$p_base" > "${p_out}plan.json" 2> "${p_out}plan.err" < /dev/null; then
    why="planner failed: $(head -c 200 "${p_out}plan.err")"
    return 1
  fi
  if ! jq -e --arg base "$p_base" --arg target "$TESTED_MERGE" \
    '(.profile == "affected-plan/1" or .profile == "affected-plan/0") and .tool == "affected" and .ok == true
     and .mutates == false and .revision == $target and .range.base == $base' "${p_out}plan.json" > /dev/null 2>&1; then
    why="malformed or mismatched planner receipt"
    return 1
  fi
  # AFP-V0-816: an UNKNOWN (or unnamed) plan scope refuses reuse even under a `run` verdict,
  # unless (AFP-V0-935, V1-3160) every unknown is a LANGUAGE_FRONTIER entry: the pinned selector
  # FALLBACKs on a module-level one and attributes the rest itself, as it does for S.
  if [ "$3" = 1 ] && ! jq -e '.plan.scope == "BOUNDED" or (.plan.scope == "UNKNOWN" and (.plan.unknown | type) == "array"
      and (.plan.unknown | length) > 0 and all(.plan.unknown[]; type == "object" and .reason == "LANGUAGE_FRONTIER"))' \
    "${p_out}plan.json" > /dev/null 2>&1; then
    why="plan scope is $(jq -r '.plan.scope // "missing" | tostring' "${p_out}plan.json" 2> /dev/null | head -c 40), not BOUNDED or a language frontier alone"
    return 1
  fi
  module=$(sed -n 's/^module[[:space:]]\{1,\}\([^[:space:]]\{1,\}\)[[:space:]]*$/\1/p' go.mod 2> /dev/null | head -n 1)
  if ! printf '%s' "$module" | grep -Eqx '[A-Za-z0-9][A-Za-z0-9._~/-]*'; then
    why="invalid root module"
    return 1
  fi
  if ! "$PR_BIN/selector" "${p_out}plan.json" "$module" "$PWD" > "${p_out}selector.txt" 2> "${p_out}selector.err" < /dev/null; then
    why="selector failed: $(head -c 200 "${p_out}selector.err")"
    return 1
  fi
  verdict=$(tail -n 1 "${p_out}selector.txt")
  case $verdict in
  "run "*) ;;
  NOTHING*)
    if [ "$3" = 1 ]; then
      : > "${p_out}selected"
      return 0
    fi
    why="selector: $(printf '%s' "$verdict" | head -c 200)"
    return 1
    ;;
  *)
    # FALLBACK is the selector's answer to an UNKNOWN plan; NOTHING selects no package.
    why="selector: $(printf '%s' "$verdict" | head -c 200)"
    return 1
    ;;
  esac
  set -f
  # shellcheck disable=SC2086 # split on spaces only; set -f stops pathname expansion
  set -- ${verdict#run }
  set +f
  if [ "$#" -eq 0 ]; then
    why="selector: run names no package"
    return 1
  fi
  for p in "$@"; do
    case $p in
    "$module" | "$module"/*) ;;
    *)
      why="invalid executable package argument"
      return 1
      ;;
    esac
    if ! printf '%s' "$p" | grep -Eqx '[A-Za-z0-9._~/-]+' || printf '%s' "$p" | grep -q '\.\.'; then
      why="invalid executable package argument"
      return 1
    fi
  done
  printf '%s\n' "$@" | LC_ALL=C sort -u > "${p_out}selected"
}
# refuse REASON: no reuse; the shard selection stays the whole queue-range selection (AFP-V0-816).
refuse() {
  cp "$sel/selected" "$sel/run"
  : > "$sel/reused"
  jq -n -c --arg reason "$1" '{applied: false, reason: $reason}' > "$sel/reuse.json" 2> /dev/null ||
    printf '%s\n' '{"applied":false,"reason":"the reuse refusal could not be recorded"}' > "$sel/reuse.json"
}
# reuse_: AFP-V0-813 to AFP-V0-815. S is the queue-range selection ($sel/selected). The record
# vouches that the pull-request run passed its narrowed selection of R..P, where R is the record
# base and P the record tree, which must be the merge of R and the entry's head. D is the
# selection of P..TESTED_MERGE (from a synthetic commit C of tree P), which holds every package
# whose tested inputs differ from the run's; Rsel is the selection of R..TESTED_MERGE. A package of
# S in Rsel and not in D was selected by the record's range with the same inputs, so it is
# reused; every other package of S runs.
reuse_() {
  if [ -z "${MQ_DELTA:-}" ]; then
    refuse "no narrowed pull-request record of this head was offered"
    return
  fi
  fields=$(printf '%s' "$MQ_DELTA" | jq -r 'select(type == "object" and (.runId | type) == "number" and .runId > 0
      and (.recordJobId | type) == "number" and .recordJobId > 0
      and all(.tree, .target, .base, .head; type == "string" and test("^[0-9a-f]{40}$")))
    | "\(.runId) \(.recordJobId) \(.tree) \(.target) \(.base) \(.head)"' 2> /dev/null) || fields=""
  if ! printf '%s' "$fields" | grep -Eqx '[0-9]+ [0-9]+( [0-9a-f]{40}){4}'; then
    refuse "the offered record is malformed"
    return
  fi
  set -f
  # shellcheck disable=SC2086 # six validated words
  set -- $fields
  set +f
  r_run=$1 r_job=$2 r_tree=$3 r_target=$4 r_base=$5 r_head=$6
  # The entry is the record head merged directly onto the queue base, so only the base moved.
  parents=$(git rev-list --parents -n 1 "$TESTED_MERGE" 2> /dev/null) || parents=""
  if [ "$parents" != "$TESTED_MERGE $QUEUE_BASE $r_head" ]; then
    refuse "the entry is not the record head merged onto the queue base"
    return
  fi
  if ! git merge-base --is-ancestor "$r_base" "$QUEUE_BASE" 2> /dev/null; then
    refuse "the record base is not an ancestor of the queue base"
    return
  fi
  tested=$(git merge-tree --write-tree --no-messages "$r_base" "$r_head" 2> /dev/null) || tested=""
  if [ "$tested" != "$r_tree" ]; then
    refuse "the record tree is not the merge of the record base and head"
    return
  fi
  # A fixed identity and date make C a pure function of the record; it is an object, not a ref.
  c=$(GIT_AUTHOR_NAME=corvint GIT_AUTHOR_EMAIL=corvint@invalid GIT_AUTHOR_DATE='@0 +0000' \
    GIT_COMMITTER_NAME=corvint GIT_COMMITTER_EMAIL=corvint@invalid GIT_COMMITTER_DATE='@0 +0000' \
    git commit-tree "$r_tree" -p "$r_base" -p "$r_head" -m "AFP-V0-813 record tree" 2> /dev/null) || c=""
  if ! hex40 "$c"; then
    refuse "the record tree could not be rebuilt as a commit"
    return
  fi
  # The exit status, not --name-only text, decides: git quotes a non-ASCII path, so a name match
  # would miss ".github/\303\251.yml". 0 is no change, 1 a change, anything else unreadable.
  for b in "$r_base" "$c"; do
    rc=0
    git diff --quiet --no-renames "$b" "$TESTED_MERGE" -- .github/ > /dev/null 2>&1 || rc=$?
    case $rc in
    0) ;;
    1)
      refuse "the record base or tree differs from the entry under .github/"
      return
      ;;
    *)
      refuse "the record range is unreadable, so a .github/ change cannot be excluded"
      return
      ;;
    esac
  done
  if ! plan_ "$c" delta- 1; then
    refuse "main delta: $why"
    return
  fi
  if ! plan_ "$r_base" record- 1; then
    refuse "record range: $why"
    return
  fi
  LC_ALL=C sort -u "$sel/selected" > "$sel/s"
  LC_ALL=C comm -12 "$sel/s" "$sel/record-selected" | LC_ALL=C comm -23 - "$sel/delta-selected" > "$sel/reused"
  LC_ALL=C comm -23 "$sel/s" "$sel/reused" > "$sel/run"
  count() { sed '/^$/d' "$1" | wc -l | tr -d ' '; }
  jq -n -c --argjson run "$r_run" --argjson job "$r_job" --arg tree "$r_tree" --arg target "$r_target" \
    --arg base "$r_base" --arg head "$r_head" --arg queueBase "$QUEUE_BASE" --arg deltaBase "$c" \
    --argjson selected "$(count "$sel/s")" --argjson delta "$(count "$sel/delta-selected")" \
    --argjson record "$(count "$sel/record-selected")" --argjson intersection "$(count "$sel/run")" \
    --argjson reused "$(count "$sel/reused")" \
    '{applied: true, reason: "", recordRunId: $run, recordJobId: $job, recordTree: $tree, recordTarget: $target,
      recordBase: $base, recordHead: $head, queueBase: $queueBase, deltaBase: $deltaBase,
      selected: $selected, deltaSelected: $delta, recordSelected: $record, intersection: $intersection, reused: $reused}' \
    > "$sel/reuse.json" 2> /dev/null || refuse "the reuse could not be recorded"
}
decide() {
  if [ "$QUEUE_BASE_REF" != refs/heads/main ]; then
    select_ "the queue base ref is $QUEUE_BASE_REF, not refs/heads/main"
    return
  fi
  if ! hex40 "$QUEUE_BASE" || ! hex40 "$TESTED_MERGE" || [ "$QUEUE_HEAD" != "$TESTED_MERGE" ]; then
    select_ "the queue base or head is not a full commit id, or the head is not the tested commit"
    return
  fi
  before=$(tree)
  if [ -z "$before" ]; then
    select_ "the tested checkout is not a clean checkout of the queue head"
    return
  fi
  # The entry is built on the queue base. A stacked entry's queue base is the entry ahead of it,
  # which main does not contain yet (AFP-V0-934, V1-3159): the selection then covers the whole
  # group from the newest commit of main the queue base descends from, a superset of the entry's
  # own range, so narrow.sh's .github/ and shadow guards cover the earlier entries too.
  if ! git merge-base --is-ancestor "$QUEUE_BASE" "$TESTED_MERGE" 2> /dev/null; then
    select_ "the queue base is not an ancestor of the queue head"
    return
  fi
  if ! git merge-base --is-ancestor "$QUEUE_BASE" refs/remotes/origin/main 2> /dev/null; then
    SELECTION_BASE=$(git merge-base "$QUEUE_BASE" refs/remotes/origin/main 2> /dev/null) || SELECTION_BASE=""
    if ! hex40 "$SELECTION_BASE" || ! git merge-base --is-ancestor "$SELECTION_BASE" refs/remotes/origin/main 2> /dev/null; then
      select_ "the queue base shares no commit with main"
      return
    fi
  fi
  if ! plan_ "$SELECTION_BASE" "" 0; then
    select_ "$why"
    return
  fi
  reuse_
  if [ "$(tree)" != "$before" ]; then
    select_ "source drift after planning"
    return
  fi
  set -f
  # shellcheck disable=SC2046 # one package per line, no spaces; set -f stops pathname expansion
  set -- $(cat "$sel/run")
  set +f
  select_ "" "$@"
}
decide
pr=${QUEUE_HEAD_REF#refs/heads/gh-readonly-queue/main/pr-}
pr=${pr%%-*}
printf '%s' "$pr" | grep -Eqx '[0-9]+' || pr=""
SELECTION="$sel/selection.json" BASE_REF=main PR_BASE=$SELECTION_BASE PR_HEAD=$TESTED_MERGE PR_NUMBER=$pr \
  exec sh "$here/narrow.sh"
