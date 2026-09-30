// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type workspaceGitIdentity struct {
	root, gitEntry, admin, common, head                            os.FileInfo
	gitText, headText, adminPath, commonPath, format, commit, tree string
}

func identityText(path string) ([]byte, os.FileInfo, error) {
	first, e := os.Lstat(path)
	if e != nil || !first.Mode().IsRegular() || first.Size() > 4096 {
		return nil, nil, errWorkspaceObservation
	}
	f, e := openWorkspaceFile(path)
	if e != nil {
		return nil, nil, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !opened.Mode().IsRegular() || !os.SameFile(first, opened) {
		return nil, nil, errWorkspaceObservation
	}
	raw, e := io.ReadAll(io.LimitReader(f, 4097))
	last, statErr := f.Stat()
	current, pathErr := os.Lstat(path)
	if e != nil || statErr != nil || pathErr != nil || len(raw) > 4096 || int64(len(raw)) != first.Size() || !os.SameFile(first, last) || !os.SameFile(first, current) || last.Size() != first.Size() || last.ModTime() != first.ModTime() || current.ModTime() != first.ModTime() {
		return nil, nil, errWorkspaceObservation
	}
	return raw, current, nil
}
func exactLine(raw []byte) bool {
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte{'\n'}) != 1 || !utf8.Valid(raw) {
		return false
	}
	for _, b := range raw[:len(raw)-1] {
		if b < 32 || b == 127 {
			return false
		}
	}
	return true
}
func directoryIdentity(path string) (os.FileInfo, error) {
	canonical, e := filepath.EvalSymlinks(path)
	if e != nil || canonical != filepath.Clean(path) || !filepath.IsAbs(path) || path != filepath.Clean(path) {
		return nil, errWorkspaceObservation
	}
	info, e := os.Lstat(path)
	if e != nil || !info.IsDir() {
		return nil, errWorkspaceObservation
	}
	return info, nil
}
func workspaceMapping(root string) (workspaceGitIdentity, error) {
	var out workspaceGitIdentity
	var e error
	out.root, e = directoryIdentity(root)
	if e != nil {
		return out, e
	}
	path := filepath.Join(root, ".git")
	out.gitEntry, e = os.Lstat(path)
	if e != nil {
		return out, e
	}
	if out.gitEntry.IsDir() {
		out.adminPath = path
	} else if out.gitEntry.Mode().IsRegular() {
		raw, info, e := identityText(path)
		if e != nil || !exactLine(raw) || !bytes.HasPrefix(raw, []byte("gitdir: ")) {
			return out, errWorkspaceObservation
		}
		out.gitEntry = info
		out.gitText = string(raw)
		admin := string(raw[len("gitdir: ") : len(raw)-1])
		if admin == "" {
			return out, errWorkspaceObservation
		}
		if !filepath.IsAbs(admin) {
			admin = filepath.Join(root, admin)
		}
		out.adminPath, e = filepath.EvalSymlinks(admin)
		if e != nil {
			return out, e
		}
	} else {
		return out, errWorkspaceObservation
	}
	out.admin, e = directoryIdentity(out.adminPath)
	if e != nil {
		return out, e
	}
	raw, info, e := identityText(filepath.Join(out.adminPath, "HEAD"))
	if e != nil || !exactLine(raw) {
		return out, errWorkspaceObservation
	}
	out.head = info
	out.headText = string(raw)
	if strings.HasPrefix(out.headText, "ref: refs/heads/") {
		ref := strings.TrimSuffix(strings.TrimPrefix(out.headText, "ref: refs/heads/"), "\n")
		if ref == "" || strings.Contains(ref, "..") || strings.ContainsAny(ref, " ~^:?*[\\") {
			return out, errWorkspaceObservation
		}
	} else if !lowerObjectID(strings.TrimSuffix(out.headText, "\n"), 40) && !lowerObjectID(strings.TrimSuffix(out.headText, "\n"), 64) {
		return out, errWorkspaceObservation
	}
	return out, nil
}
func lowerObjectID(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func parseWorkspaceGit(raw []byte, mapping workspaceGitIdentity) (workspaceGitIdentity, error) {
	if len(raw) > 16384 || len(raw) == 0 || raw[len(raw)-1] != '\n' || !utf8.Valid(raw) || bytes.ContainsAny(raw, "\x00\r") {
		return mapping, errWorkspaceObservation
	}
	lines := strings.Split(string(raw[:len(raw)-1]), "\n")
	if len(lines) != 5 {
		return mapping, errWorkspaceObservation
	}
	n := 40
	if lines[0] == "sha256" {
		n = 64
	} else if lines[0] != "sha1" {
		return mapping, errWorkspaceObservation
	}
	if !lowerObjectID(lines[3], n) || !lowerObjectID(lines[4], n) || lines[1] != mapping.adminPath {
		return mapping, errWorkspaceObservation
	}
	for _, path := range lines[1:3] {
		if len(path) > 4096 || hasGitControl(path) {
			return mapping, errWorkspaceObservation
		}
	}
	common, e := directoryIdentity(lines[2])
	if e != nil {
		return mapping, e
	}
	if !strings.HasPrefix(mapping.headText, "ref: ") && strings.TrimSuffix(mapping.headText, "\n") != lines[3] {
		return mapping, errWorkspaceObservation
	}
	mapping.common = common
	mapping.commonPath = lines[2]
	mapping.format = lines[0]
	mapping.commit = lines[3]
	mapping.tree = lines[4]
	return mapping, nil
}
func sameWorkspaceMapping(a, b workspaceGitIdentity) bool {
	return os.SameFile(a.root, b.root) && os.SameFile(a.gitEntry, b.gitEntry) && os.SameFile(a.admin, b.admin) && os.SameFile(a.head, b.head) && a.gitText == b.gitText && a.headText == b.headText && a.adminPath == b.adminPath
}
func sameWorkspaceGit(a, b workspaceGitIdentity) bool {
	return sameWorkspaceMapping(a, b) && os.SameFile(a.common, b.common) && a.commonPath == b.commonPath && a.format == b.format && a.commit == b.commit && a.tree == b.tree
}
func observeWorkspaceGit(ctx context.Context, root string) (workspaceGitIdentity, error) {
	before, e := workspaceMapping(root)
	if e != nil {
		return before, e
	}
	raw, e := boundedGit(ctx, root, "rev-parse", "--show-object-format", "--absolute-git-dir", "--path-format=absolute", "--git-common-dir", "HEAD^{commit}", "HEAD^{tree}")
	if e != nil {
		return before, e
	}
	parsed, e := parseWorkspaceGit(raw, before)
	if e != nil {
		return parsed, e
	}
	after, e := workspaceMapping(root)
	if e != nil || !sameWorkspaceMapping(before, after) {
		return after, errWorkspaceObservation
	}
	common, e := directoryIdentity(parsed.commonPath)
	if e != nil || !os.SameFile(common, parsed.common) {
		return parsed, errWorkspaceObservation
	}
	return parsed, nil
}

func hasGitControl(s string) bool {
	for _, c := range s {
		if c < 32 || c == 127 {
			return true
		}
	}
	return false
}
