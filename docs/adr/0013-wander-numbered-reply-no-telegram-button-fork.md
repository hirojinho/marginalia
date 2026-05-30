# 0013 — Wander navigates by numbered reply; no NanoClaw fork for Telegram buttons

- **Status:** Accepted
- **Date:** 2026-05-29

## Context

*Wander* (see [CONTEXT.md](../../CONTEXT.md)) delivers leisure-reading
rabbit-hole cards over Telegram via Claw. The preferred UX was tap-able inline
buttons under each card (one tap to follow a thread).

NanoClaw's Telegram channel can't do that. The agent emits **plain text**, which
the framework `chat-sdk` relays via a bare `sendMessage`; the channel exposes no
`reply_markup` on send and polls no `callback_query` updates (verified — zero
matches for `inline_keyboard|callback_query|reply_markup` in `src/`, and the
real send/receive lives in the upstream framework package, not our fork file).
Real buttons would require **forking the upstream NanoClaw dependency** to (a)
attach `reply_markup` on send and (b) poll `callback_query` and route taps back
into a freshly-spawned per-message container with the right thread context —
high effort, ongoing maintenance (we already carry one provider patch), and a
genuine callback→container state-routing problem.

## Decision

Each Wander card ends with a **numbered text menu** ("Reply 1–3 to go deeper · w
for Wikipedia · or just ask anything"). The learner replies with a number/word,
which flows through the **existing** message path unchanged — no framework fork.

## Consequences

- Cost vs a button: one extra tap (text field + a digit). In exchange: zero
  modification to a third-party runtime, and more flexibility — a free-text
  question works identically to a numbered choice, which is the truest
  rabbit-hole behaviour anyway.
- If typing ever proves annoying, the cheapest upgrade is a **send-side reply
  keyboard** (the `keyboard` field, not `inline_keyboard`): a tapped reply-key
  arrives as a normal text message, so it still needs no `callback_query`
  handling — only a send-side patch. Inline buttons + callbacks remain the
  expensive last resort.
