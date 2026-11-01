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
// source and parses nothing, so its cost is two identity reads, one status
// read and one tree listing, not a read of every tracked file. Tracked and
// Skipped hold exactly the paths an Index built from the same state holds;
// Blobs holds the mode and object ID of every tracked blob whose path is valid
// UTF-8, so a display path that replaced invalid bytes never stands for
// another entry (DCR-V1-411).
type TreeObservation struct {
	Observation
	Tracked map[string]struct{}
	Blobs   map[string]TreeBlob
	Skipped map[string]struct{}
}

// observeTreeStep runs between ObserveTree's opening identity read and its
// status read; tests use it to move HEAD inside that window.
var observeTreeStep = func(attempt int) {}

// ObserveTree reads the repository identity, then the status while the
// identified tree is listed, then the identity again. The status is kept only
// when both identity reads agree, so it was read while HEAD named the listed
// tree; otherwise the observation is repeated, at most three times, as a
// context build retries (DCR-V1-411).
func ObserveTree(ctx context.Context, root string) (*TreeObservation, error) {
	ctx, cancel := withGitDeadline(ctx)
	defer cancel()
	for attempt := 0; attempt < 3; attempt++ {
		opening, err := readIdentity(ctx, root)
		if err != nil {
			return nil, err
		}
		observeTreeStep(attempt)
		type listing struct {
			tracked, skipped map[string]struct{}
			blobs            map[string]TreeBlob
			err              error
		}
		listed := make(chan listing, 1)
		go func() {
			var l listing
			l.tracked, l.blobs, l.skipped, l.err = readTreeBlobs(ctx, root, opening)
			listed <- l
		}()
		dirtyPaths, statusSHA256, statusErr := readStatusSnapshot(ctx, root)
		var closing repositoryIdentity
		var closingErr error
		if statusErr == nil {
			closing, closingErr = readIdentity(ctx, root)
		}
		l := <-listed
		switch {
		case statusErr != nil:
			return nil, statusErr
		case l.err != nil:
			return nil, l.err
		case closingErr != nil:
			return nil, closingErr
		case closing != opening:
			continue
		}
		dirty := make(map[string]struct{}, len(dirtyPaths))
		for _, item := range dirtyPaths {
			dirty[item] = struct{}{}
		}
		return &TreeObservation{
			Observation: Observation{ObjectFormat: opening.objectFormat, CommitRevision: opening.commitRevision, Revision: opening.treeRevision, StatusSHA256: statusSHA256, DirtyPaths: keys(dirty)},
			Tracked:     l.tracked, Blobs: l.blobs, Skipped: l.skipped,
		}, nil
	}
	return nil, &Error{Message: "repository HEAD changed while observing the checkout"}
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
