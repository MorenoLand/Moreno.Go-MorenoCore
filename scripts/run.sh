#!/usr/bin/env sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TARGET=${1:-moreno}
if [ "$TARGET" != "moreno" ]; then shift; else shift || true; fi
cd "$ROOT"
case "$TARGET" in
    moreno) exec go run . "$@" ;;
    auth) exec go run . --auth "$@" ;;
    world) exec go run . --world "$@" ;;
    *) echo "usage: $0 moreno|auth|world [arguments]" >&2; exit 2 ;;
esac
