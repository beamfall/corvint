package gitstatus

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const maxGitlinks = 1024

var errGitlink = unsupported(classSubmodule, "opaque gitlink metadata or status shape is unsupported")

type indexEntries map[string]string

func stagedEntries(raw []byte) (indexEntries, error) {
	entries := indexEntries{}
	for row := range bytes.SplitSeq(raw, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		fields, name, ok := strings.Cut(string(row), "\t")
		parts := strings.Fields(fields)
		if !ok || len(parts) != 3 {
			return nil, errGitlink
		}
		if parts[0] == "160000" && parts[2] != "0" {
			return nil, errGitlink
		}
		entries[name] = parts[0]
	}
	return entries, nil
}

func (entries indexEntries) hasGitlinks() bool {
	for _, mode := range entries {
		if mode == "160000" {
			return true
		}
	}
	return false
}

func gitlinkFormat(arguments []string) (string, error) {
	format, nul := "v1", false
	for _, arg := range arguments {
		switch arg {
		case "status", "--ignored=no", "--porcelain", "--porcelain=v1", "--untracked-files=all", "--untracked-files=no", "--untracked-files=normal", "--no-renames", "--ignore-submodules=none", "--ignore-submodules=all", "--ignore-submodules=dirty", "--ignore-submodules=untracked":
		case "--porcelain=v2":
			format = "v2"
		case "-z":
			nul = true
		default:
			return "", errGitlink
		}
	}
	if !nul {
		return "", errGitlink
	}
	return format, nil
}

// Git renders the gitlink rows against an inert private worktree. Only captured
// checkout OIDs cross this boundary; Git never opens a nested live repository.
func opaqueGitlinkStatus(ctx context.Context, root, temp string, prefix, arguments []string, entries indexEntries, reader *metadataReader, files *[]capturedFile, run Runner) ([]byte, error) {
	format, err := gitlinkFormat(arguments)
	if err != nil {
		return nil, err
	}
	raw, err := run(ctx, temp, metadataLimit, append(append([]string{}, prefix...), "ls-tree", "-r", "-z", "HEAD")...)
	if err != nil {
		return nil, metadataProbeError(err)
	}
	candidates := map[string]bool{}
	for name, mode := range entries {
		if mode == "160000" {
			candidates[name] = true
		}
	}
	for row := range bytes.SplitSeq(raw, []byte{0}) {
		if !bytes.HasPrefix(row, []byte("160000 commit ")) {
			continue
		}
		_, name, ok := strings.Cut(string(row), "\t")
		if !ok {
			return nil, errGitlink
		}
		if _, present := entries[name]; !present {
			candidates[name] = false
		}
	}
	if len(candidates) > maxGitlinks {
		return nil, errGitlink
	}
	names := make([]string, 0, len(candidates))
	for name := range candidates {
		if !safeGitlinkPath(name) {
			return nil, errGitlink
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		if i > 0 && strings.HasPrefix(name, names[i-1]+"/") {
			return nil, errGitlink
		}
	}
	shadow := filepath.Join(temp, "opaque-worktree")
	if err := os.Mkdir(shadow, 0700); err != nil {
		return nil, err
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if candidates[name] {
			if err := snapshotGitlink(root, shadow, name, reader, files); err != nil {
				return nil, err
			}
		}
	}
	args := append(append([]string{}, prefix...), "--work-tree="+shadow, "status", "--porcelain="+format, "-z", "--untracked-files=no", "--ignore-submodules=dirty", "--no-renames", "--")
	for _, name := range names {
		args = append(args, ":(literal)"+name)
	}
	return run(ctx, temp, metadataLimit, args...)
}

func safeGitlinkPath(name string) bool {
	if name == "" || len(name) > 4096 || !filepath.IsLocal(name) || path.Clean(name) != name || strings.ContainsRune(name, 0) {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if strings.EqualFold(segment, ".git") {
			return false
		}
	}
	return true
}

func captureGitlink(reader *metadataReader, files *[]capturedFile, name string, limit int) ([]byte, error) {
	remaining := snapshotLimit
	for _, file := range *files {
		remaining -= len(file.data)
	}
	if remaining <= 0 {
		return nil, errGitlink
	}
	file, err := reader.captureLimited(filepath.Clean(name), min(remaining, limit))
	if err != nil {
		return nil, err
	}
	*files = append(*files, file)
	return file.data, nil
}

func gitlinkHEAD(reader *metadataReader, files *[]capturedFile, admin string) (string, error) {
	current := "HEAD"
	for depth := 0; depth < 8; depth++ {
		data, err := captureGitlink(reader, files, filepath.Join(admin, current), 4096)
		if err != nil {
			return "", err
		}
		if len(data) == 0 && current != "HEAD" {
			return packedGitlinkRef(reader, files, admin, current)
		}
		value := strings.TrimRight(string(data), "\r\n")
		if validGitlinkOID(value) {
			return value, nil
		}
		ref, ok := strings.CutPrefix(value, "ref: ")
		if !ok || !safeGitlinkRef(ref) {
			return "", errGitlink
		}
		current = ref
	}
	return "", errGitlink
}

func safeGitlinkRef(ref string) bool {
	for _, r := range ref {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return strings.HasPrefix(ref, "refs/") && safeGitlinkPath(ref) && !strings.ContainsAny(ref, "\\ ~^:?*[") && !strings.Contains(ref, "..") && !strings.Contains(ref, "@{") && !strings.HasSuffix(ref, ".lock")
}

func packedGitlinkRef(reader *metadataReader, files *[]capturedFile, admin, ref string) (string, error) {
	data, err := captureGitlink(reader, files, filepath.Join(admin, "packed-refs"), metadataLimit)
	if err != nil {
		return "", err
	}
	oid := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		candidate, name, ok := strings.Cut(line, " ")
		if ok && name == ref {
			if oid != "" || !validGitlinkOID(candidate) {
				return "", errGitlink
			}
			oid = candidate
		}
	}
	if oid == "" {
		return "", errGitlink
	}
	return oid, nil
}

func validGitlinkOID(oid string) bool {
	if len(oid) != 40 && len(oid) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(oid)
	return err == nil && strings.ToLower(oid) == oid && !bytes.Equal(decoded, make([]byte, len(decoded)))
}

// Both observations disable rename detection, so each NUL record has one path.
func mergeGitlinkStatus(ordinary, opaque []byte, arguments []string, limit int) ([]byte, error) {
	format, err := gitlinkFormat(arguments)
	if err != nil {
		return nil, err
	}
	rows := map[string][]byte{}
	for _, raw := range [][]byte{opaque, ordinary} {
		for row := range bytes.SplitSeq(raw, []byte{0}) {
			if len(row) == 0 {
				continue
			}
			var name string
			if format == "v1" {
				if len(row) < 4 {
					return nil, errGitlink
				}
				name = string(row[3:])
			} else {
				fields := bytes.SplitN(row, []byte(" "), 9)
				if row[0] == '?' || row[0] == '!' {
					name = string(row[2:])
				} else if row[0] == '1' && len(fields) == 9 {
					name = string(fields[8])
				} else if row[0] == 'u' {
					conflict := bytes.SplitN(row, []byte(" "), 11)
					if len(conflict) != 11 {
						return nil, errGitlink
					}
					name = string(conflict[10])
				} else {
					return nil, errGitlink
				}
			}
			rows[name] = row
		}
	}
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	result := []byte{}
	for _, name := range names {
		if len(result)+len(rows[name])+1 > limit {
			return nil, errGitlink
		}
		result = append(result, rows[name]...)
		result = append(result, 0)
	}
	return result, nil
}
