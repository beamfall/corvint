# Tasks source archive: offline rebuild proof

V1-0456 actual-archive regression PASS on frozen source
`66a2cd6f1f13b49b8ffb8400f37bee7846161701`, tree
`f4213e65c896e96a8064349d5ac75e96a4093dee`.

The release assembler produced a 428-file Tasks source tarball with SHA-256
`e6edadf42ed53fbc1ee4f6bee8249c53a0d21b8b430476df1f66ce0d975bffb1`.
The existing retained-bundle decoder extracted the actual tar bytes; verification
matched every extracted path to its source Git blob. Root license/provenance bytes
and immutable commit/tree identities were retained. The advertised command
`go build ./cmd/corvint-tasks` passed with isolated cold Go caches, GOWORK off,
GOPROXY off and GOSUMDB off. The negative control removed transitive package
`internal/diagnostic` from a second actual archive; the rebuild failed naming that
missing package. The combined regression passed in 18.90 seconds, without npm/VSIX.

Command: `go test -v -count=1 -timeout 30m ./internal/companionrelease -run '^TestCurrentTasksSourceArchiveBuildsOffline$'`.
Raw output: `/private/tmp/corvint-bugfix-20260928/v1-0456/archive-proof.log`.
Source stayed clean and unchanged throughout the command. This evidence entry is
after the frozen run and does not change its source identity. The baseline failure
and focused boundary/static results remain under the same private batch evidence.

Independent review, CEM/OCM/dogfood closure and integration of the prepared PR315
dependency remain pending. The full nine-companion release gate is NOT_RUN; this
proof neither qualifies the full bundle nor changes import-direction authority.
No queue state or ticket completion was written by the builder.
