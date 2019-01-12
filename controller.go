package main

import (
	"fmt"
	"log"
	"os"
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
	desktopLocked             bool
	monitorsOn                bool
	knobPressed               bool
	knobTurnedWhilePressed    bool
	knobDirectionWhilePressed int
	knobDirectionErrors       int
	disableFoobarStateLED	  bool
}

func (s *appState) reset() {
	s.desktopLocked = false
	s.monitorsOn = true
	s.disableFoobarStateLED = false
	s.resetKnobPressState(false)
}

func (s *appState) resetKnobPressState(pressed bool) {
	s.knobPressed = pressed
	s.knobTurnedWhilePressed = false
	s.knobDirectionWhilePressed = 0
	s.knobDirectionErrors = 0
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

func trackFoobarState(state *appState, cmdChan chan<- comm.Command) {
	errors := 0
	for range time.Tick(500 * time.Millisecond) {
		if state.knobPressed || state.disableFoobarStateLED {
			continue
		}

		foobarState, err := apis.GetFoobarState()
		if err != nil {
			log.Printf("could not get foobar state: %v\n", err)
			errors += 1
			if errors > 2 {
				time.Sleep(2 * time.Second)
			}
			continue
		}

		errors = 0
		if foobarState.State == apis.FoobarStatePaused {
			cmdChan <- comm.NewSetLEDCommand(knob, 'Y')
		} else {
			cmdChan <- comm.NewClearLEDCommand(knob)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <foobar-beefweb-password>\n", os.Args[0])
		return
	}

	state := &appState{}
	state.reset()

	msgChan, cmdChan := comm.OpenPort("COM6")
	go trackLockedState(state, cmdChan)
	go keepMonitorOffWhileLocked(state)
	go trackFoobarState(state, cmdChan)

	for msg := range msgChan {
		switch {
		case msg.Message == comm.Ready:
			go showFancyIntro(cmdChan, 150*time.Millisecond)
		case msg.Message == comm.ButtonReleased && msg.Source == buttonTopLeft:
			lockDesktop()
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomRight:
			toggleMonitors(cmdChan, state)
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomLeft:
			go foobarNext(state, cmdChan)
		case msg.Message == comm.ButtonPressed && msg.Source == knob:
			state.resetKnobPressState(true)
		case msg.Message == comm.ButtonReleased && msg.Source == knob:
			if !state.knobTurnedWhilePressed {
				go foobarTogglePause()
			}
			state.resetKnobPressState(false)
		case msg.Message == comm.KnobTurned && msg.Source == knob:
			if state.knobPressed {
				if !state.knobTurnedWhilePressed {
					log.Println("knob turning while pressed")
					state.knobDirectionWhilePressed = signum(msg.Value)
					state.knobTurnedWhilePressed = true
				}
				if state.knobDirectionWhilePressed != signum(msg.Value) {
					log.Println("turn direction not maching initial direction")
					state.knobDirectionErrors++
					if state.knobDirectionErrors > 5 {
						cmdChan <- comm.NewSetLEDCommand(knob, 'R')
						time.AfterFunc(150*time.Millisecond, func() {
							cmdChan <- comm.NewSetLEDCommand(knob, '0')
						})
					}
				} else {
					go foobarSeek(msg.Value)
				}
			} else {
				go foobarAdjustVolume(state, cmdChan, msg.Value)
			}
		}
	}
}
