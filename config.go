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
	Port   string
	Foobar apis.FoobarCredentials
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
	if c.Foobar.Username == "" || c.Foobar.Password == "" {
		return errors.New("no foobar credentials specified")
	}
	return nil
}
