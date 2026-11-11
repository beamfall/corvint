// Package jstestreceipt is the receipt model of the JS/TS test provider: the
// Playwright and Vitest receipt types, their qualification, screening and
// sensitive-input validation, and the projections onto the test-validity
// vocabulary. It runs nothing. internal/jstestprovider runs the adapters and
// keeps every name it had under the same name; the documentation corpus,
// the flow readers and the test-validity model read receipts through this
// package (decision 0594 item 8, docs/build-log/2026-11-11-layers-remaining-edges.md).
package jstestreceipt
