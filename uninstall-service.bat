@echo off
REM WSL2 Port Forwarder Service Uninstallation Script
REM Requires Administrator privileges

echo WSL2 Port Forwarder Service Uninstaller
echo ========================================

REM Check for Administrator privileges
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ERROR: This script requires Administrator privileges.
    echo Please run as Administrator.
    pause
    exit /b 1
)

REM Set current directory as base path
set "SERVICE_DIR=%~dp0"
set "SERVICE_NAME=WSL2PortForwarder"
set "NSSM_EXE=%SERVICE_DIR%nssm.exe"
set "LOG_DIR=%SERVICE_DIR%logs"

echo Current directory: %SERVICE_DIR%
echo.

REM Verify NSSM exists
if not exist "%NSSM_EXE%" (
    echo ERROR: nssm.exe not found in current directory
    pause
    exit /b 1
)

REM Check if service exists
echo Checking for WSL2PortForwarder service...
sc query "%SERVICE_NAME%" >nul 2>&1
if %errorLevel% neq 0 (
    echo Service "%SERVICE_NAME%" is not installed.
    echo Nothing to uninstall.
    pause
    exit /b 0
)

echo Service found. Proceeding with uninstallation...
echo.

REM Stop the service if running
echo Stopping service...
"%NSSM_EXE%" stop "%SERVICE_NAME%"

REM Wait a moment for service to stop
timeout /t 3 /nobreak >nul

REM Remove the service
echo Removing service...
"%NSSM_EXE%" remove "%SERVICE_NAME%" confirm

if %errorLevel% equ 0 (
    echo Service removed successfully!
) else (
    echo WARNING: There may have been an issue removing the service.
    echo You can try removing it manually with: sc delete %SERVICE_NAME%
)

echo.

REM Ask if user wants to remove log files
set /p REMOVE_LOGS="Remove log files? (y/n): "
if /i "%REMOVE_LOGS%"=="y" (
    if exist "%LOG_DIR%" (
        echo Removing log files...
        rmdir /s /q "%LOG_DIR%" 2>nul
        if exist "%LOG_DIR%" (
            echo WARNING: Could not remove all log files. You may need to delete manually.
            echo Log directory: %LOG_DIR%
        ) else (
            echo Log files removed.
        )
    ) else (
        echo No log directory found.
    )
)

echo.
echo Uninstallation complete!
echo.
echo The following files remain and can be deleted manually if desired:
echo   - wsl2-port-forwarder.exe
echo   - wsl2-config.json  
echo   - nssm.exe
echo   - install-service.bat
echo   - uninstall-service.bat
echo   - check-service.bat
echo.
pause