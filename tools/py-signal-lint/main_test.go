package main

import (
	"bytes"
	"strings"
	"testing"
)

// ASD-V0-574: a registered handler or lambda that raises InterruptedError fails, naming the file
// and line; SystemExit and KeyboardInterrupt handlers, unregistered functions and comments pass.
func TestASDV0574LintsSignalHandlersForInterruptedError(t *testing.T) {
	bad := map[string][]byte{
		"a.py": []byte("import signal\n\ndef interrupted(signum, frame):\n    cleanup()\n\n    raise InterruptedError(signum)\n\nsignal.signal(signal.SIGTERM, interrupted)\n"),
		"b.py": []byte("import signal\nsignal.signal(signal.SIGINT, lambda *_: (_ for _ in ()).throw(InterruptedError()))\n"),
	}
	var stdout, stderr bytes.Buffer
	if rc := run(bad, &stdout, &stderr); rc != 1 {
		t.Fatalf("bad handlers: rc=%d stderr=%s", rc, stderr.String())
	}
	got := stderr.String()
	for _, want := range []string{"a.py:6: signal handler interrupted raises InterruptedError", "b.py:2: a signal handler lambda raises InterruptedError"} {
		if !strings.Contains(got, want) {
			t.Fatalf("findings %q lack %q", got, want)
		}
	}
	good := map[string][]byte{
		"c.py": []byte("import signal\n\ndef interrupted(signum, frame):\n    # raise InterruptedError would be retried\n    raise SystemExit(128 + signum)\n\ndef other():\n    raise InterruptedError('not a handler')\n\nprior = {sig: signal.signal(sig, interrupted) for sig in (signal.SIGINT, signal.SIGTERM)}\nsignal.signal(signal.SIGHUP, signal.SIG_IGN)\n"),
	}
	stdout.Reset()
	stderr.Reset()
	if rc := run(good, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "ok (1 Python files)") {
		t.Fatalf("good handlers: rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
}

// ASD-V0-574 (rescue-c review): a registration split across lines is read as one logical call
// through its closing parenthesis, so its handler's body is still linted and a lambda there still
// fails, named at the line of signal.signal(; text after the call is not part of it.
func TestASDV0574MultilineRegistration(t *testing.T) {
	bad := map[string][]byte{
		"d.py": []byte("import signal\n\ndef interrupted(signum, frame):\n    raise InterruptedError()\n\nsignal.signal(\n    signal.SIGTERM, interrupted\n)\n"),
		"e.py": []byte("import signal\nsignal.signal(\n    signal.SIGINT,\n    lambda *_: (_ for _ in ()).throw(InterruptedError()),\n)\n"),
		"g.py": []byte("import signal\n\ndef halt(signum, frame):\n    raise InterruptedError()\n\nsignal.signal(  # on SIGTERM (\n    signal.SIGTERM,\n    halt,\n)\n"),
	}
	var stdout, stderr bytes.Buffer
	if rc := run(bad, &stdout, &stderr); rc != 1 {
		t.Fatalf("multiline registrations: rc=%d stderr=%s", rc, stderr.String())
	}
	got := stderr.String()
	for _, want := range []string{"d.py:4: signal handler interrupted raises InterruptedError", "e.py:2: a signal handler lambda raises InterruptedError", "g.py:4: signal handler halt raises InterruptedError"} {
		if !strings.Contains(got, want) {
			t.Fatalf("findings %q lack %q", got, want)
		}
	}
	good := map[string][]byte{
		"f.py": []byte("import signal, sys\n\ndef other():\n    raise InterruptedError(')')\n\nsignal.signal(\n    signal.SIGINT,\n    lambda *_: sys.exit(')'),\n)\nnote = 'InterruptedError'\n"),
	}
	stdout.Reset()
	stderr.Reset()
	if rc := run(good, &stdout, &stderr); rc != 0 {
		t.Fatalf("multiline good registration: rc=%d stderr=%q", rc, stderr.String())
	}
}
