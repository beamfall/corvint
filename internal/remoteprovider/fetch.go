// Package remoteprovider belongs exclusively to the optional remote adapter.
package remoteprovider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MaxRecordBytes = 1 << 20

var ErrRefused = errors.New("remote provider refused: configuration, transport or record boundary failed")

type Config struct {
	URL            string `json:"url"`
	SPKISHA256     string `json:"spkiSha256"`
	CredentialFile string `json:"credentialFile,omitempty"`
	CAFile         string `json:"caFile,omitempty"`
}

func LoadConfig(path string) (Config, error) {
	var config Config
	data, err := readRegular(path, 16384, false)
	if err != nil {
		return config, ErrRefused
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&config) != nil {
		return Config{}, ErrRefused
	}
	if _, err := decoder.Token(); err != io.EOF {
		return Config{}, ErrRefused
	}
	return config, nil
}

// Fetch returns complete response bytes only after all transport checks succeed.
func Fetch(ctx context.Context, config Config) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	endpoint, err := url.Parse(config.URL)
	if err != nil {
		return nil, ErrRefused
	}
	if endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" || endpoint.Opaque != "" {
		return nil, ErrRefused
	}
	pin, err := hex.DecodeString(config.SPKISHA256)
	if err != nil || len(pin) != sha256.Size {
		return nil, ErrRefused
	}
	var credential []byte
	if config.CredentialFile != "" {
		credential, err = readRegular(config.CredentialFile, 4096, true)
		if err != nil || len(credential) == 0 {
			return nil, ErrRefused
		}
		for _, b := range credential {
			if b < 0x21 || b > 0x7e {
				return nil, ErrRefused
			}
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if config.CAFile != "" {
		pem, err := readRegular(config.CAFile, 1<<20, false)
		if err != nil {
			return nil, ErrRefused
		}
		tlsConfig.RootCAs = x509.NewCertPool()
		if !tlsConfig.RootCAs.AppendCertsFromPEM(pem) {
			return nil, ErrRefused
		}
	}
	tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
		if len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
			return ErrRefused
		}
		digest := sha256.Sum256(state.PeerCertificates[0].RawSubjectPublicKeyInfo)
		if subtle.ConstantTimeCompare(pin, digest[:]) != 1 {
			return ErrRefused
		}
		return nil
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig, Proxy: nil, DisableCompression: true, DisableKeepAlives: true, MaxResponseHeaderBytes: 64 << 10, ForceAttemptHTTP2: false, TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrRefused }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, ErrRefused
	}
	if len(credential) != 0 {
		request.Header.Set("Authorization", "Bearer "+string(credential))
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, ErrRefused
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > MaxRecordBytes || response.Header.Get("Content-Encoding") != "" {
		return nil, ErrRefused
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxRecordBytes+1))
	if err != nil || len(data) > MaxRecordBytes || ctx.Err() != nil {
		return nil, ErrRefused
	}
	if len(credential) != 0 && reflectsCredential(data, credential) {
		return nil, ErrRefused
	}
	return data, nil
}

func reflectsCredential(data, credential []byte) bool {
	if bytes.Contains(data, credential) {
		return true
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return false
		}
		if err != nil {
			return true
		}
		if text, ok := token.(string); ok && strings.Contains(text, string(credential)) {
			return true
		}
	}
}
