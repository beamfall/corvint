//go:build darwin

package authoritystore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Beamfall/corvint/internal/localauthority"
)

type evidenceNode struct {
	relative string
	stat     syscall.Stat_t
}
type evidenceView struct {
	root       string
	owner, gid uint32
	ancestors  []evidenceNode
	nodes      []evidenceNode
	manifest   EvidenceManifest
}

func evidenceDescriptor(file *os.File, owner, gid uint32, directory, parent bool) (syscall.Stat_t, error) {
	var st syscall.Stat_t
	if syscall.Fstat(int(file.Fd()), &st) != nil || st.Uid != owner || st.Gid != gid || noACL(file.Fd()) != nil {
		return st, errUnavailable
	}
	mode := uint16(0440)
	kind := uint16(syscall.S_IFREG)
	if directory {
		mode = 0550
		kind = syscall.S_IFDIR
		if parent {
			mode = 0750
		}
	}
	if st.Mode&07777 != mode || st.Mode&syscall.S_IFMT != kind || !directory && st.Nlink != 1 {
		return st, errUnavailable
	}
	return st, nil
}

// openEvidenceParent is fixed-root production code. Directory handles are
// acquired without following links, with no special treatment for caller paths.
func openEvidenceParent(owner, gid uint32) (*os.File, []evidenceNode, error) {
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, errUnavailable
	}
	current := os.NewFile(uintptr(fd), "/")
	var nodes []evidenceNode
	var st syscall.Stat_t
	if auditDescriptor(current.Fd(), 0, true, false) != nil || syscall.Fstat(fd, &st) != nil {
		current.Close()
		return nil, nil, errUnavailable
	}
	nodes = append(nodes, evidenceNode{"/", st})
	prefix := ""
	for _, part := range strings.Split(strings.TrimPrefix(RootPath+"/evidence", "/"), "/") {
		prefix += "/" + part
		next, e := openAt(current.Fd(), part, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC)
		current.Close()
		if e != nil {
			return nil, nil, errUnavailable
		}
		current = os.NewFile(uintptr(next), prefix)
		if prefix == RootPath+"/evidence" {
			st, e = evidenceDescriptor(current, owner, gid, true, true)
		} else {
			e = auditDescriptor(current.Fd(), 0, true, false)
			if e == nil {
				e = syscall.Fstat(next, &st)
			}
		}
		if e != nil {
			current.Close()
			return nil, nil, errUnavailable
		}
		nodes = append(nodes, evidenceNode{prefix, st})
	}
	return current, nodes, nil
}

// AuditEvidenceParent is used by the separate authority publisher before it
// creates its private stage. It grants no reader admission and creates nothing.
func AuditEvidenceParent(owner, gid uint32) error {
	f, _, err := openEvidenceParent(owner, gid)
	if err == nil {
		err = f.Close()
	}
	return err
}

func readEvidenceLeaf(parent *os.File, name string, owner, gid uint32, limit int) ([]byte, syscall.Stat_t, error) {
	var st syscall.Stat_t
	fd, err := openAt(parent.Fd(), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC)
	if err != nil {
		return nil, st, errUnavailable
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	st, err = evidenceDescriptor(f, owner, gid, false, false)
	if err != nil || st.Size < 0 || st.Size > int64(limit) {
		return nil, st, errUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || int64(len(raw)) != st.Size {
		return nil, st, errUnavailable
	}
	after, err := evidenceDescriptor(f, owner, gid, false, false)
	if err != nil || !sameStat(st, after) {
		return nil, st, errUnavailable
	}
	return raw, st, nil
}

func openEvidenceView(ctx context.Context, root RootDocument, handle string, p publication) (*evidenceView, error) {
	owner64, ok := decimal(root.AuthorityUID)
	if !ok {
		return nil, errUnavailable
	}
	gid64, ok := decimal(root.AuthorityGID)
	if !ok || gid64 == 0 || gid64 > 65535 {
		return nil, errUnavailable
	}
	reader64, ok := decimal(root.ReaderUID)
	if !ok || reader64 == 0 || reader64 != uint64(os.Getuid()) || os.Geteuid() != os.Getuid() {
		return nil, errUnavailable
	}
	groups, err := os.Getgroups()
	if err != nil {
		return nil, errUnavailable
	}
	member := uint64(os.Getegid()) == gid64
	for _, g := range groups {
		member = member || uint64(g) == gid64
	}
	if !member {
		return nil, errUnavailable
	}
	parent, ancestors, err := openEvidenceParent(uint32(owner64), uint32(gid64))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	fd, err := openAt(parent.Fd(), handle, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC)
	if err != nil {
		return nil, errUnavailable
	}
	view := os.NewFile(uintptr(fd), handle)
	defer view.Close()
	if _, err = evidenceDescriptor(view, uint32(owner64), uint32(gid64), true, false); err != nil {
		return nil, err
	}
	raw, manifestStat, err := readEvidenceLeaf(view, "manifest.json", uint32(owner64), uint32(gid64), MaxEvidenceManifest)
	if err != nil || localauthority.BytesDigest(raw) != p.evidence.ManifestSHA256 {
		return nil, errUnavailable
	}
	var manifest EvidenceManifest
	if DecodeEvidenceManifest(raw, &manifest) != nil || validateEvidenceBinding(root, handle, p.enrollment, p.evidence, manifest) != nil {
		return nil, errUnavailable
	}
	v := &evidenceView{root: filepath.Join(RootPath, "evidence", handle, "repo"), owner: uint32(owner64), gid: uint32(gid64), ancestors: ancestors, manifest: manifest}
	nodes, err := v.audit(ctx, view, true, localauthority.BytesDigest(raw))
	if err != nil {
		return nil, err
	}
	if !evidenceNodeMatches(nodes, "manifest.json", manifestStat) {
		return nil, errUnavailable
	}
	v.nodes = nodes
	return v, nil
}

func (v *evidenceView) audit(ctx context.Context, root *os.File, hashFiles bool, manifestDigest string) ([]evidenceNode, error) {
	files := map[string]EvidenceFile{"manifest.json": {Path: "manifest.json", SHA256: manifestDigest}}
	dirs := map[string]bool{"": true, "repo": true, "repo/.git": true, "repo/.git/refs": true, "repo/.git/objects": true}
	for _, f := range v.manifest.Files {
		f.Path = "repo/" + f.Path
		files[f.Path] = f
		for d := filepath.Dir(f.Path); d != "."; d = filepath.Dir(d) {
			dirs[d] = true
		}
	}
	var nodes []evidenceNode
	buffer := make([]byte, 64<<10)
	var walk func(*os.File, string, int) error
	walk = func(dir *os.File, relative string, depth int) error {
		if ctx.Err() != nil || depth >= 32 {
			return errUnavailable
		}
		st, err := evidenceDescriptor(dir, v.owner, v.gid, true, false)
		if err != nil {
			return err
		}
		nodes = append(nodes, evidenceNode{relative, st})
		expected := []string{}
		for d := range dirs {
			if d != "" && filepath.Dir(d) == relativeDir(relative) {
				expected = append(expected, filepath.Base(d))
			}
		}
		for f := range files {
			if filepath.Dir(f) == relativeDir(relative) {
				expected = append(expected, filepath.Base(f))
			}
		}
		sort.Strings(expected)
		names, err := dir.Readdirnames(len(expected) + 1)
		if err != nil && err != io.EOF {
			return errUnavailable
		}
		sort.Strings(names)
		if len(names) != len(expected) {
			return errUnavailable
		}
		for i := range names {
			if names[i] != expected[i] {
				return errUnavailable
			}
		}
		for _, name := range names {
			if ctx.Err() != nil {
				return errUnavailable
			}
			p := name
			if relative != "" {
				p = relative + "/" + name
			}
			flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
			if dirs[p] {
				flags |= syscall.O_DIRECTORY
			}
			fd, err := openAt(dir.Fd(), name, flags)
			if err != nil {
				return errUnavailable
			}
			f := os.NewFile(uintptr(fd), p)
			if dirs[p] {
				err = walk(f, p, depth+1)
			} else {
				var stat syscall.Stat_t
				stat, err = evidenceDescriptor(f, v.owner, v.gid, false, false)
				if err == nil {
					expected := files[p]
					limit := MaxEvidenceBytes + 65536
					if p == "manifest.json" {
						limit = MaxEvidenceManifest
					} else if size, ok := decimal(expected.Size); !ok || stat.Size != int64(size) {
						err = errUnavailable
					}
					if stat.Size < 0 || stat.Size > int64(limit) {
						err = errUnavailable
					}
					if err == nil && hashFiles && p != "manifest.json" {
						sum := sha256.New()
						n, e := copyEvidence(ctx, sum, io.LimitReader(f, int64(limit)+1), buffer)
						if e != nil || n != stat.Size || hex.EncodeToString(sum.Sum(nil)) != expected.SHA256 {
							err = errUnavailable
						}
					}
					if err == nil {
						after, e := evidenceDescriptor(f, v.owner, v.gid, false, false)
						if e != nil || !sameStat(stat, after) {
							err = errUnavailable
						}
					}
					if err == nil {
						nodes = append(nodes, evidenceNode{p, stat})
					}
				}
			}
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				return errUnavailable
			}
		}
		after, err := evidenceDescriptor(dir, v.owner, v.gid, true, false)
		if err != nil || !sameStat(st, after) {
			return errUnavailable
		}
		return nil
	}
	if err := walk(root, "", 0); err != nil {
		return nil, err
	}
	return nodes, nil
}

func relativeDir(relative string) string {
	if relative == "" {
		return "."
	}
	return relative
}

func (v *evidenceView) unchanged(ctx context.Context, ref EvidenceReference) error {
	parent, ancestors, err := openEvidenceParent(v.owner, v.gid)
	if err != nil {
		return err
	}
	defer parent.Close()
	if !sameEvidenceNodes(v.ancestors, ancestors) {
		return errUnavailable
	}
	handle := filepath.Base(filepath.Dir(v.root))
	fd, err := openAt(parent.Fd(), handle, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC)
	if err != nil {
		return errUnavailable
	}
	root := os.NewFile(uintptr(fd), handle)
	defer root.Close()
	nodes, err := v.audit(ctx, root, false, ref.ManifestSHA256)
	if err != nil || !sameEvidenceNodes(v.nodes, nodes) {
		return errUnavailable
	}
	return nil
}

func sameEvidenceNodes(a, b []evidenceNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].relative != b[i].relative || !sameStat(a[i].stat, b[i].stat) {
			return false
		}
	}
	return true
}

// AuditStagedEvidence checks an unpublished authority-owned stage under the
// fixed evidence parent. It returns no production authority or caller root.
func AuditStagedEvidence(ctx context.Context, name string, manifest EvidenceManifest, owner, gid uint32) error {
	if !strings.HasPrefix(name, ".pending-") || strings.Contains(name, "/") || ValidateEvidenceManifest(manifest) != nil {
		return errUnavailable
	}
	parent, _, err := openEvidenceParent(owner, gid)
	if err != nil {
		return err
	}
	defer parent.Close()
	fd, err := openAt(parent.Fd(), name, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC)
	if err != nil {
		return errUnavailable
	}
	stage := os.NewFile(uintptr(fd), name)
	defer stage.Close()
	return AuditEvidenceStageHandle(ctx, stage, manifest, owner, gid)
}

func AuditEvidenceStageHandle(ctx context.Context, stage *os.File, manifest EvidenceManifest, owner, gid uint32) error {
	if ValidateEvidenceManifest(manifest) != nil {
		return errUnavailable
	}
	raw, manifestStat, err := readEvidenceLeaf(stage, "manifest.json", owner, gid, MaxEvidenceManifest)
	if err != nil {
		return err
	}
	var actual EvidenceManifest
	if DecodeEvidenceManifest(raw, &actual) != nil {
		return errUnavailable
	}
	a, b := localauthority.BytesDigest(raw), ""
	encoded, err := localauthority.Canonical(manifest)
	if err == nil {
		b = localauthority.BytesDigest(encoded)
	}
	if err != nil || a != b {
		return errUnavailable
	}
	view := evidenceView{owner: owner, gid: gid, manifest: manifest}
	nodes, err := view.audit(ctx, stage, true, a)
	if err == nil && !evidenceNodeMatches(nodes, "manifest.json", manifestStat) {
		return errUnavailable
	}
	return err
}

func evidenceNodeMatches(nodes []evidenceNode, p string, stat syscall.Stat_t) bool {
	for _, node := range nodes {
		if node.relative == p {
			return sameStat(node.stat, stat)
		}
	}
	return false
}

func copyEvidence(ctx context.Context, w io.Writer, r io.Reader, buffer []byte) (int64, error) {
	var total int64
	for {
		if ctx.Err() != nil {
			return total, errUnavailable
		}
		n, e := r.Read(buffer)
		if n > 0 {
			written, we := w.Write(buffer[:n])
			total += int64(written)
			if we != nil || written != n {
				return total, errUnavailable
			}
		}
		if e == io.EOF {
			return total, nil
		}
		if e != nil {
			return total, e
		}
	}
}

// Called after the four repository-binding handles exist, before opening the
// view. The kernel returns this process's live descriptor count; a separate
// tool process or a configured shell limit cannot supply the baseline.
func requireEvidenceDescriptorHeadroom() (uint64, uint64, error) {
	var limit syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit) != nil {
		return 0, 0, errUnavailable
	}
	buffer := make([]byte, 8193*8)
	n, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, 2, uintptr(os.Getpid()), 1, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if errno != 0 || n >= uintptr(len(buffer)) || n%8 != 0 {
		return 0, 0, errUnavailable
	}
	baseline := uint64(n / 8)
	if limit.Cur < baseline || limit.Cur-baseline < 48 {
		return baseline, limit.Cur, errUnavailable
	}
	return baseline, limit.Cur, nil
}

func OpenEvidencePublicationParent(owner, gid uint32) (*os.File, error) {
	f, _, err := openEvidenceParent(owner, gid)
	return f, err
}
