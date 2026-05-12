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
    "Fetch section content" [shape=box];
    "Load course profile" [shape=box];
    "Structure found?" [shape=diamond];
    "Content accessible?" [shape=diamond];
    "Use sections + content" [shape=box];
    "Use concepts only" [shape=box];
    "Build orientation" [shape=box];
    "Output to user" [shape=doublecircle];

    "Resource named" -> "Research resource structure";
    "Resource named" -> "Load course profile";
    "Research resource structure" -> "Structure found?";
    "Structure found?" -> "Fetch section content" [label="yes"];
    "Structure found?" -> "Use concepts only" [label="no"];
    "Fetch section content" -> "Content accessible?";
    "Content accessible?" -> "Use sections + content" [label="yes"];
    "Content accessible?" -> "Use concepts only" [label="no"];
    "Use sections + content" -> "Build orientation";
    "Use concepts only" -> "Build orientation";
    "Load course profile" -> "Build orientation";
    "Build orientation" -> "Output to user";
}
```

### Step 1 — Research the resource structure

Use `claw-cli web fetch <url>` or `bash curl <url>` to find:
- Abstract, table of contents, or section headers
- Author's stated purpose or framing
- How the community uses this resource (survey vs. deep read vs. reference)

If the resource is a book chapter, search for the chapter name + author + "summary" or "overview". If it's a paper, fetch the abstract and section headings from the PDF or a freely available source.

### Step 1b — Fetch the specific section content

Once you know the structure, fetch the actual content of the section the user is about to engage with:
- If it's a paper: fetch the specific section (Introduction, Methodology, etc.) from the PDF or an open-access version
- If it's a book chapter: search for the chapter content directly — try `claw-cli rag search <key terms>` against the corpus first, then fall back to `claw-cli web fetch`
- If it's a lecture/video: search for a transcript, slides, or summary of that specific lecture

Use the fetched content to identify concrete claims, definitions, or arguments in that section — not just what the section is about, but what it actually says. This is what makes the orientation specific rather than generic.

If the content is behind a paywall or otherwise inaccessible, proceed with structure only and note the limitation.

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
Ground this in the actual content fetched in Step 1b:
- Quote or paraphrase the key claims, definitions, or arguments from the section
- Say what to focus on and what to skim, with reference to specific content (not just section names)
- If section content was inaccessible, fall back to naming 3–5 core concepts and explaining what precision matters and why
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
- **Skipping research:** if you don't look up the resource, you'll miss section names and produce vague guidance. Always fetch structure first, then fetch section content.
- **Stopping at the TOC:** knowing that Section 3 exists is not enough. Fetch the actual content of the section the user is starting — the orientation should reference what the section says, not just what it's called.
