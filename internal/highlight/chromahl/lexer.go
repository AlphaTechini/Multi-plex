package chromahl

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// maxCachedNames bounds the lexer cache; beyond it lookups still work but
// are no longer remembered.
const maxCachedNames = 4096

// lexerEntry is a cached lookup result. lexer is nil for unknown languages.
type lexerEntry struct {
	lexer chroma.Lexer
	name  string // language name for fences, "" when unknown
}

// lexerCache remembers lexer lookups. chroma's Match walks every lexer's
// filename globs, which costs far more than one tokenisation of a few lines.
//
// The key is the base name rather than the extension alone: some lexers are
// chosen by whole name (Makefile, Dockerfile, go.mod, CMakeLists.txt) and
// would be shadowed by whatever owns the same extension.
type lexerCache struct {
	m sync.Map // base name -> lexerEntry
	n sync.Mutex
	c int // entries stored
}

// lookup returns the lexer entry for path.
func (lc *lexerCache) lookup(path string) lexerEntry {
	base := filepath.Base(filepath.FromSlash(path))
	if v, ok := lc.m.Load(base); ok {
		return v.(lexerEntry)
	}
	var e lexerEntry
	if l := matchLexer(base); l != nil {
		e = lexerEntry{lexer: l, name: languageName(l.Config())}
	}
	lc.n.Lock()
	if lc.c < maxCachedNames {
		lc.c++
		lc.m.Store(base, e)
	}
	lc.n.Unlock()
	return e
}

// noLexer lists names chroma would match to the wrong language (it has no Go
// module lexer and picks AMPL for *.mod); they are shown as plain text.
var noLexer = map[string]bool{"go.mod": true, "go.sum": true, "go.work": true, "go.work.sum": true}

// matchLexer finds the lexer for a base file name, or nil.
func matchLexer(base string) chroma.Lexer {
	if noLexer[base] {
		return nil
	}
	return lexers.Match(base)
}

// languageName returns a lower-case identifier usable as a code-fence
// language: the lexer's lower-cased name ("typescript"), or its first alias
// when the name has spaces.
func languageName(c *chroma.Config) string {
	if n := strings.ToLower(c.Name); n != "" && !strings.ContainsAny(n, " \t") {
		return n
	}
	if len(c.Aliases) > 0 {
		return c.Aliases[0]
	}
	return strings.ToLower(strings.ReplaceAll(c.Name, " ", "-"))
}
