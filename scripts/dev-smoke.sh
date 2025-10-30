#!/usr/bin/env bash
set -euo pipefail
export DISABLE_DOTENV=1

go test ./test/unit/... -count=1

go test ./test/e2e -run '00_|01_|02_|04_|08_' -count=1 -timeout=6m
