# V1-4679 self-review repair

The three review findings expose incomplete read sets used as negative proofs.
This repairs AZU-V1-960 / AZU-V1-961 and AFP-V0-511 within the existing contract.
It changes no wire shapes, diagnostic strings, codes, committed goldens or assertions.

## Decision and proof boundaries

1. Source alias resolution certifies a source edge only. Preserve every substituted
   candidate even when a source fallback resolves, without an extension allowlist that
   would discard unknown data formats. The existing validated alias-table grammar still
   governs substitution. `TestMixedAliasFallbackReaders_AZU_V1_960` selects the importer
   for both JSON and an unknown file type under the Playwright alias gate.
2. Package-import target strings are URLs. Certify only printable-ASCII relative `./` file URLs whose
   decoded components contain ASCII letters, digits, spaces, dots, underscores or hyphens,
   excluding empty, dot and parent segments. Decode components individually; query and
   fragment do not name file bytes. Every other form retains bounded-reader uncertainty.
   Existing conditional-map traversal keeps all alternatives, with its depth/count caps.
   `TestPackageImportURLReaders_AZU_V1_960` covers queries, fragments, percent encoding,
   encoded separators, malformed escapes, NUL, raw whitespace, unsupported punctuation and a future
   scheme. The latter forms cannot become an absence proof.
3. A frontier read-set proof requires a nonempty site list, an owned file for every site,
   and complete path tokens. Bare entries, overflow entries without records and unknown
   site forms retain uncertainty. `TestUnownedSourceBoundReads_AZU_V1_960` covers these
   negatives and a complete unrelated-overflow positive proof.

The first affected-package run exposed the shared `bystander` fixture's missing alias
site records. Its source-bound positive-proof premise now includes both overflow sites.
All existing assertions remain intact; new negative cases independently preserve missing
records. This is a fixture-evidence repair, not permission to drop an unlocated frontier.
Rollback is a revert of this repair; aggregate historical qualification remains open.

## Observed evidence and limits

All raw artifacts are outside the repository at
`/home/russell/lanes/m8-frontier-evidence/`. Go runs use `script/heavy.sh`, the configured
semaphore and the lane TMPDIR, without exported Git identity or lock overrides.

- Toolchain: `go1.27.4`. A fresh local build of `cmd/corvint` passed after the installed and
  previous candidate binaries refused with `self-binary-stale`.
- Fresh `affected --base bd52683480212e30dda560fa3334a1ad7f9ed3ab --format json` selected both
  touched packages. Plan scope remains UNKNOWN; selection is not full-suite truth.
- Whole-package `go test -count=1 -timeout 30m ./internal/liveverify/affected`: first run
  failed on the incomplete shared fixture (12.687s), final run passed (10.546s).
- Whole-package `go test -count=1 -timeout 30m ./internal/liveverify/affected/typescript`:
  passed (6.811s); after the raw-whitespace guard, the final whole-package run passed
  (5.434s), including mixed-fallback and URL regressions.
- `make -s spec-requirements-check line-citations-check` passed on final spec inputs
  (`internal/specindex` 0.957s). Citation repin found no moved citations. Independent review, full-suite truth, changed-line
  mutation qualification and Darwin evidence remain NOT_PRODUCED in this lane.
- `make changed-checks` is NOT_RUN because it invokes the whole-graph unbounded-reader check;
  this repair permits focused package tests and the spec/citation targets only.
- CEM binding/checking remains unavailable: `docs/DOGFOOD.md` records removal under decision
  0594. The host handback records the final command attempts. No seal or publication is authorized.

| Artifact | SHA-256 |
|---|---|
| `self-review-affected.json` | `0ead2cb35d281669cc919db0c103f7c994b097a034063be500135bc659616742` |
| `self-review-affected-test.log` | `9ce3f2a5424bfe53825af56dac80308e048f9f8ea274f959e8cf84ce1d816b72` |
| `self-review-affected-test-final.log` | `775d4517f1f4276c4b69c4addb6223a94ad2d49e8c3e644d0017d3d8d837080b` |
| `self-review-typescript-test.log` | `4a2d77a7cbb8a5476de0e50c7438010f29710c42a2824476ad53c87e87011934` |
| `self-review-typescript-test-final.log` | `52dc1efa5a214627d8f364c2aa61d1fe977ad3a57780c11ea2f07a864094dea6` |
| `self-review-doc-checks-final.log` | `c3301461baaad3910e666ed69e10360f3c8f8d1c1dfc4d4b388c5d9e2dc0f4ef` |
