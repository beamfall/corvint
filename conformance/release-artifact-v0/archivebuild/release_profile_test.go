// SPDX-License-Identifier: Apache-2.0
package archivebuild

import (
	"slices"
	"testing"
)

// RCF-V1-013: compiler inputs are closed even for future environment keys.
func TestEnvironmentUsesReleaseInputs(t *testing.T) {
	for _, key := range []string{"GOFLAGS", "GOOS", "GOARCH", "GOENV", "CGO_ENABLED", "GOEXPERIMENT", "GOFUTUREINPUT"} {
		t.Setenv(key, "foreign")
	}
	env := ReleaseEnvironment("linux", "arm64")
	for _, want := range []string{"GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0", "GOENV=off", "GOFLAGS=-mod=readonly", "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off"} {
		if !slices.Contains(env, want) {
			t.Errorf("missing release setting %s", want)
		}
	}
	for _, key := range []string{"GOFLAGS", "GOOS", "GOARCH", "GOENV", "CGO_ENABLED", "GOEXPERIMENT", "GOFUTUREINPUT"} {
		if slices.Contains(env, key+"=foreign") {
			t.Errorf("inherited %s", key)
		}
	}
}
