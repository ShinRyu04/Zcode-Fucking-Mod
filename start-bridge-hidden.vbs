' Launcher zcode-bridge TANPA jendela console.
' Pakai Shell.Application.ShellExecute (window state 0 = hidden);
' WScript.Shell.Run rusak/diblok di mesin ini.
Set sa = CreateObject("Shell.Application")
sa.ShellExecute "cmd.exe", "/c set PORT=20135&& set HOST=127.0.0.1&& set ZCODE_LANE_WIDTH=3&& set ZCODE_IDENTITY_DEVICE_MID=13d5e835-c51a-409d-ac34-c816a174cce0&& D:\ToolsAI\zcode-bridge\zcode-bridge.exe >> D:\ToolsAI\zcode-bridge\logs\out.log 2>&1", "D:\ToolsAI\zcode-bridge", "open", 0
