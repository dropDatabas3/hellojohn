@echo off
setlocal EnableExtensions EnableDelayedExpansion

:: ──────────────────────────────────────────────────────────────────
:: Dumpea todo el proyecto: ruta del archivo + contenido (o VACIO)
:: Ejecutar en la raíz del repo. Ej:
::   C:\Users\Usuario\Desktop\Universal Login\hellojohn> script.bat
::
:: Tip: si querés mandarlo a un archivo:
::   script.bat > proyecto_dump.txt
:: ──────────────────────────────────────────────────────────────────

:: Modo UTF-8 en consola (opcional; comentá si te molesta)
chcp 65001 >NUL

:: Extensiones binarias a ignorar (podés sumar/sacar)
set "BINARY_EXT=.exe .dll .bin .iso .img .gif .sum .png .jpg .jpeg .ico .pdf .zip .tar .gz .7z .rar .mp3 .wav .ogg .mp4 .mov .avi .heic .ttf .woff .woff2 .eot .class .jar"

:: Directorios a ignorar (opcional). Descomentá la línea del bloque IF más abajo para usarlos.
set "SKIP_DIRS=.git node_modules vendor dist build bin obj .idea .vscode .venv go.sum test.bat test.ps1"

for /r "%CD%" %%F in (*) do (
    set "SKIP=0"
    set "EXT=%%~xF"
    set "DIR=%%~dpF"

    :: Saltar directorios pesados (opcional)
    for %%D in (%SKIP_DIRS%) do (
        echo(!DIR!| findstr /I "\\%%D\\">NUL && set "SKIP=1"
    )

    :: Saltar binarios por extensión
    for %%E in (%BINARY_EXT%) do (
        if /I "![EXT]!"=="%%E" set "SKIP=1"
    )

    if "!SKIP!"=="0" (
        echo %%~fF:
        if %%~zF EQU 0 (
            echo VACIO
        ) else (
            type "%%~fF"
        )
        echo.
        echo ------------------------------------------------------------
        echo.
    )
)

endlocal
