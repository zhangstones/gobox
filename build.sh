#!/bin/bash
set -e

# Cross-compile gobox for Linux.
# Usage: ./build.sh          # build linux/amd64 (default)
#        ./build.sh -arm     # build linux/arm64

if [[ "$1" == "-arm" ]]; then
    GOOS=linux
    GOARCH=arm64
    OUTPUT="gobox-linux-arm64"
else
    GOOS=linux
    GOARCH=amd64
    OUTPUT="gobox"
fi

CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$OUTPUT" .
echo "Built $OUTPUT"
