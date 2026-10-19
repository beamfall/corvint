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

Independent review (one Codex round, read-only) reported ten findings; all ten were confirmed by
a reproduction that now passes, and none needed an accepted requirement amended:

- Calls on parameters, locals or undeclared globals were silently dropped, so a step could read
  `NONE_FOUND`. They now read `callee-not-resolved` (`CALL`), except the step function's own
  `req`/`res`/`next` API (one member deep) and JavaScript built-ins.
- Calling a `pkg.create()` client was classified as `create` (local); it is now outbound, and the
  client's initializer is scanned, so `axios.create({ baseURL: process.env.API_URL })` lists
  `API_URL`.
- Parameter defaults and later declarators (`const a = 0, b = process.env.B`) are now read.
- A dynamic `process.env[...]` subscript is reported and its expression still scanned.
- Lexer uncertainty is now a `SOURCE` unknown on each step that reads the file, not only global.
- `KEY="" # note` in a profile now reads empty.
- `preRun.unresolved` counts every kind; only `CONFIG_KEY` does not make the check INCOMPLETE.
- A `--handler` imported from a module over the bound or unreadable now refuses with
  `request-path-bound-exceeded` or `request-path-source-unavailable` instead of
  `request-path-handler-not-found`; an imported module needed by a step stays a `CALL` unknown,
  as AMAP-V0-137/138 now state.

Mutation evidence: 30 source mutations of the new rules (outbound list, initializer scan, empty
value, cheap-guard proof, finding emission, dynamic and escaping env reads, unknown packages,
fallback defaults, INCOMPLETE verdict, `--warn`, unresolved module calls, UNKNOWN step outbound,
`use` prefixes, express() receivers, destructuring defaults, constant keys, `this.m()`, global
`fetch`, depth bound, and the ten review fixes) were each applied alone and each failed at least
one `internal/requestpath` test (30/30 killed).

Limits recorded as non-goals in the spec: other frameworks, middleware mounted in other files,
data flow through parameters or locals, and value checks of configuration keys.

Rollback: revert the change; the command, package and spec section are removed together, and no
stored state depends on them.
