# TestUPDV0004PartialArchiveCancellation deadline flake (V1-1115)

Test-only. No requirement, spec or decision changes; UPD-V0-004 still traces to the same test.

The test cancelled the apply from a 500ms `context.WithTimeout`. Under load the installed-version
probe and release discovery used that budget before the archive request, so the deadline expired
first and the assertion `archive body not partially read and closed` failed with a nil body
(2 of 40 runs under a concurrent package run at the base bf04e8a0; 4 of 40 in the ticket).

`interruptedBody.Read` now calls the apply context's cancel function right after serving the first
chunk, so cancellation is ordered after the partial read. The context has no deadline; the test timeout bounds a hang. The test still asserts the apply is refused, the body was partially read and closed, the
binary is unchanged and no staging is retained.

Evidence: `go test -count=200 -run TestUPDV0004PartialArchiveCancellation ./internal/update` passed
while two concurrent `-count=5` package runs were active; `go test -race -count=1 ./internal/update`
passed.
