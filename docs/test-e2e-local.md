# E2E local (Windows)

This guide helps you run the E2E tests locally, including the 3-node HA cluster tests.

## Prerequisites

- Go toolchain
- PostgreSQL running and accessible
- STORAGE_DSN set (example below)
- PowerShell 5.1+ (default on Windows)

## 1) Open local firewall ports (once)

The HA tests use fixed ports by default:
- HTTP: 18081–18083
- Raft: 18201–18203

Run:

```powershell
./scripts/open-ports.ps1
```

## 2) Configure database

Set your DSN in the current shell:

```powershell
$env:STORAGE_DSN = "postgres://user:pass@127.0.0.1:5432/hellojohn?sslmode=disable"
```

## 3) Migrate and seed, then run tests

Use the helper script. It will:
- run migrations (unless -NoMigrate)
- run the seeder (creates admin, users, clients, RBAC)
- run the E2E tests with DISABLE_DOTENV=1

```powershell
./scripts/seed-and-test.ps1
```

Options:
- `-NoMigrate` to skip migrations
- `-Run "25_rotate_keys_ha_test"` to run a subset
- `-Short` to pass -short to go test

## 4) Notes

- The tests isolate env loading with `DISABLE_DOTENV=1` and pass `-env-file notfound.env` to service processes.
- The HA JWKS rotation test requires the seeded admin to be present; if the DB isn’t seeded, it will skip.
- If you see port binding errors, ensure the firewall rules exist and no other services are bound to the test ports.

## (Optional) TLS for Raft

TLS/mTLS is already wired. To enable for local runs, set:

- `RAFT_TLS_ENABLE=true`
- `RAFT_TLS_CERT_FILE=./certs/node1.crt`
- `RAFT_TLS_KEY_FILE=./certs/node1.key`
- `RAFT_TLS_CA_FILE=./certs/ca.crt`
- `RAFT_TLS_SERVER_NAME=localhost` (optional SNI override)

Note: all cluster peers must be configured consistently; misconfiguration will prevent quorum.
