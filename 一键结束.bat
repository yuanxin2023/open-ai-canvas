@echo off
setlocal
cd /d "%~dp0"

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\stop-local.ps1"
set "exit_code=%errorlevel%"

if not "%exit_code%"=="0" (
    echo.
    echo Failed to stop open-ai-canvas. See the error above.
    pause
    exit /b %exit_code%
)

echo.
echo open-ai-canvas stopped successfully.
pause
exit /b 0
