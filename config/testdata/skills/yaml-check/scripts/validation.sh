#!/usr/bin/env bash
set -euo pipefail

dir="${1:-.}"
errors=0

while IFS= read -r -d '' file; do
  if ! yq eval '.' "$file" > /dev/null 2>&1; then
    echo "FAIL: $file"
    errors=$((errors + 1))
  else
    echo "OK: $file"
  fi
done < <(find "$dir" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0)

if [[ $errors -gt 0 ]]; then
  echo "$errors file(s) failed validation"
  exit 1
fi

echo "All YAML files are valid"
