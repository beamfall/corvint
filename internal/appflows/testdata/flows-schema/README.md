# Released flows-file schema versions

One pair per released `corvint-flows/N` (FSV-V1-002): `N.members` lists, sorted, every member path
the reader accepts for N (`a.b` for a nested object member, `a[].b` for a member of an array
element), and `N.golden.json` is a file of version N that uses every one of them.
`TestFSVV1002MemberListsAreFrozen` fails when the reader's members for a released N differ from
`N.members`. Never edit a released pair: a member change ships as a new N with a new pair.
