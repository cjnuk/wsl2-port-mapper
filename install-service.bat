@echo off
REM WSL2 Port Forwarder Service Installation Script
REM Requires Administrator privileges

echo WSL2 Port Forwarder Service Installer
echo =====================================

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
set "SERVICE_EXE=%SERVICE_DIR%wsl2-port-forwarder.exe"
set "CONFIG_FILE=%SERVICE_DIR%wsl2-config.json"
set "NSSM_EXE=%SERVICE_DIR%nssm.exe"
set "LOG_DIR=%SERVICE_DIR%logs"

echo Current directory: %SERVICE_DIR%
echo.

REM Verify all required files exist
if not exist "%SERVICE_EXE%" (
    echo ERROR: wsl2-port-forwarder.exe not found in current directory
    pause
    exit /b 1
)

if not exist "%CONFIG_FILE%" (
    echo ERROR: wsl2-config.json not found in current directory
    pause
    exit /b 1
)

if not exist "%NSSM_EXE%" (
    echo ERROR: nssm.exe not found in current directory
    pause
    exit /b 1
)

echo All required files found.
echo.

REM Create logs directory
if not exist "%LOG_DIR%" (
    echo Creating logs directory...
    mkdir "%LOG_DIR%"
)

REM Stop and remove existing service if it exists
echo Checking for existing service...
sc query "%SERVICE_NAME%" >nul 2>&1
if %errorLevel% equ 0 (
    echo Stopping existing service...
    "%NSSM_EXE%" stop "%SERVICE_NAME%"
    timeout /t 3 /nobreak >nul
    
    echo Removing existing service...
    "%NSSM_EXE%" remove "%SERVICE_NAME%" confirm
)

REM Install service
echo Installing WSL2 Port Forwarder service...
"%NSSM_EXE%" install "%SERVICE_NAME%" "%SERVICE_EXE%" "%CONFIG_FILE%"

if %errorLevel% neq 0 (
    echo ERROR: Failed to install service
    pause
    exit /b 1
)

REM Configure service settings
echo Configuring service settings...
"%NSSM_EXE%" set "%SERVICE_NAME%" DisplayName "WSL2 Port Forwarder"
"%NSSM_EXE%" set "%SERVICE_NAME%" Description "Automatically manages port forwarding for WSL2 instances"
"%NSSM_EXE%" set "%SERVICE_NAME%" Start SERVICE_AUTO_START
"%NSSM_EXE%" set "%SERVICE_NAME%" AppExit Default Restart
"%NSSM_EXE%" set "%SERVICE_NAME%" AppRestartDelay 5000
"%NSSM_EXE%" set "%SERVICE_NAME%" AppStdout "%LOG_DIR%\service-output.log"
"%NSSM_EXE%" set "%SERVICE_NAME%" AppStderr "%LOG_DIR%\service-error.log"
"%NSSM_EXE%" set "%SERVICE_NAME%" AppRotateFiles 1
"%NSSM_EXE%" set "%SERVICE_NAME%" AppRotateOnline 1
"%NSSM_EXE%" set "%SERVICE_NAME%" AppRotateBytes 1048576

echo Service installed successfully!
echo.

REM Ask user if they want to start the service immediately
set /p START_NOW="Start the service now? (y/n): "
if /i "%START_NOW%"=="y" (
    echo Starting service...
    "%NSSM_EXE%" start "%SERVICE_NAME%"
    timeout /t 2 /nobreak >nul
    
    REM Check service status
    sc query "%SERVICE_NAME%" | find "RUNNING" >nul
    if %errorLevel% equ 0 (
        echo Service started successfully!
    ) else (
        echo WARNING: Service may not have started correctly.
        echo Check the logs in: %LOG_DIR%
    )
)

echo.
echo Installation complete!
echo.
echo Service Management Commands:
echo   Start service:   nssm start %SERVICE_NAME%
echo   Stop service:    nssm stop %SERVICE_NAME%
echo   Restart service: nssm restart %SERVICE_NAME%
echo   Service status:  sc query %SERVICE_NAME%
echo.
echo Log files location: %LOG_DIR%
echo   Output log: service-output.log
echo   Error log:  service-error.log
echo.
echo To uninstall: run uninstall-service.bat
pause