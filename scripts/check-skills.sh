#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

mapfile -t skills < <(
    {
        if [[ -d .agents/skills ]]; then
            find -L .agents/skills -mindepth 2 -maxdepth 2 -name SKILL.md -print
        fi
        if [[ -d skills ]]; then
            find skills -mindepth 2 -maxdepth 2 -name SKILL.md -print
        fi
        find plugins -mindepth 4 -maxdepth 4 -path '*/skills/*/SKILL.md' -print
        for project_skills in projects/*/*/.agents/skills; do
            if [[ -d $project_skills ]]; then
                rg --files "$project_skills" -g 'SKILL.md' || true
            fi
        done
    } | sort -u
)

fail=0
for file in "${skills[@]}"; do
    first=$(sed -n '1p' "$file")
    [[ $first == '---' ]] || {
        echo "$file: missing opening frontmatter delimiter" >&2
        fail=1
        continue
    }

    frontmatter=$(awk 'NR == 1 { next } /^---$/ { exit } { print }' "$file")
    mapfile -t keys < <(printf '%s\n' "$frontmatter" | awk -F: '/^[a-zA-Z0-9_-]+:/ { print $1 }')
    case " ${keys[*]} " in
        ' name description ')
            ;;
        ' name description disable-model-invocation ')
            value=$(printf '%s\n' "$frontmatter" | sed -n 's/^disable-model-invocation:[[:space:]]*//p')
            [[ $value == 'true' ]] || {
                echo "$file: disable-model-invocation must be exactly true" >&2
                fail=1
            }
            ;;
        *)
            echo "$file: frontmatter must contain only name then description plus optional disable-model-invocation: true" >&2
            fail=1
            ;;
    esac

    name=$(printf '%s\n' "$frontmatter" | sed -n 's/^name:[[:space:]]*//p')
    description=$(printf '%s\n' "$frontmatter" | sed -n 's/^description:[[:space:]]*//p')
    [[ $name =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]] || {
        echo "$file: invalid skill name '$name'" >&2
        fail=1
    }
    [[ -n $description ]] || {
        echo "$file: empty description" >&2
        fail=1
    }
    if [[ $description == *'<'* || $description == *'>'* ]]; then
        echo "$file: description contains angle-bracket placeholders" >&2
        fail=1
    fi

    if rg -n '\$ARGUMENTS|AskUserQuestion|TaskUpdate' "$file" >/dev/null; then
        echo "$file: contains a retired runtime-specific interface" >&2
        fail=1
    fi

    metadata="${file%/SKILL.md}/agents/openai.yaml"
    if [[ ! -f $metadata ]]; then
        echo "$file: missing agents/openai.yaml" >&2
        fail=1
    elif ! rg -F "\$$name" "$metadata" >/dev/null; then
        echo "$metadata: default prompt must reference \$$name" >&2
        fail=1
    fi
done

if ((fail)); then
    exit 1
fi

printf 'skills ok: %d\n' "${#skills[@]}"
