package comm

import (
	"bufio"
	"fmt"
	"log"
	"strings"

	"github.com/tarm/serial"
)

func parseMessage(s string) Message {
	var id, value int
	if s == "READY" {
		return Message{Message: Ready}
	} else if _, err := fmt.Sscanf(s, "RBTN.%d=1", &id); err == nil {
		return Message{Message: ButtonPressed, Source: id}
	} else if _, err := fmt.Sscanf(s, "RBTN.%d=0", &id); err == nil {
		return Message{Message: ButtonReleased, Source: id}
	} else if _, err := fmt.Sscanf(s, "RVAL.%d=%d", &id, &value); err == nil {
		return Message{Message: KnobTurned, Source: id, Value: value}
	}
	return Message{Message: invalid}
}

func serializeCommand(cmd Command) string {
	switch cmd.command {
	case reset:
		return "RST"
	case clearLED:
		return fmt.Sprintf("RLED.%d=0", cmd.target)
	case setLED:
		return fmt.Sprintf("RLED.%d=%c", cmd.target, cmd.color)
	default:
		return ""
	}
}

func serialWorker(port string, msgChan chan<- Message, cmdChan <-chan Command) {
	log.Printf("opening serial port %s\n", port)
	conn, err := serial.OpenPort(&serial.Config{Name: port, Baud: 19200})
	if err != nil {
		log.Fatalf("OpenPort: %v\n", err)
	}

	reader := bufio.NewReader(conn)
	go func() {
		for {
			line, isPrefix, err := reader.ReadLine()
			if err != nil {
				log.Fatalf("ReadLine: %v\n", err)
			}
			if isPrefix {
				log.Fatalf("got incomplete line: %s\n", string(line))
			}
			trimmed := strings.TrimSpace(string(line))
			if len(trimmed) > 0 {
				msg := parseMessage(trimmed)
				if msg.Message == invalid {
					log.Fatalf("unexpected message: %s\n", trimmed)
				}
				msgChan <- msg
			}
		}
	}()

	go func() {
		for cmd := range cmdChan {
			cmdString := serializeCommand(cmd)
			if cmdString == "" {
				log.Fatalf("unexpected command: %#v\n", cmd)
			}
			if _, err := conn.Write([]byte(cmdString + "\n")); err != nil {
				log.Fatalf("Write: %v\n", err)
			}
		}
	}()
}

func OpenPort(port string) (<-chan Message, chan<- Command) {
	msgChan := make(chan Message, 8)
	cmdChan := make(chan Command, 8)
	serialWorker(port, msgChan, cmdChan)
	log.Println("resetting rotaryboard")
	cmdChan <- NewResetCommand()
	return msgChan, cmdChan
}
