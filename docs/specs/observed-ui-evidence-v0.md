# Observed UI Evidence V0

**Owner:** Russell Lewis  
**Date:** 2026-10-25  
**Intent status:** proposed  
**Delivery status:** experimental (observed-ui-records/0, the shared staleness evaluator, ARIA structure digests and `runs server-errors` landed with synthetic fixtures, and `flows appmap impact --records` consumes the bindings; real Playwright trace qualification NOT_RUN)  
**Rollout class:** shadow-only  
**Wire profiles:** `observed-ui-records/0`, `observed-ui-routes/0`, `observed-staleness/0`,
`observed-ui-structure-diff/0`, `server-error-log/0`, `playwright-server-errors/0`

## Agent digest
- Claim: Local Playwright traces and server error logs yield revision-bound observed UI records, staleness, structure signals and attached server exceptions.
- Status: proposed/experimental (observed-ui-records/0, the shared staleness evaluator, ARIA structure digests and `runs server-errors` landed with synthetic fixtures, and `flows appmap impact --records` consumes the bindings; real Playwright trace qualification NOT_RUN)
- Exists: `flows appmap observe --records`, `flows appmap staleness`, `flows appmap structure-diff`, `runs server-errors`, `flows appmap impact --records`.
- Blocked on: owner acceptance; a real Playwright trace for qualification.
- Read next: §3 Requirements; §5 Failure modes.

## 1. Intent

Static application maps miss routes that are enums or built at run time, and page objects whose
methods hide the route stay unbound, so impact over a page-object edit can fail open (#1803).
Playwright traces from ordinary runs already record every action, its source location, every
main-frame URL and every request. This contract turns them into LOCALLY_OBSERVED records bound to
the test that produced them and the commit it ran at (#1828, #1829), states when such a record is
still current at a later commit (#1830), offers scrubbed ARIA structure digests as a candidate
structural-change signal (#1831), and attaches the server-side exception to a test that failed on an
HTTP 5xx (#1825).

Every record is learned overlay evidence (authority `learned`); none outranks project authority
(invariant 3). Missing evidence reads UNKNOWN, never "no impact" (invariant 2). All verbs are
read-only, offline and in one binary (invariants 4 and 7).

## 2. Non-goals

- No browser, recorder or network access; traces and logs are supplied files.
- No pass/fail verdict from ARIA digests; no auto-healing of selectors.
- No storage of headers, bodies, typed text, query values or accessible names.
- Impact reads bindings only to add screens and tests; it never removes an unknown, narrows a
  verdict, or reads bindings of a `--repo` repository (OUE-V0-072).
- No server log format beyond the closed `server-error-log/0` JSON-lines shape; adopters convert.

## 3. Requirements

### 3.1 Observed UI records

- `OUE-V0-001`: `flows appmap observe --records` MUST read Playwright trace zips in memory and print one `observed-ui-records/0` document with evidence `LOCALLY_OBSERVED`, authority `learned`, the revision, and per trace its test key and trace SHA-256.
- `OUE-V0-002`: The revision MUST be a full 40- or 64-hex commit ID; between 1 and 64 traces MUST be supplied and a repeated test key MUST be refused.
- `OUE-V0-003`: Each observed page URL MUST resolve to a screen in order: the map's unique screen (basis `map`), else an `observed-ui-routes/0` template (basis `route-template`), else default normalization (basis `default-normalized`); a URL whose map candidates are ambiguous or unresolved MUST read UNKNOWN with its candidates.
- `OUE-V0-004`: `observed-ui-routes/0` MUST be a closed document of path templates; the most literal matching template wins and an equally literal tie MUST read UNKNOWN.
- `OUE-V0-005`: Default normalization MUST replace numeric, UUID, hex and token-shaped path segments with `:param`, so a concrete ID never becomes a record key.
- `OUE-V0-006`: Request and response headers, bodies, typed action values (text, value, values, key, files, options, label), query values, ARIA names and texts, and action selector values (quoted strings, regex literals and unquoted `text=` values, replaced by a placeholder) MUST NOT appear in the output; each class MUST be counted under `dropped`, by key presence without decoding it where the value is not otherwise needed.
- `OUE-V0-007`: A transition MUST be recorded between consecutive distinct screens, with the typed trigger of the action whose before URL is the earlier page and whose after URL is the later one; a goto or an action with no before URL binds to its after URL. Each action MUST attribute at most one crossing, so a repeated crossing keeps its own trigger.
- `OUE-V0-008`: An endpoint MUST be recorded per method, origin and normalized path template with its status classes, the screens it was seen on, and an `unplaced` count of requests not attributable to a screen.
- `OUE-V0-009`: The output MUST stay within 8 MiB and the existing trace zip, entry and line bounds; exceeding one MUST refuse with `appmap-bound-exceeded`, while an unreadable or malformed input refuses with `appmap-invalid-query`, except a malformed stacks entry (OUE-V0-014).
- `OUE-V0-010`: A page-object, workflow or spec binding MUST come from an action's source frames (the before record's stack, `trace.stacks`, or `test.trace` joined by stepId), placed on the map file with the longest path-suffix match (an equal-length tie is not placed) and the narrowest enclosing method, with provenance test key, trace SHA-256 and `file:line`.
- `OUE-V0-011`: Every map page-object method, workflow method and spec file with no binding MUST be listed under `unbound` as UNKNOWN with a reason (`not-observed`, `trace-without-stacks` or `observed-screen-unresolved`); without a map the document MUST carry a `no-map` unknown.
- `OUE-V0-012`: The document MUST be deterministic for identical inputs and carry a SHA-256 digest over its canonical bytes with an empty digest field; loading a records document MUST refuse one whose digest does not verify.
- `OUE-V0-013`: A stacks pair MUST be read with its call ID as written, either a string (Playwright 1.64 writes IDs such as `wcuw@676`) or an integer (read as `call@N`), and matched to the action with that call ID in the same context entry; two context entries with one name MUST be refused. (proposed; V1-2739, GH #1852)
- `OUE-V0-014`: A stacks entry that is not a readable stacks document (its bytes fail to inflate or their checksum, it is not a JSON object with a `stacks` array, or a pair or call ID is malformed or null) MUST NOT discard the trace: its actions carry no stacks frames, the document MUST carry a `stacks` unknown naming the entry with reason `trace-without-stacks`, and every other entry MUST still be read; a stacks entry over the entry bound still refuses under OUE-V0-009. (proposed; V1-2739, GH #1852)
- `OUE-V0-058`: Real chromium trace zips written by Playwright 1.64.0 under the #1852 reporter's configuration (`trace: {mode: 'on', snapshots: {dom: true, aria: true}, screenshots: false, sources: true}`), one single-context and one multi-context (an API request context, a second browser context and the test page), MUST stay committed as fixtures with their generating spec and config (`internal/appmap/testdata/pw164/`) and MUST be read whole: `observe --records` attaches stack frames through string call IDs with no `trace-without-stacks` unknown and yields screens, transitions and endpoints, and `observe --trace ZIP --test-key K` yields one observation per page (ANL-V0-068). Failure mode: the 1.64 shape was hand-built and could drift from what Playwright writes. Non-goal: other browsers or Playwright versions. Rollback: delete the fixtures and TestOUEV0058RealPlaywright164Traces. (accepted under the owner's standing lane auto-accept 2026-10-11; V1-2739, GH #1852)

### 3.2 Staleness

- `OUE-V0-030`: The shared evaluator MUST state each record observed at commit C with linked repository paths at commit R, and `flows appmap staleness` MUST evaluate every screen, transition, endpoint, binding and structure record of one records document.
- `OUE-V0-031`: A record MUST read `current` when no linked path changed in C..R, `retired` when one changed and the commit distance exceeds `--retire-after` (0 never retires), and `stale-unconfirmed` otherwise; a linked directory counts as changed when any file under it changed.
- `OUE-V0-032`: A record MUST read UNKNOWN, never current, when it has no linked path (`no-linked-paths`), links an aliased root (`linked-path-outside-root`), its revision does not resolve, is not an ancestor of R, the distance or diff is unavailable (`missing-diff`), or the checkout's ancestry is rewritten (`ancestry-untrusted`).
- `OUE-V0-033`: The report MUST carry the denominator: total equals current plus stale-unconfirmed plus retired plus UNKNOWN.
- `OUE-V0-034`: An UNKNOWN record MUST NOT carry a distance.
- `OUE-V0-035`: The Git history MUST be read-only through the sanitized Git environment with optional locks and fsmonitor disabled, and renames MUST count as both sides changing.
- `OUE-V0-036`: `--at` that does not resolve to a commit MUST be refused.

### 3.3 ARIA structure digests

- `OUE-V0-040`: Each action's ARIA snapshot MUST be scrubbed and recorded per screen as a structure digest with its node count and the tests that saw it; a snapshot over 65,536 lines MUST yield no digest and be counted per trace as `aria_oversized`, never a digest of its prefix.
- `OUE-V0-041`: Scrubbing MUST keep only indentation, role and the plain values of the state attributes level, checked, disabled, expanded, pressed, selected and active; names, text, property lines and other attributes MUST be dropped and counted.
- `OUE-V0-042`: `flows appmap structure-diff` MUST state each screen `same-structure`, `structure-changed` (with base-only and head-only digests) or UNKNOWN (`observed-one-side`, `screen-unresolved`), with counts that sum to the screens compared.
- `OUE-V0-043`: The diff MUST be labelled signal `candidate` and authority `learned` and MUST NOT carry a pass/fail verdict.
- `OUE-V0-044`: The diff MUST count, per side, the traces that carried no ARIA snapshot, so absent snapshots are visible rather than read as no change.
- `OUE-V0-075`: Scrubbing MUST keep a role only from the closed WAI-ARIA 1.2 role vocabulary (plus `text`, `img` and `image`); any other token in the role position MUST be digested as `#other` and counted dropped, so free text or a secret in that position never reaches the digest (#1831).
- `OUE-V0-076`: Each `structure-changed` screen in the diff MUST name, sorted, the tests that saw its base-only or head-only digests; every other screen MUST carry an empty `tests` list.

### 3.6 Impact consumption of observed bindings

- `OUE-V0-070`: `flows appmap impact --records FILE` MUST refuse with `appmap-invalid-query` records built without an application map or for another application than the map.
- `OUE-V0-071`: A changed path with an observed binding in the `--root` repository MUST touch each bound screen the map declares, with basis `observed-binding` when no other relation already placed that screen (an existing basis is kept and the observed entry still lists the screen), and MUST add the binding's tests and observing test keys to `test_keys` (#1829).
- `OUE-V0-072`: Bindings MUST only add: they MUST NOT remove an unknown or turn `full-suite-required` into a narrower verdict, and a changed path without a binding MUST keep its existing placement or unknown reason.
- `OUE-V0-073`: The report MUST carry `records_revision`, `records_digest` and, per bound changed path, its bindings, mapped screens, unmapped screens (route templates, normalized URLs, unresolved screens), tests and trace digests.
- `OUE-V0-074`: Output with records MUST be deterministic, MUST still load through the `application-map-impact/0` verifier, and trimming MUST count dropped entries under `omitted.observed`; without `--records` the report is unchanged.

### 3.4 Server exceptions for HTTP 5xx failures

- `OUE-V0-050`: Reading a Playwright JSON report MUST record each attempt's RFC 3339 startTime when present and the HTTP 5xx status, method, path and request id its error messages name, without storing message text or query strings.
- `OUE-V0-051`: `server-error-log/0` MUST be a closed JSON-lines shape (time, method, path, an HTTP status from 100 to 599, optional request_id, exception class, message and frames); an unknown member or a malformed line MUST be refused.
- `OUE-V0-052`: A failed or timed-out attempt's 5xx MUST match server records inside [start − window, start + duration + window] (window default 2 s) on status exactly, on method and path when known (else those of request-log 5xx requests in the window), on whichever of method or path is known when only one is, and on request id when both sides carry one.
- `OUE-V0-053`: One distinct matching cause MUST read ATTACHED; none MUST read NOT_FOUND with reason "server error not found in log"; several distinct causes or a missing startTime MUST read UNKNOWN with the reason.
- `OUE-V0-054`: An attached exception MUST carry its class, its screened first message line and its first application frame, skipping gem, vendor, node_modules, site-packages, runtime and system-library frames.
- `OUE-V0-055`: An attachment whose first application frame is under a `--support-path` prefix MUST be classified `environment/fixture`; otherwise `product`.
- `OUE-V0-056`: Attachments MUST group across runs by exception class, digit-normalized message and first application frame, with occurrences, runs and tests per group, and the counts MUST sum attached plus NOT_FOUND plus UNKNOWN to the failures.
- `OUE-V0-057`: `runs server-errors` MUST refuse an unreadable server log or request log rather than report that no server error happened.

### 3.5 Reason and refusal codes

Screen reasons: `ambiguous-route-template` (OUE-V0-004), `no-template` (default normalization used because no
template matched), `non-http-url` (a page URL that is not http or https). Staleness reasons, beside
§3.2: `not-ancestor`, `ancestry-unavailable` and `distance-unavailable` (a Git answer missing).
`runs server-errors` refusals: `runs-server-log-unavailable`, `runs-request-log-unavailable`
(OUE-V0-057), beside the existing `runs-invalid-arguments` and `runs-report-unavailable`.

## 4. Acceptance evidence

| Area | Requirements | Evidence |
|---|---|---|
| Records | OUE-V0-001..014 | `internal/appmap/uirecords_test.go` (TestObserveUIRecords*), `cmd/corvint/flows_appmap_ui_test.go` |
| Staleness | OUE-V0-030..036 | `internal/observedstale/observedstale_test.go`, `cmd/corvint/flows_appmap_ui_test.go` |
| ARIA | OUE-V0-040..044, 075, 076 | TestScrubAriaDigest, TestDiffUIStructures, TestOUEV0075ScrubAriaClosedRoles, TestOUEV0076StructureDiffNamesTests |
| Impact | OUE-V0-070..074 | `internal/appmap/impact_observed_test.go` (TestOUEV0070*, TestOUEV0071*, TestOUEV0072*) |
| Server errors | OUE-V0-050..057 | `internal/playwrightruns/server_errors_test.go`, TestRunsServerErrorsCLI |

OUE-V0-058 commits two real Playwright 1.64.0 chromium traces (TestOUEV0058RealPlaywright164Traces); they carry no `aria` member at all although `snapshots.aria` was on, so the `after.ariaSnapshot` assumption remains unconfirmed. The other fixtures are synthetic; OUE-V0-013/014 use a hand-built Playwright 1.64-shaped multi-context fixture (`internal/appmap/testdata/playwright-1.64-shape`, TestObserveUIRecordsPlaywright164Stacks), and the assumption that 1.64 `before` records carry the same string call ID as the stacks pairs is unverified. NOT_RUN: a real trace that carries an aria snapshot (the
`after.ariaSnapshot` member is assumed) and a real adopter server log.

## 5. Failure modes

- A trace written by a Playwright version with a different stack or snapshot layout yields fewer
  bindings; every unmatched method still reads UNKNOWN, never bound.
- A stacks entry in an unrecognised layout is reported as a `stacks` unknown with
  `trace-without-stacks` (OUE-V0-014); its methods read UNKNOWN rather than the trace refusing.
- A screen recorded from default normalization may merge two routes; supply `--routes` or a map.
- A missing server log line reads NOT_FOUND only when the log was readable; unreadable logs refuse.
- Clock skew larger than the window turns attachments into NOT_FOUND; widen `--window-ms`.
- Records from an older revision can bind a page object whose methods moved; impact still only adds
  screens and tests, so the cost is a wider selection, never a missed unknown. Check freshness with
  `flows appmap staleness`.

## 6. Rollback

All surfaces are new verbs and new packages (`internal/observedstale`, new files under
`internal/appmap` and `internal/playwrightruns`). Reverting the change removes them; the only
edit to an existing reader adds fields to `playwrightruns.Result`, which no existing output emits. OUE-V0-013/014 change only how `internal/appmap/uirecords.go` reads stacks entries; reverting that change restores the integer-only reader. OUE-V0-075/076 and 070..074 add a role allow-list, a `tests` field on diff screens, the `--records` flag and new optional `application-map-impact/0` fields that are absent without `--records`; reverting them restores the earlier output byte for byte.
