#!/usr/bin/env bash
# Checks the agent skills layout (see .agents/skills/README.md):
#   - .agents/skills/<name>/SKILL.md is the canonical copy; .claude/skills/<name> is a relative symlink to it
#     (Claude Code reads only .claude/skills; Codex, Gemini CLI, Copilot, Cursor, OpenCode, Antigravity read .agents/skills).
#   - frontmatter name equals the directory name and matches ^[a-z0-9]+(-[a-z0-9]+)*$;
#     description is present and at most 1024 characters; SKILL.md has at most 500 lines (agentskills.io/specification).
# Every violation prints one line: ERROR: <path>: <reason>; fix: <command or edit>
set -euo pipefail
cd "$(dirname "$0")/.."

skills=.agents/skills
links=.claude/skills
fail=0
err() { echo "ERROR: $1: $2; fix: $3"; fail=1; }

# frontmatter_field <file> <key>: value of a top-level scalar key between the first two '---' lines.
# Handles single-line values and folded/literal blocks (key: > or key: |).
frontmatter_field() {
  awk -v key="$2" '
    { sub(/\r$/, "") }
    NR == 1 && $0 != "---" { exit }
    NR == 1 { next }
    $0 == "---" { exit }
    infield && /^[ \t]/ { sub(/^[ \t]+/, ""); val = val (val == "" ? "" : " ") $0; next }
    { infield = 0 }
    index($0, key ":") == 1 {
      v = substr($0, length(key) + 2); sub(/^[ \t]+/, "", v); sub(/[ \t]+$/, "", v)
      if (v ~ /^[>|][-+]?$/) { infield = 1; val = ""; next }
      gsub(/^"|"$/, "", v); gsub(/^'\''|'\''$/, "", v); val = v
    }
    END { print val }
  ' "$1"
}

[ -d "$skills" ] || { echo "skills-check: no $skills directory"; exit 0; }

count=0
for dir in "$skills"/*/; do
  name="$(basename "$dir")"
  file="$dir/SKILL.md"
  count=$((count + 1))
  if [ ! -f "$file" ]; then
    err "$dir" "missing SKILL.md" "add $file or remove the directory"
    continue
  fi
  want_link="../../$skills/$name"
  actual_link="$(readlink "$links/$name" 2>/dev/null || cat "$links/$name" 2>/dev/null || true)"
  actual_link="${actual_link%$'\r'}"
  if [ "$actual_link" != "$want_link" ]; then
    err "$links/$name" "missing or not a symlink to $want_link" "mkdir -p $links && ln -sfn $want_link $links/$name"
  fi
  fm_name="$(frontmatter_field "$file" name)"
  if [ "$fm_name" != "$name" ]; then
    err "$file" "frontmatter name '$fm_name' differs from directory '$name'" "set 'name: $name' in the frontmatter"
  fi
  if ! [[ "$name" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]]; then
    err "$dir" "name must match ^[a-z0-9]+(-[a-z0-9]+)*\$" "rename the directory and the symlink to lowercase-hyphenated"
  fi
  desc="$(frontmatter_field "$file" description)"
  if [ -z "$desc" ]; then
    err "$file" "description is missing" "add a 'description: Use when ...' line to the frontmatter"
  elif [ "${#desc}" -gt 1024 ]; then
    err "$file" "description has ${#desc} characters (max 1024)" "shorten the description"
  fi
  lines="$(wc -l < "$file")"
  if [ "$lines" -gt 500 ]; then
    err "$file" "$lines lines (max 500)" "move detail into references/ files next to SKILL.md"
  fi
done

if [ -d "$links" ]; then
  for entry in "$links"/*; do
    [ -e "$entry" ] || [ -L "$entry" ] || continue
    name="$(basename "$entry")"
    if [ ! -d "$skills/$name" ]; then
      err "$entry" "no canonical directory $skills/$name" "rm $entry, or move the skill into $skills/$name"
    fi
  done
fi

[ "$fail" -eq 0 ] || exit 1
echo "skills-check: $count skills OK"
