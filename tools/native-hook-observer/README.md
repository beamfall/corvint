# Native hook observer

This experimental native Go command projects bounded Codex App Server hook notifications without
granting host authority or starting a task, daemon, model turn, subprocess, or configuration change.
Build it explicitly before an approved measurement:

```sh
GOTOOLCHAIN=local go build -o /ABSOLUTE/PRIVATE/PATH/corvint-native-hook-observer ./tools/native-hook-observer
```

The supported modes are:

```sh
corvint-native-hook-observer observe
corvint-native-hook-observer client ABSOLUTE_CWD
corvint-native-hook-observer client ABSOLUTE_CWD --subscribe
corvint-native-hook-observer client ABSOLUTE_CWD --prearm-idle EXPECTED_INVENTORY_FILE [COMPARISON_GOLD_FILE]
```

`observe` accepts newline-delimited notification exports on stdin. Subscription modes accept only
an existing UUIDv7 task ID plus newline on bounded stdin. `--prearm-idle` reads closed, bounded regular
inventory and optional comparison files before opening the socket. Every mode remains experimental
and emits `UNQUALIFIED`; successful fixtures do not establish native qualification.
