@echo off
setlocal

set "PROJECT_ROOT=%~dp0"
set "START_SCRIPT=%PROJECT_ROOT%scripts\start-canvas.ps1"
set "POWERSHELL_EXE=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"

if not exist "%START_SCRIPT%" (
    echo [ERROR] Startup script not found: "%START_SCRIPT%"
    pause
    exit /b 1
)

if not exist "%POWERSHELL_EXE%" (
    echo [ERROR] Windows PowerShell not found: "%POWERSHELL_EXE%"
    pause
    exit /b 1
)

echo Starting AI Canvas...
set "START_ARGS="
if /I "%~1"=="--no-browser" set "START_ARGS=-NoBrowser"
"%POWERSHELL_EXE%" -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%START_SCRIPT%" %START_ARGS%

if errorlevel 1 (
    echo.
    echo [ERROR] Startup failed. Review the message above.
    pause
    exit /b 1
)

endlocal
exit /b 0
