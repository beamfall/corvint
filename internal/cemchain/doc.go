// Package cemchain holds the end-to-end fixture test of the bounded cem/1.0
// chain (CEM-V1-286, CEM-V1-287): a native Tasks attempt, a trusted runner
// plan and receipt, the plain stable producer with runner admission, and the
// Tasks cem-verdict gate that recomputes the verdict natively and through the
// independently authored portable consumer. It has no runtime code; it is its
// own package so that test may build those binaries from repository sources,
// which the read-confined Tasks and producer packages may not.
package cemchain
