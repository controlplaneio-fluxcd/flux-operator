// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// Load reads, validates, and applies default values to missing fields in the configuration
// for the Flux Status Page. If the filename is empty it returns the configuration object
// with default values applied.
func Load(filename string) (*ConfigSpec, error) {
	if filename == "" {
		var confSpec ConfigSpec
		confSpec.ApplyDefaults()
		confSpec.Version = "no-config"
		return &confSpec, nil
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	conf, err := parse(b)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration in web config file '%s': %w", filename, err)
	}
	conf.Version = "static-file"
	return conf, nil
}

func parse(b []byte) (*ConfigSpec, error) {
	var conf Config
	if err := yaml.Unmarshal(b, &conf); err != nil {
		return nil, err
	}
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	conf.Spec.ApplyDefaults()
	return &conf.Spec, nil
}
