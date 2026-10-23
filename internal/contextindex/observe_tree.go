package contextindex

import (
	"bytes"
	"context"
	"unicode/utf8"
)

// TreeBlob is one blob entry of a tree listing: its mode and object ID.
type TreeBlob struct {
	Mode, OID string
}

// TreeObservation is Observe plus the listing of the observed tree. It opens no
// source and parses nothing, so its cost is one paired identity and status read
// and one tree listing, not a read of every tracked file. Tracked and Skipped
// hold exactly the paths an Index built from the same state holds; Blobs holds
// the mode and object ID of every tracked blob whose path is valid UTF-8, so a
// display path that replaced invalid bytes never stands for another entry
// (DCP-V1-411).
type TreeObservation struct {
	Observation
	Tracked map[string]struct{}
	Blobs   map[string]TreeBlob
	Skipped map[string]struct{}
}

// ObserveTree reads the repository identity and status as Observe does and
// lists the observed tree under the same bounds Build applies.
func ObserveTree(ctx context.Context, root string) (*TreeObservation, error) {
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	observed := observeRepository(ctx, root)
	if observed.identityErr != nil {
		return nil, observed.identityErr
	}
	if observed.statusErr != nil {
		return nil, observed.statusErr
	}
	tracked, blobs, skipped, err := readTreeBlobs(ctx, root, observed.identity)
	if err != nil {
		return nil, err
	}
	dirty := make(map[string]struct{}, len(observed.dirty))
	for _, item := range observed.dirty {
		dirty[item] = struct{}{}
	}
	return &TreeObservation{
		Observation: Observation{ObjectFormat: observed.identity.objectFormat, CommitRevision: observed.identity.commitRevision, Revision: observed.identity.treeRevision, StatusSHA256: observed.statusSHA256, DirtyPaths: keys(dirty)},
		Tracked:     tracked, Blobs: blobs, Skipped: skipped,
	}, nil
}

// readTreeBlobs is readTreeEntries without the blob sizes: `ls-tree -l` opens
// every blob's object header, which costs more than the listing itself. It
// applies the same output, object-ID and source-count bounds and classifies
// non-blob entries the same way.
func readTreeBlobs(ctx context.Context, root string, identity repositoryIdentity) (map[string]struct{}, map[string]TreeBlob, map[string]struct{}, error) {
	raw, err := git(ctx, root, maxTreeBytes, nil, "ls-tree", "-r", "-z", "--full-tree", identity.treeRevision)
	if err != nil {
		return nil, nil, nil, classifyMissingObjects(ctx, root, err, gitStderr(err), identity.treeRevision)
	}
	count := bytes.Count(raw, []byte{0})
	tracked := make(map[string]struct{}, count)
	blobs := make(map[string]TreeBlob, count)
	skipped := map[string]struct{}{}
	for item := range bytes.SplitSeq(raw, []byte{0}) {
		if len(item) == 0 {
			continue
		}
		tab := bytes.IndexByte(item, '\t')
		if tab < 0 {
			return nil, nil, nil, &Error{Message: "Git tree output is malformed"}
		}
		fields := bytes.Fields(item[:tab])
		if len(fields) != 3 {
			return nil, nil, nil, &Error{Message: "Git tree output is malformed"}
		}
		path := displayPath(item[tab+1:])
		if string(fields[1]) != "blob" || string(fields[0]) == "160000" {
			skipped[path] = struct{}{}
			continue
		}
		oid := string(fields[2])
		if !validObjectID(oid, identity.objectFormat) {
			return nil, nil, nil, &Error{Message: "Git returned an invalid blob object identity"}
		}
		tracked[path] = struct{}{}
		if utf8.Valid(item[tab+1:]) {
			blobs[path] = TreeBlob{Mode: treeEntryMode(fields[0]), OID: oid}
		}
		if len(tracked) > maxIndexedSources {
			return nil, nil, nil, &Error{Message: "Git tree exceeds the source-count limit"}
		}
	}
	return tracked, blobs, skipped, nil
}
