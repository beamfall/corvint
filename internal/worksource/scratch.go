package worksource

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidateScratchLocation resolves only the ordinary on-disk .git/commondir
// layout, without launching Git or creating scratch. Marker text is not authority
// to write in caller metadata, including an external linked-worktree common dir.
func ValidateScratchLocation(ctx context.Context, directory, scratch string) error {
	if scratch == "" {
		return nil
	}
	scratchAbsolute, err := filepath.Abs(scratch)
	if err != nil {
		return err
	}
	candidate, err := filepath.EvalSymlinks(scratchAbsolute)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	current, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return err
	}
	for depth := 0; depth < 128; depth++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		marker := filepath.Join(current, ".git")
		info, err := os.Lstat(marker)
		if err == nil {
			return validateScratchMetadata(ctx, current, marker, info, candidate)
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return errors.New("ordinary repository metadata unavailable for scratch validation")
}

func validateScratchMetadata(ctx context.Context, repository, marker string, info os.FileInfo, scratch string) error {
	gitDirectory := marker
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return errors.New("unsupported Git marker for scratch validation")
		}
		raw, err := readMetadataFile(repository, ".git")
		if err != nil {
			return err
		}
		if !strings.HasPrefix(string(raw), "gitdir: ") {
			return errors.New("invalid Git marker")
		}
		gitDirectory, err = metadataDirectory(repository, strings.TrimPrefix(string(raw), "gitdir: "))
		if err != nil {
			return err
		}
	}
	gitDirectory, err := filepath.EvalSymlinks(gitDirectory)
	if err != nil {
		return err
	}
	commonDirectory := gitDirectory
	if _, err := os.Lstat(filepath.Join(gitDirectory, "commondir")); err == nil {
		raw, err := readMetadataFile(gitDirectory, "commondir")
		if err != nil {
			return err
		}
		commonDirectory, err = metadataDirectory(gitDirectory, string(raw))
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	protected := []string{repository, gitDirectory, commonDirectory}
	for _, forbidden := range protected {
		if scratch == forbidden || strings.HasPrefix(scratch, forbidden+string(os.PathSeparator)) {
			return errors.New("source scratch cannot write repository or Git metadata")
		}
	}
	return scratchAncestorsOutside(ctx, scratch, protected)
}

func metadataDirectory(base, value string) (string, error) {
	value = strings.TrimSuffix(value, "\n")
	if value == "" || !utf8.ValidString(value) {
		return "", errors.New("invalid metadata directory")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", errors.New("invalid metadata directory")
		}
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(base, value)
	}
	resolved, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("metadata directory unavailable")
	}
	return resolved, nil
}

func readMetadataFile(directory, name string) ([]byte, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Size() > 16384 {
		return nil, errors.New("unsupported metadata file")
	}
	file, err := root.OpenFile(name, noFollowReadFlags(), 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !sameEntry(before, opened) {
		return nil, errors.New("metadata file drift")
	}
	raw, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil || len(raw) > 16384 {
		return nil, errors.New("incomplete metadata file")
	}
	after, err := root.Lstat(name)
	final, statErr := file.Stat()
	if err != nil || statErr != nil || !sameEntry(opened, after) || !sameEntry(opened, final) || int64(len(raw)) != opened.Size() {
		return nil, errors.New("metadata file drift")
	}
	return raw, nil
}

// EvalSymlinks preserves case/Unicode spelling on some filesystems. Match the
// actual directory objects of every ancestor, not only their path strings.
func scratchAncestorsOutside(ctx context.Context, scratch string, protected []string) error {
	identities := make([]os.FileInfo, 0, len(protected))
	for _, path := range protected {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return errors.New("protected scratch scope unavailable")
		}
		identities = append(identities, info)
	}
	for depth := 0; depth < 128; depth++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Stat(scratch)
		if err != nil || !info.IsDir() {
			return errors.New("scratch ancestor unavailable")
		}
		for _, identity := range identities {
			if os.SameFile(info, identity) {
				return errors.New("source scratch aliases repository or Git metadata")
			}
		}
		parent := filepath.Dir(scratch)
		if parent == scratch {
			return nil
		}
		scratch = parent
	}
	return errors.New("scratch ancestry exceeds bound")
}
