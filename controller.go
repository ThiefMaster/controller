package main

import (
	"log"
	"os"
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
	knob             bool
	topLeft          bool
	bottomLeft       bool
	bottomRight      bool
	bottomLeftStart  time.Time
	bottomRightStart time.Time
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
		if !b.bottomLeft && pressed {
			b.bottomLeftStart = time.Now()
		} else if b.bottomLeft && !pressed {
			b.bottomLeftStart = time.Time{}
		}
		b.bottomLeft = pressed
	} else if msg.Source == buttonBottomRight {
		if !b.bottomRight && pressed {
			b.bottomRightStart = time.Now()
		} else if b.bottomRight && !pressed {
			b.bottomRightStart = time.Time{}
		}
		b.bottomRight = pressed
	}
}

func (b *buttonState) getButtonBottomLeftDuration() time.Duration {
	if !b.bottomLeft {
		return 0
	}
	return time.Now().Sub(b.bottomLeftStart)
}

func (b *buttonState) getButtonBottomRightDuration() time.Duration {
	if !b.bottomRight {
		return 0
	}
	return time.Now().Sub(b.bottomRightStart)
}

type appState struct {
	config                    *appConfig
	ready                     bool
	shutdown                  bool
	desktopLocked             bool
	monitorsOn                bool
	knobTurnedWhilePressed    bool
	knobDirectionWhilePressed int
	knobDirectionErrors       int
	ignoreKnobRelease         bool
	ignoreBottomLeftRelease   bool
	ignoreBottomRightRelease  bool
	disableFoobarStateLED     bool
	foobarState               apis.FoobarPlayerInfo
	tubeRemoteState           apis.TubeRemoteState
	tubeMode                  bool
	buttonState               buttonState
}

func (s *appState) reset() {
	s.ready = false
	s.shutdown = false
	s.desktopLocked = false
	s.monitorsOn = true
	s.disableFoobarStateLED = false
	s.ignoreBottomLeftRelease = false
	s.ignoreBottomRightRelease = false
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

		if state.tubeMode {
			continue
		}

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

func trackNotHubState(state *appState, cmdChan chan<- comm.Command) {
	var nhs apis.NotHubState

	go func() {
		flag := false
		for range time.Tick(150 * time.Millisecond) {
			flag = !flag
			cmdChan <- comm.NewToggleLEDCommand(LED1, nhs.Commit && flag)
			if nhs.ChanHL || nhs.PrivMsg {
				cmdChan <- comm.NewToggleLEDCommand(LED5, flag)
				cmdChan <- comm.NewToggleLEDCommand(LED4, !flag)
				cmdChan <- comm.NewToggleLEDCommand(LED5, flag)
			} else if nhs.ChanMsg {
				cmdChan <- comm.NewToggleLEDCommand(LED5, flag)
				cmdChan <- comm.NewClearLEDCommand(LED4)
			} else {
				cmdChan <- comm.NewClearLEDCommand(LED5)
				cmdChan <- comm.NewClearLEDCommand(LED4)
			}
		}
	}()

	for newState := range apis.SubscribeNotHubState(state.config.NotHub) {
		log.Printf("nothub state changed: %#v\n", newState)
		nhs = newState
	}
}

func trackMattermostNotifications(state *appState, cmdChan chan<- comm.Command) {
	// notification states
	var ns struct {
		messages bool
		mentions bool
		mux      sync.Mutex
	}

	go func() {
		flag := false
		for range time.Tick(150 * time.Millisecond) {
			flag = !flag
			ns.mux.Lock()
			if ns.mentions {
				cmdChan <- comm.NewToggleLEDCommand(LED2, flag)
				cmdChan <- comm.NewToggleLEDCommand(LED3, !flag)
				cmdChan <- comm.NewToggleLEDCommand(LED2, flag)
			} else if ns.messages {
				cmdChan <- comm.NewToggleLEDCommand(LED2, flag)
				cmdChan <- comm.NewClearLEDCommand(LED3)
			} else {
				cmdChan <- comm.NewClearLEDCommand(LED2)
				cmdChan <- comm.NewClearLEDCommand(LED3)
			}
			ns.mux.Unlock()
		}
	}()

	for newState := range apis.SubscribeMattermostState(state.config.Mattermost) {
		log.Printf("mattermost state changed: messages=%v, mentions=%v\n", newState.HasMessages, newState.HasMentions)
		ns.mux.Lock()
		ns.messages = newState.HasMessages
		ns.mentions = newState.HasMentions
		ns.mux.Unlock()
	}
}

func toggleTubeMode(state *appState, cmdChan chan<- comm.Command, enabled bool) {
	state.tubeMode = enabled
	cmdChan <- comm.NewToggleLEDCommand(buttonBottomLeft, state.tubeMode)
	if enabled {
		cmdChan <- newCommandForTubeRemoteState(state)
	} else {
		cmdChan <- newCommandForFoobarState(state)
	}
}

func switchAudioTarget(state *appState, cmdChan chan<- comm.Command) {
	if err := apis.SetNextDefaultEndpoint(); err != nil {
		log.Printf("Could not change default audio endpoint: %s\n", err)
		return
	}
	// we use `state.monitorsOn` to toggle the LED regardless of its previous state.
	// XXX: maybe we should just prohibit most actions while the system is locked?
	cmdChan <- comm.NewToggleLEDCommand(buttonBottomRight, state.monitorsOn)
	time.AfterFunc(250*time.Millisecond, func() {
		cmdChan <- comm.NewToggleLEDCommand(buttonBottomRight, !state.monitorsOn)
	})
}

func runTubeRemote(state *appState, cmdChan chan<- comm.Command) {
	for newState := range apis.RunTubeRemote(state.config.TubeRemotePort) {
		oldState := state.tubeRemoteState
		state.tubeRemoteState = newState
		log.Printf("youtube state changed: %#v\n", newState)

		if !state.tubeMode {
			continue
		}

		if newState.ActionFailed {
			cmdChan <- comm.NewSetLEDCommand(knob, 'R')
			time.AfterFunc(1*time.Second, func() {
				cmdChan <- newCommandForTubeRemoteState(state)
			})
			continue
		} else if newState.State == apis.TubeRemoteStateOffline {
			// if we came here right after action-failed, then changing the LED
			// would happen too early and we have the timer to take care of it
			if !oldState.ActionFailed {
				cmdChan <- newCommandForTubeRemoteState(state)
			}
			continue
		}

		if ((!oldState.Playing() && newState.Playing()) || oldState.Volume > 0) && newState.Volume == 0 {
			// show red when we just went silent or started playing while being silent
			cmdChan <- comm.NewSetLEDCommand(knob, 'R')
			time.AfterFunc(1*time.Second, func() {
				cmdChan <- newCommandForTubeRemoteState(state)
			})
		} else if !oldState.Offline() && oldState.Volume != 100 && newState.Volume == 100 {
			// show green if we changed the volume to max
			cmdChan <- comm.NewSetLEDCommand(knob, 'G')
			time.AfterFunc(1*time.Second, func() {
				cmdChan <- newCommandForTubeRemoteState(state)
			})
		} else {
			cmdChan <- newCommandForTubeRemoteState(state)
		}
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
				if config.NotHub.BaseURL != "" {
					go trackNotHubState(state, cmdChan)
				}
				if config.Mattermost.ServerURL != "" {
					go trackMattermostNotifications(state, cmdChan)
				}
				if config.TubeRemotePort != 0 {
					go runTubeRemote(state, cmdChan)
				}
			}
		case !state.ready:
			log.Println("ignoring input during setup")
		case msg.Message == comm.ButtonReleased && msg.Source == buttonTopLeft:
			lockDesktop(state)
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomRight:
			if !state.ignoreBottomRightRelease {
				toggleMonitors(cmdChan, state)
			}
			state.ignoreBottomRightRelease = false
		case msg.Message == comm.ButtonPressed && msg.Source == buttonBottomRight:
			time.AfterFunc(250*time.Millisecond, func() {
				if !state.shutdown && state.buttonState.getButtonBottomRightDuration() > 250*time.Millisecond {
					state.ignoreBottomRightRelease = true
					switchAudioTarget(state, cmdChan)
				}
			})
		case msg.Message == comm.ButtonReleased && msg.Source == buttonBottomLeft:
			if !state.buttonState.knob && !state.ignoreBottomLeftRelease {
				if state.tubeMode {
					toggleTubeMode(state, cmdChan, false)
				} else {
					go foobarNext(state, cmdChan)
				}
			}
			state.ignoreBottomLeftRelease = false
		case msg.Message == comm.ButtonPressed && msg.Source == buttonBottomLeft:
			if state.buttonState.knob {
				state.ignoreKnobRelease = true
				state.ignoreBottomLeftRelease = true
				if state.tubeMode {
					go tubeRemoteStop(cmdChan)
				} else {
					go foobarStop(state, cmdChan)
				}
			} else if !state.tubeMode && config.TubeRemotePort != 0 {
				time.AfterFunc(250*time.Millisecond, func() {
					if !state.shutdown && state.buttonState.getButtonBottomLeftDuration() > 250*time.Millisecond {
						state.ignoreBottomLeftRelease = true
						toggleTubeMode(state, cmdChan, true)
					}
				})
			}
		case msg.Message == comm.ButtonPressed && msg.Source == knob:
			state.resetKnobPressState(true)
		case msg.Message == comm.ButtonReleased && msg.Source == knob:
			if state.tubeMode {
				if !state.knobTurnedWhilePressed && !state.ignoreKnobRelease {
					go tubeRemoteTogglePause()
				}
			} else {
				if !state.knobTurnedWhilePressed && !state.ignoreKnobRelease {
					go foobarTogglePause(state)
				}
				state.resetKnobPressState(false)
				cmdChan <- newCommandForFoobarState(state)
			}
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
					if state.tubeMode {
						go tubeRemoteSeek(msg.Value)
					} else {
						go foobarSeek(state, msg.Value)
					}
				}
			} else {
				if state.tubeMode {
					go tubeRemoteAdjustVolume(msg.Value)
				} else {
					go foobarAdjustVolume(state, cmdChan, msg.Value)
				}
			}
		}

		if state.buttonState.topLeft && state.buttonState.bottomLeft && state.buttonState.bottomRight {
			state.shutdown = true
			log.Println("shutdown requested")
			break
		}
	}

	showFancyOutro(cmdChan)
	log.Println("exiting")
}
