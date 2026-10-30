package fake

import "github.com/alphatechini/multi-plex/internal/protocol"

var specDoc = protocol.SpecDoc{
	Slug: "token-refresh",
	Requirements: `# Requirements: token refresh

### Requirement 1: Refresh before expiry

**User story:** As an API client, I want expired tokens refreshed automatically, so that requests do not fail mid-session.

**Acceptance criteria**
1. WHEN a request is made with a token that expires within 30 seconds, THE SYSTEM SHALL refresh the token before sending the request.
2. IF the refresh fails, THEN THE SYSTEM SHALL return an authentication error and SHALL NOT retry more than once.
3. THE SYSTEM SHALL NOT include a token in any log line.

### Requirement 2: Share refreshes between callers

**User story:** As an operator, I want concurrent requests for one user to share a single refresh, so that the identity provider is not flooded.

**Acceptance criteria**
1. WHEN several requests for the same user need a refresh at once, THE SYSTEM SHALL call the identity provider once.
2. THE SYSTEM SHALL keep at most one cached token per user.
3. WHEN a cached token is older than AUTH_CACHE_TTL, THE SYSTEM SHALL discard it.

### Requirement 3: Observability

**User story:** As an on-call engineer, I want refresh counts exported, so that I can see when refreshes spike.

**Acceptance criteria**
1. WHEN a refresh succeeds, THE SYSTEM SHALL increment the refresh counter.
2. WHEN a refresh fails, THE SYSTEM SHALL increment the failure counter.
3. THE SYSTEM SHALL expose both counters on the metrics endpoint.
`,
	Design: `# Design: token refresh

## Overview

` + "`Session.Refresh`" + ` asks a per-user ` + "`Cache`" + ` for a token. The cache returns a stored token that is still
valid for longer than ` + "`refreshWindow`" + `, and otherwise calls the token source once and stores the result.

## Components

- ` + "`auth.Cache`" + `: map of user to token with a read-write lock.
- ` + "`auth.Session`" + `: owns the current token and calls the cache.
- ` + "`metrics.Set`" + `: refresh and failure counters.

## Error handling

A failed fetch is returned wrapped with context (` + "`refresh token: %w`" + `) and is not cached.

## Testing

Table tests for the window boundary, a race test with eight callers, and a fake token source that counts calls.
`,
	Tasks: `# Tasks

- [x] 1. Add Cache with per-user entries
  _Requirements: R2.2_
- [x] 2. Move the expiry check into tokenValid
  _Requirements: R1.1_
- [x] 3. Wrap refresh errors with context
  _Requirements: R1.2_
- [~] 4. Update callers in api/ and worker/
  _Requirements: R1.1_
- [ ] 5. Share one refresh between concurrent callers
  _Requirements: R2.1_
- [ ] 6. Export refresh and failure counters
  _Requirements: R3.1, R3.2, R3.3_
- [ ] 7. Document AUTH_CACHE_TTL in docs/auth.md
  _Requirements: R2.3_
`,
}
