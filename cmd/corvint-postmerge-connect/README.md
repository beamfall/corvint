# Experimental post-merge connector companion

`go run ./cmd/corvint-postmerge-connect` is separate from Core. It offers two
local reference connector kinds: a tracker hierarchy/item store and a forge
merged-change/draft store. They require no credentials and make no network calls.
Remote service adapters are **NOT_QUALIFIED**; `live` here means local fixture state.

Run the executable conformance kit (real disposable Git commits and file states):

```sh
GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/postmergeconnector ./cmd/corvint-postmerge-connect
```

Wire types and fixed templates are in `internal/postmergeconnector/types.go` and
`plan.go`; the owning proposed contract is `docs/specs/postmerge-connectors-v0.md`.
`main_test.go` constructs a complete fixture, policy and typed input, processes a
merge twice, and proves that recording/dry-run bytes agree and live upserts leave
one follow-up, one restricted finding and two draft requests. `conformance_test.go`
is the reference adapter acceptance battery, including negative cases.

All input file paths must be clean absolute paths without symlink ancestry. On
macOS, resolve `/tmp` to `/private/tmp` before choosing private fixture directories.
The product checkout is the `--root`; the **actual** isolated author checkout is
`--author-root`. Supply these from trusted CI configuration, never author outputs.

```sh
go run ./cmd/corvint-postmerge-connect plan --experimental --root "$product" --fixture "$raw" --policy "$policy" --input "$typed"
go run ./cmd/corvint-postmerge-connect read-intake --experimental --root "$product" --fixture "$raw" --policy "$policy" --input "$typed" --output "$intake" --author-root "$author"
go run ./cmd/corvint-postmerge-connect write --experimental --root "$product" --fixture "$raw" --policy "$policy" --input "$typed" --mode dry-run
go run ./cmd/corvint-postmerge-connect write --experimental --root "$product" --fixture "$raw" --policy "$policy" --input "$typed" --mode recording --output "$requests" --author-root "$author"
go run ./cmd/corvint-postmerge-connect write --experimental --root "$product" --fixture "$raw" --policy "$policy" --input "$typed" --mode live --output "$state" --author-root "$author"
```

The fixture carries raw titles/bodies; `read-intake` saves them only in its private
intake file and prints nothing. The typed input has no prose or template slots.
Policy independently pins forge, repository, change, base and merge identities,
the hierarchy stop level, classes/cap and allowed HTTPS origins. Identity pins are
caller-owned observations, not forge authentication. Findings classification is
trusted `ordinary` or `security`; `unknown` blocks, and the security class always
uses restricted triage. Paths/lines come from the exact immutable merge blob.

The host separates raw readers, authoring agents and deterministic writers. Only
the writing step may hold outward-write credentials for a future adapter. This
companion reads no credential variable and registers no agent write tool. A host
must not give a writer credential to a reader/author process; the local kit does
not attest a remote host's environment or qualify hostile directory races.

Recording atomically replaces a bounded canonical JSONL ledger in a trusted private
single-writer directory. An exact request repeat adds no new event; a changed body
adds an update with the same external key. Draft branches include the stable merge
key and content kind. Local state publishes the whole validated batch atomically;
injected remote writers must reconcile partial effects through stable-key upserts.
No remote transaction, concurrent-writer, signing or power-loss guarantee is claimed.
