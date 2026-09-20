# Decision 0322 — native taskman fixture planning and pre-edit preregistration

Date: 2026-09-19. Status: accepted fixture scope. Authority: the owner supplied the implementation
handoff, then replied exactly "approved" to the reviewed prerequisite amendment A–E retained in
`.agent-evidence/native-taskman/prerequisite-amendment-proposal.md`.

The accepted proposal SHA-256 is `52c722a56334054c65192ff7ab7971e304eab35bf94b84951af5696fdb9e04b3`.
It explicitly adopts native A1/A2/A5 from sibling task-store SPEC 4.3/8 for this extension, preserving
ATM/WQO shadow and foreign-wave behavior. Corvint produces a fixture read-only priority-first plan;
`corvint-tasks` owns all canonical mutation, reservation, atomic admission and completion work.
The implementation interpretation is `docs/specs/native-taskman-planning-v0.md`.

The Corvint pre-change base is `6a423ac091d848b8ac5b49e8002c61c252993ac3`, tree
`3165d313e3344646824cc585e4cac9c534e2cb19`. The sibling source is HEAD
`5117b9238f9ccc7caf01bc9a5e0ab1877ba76208` plus its uncommitted writer fixes: all 130 hashes in
`barrier-source-manifest.json` matched; the retained source check records its exact manifest digest.
None of the six missing historical governing copies was recovered. Current source is not silently
substituted for them, and no exact ATCP conformance or unknown upstream amendment is claimed.

Decision 0088's Python retirement remains. The approved exception permits native old-Go/new-Go
preregistration outside source and a content-addressed pre-edit freeze in place of an unavailable
CONFIG_PIN. The exact same baseline still needs a real CONFIG_PIN before promotion. The missing
historical Corvint corpus is explicitly repinned to the pre-change base; Beamfall retains original
commit `fa3b1e7fe5bc6c10e4b09b2729f364780f567a48`. Both private materializations match their pinned
Git trees; no live Beamfall checkout is measured or edited.

Before any Corvint Go edit, the native diagnostic runner was built and its cancellation/descendant
regression passed. The final preregistration is
`.agent-evidence/native-taskman/baseline-d432662db2d8b8227336e6661abfb96c64be1680baed8be5e72d6b3a95f5751c.json`,
whose SHA-256 is `d432662db2d8b8227336e6661abfb96c64be1680baed8be5e72d6b3a95f5751c`.
Its bound notes retain exact external runner source/binary identities, corpus/index/task identities,
42 CLI entrypoints and the required normal-workload gaps. Help/negative rows never cover those gaps.
The preserved rejected draft exposed a Count decoding mismatch, repaired before this final freeze.
The two-pair smoke uses only a diagnostic profile and reports NOT_RUN, not performance success.

GP remains NOT_RUN: an unrelated gate made the host CONTENDED; allocation/I/O witnesses, complete
workload/cache/environment qualification, all absolute-budget mappings, Linux and actual
IDLE_INITIALIZED/MAX_ADMITTED runtime conditions remain unavailable. Five warmups, at least 100
fresh-process samples, direct alternation, exact output/exit identity and zero-increase upper-bound
rules are unchanged. No latency budget is relaxed. No slowdown, production readiness, real queue
cutover, dispatch, publication or history deletion is authorized or claimed.

Independent authority review and implementation Gate A are retained alongside the preregistration.
Rollback removes the fixture extension and returns to the baseline native binary; WQO and the native
store require no state repair. Production promotion remains held on every outstanding GP/executor
and owner cutover record.
