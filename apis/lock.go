package apis

import (
	"log"
)


func LockDesktop() {
	if ret, _, err := lockWorkStationProc.Call(); ret == 0 {
		log.Fatalf("LockWorkStation failed: %v\n", err)
	}
}
