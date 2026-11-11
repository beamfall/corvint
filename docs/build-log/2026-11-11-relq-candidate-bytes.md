# 2026-11-11: linux-core binds its archives to the candidate's bytes (V1-0374)

Ticket V1-0374, requirement `PRS-V1-1900` (proposed, experimental; the owner's decision is
pending). Refines `PRS-V1-004` and decision 0420 item 2. Port of lane `rel-u42`'s unmerged work
of 2026-11-02 onto `b55635262`.

## What changed

- `script/release-candidate-bytes.sh` checks the rebuilt `SHA256SUMS` against an expected digest
  and the linux/amd64 archive against its row. It prints one identity record: commit, tree,
  `SHA256SUMS` digest, archive digest, binary digest, runner label and image, run URL and one
  digest line per `--report`. The verdict is `CANDIDATE_BYTES` or `SOURCE_ONLY`. A mismatched
  digest prints `NOT_CANDIDATE` and exits 1, and so does an archive that does not match its row.
  Every refusal names its recovery (`ASD-V0-997`).
- `script/release-candidate-bytes_test.sh` (`make release-candidate-bytes-test`, in `GATE_STEPS`
  and `shell-test`) covers each verdict and refusal. Until the `.github` change lands, its workflow
  cases are skipped with the reason printed. The skip applies only while `release-gates.yml` has
  exactly its pre-change sha256, so any other workflow bytes must carry the whole binding.
- A separate `.github` change adds the optional `candidate_sums_sha256` dispatch input, validates
  it in `Check inputs`, and adds the `linux-core` bind step (`candidate-identity` log) and a final
  `report-index` step. Permissions, triggers, secrets and environments are unchanged.
- `docs/RELEASE-RUNBOOK.md` describes the dispatch. Decision 0415 is not edited. The proposed
  text of an item 8 amendment goes to the owner instead, because `line-citations-check` treats a
  decision record as written once (`DCG-V0-800`). Once 0415 is edited, its citation of
  `internal/tasks/wire/limits.go`, a file decision 0594 moved out, resolves against the
  current index and fails.

## Decisions made in the lane

- The old index step also digested `host-lifecycle/*.txt` reports. Decision 0594 removed the
  `HLQ-V1` runner and that step from `linux-core`, so the port indexes only the retained logs.
  The requirement says that a `CANDIDATE_BYTES` record alone promotes nothing. Decision 0420
  item 2 still names a plain-CLI host lifecycle report that the job can no longer produce, and
  reconciling the two is the owner's call.
- The requirement's leading note says `proposed, experimental` rather than citing the standing
  lane auto-accept, as the old commit did, so `REQUIREMENTS.tsv` records it as `unrecorded`.
- The old commit's one-line edits to three specs moved `Makefile:N` citations that the new
  `GATE_STEPS` line shifts. `script/check-line-citations.sh --repin` repinned the citations that
  still move on main (`go-archive-gate-v0.md`, `release-artifact-integrity-v0.md`,
  `eol-policy-gate-v0.md`). `compat-replay-runner-v0.md` cites a commit this repository does not
  hold, so the check skips it, and it was left alone.

## Evidence

- `bash script/release-candidate-bytes_test.sh`: 16 cases PASS against the old workflow, with
  the workflow cases skipped; 25 cases PASS with the workflow binding.
- The suite fails each of three script mutants: no digest comparison, no row comparison, and a
  no-input path that prints `CANDIDATE_BYTES`.
- It also fails each of five workflow mutants: the bind step without `--expect`, the bind step
  always exiting 0, the index step without `--expect`, and `Check inputs` without its
  character-class or its length check.
- A hosted `CANDIDATE_BYTES` run of a frozen candidate is `NOT_RUN`: it needs the owner's
  dispatch. linux/amd64 stays FALLBACK.
