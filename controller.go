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
	ready                     bool
	desktopLocked             bool
	monitorsOn                bool
	knobPressed               bool
	knobTurnedWhilePressed    bool
	knobDirectionWhilePressed int
	knobDirectionErrors       int
	ignoreKnobRelease         bool
	ignoreBottomLeftRelease   bool
	disableFoobarStateLED     bool
	foobarState               apis.FoobarPlayerInfo
}

func (s *appState) reset() {
	s.ready = false
	s.desktopLocked = false
	s.monitorsOn = true
	s.disableFoobarStateLED = false
	s.ignoreBottomLeftRelease = false
	s.resetKnobPressState(false)
}

func (s *appState) resetKnobPressState(pressed bool) {
	s.knobPressed = pressed
	s.knobTurnedWhilePressed = false
	s.ignoreKnobRelease = false
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
	for newState := range apis.SubscribeFoobarState() {
		state.foobarState = newState
		if state.knobTurnedWhilePressed {
			continue
		}

		log.Printf("foobar state changed: playback=%s, volume=%f\n", newState.State, newState.Volume.Current)
		if newState.State == apis.FoobarStateOffline {
			cmdChan <- comm.NewSetLEDCommand(knob, 'R')
			time.AfterFunc(1*time.Second, func() {
				cmdChan <- comm.NewSetLEDCommand(knob, '0')
			})
		} else {
			cmdChan <- newCommandForFoobarState(state)
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

	for msg := range msgChan {
		switch {
		case msg.Message == comm.Ready:
			if !state.ready {
				state.ready = true
				go showFancyIntro(cmdChan, 75*time.Millisecond)
				go trackLockedState(state, cmdChan)
				go keepMonitorOffWhileLocked(state)
				go trackFoobarState(state, cmdChan)
			}
		case !state.ready:
			log.Println("ignoring input during setup")
		case msg.Message == comm.ButtonReleased && msg.Source == buttonTopLeft:
			lockDesktop()
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomRight:
			toggleMonitors(cmdChan, state)
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomLeft:
			if !state.knobPressed && !state.ignoreBottomLeftRelease {
				go foobarNext(state, cmdChan)
			}
			state.ignoreBottomLeftRelease = false
		case msg.Message == comm.ButtonPressed && msg.Source == buttonBottomLeft:
			if state.knobPressed {
				state.ignoreKnobRelease = true
				state.ignoreBottomLeftRelease = true
				go foobarStop(state, cmdChan)
			}
		case msg.Message == comm.ButtonPressed && msg.Source == knob:
			state.resetKnobPressState(true)
		case msg.Message == comm.ButtonReleased && msg.Source == knob:
			if !state.knobTurnedWhilePressed && !state.ignoreKnobRelease {
				go foobarTogglePause(state)
			}
			state.resetKnobPressState(false)
			cmdChan <- newCommandForFoobarState(state)
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
