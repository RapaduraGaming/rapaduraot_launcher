@echo off
:: Requires Go installed: https://go.dev/dl/
:: gonutz/wui is pure Go - no MinGW needed.

cd /d "%~dp0"

echo Baixando dependencias...
go mod tidy

echo Compilando launcher...
go build -ldflags="-H windowsgui -s -w" -o ..\RapaduraOTLauncher.exe .

if %ERRORLEVEL% == 0 (
    echo.
    echo Sucesso! RapaduraOTLauncher.exe gerado.
) else (
    echo.
    echo Erro na compilacao.
    pause
)
