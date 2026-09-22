# Native multi-repository benchmark driver

`go run ./benchmarks/runner` evaluates the pinned retrieval manifest through the native Go index
and evaluation packages. It validates revision-pinned source proof before evaluation, independently
replays baseline and learned-arm metrics, runs the deterministic exact-text control, preserves the
registered blind-v2/blind-v3 first-run provenance, and refuses release readiness for subset runs.

The driver does not run or register blind-v4 by itself. Its tests use synthetic repositories and
the already observed development manifests; they do not produce retrieval qualification evidence.
