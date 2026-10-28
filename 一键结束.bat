@echo off
setlocal
cd /d "%~dp0"
set "NO_PAUSE=0"
if /I "%~1"=="--no-pause" set "NO_PAUSE=1"

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\stop-canvas.ps1"
set "exit_code=%errorlevel%"

if not "%exit_code%"=="0" (
    echo.
    echo Failed to stop open-ai-canvas. See the error above.
    pause
    exit /b %exit_code%
)

echo.
echo open-ai-canvas stopped successfully.
if "%NO_PAUSE%"=="0" pause
exit /b 0
