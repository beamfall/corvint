# Postmerge YAML error ownership correction

Required hosted doc-gates run 36966117813, job 110710135152, failed at
93fce5172b020a2720dd55e9f962e41d982cb8c4: `unsupported-yaml`, emitted by
`internal/postmergehost/yaml.go`, was absent from the owning specs. The local
`make error-code-ownership-check` reproduced that failure before this correction.
The proposed PCH contract now names the parser's existing refusal in its failure
modes. No parser behavior, requirement ID, acceptance status or allowlist changes.

The same enrolled PR458 work continues through its known original session key under
DOGFOOD's handoff contract, preserving the original base and frozen check plan.
A distinct enrollment refused `worktree-prior-completion-stale` at the seal; that
refusal and the prior satisfied binding/seal evidence remain retained. The supported
handoff re-resolved without drift before an ordinary rename-only seal revert.
No enrollment cancellation or private state edits were used.

The original guard repair's checks and independent review bind
5ba0f90edfdc4dfd23e9b1318aac6b213c24a999 and remain historical evidence for unchanged
source. The frozen selected checks must run again on this correction's final binding;
the ownership check supplies additional direct evidence. Current results and immutable
bindings are retained with the CEM and private completion reports, not inferred from
prior green results. Hosted integration and native bug V1-0666 completion remain pending.

Corvint query, affected, handoff and CEM/OCM/frontier routes are used for context,
selection and structural evidence. Original pre-authoring chronology is unavailable;
resumed context remains stale against the original base. Nine connector obligations
remain unassessed. PCH remains proposed and experimental; hosted replay and runtime
credential isolation remain unqualified. The exhaustive gate is not run under the
owner's scoped verification policy. No additional feature routes are applicable to
this narrow documentation correction. Rollback removes this error-name clarification
and retains the observed CI failure without changing parser behavior.
