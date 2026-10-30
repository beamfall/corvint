# Shell analyzer candidate

`analyzershell` is an unregistered, unselected, Core-unreachable experimental
native-Go candidate. It accepts only the `shell` family and `shell.posix-bash`
input records containing caller-supplied POSIX/Bash source bytes. It statically
emits the closed facts `shell.import.static`, `shell.command.static`,
`shell.env.read`, `shell.env.write`, `shell.trap.static`, and
`shell.script.entry`; dynamic shell forms reject the whole request.

It never opens input paths, follows symlinks, reads an environment, starts a
process, sources/evaluates shell, accesses a network, or writes ambient state.
The command only consumes bounded stdin and writes one canonical LF-framed JSON
response. Registry, launch, admission, selection, packaging, and readiness are
`NOT_RUN`.

The pinned fixture provenance is Beamfall Core
`da38c59eb30b2121cbac37b912485b30b2e54841`, relay
`9723152fdd4ead36b32553a171d98749e4fc23e9`, and plugin SDK
`5536ecde6d12214d6786a6832e537bd5d4feca02`. These commits are inputs to
future qualification only; this isolated candidate does not resolve, clone, or
execute them.
