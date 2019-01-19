package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"sync"
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

type buttonState struct {
	knob        bool
	topLeft     bool
	bottomLeft  bool
	bottomRight bool
}

func (b *buttonState) handleMessage(msg comm.Message) {
	if msg.Message != comm.ButtonPressed && msg.Message != comm.ButtonReleased {
		return
	}
	pressed := msg.Message == comm.ButtonPressed
	if msg.Source == knob {
		b.knob = pressed
	} else if msg.Source == buttonTopLeft {
		b.topLeft = pressed
	} else if msg.Source == buttonBottomLeft {
		b.bottomLeft = pressed
	} else if msg.Source == buttonBottomRight {
		b.bottomRight = pressed
	}

}

type appState struct {
	config                    *appConfig
	ready                     bool
	desktopLocked             bool
	monitorsOn                bool
	knobTurnedWhilePressed    bool
	knobDirectionWhilePressed int
	knobDirectionErrors       int
	ignoreKnobRelease         bool
	ignoreBottomLeftRelease   bool
	disableFoobarStateLED     bool
	foobarState               apis.FoobarPlayerInfo
	buttonState               buttonState
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
	s.knobTurnedWhilePressed = false
	s.ignoreKnobRelease = false
	s.knobDirectionWhilePressed = 0
	s.knobDirectionErrors = 0
}

func trackLockedState(state *appState, cmdChan chan<- comm.Command) {
	for locked := range wts.RunMonitor() {
		log.Printf("desktop locked: %v\n", locked)
		state.desktopLocked = locked
		if !locked && state.config.Numlock {
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
	for newState := range apis.SubscribeFoobarState(state.config.Foobar) {
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

func trackIRCNotifications(state *appState, cmdChan chan<- comm.Command) {
	// notification states
	var ns struct {
		channel int
		private int
		commit  int
		mux     sync.Mutex
	}

	go func() {
		flag := false
		for range time.Tick(150 * time.Millisecond) {
			flag = !flag
			ns.mux.Lock()
			cmdChan <- comm.NewToggleLEDCommand(LED1, ns.commit != 0 && flag)
			if ns.channel == 2 || ns.private != 0 {
				cmdChan <- comm.NewToggleLEDCommand(LED5, flag)
				cmdChan <- comm.NewToggleLEDCommand(LED4, !flag)
				cmdChan <- comm.NewToggleLEDCommand(LED5, flag)
			} else if ns.channel != 0 {
				cmdChan <- comm.NewToggleLEDCommand(LED5, flag)
				cmdChan <- comm.NewClearLEDCommand(LED4)
			} else {
				cmdChan <- comm.NewClearLEDCommand(LED5)
				cmdChan <- comm.NewClearLEDCommand(LED4)
			}
			ns.mux.Unlock()
		}
	}()

	for range time.Tick(500 * time.Millisecond) {
		file, err := os.Open(state.config.IRCFile)
		if err != nil {
			log.Printf("could not open irc notification file: %v\n", err)
			continue
		}
		reader := bufio.NewReader(file)
		var lines [3]int
		for i := 0; i < 3; i++ {
			line, isPrefix, err := reader.ReadLine()
			if err != nil || isPrefix {
				continue
			}
			lines[i], _ = strconv.Atoi(string(line))
		}
		file.Close()
		ns.mux.Lock()
		ns.channel = lines[0]
		ns.private = lines[1]
		ns.commit = lines[2]
		ns.mux.Unlock()
	}
}

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	config := &appConfig{}
	if err := config.load(configPath); err != nil {
		log.Fatalln(err)
	}

	state := &appState{config: config}
	state.reset()

	msgChan, cmdChan := comm.OpenPort(config.Port)

	for msg := range msgChan {
		if state.ready {
			state.buttonState.handleMessage(msg)
		}
		switch {
		case msg.Message == comm.Ready:
			if !state.ready {
				state.ready = true
				go showFancyIntro(cmdChan, 75*time.Millisecond)
				go trackLockedState(state, cmdChan)
				go keepMonitorOffWhileLocked(state)
				go trackFoobarState(state, cmdChan)
				if config.IRCFile != "" {
					go trackIRCNotifications(state, cmdChan)
				}
			}
		case !state.ready:
			log.Println("ignoring input during setup")
		case msg.Message == comm.ButtonReleased && msg.Source == buttonTopLeft:
			lockDesktop(state)
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomRight:
			toggleMonitors(cmdChan, state)
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomLeft:
			if !state.buttonState.knob && !state.ignoreBottomLeftRelease {
				go foobarNext(state, cmdChan)
			}
			state.ignoreBottomLeftRelease = false
		case msg.Message == comm.ButtonPressed && msg.Source == buttonBottomLeft:
			if state.buttonState.knob {
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
			if state.buttonState.knob {
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
					go foobarSeek(state, msg.Value)
				}
			} else {
				go foobarAdjustVolume(state, cmdChan, msg.Value)
			}
		}

		if state.buttonState.topLeft && state.buttonState.bottomLeft && state.buttonState.bottomRight {
			log.Println("shutdown requested")
			break
		}
	}

	showFancyOutro(cmdChan)
	log.Println("exiting")
}
