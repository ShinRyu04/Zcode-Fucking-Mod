@echo off
cd /d "%~dp0"
if not exist logs mkdir logs
set PORT=20135
set HOST=127.0.0.1
rem Optional: per-device identity used for billing/balance. Set ZCODE_DEVICE_MID
rem in your own environment (never commit it) to the deviceId from data\accounts.json.
if defined ZCODE_DEVICE_MID set ZCODE_IDENTITY_DEVICE_MID=%ZCODE_DEVICE_MID%
rem Parallel inference requests. Upstream allows a single active generation;
rem leave unset unless you know your lane tolerates more.
if not defined ZCODE_LANE_WIDTH set ZCODE_LANE_WIDTH=1
echo [%date% %time%] start >> logs\out.log
zcode-bridge.exe >> logs\out.log 2>&1
