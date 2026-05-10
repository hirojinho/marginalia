#!/usr/bin/env bash
# Idempotent installer: symlinks the tracked pre-commit hook into .git/hooks/.
set -e

ROOT="$(git rev-parse --show-toplevel)"
SRC="$ROOT/scripts/git-hooks/pre-commit"
DST="$ROOT/.git/hooks/pre-commit"

if [ ! -f "$SRC" ]; then
  echo "FAIL: $SRC not found"
  exit 1
fi

if [ -e "$DST" ] && [ ! -L "$DST" ]; then
  if [ -s "$DST" ]; then
    cp "$DST" "$DST.backup.$(date +%s)"
    echo "Backed up existing hook to $DST.backup.*"
  fi
  rm "$DST"
fi

ln -sfn "$SRC" "$DST"
chmod +x "$SRC"
echo "Installed pre-commit hook: $DST -> $SRC"
