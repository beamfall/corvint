#!/bin/sh
set -eu

# Ten cases, on a fixture repository with one receipt pinning one subject: a current tree
# passes --check; a changed unpinned path passes --check and is listed as unpinned (UCV0-018);
# a changed subject fails --check with the receipt named and nothing written, and only the
# unpinned path is listed; --changed replaces the worktree diff;
# a repin rewrites the subject digest, repositoryRevision and ledger digest and nothing else;
# a missing subject and a hand-edited receipt are each refused with nothing written; a receipt
# that left its ledger pin names the commit holding the pinned bytes and the exact restore and
# repin command, shell-quoted, or asks for review when no recent commit holds them (UCV0-019);
# a span subject ignores edits elsewhere and is refused when its lines change, and a symbol
# subject is repinned by the runner's digest (UCV0-630).

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-repin-receipts-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
sha() { perl -MDigest::SHA=sha256_hex -0777 -ne 'print sha256_hex($_)' "$test_root/$1"; }
run() { (cd "$test_root" && script/repin-use-case-receipts.sh "$@" 2>&1); }

# The receipt directory holds a space, so every printed command must quote the path (UCV0-019).
receipts='conformance/use-cases-v0/receipts/UC X'
mkdir -p "$test_root/script" "$test_root/$receipts" "$test_root/docs"
cp "$source_root/script/repin-use-case-receipts.sh" "$test_root/script/"
printf 'subject v1\n' > "$test_root/docs/subject.md"
printf 'help v1\n' > "$test_root/docs/help.txt"
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

# 1. A current tree passes --check and lists nothing as unpinned.
output=$(run --check) || fail "a current tree failed --check: $output"
case $output in *"0 changed path(s)"*"all 1 receipts current"*) ;; *) fail "current tree output: $output" ;; esac

# 2. A changed path no receipt pins is listed but leaves --check passing (UCV0-018).
printf 'help v2\n' > "$test_root/docs/help.txt"
output=$(run --check) || fail "an unpinned change failed --check: $output"
case $output in *"1 not pinned by any receipt"*"unpinned docs/help.txt"*"all 1 receipts current"*) ;; *) fail "unpinned path not listed: $output" ;; esac

# 3. A changed subject fails --check, names the receipt, writes nothing, and the pinned subject
# is not listed as unpinned beside the unpinned path.
printf 'subject v2\n' > "$test_root/docs/subject.md"
if output=$(run --check); then fail "a changed subject passed --check"; fi
case $output in *"stale $receipts/contract.json"*) ;; *) fail "--check did not name the receipt: $output" ;; esac
case $output in *"2 changed path(s)"*"1 not pinned by any receipt"*"unpinned docs/help.txt"*) ;; *) fail "pinned/unpinned listing: $output" ;; esac
case $output in *"unpinned docs/subject.md"*) fail "a pinned subject was listed as unpinned: $output" ;; esac
unchanged "--check"

# 4. --changed replaces the worktree diff; the receipt and ledger count as pinned.
if output=$(run --check --changed ./docs/subject.md docs/other.md "$receipts/contract.json"); then fail "--changed altered the exit status"; fi
case $output in *"3 changed path(s) in --changed, 1 not pinned"*"unpinned docs/other.md"*) ;; *) fail "--changed listing: $output" ;; esac
case $output in *"unpinned docs/help.txt"*|*"unpinned docs/subject.md"*|*"unpinned $receipts"*) fail "--changed listed a path it was not given or a pinned one: $output" ;; esac

# 5. A repin updates the subject digest, the revision and the ledger pin, and nothing else.
output=$(run) || fail "the repin failed: $output"
grep -q "\"sha256\": \"$(sha docs/subject.md)\"" "$test_root/$receipts/contract.json" || fail "subject digest not repinned"
grep -q "\"repositoryRevision\": \"$head\"" "$test_root/$receipts/contract.json" || fail "revision not repinned"
grep -q "\"sha256\": \"$(sha "$receipts/contract.json")\"" "$test_root/conformance/use-cases-v0/ledger.json" || fail "ledger not repinned"
test "$(git -C "$test_root" diff --numstat -- conformance | awk '{ s += $1 + $2 } END { print s }')" = 6 || fail "the repin changed other lines"
output=$(run --check) || fail "the repinned tree failed --check: $output"
git -C "$test_root" commit -qam repinned

# 6. A missing subject is refused and nothing is written.
printf 'subject v3\n' > "$test_root/docs/subject.md"
mv "$test_root/docs/subject.md" "$test_root/subject.moved"
if output=$(run); then fail "a missing subject was repinned"; fi
case $output in *"subject docs/subject.md is missing"*) ;; *) fail "missing subject not named: $output" ;; esac
unchanged "a missing-subject refusal"
mv "$test_root/subject.moved" "$test_root/docs/subject.md"

# 7. A receipt whose bytes no longer match the ledger pin is refused, not blessed.
printf ' ' >> "$test_root/$receipts/contract.json"
if output=$(run); then fail "a hand-edited receipt was repinned"; fi
case $output in *"do not match the ledger pin"*) ;; *) fail "hand edit not named: $output" ;; esac
git -C "$test_root" diff --quiet -- conformance/use-cases-v0/ledger.json || fail "the refusal wrote the ledger"
# UCV0-019: the refusal names the commit holding the pinned bytes and the exact restore and repin.
pinned_head=$(git -C "$test_root" rev-parse HEAD)
fix="git restore --source=$pinned_head -- '$receipts/contract.json' && script/repin-use-case-receipts.sh"
case $output in *"fix: the pinned bytes are at $pinned_head; restore them and repin: $fix"*) ;; *) fail "no recovery named: $output" ;; esac
output=$(cd "$test_root" && eval "$fix" 2>&1) || fail "the named recovery failed: $output"
output=$(run --check) || fail "the recovered tree failed --check: $output"

# 8. A receipt carried by a cherry-pick without its ledger (UCV0-019): the committed receipt left
# the ledger pin, which an older commit still holds; the named command restores and repins it.
printf 'subject v4\n' > "$test_root/docs/subject.md"
run > /dev/null || fail "the setup repin failed"
git -C "$test_root" add -A
git -C "$test_root" commit -qm "carried receipt" -- "$receipts/contract.json" docs/subject.md
git -C "$test_root" checkout -q HEAD -- conformance/use-cases-v0/ledger.json
before=$(git -C "$test_root" rev-parse HEAD~1)
if output=$(run); then fail "a carried receipt was repinned"; fi
fix="git restore --source=$before -- '$receipts/contract.json' && script/repin-use-case-receipts.sh"
case $output in *"$fix"*) ;; *) fail "carried receipt recovery not named: $output" ;; esac
output=$(cd "$test_root" && eval "$fix" 2>&1) || fail "the carried recovery failed: $output"
output=$(run --check) || fail "the carried recovery left a stale tree: $output"
git -C "$test_root" commit -qam recovered

# 9. With no commit holding the pinned bytes, the refusal asks for review and names no restore.
perl -0pi -e 's/("sha256": ")[0-9a-f]{64}/${1}0000000000000000000000000000000000000000000000000000000000000000/' "$test_root/conformance/use-cases-v0/ledger.json"
if output=$(run); then fail "an unpinned receipt was repinned"; fi
case $output in *"no commit among the last 50"*"review the edit"*) ;; *) fail "no-history refusal: $output" ;; esac
case $output in *"git restore"*) fail "a restore was named with no pinned commit: $output" ;; esac

# 10. UCV0-630: a span subject stays current when other lines change and is refused, not
#     repinned, when its own lines change; a symbol subject is repinned by the digest
#     `go run ./conformance/use-cases-v0 -subject-digest` prints (a stub `go` here).
git -C "$test_root" checkout -q -- conformance
mkdir -p "$test_root/bin"
x=1111111111111111111111111111111111111111111111111111111111111111
y=2222222222222222222222222222222222222222222222222222222222222222
printf '%s\n' "$x" > "$test_root/bin/digest"
cat > "$test_root/bin/go" <<'STUB'
#!/bin/sh
[ "$*" = "run ./conformance/use-cases-v0 -subject-digest docs/demo.go#Cited" ] || exit 2
cat "$(dirname "$0")/digest"
STUB
chmod +x "$test_root/bin/go"
printf 'a\nb\nc\n' > "$test_root/docs/span.md"
printf 'package demo\n' > "$test_root/docs/demo.go"
span=$(printf 'b\n' | perl -MDigest::SHA=sha256_hex -0777 -ne 'print sha256_hex($_)')
cat > "$test_root/$receipts/contract.json" <<EOF
{
  "repositoryRevision": "0000000000000000000000000000000000000000",
  "subjects": [
    {"lines": "2-2", "path": "docs/span.md", "sha256": "$span"},
    {"path": "docs/demo.go", "sha256": "$x", "symbol": "Cited"}
  ]
}
EOF
perl -0pi -e 's/("sha256": ")[0-9a-f]{64}/${1}'"$(sha "$receipts/contract.json")"'/' "$test_root/conformance/use-cases-v0/ledger.json"
git -C "$test_root" add -A
git -C "$test_root" commit -qm pins
srun() { (cd "$test_root" && PATH="$test_root/bin:$PATH" script/repin-use-case-receipts.sh "$@" 2>&1); }
printf 'a\nb\nz\n' > "$test_root/docs/span.md"
output=$(srun --check) || fail "an edit outside a pinned span made the receipt stale: $output"
case $output in *"all 1 receipts current"*) ;; *) fail "span output: $output" ;; esac
printf '%s\n' "$y" > "$test_root/bin/digest"
if output=$(srun --check); then fail "a changed symbol digest passed --check: $output"; fi
case $output in *"stale $receipts/contract.json"*"subject docs/demo.go"*) ;; *) fail "symbol --check output: $output" ;; esac
output=$(srun) || fail "the symbol repin failed: $output"
grep -q "\"sha256\": \"$y\", \"symbol\": \"Cited\"" "$test_root/$receipts/contract.json" ||
  fail "the symbol subject was not repinned: $(cat "$test_root/$receipts/contract.json")"
output=$(srun --check) || fail "the symbol repin left a stale tree: $output"
git -C "$test_root" commit -qam symbol
printf 'a\nB\nz\n' > "$test_root/docs/span.md"
if output=$(srun); then fail "a changed span was repinned: $output"; fi
case $output in *"subject docs/span.md lines 2-2 changed"*"repin the span by hand"*) ;; *) fail "span refusal: $output" ;; esac
unchanged "a changed span"

printf 'repin-use-case-receipts_test: 10 cases passed\n'
