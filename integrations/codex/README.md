# Corvint for Codex

This package is the source of the native Codex CLI/Desktop integration. The qualified install does
not use this directory: the installable plugin is published from a committed revision into a user-owned
marketplace, `~/.local/share/corvint/codex-marketplace/corvint-full-candidate`, whose `PUBLISH.json`
records the source revision, the stamped plugin version and the three transforms applied to this
tree (version stamp; `hooks/hooks.json` replaced by the `prepare-qualified-hooks` rendering for the
installed root release; `scripts/` omitted because the adapter lives in the root release). Add that
marketplace with `codex plugin marketplace add ~/.local/share/corvint/codex-marketplace/corvint-full-candidate`,
install with `codex plugin add corvint@corvint-full-candidate`, then review and trust its lifecycle hooks
with `/hooks`. After a republish, upgrade with `codex plugin marketplace upgrade corvint-full-candidate`;
disable or uninstall with `codex plugin remove corvint@corvint-full-candidate`. Removing the plugin
leaves Corvint repository data unchanged.

To try the preview without that publish step, put `corvint` on `PATH` (see
[Try it](../../README.md#try-it-in-a-minute)) and, from the root of a Corvint checkout, run
`codex plugin marketplace add ./integrations/codex` and `codex plugin add corvint@corvint-source`. This
installs the source tree as-is: its `hooks/hooks.json` calls `corvint adapter codex` from `PATH`
rather than the qualified root-release rendering. Remove it with
`codex plugin remove corvint@corvint-source` and `codex plugin marketplace remove corvint-source`.

Before refreshing a local plugin with `codex plugin add`, wait for active tasks using its hooks
to finish: reinstall can prune the old version cache while those tasks retain absolute hook paths.
Record the installed version and preserve its exact artifact, including file modes, before refresh.
Verify the new cache against its source and start a new task to pick it up. If an upgrade removes
a still-referenced cache, restore that exact old artifact atomically at its original version path
without selecting it as the installed version or changing the new cache or hook trust. Keep it until
the affected tasks end; source/cache equality alone does not verify active hook paths.

The package provides the Corvint skill plus bounded `SessionStart`, `UserPromptSubmit`, `Stop`, and
`SessionEnd` hooks. It intentionally omits an MCP declaration until Corvint ships and conforms a real
MCP server. Current status is `FALLBACK`: supported context works when the local `corvint`
executable is available. A separately enrolled local completion policy can request one bounded
Stop remediation; formal closing Frontier authority remains unavailable.

The hook routes all four native events to `corvint`, including `UserPromptSubmit` and
`SessionStart(source=compact)`. Prompt input is trimmed to the core's canonical task and carries
only that task plus an optional session-ID hash. A missing Go binary emits a visible host-valid
fallback and does not block unrelated coding; no event routes to a legacy runtime. Every runtime
response passes the separate `corvint-dogfood-event/0` full-result digest, adapter, event, repository,
support-level, prompt-privacy and local-policy validation boundary before repository data is placed
in the fixed untrusted-data envelope. The legacy `corvint-harness-event/0` CLI profile stays unchanged.

The installed `corvint` process owns input normalization, the bounded native event, validation,
and host rendering under the existing event deadline. No adapter subprocess or detached worker is
created. Oversized input or output, interruption, runtime failure, and deadline expiry produce fixed
visible fallback reasons.

Codex also exposes [`PostToolUse`](https://developers.openai.com/codex/hooks#posttooluse), but its
native event supplies raw tool arguments and tool output, not Corvint's normalized changed paths,
evidence handles, or verification observations. This plugin therefore does not register that hook:
forwarding those raw bodies would violate `AHI-014`, while an empty receipt after every tool call
would add overhead without evidence value.

The bundled Corvint skill can invoke the Go authority-start query directly for roadmap, orientation,
work-queue, and workflow-gate tasks. The plugin does not reinterpret that context receipt as a
harness-event receipt. Automatic prompt context remains `FALLBACK` and is injected only after the
native core returns a valid bounded receipt. The new profile separates root governance, declared
enrollment scope and explicit task anchors. Vague follow-ups retain governance with
`explicit-task-anchor-required`, withholding incidental lexical matches. General natural-language
discovery remains available through direct `query`.

After Codex compacts a session, `SessionStart(source=compact)` asks Corvint to rehydrate a bounded
impact packet from exact dirty paths tracked at the pinned revision. Untracked paths remain an
explicit count/digest gap rather than invented evidence. This restores repository evidence only:
the adapter never reads Codex's transcript or persists prompt/model text, so decisions and other
uncommitted task state remain an explicit gap.

Tested source adapter version: `0.2.2`. The shipped compatibility metadata records Codex `0.149.0`
static manifest validation, the state at package build. The published matrix
(`../compatibility.json`) records the later installed-lifecycle PASS for Codex CLI `0.153.2` with
adapter `0.2.2` on darwin-arm64 against Corvint 0.8.1. Every other host version, adapter version and
OS remains untested `FALLBACK` by contract.

The bundled skill drives `dogfood begin/status/verify/finish/review` for authorized substantive Corvint
changes. It still judges citation meaning and check adequacy. The core executes selected checks,
binds report inspection and validates the exact final artifact set before local satisfaction.
Only explicit enrollment can enable Stop remediation. Recursive Stop, cancellation, unsupported
hosts and timeout release without a satisfaction claim. This is local workflow enforcement under
decision 0009 option 2; it is not an execution attestation or host-conformance promotion.

Run `corvint index --if-stale` explicitly after replacing the binary or changing the committed
tree to refresh the derived snapshot. A fresh worktree without that snapshot can exceed the native
event deadline and fail open; the hook never silently warms persistent state. Original explicit
query/impact receipts may retain their caller-entered task in the private provenance archive; this
adds no capture of native prompt bodies or transcripts. Secret-shaped copies are refused visibly.
