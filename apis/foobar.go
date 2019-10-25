package apis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/thiefmaster/eventsource"
)

var (
	client = http.Client{Timeout: 300 * time.Millisecond}
)

const (
	FoobarStateOffline = "offline"
	FoobarStateStopped = "stopped"
	FoobarStatePlaying = "playing"
	FoobarStatePaused  = "paused"
)

type foobarPlayerJSON struct {
	Player FoobarPlayerInfo `json:"player"`
}

type FoobarPlayerInfo struct {
	State  string `json:"playbackState"`
	Volume struct {
		Min     float64 `json:"min"`
		Max     float64 `json:"max"`
		Current float64 `json:"value"`
	} `json:"volume"`
}

func foobarRequest(method, path string, payload interface{}, credentials HTTPCredentials) ([]byte, error) {
	var reqBody io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("json marshal failed: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}
	req, err := newRequest(method, path, reqBody, credentials)
	if err != nil {
		return nil, fmt.Errorf("newRequest failed: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, errors.New("foobar request timed out")
		} else {
			return nil, fmt.Errorf("foobar request failed: %v", err)
		}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read foobar response: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("foobar request returned status %v: %v", resp.StatusCode, string(body))
	}
	return body, nil
}

func subscribeFoobarState(eventChan chan<- FoobarPlayerInfo, credentials HTTPCredentials) {
	req, err := newRequest("GET", "/api/query/updates?player=true", nil, credentials)
	if err != nil {
		log.Fatalf("newRequest failed: %v", err)
	}

	stream, err := eventsource.SubscribeWithRequest("", req)
	if err != nil {
		log.Printf("subscribe failed: %v\n", err)
		time.Sleep(1 * time.Second)
		defer subscribeFoobarState(eventChan, credentials)
		return
	}

	stream.InitialRetryDelay = 500 * time.Millisecond
	stream.MaxRetryDelay = 5 * time.Second
	stream.Logger = log.New(os.Stderr, "", log.LstdFlags)
	lastState, err := getFoobarState(credentials)
	if err != nil {
		log.Printf("could not get initial foobar state: %v\n", err)
	}
	eventChan <- lastState
	for {
		select {
		case event := <-stream.Events:
			data := event.Data()
			if data == "" || data == "{}" {
				continue
			}
			var status foobarPlayerJSON
			if err := json.Unmarshal([]byte(data), &status); err != nil {
				log.Printf("could not unmarshal foobar event: %v\n", err)
			} else if status.Player != lastState {
				eventChan <- status.Player
				lastState = status.Player
			}
		case err := <-stream.Errors:
			log.Printf("foobar event stream error: %v\n", err)
			newState := FoobarPlayerInfo{State: FoobarStateOffline, Volume: lastState.Volume}
			if newState != lastState {
				eventChan <- newState
				lastState = newState
			}
		}
	}
}

func SubscribeFoobarState(credentials HTTPCredentials) <-chan FoobarPlayerInfo {
	eventChan := make(chan FoobarPlayerInfo)
	go subscribeFoobarState(eventChan, credentials)
	return eventChan
}

func getFoobarState(credentials HTTPCredentials) (FoobarPlayerInfo, error) {
	var status foobarPlayerJSON
	body, err := foobarRequest("GET", "/api/player", nil, credentials)
	if err != nil {
		return status.Player, err
	}
	if err := json.Unmarshal(body, &status); err != nil {
		return status.Player, fmt.Errorf("could not parse foobar json: %v", err)
	}
	return status.Player, nil
}

func FoobarNext(credentials HTTPCredentials) error {
	if _, err := foobarRequest("POST", "/api/player/next", nil, credentials); err != nil {
		return err
	}
	return nil
}

func FoobarStop(credentials HTTPCredentials) error {
	if _, err := foobarRequest("POST", "/api/player/stop", nil, credentials); err != nil {
		return err
	}
	return nil
}

func FoobarTogglePause(state FoobarPlayerInfo, credentials HTTPCredentials) error {
	if state.State == FoobarStateStopped {
		if _, err := foobarRequest("POST", "/api/player/play", nil, credentials); err != nil {
			return err
		}
		return nil
	}
	if _, err := foobarRequest("POST", "/api/player/pause/toggle", nil, credentials); err != nil {
		return err
	}
	return nil
}

func FoobarAdjustVolume(state FoobarPlayerInfo, delta float64, credentials HTTPCredentials) (newVolume float64, isMin, isMax bool, err error) {
	if state.Volume.Current < -50 {
		delta *= 10
	} else if state.Volume.Current < -20 {
		delta *= 5
	} else if state.Volume.Current < -15 {
		delta *= 3
	} else if state.Volume.Current > -10 {
		delta /= 2
	}
	newVolume = math.Max(state.Volume.Min, math.Min(state.Volume.Max, state.Volume.Current+float64(delta)))
	payload := struct {
		Volume float64 `json:"volume"`
	}{
		Volume: newVolume,
	}
	if _, err := foobarRequest("POST", "/api/player", payload, credentials); err != nil {
		return 0, false, false, err
	}
	return payload.Volume, payload.Volume == state.Volume.Min, payload.Volume == state.Volume.Max, nil
}

func FoobarSeekRelative(delta int, credentials HTTPCredentials) error {
	payload := struct {
		RelativePosition int `json:"relativePosition"`
	}{
		RelativePosition: delta,
	}
	if _, err := foobarRequest("POST", "/api/player", payload, credentials); err != nil {
		return err
	}
	return nil
}
