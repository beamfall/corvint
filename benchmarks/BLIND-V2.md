# Blind V2 evidence

The five `heldout-v2` cases were authored and frozen without Corvint implementation or output access.
Their first Corvint run is preserved as negative evidence, not a release pass:

- manifest SHA-256: `1365eec079f861d8abe03123afdeefa4bd9da23f51bb2c7efce4366f553c87b1`
- Corvint identity: `working-tree:f6b6c6b564667652c6396f65e88f395493e23176e7b6ccce13def07902f76ad3`
- raw artifact: `benchmarks/results/blind-v2-first-run.json`
- raw artifact SHA-256: `6cd5bd34b0d0be2b7bceaa1f0bb44a69ed03bc36be1217862bff2a51cf28a356`
- result: 5 cases; recall 0.071429; 7/10 critical items missed; precision 0.182141;
  top-5 success 0.4; abstention and epistemic-state accuracy 0.0

The raw artifact was generated before any result-driven repair. Once observed, these cases cease to
be held out: later executions are challenge replays and the runner labels them
`development-after-first-run`. The V4 development corpus includes the older blind-v1 cases, which
the runner labels `development-after-observation`; their passing score is not independent evidence.

The raw first-run bytes are preserved in the repository at the path above. Later replays must never
overwrite that artifact.

The release runner also discloses `revision_pinned_cases`. Five legacy Beamfall development cases
pre-date human span annotations; their selectors are mechanically resolved at the pinned tree but
they are not counted as revision-pinned ground truth.
