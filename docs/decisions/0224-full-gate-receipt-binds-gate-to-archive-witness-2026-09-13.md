# Decision 0224 — a private full-gate receipt binds a complete `make gate` to HEAD and the archive witness

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The Go-only cutover lists three open release items: an exact-target full gate, an artifact witness,
and final CEM/OCM closure. `script/go-archive-gate` already records a private archive witness bound
to the exact commit, tree and archive digests. Nothing mechanical recorded that the complete
`make gate` passed at that same commit, so `script/release-checklist` could show a current
go-archive `PASS` from a lone `make go-archive-gate` run.

Three options were weighed. A prose note in the spec was rejected because it is unverifiable. A
check that the gate's archive list equals the manifest targets was rejected as redundant:
`go-archive-gate` already fails at run time on a closed-set mismatch, and "exact target" in this
spec means the exact release commit. Chosen: `make gate` removes `<git-dir>/corvint/release-gate-receipt`
before its first step. After its last step it runs `script/gate-receipt record`, which writes one
mode-0600 line binding HEAD's commit and tree to the SHA-256 of the archive witness from the same
run. Recording refuses, leaving no receipt, unless the worktree is clean and `archive-status` judges
that witness `PASS` for HEAD. `release-checklist` gains a seventh row, `full-gate`, placed after
go-archive.

Consequences stated plainly. The receipt is local operator evidence at the same trust level as the
archive witness. It is not a signature, a CI attestation or a publication receipt, and anyone with
write access to the git directory can forge it. CI still runs only `make go-archive-gate`, so CI
never produces the receipt. Recording is the gate recipe, so even under `make -j` it runs only
after every prerequisite has succeeded. The receipt at the release candidate stays `NOT_RUN` until the coordinator
runs `make gate` there.

Amendment, 2026-09-13 (bug review). Recording only at the end let a commit made while the gate ran,
or a worktree dirty when it started, bind steps that ran on other content to the final HEAD, and a
failing `git status` counted as clean. The clear step now also stamps HEAD's commit and tree when the
worktree is clean, and recording refuses unless that stamp still equals HEAD and `git status`
succeeds. Under `make -j` the stamp is taken alongside the first steps, not strictly before them.
`release-checklist` also treats bytes after the receipt's one LF as noncanonical.

Amendment, 2026-09-13 (`make -j` ordering). GNU make 3.81 under `-j4` starts sibling prerequisites
before the first one finishes, so a commit landing while the clear ran could be stamped as the
start of steps that had already begun on other content. When `gate` is a goal, every other gate
step now order-only depends on `gate-receipt-clear`; a step run on its own still does not clear.

Spec: `GOC-V0-010` in `docs/specs/go-only-cutover-v0.md`; `ARTIFACT-RDY-V0-001/002` in
`docs/specs/release-artifact-integrity-v0.md`. Evidence: `script/gate-receipt_test.sh`,
`script/release-checklist_test.sh`.
