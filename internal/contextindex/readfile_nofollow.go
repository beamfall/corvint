//go:build darwin || linux

package contextindex

import (
	"crypto/sha1"
	"crypto/sha256"
	"hash"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func readCleanFile(repositoryRoot, relative string) ([]byte, bool) {
	opener, opened := newCleanFileOpener(repositoryRoot)
	if !opened {
		return nil, false
	}
	defer opener.close()
	return readCleanFileUsing(opener, relative)
}

// readCleanFileUsing is readCleanFile against an opener the caller keeps open.
// Resolving a parent directory costs an open per component, and a whole-tree
// pass arrives in tree order, so holding the chain across a run of files under
// one directory resolves it once per run instead of once per file.
func readCleanFileUsing(opener *cleanFileOpener, relative string) ([]byte, bool) {
	if opener == nil {
		return nil, false
	}
	var data []byte
	ok := withCleanFileUsing(opener, relative, func(file *os.File, size int64) bool {
		// The stat already bounded size, so the body is read into one
		// exactly-sized buffer instead of io.ReadAll's doubling growth. The
		// extra byte is what detects a file that grew during the read.
		buffer := make([]byte, size+1)
		count, err := io.ReadFull(file, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return false
		}
		if int64(count) != size {
			return false
		}
		data = buffer[:count:count]
		return true
	})
	return data, ok
}

// verifyCleanFile streams a clean worktree candidate only long enough to
// verify its immutable Git blob identity and generated-file header. Query
// inventory intentionally retains neither body; authority closure loading
// below is the only path that needs its bytes.
func verifyCleanFile(repositoryRoot, relative, oid, objectFormat string) (generated, ok bool) {
	var scratch cleanFileVerificationScratch
	return verifyCleanFileWithScratch(repositoryRoot, relative, oid, objectFormat, &scratch)
}

func verifyCleanFileWithScratch(repositoryRoot, relative, oid, objectFormat string, scratch *cleanFileVerificationScratch) (generated, ok bool) {
	opener, opened := newCleanFileOpener(repositoryRoot)
	if !opened {
		return false, false
	}
	defer opener.close()
	return verifyCleanFileUsing(opener, relative, oid, objectFormat, scratch)
}

func verifyCleanFileUsing(opener *cleanFileOpener, relative, oid, objectFormat string, scratch *cleanFileVerificationScratch) (generated, ok bool) {
	ok = withCleanFileUsing(opener, relative, func(file *os.File, size int64) bool {
		hasher, valid := opener.blobHasher(size, objectFormat)
		if !valid {
			return false
		}
		prefixLength := 0
		for remaining := size; remaining > 0; {
			chunk := scratch.buffer[:]
			if int64(len(chunk)) > remaining {
				chunk = chunk[:remaining]
			}
			count, err := file.Read(chunk)
			if count > 0 {
				if prefixLength < len(scratch.prefix) {
					prefixLength += copy(scratch.prefix[prefixLength:], chunk[:count])
				}
				_, _ = hasher.Write(chunk[:count])
				remaining -= int64(count)
			}
			if err != nil && err != io.EOF {
				return false
			}
			if count == 0 {
				return remaining == 0 && err == io.EOF
			}
		}
		generated = generatedHeader.Match(scratch.prefix[:prefixLength])
		return equalHexDigest(hasher.Sum(opener.digest[:0]), oid)
	})
	if !ok {
		return false, false
	}
	return generated, ok
}

// blobHasher returns this opener's hasher for objectFormat, primed with the
// Git blob header. One hasher per opener is one per verification worker, so
// resetting it is what removes a hash allocation from every clean file.
func (opener *cleanFileOpener) blobHasher(size int64, objectFormat string) (hash.Hash, bool) {
	var hasher hash.Hash
	switch objectFormat {
	case "sha1":
		if opener.sha1Hasher == nil {
			opener.sha1Hasher = sha1.New()
		}
		hasher = opener.sha1Hasher
	case "sha256":
		if opener.sha256Hasher == nil {
			opener.sha256Hasher = sha256.New()
		}
		hasher = opener.sha256Hasher
	default:
		return nil, false
	}
	hasher.Reset()
	header := append(opener.header[:0], "blob "...)
	header = strconv.AppendInt(header, size, 10)
	header = append(header, 0)
	_, _ = hasher.Write(header)
	return hasher, true
}

func equalHexDigest(digest []byte, oid string) bool {
	if len(oid) != len(digest)*2 {
		return false
	}
	for index, value := range digest {
		if oid[index*2] != "0123456789abcdef"[value>>4] || oid[index*2+1] != "0123456789abcdef"[value&0x0f] {
			return false
		}
	}
	return true
}

// cleanFileOpener holds an O_NOFOLLOW-validated view of the worktree. Opening a
// root and re-rooting at every path component confines every open to the
// repository, but os.Root still follows a directory or leaf symlink whose
// target stays inside it; callers are safe only because the bytes must hash to
// the committed blob. The chain is also the dominant cost of a query build: a CPU
// profile over a 3,219-file repository put 20% of the build inside this chain
// and 50% more in the leaf open, against 8% in reading and hashing. The chain
// is therefore held between files and rebuilt only from the first component
// that actually differs, because entries arrive in tree order and consecutive
// paths share nearly all of their directories. Reusing a chain is equivalent to
// rebuilding it: the same components were opened with the same flags.
type cleanFileOpener struct {
	base   *os.Root
	roots  []*os.Root
	held   []string
	parent *os.Root
	// parentPath is the directory prefix of the last resolved path, held as a
	// sub-slice of that path so caching costs no allocation.
	parentPath   string
	sha1Hasher   hash.Hash
	sha256Hasher hash.Hash
	header       [32]byte
	digest       [64]byte
}

func newCleanFileOpener(repositoryRoot string) (*cleanFileOpener, bool) {
	base, err := os.OpenRoot(repositoryRoot)
	if err != nil {
		return nil, false
	}
	return &cleanFileOpener{base: base, parent: base, parentPath: ""}, true
}

// releaseFrom closes the chain below depth, leaving the first depth components
// open and reusable.
func (opener *cleanFileOpener) releaseFrom(depth int) {
	for index := len(opener.roots) - 1; index >= depth; index-- {
		_ = opener.roots[index].Close()
	}
	opener.roots = opener.roots[:depth]
	opener.held = opener.held[:depth]
}

func (opener *cleanFileOpener) rootAt(depth int) *os.Root {
	if depth == 0 {
		return opener.base
	}
	return opener.roots[depth-1]
}

func (opener *cleanFileOpener) close() {
	opener.releaseFrom(0)
	_ = opener.base.Close()
}

// resolveParent returns the root for the directory prefix, reusing the held
// chain's matching components and opening only the ones that differ.
func (opener *cleanFileOpener) resolveParent(directory string) (*os.Root, bool) {
	if opener.parent != nil && opener.parentPath == directory {
		return opener.parent, true
	}
	shared, offset := opener.sharedDepth(directory)
	opener.releaseFrom(shared)
	current := opener.rootAt(shared)
	for offset < len(directory) {
		component := directory[offset:componentEnd(directory, offset)]
		next, err := opener.descend(current, component)
		if !err {
			opener.releaseFrom(shared)
			opener.parent, opener.parentPath = nil, ""
			return nil, false
		}
		opener.roots = append(opener.roots, next)
		opener.held = append(opener.held, component)
		current = next
		offset += len(component) + 1
	}
	opener.parent, opener.parentPath = current, directory
	return current, true
}

// descend opens one directory component. Root.OpenRoot re-roots at the opened
// descriptor directly; opening a directory File and then re-opening it through
// its /dev/fd path cost a second, fully resolved path open per component.
func (opener *cleanFileOpener) descend(current *os.Root, component string) (*os.Root, bool) {
	if !validComponent(component) {
		return nil, false
	}
	next, err := current.OpenRoot(component)
	if err != nil {
		return nil, false
	}
	return next, true
}

// sharedDepth reports how many held components directory still starts with,
// and the byte offset in directory where the first differing component begins.
func (opener *cleanFileOpener) sharedDepth(directory string) (int, int) {
	depth, offset := 0, 0
	for depth < len(opener.held) && offset < len(directory) {
		end := componentEnd(directory, offset)
		if directory[offset:end] != opener.held[depth] {
			break
		}
		depth++
		offset = end + 1
	}
	return depth, offset
}

func componentEnd(path string, offset int) int {
	if end := strings.IndexByte(path[offset:], '/'); end >= 0 {
		return offset + end
	}
	return len(path)
}

func validComponent(component string) bool {
	return component != "" && component != "." && component != ".."
}

func withCleanFile(repositoryRoot, relative string, consume func(*os.File, int64) bool) bool {
	opener, ok := newCleanFileOpener(repositoryRoot)
	if !ok {
		return false
	}
	defer opener.close()
	return withCleanFileUsing(opener, relative, consume)
}

func withCleanFileUsing(opener *cleanFileOpener, relative string, consume func(*os.File, int64) bool) bool {
	separator := strings.LastIndexByte(relative, '/')
	directory, name := "", relative
	if separator >= 0 {
		directory, name = relative[:separator], relative[separator+1:]
	}
	if !validComponent(name) {
		return false
	}
	current, ok := opener.resolveParent(directory)
	if !ok {
		return false
	}
	file, err := current.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return false
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > maxSourceBytes {
		return false
	}
	if !consume(file, before.Size()) {
		return false
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || !sameMetadata(before, after) {
		return false
	}
	return true
}

func sameMetadata(left, right os.FileInfo) bool {
	return left.Size() == right.Size() && left.Mode() == right.Mode() && left.ModTime() == right.ModTime()
}
