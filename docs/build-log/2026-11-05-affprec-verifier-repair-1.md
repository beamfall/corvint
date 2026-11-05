# affprec verifier repair 1: deterministic fixture identities

Date: 2026-11-05
Owner direction: affprec verifier repair 1, limited to the supplied failing checks.

## Finding and decision

The verifier's default-context golden failures differ in commit identity, with
identical tree and evidence blobs. Its selected cmd/corvint unit also fails
frozen init/adopt inventory comparisons because their repository revision and
revision-derived receipt IDs changed. The fixtures pin dates and configure
user.name/email, but inherited GIT_AUTHOR_* and GIT_COMMITTER_* identity values
have higher Git precedence than those settings. Exporting the required lane
commit identity reproduces all these failures on the unchanged repair base
2824d412c20cbb14c6fc4a5fb12deb1b986d866d.

Pin the existing fixture author and committer names/emails in the child process
environments, alongside the already pinned dates. Preserve the production code,
all frozen bytes and every existing assertion. This restores TCP-V0-024 and
TCP-V0-150 context compatibility and CCF-V1-002 frozen inventory evidence without
a wire change under CCF-V1-006. No capability or requirement amendment is needed.
Rollback removes only these fixture environment additions; it would restore the
observed sensitivity to inherited identity. The six original lane tickets and
AFP-V0-9000 through AFP-V0-9005 retain their implementation and acceptance scope.

## Verification evidence and limits

Raw output lives in /home/russell/lanes/affprec-evidence/repair1/.
The existing pre-change capture is reused; no new pre-change producer ran.
The affected route selected cmd/corvint before running tests. Its installed
binary reports self-binary-stale, retained in the handback; its advice does not
replace the required whole-package run. Both fixtures are checked with Russell's
identity exported, reproducing the host verifier environment.

- repro.log: unchanged fixture reproduction fails both context golden tests and
  frozen init/adopt inventory cases; command exits 1.
- focused.log: script/heavy.sh go test -count=1 -timeout 30m -run
  'Freeze|GPKV0002|Golden|TestCoreVerbsEmitTheFrozenProfiles' ./cmd/corvint/
  passes, including the exact host failing selection and additional Core cases.
- whole-cmd-corvint.log: whole-package rerun and terminal results are recorded
  in the repair handback. No filtered-only acceptance is claimed.

Independent review remains reserved to orchestration. The prior production
selection-baseline evidence remains applicable to unchanged production source;
this repair changes fixture setup only. No new performance, Darwin, Windows,
external interoperability or repository-wide gate evidence is claimed. Native
Tasks writes, publication and sealing remain prohibited. CEM rebind/check and
reviewer report inspection are recorded in the handback after verification.
