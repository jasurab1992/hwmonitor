@echo off
cd /d %~dp0
echo Building lhm_bridge...
dotnet publish -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -p:PublishTrimmed=true -o publish
if %ERRORLEVEL% EQU 0 (
    echo OK: publish\lhm_bridge.exe
    copy /Y publish\lhm_bridge.exe ..\internal\collectors\drivers\lhm_bridge.exe >nul
    echo Copied to drivers\lhm_bridge.exe
) else (
    echo BUILD FAILED
)
