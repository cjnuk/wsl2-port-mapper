@echo off
REM WSL2 Port Forwarder Service Status Checker

echo WSL2 Port Forwarder Service Status
echo ===================================

set "SERVICE_NAME=WSL2PortForwarder"
set "LOG_DIR=%~dp0logs"

echo Service Status:
echo --------------
sc query "%SERVICE_NAME%" 2>nul
if %errorLevel% neq 0 (
    echo Service "%SERVICE_NAME%" is not installed.
    echo Run install-service.bat to install.
    goto :show_wsl_instances
)

echo.
echo Current Port Forwarding Rules:
echo -----------------------------
netsh interface portproxy show v4tov4
if %errorLevel% neq 0 (
    echo No port forwarding rules found or access denied.
)

:show_wsl_instances
echo.
echo Running WSL2 Instances:
echo ----------------------
wsl --list --running --quiet 2>nul
if %errorLevel% neq 0 (
    echo WSL command failed or no instances running.
)

echo.
echo Recent Service Output (last 10 lines):
echo -------------------------------------
if exist "%LOG_DIR%\service-output.log" (
    powershell -Command "Get-Content '%LOG_DIR%\service-output.log' -Tail 10"
) else (
    echo No output log file found.
)

echo.
echo Recent Service Errors (last 10 lines):
echo -------------------------------------
if exist "%LOG_DIR%\service-error.log" (
    powershell -Command "Get-Content '%LOG_DIR%\service-error.log' -Tail 10"
) else (
    echo No error log file found.
)

echo.
echo Log Files Location: %LOG_DIR%
echo.
pause