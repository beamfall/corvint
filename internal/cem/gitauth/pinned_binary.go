package gitauth

import (
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// OpenPinned uses a host-validated executable for this repository's bounded
// object reads, including co-process and streamed reads. The caller must protect
// the executable and pin its identity before exposing an author capability.
// The path is a local host assertion, not executable authentication or a sandbox.
func OpenPinned(root string, budget *gitrun.Budget, binary string) (*Repository, error) {
	if !filepath.IsAbs(binary) || filepath.Clean(binary) != binary {
		return nil, unavailable("pinned Git executable requires a canonical absolute path")
	}
	repository, err := Open(root, budget)
	if err != nil {
		return nil, err
	}
	repository.gitBinary = binary
	return repository, nil
}
