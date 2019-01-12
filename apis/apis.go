package apis

import "golang.org/x/sys/windows"

var (
	user32              = windows.MustLoadDLL("user32.dll")
	lockWorkStationProc = user32.MustFindProc("LockWorkStation")
	getKeyStateProc     = user32.MustFindProc("GetKeyState")
	keybdEventProc      = user32.MustFindProc("keybd_event")
)

const (
	VK_NUMLOCK            = 0x90
	KEYEVENTF_EXTENDEDKEY = 0x1
	KEYEVENTF_KEYUP       = 0x2
)
