# 16 · Visual design

The look and feel of Multiplex: principles, design tokens, components, every main screen at two sizes, focus, motion and responsive rules. [06](06-ui-layout.md) fixes the layout and [07](07-diff-viewer.md) the diff viewer's behaviour; this doc fixes how they look.

The design is implemented, not only drawn:

- `internal/ui/theme`: the tokens (colours with explicit 256-colour fallbacks, glyphs, motion timings) and the theme registry.
- `internal/ui/kit`: the primitives every screen is built from (panel, tabs, rows, cards, banner, diff line, modal, toast, status bar…). See its package doc for the rules developers follow.
- `cmd/mpx-preview`: renders the screens below with the real theme and kit and fake data. Run it in your own terminal to judge the design there:

  ```
  go run ./cmd/mpx-preview -screen diff -w 180 -h 48
  go run ./cmd/mpx-preview -screen overview -w 110 -h 32 -profile 256
  go run ./cmd/mpx-preview -screen approval -ascii
  ```

  Screens: `overview`, `diff`, `approval`, `palette`, `empty`, `newsession`, `kit` (component gallery), `tokens` (the colour table as Markdown). Flags: `-w`, `-h`, `-profile truecolor|256`, `-ascii`, `-reduce-motion`, `-frame N` (spinner), `-settle` / `-fade` (diff file-switch animation, 0..1), `-fresh` (new-hunk highlight, 0..1), `-plain` (no colour; how the mockups in this doc were made).

The mockups in this doc are the preview's `-plain` output, so they match the code cell for cell. Colour, which plain text cannot show, is described next to each one.

## Principles

1. **Calm by default, loud only when it needs you.** The screen is mostly neutral greys on the Dark+ editor background. Saturated colour is reserved for meaning: a blocked session, a decision, an approval, a diff. If everything is coloured, nothing is.
2. **Hierarchy from type and space, not boxes.** Five text tiers, section labels, whitespace and one raised surface do the work. The three regions are framed; nothing inside them is (no box inside a box).
3. **Exact geometry.** Every primitive returns exactly the size it was given, so content updates never shift the layout. Alignment is to the cell: gutters, label columns and right edges line up across rows.
4. **One focus, always visible.** Exactly one region has the accent frame. Selection inside a region is a surface change plus an accent bar, never colour alone.
5. **Motion explains change, then gets out of the way.** A file switch, a new hunk, a working agent. Nothing animates for decoration; everything can be turned off.

## Colour

### Approach

- **Baseline:** VS Code Dark+ (MIT). Editor background `#1E1E1E`, editor foreground `#D4D4D4`, the Dark+ token colours for code, the Dark+ focus blue `#3794FF` and button blue `#0E639C`.
- **Refined into a UI palette:** surfaces step in small, even increments around the editor background (sunken → base → raised → overlay); text has five tiers; state and severity colours are tuned to read at the same weight on the base surface.
- **Truecolor first, 256 designed:** every token has an explicit 256-colour index chosen by hand, not left to nearest-colour conversion. `theme.Theme.For(profile)` resolves the theme for the terminal's profile; on ANSI-16 terminals the writer reduces the 256 index further. The 256 diff deliberately differs: the 256 palette has no quiet dark greens or reds (22 and 52 are loud as full-line backgrounds), so added and removed lines keep the base background and are carried by the coloured gutter, the sign and the word-level emphasis.
- **Only semantic tokens in code.** Screens and kit read `t.Text.Muted`, `t.State.Blocked`, `t.Diff.AddLineBg`; hex appears only in theme files.

### Tokens (Dark+)

Generated with `mpx-preview -screen tokens`; the usage column is the contract.

| Token | Hex | 256 | Usage |
|---|---|---|---|
| `Surface.Sunken` | `#181818` | 233 | Status bar, input wells, folded-lines band, command block in approvals, scrim target behind modals |
| `Surface.Base` | `#1E1E1E` | 234 | Default background of every region (Dark+ editor background) |
| `Surface.Raised` | `#252526` | 235 | Selected insight card, comment box, file banner at rest |
| `Surface.Overlay` | `#2A2A2C` | 236 | Modals, command palette, toasts |
| `Surface.Control` | `#3A3D41` | 238 | Secondary buttons, key caps, unfocused choice pill |
| `Surface.Hover` | `#2A2D2E` | 236 | Selected row in an unfocused list |
| `Surface.Selected` | `#04395E` | 24 | Selected row in the focused list (with an accent bar) |
| `Surface.TextSelection` | `#264F78` | 24 | Selected text or lines (copy, multi-line select) |
| `Border.Subtle` | `#2D2D2D` | 235 | Separators inside a region (file rail divider) |
| `Border.Default` | `#444444` | 238 | Frames of unfocused regions, rules, progress track |
| `Border.Focus` | `#3794FF` | 33 | Frame of the focused region and of ordinary dialogs (only one on screen) |
| `Text.Strong` | `#F0F0F0` | 255 | Titles, active tab, selected row, key names in hints |
| `Text.Primary` | `#D4D4D4` | 252 | Body text and code (Dark+ editor foreground) |
| `Text.Secondary` | `#A9A9A9` | 248 | Supporting text: card bodies, descriptions, unfocused titles |
| `Text.Muted` | `#858585` | 244 | Metadata, labels, line numbers, hint labels |
| `Text.Faint` | `#5A5A5A` | 240 | Placeholders, separators, ages, inactive card actions |
| `Text.OnAccent` | `#FFFFFF` | 231 | Text on `Accent.Fill` (primary button) |
| `Text.OnState` | `#1A1A1A` | 234 | Dark text on bright pills (approvals pill) |
| `Accent.Base` | `#3794FF` | 33 | Focus ring, selection bar, cursor, links and file refs, matched characters |
| `Accent.Fill` | `#0E639C` | 25 | Primary button background (Dark+ button) |
| `Accent.Subtle` | `#1D3349` | 236 | Active tab pill, focused choice, file banner right after a switch |
| `State.Starting` | `#9D9D9D` | 247 | Session creating its worktree or launching |
| `State.Working` | `#4FC1FF` | 75 | Agent running a turn (spinner) |
| `State.Blocked` | `#E8AB53` | 215 | Waiting for you: approval or question. The loudest state |
| `State.Done` | `#73C991` | 114 | Finished, not yet viewed; checks and "reviewed" |
| `State.Idle` | `#6E6E6E` | 242 | Waiting for a task |
| `State.Error` | `#F14C4C` | 203 | Crashed agent or failed setup |
| `Severity.Info` | `#3794FF` | 33 | Insight card: note |
| `Severity.Warning` | `#E8AB53` | 215 | Insight card: risk. Same amber as Blocked: amber always means "look at this" |
| `Severity.Decision` | `#C586C0` | 176 | Insight card: needs an answer. Dark+ control-keyword purple, used nowhere else in the chrome |
| `Diff.AddLineBg` | `#1C2E22` | 234 | Added line background |
| `Diff.AddWordBg` | `#2B5937` | 22 | Changed words inside an added line |
| `Diff.AddGutterBg` | `#1A291E` | 22 | Line-number gutter of an added line |
| `Diff.AddSign` | `#73C991` | 114 | `+` sign, added counts, `A` status |
| `Diff.DelLineBg` | `#35201F` | 234 | Removed line background |
| `Diff.DelWordBg` | `#6A2C2A` | 52 | Changed words inside a removed line |
| `Diff.DelGutterBg` | `#2F1E1D` | 52 | Line-number gutter of a removed line |
| `Diff.DelSign` | `#F48771` | 210 | `−` sign, removed counts, `D` status |
| `Diff.HunkBg` | `#1C2530` | 235 | Hunk header row |
| `Diff.HunkFg` | `#6F9FCF` | 68 | `@@ -a,b +c,d @@` range |
| `Diff.LineNumber` | `#6E6E6E` | 242 | Gutter line numbers |
| `Diff.FoldFg` | `#707070` | 242 | "··· 42 unchanged lines ···" |
| `Diff.NewHunkBg` | `#24466B` | 24 | Start colour of the new-hunk highlight (decays to the line background) |
| `Diff.Renamed` | `#C586C0` | 176 | `R` status |
| `Diff.Modified` | `#E2C08D` | 180 | `M` status, "modified" (Dark+ git decoration colour) |
| `Syntax.Plain` | `#D4D4D4` | 252 | Code text, operators, punctuation |
| `Syntax.Keyword` | `#569CD6` | 74 | `func`, `var`, `type`, `nil` |
| `Syntax.Control` | `#C586C0` | 175 | `if`, `return`, `for`, `defer` |
| `Syntax.String` | `#CE9178` | 173 | String and rune literals |
| `Syntax.Number` | `#B5CEA8` | 151 | Numbers |
| `Syntax.Comment` | `#6A9955` | 65 | Comments |
| `Syntax.Function` | `#DCDCAA` | 187 | Function and method names |
| `Syntax.Type` | `#4EC9B0` | 79 | Type names |
| `Syntax.Variable` | `#9CDCFE` | 117 | Variables, parameters, fields; inline code in prose |
| `Syntax.Constant` | `#4FC1FF` | 75 | Constants, enum members |
| `Syntax.Regexp` | `#D16969` | 167 | Regular expressions, escapes |

Contrast on `Surface.Base` (WCAG): Strong 14.6:1, Primary 11.2:1, Secondary 7.1:1, Muted 4.5:1, Faint 2.4:1; Accent 5.4:1, Working 8.3:1, Blocked 8.2:1, Decision 6.0:1. Muted is the lowest tier for text you must read; Faint is only for text you can ignore (separators, placeholders, inactive actions).

### Colour rules

- **Amber means "you are needed"**: blocked sessions, approvals, warnings. It is the only colour that appears in the status bar as a filled pill.
- **Blue means "interactive or focused"**: focus frame, selection bar, cursor, file references you can open. Working sessions use a lighter, cyan-leaning blue so they read as activity, not focus.
- **Green and red are for diffs and outcomes only** (added/removed, done/error), never for decoration.
- **Purple is reserved for decisions** (and renames in the file rail, where no decision cards appear).
- Backgrounds are always painted. Every cell carries its surface colour, so the app looks the same whatever the terminal's own background is. Foreign content (the agent's terminal) is backfilled onto the panel surface (`kit.Backfill`).

## Type and text hierarchy

Terminals have one size and one family, so hierarchy comes from tier, weight and case.

| Use | Style |
|---|---|
| Region title (focused) | `Text.Strong`, **bold** |
| Region title (unfocused) | `Text.Secondary`, regular |
| Screen headline (task title, dialog title) | `Text.Strong`, **bold** |
| Section label (SESSIONS, SUMMARY, INSIGHTS) | UPPER CASE, `Text.Muted`, **bold**; count or meta right-aligned in `Text.Faint` |
| Body | `Text.Primary` |
| Supporting text | `Text.Secondary` |
| Metadata, labels | `Text.Muted` |
| Separators, placeholders, ages | `Text.Faint` |
| Key names in hints | `Text.Secondary` **bold**, label `Text.Muted` |
| Severity label on cards | UPPER CASE, severity colour, **bold** |
| Unread card or unseen session | name **bold** |
| File refs (`src/auth/cache.go:L12-L30`) | `Accent.Base`, no underline (underline is reserved for links that leave the app) |
| Code in diffs | Dark+ syntax colours; *italic* only where the syntax theme asks for it |

Rules:

- **Bold** marks the one thing to read first in a block (title, active tab, selected item). Never bold whole paragraphs.
- *Italic* is not used for UI text (several Windows fonts render it poorly). Faint (SGR 2) is not used either: it renders inconsistently across terminals. Use the `Text.Faint` colour.
- Underline only for external links. Strikethrough only for dismissed items in history.
- UPPER CASE only for section labels and severity labels, never for sentences.
- Sentence case everywhere else ("New session", not "New Session"). Lower-case labels in key hints ("palette", "next file").

## Glyphs

Every glyph is exactly one cell wide in both sets, so switching sets never moves anything. The ASCII set is used when the user sets `glyphs = "ascii"` (fonts without the symbols, legacy code pages, screen readers that read symbols badly).

| Role | Unicode | ASCII |
|---|---|---|
| Working (spinner, 80 ms per frame) | `⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏` | `\| / - \` |
| Working (static, reduced motion) | `◐` | `~` |
| Starting | `◌` | `.` |
| Blocked | `●` | `!` |
| Done, reviewed, check | `✓` | `+` |
| Idle | `○` | `o` |
| Error, cross | `✕` | `x` |
| Inline separator | `·` | `\|` |
| Ellipsis (truncation) | `…` | `~` |
| Chevrons (prompt, breadcrumb) | `›` `‹` | `>` `<` |
| Disclosure | `▾` `▸` | `v` `>` |
| Arrows (key hints) | `↑ ↓ ← →` | `^ v < >` |
| Diff signs | `+` `−` | `+` `-` |
| Branch | `⎇` | `@` |
| Card / banner / selection bar | `▌` | `\|` |
| Comment / field bar | `▎` | `\|` |
| Fold dots | `·` | `.` |
| Progress fill / track | `━` `─` | `=` `-` |
| Rule | `─` | `-` |
| Frame | `╭ ╮ ╰ ╯ ─ │` (rounded, light) | `+ + + + - \|` |
| Keys | `⌃` ctrl, `⇧` shift, `⌥` alt, `↵` enter, `␣` space, `⌫` backspace, `esc`, `tab` | `C-`, `S-`, `M-`, `enter`, `space`, `bksp`, `esc`, `tab` |

Key hints are written as specs (`"ctrl+\\ p"`, shown `⌃\ p`) and formatted by `kit.FormatKeys`, so one place decides how keys look. Avoid emoji and symbols with emoji presentation (`⚠`, `✔`, `❌`): terminals disagree about their width.

## Spacing and structure

- **Grid:** one cell. Horizontal rhythm uses 1, 2 and 3 cells; vertical rhythm uses 0 or 1 blank line. Never two blank lines in a row inside a region.
- **Regions:** sidebar, main view and agent panel are rounded light frames placed side by side with no gap; the status bar sits below them on the sunken surface. The top edge of a frame carries its title (left) and meta (right); the main view's top edge carries the mode tabs instead. The bottom edge may carry key hints (agent panel).
- **Padding inside regions:** text content starts 2 cells in from the frame (Overview), 1 cell for lists that draw their own marker column (sidebar, file rail, palette), 0 for edge-to-edge content (diff lines, agent terminal).
- **Lists:** column 0 is the marker column (accent bar `▌` on the selected row of the focused list), then 1 cell, then content, with 1 cell of right padding. Right-aligned metadata (counts, ages, key hints) always ends 1 cell before the edge.
- **Label columns:** key/value blocks (summary, approval details) use a fixed label column (12 cells in Overview, 10 in dialogs), muted labels, primary values.
- **Cards, comments, toasts:** no frame. A severity- or tone-coloured bar down the left edge, the content indented under the title, and the raised or overlay surface when selected or floating.
- **Dialogs:** overlay surface, rounded frame in the focus colour (amber for approvals), 2 cells of horizontal padding, 1 blank line top and bottom, title row, a blank line, body, a blank line, buttons.
- **Gaps between sections:** one blank line, then a section label.
- **No box inside a box.** The only nested frame is a dialog over the (dimmed) screen. Code blocks inside dialogs use the sunken surface, not a frame.

## Layout and focus

```
╭─ sidebar ─╮╭─ [Overview] Diff Code Specs Term ───── meta ─╮╭─ ⠹ session ───── agent ─╮
│ 28 cols   ││ main view: the rest                          ││ 30% of width, min 60    │
╰───────────╯╰──────────────────────────────────────────────╯╰─ hints ─────────────────╯
 status bar: workspace › session   state   ⎇ branch          [approvals]   key hints
```

**Focus model.** Exactly one of sidebar, main view and agent panel is focused. Its frame and title switch to `Border.Focus` and `Text.Strong` bold; the other two frames are `Border.Default` with `Text.Secondary` titles. Inside the focused region the selected row gets `Surface.Selected` and the accent bar; in unfocused regions the selected row keeps a quieter `Surface.Hover` and no bar, so you can still see what is selected where. Dialogs take focus: their frame is the only accent frame on screen while open (the screen behind is dimmed).

Typing goes to the focused region. When the agent panel is focused, every key except the prefix goes to the agent's terminal ([06](06-ui-layout.md)).

## Screens

Mockups are exact at the stated size. In colour (see the PNG renders, or run the preview):

- frames are `Border.Default`, the focused one `Border.Focus` blue;
- the active tab is a blue-tinted pill (`Accent.Subtle`), other tabs muted;
- `⠹` (working) is light blue, `●` (blocked) and the approvals pill are amber, `✓` (done) green;
- card bars are purple (decision), amber (warning), blue (info); the selected card sits on `Surface.Raised` and its actions are bright, the others' actions faint.


### Overview mode

The selected session's latest report, its insight cards and its warnings. Sidebar unfocused (its selected session on `Surface.Hover`), main view focused (blue frame), agent panel unfocused.

- Header: the task as the headline, then state badge · role · branch · turn and elapsed time.
- **Summary** (`report_progress`, [08](08-insights-and-steering.md)) as a key/value block with a 12-cell muted label column. Done items carry a green `✓`, open questions a purple `?`, files are accent refs.
- **Insights**, newest first, numbered. Card 1 is a decision (purple bar) and is selected (`Surface.Raised`, bright actions `a ask  s steer  i interrupt + steer  d dismiss`). Card 2 is a warning (amber), card 3 info (blue).
- **Conflicts** (amber dot) and **Spec** progress close the view.
- The sidebar shows three sessions: working (spinner, 3 unread cards), blocked (amber dot, "needs approval" in amber, badge in amber) and done (green check, name bold until viewed).
- At 110 × 32 the sidebar and agent panel are rails, the summary is compact and the card list ends with "↓ 1 more card below".

**180 × 48**

```
╭─ multi-plex ──── ⎇ main ─╮╭─  Overview 3   Diff 7   Code   Specs   Term  ────────────────────────── updated 32s ago ─╮╭─ ⠹ auth-refactor ─────────────────────────────── claude ─╮
│                          ││                                                                                          ││                                                          │
│ SESSIONS               3 ││  Cache auth tokens per user and update the callers                                       ││ ● Read(src/auth/session.go)                              │
│  ⠹ auth-refactor       3 ││  ⠹ working  ·  implementer  ·  ⎇ mpx/auth-refactor  ·  turn 6 · 14m                      ││   ⎿  Read 182 lines                                      │
│    implementer · 14m     ││                                                                                          ││                                                          │
│  ● api-pagination      1 ││  SUMMARY                                                                  report 6 of 6  ││ ● Update(src/auth/session.go)                            │
│    needs approval        ││  Done        ✓ Added token cache to Session.Refresh                                      ││   ⎿  Updated with 24 additions and 9 removals            │
│  ✓ docs-sweep            ││              ✓ Moved the expiry check into tokenValid()                                  ││                                                          │
│    done · 6 files        ││  Decided     Cache TTL is 5 minutes, matching the token lifetime                         ││ ● Bash(go test ./internal/auth/...)                      │
│                          ││  Open        ? Should the cache be per-user or global?                                   ││   ⎿  ok   internal/auth   0.412s                         │
│ SPECS                  1 ││  Next        Update callers in api/ and worker/                                          ││                                                          │
│   token-refresh ━─── 3/7 ││  Files       src/auth/session.go  src/auth/cache.go  +5 more                             ││ ● Update(src/auth/cache.go)                              │
│                          ││                                                                                          ││   ⎿  Created with 41 lines                               │
│ STEERING                 ││  INSIGHTS                                                                         2 new  ││                                                          │
│   AGENTS.md              ││  ▌  1 DECISION NEEDED  Cache scope: per-user or global?                             2m   ││ ● Token cache is in place and the auth tests pass.       │
│   CLAUDE.md              ││  ▌    A per-user cache costs about 4 MB at 10k users. A global cache is smaller but      ││   Before I update the callers in api/ and worker/:       │
│   ~/.claude/CLAUDE.md    ││  ▌    would share tokens across tenants if a key is ever wrong. I am holding the…        ││   should the cache be per-user or global? I raised it as │
│   .multiplex/            ││  ▌    src/auth/cache.go:L12-L30                                                          ││   card 1.                                                │
│                          ││  ▌    a ask   s steer   i interrupt + steer   d dismiss                                  ││                                                          │
│ ROLES                  4 ││                                                                                          ││ ✶ Updating callers… (esc to interrupt)                   │
│ SKILLS                 6 ││  ▌  2 WARNING  Changing Session.Refresh signature                                   6m   ││                                                          │
│ MCP                    2 ││  ▌    Refresh now takes a cache argument. Callers in api/ and worker/ need updating;     ││                                                          │
│                          ││  ▌    api-pagination also edits src/api/client.go.                                       ││                                                          │
│                          ││  ▌    src/auth/session.go:L38-L52                                                        ││                                                          │
│                          ││  ▌    a ask   s steer   i interrupt + steer   d dismiss                                  ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││  ▌  3 INFO  Assumed tokens expire after 5 minutes                                  11m   ││                                                          │
│                          ││  ▌    Matched the TTL to the identity provider's default token lifetime. Override with   ││                                                          │
│                          ││  ▌    AUTH_CACHE_TTL.                                                                    ││                                                          │
│                          ││  ▌    src/auth/cache.go:L8                                                               ││                                                          │
│                          ││  ▌    a ask   s steer   i interrupt + steer   d dismiss                                  ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││  CONFLICTS                                                                               ││                                                          │
│                          ││  ● Will conflict with api-pagination in src/api/client.go   merge-tree, checked 1m ago   ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││  SPEC                                                                                    ││                                                          │
│                          ││  token-refresh   ━━━━━━━━━━────────────── 3/7  tasks   5/9 criteria verified             ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││ ──────────────────────────────────────────────────────── │
│                          ││                                                                                          ││ > Try "update the callers in api/"                       │
│                          ││                                                                                          ││ ──────────────────────────────────────────────────────── │
│                          ││                                                                                          ││   ? for shortcuts                                        │
╰──────────────────────────╯╰──────────────────────────────────────────────────────────────────────────────────────────╯╰─ esc interrupt   ⌃\ z zoom ──────────────────────────────╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                                                   ● 1 approval · api-pagination    ⌃\ p palette   ⌃\ n new   ? keys
```

**110 × 32**

```
╭────╮╭─  Overview 3   Diff 7   Code   Specs   Term  ─────────────────────────────────── updated 32s ago ─╮╭─╮
│    ││                                                                                                   ││⠹│
│  ⠹ ││  Cache auth tokens per user and update the callers                                                ││ │
│  3 ││  ⠹ working  ·  implementer  ·  ⎇ mpx/auth-refactor  ·  turn 6 · 14m                               ││ │
│    ││                                                                                                   ││ │
│  ● ││  SUMMARY                                                                           report 6 of 6  ││ │
│  1 ││  Done        ✓ Added token cache to Session.Refresh; moved expiry check into tokenValid           ││ │
│    ││  Open        ? Should the cache be per-user or global?                                            ││ │
│  ✓ ││  Next        Update callers in api/ and worker/                                                   ││ │
│    ││  Files       src/auth/session.go  src/auth/cache.go  +5 more                                      ││ │
│    ││                                                                                                   ││ │
│    ││  INSIGHTS                                                                                  2 new  ││ │
│    ││  ▌  1 DECISION NEEDED  Cache scope: per-user or global?                                      2m   ││ │
│    ││  ▌    A per-user cache costs about 4 MB at 10k users. A global cache is smaller but would share   ││ │
│    ││  ▌    tokens across tenants if a key is ever wrong. I am holding the caller updates until you…    ││ │
│    ││  ▌    src/auth/cache.go:L12-L30                                                                   ││ │
│    ││  ▌    a ask   s steer   i interrupt + steer   d dismiss                                           ││ │
│    ││                                                                                                   ││ │
│    ││  ▌  2 WARNING  Changing Session.Refresh signature                                            6m   ││ │
│    ││  ▌    Refresh now takes a cache argument. Callers in api/ and worker/ need updating; api-         ││ │
│    ││  ▌    pagination also edits src/api/client.go.                                                    ││ │
│    ││  ▌    src/auth/session.go:L38-L52                                                                 ││ │
│    ││  ▌    a ask   s steer   i interrupt + steer   d dismiss                                           ││ │
│    ││     ↓ 1 more card below                                                                           ││ │
│    ││                                                                                                   ││ │
│    ││  CONFLICTS                                                                                        ││ │
│    ││  ● Will conflict with api-pagination in src/api/client.go   merge-tree, checked 1m ago            ││ │
│    ││                                                                                                   ││ │
│    ││  SPEC                                                                                             ││ │
│    ││  token-refresh   ━━━━━━━━━━────────────── 3/7  tasks   5/9 criteria verified                      ││ │
╰────╯╰───────────────────────────────────────────────────────────────────────────────────────────────────╯╰─╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                ● 1 approval · api-pagination
```

### Diff mode

One file at a time ([07](07-diff-viewer.md)). In Diff mode the sidebar is its icon rail, so the code gets the width.

- **File rail** (180 only): grouped by directory, status letter in its colour (`M` gold, `A` green, `R` purple, `D` red), reviewed files marked `✓` and dimmed to muted, counts right-aligned. The current file is selected (`Surface.Hover`, since the rail is not focused).
- **Banner:** `src/auth/session.go · 3 of 7`, status, `+24 −9`, reviewed state, on `Surface.Raised` (tinted blue for 600 ms after a switch).
- **Hunk header** on `Diff.HunkBg`: the range in `Diff.HunkFg`, the enclosing declaration muted.
- **Unified lines:** old and new numbers, sign, Dark+ syntax colours on the line background. **Word-level highlights** (not visible in plain text) put `Diff.AddWordBg` / `Diff.DelWordBg` behind exactly the changed words: `time.Until(s.expiry) > refreshWindow` → `s.tokenValid(time.Now())`, `s.client.Token(ctx)` → `s.cache.Get(ctx, s.userID, s.client.Token)`, `err` → `fmt.Errorf("refresh token: %w", err)`.
- Lines 44–46 are selected (accent `▌` in the marker column, strong line numbers) and carry a **draft comment** being edited (thin blue bar, cursor, `↵ queue  ⌃s send now  esc cancel`).
- **Folded** context: "··· 42 unchanged lines ···" on a sunken band.
- **End of file:** a centred rule, "end of file · next src/auth/session_test.go ]"; the view never scrolls into the next file.
- **Toast** (180 only) confirms the queued comment, bottom-right above the status bar.
- At 110 × 32 the file rail collapses: the banner keeps "3 of 7" and a hint row offers `f files  [ prev file  ] next file  n next change  c comment`.

**180 × 48**

```
╭────╮╭─  Overview 3   Diff 7   Code   Specs   Term  ────────────────────────────────────────────── unified · 7 files ─╮╭─ ⠹ auth-refactor ─────────────────────────────── claude ─╮
│    ││                                │▌ src/auth/session.go  ·  3 of 7            modified   +24 −9   ○ not reviewed ││                                                          │
│  ⠹ ││ FILES             2/7 reviewed │                                                                               ││ ● Read(src/auth/session.go)                              │
│  3 ││                                │           @@ -38,13 +38,15 @@  func (s *Session) Refresh(ctx context.Context)…││   ⎿  Read 182 lines                                      │
│    ││  src/api/                      │   38   38   func (s *Session) Refresh(ctx context.Context) error {            ││                                                          │
│  ● ││    M  client.go          +6 −2 │   39   39       s.mu.Lock()                                                   ││ ● Update(src/auth/session.go)                            │
│  1 ││                                │   40   40       defer s.mu.Unlock()                                           ││   ⎿  Updated with 24 additions and 9 removals            │
│    ││  src/auth/                     │   41      −     if time.Until(s.expiry) > refreshWindow {                     ││                                                          │
│  ✓ ││  ✓ A  cache.go             +41 │        41 +     if s.tokenValid(time.Now()) {                                 ││ ● Bash(go test ./internal/auth/...)                      │
│    ││    M  session.go        +24 −9 │   42   42           return nil                                                ││   ⎿  ok   internal/auth   0.412s                         │
│    ││    M  session_test.go   +88 −4 │   43   43       }                                                             ││                                                          │
│    ││  ✓ R  token.go           +3 −3 │▌  44      −     tok, err := s.client.Token(ctx)                               ││ ● Update(src/auth/cache.go)                              │
│    ││                                │▌       44 +     tok, err := s.cache.Get(ctx, s.userID, s.client.Token)        ││   ⎿  Created with 41 lines                               │
│    ││  src/worker/                   │▌  45   45       if err != nil {                                               ││                                                          │
│    ││    M  refresh.go         +5 −5 │▌  46      −         return err                                                ││ ● Token cache is in place and the auth tests pass.       │
│    ││                                │▌       46 +         return fmt.Errorf("refresh token: %w", err)               ││   Before I update the callers in api/ and worker/:       │
│    ││  docs/                         │           ▎ You · draft                                               L44–L46 ││   should the cache be per-user or global? I raised it as │
│    ││    M  auth.md           +12 −1 │           ▎ Wrap with the user ID too, so the logs say whose refresh failed.  ││   card 1.                                                │
│    ││                                │           ▎ And this path runs on every request: keep the cache lookup lock-  ││                                                          │
│    ││                                │           ▎ free.                                                             ││ ✶ Updating callers… (esc to interrupt)                   │
│    ││                                │           ▎ ↵ queue   ⌃s send now   esc cancel                                ││                                                          │
│    ││                                │   47   47       }                                                             ││                                                          │
│    ││                                │        48 +     s.metrics.Refreshes.Inc()                                     ││                                                          │
│    ││                                │   48   49       s.token, s.expiry = tok.Value, tok.Expiry                     ││                                                          │
│    ││                                │   49   50       return nil                                                    ││                                                          │
│    ││                                │   50   51   }                                                                 ││                                                          │
│    ││                                │             ··· 42 unchanged lines ···                                        ││                                                          │
│    ││                                │           @@ -93,6 +95,14 @@  func (s *Session) Close() error                 ││                                                          │
│    ││                                │   93   95       return s.client.Close()                                       ││                                                          │
│    ││                                │   94   96   }                                                                 ││                                                          │
│    ││                                │   95   97                                                                     ││                                                          │
│    ││                                │        98 + // tokenValid reports whether the token outlives the refresh wind…││                                                          │
│    ││                                │        99 + func (s *Session) tokenValid(now time.Time) bool {                ││                                                          │
│    ││                                │       100 +     return s.token != "" && s.expiry.Sub(now) > refreshWindow     ││                                                          │
│    ││                                │       101 + }                                                                 ││                                                          │
│    ││                                │   96  102                                                                     ││                                                          │
│    ││                                │                                                                               ││                                                          │
│    ││                                │───────────── end of file  ·  next  src/auth/session_test.go   ]  ─────────────││                                                          │
│    ││                                │                                                                               ││                                                          │
│    ││                                │                                                                               ││                                                          │
│    ││                                │                               ▌                                               ││                                                          │
│    ││                                │                               ▌ ✓  Comment queued for auth-refactor           ││                                                          │
│    ││                                │                               ▌    2 queued · sent at the agent's next turn   ││ ──────────────────────────────────────────────────────── │
│    ││                                │                               ▌    ⌃s send now   u undo                       ││ > Try "update the callers in api/"                       │
│    ││  [ prev   ] next   v reviewed  │                               ▌                                               ││ ──────────────────────────────────────────────────────── │
│    ││                                │                                                                               ││   ? for shortcuts                                        │
╰────╯╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯╰─ esc interrupt   ⌃\ z zoom ──────────────────────────────╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                                                   ● 1 approval · api-pagination    ⌃\ p palette   ⌃\ n new   ? keys
```

**110 × 32**

```
╭────╮╭─  Overview 3   Diff 7   Code   Specs   Term  ───────────────────────────────── unified · 7 files ─╮╭─╮
│    ││▌ src/auth/session.go  ·  3 of 7                                modified   +24 −9   ○ not reviewed ││⠹│
│  ⠹ ││  f files   [ prev file   ] next file   n next change   c comment                                  ││ │
│  3 ││           @@ -38,13 +38,15 @@  func (s *Session) Refresh(ctx context.Context) error               ││ │
│    ││   38   38   func (s *Session) Refresh(ctx context.Context) error {                                ││ │
│  ● ││   39   39       s.mu.Lock()                                                                       ││ │
│  1 ││   40   40       defer s.mu.Unlock()                                                               ││ │
│    ││   41      −     if time.Until(s.expiry) > refreshWindow {                                         ││ │
│  ✓ ││        41 +     if s.tokenValid(time.Now()) {                                                     ││ │
│    ││   42   42           return nil                                                                    ││ │
│    ││   43   43       }                                                                                 ││ │
│    ││▌  44      −     tok, err := s.client.Token(ctx)                                                   ││ │
│    ││▌       44 +     tok, err := s.cache.Get(ctx, s.userID, s.client.Token)                            ││ │
│    ││▌  45   45       if err != nil {                                                                   ││ │
│    ││▌  46      −         return err                                                                    ││ │
│    ││▌       46 +         return fmt.Errorf("refresh token: %w", err)                                   ││ │
│    ││           ▎ You · draft                                                                   L44–L46 ││ │
│    ││           ▎ Wrap with the user ID too, so the logs say whose refresh failed. And this path runs   ││ │
│    ││           ▎ on every request: keep the cache lookup lock-free.                                    ││ │
│    ││           ▎ ↵ queue   ⌃s send now   esc cancel                                                    ││ │
│    ││   47   47       }                                                                                 ││ │
│    ││        48 +     s.metrics.Refreshes.Inc()                                                         ││ │
│    ││   48   49       s.token, s.expiry = tok.Value, tok.Expiry                                         ││ │
│    ││   49   50       return nil                                                                        ││ │
│    ││   50   51   }                                                                                     ││ │
│    ││             ··· 42 unchanged lines ···                                                            ││ │
│    ││           @@ -93,6 +95,14 @@  func (s *Session) Close() error                                     ││ │
│    ││   93   95       return s.client.Close()                                                           ││ │
│    ││   94   96   }                                                                                     ││ │
│    ││   95   97                                                                                         ││ │
╰────╯╰───────────────────────────────────────────────────────────────────────────────────────────────────╯╰─╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                ● 1 approval · api-pagination
```

### Approval prompt

A gated action ([04](04-safety.md), Layer 5) from a session that is not the one on screen. The screen behind is dimmed toward `Surface.Sunken` (scrim 0.7) and the dialog's frame is amber: the only amber frame the app ever draws.

Contents, in reading order: what is being asked ("cannot be undone"), which session (amber dot), a one-line plain-language reason, the **exact command** on a sunken code block (wrapped onto more lines when long, never truncated), the **working directory** (session worktree, full path truncated from the left), **why it was gated** (each layer and rule, with the rule's file and line), the **affected files** (count, size, first files, `l list all`), then how approval works (Windows Hello here; Touch ID, polkit or password elsewhere) and whether a session-wide rule is active. Buttons: **Approve once** (primary, `↵`, starts the biometric check), **Deny** (`esc`), **Deny with message** (`m`, opens a one-line input in place of the buttons). Enter never approves by itself: it starts the OS check, which an agent cannot pass.

**180 × 48**

```
╭─ multi-plex ──── ⎇ main ─╮╭─  Overview 3   Diff 7   Code   Specs   Term  ────────────────────────── updated 32s ago ─╮╭─ ⠹ auth-refactor ─────────────────────────────── claude ─╮
│                          ││                                                                                          ││                                                          │
│ SESSIONS               3 ││  Cache auth tokens per user and update the callers                                       ││ ● Read(src/auth/session.go)                              │
│  ⠹ auth-refactor       3 ││  ⠹ working  ·  implementer  ·  ⎇ mpx/auth-refactor  ·  turn 6 · 14m                      ││   ⎿  Read 182 lines                                      │
│    implementer · 14m     ││                                                                                          ││                                                          │
│  ● api-pagination      1 ││  SUMMARY                                                                  report 6 of 6  ││ ● Update(src/auth/session.go)                            │
│    needs approval        ││  Done        ✓ Added token cache to Session.Refresh                                      ││   ⎿  Updated with 24 additions and 9 removals            │
│  ✓ docs-sweep            ││              ✓ Moved the expiry check into tokenValid()                                  ││                                                          │
│    done · 6 files        ││  Decided     Cache TTL is 5 minutes, matching the token lifetime                         ││ ● Bash(go test ./internal/auth/...)                      │
│                          ││  Open        ?╭──────────────────────────────────────────────────────────────────────────────────────────╮rnal/auth   0.412s                         │
│ SPECS                  1 ││  Next        U│                                                                                          │                                           │
│   token-refresh ━─── 3/7 ││  Files       s│  Approve a change that cannot be undone?                               ● api-pagination  │uth/cache.go)                              │
│                          ││               │                                                                                          │ith 41 lines                               │
│ STEERING                 ││  INSIGHTS     │  The agent wants to delete files outside its worktree. They are not in any checkpoint.   │                                           │
│   AGENTS.md              ││  ▌  1 DECISION│                                                                                          │is in place and the auth tests pass.       │
│   CLAUDE.md              ││  ▌    A per-us│  Command    Remove-Item -Recurse -Force ..\..\multi-plex\testdata\golden                 │ate the callers in api/ and worker/:       │
│   ~/.claude/CLAUDE.md    ││  ▌    would sh│                                                                                          │ache be per-user or global? I raised it as │
│   .multiplex/            ││  ▌    src/auth│  In        api-pagination worktree  …TA%\Multiplex\worktrees\multi-plex\api-pagination   │                                           │
│                          ││  ▌    a ask   │  Why       Layer 2  writes outside the session's worktree                                │                                           │
│ ROLES                  4 ││               │            Layer 4  recursive delete  rule rm-recursive · .multiplex/rules.toml:14       │lers… (esc to interrupt)                   │
│ SKILLS                 6 ││  ▌  2 WARNING │  Affects   testdata/golden/  14 files · 1.2 MB · main checkout                           │                                           │
│ MCP                    2 ││  ▌    Refresh │            testdata/golden/session_refresh.json                                          │                                           │
│                          ││  ▌    api-pagi│            testdata/golden/session_expiry.json                                           │                                           │
│                          ││  ▌    src/auth│            + 12 more  l list all                                                         │                                           │
│                          ││  ▌    a ask   │                                                                                          │                                           │
│                          ││               │  ──────────────────────────────────────────────────────────────────────────────────────  │                                           │
│                          ││  ▌  3 INFO  As│                                                                                          │                                           │
│                          ││  ▌    Matched │  Windows Hello will confirm it is you. Approval covers this one command.                 │                                           │
│                          ││  ▌    AUTH_CAC│  Session-wide approval for this rule is off.                                             │                                           │
│                          ││  ▌    src/auth│                                                                                          │                                           │
│                          ││  ▌    a ask   │   Approve once  ↵    Deny  esc    Deny with message  m                                   │                                           │
│                          ││               │                                                                                          │                                           │
│                          ││  CONFLICTS    ╰──────────────────────────────────────────────────────────────────────────────────────────╯                                           │
│                          ││  ● Will conflict with api-pagination in src/api/client.go   merge-tree, checked 1m ago   ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││  SPEC                                                                                    ││                                                          │
│                          ││  token-refresh   ━━━━━━━━━━────────────── 3/7  tasks   5/9 criteria verified             ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││ ──────────────────────────────────────────────────────── │
│                          ││                                                                                          ││ > Try "update the callers in api/"                       │
│                          ││                                                                                          ││ ──────────────────────────────────────────────────────── │
│                          ││                                                                                          ││   ? for shortcuts                                        │
╰──────────────────────────╯╰──────────────────────────────────────────────────────────────────────────────────────────╯╰─ esc interrupt   ⌃\ z zoom ──────────────────────────────╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                                                   ● 1 approval · api-pagination    ⌃\ p palette   ⌃\ n new   ? keys
```

**110 × 32**

```
╭────╮╭─  Overview 3   Diff 7   Code   Specs   Term  ─────────────────────────────────── updated 32s ago ─╮╭─╮
│    ││                                                                                                   ││⠹│
│  ⠹ ││  Cache auth tokens per user and update the callers                                                ││ │
│  3 ││  ╭──────────────────────────────────────────────────────────────────────────────────────────╮     ││ │
│    ││  │                                                                                          │     ││ │
│  ● ││  │  Approve a change that cannot be undone?                               ● api-pagination  │f 6  ││ │
│  1 ││  │                                                                                          │     ││ │
│    ││  │  The agent wants to delete files outside its worktree. They are not in any checkpoint.   │     ││ │
│  ✓ ││  │                                                                                          │     ││ │
│    ││  │  Command    Remove-Item -Recurse -Force ..\..\multi-plex\testdata\golden                 │     ││ │
│    ││  │                                                                                          │     ││ │
│    ││  │  In        api-pagination worktree  …TA%\Multiplex\worktrees\multi-plex\api-pagination   │new  ││ │
│    ││  │  Why       Layer 2  writes outside the session's worktree                                │2m   ││ │
│    ││  │            Layer 4  recursive delete  rule rm-recursive · .multiplex/rules.toml:14       │re   ││ │
│    ││  │  Affects   testdata/golden/  14 files · 1.2 MB · main checkout                           │…    ││ │
│    ││  │            testdata/golden/session_refresh.json                                          │     ││ │
│    ││  │            testdata/golden/session_expiry.json                                           │     ││ │
│    ││  │            + 12 more  l list all                                                         │     ││ │
│    ││  │                                                                                          │6m   ││ │
│    ││  │  ──────────────────────────────────────────────────────────────────────────────────────  │     ││ │
│    ││  │                                                                                          │     ││ │
│    ││  │  Windows Hello will confirm it is you. Approval covers this one command.                 │     ││ │
│    ││  │  Session-wide approval for this rule is off.                                             │     ││ │
│    ││  │                                                                                          │     ││ │
│    ││  │   Approve once  ↵    Deny  esc    Deny with message  m                                   │     ││ │
│    ││  │                                                                                          │     ││ │
│    ││  ╰──────────────────────────────────────────────────────────────────────────────────────────╯     ││ │
│    ││                                                                                                   ││ │
│    ││  SPEC                                                                                             ││ │
│    ││  token-refresh   ━━━━━━━━━━────────────── 3/7  tasks   5/9 criteria verified                      ││ │
╰────╯╰───────────────────────────────────────────────────────────────────────────────────────────────────╯╰─╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                ● 1 approval · api-pagination
```

### Command palette

`⌃\ p`. A flush overlay (no inner padding) near the top of the screen over the dimmed view: the query with a block cursor, `esc close`, a rule, then results grouped by section with the matched characters in accent bold, key bindings right-aligned and muted. The selected row is `Surface.Selected` with the accent bar. Footer: navigation hints. `/` at the start of the query lists slash commands.

**180 × 48**

```
╭────╮╭─  Overview 3   Diff 7   Code   Specs   Term  ────────────────────────────────────────────── unified · 7 files ─╮╭─ ⠹ auth-refactor ─────────────────────────────── claude ─╮
│    ││                                │▌ src/auth/session.go  ·  3 of 7            modified   +24 −9   ○ not reviewed ││                                                          │
│  ⠹ ││ FILES             2/7 reviewed │                                                                               ││ ● Read(src/auth/session.go)                              │
│  3 ││                                │           @@ -38,13 +38,15 @@  func (s *Session) Refresh(ctx context.Context)…││   ⎿  Read 182 lines                                      │
│    ││  src/api/                      │   38   38   func (s *Session) Refresh(ctx context.Context) error {            ││                                                          │
│  ● ││    M  client.go          +6 −2 │   39   39       s.mu.Lock()                                                   ││ ● Update(src/auth/session.go)                            │
│  1 ││                                │   40   40 ╭────────────────────────────────────────────────────────────────────────────╮dated with 24 additions and 9 removals            │
│    ││  src/auth/                     │   41      │                                                                            │                                                  │
│  ✓ ││  ✓ A  cache.go             +41 │        41 │ › new                                                            esc close │go test ./internal/auth/...)                      │
│    ││    M  session.go        +24 −9 │   42   42 │                                                                            │   internal/auth   0.412s                         │
│    ││    M  session_test.go   +88 −4 │   43   43 │────────────────────────────────────────────────────────────────────────────│                                                  │
│    ││  ✓ R  token.go           +3 −3 │▌  44      │                                                                            │e(src/auth/cache.go)                              │
│    ││                                │▌       44 │ SESSIONS                                                                   │eated with 41 lines                               │
│    ││  src/worker/                   │▌  45   45 │▌ New session                                                         ⌃\ n  │                                                  │
│    ││    M  refresh.go         +5 −5 │▌  46      │  New session from spec task                                                │ cache is in place and the auth tests pass.       │
│    ││                                │▌       46 │                                                                            │e I update the callers in api/ and worker/:       │
│    ││  docs/                         │           │ SPECS                                                                      │d the cache be per-user or global? I raised it as │
│    ││    M  auth.md           +12 −1 │           │  New spec from an idea                                                     │1.                                                │
│    ││                                │           │                                                                            │                                                  │
│    ││                                │           │ INSIGHTS                                                                   │ing callers… (esc to interrupt)                   │
│    ││                                │           │  Jump to newest card                                                 ⌃\ 1  │                                                  │
│    ││                                │   47   47 │                                                                            │                                                  │
│    ││                                │        48 │ RECENT                                                                     │                                                  │
│    ││                                │   48   49 │  Network: allow host for this session                                      │                                                  │
│    ││                                │   49   50 │                                                                            │                                                  │
│    ││                                │   50   51 │────────────────────────────────────────────────────────────────────────────│                                                  │
│    ││                                │           │ ↑ ↓ move   ↵ run   tab complete   / slash commands                         │                                                  │
│    ││                                │           ╰────────────────────────────────────────────────────────────────────────────╯                                                  │
│    ││                                │   93   95       return s.client.Close()                                       ││                                                          │
│    ││                                │   94   96   }                                                                 ││                                                          │
│    ││                                │   95   97                                                                     ││                                                          │
│    ││                                │        98 + // tokenValid reports whether the token outlives the refresh wind…││                                                          │
│    ││                                │        99 + func (s *Session) tokenValid(now time.Time) bool {                ││                                                          │
│    ││                                │       100 +     return s.token != "" && s.expiry.Sub(now) > refreshWindow     ││                                                          │
│    ││                                │       101 + }                                                                 ││                                                          │
│    ││                                │   96  102                                                                     ││                                                          │
│    ││                                │                                                                               ││                                                          │
│    ││                                │───────────── end of file  ·  next  src/auth/session_test.go   ]  ─────────────││                                                          │
│    ││                                │                                                                               ││                                                          │
│    ││                                │                                                                               ││                                                          │
│    ││                                │                               ▌                                               ││                                                          │
│    ││                                │                               ▌ ✓  Comment queued for auth-refactor           ││                                                          │
│    ││                                │                               ▌    2 queued · sent at the agent's next turn   ││ ──────────────────────────────────────────────────────── │
│    ││                                │                               ▌    ⌃s send now   u undo                       ││ > Try "update the callers in api/"                       │
│    ││  [ prev   ] next   v reviewed  │                               ▌                                               ││ ──────────────────────────────────────────────────────── │
│    ││                                │                                                                               ││   ? for shortcuts                                        │
╰────╯╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯╰─ esc interrupt   ⌃\ z zoom ──────────────────────────────╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                                                   ● 1 approval · api-pagination    ⌃\ p palette   ⌃\ n new   ? keys
```

**110 × 32**

```
╭────╮╭─  Overview 3   Diff 7   Code   Specs   Term  ───────────────────────────────── unified · 7 files ─╮╭─╮
│    ││▌ src/auth/session.go  ·  3 of 7                                modified   +24 −9   ○ not reviewed ││⠹│
│  ⠹ ││  f files   [ prev file   ] next file   n next change   c comment                                  ││ │
│  3 ││           @@ -38,13 +38,15 @@  func (s *Session) Refresh(ctx context.Context) error               ││ │
│    ││   38   3╭────────────────────────────────────────────────────────────────────────────╮            ││ │
│  ● ││   39   3│                                                                            │            ││ │
│  1 ││   40   4│ › new                                                            esc close │            ││ │
│    ││   41    │                                                                            │            ││ │
│  ✓ ││        4│────────────────────────────────────────────────────────────────────────────│            ││ │
│    ││   42   4│                                                                            │            ││ │
│    ││   43   4│ SESSIONS                                                                   │            ││ │
│    ││▌  44    │▌ New session                                                         ⌃\ n  │            ││ │
│    ││▌       4│  New session from spec task                                                │            ││ │
│    ││▌  45   4│                                                                            │            ││ │
│    ││▌  46    │ SPECS                                                                      │            ││ │
│    ││▌       4│  New spec from an idea                                                     │            ││ │
│    ││         │                                                                            │    L44–L46 ││ │
│    ││         │ INSIGHTS                                                                   │path runs   ││ │
│    ││         │  Jump to newest card                                                 ⌃\ 1  │            ││ │
│    ││         │                                                                            │            ││ │
│    ││   47   4│ RECENT                                                                     │            ││ │
│    ││        4│  Network: allow host for this session                                      │            ││ │
│    ││   48   4│                                                                            │            ││ │
│    ││   49   5│────────────────────────────────────────────────────────────────────────────│            ││ │
│    ││   50   5│ ↑ ↓ move   ↵ run   tab complete   / slash commands                         │            ││ │
│    ││         ╰────────────────────────────────────────────────────────────────────────────╯            ││ │
│    ││           @@ -93,6 +95,14 @@  func (s *Session) Close() error                                     ││ │
│    ││   93   95       return s.client.Close()                                                           ││ │
│    ││   94   96   }                                                                                     ││ │
│    ││   95   97                                                                                         ││ │
╰────╯╰───────────────────────────────────────────────────────────────────────────────────────────────────╯╰─╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                ● 1 approval · api-pagination
```

### Empty state

First run in a workspace. The layout is already the real one, so the user learns it: sidebar with sections, main view, agent panel. The main view welcomes with a letter-spaced wordmark, one sentence on what a session is, and the three keys that matter as key caps.

**180 × 48**

```
╭─ multi-plex ──── ⎇ main ─╮╭─  Welcome  ──────────────────────────────────────────────────────────────────────────────╮╭─ Agent ──────────────────────────────────────────────────╮
│                          ││                                                                                          ││                                                          │
│ SESSIONS                 ││                                                                                          ││                                                          │
│   none yet               ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│ SPECS                  1 ││                                                                                          ││                                                          │
│   token-refresh ━─── 3/7 ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│ STEERING                 ││                                                                                          ││                                                          │
│   AGENTS.md              ││                                                                                          ││                                                          │
│   CLAUDE.md              ││                                                                                          ││                                                          │
│   ~/.claude/CLAUDE.md    ││                                                                                          ││                                                          │
│   .multiplex/            ││                                                                                          ││                                                          │
│                          ││                                    m u l t i p l e x                                     ││                                                          │
│ ROLES                  4 ││                                                                                          ││                                                          │
│ SKILLS                 6 ││                                     No sessions yet                                      ││                                                          │
│ MCP                    2 ││                                                                                          ││                                                          │
│                          ││                    A session is one agent working on one task in its                     ││                                                          │
│                          ││                        own worktree and branch. Start one and its                        ││       The agent's own terminal appears here when a       │
│                          ││                         summary, diff and terminal appear here.                          ││                     session starts.                      │
│                          ││                                                                                          ││                                                          │
│                          ││                                     n   new session                                      ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                  ⌃\ p   command palette                                  ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                     ?   all keys                                         ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
╰──────────────────────────╯╰──────────────────────────────────────────────────────────────────────────────────────────╯╰──────────────────────────────────────────────────────────╯
 multi-plex   ⎇ main   no sessions                                                                                                                 ⌃\ p palette   ⌃\ n new   ? keys
```

**110 × 32**

```
╭────╮╭─  Welcome  ───────────────────────────────────────────────────────────────────────────────────────╮╭─╮
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                         m u l t i p l e x                                         ││ │
│    ││                                                                                                   ││ │
│    ││                                          No sessions yet                                          ││ │
│    ││                                                                                                   ││ │
│    ││                         A session is one agent working on one task in its                         ││ │
│    ││                            own worktree and branch. Start one and its                             ││ │
│    ││                              summary, diff and terminal appear here.                              ││ │
│    ││                                                                                                   ││ │
│    ││                                          n   new session                                          ││ │
│    ││                                                                                                   ││ │
│    ││                                       ⌃\ p   command palette                                      ││ │
│    ││                                                                                                   ││ │
│    ││                                          ?   all keys                                             ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
│    ││                                                                                                   ││ │
╰────╯╰───────────────────────────────────────────────────────────────────────────────────────────────────╯╰─╯
 multi-plex   ⎇ main   no sessions                                           ⌃\ p palette   ⌃\ n new   ? keys
```

### New session dialog

`⌃\ n` or `n`. Task (multi-line, focused: strong label, blue bar, cursor; `⌃t` picks a spec task instead), role as a segmented choice with a one-line description of the selected role, base branch, and a read-only preview of the branch and worktree that will be created plus what setup will run. Inputs are inset wells on `Surface.Sunken`. **Start session** is `⌃↵` because `↵` inserts a new line in the task.

**180 × 48**

```
╭─ multi-plex ──── ⎇ main ─╮╭─  Overview 3   Diff 7   Code   Specs   Term  ────────────────────────── updated 32s ago ─╮╭─ ⠹ auth-refactor ─────────────────────────────── claude ─╮
│                          ││                                                                                          ││                                                          │
│ SESSIONS               3 ││  Cache auth tokens per user and update the callers                                       ││ ● Read(src/auth/session.go)                              │
│  ⠹ auth-refactor       3 ││  ⠹ working  ·  implementer  ·  ⎇ mpx/auth-refactor  ·  turn 6 · 14m                      ││   ⎿  Read 182 lines                                      │
│    implementer · 14m     ││                                                                                          ││                                                          │
│  ● api-pagination      1 ││  SUMMARY                                                                  report 6 of 6  ││ ● Update(src/auth/session.go)                            │
│    needs approval        ││  Done        ✓ Added token cache to Session.Refresh                                      ││   ⎿  Updated with 24 additions and 9 removals            │
│  ✓ docs-sweep            ││              ✓ Moved the expiry check into tokenValid()                                  ││                                                          │
│    done · 6 files        ││  Decided     Cache TTL is 5 minutes, matching the token lifetime                         ││ ● Bash(go test ./internal/auth/...)                      │
│                          ││  Open        ? Should the cache be per-user or global?                                   ││   ⎿  ok   internal/auth   0.412s                         │
│ SPECS                  1 ││  Next        Updat╭──────────────────────────────────────────────────────────────────────────────────╮                                               │
│   token-refresh ━─── 3/7 ││  Files       src/a│                                                                                  │rc/auth/cache.go)                              │
│                          ││                   │  New session                                                         multi-plex  │ed with 41 lines                               │
│ STEERING                 ││  INSIGHTS         │                                                                                  │                                               │
│   AGENTS.md              ││  ▌  1 DECISION NEE│  Task                                                      from a spec task: ⌃t  │che is in place and the auth tests pass.       │
│   CLAUDE.md              ││  ▌    A per-user c│  ▎ Make the token cache per-user and update the callers in api/ and worker/.     │ update the callers in api/ and worker/:       │
│   ~/.claude/CLAUDE.md    ││  ▌    would share │  ▎ Keep the public Session API unchanged.                                        │he cache be per-user or global? I raised it as │
│   .multiplex/            ││  ▌    src/auth/cac│  ▎                                                                               │                                               │
│                          ││  ▌    a ask   s st│                                                                                  │                                               │
│ ROLES                  4 ││                   │  Role                                                                            │ callers… (esc to interrupt)                   │
│ SKILLS                 6 ││  ▌  2 WARNING  Cha│   implementer   planner   reviewer   resolver                                    │                                               │
│ MCP                    2 ││  ▌    Refresh now │   Writes code against the task and runs the tests before it finishes.            │                                               │
│                          ││  ▌    api-paginati│                                                                                  │                                               │
│                          ││  ▌    src/auth/ses│  Base branch                             Branch and worktree                     │                                               │
│                          ││  ▌    a ask   s st│  ▎ main  ▾                               ⎇ mpx/token-cache-per-user              │                                               │
│                          ││                   │                                                                                  │                                               │
│                          ││  ▌  3 INFO  Assume│  Setup runs in the background: .env.example, pnpm install, ports 4100–4109.      │                                               │
│                          ││  ▌    Matched the │                                                                                  │                                               │
│                          ││  ▌    AUTH_CACHE_T│   Start session  ⌃↵    Cancel  esc                               tab next field  │                                               │
│                          ││  ▌    src/auth/cac│                                                                                  │                                               │
│                          ││  ▌    a ask   s st╰──────────────────────────────────────────────────────────────────────────────────╯                                               │
│                          ││                                                                                          ││                                                          │
│                          ││  CONFLICTS                                                                               ││                                                          │
│                          ││  ● Will conflict with api-pagination in src/api/client.go   merge-tree, checked 1m ago   ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││  SPEC                                                                                    ││                                                          │
│                          ││  token-refresh   ━━━━━━━━━━────────────── 3/7  tasks   5/9 criteria verified             ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││                                                          │
│                          ││                                                                                          ││ ──────────────────────────────────────────────────────── │
│                          ││                                                                                          ││ > Try "update the callers in api/"                       │
│                          ││                                                                                          ││ ──────────────────────────────────────────────────────── │
│                          ││                                                                                          ││   ? for shortcuts                                        │
╰──────────────────────────╯╰──────────────────────────────────────────────────────────────────────────────────────────╯╰─ esc interrupt   ⌃\ z zoom ──────────────────────────────╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                                                   ● 1 approval · api-pagination    ⌃\ p palette   ⌃\ n new   ? keys
```

**110 × 32**

```
╭────╮╭─  Overview 3   Diff 7   Code   Specs   Term  ─────────────────────────────────── updated 32s ago ─╮╭─╮
│    ││                                                                                                   ││⠹│
│  ⠹ ││  Cache auth tokens per user and update the callers                                                ││ │
│  3 ││  ⠹ working  ·  implementer  ·  ⎇ mpx/auth-refactor  ·  turn 6 · 14m                               ││ │
│    ││      ╭──────────────────────────────────────────────────────────────────────────────────╮         ││ │
│  ● ││  SUMM│                                                                                  │ 6 of 6  ││ │
│  1 ││  Done│  New session                                                         multi-plex  │         ││ │
│    ││  Open│                                                                                  │         ││ │
│  ✓ ││  Next│  Task                                                      from a spec task: ⌃t  │         ││ │
│    ││  File│  ▎ Make the token cache per-user and update the callers in api/ and worker/.     │         ││ │
│    ││      │  ▎ Keep the public Session API unchanged.                                        │         ││ │
│    ││  INSI│  ▎                                                                               │  2 new  ││ │
│    ││  ▌  1│                                                                                  │    2m   ││ │
│    ││  ▌   │  Role                                                                            │ share   ││ │
│    ││  ▌   │   implementer   planner   reviewer   resolver                                    │ you…    ││ │
│    ││  ▌   │   Writes code against the task and runs the tests before it finishes.            │         ││ │
│    ││  ▌   │                                                                                  │         ││ │
│    ││      │  Base branch                             Branch and worktree                     │         ││ │
│    ││  ▌  2│  ▎ main  ▾                               ⎇ mpx/token-cache-per-user              │    6m   ││ │
│    ││  ▌   │                                                                                  │         ││ │
│    ││  ▌   │  Setup runs in the background: .env.example, pnpm install, ports 4100–4109.      │         ││ │
│    ││  ▌   │                                                                                  │         ││ │
│    ││  ▌   │   Start session  ⌃↵    Cancel  esc                               tab next field  │         ││ │
│    ││     ↓│                                                                                  │         ││ │
│    ││      ╰──────────────────────────────────────────────────────────────────────────────────╯         ││ │
│    ││  CONFLICTS                                                                                        ││ │
│    ││  ● Will conflict with api-pagination in src/api/client.go   merge-tree, checked 1m ago            ││ │
│    ││                                                                                                   ││ │
│    ││  SPEC                                                                                             ││ │
│    ││  token-refresh   ━━━━━━━━━━────────────── 3/7  tasks   5/9 criteria verified                      ││ │
╰────╯╰───────────────────────────────────────────────────────────────────────────────────────────────────╯╰─╯
 multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                ● 1 approval · api-pagination
```

## Components

All of these are `kit` primitives; the mockups above are built only from them.

### Status bar

One row on `Surface.Sunken`, full width. Left, in priority order: workspace (strong, bold), `› session`, state badge, `⎇ branch` (muted). Right: attention pills (amber, dark text, only when something waits for you), then key hints. Segments are separated by three spaces, not glyphs. As the terminal narrows it drops, in order: key hints, then left segments from the end (branch first, then state), then right segments; the workspace and the approvals pill are the last to go.

```
   multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor                              ⌃\ p palette   ⌃\ n new   ? keys

   multi-plex   › auth-refactor   ⠹ working   ⎇ mpx/auth-refactor             ● 2 approvals    ⌃\ p palette   ⌃\ n new   ? keys

   multi-plex   › auth-refactor   ⠹ working                        ● 2 approvals
```

### Agent panel frame

Title is the session's state dot and name; meta is the agent CLI; the bottom edge carries the panel's own hints (`esc interrupt` is the CLI's key, shown because interrupting is only possible from here). Focused: blue frame, strong bold title. Unfocused: grey frame, secondary title. Content is the agent CLI's real terminal, edge to edge, backfilled onto `Surface.Base`.

```
  ╭─ ⠹ auth-refactor ─────────────────────────────── claude ─╮  ╭─ ⠹ auth-refactor ─────────────────────────────── claude ─╮
  │                                                          │  │                                                          │
  │ ● Token cache is in place and the auth tests pass.       │  │ ● Token cache is in place and the auth tests pass.       │
  │   Before I update the callers in api/ and worker/:       │  │   Before I update the callers in api/ and worker/:       │
  │   should the cache be per-user or global? I raised it as │  │   should the cache be per-user or global? I raised it as │
  │   card 1.                                                │  │   card 1.                                                │
  ╰─ esc interrupt ──────────────────────────────────────────╯  ╰─ esc interrupt ──────────────────────────────────────────╯
```

Collapsed (terminal narrower than 138 columns, see Responsive rules), the panel is a 3-column rail with the state dot at the top; `⌃\ a` opens it as a 60-column drawer over the right of the main view, focused.

### State badges, tabs, buttons, keys

```
  ◌ starting     ⠹ working     ● blocked     ✓ done     ○ idle     ✕ error

   Overview 3   Diff 7   Code   Specs   Term        Approve once  ↵    Deny  esc    Cancel  esc
  ⌃\ p palette   ⇧tab back   ↵ open   ↑ ↓ move       n   ⌃\ p       ━━━━━━━━──────────── 3/7
```

- **State dot / badge:** dot in the state colour; the label is `Text.Secondary` for quiet states and the state colour for loud ones (blocked, error).
- **Tabs:** every tab has one cell of padding each side, so changing the active tab never moves the others. Active: `Accent.Subtle` pill, strong bold text. Badges (counts) are faint, amber when something there needs you.
- **Buttons:** ` Label  key `. Primary (one per dialog) on `Accent.Fill` with white text; secondary on `Surface.Control`; ghost is text only. The key is always part of the button.
- **Key caps** (`Keycap`) for the empty state and help; inline **key hints** (`KeyHint`) everywhere else.
- **Progress:** a thin line, accent fill on a `Border.Default` track, optional `3/7`.

### Toasts

Bottom-right of the main view, one row above the status bar, 46 columns, on `Surface.Overlay` with a tone bar on the left and one blank row of padding above and below. Title strong bold, detail secondary, optional actions as hints. Info and success toasts leave after 4 s (`Motion.ToastDuration`); warning and error toasts stay until dismissed or acted on. At most three stack upward; a new one pushes the oldest out.

```
  ▌                                         ▌                                         ▌
  ▌ ✓  3 comments sent                      ▌ ●  api-pagination needs approval        ▌ ✕  docs-sweep exited (code 1)
  ▌    to auth-refactor · src/auth/sessi…   ▌    ⌃\ ! go there                        ▌    agent CLI crashed · restart with r
  ▌                                         ▌                                         ▌
```

### Cards

A severity bar (`▌`) down the left edge, then: number (muted, right-aligned in 2 cells), severity label (UPPER CASE, severity colour, bold), title (bold when unread), age (faint, right). Body: secondary, wrapped, clamped to 2 lines with an ellipsis, so cards never change height. Refs in accent. Actions as key hints, bright on the selected card and faint on the others (they keep their row, so selection never shifts the list). The selected card sits on `Surface.Raised`. Number keys `1`–`9` select a card.

### Diff rows

`DiffLine` renders: marker column (accent `▌` for selected lines, thin accent `▎` for new lines with reduced motion), old and new line numbers (4 cells each, muted), the sign in its colour, then the code on the line background with changed words on the word background. Hunk headers span the row on `Diff.HunkBg`. Folded regions are a band on `Surface.Sunken` with "··· N unchanged lines ···" at the code column and, when selected, `↵ expand`. Code never wraps; long lines end in `…` and horizontal scroll is a key away. (The opposite holds in the approval prompt: the command is wrapped, never cut, because it is what you are approving.)

The file banner (`Banner`) is the sticky first row of the diff: the directory in secondary, the file name strong bold, `· 3 of 7`, then status, counts and reviewed state on the right. The end-of-file marker (`FileEnd`) is a centred rule with "end of file · next path ]" and never scrolls into the next file.

Inline comments (`CommentBox`) sit directly under the last selected line, indented to the code column, on `Surface.Raised` with a thin bar (accent while editing, faint otherwise): author, status (draft muted, queued blue, sent green, outdated amber) and line range, then the body, then, while editing, its keys.

## Motion

All timings live in `theme.Motion` and are switched off together by reduce motion.

| Motion | Spec | Reduced motion |
|---|---|---|
| **File switch banner** | On switching files the banner's background starts at `Accent.Subtle` and its bar at `Accent.Base`, then eases to `Surface.Raised` and `Text.Faint` over `BannerHold` = 600 ms (`Banner.Settle` 0→1, ease-out cubic, OKLab interpolation via `theme.Blend`). | Banner drawn at rest immediately. The path, "3 of 7" and the content change are the signal. |
| **Content fade** | The diff body below the banner fades in from `Surface.Base` over `FadeDuration` = 150 ms in `FadeSteps` = 9 frames (one per 16.7 ms, 60 fps). Frame *i* uses amount `1 − easeOutCubic(i/9)` in `kit.Dim(t, body, t.Surface.Base, amount)`; colours interpolate in OKLab so mid-tones do not go muddy. Frame 0 (amount 1) is drawn in the same update as the switch, so the switch itself stays within the one-frame budget ([03](03-performance.md)); frames 1–9 follow on ticks, frame 9 being the settled content. | No fade. |
| **New hunk highlight** | When a live update adds or changes a hunk, its rows start at `Diff.NewHunkBg` and decay to their normal background over `HunkDecay` = 1.2 s in `HunkDecaySteps` = 8 steps (every 150 ms, `DiffLine.Highlight` 1→0, ease-out). Only the affected rows redraw. | A thin accent bar `▎` in the marker column until the file is next scrolled or switched. |
| **Spinner** | Working sessions show the braille spinner, one frame per `SpinnerInterval` = 80 ms (12.5 fps). Only the spinner cells redraw; all sessions share one tick. When the terminal window loses focus the tick stops. | Static `◐`. |
| **Toasts** | Appear and leave without animation (sliding would mean redrawing the main view each frame); timing only. | Same. |

Animations run only when `t.Animated()` is true: truecolor and reduce motion off. In 256 colours intermediate shades would snap to the same index, so the static forms are used. Animation never delays input: a key pressed mid-fade finishes the fade instantly and is handled in the same frame.

## Responsive rules

Widths are in columns. Height below the regions is always one status bar row.

| Terminal width | Sidebar | Main view | Agent panel | Diff file rail |
|---|---|---|---|---|
| ≥ 176 (e.g. 180) | full, 28 (icon rail in Diff mode) | rest (≥ 88) | 30%, min 60 | shown from 178 columns |
| 138–175 | icon rail, 6 | rest (≥ 72) | 30%, min 60 | collapsed |
| 80–137 (e.g. 110) | icon rail, 6 | rest | rail, 3 (opens as a 60-column drawer) | shown from 121 columns, else collapsed (banner + `f files`) |
| < 80 or < 24 rows | — | "Terminal too small" message with the current and minimum size | — | — |

What collapses first, in order:

1. **Diff mode takes the sidebar's width.** In Diff mode the sidebar shows as its icon rail at any width (reading code wins). It can be pinned open in settings.
2. **File rail** (inside Diff) collapses when the main view is narrower than 112 columns. The banner keeps "3 of 7", `[`/`]` still move between files, and `f` opens the file list as an overlay.
3. **Sidebar → icon rail** (6 columns: state dot and badge per session) when keeping it would leave the main view under 88 columns.
4. **Agent panel → rail** when keeping it at 60 columns (its minimum, [06](06-ui-layout.md)) would leave the main view under 72 columns. It never shrinks below 60: it becomes a drawer instead.
5. Inside regions: summaries go compact (one "Done" line), card lists show what fits plus "↓ N more cards", status bar segments drop as described above. Text truncates with `…`; paths truncate from the left so the file name stays (`…/auth/session.go`).

Side-by-side diff needs a main view of at least 200 columns ([07](07-diff-viewer.md)); below that the toggle is disabled with a hint.

Minimum supported size: **80 × 24**.

## Fallbacks and terminals

- **Colour profile:** detected by Bubble Tea; `theme.For(profile)` picks truecolor or the designed 256 palette. 16-colour terminals get the 256 indexes reduced by the writer: usable, not designed. `NO_COLOR` is honoured by the writer (text and glyphs only; selection falls back to the marker bar).
- **Glyphs:** Unicode by default. The ASCII set keeps every width, so the layout is identical.
- **Ambiguous-width characters:** the glyph set avoids emoji and characters with emoji presentation. `●`, `○`, `·` and `…` are East Asian Ambiguous; terminals configured to draw ambiguous characters wide (some CJK setups) should use the ASCII set.
- **Background:** always painted, so light-background terminals still show the dark theme correctly. A light theme is a separate theme file, not an automatic inversion.
- **Box drawing:** frames use light rounded corners. Terminals draw them edge to edge; there is no gap between rows.

## Rendering cost

The look has to fit the frame budget in [03](03-performance.md) (16 ms per frame, and a spinner tick should cost well under 2 ms). The kit is built for that:

- Widths are measured with a fast path that gives exactly `ansi.StringWidth`'s answer for printable ASCII, CSI/OSC sequences and runes that always stand alone (box drawing, symbols, Latin, Greek, Cyrillic, CJK). Anything else (combining marks, joiners, emoji sequences, complex scripts) goes through the full grapheme segmenter. A test checks every BMP rune and random mixes against `ansi.StringWidth`; truncation is checked the same way against `ansi.Truncate`.
- `Backfill` and `Dim` rewrite a whole block in one pass, and SGR sequences for theme colours are built once and reused.
- Measured on the CI-class Linux machine (`go test -bench` in `internal/ui/kit`, `internal/ui/app`, `internal/ui/views/diffview`): a 92 × 47 panel ≈ 0.1 ms, one diff row ≈ 10 µs, a full 180 × 48 frame ≈ 0.7 ms, an agent-output frame ≈ 0.2 ms, a cold 10k-line diff frame ≈ 1.2 ms.

Screens keep their own caches on top (unchanged regions are not re-rendered), so most frames redraw one region.

## Open decisions

1. **Prefix key.** Decided: `Ctrl+\` (shown `⌃\`), free in Claude Code, opencode, Codex, Windows Terminal, tmux and macOS ([15](15-open-questions.md) Q2); `Ctrl+Space` is the configured alternative. Terminal delivery is still to be checked by hand in each target terminal. Hints are key specs, so a user keymap changes every hint at once.
2. **Mac key glyphs on Windows and Linux.** `⌃ ⇧ ⌥` are compact and consistent, but Windows users may read `ctrl` faster. Option: a `keys = "words"` glyph setting (`ctrl+\ p`). Needs the author's call.
3. **Diff mode collapsing the sidebar** at every width. It buys about 22 columns of code at 180 columns. Alternative: collapse only below 200 columns.
4. **Agent panel as a drawer below 138 columns.** Keeps the 60-column minimum from [06](06-ui-layout.md) but hides the agent by default on small terminals. Alternative: swap main view and agent panel with one key (zoom) instead of a drawer.
5. **Painted background vs the terminal's own.** Painting `#1E1E1E` everywhere gives a consistent look but ignores the user's terminal background and transparency. Option: a `background = "terminal"` setting that leaves `Surface.Base` unpainted.
6. **Light theme.** Not designed yet. The token set is ready for it (a `LightPlus` file registering itself).
