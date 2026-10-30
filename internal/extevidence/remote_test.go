package extevidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/remoteprovider"
)

func buildRemoteProvider(t *testing.T) string {
	t.Helper()
	compiler, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "corvint-remote-provider")
	result := procgroup.Run(context.Background(), procgroup.Spec{Argv: []string{compiler, "build", "-o", binary, "./cmd/corvint-remote-provider"}, Dir: root, Env: os.Environ(), Timeout: time.Minute, OutputLimit: 1 << 20})
	if result.Err != nil || result.ExitStatus != 0 || !result.OwnedProcessGroupCleanup {
		t.Fatalf("build adapter: %v %s", result.Err, result.Stderr)
	}
	return binary
}

func TestRemoteTransportConformance(t *testing.T) {
	t.Parallel()
	t.Run("EEP-REMOTE-004 unchanged records", func(t *testing.T) {
		binary := buildRemoteProvider(t)
		var mu sync.Mutex
		files := map[string]string{}
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			path := files[r.URL.Path]
			mu.Unlock()
			data, err := os.ReadFile(path)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		}))
		defer server.Close()
		ca := writeRecord(t, t.TempDir(), "ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
		pin := sha256.Sum256(server.Certificate().RawSubjectPublicKeyInfo)
		transportConformance(t, func(file string) string {
			mu.Lock()
			endpoint := "/" + strconv.Itoa(len(files))
			files[endpoint] = file
			mu.Unlock()
			config, _ := json.Marshal(remoteprovider.Config{URL: server.URL + endpoint, SPKISHA256: hex.EncodeToString(pin[:]), CAFile: ca})
			path := writeRecord(t, t.TempDir(), "config.json", config)
			argv, _ := json.Marshal([]string{binary, "--allow-network", "--config", path})
			source, err := ParseCommand(string(argv))
			if err != nil {
				t.Fatal(err)
			}
			return source
		})
	})
}
