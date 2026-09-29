# OpenCode 2.0 qualification range

Issue 363 found that the native qualification producer hard-coded OpenCode 2.0.18 even though the
package declaration admitted compatible 2.0 releases. An installation running 2.0.19 could never
produce a matching qualification record and `corvint_status` therefore remained `UNQUALIFIED` with
`qualification-record-unavailable`.

Adapter 0.7.3 admits qualification campaigns for `>=2.0.18 <2.1.0`. The producer reads the actual
host version from the executable in an isolated home, refuses versions outside that range, and
requires the live plugin setup callback to report the same value. The resulting record remains bound
to the exact host version, host executable digest, adapter package bytes, Corvint executable, OS and
architecture. No record is inferred from the range and status remains read-only. Missing or drifted
records return the supported range and the action needed to qualify the exact installation.

The initially installed host was OpenCode 2.0.18. The official
`@opencode/cli-darwin-arm64@2.0.19` archive was then retained in task scratch from its npm release:
archive SHA-1 `719c472fcb3453c78f3900f28635813debd60485`, binary SHA-256
`4d05fc8d3592e4499299e9f6c3411e6f2d1773f61e380da08550ab036d613cd7`, Mach-O arm64, and isolated
`--version` output `opencode v2.0.19`. The focused producer/consumer regression constructs and
validates an exact 2.0.19 tuple. A complete live 2.0.19 campaign remains `NOT_RUN` until the reviewed
source is committed so the clean-source and immutable-input gate can bind the actual record. This is
compatibility and producer evidence, not a claim that all future 2.0 releases were live-tested.

Execution authority remains `NONE`, Frontier remains `UNAVAILABLE`, and legacy lifecycle receipts
remain `FALLBACK`. Dirty or mixed Corvint source checkouts still refuse qualification before a record
can be published. Roll back by reverting adapter 0.7.3 and its AHI-032 amendment; existing exact
records then remain invalid because their package bytes no longer match.
