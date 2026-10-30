package verify

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// stableArtifacts reads every artifact twice and returns the checks and the
// verified bytes by digest, which receipt authentication then reads.
func stableArtifacts(ctx context.Context, path string, artifacts []wire.StableArtifact) ([]StableArtifactCheck, map[string][]byte, error) {
	checks := []StableArtifactCheck{}
	contents := map[string][]byte{}
	// The explicit root remains required even for the empty set. No filesystem
	// discovery or synthetic artifact is needed for that empty declaration.
	if len(artifacts) == 0 {
		return checks, contents, nil
	}
	root, closeRoot, e := stableOpenRoot(path)
	if e != nil {
		return nil, nil, cemcode.New("artifact-unavailable", "artifact root")
	}
	defer closeRoot()
	first := make([][]byte, len(artifacts))
	for pass := 0; pass < 2; pass++ {
		total := 0
		for i, a := range artifacts {
			if ctx.Err() != nil {
				return nil, nil, cemcode.New("verification-timeout", "artifact deadline")
			}
			b, e := stableReadArtifact(root, a.Path, wire.CandidateMaxArtifactBytes)
			if e != nil {
				return nil, nil, e
			}
			total += len(b)
			if total > wire.CandidateMaxTotalArtifactBytes {
				return nil, nil, cemcode.New("artifact-resource-limit", "artifact aggregate")
			}
			if ctx.Err() != nil {
				return nil, nil, cemcode.New("verification-timeout", "artifact deadline")
			}
			if pass == 1 && !bytes.Equal(first[i], b) {
				return nil, nil, cemcode.New("artifact-changed-during-verification", "artifact changed")
			}
			if stableDigest(b) != a.Sha256 {
				return nil, nil, cemcode.New("artifact-digest-mismatch", "artifact mismatch")
			}
			if pass == 0 {
				first[i] = b
			} else {
				checks = append(checks, StableArtifactCheck{a.Kind, a.Path, a.Sha256, len(b)})
				contents[a.Sha256] = b
			}
		}
	}
	return checks, contents, nil
}
func stableReadBoundedFile(f *os.File, bound int) ([]byte, error) {
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() {
		return nil, cemcode.New("artifact-unavailable", "regular artifact required")
	}
	if st.Size() > int64(bound) {
		return nil, cemcode.New("artifact-resource-limit", "artifact bound")
	}
	b, e := io.ReadAll(io.LimitReader(f, int64(bound)+1))
	if e != nil {
		return nil, cemcode.New("artifact-unavailable", "artifact read")
	}
	if len(b) > bound {
		return nil, cemcode.New("artifact-resource-limit", "artifact bound")
	}
	return b, nil
}

// stableArtifactRoot pins the opened directory and checks its original named
// ancestry before and after each read. It does not claim hostile ABA immunity.
type stableArtifactRoot struct {
	root     *os.Root
	validate func() error
}

// RecheckStableArtifacts re-reads, under the same bounds and double read as
// Stable, every artifact the stable map raw declares from root and returns
// their checks. A caller that reuses an accepted verdict for the same map
// bytes compares them with the verdict's artifactChecks, because the artifact
// root is a mutable worktree that no map or commit digest covers (OCM-V0-027).
func RecheckStableArtifacts(ctx context.Context, root string, raw []byte) ([]StableArtifactCheck, error) {
	document, err := wire.ParseStable(raw)
	if err != nil {
		return nil, err
	}
	checks, _, err := stableArtifacts(ctx, root, document.Artifacts)
	return checks, err
}
