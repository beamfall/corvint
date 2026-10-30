# Test Claim Qualification V0

The binding contract is [`specs/test-claim-qualification-v0.md`](specs/test-claim-qualification-v0.md).
The reference library is implemented in `context_corvint_test_claim` with its bounded JUnit projection
isolated in `context_corvint_test_claim_junit`. The result wire schema is
[`tcq-0.schema.json`](tcq-0.schema.json).

V0 is library-only. `evaluate_tcq` consumes immutable raw canonical CEM 0.2 and OCM 0.1 bytes plus
independent expected-base and target revisions. No public skip-verification compositor seam ships;
a future shared WP5 compositor must enter the module-private evaluator only from the same successful
raw authority path. The API neither executes tests, opens report paths, persists artifacts, invokes
a shell, nor emits a frontier decision. It makes one private immutable copy per supplied artifact
and completes all raw size and JSON resource preflights before semantic parsing or Git authority.

The sole positive relation is `test-report-matched-v0`, always with
`authorityClass: CALLER_REPORTED`. It is useful evidence for review, but it is mechanically
ineligible to close the default Verified Absence Frontier. JavaScript and TypeScript claims abstain
in V0; Python 3.9 and Go are the only admitted source profiles.

Implementation and deterministic unit evidence are present. The independently labelled 60-edge
corpus, reporter compatibility runs on real repositories, and ten-change Corvint dogfood promotion
gate remain `NOT_RUN`; this implementation must not be advertised as validated reporter
compatibility or trusted execution evidence.
