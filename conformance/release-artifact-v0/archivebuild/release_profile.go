// SPDX-License-Identifier: Apache-2.0
package archivebuild

import "os"

// ReleaseTrimpath is the path-independent build flag used for release and qualification.
const ReleaseTrimpath = "-trimpath"

// ReleaseEnvironment fixes the target, build flags and offline module settings.
// Only toolchain, cache and temporary directory locations come from the host.
func ReleaseEnvironment(goos, goarch string) []string {
	pairs := []string{
		"CGO_ENABLED=0", "GOENV=off", "GOFLAGS=-mod=readonly", "GOPROXY=off",
		"GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOOS=" + goos, "GOARCH=" + goarch,
	}
	for _, key := range []string{"HOME", "PATH", "TMPDIR", "TEMP", "TMP", "GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT", "SystemRoot", "USERPROFILE"} {
		if value, ok := os.LookupEnv(key); ok {
			pairs = append(pairs, key+"="+value)
		}
	}
	return pairs
}
