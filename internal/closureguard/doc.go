// Package closureguard holds the repository's `go list` import-closure guards
// (AFP-V0-9600, V1-5071). Each guard is a self-contained test that lists a
// command's dependency closure from the module root and checks which packages
// it reaches. The package imports only standard packages the affected-plan
// classifier allows and has no other read channel, so it is a proven
// import-graph reader (AFP-V0-9010): a body-only edit elsewhere does not
// select it, and an import-set change still does.
package closureguard
