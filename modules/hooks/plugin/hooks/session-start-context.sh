#!/usr/bin/env bash
# Stateless SessionStart facts for a Lasso workspace. No fetch; no task state.
set -euo pipefail

PATH="$PATH:/run/current-system/sw/bin:/usr/bin"

# Prefer Lasso env; accept Columbus names during instance migration.
config_path=${LASSO_CONFIG:-${COLUMBUS_CONFIG:-/etc/lasso/config.json}}
machine_id=${LASSO_MACHINE_ID:-${COLUMBUS_MACHINE_ID:-}}
host=${HOSTNAME:-$(uname -n 2>/dev/null || printf 'unknown')}

if [[ -z $machine_id && -r $config_path ]] && command -v jq >/dev/null 2>&1; then
    machine_id=$(jq -r '.machine.id // empty' "$config_path" 2>/dev/null || true)
fi

printf 'Lasso workspace startup facts (read-only; no fetch was performed):\n'
if [[ -n $machine_id ]]; then
    if [[ $machine_id == "$host" ]]; then
        printf -- '- machine: %s (hostname verified)\n' "$machine_id"
    else
        printf -- '- machine: %s (WARNING: hostname is %s)\n' "$machine_id" "$host"
    fi
else
    printf -- '- machine: unavailable (do not infer identity from checkout paths; verify %s)\n' "$config_path"
fi

if ! repo=$(git rev-parse --show-toplevel 2>/dev/null); then
    printf -- '- Git workspace: none\n'
    exit 0
fi

branch=$(git symbolic-ref --quiet --short HEAD 2>/dev/null || printf 'detached')
upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)
git_dir=$(git rev-parse --path-format=absolute --git-dir 2>/dev/null || true)
common_dir=$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)
worktree_kind=linked
if [[ -n $git_dir && $git_dir == "$common_dir" ]]; then
    worktree_kind=primary
fi

status=$(git status --porcelain=v2 --branch --untracked-files=normal 2>/dev/null || true)
dirty=clean
if grep -qv '^# ' <<<"$status"; then
    dirty=dirty
fi

divergence=unavailable
if [[ -n $upstream ]]; then
    counts=$(git rev-list --left-right --count "HEAD...@{upstream}" 2>/dev/null || true)
    if [[ -n $counts ]]; then
        read -r ahead behind <<<"$counts"
        divergence="ahead $ahead, behind $behind"
    fi
fi

printf -- '- repository: %s\n' "$repo"
printf -- '- worktree: %s; branch: %s; status: %s\n' "$worktree_kind" "$branch" "$dirty"
printf -- '- upstream: %s; cached divergence: %s\n' "${upstream:-none}" "$divergence"
printf -- '- remote freshness: unknown until a successful fetch in the current task\n'
printf -- '- coordination: use Codex Handoff to move one live task; Claude moves only committed Git into a fresh target-host worktree; parallel tasks use separate worktrees and branches; never pull, reset, clean, or auto-stash a dirty checkout\n'

# Optional instance deployment revision (path configurable).
deployment_revision_file=${LASSO_DEPLOYMENT_REVISION_FILE:-/run/current-system/etc/lasso-deployment/revision}
if [[ -n $machine_id && -r $deployment_revision_file ]]; then
    IFS= read -r deployment_revision <"$deployment_revision_file" || true
    if [[ -n ${deployment_revision:-} ]]; then
        printf -- '- active %s deployment revision: %s; a deployment candidate must descend from it\n' \
            "$machine_id" "$deployment_revision"
    fi
fi

# Optional NixOS host set from config: machine.nixos_hosts = ["…"]
nixos_hint=false
if [[ -r $config_path ]] && command -v jq >/dev/null 2>&1; then
    if [[ -n $machine_id ]] && jq -e --arg id "$machine_id" '.machine.nixos_hosts // [] | index($id) != null' "$config_path" >/dev/null 2>&1; then
        nixos_hint=true
    fi
fi
if [[ $nixos_hint == true ]]; then
    printf -- '- NixOS transaction: commit in an isolated task worktree, then use the host-owned build/activate contract; publication is not an activation prerequisite\n'
fi

resolve_workspace_root() {
    if [[ -n ${LASSO_ROOT:-} ]]; then
        printf '%s\n' "$LASSO_ROOT"
        return
    fi
    if [[ -n ${COLUMBUS_ROOT:-} ]]; then
        printf '%s\n' "$COLUMBUS_ROOT"
        return
    fi
    if [[ -r $config_path ]] && command -v jq >/dev/null 2>&1; then
        configured_root=$(jq -r '.paths.root // empty' "$config_path" 2>/dev/null || true)
        if [[ -n $configured_root ]]; then
            printf '%s\n' "$configured_root"
            return
        fi
    fi
    # Walk up from the current repo for project-defs/registry.toml.
    dir=$repo
    while [[ -n $dir && $dir != / ]]; do
        if [[ -f $dir/project-defs/registry.toml ]]; then
            printf '%s\n' "$dir"
            return
        fi
        dir=$(dirname "$dir")
    done
}

if [[ $worktree_kind == primary ]]; then
    workspace_root=$(resolve_workspace_root || true)
    protected=false
    if [[ -n ${workspace_root:-} ]]; then
        if [[ $repo == "$workspace_root" ]]; then
            protected=true
        elif [[ $repo == "$workspace_root"/projects/* ]]; then
            relative=${repo#"$workspace_root"/projects/}
            project=${relative%%/*}
            [[ -f $workspace_root/project-defs/$project/project.toml ]] && protected=true
        fi
    fi
    if [[ $protected == true ]]; then
        printf -- '- MUTATION FENCE: this is a stable integration checkout. Keep it available to headless tools; create or resume an isolated task worktree from freshly fetched origin/main before editing, committing, or deploying.\n'
    fi
fi
