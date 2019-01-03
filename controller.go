package main

import (
	"log"
	"time"

	"github.com/thiefmaster/controller/comm"
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

func main() {
	msgChan, cmdChan := comm.OpenPort("COM4")
	for msg := range msgChan {
		switch {
		case msg.Message == comm.Ready:
			go func() {
				cmdChan <- comm.NewSetLEDCommand(0, 'R')
				time.Sleep(150 * time.Millisecond)
				cmdChan <- comm.NewSetLEDCommand(0, 'G')
				time.Sleep(150 * time.Millisecond)
				cmdChan <- comm.NewSetLEDCommand(0, 'Y')
				time.Sleep(150 * time.Millisecond)
				for i := 0; i < 4; i++ {
					cmdChan <- comm.NewClearLEDCommand(i)
					time.Sleep(150 * time.Millisecond)
				}
				time.Sleep(150 * time.Millisecond)
				for i := 4; i < 9; i++ {
					cmdChan <- comm.NewClearLEDCommand(i)
					time.Sleep(150 * time.Millisecond)
				}
			}()
		case msg.Message == comm.ButtonPressed && msg.Source == buttonBottomLeft:
			go func() {
				for i := 4; i < 9; i++ {
					if i > 4 {
						cmdChan <- comm.NewClearLEDCommand(i - 1)
					}
					cmdChan <- comm.NewSetLEDCommand(i, '1')
					time.Sleep(250 * time.Millisecond)
				}
				cmdChan <- comm.NewClearLEDCommand(8)
			}()
		case msg.Message == comm.KnobTurned && msg.Source == knob:
			log.Printf("value: %d\n", msg.Value)
		}
	}
}
