package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// corvintModuleLine is the first line of the Corvint repository's go.mod.
const corvintModuleLine = "module github.com/Beamfall/corvint"

// dogfoodWorkflowApplies reports whether the repository at root is Corvint itself or already
// holds a dogfood enrollment, the two cases in which the trusted dogfood workflow argv is
// relevant (AHI-230, V1-2754). Anything unreadable or absent is "no": the guidance is only an
// optional hint, so the safe failure is to inject nothing.
func dogfoodWorkflowApplies(root string) bool {
	if goModFirstLine(filepath.Join(root, "go.mod")) == corvintModuleLine {
		return true
	}
	gitDir := privateGitDir(root)
	if gitDir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(gitDir, "corvint", "local-completion"))
	return err == nil && info.IsDir()
}

// goModFirstLineLimit caps the go.mod bytes read for the module line (V1-3022).
const goModFirstLineLimit = 4096

// goModFirstLine returns the trimmed first line of a regular go.mod, or "" when
// the file is absent, not a regular file (a FIFO is never opened for a
// blocking read), or its first line does not end within goModFirstLineLimit
// bytes.
func goModFirstLine(path string) string {
	file, err := openBoundedRegularFile(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	line, err := bufio.NewReader(io.LimitReader(file, goModFirstLineLimit)).ReadString('\n')
	if err != nil && (err != io.EOF || len(line) >= goModFirstLineLimit) {
		return ""
	}
	return strings.TrimSpace(line)
}

// privateGitDir resolves root/.git: the directory itself, or the target of a bounded gitfile.
func privateGitDir(root string) string {
	marker := filepath.Join(root, ".git")
	info, err := os.Lstat(marker)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return marker
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return ""
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		return ""
	}
	target, found := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir: ")
	if !found || target == "" {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return filepath.Clean(target)
}
