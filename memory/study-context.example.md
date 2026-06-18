# Study Context

This file is a snapshot of the learner's academic profile, active courses, and study
preferences. The agent reads it as a briefing doc (not ground truth) when assembling a
session — treat it as background, and ask the learner when in doubt.

Replace the sections below with your own profile. Everything here is an example.

---

## Learner Profile

_Who is studying, their background, and what they're working toward. Example:_

> A graduate student bridging software engineering into a deeper area. Studies in limited,
> focused windows; intrinsically motivated.

---

## Active Courses & Study Tracks

_One subsection per course or track. For each, note the goal, key references, where the
canonical study plan lives, and where notes/interests are kept. Example:_

### Example Course
- Goal: what mastering this course should enable
- Primary references: textbook(s), lecture notes
- Study plan (canonical): `$VAULT_ROOT/data/plans/<course>.json` — read via
  `claw-cli plan status --course <course>`, update via `claw-cli plan toggle`
- Interests log: `$VAULT_ROOT/memory/courses/<course>/interests.md`

---

## Study Preferences (how to work with the learner)

_How the learner likes to be taught: orientation format, note-taking style, chunking,
how new concepts are introduced, and what to avoid. Fill in with your own preferences._

---

## Interest Logs

When the learner shows curiosity about something tangential, log it to the relevant
course's `interests.md` — do not add it to the active plan unless asked.
