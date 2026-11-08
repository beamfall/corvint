# Runner selection baseline update

Copy this template to `docs/build-log/YYYY-MM-DD-<slug>.md` in the same change
as the soundness fix and its baseline update (RSP-V0-039). Do not weaken a
soundness assertion to preserve an old narrowing count.

- Ticket and governing requirement: RSP-V0-039; <ticket>
- Immutable comparison: <base SHA>..<head SHA>; <runner/configuration identity>
- Soundness fix: <the missing dependency or invalid exclusion, and its cause>
- Old selection: <FULL or exact selected identities and baseline path>
- Corrected selection: <FULL or exact selected identities and baseline path>
- Tests re-selected: <each identity newly required; FULL with unknown inventory
  must remain explicitly unknown, never an empty set>
- Baseline change: <old/new counts, retained replay digests, why soundness wins>
- Positive evidence: <requirement-named fixture and observed result>
- Negative evidence: <conflicting old baseline rejected; missing log/test rejected>
- Validation: <commands, exit codes, immutable evidence bindings; NOT_RUN reasons>
- Review: <independent review reference or NOT_PRODUCED reason>
- Rollback: <restore FULL/shadow; never restore the unsound exclusion>
- Owner sampling probability and CI cost: <owner-set value and source revision,
  or UNSET; measured cost and evidence, or NOT_OBSERVED>

`internal/runnerpolicy.CheckBaselineUpdate` checks that the baseline includes
the fixed run set and that the entry names the fix and newly selected tests.
A FULL baseline remains acceptable when the inventory is unavailable. Record
actual replay and review evidence above; this template is not passing evidence.
