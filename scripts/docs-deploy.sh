#!/usr/bin/env zsh
#
# Deploy the built docs site (_site/) to docs.f5w.nl/f5w-docgen: run the ansible
# `docs_deploy` playbook (ansible-g8x) against the f5w-services droplet, which
# stages the tree into a content-addressed release dir and swaps it in with an
# atomic symlink flip. Adapted from the footfall repo's wrapper; ansible owns
# the placement and its atomicity (playbooks/docs_deploy.yml). This wrapper only
# builds the tree fresh (via `make docs-deploy`, whose `docs` prerequisite runs
# first) and invokes the playbook with the right source dir and repo slug.
#
# Runs from the Mac only, against a REMOTE host only - a hard guard refuses
# localhost and local test hosts. `make docs-deploy` builds _site/ fresh before
# invoking this wrapper; run directly, it requires an already-built _site/ and
# fails fast if it is missing.
#
# The playbook runs under `op run --env-file=.op-env` (the ansible repo's secret
# convention) so 1Password injects the vaulted config; this needs the `op` CLI
# and the ansible repo's .op-env file.
#
# Usage:
#   scripts/docs-deploy.sh              # deploy _site/ to docs.f5w.nl/f5w-docgen
#
# Config is defined once here and overridable from the environment:
#   F5W_DOCGEN_ANSIBLE_DIR          ansible control repo   (default: ~/Code/ansible)
#   F5W_DOCGEN_DOCS_DEPLOY_LIMIT    inventory host/limit   (default: f5w-services)
#   F5W_DOCGEN_DOCS_REPO            repo slug / URL segment (default: f5w-docgen)
#   F5W_DOCGEN_DOCS_DEPLOY_PLAYBOOK playbook to run        (default: playbooks/docs_deploy.yml)

set -euo pipefail

# -------------------------------------------------------------------------
# Config
# -------------------------------------------------------------------------
ANSIBLE_DIR="${F5W_DOCGEN_ANSIBLE_DIR:-${HOME}/Code/ansible}"
# Unset-only fallback (no colon): an explicit empty override is a mistake the
# remote-only guard below catches, not silently the default.
DEPLOY_LIMIT="${F5W_DOCGEN_DOCS_DEPLOY_LIMIT-f5w-services}"
DOCS_REPO="${F5W_DOCGEN_DOCS_REPO:-f5w-docgen}"
DEPLOY_PLAYBOOK="${F5W_DOCGEN_DOCS_DEPLOY_PLAYBOOK:-playbooks/docs_deploy.yml}"

script_dir="${0:A:h}"
repo_root="${script_dir:h}"
site_dir="${repo_root}/_site"

# -------------------------------------------------------------------------
# Remote-only guard (before anything - the deploy must never target the Mac)
# -------------------------------------------------------------------------
case "$DEPLOY_LIMIT" in
"" | localhost | 127.0.0.1 | ::1 | *.test)
  print -u2 -- "ERROR: docs deploy is remote-only; refusing target '${DEPLOY_LIMIT}'."
  print -u2 -- "Set F5W_DOCGEN_DOCS_DEPLOY_LIMIT to a remote host (default: f5w-services)."
  exit 1
  ;;
esac

# -------------------------------------------------------------------------
# Repo-slug guard: docs_repo becomes a path segment on the droplet, so validate
# it here at the door too (the playbook re-asserts it - SEC-6, every hop).
# -------------------------------------------------------------------------
if [[ ! "$DOCS_REPO" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
  print -u2 -- "ERROR: F5W_DOCGEN_DOCS_REPO='${DOCS_REPO}' is not a valid slug (^[a-z0-9][a-z0-9-]*\$)."
  exit 1
fi

# -------------------------------------------------------------------------
# Preflight (built tree first, then the ansible checkout / tooling / secrets)
# -------------------------------------------------------------------------
if [[ ! -d "$site_dir" ]]; then
  print -u2 -- "ERROR: no built docs tree at ${site_dir}."
  print -u2 -- "Build it first (make docs), or run 'make docs-deploy', which builds it."
  exit 1
fi

if [[ ! -f "${ANSIBLE_DIR}/${DEPLOY_PLAYBOOK}" ]]; then
  print -u2 -- "ERROR: the ansible docs-deploy playbook is not present at ${ANSIBLE_DIR}/${DEPLOY_PLAYBOOK}."
  print -u2 -- "Check out the ansible repo's docs_deploy playbook (ansible-g8x) first."
  exit 1
fi

if ! command -v ansible-playbook >/dev/null 2>&1; then
  print -u2 -- "ERROR: ansible-playbook not found on PATH; install ansible on the Mac."
  exit 1
fi

# The playbook resolves op:// secrets from env vars injected by `op run`, so both
# the op CLI and the ansible repo's .op-env must be present.
if ! command -v op >/dev/null 2>&1; then
  print -u2 -- "ERROR: 1Password CLI (op) not found on PATH; needed to inject the ansible secrets."
  exit 1
fi

if [[ ! -f "${ANSIBLE_DIR}/.op-env" ]]; then
  print -u2 -- "ERROR: ${ANSIBLE_DIR}/.op-env not found; ansible secrets cannot be injected."
  exit 1
fi

# -------------------------------------------------------------------------
# Run the ansible playbook (it does the atomic stage-and-swap on the droplet)
# -------------------------------------------------------------------------
playbook_args=(
  "$DEPLOY_PLAYBOOK"
  --limit "$DEPLOY_LIMIT"
  -e "docs_repo=${DOCS_REPO}"
  -e "docs_source_dir=${site_dir}"
)

print -- "Deploying docs (${site_dir} -> ${DOCS_REPO}) to '${DEPLOY_LIMIT}'..."

# Run from the ansible repo so ansible.cfg, roles_path, and the inventory all
# resolve; a subshell keeps this script's working directory unchanged. `op run`
# injects the op:// secrets the playbook's group_vars reference (the repo convention).
(cd "$ANSIBLE_DIR" && op run --env-file=.op-env -- ansible-playbook "${playbook_args[@]}")

print -- "Docs deploy complete: https://docs.f5w.nl/${DOCS_REPO}/ (via '${DEPLOY_LIMIT}')."
