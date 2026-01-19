# V2 Runtime Wiring Report

**Date:** 2026-01-13
**Status:** ✅ Enabled (via Stubs)

## 1. Executive Summary
The HTTP V2 stack has been physically wired ("cableado") to allow runtime execution. A composition root (`app/v2/app.go`) and a server wiring builder (`server/wiring.go`) have been implemented. The central router aggregator (`router.go`) has been updated to register all implemented domains.

**Key Achievement:** V2 is no longer "dead code". It can be instantiated as a standard `http.Handler`.

## 2. Changes Implemented

### A. Router Aggregator (`router.go`)
- **Action:** Enabled registration for:
  - `Auth` (Login, Register)
  - `OAuth2` (Authorize, Token)
  - `OIDC` (Discovery, UserInfo)
  - `Social` (Exchange, Start, Callback)
  - `Email` (Flows)
  - `Security` (CSRF)
  - `Session` (Cookies)
  - `Health` (Readyz)
- **Status:** All domains with existing controllers are now registered.

### B. Composition Root (`app/v2/app.go`)
- **Action:** Implemented `New(cfg, deps)` builder.
- **Logic:**
  1.  Instantiates `services.New(deps)` (Domain Services Layer).
  2.  Instantiates Controllers (using aggregators like `auth.NewControllers`).
  3.  Creates `http.NewServeMux()` and calls `router.RegisterV2Routes`.
- **Resolutions:** Fixed missing dependencies in controller constructors (OAuth, Session, Health).

### C. Server Wiring (`server/wiring.go`)
- **Action:** Implemented `BuildV2Handler()`.
- **Strategy:** Uses **Local Mocks** ("Stubs") for dependencies that are hard to instantiate in a vacuum (DB, KeyStore, ControlPlane).
- **Mocks Included:**
  - `MockKeystore`: for JWT signing keys.
  - `MockControlPlane`: for Tenant/Client resolution.
  - `NoOpEmailService`: to bypass SMTP requirements.
  - `NoOpSocialCache`: for social state storage.
- **Result:** Code compiles and allows booting the V2 handler for testing/verification without external infra.

### D. Route Collision Fix
- **Action:** Removed duplicate registration of `/v2/auth/providers` in `social_routes.go`.
- **Resolution:** This path is now exclusively handled by the `Auth` router (as declared in `auth_routes.go`), avoiding runtime panics.

## 3. Runtime Route Inventory (Alive)

The following domains are now "live" in the V2 Handler:

| Domain | Status | Notes |
| :--- | :--- | :--- |
| **Admin** | ✅ Live | Tenants, Users, Clients, Scopes CRUD. |
| **Auth** | ✅ Live | `/v2/auth/login`, `/register`, `/refresh`. |
| **MFA** | ✅ Live | TOTP flows. |
| **OIDC** | ✅ Live | Discovery, UserInfo (stubbed data). |
| **OAuth2** | ✅ Live | Authorize, Token (stubbed data). |
| **Health** | ✅ Live | `/readyz`. |
| **Social** | ✅ Live | `/v2/auth/social/*` (stubbed cache). |

## 4. Remaining Gaps & Placeholders

| Gap | Description | Plan |
| :--- | :--- | :--- |
| **Assets** | ❌ Router file exists but is empty. | Implement `AssetsController` (FileServer). |
| **Dev** | ❌ Router file exists but is empty. | Implement `DevController` (Shutdown). |
| **Real Deps** | ⚠️ Using Stubs. | Replace `Mock*` structs with real implementations in `wiring.go` once DB/Config is available. |

## 5. Next Steps
1.  **Switch to Real Deps:** Update `BuildV2Handler` to accept `Config` objects and initialize real `pgxpool`, `EmailService`, etc.
2.  **Mount V2:** In `cmd/server/main.go`, mount this V2 handler under `/v2` prefix or on a separate port.
3.  **End-to-End Tests:** Verify flows (Login, Admin) against a real database.
