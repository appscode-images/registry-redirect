#!/usr/bin/env bash

set -euxo pipefail

crane version >/dev/null \
  || { echo "install crane: https://github.com/google/go-containerregistry/blob/main/cmd/crane"; exit 1; }

# Kill whatever's running on :8080
# kill -9 $(lsof -ti:8080)

# Run the redirector in the background, kill it when the script exits.
go build && ./registry-redirect &
PID=$!
echo "server running with pid $PID"
trap 'kill $PID' EXIT

sleep 3  # Server isn't immediately ready.

echo "Testing appscode/alpine"

crane digest localhost:8080/appscode/alpine
crane manifest localhost:8080/appscode/alpine
crane ls localhost:8080/appscode/alpine
crane pull localhost:8080/appscode/alpine /dev/null
crane validate --remote=localhost:8080/appscode/alpine

echo "Testing kubedb/busybox"

crane digest localhost:8080/kubedb/busybox
crane manifest localhost:8080/kubedb/busybox
crane ls localhost:8080/kubedb/busybox
crane pull localhost:8080/kubedb/busybox /dev/null
crane validate --remote=localhost:8080/kubedb/busybox

echo PASSED
