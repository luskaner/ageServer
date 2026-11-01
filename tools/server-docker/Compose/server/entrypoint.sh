#!/bin/sh

if [ -z "${GAME_ID}" ]; then
    echo "GAME_ID is required (one of: age1, age2, age3, age4, athens)." >&2
    exit 1
fi

LOG_SUBFOLDER=$(date +"%Y-%m-%dT%H-%M-%S")
exec ./server -e "$GAME_ID" --log --flatLog --logRoot=/app/logs/server/$GAME_ID/$LOG_SUBFOLDER $SERVER_ARGS
