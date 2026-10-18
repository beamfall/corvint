# Change-compiled E2E: expert panel amendment to CCE-V0

Owner input, 2026-10-17 (Claude Code session), verbatim: "i want you to use experts to ensure that
we have the perfect system to be able to do" the change-compiled E2E idea (recorded in
`2026-10-17-change-compiled-e2e-direction.md`), and "I want a world class solution that is a
perfect additional and use case for corvint".

## Panel

Six read-only lenses, each an Opus 5.5 subagent at high effort with one shared charter (read-only,
file:line evidence, at most eight findings, candidate requirements, cheapest falsifier): statistics
of the shadow gate, E2E automation platform, product strategy, agent authoring and authority,
change-impact soundness, and oracle and proof quality. Every lens returned CONCERNS. The
orchestrator re-read every cited line in the first slice (PR beamfall/corvint#1081, `internal/changee2e`)
before recording a finding; two citations were off by a few lines and are recorded at the verified
lines below.

## Verified findings in the first slice

- Obligations: `obligations.go:74` builds the catalog only at head; `obligations.go:161-168` derive
  the overall state only from the counts of unknown spans and obligations. Impact unknowns and
  unplaced changed paths are kept as unknown spans (`obligations.go:92-118`), but a changed path that
  impact places on a screen with no flow step is accounted for and yields no obligation, so a diff of
  only such paths reads `NONE`. The catalog's own map unknowns and the impact relation's omissions
  are never read, and no base map is joined, so a screen, flow or outcome the diff removes leaves no
  trace. (Codex corrected the first wording of this finding.)
- Proof: `prove.go:356-368` stages the whole revision, so the base run executes base page objects
  and fixtures; `prove.go:586-593` returns `FAILS_AT_HEAD` before reading base; the two BBF-V0
  controls edit only the assertion template, so they are killed by construction.
- Authority and isolation: the oracle comes from the head revision, so a change can edit its own
  outcome; `cmd/corvint-test/main.go:105-128` exits 0 whatever the verdict;
  `internal/jstestprovider/external.go:182` passes the whole environment; `prove.go:345` keeps only
  the receipt digest.
- Vocabulary: `compile.go:146` always enters at the flow's first step; `compile.go:343-366` gives
  fixtures no arguments or outputs; `catalog.go:179-181` leaves every edge unadmitted;
  `catalog.go:219` admits three matchers; `catalog.go:254` rejects parameterised URLs;
  `prove.go:69-76` is Chromium only.
- Shadow gate (draft text): seeded regressions were BBF-V0 controls, which edit the test, so parity
  compared self-sabotage; an escape metric over merged changes is vacuous because merged changes
  pass the hosted suite; one caught regression bounds a miss rate only below 95%, and a zero-miss
  bound below 5% needs 59.
- Tests: the live prove tests skip without `CORVINT_PLAYWRIGHT_MODULES` and no CI lane sets it.

## Decisions

- Reframe: V0 sells a per-change proof to the agent and reviewer, measured by a per-change
  usefulness rate (kill below 40%) and time-to-verdict; hosted-test retirement is the V2 payoff.
- Added CCE-V0-016..025: oracle origin, test tree held at head, structure controls, obligation
  soundness, closed unknown reasons with reviewed `reach_exempt`, environment block, isolation and
  report binding, a closed exit-code table, compile hygiene, and `untrusted_text`.
- Amended CCE-V0-003 (optional members), 004 (only the relations actually consulted), 009 (ladder
  with `REGRESSION_DETECTED`, `SCENARIO_BROKEN`, `NEW_REACH` and error-location matching), 011 and
  012 (application-side mutants, bug-fix pairs, every non-scoring verdict a miss, preregistration,
  Clopper-Pearson bounds with Holm correction). The first slice's PASS rows stay, annotated as
  evidence for the pre-amendment text.
- A falsification spike runs before more V0 build: 10 historical bug-fix commits on a real
  Playwright repository plus two oracle-review falsifiers.
- Rejected: an LLM in the compile path, and runtime coverage that can narrow reach. Both would
  break invariant 2.

Tickets: V1-1677 (spike), V1-1678..V1-1682 and V1-1687 (V0 work, now dependencies of umbrella
V1-1666), V1-1683..V1-1686 (V1 and later roadmap). Owner questions 6..8 were added to the spec.

## Independent review

One read-only Codex round on the amendment returned eight findings, all confirmed and fixed here:
shadow catches are now counted once per target identity across eligible scenarios; a `prove`
report bound to `UNKNOWN` or `INCOMPLETE` inputs exits 3; the positive ladder rungs need a complete,
all-passing head pool; the Holm family, test and sample sizes are stated (59 unadjusted, 72 for the
first step with two candidates) with entries as a second unit; the shadow window has three typed
entries with explicit application and test revisions; obligation overflow is exempt from the
refuse-on-limit rule; the test tree includes runner configuration and dependency resolution; and the
obligations finding above was corrected.

## Not done

No implementation. The amended requirements are NOT_STARTED and inherit `proposed` intent.
