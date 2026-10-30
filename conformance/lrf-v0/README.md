# Lexical Relevance Floor V0 conformance companion

This directory held the 41 spec-owned inputs and frozen expected outputs in `cases.json`. Every
case's request binds a cem/0.x map, so the corpus was retired with the cem/0.x wire (decision 0525
§1, CEM-V1-217) and `cases.json` is removed. `retired.json` records each retired case with its
action and its input and output digests from `manifest.json`, and names the stable successor.
`manifest.json` and `registration.json` stay byte-frozen as the historical preregistration. The
`internal/lrf` unit tests still run with `go test ./internal/lrf ./internal/lrfrepo`. The Python
driver is retired under GOC-V0-002.

The companion applies only to a future implementation and evaluation begun after its registered
manifest digest was published. The original WP3 runtime build began earlier and cannot count as
that preregistered experiment. Deterministic conformance does not replace a new evaluation following
the specification's publication order.

Historically the Python evaluator's `failure.bound` differed from two frozen deletion-case
expectations. That recorded failure remains a failure; removing its driver does not turn it into
PASS or native performance qualification.
