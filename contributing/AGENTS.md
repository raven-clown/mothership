# AGENTS.md

Instructions for coding agents working in this repository. Keep this file current: when a command, a package layout or a rule changes, update it in the same commit.

## Project

<one paragraph: what this project is and who uses it>

It is one of four projects described in the shared contract at https://github.com/raven-clown/mothership/blob/main/CONTRACT.md. Follow the contract. A change to how the projects connect goes to the contract first.

## Commands

```shell
go build ./...
go test -race ./...
golangci-lint run ./...
make check   # lint plus tests with the 80% coverage check
```

## Layout

<the main packages and what each one holds>

## Rules

- Follow CONTRIBUTING.md: tests with every change, a failing test first for a bug fix, table-driven tests, no network in unit tests, no bare `time.Sleep`.
- Commit after every change, with a Conventional Commits message, and push it.
- Never mention AI, assistants or code generation in code, comments, docs, commit messages, pull requests or issues.
- Never add `Co-Authored-By`, `Generated with` or similar trailers.
- No em-dashes. Keep comments short and about why.
- Every feature ships in the free build. Never add a license check or a paid-only code path.
- Update CHANGELOG.md under Unreleased for anything users will notice, and update README.md when usage changes.
