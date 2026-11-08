# Tracing dependency repair (V1-5106)

Date: 2026-11-07. Intent: RSP-V0-007, RSP-V0-021, RSP-V0-022.

The independent findings identified three unsound dependency classes: unchecked
syscall effects, ambiguous failure operands, and collapsed symlink chains. Repair
retains the existing plain result contract and incompleteness reasons. No wire,
command, golden, assertion, or runner qualification changes.

## Decisions and closed forms

The seccomp bypass is now an explicit named allowlist: close/lseek/dup/dup2;
mprotect/munmap/brk/mremap; rt_sigaction/rt_sigprocmask/rt_sigreturn/sigaltstack;
pause/nanosleep/sched_yield; getpid/gettid/getuid/getgid/geteuid/getegid/getppid/
getpgrp/getgroups/getresuid/getresgid; gettimeofday/time/getrlimit; exit/exit_group/
wait4. These operations address local memory, descriptor lifetime, local signal
setup, identity, clocks, resource queries or process lifetime. All other numbers
trap. The audit removed readiness/descriptor control, process-targeted scheduling,
credential changes, timer operations, system clock changes, and unproven multiplexed
operations, as well as utimensat and rt_sigqueueinfo. Unsupported effects preserve
execution but mark all trace components incomplete.

Trapped startup forms are separately bounded: arch_prctl's four segment-base
operations (0x1001..0x1004), fixed-pointer set_tid_address, 24-byte set_robust_list,
self-only read-only prlimit64, getrandom's three known flag bits, and 32-byte rseq
register/unregister. Unknown selectors/layouts fail closed using the existing
unsupported-syscall reason. The fixture retains complete single-process collection.

Failed path operations retain every operand. ENOENT/ENOTDIR also retain each
repository ancestor: a sibling addition can create a missing destination directory.
Other errors retain both operands as reads. This deliberately overselects instead
of guessing which operand the errno describes; there is no errno-to-operand list.

Symlink discovery walks ordinary absolute filesystem paths component by component,
expands relative/absolute textual links before continuing, and records each link
identity. Directories and regular files are the supported terminal forms. Resolution
errors, cycles/over-40-hop walks, procfs magic links, nonregular terminal objects and
nonabsolute inputs are unproven. Missing tails remain failed-lookup evidence. The
existing lexical/resolved final-file observation is retained. Rollback is a revert
of this repair; it needs no data migration and would restore the reported gaps.

## Evidence and limits

`TestRSPV0022ClosedSyscallForms` covers utimensat success versus deletion, signal-zero
rt_sigqueueinfo to the parent (no signal delivered), a future syscall number, and
unknown selectors/layouts of the admitted startup forms.
`TestRSPV0021RenameDestinationParent` covers rename, renameat and renameat2 failing
on missing/out, then succeeding after missing/anything creates that directory.
`TestRSPV0007ChainedSymlink` retargets only bridge in alias -> bridge -> a/b and
checks both observed byte exit codes and retained links with reads still present.
`TestRSPV0007UnprovenLinkForms` covers cycles, special files, procfs and relative
inputs; the existing complete-path and unsupported-matching assertions remain.

All existing and new tests in the only touched Go package passed through
`script/heavy.sh GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/proctrace/`:
3.048s package time on this shared host, not a performance or qualification claim.
Focused vet also passed. Raw test evidence remains outside the repository at
`/home/russell/lanes/rspt4-evidence/repair-tests-final.log`, SHA-256
`b6e5c212e54e404daf1fe425bb9b4e82312ba3f942c2d22d4a5c7d2d57cc4b87`.

Self-development routes: affected selection used the owner-specified
8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734 base (repair-affected.json outside the repo);
prechange context is reused from the original round; final CEM/report route follows
this repair. Whole-repository checks, model-backed review, sealing, publication and
Tasks writes are NOT_RUN by explicit repair-round instruction. Host review and
full-corpus traced/untraced qualification still gate V1-5106 acceptance. Retrieval,
learning, integration adapters and external providers are not touched or invoked.
