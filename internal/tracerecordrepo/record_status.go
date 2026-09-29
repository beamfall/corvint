package tracerecordrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/trace"
)

func recordSourceStatus(ctx context.Context, root, observedSHA string, dirty []string, objectIDLength int, ownedStage string) error {
	if len(dirty) == 0 {
		if observedSHA != fmt.Sprintf("%x", sha256.Sum256(nil)) {
			return &repositoryChangedError{}
		}
		return nil
	}
	// Expand the exact status observation, never refresh its authority. A race
	// between probes refuses even if both versions contain only private paths.
	raw, err := runGit(ctx, gitrun.NewDefaultBudget(), root, 8<<20, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return err
	}
	paths, err := privateStatusPaths(raw, observedSHA, objectIDLength)
	if err != nil {
		return err
	}
	return trace.ValidateRecordStatusPaths(root, paths, objectIDLength, ownedStage)
}

func privateStatusPaths(raw []byte, observedSHA string, objectIDLength int) ([]string, error) {
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != observedSHA || len(raw) == 0 || raw[len(raw)-1] != 0 {
		return nil, &repositoryChangedError{}
	}
	fields := bytes.Split(raw[:len(raw)-1], []byte{0})
	if len(fields) > trace.MaxTraceFiles+2 {
		return nil, &repositoryChangedError{}
	}
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		// Refuse every tracked status, including either endpoint of a rename;
		// only exact untracked records can be private recorder artifacts.
		if !bytes.HasPrefix(field, []byte("?? ")) || !trace.IsRecordArtifactPath(string(field[3:]), objectIDLength) {
			return nil, &repositoryChangedError{}
		}
		paths = append(paths, string(field[3:]))
	}
	return paths, nil
}
