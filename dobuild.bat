@echo off
cd /d C:\Users\jasse\Desktop\claude\HWmonitor
"C:\Program Files\Go\bin\go.exe" build -o hwmonitor.exe . > build_output.txt 2>&1
echo %ERRORLEVEL% > build_exit.txt
