// Package corpusserve implements an optional experimental JSON HTTP companion.
// This is not MCP Streamable HTTP. It has no repository, provider or write API.
package corpusserve

import (
	"context"
	json "encoding/json/v2"
	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"io"
	"net/http"
	"time"
)

const MaxConcurrent = 2

func Handler(reader *corpusindex.Reader) http.Handler {
	slots := make(chan struct{}, MaxConcurrent)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		refusal := func(status int, code string) {
			w.WriteHeader(status)
			_ = json.MarshalWrite(w, map[string]any{"profile": "corvint-corpus-http-refusal/1", "code": code, "state": "abstained", "trust_envelope": map[string]any{"content_status": "unknown", "source_revision": "unknown", "corpus_revision": reader.Digest(), "freshness": "unknown", "retirement": "unknown", "citations": []any{}, "limitations": []string{"request refused; no evidence answer"}}}, json.Deterministic(true))
		}
		if r.URL.Path != "/query" || r.URL.RawQuery != "" {
			refusal(404, "unknown-operation")
			return
		}
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			refusal(405, "read-query-post-required")
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			refusal(415, "json-required")
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			refusal(503, "concurrency-bound")
			return
		}
		input, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
		if e != nil {
			refusal(413, "request-bound")
			return
		}
		q, e := corpusindex.ParseRequest(input)
		if e != nil {
			refusal(400, "invalid-query")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		receipt, e := reader.Query(ctx, q)
		if e != nil {
			refusal(422, "query-refused")
			return
		}
		data, e := doccorpus.Encode(receipt)
		if e != nil {
			refusal(422, "receipt-bound")
			return
		}
		_, _ = w.Write(data)
	})
}
func NewServer(address string, r *corpusindex.Reader) *http.Server {
	return &http.Server{Addr: address, Handler: Handler(r), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
}
