package apis

import (
	"log"

	"golang.org/x/sys/windows"
)

var (
	user32              = windows.MustLoadDLL("user32.dll")
	lockWorkStationProc = user32.MustFindProc("LockWorkStation")
)

func LockDesktop() {
	if ret, _, err := lockWorkStationProc.Call(); ret == 0 {
		log.Fatalf("LockWorkStation failed: %v\n", err)
	}
}
