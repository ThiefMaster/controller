package apis

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/thiefmaster/eventsource"
)

type NotHubState struct {
	ChanHL  bool
	ChanMsg bool
	Commit  bool
	PrivMsg bool
}

func subscribeNotHubState(eventChan chan<- NotHubState, credentials HTTPCredentials) {
	req, err := newRequest("GET", "/updates", nil, credentials)
	if err != nil {
		log.Fatalf("newRequest failed: %v", err)
	}

	stream, err := eventsource.SubscribeWithRequest("", req)
	if err != nil {
		log.Printf("subscribe failed: %v\n", err)
		time.Sleep(1 * time.Second)
		defer subscribeNotHubState(eventChan, credentials)
		return
	}

	stream.InitialRetryDelay = 500 * time.Millisecond
	stream.MaxRetryDelay = 5 * time.Second
	stream.Logger = log.New(os.Stderr, "", log.LstdFlags)
	var lastState NotHubState
	initialStateSent := false
	for {
		select {
		case event := <-stream.Events:
			data := event.Data()
			var newState NotHubState
			if err := json.Unmarshal([]byte(data), &newState); err != nil {
				log.Printf("could not unmarshal nothub event: %v\n", err)
			} else if newState != lastState || !initialStateSent {
				eventChan <- newState
				lastState = newState
				initialStateSent = true
			}
		case err := <-stream.Errors:
			log.Printf("nothub event stream error: %v\n", err)
			newState := NotHubState{}
			if newState != lastState {
				eventChan <- newState
				lastState = newState
			}
		}
	}
}

func SubscribeNotHubState(credentials HTTPCredentials) <-chan NotHubState {
	eventChan := make(chan NotHubState)
	go subscribeNotHubState(eventChan, credentials)
	return eventChan
}
