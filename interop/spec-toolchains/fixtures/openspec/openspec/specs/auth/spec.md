# auth Specification

## Purpose
Authenticate users and manage their sessions so that only signed-in users reach protected pages.

## Requirements
### Requirement: User Login
The system SHALL authenticate a user who submits a registered email address and the matching password.

#### Scenario: Valid credentials
- **WHEN** a user submits a registered email address and the matching password
- **THEN** the system signs the user in and opens the dashboard

### Requirement: Session Expiry
The system SHALL end a session after 30 days without activity.

#### Scenario: Stale session
- **WHEN** a session has had no activity for 30 days
- **THEN** the next request is redirected to the sign-in page

### Requirement: Remember Me
The system SHALL keep a user signed in across browser restarts when the user chooses to be remembered.

#### Scenario: Remembered sign-in
- **WHEN** a remembered user restarts the browser
- **THEN** the user is still signed in

### Requirement: Password Reset
The system SHALL email a single-use reset link to a registered address on request.

#### Scenario: Reset requested
- **WHEN** a user requests a reset for a registered email address
- **THEN** the system emails a link that works once
