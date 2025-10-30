#!/usr/bin/env bash
set -euo pipefail
export E2E_SKIP_GLOBAL_SERVER=1
export DISABLE_DOTENV=1

go test ./test/e2e -run 'Test_40_Leader_Gating_Redirect' -count=1 -timeout=4m

go test ./test/e2e -run 'Test_41_SnapshotRestore_JWKSIdentical' -count=1 -timeout=6m

go test ./test/e2e -run 'Test_42_RequireLeader_Wiring_Smoke' -count=1 -timeout=2m
