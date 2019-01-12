package main

import (
	"log"
	"time"

	"github.com/thiefmaster/controller/apis"
	"github.com/thiefmaster/controller/comm"
	"github.com/thiefmaster/controller/ddc"
)

func showFancyIntro(state *appState, cmdChan chan<- comm.Command, delay time.Duration) {
	for i := knob; i <= buttonBottomRight; i++ {
		cmdChan <- comm.NewClearLEDCommand(i)
	}
	for i := LED5; i <= LED1; i++ {
		cmdChan <- comm.NewClearLEDCommand(i)
		time.Sleep(delay)
	}
	state.ready = true
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

func lockDesktop() {
	log.Println("locking desktop")
	apis.LockDesktop()
	apis.SetNumLock(false)
}

func foobarNext(state *appState, cmdChan chan<- comm.Command) {
	log.Println("playing next song")
	if err := apis.FoobarNext(); err != nil {
		log.Printf("foobar next failed: %v\n", err)
		return
	}
	cmdChan <- comm.NewSetLEDCommand(knob, 'R')
	cmdChan <- comm.NewSetLEDCommand(buttonBottomLeft, '1')
	time.AfterFunc(150 * time.Millisecond, func() {
		cmdChan <- comm.NewSetLEDCommand(knob, 'G')
		cmdChan <- comm.NewClearLEDCommand(buttonBottomLeft)
		time.AfterFunc(150 * time.Millisecond, func() {
			cmdChan <- newCommandForFoobarState(state)
		})
	})
}

func foobarTogglePause() {
	log.Println("toggling pause")
	if err := apis.FoobarTogglePause(); err != nil {
		log.Printf("foobar pause failed: %v\n", err)
		return
	}
}

func foobarAdjustVolume(state *appState, cmdChan chan<- comm.Command, delta int) {
	log.Printf("adjusting volume by %+d\n", delta)
	volume, isMin, isMax, err := apis.FoobarAdjustVolume(float64(delta))
	if err != nil {
		log.Printf("foobar pause failed: %v\n", err)
		return
	}
	log.Printf("new volume: %f\n", volume)
	if isMin {
		cmdChan <- comm.NewSetLEDCommand(knob, 'R')
		time.AfterFunc(1 * time.Second, func() {
			cmdChan <- newCommandForFoobarState(state)
		})
	} else if isMax {
		cmdChan <- comm.NewSetLEDCommand(knob, 'G')
		time.AfterFunc(1 * time.Second, func() {
			cmdChan <- newCommandForFoobarState(state)
		})
	}
}

func foobarSeek(delta int) {
	log.Printf("seeking %+d", delta)
	if err := apis.FoobarSeekRelative(delta * 5); err != nil {
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
