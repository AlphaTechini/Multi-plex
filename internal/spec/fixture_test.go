package spec

// requirementsMD follows the example in docs/09-specs.md, plus a second requirement.
const requirementsMD = `# Auth refresh

## Requirements

### Requirement 1: Token refresh

**User story:** As an API client, I want expired tokens refreshed automatically, so that requests do not fail mid-session.

**Acceptance criteria**
1. WHEN a request is made with a token that expires within 60 seconds, THE SYSTEM SHALL refresh the token before sending the request.
2. IF the refresh fails, THEN THE SYSTEM SHALL return an authentication error and SHALL NOT retry more than once.

### Requirement 2: Logging

**User story:** As an operator, I want refresh events logged, so that I can audit them.

**Acceptance criteria**
1. WHEN a refresh succeeds, THE SYSTEM SHALL log the token family and expiry time
   without logging the token itself.
`

// tasksMD has nested tasks, requirement citations, every checkbox style and an unnumbered task.
// Line numbers: 1.1 is line 4, 1.2 is line 6, 2 is line 8, 2.2 is line 11, 3 is line 14.
const tasksMD = `# Implementation plan

- [x] 1. Token store
  - [x] 1.1 Add store interface
    - _Requirements: R1.1_
  - [ ] 1.2 Persist expiry times
    - _Requirements: R1.1, R1.2_
- [-] 2. Refresh logic
  - [X] 2.1 Refresh before expiry
    - _Requirements: R1.1_
  - [ ] 2.2 Retry once
    - _Requirements: R1.2, R1.2_
    - Details: not a requirement
* [ ] 3. Docs
  - _Requirements: R2.1_
`
