package main

import (
	"log"
	"time"

	"github.com/thiefmaster/controller/apis"
	"github.com/thiefmaster/controller/comm"
	"github.com/thiefmaster/controller/ddc"
)

func showFancyIntro(cmdChan chan<- comm.Command, delay time.Duration) {
	cmdChan <- comm.NewSetLEDCommand(0, 'R')
	time.Sleep(delay)
	cmdChan <- comm.NewSetLEDCommand(0, 'G')
	time.Sleep(delay)
	cmdChan <- comm.NewSetLEDCommand(0, 'Y')
	time.Sleep(delay)
	for i := 0; i < 4; i++ {
		cmdChan <- comm.NewClearLEDCommand(i)
		time.Sleep(delay)
	}
	time.Sleep(delay)
	for i := 4; i < 9; i++ {
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
