#!/usr/bin/env bash
# usage: script/heavy-groups.sh [--budget SECONDS] [--log FILE] PACKAGE...
#
# Prints PACKAGE... as groups, one group per line (packages separated by spaces), so one
# `script/heavy.sh go test ... GROUP` per line stays under the foreground tool limit (ASD-V0-832,
# V1-3344). A package's cost is the run= seconds of its most recent completed (rc 0 or 1)
# whole-package run in the heavy.sh selection log (ASD-V0-030): a `local` or `offload` line whose
# `go test` command, after any `env` and NAME=VALUE words, has no -run, -skip or -bench flag and
# names exactly that one package (./x and ./x/ are the same package).
#
# Groups are packed largest cost first into the first group whose total stays at most the budget
# (default 480 s). A package with no recorded cost runs alone and is named on stderr as
# `heavy-groups: unknown cost PKG`; a package whose cost alone exceeds the budget runs alone and is
# named as `heavy-groups: over budget PKG Ns; shard it (make test-shard)` (ASD-V0-026).
#
# --log defaults to HEAVY_SELECTION_LOG, else HEAVY_LOCK_DIR/selection.log (HEAVY_LOCK_DIR default
# /tmp/corvint-heavy-UID), as heavy.sh writes it. Output is deterministic for the same log and
# arguments. Exit 0 on success, 2 on a usage error or a budget that is not a positive integer.
#
# Example: script/heavy-groups.sh --budget 300 ./internal/tasks/cli ./internal/tasks/transaction
set -uo pipefail
export LC_ALL=C
usage() { sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; }
budget=480
log=${HEAVY_SELECTION_LOG:-${HEAVY_LOCK_DIR:-/tmp/corvint-heavy-$(id -u)}/selection.log}
while (($#)); do
  case $1 in
    -h | --help) usage; exit 0 ;;
    --budget | --log) [[ $# -ge 2 ]] || { printf 'heavy-groups: %s needs a value\n' "$1" >&2; exit 2; }
      case $1 in --budget) budget=$2 ;; --log) log=$2 ;; esac
      shift 2 ;;
    --) shift; break ;;
    -*) printf 'heavy-groups: unknown flag %s\n' "$1" >&2; usage >&2; exit 2 ;;
    *) break ;;
  esac
done
[[ $budget =~ ^[1-9][0-9]*$ ]] || { printf 'heavy-groups: --budget must be a positive integer of seconds, got %s\n' "$budget" >&2; exit 2; }
(($#)) || { printf 'heavy-groups: no packages given\n' >&2; usage >&2; exit 2; }
[[ -r $log ]] || log=/dev/null

printf '%s\n' "$@" | awk -v budget="$budget" '
function norm(p) { sub(/^\.\//, "", p); sub(/\/+$/, "", p); return p == "" ? "." : p }
# record(line): the latest completed whole-package run= seconds per package.
function record(line,   n, w, i, rc, run, cmd, pkg, npkg, valflag) {
  if (match(line, / rc=[0-9]+ /) == 0) return
  rc = substr(line, RSTART + 4, RLENGTH - 5)
  if (rc != "0" && rc != "1") return
  if (match(line, / run=[0-9.]+s /) == 0) return
  run = substr(line, RSTART + 5, RLENGTH - 7)
  if (match(line, / cmd=/) == 0) return
  cmd = substr(line, RSTART + 5)
  sub(/ script=[0-9a-f]+$/, "", cmd)
  n = split(cmd, w, " ")
  i = 1
  while (i <= n && (w[i] == "env" || w[i] ~ /^[A-Za-z_][A-Za-z0-9_]*=/)) i++
  if (w[i] != "go" || w[i + 1] != "test") return
  npkg = 0
  for (i += 2; i <= n; i++) {
    if (w[i] == "-args") break
    if (w[i] ~ /^--?(run|skip|bench)(=|$)/) return
    if (w[i] ~ /^-/) {
      # Flags whose value is the next word.
      if (w[i] ~ /^--?(timeout|count|p|parallel|tags|cpu|benchtime|coverprofile|covermode|coverpkg|o|exec|shuffle|fuzztime|ldflags|gcflags|mod)$/) i++
      continue
    }
    npkg++; pkg = w[i]
  }
  if (npkg == 1) cost[norm(pkg)] = run + 0
}
FILENAME == "-" { pkgs[++np] = $0; next }
($2 == "local" || $2 == "offload") { record($0) }
END {
  for (i = 1; i <= np; i++) {
    k = norm(pkgs[i])
    if (k in cost) c[i] = cost[k]
    else { c[i] = budget; alone[i] = 1; printf "heavy-groups: unknown cost %s\n", pkgs[i] > "/dev/stderr" }
    if (c[i] > budget) { alone[i] = 1; printf "heavy-groups: over budget %s %ss; shard it (make test-shard)\n", pkgs[i], c[i] > "/dev/stderr" }
    ord[i] = i
  }
  # Largest cost first; ties by package name, so the output is deterministic.
  for (i = 2; i <= np; i++) for (j = i; j > 1; j--) {
    a = ord[j - 1]; b = ord[j]
    if (c[a] > c[b] || (c[a] == c[b] && pkgs[a] <= pkgs[b])) break
    ord[j - 1] = b; ord[j] = a
  }
  ng = 0
  for (i = 1; i <= np; i++) {
    p = ord[i]; placed = 0
    if (!alone[p]) for (g = 1; g <= ng; g++) if (!solo[g] && total[g] + c[p] <= budget) {
      total[g] += c[p]; line[g] = line[g] " " pkgs[p]; placed = 1; break
    }
    if (!placed) { ng++; total[ng] = c[p]; line[ng] = pkgs[p]; solo[ng] = alone[p] }
  }
  for (g = 1; g <= ng; g++) print line[g]
}' - "$log"
