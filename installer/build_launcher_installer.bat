@echo off
:: Builds RapaduraOT-Setup.exe (launcher-only installer).
:: Requires NSIS: https://nsis.sourceforge.io/

cd /d "%~dp0"

set MAKENSIS=makensis.exe
where makensis >NUL 2>&1
if %ERRORLEVEL% neq 0 set MAKENSIS=%ProgramFiles%\NSIS\makensis.exe
if not exist "%MAKENSIS%" set MAKENSIS=%ProgramFiles(x86)%\NSIS\makensis.exe

if not exist "%MAKENSIS%" (
    echo ERROR: makensis.exe not found. Install NSIS from https://nsis.sourceforge.io/
    pause
    exit /b 1
)

if not exist "..\RapaduraOTLauncher.exe" (
    echo ERROR: RapaduraOTLauncher.exe not found. Build with launcher\build.bat first.
    pause
    exit /b 1
)

echo Building RapaduraOT-Setup.exe...
"%MAKENSIS%" launcher_installer.nsi

if %ERRORLEVEL% == 0 (echo. & echo Done: RapaduraOT-Setup.exe generated.) else (echo. & echo Build failed. & pause)