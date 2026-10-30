package fake

import "strings"

// sessionGoHead is lines 1-37 of src/auth/session.go and sessionGoGap the 42
// unchanged lines between its two hunks (old 51-92, new 53-94).
const sessionGoHead = `package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"example.com/app/internal/metrics"
)

// refreshWindow is how long before expiry a token is renewed.
const refreshWindow = 30 * time.Second

// TokenSource fetches a fresh token from the identity provider.
type TokenSource interface {
	Token(ctx context.Context) (Token, error)
	Close() error
}

// Session holds the credentials of one signed-in user.
type Session struct {
	mu      sync.Mutex
	userID  string
	client  TokenSource
	cache   *Cache
	metrics *metrics.Set
	token   string
	expiry  time.Time
}

// NewSession returns a session that renews its token through client.
func NewSession(userID string, client TokenSource, m *metrics.Set) *Session {
	return &Session{userID: userID, client: client, metrics: m}
}

// Refresh renews the token when it is close to expiring.`

const sessionGoGap = `
// Token returns the current token without refreshing it.
func (s *Session) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

// Expiry returns when the current token stops being valid.
func (s *Session) Expiry() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.expiry
}

// UserID returns the user the session belongs to.
func (s *Session) UserID() string { return s.userID }

// String implements fmt.Stringer without exposing the token.
func (s *Session) String() string { return "session(" + s.userID + ")" }

// Invalidate drops the token so the next Refresh fetches a new one.
func (s *Session) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token, s.expiry = "", time.Time{}
}

// HasToken reports whether the session currently holds a token.
// It does not check the expiry; use tokenValid for that.
func (s *Session) HasToken() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token != ""
}

// Cache returns the token cache the session refreshes through.
// It is nil for sessions created before the cache existed.
func (s *Session) Cache() *Cache { return s.cache }

// Close releases the token source.
func (s *Session) Close() error {`

const cacheGo = `package auth

import (
	"context"
	"sync"
	"time"
)

// cacheTTL matches the identity provider's default token lifetime.
var cacheTTL = 5 * time.Minute

type entry struct {
	token   string
	expires time.Time
}

// Cache keeps one token per user so concurrent requests share a refresh.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]entry
	now     func() time.Time
}

// NewCache returns an empty cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]entry), now: time.Now}
}

// Get returns the cached token for user, fetching one with fetch when the
// cached token is missing or about to expire.
func (c *Cache) Get(ctx context.Context, user string, fetch func(context.Context) (Token, error)) (Token, error) {
	c.mu.RLock()
	e, ok := c.entries[user]
	c.mu.RUnlock()
	if ok && e.expires.Sub(c.now()) > refreshWindow {
		return Token{Value: e.token, Expiry: e.expires}, nil
	}
	tok, err := fetch(ctx)
	if err != nil {
		return Token{}, err
	}
	c.mu.Lock()
	c.entries[user] = entry{token: tok.Value, expires: tok.Expiry}
	c.mu.Unlock()
	return tok, nil
}`

var sessionGoFile = fileDiff{
	path: "src/auth/session.go", status: "M",
	hunks: []hunk{
		{oldStart: 38, newStart: 38, section: "func (s *Session) Refresh(ctx context.Context) error", lines: join(
			prefixed(" ", "func (s *Session) Refresh(ctx context.Context) error {", "\ts.mu.Lock()", "\tdefer s.mu.Unlock()"),
			prefixed("-", "\tif time.Until(s.expiry) > refreshWindow {"),
			prefixed("+", "\tif s.tokenValid(time.Now()) {"),
			prefixed(" ", "\t\treturn nil", "\t}"),
			prefixed("-", "\ttok, err := s.client.Token(ctx)"),
			prefixed("+", "\ttok, err := s.cache.Get(ctx, s.userID, s.client.Token)"),
			prefixed(" ", "\tif err != nil {"),
			prefixed("-", "\t\treturn err"),
			prefixed("+", "\t\treturn fmt.Errorf(\"refresh token: %w\", err)"),
			prefixed(" ", "\t}"),
			prefixed("+", "\t// Count successful refreshes for the metrics endpoint.", "\ts.metrics.Refreshes.Inc()"),
			prefixed(" ", "\ts.token, s.expiry = tok.Value, tok.Expiry", "\treturn nil", "}"),
		)},
		{oldStart: 93, newStart: 95, section: "func (s *Session) Close() error", lines: join(
			prefixed(" ", "\treturn s.client.Close()", "}", ""),
			prefixed("+",
				"// tokenValid reports whether the token outlives the refresh window.",
				"func (s *Session) tokenValid(now time.Time) bool {",
				"\treturn s.token != \"\" && s.expiry.Sub(now) > refreshWindow",
				"}"),
			prefixed(" ", ""),
		)},
	},
}

var cacheGoFile = fileDiff{
	path: "src/auth/cache.go", status: "A",
	hunks: []hunk{{newStart: 1, lines: prefixed("+", strings.Split(cacheGo, "\n")...)}},
}

var sessionTestFile = fileDiff{
	path: "src/auth/session_test.go", status: "M",
	hunks: []hunk{
		{oldStart: 21, newStart: 21, section: "func TestRefreshKeepsFreshToken(t *testing.T)", lines: join(
			prefixed(" ", "\ts := NewSession(\"u1\", src, nil)", "\tif err := s.Refresh(ctx); err != nil {", "\t\tt.Fatal(err)", "\t}"),
			prefixed("-", "\tif src.calls != 1 {", "\t\tt.Fatalf(\"calls = %d, want 1\", src.calls)"),
			prefixed("+", "\tif got := src.calls.Load(); got != 1 {", "\t\tt.Fatalf(\"calls = %d, want 1\", got)"),
			prefixed(" ", "\t}", "}"),
		)},
		{oldStart: 30, newStart: 30, section: "func TestRefreshKeepsFreshToken(t *testing.T)", lines: join(
			prefixed(" ", "", "func TestRefreshRenewsExpiringToken(t *testing.T) {"),
			prefixed("+",
				"\tctx := context.Background()",
				"\tsrc := &fakeSource{ttl: 10 * time.Second}",
				"\ts := NewSession(\"u1\", src, nil)",
				"\ts.cache = NewCache()",
				"\tfor range 3 {",
				"\t\tif err := s.Refresh(ctx); err != nil {",
				"\t\t\tt.Fatalf(\"refresh: %v\", err)",
				"\t\t}",
				"\t}",
				"\t// ttl is inside refreshWindow, so every call must hit the source.",
				"\tif got := src.calls.Load(); got != 3 {",
				"\t\tt.Fatalf(\"calls = %d, want 3\", got)",
				"\t}",
				"}",
				"",
				"func TestCacheSharesRefreshBetweenCallers(t *testing.T) {",
				"\tctx := context.Background()",
				"\tsrc := &fakeSource{ttl: time.Hour}",
				"\tc := NewCache()",
				"\tvar wg sync.WaitGroup",
				"\tfor range 8 {",
				"\t\twg.Add(1)",
				"\t\tgo func() {",
				"\t\t\tdefer wg.Done()",
				"\t\t\tif _, err := c.Get(ctx, \"u1\", src.Token); err != nil {",
				"\t\t\t\tt.Errorf(\"get: %v\", err)",
				"\t\t\t}",
				"\t\t}()",
				"\t}",
				"\twg.Wait()",
				"\tif got := src.calls.Load(); got > 2 {",
				"\t\tt.Fatalf(\"source called %d times for 8 callers\", got)",
				"\t}",
				"}",
				""),
			prefixed(" ", "type fakeSource struct {"),
			prefixed("-", "\tcalls int"),
			prefixed("+", "\tcalls atomic.Int32"),
			prefixed(" ", "\tttl   time.Duration", "}"),
		)},
	},
}

var tokenGoFile = fileDiff{
	path: "src/auth/token.go", oldPath: "src/auth/tokens.go", status: "R",
	hunks: []hunk{{oldStart: 4, newStart: 4, section: "type Token struct", lines: join(
		prefixed(" ", "// Token is a bearer token and the time it stops being valid.", "type Token struct {"),
		prefixed("-", "\tValue  string", "\tExpires time.Time", "\tScope  string"),
		prefixed("+", "\tValue  string", "\tExpiry time.Time", "\tScope  string"),
		prefixed(" ", "}"),
	)}},
}

var clientGoFile = fileDiff{
	path: "src/api/client.go", status: "M",
	hunks: []hunk{{oldStart: 52, newStart: 52, section: "func (c *Client) Do(ctx context.Context, req *Request) (*Response, error)", lines: join(
		prefixed(" ", "func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {"),
		prefixed("-", "\ttok, err := c.session.Refresh(ctx)", "\tif err != nil {"),
		prefixed("+", "\tif err := c.session.Refresh(ctx); err != nil {"),
		prefixed(" ", "\t\treturn nil, err", "\t}"),
		prefixed("+", "\treq.Header.Set(\"Authorization\", \"Bearer \"+c.session.Token())"),
		prefixed("-", "\treq.Header.Set(\"Authorization\", \"Bearer \"+tok)"),
		prefixed(" ", "\treturn c.send(ctx, req)", "}"),
		prefixed("+", "", "// Page requests one page of a list endpoint.", "func (c *Client) Page(ctx context.Context, path string, cursor string) (*Response, error) {", "\treturn c.Do(ctx, NewRequest(\"GET\", path+\"?cursor=\"+url.QueryEscape(cursor)))", "}"),
	)}},
}

var refreshWorkerFile = fileDiff{
	path: "src/worker/refresh.go", status: "M",
	hunks: []hunk{{oldStart: 17, newStart: 17, section: "func (w *Worker) refreshAll(ctx context.Context)", lines: join(
		prefixed(" ", "func (w *Worker) refreshAll(ctx context.Context) {", "\tfor _, s := range w.sessions {"),
		prefixed("-", "\t\ttok, err := s.Refresh(ctx)", "\t\tif err != nil {", "\t\t\tw.log.Printf(\"refresh %s: %v\", s.UserID(), err)", "\t\t\tcontinue", "\t\t}", "\t\tw.store.Save(s.UserID(), tok)"),
		prefixed("+", "\t\tif err := s.Refresh(ctx); err != nil {", "\t\t\tw.log.Printf(\"refresh %s: %v\", s.UserID(), err)", "\t\t\tcontinue", "\t\t}", "\t\tw.store.Save(s.UserID(), s.Token())"),
		prefixed(" ", "\t}", "}"),
	)}},
}

var authDocFile = fileDiff{
	path: "docs/auth.md", status: "M",
	hunks: []hunk{{oldStart: 9, newStart: 9, section: "## Token lifetime", lines: join(
		prefixed(" ", "## Token lifetime", ""),
		prefixed("-", "Tokens are refreshed on every request."),
		prefixed("+",
			"Tokens are cached per user for five minutes, matching the identity provider's",
			"default lifetime, and renewed 30 seconds before they expire. Concurrent requests",
			"for the same user share one refresh.",
			"",
			"| Setting | Default | Meaning |",
			"|---|---|---|",
			"| `AUTH_CACHE_TTL` | `5m` | How long a cached token is trusted |",
			"| `refreshWindow` | `30s` | Renew this long before expiry |",
			""),
		prefixed(" ", "", "See [sessions](sessions.md) for how the cache is wired in."),
	)}},
}

var paginationDocFile = fileDiff{
	path: "docs/api.md", status: "M",
	hunks: []hunk{{oldStart: 22, newStart: 22, section: "## Pagination", lines: join(
		prefixed(" ", "## Pagination", ""),
		prefixed("-", "List endpoints return everything."),
		prefixed("+", "List endpoints return one page. Pass `cursor` from the previous response to get the next page.", "The default page size is 50; use `limit` (1-200) to change it."),
		prefixed(" ", ""),
	)}},
}

// sessionFiles lists the files each fake session changed.
func sessionFiles(slug string) []fileDiff {
	switch slug {
	case "auth-refactor":
		return []fileDiff{clientGoFile, cacheGoFile, sessionGoFile, sessionTestFile, tokenGoFile, refreshWorkerFile, authDocFile}
	case "api-pagination":
		return []fileDiff{clientGoFile, paginationDocFile}
	case "docs-sweep":
		return []fileDiff{authDocFile, paginationDocFile}
	}
	return nil
}

// fileContent returns a fake file's text at the working tree (rev "") or at
// the base (any other rev), and whether the path is known.
func fileContent(slug, path, rev string) (string, bool) {
	newSide := rev == ""
	if f, ok := findFile(slug, path); ok {
		if f.path == "src/auth/session.go" {
			return f.content(newSide, sessionFill(newSide)), true
		}
		if f.status == "A" && !newSide {
			return "", false
		}
		return f.content(newSide, genericFill(path)), true
	}
	if c, ok := steeringFiles[path]; ok {
		return c, true
	}
	return "", false
}

func findFile(slug, path string) (fileDiff, bool) {
	for _, f := range sessionFiles(slug) {
		if f.path == path {
			return f, true
		}
	}
	return fileDiff{}, false
}

// sessionFill produces src/auth/session.go's unchanged stretches: lines 1-37,
// then the 42 lines between the two hunks (old 51-92, new 53-94).
func sessionFill(newSide bool) func(from, to int) []string {
	head := strings.Split(sessionGoHead, "\n")
	gap := strings.Split(sessionGoGap, "\n")
	gapStart := 51
	if newSide {
		gapStart = 53
	}
	return func(from, to int) []string {
		var out []string
		for n := from; n <= to; n++ {
			switch {
			case n <= len(head):
				out = append(out, head[n-1])
			case n-gapStart >= 0 && n-gapStart < len(gap):
				out = append(out, gap[n-gapStart])
			}
		}
		return out
	}
}

// genericFill produces plausible unchanged lines for files we only know the
// hunks of.
func genericFill(path string) func(from, to int) []string {
	md := strings.HasSuffix(path, ".md")
	return func(from, to int) []string {
		var out []string
		for n := from; n <= to; n++ {
			switch {
			case md && n%5 == 0:
				out = append(out, "")
			case md:
				out = append(out, "Documentation for line "+itoa(n)+".")
			case n%7 == 0:
				out = append(out, "")
			case n%7 == 1:
				out = append(out, "// helper"+itoa(n)+" is part of the package internals.")
			case n%7 == 2:
				out = append(out, "func helper"+itoa(n)+"() {")
			case n%7 == 6:
				out = append(out, "}")
			default:
				out = append(out, "\t_ = "+itoa(n))
			}
		}
		return out
	}
}
