# 2026-10-11 navigation-events producer (issue #814)

Branch `claude/issue-814-navigation-events`, from `645304a2f`. Requirements ANL-V0-037..046
(proposed).

## Decision

`internal/appmap/navigation-events.cjs` writes the existing closed `playwright-navigation-events/0`
document per test. It is installed by one `require(...).install({outputDir})` line in
`playwright.config`, not as a reporter or fixture. A reporter runs in the runner process and sees no
pages. An auto-fixture would have to be imported by every spec. The config is loaded in every worker,
and Playwright's client instrumentation (`playwright-core` `_instrumentation.addListener`) delivers:

- API call begin and end, which give the in-flight method and selector;
- browser context create and close, which give per-test pages and the write point.

It does this whatever the trace setting. `test.info()` names the attempt. HTTP redirect hops come
from the main-frame navigation request's `redirectedFrom()` chain. The file lives under `internal/`
(AGPL), next to the existing `.cjs` reporters.

## Live evidence

- **Setup.** The host already had Playwright 1.61.1 and Chromium build 1228; nothing was installed.
  The suite was a local two-test suite (a five-step walk, and a `goto` that receives a 302) against a
  loopback server, with a `react-router/0` map.
- **Acceptance:** with `trace: 'retain-on-failure'`, both passing tests wrote events files and no
  trace zip. `corvint flows appmap observe --trace DIR` accepted the directory.
- **Same output with trace on:** the files from a `trace: 'on'` run are byte-identical
  (`diff -r`). Their sha256 values are `e6e9271d…432a` (walk) and `6a87dd21…1cb6` (redirect). They
  are kept as `testdata/navigation-events/live-events/`.
- **Parity, walk:** observe over the walk's events and over its trace zip gives identical visits and
  five identical edges. This includes a delayed client navigation attributed to the in-flight
  `expect` call, which reads UNKNOWN in both.
- **Parity, redirect:** the producer shows an `/old` visit plus a redirect visit. The trace shows
  only `/home`. They agree once the chain is folded (ANL-V0-042).
- **Test fixtures:** the zip fixtures keep only the run's `0-trace.trace` entry, byte for byte. The
  other entries could carry local paths. `TestANLV0042LiveParityWithTraceZip` replays both.

## Review round 1 repairs

Codex round 1 failed the change; each repair was checked live on the same suite.

- **Redaction before writing (ANL-V0-044).** A `goto` to `http://u:p@…/login?token=abc&x=1` was
  written as `…/login?token=REDACTED&x=1`. The URL credentials were dropped and no secret value was
  written.
- **`Browser.close()` (ANL-V0-039).** A test that launches its own browser and closes it with
  `Browser.close()` now writes its document. The round-1 producer, run on the same test, wrote
  nothing.
- **Clearing earlier documents (ANL-V0-046).** The run started with a stale producer document and
  an unrelated JSON file in `DIR`. After the run, the stale document was gone and the unrelated
  file was kept. The two documents written were byte-identical to the fixtures.
- **Size bound (ANL-V0-043).** A document over 4 MiB is withheld.
- **Directory limit.** The help text now names the 64-file directory limit.

## Review round 2 repairs

- **Redaction mirrors observe (ANL-V0-044).** The fragment is decoded once before its query is,
  matching `url.Parse`; `;` and undecodable pairs are dropped, matching Go's query parser.
  `TestANLV0044ProducerRedactionMatchesObserve` runs the producer's redaction through Node and
  checks two things: no secret reaches the output, and observe's `redactURL` of the written URL
  equals its `redactURL` of the raw URL.
- **Memory (ANL-V0-045).** Event buffers are released after each write. A second browser context
  closing in the same attempt withholds the document (ANL-V0-043).
- **Live rerun.** After round 2, a retain-on-failure run wrote both documents byte-identical to
  the fixtures. The `Browser.close()` test still wrote its redacted document.

## Review round 3 repairs (after the review cap)

Codex round 3 failed the change on three points, and all three are repaired here. These repairs
were not re-reviewed, because the lane allows at most three review rounds.

- **Secret-name matching.** The secret-name pattern now uses Unicode case folding (`/iu`), as Go's
  `(?i)` does. Before this, a Kelvin-sign `toKen` key was not matched.
- **Unescaping.** Unescaping is now byte-wise, like Go's, so `%FF` is accepted.
- **Bad pairs in a hash-route query.** Any bad pair now replaces the whole fragment query with
  `_=REDACTED`, as observe does.
- **Tests.** The Go differential test covers each case above, plus non-ASCII queries and fragments.
- **Live check.** A live rerun was byte-identical to the fixtures.

## Review round 4 repairs

Round 4 was run by the orchestrator and confirmed the round-3 repairs.

- **Fragment preservation.** A fragment that redaction leaves unchanged now keeps its original
  bytes, as Go's escaped fragment does. Before, `#!/%68ome?x=1` became `#!/home?x=1`, which broke
  observe's literal route match.
- **URL length.** Committed URLs and redirect hops are now bounded before redaction, as well as
  after. Before, a 5020-byte `?token=` URL was written once redaction shortened it, although the
  trace reader rejects that URL.
- **Tests.** The Node and Go differential tests cover both cases.

## Review round 5 repairs

Round 5 (`gpt-6.1-sol`, read-only) found one P2.

- **Route bytes under a redacted hash query.** A changed fragment was rebuilt from its decoded
  bytes, so `#!/%68ome?token=x` became `#!/home?token=REDACTED`. Observe matches screens on the
  literal route, so under a `/home` screen the trace left `/%68ome` unmapped while the producer
  resolved it. The route now keeps its bytes up to the first literal `?`, and only the query is
  re-escaped. A route holding an encoded `?` cannot do both and is withheld.
- **Unparseable URLs (found while testing the repair).** The producer wrote `REDACTED` for a URL
  with a bad path or fragment escape. Observe redacts that visit's URL but still matches its screen
  on the raw route, so `#!/home?q=%zz` resolved `home` from the trace and `UNKNOWN` from the
  producer. The producer now keeps the path and hash route, then appends `?%` so the URL stays
  unparseable.
- **Tests.** `TestANLV0044ProducerRedactionMatchesObserve` now compares the decoded URL. It also
  runs both documents through `Observe` under a `#!` map, requires equal screen and status for each
  visit, and requires at least one `home` visit. The Node tests cover the withheld routes. The live
  rerun produced files byte-identical to the `live-events` fixtures.

## Overhead

The run was 80 tests (`--repeat-each=40`, one worker) on a loaded shared host, three runs each:

| Producer | Run 1 | Run 2 | Run 3 | Median |
| --- | --- | --- | --- | --- |
| Off | 71.09 s | 72.05 s | 70.44 s | 71.09 s |
| On | 79.62 s | 70.94 s | 71.96 s | 71.96 s |

- That is about +1.2% on the median, which is within host noise. The 79.6 s run is an outlier.
- All 80 files were written.
- Per call, the cost is one listener callback that pushes or filters a small array. Per test, it is
  two page listeners and one sub-kilobyte write.

## Limits

- **Private API.** `_instrumentation` is not public Playwright API and is qualified on 1.61.1 only.
  A missing hook warns once and records nothing (ANL-V0-038, ANL-V0-045).
- **Test key.** `test_key` is Playwright's `testId`, not the qualified receipt test key, which the
  producer cannot compute.
- **Abstention.** A second page, more than 4096 events, a URL over 4096 bytes or a document over
  4 MiB writes no document (ANL-V0-043).
- **Parity boundary.** A call that commits several URLs (a client-side redirect) yields extra visits
  that the trace's snapshots cannot see. The parity check covers only the two live shapes above.
- **Adopter suite.** No adopter-suite run was made.
- **Clearing scope.** The runner clears `DIR` whenever it loads the config, including on `--list`.
  A retry that never opens a browser context leaves the earlier attempt's document in place.
- **Overhead measurement.** The overhead was measured before the round-1 repairs. Those repairs add
  string work only when a URL commits.

## Rollback

Delete the `.cjs`, its tests and fixtures, and the `observe` help sentence. Suites drop their one
config line. No wire format, store or map changes.
