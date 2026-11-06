' Stop zcode-bridge TANPA jendela console.
Set sa = CreateObject("Shell.Application")
sa.ShellExecute "cmd.exe", "/c D:\ToolsAI\zcode-bridge\stop-bridge.cmd", "D:\ToolsAI\zcode-bridge", "open", 0
