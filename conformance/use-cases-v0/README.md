# Corvint main use-case conformance V0

Run from the repository root:

```sh
go run ./conformance/use-cases-v0
```

Success means the closed ledger and every populated evidence envelope are internally valid and
content-addressed. It does not mean an `UNPROVEN` use case is delivered, nor independently establish
the truth or quality of a receipt subject. The canonical ledger intentionally starts with nineteen
`UNPROVEN` rows.
