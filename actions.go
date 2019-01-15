package main

import (
	"log"
	"time"

	"github.com/thiefmaster/controller/apis"
	"github.com/thiefmaster/controller/comm"
	"github.com/thiefmaster/controller/ddc"
)

func showFancyIntro(cmdChan chan<- comm.Command, delay time.Duration) {
	for i := knob; i <= buttonBottomRight; i++ {
		cmdChan <- comm.NewClearLEDCommand(i)
	}
	for i := LED5; i <= LED1; i++ {
		cmdChan <- comm.NewClearLEDCommand(i)
		time.Sleep(delay)
	}
}

func showFancyOutro(cmdChan chan<- comm.Command) {
	for j := 0; j < 2; j++ {
		for i := LED5; i <= LED1; i++ {
			cmdChan <- comm.NewSetLEDCommand(i, '1')
			time.Sleep(50 * time.Millisecond)
		}
		for i := LED1; i >= LED5; i-- {
			cmdChan <- comm.NewClearLEDCommand(i)
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func toggleMonitors(cmdChan chan<- comm.Command, state *appState) {
	if state.monitorsOn {
		log.Printf("turning monitors off")
		ddc.SetMonitorsStandby()
	} else {
		log.Printf("turning monitors on")
		ddc.SetMonitorsOn()
	}
	state.monitorsOn = !state.monitorsOn
	cmdChan <- comm.NewToggleLEDCommand(buttonBottomRight, !state.monitorsOn)
}

func lockDesktop(state *appState) {
	log.Println("locking desktop")
	apis.LockDesktop()
	if state.config.Numlock {
		apis.SetNumLock(false)
	}
}

func foobarNext(state *appState, cmdChan chan<- comm.Command) {
	log.Println("playing next song")
	if err := apis.FoobarNext(state.config.Foobar); err != nil {
		log.Printf("foobar next failed: %v\n", err)
		return
	}
	cmdChan <- comm.NewSetLEDCommand(knob, 'R')
	cmdChan <- comm.NewSetLEDCommand(buttonBottomLeft, '1')
	time.AfterFunc(150*time.Millisecond, func() {
		cmdChan <- comm.NewSetLEDCommand(knob, 'G')
		cmdChan <- comm.NewClearLEDCommand(buttonBottomLeft)
		time.AfterFunc(150*time.Millisecond, func() {
			cmdChan <- newCommandForFoobarState(state)
		})
	})
}

func foobarStop(state *appState, cmdChan chan<- comm.Command) {
	log.Println("stopping playback")
	if err := apis.FoobarStop(state.config.Foobar); err != nil {
		log.Printf("foobar stop failed: %v\n", err)
		return
	}
	go func() {
		for i := 0; i < 5; i++ {
			cmdChan <- comm.NewSetLEDCommand(knob, 'R')
			time.Sleep(100 * time.Millisecond)
		}
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		for i := 0; i < 5; i++ {
			time.Sleep(100 * time.Millisecond)
			cmdChan <- comm.NewSetLEDCommand(knob, 'Y')
		}
		cmdChan <- newCommandForFoobarState(state)
	}()
}

func foobarTogglePause(state *appState) {
	log.Println("toggling pause")
	if err := apis.FoobarTogglePause(state.foobarState, state.config.Foobar); err != nil {
		log.Printf("foobar pause failed: %v\n", err)
	}
}

func foobarAdjustVolume(state *appState, cmdChan chan<- comm.Command, delta int) {
	log.Printf("adjusting volume by %+d\n", delta)
	volume, isMin, isMax, err := apis.FoobarAdjustVolume(state.foobarState, float64(delta), state.config.Foobar)
	if err != nil {
		log.Printf("foobar pause failed: %v\n", err)
		return
	}
	log.Printf("new volume: %f\n", volume)
	if isMin {
		cmdChan <- comm.NewSetLEDCommand(knob, 'R')
		time.AfterFunc(1*time.Second, func() {
			cmdChan <- newCommandForFoobarState(state)
		})
	} else if isMax {
		cmdChan <- comm.NewSetLEDCommand(knob, 'G')
		time.AfterFunc(1*time.Second, func() {
			cmdChan <- newCommandForFoobarState(state)
		})
	}
}

func foobarSeek(state *appState, delta int) {
	log.Printf("seeking %+d", delta)
	if err := apis.FoobarSeekRelative(delta * 5, state.config.Foobar); err != nil {
		log.Printf("foobar seek failed: %v\n", err)
		return
	}
}

func newCommandForFoobarState(state *appState) comm.Command {
	if state.foobarState.State == apis.FoobarStatePaused {
		return comm.NewSetLEDCommand(knob, 'Y')
	} else {
		return comm.NewClearLEDCommand(knob)
	}
}
