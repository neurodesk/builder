#!/usr/bin/env bash

set -ex

export CGO_ENABLED=0
export GOOS=linux

./go.sh build -o tester ./cmd/tester