package gokernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type chunkWriter struct {
	output bytes.Buffer
	fail   bool
}

func (writer *chunkWriter) Write(chunk []byte) (int, error) {
	if writer.fail {
		return 0, errors.New("closed")
	}
	return writer.output.Write(chunk)
}

// TestAFPV0398WriteCanonicalJSONMatchesCanonicalJSON pins the streamed
// receipt bytes to CanonicalJSON plus one newline, including escapes split
// across writes at every offset.
func TestAFPV0398WriteCanonicalJSONMatchesCanonicalJSON(t *testing.T) {
	values := []any{
		map[string]any{"b": "line sep par", "a": `back\slash \\u2028 kept`, "<": "&>"},
		map[string]any{"tail": `ends with \`},
		map[string]any{"odd": "\\\u2028", "even": "\\u2028", "triple": "\\\\\u2029x"},
		map[string]any{"many": strings.Repeat("a\u2028b\\u2029", 40)},
		[]any{" ", ` `, "plain", 1.5, nil, true},
		map[string]any{},
	}
	for _, value := range values {
		want, err := CanonicalJSON(value)
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, '\n')
		var whole chunkWriter
		if err := WriteCanonicalJSON(&whole, value); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(whole.output.Bytes(), want) {
			t.Fatalf("streamed %q, want %q", whole.output.Bytes(), want)
		}
		// The encoder makes one Write; split the raw encoding at every offset
		// to prove a separator escape spanning writes is still restored.
		raw := rawEncoding(t, value)
		for cut := 0; cut <= len(raw); cut++ {
			var split chunkWriter
			restorer := &separatorWriter{output: &split}
			if _, err := restorer.Write(raw[:cut]); err != nil {
				t.Fatal(err)
			}
			if _, err := restorer.Write(raw[cut:]); err != nil {
				t.Fatal(err)
			}
			if err := restorer.flush(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(split.output.Bytes(), want) {
				t.Fatalf("split at %d: %q, want %q", cut, split.output.Bytes(), want)
			}
		}
	}
}

// rawEncoding is the encoder's output before separator restoration.
func rawEncoding(t *testing.T, value any) []byte {
	t.Helper()
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestAFPV0398WriteCanonicalJSONSeparatesOutputFailure(t *testing.T) {
	err := WriteCanonicalJSON(&chunkWriter{fail: true}, map[string]any{"a": 1})
	if !IsCanonicalOutputFailure(err) {
		t.Fatalf("output failure = %v, want an output failure", err)
	}
	var written chunkWriter
	err = WriteCanonicalJSON(&written, map[string]any{"a": func() {}})
	if err == nil || IsCanonicalOutputFailure(err) || written.output.Len() != 0 {
		t.Fatalf("encoding failure = %v after %d bytes, want a non-output error and no bytes", err, written.output.Len())
	}
	// A marshaler cannot pass its own error off as an output failure.
	err = WriteCanonicalJSON(&written, outputLookalike{})
	if err == nil || IsCanonicalOutputFailure(err) {
		t.Fatalf("marshaler error = %v, want a non-output error", err)
	}
}

type outputLookalike struct{}

func (outputLookalike) MarshalJSON() ([]byte, error) {
	return nil, errors.New("canonical JSON output failed")
}

// TestAFPV0398SeparatorFastPathClipsCapacity proves the unchanged-document
// fast path cannot let a caller's append write into the encoder's buffer.
func TestAFPV0398SeparatorFastPathClipsCapacity(t *testing.T) {
	backing := make([]byte, 3, 16)
	copy(backing, `"a"`)
	restored := restoreJSONSeparators(backing)
	if cap(restored) != len(restored) {
		t.Fatalf("fast path capacity %d, want %d", cap(restored), len(restored))
	}
}
