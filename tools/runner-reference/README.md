# Out-of-process reference twin

```sh
go build -o /tmp/corvint-runner-reference ./tools/runner-reference
/tmp/corvint-runner-reference --help
```

This explicitly built, unqualified reference serves `corvint-runner/1` over stdio.
The plugin name is `reference`, version `1`, runner `reference-go-test`. It shares the
compiled-in `runnerprotocol.Reference` implementation and existing native Go
report parser. It describes commands, reads core-captured artifacts, and produces
absent coverage components. It never starts the runner or writes evidence.

Pin the actual executable SHA-256 in an operator manifest, install through
`corvint-runner-plugin install --manifest FILE`, and copy the pin into the project's
tracked discovery file. A reference pin does not qualify a runner or permit
narrowing. This example is not in the default release manifest.

[Command snapshot](command-tree.json), [conformance kit](../../internal/runnerprotocol/README.md).
