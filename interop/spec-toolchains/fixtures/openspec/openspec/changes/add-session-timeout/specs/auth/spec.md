## ADDED Requirements
### Requirement: Idle Timeout Warning
The system SHALL warn a signed-in user two minutes before an idle session expires.

#### Scenario: Warning shown
- **WHEN** a session is two minutes from its idle timeout
- **THEN** the page shows a warning with an option to stay signed in

## MODIFIED Requirements
### Requirement: Session Expiry
The system SHALL end a session after 30 minutes without activity.

#### Scenario: Stale session
- **WHEN** a session has had no activity for 30 minutes
- **THEN** the next request is redirected to the sign-in page

#### Scenario: Activity extends the session
- **WHEN** a user makes a request before the timeout
- **THEN** the 30-minute window restarts from that request

## REMOVED Requirements
### Requirement: Remember Me
**Reason**: Persistent sessions conflict with the idle timeout.
**Migration**: Users sign in again after a browser restart.

## RENAMED Requirements
- FROM: `### Requirement: User Login`
- TO: `### Requirement: Credential Sign-In`
