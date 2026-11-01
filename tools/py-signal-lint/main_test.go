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
