package intent

import "github.com/Beamfall/corvint/internal/tasks/wire"

// SafeReuse is an operator-declared local reset/verify procedure, not physical authority.
type SafeReuse struct {
	Reset, Verify                           PoolCommand
	EnvFile                                 string
	TimeoutSeconds, MaxAttempts, ExpectExit wire.Count
	ExpectStdout                            string
}

func readSafeReuse(r *wire.Reader, allowed []string) *SafeReuse {
	r.Closed(wire.OptionalKeys(r.Value(), []string{"argv", "envKeys", "verify", "timeoutSeconds", "maxAttempts"}, "env", "cwd")...)
	s := &SafeReuse{TimeoutSeconds: boundCount(r.Field("timeoutSeconds"), 1, 900), MaxAttempts: boundCount(r.Field("maxAttempts"), 1, 2)}
	// PSR-V0-012: an optional cwd applies to both reset and verify; omission
	// keeps REPOSITORY and the legacy bytes.
	cwd, pinned := CwdRepository, (*PinnedCwd)(nil)
	if wire.Has(r.Value(), "cwd") {
		cwd, pinned = readPoolCwd(r.Field("cwd"))
	}
	s.Reset = PoolCommand{Cwd: cwd, Pinned: pinned, TimeoutSeconds: s.TimeoutSeconds, Env: r.Field("envKeys").Strings(64, false, (*wire.Reader).Label)}
	for _, a := range r.Field("argv").Array(128, true) {
		s.Reset.Argv = append(s.Reset.Argv, a.Prose(1, 4096))
	}
	if len(s.Reset.Argv) == 0 {
		r.Fail(wire.CodeMalformed, "empty reset argv")
	}
	subsetOf(r, s.Reset.Env, allowed, "safe reuse environment")
	v := r.Field("verify")
	v.Closed("argv", "envKeys", "expectExit", "expectStdout")
	s.Verify = PoolCommand{Cwd: cwd, Pinned: pinned, TimeoutSeconds: s.TimeoutSeconds, Env: v.Field("envKeys").Strings(64, false, (*wire.Reader).Label)}
	for _, a := range v.Field("argv").Array(128, true) {
		s.Verify.Argv = append(s.Verify.Argv, a.Prose(1, 4096))
	}
	if len(s.Verify.Argv) == 0 {
		v.Fail(wire.CodeMalformed, "empty verify argv")
	}
	subsetOf(v, s.Verify.Env, s.Reset.Env, "verify environment")
	s.ExpectExit = boundCount(v.Field("expectExit"), 0, 255)
	s.ExpectStdout = v.Field("expectStdout").Prose(1, 4096)
	if wire.Has(r.Value(), "env") {
		x := r.Field("env").Prose(6, 4096)
		if len(x) < 6 || x[:5] != "file:" {
			r.Fail(wire.CodeMalformed, "safe reuse env requires file:PATH")
		} else {
			s.EnvFile = x[5:]
		}
	}
	return s
}
