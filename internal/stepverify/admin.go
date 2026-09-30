package stepverify

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

// This is an admitted observation profile, not a general Git config parser.
// Reject includes and execution/storage extensions before any Git is invoked.
func supportedConfig(data []byte) bool {
	section := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			parts := strings.Fields(strings.ToLower(line[1 : len(line)-1]))
			if len(parts) == 0 {
				return false
			}
			section = parts[0]
			if section == "include" || section == "includeif" {
				return false
			}
			switch section {
			case "core", "user", "remote", "branch", "filter", "diff", "extensions", "credential", "advice", "init":
			default:
				return false
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || section == "" {
			return false
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.ToLower(strings.Trim(strings.TrimSpace(value), "\""))
		truth := value == "true" || value == "yes" || value == "on" || value == "1"
		if section == "extensions" && (key != "objectformat" && key != "worktreeconfig" || key == "objectformat" && value != "sha256" || key == "worktreeconfig" && !truth) {
			return false
		}
		if section == "core" && (key == "bare" && truth || key == "sparsecheckout" && truth || key == "splitindex" && truth || key == "repositoryformatversion" && value != "0" && value != "1") {
			return false
		}
		if section == "remote" && (key == "promisor" || key == "partialclonefilter") {
			return false
		}
		if strings.HasSuffix(line, "\\") {
			return false
		}
	}
	return true
}

func supportedIndex(data []byte, width int) bool {
	if len(data) == 0 {
		return true
	}
	if len(data) < 12+width || string(data[:4]) != "DIRC" || binary.BigEndian.Uint32(data[4:8]) != 2 {
		return false
	}
	count := int(binary.BigEndian.Uint32(data[8:12]))
	if count > MaxEntries {
		return false
	}
	at := 12
	for range count {
		start := at
		fixed := 42 + width
		if at+fixed > len(data)-width {
			return false
		}
		if binary.BigEndian.Uint16(data[at+fixed-2:at+fixed])&0x4000 != 0 {
			return false
		}
		at += fixed
		for at < len(data)-width && data[at] != 0 {
			at++
		}
		if at >= len(data)-width {
			return false
		}
		at++
		for (at-start)%8 != 0 {
			if at >= len(data)-width || data[at] != 0 {
				return false
			}
			at++
		}
	}
	for at < len(data)-width {
		if at+8 > len(data)-width {
			return false
		}
		name := string(data[at : at+4])
		size := int(binary.BigEndian.Uint32(data[at+4 : at+8]))
		at += 8
		if name != "TREE" || size < 0 || size > len(data)-width-at {
			return false
		}
		at += size
	}
	if at != len(data)-width {
		return false
	}
	if width == 20 {
		sum := sha1.Sum(data[:at])
		return bytes.Equal(sum[:], data[at:])
	}
	if width == 32 {
		sum := sha256.Sum256(data[:at])
		return bytes.Equal(sum[:], data[at:])
	}
	return false
}

func admin(ctx context.Context, c Checkout, h Host, b *inventoryBudget) (entries []Entry, gitdir, common, commit, tree string, resultErr error) {
	entries = []Entry{}
	defer func() { sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path }) }()
	gitdir, common, err := worktreeDirectories(c.Root)
	if err != nil {
		return entries, "", "", "", "", ErrUnsupported
	}
	for _, dir := range []string{gitdir, common} {
		r, err := safeopen.Root(dir)
		if err != nil {
			return entries, gitdir, common, "", "", ErrUnsupported
		}
		r.Close()
	}
	work, err := safeopen.Root(c.Root)
	if err != nil {
		return entries, gitdir, common, "", "", ErrUnsupported
	}
	defer work.Close()
	marker, err := work.Lstat(".git")
	if err != nil {
		return entries, gitdir, common, "", "", ErrUnsupported
	}
	var markerData []byte
	if marker.Mode().IsRegular() {
		markerData, marker, err = regular(work, ".git", b)
		if err != nil {
			return entries, gitdir, common, "", "", err
		}
	} else if !marker.IsDir() {
		return entries, gitdir, common, "", "", ErrUnsupported
	}
	e, _ := entry("worktree/.git", marker, markerData)
	entries = append(entries, e)
	b.entries++
	files := map[string]bool{"HEAD": true, "config": true, "config.worktree": true, "index": true, "packed-refs": true, "description": true, "COMMIT_EDITMSG": true, "FETCH_HEAD": true, "ORIG_HEAD": true, "commondir": true, "gitdir": true, "locked": true}
	dirs := map[string]bool{"refs": true, "hooks": true, "info": true, "branches": true}
	protected := map[string]bool{"objects": true, "logs": true, "worktrees": true, "corvint": true, "taskman": true}
	rawIndex := []byte{}
	objectWidth := 20
	for _, part := range []struct{ label, path string }{{"git", gitdir}, {"common", common}} {
		root, err := safeopen.Root(part.path)
		if err != nil {
			return entries, gitdir, common, "", "", ErrUnsupported
		}
		rootInfo, err := root.Stat(".")
		if err != nil {
			root.Close()
			return entries, gitdir, common, "", "", ErrUnsupported
		}
		rootEntry, err := entry(part.label, rootInfo, nil)
		if err != nil {
			root.Close()
			return entries, gitdir, common, "", "", err
		}
		entries = append(entries, rootEntry)
		b.entries++
		rootIdentity, rootStamp, _, ok := nativeInfo(rootInfo)
		if !ok {
			root.Close()
			return entries, gitdir, common, "", "", ErrUnsupported
		}
		fd, err := safeopen.InRoot(root, ".", os.O_RDONLY, 0, true)
		if err != nil {
			root.Close()
			return entries, gitdir, common, "", "", ErrUnsupported
		}
		items, err := fd.ReadDir(MaxEntries + 1)
		fd.Close()
		if err != nil && len(items) == 0 || len(items) > MaxEntries {
			root.Close()
			return entries, gitdir, common, "", "", ErrUnsupported
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })
		for _, item := range items {
			name := item.Name()
			info, err := root.Lstat(name)
			if err != nil || !files[name] && !dirs[name] && !protected[name] || protected[name] && !info.IsDir() || dirs[name] && !info.IsDir() || files[name] && !info.Mode().IsRegular() {
				root.Close()
				return entries, gitdir, common, "", "", ErrUnsupported
			}
			if protected[name] {
				continue
			}
		}
		for _, name := range sortedNames(files) {
			path := part.label + "/" + name
			info, err := root.Lstat(name)
			if os.IsNotExist(err) {
				entries = append(entries, Entry{Path: path, Kind: "ABSENT", Digest: hash(nil), Identity: hash(nil)})
				b.entries++
				continue
			}
			if err != nil {
				root.Close()
				return entries, gitdir, common, "", "", ErrUnsupported
			}
			data, info, err := regular(root, name, b)
			if err != nil {
				root.Close()
				return entries, gitdir, common, "", "", err
			}
			e, _ := entry(path, info, data)
			entries = append(entries, e)
			b.entries++
			if (name == "config" || name == "config.worktree") && !supportedConfig(data) {
				root.Close()
				return entries, gitdir, common, "", "", ErrUnsupported
			}
			if (name == "config" || name == "config.worktree") && configSHA256(data) {
				objectWidth = 32
			}
			if part.label == "git" && name == "index" {
				rawIndex = data
			}
		}
		for _, name := range sortedNames(dirs) {
			info, err := root.Lstat(name)
			if os.IsNotExist(err) {
				entries = append(entries, Entry{Path: part.label + "/" + name, Kind: "ABSENT", Digest: hash(nil), Identity: hash(nil)})
				b.entries++
				continue
			}
			if err != nil || !info.IsDir() {
				root.Close()
				return entries, gitdir, common, "", "", ErrUnsupported
			}
			rows, raw, err := inventory(ctx, filepath.Join(part.path, name), b, false)
			if err != nil {
				root.Close()
				return entries, gitdir, common, "", "", err
			}
			for _, row := range rows {
				if row.Kind == "SYMLINK" {
					root.Close()
					return entries, gitdir, common, "", "", ErrUnsupported
				}
				if name == "info" && row.Path != "." && row.Path != "exclude" && row.Path != "attributes" && row.Path != "grafts" {
					root.Close()
					return entries, gitdir, common, "", "", ErrUnsupported
				}
				if row.Path == "." {
					row.Path = part.label + "/" + name
				} else {
					row.Path = part.label + "/" + name + "/" + row.Path
				}
				entries = append(entries, row)
			}
			_ = raw
		}
		after, err := root.Stat(".")
		if err != nil {
			root.Close()
			return entries, gitdir, common, "", "", ErrDrift
		}
		afterIdentity, afterStamp, _, ok := nativeInfo(after)
		if !ok || afterIdentity != rootIdentity || afterStamp != rootStamp {
			root.Close()
			return entries, gitdir, common, "", "", ErrDrift
		}
		root.Close()
	}
	if b.entries > MaxEntries || !supportedIndex(rawIndex, objectWidth) {
		return entries, gitdir, common, "", "", ErrUnsupported
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	repository, err := gitauth.OpenPinned(c.Root, gitrun.NewBudget(1024, 30_000_000_000), h.GitBinary)
	if err != nil {
		return entries, gitdir, common, "", "", ErrUnsupported
	}
	commit, err = repository.Resolve(ctx, "HEAD")
	if err != nil {
		return entries, gitdir, common, "", "", ErrUnsupported
	}
	tree, err = repository.CommitTree(ctx, commit)
	if err != nil {
		return entries, gitdir, common, "", "", ErrUnsupported
	}
	// Fixed read-only probe excludes gitlinks anywhere in the committed tree.
	env := append(gokernel.SanitizedGitEnvironment(), "GIT_GRAFT_FILE="+os.DevNull, "GIT_ATTR_NOSYSTEM=1")
	args := []string{"--no-optional-locks", "--git-dir=" + gitdir, "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false", "-c", "core.attributesFile=" + os.DevNull, "ls-tree", "-r", "-z", commit}
	data, err := gitrun.Run(ctx, gitrun.NewBudget(1, 10_000_000_000), gitrun.Options{Binary: h.GitBinary, Dir: c.Root, Env: env, StdoutLimit: 8 << 20}, args...)
	if err != nil {
		return entries, gitdir, common, "", "", ErrUnsupported
	}
	for _, record := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(record, "160000 ") {
			return entries, gitdir, common, "", "", ErrUnsupported
		}
	}
	return entries, gitdir, common, commit, tree, nil
}

func indexDigest(entries []Entry) string {
	for _, e := range entries {
		if e.Path == "git/index" {
			return digest(e)
		}
	}
	return hash(nil)
}

func sortedNames(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
func configSHA256(data []byte) bool {
	section := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.ToLower(raw))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && section == "extensions" && strings.TrimSpace(key) == "objectformat" && strings.Trim(strings.TrimSpace(value), "\"") == "sha256" {
			return true
		}
	}
	return false
}
