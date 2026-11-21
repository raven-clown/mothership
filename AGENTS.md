# AGENTS.md

Instructions for coding agents working in this repository. Keep this file current: when a file's purpose or a rule changes, update it in the same commit.

## What this repository is

The shared home of airlock, rosetta, transponder and logbook. `CONTRACT.md` is the agreement between the four projects, and `docs/scale.md` explains its scale, limits and usage event rules in detail. `docs/assets/` holds the README art. `contributing/` is the kit each project copies to its own root: CONTRIBUTING.md, AGENTS.md template, issue and pull request templates, CI workflows, `.golangci.yml`, `.testcoverage.yml`, `Makefile`, `ruleset.json` and `scripts/setup-repo.sh`.

## The stack and the end-to-end test

- `deploy/compose/compose.yaml` runs all four, plus Jaeger. `Dockerfile` next to it builds any project from its GitHub repository at a tag or branch (`PROJECT`, `REF`), since not every project publishes an image yet. The released tags are the defaults in `compose.yaml`; `AIRLOCK_REF`, `ROSETTA_REF`, `TRANSPONDER_REF` and `LOGBOOK_REF` override them.
- One YAML file per project sits next to it. Keys are public demo keys. `transponder-seed` runs transponder's own `sim seed` for the `tp_demo_*` keys, and `logbook-seed.sh` makes the logbook key airlock uses and keeps it in the `keys` volume.
- `e2e/` is its own Go module, built only with `-tags e2e`. It talks to the running stack over its localhost ports and reads airlock's audit records and logbook's request logs with `docker compose logs`.
- A step a project can't do yet is `t.Skip` with the link to that project's issue, kept in the `gap*` constants in `e2e/diagram_test.go`. When the issue closes, remove the skip in the same change that bumps the project's tag.
- `.github/workflows/e2e.yml` runs the stack and the test on pull requests, on main and every night. The nightly run also builds every project from main.

Commands:

```shell
cd deploy/compose && docker compose up --build --wait
cd e2e && go test -tags e2e -count=1 -v ./...
cd deploy/compose && docker compose down --volumes
```

When a project releases, bump its default tag in `compose.yaml` and in the README table if a port or key changed.

## Rules

- A change to `CONTRACT.md` affects all four projects. Say in the commit message what changed, and tell each project's maintainer.
- When `contributing/` changes, the four repositories need the same change.
- Action versions in the workflows are pinned to commit SHAs, with the tag in a comment. Keep both in step.
- Commit after every change, with a Conventional Commits message, and push it.
- Never mention AI, assistants or code generation in docs, commit messages, pull requests or issues. Never add `Co-Authored-By`, `Generated with` or similar trailers.
- No em-dashes.
