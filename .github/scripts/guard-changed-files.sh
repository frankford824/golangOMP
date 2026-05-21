#!/usr/bin/env bash
set -euo pipefail

base_ref="${1:-}"
head_ref="${2:-HEAD}"

if [[ -z "$base_ref" ]]; then
  echo "usage: $0 <base-ref> [head-ref]" >&2
  exit 2
fi

changed_files="$(git diff --name-only --diff-filter=ACMR "$base_ref" "$head_ref" || true)"

if [[ -z "$changed_files" ]]; then
  echo "No changed files to guard."
  exit 0
fi

blocked=()
package_lock_changes=()
other_changes=()

while IFS= read -r path; do
  [[ -z "$path" ]] && continue

  case "$path" in
    node_modules/*|*/node_modules/*|.gomodcache/*|*/.gomodcache/*)
      continue
      ;;
  esac

  if [[ "$path" =~ (^|/)(mock|mocks|__mocks__)(/|$) ]]; then
    blocked+=("$path :: mock directory changes are forbidden")
    continue
  fi

  if [[ "$path" =~ (^|/)[^/]*([._-]mock|mock[._-]|mockData)[^/]*$ ]]; then
    blocked+=("$path :: mock data/file changes are forbidden")
    continue
  fi

  case "$path" in
    package.json|*/package.json|package-lock.json|*/package-lock.json|pnpm-lock.yaml|*/pnpm-lock.yaml|yarn.lock|*/yarn.lock)
      package_lock_changes+=("$path")
      ;;
    *)
      other_changes+=("$path")
      ;;
  esac
done <<< "$changed_files"

if (( ${#package_lock_changes[@]} > 0 )); then
  dependency_review_marker="[deps-review]"
  commit_messages="$(git log --format=%B "$base_ref..$head_ref" || true)"

  if (( ${#other_changes[@]} > 0 )); then
    blocked+=("package/lock changes must be isolated in a dependency-only push")
    for path in "${package_lock_changes[@]}"; do
      blocked+=("$path :: package and lockfile changes cannot be mixed with application changes")
    done
  elif [[ "$commit_messages" != *"$dependency_review_marker"* ]]; then
    blocked+=("dependency-only package/lock changes require ${dependency_review_marker} in the commit message")
    for path in "${package_lock_changes[@]}"; do
      blocked+=("$path :: package and lockfile changes require explicit owner dependency review")
    done
  else
    echo "Dependency-only package/lock change allowed by ${dependency_review_marker} marker."
  fi
fi

if (( ${#blocked[@]} > 0 )); then
  echo "::error::Forbidden mock/package/lock changes detected."
  echo
  echo "This repository does not allow routine commits that change mock data or package/lock files."
  echo "If a dependency change is genuinely required, split it into an owner-reviewed dependency-only change."
  echo
  printf '%s\n' "${blocked[@]}"
  exit 1
fi

echo "Mock/package/lock guard passed."
