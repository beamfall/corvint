# Decision 0126 — Legacy regular-file tree modes read as Git's canonical mode

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

Git still reads trees whose regular-file entries carry a legacy mode such as `100664`, and it
normalizes each one when reading: `100755` when the owner-execute bit is set, `100644` otherwise.
`git mktree` stores `100664` verbatim; `ls-tree`, `cat-file -p`, and `diff` all report `100644`, and a
`100644` to `100664` rewrite of the same blob diffs to zero bytes (Git 2.50.1, scratch fixture).

`CEM-CB-023`/`CEM-CB-024` compared raw verified tree modes against Git's normalized view. A
genuine, correctly hashed tree with a `100664` entry therefore failed literal-path lookup, and a
mode-only legacy rewrite failed canonical derivation, both as `repository-object-unavailable`,
which claims object corruption that is not there.

The owner call: a verified raw regular-file mode (`100` plus three octal digits) is taken in Git's
canonical form before comparison and before it is returned. Every other raw mode is compared as
stored, so a mode Git lists differently still fails closed. The tree body is still re-hashed, so
no unverified byte is trusted. The CEM wire and patch grammar are unchanged: they already see only
`100644`/`100755`, because Git's listing and diff normalize. `interop/cem01-go` reads modes through
`ls-tree`, so it already treats `100664` as `100644` and needs no change.

Rollback: revert `canonicalMode` in `internal/cem/gitauth/object.go`, the `CEM-CB-023`/`024`
amendment, and `CEM-CB-GIT-008`; legacy-mode trees then fail closed again.
