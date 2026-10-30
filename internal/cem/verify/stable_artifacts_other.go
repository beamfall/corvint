//go:build !darwin && !linux

package verify

import (
	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func stableOpenRoot(string) (*stableArtifactRoot, func(), error) {
	return nil, nil, cemcode.New("unsupported-process-containment", "platform not admitted")
}
func stableReadArtifact(*stableArtifactRoot, string, int) ([]byte, error) {
	return nil, cemcode.New("artifact-unavailable", "platform not admitted")
}
