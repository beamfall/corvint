# Subject-ancestor governing instructions — V1-0411

Slice base: `de427c57f8bcd6a01a7cfb4d460f6a172e6c6586`; batch base remains
`b4fb66a6252ca94c53031cf627ee9da79bc6e0c1`. Owning intent: TCP-V0-008,
TCP-V0-011 and TCP-V0-047 in `docs/specs/task-context-packet-v0.md`.

## Reproduction and contract

The full CLI fixture has root and `cache/` AGENTS.md and CLAUDE.md files, a sibling
instruction file, and a subject in `cache/`. Before the change its first governing
row was the root AGENTS.md; the new regression fails. After the change it returns
both local files before both root files, each with its own pinned evidence row.

For a subject, governing candidates are exact tracked AGENTS.md and CLAUDE.md paths
in the subject directory and its ancestors, closest first and AGENTS before CLAUDE
within a directory. The subject itself, siblings, descendants and prefix lookalikes
are excluded. The first 50 candidates match the maximum packet limit; an unread
candidate remains a disclosed gap, and candidates beyond the cap remain counted.
Readable ancestors apply independently. The original one-row root fallback remains
when no scoped candidate exists; subjectless/project-operation admission is unchanged.

Final-limit omissions remain explicit in critical_missing. Instruction routing walks
reserved governing files in the same precedence order with the existing shared
two-row cap; each route names its actual instruction file and line. Existing source
read bounds, exclusions and trust classification are retained; no instruction body
is added to the packet. TCP-V0-006/008/009/011/047 and requirement locators are updated.
The conservative source audit advances analyzer/91 to analyzer/92; extraction tables
and wire fields are unchanged. Rollback reverts this subject-scoped selection and
multi-file routing amendment, retaining qualification evidence.

## Evidence and remaining work

Focused context, CLI, ancestor, project-operation eligibility, local-prompt and
analyzer-schema regressions pass. Fixtures cover six governing ancestors, explicit
subjectless behavior, an instruction file as subject, a one-row budget, an excluded
unread ancestor, the 50-candidate cap and source-attributed shared-cap routing.

Private evidence is `/private/tmp/corvint-bugfix-20260928/v1-0411/`, including the
failing pre-fix and passing post-fix full-command runs. `affected` ran before tests.
Original pre-change receipts and the existing keyed completion enrollment are
retained. The existing scratch runner and its proven interruption cleanup are reused.

Independent review, frozen retrieval evaluations, held-out cobra qualification,
and final CEM/OCM/dogfood verification remain pending for the combined candidate.
The coordinator deferred expensive final checks until the already-planned source
slices finish. The ticket's legacy full-gate metadata is explicitly overridden by
the owner's scoped-check instruction: the exhaustive gate is NOT_RUN. This entry
is implementation evidence, not ticket completion or release qualification.
