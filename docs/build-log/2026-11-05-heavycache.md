<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4725: stable Go test scratch through heavy admission

Owner intent is the heavycache lane charter, based at
`cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`. The local Tasks journal and the staged
2026-11-04 snapshot both refuse to show V1-4725; its native state is NOT_OBSERVED.
No Tasks write or publication is authorized for this lane.

The charter amends ASD-V0-734: Go test binaries need a stable TMPDIR for result-cache
reuse. Other commands retain fresh private scratch and cleanup. GOTMPDIR remains
unchanged, including when unset; remote build scratch retains its existing run directory.

## Decision and cleanup boundary

Use a current-user-owned mode-0700 `heavy-go-test-UID` namespace beneath the caller's
temporary root, or the configured remote temporary root. Direct, env-assignment and
leading-assignment Go test argv forms share the same namespace. An argv TMPDIR assignment
selects its own root and is rewritten to that root's stable namespace. Existing env
option prefixes (`--`, `-u`, `-i` and their unset spellings) are parsed before assignments;
the stable assignment is inserted after them so env cannot erase the exception. No test flags or cache
policy change, including `-count=1` semantics.

Neither existing cleanup selector covers arbitrary leaked t.TempDir trees:
`script/go-cache-trim.sh` selects immediate `go-build*` directories, and
`script/lane-tmp-sweep.sh` selects `corvint-*-test.XXXXXX` and `heavy-run.XXXXXX`.
Therefore the exception adds bounded namespace cleanup in `slot_perl`'s
`clear_test_tmp`: each test holds a shared lock, and only an exclusive holder clears
contents. Startup recovers wrapper-KILL leftovers before joining readers; completion
cleans after the last overlapping test. Read-only directories are made accessible
without following symlinks. One stable directory and its lock remain per temporary
root/user. A killed wrapper's leftovers wait for the next nonconcurrent admission;
there is no elapsed-time cleanup claim when no further test runs occur. This avoids
adding a general deletion rule for arbitrary `Test*` directories in shared scratch.

## Evidence and retained failures

Pre-change context/query/impact were recorded at BASE by `make dogfood-prechange`.
The installed binary's affected plan retains `self-binary-stale` and LANGUAGE_FRONTIER
uncertainty; broad declared-reader selections do not justify an exhaustive test campaign.
There are no changed Go packages. The charter explicitly adds the archivepath live proof.
Learning/evaluation, provider qualification and release gates are outside this shell change.
Complete task cost, billed tokens and savings remain NOT_OBSERVED.

Focused admission fixtures pass all five existing groups, including repeated stable
TMPDIR for direct/env/leading forms, count bypass, unchanged GOTMPDIR, recovery,
overlapping lock holders, and the existing non-Go private-directory/cap/timeout checks.
The first successful consecutive whole-package archivepath proof returned `0.014s` then `(cached)`.
Their raw log SHA-256 digests are respectively
`c721f5d37ab3e3e1f0c73bfdd35c354f6f6197555d229da32732ebad2bf61e98` and
`35131d64a78115bf6ab9caa6671972be3785a09913c56e74c0abf0844455e5e8`.

An earlier candidate set GOTMPDIR to fresh build scratch. Go 1.27 t.TempDir reads
GOTMPDIR, so that candidate still missed the cache; retained test-cache diagnostics
show the changing input. Removing that assignment restores the unchanged environment.
The remote mode-000 leak fixture also exposed File::Path's inability to enter such a
directory on Unix; a standalone reproducer now leaves only the namespace and lock.
An early focused run was invalidated by editing its shell source while it ran; its
failures and parser error are retained, followed by the clean passing focused rerun.
The identity fixture's empty-root assertion exposed the new persistent namespace:
its teardown now removes only the known lock and uses rmdir to prove the namespace
is otherwise empty, retaining the original scratch-index assertion verbatim.
Further repros showed that argv TMPDIR assignments bypass Perl's inherited environment,
and that env option prefixes prevented Go-test recognition. Both cases now select the
stable child, with regression coverage. The 63-group shell suite passed before that
last option-prefix repair; the final complete run is recorded below.

Raw receipts and failure logs stay outside the repository in
`/home/russell/lanes/heavycache-evidence/`. Independent review belongs to the
orchestrator. Real remote cache reuse is NOT_OBSERVED: remote GOTMPDIR remains
per-run by charter, and Go 1.27 t.TempDir also reads that variable. The remote fixture
qualifies stable TMPDIR and cleanup, without claiming remote result-cache hits. Doc-gates and selection-baseline are NOT_RUN by explicit instruction.
Rollback restores the original per-run Go test TMPDIR assignment and removes the
namespace exception and its fixtures; no wire format, golden or core-freeze output changes.

Final-source live proof (cache already warm):

- `proof-accepted-1.log`: `(cached)`; SHA-256 `9c833a70b7de0517d115bd511c4acddad4083647c522ae9ca86a17dcec19076d`.
- `proof-accepted-2.log`: `(cached)`; SHA-256 `591c14a10dd936593dac9893c11e4d5e9141bcced402a135bcdf9024d0ea7f7e`.

A refreshed affected receipt from the source-built binary retains `scope=UNKNOWN dirty=5 selected=86 excluded=567 unknown=1 LANGUAGE_FRONTIER=1`; no degradation was reported.

Final validation: `script/heavy.sh bash script/lane-scripts_test.sh` passed all 63 groups;
the final focused admission run passed all five groups, and the whole selected
`internal/lrfrepo` package passed with `-count=1 -timeout 30m` (25.947 s).
`bash -n script/heavy.sh`, spec regeneration and line-citations-check passed.
- `lane-scripts-accepted.log` SHA-256: `ad253bd2b0c1c1fee0d839403b18978982bbbfc34448f01bdd1a51f559b41002`.
- `lrfrepo-accepted.log` SHA-256: `0eaa30a539fd65a03aa3d0019a8994a780ea2fcc8fb747651edb19ba2e58f97f`.
- `admission-delivered.log` SHA-256: `ff0968346e390174d0070adc92bf13c7dcce7de17742f823d464f6e6836285d7`.
