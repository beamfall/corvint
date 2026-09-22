package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxPointerBytes = 4_096
	maxConfigBytes  = 1_048_576
	maxConfigLine   = 4_096
)

func prequalifyLayout(ctx context.Context, root string) (layout, *Failure) {
	worktree, failure := openDirectory(root)
	if failure != nil {
		return layout{}, failure
	}
	value := layout{worktree: worktree}
	fail := func(reason FailureReason) (layout, *Failure) {
		closeLayout(&value)
		return layout{}, unavailable(reason)
	}
	failWith := func(failure *Failure) (layout, *Failure) {
		closeLayout(&value)
		return layout{}, failure
	}

	gitPath := filepath.Join(root, ".git")
	gitInfo, err := value.worktree.root.Lstat(".git")
	if err != nil || gitInfo.Mode()&os.ModeSymlink != 0 {
		return fail(ReasonUnavailable)
	}
	if gitInfo.IsDir() {
		gitDirectory, openFailure := openChildDirectory(value.worktree, ".git", gitPath)
		if openFailure != nil {
			return failWith(openFailure)
		}
		value.gitDir = gitDirectory
		value.gitEntry = stableFileEvidence{identity: gitDirectory.identity}
	} else {
		gitEntry, body, readFailure := stableReadRelative(ctx, value.worktree, ".git", maxPointerBytes)
		if readFailure != nil {
			return fail(ReasonUnavailable)
		}
		value.gitEntry = gitEntry
		value.pointer = gitEntry
		target, ok := parsePointerBody(body, "gitdir: ", filepath.Dir(gitPath))
		if !ok {
			return fail(ReasonUnavailable)
		}
		gitDirectory, openFailure := openDirectory(target)
		if openFailure != nil {
			return failWith(openFailure)
		}
		value.gitDir = gitDirectory
	}

	commonPath := value.gitDir.path
	commonFilePath := filepath.Join(value.gitDir.path, "commondir")
	if _, err := value.gitDir.root.Lstat("commondir"); err == nil {
		commonFile, body, readFailure := stableReadRelative(ctx, value.gitDir, "commondir", maxPointerBytes)
		if readFailure != nil {
			return fail(ReasonUnavailable)
		}
		value.commonFile = commonFile
		target, ok := parsePointerBody(body, "", filepath.Dir(commonFilePath))
		if !ok {
			return fail(ReasonUnavailable)
		}
		commonPath = target
	} else if !os.IsNotExist(err) {
		return fail(ReasonUnavailable)
	}
	var common directoryBinding
	var openFailure *Failure
	if value.commonFile.identity.platformKind == 0 {
		common, openFailure = openChildDirectory(value.gitDir, ".", value.gitDir.path)
	} else {
		common, openFailure = openDirectory(commonPath)
	}
	if openFailure != nil {
		return failWith(openFailure)
	}
	value.common = common
	objects, openFailure := openChildDirectory(common, "objects", filepath.Join(common.path, "objects"))
	if openFailure != nil {
		return failWith(openFailure)
	}
	value.objects = objects

	if value.pointer.bytes != 0 {
		reciprocalPath := filepath.Join(value.gitDir.path, "gitdir")
		reciprocal, body, readFailure := stableReadRelative(ctx, value.gitDir, "gitdir", maxPointerBytes)
		if readFailure != nil {
			return fail(ReasonUnavailable)
		}
		target, ok := parsePointerBody(body, "", filepath.Dir(reciprocalPath))
		if !ok {
			return fail(ReasonUnavailable)
		}
		targetEvidence, _, readFailure := stableReadFile(ctx, target, maxPointerBytes)
		if readFailure != nil || targetEvidence.identity != value.gitEntry.identity {
			return fail(ReasonUnavailable)
		}
		value.reciprocal = reciprocal
	}

	for _, base := range uniqueDirectoryBindings(value.gitDir, value.common) {
		if entryExistsRelative(base, filepath.Join("objects", "info", "alternates")) {
			return fail(ReasonUnsupportedAlternates)
		}
	}

	type configLocation struct {
		directory directoryBinding
		name      string
	}
	configLocations := []configLocation{{directory: value.common, name: "config"}}
	for _, base := range uniqueDirectoryBindings(value.common, value.gitDir) {
		if _, err := base.root.Lstat("config.worktree"); err == nil {
			configLocations = append(configLocations, configLocation{directory: base, name: "config.worktree"})
			if base.identity == value.common.identity {
				value.commonWorktreeConfig = true
			}
			if base.identity == value.gitDir.identity {
				value.gitWorktreeConfig = true
			}
		} else if !os.IsNotExist(err) {
			return fail(ReasonUnavailable)
		}
	}
	configEvidence := make([]stableFileEvidence, 0, len(configLocations))
	for _, location := range configLocations {
		evidence, body, readFailure := stableReadRelative(ctx, location.directory, location.name, maxConfigBytes)
		if readFailure != nil || !validRepositoryConfig(body) {
			return fail(ReasonUnavailable)
		}
		duplicate := false
		for _, existing := range configEvidence {
			if existing.identity == evidence.identity {
				duplicate = true
				break
			}
		}
		if !duplicate {
			configEvidence = append(configEvidence, evidence)
		}
	}
	sort.Slice(configEvidence, func(left, right int) bool {
		return lessFileIdentity(configEvidence[left].identity, configEvidence[right].identity)
	})
	value.configCount = uint8(len(configEvidence))
	copy(value.configs[:], configEvidence)
	return value, nil
}

func retainedLayoutFilesStable(ctx context.Context, value layout) bool {
	if value.pointer.identity.platformKind != 0 {
		evidence, _, failure := stableReadRelative(ctx, value.worktree, ".git", maxPointerBytes)
		if failure != nil || evidence != value.pointer {
			return false
		}
	}
	if value.commonFile.identity.platformKind != 0 {
		evidence, _, failure := stableReadRelative(ctx, value.gitDir, "commondir", maxPointerBytes)
		if failure != nil || evidence != value.commonFile {
			return false
		}
	} else {
		if _, err := value.gitDir.root.Lstat("commondir"); !os.IsNotExist(err) {
			return false
		}
	}
	if value.reciprocal.identity.platformKind != 0 {
		evidence, _, failure := stableReadRelative(ctx, value.gitDir, "gitdir", maxPointerBytes)
		if failure != nil || evidence != value.reciprocal {
			return false
		}
	}
	for _, directory := range uniqueDirectoryBindings(value.gitDir, value.common) {
		if entryExistsRelative(directory, filepath.Join("objects", "info", "alternates")) {
			return false
		}
	}

	type configLocation struct {
		directory directoryBinding
		expected  bool
	}
	locations := []configLocation{{directory: value.common, expected: value.commonWorktreeConfig}}
	if value.gitDir.identity != value.common.identity {
		locations = append(locations, configLocation{directory: value.gitDir, expected: value.gitWorktreeConfig})
	}
	configs := make([]stableFileEvidence, 0, 3)
	commonConfig, _, failure := stableReadRelative(ctx, value.common, "config", maxConfigBytes)
	if failure != nil {
		return false
	}
	configs = append(configs, commonConfig)
	for _, location := range locations {
		_, err := location.directory.root.Lstat("config.worktree")
		present := err == nil
		if !present && !os.IsNotExist(err) || present != location.expected {
			return false
		}
		if present {
			evidence, _, readFailure := stableReadRelative(ctx, location.directory, "config.worktree", maxConfigBytes)
			if readFailure != nil {
				return false
			}
			duplicate := false
			for _, existing := range configs {
				if existing.identity == evidence.identity {
					duplicate = true
					break
				}
			}
			if !duplicate {
				configs = append(configs, evidence)
			}
		}
	}
	sort.Slice(configs, func(left, right int) bool {
		return lessFileIdentity(configs[left].identity, configs[right].identity)
	})
	if len(configs) != int(value.configCount) {
		return false
	}
	for index := range configs {
		if configs[index] != value.configs[index] {
			return false
		}
	}
	return true
}

func stableReadFile(ctx context.Context, path string, limit uint64) (stableFileEvidence, []byte, *Failure) {
	parent, failure := openDirectory(filepath.Dir(path))
	if failure != nil {
		return stableFileEvidence{}, nil, failure
	}
	defer closeDirectoryBinding(&parent)
	return stableReadRelative(ctx, parent, filepath.Base(path), limit)
}

func stableReadRelative(ctx context.Context, parent directoryBinding, name string, limit uint64) (stableFileEvidence, []byte, *Failure) {
	if parent.root == nil || name == "" || name == "." || filepath.Base(name) != name {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	before, err := parent.root.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() ||
		before.Size() < 0 || uint64(before.Size()) > limit {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	file, opened := openNoFollowFile(parent.root, name)
	if !opened {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	defer file.Close()
	stat1, err := file.Stat()
	identity1, ok := identityFromFile(file, stat1)
	beforeIdentity, beforeQualified := identityFromFile(file, before)
	if err != nil || !ok || !stat1.Mode().IsRegular() || !os.SameFile(before, stat1) ||
		!beforeQualified || beforeIdentity != identity1 || identity1.linkCount != 1 || uint64(stat1.Size()) > limit {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	first, ok := readBoundedFile(ctx, file, limit)
	if !ok {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	stat2, err := file.Stat()
	identity2, identityOK := identityFromFile(file, stat2)
	if err != nil || !identityOK || identity2 != identity1 {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	stat3, err := file.Stat()
	identity3, identityOK := identityFromFile(file, stat3)
	if err != nil || !identityOK || identity3 != identity1 {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	second, ok := readBoundedFile(ctx, file, limit)
	if !ok {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	stat4, err := file.Stat()
	identity4, identityOK := identityFromFile(file, stat4)
	if err != nil || !identityOK || identity4 != identity1 || !bytes.Equal(first, second) {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	after, err := parent.root.Lstat(name)
	afterIdentity, afterQualified := identityFromFile(file, after)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, after) ||
		!afterQualified || afterIdentity != identity1 {
		return stableFileEvidence{}, nil, unavailable(ReasonUnavailable)
	}
	return stableFileEvidence{identity: identity1, bytes: uint64(len(first)), digest: sha256.Sum256(first)}, first, nil
}

func readBoundedFile(ctx context.Context, file *os.File, limit uint64) ([]byte, bool) {
	reader := &contextReader{ctx: ctx, reader: file}
	output := make([]byte, 0, min(int(limit), 64<<10))
	buffer := make([]byte, 64<<10)
	for {
		remaining := limit + 1 - uint64(len(output))
		chunk := buffer
		if uint64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		count, err := reader.Read(chunk)
		if count > 0 {
			output = append(output, chunk[:count]...)
			if uint64(len(output)) > limit {
				return nil, false
			}
		}
		if err == io.EOF {
			return output, true
		}
		if err != nil || count == 0 {
			return nil, false
		}
	}
}

func parsePointerBody(body []byte, prefix, parent string) (string, bool) {
	if len(body) == 0 || body[len(body)-1] != '\n' || bytes.Count(body, []byte{'\n'}) != 1 ||
		!bytes.HasPrefix(body, []byte(prefix)) {
		return "", false
	}
	raw := string(body[len(prefix) : len(body)-1])
	if raw == "" || !utf8.ValidString(raw) {
		return "", false
	}
	for _, character := range raw {
		if character == 0 || unicode.IsControl(character) {
			return "", false
		}
	}
	if filepath.Clean(raw) != raw {
		return "", false
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(parent, raw)
	}
	if !filepath.IsAbs(raw) || filepath.Clean(raw) != raw {
		return "", false
	}
	return raw, true
}

func validRepositoryConfig(body []byte) bool {
	if !utf8.Valid(body) || bytes.ContainsAny(body, "\r\x00\\") ||
		(len(body) != 0 && body[len(body)-1] != '\n') {
		return false
	}
	inSection := false
	for _, rawLine := range bytes.Split(body, []byte{'\n'}) {
		if len(rawLine) > maxConfigLine {
			return false
		}
		for _, character := range string(rawLine) {
			if unicode.IsControl(character) && character != '\t' {
				return false
			}
		}
		line := strings.Trim(string(rawLine), " \t")
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section, ok := parseConfigSection(line)
			if !ok {
				return false
			}
			folded := strings.ToLower(section)
			if folded == "include" || folded == "includeif" ||
				strings.HasPrefix(folded, "include.") || strings.HasPrefix(folded, "includeif.") {
				return false
			}
			inSection = true
			continue
		}
		if !inSection || !validConfigVariable(line) {
			return false
		}
	}
	return true
}

func parseConfigSection(line string) (string, bool) {
	if len(line) < 3 || line[0] != '[' || line[len(line)-1] != ']' {
		return "", false
	}
	inside := strings.Trim(line[1:len(line)-1], " \t")
	if inside == "" {
		return "", false
	}
	end := 0
	for end < len(inside) && isConfigNameByte(inside[end], end == 0) {
		end++
	}
	if end == 0 {
		return "", false
	}
	section := inside[:end]
	rest := strings.Trim(inside[end:], " \t")
	if rest == "" {
		return section, true
	}
	if len(rest) < 2 || rest[0] != '"' || rest[len(rest)-1] != '"' {
		return "", false
	}
	subsection := rest[1 : len(rest)-1]
	if strings.ContainsRune(subsection, '"') {
		return "", false
	}
	for _, character := range subsection {
		if character < 0x20 || character > 0x7e {
			return "", false
		}
	}
	return section, true
}

func validConfigVariable(line string) bool {
	end := 0
	for end < len(line) && isConfigVariableByte(line[end], end == 0) {
		end++
	}
	if end == 0 {
		return false
	}
	rest := strings.Trim(line[end:], " \t")
	if rest == "" {
		return true
	}
	if rest[0] != '=' {
		return false
	}
	value := strings.Trim(rest[1:], " \t")
	if strings.ContainsAny(value, "#;") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	return true
}

func isConfigNameByte(value byte, first bool) bool {
	if first {
		return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
	}
	return isConfigNameByte(value, true) || value >= '0' && value <= '9' || value == '.' || value == '-'
}

func isConfigVariableByte(value byte, first bool) bool {
	if first {
		return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
	}
	return isConfigVariableByte(value, true) || value >= '0' && value <= '9' || value == '-'
}

func entryExistsRelative(parent directoryBinding, path string) bool {
	if parent.root == nil {
		return true
	}
	_, err := parent.root.Lstat(path)
	return err == nil || !os.IsNotExist(err)
}

func uniqueDirectoryBindings(values ...directoryBinding) []directoryBinding {
	result := make([]directoryBinding, 0, len(values))
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1].identity != value.identity {
			result = append(result, value)
		}
	}
	return result
}

func lessFileIdentity(left, right fileIdentity) bool {
	if left.platformKind != right.platformKind {
		return left.platformKind < right.platformKind
	}
	if left.device != right.device {
		return left.device < right.device
	}
	if left.inode != right.inode {
		return left.inode < right.inode
	}
	if left.volumeSerial != right.volumeSerial {
		return left.volumeSerial < right.volumeSerial
	}
	return left.fileIndex < right.fileIndex
}
