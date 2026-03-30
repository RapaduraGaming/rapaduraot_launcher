@echo off
:: Builds RapaduraOTLauncher.exe
:: Requires Go: https://go.dev/dl/
:: The Windows resource file (icon + manifest) is pre-generated.
:: To regenerate it, run: go-winres make --in winres\winres.json

cd /d "%~dp0"

echo Baixando dependencias...
go mod tidy

echo Compilando launcher...
go build -ldflags="-H windowsgui -s -w" -o RapaduraOTLauncher.exe .

if %ERRORLEVEL% == 0 (
    echo.
    echo Sucesso! RapaduraOTLauncher.exe gerado.
) else (
    echo.
    echo Erro na compilacao.
    pause
)