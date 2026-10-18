# Feature Specification: Session Timeout

**Feature Branch**: `001-session-timeout`  
**Created**: 2026-10-17  
**Status**: Draft  
**Input**: User description: "Sign users out after a period of inactivity"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Idle sign-out (Priority: P1)

A signed-in user who leaves the browser unattended is signed out automatically.

**Why this priority**: An unattended session exposes the account to anyone at the device.

**Independent Test**: Leave a signed-in session idle for 30 minutes and make one request.

**Acceptance Scenarios**:

1. **Given** a signed-in user, **When** the session is idle for 30 minutes, **Then** the next request shows the sign-in page.

### Edge Cases

- A request that arrives exactly at the timeout ends the session.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST end a session after 30 minutes without activity.
- **FR-002**: System MUST warn the user two minutes before an idle session ends.
- **FR-003**: Users MUST be able to stay signed in from the warning.
- **FR-004**: System MUST retain idle sign-out records for [NEEDS CLARIFICATION: retention period not specified]

### Key Entities *(include if feature involves data)*

- **Session**: A signed-in user's authenticated state and its last activity time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 95% of idle sessions end within 31 minutes of the last activity.
- **SC-002**: Users who stay signed in from the warning lose no unsaved input.

## Assumptions

- Activity means any authenticated request from the user's browser.
