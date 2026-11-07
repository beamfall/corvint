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
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() < 1 || exit.ExitCode() > 127 {
			return nil, false, err
		}
	}
	result, clean := resolveAppend(merged, size)
	return result, clean, nil
}

func resolveAppend(data []byte, size int) ([]byte, bool) {
	lines := strings.SplitAfter(string(data), "\n")
	lt, bar, eq, gt := strings.Repeat("<", size), strings.Repeat("|", size), strings.Repeat("=", size), strings.Repeat(">", size)
	var out bytes.Buffer
	clean := true
	for i := 0; i < len(lines); {
		line := lines[i]
		if !strings.HasPrefix(line, lt+" ") && strings.TrimRight(line, "\r\n") != lt {
			out.WriteString(line)
			i++
			continue
		}
		start, part := i, 0
		var sides [3]string
		i++
		for ; i < len(lines); i++ {
			s := lines[i]
			if strings.HasPrefix(s, bar) && part == 0 {
				part = 1
			} else if strings.TrimRight(s, "\r\n") == eq && part == 1 {
				part = 2
			} else if strings.HasPrefix(s, gt) && part == 2 {
				break
			} else {
				sides[part] += s
			}
		}
		if i == len(lines) {
			out.WriteString(strings.Join(lines[start:], ""))
			return out.Bytes(), false
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
	return out.Bytes(), clean
}
