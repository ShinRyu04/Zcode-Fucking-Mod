@echo off
rem Opens the compact t/s overlay in an Edge app window and pins it on top.
rem Set PORT to wherever the bridge listens (start-bridge.cmd uses 20135).
setlocal
if not defined PORT set "PORT=20135"
set "URL=http://127.0.0.1:%PORT%/overlay"
start "" msedge.exe --app=%URL% --window-size=340,230 --window-position=60,60 --user-data-dir="%LOCALAPPDATA%\zcode-overlay"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0overlay.ps1"
endlocal
