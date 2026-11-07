@echo off
cd /d "%~dp0"
if not exist logs mkdir logs
set PORT=20135
set HOST=127.0.0.1
rem Supervisor: starts zcode-bridge.exe when /health is down and restarts it
rem after a crash. Close from the tray icon (Keluar).
echo [%date% %time%] companion start >> logs\companion.log
if not exist zcode-companion.exe (
  echo zcode-companion.exe not built. Run: go build -o zcode-companion.exe ./cmd/zcode-companion
  exit /b 1
)
start "" zcode-companion.exe
