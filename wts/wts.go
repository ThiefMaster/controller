package wts

// based on https://github.com/bbigras/go-windows-session-notifications

// #cgo LDFLAGS: -lwtsapi32
/*
#include <windows.h>
extern void Start();
*/
import "C"

// https://docs.microsoft.com/en-us/windows/desktop/TermServ/wm-wtssession-change
const (
	WTS_SESSION_LOCK   = 0x7
	WTS_SESSION_UNLOCK = 0x8
)

type Message struct {
	UMsg  int
	Param int
}

var (
	chanLocked = make(chan bool, 100)
)

//export relayMessage
func relayMessage(wParam C.uint) {
	switch int(wParam) {
	case WTS_SESSION_LOCK:
		chanLocked <- true
	case WTS_SESSION_UNLOCK:
		chanLocked <- false
	}
}

func RunMonitor() chan bool {
	C.Start()
	return chanLocked
}
