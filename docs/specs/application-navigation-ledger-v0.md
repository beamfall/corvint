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
- Exists: smallest single-map CLI slice; synthetic receipts only; static page-object step effects for prefixes (ANL-V0-035/036, V1-1164, decision 0520); a config-installed Playwright navigation-events producer live-checked on Playwright 1.61.1 (ANL-V0-037..046, issue #814); two-repository join (ANL-V0-060..063, V1-2698/V1-2701): `affected --appmap-impact` binds an application-repository report to a declared `--repository` checkout and records `bindings`; unmapped or ignored application paths read `full-suite-required`; two-root `impact --app-root --test-root [--at] [--test-at]` writes `impact-two-root/0` with both pins and never an empty selection for a change; reports pin their test root and `affected` checks it against HEAD and rechecks bound checkouts (ANL-V0-064..067); MCP `corvint.impact` stays single-root; the reference hop skips over-bound files as named unknowns, ignores identifiers and follows AngularJS DI registrations, and each unknown names its file and line or bound (ANL-V0-069..072).
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
  base or revision fails closed. (accepted, decision 0518; V1-1137) ANL-V0-048 amends the
  non-UI case: a changed non-UI path is unknown too, never ignored, so `no-navigation-impact`
  needs every changed path placed.

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
  to disk. Each root entry named `trace.trace` or `N-trace.trace` is decompressed, streamed
  as NDJSON within 64 MiB and 16 MiB per record; nested, `..` or other entries are never opened as
  the trace, and an invalid record (including one without a `type`), no main-frame snapshot in any
  context, or more than 4096 navigations refuse with exit 2. Several contexts and pages are read as
  ANL-V0-068 states (amended by ANL-V0-068; earlier two context traces or two pages refused). Each change of
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
  0519 (V1-1162)) Amended by ANL-V0-069 (an over-bound file is a named, scanned skip), ANL-V0-070
  (identifiers and non-path markup words are not mentions), ANL-V0-071 (AngularJS DI registrations
  are followed) and ANL-V0-072 (each unknown names its file and line or bound).
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
- `ANL-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) Corvint ships a navigation-events producer as the repository file
  `internal/appmap/navigation-events.cjs` (Apache-2.0, like the rest of the repository). A suite
  enables it with one line in its Playwright config,
  `require('<path>/navigation-events.cjs').install({outputDir: 'DIR'})`, and changes no spec, page
  object or fixture source. `install` and `createRecorder` without an `outputDir` throw at config
  load.
- `ANL-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) The config is loaded by the runner and by every worker; in each, `install` resolves
  the suite's own `playwright` and `playwright-core` from the working directory and adds one
  listener to Playwright's client instrumentation. It needs no trace, screenshot, video or reporter
  setting and is unaffected by them. When Playwright cannot be resolved or the instrumentation hook
  is absent, it writes one stderr line and records nothing, so no document is ever partial.
  Recording is per test attempt (`testInfo.testId`, `retry`); a context created outside a test
  records nothing.
- `ANL-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) When a test's browser context closes (the before-close hook, or the context `close`
  event when `Browser.close()` or a crash closes it), the producer writes once one closed
  `playwright-navigation-events/0` document as one JSON line to `DIR/<testId>.json` (through a
  temporary file and rename), with `test_key` set to Playwright's stable test ID. A later attempt
  of the same test removes the earlier attempt's file when its first context opens.
  `observe --trace` accepts each file, and `DIR` itself while it holds at most 64 documents (the
  ANL-V0-006 directory limit); a larger run is observed file by file.
  The test key is not a qualified receipt test key; binding observations to receipts stays the
  caller's mapping, as for `--test-key` (ANL-V0-022).
- `ANL-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) Each URL commit of the test page's main frame (`framenavigated`) is one event,
  except an `about:` URL or a URL equal to the previous commit. The event carries an action only
  when a Playwright call is in flight (begun and not ended) and has not yet been credited with a
  commit: the most recently begun such call, as `kind` (its protocol method, the same name a trace
  `before` record carries, and only when it matches `^[A-Za-z][A-Za-z0-9]{0,63}$`) and `selector`
  (only a string of at most 1024 bytes). Any other commit carries no action and so reads `UNKNOWN`.
  Sub-frame commits are ignored.
- `ANL-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) When the committed document's main-frame navigation request was reached through
  HTTP redirects (its `redirectedFrom` chain, matched to the commit URL without the fragment), the
  first hop is the `navigate` event that carries the action and every later hop and the final URL
  are `redirect` events. No other commit is labelled `redirect`.
- `ANL-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) On the same run, `observe` over the producer's file yields the same visits and
  edges as `observe --trace ZIP --test-key KEY` over that test's trace with the same key, once each
  redirect chain is folded to its final URL carrying the first hop's action; a run without HTTP
  redirects needs no folding. The producer is a superset where snapshots are blind: it shows
  redirect hops, and a call that commits more than one URL (a client-side redirect) yields one
  event per commit where the trace shows only the last.
- `ANL-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) A test attempt that opens a second page (a popup or another context), closes a
  second browser context after its first, records more
  than 4096 events, commits a URL or redirect hop longer than 4096 bytes as committed (the
  ANL-V0-022 bound, checked before redaction) or after redaction, or whose document would exceed the 4 MiB
  reporter file limit gets no document: the producer removes
  any earlier file for that test and writes one stderr line naming the test and reason, mirroring the
  ANL-V0-022 refusals instead of truncating.
- `ANL-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) The producer reads only main-frame URLs, navigation request redirect chains and each
  call's protocol method and selector. It never records typed or filled values, other call
  parameters, page content, headers or bodies. Before an event URL is kept it applies the ANL-V0-007
  redaction as `observe` parses the URL (byte-wise unescaping, Unicode simple case folding): URL
  credentials are dropped; in the URL's query a pair holding `;` or a bad escape is dropped, and a
  value whose decoded name is secret-shaped or whose decoded value looks opaque becomes `REDACTED`.
  A hash-route query is decoded once with the fragment, then redacted the same way, except that any
  bad pair replaces the whole fragment query with `_=REDACTED`. An unchanged fragment keeps its
  original bytes, as Go's escaped fragment does; a changed one keeps its route bytes up to the first
  literal `?` and rewrites only the query so one unescape returns it. A route whose first decoded
  `?` is not its first literal `?` cannot keep its bytes, and the attempt is withheld with a
  warning. When `observe` cannot parse the URL (a bad escape in the path or fragment), it writes
  the visit as `REDACTED` but matches the screen on the raw path and hash route, so the producer
  keeps only the path, the query's `?` if present, and the hash route up to its first `?` or `#`,
  followed by `?%` so the URL stays unparseable; such a route holding an encoded `?` is withheld.
  `observe` then decodes the
  same URL from the written URL as from the committed URL and resolves the same screen and status.
- `ANL-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) Its cost is one in-process listener call per Playwright call, two page event
  listeners and one small file write per test; it opens no network, process or browser. A written
  or withheld attempt keeps only its retry and page count, not its events. The
  measured overhead and the qualified Playwright version are recorded in the build log. It uses
  Playwright's private client instrumentation hook, qualified on Playwright 1.61.1 only; another
  version needs a fresh live parity run before its output is trusted.
- `ANL-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11); issue #814) The runner process (where Playwright has not set `TEST_WORKER_INDEX`) loads the
  config once before any worker starts; there `install` removes from `DIR`, not recursively, every
  regular `*.json` file that begins with `{"schema":"playwright-navigation-events/0"`, every
  regular `*.json.corvint-navigation-events.tmp` file (the private suffix the producer writes its
  temporary file under) and every regular `*.json.tmp` file that begins with that same prefix (an
  earlier producer's temporary file), so `DIR` holds only the current run's documents. Other files,
  including another tool's `*.json.tmp`, are never removed (V1-3025). Any command that loads the config in the runner, including `--list`, clears `DIR`.
- `ANL-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1216) For an index
  file, an occurrence of the `index` token that the `ANL-V0-032` JavaScript lexer leaves in code
  position (an identifier, property or object key such as a loop `index`, `row.index` or
  `{ index: 0 }`) in a confidently lexed script is not a mention, because `index` is a path segment
  and never the binding an import names. Such a file no longer makes an index-file change
  `unresolved-reference`. The directory-name token, an `index` inside a comment, string, template or
  regular expression, any mention in a file the lexer does not read or doubts, and every other
  `ANL-V0-032` rule are unchanged, so the hop still never narrows on an unprovable mention.
- `ANL-V0-048`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2697, issue #1803)
  `flows appmap impact` fails closed: every changed path is placed by a map element (ANL-V0-015,
  ANL-V0-032, ANL-V0-049) or listed under `unknown` with a reason. A changed path that is not a UI
  path (a Ruby model or worker, a server file, configuration, documentation) reads unknown
  `non-ui-path`, so a change no map element places can never read `targeted` or
  `no-navigation-impact`. `ignored_paths` is reserved for paths an explicit declared exemption
  covers; the map declares no exemption, so it reads `0`. The verdict rule is unchanged: any
  unknown forces `full-suite-required`. Failure mode: a server-only change that could break any
  screen read `no-navigation-impact` and skipped every browser test. Non-goal: an exemption
  declaration, which needs its own owner-declared contract.
- `ANL-V0-049`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2697, issue #1803)
  A changed test file the map records that is not a spec (a page object, workflow, scenario or
  helper) selects every spec that imports it, directly or through other test files, by walking the
  map's resolved test imports backwards; a bound page object also keeps its screen. The path is
  unknown `test-import-graph-incomplete` when the primary test import graph may hold an edge the
  map did not record (a test file `excluded-by-index` or with `unparsed-imports`, an
  `unresolved-import` including a computed load, an `import-outside-tests`, or a resolved code
  module such as `.mts`/`.cts` the map records no test file for), and unknown `test-support-unreached` when the
  walk finds no spec. A changed UI path whose ANL-V0-032 reference hop ends at a test file that
  is not a spec walks that file's importers the same way and reads `test-import-graph-incomplete`
  under the same condition. Failure mode: a page-object-only change selected no spec and read
  `no-navigation-impact`. Non-goal: reading imports the index does not record.

The 2026-10-25 two-repository lane (V1-2698 and V1-2701; GitHub #1804, #1808) adds ANL-V0-060..063,
proposed and experimental under the standing lane auto-accept (2026-10-11). They change only the
`affected` join and the `impact` argument contract; the impact verdict logic in
`internal/appmap/impact.go` is unchanged. A follow-up the same day (V1-2701, #1808 items 1 and 2)
adds ANL-V0-064..067 under the same auto-accept: a two-root `impact` over committed objects, at
`HEAD` or a stated revision, and a test-root pin on the report that `affected` checks. Non-goals:
any narrowing of `affected` checks or plan; a two-root MCP `corvint.impact` (MMR-V0-005 lets a tool
call reach only the repository its `repository` alias names, so a cross-root call needs an owner
amendment of MMR-V0; the MCP tool keeps its single-root contract; owner question V1-2718); covering uncommitted paths in
either root (they fail closed or refuse instead).

- `ANL-V0-060`: (proposed; V1-2698, #1804) `affected --base FULL_COMMIT_ID --appmap-impact REPORT`
  MUST accept, with or without `--provider`, `--repository ID=DIR` paired with exactly one
  `--repository-base ID=FULL_COMMIT_ID` (the `e2e-safe` pairing rule, AFU-V1-067). A report is
  checked first against each declared repository whose `--repository-base` is the report's base,
  then against `--base` and `HEAD`, so a clean `--root` sharing both commits never hides a declared
  checkout's state: DIR (relative to `--root` unless absolute) must be the top level of a Git
  checkout with trusted ancestry, that base must be one of its commits, its `HEAD` must be the
  report's head, and a `HEAD` that moves while its status is read refuses. Each `app_navigation` row records `bindings`, one per report:
  `base`, `head`, `impact_digest` and `repository` (absent when the report was checked against
  `--root`). A report bound to neither refuses `appmap-invalid-query` and names the report's range
  and the supported layout `--repository ID=DIR --repository-base ID=FULL_COMMIT`, with the reason
  each candidate checkout failed. Failure mode: before this, an impact report built in the
  application repository could never be joined to the test repository's plan (GitHub #1804).
  Acceptance: `TestANLV0060AffectedBindsReportToDeclaredRepository`.
- `ANL-V0-061`: (proposed; V1-2698) An uncommitted path in a bound repository checkout makes the
  row `full-suite-required` with reason `repository-worktree-not-covered`, and a report that
  counts ignored paths (`ignored_paths` > 0) makes it `full-suite-required` with reason
  `impact-ignored-paths`, each also an `APP_NAVIGATION_FULL_SUITE_REQUIRED` unknown; the existing
  `worktree-not-covered` rule for `--root` still applies. Acceptance:
  `TestANLV0060AffectedBindsReportToDeclaredRepository`.
- `ANL-V0-062`: (proposed; V1-2701, #1808 item 3) Documented guarantee: an application path that no
  map element places, or that the report does not walk to a screen, MUST yield `UNKNOWN`
  (`impact-unknown-paths`) or the `full-suite-required` fallback on the `affected` row, never
  `no-navigation` and never an empty spec list read as "no tests affected", in the single-root
  and the two-repository layout alike. ANL-V0-015/016 give the report-side half; ANL-V0-061's
  `impact-ignored-paths` closes the join-side half independent of how the report classifies a
  non-UI path. Acceptance: `TestANLV0060AffectedBindsReportToDeclaredRepository` (a `.go` and a
  `.txt` application change behind a targeted router change read `full-suite-required`).
- `ANL-V0-063`: (proposed; V1-2701, #1808 items 1 and 2; amended by ANL-V0-064) Single-root
  `corvint impact` reads one `--root` at its `HEAD`; there `--at` and `--revision`, and an
  `--app-root` or `--test-root` beside `--root`, MUST refuse with the typed code
  `unsupported-impact-layout` before reading the repository, naming the two-root `impact` usage
  (ANL-V0-064) and the `affected --appmap-impact --repository/--repository-base` join. Acceptance:
  `TestANLV0063ImpactRefusesUnsupportedLayout`. Rollback for ANL-V0-060..063: remove the binding
  helper, the `bindings` member, the two reasons and the refused option names; reports and maps
  are unchanged and no state migrates.
- `ANL-V0-064`: (proposed; V1-2701, #1808 item 1) `corvint impact --app-root DIR --test-root
  ALIAS=ABSOLUTE_ROOT --manifest FILE --base COMMIT [--manifest-repo ALIAS] [--at REV] [--test-at
  REV] [--ledger FILE]` MUST build the application map with routers and flows from DIR (the top
  level of a Git checkout with trusted ancestry) and the tests from the aliased root whose alias the
  manifest's `tests.repo` names (AMAP-V0-017/018), take the diff `--base..--at` (default `HEAD`) in
  DIR, and write one `impact-two-root/0` JSON document: `app`, `authority` `learned`, `app_root`
  (`base`, `head`, `stated`, `dirty_paths`), `test_root` (the map's pin: `repo`, `revision`,
  `inputs`, `stated`), `map_digest`, `impact_digest`, `changed_paths`, `ignored_paths`,
  `reported_verdict`, `verdict`, `reasons`, `spec_files` and `test_keys` (test-repository paths and
  keys) and `unknown`. It reads committed objects only and writes nothing else. It refuses
  `unsupported-impact-layout` for a missing `--app-root`, `--test-root`, `--manifest` or `--base`,
  an unaliased `--test-root`, a `--manifest-repo` other than the test alias, any `--root`,
  `--revision` or PATH, and a manifest whose `tests.repo` is not the test alias; an alias the
  manifest does not name refuses `appmap-root-unavailable`. Failure mode: before this, `impact`
  had no two-root answer and the adopter had to chain three commands by hand (GitHub #1808).
  Acceptance: `TestANLV0064TwoRootImpact`, `TestANLV0064TwoRootImpactRefusesInvalidLayout`.
- `ANL-V0-065`: (proposed; V1-2701, #1808 item 3) In `impact-two-root/0`, `verdict` MUST be
  `full-suite-required`, with an `APP_NAVIGATION_FULL_SUITE_REQUIRED: APP: REASONS` line in
  `unknown`, when the report lists unknown paths (`impact-unknown-paths`, each also an
  `UNBOUND_APPLICATION_PATH: PATH: REASON` line), counts ignored paths (`impact-ignored-paths`),
  omitted a spec file (`spec-files-over-bound`), when the application root read at `HEAD` has
  uncommitted paths (`app-worktree-not-covered`), or when a change (`changed_paths` > 0) selected
  no spec file (`change-selected-no-spec-files`). An empty `spec_files` with no reason therefore
  means only that the application diff is empty. Acceptance: `TestANLV0064TwoRootImpact`,
  `TestANLV0065TwoRootImpactNeverEmptyForAChange`.
- `ANL-V0-066`: (proposed; V1-2701, #1808 item 2) `--at REV` reads the application at REV and
  `--test-at REV` the test root at REV (AMAP-V0-200) from committed objects only, marking each pin
  `stated`, so the answer does not depend on either working tree; a revision that is not a commit
  refuses typed (`appmap-invalid-revision`, `appmap-root-unavailable`). Without `--test-at` a test
  root with uncommitted manifest-declared paths refuses `appmap-root-dirty`. `--at` is resolved to
  one commit before the map and the diff read it. After the report is computed, every root read at
  `HEAD` is read again: an application `HEAD` or status, a test-root `HEAD` or status, or a map or
  diff revision other than the resolved `--at`, that differs from what the map and diff used
  refuses `impact-root-drift` with no output. Likewise `affected` reads every bound `--repository`
  checkout again after its final `HEAD` recheck and refuses `unsupported-affected-drift` if one
  moved or its dirty path set changed. Acceptance: `TestANLV0066TwoRootImpactAtRevision`,
  `TestANLV0066TwoRootImpactDriftRecheck`, `TestANLV0067AffectedChecksTestRootPin`.
- `ANL-V0-067`: (proposed; V1-2701) An `application-map-impact/0` report built from a map with an
  aliased tests root MUST carry `test_root`, a copy of that map pin, inside the digested document;
  single-root reports omit it and keep their bytes. `affected` fails the app's row closed to
  `full-suite-required` with reason `test-root-not-at-head` when a report's `test_root.revision`
  is not the receipt's `HEAD` (the test repository it runs in), so spec files are never joined
  against a test tree other than the one they were read from. Acceptance:
  `TestANLV0067AffectedChecksTestRootPin`. Rollback for ANL-V0-064..067: remove
  `cmd/corvint/impact_tworoot.go`, restore the `runEvalInvocation` call in `cmd/corvint/main.go`,
  drop `ImpactReport.TestRoot`, the reason and the checkout recheck; no state migrates. Live
  qualification on an adopter's two repositories: `NOT_RUN`.
- `ANL-V0-068`: (proposed; V1-1202, GH #1852) A trace zip MAY hold up to 16 context traces
  (`trace.trace` and `N-trace.trace`; more, or two entries with one name, refuse with exit 2).
  Every context is read, the root first and then by the value of N, and every call ID and page ID
  is keyed by its context entry, so an action in one context never attributes a navigation in
  another; an action is attributed only when its `before` snapshot is on the same page. Each page with a main-frame snapshot yields one
  observation trace, ordered by context then first snapshot, sharing the archive digest and the
  caller's test key; with more than one page each trace carries `page` (the prefixed page ID),
  while a single-page zip omits it and keeps its earlier bytes. Contexts with no page (an API
  request context) contribute nothing. The 4096-navigation bound covers all pages together, and
  more than 64 pages refuse. Acceptance: `TestANLV0068MultiContextTraceZip`,
  `TestANLV0068TraceZipKeysAndBounds` on a hand-built Playwright 1.64-shaped fixture
  (`internal/appmap/testdata/playwright-1.64-shape`). Rollback: restore the single-context reader
  and drop `ObservedTrace.Page`; no state migrates. Qualification on a real Playwright 1.64 trace:
  `NOT_RUN`.
- `ANL-V0-069`: (proposed under the owner's standing auto-accept (2026-10-11); V1-2775, GH #1875)
  A tracked UI-extension file over 1 MiB, or one that would carry the files read in path order past
  the 32 MiB total, is not loaded into the `ANL-V0-032` graph; the rest of the graph is still used.
  Each such file is scanned (at most 64 MiB per file and 256 MiB in all) for its distinct maximal
  runs of name bytes (`[A-Za-z0-9_$-]`, at most 262144 runs of at most 256 bytes). A walk to a file
  whose path tokens or DI names (ANL-V0-071) the skipped file may hold, meaning every name part of
  one token is one of its runs, reads unknown `reference-graph-bound` with a detail naming the
  skipped file and the bound (`F exceeds 1048576 bytes`, `F exceeds the 33554432-byte reference
  graph total`). A skipped file that provably holds none of them leaves the report unchanged. A
  skipped file that could not be scanned, or exceeded the run bound, may hold anything. A changed
  script that is itself skipped reads that bound, since its registrations cannot be read. The file
  count bound reads `more than 20000 tracked files` and the walked-node bound
  `more than 64 walked files (next F)`. Non-goals: reading oversized files as references or raising
  the bounds. Failure mode: a skipped file that does mention the path but was scanned is still
  never trusted, so the path stays unknown; the hop never narrows on a skipped file.
- `ANL-V0-070`: (proposed under the owner's standing auto-accept (2026-10-11); V1-2776, GH #1876)
  An occurrence of a path token that the `ANL-V0-032` lexer leaves in code position in a confidently
  lexed script (an identifier, property or object key such as `var format`, `x.format()` or
  `{ format: 1 }`) is not a mention, except inside the argument span of any `require(` or `import(`
  call, matched by bracket depth (an unclosed call runs to the end of the file), which refuses as a
  computed load; no lookback bound applies. In a file the lexer does
  not read (markup, styles, `.vue`, `.svelte`), an occurrence of a script's stem is not a mention
  unless it may name a path: beside `/` or `\`, filling a whole quoted value, followed by a script
  extension, or on a line holding `require(` or `import`. Every occurrence of a non-script target's
  base name, an index file's `index` in markup, any mention in a comment, string, template or
  regex, any mention in a doubted script and every other `ANL-V0-032` rule are unchanged.
  Failure mode accepted: scripts coupled only through global identifiers (concatenated bundles
  without imports) are not followed; such a walk usually reads `unmapped-path` and the path stays
  unknown.
- `ANL-V0-071`: (proposed under the owner's standing auto-accept (2026-10-11); V1-2776, GH #1876)
  When the hop walks a script, each `.provider`, `.factory`, `.service`, `.value`, `.constant`,
  `.decorator`, `.animation`, `.filter`, `.controller`, `.directive` or `.component` call in code
  position registers its first argument, which MUST be exactly one single-line string literal the
  lexer delimits: the next code byte after its closing quote is `,` or `)`, so `'Club' + 'Service'`
  is not a name. A first argument that is a function or arrow function (`rows.filter(fn)`-style
  array methods) registers nothing. Each registered name yields DI tokens: the name, a provider's
  `NameProvider`, a filter's `NameFilter`, and an animation's class without its leading dot. A
  directive or component also matches every maximal markup run of `[A-Za-z0-9_$:-]` that AngularJS
  normalization maps to its name: lower-cased, one leading `x` or `data` prefix with a `:`, `-` or
  `_` separator stripped (the unstripped form is also tried), and camel-cased across every run of
  those separators, so `data_my_dir`, `x:my-dir` and `MY-DIR` all match `myDir`. A skipped file
  (ANL-V0-069) may hold such a spelling when every lower-cased camel word of the name lies inside
  one of its runs. Every bounded occurrence of a DI token in another read file (a DI array
  annotation, `$inject`, an implicit parameter, a router state's `controller: 'Name'`, a markup
  filter pipe `| name`, tag or attribute, or a comment) references the script at that line,
  whatever the markup no-mention rule of ANL-V0-070 says about path tokens, so the hop may widen
  through it but never narrows. A registration whose first argument is any other expression (a
  variable, a concatenation, an object map), or any registration call in a doubted or unlexed
  script, reads `unresolved-reference`. This applies
  to every script, not only ui-router maps, because the map does not record router dialects.
  Non-goal: module-scoped DI resolution; two registrations of one name both reference.
- `ANL-V0-072`: (proposed under the owner's standing auto-accept (2026-10-11); V1-2776, V1-2775)
  `ImpactUnknown` carries `detail` (omitted when empty). For `unresolved-reference` it is the
  referrer and 1-based line of the first unprovable mention or registration (`F:N`); for
  `reference-graph-bound` the bound (ANL-V0-069); for `unmapped-path` the walked file nothing
  references or that reaches no terminal. `impact-two-root/0` unknown lines append `: detail`.
  The resolved basis `template-reference:<file>` is unchanged. A reader built before this field
  rejects a report carrying it (`DisallowUnknownFields`), so reports and readers move together.
  Acceptance (ANL-V0-069..072): `TestANLV0069OversizedFileIsANamedBound`,
  `TestANLV0069TotalBoundNamesTheFile`, `TestANLV0069NodeBoundNamesTheBound`,
  `TestANLV0070CommonStemIdentifiersAreNotReferences`,
  `TestANLV0071AngularDIRegistrationsAreReferences`, `TestANLV0072TwoRootUnknownNamesItsDetail`,
  `TestANLV0032TemplateHopPlacesReferencedFiles`. Rollback: revert `internal/appmap/impacthop.go`,
  the `Detail` field and the two-root line suffix; no state migrates. Live qualification on an
  AngularJS adopter: `NOT_RUN`.
- `ANL-V0-073`: (proposed under the owner's standing auto-accept (2026-10-11); V1-3024) A URL
  fragment holding no decoded `?` is redacted too, by `observe` and by the producer alike. One
  holding `=` (an OAuth implicit-flow callback such as `#access_token=...`) is redacted as a query
  under ANL-V0-007, any bad pair replacing it with `_=REDACTED`; one without `=` that does not start
  with `/` and looks opaque (24 or more token characters) becomes `REDACTED`. An unchanged fragment,
  such as a route (`#/home`, `#/r=1`), keeps its bytes and its screen. When `observe` cannot parse
  the URL, the producer withholds a hash route holding `=` or an opaque token instead of keeping it
  as ANL-V0-044 keeps a route. Acceptance: `TestANLV0073FragmentRedaction`,
  `TestANLV0044ProducerRedactionMatchesObserve`, `TestANLV0037ProducerNodeUnit`. Rollback: revert
  the bare-fragment branches of `redactURL` in `observe.go` and `navigation-events.cjs`.

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
ANL-V0-048/049 change only how `Impact` classifies changed paths; revert that change to roll back.
The `application-map-impact/0` fields are unchanged by them, so no reader migrates. ANL-V0-072 adds
the optional unknown `detail`; revert it with the hop changes of ANL-V0-069..071. AMAP-V0-210 adds the optional
unknown `remedy` (the declaration that would resolve it, or the stated uncertainty); revert it with
`internal/appmap/remedy.go`.

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
| ANL-V0-008 | `internal/appmap/observe.go` `trimToBudget` | `TestANLV0008InputsAndBudget`, `TestANLV0008BudgetTrimIsLinear` |
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
| ANL-V0-044 | `internal/appmap/navigation-events.cjs` | `TestANLV0044ProducerRedactionMatchesObserve`, `TestANLV0037ProducerNodeUnit`, `TestANLV0039ObserveAcceptsProducerOutput` |
| ANL-V0-045 | `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-11-navigation-events-producer.md` | `TestANLV0042LiveParityWithTraceZip` |
| ANL-V0-046 | `internal/appmap/navigation-events.cjs` | `TestANLV0037ProducerNodeUnit` |
| ANL-V0-047 | `internal/appmap/impacthop.go` `references` | `TestANLV0047IndexIdentifiersDoNotHideTheIndexFileReference` |
| ANL-V0-048 | `internal/appmap/impact.go` `Impact` | `TestANLV0048RubyOnlyChangeFailsClosed`, `TestANLV0015RouterChangeReachesDescendants`, `TestANLV0016UnknownForcesFullSuite` |
| ANL-V0-049 | `internal/appmap/impact.go` `importingSpecs`, `testGraphIncomplete` | `TestANLV0049PageObjectOnlyChangeSelectsImportingSpecs`, `TestANLV0049GraphIncompleteSourcesAndHopImporters` |
| ANL-V0-060 | `cmd/corvint/affected_appmap.go` `affectedBindImpactReport`, `cmd/corvint/affected.go`, `internal/appflows/repositories.go` `OpenCheckout` | `TestANLV0060AffectedBindsReportToDeclaredRepository` |
| ANL-V0-061 | `cmd/corvint/affected_appmap.go` | `TestANLV0060AffectedBindsReportToDeclaredRepository` |
| ANL-V0-062 | `cmd/corvint/affected_appmap.go`, `internal/appmap/impact.go` | `TestANLV0060AffectedBindsReportToDeclaredRepository` |
| ANL-V0-063 | `cmd/corvint/affected_appmap.go` `impactUnsupportedLayout` | `TestANLV0063ImpactRefusesUnsupportedLayout` |
| ANL-V0-064 | `cmd/corvint/impact_tworoot.go` `runTwoRootImpact`, `cmd/corvint/main.go` | `TestANLV0064TwoRootImpact`, `TestANLV0064TwoRootImpactRefusesInvalidLayout` |
| ANL-V0-065 | `cmd/corvint/impact_tworoot.go` `twoRootImpactVerdict` | `TestANLV0064TwoRootImpact`, `TestANLV0065TwoRootImpactNeverEmptyForAChange` |
| ANL-V0-066 | `cmd/corvint/impact_tworoot.go` `twoRootImpactDrift`, `cmd/corvint/affected_appmap.go`, `cmd/corvint/affected.go` | `TestANLV0066TwoRootImpactAtRevision`, `TestANLV0066TwoRootImpactDriftRecheck`, `TestANLV0067AffectedChecksTestRootPin` |
| ANL-V0-067 | `internal/appmap/impact.go` `testRootPin`, `cmd/corvint/affected_appmap.go` | `TestANLV0067AffectedChecksTestRootPin` |
| ANL-V0-068 | `internal/appmap/tracezip.go` `LoadTraceZip`, `internal/appmap/observe.go`, `cmd/corvint/flows_appmap_observe.go` | `TestANLV0068MultiContextTraceZip`, `TestANLV0068TraceZipKeysAndBounds`, `TestANLV0022TraceZip` |
| ANL-V0-069 | `internal/appmap/impacthop.go` `loadReferenceGraph`, `hopRuns`, `mayMention`, `templateHop` | `TestANLV0069OversizedFileIsANamedBound`, `TestANLV0069TotalBoundNamesTheFile`, `TestANLV0069NodeBoundNamesTheBound` |
| ANL-V0-070 | `internal/appmap/impacthop.go` `hopClassify`, `hopLoadSpans`, `hopPathLike` | `TestANLV0070CommonStemIdentifiersAreNotReferences`, `TestANLV0032TemplateHopPlacesReferencedFiles`, `TestANLV0047IndexIdentifiersDoNotHideTheIndexFileReference` |
| ANL-V0-071 | `internal/appmap/impacthop.go` `registrations`, `hopDINames`, `hopNgForms`, `references` | `TestANLV0071AngularDIRegistrationsAreReferences`, `TestANLV0071FiltersAndDirectiveSpellingsAreReferences` |
| ANL-V0-072 | `internal/appmap/impact.go` `ImpactUnknown.Detail`, `cmd/corvint/impact_tworoot.go` | `TestANLV0070CommonStemIdentifiersAreNotReferences`, `TestANLV0071AngularDIRegistrationsAreReferences`, `TestANLV0072TwoRootUnknownNamesItsDetail` |
| ANL-V0-073 | `internal/appmap/observe.go` `redactURL`, `internal/appmap/navigation-events.cjs` `redactBareFragment` | `TestANLV0073FragmentRedaction`, `TestANLV0044ProducerRedactionMatchesObserve`, `TestANLV0037ProducerNodeUnit` |

## Follow-ups and promotion

Proposed follow-ups: typed trigger declarations in map flow steps and spec annotation parsing
(ANL-V0-021 types observed triggers and reporter annotations only); component/template
file and page-object anchors absent from AMAP; multi-map per-app aggregation; costs beyond declared
step counts; test-plan integration; live adopter qualification. UNKNOWN trigger is not a typed
trigger acceptance claim. Promotion needs owner acceptance plus demonstrated useful reuse on an
owner-controlled suite; synthetic checks alone retain experimental status. Kill or revise the
projection if declared adjacency cannot safely support that suite. Live qualification is NOT_RUN,
except that the ANL-V0-037..046 producer ran live on a local two-test Playwright 1.61.1 suite
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-11-navigation-events-producer.md`); an adopter suite run remains NOT_RUN.
