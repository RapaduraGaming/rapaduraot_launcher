@echo off
:: Requires Go installed: https://go.dev/dl/
:: Pure Go - no MinGW/CGO needed.

cd /d "%~dp0"

echo Instalando go-winres (ícone e manifest no executável)...
go install github.com/tc-hib/go-winres@latest

echo Gerando recursos Windows (icon + DPI manifest)...
go-winres make --in winres\winres.json --out rsrc_windows_amd64.syso
if %ERRORLEVEL% neq 0 (
    echo Erro ao gerar recursos. Continuando sem ícone...
)

echo Baixando dependencias...
go mod tidy

echo Compilando launcher...
go build -ldflags="-H windowsgui -s -w" -o ..\RapaduraOTLauncher.exe .

if %ERRORLEVEL% == 0 (
    echo.
    echo Sucesso! RapaduraOTLauncher.exe gerado.
    del /f /q rsrc_windows_amd64.syso 2>nul
) else (
    echo.
    echo Erro na compilacao.
    pause
)
