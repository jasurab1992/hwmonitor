@echo off
cd /d %~dp0

:: ── Download smartctl.exe if not already present ──────────────────────────────
if not exist "internal\collectors\drivers\smartctl.exe" (
    echo Downloading smartctl.exe ^(smartmontools 7.4^)...
    powershell -NoProfile -ExecutionPolicy Bypass -Command ^
        "$zip = [System.IO.Path]::GetTempFileName() + '.zip';" ^
        "$out = [System.IO.Path]::Combine($env:TEMP, 'smartctl_extract');" ^
        "Invoke-WebRequest -Uri 'https://downloads.sourceforge.net/project/smartmontools/smartmontools/7.4/smartmontools-7.4-1.win32.zip' -OutFile $zip -UseBasicParsing;" ^
        "Expand-Archive -Path $zip -DestinationPath $out -Force;" ^
        "Copy-Item (Get-ChildItem -Recurse -Filter smartctl.exe $out | Select-Object -First 1).FullName 'internal\collectors\drivers\smartctl.exe';" ^
        "Remove-Item $zip, $out -Recurse -Force -ErrorAction SilentlyContinue"
    if not exist "internal\collectors\drivers\smartctl.exe" (
        echo WARNING: Could not download smartctl.exe. Building without it.
        goto :build_plain
    )
    echo smartctl.exe downloaded OK.
)

:: ── Build with embedded smartctl ──────────────────────────────────────────────
echo Building with embedded smartctl...
"C:\Program Files\Go\bin\go.exe" build -tags embed_smartctl -o hwmonitor.exe .
goto :result

:build_plain
echo Building without embedded smartctl...
"C:\Program Files\Go\bin\go.exe" build -o hwmonitor.exe .

:result
if %ERRORLEVEL% EQU 0 (
    echo BUILD OK
) else (
    echo BUILD FAILED
)
