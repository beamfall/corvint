# Corvint main use-case conformance V0

Run from the repository root:

```sh
go run ./conformance/use-cases-v0
```

Success means the closed ledger and every populated evidence envelope are internally valid and
content-addressed. It does not mean an `UNPROVEN` use case is delivered, nor independently establish
the truth or quality of a receipt subject. The canonical `/1` ledger has twenty-two
`UNPROVEN` rows, including three narrowly scoped daily-workflow jobs (decision 0332).
The reader still accepts the exact historical nineteen-row `/0` profile. Old readers reject `/1`;
do not rewrite archived `/0` evidence or add new IDs under that profile.
