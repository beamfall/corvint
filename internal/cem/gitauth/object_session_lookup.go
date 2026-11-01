package gitauth

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// sessionReader asks the object session's co-process for one record.
type sessionReader func(request string, admit func(header string, fields []string, size int) bool) (string, []byte, bool, error)

// batchReplay runs a one-shot streamed batch, charging or reusing its
// operation.
type batchReplay func(options gitrun.Options, consumer io.Writer, args []string) error

// sessionTreeEntry is lookupTreeEntry inside an object session (DCR-V1-985,
// V1-2745). One operation charge covers the lookup, as the one-shot batch it
// replaces would: the session's co-process answers the walk, and when it
// cannot, the original one-shot batch replays under the same reservation, so
// every refusal and error is the one-shot's own.
func (r *Repository) sessionTreeEntry(ctx context.Context, revision string, components, prefixes, requests []string) (TreeEntry, bool, error) {
	read, replay, err := r.reserveSessionOperation(ctx)
	if err != nil {
		return TreeEntry{}, false, err
	}
	entry, exists, served, err := r.servedTreeEntry(ctx, read, revision, components)
	if err != nil || served {
		return entry, exists, err
	}
	stream, err := r.runCommitBatch(prefixes, requests, replay)
	if err != nil {
		return TreeEntry{}, false, err
	}
	return walkTreeRecords(ctx, stream, components)
}

// reserveSessionOperation charges one logical operation and returns the
// session read and one-shot replay that share it, as sessionRead does.
func (r *Repository) reserveSessionOperation(ctx context.Context) (sessionReader, batchReplay, error) {
	options := r.gitOptions(0, nil)
	batch := append(r.pinnedArgs(), "cat-file", "--batch")
	if r.budget.Stable() {
		reservation, err := r.budget.Reserve()
		if err != nil {
			return nil, nil, err
		}
		read := func(request string, admit func(string, []string, int) bool) (string, []byte, bool, error) {
			return r.objects.ReadReserved(ctx, reservation, options, batch, request, admit)
		}
		replay := func(options gitrun.Options, consumer io.Writer, args []string) error {
			return gitrun.RunReservationStream(ctx, r.budget, reservation, options, consumer, args...)
		}
		return read, replay, nil
	}
	perOp, err := r.budget.ReserveOperation(0)
	if err != nil {
		return nil, nil, err
	}
	deadline := time.Now().Add(perOp)
	read := func(request string, admit func(string, []string, int) bool) (string, []byte, bool, error) {
		return r.objects.Read(ctx, time.Until(deadline), options, batch, request, admit)
	}
	replay := func(options gitrun.Options, consumer io.Writer, args []string) error {
		return gitrun.RunReservedStream(ctx, perOp, options, consumer, args...)
	}
	return read, replay, nil
}

// servedTreeEntry walks the path through the session by object ID: the
// peeled commit, its tree, then each directory tree named by its verified
// parent's entry. It answers (served) only when every record is present,
// self-hashes to its name and the one-shot batch's trailing records would
// keep the MaxTreeBytes allowance; an absent or non-tree directory counts
// the one-shot batch's remaining `missing` records toward it. Otherwise
// served is false and the caller replays the one-shot batch.
func (r *Repository) servedTreeEntry(ctx context.Context, read sessionReader, revision string, components []string) (TreeEntry, bool, bool, error) {
	width := 0
	switch r.ObjectFormat {
	case "sha1":
		width = 40
	case "sha256":
		width = 64
	}
	commit, body, _, ok, err := servedObject(ctx, read, revision+"^{commit}", "commit", "", width, MaxTreeBytes)
	if err != nil || !ok {
		return TreeEntry{}, false, false, err
	}
	width = len(commit)
	line, _, found := bytes.Cut(body, []byte{'\n'})
	if !found || len(line)+1 != width+6 || !bytes.HasPrefix(line, []byte("tree ")) || !wire.IsGitOid(string(line[5:])) {
		return TreeEntry{}, false, false, nil
	}
	tail := 0
	_, body, used, ok, err := servedObject(ctx, read, string(line[5:]), "tree", string(line[5:]), width, MaxTreeBytes)
	if err != nil || !ok {
		return TreeEntry{}, false, false, err
	}
	tail += used
	parents := components[:len(components)-1]
	for i, name := range parents {
		entry, exists, err := findTreeEntry(body, name, width/2)
		if err != nil {
			return TreeEntry{}, false, false, nil
		}
		if !exists || entry.Type != "tree" {
			for j := i; j < len(parents); j++ {
				tail += len(revision) + 1 + len(strings.Join(parents[:j+1], "/")) + len(" missing\n")
			}
			return TreeEntry{}, false, tail <= MaxTreeBytes, nil
		}
		_, body, used, ok, err = servedObject(ctx, read, entry.OID, "tree", entry.OID, width, MaxTreeBytes-tail)
		if err != nil || !ok {
			return TreeEntry{}, false, false, err
		}
		tail += used
	}
	entry, exists, err := findTreeEntry(body, components[len(components)-1], width/2)
	if err != nil {
		return TreeEntry{}, false, false, nil
	}
	return entry, exists, true, nil
}

// servedObject reads one session record of kind whose printed bytes fit
// room and self-hash to its object ID (want, when named); used is the
// record's length in one-shot batch output.
func servedObject(ctx context.Context, read sessionReader, request, kind, want string, width, room int) (string, []byte, int, bool, error) {
	admit := func(header string, fields []string, size int) bool {
		return fields[1] == kind && (want == "" || fields[0] == want) && wire.IsGitOid(fields[0]) &&
			(width == 0 || len(fields[0]) == width) && len(header)+size+2 <= room
	}
	header, body, ok, err := read(request, admit)
	if err != nil || !ok {
		return "", nil, 0, false, err
	}
	oid, _, _ := strings.Cut(header, " ")
	if requireObjectIdentity(ctx, kind, oid, body) != nil {
		return "", nil, 0, false, nil
	}
	return oid, body, len(header) + len(body) + 2, true, nil
}
