package apis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	client = http.Client{Timeout: 300 * time.Millisecond}
)

const (
	FoobarStateStopped = "stopped"
	FoobarStatePlaying = "playing"
	FoobarStatePaused  = "paused"
)

type FoobarPlayerInfo struct {
	State  string `json:"playbackState"`
	Volume struct {
		Min     float64 `json:"min"`
		Max     float64 `json:"max"`
		Current float64 `json:"value"`
	} `json:"volume"`
}

func foobarRequest(method, path string, payload interface{}) ([]byte, error) {
	var reqBody io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("json marshal failed: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}
	req, err := http.NewRequest(method, "http://localhost:48321"+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("NewRequest failed: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth("foobar", os.Args[1])
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

func GetFoobarState() (FoobarPlayerInfo, error) {
	var status struct {
		Player FoobarPlayerInfo `json:"player"`
	}
	body, err := foobarRequest("GET", "/api/player", nil)
	if err != nil {
		return status.Player, err
	}
	if err := json.Unmarshal(body, &status); err != nil {
		return status.Player, fmt.Errorf("could not parse foobar json: %v", err)
	}
	return status.Player, nil
}

func FoobarNext() error {
	if _, err := foobarRequest("POST", "/api/player/next", nil); err != nil {
		return err
	}
	return nil
}

func FoobarTogglePause() error {
	state, err := GetFoobarState()
	if err != nil {
		return err
	}
	if state.State == FoobarStateStopped {
		if _, err := foobarRequest("POST", "/api/player/play", nil); err != nil {
			return err
		}
		return nil
	}
	if _, err := foobarRequest("POST", "/api/player/pause/toggle", nil); err != nil {
		return err
	}
	return nil
}

func FoobarAdjustVolume(delta float64) (newVolume float64, isMin, isMax bool, err error) {
	state, err := GetFoobarState()
	if err != nil {
		return 0, false, false, err
	}
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
	if _, err := foobarRequest("POST", "/api/player", payload); err != nil {
		return 0, false, false, err
	}
	return payload.Volume, payload.Volume == state.Volume.Min, payload.Volume == state.Volume.Max, nil
}

func FoobarSeekRelative(delta int) error {
	payload := struct {
		RelativePosition int `json:"relativePosition"`
	}{
		RelativePosition: delta,
	}
	if _, err := foobarRequest("POST", "/api/player", payload); err != nil {
		return err
	}
	return nil
}
