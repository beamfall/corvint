# Friction lane w5: appmap, docs corpus, inventory and MCP tickets

Native tickets V1-3106, V1-3110 to V1-3117, V1-3140, V1-3150, V1-3161, V1-3162 and V1-3164;
immutable base `c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`.

## Already fixed on the base

Eleven of the fourteen tickets were already delivered in the 1.0 snapshot (`9477995d7`), each with a
named requirement and focused tests. This lane reran those tests on the base rather than reworking
the code:

| Ticket | GitHub | Requirements | Evidence |
| --- | --- | --- | --- |
| V1-3110 | #2004 | `AMAP-V0-423` | `TestAMAPV0423StaticInjectField`, `TestAMAPV0423StaticInjectRefusals` |
| V1-3111 | #2005 | `AMAP-V0-424` | `TestAMAPV0424ElementWiseArrayUses`, `TestAMAPV0424ElementWiseRefusals` |
| V1-3161 | #2020 | `AMAP-V0-425..427` | `TestAMAPV0425ParameterPatternsAndMethodConsts`, `TestAMAPV0427ReportedFormsStayRefused` |
| V1-3112 | #2006 | `DCI-V1-020` | `TestDCIV1020WeakSignalAtRealisticSize` |
| V1-3113 | #2007 | `DCI-V1-021..023` | `TestDCIV1021*`, `TestDCIV1022*`, `TestDCIV1023*` |
| V1-3114 | #2008 | `DCI-V1-024` | `TestDCIV1024*` |
| V1-3115 | #2009 | `DCI-V1-025` | `TestDCIV1025CheckoutRefusalKeepsGitauthReason` |
| V1-3162 | #2021 | `DCI-V1-027..030` | `TestDCIV1027*`, `TestDCIV1028*`, `TestDCIV1029*` |
| V1-3116 | #2010 | `MMR-V0-010`, `MMR-V0-011` | `TestMMRV0010*`, `TestMMRV0011*` |
| V1-3117 | #2011 | `MMR-V0-012..017` | `TestMMRV0012*` to `TestMMRV0017*` |
| V1-3140 | none | `GPK-V0-095` | `TestGPKV0095*` |

The GitHub issue bodies were not re-read here, because `gh` is not installed on this host. The
reproductions come from the ticket text, which quotes each issue's example.

## Fixed in this lane

- V1-3150, `GPK-V0-295`: `evalBlankCommentsAndStrings`, the `GPK-V0-093` admission mask, scanned a
  backtick to the next backtick. So in `` `${`leak()`}` `` the nested string became code, and a
  backslash-escaped backtick ended a web template early. Web files now use the `GPK-V0-095` reading
  (`evalWebTemplateEnd`). Go raw strings keep the flat reading. The regression leaked `leak` and
  `Component` on the base. `DR-0046` records that the admission mask now matches.
- V1-3164, `AMAP-V0-650`: the `AMAP-V0-384` plain-last-element exemption still applied to a
  literal spreading an enclosing parameter, such as `function go(i) { return [...i, 0] }`. A caller
  could pop the plain element, push a function and register the rebuilt list without any finding.
  Such a literal is now judged like any other array. A spread of a module const, such as
  `{ list: [...PIPES, 'x'] }` from `AMAP-V0-424`, keeps the exemption. Rejected alternative:
  dropping the exemption for every spread, which reblocked that accepted form.
- V1-3106, `DCI-V1-230`: the package encodes with `encoding/json/v2`, whose `omitempty` omits an
  encoded `[]`. A reflection audit of every `Manifest`-reachable `omitempty` field found 13 that drop
  an explicitly empty value. Nine were already refused at validation. `additional_repositories`,
  `over_bound_inputs`, `derive.paths` and `derive.kinds` were admitted and then dropped, so they are
  now refused. The suspected `production_scope` case was refuted: it is a pointer whose empty target
  encodes `{"include":[]}`, and an empty `include` is already refused.

## Limits

The `AMAP-V0-650` parameter reading is lexical. It treats any parenthesized list before a `{` as a
parameter list, which can only keep a literal blocked. It does not read a spread whose operand is a
call or a parenthesized expression.
