@echo off
cd /d "%~dp0"
if not exist logs mkdir logs
set PORT=20135
set HOST=127.0.0.1
rem deviceId akun e1469b56 dari data\accounts.json - wajib untuk billing/balance
set ZCODE_IDENTITY_DEVICE_MID=13d5e835-c51a-409d-ac34-c816a174cce0
rem jumlah request inference paralel (default 1)
set ZCODE_LANE_WIDTH=3
echo [%date% %time%] start >> logs\out.log
zcode-bridge.exe >> logs\out.log 2>&1
