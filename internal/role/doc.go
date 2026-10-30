// Package role loads agent roles: short Markdown files with a front matter
// block (name, description, agent, tools) and a prompt body. Roles come from
// the workspace (.multiplex/roles), the user's config folder, and built-in
// defaults embedded in the binary. See docs/10-agent-roles-and-skills.md.
package role
