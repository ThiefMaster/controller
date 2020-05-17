package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/thiefmaster/controller/apis"
	"gopkg.in/yaml.v2"
)

type appConfig struct {
	Port           string
	Foobar         apis.HTTPCredentials
	NotHub         apis.HTTPCredentials
	Mattermost     apis.MattermostSettings
	TubeRemotePort int `yaml:"tubeRemotePort"`
	Numlock        bool
}

func (c *appConfig) load(path string) error {
	log.Printf("loading config file: %s\n", path)
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not open config file: %v", err)
	}
	if err = yaml.UnmarshalStrict(yamlFile, c); err != nil {
		return fmt.Errorf("could not parse config file: %v", err)
	}
	if err := c.validate(); err != nil {
		return fmt.Errorf("config invalid: %v", err)
	}
	return nil
}

func (c *appConfig) validate() error {
	if c.Port == "" {
		return errors.New("no port specified")
	}
	if c.Foobar.BaseURL == "" {
		return errors.New("no foobar url specified")
	}
	if c.Mattermost.ServerURL != "" {
		if c.Mattermost.AccessToken == "" {
			return errors.New("no mattermost token specified")
		}
		if c.Mattermost.TeamName == "" {
			return errors.New("no mattermost team specified")
		}
		if c.Mattermost.ChannelName == "" {
			return errors.New("no mattermost chammel specified")
		}
	}
	if c.TubeRemotePort != 0 && (c.TubeRemotePort < 1024 || c.TubeRemotePort > 65535) {
		return errors.New("invalid tuberemote port specified")
	}
	return nil
}
