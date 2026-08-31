#!/usr/bin/env sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BIN="$ROOT/bin"
AUTH_DUMP=${1:-}
CHARACTERS_DUMP=${2:-}
WORLD_DUMP=${3:-}
DATA_ARCHIVE=${4:-}
mkdir -p "$BIN"
if [ ! -f "$BIN/authserver.conf" ]; then
    cp "$ROOT/configs/authserver.conf.dist" "$BIN/authserver.conf"
    sed -i 's/^DataDir[[:space:]]*=.*/DataDir = "bin"/' "$BIN/authserver.conf"
fi
if [ ! -f "$BIN/worldserver.conf" ]; then
    cp "$ROOT/configs/worldserver.conf.dist" "$BIN/worldserver.conf"
    sed -i 's/^DataDir[[:space:]]*=.*/DataDir = "bin"/' "$BIN/worldserver.conf"
    sed -i 's/^Eluna\.ScriptPath[[:space:]]*=.*/Eluna.ScriptPath = "bin\/lua_scripts"/' "$BIN/worldserver.conf"
fi
if [ -n "$DATA_ARCHIVE" ]; then unzip -o "$DATA_ARCHIVE" -d "$ROOT" >/dev/null; fi
if [ -n "$AUTH_DUMP" ] || [ -n "$CHARACTERS_DUMP" ] || [ -n "$WORLD_DUMP" ]; then
    [ -n "$AUTH_DUMP" ] && [ -n "$CHARACTERS_DUMP" ] && [ -n "$WORLD_DUMP" ] || { echo 'auth, characters, and world dumps must be supplied together' >&2; exit 2; }
    (cd "$ROOT" && go run ./tools/dbtool import-sql --output-dir "$BIN" --auth "$AUTH_DUMP" --characters "$CHARACTERS_DUMP" --world "$WORLD_DUMP" --force)
fi
printf 'Setup files are in %s\n' "$BIN"
