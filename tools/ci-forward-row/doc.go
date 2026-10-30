// SPDX-License-Identifier: Apache-2.0

// Package forwardrow holds the focused tests of the AFP-V0-047 forward-row
// scripts in .github/forward-rows: the go-product-shard outcomes and selection
// documents, the AFP-V0-029 row classifier the ci-control-plane forward-rows
// job runs, and the V1-0811 bar count. The scripts are sh and jq only; this
// package runs them against fixtures derived from real CI runs.
package forwardrow
