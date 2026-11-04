# Labels

Every repository uses the same labels.

| Label | Meaning |
|---|---|
| `good first issue` | Small, says exactly which files to change and which tests to add |
| `help wanted` | Bigger, with the API, behavior and tests written out in the issue |
| `bug` | Something doesn't match the docs |
| `enhancement` | A new feature |
| `docs` | Documentation only |
| `adapter` | A new provider, backend, protocol or storage adapter. These are self-contained and the easiest way to add something big |
| `needs design` | Not ready to build yet: the API is still being agreed in the issue |
| `claimed` | Someone said they're working on it |

## How issues are written

An issue someone else can pick up without asking anything:

- **Why:** the problem, in a sentence or two
- **Proposed API:** the exact names, config keys or endpoints
- **Behavior:** the normal case, the errors and the edges
- **Tests:** the cases the pull request should cover
- **Where:** the files or packages to change, for a `good first issue`

Open issues in batches tied to a milestone, with a few `good first issue` items in each, so new people always have somewhere to start.
