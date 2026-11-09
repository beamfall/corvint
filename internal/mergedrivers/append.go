// SPDX-License-Identifier: Apache-2.0
// Package mergedrivers contains the parent-only merge rules of DWC-V0-7700..7703.
package mergedrivers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// Append runs diff3 and resolves only conflicts whose base consists of whitespace.
// Unresolved hunks are retained verbatim. parent is a private scratch directory,
// or empty to use the caller's temporary directory (DWC-V0-7700).
func Append(ctx context.Context, parent string, base, ours, theirs []byte, size int) ([]byte, bool, error) {
	if size < 1 || size > 1024 {
		return nil, false, fmt.Errorf("invalid marker size")
	}
	size = appendMarkerSize(size, base, ours, theirs)
	dir, err := os.MkdirTemp(parent, "append-union-")
	if err != nil {
		return nil, false, err
	}
	defer os.RemoveAll(dir)
	for name, data := range map[string][]byte{"base": base, "ours": ours, "theirs": theirs} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return nil, false, err
		}
	}
	command := exec.CommandContext(ctx, "git", "merge-file", "-p", "--diff3", fmt.Sprintf("--marker-size=%d", size),
		"-L", "ours", "-L", "base", "-L", "theirs", "ours", "base", "theirs")
	command.Dir = dir
	command.Env = gokernel.SanitizedGitEnvironment()
	merged, err := command.Output()
	if err == nil {
		// A clean merge may contain literal conflict markers; it needs no parsing.
		return merged, true, nil
	}
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() < 1 || exit.ExitCode() > 127 {
			return nil, false, err
		}
	}
	result, clean := resolveAppend(merged, size)
	return result, clean, nil
}

// appendMarkerSize makes every generated delimiter distinguishable from content,
// including literal Markdown underlines and labeled marker examples (DWC-V0-7700).
func appendMarkerSize(size int, inputs ...[]byte) int {
	if size < 7 {
		size = 7
	}
	for _, data := range inputs {
		for _, line := range bytes.Split(data, []byte("\n")) {
			if len(line) == 0 || !strings.ContainsRune("<|=>", rune(line[0])) {
				continue
			}
			run := 1
			for run < len(line) && line[run] == line[0] {
				run++
			}
			if run >= size {
				size = run + 1
			}
		}
	}
	return size
}

// appendDelimiter recognizes only the exact width and labels emitted above.
// Shorter marker runs are input text by construction; any other line reaching
// the chosen width indicates malformed output and must never be resolved.
func appendDelimiter(line string, size int) (byte, bool) {
	line = strings.TrimRight(line, "\r\n")
	if line == "" || !strings.ContainsRune("<|=>", rune(line[0])) {
		return 0, true
	}
	run := 1
	for run < len(line) && line[run] == line[0] {
		run++
	}
	if run < size {
		return 0, true
	}
	labels := map[byte]string{'<': " ours", '|': " base", '=': "", '>': " theirs"}
	if run != size || line[run:] != labels[line[0]] {
		return 0, false
	}
	return line[0], true
}

func resolveAppend(data []byte, size int) ([]byte, bool) {
	if size < 1 {
		return data, false
	}
	lines := strings.SplitAfter(string(data), "\n")
	var out bytes.Buffer
	clean, sawConflict := true, false
	for i := 0; i < len(lines); {
		marker, valid := appendDelimiter(lines[i], size)
		if !valid || (marker != 0 && marker != '<') {
			return data, false
		}
		if marker == 0 {
			out.WriteString(lines[i])
			i++
			continue
		}
		sawConflict = true
		start, part := i, 0
		var sides [3]string
		i++
		for ; i < len(lines); i++ {
			marker, valid = appendDelimiter(lines[i], size)
			if !valid {
				return data, false
			}
			if marker == 0 {
				sides[part] += lines[i]
				continue
			}
			if marker != "|=>"[part] {
				return data, false
			}
			if part == 2 {
				break
			}
			part++
		}
		if i == len(lines) {
			return data, false
		}
		i++
		if strings.TrimSpace(sides[1]) != "" {
			out.WriteString(strings.Join(lines[start:i], ""))
			clean = false
			continue
		}
		o, t := sides[0], sides[2]
		if o != "" && !strings.HasSuffix(o, "\n") {
			o += "\n"
		}
		out.WriteString(o)
		olines := strings.SplitAfter(o, "\n")
		last := len(olines) - 1
		if olines[last] == "" && last > 0 {
			last--
		}
		if o != "" && t != "" && strings.TrimSpace(olines[last]) != "" && strings.HasPrefix(strings.TrimLeft(t, " \t\r\n"), "#") && strings.TrimSpace(strings.SplitAfter(t, "\n")[0]) != "" {
			out.WriteByte('\n')
		}
		out.WriteString(t)
	}
	return out.Bytes(), clean && sawConflict
}
