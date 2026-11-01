# gotest-evidence fixtures

`count2.json` and `flaky-fail.json` are actual, unedited `go test -json` streams of the module
whose only test file is `flaky_test.go.txt` (renamed so it is not compiled here), captured
2026-10-31 with go1.27.3 linux/amd64:

- `count2.json`: `GOTOOLCHAIN=local go test -json -count=2 .` (exit 1: `TestFail` fails by design).
  SHA-256 `435109580a227c033309600b5bf13d097d9f221198d32f8df9788e33fc13cdaf`.
- `flaky-fail.json`: `FLAKY_FAIL=1 GOTOOLCHAIN=local go test -json -count=1 .`, which also fails
  `TestTable/two/nested`. SHA-256 `bb3fa821805ad9c2b9f683dfa3eba4060095ca5c63eef0518cd9e822e2aa4953`.
