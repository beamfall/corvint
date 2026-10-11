# Application Navigation Ledger V0

Owner: Russell Lewis
Date: 2026-10-10
Intent status: accepted (decision 0518)
Delivery status: experimental
Authoritative inputs: owner requests V1-1132 (issue #774), V1-1162, V1-1163, V1-1166, V1-1167, V1-1133 (issue #775), V1-1134 (issue #776), V1-1164 (issues #776/#777), V1-1135 (issue #777), V1-1137 (issue #779); issue #814; V1-1159; V1-1160; V1-1161; V1-1165; `AGENTS.md`;
`docs/SPEC-DRIVEN-DEVELOPMENT.md`; AMAP-V0 and RVN-V0.

## Agent digest
- Claim: A revision-evaluated edge ledger reuses qualified receipts and fails closed on unproven assumptions.
- Status: accepted (decision 0518; V1-1132..1135, V1-1137); experimental.
- Exists: smallest single-map CLI slice; synthetic receipts only; static page-object step effects for prefixes (ANL-V0-035/036, V1-1164, decision 0520); a config-installed Playwright navigation-events producer live-checked on Playwright 1.61.1 (ANL-V0-037..046, issue #814).
- Blocked on: live adopter qualification.
- Read next: Requirements; Wire and limits; Traceability; Follow-ups.

## User job and current state

The adopter needs to reuse a previously proven navigation transition without launching a browser.
At base `55aa6bb9`, `internal/appmap/verify.go` verifies individual declared step/test bindings;
`types.go` stores ordered flow steps, stable screen/selector IDs, free-text actions and flow anchors.
No existing field reliably encodes click versus redirect versus form submission. Router templates
are URL templates, not component source files. This slice makes neither missing fact up.

## Requirements

- `ANL-V0-001`: Derive one edge per distinct (from screen ID, to screen ID, trigger, selector ID)
  from consecutive resolved flow steps. Use the destination step's exact action when it is
  `goto`, `click`, `redirect` or `form submit`; otherwise trigger is `UNKNOWN`. The selector ID
  belongs to that destination step. Retain same-screen edges, sort by content-derived edge ID,
  count unresolved pairs separately. An UNKNOWN trigger describes only declared adjacency, not
  an observed interaction. (accepted, decision 0518; V1-1132)
- `ANL-V0-002`: Reuse RVN verification for each common test key bound to both endpoint steps,
  including sidecar declarations. Both endpoints must verify against the same receipt and app
  revision to prove a crossing. Report PROVEN, UNVERIFIED_AT_HEAD, CONTRADICTED or UNPROVEN;
  per-key evaluated unknowns and unplaced failures block proof; a key with no supplied receipt
  does not block another key's proof; a verified crossing for that key at any revision not observed
  reads `observation-revision-mismatch` and blocks. Check both router lineages and the flow
  anchor at evaluated and receipt revisions; a changed flow selector or router URL template
  demotes placed evidence. Keep learned authority and receipt/test/flow/step references.
  (accepted, decision 0518; V1-1132)
- `ANL-V0-003`: Deduplicate crossings by test key; choose the proven test with fewest declared
  flow steps, breaking ties by test key, then lexically. Print other proven test keys as redundant-crossers with
  a total count. Compute single-app proven/total coverage. A placed contradiction outranks a
  pass; unknown or stale evidence prevents promotion, and a skipped, unresolved-revision or
  otherwise inconclusive outcome for a proving test key blocks it even beside a fresh pass. (accepted, decision 0518; V1-1132)
- `ANL-V0-004`: Accept a closed JSON sidecar with tests declaring `proves` and `assumes` edge
  IDs. Proves binds that test to both steps of each occurrence but never creates a passing
  outcome. Check the union of repeatable `--require` and all assumes against the full ledger;
  absent, unproven, contradicted and stale edges fail with exit 1 and a sorted unsatisfied list.
  Invalid inputs refuse with exit 2. (accepted, decision 0518; V1-1132)
- `ANL-V0-005`: Emit deterministic `application-edge-ledger/0` JSON with evaluated revision,
  map digest, a digest of the complete ledger (digest field empty during hashing), and byte-capped
  projection with omitted counts. Trimming never changes coverage or assumption verdict. Use
  AMAP budget rules, safe bounded receipt/sidecar reads and no writes, browser or network.
  (accepted, decision 0518; V1-1132)
- `ANL-V0-006`: `corvint flows appmap observe --map FILE --trace FILE|DIR [--revision REV]`
  reads closed `playwright-navigation-events/0` reporter JSON (`test_key`, ordered `events` of kind
  `navigate` or `redirect`, each with a URL and an optional triggering action with kind and selector;
  at most 64 files of 4 MiB and 4096 events each) and prints `application-navigation-observation/0`:
  per trace the ordered visits resolved through the map's URL lookup (hash prefix aware), and one
  observed edge per consecutive pair with trigger `goto`, `click`, `redirect`, `form submit` or
  `UNKNOWN`; `goto`, `click` or `form submit` (an action of kind `submit`) only when the event's
  action says so, so an event without action metadata or with an action outside the typed-trigger
  vocabulary of `ANL-V0-021` reads `UNKNOWN`. A URL matching several screens reads UNKNOWN with sorted candidates; one matching none
  reads `unmapped-url`; an edge with an unresolved endpoint reads UNKNOWN. Observations are learned
  overlay facts carrying the trace digest and app revision and never add or change a static map
  node or edge. A Playwright trace zip is read only as `ANL-V0-022` states; a directory holding one is refused.
  (accepted, decision 0518; V1-1133)
- `ANL-V0-007`: Observation output MUST NOT contain URL userinfo, query values (including those
  inside a hash-route fragment) whose name is secret-shaped or whose value is opaque (24 or more
  token characters), or any action input value; such values print `REDACTED` or are dropped.
  (accepted, decision 0518; V1-1133)
- `ANL-V0-008`: Observation output is deterministic with a digest of the complete observation,
  byte-capped by AMAP budget rules with omitted visit and edge counts that never alter the digest,
  read-only and offline. (accepted, decision 0518; V1-1133)
- `ANL-V0-009`: `corvint test-plan prefixes --map FILE [--ledger FILE] [--facts FILE]` derives, per
  spec with a resolved test join, the ordered chain of the map's `test-sequence` edges and flags the
  spec when every edge of a chain of two or more edges is PROVEN in the `--ledger`
  (`application-edge-ledger/0`, matched on from and to screen) and the chain ends on a screen that is
  resolved, not abstract, has a template and whose lineage is FRESH at the evaluated revision. Each
  flag prints `{test, prefix_edges, proven_by, entry_template, saved_steps}` plus the entry screen,
  parameters, a `goto(template)` or session-fixture suggestion and the other flagged specs sharing the
  first edge. `saved_steps` is the edge count minus one. Without a ledger nothing flags.
  (accepted, decision 0518; V1-1134)
- `ANL-V0-010`: A spec abstains with a reason, never flags, when its join is UNKNOWN
  (`join-unknown`), its chain is discontinuous (`sequence-discontinuous`), any prefix edge is declared an assertion target
  (`assertion-target`) or mutating (`state-change`) in the `--facts` sidecar
  (`test-prefix-facts/0`, one entry per spec path; a duplicate path is refused; keyed `FROM_SCREEN>TO_SCREEN` per spec path), has no declared read-only
  effect (`effects-unknown`), is not proven (`edge-not-proven`), or ends on an entry that is not
  deep-linkable (`entry-not-deep-linkable`) or not FRESH (`entry-template-not-fresh`), or saves
  nothing (`nothing-to-save`, a one-edge chain). An edge that carries a derived effect is governed
  by ANL-V0-036 instead of the read-only declaration. (accepted, decision 0518; V1-1134; the derived-effect clause is ANL-V0-036, decision 0520)
- `ANL-V0-011`: The report is deterministic with a digest of the complete report, byte-capped with
  omitted counts, learned authority, read-only and offline. A ledger not built from the map (app, map digest, evaluated revision) or trimmed to its byte cap is refused. With `--check` the command exits 1
  (`test-plan-mismatch`) when a flagged test is absent from `--baseline` or any flagged row was trimmed by the byte cap (an earlier report; any flag
  fails without one), and 2 on invalid input. (accepted, decision 0518; V1-1134)
- `ANL-V0-012`: `test-plan page-state --input FILE` reads one `test-consolidation-input/0` document and
  groups variations that share app, screen, setup, user, org and the exact required fact set. A group
  has at least two variations (one visit can serve them), lists their IDs, specs and the union of
  assertions, and reports `visits_saved` as the variation count minus one. Groups may span specs.
  (accepted, decision 0518; V1-1135)
- `ANL-V0-013`: A variation joins no group and is listed as blocked with a reason when any anchor is
  missing (`anchor-missing:<members>`), it is destructive or its destructive flag is unknown
  (`destructive`), it declares changes (`changes-state`, so a post-change state is never merged with
  a pre-change one). (accepted, decision 0518; V1-1135)
- `ANL-V0-014`: The report (`test-page-state-groups/0`) is deterministic, sorted, independent of input
  order, carries the input digest and a digest of the report, is read-only and offline, and does not
  alter `consolidate` or `check`. Invalid input or use exits 2. (accepted, decision 0518; V1-1135)

- `ANL-V0-015`: `flows appmap impact --map FILE --base COMMIT [--ledger FILE]` prints
  `application-map-impact/0` for the paths changed between `--base` and the evaluated revision. A
  changed router file (a screen's anchor, name or parent anchor) selects that screen and every
  descendant by lineage (`router-anchor`, `lineage:<screen>`); a changed attributed test file or flow
  anchor selects its screens (`test-attribution`, `test-file`, `flow-anchor`). The report lists the
  edges touching selected screens (with ledger status, `flipped` for `UNVERIFIED_AT_HEAD` and the
  cheapest re-proving test), the redundant crossing test keys and the attributed spec files; without
  a ledger the map's edges are listed as `NOT_IN_LEDGER`; a ledger not built from the map at the evaluated revision, or trimmed, is refused. (accepted, decision 0518; V1-1137)
- `ANL-V0-016`: A changed UI path (html, script, style or template extension) that no map element
  places is listed as unknown (`unmapped-path`, or `manifest-changed` for the manifest) and forces the
  verdict `full-suite-required` for the app; otherwise the verdict is `targeted` or
  `no-navigation-impact`. The map binds no template or component files itself; ANL-V0-032 places
  the ones its reference hop can prove (this amendment accepted, decision 0519 (V1-1162)).
  The report is deterministic, byte-capped with omitted
  counts that never drop the verdict, digested, learned, read-only and offline; an unresolvable
  base or revision fails closed. (accepted, decision 0518; V1-1137)

- `ANL-V0-021`: Observed triggers are typed from the triggering action's kind: `goto`, `click`,
  `submit` (`form submit`), `press` (`key press`), `goBack` (`history back`), `goForward`
  (`history forward`) and `reload` (`reload`); a `redirect` event reads `redirect`; any other kind
  or no action reads `UNKNOWN`. An event may carry `annotation: {"trigger": T}` with T from that
  typed vocabulary (not `UNKNOWN`); it prints as the edge's `declared_trigger`, never sets or
  replaces `trigger`, and an annotation outside the vocabulary is refused with exit 2. Amends the
  ANL-V0-006 action list. (accepted, decision 0519 (V1-1161))
- `ANL-V0-022`: `observe --trace FILE.zip --test-key KEY` reads one Playwright trace zip (a regular,
  unsymlinked file of at most 64 MiB whose central-directory record signatures number at most
  65536, counted before the directory is built) in memory without extracting anything
  to disk. Exactly one root entry named `trace.trace` or `N-trace.trace` is decompressed, streamed
  as NDJSON within 64 MiB and 16 MiB per record; nested, `..` or other entries are never opened as
  the trace, and two context traces, an invalid record (including one without a `type`), more than one page with main-frame
  snapshots, no main-frame snapshot, or more than 4096 navigations refuse with exit 2. Each change of
  the main-frame snapshot URL (ignoring `about:` URLs) is one `navigate` event; it carries the
  action (`before` record method and selector, never its other parameters) only when the change lies
  between that action's `before` snapshot and its `input` or `after` snapshot, so any other change
  reads `UNKNOWN`. Intermediate redirect hops are invisible to snapshots and are not claimed. The
  trace digest is the archive digest; the test key is the caller's, since the archive carries none.
  `--test-key` is refused without a zip. (accepted, decision 0519 (V1-1159))
- `ANL-V0-023`: `edges --observation FILE` (repeatable, at most 16) feeds observed navigation edges
  into the ledger only when named; no read command applies observations implicitly or writes them.
  A file must be a complete `application-navigation-observation/0` document of at most 4 MiB built
  from the same app and map digest, with no omitted visit or edge, an intact digest, and each trace in the shape `observe` emits
  (visits in sequence over map screens, one edge per adjacent visit pair naming those visits'
  screens, resolved exactly when both visits are, with a typed or `UNKNOWN` trigger), else exit 2.
  Each resolved observed edge binds its trace's test key to every ledger edge with the same from and
  to screens whose trigger is `UNKNOWN` or equals the observed trigger (so an `UNKNOWN` observation
  never binds a typed edge), exactly as a declared `proves` would, but never creates an outcome: the
  key proves the edge only through ANL-V0-002 verification of its outcomes whose application
  revision equals an observation revision or does not resolve in the repository; its outcomes
  placed at other resolved commits
  are not read for that crossing, for proof or for ANL-V0-003 uncertainty, so a key with none reads `no-receipt-outcome` and, like an absent declared receipt, does
  not block another key's proof. At most 16 files are named; a 17th refuses before any is read.
  The ledger lists the observation digests, each row's `observed_by` test keys and the count of
  resolved observed edges matching no ledger edge (`observed_unmatched`); the static map is
  unchanged. (accepted, decision 0519 (V1-1160))
- `ANL-V0-024`: An edge cannot be proven only by the test it is used to select. An edge a declared
  test assumes is satisfied only when it is proven and the full ledger's provers (the evidence test
  key plus every redundant crosser, read before any budget trimming) include a test key other than
  that assuming test; otherwise the edge is listed unsatisfied with exit 1 although its row stays
  `PROVEN`. A `--require` carries no test identity and keeps the ANL-V0-004 verdict.
  (accepted, decision 0519 (V1-1165))
- `ANL-V0-031`: `corvint affected --base FULL_COMMIT_ID --appmap-impact REPORT` (repeatable up to 16,
  one report per map (ANL-V0-034); refused without `--base` or with `--snapshot` or `--playwright-config`) reads
  each complete `application-map-impact/0` report (a bounded regular file, strictly decoded with nothing after it, `authority` `learned`, no
  omitted rows, a verdict that follows from its own lists and a matching digest; otherwise refused) whose
  `base` is `--base` and whose `head` is the receipt revision, and adds `advice.app_navigation`: per
  app, sorted, the learned authority, sorted `impact_digests` and `map_digests`, `reported_verdict`, `screens` count,
  `spec_files` and the effective `verdict` with its `reasons`. The effective verdict is
  `full-suite-required` when the report lists unknown paths (`impact-unknown-paths`), the worktree
  has uncommitted paths the report cannot cover (`worktree-not-covered`) or the spec list exceeds 64
  (`spec-files-over-bound`); each such app also adds the `advice.unknown` line
  `APP_NAVIGATION_FULL_SUITE_REQUIRED: APP: REASONS`. The member is advisory: it never adds to or
  removes from `advice.checks`, the plan or the provider projection, so no selection is narrowed.
  This amends the AFP-V0-009 advice member set, also accepted by decision 0519. (accepted, decision 0519 (V1-1167))
- `ANL-V0-032`: A changed UI path no map element places is walked through the tracked UI-extension
  files at the evaluated revision (at most 20000 files, 1 MiB each, 32 MiB in all, 64 walked nodes).
  A file references the path when every occurrence of the path's token (a script's stem, an index
  file's directory or `index`, any other file's base name; a property `x.token` counts) lies inside
  one quoted string literal on its line that resolves to it (relative to the referrer for `./` and
  `../`, repository-relative otherwise, with script extensions and `/index.*` tried; no `${`), in a
  `.js`, `.jsx`, `.ts`, `.tsx`, `.mjs` or `.cjs` file whose JavaScript lexing has no doubt, as a
  string literal the lexer itself delimits. A mention in any other file (markup, styles, `.vue` or
  `.svelte` components) proves nothing and refuses. A walk ends at a router file anchoring screens, which
  selects the screens whose anchor spans hold its reference lines (every screen of the file when a
  reference lies outside all spans or the file's blob differs from the anchored one), or at a map test
  file selecting a screen or spec (a test file selecting neither is walked through). Selected screens carry `template-reference:<file>` and
  their lineage descendants. Any mention that is not such a literal (`unresolved-reference`), a walked
  file nothing references or that reaches no terminal (an import cycle, `unmapped-path`) or an exceeded bound (`reference-graph-bound`) keeps the
  path unknown, so the hop can widen a selection but never narrows the verdict. (accepted, decision
  0519 (V1-1162))
- `ANL-V0-033`: `test-plan page-state --input FILE --check --baseline FILE` (the two flags only
  together) reads the baseline as a `test-page-state-groups/0` report (a bounded regular file,
  strictly decoded with nothing after it, with a digest matching its content; otherwise exit 2), prints the fresh report and
  exits 1 (`test-plan-mismatch`) when, by group key, a group was added, removed or changed in any
  member, or a blocked variation was added, removed or changed its reason, naming each set (at most
  eight IDs each, with the remaining count). An input edit that leaves every group and block
  unchanged is not drift. (accepted, decision 0519 (V1-1166))
- `ANL-V0-034`: Several `--appmap-impact` reports may name one app when each comes from a different
  map (two reports of one app and map digest refuse). They aggregate into one `app_navigation` row:
  both digest lists, the count of distinct screen IDs, the union of spec files, and the most
  conservative reported verdict (`full-suite-required` over `targeted` over `no-navigation-impact`);
  any report with unknown paths gives `impact-unknown-paths`. An unknown in one map is never cleared
  by another map placing the path. (accepted, decision 0519 (V1-1163))
- `ANL-V0-035`: `flows appmap build` classifies every `test-sequence` edge (AMAP-V0-008) by static
  analysis of the spec tokens strictly between the two page-object constructions the edge joins,
  stored as `effect` on the edge. A call on a page-object instance (`NAME = new CLASS(` or
  `NAME: TYPE = new CLASS(`, CLASS imported from a test file; a name bound to two files, or
  assigned, member-assigned or destructured anywhere else in the spec, is not followed) reads that
  file's one class member of that name, following `this.m()` calls up to 8 deep. Constructing a
  page-object class, in the step or at either construction that bounds it, reads everything the
  construction runs: that file's tokens outside its recognized non-constructor methods, plus the
  closing construction's arguments. A page-object file whose class `extends` another or declares a
  `get`/`set` accessor is not read: its methods and constructions are `unknown` (`extends`,
  `accessor`). A page object may assign its own fields (`this.NAME = ...`) when NAME is not one of
  its members. The step is `state-effect` when any call fills, types, presses, checks, selects, uploads,
  clears, drags or submits; issues a request (`fetch(`, or an HTTP verb on a `request` receiver);
  or writes storage or cookies (`setItem`, `removeItem`, `addCookies`, `clearCookies`). Otherwise
  it is `unknown` when any call is not provable: a navigation, wait, read or click whose receiver is
  not the Playwright page itself (`page`, or `this.page` in a page object, in a file that never
  rebinds `page` and assigns `this.page` only `page`), a click, double-click or tap whose receiver
  is not `getByRole('link', ...)` with exactly the literal `'link'` as its first argument
  (optionally narrowed by `first`, `last` or `nth`), `evaluate`, `route`,
  any unlisted method or bare helper (on `JSON`, `Math`, `Number`, `String`, `Object`, `Array`,
  `Promise`, `Date` and `console` only listed non-mutating members are admitted), a computed,
  optional, non-null asserted (`f!(`), chained, generic (`f<T>(`) or tagged-template invocation or
  receiver, an assignment,
  increment or decrement of a member, `delete`, a missing, duplicated or recursive member, a
  member whose one-line header gives a parameter a default or holds a template substitution
  (`default-parameter`), a `test.step`, `expect.poll`, `toPass`, `toThrow` or `toThrowError`
  whose callback argument is not wholly a function written in place, so `function() {} && reset`
  is not one (`callback`), a template substitution that
  may call, assign, update or delete, or source the lexer cannot bound.
  Otherwise it is `navigation-only`: navigation (`goto`, `reload`, `goBack`, `goForward`), waits,
  locator construction, reads and `expect` assertions only; `hover`, `focus` and scrolling can run
  application handlers and are `unknown`. A non-navigation step records the
  deciding call's `effect_ref` (`PATH:LINE`) and `effect_call`; every step records whole-file anchors
  (`effect_from`) for the spec and each page-object file read, since imports, bindings and other
  members decide which methods are read. Other edges and older maps carry none of these fields.
  (accepted, decision 0520; V1-1164)
- `ANL-V0-036`: `test-plan prefixes` treats a derived `navigation-only` step as read-only without a
  sidecar declaration, but only when the edge span and every `effect_from` anchor are FRESH at the
  evaluated revision (otherwise `step-effect-not-fresh`). A `state-effect` step abstains with
  `step-state-effect` and an `unknown` step with `step-effect-unknown`; each names the step
  (`edge`), the deciding call (`ref`, `call`). A `read_only_edges` declaration never upgrades a
  derived `state-effect` or `unknown` step; declared assertion targets and mutations still abstain
  as ANL-V0-010 states. An edge without a derived effect keeps the ANL-V0-010 sidecar rule.
  (accepted, decision 0520; V1-1164)
- `ANL-V0-037`: Corvint ships a navigation-events producer as the repository file
  `internal/appmap/navigation-events.cjs` (AGPL, like the other `internal/` reporters). A suite
  enables it with one line in its Playwright config,
  `require('<path>/navigation-events.cjs').install({outputDir: 'DIR'})`, and changes no spec, page
  object or fixture source. `install` and `createRecorder` without an `outputDir` throw at config
  load. (proposed; issue #814)
- `ANL-V0-038`: The config is loaded by the runner and by every worker; in each, `install` resolves
  the suite's own `playwright` and `playwright-core` from the working directory and adds one
  listener to Playwright's client instrumentation. It needs no trace, screenshot, video or reporter
  setting and is unaffected by them. When Playwright cannot be resolved or the instrumentation hook
  is absent, it writes one stderr line and records nothing, so no document is ever partial.
  Recording is per test attempt (`testInfo.testId`, `retry`); a context created outside a test
  records nothing. (proposed; issue #814)
- `ANL-V0-039`: When a test's browser context closes (the before-close hook, or the context `close`
  event when `Browser.close()` or a crash closes it), the producer writes once one closed
  `playwright-navigation-events/0` document as one JSON line to `DIR/<testId>.json` (through a
  temporary file and rename), with `test_key` set to Playwright's stable test ID. A later attempt
  of the same test removes the earlier attempt's file when its first context opens.
  `observe --trace` accepts each file, and `DIR` itself while it holds at most 64 documents (the
  ANL-V0-006 directory limit); a larger run is observed file by file.
  The test key is not a qualified receipt test key; binding observations to receipts stays the
  caller's mapping, as for `--test-key` (ANL-V0-022). (proposed; issue #814)
- `ANL-V0-040`: Each URL commit of the test page's main frame (`framenavigated`) is one event,
  except an `about:` URL or a URL equal to the previous commit. The event carries an action only
  when a Playwright call is in flight (begun and not ended) and has not yet been credited with a
  commit: the most recently begun such call, as `kind` (its protocol method, the same name a trace
  `before` record carries, and only when it matches `^[A-Za-z][A-Za-z0-9]{0,63}$`) and `selector`
  (only a string of at most 1024 bytes). Any other commit carries no action and so reads `UNKNOWN`.
  Sub-frame commits are ignored. (proposed; issue #814)
- `ANL-V0-041`: When the committed document's main-frame navigation request was reached through
  HTTP redirects (its `redirectedFrom` chain, matched to the commit URL without the fragment), the
  first hop is the `navigate` event that carries the action and every later hop and the final URL
  are `redirect` events. No other commit is labelled `redirect`. (proposed; issue #814)
- `ANL-V0-042`: On the same run, `observe` over the producer's file yields the same visits and
  edges as `observe --trace ZIP --test-key KEY` over that test's trace with the same key, once each
  redirect chain is folded to its final URL carrying the first hop's action; a run without HTTP
  redirects needs no folding. The producer is a superset where snapshots are blind: it shows
  redirect hops, and a call that commits more than one URL (a client-side redirect) yields one
  event per commit where the trace shows only the last. (proposed; issue #814)
- `ANL-V0-043`: A test attempt that opens a second page (a popup or another context), records more
  than 4096 events, commits a URL longer than 4096 bytes, or whose document would exceed the 4 MiB
  reporter file limit gets no document: the producer removes
  any earlier file for that test and writes one stderr line naming the test and reason, mirroring the
  ANL-V0-022 refusals instead of truncating. (proposed; issue #814)
- `ANL-V0-044`: The producer reads only main-frame URLs, navigation request redirect chains and each
  call's protocol method and selector. It never records typed or filled values, other call
  parameters, page content, headers or bodies. Before an event URL is kept it applies the ANL-V0-007
  redaction: URL credentials are dropped, and a query value (also inside a hash route) whose name
  is secret-shaped or whose value looks opaque, or which cannot be decoded, becomes `REDACTED`; no
  other byte of the URL changes. `observe` redacts again with the same rule. (proposed; issue #814)
- `ANL-V0-045`: Its cost is one in-process listener call per Playwright call, two page event
  listeners and one small file write per test; it opens no network, process or browser. The
  measured overhead and the qualified Playwright version are recorded in the build log. It uses
  Playwright's private client instrumentation hook, qualified on Playwright 1.61.1 only; another
  version needs a fresh live parity run before its output is trusted. (proposed; issue #814)
- `ANL-V0-046`: The runner process (where Playwright has not set `TEST_WORKER_INDEX`) loads the
  config once before any worker starts; there `install` removes from `DIR`, not recursively, every
  regular `*.json` file that begins with `{"schema":"playwright-navigation-events/0"` and every
  `*.json.tmp` file, so `DIR` holds only the current run's documents. Other files are never
  removed. Any command that loads the config in the runner, including `--list`, clears `DIR`.
  (proposed; issue #814)

## Wire and limits

`corvint flows appmap edges --map FILE [--receipt FILE]... [--revision REV]
[--declarations FILE] [--require EDGE_ID]... [--observation FILE]... [--budget N | --full]`

Sidecar: `{"schema":"application-edge-declarations/0","tests":[{"test_key":"KEY",
"proves":["edge:HEX"],"assumes":["edge:HEX"]}]}`. At most 64 test declarations, 64 IDs per
list, 64 command requirements, 4096 adjacent pairs and 256 distinct test keys per pair.
Sidecars use the RVN regular-file reader capped at 4 MiB; receipts retain RVN's 16-file limit.
Default output cap is 6144 bytes. Counts cover complete lists even when rows are omitted.
Reused errors: `appmap-invalid-query`, `appmap-budget-too-small` and RVN receipt errors.
Ledger-internal verification reasons: `endpoint-unverified` (an endpoint lacks proof),
`crossing-evidence-mismatch` (endpoint proofs differ in receipt or revision), and
`flow-anchor-unverified` (flow freshness cannot support the receipt). These prevent PROVEN;
this slice reports the aggregate UNPROVEN state without projecting every rejected crossing.
Map/receipt digests are identity, not authenticity. Binding remains owner-labelled run evidence;
UNKNOWN trigger proof means the declared screen adjacency only. No claim of observed step ordering.

## Implementation plan and rollback

Add `internal/appmap/ledger.go`, a CLI file and focused tests; minimal dispatch/help and exit-code
hooks. Reuse the private RVN verifier and freshness cache. Run focused baseline, implementation
tests, vet and requested doc gates; independent review before handoff. Remove these additive hooks
and files to roll back; no store or map-format migration. Keep requirement IDs stable.
The navigation-events producer (ANL-V0-037..046) is one CommonJS file, its Node unit test and
fixtures; remove them and the `observe` help sentence to roll back. Suites remove their one config
line; the closed `playwright-navigation-events/0` format is unchanged.

## Traceability

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| ANL-V0-001 | `internal/appmap/ledger.go` | `TestANLV0001Identity` |
| ANL-V0-002 | `internal/appmap/ledger.go` | `TestANLV0002EvidenceAndDrift`, `TestANLV0002MixedEvidenceFailsClosed` |
| ANL-V0-003 | `internal/appmap/ledger.go` | `TestANLV0003Redundancy`, `TestANLV0003SameKeyUncertaintyBlocks` |
| ANL-V0-004 | `cmd/corvint/flows_appmap_edges.go` | `TestANLV0004EdgesCLI` |
| ANL-V0-005 | `internal/appmap/ledger.go` | `TestANLV0005Budgets` |
| ANL-V0-006 | `internal/appmap/observe.go`, `cmd/corvint/flows_appmap_observe.go` | `TestANLV0006Observe`, `TestANLV0006HashRouted`, `TestANLV0006ObserveCLI` |
| ANL-V0-007 | `internal/appmap/observe.go` | `TestANLV0007Redaction` |
| ANL-V0-008 | `internal/appmap/observe.go` | `TestANLV0008InputsAndBudget` |
| ANL-V0-009 | `internal/appmap/prefixes.go` | `TestANLV0009FlagsProvenPrefix` |
| ANL-V0-010 | `internal/appmap/prefixes.go` | `TestANLV0010NeverFlagsTargetsOrChanges`, `TestANLV0010RefusesDuplicateFactsAndForeignLedger` |
| ANL-V0-011 | `internal/appmap/prefixes.go`, `cmd/corvint/test_plan_prefixes.go` | `TestANLV0011StaleEntryAndBudget`, `TestANLV0011PrefixesCLI` |
| ANL-V0-012 | `internal/testplan/pagestate.go` | `TestANLV0012GroupsSharedPageState`, `TestANLV0012RequiredSetsStayDistinct` |
| ANL-V0-013 | `internal/testplan/pagestate.go` | `TestANLV0013BlocksAndSeparates` |
| ANL-V0-014 | `internal/testplan/pagestate.go`, `cmd/corvint/test_plan_pagestate.go` | `TestANLV0012GroupsSharedPageState`, `TestANLV0014PageStateCLI` |
| ANL-V0-015 | `internal/appmap/impact.go`, `cmd/corvint/flows_appmap_impact.go` | `TestANLV0015RouterChangeReachesDescendants`, `TestANLV0015ImpactCLI` |
| ANL-V0-016 | `internal/appmap/impact.go` | `TestANLV0016UnknownForcesFullSuite` |
| ANL-V0-021 | `internal/appmap/observe.go` | `TestANLV0021TypedTriggersAndAnnotations` |
| ANL-V0-022 | `internal/appmap/tracezip.go`, `cmd/corvint/flows_appmap_observe.go` | `TestANLV0022TraceZip`, `TestANLV0006ObserveCLI` |
| ANL-V0-023 | `internal/appmap/ledger.go`, `cmd/corvint/flows_appmap_edges.go` | `TestANLV0023ObservedEdgesFeedLedger`, `TestANLV0023EdgesObservationCLI` |
| ANL-V0-024 | `internal/appmap/ledger.go` | `TestANLV0024SelfProofExcluded`, `TestANLV0004EdgesCLI` |
| ANL-V0-031 | `internal/appmap/impact.go`, `cmd/corvint/affected_appmap.go` | `TestANLV0031AffectedRoutesImpactVerdict` |
| ANL-V0-032 | `internal/appmap/impacthop.go`, `internal/appmap/impact.go` | `TestANLV0032TemplateHopPlacesReferencedFiles` |
| ANL-V0-033 | `internal/testplan/pagestate.go`, `cmd/corvint/test_plan_pagestate.go` | `TestANLV0033PageStateDrift` |
| ANL-V0-034 | `cmd/corvint/affected_appmap.go` | `TestANLV0034AffectedAggregatesMapsPerApp` |
| ANL-V0-035 | `internal/appmap/effects.go`, `internal/appmap/compile.go` | `TestANLV0035StepEffects` |
| ANL-V0-036 | `internal/appmap/prefixes.go` | `TestANLV0036PrefixesUseStepEffects` |
| ANL-V0-037 | `internal/appmap/navigation-events.cjs` | `TestANLV0037ProducerNodeUnit` |
| ANL-V0-038 | `internal/appmap/navigation-events.cjs` | `TestANLV0037ProducerNodeUnit`, `TestANLV0039ObserveAcceptsProducerOutput` |
| ANL-V0-039 | `internal/appmap/navigation-events.cjs` | `TestANLV0039ObserveAcceptsProducerOutput`, `TestANLV0037ProducerNodeUnit` |
| ANL-V0-040 | `internal/appmap/navigation-events.cjs` | `TestANLV0037ProducerNodeUnit`, `TestANLV0039ObserveAcceptsProducerOutput` |
| ANL-V0-041 | `internal/appmap/navigation-events.cjs` | `TestANLV0037ProducerNodeUnit`, `TestANLV0042LiveParityWithTraceZip` |
| ANL-V0-042 | `internal/appmap/navigation-events.cjs` | `TestANLV0042LiveParityWithTraceZip` |
| ANL-V0-043 | `internal/appmap/navigation-events.cjs` | `TestANLV0037ProducerNodeUnit` |
| ANL-V0-044 | `internal/appmap/navigation-events.cjs` | `TestANLV0037ProducerNodeUnit`, `TestANLV0039ObserveAcceptsProducerOutput` |
| ANL-V0-045 | `docs/build-log/2026-10-11-navigation-events-producer.md` | `TestANLV0042LiveParityWithTraceZip` |
| ANL-V0-046 | `internal/appmap/navigation-events.cjs` | `TestANLV0037ProducerNodeUnit` |

## Follow-ups and promotion

Proposed follow-ups: typed trigger declarations in map flow steps and spec annotation parsing
(ANL-V0-021 types observed triggers and reporter annotations only); component/template
file and page-object anchors absent from AMAP; multi-map per-app aggregation; costs beyond declared
step counts; test-plan integration; live adopter qualification. UNKNOWN trigger is not a typed
trigger acceptance claim. Promotion needs owner acceptance plus demonstrated useful reuse on an
owner-controlled suite; synthetic checks alone retain experimental status. Kill or revise the
projection if declared adjacency cannot safely support that suite. Live qualification is NOT_RUN,
except that the ANL-V0-037..046 producer ran live on a local two-test Playwright 1.61.1 suite
(`docs/build-log/2026-10-11-navigation-events-producer.md`); an adopter suite run remains NOT_RUN.
