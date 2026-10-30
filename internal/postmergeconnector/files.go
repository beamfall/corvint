package postmergeconnector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// File transport requires a trusted private single-writer directory. These
// checks reject static aliases; they do not qualify hostile directory races.
func safePath(name string) error {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return fmt.Errorf("path-must-be-clean-absolute")
	}
	for p := name; ; p = filepath.Dir(p) {
		i, e := os.Lstat(p)
		if os.IsNotExist(e) && p == name {
			continue
		}
		if e != nil {
			return fmt.Errorf("file-parent-unavailable")
		}
		if i.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink-path-refused")
		}
		if p == name {
			if !i.Mode().IsRegular() {
				return fmt.Errorf("nonregular-file-refused")
			}
		} else if !i.IsDir() {
			return fmt.Errorf("non-directory-parent")
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	return nil
}
func ReadFile(name string) ([]byte, error) {
	if err := safePath(name); err != nil {
		return nil, err
	}
	before, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	f, err := openRegular(name, before)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err == nil && len(b) > MaxBytes {
		err = fmt.Errorf("file-too-large")
	}
	return b, err
}
func within(root, name string) (bool, error) {
	rel, err := filepath.Rel(root, name)
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."), err
}

// CheckDestination applies to every mutable output, including read-intake.
func CheckDestination(name, authorRoot string, inputs []string) error {
	if err := safePath(name); err != nil {
		return err
	}
	if !filepath.IsAbs(authorRoot) || filepath.Clean(authorRoot) != authorRoot {
		return fmt.Errorf("author-root-must-be-clean-absolute-directory")
	}
	author, err := filepath.EvalSymlinks(authorRoot)
	if err != nil {
		return fmt.Errorf("author-root-unavailable")
	}
	info, err := os.Stat(author)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("author-root-must-be-clean-absolute-directory")
	}
	inside, err := within(author, name)
	if err != nil {
		return fmt.Errorf("author-containment-unavailable")
	}
	if inside {
		return fmt.Errorf("output-inside-author-worktree")
	}
	dst, err := os.Lstat(name)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && !singleLink(dst) {
		return fmt.Errorf("output-hardlink-refused")
	}
	for _, input := range inputs {
		if input == name {
			return fmt.Errorf("output-input-alias")
		}
		i, e := os.Stat(input)
		if e != nil {
			return fmt.Errorf("input-unavailable")
		}
		if i.IsDir() {
			protected, e := filepath.EvalSymlinks(input)
			if e != nil {
				return fmt.Errorf("protected-directory-unavailable")
			}
			inside, e := within(protected, name)
			if e != nil || inside {
				return fmt.Errorf("output-inside-protected-directory")
			}
		}
		if dst != nil && os.SameFile(i, dst) {
			return fmt.Errorf("output-input-alias")
		}
	}
	return nil
}
func atomicWrite(name, authorRoot string, inputs []string, b []byte) error {
	if len(b) > MaxBytes {
		return fmt.Errorf("output-too-large")
	}
	if err := CheckDestination(name, authorRoot, inputs); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".postmerge-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	n, e := f.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	if err := CheckDestination(name, authorRoot, inputs); err != nil {
		return err
	}
	return os.Rename(tmp, name)
}
func SaveIntake(name, authorRoot string, inputs []string, in Intake) error {
	b, e := Encode(in)
	if e != nil {
		return e
	}
	return atomicWrite(name, authorRoot, inputs, b)
}
func SaveState(name, authorRoot string, inputs []string, state State) error {
	b, e := Encode(state)
	if e != nil {
		return e
	}
	return atomicWrite(name, authorRoot, inputs, b)
}

// Record reconciles exact request repeats in a canonical JSONL ledger. Updating
// a body appends a new observation with the same external item key.
func Record(ctx context.Context, root string, f Fixture, p Policy, plan Plan, name, authorRoot string, inputs []string) error {
	if err := ValidatePlan(ctx, root, f, p, plan); err != nil {
		return err
	}
	protected, err := ProtectedGitPaths(ctx, root)
	if err != nil {
		return err
	}
	inputs = append(append([]string(nil), inputs...), protected...)
	if err := CheckDestination(name, authorRoot, inputs); err != nil {
		return err
	}
	prior := []byte{}
	seen := map[string]bool{}
	if _, e := os.Lstat(name); e == nil {
		var err error
		prior, err = ReadFile(name)
		if err != nil {
			return err
		}
		if len(prior) > 0 && prior[len(prior)-1] != '\n' {
			return fmt.Errorf("ledger-incomplete")
		}
		for _, line := range bytes.Split(prior, []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			var r Request
			if Decode(line, &r) != nil || r.Profile != Profile || !strings.HasPrefix(r.Key, "pm-") {
				return fmt.Errorf("ledger-invalid")
			}
			b, _ := Encode(r)
			if !bytes.Equal(b[:len(b)-1], line) {
				return fmt.Errorf("ledger-noncanonical")
			}
			seen[digest(r)] = true
		}
	}
	for _, r := range plan.Requests {
		if !seen[digest(r)] {
			b, _ := Encode(r)
			prior = append(prior, b...)
			seen[digest(r)] = true
		}
	}
	return atomicWrite(name, authorRoot, inputs, prior)
}
