package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type shortWriter struct{ calls int }

type midReadError struct{ calls int }

func (w *shortWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.calls == 1 {
		return 1, nil
	}
	return 0, errors.New("closed")
}

func (r *midReadError) Read(data []byte) (int, error) {
	r.calls++
	if r.calls == 1 {
		return copy(data, "partial"), nil
	}
	return 0, errors.New("stdin failed")
}
func TestWriteAllRejectsPartialWrite(t *testing.T) {
	if err := writeAll(&shortWriter{}, []byte("shader")); err == nil {
		t.Fatal("partial write accepted")
	}
}
func TestWriteAllWritesWholeFrame(t *testing.T) {
	var out bytes.Buffer
	if err := writeAll(&out, []byte("{}\n")); err != nil || out.String() != "{}\n" {
		t.Fatalf("out=%q err=%v", out.String(), err)
	}
}

func TestVersionDeclaresCEMAndOCMUnsupported(t *testing.T) {
	want := []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"shader","status":"EXPERIMENTAL_UNSELECTABLE","cem":"UNSUPPORTED","ocm":"UNSUPPORTED"}` + "\n")
	if !bytes.Equal(versionFrame(), want) {
		t.Fatalf("version=%q", versionFrame())
	}
}

// TestRunReadFailureWritesNoncanonicalSentinel pins ACP-012: a stdin read
// failure is the NONCANONICAL_REQUEST sentinel with exit 0.
func TestRunReadFailureWritesNoncanonicalSentinel(t *testing.T) {
	var output bytes.Buffer
	if code := run(nil, &midReadError{}, &output); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if got, want := output.String(), `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}`+"\n"; got != want {
		t.Fatalf("output=%q want=%q", got, want)
	}
}

func TestRunReturnsNonzeroForEveryProtocolWriteFailure(t *testing.T) {
	for _, vector := range []struct {
		name  string
		args  []string
		input io.Reader
	}{
		{"version", []string{"--version"}, strings.NewReader("")},
		{"invalid-arguments", []string{"unexpected"}, strings.NewReader("")},
		{"analysis", nil, strings.NewReader("{}\n")},
		{"read-failure", nil, &midReadError{}},
	} {
		t.Run(vector.name, func(t *testing.T) {
			if code := run(vector.args, vector.input, &shortWriter{}); code != 1 {
				t.Fatalf("exit=%d", code)
			}
		})
	}
}
