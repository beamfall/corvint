# RSP: prove each runner in isolation, reuse until its contract changes

Owner ask (2026-11-07): each runner must be proven in isolation and then trusted unless the plugin
contract changes. The spec already qualified each runner alone (`RSP-V0-033`..`035`) and reused the
record until an identity changed (`RSP-V0-036`), but left the plugin digest of a compiled-in runner
undefined: read as the Corvint binary digest, every Corvint commit would requalify every runner.

Decision (orchestrator-made under the owner's delegation): `RSP-V0-059` defines that digest as the
runner's transitive in-module package closure plus the SPI version, and `RSP-V0-060` adds a cheap
per-runner contract suite (the conformance kit over fixtures) keyed by the same digest, which must
pass before qualification. Implementation is carried into the RSP integration repair (T6 identity,
T10 kit adapter). Runner-version, tracing-backend, selector, record-format and environment changes
still invalidate, as `RSP-V0-036` requires.

Follow-up (owner, same day: patch releases and small changes must not mean daily requalification):
`RSP-V0-061` carries a qualification across a runner-version, plugin-digest or tracing-backend
change by one equivalence run of the corpus (equal inventory and selector-consulted record
components against the original baseline), with no trust in version numbers; `RSP-V0-062` re-scores
retained trials offline for a selector change. Record-format and execution-configuration changes
still need full requalification. Qualification is event-driven, never scheduled.
