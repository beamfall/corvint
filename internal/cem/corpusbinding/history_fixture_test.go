package corpusbinding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	publicRoot     = "9477995d776ddf88d7f6a82790dd9c22d51da765"
	archiveRoot    = "1c4d8793cf71cbb01b8509b68a6cb1502a40a13e"
	manifestSHA256 = "a5d7fdef6e4884e45d125806ba2ce2387b22da7994f05cef9fb8517bbae71beb"
	fixturePath    = "testdata/public-root-history/"
)

//go:embed testdata/public-root-history/*
var historicalFiles embed.FS

type fixtureObject struct {
	OID  string `json:"oid"`
	Type string `json:"type"`
	Size int    `json:"size"`
}
type retainedFixture struct {
	Name   string `json:"name"`
	OID    string `json:"oid"`
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}
type fixtureNotice struct {
	File       string   `json:"file"`
	OID        string   `json:"oid"`
	SourcePath string   `json:"sourcePath"`
	SHA256     string   `json:"sha256"`
	Commits    []string `json:"commits"`
}
type historyManifest struct {
	Profile                string            `json:"profile"`
	ArchiveRepository      string            `json:"archiveRepository"`
	ArchiveCommit          string            `json:"archiveCommit"`
	PublicRoot             string            `json:"publicRoot"`
	PackSHA256             string            `json:"packSHA256"`
	PackBytes              int               `json:"packBytes"`
	Objects                []fixtureObject   `json:"objects"`
	Fixtures               []retainedFixture `json:"fixtures"`
	Notices                []fixtureNotice   `json:"notices"`
	SourceAncestryObserved []string          `json:"sourceAncestryObserved"`
}
type historyFixture struct {
	repo     string
	manifest historyManifest
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readHistoryInputs() (historyManifest, []byte, error) {
	var m historyManifest
	raw, err := historicalFiles.ReadFile(fixturePath + "manifest.json")
	if err != nil {
		return m, nil, err
	}
	if digestBytes(raw) != manifestSHA256 {
		return m, nil, fmt.Errorf("historical manifest digest mismatch")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&m); err != nil {
		return m, nil, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return m, nil, fmt.Errorf("historical manifest trailing data: %v", err)
	}
	if m.Profile != "corvint-public-root-corpus-fixture/1" || m.PublicRoot != publicRoot || m.ArchiveCommit != archiveRoot {
		return m, nil, fmt.Errorf("historical manifest identity mismatch")
	}
	encoded, err := historicalFiles.ReadFile(fixturePath + "objects.pack.base64")
	if err != nil {
		return m, nil, err
	}
	// Size is pinned by the manifest digest, not an operator-controlled limit.
	if len(encoded) > base64.StdEncoding.EncodedLen(m.PackBytes)*2 {
		return m, nil, fmt.Errorf("historical encoded pack exceeds pinned bound")
	}
	pack, err := base64.StdEncoding.Strict().DecodeString(string(encoded))
	return m, pack, err
}

func openHistoryFixture(parent string) (*historyFixture, error) {
	m, pack, err := readHistoryInputs()
	if err != nil {
		return nil, err
	}
	h, err := importHistoryPack(parent, m, pack)
	if err != nil {
		return nil, err
	}
	for _, notice := range m.Notices {
		data, err := historicalFiles.ReadFile(fixturePath + notice.File)
		if err != nil || digestBytes(data) != notice.SHA256 {
			return nil, fmt.Errorf("historical notice %s mismatch: %v", notice.File, err)
		}
		blob, err := readGitBlob(h.repo, notice.OID)
		if err != nil || !bytes.Equal(data, blob) {
			return nil, fmt.Errorf("historical notice %s object mismatch: %v", notice.File, err)
		}
		for _, commit := range notice.Commits {
			original, err := readGitBlob(h.repo, commit+":"+notice.SourcePath)
			if err != nil || !bytes.Equal(original, data) {
				return nil, fmt.Errorf("historical notice %s source mismatch at %s: %v", notice.File, commit, err)
			}
		}
	}
	return h, nil
}

func importHistoryPack(parent string, m historyManifest, pack []byte) (*historyFixture, error) {
	if len(pack) != m.PackBytes || digestBytes(pack) != m.PackSHA256 {
		return nil, fmt.Errorf("historical pack digest/size mismatch")
	}
	template := filepath.Join(parent, "empty-template")
	if err := os.Mkdir(template, 0700); err != nil {
		return nil, err
	}
	repo := filepath.Join(parent, "objects.git")
	if _, err := fixtureGit(parent, nil, 4096, "init", "--bare", "--object-format=sha1", "--template="+template, repo); err != nil {
		return nil, err
	}
	// No --fix-thin: every delta base must be inside this bounded pack.
	if _, err := fixtureGit(repo, pack, 1024, "index-pack", "--stdin"); err != nil {
		return nil, fmt.Errorf("historical pack import: %w", err)
	}
	if err := checkObjectInventory(repo, m.Objects); err != nil {
		return nil, err
	}
	return &historyFixture{repo: repo, manifest: m}, nil
}

func checkObjectInventory(repo string, objects []fixtureObject) error {
	raw, err := fixtureGit(repo, nil, (len(objects)+1)*80, "cat-file", "--batch-all-objects", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
	if err != nil {
		return fmt.Errorf("historical object inventory: %w", err)
	}
	var want []string
	for _, object := range objects {
		want = append(want, fmt.Sprintf("%s %s %d", object.OID, object.Type, object.Size))
	}
	got := strings.Split(strings.TrimSpace(string(raw)), "\n")
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("historical object inventory mismatch")
	}
	return nil
}

func (h *historyFixture) rootFixtures(repo string) (map[string][]byte, error) {
	if _, err := fixtureGit(repo, nil, 1024, "merge-base", "--is-ancestor", publicRoot, "HEAD"); err != nil {
		return nil, &proofUnavailable{fmt.Errorf("public root %s unavailable or not an ancestor: %w", publicRoot, err)}
	}
	raw, err := fixtureGit(repo, nil, 8192, "ls-tree", "--full-tree", "-rz", "--name-only", publicRoot, "--", ".corvint/changes")
	if err != nil {
		return nil, &proofUnavailable{err}
	}
	var want []string
	for _, f := range h.manifest.Fixtures {
		want = append(want, ".corvint/changes/"+f.Name)
	}
	sort.Strings(want)
	got := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return nil, fmt.Errorf("historical root fixture inventory mismatch")
	}
	retained := make(map[string][]byte, len(want))
	for _, f := range h.manifest.Fixtures {
		path := ":.corvint/changes/" + f.Name
		data, err := readGitBlob(repo, publicRoot+path)
		if err != nil {
			return nil, err
		}
		if len(data) != f.Size || digestBytes(data) != f.SHA256 {
			return nil, fmt.Errorf("historical root bytes mismatch: %s", f.Name)
		}
		archived, err := readGitBlob(h.repo, archiveRoot+path)
		if err != nil || !bytes.Equal(data, archived) {
			return nil, fmt.Errorf("historical archive path mismatch: %s: %v", f.Name, err)
		}
		object, err := readGitBlob(h.repo, f.OID)
		if err != nil || !bytes.Equal(data, object) {
			return nil, fmt.Errorf("historical fixture blob mismatch: %s: %v", f.Name, err)
		}
		retained[f.Name] = data
	}
	return retained, nil
}

// All Git subprocesses, including current-repository reads, discard inherited
// repository/object/config injection. The fixture repository has no alternates,
// refs or remotes; its template is empty and it never borrows host objects.
func fixtureGitEnv() []string {
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			env = append(env, item)
		}
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
}

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		return 0, fmt.Errorf("Git output exceeds pinned/read-object bound")
	}
	return b.buffer.Write(p)
}

func fixtureGit(repo string, input []byte, limit int, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	argv := append([]string{"-c", "protocol.allow=never", "-C", repo}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = fixtureGitEnv()
	cmd.Stdin = bytes.NewReader(input)
	stdout, stderr := &cappedBuffer{limit: limit}, &cappedBuffer{limit: 4096}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.buffer.String())
	}
	return stdout.buffer.Bytes(), nil
}

func readGitBlob(repo, name string) ([]byte, error) {
	raw, err := fixtureGit(repo, nil, 128, "cat-file", "-s", name)
	if err != nil {
		return nil, &proofUnavailable{fmt.Errorf("object %s unavailable: %w", name, err)}
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || size < 0 {
		return nil, fmt.Errorf("object %s has invalid size", name)
	}
	data, err := fixtureGit(repo, nil, size, "cat-file", "blob", name)
	if err != nil {
		return nil, &proofUnavailable{fmt.Errorf("object %s unavailable: %w", name, err)}
	}
	if len(data) != size {
		return nil, fmt.Errorf("object %s read size mismatch", name)
	}
	return data, nil
}
