#!/bin/sh

cd "$(dirname "$0")"
if "$@"; then
  echo "Program finished successfully, closing in 10 seconds..."
  sleep 10
else
  echo "Program finished with errors..."
  echo "Press any key to exit..."
  # shellcheck disable=SC2034
  read -r dummy
fi