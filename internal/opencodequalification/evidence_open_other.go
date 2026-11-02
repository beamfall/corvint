//go:build !darwin && !linux

package opencodequalification

// evidenceOpenFlags is empty here; readRegularBounded still checks lstat and the opened file.
const evidenceOpenFlags = 0
