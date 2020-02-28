// Copyright 2019 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"

	"io/ioutil"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

type Options struct {
	// The path of configuration file.
	configFile string
	// The configuration object
	config     *ControllerConfig
	controller string
}

func newOptions() *Options {
	return &Options{
		config: new(ControllerConfig),
	}
}

// addFlags adds flags to fs and binds them to options.
func (o *Options) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.configFile, "config", o.configFile, "The path to the configuration file")
	fs.StringVar(&o.controller, "controller", "ref", "The NP Controller implementation to use (ref or ddlog)")
}

// complete completes all the required options.
func (o *Options) complete(args []string) error {
	if len(o.configFile) > 0 {
		c, err := o.loadConfigFromFile(o.configFile)
		if err != nil {
			return err
		}
		o.config = c
	}
	return nil
}

// validate validates all the required options.
func (o *Options) validate(args []string) error {
	if len(args) != 0 {
		return errors.New("No arguments are supported")
	}
	if o.controller != "ref" && o.controller != "ddlog" {
		return fmt.Errorf("NP Controller implementation '%s' is not supported", o.controller)
	}
	return nil
}

func (o *Options) loadConfigFromFile(file string) (*ControllerConfig, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var c ControllerConfig
	err = yaml.UnmarshalStrict(data, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
