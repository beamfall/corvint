# Citation runner executable lookup repair

Self-review finding P2 for V1-4851 identified a mismatch between file identity
and executable selection under `DCW-V0-8760`: statting a bare name inspected the
working directory, whereas the external runner resolved that name through PATH.
Different same-named executables could therefore mix citation implementations.

The admission class is executable identity after successful standard `os/exec`
lookup. Its closed allowlist requires successful `exec.LookPath`, successful
stat of both resolved selection and self, and `os.SameFile`. Any lookup error,
including refused implicit current-directory resolution, retains the existing
external runner without translating its output or status. No special list of
bad names or PATH forms governs admission.

`TestDogfoodInProcessCitationsResolvePATH` compares exit status, stdout and stderr
with the external runner for different same-named cwd/PATH executables, a missing
PATH selection, and an unproven implicit current-directory selection. Existing
same-file parity and missing-selection tests remain unchanged. These shell fixture
regressions run on POSIX; Windows runtime coverage remains unobserved in this lane.
Commands, outcomes and binding limits are retained in the external lane handback.

No wire, diagnostics, goldens, citation authority or public interface changes.
Rollback of the optimization remains the existing spec's external-runner fallback;
do not restore raw-name stat admission. V1-4852 is unchanged by this repair.
