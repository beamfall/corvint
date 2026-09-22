# Lexical Relevance Floor V0 conformance companion

This directory retains the 41 spec-owned inputs and frozen expected outputs. Run their native
consumer with `go test ./internal/lrf ./internal/lrfrepo`. The Python driver is retired under
GOC-V0-002; expectations and original provenance remain unchanged.

The companion applies only to a future implementation and evaluation begun after its registered
manifest digest was published. The original WP3 runtime build began earlier and cannot count as
that preregistered experiment. Deterministic conformance does not replace a new evaluation following
the specification's publication order.

Historically the Python evaluator's `failure.bound` differed from two frozen deletion-case
expectations. That recorded failure remains a failure; removing its driver does not turn it into
PASS or native performance qualification.
