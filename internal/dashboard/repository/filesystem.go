package repository

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxExecutableBytes uint64 = 256 << 20

func openDirectory(path string) (directoryBinding, *Failure) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return directoryBinding{}, unavailable(ReasonUnavailable)
	}
	base, components := directoryBaseAndComponents(path)
	currentRoot, err := os.OpenRoot(base)
	if err != nil {
		return directoryBinding{}, unavailable(ReasonUnavailable)
	}
	currentPath := base
	current, ok := bindRootDirectory(currentRoot, currentPath)
	if !ok {
		_ = currentRoot.Close()
		return directoryBinding{}, unavailable(ReasonUnavailable)
	}
	for _, component := range components {
		childPath := filepath.Join(currentPath, component)
		child, failure := openChildDirectory(current, component, childPath)
		if failure != nil {
			closeDirectoryBinding(&current)
			return directoryBinding{}, failure
		}
		closeDirectoryBinding(&current)
		currentPath = childPath
		current = child
	}
	current.path = path
	return current, nil
}

func openChildDirectory(parent directoryBinding, name, path string) (directoryBinding, *Failure) {
	if parent.root == nil || name == "" || strings.ContainsAny(name, "/\\") {
		return directoryBinding{}, unavailable(ReasonUnavailable)
	}
	before, err := parent.root.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return directoryBinding{}, unavailable(ReasonUnavailable)
	}
	childFile, childRoot, opened := openNoFollowDirectory(parent.root, name)
	if !opened {
		return directoryBinding{}, unavailable(ReasonUnavailable)
	}
	childInfo, statErr := childFile.Stat()
	childIdentity, bound := identityFromFile(childFile, childInfo)
	childDirectory, directoryBound := directoryIdentityFromFile(childFile, childInfo)
	beforeDirectory, beforeQualified := directoryIdentityFromFile(childFile, before)
	child := directoryBinding{path: path, file: childFile, root: childRoot, identity: childIdentity}
	if statErr != nil || !bound || !directoryBound || !childInfo.IsDir() {
		closeDirectoryBinding(&child)
		return directoryBinding{}, unavailable(ReasonUnavailable)
	}
	// Path re-identification compares directory identity only (decision 0175):
	// a sibling entry creation changes modification time, size, and link count
	// without changing what the path resolves to.
	if !beforeQualified || beforeDirectory != childDirectory {
		closeDirectoryBinding(&child)
		return directoryBinding{}, changed()
	}
	after, afterErr := parent.root.Lstat(name)
	afterDirectory, afterQualified := directoryIdentityFromFile(childFile, after)
	if afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !afterQualified ||
		!os.SameFile(before, after) || afterDirectory != childDirectory {
		closeDirectoryBinding(&child)
		return directoryBinding{}, changed()
	}
	return child, nil
}

func directoryBaseAndComponents(value string) (string, []string) {
	volume := filepath.VolumeName(value)
	remainder := strings.TrimLeft(value[len(volume):], "/\\")
	base := string(os.PathSeparator)
	if volume != "" {
		base = volume + string(os.PathSeparator)
	}
	components := strings.FieldsFunc(remainder, func(character rune) bool {
		return character == '/' || character == '\\'
	})
	return base, components
}

func bindRootDirectory(root *os.Root, path string) (directoryBinding, bool) {
	if root == nil {
		return directoryBinding{}, false
	}
	file, err := root.Open(".")
	if err != nil {
		return directoryBinding{}, false
	}
	info, err := file.Stat()
	identity, qualified := identityFromFile(file, info)
	if err != nil || !qualified || !info.IsDir() {
		_ = file.Close()
		return directoryBinding{}, false
	}
	return directoryBinding{path: path, file: file, root: root, identity: identity}, true
}

func closeDirectoryBinding(binding *directoryBinding) {
	if binding == nil {
		return
	}
	if binding.file != nil {
		_ = binding.file.Close()
	}
	if binding.root != nil {
		_ = binding.root.Close()
	}
	*binding = directoryBinding{}
}

func reopenDirectory(binding directoryBinding) (directoryBinding, *Failure) {
	reopened, failure := openDirectory(binding.path)
	if failure != nil {
		return directoryBinding{}, failure
	}
	if reopened.identity != binding.identity {
		closeDirectoryBinding(&reopened)
		return directoryBinding{}, unavailable(ReasonChanged)
	}
	return reopened, nil
}

func retainedDirectoryStable(binding directoryBinding) bool {
	if binding.file == nil || binding.root == nil {
		return false
	}
	info, err := binding.file.Stat()
	identity, qualified := identityFromFile(binding.file, info)
	if err != nil || !qualified || identity != binding.identity {
		return false
	}
	reopened, failure := reopenDirectory(binding)
	if failure != nil {
		return false
	}
	closeDirectoryBinding(&reopened)
	return true
}

func retainedLayoutStable(value layout) bool {
	return retainedDirectoryStable(value.worktree) &&
		retainedDirectoryStable(value.gitDir) &&
		retainedDirectoryStable(value.common) &&
		retainedDirectoryStable(value.objects)
}

func closeLayout(value *layout) {
	if value == nil {
		return
	}
	for _, file := range []*os.File{value.worktree.file, value.gitDir.file, value.common.file, value.objects.file} {
		if file != nil {
			_ = file.Close()
		}
	}
	for _, root := range []*os.Root{value.worktree.root, value.gitDir.root, value.common.root, value.objects.root} {
		if root != nil {
			_ = root.Close()
		}
	}
	*value = layout{}
}

func openExecutableStableRead(ctx context.Context, path string) (*os.File, executableEvidence, *Failure) {
	parent, parentFailure := openDirectory(filepath.Dir(path))
	if parentFailure != nil {
		return nil, executableEvidence{}, parentFailure
	}
	defer closeDirectoryBinding(&parent)
	name := filepath.Base(path)
	beforePath, err := parent.root.Lstat(name)
	if err != nil || beforePath.Mode()&os.ModeSymlink != 0 || !beforePath.Mode().IsRegular() || beforePath.Size() < 0 || uint64(beforePath.Size()) > maxExecutableBytes {
		return nil, executableEvidence{}, unavailable(ReasonUnavailable)
	}
	file, opened := openNoFollowFile(parent.root, name)
	if !opened {
		return nil, executableEvidence{}, unavailable(ReasonUnavailable)
	}
	fail := func() (*os.File, executableEvidence, *Failure) {
		_ = file.Close()
		return nil, executableEvidence{}, unavailable(ReasonUnavailable)
	}
	failChanged := func() (*os.File, executableEvidence, *Failure) {
		_ = file.Close()
		return nil, executableEvidence{}, changed()
	}

	stat1, err := file.Stat()
	if err != nil || !stat1.Mode().IsRegular() || stat1.Size() < 0 || uint64(stat1.Size()) > maxExecutableBytes {
		return fail()
	}
	if !os.SameFile(beforePath, stat1) {
		return failChanged()
	}
	identity1, qualified := identityFromFile(file, stat1)
	if !qualified {
		return fail()
	}
	count1, digest1, ok := readExecutablePass(ctx, file)
	if !ok {
		return fail()
	}
	stat2, err := file.Stat()
	if err != nil {
		return fail()
	}
	identity2, qualified := identityFromFile(file, stat2)
	if !qualified || identity2 != identity1 {
		return failChanged()
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fail()
	}
	stat3, err := file.Stat()
	if err != nil {
		return fail()
	}
	identity3, qualified := identityFromFile(file, stat3)
	if !qualified || identity3 != identity1 {
		return failChanged()
	}
	count2, digest2, ok := readExecutablePass(ctx, file)
	if !ok {
		return fail()
	}
	stat4, err := file.Stat()
	if err != nil {
		return fail()
	}
	identity4, qualified := identityFromFile(file, stat4)
	if !qualified || identity4 != identity1 || count1 != count2 || digest1 != digest2 {
		return failChanged()
	}
	afterPath, err := parent.root.Lstat(name)
	afterIdentity, afterQualified := identityFromFile(file, afterPath)
	if err != nil || afterPath.Mode()&os.ModeSymlink != 0 || !os.SameFile(beforePath, afterPath) ||
		!afterQualified || afterIdentity != identity1 {
		return failChanged()
	}
	return file, executableEvidence{identity: identity1, bytes: count1, digest: digest1}, nil
}

func readExecutablePass(ctx context.Context, file *os.File) (uint64, [sha256.Size]byte, bool) {
	return readDigestPass(ctx, file, maxExecutableBytes)
}

func readDigestPass(ctx context.Context, file *os.File, limit uint64) (uint64, [sha256.Size]byte, bool) {
	buffer := make([]byte, 64<<10)
	reader := &contextReader{ctx: ctx, reader: file}
	hash := sha256.New()
	var count uint64
	for {
		remaining := limit + 1 - count
		chunk := buffer
		if uint64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		readCount, err := reader.Read(chunk)
		if readCount > 0 {
			count += uint64(readCount)
			_, _ = hash.Write(chunk[:readCount])
			if count > limit {
				return 0, [sha256.Size]byte{}, false
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil || readCount == 0 {
			return 0, [sha256.Size]byte{}, false
		}
	}
	if count > limit {
		return 0, [sha256.Size]byte{}, false
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return count, digest, true
}

func verifyExecutableBinding(path string, held *os.File, baseline executableEvidence) bool {
	if held == nil {
		return false
	}
	heldInfo, err := held.Stat()
	if err != nil || !heldInfo.Mode().IsRegular() {
		return false
	}
	heldIdentity, ok := identityFromFile(held, heldInfo)
	if !ok || heldIdentity != baseline.identity || uint64(heldInfo.Size()) != baseline.bytes {
		return false
	}
	parent, failure := openDirectory(filepath.Dir(path))
	if failure != nil {
		return false
	}
	defer closeDirectoryBinding(&parent)
	name := filepath.Base(path)
	pathInfo, err := parent.root.Lstat(name)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return false
	}
	reopened, opened := openNoFollowFile(parent.root, name)
	if !opened {
		return false
	}
	defer reopened.Close()
	reopenedInfo, err := reopened.Stat()
	if err != nil || !os.SameFile(pathInfo, reopenedInfo) {
		return false
	}
	reopenedIdentity, ok := identityFromFile(reopened, reopenedInfo)
	afterPath, err := parent.root.Lstat(name)
	afterIdentity, afterQualified := identityFromFile(reopened, afterPath)
	return ok && err == nil && afterQualified && afterPath.Mode()&os.ModeSymlink == 0 &&
		os.SameFile(pathInfo, afterPath) && reopenedIdentity == baseline.identity && afterIdentity == baseline.identity
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(destination []byte) (int, error) {
	select {
	case <-reader.ctx.Done():
		return 0, reader.ctx.Err()
	default:
		return reader.reader.Read(destination)
	}
}
