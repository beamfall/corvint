# Reference precedence public cases (CEM-V1-176, CEM-V1-177)

Status: like the rest of `protocol/cem-1.0/stable/`, implemented and run by both
consumers; the technical contract is proposed and not promoted.

Six copies of `../neutral-criterion/stable.json`, each with one graph mutation
whose stage and code `../ALGORITHMS.md` fixes (the CEM-V1-176 paragraph after
the graph precedence rule). Every case fails before Git access, but the cases
still run against the neutral-criterion repository and artifacts.

- `cases/<name>.json`: the mutated maps, canonical JSON plus one LF.
- `results/<name>.json`: the exact result line, nested member order included,
  as the native verifier wrote it with `runtime.goVersion` `go1.27.2`.
- `EXPECTED.json`: profile `cem-stable-reference-precedence/1`. It names the
  repository recipe and artifact root it reuses, the two revisions, and for each
  case its `name`, `description`, `map`, `result`, `exit`, `outcome`, `stage`
  and `code`.

Run each case with either consumer (absolute paths, `--artifacts` set to the
neutral-criterion directory):

```sh
corvint cem verify-stable --repository R --map CASE/MAP \
  --expected-base BASE --target TARGET --artifacts NEUTRAL
cem01-go verify-stable-canonical --repository R --map CASE/MAP \
  --expected-base BASE --target TARGET --artifacts NEUTRAL
```

Both consumers must write the same line. The tests named `TestCEMV1177*` in
`internal/cem/verify` and `interop/cem01-go` compare their own line with
`results/<name>.json` byte for byte.
