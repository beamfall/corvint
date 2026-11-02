#!/usr/bin/env bash
# retained-data-digest.sh: digest a benchmark data directory kept outside the repository, and check
# the manifest that records where such directories live (ASD-V0-646, V1-3754).
#
#   script/retained-data-digest.sh DIR
#       prints `sha256=<hex> files=<n> bytes=<n>` for DIR: the SHA-256 of the sorted `sha256sum`
#       listing of every regular file under DIR (paths relative to DIR), so the digest does not
#       depend on the order the files were written in or on where DIR is mounted.
#   script/retained-data-digest.sh --check [FILE]    (default benchmarks/retained-data.tsv)
#       checks the manifest: the header, ten tab-separated columns, unique ids, a 64-hex sha256,
#       decimal files and bytes, a 40-hex producing_commit and a YYYY-MM-DD recorded date, where any
#       value but id, ticket, host, path and recorded may be NOT_OBSERVED with the reason in note.
#
# Exit 0 on success, 1 on a manifest finding, 2 on a usage error, an unreadable input, or a failed
# read, count or hash while digesting DIR.
set -uo pipefail
export LC_ALL=C
me='retained-data-digest'
usage() { printf 'usage: script/retained-data-digest.sh DIR | --check [FILE]\n' >&2; exit 2; }
header=$'id\tticket\thost\tpath\tsha256\tfiles\tbytes\tproducing_commit\trecorded\tnote'
check() {
  local file=$1
  [[ -r $file ]] || { printf '%s: cannot read %s\n' "$me" "$file" >&2; return 2; }
  awk -F '\t' -v me="$me" -v f="$file" -v header="$header" '
    function bad(msg) { printf "%s: FAIL %s:%d %s\n", me, f, NR, msg; n++ }
    function form(v, re) { return v == "NOT_OBSERVED" || v ~ re }
    NR == 1 { if ($0 != header) bad("header is not the ten ASD-V0-646 columns"); next }
    {
      rows++
      if (NF != 10) { bad("has " NF " columns, want 10"); next }
      if ($1 !~ /^[a-z0-9][a-z0-9._-]*$/) bad("id " $1 " is not lowercase letters, digits, dot, dash or underscore")
      else if ($1 in seen) bad("id " $1 " repeats line " seen[$1])
      seen[$1] = NR
      if ($2 !~ /^V1-[0-9]+$/) bad("ticket " $2 " is not V1-N")
      if ($3 == "" || $4 == "") bad("host and path must be set")
      if (!form($5, "^[0-9a-f]{64}$")) bad("sha256 " $5 " is not 64 hex digits or NOT_OBSERVED")
      if (!form($6, "^[0-9]+$")) bad("files " $6 " is not a count or NOT_OBSERVED")
      if (!form($7, "^[0-9]+$")) bad("bytes " $7 " is not a count or NOT_OBSERVED")
      if (!form($8, "^[0-9a-f]{40}$")) bad("producing_commit " $8 " is not a full commit SHA or NOT_OBSERVED")
      if ($9 !~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) bad("recorded " $9 " is not YYYY-MM-DD")
      if ($0 ~ /NOT_OBSERVED/ && $10 == "") bad("a NOT_OBSERVED value needs its reason in note")
    }
    END {
      if (NR == 0) bad("is empty")
      if (n) exit 1
      printf "%s: PASS %s: %d retained-data rows\n", me, f, rows
    }' "$file"
}
case ${1-} in
  --check) (($# <= 2)) || usage; check "${2:-benchmarks/retained-data.tsv}"; exit $? ;;
  '' | -*) usage ;;
esac
(($# == 1)) || usage
dir=$1
[[ -d $dir && -r $dir ]] || { printf '%s: %s is not a readable directory\n' "$me" "$dir" >&2; exit 2; }
# Every pipeline fails closed (pipefail): a failed find, read or hash exits 2 instead of printing
# bytes=0 or an empty sha256.
die() { printf '%s: %s\n' "$me" "$*" >&2; exit 2; }
listing=$(cd "$dir" && find . -type f -print0 | sort -z | xargs -0 -r sha256sum) || die "cannot hash the files under $dir"
files=0 bytes=0
if [[ -n $listing ]]; then
  files=$(printf '%s\n' "$listing" | wc -l | tr -d ' ') || die "cannot count the files under $dir"
  bytes=$(cd "$dir" && find . -type f -print0 | xargs -0 -r cat | wc -c | tr -d ' ') || die "cannot read the files under $dir to count their bytes"
fi
[[ $files =~ ^[0-9]+$ && $bytes =~ ^[0-9]+$ ]] || die "the file or byte count for $dir is not a number (files=$files bytes=$bytes)"
digest=$(printf '%s' "$listing" | sha256sum | cut -d' ' -f1) || die "cannot hash the listing of $dir"
[[ $digest =~ ^[0-9a-f]{64}$ ]] || die "the listing digest of $dir is not 64 hex digits ('$digest')"
printf 'sha256=%s files=%s bytes=%s\n' "$digest" "$files" "$bytes"
