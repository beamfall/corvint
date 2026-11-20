# Flow link selection and execution obligations

V1-5695 retains the existing FLP-V1-007 refusal when the selected Playwright config cannot be
bound to the launch. The check resolves the declared command from the same instance, runner or
language entry the session uses, with its working directory. It admits direct `playwright test`
and `npx playwright test` with one matching config argument. An implicit config must be the sole
`playwright.config.*` entry in that directory and be the standard TypeScript or JavaScript file.
Providers, wrappers, extra options and ambiguous ownership retain the refusal.

A declaration without a discovery config has no config identity tying its reviewed intent links
to a canonical browser inventory. Its existing file join therefore contributes selections and
witnesses independently. Accepted canonical links contribute a witness for each selecting variation
and changed path (APK-V0-003). Status merges by variation, test key and project before counting
rows (AFU-V1-161).

For FLP-V1-011, CI records explicit whole-instance obligations as required checks with the plan's
reason. It resolves a declared language command when available. A local inventory fallback is
specific to the contract adapter, which the Go-only CI profile does not launch (STR-V0-012).
