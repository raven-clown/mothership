# AGENTS.md

Instructions for coding agents working in this repository. Keep this file current: when a file's purpose or a rule changes, update it in the same commit.

## What this repository is

The shared home of airlock, rosetta, transponder and logbook. It holds no Go code yet. `CONTRACT.md` is the agreement between the four projects, and `docs/scale.md` explains its scale, limits and usage event rules in detail. `docs/assets/` holds the README art. `contributing/` is the kit each project copies to its own root: CONTRIBUTING.md, AGENTS.md template, issue and pull request templates, CI workflows, `.golangci.yml`, `.testcoverage.yml`, `Makefile`, `ruleset.json` and `scripts/setup-repo.sh`.

## Rules

- A change to `CONTRACT.md` affects all four projects. Say in the commit message what changed, and tell each project's maintainer.
- When `contributing/` changes, the four repositories need the same change.
- Action versions in the workflows are pinned to commit SHAs, with the tag in a comment. Keep both in step.
- Commit after every change, with a Conventional Commits message, and push it.
- Never mention AI, assistants or code generation in docs, commit messages, pull requests or issues. Never add `Co-Authored-By`, `Generated with` or similar trailers.
- No em-dashes.
