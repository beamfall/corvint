#!/bin/sh
set -eu

# Five cases, on a fixture repository with one receipt pinning one subject: a current tree
# passes --check; a changed subject fails --check with the receipt named and nothing written;
# a repin rewrites the subject digest, repositoryRevision and ledger digest and nothing else;
# a missing subject and a hand-edited receipt are each refused with nothing written.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-repin-receipts-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
sha() { perl -MDigest::SHA=sha256_hex -0777 -ne 'print sha256_hex($_)' "$test_root/$1"; }
run() { (cd "$test_root" && script/repin-use-case-receipts.sh "$@" 2>&1); }

receipts=conformance/use-cases-v0/receipts/UC-X
mkdir -p "$test_root/script" "$test_root/$receipts" "$test_root/docs"
cp "$source_root/script/repin-use-case-receipts.sh" "$test_root/script/"
printf 'subject v1\n' > "$test_root/docs/subject.md"
cat > "$test_root/$receipts/contract.json" <<EOF
{
  "repositoryRevision": "0000000000000000000000000000000000000000",
  "subjects": [
    {
      "path": "docs/subject.md",
      "sha256": "$(sha docs/subject.md)"
    }
  ]
}
EOF
cat > "$test_root/conformance/use-cases-v0/ledger.json" <<EOF
{
  "evidenceClasses": ["contract", "implementation"],
  "useCases": [
    {"evidence": {"contract": {"path": "$receipts/contract.json", "sha256": "$(sha "$receipts/contract.json")"}}, "id": "UC-X"}
  ]
}
EOF
git -C "$test_root" init -q
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture
git -C "$test_root" add -A
git -C "$test_root" commit -qm fixture
head=$(git -C "$test_root" rev-parse HEAD)
unchanged() { git -C "$test_root" diff --quiet -- conformance || fail "$1 wrote a file"; }

# 1. A current tree passes --check.
output=$(run --check) || fail "a current tree failed --check: $output"

# 2. A changed subject fails --check, names the receipt, and writes nothing.
printf 'subject v2\n' > "$test_root/docs/subject.md"
if output=$(run --check); then fail "a changed subject passed --check"; fi
case $output in *"stale $receipts/contract.json"*) ;; *) fail "--check did not name the receipt: $output" ;; esac
unchanged "--check"

# 3. A repin updates the subject digest, the revision and the ledger pin, and nothing else.
output=$(run) || fail "the repin failed: $output"
grep -q "\"sha256\": \"$(sha docs/subject.md)\"" "$test_root/$receipts/contract.json" || fail "subject digest not repinned"
grep -q "\"repositoryRevision\": \"$head\"" "$test_root/$receipts/contract.json" || fail "revision not repinned"
grep -q "\"sha256\": \"$(sha "$receipts/contract.json")\"" "$test_root/conformance/use-cases-v0/ledger.json" || fail "ledger not repinned"
test "$(git -C "$test_root" diff --numstat -- conformance | awk '{ s += $1 + $2 } END { print s }')" = 6 || fail "the repin changed other lines"
output=$(run --check) || fail "the repinned tree failed --check: $output"
git -C "$test_root" commit -qam repinned

# 4. A missing subject is refused and nothing is written.
printf 'subject v3\n' > "$test_root/docs/subject.md"
mv "$test_root/docs/subject.md" "$test_root/subject.moved"
if output=$(run); then fail "a missing subject was repinned"; fi
case $output in *"subject docs/subject.md is missing"*) ;; *) fail "missing subject not named: $output" ;; esac
unchanged "a missing-subject refusal"
mv "$test_root/subject.moved" "$test_root/docs/subject.md"

# 5. A receipt whose bytes no longer match the ledger pin is refused, not blessed.
printf ' ' >> "$test_root/$receipts/contract.json"
if output=$(run); then fail "a hand-edited receipt was repinned"; fi
case $output in *"do not match the ledger pin"*) ;; *) fail "hand edit not named: $output" ;; esac
git -C "$test_root" diff --quiet -- conformance/use-cases-v0/ledger.json || fail "the refusal wrote the ledger"

printf 'repin-use-case-receipts_test: 5 cases passed\n'
