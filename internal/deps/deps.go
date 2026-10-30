//go:build tools

package deps

import (
	_ "charm.land/bubbles/v2/textinput"
	_ "charm.land/bubbletea/v2"
	_ "charm.land/lipgloss/v2"
	_ "github.com/Microsoft/go-winio"
	_ "github.com/alecthomas/chroma/v2"
	_ "github.com/charmbracelet/x/vt"
	_ "github.com/charmbracelet/x/xpty"
	_ "github.com/modelcontextprotocol/go-sdk/mcp"
	_ "modernc.org/sqlite"
)
