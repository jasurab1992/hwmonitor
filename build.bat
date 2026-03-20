@echo off
cd /d %~dp0
set TAGS=

:: ── Download smartctl.exe ─────────────────────────────────────────────────────
if not exist "internal\collectors\drivers\smartctl.exe" (
    echo Downloading smartctl.exe ^(smartmontools 7.4^)...
    powershell -NoProfile -ExecutionPolicy Bypass -Command ^
        "$zip = [IO.Path]::GetTempFileName() + '.zip';" ^
        "$out = [IO.Path]::Combine($env:TEMP, 'smartctl_extract');" ^
        "Invoke-WebRequest -Uri 'https://downloads.sourceforge.net/project/smartmontools/smartmontools/7.4/smartmontools-7.4-1.win32.zip' -OutFile $zip -UseBasicParsing;" ^
        "Expand-Archive -Path $zip -DestinationPath $out -Force;" ^
        "Copy-Item (Get-ChildItem -Recurse -Filter smartctl.exe $out | Select-Object -First 1).FullName 'internal\collectors\drivers\smartctl.exe';" ^
        "Remove-Item $zip,$out -Recurse -Force -ErrorAction SilentlyContinue"
    if exist "internal\collectors\drivers\smartctl.exe" (
        echo smartctl.exe OK
    ) else (
        echo WARNING: Could not download smartctl.exe
    )
)

:: ── Download ipmitool.exe ─────────────────────────────────────────────────────
if not exist "internal\collectors\drivers\ipmitool.exe" (
    echo Downloading ipmitool.exe ^(1.8.19^)...
    powershell -NoProfile -ExecutionPolicy Bypass -Command ^
        "Invoke-WebRequest -Uri 'https://github.com/ipmitool/ipmitool/releases/download/IPMITOOL_1_8_19/ipmitool.exe' -OutFile 'internal\collectors\drivers\ipmitool.exe' -UseBasicParsing"
    if exist "internal\collectors\drivers\ipmitool.exe" (
        echo ipmitool.exe OK
    ) else (
        echo WARNING: Could not download ipmitool.exe
    )
)

:: ── Assemble build tags ───────────────────────────────────────────────────────
if exist "internal\collectors\drivers\smartctl.exe"  set TAGS=%TAGS%embed_smartctl,
if exist "internal\collectors\drivers\ipmitool.exe"  set TAGS=%TAGS%embed_ipmitool,

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
