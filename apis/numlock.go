package apis

func SetNumLock(enabled bool) {
	ret, _, _ := getKeyStateProc.Call(VK_NUMLOCK)
	if currentlyEnabled := (ret & 1) == 1; currentlyEnabled == enabled {
		return
	}
	keybdEventProc.Call(uintptr(VK_NUMLOCK), uintptr(0x45), KEYEVENTF_EXTENDEDKEY, 0)
	keybdEventProc.Call(uintptr(VK_NUMLOCK), uintptr(0x45), KEYEVENTF_EXTENDEDKEY | KEYEVENTF_KEYUP, 0)
}
