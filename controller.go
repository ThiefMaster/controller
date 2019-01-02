package main

import (
	"log"
	"time"

	"github.com/thiefmaster/controller/comm"
)

func main() {
	msgChan, cmdChan := comm.OpenPort("COM4")
	for msg := range msgChan {
		switch {
		case msg.Message == comm.Ready:
			cmdChan <- comm.MakeSetLED(0, 'R')
			time.Sleep(150 * time.Millisecond)
			cmdChan <- comm.MakeSetLED(0, 'G')
			time.Sleep(150 * time.Millisecond)
			cmdChan <- comm.MakeSetLED(0, 'Y')
			time.Sleep(150 * time.Millisecond)
			for i := 0; i < 4; i++ {
				cmdChan <- comm.MakeClearLED(i)
				time.Sleep(150 * time.Millisecond)
			}
			time.Sleep(250 * time.Millisecond)
			for i := 4; i < 9; i++ {
				cmdChan <- comm.MakeClearLED(i)
				time.Sleep(150 * time.Millisecond)
			}
			time.Sleep(250 * time.Millisecond)
		case msg.Message == comm.ButtonPressed && msg.Source == 2:
			go func() {
				for i := 4; i < 9; i++ {
					if i > 4 {
						cmdChan <- comm.MakeClearLED(i - 1)
					}
					cmdChan <- comm.MakeSetLED(i, '1')
					time.Sleep(250 * time.Millisecond)
				}
				cmdChan <- comm.MakeClearLED(8)
			}()
		case msg.Message == comm.KnobTurned && msg.Source == 0:
			log.Printf("value: %d\n", msg.Value)
		}
	}
}
