# V1-4679: independent review resolution repair

Date: 2026-11-18
Owner: Russell Lewis
Base: `55decc6a041a1df3f8ae43e95ba21cea669e7d80`
Pre-repair head: `0314af9d3b78132f2b9505cf377b28af93109cc8`
Status: PARTIAL qualification. AZU-V0-009 remains unqualified.

## Findings and decision

The baseUrl resolver's boolean reports source ambiguity, not candidate ownership. The
AZU-V1-960 repair enumerates all validated alias/baseUrl candidates independently of it.
Source resolution never certifies the absence of a foreign read.

The second finding is incomplete file/directory resolution (also V1-4683). The closed
proof admits valid repository-relative literal paths containing ASCII letters, digits,
spaces, slash, dot, underscore, hyphen, at-sign and hash, with arbitrary filename suffixes.
It preserves the exact candidate, its `.json` suffix alternative and its `index.json`
directory entry. It certifies absence of package redirection only when the directory's
package manifest is absent, or the candidate is a regular file. Existing manifests of
any shape, unavailable metadata and every unsupported path form retain bounded read
uncertainty. This covers package-main alternatives conservatively without implementing
another package loader. Relative literals outside the repository keep their existing
unresolved-import claim; they do not name a tracked foreign path.

`TestForeignModuleResolutionBoundary_AZU_V1_960` exercises 24 combinations: relative,
baseUrl and paths imports of extensionless JSON, directory index, JSON-suffix fallback,
dotted-directory index, package-main redirection, an unknown manifest form, an unfamiliar
filename suffix and unknown path syntax. It asserts both the read-set proof state and
reader selection or retained uncertainty. Existing tests and assertions are unchanged.

The third finding cannot be qualified within this round: the owner prohibits whole-repository
selection/tests and assigns qualification to the host. No new accepted AZU-V0-009 replay,
full-suite truth or changed-line mutation receipts were produced. The old seven-merge
comparison predates this repair and is not evidence for the final implementation. Keep
V1-4679 open until the host retains final-implementation independent truth and accepted
replay. Neither selection-set parity nor these regression results prove zero missed tests.

Rollback this repair separately from fixture narrowing; reverting it restores the two
reported missing-read risks. No wire field, diagnostic string, code, golden or core-freeze
output was changed. Directory-package uncertainty can widen selection and is intentional.

## Verification and retained evidence

Go `go1.27.4`; Linux; `TMPDIR=/home/russell/lanes/m8-frontier.tmp`; heavy wrapper uses the
configured semaphore without overrides. Identity variables were absent from tests.
The installed binary refused `affected --base` with `self-binary-stale`. A fresh binary
successfully selected the TypeScript, selector and specindex packages against the full base,
with UNKNOWN scope retained. Tests each name one package and run through `script/heavy.sh`.

- Final whole TypeScript package: PASS, 4.868 s, including existing tests and all 24 new cases.
- Whole selector integration package: PASS, 8.371 s.
- First TypeScript run: FAIL, 4.916 s. Overly narrow suffix handling introduced global token
  uncertainty for ordinary dotted filenames and package-import spellings, losing known readers
  and fixture closure. The final proof models arbitrary literal suffixes and keeps package-import
  spellings. Tests were not weakened. An intermediate whole-package repair passed in 4.897 s.
- Final binary build: PASS. It is retained for host qualification, with dirty build metadata at
  the pre-repair head. Its implementation bytes are pinned below; no historical replay is claimed.

Raw logs and binaries remain under `/home/russell/lanes/m8-frontier-evidence/`.

| Artifact | SHA-256 |
|---|---|
| `corvint-independent-repair-final` | `2c18bc86589f2eb7b05f4564537589c586e427d6e123123bf831f4a4ab314799` |
| `independent-typescript-test-final2.log` | `430f748362bcff73c3956c40bea18ce97b6ac6b46402b70c8f6dfc08ba9f22aa` |
| `independent-affected-test.log` | `1cf1725f2b45f712e1adea063bf6e4e7b75f55efd0fd5f1471b3c64fecfd7cf1` |
| `foreign_module_candidates.go` | `f39dc38b06c62f78910828479689ae694533c6173744d66cb5223bbabc780d3d` |
| `playwright_aliases.go` | `bc2005b24a2abe6caf682381c8ed3f8a151ec3294198ed2771360a276564ea80` |
| `typescript.go` | `bd0441c4d9719d4b9409c5253d64add1b13d20f241a68dc976c25dd9d8426253` |
| `frontier_foreign_reads_test.go` | `9935657c1d409e8bd1dccb197b492b0f38b437ed67e2d23e2e25bde5941e1a14` |

Source artifact basenames above are under `internal/liveverify/affected/typescript/`.
The final command record, citation plan and intent file remain in the lane scratch directory.
CEM bind/check are externally blocked by the removed targets documented in `docs/DOGFOOD.md`;
restoring that workflow is outside this repair. No dogfood-change, seal, publication, task-store
write or model-backed review is authorized in this round.

Final focused documentation checks: `script/check-line-citations.sh --repin` found no
moved citations; `make -s spec-requirements-check line-citations-check` passed (specindex
1.049 s through the heavy wrapper). The 184 skipped untracked-basename citations remain
reported. The final candidate's current-worktree affected plan against the full base
selected 365 units (198 tests, 167 drivers), with UNKNOWN scope and four unknowns: two
Go build-constraint frontiers and two unselected binary consumers. It is only a selection
receipt, not independent truth. `independent-affected-stable.json` has SHA-256
`771e6721772bdb17d8edb9a27db99a66f1979c177ccb78e9fa5e1cf1944828ff`.
An earlier final-plan attempt refused with `unsupported-affected-drift` while this evidence
entry was written/staged; the separate successful run used a stable worktree. Both are retained.
