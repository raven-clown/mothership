<!--
Title: use Conventional Commits, such as `fix(server): keep multiline SSE data`. It becomes the commit on main.
Workflows from a fork start once a maintainer approves them, so the checks can show as waiting for a while.
-->

Closes #

## What and why

<!-- What changes for users or callers, and the problem it solves. -->

## How I tested it

<!-- The tests you added and anything you ran by hand. -->

## What I didn't test

<!-- Platforms, providers or setups you couldn't try. "Nothing" is a fine answer. -->

## Checklist

- [ ] Tests cover the change, and a bug fix has a test that fails without it
- [ ] `make check` passes (lint, tests and the 80% coverage check)
- [ ] `CHANGELOG.md` has a line under Unreleased, if users will notice the change
- [ ] README and docs are updated, if usage, config or commands changed
- [ ] The OpenAPI file is updated, if the admin API changed
- [ ] A schema change comes with a migration, and existing data still loads
- [ ] Nothing breaks for current users, or the description says what breaks and why
