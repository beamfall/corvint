# Explicit runner-plugin installation

Build this development tool explicitly:

```sh
go build -o /tmp/corvint-runner-plugin ./tools/runner-plugin
/tmp/corvint-runner-plugin install --manifest ./runner-plugins.json
```

The manifest uses the strict RSP-V0-028 discovery schema. Every executable digest
must match. Relative source paths resolve beside this operator-supplied manifest.
The operator store defaults to the user config directory's
`corvint/runner-plugins`, with immutable executables under their SHA-256 digest
and an atomic `runner-plugins.json`. `--store DIRECTORY` permits an explicit store
outside every repository. A busy install lock refuses; installation never waits
or downloads. Multiple manifest entries install in order; earlier successful
entries remain if a later entry fails. Repeating an admitted install is idempotent.

Repository discovery pins still govern use. Installing a plugin changes no
repository pin, release manifest or qualification. Linux safe descriptor access
is required; unsupported hosts refuse. This tool is not bundled by default.

`--help` gives usage, required inputs and an example.
[Command snapshot](command-tree.json), [host and kit](../../internal/runnerprotocol/README.md).
