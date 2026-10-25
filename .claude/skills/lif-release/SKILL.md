---
name: lif-release
description: Prepare, review, and publish LIF GitHub release notes from canonical wiki release pages. Use for release notes, GitHub releases, version tags, GoReleaser output, or external PR attribution.
---

# LIF Release Flow

`wiki/releases/vX-Y-Z.md` is canonical release prose. GitHub receives its Markdown body after Astro frontmatter. Do not maintain two summaries.

## Boundaries

- GoReleaser creates artifacts and initial GitHub release after a tag push.
- `scripts/release-github-notes.sh` only validates or updates an existing release body.
- Never create tags, push refs, run GoReleaser, or call `gh release edit` while drafting notes.
- Never publish without direct PO authorization at point of action.
- This workflow does not change the contents or timing of v0.17.0.

## Prepare release prose

1. Identify target milestone, target tag, latest release tag, merged containers, and linked PRs.
2. Read item `summary`/`goal`, container boundaries, user-facing docs, and relevant PR evidence. Use raw commits for audit only; do not turn every commit into release prose.
3. Create or update `wiki/releases/vX-Y-Z.md` with matching frontmatter `title: "vX.Y.Z"`.
4. Write for users: headline first; upgrade notes only when behavior requires action; grouped feature, fix, or limit sections; source links only when they help audit.
5. Use semantic container merges to find themes. Keep individual commits atomic in history.
6. Run `just release-notes-check vX.Y.Z`.

## External PRs

An external contribution is a PR authored outside LIF maintainers. Confirm from PR author and GitHub association; do not infer from a commit name alone.

For each external contribution, add an `## External contributions` entry with:

- contributor name,
- PR link,
- concise user-visible result.

Do not list upstream synchronization, Dependabot updates, or maintainer-authored PRs as external contributions. Put them in normal maintenance/fix prose when relevant. Omit this section when no external contribution ships.

## Publish order

1. PO approves release scope and tag action.
2. Run existing `just release <level>` flow. It tests, snapshots, tags, pushes, and watches GitHub Actions.
3. Confirm release workflow succeeded and GitHub release exists.
4. PO explicitly authorizes body publication for named tag.
5. Run `just release-notes-publish vX.Y.Z`.
6. Read GitHub release body and compare it with wiki Markdown body. Record mismatch as a release blocker.

`release-notes-publish` is consequential. Do not run it for checks. Use `just release-notes-check` for local validation.

## Completion check

- Wiki release page is tracked and reviewed.
- Frontmatter tag matches release tag.
- GitHub body equals Markdown body after frontmatter.
- External PR attribution is present only for verified external contributions.
- Release workflow is green.
