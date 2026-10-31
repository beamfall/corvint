package behaviorfalsify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// gorootMembers are the GOROOT entries a go test run reads: the driver's release files, its
// binaries, the timezone and other runtime data, the assembler headers, the compiler toolchain and
// the standard library sources (BBF-V0-820). Missing optional members are skipped.
var gorootMembers = []string{"VERSION", "go.env", "bin", "lib", filepath.Join("pkg", "include"), filepath.Join("pkg", "tool"), "src"}

var (
	errGOROOTChanged     = errors.New("provider toolchain GOROOT changed before staging")
	errGOROOTUnsupported = errors.New("provider toolchain GOROOT holds an unsupported entry")
	// errBudgetExhausted is the refusal when the run is cancelled or its command budget is spent
	// while the GOROOT is digested or copied (BBF-V0-820).
	errBudgetExhausted = errors.New("command budget exhausted")
)

// gorootDigest digests every regular file under the GOROOT members, in lexical walk order, as
// its relative path, executable bit, size and content digest. When into is not empty each file is
// also written there from the same bytes it digests, so the copy holds exactly what was digested.
func gorootDigest(ctx context.Context, goroot, into string) (string, error) {
	tree := sha256.New()
	for _, member := range gorootMembers {
		start := filepath.Join(goroot, member)
		if _, err := os.Lstat(start); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		err := filepath.WalkDir(start, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return errBudgetExhausted
			}
			relative, err := filepath.Rel(goroot, path)
			if err != nil {
				return err
			}
			switch {
			case entry.IsDir() && entry.Name() == "testdata":
				return filepath.SkipDir
			case entry.IsDir():
				if into != "" {
					return os.MkdirAll(filepath.Join(into, relative), 0o700)
				}
				return nil
			case !entry.Type().IsRegular():
				return errGOROOTUnsupported
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			sum, err := digestCopy(ctx, path, into, relative, info.Mode().Perm()&0o111 != 0)
			if err != nil {
				return err
			}
			fmt.Fprintf(tree, "%s\x00%t\x00%d\x00%s\n", filepath.ToSlash(relative), info.Mode().Perm()&0o111 != 0, info.Size(), sum)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(tree.Sum(nil)), nil
}

// digestCopy reads path once, returning its content digest and, when into is set, writing the
// same bytes to into/relative.
func digestCopy(ctx context.Context, path, into, relative string, executable bool) (string, error) {
	source, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer source.Close()
	hash := sha256.New()
	writer := io.Writer(hash)
	var target *os.File
	if into != "" {
		mode := os.FileMode(0o600)
		if executable {
			mode = 0o700
		}
		if target, err = os.OpenFile(filepath.Join(into, relative), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode); err != nil {
			return "", err
		}
		writer = io.MultiWriter(hash, target)
	}
	_, err = io.Copy(writer, contextReader{ctx: ctx, r: source})
	if target != nil {
		if closeErr := target.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// contextReader stops a copy at the next read once ctx is done.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c contextReader) Read(p []byte) (int, error) {
	if c.ctx.Err() != nil {
		return 0, errBudgetExhausted
	}
	return c.r.Read(p)
}

// stageGOROOT copies the GOROOT members into private, refusing when the bytes it copies do not
// digest to want, the digest taken at the driver's check (BBF-V0-820).
func stageGOROOT(ctx context.Context, goroot, want, private string) error {
	if err := os.MkdirAll(private, 0o700); err != nil {
		return errors.New("provider toolchain GOROOT staging failed")
	}
	got, err := gorootDigest(ctx, goroot, private)
	switch {
	case errors.Is(err, errGOROOTUnsupported), errors.Is(err, errBudgetExhausted):
		return err
	case err != nil:
		return errors.New("provider toolchain GOROOT staging failed")
	case got != want:
		return errGOROOTChanged
	}
	return nil
}

// placeVerifiedDriver replaces the private GOROOT's copy of the driver with the verified driver
// bytes staged at staged, so the driver that runs is the checked one (BBF-V0-214).
func placeVerifiedDriver(staged, private, goroot, driverPath string) (string, error) {
	relative, err := filepath.Rel(goroot, driverPath)
	if err != nil {
		return "", errors.New("provider toolchain GOROOT staging failed")
	}
	verified, err := os.ReadFile(staged)
	if err != nil {
		return "", errors.New("provider toolchain GOROOT staging failed")
	}
	driver := filepath.Join(private, relative)
	if err := os.MkdirAll(filepath.Dir(driver), 0o700); err != nil {
		return "", errors.New("provider toolchain GOROOT staging failed")
	}
	if err := os.Remove(driver); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", errors.New("provider toolchain GOROOT staging failed")
	}
	if err := os.WriteFile(driver, verified, 0o700); err != nil {
		return "", errors.New("provider toolchain GOROOT staging failed")
	}
	return driver, nil
}
