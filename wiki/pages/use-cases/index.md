---
title: "Use cases"
description: "Real problems, solved with Lif."
---

## Purpose
Entry page of the Use cases chapter. It separates use case, feature and blog post, introduces the five scenarios and helps readers choose one.

## Audience
People who recognize their situation and want to find a fitting preset or structure. Also agents that recommend a scenario to a user.

## Required sections

### Introduction
- Explains: A use case shows a generally applicable solution to a recognizable problem; a feature explains a capability; a blog post shares a concrete experience or experiment.
- Stresses: Use cases explain a complete application, not every command one by one.
- Links to Features (`features`) and Reference (`reference`) for details.
- Note: preset names and hierarchies are examples, not a fixed requirement (D39).
- Source: CONCEPT.md §5 (Purpose, Recommended Content Pattern), §6 (Editorial Distinction); DESIGN.md D33, D39

### The five use cases
- One line per scenario: problem in one sentence, preset or structure, slug.
- Personal tasks (`use-cases/personal-tasks`): preset `todo` (one leaf type, grouping via tags) or `simple`.
- Home lab (`use-cases/homelab`): lean custom configuration with custom `types` (type names are a suggestion, `TODO: verify`).
- Software project (`use-cases/software-projects`): preset `classic` (milestone > epic > feature > bug/task).
- Release management (`use-cases/release-management`): preset `complex` (release, bucket, feature/improvement/chore, story/bug/task).
- Writing or research project (`use-cases/writing-research`): custom `types` for a non-technical example.
- Source: FACTS.md §9(b), §3; DESIGN.md D33, D42; type names for Home lab and Writing are suggestions → `TODO: verify` (build with `lif init`/`lif check` in phase 5)

### How to pick one
- Table: situation, preset or structure, page.
- Shows the four bundled presets `classic`, `todo`, `simple`, `complex` (selected with `lif init --profile`) and when a custom `types` set fits better.
- Explains: A preset is written out into the configuration at init, not referenced by name; it can be changed freely later.
- Links to Reference: Configuration (`reference/configuration`) as the canonical home of the presets.
- Source: FACTS.md §9(b), §4.3; DESIGN.md D39

### The common pattern
- Explains the seven sections of every use-case page: Problem, Solution, Structure, Workflow, Benefits, Limits, References.
- Structure shows the preset or `types` table plus hierarchy; Workflow is a concrete command loop; Limits states honestly what Lif does not do for the scenario.
- Note: Example configurations are built and checked with `lif init` and `lif check` before publication.
- Source: CONCEPT.md §5 (Recommended Content Pattern); DESIGN.md D33; the Limits section extends the seed pattern (skeleton decision)

### Experiments live on the Blog
- Explains: Experimental scenarios live in the blog with tag `experiment`, not under Use cases.
- Names Managing Lif items in Obsidian (`blog/obsidian-experiment`) as an example.
- Links to the blog overview (`blog`).
- Source: DESIGN.md D34, D42
