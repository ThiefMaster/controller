package main

import (
	"log"
	"time"

	"github.com/thiefmaster/controller/comm"
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
		cmdChan <- comm.NewToggleLEDCommand(buttonTopLeft, state.desktopLocked)
	}
}

func main() {
	state := &appState{}
	state.reset()

	msgChan, cmdChan := comm.OpenPort("COM4")
	go trackLockedState(state, cmdChan)

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
