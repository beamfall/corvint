# V1-1941: request-path environment keys, outbound calls and guard order

Issue 1200 asks, before a test batch runs against a target environment, which environment and
configuration keys one route's request path reads, which guards call out to the network, whether
such a guard runs before a cheaper validation guard, and whether the target profile leaves a
required key unset or empty. `corvint flows request-path` (AMAP-V0-130..139 in
`docs/specs/application-map-v0.md`, accepted under the owner's standing auto-accept of 2026-10-11)
answers it from committed Express-style JavaScript/TypeScript. Delivery is experimental.

Decisions:

- A new package, `internal/requestpath`, reuses `appmap.LexJS` and reads Git blobs directly. It
  adds no application-map, manifest or index member, so no persisted format changes, and it stays
  disjoint from the V1-1940 test-environment contract (`corvint tests env`), which reads tests,
  not application request paths.
- Outbound classification is a closed list of HTTP client packages plus the global `fetch`.
  Packages on a local list (validation, JWT, Express, config) read as no outbound call; any other
  package reads `package-not-followed`. A step with no call found reads `NONE_FOUND`, never
  "none", and any unresolved call, middleware, source or bound turns it `UNKNOWN`.
- The guard-order finding requires the later guard to be proven cheap (`NONE_FOUND` with no
  unknowns); an unknown later guard is not claimed cheap.
- The profile is reduced to key-presence booleans at parse time, so no value can reach the report.
  Fail-closed is the default; `--warn` schedules with the same findings. Unresolved entries give
  `INCOMPLETE`, which also does not schedule by default, because a key read there is unchecked.

Mutation evidence: 20 source mutations of the new rules (outbound list, initializer scan, empty
value, cheap-guard proof, finding emission, dynamic and escaping env reads, unknown packages,
fallback defaults, INCOMPLETE verdict, `--warn`, unresolved module calls, UNKNOWN step outbound,
`use` prefixes, express() receivers, destructuring defaults, constant keys, `this.m()`, global
`fetch`, depth bound) were each applied alone and each failed at least one
`internal/requestpath` test (20/20 killed).

Limits recorded as non-goals in the spec: other frameworks, middleware mounted in other files,
data flow through parameters or locals, and value checks of configuration keys.

Rollback: revert the change; the command, package and spec section are removed together, and no
stored state depends on them.
