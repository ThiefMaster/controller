package ddc

// #cgo LDFLAGS: -ldxva2
/*
#include <windows.h>
extern BOOL setVCPFeatureAll(BYTE code, DWORD value);
*/
import "C"
import "log"

const (
	// command codes
	monitorPowerState = 0xd6
	// MonitorPowerState args
	monitorOn      = 1
	monitorStandby = 4
)

func setVCPFeatureAll(code byte, value int) {
	if C.setVCPFeatureAll(C.uchar(code), C.ulong(value)) == 0 {
		log.Printf("setVCPFeatureAll failed")
	}
}

func SetMonitorsOn() {
	setVCPFeatureAll(monitorPowerState, monitorOn)
}

func SetMonitorsStandby() {
	setVCPFeatureAll(monitorPowerState, monitorStandby)
}
