# Application Flow Navigation Execution V0

Intent status: proposed technical profile under AFU-V1-029
Delivery status: experimental; qualification pending

## Agent digest
- Claim: An explicit companion can execute a bounded UI navigation packet against an owned cooperative local fixture.
- Status: proposed technical profile under AFU-V1-029; experimental; qualification pending; no API observation or per-test evidence authority.
- Exists: immutable navigation packets, committed origin admission, bounded observer process ownership.
- Blocked on: implementation and live safety/cleanup qualification.
- Read next: Requirements; Contract; Acceptance; Rollback.

## User and scope

The owner requested a working Corvint 1.0 navigation agent. This technical slice connects the
existing packet to a real browser without treating prose, page content or editable grant fields as
instructions. It is a separate optional companion profile. The existing intent/packet schemas and
read-only CLI/MCP routes do not change. The fixture server and provider are explicitly trusted
local code; this is not an adversarial OS/browser sandbox or permission to run arbitrary setup.

## Requirements

- `NEX-V0-001`: Execution MUST require explicit experimental, trusted-local and observe flags, an
  independently supplied maximum effect (default read), committed execution input and a native
  packet rederived at the same resolved HEAD. Any packet mismatch MUST refuse before startup.
- `NEX-V0-002`: The execution input MUST be a separate closed committed document with exact
  flow/step mappings and operation navigate, fill, click or observe. Prose MUST NOT be executed.
  Every expected outcome MUST have exactly one visible/hidden observation using an intent-style
  role/name or test-ID locator. Missing/extra/unknown mappings, routes, readiness or fixtures refuse.
- `NEX-V0-003`: Every effect and request MUST pass the independent grant and committed origins
  gate before dispatch. Non-GET and form submissions require at least write-irreversible. Exact
  loopback origin, no redirect, blocked WebSocket/service-worker and bounded traffic apply to
  initial navigation, readiness, background traffic and recovery. Unknown attribution or an
  exhausted budget refuses forwarding and prevents a passing receipt. No reset is implicit.
- `NEX-V0-004`: Preconditions MUST follow packet order. At most one declared recovery may execute
  after a failed step; it uses the same guards, does not replay the original, and cannot turn the
  original failure into a passed observation. Nested recovery is unsupported.
- `NEX-V0-005`: The bounded receipt MUST bind revision/tree, packet, execution input, committed
  origins, manifest/source, provider and actual runtime identities. Recheck these bindings before
  startup and after execution. No drift, interruption, timeout or incomplete cleanup may pass.
- `NEX-V0-006`: Receipts MUST contain only closed results/reasons and bounded IDs/methods, never
  fixture values or their digests, DOM prose, raw requests/responses or runtime error strings.
  They MUST NOT be test-run-evidence/0 or LOCALLY_OBSERVED records.
- `NEX-V0-007`: The compiled companion MUST prove real-browser success and pre-dispatch refusal,
  and SIGINT, SIGTERM and timeout MUST leave no owned Node/server/browser descendants or PASS receipt.

## Contract

`corvint-web-flows --experimental --trusted-local --observe --root ROOT --manifest FILE
--assets DIR --flows DIR --navigation PACKET --execution FILE [--max-effect CLASS]
[--fixtures FILE]` selects execution. The existing V0 manifest supplies only captured source,
server and identity bindings. Its scenarios and reset endpoint are never executed in this mode.
Server startup is trusted to initialize an isolated local fixture without external mutations.

`application-navigation-execution-input/0` contains `schema` and `steps`. Every step contains
`flow_id`, `step_id`, `operation`, `observations`; each observation contains `outcome_id`,
`condition` (visible or hidden), `locator` (role/name or test_id). The input is loaded from a regular
committed file at the packet revision and mapped one-to-one to main and recovery packet steps.
The packet must be generated without evidence, registry or traffic inputs; unsupported packets
refuse rather than dropping evidence or rewriting verification labels. Existing intent and packet
wire schemas remain unchanged.

All executable steps need a literal UI route, packet locator, visible readiness locator and at
least one expected outcome. Navigate loads its route then waits for readiness. Other operations
require the current exact route before readiness. Fill alone accepts input_fixture, whose value
comes from the separate private fixtures object (`fixture ID: string`); other operations cannot
consume a value. Fixture values are never used as code, locators, paths or evidence.
Each step waits for all declared visible/hidden observations. Hidden means one matched element
that is not visible; a missing element is not a successful hidden observation. Ambiguous locators
refuse. Recovery may run once after failure; no further normal steps execute after recovery.

Only canonical `http://127.0.0.1:PORT` from the captured manifest is admitted. The provider installs
request interception and form guards before page code; both submit events and programmatic
submit/requestSubmit are guarded. GET is a transport classification, not proof of server semantic
purity; cooperative server code is part of trusted-local admission. Browser-origin requests are
proxied without redirect following. Response/request limits and maximum8192 traffic records apply.
Non-GET outside an active transition refuses. Non-HTTP transports remain unqualified and blocked
where supported; no hostile-page security claim is made.

Execution uses the existing process-group and browser lifecycle. Maximum duration is bounded by
the companion (default five minutes, caller may lower it); no persistent state or service is added.
An `application-navigation-execution-receipt/0` has CALLER_REPORTED authority, passed/incomplete
status, closed per-step outcomes, method-only traffic and cleanup facts. Recovery remains visible.

## Non-goals and failure modes

No API action observer, crawl, CSS/JS selectors, arbitrary assertions, implicit setup/reset,
credential acquisition, extension of packet/intent schemas, evidence promotion, hostile runtime
containment, or AFU-V1-014 closure. Unknown input, stale/tampered bindings, unavailable dependencies,
request/effect refusal, observation failure, timeout or incomplete cleanup refuse or remain incomplete.

## Acceptance

Focused Go admission and existing navigation goldens; Node policy checks; compiled companion real
navigate/readiness/fill/click/observation witness; default read and unlisted/uncommitted origin cause
zero writes; declared read POST and GET forms refuse before dispatch; recovery/preconditions obey
identical guards; stale packet/source/provider/execution and unknown fixture/operation/readiness
refuse; no secret or body output; actual SIGINT/SIGTERM/timeout descendant absence.

## Rollback

Revert the separate companion execution modules and this proposed input profile. Existing read
packets and the original V0 scenario observer continue unchanged; no migration is required.
