@echo off
setlocal EnableExtensions

set "ROOT_DIR=%~dp0"
if "%ROOT_DIR:~-1%"=="\" set "ROOT_DIR=%ROOT_DIR:~0,-1%"

set "TARGET_ARCH=%~1"
if "%TARGET_ARCH%"=="" set "TARGET_ARCH=amd64"

if /I not "%TARGET_ARCH%"=="amd64" if /I not "%TARGET_ARCH%"=="arm64" (
    echo [ERROR] Unsupported target arch "%TARGET_ARCH%". Use amd64 or arm64.
    exit /b 1
)

where go >nul 2>nul
if errorlevel 1 (
    echo [ERROR] go was not found in PATH.
    exit /b 1
)

echo.
echo [GENERATE] supervisor config types
go run "%ROOT_DIR%\tools\generate_supervisor_configs.go" "%ROOT_DIR%"
if errorlevel 1 (
    echo [ERROR] Failed to generate supervisor config types
    exit /b 1
)

set "OUTPUT_DIR=%ROOT_DIR%\linux_build"
set "CACHE_DIR=%ROOT_DIR%\.gocache"

if exist "%OUTPUT_DIR%" rmdir /s /q "%OUTPUT_DIR%"
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"
if not exist "%OUTPUT_DIR%" (
    echo [ERROR] Failed to create %OUTPUT_DIR%
    exit /b 1
)
mkdir "%CACHE_DIR%" >nul 2>nul

set "GOOS=linux"
set "GOARCH=%TARGET_ARCH%"
set "CGO_ENABLED=0"
set "GOCACHE=%CACHE_DIR%"

set "SERVICE_NAME=svr_login"
set "SERVICE_DIR=%ROOT_DIR%\%SERVICE_NAME%"
set "SERVICE_OUT_DIR=%OUTPUT_DIR%\%SERVICE_NAME%"
if not exist "%SERVICE_OUT_DIR%" mkdir "%SERVICE_OUT_DIR%"
if not exist "%SERVICE_OUT_DIR%" (
    echo [ERROR] Failed to create %SERVICE_OUT_DIR%
    exit /b 1
)
echo.
echo [BUILD] %SERVICE_NAME% ^(linux/%TARGET_ARCH%^)
go -C "%SERVICE_DIR%" build -buildvcs=false -o "%SERVICE_OUT_DIR%\%SERVICE_NAME%"
if errorlevel 1 (
    echo [ERROR] Build failed for %SERVICE_NAME%
    exit /b 1
)
copy /y "%SERVICE_DIR%\*.json" "%SERVICE_OUT_DIR%" >nul 2>nul
copy /y "%SERVICE_DIR%\*.csv" "%SERVICE_OUT_DIR%" >nul 2>nul
echo [OK] %SERVICE_NAME% copied to %SERVICE_OUT_DIR%

set "SERVICE_NAME=svr_game"
set "SERVICE_DIR=%ROOT_DIR%\%SERVICE_NAME%"
set "SERVICE_OUT_DIR=%OUTPUT_DIR%\%SERVICE_NAME%"
if not exist "%SERVICE_OUT_DIR%" mkdir "%SERVICE_OUT_DIR%"
if not exist "%SERVICE_OUT_DIR%" (
    echo [ERROR] Failed to create %SERVICE_OUT_DIR%
    exit /b 1
)
echo.
echo [BUILD] %SERVICE_NAME% ^(linux/%TARGET_ARCH%^)
go -C "%SERVICE_DIR%" build -buildvcs=false -o "%SERVICE_OUT_DIR%\%SERVICE_NAME%"
if errorlevel 1 (
    echo [ERROR] Build failed for %SERVICE_NAME%
    exit /b 1
)
copy /y "%SERVICE_DIR%\*.json" "%SERVICE_OUT_DIR%" >nul 2>nul
copy /y "%SERVICE_DIR%\*.csv" "%SERVICE_OUT_DIR%" >nul 2>nul
echo [OK] %SERVICE_NAME% copied to %SERVICE_OUT_DIR%

set "SERVICE_NAME=svr_gateway"
set "SERVICE_DIR=%ROOT_DIR%\%SERVICE_NAME%"
set "SERVICE_OUT_DIR=%OUTPUT_DIR%\%SERVICE_NAME%"
if not exist "%SERVICE_OUT_DIR%" mkdir "%SERVICE_OUT_DIR%"
if not exist "%SERVICE_OUT_DIR%" (
    echo [ERROR] Failed to create %SERVICE_OUT_DIR%
    exit /b 1
)
echo.
echo [BUILD] %SERVICE_NAME% ^(linux/%TARGET_ARCH%^)
go -C "%SERVICE_DIR%" build -buildvcs=false -o "%SERVICE_OUT_DIR%\%SERVICE_NAME%"
if errorlevel 1 (
    echo [ERROR] Build failed for %SERVICE_NAME%
    exit /b 1
)
copy /y "%SERVICE_DIR%\*.json" "%SERVICE_OUT_DIR%" >nul 2>nul
copy /y "%SERVICE_DIR%\*.csv" "%SERVICE_OUT_DIR%" >nul 2>nul
echo [OK] %SERVICE_NAME% copied to %SERVICE_OUT_DIR%

set "SERVICE_NAME=svr_supervisor"
set "SERVICE_DIR=%ROOT_DIR%\%SERVICE_NAME%"
set "SERVICE_OUT_DIR=%OUTPUT_DIR%\%SERVICE_NAME%"
if not exist "%SERVICE_OUT_DIR%" mkdir "%SERVICE_OUT_DIR%"
if not exist "%SERVICE_OUT_DIR%" (
    echo [ERROR] Failed to create %SERVICE_OUT_DIR%
    exit /b 1
)
echo.
echo [BUILD] %SERVICE_NAME% ^(linux/%TARGET_ARCH%^)
go -C "%SERVICE_DIR%" build -buildvcs=false -o "%SERVICE_OUT_DIR%\%SERVICE_NAME%"
if errorlevel 1 (
    echo [ERROR] Build failed for %SERVICE_NAME%
    exit /b 1
)
copy /y "%SERVICE_DIR%\*.json" "%SERVICE_OUT_DIR%" >nul 2>nul
copy /y "%SERVICE_DIR%\*.csv" "%SERVICE_OUT_DIR%" >nul 2>nul
echo [OK] %SERVICE_NAME% copied to %SERVICE_OUT_DIR%

echo.
echo Build completed successfully.
echo Output directory: %OUTPUT_DIR%
exit /b 0
