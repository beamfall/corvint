# Citation support fixtures (CEM-V1-080..082)

Each case directory holds `tree.json` (base and target file trees), `map.json` (the stable map),
`patch.diff` (the canonical patch), `revisions.txt` (expected base and target commit IDs) and the
expected `citation` block. Rebuild a repository by committing `base` then `target` with author and
committer `Fixture <fixture@example.invalid>`, date `2026-10-11T00:00:00Z`, `GIT_CONFIG_GLOBAL=/dev/null`
and messages `base` / `target`; the commit IDs must equal `revisions.txt`. `map.json` must ACCEPT
under `verify-stable` (empty artifacts directory); the block is then computed from `map.json`,
`patch.diff` and the base span bytes. An ACCEPT stable result carries the same block as its
`citation` member (CEM-V1-092); a REJECT or refusal carries none.
`citation_test.go` regenerates every file with `CEM01_CITATION_UPDATE=1`.

- `rubber-stamp`: 24-hunk PATCH-24 citing one umbrella span; expected block `rubber-stamp/citation.json`.
- `careful`: PATCH-24 with per-hunk spans (8 ID, 8 path, 8 plain); expected block `careful/citation.json`.
- `split-20`: 20 hunks citing 20 disjoint umbrella spans (specificity 0); expected block `split-20/citation.json`.
- `lexical`: ID boundary, annotation, unknown and mechanical hunks; expected block `lexical/citation.json`.
