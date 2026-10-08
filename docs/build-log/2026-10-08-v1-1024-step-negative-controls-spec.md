# V1-1024 step-level negative controls: proposed LPCV amendment

Ticket V1-1024, GitHub #682. This change is spec-only: no code, fixture or behaviour changed.

## Decision: amend LPCV rather than add a new spec

The strength axis is defined in Live Proof-Carrying Verification V0. `LPCV-V0-047` defines it as
the mutation witness, and `LPCV-V0-048` keeps it `NOT_MEASURED` without one. The Go projector in
`internal/testvalidity/projection.go` carries the states `KILLED`, `SURVIVED`, `NOT_MEASURED` and
`UNSUPPORTED`. The two existing Playwright control paths both consume that axis:

- `PTF-V0-006`/`PTF-V0-007` use a hand-authored served-response `ChangedFixtureValue`.
- `NEA-V0-003` uses approved behavior-falsification plans.

The amendment therefore adds proposed `LPCV-V0-057..070` to the owning spec instead of creating a new
prefix. It reuses the existing axis states, so the shared vectors and the VS Code mirror are untouched.
The owner's `UNPROVEN (manual control needed)` becomes a rendering of `NOT_MEASURED` with reason
`step-fault-underivable` and `requires: manual-control`. Intent acceptance is human-owned, and the
LPCV header, digest, README and INDEX record the requirements as proposed.

## Constraints from existing contracts

- `LPCV-V0-051` makes `test-validity` run no test and write nothing. `negate` is written as an
  explicit proposed exception, and the read modes stay byte-identical.
- The core binary does not load Node (invariant 7, and the PWP statement that the default binary is
  unchanged). Execution is delegated to the companion named by `--provider`, never one found on `PATH`
  (`LPCV-V0-042`). Whether to move the verb to the companion alone is left as an open owner question.
- PWP excludes trace content, and external-mode argv admits no trace option (the
  `allowedExternalOption` function in `internal/jstestprovider/external.go`). The trace is therefore
  recorded through the controlled config and kept only transiently.
- Discovery's 256-entry and 16-attempt bounds and the provider's 32-file prune (`LPCV-V0-053`,
  `LPCV-V0-055`) cannot hold thousands of controls. Results therefore go to a separate
  `.corvint/strength-evidence/` directory, keyed by test identity.
- A spurious network dependency must not accuse a test, so a network survival alone never yields
  `SURVIVED`.
- A joint `--all-steps` run cannot satisfy "only that step fails", so it uses per-step marker
  attribution and never decides `SURVIVED`.

## Evidence

Failing-before and passing-after evidence for behaviour is `NOT_RUN`, because no behaviour changed.
The doc gates and `go test ./internal/specindex` were run on this change and are reported in the lane
handoff. The live matrix (`LPCV-V0-070`), the Go tests, the owner sample comparison against
hand-authored controls and the G2 cost measurement are `NOT_RUN`. The figures of about 3,600 controls
and 3,614 rows come from the owner's issue and are `NOT_OBSERVED`.

## Non-goals and rollback

The non-goals are those listed in the spec: other runners, mutation of application or test source,
retained traces, and changes to NEA or PTF verdicts.

Rollback: revert this commit. No retained state or wire format exists yet.

## Independent review

Codex (gpt-6-astra, read-only) reviewed `78afb59f` and raised four findings. All four were accepted
and fixed:

- P1: a single-step document had no complete denominator. A retained baseline `inventory` and
  `assertionsOutsideSteps` now gate the `KILLED` join.
- P2: per-step plan digests in the binding made `--step A` then `--step B` replace each other. The
  common binding is now separate from each entry's `planDigest`.
- P2: DOM derivation lacked the literal and polarity restriction. Negated and non-literal assertions
  now derive no fault, and a marker that satisfies the expectation gives `fault-does-not-falsify`.
- P2: marker generation was circular with the plan digest. The marker now comes from a
  pre-injection seed digest.

The re-review of `8131a024` confirmed the four fixes. It raised one further P2: an empty
denominator could yield `KILLED` vacuously. The fix requires at least one assertion-bearing
inventory step.
