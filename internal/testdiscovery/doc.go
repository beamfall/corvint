// Package testdiscovery is the committed test-source reader that test
// discovery needs: the bounded Git blob reader, the JavaScript/TypeScript
// token model and the RSpec line reader, and BuildTestDiscovery, which
// derives the Playwright and RSpec test-discovery input of the documentation
// corpus. It runs no tests. internal/testsource builds its lint, validity and
// parity checks on these readers and keeps every name it had under the same
// name; corvint-docs reads discovery through this package (decision 0594
// item 8, docs/build-log/2026-11-11-layers-remaining-edges.md).
package testdiscovery
