package fake

// steeringFiles are the contents served for the steering files the sidebar
// opens read-only in the Code view.
var steeringFiles = map[string]string{
	"AGENTS.md": `# Instructions for coding agents

## Before you write code

1. Read README.md, then the docs relevant to your task.
2. If what you are about to build disagrees with a doc, stop and ask.

## Platform rules

- Every feature must work on Windows, macOS and Linux.
- Use filepath, not string concatenation, for paths.

## Safety rules

- Never log secrets, tokens or the contents of credential files.
`,
	"CLAUDE.md": "@AGENTS.md\n",
	".multiplex/rules.toml": `# Safety rules for this workspace.
[[rule]]
id = "rm-recursive"
match = "recursive delete"
action = "ask"
`,
}
