package releasegate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type gitIdentity struct{ path, sha256 string }

func resolveGitExecutable(root, executable, pinnedSHA256 string) (gitIdentity, error) {
	if !filepath.IsAbs(executable) || !validSHA256(pinnedSHA256) {
		return gitIdentity{}, errors.New("release gate requires an externally pinned absolute Git executable and SHA-256")
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return gitIdentity{}, errors.New("Git executable symlink resolution failed")
	}
	if !filepath.IsAbs(resolved) {
		return gitIdentity{}, errors.New("Git executable is not absolute after resolution")
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return gitIdentity{}, errors.New("release root cannot be resolved")
	}
	if relative, err := filepath.Rel(rootResolved, resolved); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return gitIdentity{}, errors.New("Git executable must be external to the scanned release root")
	}
	identity := gitIdentity{path: resolved, sha256: pinnedSHA256}
	if err := verifyGitIdentity(identity); err != nil {
		return gitIdentity{}, err
	}
	return identity, nil
}

// verifyGitIdentity is intentionally repeated immediately before every process
// launch: pathname replacement after initial validation must not switch the
// executable underneath a committed scan.
func verifyGitIdentity(identity gitIdentity) error {
	if !filepath.IsAbs(identity.path) || !validSHA256(identity.sha256) {
		return errors.New("Git executable identity is incomplete")
	}
	info, err := os.Stat(identity.path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 || info.Size() <= 0 || info.Size() > maxBlobBytes {
		return errors.New("Git executable is not an executable regular file")
	}
	file, err := os.Open(identity.path)
	if err != nil {
		return errors.New("Git executable cannot be identity-bound")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBlobBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxBlobBytes {
		return errors.New("Git executable cannot be identity-bound")
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != identity.sha256 {
		return errors.New("Git executable digest does not match external pin")
	}
	return nil
}

func runGitAt(ctx context.Context, identity gitIdentity, root string, limit int, args ...string) ([]byte, error) {
	if err := verifyGitIdentity(identity); err != nil {
		return nil, err
	}
	call, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	return runGitProcess(call, identity, root, limit, nil, args...)
}

func batchReadBlobs(ctx context.Context, identity gitIdentity, root string, entries []treeEntry) (map[string][]byte, error) {
	unique := map[string]treeEntry{}
	var requests strings.Builder
	for _, entry := range entries {
		if entry.mode != "100644" && entry.mode != "100755" {
			continue
		}
		if entry.size > maxBlobBytes {
			return nil, errors.New("release tree blob exceeds batch bound")
		}
		if _, seen := unique[entry.oid]; !seen {
			unique[entry.oid] = entry
			requests.WriteString(entry.oid)
			requests.WriteByte('\n')
		}
	}
	if err := verifyGitIdentity(identity); err != nil {
		return nil, err
	}
	call, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	output, err := runGitProcess(call, identity, root, maxScanBytes, []byte(requests.String()), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(unique))
	for len(output) > 0 {
		line, rest, found := bytes.Cut(output, []byte{'\n'})
		if !found {
			return nil, errors.New("malformed cat-file batch header")
		}
		fields := strings.Fields(string(line))
		if len(fields) != 3 || fields[1] != "blob" || !validOID(fields[0]) {
			return nil, errors.New("malformed cat-file batch object")
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 || size > maxBlobBytes || int64(len(rest)) < size+1 {
			return nil, errors.New("invalid cat-file batch object size")
		}
		data := rest[:size]
		if rest[size] != '\n' {
			return nil, errors.New("malformed cat-file batch frame")
		}
		entry, ok := unique[fields[0]]
		if !ok || entry.size != size {
			return nil, errors.New("cat-file batch returned unexpected object")
		}
		if _, duplicate := result[fields[0]]; duplicate {
			return nil, errors.New("cat-file batch returned duplicate object")
		}
		result[fields[0]] = append([]byte(nil), data...)
		output = rest[size+1:]
	}
	if len(result) != len(unique) {
		return nil, errors.New("cat-file batch omitted tree blob")
	}
	return result, nil
}

type limitBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitBuffer) Write(v []byte) (int, error) {
	remain := b.limit - b.Len()
	if remain <= 0 {
		b.exceeded = true
		return len(v), nil
	}
	if len(v) > remain {
		_, _ = b.Buffer.Write(v[:remain])
		b.exceeded = true
		return len(v), nil
	}
	return b.Buffer.Write(v)
}
func sanitizedGitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, ok := os.LookupEnv(name); ok {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment, "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=")
}
