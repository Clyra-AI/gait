#!/usr/bin/env bash
set -euo pipefail

REPO="Clyra-AI/gait"
SOURCE_DIR="docs/wiki"
COMMIT_MESSAGE="docs(wiki): sync wiki pages from docs/wiki"

usage() {
  cat <<'USAGE'
Usage: scripts/publish_wiki.sh [--repo owner/name] [--source-dir docs/wiki] [--message "commit message"]

Syncs markdown pages from the source directory to the GitHub wiki git repo.

Example:
  bash scripts/publish_wiki.sh --repo Clyra-AI/gait
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      REPO="${2:-}"
      shift 2
      ;;
    --source-dir)
      SOURCE_DIR="${2:-}"
      shift 2
      ;;
    --message)
      COMMIT_MESSAGE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ ! -d "$SOURCE_DIR" ]]; then
  echo "source directory not found: $SOURCE_DIR" >&2
  exit 2
fi

WIKI_REMOTE="git@github.com:${REPO}.wiki.git"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "cloning wiki remote: $WIKI_REMOTE"
if ! git clone "$WIKI_REMOTE" "$TMP_DIR/wiki" >/dev/null 2>&1; then
  cat >&2 <<'EOF'
unable to clone wiki remote.

likely cause: wiki git repo is not initialized yet.
one-time setup:
  1) open the repository wiki in the browser
  2) create and save any page (for example "Home")
  3) rerun this script
EOF
  exit 2
fi

find "$TMP_DIR/wiki" -mindepth 1 -maxdepth 1 ! -name '.git' -exec rm -rf {} +
cp -R "$SOURCE_DIR"/. "$TMP_DIR/wiki"/

cd "$TMP_DIR/wiki"
git add -A

if git diff --cached --quiet; then
  echo "wiki is already up to date"
  exit 0
fi

git commit -m "$COMMIT_MESSAGE" >/dev/null
git push origin HEAD
echo "wiki sync complete"
