#!/bin/sh

cd "$(dirname "$0")"
./genCert
echo "Press any key to exit..."
# shellcheck disable=SC2034
read -r dummy