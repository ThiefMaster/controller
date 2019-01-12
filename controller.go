package main

import (
	"log"
	"time"

	"github.com/thiefmaster/controller/apis"
	"github.com/thiefmaster/controller/comm"
	"github.com/thiefmaster/controller/ddc"
	"github.com/thiefmaster/controller/wts"
)

const (
	knob = iota
	buttonTopLeft
	buttonBottomLeft
	buttonBottomRight
	LED5
	LED4
	LED3
	LED2
	LED1
)

type appState struct {
	desktopLocked bool
	monitorsOn    bool
}

func (s *appState) reset() {
	s.desktopLocked = false
	s.monitorsOn = true
}

func trackLockedState(state *appState, cmdChan chan<- comm.Command) {
	for locked := range wts.RunMonitor() {
		log.Printf("desktop locked: %v\n", locked)
		state.desktopLocked = locked
		if !locked {
			apis.SetNumLock(true)
		}
		cmdChan <- comm.NewToggleLEDCommand(buttonTopLeft, state.desktopLocked)
	}
}

func keepMonitorOffWhileLocked(state *appState) {
	// Sometimes the monitors wake up from standby even though they
	// shouldn't. This seems to happen randomly or in some cases when
	// connecting to the PC remotely. Let's force them back off!
	for range time.Tick(5 * time.Second) {
		if !state.monitorsOn && state.desktopLocked {
			ddc.SetMonitorsStandby()
		}
	}
}

func main() {
	state := &appState{}
	state.reset()

	msgChan, cmdChan := comm.OpenPort("COM6")
	go trackLockedState(state, cmdChan)
	go keepMonitorOffWhileLocked(state)

	for msg := range msgChan {
		switch {
		case msg.Message == comm.Ready:
			go showFancyIntro(cmdChan, 150*time.Millisecond)
		case msg.Message == comm.ButtonReleased && msg.Source == buttonTopLeft:
			lockDesktop()
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomRight:
			toggleMonitors(state)
			cmdChan <- comm.NewToggleLEDCommand(buttonBottomRight, !state.monitorsOn)
		}
	}
}
