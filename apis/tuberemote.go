package apis

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

const (
	TubeRemoteStateOffline = "offline"
	TubeRemoteStateStopped = "stopped"
	TubeRemoteStatePlaying = "playing"
	TubeRemoteStatePaused  = "paused"
)

type TubeRemoteState struct {
	State        string
	Volume       int
	ActionFailed bool
}

func (s *TubeRemoteState) Playing() bool {
	return s.State == TubeRemoteStatePlaying
}

func (s *TubeRemoteState) Offline() bool {
	return s.State == TubeRemoteStateOffline
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header["Origin"]
			if len(origin) == 0 {
				return true
			}
			u, err := url.Parse(origin[0])
			if err != nil {
				return false
			}
			return u.Scheme == "moz-extension"
		},
	}
	broadcastChan    = make(chan string)
	eventChan        = make(chan TubeRemoteState)
	activeConn       *websocket.Conn
	initialStateSent = false
	lastState        TubeRemoteState
)

func ws(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %s\n", err)
		return
	}
	defer func() {
		c.Close()
		if c == activeConn {
			activeConn = nil
		}
	}()
	if activeConn != nil {
		log.Printf("closing previous websocket conn %p\n", activeConn)
		activeConn.Close()
	}
	activeConn = c
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("websocket read failed: %s\n", err)
			break
		}
		if c != activeConn {
			// not sure if this can happen, but let's ignore such cases just in case
			log.Println("ignoring websocket read on old socket")
			break
		}
		var newState TubeRemoteState
		if err := json.Unmarshal(message, &newState); err != nil {
			log.Printf("could not unmarshal tuberemote message: %v\n", err)
		} else if newState != lastState || !initialStateSent {
			eventChan <- newState
			lastState = newState
			initialStateSent = true
		}
	}
}

func tubeRemoteWriter() {
	for msg := range broadcastChan {
		if activeConn == nil {
			log.Printf("discarding %s\n", msg)
			continue
		}
		if err := activeConn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			log.Printf("websocket write failed: %s\n", err)
		}
	}
}

func tubeRemoteListener(port int) {
	err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)
	log.Fatalf("tuberemote server exited: %v", err)
}

func RunTubeRemote(port int) <-chan TubeRemoteState {
	http.HandleFunc("/ws", ws)
	go func() {
		for range time.Tick(250 * time.Millisecond) {
			if activeConn != nil {
				broadcastChan <- `{"action": "getStatus"}`
			}
		}
	}()
	go tubeRemoteWriter()
	go tubeRemoteListener(port)
	return eventChan
}

func TubeRemoteTogglePause() {
	broadcastChan <- `{"action": "togglePlayback"}`
}

func TubeRemoteStop() {
	broadcastChan <- `{"action": "stopPlayback"}`
}

func TubeRemoteAdjustVolume(delta int) {
	broadcastChan <- fmt.Sprintf(`{"action": "changeVolume", "delta": %d}`, delta)
}

func TubeRemoteSeek(delta int) {
	broadcastChan <- fmt.Sprintf(`{"action": "seekBy", "delta": %d}`, delta)
}
