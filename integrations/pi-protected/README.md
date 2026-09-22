# Protected Pi runtime (experimental)

This optional macOS arm64 distribution embeds Pi 0.85.1, the Corvint extension,
provider modules, image worker/WASM, themes and export templates in a Node SEA
executable. Native TUI and RPC remain upstream Pi. Ordinary extensible Pi uses
`../pi`; this distribution has a fixed extension set.

The executable currently delivers **FALLBACK** context and tools. It is not an
accepted execution root or independently qualified FULL installation. The governing
contract is `docs/specs/protected-pi-runtime-v0.md`.

## Build and verify

Use Bun 1.3.11, Go 1.27.1 and the checked-in npm lockfile. Dependency installation
is explicit: `npm ci --ignore-scripts --prefix integrations/pi-protected`.
Set `CORVINT_PI_NODE_ARCHIVE` to the official Node 22.23.2 darwin-arm64 tarball;
the build rejects any SHA256 other than
`61130f394c1630d211dd50aecc4353d379480f36d3ac913cd85dbba1aed585c6`.

`make pi-protected-test` builds and signs an experimental release, then exercises
native RPC/TUI, reload/replacement, image tools, hostile executable resources,
startup injection positive controls and interruption cleanup. It needs loopback
networking for a local model fixture, `sandbox-exec`, Python 3 and Apple command-line
tools. It sends no prompts to an external model. The default Core gate does not
download or install these optional dependencies.

Outputs are in `integrations/pi-protected/build/release`. `manifest.json` records
the exact experimental image and embedded inputs. Ad-hoc signing establishes an
image identity; it supplies no authority or completed qualification. The optional
`tools/local-authority` preparer accepts `prepare-pi-release OUTPUT BUILD_DIR GO_ROOT
GIT_BINARY SOURCE_REVISION ADAPTER_TEMPLATE`; `install-pi-release BUNDLE MANIFEST_SHA256`
is an explicit protected operator step. Preparation produces reviewable caller-owned
files only. Installation refuses another host's admission and creates no accepted
root, key, campaign or qualification. License/input review remains unfinished.

## Runtime data

Run `pi-protected --mode tui` or `pi-protected --mode rpc`. Optional paired flags
are `--provider`, `--model`, `--thinking`, `--data-dir` and `--session`. Unknown or
repeated flags refuse. The default private data directory is
`~/.corvint/pi-protected`; it contains settings, credentials and native Pi sessions.
Global and project Pi extensions/settings are not loaded.

`settings.json` admits `provider`, `model`, `thinkingLevel`, `theme` (`dark`/`light`)
and an optional data-only `endpoint`. The endpoint admits `api`, `baseUrl`,
`contextWindow`, `maxTokens` and optional `input` (`["text"]` or `["text","image"]`).
Supported endpoint APIs are `openai-completions`, `openai-responses`,
`anthropic-messages` and `google-generative-ai`. URLs require HTTPS or loopback HTTP.
Built-in models need no endpoint definition.

Native login writes `auth.json`. Literal API keys and built-in OAuth credentials
are data; shell-resolved credentials and credential-chain entries without a literal
key refuse. Ambient provider configuration is cleared. Package/model-update traffic
is disabled; explicitly requested provider traffic remains available. Built-in
coding tools still run with the user's permissions.

Rebuilding changes the image identity. Do not reuse an earlier image's admission
or qualification. Removing this experimental directory and selecting ordinary Pi
restores the ordinary FALLBACK path without changing any protected store.

An installed image uses QLF/2 only with a separately admitted Pi root/campaign.
Missing admission preserves ordinary context with an explicit degradation. An idle
successful turn can request at most one permitted remediation; recursive unresolved
state is displayed and releases. Abort/error, untrusted or replaced sessions, queued
input and malformed replies cannot trigger continuation. Post-tool observations and
explicit record/outcome routes remain ordinary caller-reported operations.
