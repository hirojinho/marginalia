#!/usr/bin/env bash
# Lint overnight-pipeline spec files.
#
# Catches the structural mistakes that the orchestrator's schema check rejects
# AFTER picking the ticket (wasting a pipeline night). Run locally before
# committing to the queue. Args: spec file paths. Exits non-zero on any
# violation, with a human-readable report per file.
#
# Contract enforced (mirrors specs/README.md):
#   1. Frontmatter has all mandatory fields.
#   2. Filename stem matches `id:` field.
#   3. Body has `## Goal`, `## Implementation plan`,
#      `## Verification recipe`, `## Done criteria`.
#   4. The `## Verification recipe` section contains BOTH
#      `### Pre-baseline` and `### Post-acceptance` sub-sections.

set -euo pipefail

REQUIRED_FRONTMATTER=(
  id title max_wall_clock_minutes max_diff_lines max_retries max_tokens
  requires_visual_approval allow_web_search
)
REQUIRED_BODY_SECTIONS=(
  "## Goal"
  "## Implementation plan"
  "## Verification recipe"
  "## Done criteria"
)
REQUIRED_VERIFIER_SUBSECTIONS=(
  "### Pre-baseline"
  "### Post-acceptance"
)

fail=0

check_spec() {
  local file="$1"
  local errors=()

  if [ ! -f "$file" ]; then
    echo "FAIL $file: file not found"
    fail=1
    return
  fi

  # Frontmatter block: lines between the first two '---' markers.
  local fm
  fm=$(awk 'BEGIN{n=0} /^---$/{n++; if(n==2) exit; next} n==1{print}' "$file")
  if [ -z "$fm" ]; then
    errors+=("missing or empty YAML frontmatter")
  else
    for key in "${REQUIRED_FRONTMATTER[@]}"; do
      if ! grep -qE "^${key}:[[:space:]]" <<<"$fm"; then
        errors+=("frontmatter missing required field: ${key}")
      fi
    done

    # Filename stem must match `id:` field.
    local id_val
    id_val=$(grep -E '^id:[[:space:]]' <<<"$fm" | head -1 | sed -E 's/^id:[[:space:]]*//; s/[[:space:]]+$//')
    local stem
    stem=$(basename "$file" .md)
    if [ -n "$id_val" ] && [ "$id_val" != "$stem" ]; then
      errors+=("id field ('$id_val') does not match filename stem ('$stem')")
    fi
  fi

  # Body sections.
  for section in "${REQUIRED_BODY_SECTIONS[@]}"; do
    if ! grep -qxF "$section" "$file" && ! grep -qE "^${section}([[:space:]]|$)" "$file"; then
      errors+=("missing body section: '$section'")
    fi
  done

  # Verifier sub-sections: only check if `## Verification recipe` exists; emit
  # both sub-section misses separately so the author sees the full diagnosis.
  local verifier_block
  verifier_block=$(awk '
    /^## / { in_v = 0 }
    /^## Verification recipe[[:space:]]*$/ { in_v = 1; next }
    in_v { print }
  ' "$file")
  if [ -n "$verifier_block" ]; then
    for sub in "${REQUIRED_VERIFIER_SUBSECTIONS[@]}"; do
      if ! grep -qE "^${sub}([[:space:]]|$)" <<<"$verifier_block"; then
        errors+=("'## Verification recipe' is missing required sub-section starting with '$sub'")
      fi
    done
  fi

  if [ ${#errors[@]} -eq 0 ]; then
    echo "OK   $file"
  else
    echo "FAIL $file"
    for e in "${errors[@]}"; do
      echo "       - $e"
    done
    fail=1
  fi
}

if [ $# -eq 0 ]; then
  echo "usage: $0 <spec.md> [spec.md ...]" >&2
  exit 2
fi

for f in "$@"; do
  check_spec "$f"
done

exit "$fail"
