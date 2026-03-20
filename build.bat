@echo off
cd /d %~dp0
"C:\Program Files\Go\bin\go.exe" build -o hwmonitor.exe .
if %ERRORLEVEL% EQU 0 (
    echo BUILD OK
) else (
    echo BUILD FAILED
)
