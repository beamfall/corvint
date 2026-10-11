# Probed-tree parity repeated-run evidence

Date: 2026-10-10

## Context

V1-1064 retains additional evidence for TM-V0-008 on `origin/main`
at `29e1f653486fca13216606683d3a5c2b05cacd36`.

## Evidence

On Go 1.27.1 darwin/arm64 with `GOMAXPROCS=3`,
`go test -count=20 -run TestTMV0008_ProbedTreeReadParity ./internal/tasks/cli`
passed (`ok`, 31.0s).

Main's normalization, delivered by `777c15f7`, applies the `observedAtField`
regexp in `internal/tasks/cli/probed_tree_parity_test.go` to both envelopes
before `bytes.Equal`. Any other byte difference still fails.

## Qualification Limits

This evidence covers darwin/arm64 on one host only. The flake is probabilistic;
a passing repeated run does not prove its absence. Hosted CI is `NOT_RUN`.

The earlier ticket-branch `008325f9` count=30 run is not retained or claimed:
that commit is not an ancestor of the pinned main source.

This entry records evidence only: no source change and no new requirement.
