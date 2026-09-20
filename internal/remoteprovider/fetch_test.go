package remoteprovider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testConfig(t *testing.T, handler http.HandlerFunc) Config {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.Config.ErrorLog = nil
	server.StartTLS()
	t.Cleanup(server.Close)
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(server.Certificate().RawSubjectPublicKeyInfo)
	return Config{URL: server.URL, SPKISHA256: hex.EncodeToString(pin[:]), CAFile: ca}
}

func TestRemoteFailures(t *testing.T) {
	for _, mode := range []string{"oversize", "status", "redirect", "encoding", "truncated", "unavailable", "pin", "trust", "scheme", "userinfo", "query", "fragment", "credential-reflection", "credential-escaped", "credential-mode", "credential-symlink", "credential-whitespace", "timeout"} {
		t.Run("EEP-REMOTE-002 "+mode, func(t *testing.T) {
			credential := "provider-secret-DO-NOT-LEAK"
			config := testConfig(t, func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "oversize":
					_, _ = w.Write(bytes.Repeat([]byte("x"), MaxRecordBytes+1))
				case "status":
					w.WriteHeader(503)
				case "redirect":
					http.Redirect(w, r, "https://must-not-be-contacted.invalid", 302)
				case "encoding":
					w.Header().Set("Content-Encoding", "gzip")
					io.WriteString(w, "body")
				case "truncated":
					w.Header().Set("Content-Length", "100")
					io.WriteString(w, "short")
				case "credential-reflection":
					io.WriteString(w, credential)
				case "credential-escaped":
					io.WriteString(w, `{"summary":"\u0070rovider-secret-DO-NOT-LEAK"}`)
				case "timeout":
					<-r.Context().Done()
				default:
					io.WriteString(w, `{"record":"ok"}`)
				}
			})
			switch mode {
			case "unavailable":
				config.URL = "https://127.0.0.1:1"
			case "pin":
				config.SPKISHA256 = strings.Repeat("0", 64)
			case "trust":
				config.CAFile = ""
			case "scheme":
				config.URL = "http://127.0.0.1:1"
			case "userinfo":
				config.URL = strings.Replace(config.URL, "https://", "https://secret@", 1)
			case "query":
				config.URL += "?token=secret"
			case "fragment":
				config.URL += "#secret"
			}
			if strings.HasPrefix(mode, "credential-") {
				config.CredentialFile = filepath.Join(t.TempDir(), "credential")
				value := credential
				if mode == "credential-whitespace" {
					value += "\n"
				}
				if err := os.WriteFile(config.CredentialFile, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
				if mode == "credential-mode" {
					if err := os.Chmod(config.CredentialFile, 0644); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "credential-symlink" {
					original := config.CredentialFile
					config.CredentialFile += ".link"
					if err := os.Symlink(original, config.CredentialFile); err != nil {
						t.Fatal(err)
					}
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if mode == "timeout" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			defer cancel()
			data, err := Fetch(ctx, config)
			if err != ErrRefused || len(data) != 0 {
				t.Fatalf("failure published data: bytes=%d err=%v", len(data), err)
			}
			if strings.Contains(err.Error(), credential) {
				t.Fatal("credential in error")
			}
		})
	}
	t.Run("EEP-REMOTE-003 private bearer and exact bytes", func(t *testing.T) {
		body := []byte(" {\n\"record\":\"bytes\"}\n")
		config := testConfig(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer token" {
				w.WriteHeader(401)
				return
			}
			if r.Header.Get("Accept-Encoding") != "" {
				w.WriteHeader(400)
				return
			}
			_, _ = w.Write(body)
		})
		config.CredentialFile = filepath.Join(t.TempDir(), "credential")
		if err := os.WriteFile(config.CredentialFile, []byte("token"), 0600); err != nil {
			t.Fatal(err)
		}
		data, err := Fetch(context.Background(), config)
		if err != nil || !bytes.Equal(body, data) {
			t.Fatalf("bytes changed: %q %v", data, err)
		}
	})
}

func TestRemoteConfig(t *testing.T) {
	for _, body := range []string{`{}`, `{"url":"https://host","unknown":"x"}`, `{} {}`, `{"url":null}`} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		config, err := LoadConfig(path)
		if err == nil {
			_, err = Fetch(context.Background(), config)
		}
		if err != ErrRefused {
			t.Fatalf("admitted bad config %s", body)
		}
	}
	config := Config{URL: "https://example.invalid", SPKISHA256: strings.Repeat("0", 64)}
	body, _ := json.Marshal(config)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil || got != config {
		t.Fatalf("config round trip: %+v %v", got, err)
	}
}
