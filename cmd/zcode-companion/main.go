// Command zcode-companion is the optional Windows desktop shell around
// zcode-bridge: a tray icon with menu actions, supervision of the bridge
// process, a login autostart toggle and a release update check. It is a
// separate binary so headless servers keep using cmd/zcode-bridge alone.
//
// Everything here is standard library plus Win32 syscalls; no asset files and
// no third-party packages. Run `zcode-companion -diagnose` for a plain-text
// report, `-no-tray` for supervision without a tray icon, or `-autostart=on`
// / `-autostart=off` to change login startup non-interactively.
package main

import (
	"flag"
	"fmt"
	"os"
)

const companionVersion = "0.1.0"

func main() {
	var (
		diagnose    = flag.Bool("diagnose", false, "print a plain-text status report and exit")
		noTray      = flag.Bool("no-tray", false, "supervise the bridge without a tray icon")
		autostartIn = flag.String("autostart", "", "set login autostart: on or off")
		checkUpdate = flag.Bool("check-update", false, "query the latest published release and exit")
	)
	flag.Parse()

	if *autostartIn != "" {
		on := *autostartIn == "on" || *autostartIn == "1" || *autostartIn == "true"
		if *autostartIn != "on" && *autostartIn != "off" && *autostartIn != "1" && *autostartIn != "0" && *autostartIn != "true" && *autostartIn != "false" {
			fmt.Fprintln(os.Stderr, "autostart expects on or off")
			os.Exit(2)
		}
		if err := setAutostart(on); err != nil {
			fmt.Fprintln(os.Stderr, "autostart:", err)
			os.Exit(1)
		}
		state := "off"
		if on {
			state = "on"
		}
		fmt.Println("autostart", state)
		return
	}
	if *checkUpdate {
		info, err := checkLatestRelease(companionVersion)
		if err != nil {
			fmt.Fprintln(os.Stderr, "update check:", err)
			os.Exit(1)
		}
		fmt.Print(info.String())
		return
	}
	if *diagnose {
		if err := runDiagnose(); err != nil {
			fmt.Fprintln(os.Stderr, "diagnose:", err)
			os.Exit(1)
		}
		return
	}
	if err := runCompanion(*noTray); err != nil {
		fmt.Fprintln(os.Stderr, "zcode-companion:", err)
		os.Exit(1)
	}
}
