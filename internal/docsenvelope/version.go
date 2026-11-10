// SPDX-License-Identifier: Apache-2.0

package docsenvelope

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/versionline"
)

const (
	// VersionMismatchCode is the coded refusal an executable emits when the
	// sibling it would run is installed beside it at another version
	// (CMS-V0-002: every executable refuses to run beside a mismatched
	// version, with a named error).
	VersionMismatchCode = "version-mismatch"
	// versionProbeTimeout bounds the sibling's --version answer.
	versionProbeTimeout = 10 * time.Second
	// versionProbeLimit bounds the bytes read from it: one line.
	versionProbeLimit = 4096
)

// Beside reports whether executable lives in the running executable's own
// directory, the place Locate looks first. Only a sibling installed there is
// part of this install's set; one found on PATH is not checked.
func Beside(executable string) bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	return filepath.Dir(self) == filepath.Dir(executable)
}

// ProbeVersion runs `executable --version` with no inherited arguments, reads
// one bounded line and parses it: the `NAME VERSION (build N, commit SHA)`
// form (PUB-V0-053) or the plain `NAME VERSION` an unstamped companion prints.
func ProbeVersion(ctx context.Context, executable string) (versionline.Line, error) {
	ctx, cancel := context.WithTimeout(ctx, versionProbeTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "--version")
	command.Stdin = nil
	var stdout bytes.Buffer
	command.Stdout = &limitedWriter{w: &stdout, remaining: versionProbeLimit}
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		return versionline.Line{}, fmt.Errorf("%s --version: %w", filepath.Base(executable), err)
	}
	return ParseVersionLine(stdout.String())
}

// ParseVersionLine reads a plain --version line of either form.
func ParseVersionLine(text string) (versionline.Line, error) {
	text = strings.TrimSpace(text)
	if line, ok := versionline.Parse(text); ok {
		return line, nil
	}
	fields := strings.Fields(text)
	if len(fields) == 2 && !strings.ContainsAny(text, "\n\r") {
		return versionline.Line{Name: fields[0], Version: fields[1]}, nil
	}
	return versionline.Line{}, fmt.Errorf("--version printed %q, not a version line", text)
}

// Mismatch names why other is not the same release as self, or returns "" when
// the two agree: the versions must be equal, and when both lines carry a
// stamped commit those must be equal too. Two unstamped development builds
// agree on their version alone; a release build never agrees with a
// development build, because the release prints the compiled version and the
// development build prints its `.dev` form (PUB-V0-052).
func Mismatch(self, other versionline.Line) string {
	if self.Version != other.Version {
		return fmt.Sprintf("%s is version %s, %s is version %s", other.Name, other.Version, self.Name, self.Version)
	}
	if self.Commit != "" && other.Commit != "" && self.Commit != "unknown" && other.Commit != "unknown" && self.Commit != other.Commit {
		return fmt.Sprintf("%s was built from commit %s, %s from commit %s", other.Name, other.Commit, self.Name, self.Commit)
	}
	return ""
}

// CheckBeside runs the CMS-V0-002 check for a sibling Locate found: when it is
// installed beside the running executable, its --version must agree with self.
// The returned message is empty when the sibling is on PATH or agrees; a
// sibling that cannot state its version is a mismatch, never a pass.
func CheckBeside(ctx context.Context, executable string, self versionline.Line) string {
	if !Beside(executable) {
		return ""
	}
	other, err := ProbeVersion(ctx, executable)
	if err != nil {
		return filepath.Base(executable) + " beside " + self.Name + " did not state its version (" + err.Error() + "); install the set at one version"
	}
	reason := Mismatch(self, other)
	if reason == "" {
		return ""
	}
	return reason + "; the executables beside each other must be one release: install the set at one version"
}

// limitedWriter discards bytes past its bound so a hostile --version cannot
// grow memory; the probe reads one line.
type limitedWriter struct {
	w         io.Writer
	remaining int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > l.remaining {
		p = p[:l.remaining]
	}
	l.remaining -= len(p)
	if len(p) > 0 {
		if _, err := l.w.Write(p); err != nil {
			return 0, err
		}
	}
	return n, nil
}
