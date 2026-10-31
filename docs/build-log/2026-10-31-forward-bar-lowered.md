# Forward shadow bar lowered to 8 complete rows and 1 failing — AFP-V0-029

Decision (owner chat, 2026-10-31: "Lower the bar"; the values were chosen by the orchestrator
under owner delegation): the AFP-V0-029 shadow narrowing bar moves from 20 complete rows of the
epoch, at least 5 with failing packages and no non-`FLAKY` miss in the epoch, to 8 complete rows,
at least 1 with failing packages and no non-`FLAKY` miss in the epoch.

Unchanged: the no-miss rule over the whole epoch; the AFP-V0-030 kill switch (2 `MISS` or
3 `INCOMPLETE` in the latest 20 rows, with its latest-20 window, which `bar.jq` keeps at 20);
`INFRA` (AFP-V0-052) and `NARROWED` (AFP-V0-099) handling; epoch handling (CIN-V0-001); and the
coverage rule, under which a listing that neither ended nor reached an earlier epoch is
NOT ESTABLISHED, never MET.

Rationale: after a day, epoch 2 held 2 complete rows and 0 with failing packages. A single full
pull-request run costs 8 shards of 20 to 30 minutes each, so the 20/5 bar kept every pull request
on FULL for weeks of runs. Safety does not come from the row count: any non-`FLAKY` miss in the
epoch keeps the bar NOT MET, and the kill switch returns pull-request runs to FULL
(`forward-qualification-tripped`) on 2 `MISS` or 3 `INCOMPLETE` rows in the latest 20. The
lower count only shortens the qualifying sample; it accepts a weaker statistical claim about the
selector's miss rate in exchange for earlier narrowing that the zero-miss rule and the kill switch
still bound.

Changed: `.github/forward-rows/bar.jq` (threshold and summary text), `.github/forward-rows/verdict.sh`
(summary text), AFP-V0-029, AFP-V0-030 and AFP-V0-047 in `docs/specs/affected-plan-v0.md` with
amendment notes, and `docs/specs/ci-narrowing-v0.md`.

Evidence: `TestAFPV0047BarCountsArtifactNames` now proves each boundary at the new thresholds
(7 complete with enough failing is NOT MET; 8 complete with 0 failing is NOT MET; exactly 8 and 1
is MET; an INCOMPLETE or INFRA row is not complete; a MISS blocks an otherwise exact bar; an
uncovered listing is NOT ESTABLISHED; 2 MISS trips the switch; a failing row older than the
latest-20 window still counts), and `TestAFPV0099BarExcludesNarrowedRows`. A local mutation of
`bar.jq` to 7, 9, 0 or 2 failed the first test each time.

Rollback: revert the commit; the bar returns to 20 complete rows and 5 failing on the next
`forward-rows` job, with no row or verdict migration.

Dogfood intent scope: the CEM's intent scope is `docs/specs/ci-narrowing-v0.md` alone.
`docs/specs/affected-plan-v0.md`, the owning spec of AFP-V0-029, AFP-V0-030 and AFP-V0-047, holds
more than 256 requirements, so `ocm prepare` refuses it (`too-many-obligations`; DOGFOOD.md §4,
OCM-V0-023 (3)). The hunks instead cite the bar's origin in decision 0431 (item 7) and the
unchanged AFP-V0-030 and AFP-V0-047 rows of `docs/specs/REQUIREMENTS.tsv`. Omitting the spec is not
a claim that its requirements hold; they are carried by the tests above.
