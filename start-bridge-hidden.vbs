Set sa = CreateObject("Shell.Application")
Set fso = CreateObject("Scripting.FileSystemObject")
here = fso.GetParentFolderName(WScript.ScriptFullName)
sa.ShellExecute "cmd.exe", "/c start-bridge.cmd", here, "open", 0
