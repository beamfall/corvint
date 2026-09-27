## 2026-09-27 V1-0165: retrieval-bench refuses a base_commit that is not a hex object id

`readSamples` checked only that `base_commit` was non-empty, and `resolveSnapshot` joins it into
`<corpus>/OWNER__NAME/<base_commit>.chunks.jsonl`. A samples row with `../` components in
`base_commit` could therefore name a chunk file outside the corpus directory.

Decision: `readSamples` refuses any `base_commit` that is not a full lowercase hex Git object id
(40 or 64 characters), with the row's line number. The check runs after decoding, so it covers plain
samples and ContextBench rows alike. `--snapshot` keys keep their `repo@commit` form; the committed
fixtures already use full ids. `TestReadSamplesSkipsUnlabeledNoGold` used the placeholder `c` and now
uses a full id.

Evidence: `TestReadSamplesRefusesBaseCommitThatIsNotAnObjectID` refuses a traversal, an abbreviated
id and an uppercase id in plain samples, and a traversal in a ContextBench row, each on line 2.
`go test ./tools/retrieval-bench` passes.

Rollback: revert the change; `base_commit` is again checked only for presence.
