package main

import (
	"context"
	"errors"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// compileLegacyCorpus runs the static legacy inventory producer (DCP-V1-060..069). It reads Git
// blobs at --revision and the optional --input registry and writes nothing.
func compileLegacyCorpus(ctx context.Context, o corpusOptions) ([]byte, error) {
	var registry []byte
	if o.input != "" {
		var err error
		if registry, err = doccorpus.ReadFile(o.root, o.input); err != nil {
			return nil, err
		}
	}
	data, err := doccorpus.BuildLegacyInventory(ctx, doccorpus.LegacyInventoryInput{Root: o.root, Runner: o.runner, Revision: o.revision, Path: o.path, Registry: registry})
	if errors.Is(err, doccorpus.ErrLegacyMappingRefused) {
		return data, errCorpusCheckRefused
	}
	return data, err
}
