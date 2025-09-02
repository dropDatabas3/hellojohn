@echo off
setlocal EnableExtensions EnableDelayedExpansion

REM ── Defaults ────────────────────────────────────────────
set "TENANT=77407778-1e19-4623-a61a-0593ebcfa08e"
set "CLIENT=web-frontend"
set "BASE=http://localhost:8080"
set "RATE_BURST=0"
set "RealEmail=john@gmail.com"
set "AUTOMAIL=0"     REM 0/1, true/false, yes/no, auto
set "USEREAL=0"      REM 0/1, true/false, yes/no
set "EMAILTAG="      REM opcional (ej: prueba123)

REM ── Overrides por argumentos ───────────────────────────
if not "%~1"=="" set "TENANT=%~1"
if not "%~2"=="" set "CLIENT=%~2"
if not "%~3"=="" set "BASE=%~3"
if not "%~4"=="" set "RATE_BURST=%~4"
if not "%~5"=="" set "RealEmail=%~5"
if not "%~6"=="" set "AUTOMAIL=%~6"
if not "%~7"=="" set "USEREAL=%~7"
if not "%~8"=="" set "EMAILTAG=%~8"

REM ── Normalizar booleanos (1/0) ─────────────────────────
set "AUTOFLAG=0"
for %%A in (1 true yes on y si auto) do (
  if /I "%AUTOMAIL%"=="%%A" set "AUTOFLAG=1"
)

set "USEREALFLAG=0"
for %%A in (1 true yes on y si) do (
  if /I "%USEREAL%"=="%%A" set "USEREALFLAG=1"
)

REM ── Armar switches sólo si están activos ───────────────
set "SW_AUTOMAIL="
if "%AUTOFLAG%"=="1" set "SW_AUTOMAIL=-AutoMail"

set "SW_USEREAL="
if "%USEREALFLAG%"=="1" set "SW_USEREAL=-UseRealEmail"

echo [info] BASE=%BASE%  TENANT=%TENANT%  CLIENT=%CLIENT%  EMAIL=%RealEmail%  AUTO=%AUTOFLAG%  REAL=%USEREALFLAG%  TAG=%EMAILTAG%

REM ── Llamar a PowerShell ─────────────────────────────────
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0test.ps1" ^
  -TenantId "%TENANT%" ^
  -ClientId "%CLIENT%" ^
  -Base "%BASE%" ^
  -RateBurst %RATE_BURST% ^
  -RealEmail "%RealEmail%" ^
  %SW_AUTOMAIL% ^
  %SW_USEREAL% ^
  -EmailTag "%EMAILTAG%"

set "ERR=%ERRORLEVEL%"

if NOT "%ERR%"=="0" (
  echo [FAIL] Smoke/contract tests fallaron con codigo %ERR%
  exit /b %ERR%
) else (
  echo [OK] Tests completos
  exit /b 0
)
