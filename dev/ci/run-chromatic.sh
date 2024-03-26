#!/usr/bin/env bash

set -ex

ARGS=("$@")

# Unfortunately, The BUILDKITE_RETRY_COUNT value is a string
if [[ ${BUILDKITE_RETRY_COUNT:-"0"} != "0" ]]; then
  # Chromatic fails with exit-code 1 if the commit stays the same and instructs one to add `--force-rebuild`
  # So we detect when a build is retried and then add the flag accordingly
  ARGS=("${ARGS[@]}" --force-rebuild true)
else
  echo "SIMULATING FAILURE"
  exit 1
fi

pnpm chromatic "${ARGS[@]}"
