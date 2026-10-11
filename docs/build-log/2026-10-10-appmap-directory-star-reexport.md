# appmap directory import and star export reads (V1-1144, GitHub #786)

Finding: the reader already resolves a directory import to `index.{ts,js}` and follows one
`export * from` level when exactly one module declares the name (AMAP-V0-025). A minimal fixture of the
reported shape (directory import, star barrel, string enum) resolved before any change, so the
adopter's failure has another cause that the issue text does not isolate (most likely a further
read or write of the table in an examined file). That is stated, not assumed away.

Delivered: optional `detail` on `Unknown` naming the failed hop of a `not-read-whole`, and the
candidate paths on an ambiguous star re-export (AMAP-V0-071/072), plus a fixture for the reported
shape (AMAP-V0-070). The ambiguity reason stays `ambiguous-barrel`: renaming it to
`ambiguous-reexport` would change existing fixtures' bytes, which the ticket forbids; the name
appears in `detail`. Maps with a not-read-whole unknown gain the member and a new digest.
