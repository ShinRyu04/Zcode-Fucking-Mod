# Pins the zcode-bridge overlay window on top of other windows.
# Called by overlay.cmd after Edge opens the app window; best effort, since the
# window can take a few seconds to appear.
$signature = @'
[DllImport("user32.dll")]
public static extern bool SetWindowPos(IntPtr hWnd, IntPtr hWndInsertAfter, int X, int Y, int cx, int cy, uint uFlags);
'@
Add-Type -MemberDefinition $signature -Name Win -Namespace Overlay | Out-Null

$topmost = [IntPtr]::new(-1)   # HWND_TOPMOST
$noActivate = 0x0010           # SWP_NOACTIVATE
$flags = $noActivate -bor 0x0001 -bor 0x0002  # keep size and position

for ($i = 0; $i -lt 40; $i++) {
    $window = Get-Process -Name msedge -ErrorAction SilentlyContinue |
        Where-Object { $_.MainWindowTitle -eq 'zcode-bridge overlay' } |
        Select-Object -First 1
    if ($window) {
        [Overlay.Win]::SetWindowPos($window.MainWindowHandle, $topmost, 0, 0, 0, 0, $flags) | Out-Null
        exit 0
    }
    Start-Sleep -Milliseconds 500
}
Write-Host "overlay window not found (open /overlay manually and pin it)"
exit 1
