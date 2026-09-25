package appflows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// treeEntry is one direct child of the flows directory at a commit.
type treeEntry struct {
	name string
	mode string
	kind string
	oid  string
	size string
}

// LoadIntentsAt reads and validates the intents committed in dir at revision, never the working
// tree, so a dirty or untracked intent file cannot reach an export that names the revision
// (AFU-V1-001, AFU-V1-003). Every entry keeps the working-tree bounds: count before any read,
// size, Decode and the secret screen.
func LoadIntentsAt(ctx context.Context, root, dir, revision string) (IntentSet, error) {
	dir = filepath.ToSlash(dir)
	if !safePath(dir) || dir == "." {
		return IntentSet{}, errors.New("--flows must name a directory inside the repository root")
	}
	rev, err := ResolveRevision(ctx, root, revision)
	if err != nil {
		return IntentSet{}, err
	}
	entries, err := flowTree(ctx, root, rev, dir)
	if err != nil {
		return IntentSet{}, err
	}
	candidates := []treeEntry{}
	for _, e := range entries {
		if intentCandidate(e.name) {
			candidates = append(candidates, e)
		}
	}
	if err = candidateBound(len(candidates)); err != nil {
		return IntentSet{}, err
	}
	set := emptySet(dir)
	set.Revision = rev
	for _, e := range candidates {
		raw, err := committedBlob(ctx, root, e)
		if err != nil {
			return set, err
		}
		if err = set.add(e.name, raw); err != nil {
			return set, err
		}
	}
	return set, set.validate()
}

func flowTree(ctx context.Context, root, rev, dir string) ([]treeEntry, error) {
	out, err := git(ctx, root, "--literal-pathspecs", "ls-tree", "-z", "-l", "--full-tree", rev, "--", dir+"/")
	if err != nil {
		return nil, errors.New("flows directory unreadable at the evaluated revision")
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("flows directory %s is absent at the evaluated revision; commit it before exporting", dir)
	}
	entries := []treeEntry{}
	for _, line := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		meta, name, _ := strings.Cut(string(line), "\t")
		fields := strings.Fields(meta)
		if len(fields) != 4 || path.Dir(name) != dir {
			return nil, errors.New("flows directory unreadable at the evaluated revision")
		}
		entries = append(entries, treeEntry{name: path.Base(name), mode: fields[0], kind: fields[1], oid: fields[2], size: fields[3]})
	}
	return entries, nil
}

// committedBlob reads one regular committed file after checking its recorded size (AFU-V1-036).
func committedBlob(ctx context.Context, root string, e treeEntry) ([]byte, error) {
	regular := e.kind == "blob" && (e.mode == "100644" || e.mode == "100755")
	if !regular {
		return nil, fmt.Errorf("%s: flow input must be a regular committed file", e.name)
	}
	size, err := strconv.Atoi(e.size)
	if err != nil || size > MaxBytes {
		return nil, fmt.Errorf("%s: flow input exceeds byte limit", e.name)
	}
	raw, err := git(ctx, root, "cat-file", "blob", e.oid)
	if err != nil || len(raw) != size {
		return nil, fmt.Errorf("%s: committed flow input unavailable", e.name)
	}
	return raw, nil
}
