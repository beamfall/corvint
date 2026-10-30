//go:build darwin || linux

package gitstatus

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleConfigRecognizesOnlyProvenSHA1Subset(t *testing.T) {
	base := "[core]\nrepositoryformatversion = 0\nbare = false\n[user]\nname = Test User\n"
	if safe, sha1Format := simpleConfig([]byte(base)); !safe || !sha1Format {
		t.Fatal("ordinary explicit SHA-1 configuration missed")
	}
	if safe, sha1Format := simpleConfig([]byte("[user]\nname = Test\n")); !safe || sha1Format {
		t.Fatal("inert worktree config must not establish object format")
	}
	cloned := "[remote \"origin\"]\nurl = /srv/corvint.git\nfetch = +refs/heads/*:refs/remotes/origin/*\n" +
		"pushurl = /srv/push.git\npush = refs/heads/main\ntagOpt = --no-tags\n" +
		"[branch \"codex/lane-1\"]\nremote = origin\nmerge = refs/heads/codex/lane-1\nrebase = true\npushRemote = origin\n" +
		"[extensions]\nworktreeConfig = true\n"
	if safe, sha1Format := simpleConfig([]byte(base + cloned)); !safe || !sha1Format {
		t.Fatal("the configuration git clone and git worktree write missed")
	}
	for _, suffix := range []string{
		"[remote \"origin\"]\nurl = \"/srv/quoted.git\"\n",
		"[remote \"a\"b\"]\nurl = /srv/corvint.git\n",
		"[remote \"\"]\nurl = /srv/corvint.git\n",
		"[remote \"origin\"]\npromisor = true\n",
		"[remote.origin]\nurl = /srv/corvint.git\n",
		"[branch \"main\"]\ndescription = text\n",
		"[extensions]\nobjectformat = sha256\n",
		"[extensions]\nworktreeConfig = maybe\n",
		"[submodule \"vendored\"]\nurl = /srv/vendored.git\n",
		"[filter \"hostile\"]\nclean = touch marker\n",
		"[filter.hostile]\nprocess = touch marker\n",
		"[include]\npath = outside\n",
		"[includeIf \"gitdir:/\"]\npath = outside\n",
		"[core]\nworktree = outside\n",
		"[core]\nattributesFile = outside\n",
		"[core]\nrepositoryformatversion = 1\n[extensions]\nobjectformat = sha256\n",
		"[core]\nrepositoryformatversion = 2\n",
		"[user]\nname = first\\\n[filter.hostile]\nclean = touch marker\n",
		"[user]\nname = \"first\nsecond\"\n",
		"[user]\nname = Test\r[filter.hostile]\nclean = touch marker\n",
		"[user]\nname = Test\x00hidden\n",
		"# harmless syntax still belongs to Git's parser\n",
	} {
		if safe, _ := simpleConfig([]byte(base + suffix)); safe {
			t.Errorf("ambiguous or unsupported configuration admitted: %q", suffix)
		}
	}
}

func TestPrivateStatusSimpleMetadataAndFallbacks(t *testing.T) {
	for _, kind := range []string{"sha1-v2", "sha1-v3", "cloned", "sha256", "v4", "comment", "unknown-extension"} {
		t.Run(kind, func(t *testing.T) {
			var initArgs []string
			if kind == "sha256" {
				initArgs = []string{"--object-format=sha256"}
			}
			root := fixture(t, initArgs...)
			indexPath := filepath.Join(root, ".git", "index")
			switch kind {
			case "sha1-v3":
				gitTest(t, root, "update-index", "--index-version=3")
			case "cloned":
				gitTest(t, root, "remote", "add", "origin", filepath.Join(filepath.Dir(root), "origin.git"))
				gitTest(t, root, "config", "branch.main.remote", "origin")
				gitTest(t, root, "config", "branch.main.merge", "refs/heads/main")
				gitTest(t, root, "config", "extensions.worktreeConfig", "true")
			case "v4":
				gitTest(t, root, "update-index", "--index-version=4")
			case "comment":
				path := filepath.Join(root, ".git", "config")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeTest(t, path, string(data)+"# keep the real parser for comments\n")
			case "unknown-extension":
				data, err := os.ReadFile(indexPath)
				if err != nil {
					t.Fatal(err)
				}
				data = append(data[:len(data)-sha1.Size], []byte("ZZZZ\x00\x00\x00\x00")...)
				digest := sha1.Sum(data)
				if err := os.WriteFile(indexPath, append(data, digest[:]...), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			probed := false
			run := func(ctx context.Context, directory string, limit int, args ...string) ([]byte, error) {
				calls++
				for _, arg := range args {
					probed = probed || arg == "ls-files"
				}
				return testRun(ctx, directory, limit, args...)
			}
			args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}
			want := gitTest(t, root, args...)
			got, err := Status(context.Background(), root, metadataLimit, run, args...)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("private status=%q want=%q error=%v", got, want, err)
			}
			fast := kind == "sha1-v2" || kind == "sha1-v3" || kind == "cloned"
			if fast && calls != 1 {
				t.Fatalf("simple metadata used %d Git processes, want 1", calls)
			}
			if !fast && !probed {
				t.Fatal("unsupported fast-path metadata bypassed private Git probes")
			}
		})
	}
}

func TestSimpleIndexRejectsTruncationHostileFramingAndUnsupportedMetadata(t *testing.T) {
	root := fixture(t)
	data, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if !simpleIndex(data) {
		t.Fatal("ordinary Git index missed")
	}
	for end := 0; end < len(data); end++ {
		if simpleIndex(data[:end]) {
			t.Fatalf("truncated index admitted at %d", end)
		}
	}
	for _, kind := range []string{"entry-count", "gitlink", "entry-name-length", "extended-v2", "index-version-4", "extension-length", "extension-overrun-by-one", "split-index", "unknown-extension"} {
		t.Run(kind, func(t *testing.T) {
			changed := bytes.Clone(data[:len(data)-sha1.Size])
			switch kind {
			case "entry-count":
				binary.BigEndian.PutUint32(changed[8:12], ^uint32(0))
			case "index-version-4":
				// Fixed-length v2 entries otherwise parse cleanly; only the
				// explicit version guard must refuse this framing.
				binary.BigEndian.PutUint32(changed[4:8], 4)
			case "gitlink":
				binary.BigEndian.PutUint32(changed[12+24:12+28], 0o160000)
			case "entry-name-length":
				binary.BigEndian.PutUint16(changed[12+60:12+62], 0xfff)
			case "extended-v2":
				changed[12+60] |= 0x40
			case "extension-length":
				changed = append(changed, []byte("TREE\xff\xff\xff\xff")...)
			case "extension-overrun-by-one":
				changed = append(changed, []byte("TREE\x00\x00\x00\x01")...)
			case "split-index":
				changed = append(changed, []byte("link\x00\x00\x00\x00")...)
			case "unknown-extension":
				changed = append(changed, []byte("ZZZZ\x00\x00\x00\x00")...)
			}
			digest := sha1.Sum(changed)
			if simpleIndex(append(changed, digest[:]...)) {
				t.Fatal("unsupported index bypassed private Git validation")
			}
		})
	}
}
