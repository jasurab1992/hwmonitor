@echo off
cd /d %~dp0
set TAGS=

:: ── Download smartctl.exe via winget (smartmontools 7.5) ──────────────────────
if not exist "internal\collectors\drivers\smartctl.exe" (
    echo Downloading smartctl.exe via winget...
    winget install smartmontools.smartmontools --silent --accept-source-agreements --accept-package-agreements >nul 2>&1
    if exist "C:\Program Files\smartmontools\bin\smartctl.exe" (
        copy /Y "C:\Program Files\smartmontools\bin\smartctl.exe" "internal\collectors\drivers\smartctl.exe" >nul
        echo smartctl.exe OK
    ) else (
        echo WARNING: Could not download smartctl.exe - install smartmontools manually
        echo   winget install smartmontools.smartmontools
        echo   then copy smartctl.exe to internal\collectors\drivers\
    )
)

:: ── ipmitool: no pre-built Windows binary is published by the ipmitool project ─
:: ── Install manually if IPMI/BMC ambient temps are needed:                      ─
:: ──   https://github.com/ipmitool/ipmitool (build from source)               ─
:: ──   or use vendor tools (Dell DRAC, HPE iLO) which may include ipmitool.exe ─
if not exist "internal\collectors\drivers\ipmitool.exe" (
    echo NOTE: ipmitool.exe not found - IPMI ambient temperature collection disabled.
    echo       Place ipmitool.exe in internal\collectors\drivers\ to enable.
)

:: ── Build lhm_bridge (C# LibreHardwareMonitor bridge) ────────────────────────
if not exist "internal\collectors\drivers\lhm_bridge.exe" (
    echo Building lhm_bridge...
    if exist "lhm_bridge\lhm_bridge.csproj" (
        dotnet publish lhm_bridge\lhm_bridge.csproj -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -o lhm_bridge\publish >nul 2>&1
        if exist "lhm_bridge\publish\lhm_bridge.exe" (
            copy /Y "lhm_bridge\publish\lhm_bridge.exe" "internal\collectors\drivers\lhm_bridge.exe" >nul
            echo lhm_bridge.exe OK
        ) else (
            echo WARNING: lhm_bridge build failed - CPU temp/voltage/fan data from LHM disabled.
        )
    ) else (
        echo NOTE: lhm_bridge project not found - LHM collector disabled.
    )
)

:: ── Assemble build tags ───────────────────────────────────────────────────────
if exist "internal\collectors\drivers\smartctl.exe"  set TAGS=%TAGS%embed_smartctl,
if exist "internal\collectors\drivers\ipmitool.exe"  set TAGS=%TAGS%embed_ipmitool,
if exist "internal\collectors\drivers\lhm_bridge.exe" set TAGS=%TAGS%embed_lhm,

:: Strip trailing comma
if defined TAGS set TAGS=%TAGS:~0,-1%

:: ── Build ─────────────────────────────────────────────────────────────────────
if defined TAGS (
    echo Building with tags: %TAGS%
    "C:\Program Files\Go\bin\go.exe" build -tags "%TAGS%" -o hwmonitor.exe .
) else (
    echo Building without embedded tools...
    "C:\Program Files\Go\bin\go.exe" build -o hwmonitor.exe .
)

if %ERRORLEVEL% EQU 0 (
    echo BUILD OK
) else (
    echo BUILD FAILED
)
