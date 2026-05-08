# Study App — Design Spec v1

## Identity

Japanese minimalism meets monochrome. Inspired by MUJI, Kenya Hara, and kissaten typography. Space is the design. No decorative elements — only what serves the content.

## Color

| Token | Hex | Usage |
|---|---|---|
| `--bg` | `#FAFAFA` | Page background (soft white, paper-like) |
| `--surface` | `#F5F5F5` | Cards, input areas, sidebars |
| `--border` | `#E0E0E0` | Subtle dividers between sections |
| `--text` | `#1A1A1A` | Body text (near-black) |
| `--text-dim` | `#888888` | Labels, timestamps, secondary |
| `--accent` | `#000000` | Interactive elements (send button, links) |
| `--accent-hover` | `#333333` | Hover state |
| `--msg-user` | `#1A1A1A` | User message bubble background |
| `--msg-user-text` | `#FFFFFF` | User message text |
| `--msg-ai` | `#FFFFFF` | AI message bubble background |
| `--msg-ai-border` | `#E0E0E0` | AI bubble border |
| `--error` | `#CC3333` | Error messages |

## Typography

- **Font:** `"Inter", system-ui, sans-serif` — clean, readable at all sizes
- **Scale:**
  - 11px / 0.6875rem — labels, timestamps (capitalized, letter-spaced)
  - 14px / 0.875rem — body text, chat messages
  - 18px / 1.125rem — section headings
  - 24px / 1.5rem — page title
- **Line height:** 1.6 for body, 1.3 for headings
- **Font weight:** 400 body, 500 medium labels, 600 semibold headings

## Spacing

- Base unit: 8px
- Chat padding: 24px horizontal, 16px vertical
- Between messages: 12px
- Input area padding: 16px 24px
- Bubble padding: 12px 16px

## Layout

```
┌──────────────────────────────────────┐
│ Header                               │
│  Study App            [menu? theme?] │
├──────────────────────────────────────┤
│                                      │
│  Messages area                       │
│  (flex-grow: 1, overflow-y: scroll)  │
│                                      │
│  ┌─────────────────────────┐         │
│  │ User message (right)    │         │
│  └─────────────────────────┘         │
│                                      │
│  ┌─────────────────────────┐         │
│  │ AI message  (left)      │         │
│  └─────────────────────────┘         │
│                                      │
├──────────────────────────────────────┤
│ [ Input field              ] [Send] │
└──────────────────────────────────────┘
```

- **Header:** 48px height, bottom border `1px solid var(--border)`
- **Messages:** fills remaining space
- **Input:** fixed at bottom, 64px height

## Components

### Chat Bubble (User)
- Background: `var(--msg-user)` (#1A1A1A)
- Text: `var(--msg-user-text)` (#FFFFFF)
- Border-radius: 12px (top-right: 4px)
- Max-width: 75%
- Align: right (self-end)
- Label: "You" — 11px caps, dim, above bubble

### Chat Bubble (AI)
- Background: `var(--msg-ai)` (#FFFFFF)
- Border: `1px solid var(--border)`
- Border-radius: 12px (top-left: 4px)
- Max-width: 85%
- Align: left (self-start)
- Label: "Claw" — 11px caps, dim, above bubble

### Input Area
- Background: `var(--surface)`
- Border-top: `1px solid var(--border)`
- Input field: border `1px solid var(--border)`, radius 8px, focus: `var(--accent)`
- Send button: `var(--accent)` bg, white text, radius 8px, hover: `var(--accent-hover)`

### Typography in Messages
- Paragraphs separated by 8px margin-bottom
- Code inline: background `#F0F0F0`, font `"JetBrains Mono", monospace`, size 13px
- Code blocks: same, with 12px padding, radius 8px
- Links: `var(--accent)`, underline on hover

### Scrollbar
- Thin (6px), track `transparent`, thumb `var(--border)`

## States

- **Loading:** none (streaming tokens ARE the loading state)
- **Empty state:** centered text "Ask me anything" in `var(--text-dim)`
- **Error:** bubble with `var(--error)` border, "Something went wrong" message
- **Sending:** send button disabled, opacity 0.5

## Future (not MVP)

- Tab bar for Chat / Plan / Notes / PDF
- Sidebar for navigation
- Theme toggle (spinoff with light invert)
- Responsive breakpoints (768px tablet, 480px mobile)
