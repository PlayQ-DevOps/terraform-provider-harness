#!/usr/bin/env bash
#
# Validates .changelog entries.
#
#   scripts/check-changelog.sh <base-ref>   # entries added/modified since <base-ref>...HEAD
#   scripts/check-changelog.sh --all        # every entry in .changelog/
#
# A change set must add or modify at least one entry unless NO_CHANGELOG=1 (a pull-request
# check sets it when the pull request carries the "no-changelog" label). Every checked
# entry must start with a ```release-note:<type> fence whose type is one the changelog
# template renders; any other type is silently dropped from CHANGELOG.md by changelog-build.
set -euo pipefail

__dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$(dirname "$__dir")"

VALID_TYPES="breaking-change|note|feature|new-resource|new-data-source|new-guide|new-ephemeral-resource|enhancement|bug"

entries=()
if [ "${1:-}" = "--all" ]; then
    while IFS= read -r f; do entries+=("$f"); done < <(find .changelog -name '*.txt' | sort)
else
    base="${1:-origin/main}"
    if [ "${NO_CHANGELOG:-0}" = "1" ]; then
        echo "changelog check skipped (NO_CHANGELOG=1)"
        exit 0
    fi
    while IFS= read -r f; do entries+=("$f"); done < <(git diff --name-only --diff-filter=AM "$base...HEAD" -- '.changelog/*.txt')
    if [ ${#entries[@]} -eq 0 ]; then
        echo "::error::No .changelog entry was added or modified relative to $base."
        echo "Add one with 'make changelog-entry ENTRY=<name> TYPE=<type>' (types: ${VALID_TYPES//|/, })"
        echo "or label the pull request 'no-changelog' if the change is not user-facing."
        exit 1
    fi
fi

rc=0
for f in "${entries[@]}"; do
    [ -f "$f" ] || continue
    first="$(head -n1 "$f" | tr -d '\r')"
    if ! [[ "$first" =~ ^\`\`\`release-note:($VALID_TYPES)$ ]]; then
        echo "::error file=$f::first line is '$first'; it must be \`\`\`release-note:<type> with type one of ${VALID_TYPES//|/, }"
        rc=1
    fi
    if ! grep -q '^```$' "$f"; then
        echo "::error file=$f::missing closing \`\`\` fence"
        rc=1
    fi
done
if [ $rc -eq 0 ]; then
    echo "changelog entries OK (${#entries[@]} checked)"
fi
exit $rc
