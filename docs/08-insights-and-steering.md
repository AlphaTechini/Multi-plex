# 08 · Insights, steering and summaries

## The problem

Agents often surface things you should know while they work ("by the way, this will change the public API", "I assumed X"). Today those notes scroll past in the agent's output. To ask about one you either interrupt the agent or copy the text and ask after it finishes. And if the note means the current plan is wrong, there is no quick way to change course.

## Insight cards

- Agents create cards by calling the MCP tool **`note_to_user`**:

  ```json
  {
    "title": "Changing Session.Refresh signature",
    "body": "Callers in api/ and worker/ will need updating.",
    "severity": "info | warning | decision-needed",
    "refs": ["src/auth/session.go:L40-L52"]
  }
  ```

- Cards appear **numbered** in the Overview mode of the main view, newest first, and as a count badge on the session in the sidebar.
- Using a tool instead of formatted text means cards are structured and clickable, and do not depend on parsing the agent's output.

### Card actions

| Action | What happens | Effect on the agent's main conversation |
|---|---|---|
| **Ask** | Opens a side thread with the card's content already included. You ask your question there. | None. The main conversation is not changed. |
| **Steer** | Sends a message to the agent, prefilled with the card reference, that it picks up at its next turn boundary | Added to the conversation |
| **Interrupt + steer** | Stops the current step, then sends the message | Added to the conversation |
| **Dismiss** | Marks the card as read | None |

"Ask" and "Steer" are deliberately separate. A side question (like Claude Code's `/btw`) never changes the main session. Pivoting the session needs a real message to it.

### Wrapper mode (Claude Code)

- **Ask** types `/btw <card context> <your question>` into the agent's terminal. Claude Code's `/btw` answers without adding to the conversation history and works while it is processing.
- **Steer** types the message into the agent's input. Whether Claude Code queues it for the next turn or handles it immediately must be verified (CC-08).
- **Interrupt** sends the CLI's interrupt key (Esc for Claude Code), then the message.

### Native mode (v2)

Side threads are separate model calls that reuse the cached conversation prefix, so they see everything the agent knows. Steering inserts a user message at the next turn boundary.

## Summaries

- Agents call **`report_progress`** at the end of each turn (and at milestones):

  ```json
  {
    "done": ["Added token cache to Session.Refresh"],
    "files_changed": ["src/auth/session.go", "src/auth/session_test.go"],
    "decisions": ["Cache TTL is 5 minutes, matching the token lifetime"],
    "open_questions": ["Should the cache be per-user or global?"],
    "next": "Update callers in api/"
  }
  ```

- The Overview mode renders the latest report as the session's summary, with history available.
- Structured fields keep summaries short and consistent, and avoid filling the conversation with long prose.

## Telling agents to use these tools

A short instruction is added to every session (as a skill or appended system prompt, depending on the CLI):

- Call `report_progress` at the end of every turn.
- Call `note_to_user` for anything the user should know that does not need an answer right now. Use `decision-needed` when it does.
- Do not repeat card or summary content in long prose.

Compliance is checked in CC-07. If an agent forgets, the Overview falls back to the agent's last message.
