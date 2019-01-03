package main

import (
	"log"
	"time"

	"github.com/thiefmaster/controller/apis"
	"github.com/thiefmaster/controller/comm"
	"github.com/thiefmaster/controller/ddc"
)

func showFancyIntro(cmdChan chan<- comm.Command, delay time.Duration) {
	cmdChan <- comm.NewSetLEDCommand(knob, 'R')
	time.Sleep(delay)
	cmdChan <- comm.NewSetLEDCommand(knob, 'G')
	time.Sleep(delay)
	cmdChan <- comm.NewSetLEDCommand(knob, 'Y')
	time.Sleep(delay)
	for i := knob; i <= buttonBottomRight; i++ {
		cmdChan <- comm.NewClearLEDCommand(i)
		time.Sleep(delay)
	}
	time.Sleep(delay)
	for i := LED5; i <= LED1; i++ {
		cmdChan <- comm.NewClearLEDCommand(i)
		time.Sleep(delay)
	}
}

func toggleMonitors(state *appState) {
	if state.monitorsOn {
		log.Printf("turning monitors off")
		ddc.SetMonitorsStandby()
	} else {
		log.Printf("turning monitors on")
		ddc.SetMonitorsOn()
	}
	state.monitorsOn = !state.monitorsOn
}

func lockDesktop() {
	log.Println("locking desktop")
	apis.LockDesktop()
}
