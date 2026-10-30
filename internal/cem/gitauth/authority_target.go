package gitauth

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// RequireCleanTarget checks staged/untracked state and hashes raw worktree
// bytes against the target tree. git status/diff-files may execute claimant
// clean filters or hide dirty bytes; they are deliberately not used here.
// Unsupported symlinks/submodules or bounds leave currentness unavailable.
func (r *Repository) RequireTargetState(ctx context.Context, target string) error {
	head, err := r.Resolve(ctx, "HEAD")
	if err != nil || head != target {
		return unavailable("authority target drift")
	}
	staged, err := r.git(ctx, 64<<10, "--work-tree="+r.Root, "diff-index", "--cached", "--raw", "-z", "--no-renames", "--no-ext-diff", target, "--")
	if err != nil || len(staged) != 0 {
		return unavailable("authority staged drift")
	}
	untracked, err := r.git(ctx, 64<<10, "--work-tree="+r.Root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil || len(untracked) != 0 {
		return unavailable("authority untracked drift")
	}
	return nil
}

// RequireCleanTarget adds one complete raw-file observation to current Git state.
func (r *Repository) RequireCleanTarget(ctx context.Context, target string) error {
	if err := r.RequireTargetState(ctx, target); err != nil {
		return err
	}
	expected := r
	if r.objectView != nil {
		expected = r.objectView
	}
	tree, err := expected.git(ctx, 4<<20, "ls-tree", "-r", "-z", target)
	if err != nil {
		return err
	}
	root, err := openAuthorityRoot(r.Root)
	if err != nil {
		return unavailable("authority worktree unavailable")
	}
	defer root.Close()
	rootInfo, err := root.Stat(".")
	namedRoot, namedErr := os.Lstat(r.Root)
	if err != nil || namedErr != nil || !sameAuthorityMetadata(rootInfo, namedRoot) {
		return unavailable("authority root changed during open")
	}
	rows := bytes.Split(tree, []byte{0})
	if len(rows) > 8193 || len(rows) < 2 || len(rows[len(rows)-1]) != 0 {
		return unavailable("authority tree bound")
	}
	parents := newAuthorityParents(ctx, r.Root, root, rootInfo)
	defer parents.close()
	if err = scanAuthorityRows(ctx, parents, rows[:len(rows)-1]); err != nil {
		return err
	}
	if err = parents.check(); err != nil {
		return err
	}
	namedRoot, err = os.Lstat(r.Root)
	if err != nil || !sameAuthorityMetadata(rootInfo, namedRoot) {
		return unavailable("authority root changed during read")
	}
	return r.RequireTargetState(ctx, target)
}

func rawFileMatches(ctx context.Context, root *os.Root, path, mode, oid string, remaining *int64) error {
	file, err := openAuthorityFileContext(ctx, root, path)
	if err != nil {
		return unavailable("authority worktree file unavailable")
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > 16<<20 || before.Size() > *remaining {
		return unavailable("authority file bound")
	}
	if (before.Mode()&0111 != 0) != (mode == "100755") {
		return unavailable("authority mode drift")
	}
	*remaining -= before.Size()
	return hashAuthorityFile(ctx, file, before, mode, oid, make([]byte, 64<<10))
}

func hashAuthorityFile(ctx context.Context, file *authorityFile, before os.FileInfo, mode, oid string, buffer []byte) error {
	var digest hash.Hash
	switch len(oid) {
	case 40:
		digest = sha1.New()
	case 64:
		digest = sha256.New()
	default:
		return unavailable("authority object format")
	}
	fmt.Fprintf(digest, "blob %d\x00", before.Size())
	reader := io.LimitReader(file, before.Size()+1)
	total := int64(0)
	for {
		if ctx.Err() != nil {
			return unavailable("authority snapshot cancelled")
		}
		n, readErr := reader.Read(buffer)
		if n > 0 {
			digest.Write(buffer[:n])
			total += int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return unavailable("authority file read unavailable")
		}
	}
	after, err := file.Stat()
	if err != nil || !sameAuthorityMetadata(before, after) || (after.Mode()&0111 != 0) != (mode == "100755") || total != before.Size() {
		return unavailable("authority file changed during read")
	}
	if err := file.validate(ctx); err != nil {
		return err
	}
	if hex.EncodeToString(digest.Sum(nil)) != oid {
		return unavailable("authority worktree drift")
	}
	return nil
}

// Each window joins acquisition before ordered admission, hashes only its
// admitted prefix, and joins every hash before choosing the earliest row error.
func scanAuthorityRows(ctx context.Context, parents *authorityParents, rows [][]byte) error {
	type prepared struct {
		parent          *authorityParent
		file            *authorityFile
		before          os.FileInfo
		mode, oid, leaf string
		err             error
	}
	buffers := [8][]byte{}
	for i := range buffers {
		buffers[i] = make([]byte, 64<<10)
	}
	remaining := int64(128 << 20)
	for begin := 0; begin < len(rows); begin += 8 {
		end := min(begin+8, len(rows))
		items := make([]prepared, end-begin)
		var opened sync.WaitGroup
		for j := range items {
			row := rows[begin+j]
			header, path, ok := bytes.Cut(row, []byte{'\t'})
			fields := strings.Fields(string(header))
			if !ok || len(fields) != 3 || fields[1] != "blob" || wire.ValidatePath(string(path)) != nil {
				items[j].err = unavailable("authority unsupported entry")
				continue
			}
			if fields[0] != "100644" && fields[0] != "100755" {
				items[j].err = unavailable("authority unsupported mode")
				continue
			}
			parentPath, leaf := authoritySplit(string(path))
			parent, err := parents.get(parentPath)
			if err != nil {
				items[j].err = err
				continue
			}
			items[j].parent = parent
			items[j].leaf = leaf
			items[j].mode = fields[0]
			items[j].oid = fields[2]
			opened.Add(1)
			go func(j int) {
				defer opened.Done()
				item := &items[j]
				item.file, item.err = openAuthorityFileContext(ctx, item.parent.root, item.leaf)
				if item.err == nil {
					item.before, item.err = item.file.Stat()
				}
			}(j)
		}
		opened.Wait()
		admitted := 0
		for j := range items {
			item := &items[j]
			if ctx.Err() != nil {
				item.err = unavailable("authority snapshot cancelled")
			}
			if item.err != nil {
				break
			}
			info := item.before
			if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > 16<<20 || info.Size() > remaining {
				item.err = unavailable("authority file bound")
				break
			}
			if (info.Mode()&0111 != 0) != (item.mode == "100755") {
				item.err = unavailable("authority mode drift")
				break
			}
			remaining -= info.Size()
			admitted++
		}
		for j := admitted; j < len(items); j++ {
			if items[j].file != nil {
				items[j].file.Close()
				items[j].file = nil
			}
		}
		var hashed sync.WaitGroup
		for j := 0; j < admitted; j++ {
			hashed.Add(1)
			go func(j int) {
				defer hashed.Done()
				item := &items[j]
				item.err = hashAuthorityFile(ctx, item.file, item.before, item.mode, item.oid, buffers[j])
				item.file.Close()
			}(j)
		}
		hashed.Wait()
		for j := range items {
			if items[j].parent != nil {
				parents.release(items[j].parent)
			}
		}
		for j := range items {
			if items[j].err != nil {
				return items[j].err
			}
		}
	}
	return nil
}
