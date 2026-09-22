//go:build darwin

package authoritystore

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestDarwinReciprocalSocketWitness(t *testing.T) {
	// No subprocess or durable service: every listener/connection is owned by
	// this test process and closed on every return, failure, and process exit.
	path := filepath.Join(t.TempDir(), "w.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	pid := uint32(os.Getpid())
	before, err := inspectProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifySocketPair(context.Background(), pid, pid, path); err != nil {
		t.Fatal("live reciprocal relationship missing", err)
	}
	after, err := inspectProcess(pid)
	if err != nil || before != after {
		t.Fatal("process identity changed")
	}
	if verifySocketPair(context.Background(), pid, pid, path+"-foreign") == nil {
		t.Fatal("foreign endpoint admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if verifySocketPair(ctx, pid, pid, path) == nil {
		t.Fatal("cancelled witness admitted")
	}
	client.Close()
	if verifySocketPair(context.Background(), pid, pid, path) == nil {
		t.Fatal("closed peer admitted")
	}
}

func TestDarwinSharedQualificationRefusesMissingAndDuplicateSurfaces(t *testing.T) {
	q := &HostQualification{ControlSocket: "/tmp/control.sock", QualifiedSurfaces: []SurfaceQualification{{Surface: "codex-desktop", EvidenceSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}
	if validSharedQualification(q) {
		t.Fatal("Desktop-only shared runtime admitted")
	}
	q.QualifiedSurfaces = append(q.QualifiedSurfaces, SurfaceQualification{Surface: "codex-cli", EvidenceSHA256: q.QualifiedSurfaces[0].EvidenceSHA256})
	if !validSharedQualification(q) {
		t.Fatal("complete bounded surfaces rejected")
	}
	q.QualifiedSurfaces = append(q.QualifiedSurfaces, q.QualifiedSurfaces[0])
	if validSharedQualification(q) {
		t.Fatal("duplicate surface admitted")
	}
	q.QualifiedSurfaces = q.QualifiedSurfaces[:1]
	q.QualifiedSurfaces[0].Surface = "unknown"
	if validSharedQualification(q) {
		t.Fatal("unknown surface admitted")
	}
}
