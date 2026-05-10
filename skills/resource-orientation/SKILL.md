---
name: resource-orientation
description: Use when the user says they are starting, about to begin, or opening a specific resource — paper, book chapter, lecture, or video. Triggers on phrases like "I'm starting X", "I'm about to read X", "beginning X now".
---

# Resource Orientation

## Overview

Before the user engages with a resource, provide a structured orientation: how to read it, what matters most (specific sections when findable), what to hold in mind, and how to take notes effectively.

## Trigger Signals

- "I'm starting [resource]"
- "I'm about to read / watch / go through [resource]"
- "Beginning [resource] now"
- User names a specific paper, chapter, or lecture for the first time

## Process

```dot
digraph orientation {
    "Resource named" [shape=doublecircle];
    "Research resource structure" [shape=box];
    "Load course profile" [shape=box];
    "Structure found?" [shape=diamond];
    "Use sections" [shape=box];
    "Use concepts" [shape=box];
    "Build orientation" [shape=box];
    "Output to user" [shape=doublecircle];

    "Resource named" -> "Research resource structure";
    "Resource named" -> "Load course profile";
    "Research resource structure" -> "Structure found?";
    "Structure found?" -> "Use sections" [label="yes"];
    "Structure found?" -> "Use concepts" [label="no"];
    "Use sections" -> "Build orientation";
    "Use concepts" -> "Build orientation";
    "Load course profile" -> "Build orientation";
    "Build orientation" -> "Output to user";
}
```

### Step 1 — Research the resource

Use `claw-cli web fetch <url>` or `bash curl <url>` to find:
- Abstract, table of contents, or section headers
- Author's stated purpose or framing
- How the community uses this resource (survey vs. deep read vs. reference)

If the resource is a book chapter, search for the chapter name + author + "summary" or "overview". If it's a paper, fetch the abstract and section headings from the PDF or a freely available source.

### Step 2 — Load course objectives

The course profile is already available in AGENTS.md, which is auto-loaded in the sandbox. Read it from context. Identify:
- The **learning lens** (e.g., formal/theoretical framing)
- The **syllabus topics** this resource serves
- **Specific directives** for this type of content (e.g., "treat safety constraints as formal invariants")
- **Avoid list** (e.g., no ODE framing, no numerical methods as primary output)

### Step 3 — Build the orientation

Produce the following sections:

#### How to approach this resource
Match the reading strategy to the resource type:
- **Taxonomy paper** (like Laprie): read for definitions, build a concept map, annotate distinctions
- **Engineering book chapter**: skim structure first, then read for formal underpinnings, skip heuristics without formal basis
- **Lecture/video**: pause after each claim and restate it formally before continuing

#### Most important sections / concepts
Be as specific as the research allows:
- If sections are known: name them, say what to focus on and what to skim
- If not: name the 3–5 core concepts, explain what precision matters and why
- Filter all of this through the course lens from Step 2

#### Things to hold in mind
2–4 orienting thoughts tied to course objectives. These are conceptual anchors — the ideas that should stay active while reading.

#### What's important for notes
Identify what from this resource is worth capturing later — don't prescribe how to take notes (that belongs to the note-taking skill). Two lenses:
- **For the course:** What concepts, arguments, or claims from this resource are load-bearing for later syllabus topics? What will reappear or be built upon?
- **For the user's interests:** What ideas connect to their broader intellectual threads (formal foundations, thesis directions, cross-domain links)? What's worth sitting with beyond the course requirements?

## Output Format

Use clear section headers. Keep the whole output under ~400 words — dense but scannable. Lead with the most important thing first (usually the approach or the key section list).

## Common Mistakes

- **Too generic:** "read carefully and take notes" is useless. Name sections, name concepts, name the exercise.
- **Ignoring the lens:** always filter through the user's theoretical framing, not the author's intended audience.
- **Adding exercises:** exercises belong in the study plan, not in orientation. Don't include focus exercises or "how to use Claude" sections.
- **Skipping research:** if you don't look up the resource, you'll miss section names and produce vague guidance. Always search first.
