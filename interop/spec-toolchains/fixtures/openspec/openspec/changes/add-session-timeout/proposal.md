# Change: Add an idle session timeout

## Why
Sessions stay open for 30 days, so an unattended browser exposes a signed-in account for far too long.

## What Changes
- Add a warning before an idle session expires.
- Shorten session expiry from 30 days to 30 minutes without activity.
- **BREAKING** Remove the remember-me option.
- Rename the User Login requirement to Credential Sign-In.

## Impact
- Affected specs: auth
