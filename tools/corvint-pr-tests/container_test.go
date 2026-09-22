package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContainerProfileAndArchive(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		mounts := map[string]string{"/input": "/snapshot", "/trusted": "/tools", "/profile": "/profile"}
		args, err := containerCreateArgs(mounts)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(args, " ")
		for _, want := range []string{containerImage, "--pull=never", "--network=none", "--log-driver=none", "--read-only", "--cpus=2", "--memory=8589934592", "--memory-swap=8589934592", "readonly"} {
			if !strings.Contains(joined, want) {
				t.Fatal(want)
			}
		}
		if _, err = mountArg("/path,with-option", "/input"); err == nil {
			t.Fatal("mount option injection")
		}
		var v containerInspection
		v.ID = strings.Repeat("a", 64)
		v.Image = containerConfig
		v.Config.Image = containerImage
		v.Config.User = "65532:65532"
		v.Config.Entrypoint = []string{"/bin/sleep"}
		v.Config.Cmd = []string{"infinity"}
		h := &v.HostConfig
		h.LogConfig.Type = "none"
		h.NetworkMode = "none"
		h.ReadonlyRootfs = true
		h.NanoCpus = 2_000_000_000
		h.Memory = memoryBytes
		h.MemorySwap = memoryBytes
		h.ShmSize = 1 << 20
		h.CapDrop = []string{"ALL"}
		h.SecurityOpt = []string{"no-new-privileges"}
		h.Tmpfs = map[string]string{"/work": workTmpfs}
		// Use JSON to construct the anonymous mount fields without duplicating their type.
		b, _ := json.Marshal(v)
		var raw map[string]any
		_ = json.Unmarshal(b, &raw)
		ms := []map[string]any{}
		for dest, source := range mounts {
			ms = append(ms, map[string]any{"Type": "bind", "Source": source, "Destination": dest, "RW": false})
		}
		raw["Mounts"] = ms
		encode := func() []byte { b, _ := json.Marshal([]any{raw}); return b }
		if err = inspectProfile(encode(), mounts); err != nil {
			t.Fatal(err)
		}

		security := raw["HostConfig"].(map[string]any)
		config := raw["Config"].(map[string]any)
		for _, imageID := range []string{containerConfig, containerManifest} {
			raw["Image"] = imageID
			for _, option := range []string{"no-new-privileges", "no-new-privileges:true"} {
				security["SecurityOpt"] = []string{option}
				if err = inspectProfile(encode(), mounts); err != nil {
					t.Fatalf("observed identity %s/%s: %v", imageID, option, err)
				}
			}
		}
		for _, options := range [][]string{nil, {}, {"no-new-privileges:false"}, {"no-new-privileges:true", "seccomp=unconfined"}, {"no-new-privileges", "no-new-privileges"}} {
			security["SecurityOpt"] = options
			if err = inspectProfile(encode(), mounts); err == nil {
				t.Fatalf("security policy admitted: %v", options)
			}
		}
		security["SecurityOpt"] = []string{"no-new-privileges:true"}
		for _, reference := range []string{"golang:1.27.0", "docker.io/library/golang@sha256:" + strings.Repeat("0", 64)} {
			config["Image"] = reference
			if err = inspectProfile(encode(), mounts); err == nil {
				t.Fatalf("mutable/wrong reference admitted: %s", reference)
			}
		}
		config["Image"] = containerImage
		security["Tmpfs"] = map[string]string{"/work": strings.Replace(workTmpfs, ",exec", ",noexec", 1)}
		if err = inspectProfile(encode(), mounts); err == nil {
			t.Fatal("changed tmpfs policy admitted")
		}
		security["Tmpfs"] = map[string]string{"/work": workTmpfs}
		ms[0]["RW"] = true
		if err = inspectProfile(encode(), mounts); err == nil {
			t.Fatal("writable source admitted")
		}
		ms[0]["RW"] = false
		raw["Image"] = "sha256:" + strings.Repeat("0", 64)
		if err = inspectProfile(encode(), mounts); err == nil {
			t.Fatal("wrong image admitted")
		}
		for _, tc := range []struct {
			name  string
			kind  byte
			size  int64
			extra bool
			ok    bool
		}{{"go.json", tar.TypeReg, 3, false, true}, {"../go.json", tar.TypeReg, 3, false, false}, {"go.json", tar.TypeSymlink, 0, false, false}, {"go.json", tar.TypeReg, 4, false, false}, {"go.json", tar.TypeReg, 3, true, false}} {
			var b bytes.Buffer
			w := tar.NewWriter(&b)
			_ = w.WriteHeader(&tar.Header{Name: tc.name, Typeflag: tc.kind, Size: tc.size, Mode: 0600})
			if tc.size > 0 {
				_, _ = w.Write([]byte("data")[:tc.size])
			}
			if tc.extra {
				_ = w.WriteHeader(&tar.Header{Name: "extra", Mode: 0600, Size: 0, Typeflag: tar.TypeReg})
			}
			_ = w.Close()
			var dst bytes.Buffer
			err := copyRegularTar(&dst, &b, "go.json", 3)
			if (err == nil) != tc.ok {
				t.Fatalf("archive %+v: %v", tc, err)
			}
		}
	})
}
func TestColdRuntime(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		o := options{out: t.TempDir()}
		if err := prepareCache(o); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"home", "tmp", "cache"} {
			if err := os.WriteFile(filepath.Join(runtimePath(o), name, "old"), []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := resetPrivateRuntime(o); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"home", "tmp", "cache"} {
			entries, err := os.ReadDir(filepath.Join(runtimePath(o), name))
			if err != nil || len(entries) != 0 {
				t.Fatalf("%s not empty: %v", name, err)
			}
		}
	})
}

func TestFrozenRowIndex(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		for _, n := range []int{1, 27, 200} {
			got, err := shadowIndices(options{row: n})
			if err != nil || !equal(got, []int{n - 1}) {
				t.Fatalf("row %d: %v %v", n, got, err)
			}
		}
		for _, o := range []options{{row: -1}, {row: 201}, {row: 1, rowsSet: true}, {rows: 201}} {
			if _, err := shadowIndices(o); err == nil {
				t.Fatalf("admitted %+v", o)
			}
		}
	})
}

func TestBoundedBufferCopy(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		b := &boundedBuffer{limit: 8}
		_, err := io.Copy(b, struct{ io.Reader }{strings.NewReader(strings.Repeat("x", 9))})
		if err == nil || b.Len() > 8 {
			t.Fatalf("io.Copy bypass: %d %v", b.Len(), err)
		}
	})
}
func TestContainerCheckoutAndExport(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		if shadowCheckout(options{profile: "/profile/profile.json", out: "/work/out"}) != containerCheckout || containerCheckout != "/work/checkout" {
			t.Fatal("container source path differs")
		}
		for _, name := range exportFiles("run") {
			if name == "expected.json" || name == "row.json" {
				t.Fatal("run requires historical-only file")
			}
		}
		if !equal(exportFiles("freeze"), []string{"corpus.json"}) || !equal(exportFiles("qualify"), []string{"qualification.json"}) {
			t.Fatal("wrong terminal manifest")
		}
	})
}

func TestContainerTmpfsExecution(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		args, err := containerCreateArgs(nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, arg := range args {
			if strings.HasPrefix(arg, "--tmpfs=/work:") {
				found = true
				opts := strings.Split(strings.TrimPrefix(arg, "--tmpfs=/work:"), ",")
				execCount := 0
				for _, opt := range opts {
					if opt == "exec" {
						execCount++
					}
					if opt == "noexec" || opt == "ro" {
						t.Fatalf("non-executable/read-only work mount: %s", arg)
					}
				}
				if execCount != 1 {
					t.Fatalf("explicit exec missing/duplicated: %s", arg)
				}
			}
		}
		if !found {
			t.Fatal("work tmpfs missing")
		}
	})
}
