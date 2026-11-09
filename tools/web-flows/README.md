# Experimental web flow observer

Corvint connects declared behavior, source-test assertion candidates and real browser observations.
The [proposed contract](../../docs/specs/application-flow-understanding-v0.md) remains experimental.
It does not claim to discover every flow or prove application correctness.

The default Go CLI reads reports without Node, a browser, a server or network access. Build the
optional companion separately; it requires Node, the pinned packages in this directory and Chromium:

```sh
go build -o /tmp/corvint ./cmd/corvint
go build -o /tmp/corvint-web-flows ./cmd/corvint-web-flows
(cd tools/web-flows && npm ci --ignore-scripts && npx playwright install chromium)
```

Use a disposable trusted local application. Commit its manifest and every declared source/test file.
The manifest declares an exact `http://127.0.0.1:PORT` origin, owned server argv, reset endpoint and
identity endpoint, followed by explicit scenarios. See the complete manifest assembled in
[the fixture evaluation](test/e2e.mjs) and [cooperative server](test/fixture/server.cjs).
The server must echo `CORVINT_FLOW_RUN_ID`, served frontend digest, backend source digest and fixture
seed. The frontend digest is SHA-256 over sorted `path + NUL + source SHA-256 + newline` records.
Identity is checked before actions and before publication. This is cooperative caller-reported
evidence, not authenticated execution provenance.

```sh
# Source-only: no server or browser starts.
/tmp/corvint-web-flows --experimental --trusted-local \
  --root /path/to/app --manifest flows.json --assets /path/to/corvint/tools/web-flows > /tmp/flows.json

# Add --observe to explicitly start the declared server and browser.
/tmp/corvint-web-flows --experimental --trusted-local --observe \
  --root /path/to/app --manifest flows.json --assets /path/to/corvint/tools/web-flows > /tmp/flows-live.json

/tmp/corvint --root /path/to/app flows --manifest flows.json --evidence /tmp/flows-live.json
/tmp/corvint --root /path/to/app flows record --manifest flows.json \
  --evidence /tmp/flows-live.json --output /private/path/new-observation.json
```

Recording exclusively creates a new private file after screening and validation; it never overwrites
accepted intent or changes ranking. Later reads can consume the file until its source bindings drift.
Dirty declared inputs are refused; a different committed tree makes retained observations stale.

Supported source tests are direct top-level literal Playwright tests with `page.goto`, ID-selector
`fill`/`click`, `page.reload`, and `toHaveText`, `toContainText`, `toHaveCount` or `toBeVisible`.
Setup, helpers, branches, dynamic tests and other imports invalidate negative inventory claims.
Mapping requires the same route, full action sequence and exact assertion target/operator/value.
A matching assertion remains a static candidate; running a declared scenario never claims that the
source test passed. Backend JSON-pointer probes independently check persistence/permissions.

The observer discovers sanitized DOM controls and structural state hashes while executing only
declared actions. Untouched controls and unsupported surfaces remain frontier entries. Outcomes
separate UI matches, backend matches, contradictions, hypothesis agreement and inconclusive runs.
All reports retain `complete: false`. Role labels are caller declarations; use distinct role-bearing
routes and a trusted fixture that actually enforces them. Conflicting roles for one route are refused;
reports explicitly label role identity `caller-declared-unverified`.

The observer blocks cross-origin browser HTTP requests, redirects, WebSockets and service workers.
It is not an OS sandbox: trusted server code and other transports are outside that qualification.
Successful publication requires owned process-group cleanup, browser close and observed server exit;
escaped daemon descendants remain unqualified. Runtime, input, output and exploration are bounded.
Artifacts retain hashes/structural identifiers and comparison outcomes, not raw DOM text, input
values, response bodies or screenshots. Inputs must contain no credentials or private production data.

Run the optional evaluation with `bash script/web-flows-gate`. It exercises a disposable application,
a renamed-control variant, missing assertions/tests, two backend defects, stale/identity failures,
HTTP/WebSocket sentinels and real Chromium interruption cleanup. General application accuracy and
time savings remain unmeasured.
