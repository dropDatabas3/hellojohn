@echo off
REM Script para crear estructura de login-svc
set BASE=login-svc

REM Crear carpetas
mkdir %BASE%
mkdir %BASE%\cmd\login-svc
mkdir %BASE%\configs
mkdir %BASE%\deployments
mkdir %BASE%\internal\app
mkdir %BASE%\internal\audit
mkdir %BASE%\internal\claims\resolver
mkdir %BASE%\internal\config
mkdir %BASE%\internal\grpc
mkdir %BASE%\internal\http\handlers
mkdir %BASE%\internal\jwt
mkdir %BASE%\internal\oauth
mkdir %BASE%\internal\rate
mkdir %BASE%\internal\store\pg

REM Crear archivos vacíos
type nul > %BASE%\cmd\login-svc\main.go
type nul > %BASE%\configs\config.example.yaml
type nul > %BASE%\deployments\docker-compose.yml
type nul > %BASE%\deployments\Dockerfile

type nul > %BASE%\internal\app\app.go
type nul > %BASE%\internal\audit\audit.go
type nul > %BASE%\internal\claims\cel_engine.go
type nul > %BASE%\internal\claims\jsonschema.go
type nul > %BASE%\internal\claims\resolver\resolver.go
type nul > %BASE%\internal\claims\resolver\provider.go
type nul > %BASE%\internal\claims\resolver\webhook.go
type nul > %BASE%\internal\claims\resolver\static.go
type nul > %BASE%\internal\claims\resolver\expr.go
type nul > %BASE%\internal\config\config.go
type nul > %BASE%\internal\grpc\server.go
type nul > %BASE%\internal\http\server.go
type nul > %BASE%\internal\http\middleware.go
type nul > %BASE%\internal\http\routes.go
type nul > %BASE%\internal\http\handlers\auth_login.go
type nul > %BASE%\internal\http\handlers\auth_refresh.go
type nul > %BASE%\internal\http\handlers\oauth_start.go
type nul > %BASE%\internal\http\handlers\oauth_callback.go
type nul > %BASE%\internal\http\handlers\jwks.go
type nul > %BASE%\internal\http\handlers\registry_clients.go
type nul > %BASE%\internal\jwt\issuer.go
type nul > %BASE%\internal\jwt\jwks.go
type nul > %BASE%\internal\jwt\keys.go
type nul > %BASE%\internal\oauth\google.go
type nul > %BASE%\internal\oauth\facebook.go
type nul > %BASE%\internal\rate\limiter.go
type nul > %BASE%\internal\store\pg\store.go
type nul > %BASE%\internal\store\pg\migrator.go

type nul > %BASE%\.env.example

echo Estructura creada en %BASE%
pause
