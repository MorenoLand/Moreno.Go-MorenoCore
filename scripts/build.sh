#!/usr/bin/env sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TARGET=${1:-moreno}
OUTPUT_DIRECTORY=${2:-bin}
mkdir -p "$ROOT/$OUTPUT_DIRECTORY"
cd "$ROOT"
case "$TARGET" in
    moreno) go build -trimpath -o "$OUTPUT_DIRECTORY/MorenoCore" . ;;
    auth) go build -trimpath -o "$OUTPUT_DIRECTORY/AuthServer" ./server/authserver ;;
    world|gameserver) go build -trimpath -o "$OUTPUT_DIRECTORY/WorldServer" ./server/worldserver ;;
    all)
        go build -trimpath -o "$OUTPUT_DIRECTORY/MorenoCore" .
        go build -trimpath -o "$OUTPUT_DIRECTORY/AuthServer" ./server/authserver
        go build -trimpath -o "$OUTPUT_DIRECTORY/WorldServer" ./server/worldserver
        ;;
    *) echo "usage: $0 moreno|auth|world|gameserver|all [output-directory]" >&2; exit 2 ;;
esac
