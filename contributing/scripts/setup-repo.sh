#!/usr/bin/env sh
# Usage: setup-repo.sh owner/name [ruleset.json]
# Applies the shared merge rules, labels and security settings to a repository.
set -eu
repo="$1"
ruleset="${2:-ruleset.json}"
here="$(cd "$(dirname "$0")/.." && pwd)"

gh api -X PATCH "repos/$repo" \
  -F allow_squash_merge=true -F allow_merge_commit=false -F allow_rebase_merge=false \
  -F delete_branch_on_merge=true -F allow_update_branch=true \
  -f squash_merge_commit_title=PR_TITLE -f squash_merge_commit_message=PR_BODY >/dev/null

gh api -X PUT "repos/$repo/actions/permissions/fork-pr-contributor-approval" \
  -f approval_policy=all_external_contributors >/dev/null

gh api -X PUT "repos/$repo/private-vulnerability-reporting" >/dev/null || true
gh api -X PATCH "repos/$repo" --input - >/dev/null <<JSON || true
{"security_and_analysis":{"secret_scanning":{"status":"enabled"},"secret_scanning_push_protection":{"status":"enabled"}}}
JSON

while IFS='|' read -r name color description; do
  [ -z "$name" ] && continue
  gh label create "$name" --repo "$repo" --color "$color" --description "$description" --force >/dev/null
done <<LABELS
good first issue|7057ff|Small, says exactly which files to change and which tests to add
help wanted|008672|Bigger, with the API, behavior and tests written out
bug|d73a4a|Something doesn't match the docs
enhancement|a2eeef|A new feature
docs|0075ca|Documentation only
adapter|fbca04|A new provider, backend, protocol or storage adapter
needs design|e4e669|The API is still being agreed in the issue
claimed|c5def5|Someone said they're working on it
LABELS

existing="$(gh api "repos/$repo/rulesets" --jq '.[] | select(.name == "main") | .id')"
if [ -n "$existing" ]; then
  gh api -X PUT "repos/$repo/rulesets/$existing" --input "$here/$ruleset" >/dev/null
else
  gh api -X POST "repos/$repo/rulesets" --input "$here/$ruleset" >/dev/null
fi

echo "configured $repo"
