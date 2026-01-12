# V1 Handlers Inventory

Inventory of HTTP handlers for `hellojohn` V1 service.
Generated to facilitate Data Layer refactoring.

## Admin
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `AdminClientsFSHandler` | `admin_clients_fs.go` | Wired | `main.go:L892` | `GET /v1/admin/clients`; `POST /v1/admin/clients`; `PUT/PATCH/DELETE /v1/admin/clients/{clientId}` | JSON(cp.ClientInput); Header(X-Tenant-Slug/ID); Query(tenant/id) | 200 JSON list/obj; 204 | ControlPlane | Clients; Tenants | `ListClients`; `UpsertClient`; `DeleteClient`; `Cluster.Apply` | `cpctx.Provider`; `ClusterNode` | Resolves tenant slug with fallback; supports cluster raft; FS-based | ✅ V2: `/v2/admin/clients` (2025-12-18) |
| `AdminClientsHandler` | `admin_clients.go` | NotWired/Legacy | - | `POST /v1/admin/clients`; `GET /v1/admin/clients`; `GET/PUT/DELETE /v1/admin/clients/{id}`; `POST .../revoke` | JSON(core.Client); Query(tenant/id) | 200/201 JSON; 204 | DataPlane | Clients | `CreateClient`; `ListClients`; `GetClientByID`; `UpdateClient`; `DeleteClient`; `RevokeAllRefreshTokensByClient` | `c.Store` | DB-based implementation unused in V1; mixes routing/logic | ⬜ |
| `AdminConsentsHandler` | `admin_consents.go` | Wired | `main.go:L887` | `GET /v1/admin/consents`; `POST .../upsert`; `POST .../revoke`; `GET .../by-user/{uid}`; `DELETE .../{uid}/{cid}` | JSON(userId, clientId, scopes); Query(active_only) | DataPlane | Consents; Users; Tokens | `UpsertConsent`; `ListConsentsByUser`; `RevokeConsent`; `GetConsent`; `Store.GetClientByClientID`; `Store.RevokeAllRefreshTokens` | `c.ScopesConsents`; `c.Store` | Best-effort token revocation; resolves public clientID to UUID via Store | - | ✅ V2: `/v2/admin/consents` (2025-12-18) |
| `AdminUsersHandler` | `admin_users.go` | Wired | `main.go:L910` | `POST /v1/admin/users/disable`; `POST .../enable`; `POST .../resend-verification` | JSON(user_id, tenant_id, reason, duration) | 204 | Both | Users; Tokens; Email | `DisableUser`; `EnableUser`; `GetUserByID`; `RevokeAllRefreshByUser`; `CreateEmailVerification`; `SendEmail` | `c.Store`; `c.TenantSQLManager`; `c.SenderProvider` | Resolves tenant store dynamic; sends emails inline (danger); mixed slug/uuid usage | ✅ V2: `/v2/admin/users` (2025-12-18) |
| `AdminScopesFSHandler` | `admin_scopes_fs.go` | Wired | `main.go:L880` | `GET/POST/PUT /v1/admin/scopes`; `DELETE /v1/admin/scopes/{name}` | JSON(cp.Scope); Header(X-Tenant-Slug); Query(tenant) | 200 JSON list/obj; 200 {status:ok} | ControlPlane | Scopes | `ListScopes`; `UpsertScope`; `DeleteScope`; `Cluster.Apply` | `cpctx.Provider`; `ClusterNode` | FS-based; Cluster-aware; Inconsistent tenant res (no ID support) | ✅ V2: `/v2/admin/scopes` (2025-12-18) |
| `AdminScopesHandler` | `admin_scopes.go` | NotWired/Legacy | - | `GET/POST /v1/admin/scopes`; `PUT/DELETE /v1/admin/scopes/{id}` | JSON(name, desc); Query(tenant_id) | 200/201 JSON; 204 | DataPlane | Scopes | `ListScopes`; `CreateScope`; `UpdateScopeDescription`; `DeleteScope` | `c.ScopesConsents` | DB-based impl; enforce name validation; "patch-like" PUT | ⬜ |
| `UserProfileHandler` | `admin_users.go` | Wired | `main.go:L1060` | `GET/PUT /v1/admin/users/{id}/profile` | JSON(Metadata) | 200 JSON | DataPlane | Users; Metadata | `GetUserByID`; `UpdateUserMetadata` | `c.Store` | Updates user metadata (profile); Restricted to Admin. | ⬜ |
| `AdminRBACUsersRolesHandler` | `admin_rbac.go` | Wired | `main.go:L899` | `GET/POST /v1/admin/rbac/users/{uid}/roles` | JSON(add, remove) | 200 JSON {user_id, roles} | DataPlane | Users; Roles (RBAC) | `GetUserRoles`; `AssignUserRoles`; `RemoveUserRoles` | `c.Store` (RBAC interfaces) | Uses legacy context interface `ctxCtx`; Type assertions on Store | ✅ V2: `/v2/admin/rbac/users/{uid}/roles` (2025-12-19) |
| `AdminRBACRolePermsHandler` | `admin_rbac.go` | Wired | `main.go:L904` | `GET/POST /v1/admin/rbac/roles/{role}/perms` | JSON(add, remove); Bearer Token (tid claim) | 200 JSON {role, perms} | DataPlane | Roles; Perms (RBAC) | `GetRolePerms`; `AddRolePerms`; `RemoveRolePerms` | `c.Store` (RBAC interfaces); `c.Issuer` | Parses Bearer token internally to get tenant_id | ✅ V2: `/v2/admin/rbac/roles/{role}/perms` (2025-12-19) |

## OIDC/Discovery
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `JWKSHandler` | `jwks.go` | Wired | `main.go:L869` | `GET/HEAD /.well-known/jwks.json`; `GET/HEAD .../jwks/{slug}.json` | Path(slug) | 200 JSON (JWKS) | ControlPlane | Keys (JWKS) | `GetGlobalJWKS`; `GetTenantJWKS` | `h.Cache` (`jwtx.NewJWKSCache`) | Serves cached JWKS; Manual path parsing for slug; Supports HEAD; No-Store headers | ✅ V2: `/.well-known/jwks.json`, `/.well-known/jwks/{slug}.json` (2025-12-19) |
| `OIDCDiscoveryHandler` | `oidc_discovery.go` | Wired | `routes.go:L100` | `GET/HEAD /.well-known/openid-configuration` | - | 200 JSON (OIDC Metadata) | ControlPlane | Issuer; Routes | - | `c.Issuer` | Global discovery; Cache 10m public. | ✅ V2: `/.well-known/openid-configuration` (2025-12-19) |
| `TenantOIDCDiscoveryHandler` | `oidc_discovery.go` | Wired | `main.go:L1096` | `GET/HEAD /t/{slug}/.well-known/openid-configuration` | Path(slug) | 200 JSON (OIDC Metadata) | ControlPlane | Issuer; Routes; Tenants | `GetTenantBySlug` | `c.Issuer`; `cpctx.Provider` | Per-tenant discovery; Manual regex path parsing; Resolves effective issuer mode; No-Store. | ✅ V2: `/t/{slug}/.well-known/openid-configuration` (2025-12-19) |

## Auth
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `ReadyzHandler` | `readyz.go` | Wired | `routes.go:L85`;`main.go:L954` | `GET /readyz` | Header(X-Service-Version)? | 200 JSON (Status) | Both | Cluster; Keys; DB; Redis; Tenants | `Ping` (DB); `ActiveKID/Sign/Verify` (Keys); `Stats` (Cluster/Pools) | `c.ClusterNode`; `c.TenantSQLManager`; `c.Issuer`; `c.Store` | Health check with deep probes (Key self-sign, DB Ping); degrades if FS/DB issues; leaks internal version/commit. | ✅ V2: `/readyz` (2025-12-19) |
| `UserInfoHandler` | `userinfo.go` | Wired | `routes.go:L108` | `GET/POST /userinfo` | Bearer Token | 200 JSON Claims | Both | Users; Scopes; Tenants | `GetUserByID` | `c.Store`; `c.TenantSQLManager`; `cpctx.Provider`; `c.Issuer` | OIDC UserInfo; resolves tenant by issuer path or claim; gates email by scope; includes all custom_fields. | ✅ V2: `/userinfo` (2025-12-19) |
| `AuthLoginHandler` | `auth_login.go` | Wired | `routes.go:L111` | `POST /v1/auth/login` | JSON(tenant/client, email, pwd) | 200 JSON {tokens} | Both | Users; Tokens; Tenants; Clients | `GetUserByEmail`; `CheckPassword`; `CreateRefreshTokenTC`; `GetClient`; `GetTenant` | `c.Store`; `c.Issuer`; `cpctx.Provider` | God-function for login; Mixes FS/DB; Duplicates token issuance | ✅ V2 Core: `/v2/auth/login` (2025-12-19) |
| `AuthRegisterHandler` | `auth_register.go` | Wired | `main.go:L871` | `POST /v1/auth/register` | JSON(tenant_id, client_id, email, password, custom_fields) | 200 JSON {user_id, tokens?}; 4xx; 502 | DataPlane | Users; Identities (Pwd); Tokens; Emails | `CreateUser`; `CreatePasswordIdentity`; `CreateRefreshToken` (legacy/TC); `IssueAccess`; `FSAdminRegister` | Handler loaded with business logic (token issuance, email, blacklist, FS-admin branch). | - | ✅ V2: `/v2/auth/register` (2025-12-22) |
| `AuthRefreshHandler` | `auth_refresh.go` | Wired | `main.go:L872` | `POST /v1/auth/refresh` | JSON(client_id, refresh_token, tenant_id?) | 200 JSON {access_token, refresh_token} | DataPlane | Tokens; Users; Scopes | `GetRefreshTokenByHash`; `CreateRefreshToken` (rotate); `RevokeRefreshToken`; `IssueAccess`; `GetUserByID` | `c.TenantSQLManager`; `c.Issuer`; `claims.Hook` | Mixes DB refresh (stateful) with JWT refresh (stateless admin). Complex tenant resolution. | ✅ V2: `/v2/auth/refresh` (2025-12-22) |
| `AuthLogoutAllHandler` | `auth_logout_all.go` | Wired | `main.go:L876` | `POST /v1/auth/logout-all` | JSON(user_id, client_id?) | 204 No Content | DataPlane | Tokens | `RevokeAllRefreshTokens` | `c.TenantSQLManager` | Revokes only refresh tokens; type assertion `RevokeAllRefreshTokens`. | ✅ V2: `/v2/auth/logout-all` (2025-12-22) |
| `AuthLogoutHandler` | `auth_refresh.go` | Wired | `main.go:L873` | `POST /v1/auth/logout` | JSON(client_id, refresh_token) | 204 No Content | DataPlane | Tokens | `RevokeRefreshByHashTC`; `GetRefreshTokenByHash` | `c.TenantSQLManager` | Idempotent; uses TC interface definition locally. | ✅ V2: `/v2/auth/logout` (2025-12-22) |
| `AuthConfigHandler` | `auth_config.go` | Wired | `main.go:L1031` | `GET /v1/auth/config` | Query(client_id) | 200 JSON Config | Both | Clients; Tenants | `GetClientByClientID`; `ListTenants`; `GetTenantByID/Slug`; `ReadLogo` | `c.Store`; `cpctx.Provider`; FS | O(N) fallback lookup; reads logo from disk inline; mixes SQL/FS sources. | ✅ V2: `/v2/auth/config` (2025-12-24) |
| `ProvidersHandler` | `providers.go` | Wired | `main.go:L1101` | `GET /v1/auth/providers` | Query(tenant_id, client_id, redirect_uri) | 200 JSON (Providers List) | Both | Clients; Providers (Config) | `ValidateRedirectURI` (via Store hook) | `c.Store`; `cfg.Providers` | UI Bootstrap; Checks Google enabled/ready params; Validates redirect_uri using Store adapter; Generates start_url. | ✅ V2: `/v2/auth/providers` (2025-12-24) |
| `PublicFormsHandler` | `public_forms.go` | NotWired | - | `GET /v1/public/tenants/{slug}/forms/{type}` | Path(slug, type) | 200 JSON Config | ControlPlane | Tenants (Settings) | `GetTenantBySlug` | `CPProvider` | Returns login/register form config from tenant settings; Not wired in routes. | ⬜ |
| `CompleteProfileHandler` | `auth_complete_profile.go` | Wired | `main.go:L1019` | `POST /v1/auth/complete-profile` | JSON(custom_fields); Bearer Token | 200 JSON | DataPlane | Users; Schema | `GetUserByID`; `Pool.Query` (Introspect); `Pool.Exec` (Update) | `c.Store`; `c.TenantSQLManager` | Introspects DB schema per request; dynamic SQL; manual auth/jwt parsing. | ✅ V2: `/v2/auth/complete-profile` (2025-12-24) |
| `MeHandler` | `me.go` | Wired | `routes.go:L115` | `GET /v1/me` | Bearer Token | 200 JSON {claims} | Both | Users (Claims) | - | `c.Issuer` | Manual JWT parsing; duplicates RequireAuth logic; returns raw claims. | ✅ V2: `/v2/me` (2025-12-24) |
| `ProfileHandler` | `profile.go` | Wired | `routes.go:L122` | `GET /v1/profile` | Bearer Token (profile:read) | 200 JSON {profile} | DataPlane | Users | `GetUserByID` | `c.Store`; `httpx.GetClaims` | Protected resource; Multi-tenant guard based on tid claim; Best-effort updated_at. | ✅ V2: `/v2/profile` (2025-12-24) |
| (Logic) `ClaimsHook` | `claims_hook.go` | Wired (Internal) | `main.go:L1190` | - | Fn(ctx, event) -> (std, extra) | - | Both | Claims | - | `c.ClaimsHook` | Logic helper used by Login/Refresh/Register/Token handlers to inject claims safely. | ⬜ |

## OAuth
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `OAuthAuthorizeHandler` | `oauth_authorize.go` | Wired | `routes.go:L104` | `GET /oauth2/authorize` | Query(client_id, redirect_uri, scope, response_type, etc); Cookie(session)?; Header(Auth)? | 302 Redirect (code/login); 200 JSON (mfa_required) | Both | Clients; Tenants; Users; Sessions; MFA; Codes | `LookupClient` (Helper); `GetMFATOTP`; `IsTrustedDevice` | `c.Cache` (sid/code/mfa); `c.Store` (Global/Tenant); `c.Issuer` | PKCE required; Step-up MFA logic inline; State machine (Login->MFA->Code); Mixed Tenant/Global store resolution. | ✅ V2: `/oauth2/authorize` (2025-12-30) |
| `OAuthTokenHandler` | `oauth_token.go` | Wired | `routes.go:L105` | `POST /oauth2/token` | Form(grant_type, code, refresh_token, etc) | 200 JSON (Token Response) | Both | Tokens; Users; Clients; Tenants | `LookupClient` (Helper); `Get/Revoke/CreateRefreshTokenTC`; `GetUserByID`; `GetUserRoles/Perms` | `c.Cache` (code); `c.Store`/`c.TenantSQLManager`; `c.Issuer` | Huge monorepo handler; Handles AuthCode, Refresh, ClientCreds; Complex store resolution; Rotating refresh logic. | ✅ V2: `/oauth2/token` (2026-01-12) |
| `ConsentAcceptHandler` | `oauth_consent.go` | Wired | `routes.go:L124` | `POST .../auth/consent/accept` | JSON(consent_token, approve) | 302 Redirect (code/error) | DataPlane | Consents; Codes; Cache | `UpsertConsentTC`; `UpsertConsent` (legacy); `GenerateOpaqueToken` | `c.Cache` (token/code); `c.ScopesConsents` | Consumes one-shot `consent_token`; Persists consent; Emits auth code (hashed in cache). | ✅ V2: `/v2/auth/consent/accept` (2026-01-12) |
| `OAuthIntrospectHandler` | `oauth_introspect.go` | Wired | `routes.go:L107` | `POST /oauth2/introspect` | Form(token); BasicAuth(client) | 200 JSON {active: bool...} | DataPlane | Tokens; Users; Tenants | `GetRefreshTokenByHash`; `KeyfuncFromTokenClaims` | `c.Store` (Global); `c.Issuer`; `cpctx.Provider` | Supports opaque refresh (DB) & JWT (EdDSA); Validates issuer expected mode; Missing client ownership check. | ✅ V2: `/oauth2/introspect` (2025-12-24) |
| `OAuthRevokeHandler` | `oauth_revoke.go` | Wired | `routes.go:L106` | `POST /oauth2/revoke` | Form/JSON(token) | 200 OK | DataPlane | Tokens | `GetRefreshTokenByHash`; `RevokeRefreshToken` | `c.Store` | Revokes only opaque refresh tokens; Idempotent; No client auth required (risk). | ✅ V2: `/oauth2/revoke` (2025-12-24) |
| (Start) | `oauth_start.go` | NotWired/TODO | - | - | - | - | - | - | - | - | File contains only TODO. | ⬜ |
| (Callback) | `oauth_callback.go` | NotWired/TODO | - | - | - | - | - | - | - | - | File contains only TODO. | ⬜ |

## MFA
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `MFAHandler` | `mfa_totp.go` | Wired | `routes.go:L139` | `POST /v1/mfa/totp/enroll`; `.../verify`; `.../challenge`; `.../disable`; `.../mfa/recovery/rotate` | JSON(code, recovery, mfa_token); Header(X-User-ID)* | 200 JSON | DataPlane | Users; MFATOTP; RecoveryCodes; TrustedDevices; Tokens | `UpsertMFATOTP`; `GetMFATOTP`; `ConfirmMFATOTP`; `UpdateMFAUsedAt`; `InsertRecoveryCodes`; `UseRecoveryCode`; `DeleteRecoveryCodes`; `AddTrustedDevice`; `DisableMFATOTP`; `GetUserByID/Email`; `CheckPassword`; `IssueAccess`; `CreateRefreshTokenTC` | `c.Store` (MFA interfaces); `c.Cache`; `c.Issuer`; `c.MultiLimiter` | Monolith handler; /challenge emits tokens; Header auth for sensitive ops (risk); encryption with master key. | ✅ COMPLETADO: 5/5 endpoints en V2 `/v2/mfa/totp/*`. Challenge impl con cache V1 compat. (2026-01-12) |

## Session

| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `DynamicSocialHandler` | `social_dynamic.go` | Wired | `routes.go:L149` | `GET /v1/auth/social/{provider}/{action}` | Path(provider, action); Query(tenant/state) | 302 Redirect (Start/Callback) | Both | Tenants; Users; Identities; Tokens | `GetTenantBySlug`; `Decrypt` (SecretBox); `GetPG` (Pool); `UpsertUser/Identity` | `cpctx.Provider`; `c.TenantSQLManager`; `c.Issuer` | Routes social flows by path; Resolves tenant config at runtime; Mixes FS config with DB user provisioning; Handles OIDC exchange. | ⬜ |
| `GoogleHandler` | `social_google.go` | Impl (via Dynamic) | - | - | Query(tid, cid, redir); State(JWT) | 302 Redirect (Google/Client) | Both | Users; Identities; Tokens; MFA | `EnsureUserAndIdentity` (Pool); `InsertRefreshToken` (Pool); `GetMFATOTP` | `h.pool`; `c.Store` (Global/RBAC); `google.OIDC` | Implements Start/Callback logic; Emits tokens/login_code; Checks MFA; Mixed Global/Tenant store usage (Risk). | ⬜ |
| `SocialExchangeHandler` | `social_exchange.go` | Wired | `routes.go:L146` | `POST /v1/auth/social/exchange` | JSON(code, client_id, tenant_id?) | 200 JSON (Tokens) | Both | Cache (Code) | `Get` (social:code); `Delete` (social:code) | `c.Cache` | One-shot exchange for login_code; Validates client_id/tenant_id match; No rate limit on body. | ✅ V2: `/v2/auth/social/exchange` (2025-12-24) |
| `SocialResultHandler` | `social_result.go` | Wired (Conditional) | `main.go:L1112` | `GET /v1/auth/social/result` | Query(code, peek) | 200 HTML/JSON | Both | Cache (Code) | `Get` (social:code); `Delete` (social:code) | `c.Cache` | Debug/Viewer for login_code; HTML with postMessage('*') (Risk); Peek mode for replay. | ✅ V2: `/v2/auth/social/result` (2025-12-30) |
| `SessionLoginHandler` | `session_login.go` | Wired | `routes.go:L120` | `POST /v1/session/login` | JSON(email, password, tenant_id?) | 200 JSON/Cookie | Both | Users; Identities; Sessions | `CheckPasswordIdentity`; `CreateSession`; `GetTenantBySlug` (FS) | `c.Store` (Global/Tenant); `c.Auth` | Fallback logic global vs tenant store; cookies handling | ✅ V2: `/v2/session/login` (2025-12-30) |
| `SessionLogoutHandler` | `session_logout_util.go` | Wired | `routes.go:L121` | `POST /v1/session/logout` | Cookie(sid) | 200 JSON/Cookie | Both | Sessions | `DeleteSession` | `c.Store` | Clears cookie | ✅ V2: `/v2/session/logout` (2025-12-24) |
| (Util) | `session_logout_util.go` | - | - | - | - | - | - | - | - | - | Helper file (tokensSHA256 duplicado). Not a handler. | - |

## EmailFlows
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `EmailFlowsHandler` | `email_flows.go` | Wired | `routes.go:L127` | `POST .../verify-email/start`; `GET .../verify-email`; `POST .../forgot`; `POST .../reset` | JSON(DTOs); Query(token) | 204; 200 JSON; 302 | Both | Users; Tokens; Email; Tenants | `LookupUserIDByEmail`; `CreateEmailVerification`; `UseEmailVerification`; `SetEmailVerified`; `CreatePasswordReset`; `UsePasswordReset`; `UpdatePasswordHash`; `RevokeAllRefreshTokens` | `c.Users`; `c.Tokens`; `c.SenderProvider` | Template Method style; Mixed HTTP/Business; Soft-fail mail send | ✅ V2: `/v2/auth/verify-email/*`, `/v2/auth/forgot`, `/v2/auth/reset` (2025-12-30) |

## CSRF
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `CSRFGetHandler` | `csrf.go` | Wired | `routes.go:L136` | `GET /v1/csrf` | - | 200 JSON {csrf_token} | System | - | - | `crypto/rand` | Double-submit cookie pattern; Hardcoded Secure=false | ✅ V2: `/v2/csrf` (2025-12-24) |

## Admin/Tenants
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `SendTestEmail` | `admin_mailing.go` | Wired (Helper) | `admin_tenants_fs.go` | `POST .../mailing/test` (via AdminTenants) | JSON(to, smtp?) | 200 {status:ok} | Both | Email | `SendEmail` | `email.NewSMTPSender`; `os.Getenv(SIGNING_MASTER_KEY)` | Helper func; decrypts SMTP password using env var; direct SMTP send | ✅ V2: `/v2/admin/mailing/test` (2025-12-30) |
| `AdminTenantsFSHandler` | `admin_tenants_fs.go` | Wired | `main.go:L1053` | `GET/POST /v1/admin/tenants`; `.../tenants/{slug}`; `.../settings`; `.../keys/rotate`; `.../migrate`; `.../user-store/migrate`; `.../schema/apply`; `.../users`; `.../infra-stats` | JSON(Tenant, Settings, Users, DSN); Query(graceSeconds) | 200/201 JSON; 204 | Both | Tenants; Users; Keys; Schema; Cache | `UpsertTenant`; `UpdateSettings`; `MigrateTenant`; `RotateKeys`; `ApplySchema`; `GetStats`; `CreateUser`; `UpdateUser` | `cpctx.Provider`; `c.ClusterNode`; `c.TenantSQLManager`; `c.Issuer` | "God Handler"; manual mux; mixes CP/DP; partial cluster support; duplicate user logic. | ⬜ |
| `AdminUsersHandler` | `admin_users.go` | Wired | `main.go:L1060` | `POST /v1/admin/users/disable`; `.../enable`; `.../resend-verification` | JSON(user_id, tenant_id, reason, duration) | 204 No Content | DataPlane | Users; Tokens; Emails | `DisableUser`; `EnableUser`; `RevokeAllRefreshByUser`; `CreateEmailVerification`; `SendEmail` | `c.Store`; `c.TenantSQLManager`; `cpctx.Provider` (templates) | Actions not CRUD; mixes Slug/UUID resolution; sends logic-heavy emails inline. | ✅ V2: `/v2/admin/users/disable`, `/v2/admin/users/enable`, `/v2/admin/users/resend-verification` (2025-12-30) |

## Admin/Infra
| HandlerID | File | WiredStatus | WiredEvidence | RoutesHandled | Inputs | Outputs | Plane | EntitiesTouched | DataOps | Dependencies | Notes | Migration |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| (none) | `admin_keys.go` | NotWired/Legacy | - | - | - | - | - | - | - | - | Deprecated; empty file | - |

## DATA_LAYER_CAPABILITIES (Delta)
| Capability | Methods/Ops | UsedByHandlers | Notes |
| :--- | :--- | :--- | :--- |
| ClientRepo[CP] | `ListClients`; `UpsertClient`; `GetClient`; `DeleteClient` | `AdminClientsFSHandler` | Via `cpctx.Provider` |
| TenantRepo[CP] | `GetTenantByID` | `AdminClientsFSHandler` | For slug resolution fallback |
| ClusterApplier | `Apply(Mutation)` | `AdminClientsFSHandler` | For raft replication |
| ClientRepo[DP] | `Create`; `List`; `Get`; `Update`; `Delete`; `RevokeTokens`; `GetByClientID` | `AdminClientsHandler`; `AdminConsentsHandler` | `AdminClientsHandler` is legacy; Consents uses for lookup |
| ConsentRepo[DP] | `UpsertConsent`; `ListConsentsByUser`; `RevokeConsent`; `GetConsent` | `AdminConsentsHandler` | - |
| TokenRepo[DP] | `RevokeAllRefreshTokens` | `AdminConsentsHandler`; `AdminClientsHandler` | Best-effort in Consents |
| RBACRepo[DP] | `GetUserRoles`; `AssignUserRoles`; `RemoveUserRoles`; `GetRolePerms`; `AddRolePerms`; `RemoveRolePerms` | `AdminRBACUsersRolesHandler`; `AdminRBACRolePermsHandler` | Interface assertion on Store |
| ScopeRepo[CP] | `ListScopes`; `UpsertScope`; `DeleteScope` | `AdminScopesFSHandler` | Via `cpctx.Provider` |
| ScopeRepo[DP] | `ListScopes`; `CreateScope`; `UpdateScopeDescription`; `DeleteScope` | `AdminScopesHandler` | Not wired |
| UserRepo[DP] | `GetUserByID`; `DisableUser`; `EnableUser`; `GetUserByEmail`; `CheckPassword` | `AdminUsersHandler`; `AuthLoginHandler`; `CompleteProfileHandler`; `AuthRefreshHandler` | Login uses TenantSQLManager; Admin uses Store/Manager dynamic |
| TokenRepo[DP] | `RevokeAllRefreshTokens`; `RevokeAllRefreshByUser`; `CreateEmailVerification`; `CreateRefreshTokenTC` | `AdminConsentsHandler`; `AdminUsersHandler`; `AuthLoginHandler` | Inconsistent revoke methods; TC = Tenant Context? |
| ClientRepo[DP] | `GetClientByClientID` | `AuthConfigHandler`; `AuthLoginHandler` | Used for config & login gating |
| MFARepo[DP] | `GetMFATOTP`; `IsTrustedDevice` | `AuthLoginHandler` | Optional interfaces on Repo |
| SchemaRepo[DP] | `IntrospectColumns` | `CompleteProfileHandler` | Ad-hoc query to information_schema |
| Mailer[Tenant] | `SendEmail` | `SendTestEmail`; `AdminUsersHandler` | Direct or Provider |
| TokenRepo[DP] | `GetRefreshTokenByHash`; `CreateRefreshTokenTC`; `RevokeRefreshToken`; `RevokeRefreshByHashTC` | `AuthRefreshHandler`; `AuthLogoutHandler` | Complex rotation logic; multiple interfaces (legacy vs TC) |
| UserRepo[DP] | `CreateUser`; `CreatePasswordIdentity`; `UpdateUser`; `DisableUser`; `EnableUser`; `UpdateUserProfile` | `AuthRegisterHandler`; `AdminTenantsFSHandler`; `AdminUsersHandler`; `CompleteProfileHandler` | AdminTenants duplicates CreateUser; CompleteProfile uses dynamic SQL |
| FSAdmin[Global] | `Register` | `AuthRegisterHandler` | Logic inside handler (helper `FSAdminRegister`) |
| TenantManager[CP] | `MigrateTenant`; `ApplySchema`; `GetStats`; `Ping` | `AdminTenantsFSHandler` | Infra ops mixed in handler |
| KeyManager[CP] | `RotateFor` | `AdminTenantsFSHandler` | Issuer keys rotation |
| SchemaRepo[DP] | `Introspect` | `CompleteProfileHandler` | Explicit SQL query to information_schema per request |
| ClientRepo[DP] | `GetClientByClientID`; `GetClient` (FS) | `AuthConfigHandler` | Mixed SQL/FS lookup/fallback |
| UserRepo[DP] | `LookupUserIDByEmail`; `SetEmailVerified`; `UpdatePasswordHash`; `RevokeAllRefreshTokens` | `EmailFlowsHandler` | - |
| TokenRepo[DP] | `CreateEmailVerification`; `UseEmailVerification`; `CreatePasswordReset`; `UsePasswordReset` | `EmailFlowsHandler` | - |
| Mailer[Tenant] | `Send` (Verify/Reset) | `EmailFlowsHandler` | - |
| TenantRepo[CP] | `GetTenantBySlug` | `EmailFlowsHandler`; `AuthLoginHandler` | - |
| UserRepo[DP] | `GetUserByEmail`; `CheckPassword` | `AuthLoginHandler` | - |
| TokenRepo[DP] | `CreateRefreshTokenTC` | `AuthLoginHandler` | - |
| KeyManager[Shared] | `GetGlobal`; `GetByTenant` | `JWKSHandler` | Cached internally by `jwtx.JWKSCache` |
| MFARepo[DP] | `UpsertMFATOTP`; `GetMFATOTP`; `ConfirmMFATOTP`; `UpdateMFAUsedAt`; `DisableMFATOTP`; `InsertRecoveryCodes`; `UseRecoveryCode`; `DeleteRecoveryCodes` | `MFAHandler` | Feature detection on Store |
| DeviceRepo[DP] | `AddTrustedDevice`; `IsTrustedDevice` | `MFAHandler`; `OAuthAuthorizeHandler` | Feature detection |
| Cache[Shared] | `Get/Set/Delete` (code, sid, mfa_req) | `OAuthAuthorizeHandler`; `MFAHandler` | Critical for auth flows state |
| ClientRepo[DP] | `LookupClient` (Helper) | `OAuthAuthorizeHandler` | Wraps Store/FS fallback logic |
| ConsentRepo[DP] | `UpsertConsentTC`; `UpsertConsent` | `ConsentAcceptHandler` | TC version preferred |
| TokenRepo[DP] | `GetRefreshTokenByHash`; `RevokeRefreshToken`; `CreateRefreshTokenTC`; `RevokeRefreshTokensByUserClientTC`; `GetRefreshTokenByHashTC` | `OAuthIntrospectHandler`; `OAuthRevokeHandler`; `OAuthTokenHandler` | Critical: inconsistent hash usage (hex vs base64url) |
| Cache[Shared] | `Get/Delete` (consent_challenge); `Set` (auth_code); `Get/Delete` (code:xxx); `Get/Delete` (social:code) | `ConsentAcceptHandler`; `OAuthTokenHandler`; `SocialExchangeHandler`; `SocialResultHandler` | Inconsistent key prefix (oidc:code: vs code: vs social:code:) |
| ClusterRepo[Shared] | `Stats`; `IsLeader`; `LeaderID` | `ReadyzHandler` | Diag only |
| Issuer[Shared] | `IssueAccessForTenant`; `IssueIDTokenForTenant`; `ActiveKID`; `SignRaw`; `Keyfunc` | `OAuthTokenHandler`; `ReadyzHandler`; `DynamicSocialHandler`; `UserInfoHandler` | Internal logic; Readyz uses it for self-check; Social uses it for state signing; UserInfo validates with it |
| SecretBox[CP] | `Decrypt` | `DynamicSocialHandler` | Decrypts provider secrets from tenant config |
| RedirectAllowlist[Shared] | `Check` | `SessionLogoutHandler` | In-memory map check |
| UserRepo[DP] | `GetUserByID` | `UserInfoHandler` | Reads user claims |
| IdentityRepo[DP] | `EnsureUserAndIdentity`; `InsertRefreshToken` | `GoogleHandler` | Provisions social users; inserts tokens manually via SQL |

## ROUTES_DELTA
- `GET /userinfo`, `POST /userinfo` (UserInfoHandler) - Not in master list? (Check required)
- `POST /v1/auth/social/exchange` (SocialExchange)
- `GET /v1/auth/social/result` (SocialResult)
- `POST /v1/admin/tenants/{slug}/mailing/test` (AdminTenantsFS)
- `PUT /v1/admin/tenants/{slug}/clients/{clientID}` (AdminTenantsFS)
- `PUT /v1/admin/tenants/{slug}/scopes` (AdminTenantsFS - Bulk)
- `GET /v1/admin/tenants/{slug}/infra-stats` (AdminTenantsFS)

## ROUTE_MISMATCHES
- `admin_clients.go` routes match standard REST but are unused.
- `admin_clients_fs.go` handles clients by `clientId` (string/public) not UUID, reflecting CP nature.
- `POST /v1/admin/tenants/{slug}/migrate` vs `/user-store/migrate` (Duplicate endpoints in AdminTenantsFS)
